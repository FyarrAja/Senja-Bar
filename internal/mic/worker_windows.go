//go:build windows

package mic

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type Config struct {
	Input                string
	Deep                 bool
	Gate                 int
	Enabled              bool
	InputGain            int
	Monitor              bool
	Output               string
	NoiseReduction       int
	MonitorVolume        int
	SmartVoice           bool
	NeuralSuppression    bool
	PreviewSample        bool
	LiveMonitorRequested bool
}
type Status struct {
	Active  bool
	Level   int
	Message string
	Updated time.Time
}

func directory() string { return filepath.Join(os.Getenv("LOCALAPPDATA"), "Senja") }
func Load() Config {
	c := Config{Gate: -38, InputGain: 100, NoiseReduction: 65, MonitorVolume: 70, NeuralSuppression: true}
	b, err := os.ReadFile(filepath.Join(directory(), "mic.json"))
	if err == nil {
		_ = json.Unmarshal(b, &c)
	}
	if c.Gate < -60 || c.Gate > -20 {
		c.Gate = -38
	}
	if c.InputGain < 25 || c.InputGain > 200 {
		c.InputGain = 100
	}
	c.NoiseReduction = max(0, min(100, c.NoiseReduction))
	c.MonitorVolume = max(0, min(100, c.MonitorVolume))
	return c
}
func Save(c Config) error { return writeJSON("mic.json", c) }

// Each new test requires a fresh choice to hear live playback. Never replay a
// persisted monitor setting unexpectedly when the microphone is started again.
func PrepareTest(c Config) Config {
	c.Enabled = true
	c.Monitor = false
	c.PreviewSample = false
	c.LiveMonitorRequested = false
	return c
}

// ToggleLive owns both capture and playback; turning it off leaves no hidden
// microphone test running in the worker.
func ToggleLive(c Config, active bool) Config {
	on := !(active && c.Monitor)
	c.Enabled, c.Monitor, c.LiveMonitorRequested = on, on, on
	c.PreviewSample = false
	if on {
		c.MonitorVolume = min(50, c.MonitorVolume)
	}
	return c
}
func Disable() error {
	c := Load()
	if !c.Enabled {
		return nil
	}
	c.Enabled = false
	c.Monitor, c.PreviewSample, c.LiveMonitorRequested = false, false, false
	return Save(c)
}
func writeJSON(name string, v any) error {
	if err := os.MkdirAll(directory(), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(directory(), name)
	if err = os.WriteFile(path+".tmp", b, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
func ReadStatus() Status {
	if s, ok := readMeter(); ok {
		return s
	}
	var s Status
	b, _ := os.ReadFile(filepath.Join(directory(), "mic-status.json"))
	_ = json.Unmarshal(b, &s)
	// Do not surface the retired external-cable adapter's persisted error.
	if strings.Contains(s.Message, "CABLE") {
		s.Message = "Mikrofon belum aktif"
		s.Active = false
		s.Level = 0
	}
	if time.Since(s.Updated) > 3*time.Second {
		s.Active = false
		s.Level = 0
		if s.Message == "" || strings.HasPrefix(s.Message, "Aktif") || strings.HasPrefix(s.Message, "Uji mic aktif") {
			s.Message = "Mikrofon belum aktif"
		}
	}
	return s
}
func Launch() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(path, "--mic-engine")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err = cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
func RunWorker() {
	k := syscall.NewLazyDLL("kernel32.dll")
	name, _ := syscall.UTF16PtrFromString("Local\\SenjaMicWorker")
	h, _, e := k.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return
	}
	defer k.NewProc("CloseHandle").Call(h)
	if e == syscall.Errno(183) {
		return
	}
	c := Load()
	if !c.Enabled {
		return
	}
	if !c.PreviewSample {
		liveRequested := c.LiveMonitorRequested
		c = PrepareTest(c)
		c.Monitor = liveRequested
	}
	if err := Save(c); err != nil {
		_ = writeJSON("mic-status.json", Status{Message: err.Error(), Updated: time.Now()})
		return
	}
	session, err := startConfigured(c)
	if err != nil {
		_ = writeJSON("mic-status.json", Status{Message: err.Error(), Updated: time.Now()})
		return
	}
	defer func() { session.Stop() }()
	session.SetInputGain(c.InputGain)
	session.SetNoiseReduction(c.NoiseReduction)
	session.SetMonitorVolume(c.MonitorVolume)
	session.SetSmartVoice(c.SmartVoice)
	meter := openMeter(true)
	defer meter.close()
	defer func() { meter.publish(Status{Updated: time.Now()}) }()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	lastConfig := time.Time{}
	lastJSON := time.Time{}
	for {
		s := Status{Active: session.Active(), Level: session.Level(), Message: "Uji mic aktif · pemrosesan lokal Senja", Updated: time.Now()}
		if c.Monitor {
			s.Message = "Dengar langsung aktif · gunakan headset"
		}
		if c.PreviewSample {
			switch session.previewStage.Load() {
			case 0:
				s.Message = "Menyiapkan mic · tunggu sebentar"
			case 1:
				s.Message = "Bicara 3 detik · contoh hanya di RAM"
			case 2:
				s.Message = "Memutar sekali · tangkapan mic dihentikan"
			}
		}
		if !s.Active {
			if err := session.Error(); err != nil {
				s.Message = err.Error()
			} else {
				s.Message = "Mikrofon dihentikan"
				if c.PreviewSample {
					s.Message = "Contoh selesai · tidak ada file disimpan"
				}
			}
		}
		meter.publish(s)
		if !s.Active || time.Since(lastJSON) >= time.Second {
			_ = writeJSON("mic-status.json", s)
			lastJSON = time.Now()
		}
		if !s.Active {
			current := Load()
			current.Enabled, current.Monitor, current.PreviewSample, current.LiveMonitorRequested = false, false, false, false
			_ = Save(current)
			return
		}
		<-ticker.C
		if time.Since(lastConfig) < 200*time.Millisecond {
			continue
		}
		lastConfig = time.Now()
		next := Load()
		if next.LiveMonitorRequested {
			next.LiveMonitorRequested = false
			_ = Save(next)
		}
		if !next.Enabled {
			session.Stop()
			_ = writeJSON("mic-status.json", Status{Message: "Mikrofon dihentikan", Updated: time.Now()})
			return
		}
		if next.Input != c.Input || next.Monitor != c.Monitor || next.NeuralSuppression != c.NeuralSuppression || next.PreviewSample != c.PreviewSample {
			session.Stop()
			replacement, err := startConfigured(next)
			if err != nil {
				_ = writeJSON("mic-status.json", Status{Message: err.Error(), Updated: time.Now()})
				return
			}
			session = replacement
			c = next
		}
		session.Configure(next.Deep, next.Gate)
		session.SetInputGain(next.InputGain)
		session.SetNoiseReduction(next.NoiseReduction)
		session.SetMonitorVolume(next.MonitorVolume)
		session.SetSmartVoice(next.SmartVoice)
	}
}

func startConfigured(c Config) (*Session, error) {
	var input Device
	found := false
	for _, d := range Devices(true) {
		if d.Name == c.Input {
			input = d
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("mikrofon tidak ditemukan; muat ulang perangkat")
	}
	if !c.Monitor && !c.PreviewSample {
		return startSessionWithConfig(input, nil, c.Deep, c.Gate, c.NeuralSuppression, &c)
	}
	// WAVE_MAPPER resolves the current Windows preferred playback device.
	// Old per-PC output names in migrated profiles are deliberately ignored.
	output := defaultPreviewOutput()
	return startSessionWithConfig(input, &output, c.Deep, c.Gate, c.NeuralSuppression, &c)
}

func defaultPreviewOutput() Device {
	return Device{ID: ^uint32(0), Name: "Output default Windows"}
}

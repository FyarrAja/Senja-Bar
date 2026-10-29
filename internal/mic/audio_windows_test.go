//go:build windows

package mic

import (
	"os"
	"strings"
	"testing"
	"time"

	"unsafe"
)

func TestNativeWaveStructLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) == 8 && (unsafe.Sizeof(header{}) != 48 || unsafe.Offsetof(header{}.Flags) != 24) {
		t.Fatal("WAVEHDR does not match Windows x64 ABI")
	}
	if unsafe.Sizeof(caps{}) != 84 || unsafe.Offsetof(format{}.Extra) != 16 {
		t.Fatal("wave format/caps ABI mismatch")
	}
}

// Explicit opt-in: briefly monitor processed audio only through an enumerated
// physical headset. Never fall back to a speaker or the system default.
func TestLocalHeadsetMonitor(t *testing.T) {
	if os.Getenv("SENJA_MONITOR_INTEGRATION") != "1" {
		t.Skip("explicit local headset monitor test")
	}
	var input, output Device
	foundInput, foundOutput := false, false
	for _, d := range Devices(true) {
		if strings.Contains(strings.ToLower(d.Name), "rexus") {
			input = d
			foundInput = true
			break
		}
	}
	for _, d := range Devices(false) {
		if DeviceKind(d.Name, false) == "headset" {
			output = d
			foundOutput = true
			break
		}
	}
	if !foundInput || !foundOutput {
		t.Skip("Rexus and physical headset required")
	}
	var session *Session
	var err error
	if os.Getenv("SENJA_NEURAL_DLL") != "" {
		session, err = startSession(input, &output, true, -38, true)
	} else {
		session, err = StartMonitor(input, output, true, -38)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer session.Stop()
	session.SetInputGain(100)
	session.SetSmartVoice(true)
	session.SetNoiseReduction(85)
	session.SetMonitorVolume(35)
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	<-deadline.C
	if !session.Active() {
		t.Fatalf("monitor stopped: %v", session.Error())
	}
	t.Logf("processed monitor active: %s -> %s, level=%d%%; playback 35%%", input.Name, output.Name, session.Level())
}
func TestLiveDeviceEnumeration(t *testing.T) {
	if os.Getenv("SENJA_NATIVE_INTEGRATION") != "1" {
		t.Skip("read-only native device enumeration")
	}
	in, out := Devices(true), Devices(false)
	if len(in) == 0 || len(out) == 0 {
		t.Fatalf("no audio endpoints input=%d output=%d", len(in), len(out))
	}
	t.Logf("input=%v output=%v", in, out)
}

func TestWorkerDoesNotRestoreLivePlaybackAtStartup(t *testing.T) {
	if os.Getenv("SENJA_MONITOR_INTEGRATION") != "1" {
		t.Skip("explicit real capture startup test")
	}
	var input Device
	for _, d := range Devices(true) {
		if strings.Contains(strings.ToLower(d.Name), "rexus") {
			input = d
			break
		}
	}
	if input.Name == "" {
		t.Skip("Rexus required")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	c := Config{Input: input.Name, Output: "missing output must never be opened", Enabled: true, Monitor: true, MonitorVolume: 100, InputGain: 100, Gate: -38, Deep: true, SmartVoice: true, NeuralSuppression: os.Getenv("SENJA_NEURAL_DLL") != ""}
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { RunWorker(); close(done) }()
	defer func() {
		_ = Disable()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("worker did not release capture")
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s := ReadStatus(); s.Active {
			if Load().Monitor {
				t.Fatal("persisted playback was restored")
			}
			t.Log("worker started capture with missing output ignored; playback disabled")
			return
		}
		select {
		case <-done:
			t.Fatalf("startup failed: %+v", ReadStatus())
		default:
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("worker failed to start")
}

func TestSamplePreviewCapturesThenPlaysOnce(t *testing.T) {
	if os.Getenv("SENJA_MONITOR_INTEGRATION") != "1" {
		t.Skip("explicit local audio preview test")
	}
	var input, output Device
	for _, d := range Devices(true) {
		if strings.Contains(strings.ToLower(d.Name), "rexus") {
			input = d
			break
		}
	}
	for _, d := range Devices(false) {
		if DeviceKind(d.Name, false) == "headset" {
			output = d
			break
		}
	}
	if input.Name == "" || output.Name == "" {
		t.Skip("physical Rexus and headset required")
	}
	c := Config{Input: input.Name, Output: "deleted headset from an old PC", PreviewSample: true, InputGain: 100, MonitorVolume: 35, SmartVoice: true, NoiseReduction: 85, Deep: true, Gate: -38, NeuralSuppression: os.Getenv("SENJA_NEURAL_DLL") != ""}
	started := time.Now()
	s, err := startConfigured(c)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	seenCapture, seenPlayback := false, false
	lastStage := int32(0)
	for s.Active() && time.Since(started) < 10*time.Second {
		stage := s.previewStage.Load()
		if stage < lastStage {
			t.Fatal("preview looped back to capture during playback")
		}
		lastStage = stage
		seenCapture = seenCapture || stage == 1
		seenPlayback = seenPlayback || stage == 2
		time.Sleep(10 * time.Millisecond)
	}
	if s.Active() || s.Error() != nil || !seenCapture || !seenPlayback {
		t.Fatalf("preview active=%v err=%v capture=%v playback=%v", s.Active(), s.Error(), seenCapture, seenPlayback)
	}
	if time.Since(started) < 6*time.Second {
		t.Fatal("preview did not capture and play full three-second sample")
	}
	if writes := s.playbackSubmissions.Load(); writes != 1 {
		t.Fatalf("preview split or repeated: %d playback submissions", writes)
	}
	t.Logf("single capture/play cycle finished in %s; capture stops before playback", time.Since(started))
}

func TestExplicitLiveMonitorStartsAndStopsWholeWorker(t *testing.T) {
	if os.Getenv("SENJA_MONITOR_INTEGRATION") != "1" {
		t.Skip("explicit real headset monitoring")
	}
	var input Device
	for _, d := range Devices(true) {
		if strings.Contains(strings.ToLower(d.Name), "rexus") {
			input = d
			break
		}
	}
	if input.Name == "" {
		t.Skip("physical Rexus required")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	c := Config{Input: input.Name, Enabled: true, Monitor: true, LiveMonitorRequested: true, MonitorVolume: 35, InputGain: 100, Gate: -38, Deep: true, SmartVoice: true, NeuralSuppression: os.Getenv("SENJA_NEURAL_DLL") != ""}
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { RunWorker(); close(done) }()
	defer func() {
		_ = Disable()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("live worker failed to stop")
		}
	}()
	waitFor := func(want string) {
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			s := ReadStatus()
			if s.Active && s.Message == want {
				return
			}
			select {
			case <-done:
				t.Fatalf("worker stopped: %+v", s)
			default:
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatalf("did not reach %s: %+v", want, ReadStatus())
	}
	waitFor("Dengar langsung aktif · gunakan headset")
	if current := Load(); !current.Monitor || current.LiveMonitorRequested {
		t.Fatal("explicit live request was not consumed correctly")
	}
	time.Sleep(3 * time.Second)
	if !ReadStatus().Active {
		t.Fatal("live monitoring stopped unexpectedly")
	}
	c = ToggleLive(Load(), true)
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("toggle off left the microphone worker running")
	}
	if Load().Enabled || ReadStatus().Active {
		t.Fatal("capture/playback still active after toggle off")
	}
	t.Log("explicit live playback active, then toggle off stopped the complete audio worker")
}

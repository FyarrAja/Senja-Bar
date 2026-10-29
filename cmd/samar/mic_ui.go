//go:build windows

package main

import (
	"fmt"
	"samar/internal/mic"
	"strings"
	"time"
)

const cmdMic = 118

type nativeMic struct {
	config                            mic.Config
	devices                           []mic.Device
	loaded, details                   bool
	message                           string
	pending                           time.Time
	status                            mic.Status
	level, reveal                     nativeMotion
	monitorMotion                     nativeMotion
	dropdown, offset, closingDropdown int
}

func init() {
	nativeControlCommands[48] = cmdMic
	nativeControlBoxes[48] = [4]int{-104, 252, 96, 42}
	for i := 49; i <= 55; i++ {
		nativeControlCommands[i] = 800 + uint32(i-49)
	}
	for i := 56; i <= 63; i++ {
		nativeControlCommands[i] = 820 + uint32(i-56)
	}
	nativeControlCommands[64] = 807
	nativeControlCommands[65] = 808
	nativeControlCommands[66] = 809
	nativeControlCommands[67] = 810
	nativeControlCommands[68] = 811
	nativeControlCommands[69] = 812
	nativeControlCommands[70] = 813
}
func (n *nativeGlassPanel) layoutMicControls() {
	nativeControlBoxes[49] = [4]int{20, 92, 318, 44}
	nativeControlBoxes[65] = [4]int{20, 144, 318, 36}
	nativeControlBoxes[64] = [4]int{20, 244, 318, 24}
	nativeControlBoxes[50] = [4]int{20, 268, 318, 24}
	nativeControlBoxes[51] = [4]int{20, 292, 318, 24}
	nativeControlBoxes[68] = [4]int{20, 316, 318, 24}
	nativeControlBoxes[70] = [4]int{20, 348, 318, 44}
	nativeControlBoxes[66] = [4]int{}
	nativeControlBoxes[67] = [4]int{}
	nativeControlBoxes[52] = [4]int{}
	nativeControlBoxes[53] = [4]int{}
	nativeControlBoxes[54] = [4]int{}
	nativeControlBoxes[55] = [4]int{309, 20, 29, 30}
	nativeControlBoxes[69] = [4]int{252, 20, 49, 30}
	n.layoutMicDropdown()
}
func (n *nativeGlassPanel) loadMic() {
	m := &n.mic
	m.config = mic.Load()
	m.devices = mic.Devices(true)
	m.loaded = true
	m.status = mic.ReadStatus()
	if m.config.Input == "" {
		for _, d := range m.devices {
			if strings.Contains(strings.ToLower(d.Name), "rexus") {
				m.config.Input = d.Name
				break
			}
		}
	}
	if m.config.Input == "" && len(m.devices) > 0 {
		m.config.Input = m.devices[0].Name
	}
}
func (n *nativeGlassPanel) micAction(command uint32) bool {
	if command == cmdMic {
		n.micPage = true
		n.systemPage = false
		n.resourcePage = false
		n.resources.optionOpen = 0
		n.focus = -1
		if !n.mic.loaded {
			n.loadMic()
		}
		user32.NewProc("SetTimer").Call(n.hwnd, 0x534d, 50, 0)
	} else if n.micPage && command >= 820 && command <= 827 {
		n.selectMicOption(int(command - 820))
		return true
	} else if !n.micPage || command < 800 || command > 813 {
		return false
	} else {
		m := &n.mic
		m.message = ""
		status := mic.ReadStatus()
		switch command {
		case 813:
			if n.preview {
				break
			}
			m.config = mic.ToggleLive(mic.Load(), status.Active)
			if err := mic.Save(m.config); err != nil {
				m.message = err.Error()
			} else if !status.Active {
				if err := mic.Launch(); err != nil {
					m.message = err.Error()
				}
			}
		case 800:
			if !status.Active {
				m.devices = mic.Devices(true)
				n.openMicDropdown(1)
			}
		case 801:
			n.openMicDropdown(4)
		case 802:
			n.openMicDropdown(2)
		case 807:
			n.openMicDropdown(3)
		case 808:
			n.openMicDropdown(5)
		case 809:
			{
				if n.preview {
					break
				}
				if status.Active && m.config.PreviewSample {
					break
				}
				m.config.Monitor = false
				m.config.LiveMonitorRequested = false
				m.config.PreviewSample = true
				m.config.Enabled = true
				if err := mic.Save(m.config); err != nil {
					m.message = err.Error()
				} else if !status.Active {
					if err := mic.Launch(); err != nil {
						m.message = err.Error()
					}
				}
			}
		case 811:
			n.openMicDropdown(7)
		case 812:
			m.config.SmartVoice = !m.config.SmartVoice
			if err := mic.Save(m.config); err != nil {
				m.message = err.Error()
			}
		case 803:
			if n.preview {
				m.message = "Preview · pengambilan suara tidak dijalankan"
				break
			}
			m.config.Enabled = !status.Active
			if m.config.Enabled {
				m.config = mic.PrepareTest(m.config)
			} else {
				m.config.Monitor, m.config.PreviewSample, m.config.LiveMonitorRequested = false, false, false
			}
			if err := mic.Save(m.config); err != nil {
				m.message = err.Error()
				break
			}
			if m.config.Enabled {
				if err := mic.Launch(); err != nil {
					m.message = err.Error()
				} else {
					m.message = "Menyiapkan uji mic…"
					m.pending = time.Now()
				}
			}
		case 806:
			m.details = !m.details
		}
		if m.dropdown == 0 {
			n.focus = -1
		}
	}
	user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
	return true
}
func (n *nativeGlassPanel) drawMic(ink, muted, card, group uint32) {
	n.layoutMicControls()
	m := &n.mic
	s := m.status
	if !m.pending.IsZero() && (s.Updated.After(m.pending) || time.Since(m.pending) > 5*time.Second) {
		m.pending = time.Time{}
		m.message = ""
	}
	n.text("Mikrofon", 20, 20, 220, 36, 26, true, ink)
	n.text("Bentuk suara, lalu dengarkan hasilnya", 20, 61, 318, 23, 12, false, muted)
	saved := n.beginPress(55)
	n.round(309, 20, 29, 30, 10, card, false)
	info := "?"
	if m.details {
		info = "‹"
	}
	n.drawText(info, 309, 20, 29, 30, 14, true, muted, true)
	n.endPress(saved)
	if !m.details {
		saved = n.beginPress(69)
		fill := card
		if m.config.SmartVoice {
			fill = 0xff2f77e8
		}
		n.round(252, 20, 49, 30, 10, fill, false)
		n.drawText("Auto", 252, 20, 49, 30, 11, true, ink, true)
		n.endPress(saved)
	}
	if m.details {
		n.text("Suara diproses di perangkat ini", 20, 112, 318, 28, 16, true, ink)
		n.drawText("RNNoise meredam noise secara lokal. Auto memakai deteksi ucapan dan penguatan tetap supaya volume tidak mengejar tiap suku kata. Deep menambah body bass, tanpa mengubah pitch. Belum ada pemisah identitas pembicara.", 20, 154, 318, 120, 13, false, muted, false)
		n.line(20, 292, 338, 292, card, 1)
		n.text("Mikrofon untuk aplikasi lain", 20, 310, 318, 27, 16, true, ink)
		n.drawText("Aktifkan Dengar suara sendiri untuk menjalankan audio ke output default Windows. Matikan untuk menghentikan audio sepenuhnya. Gunakan headset; ada jeda pemrosesan. Belum menjadi input FiveM/Discord.", 20, 352, 318, 110, 13, false, muted, false)
		n.text("Tekan ‹ untuk kembali", 20, 486, 318, 24, 12, false, muted)
		return
	}
	saved = n.beginPress(49)
	n.round(20, 92, 318, 44, 12, card, false)
	name := m.config.Input
	if name == "" {
		name = "Pilih mikrofon"
	}
	n.drawDeviceIcon(mic.DeviceKind(name, true), 32, 103, muted)
	n.text(name, 57, 97, 241, 21, 12, true, ink)
	caption := fmt.Sprintf("Input · redam latar %d%%", m.config.NoiseReduction)
	if m.config.SmartVoice {
		caption = fmt.Sprintf("Auto suara · redam latar %d%%", m.config.NoiseReduction)
	}
	if m.config.NeuralSuppression {
		caption = "RNNoise · filter ucapan"
		if m.config.SmartVoice {
			caption = "RNNoise · Auto suara"
		}
	}
	n.text(caption, 57, 117, 240, 15, 10, false, muted)
	n.text("⌄", 311, 102, 20, 24, 16, false, muted)
	n.endPress(saved)
	saved = n.beginPress(65)
	n.round(20, 144, 318, 36, 10, card, false)
	preset := "Kustom"
	if index := mic.PresetIndex(m.config); index >= 0 {
		preset = mic.Presets[index].Name
	}
	n.text("Preset suara", 33, 152, 128, 22, 12, true, ink)
	n.drawPresetIcon(mic.PresetIndex(m.config), 173, 152, muted)
	n.text(preset, 197, 152, 116, 22, 12, true, ink)
	n.drawMicChevron(321, 159, muted)
	n.endPress(saved)
	label := "Mikrofon nonaktif"
	if s.Active {
		label = "Mikrofon aktif"
	}
	n.text(label, 20, 188, 220, 22, 11, true, muted)
	n.drawText(micDBLabel(m.level.value), 266, 188, 72, 22, 11, true, muted, true)
	n.drawMicMeter(20, 212, 318, 26, m.level.value, card)
	for i, label := range []string{"Level input", "Redam saat diam", "Karakter suara", "Volume dengar"} {
		y := 244 + i*24
		control := 64
		value := fmt.Sprintf("%d%%", m.config.InputGain)
		if i == 1 {
			control = 50
			value = fmt.Sprintf("%d dB", m.config.Gate)
			if m.config.SmartVoice {
				label = "Gate adaptif"
				value = "Otomatis"
				if m.config.NeuralSuppression {
					label = "Gate ucapan"
					value = "Otomatis"
				}
			}
		}
		if i == 2 {
			control = 51
			value = "Natural"
			if m.config.Deep {
				value = "Bass"
			}
		}
		if i == 3 {
			control = 68
			value = fmt.Sprintf("%d%%", m.config.MonitorVolume)
		}
		saved = n.beginPress(control)
		n.text(label, 20, y+3, 190, 21, 12, true, ink)
		n.drawText(value+"  ⌄", 230, y, 108, 24, 12, true, muted, true)
		if i < 3 {
			n.line(20, y+23, 338, y+23, card, 1)
		}
		n.endPress(saved)
	}
	saved = n.beginPress(70)
	n.round(20, 348, 318, 44, 10, card, false)
	n.drawDeviceIcon("headset", 32, 359, muted)
	n.text("Dengar suara sendiri", 57, 353, 216, 21, 12, true, ink)
	n.text("Gunakan headset · ada jeda pemrosesan", 57, 373, 216, 15, 10, false, muted)
	position := 0.0
	if m.config.Monitor && s.Active {
		position = 17
	}
	n.move(&m.monitorMotion, position)
	n.round(289, 360, 36, 20, 10, nativeSwitchTrack(m.monitorMotion.value, n.light), false)
	n.roundFloat(292+float32(m.monitorMotion.value)*13/17, 363, 14, 14, 7, 0xffffffff, false, 1)
	n.endPress(saved)
	n.text("Mengikuti output default Windows", 20, 405, 318, 18, 11, false, muted)
	message := "Aktifkan dengar suara sendiri untuk mulai."
	if s.Active && m.config.Monitor {
		message = s.Message
	}
	if m.message != "" {
		message = m.message
	} else if !s.Active && s.Message != "" && s.Message != "Mikrofon belum aktif" && s.Message != "Mikrofon dihentikan" {
		message = s.Message
	}
	n.drawText(message, 20, 438, 318, 36, 11, false, muted, false)
	n.text("Pemrosesan lokal · belum masuk ke FiveM/Discord", 20, 488, 318, 20, 10, false, muted)
	n.drawMicDropdown(ink, muted)
}
func (n *nativeGlassPanel) pollMic() bool {
	previous := n.mic.status
	n.mic.status = mic.ReadStatus()
	s := n.mic.status
	if previous.Active && !s.Active {
		n.mic.config = mic.Load()
	}
	changed := s.Active != previous.Active || micMeterBin(s.Level) != micMeterBin(previous.Level) || s.Message != previous.Message
	if !n.mic.pending.IsZero() && time.Since(n.mic.pending) > 5*time.Second {
		changed = true
	}
	level := float64(n.mic.status.Level)
	if !n.mic.status.Active {
		level = 0
	}
	if !n.mic.level.ready {
		n.mic.level.set(0, time.Now(), true)
	}
	// Meter telemetry already arrives at 20Hz. Do not launch a 120Hz full
	// layered-window animation for each audio peak; controls keep their own
	// smooth hover/switch/navigation animations.
	n.mic.level.set(level, time.Now(), true)
	return changed
}

func micMeterBin(level int) int { return int(micMeterFraction(float64(level)) * 64) }

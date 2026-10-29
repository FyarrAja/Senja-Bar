//go:build windows

package main

import (
	"fmt"
	"samar/internal/mic"
	"time"
)

var micGains = []int{50, 75, 100, 125, 150}
var micGates = []int{-50, -44, -38, -32, -26}
var micMonitorVolumes = []int{25, 50, 70, 85, 100}

func micDropdownLimit(kind int) int {
	if kind == 5 {
		return 7
	}
	return 6
}
func micDropdownStep(kind int) int {
	if kind == 5 {
		return 44
	}
	return 36
}
func (n *nativeGlassPanel) monitorChoices() []int {
	if n.mic.config.Monitor {
		return []int{25, 35, 50}
	}
	return micMonitorVolumes
}

func (n *nativeGlassPanel) micOptions(kind int) []string {
	switch kind {
	case 1:
		out := make([]string, len(n.mic.devices))
		for i, d := range n.mic.devices {
			out[i] = d.Name
		}
		return out
	case 2:
		return []string{"Natural", "Bass"}
	case 3:
		return []string{"50%", "75%", "100% · standar", "125%", "150%"}
	case 4:
		return []string{"−50 dB · lebih sensitif", "−44 dB", "−38 dB · seimbang", "−32 dB", "−26 dB · redam lebih kuat"}
	case 5:
		out := make([]string, len(mic.Presets))
		for i, p := range mic.Presets {
			out[i] = p.Name
		}
		return out
	case 7:
		var values []string
		for _, v := range n.monitorChoices() {
			values = append(values, fmt.Sprintf("%d%%", v))
		}
		return values
	}
	return nil
}
func (n *nativeGlassPanel) micSelected(kind int) int {
	switch kind {
	case 1:
		for i, d := range n.mic.devices {
			if d.Name == n.mic.config.Input {
				return i
			}
		}
	case 2:
		if n.mic.config.Deep {
			return 1
		}
		return 0
	case 3:
		for i, v := range micGains {
			if v == n.mic.config.InputGain {
				return i
			}
		}
	case 4:
		for i, v := range micGates {
			if v == n.mic.config.Gate {
				return i
			}
		}
	case 5:
		return mic.PresetIndex(n.mic.config)
	case 7:
		for i, v := range n.monitorChoices() {
			if v == n.mic.config.MonitorVolume {
				return i
			}
		}
	}
	return -1
}
func (n *nativeGlassPanel) openMicDropdown(kind int) {
	m := &n.mic
	if len(n.micOptions(kind)) == 0 {
		m.message = "Mic belum ditemukan · sambungkan lalu buka pilihan input"
		return
	}
	m.dropdown, m.closingDropdown, m.offset = kind, 0, 0
	selected := n.micSelected(kind)
	if selected >= 6 && kind != 5 {
		m.offset = selected / 6 * 6
	}
	m.reveal.set(0, time.Now(), true)
	n.move(&m.reveal, 1)
	n.focus = 56 + max(0, selected-m.offset)
	n.layoutMicDropdown()
}
func (n *nativeGlassPanel) closeMicDropdown() {
	m := &n.mic
	m.closingDropdown, m.dropdown = m.dropdown, 0
	n.move(&m.reveal, 0)
	n.focus = -1
	user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
}
func (n *nativeGlassPanel) micDropdownGeometry(kind int) (int, int, int) {
	count := min(micDropdownLimit(kind), len(n.micOptions(kind))-n.mic.offset)
	height := count*micDropdownStep(kind) + 12
	if len(n.micOptions(kind)) > micDropdownLimit(kind) {
		height += 34
	}
	y := 144
	if kind == 2 {
		y = 344
	}
	if kind == 3 {
		y = 280
	}
	if kind == 4 {
		y = 312
	}
	if kind == 5 {
		y = 184
	}
	if kind == 7 {
		y = 344
	}
	y = min(y, 504-height)
	return y, height, count
}
func (n *nativeGlassPanel) layoutMicDropdown() {
	if n.mic.dropdown == 0 {
		return
	}
	y, _, count := n.micDropdownGeometry(n.mic.dropdown)
	for i := 0; i < micDropdownLimit(n.mic.dropdown); i++ {
		nativeControlBoxes[56+i] = [4]int{24, y + 6 + i*micDropdownStep(n.mic.dropdown), 310, micDropdownStep(n.mic.dropdown)}
	}
	if n.mic.dropdown != 5 {
		nativeControlBoxes[62] = [4]int{28, y + 6 + count*36, 145, 30}
		nativeControlBoxes[63] = [4]int{181, y + 6 + count*36, 145, 30}
	}
}
func (n *nativeGlassPanel) selectMicOption(row int) {
	m := &n.mic
	if m.dropdown == 0 {
		return
	}
	if row >= 6 && m.dropdown != 5 {
		delta := -6
		if row == 7 {
			delta = 6
		}
		m.offset = max(0, min((len(n.micOptions(m.dropdown))-1)/6*6, m.offset+delta))
		n.focus = 56
		n.layoutMicDropdown()
	} else {
		index := m.offset + row
		if index >= len(n.micOptions(m.dropdown)) {
			return
		}
		switch m.dropdown {
		case 1:
			m.config.Input = m.devices[index].Name
		case 2:
			m.config.Deep = index == 1
		case 3:
			m.config.InputGain = micGains[index]
		case 4:
			m.config.Gate = micGates[index]
			m.config.SmartVoice = false
		case 5:
			m.config = mic.ApplyPreset(m.config, index)
		case 7:
			m.config.MonitorVolume = n.monitorChoices()[index]
		}
		if err := mic.Save(m.config); err != nil {
			m.message = fmt.Sprint(err)
		}
		n.closeMicDropdown()
	}
	user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
}
func (n *nativeGlassPanel) drawMicDropdown(ink, muted uint32) {
	m := &n.mic
	kind := m.dropdown
	if kind == 0 {
		kind = m.closingDropdown
	}
	if kind == 0 || m.reveal.value < .01 {
		return
	}
	y, height, count := n.micDropdownGeometry(kind)
	reveal := float32(max(0., min(1., m.reveal.value)))
	var saved uint32
	// Clip in the same projected coordinate system as the panel controls.
	saved = n.beginPress(-1)
	nativeG("GdipSetClipRect", n.graphics, nativeF(n.projectX(18)), nativeF(float32(y-2)), nativeF(n.projectX(322)), nativeF(float32(height+4)*reveal), 1)
	background, edge, selection := uint32(0xff24272d), uint32(0xff3b4048), uint32(0xff2c3c52)
	if n.light {
		background, edge, selection = 0xfffafbfc, 0xffd9dfe7, 0xffe8f0fc
	}
	n.round(18, y-2, 322, height+4, 13, 0x5010151b, false)
	n.round(20, y, 318, height, 12, background, false)
	n.round(20, y, 318, height, 12, edge, true)
	options := n.micOptions(kind)
	selected := n.micSelected(kind)
	for i := 0; i < count; i++ {
		index := m.offset + i
		step := micDropdownStep(kind)
		top := y + 6 + i*step
		if index == selected || n.keyboardFocus && n.focus == 56+i {
			n.round(25, top+1, 308, step-2, 7, selection, false)
		}
		textX, textW := 34, 268
		if kind == 1 {
			n.drawDeviceIcon(mic.DeviceKind(options[index], kind == 1), 33, top+8, muted)
			textX, textW = 56, 244
		}
		if kind == 5 {
			n.drawPresetIcon(index, 33, top+11, muted)
			n.text(options[index], 58, top+3, 235, 21, 12, true, ink)
			n.text(mic.Presets[index].Detail, 58, top+23, 235, 17, 10, false, muted)
		} else {
			n.text(options[index], textX, top+8, textW, 22, 12, index == selected, ink)
		}
		if index == selected {
			cy := top + step/2
			n.line(309, cy, 312, cy+3, 0xff8ab7ff, 1.6)
			n.line(312, cy+3, 318, cy-4, 0xff8ab7ff, 1.6)
		}
	}
	if len(options) > micDropdownLimit(kind) {
		previousColor, nextColor := muted, muted
		if m.offset == 0 {
			previousColor = 0xff7d838e
		}
		if m.offset+6 >= len(options) {
			nextColor = 0xff7d838e
		}
		n.drawText("‹ Sebelumnya", 28, y+6+count*36, 145, 30, 11, true, previousColor, true)
		n.drawText("Berikutnya ›", 181, y+6+count*36, 145, 30, 11, true, nextColor, true)
	}
	n.endPress(saved)
}

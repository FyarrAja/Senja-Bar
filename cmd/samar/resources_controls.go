//go:build windows

package main

import (
	"fmt"
	"time"
)

func resourceOptionValues(command uint32) []int {
	switch command {
	case 315:
		return []int{75, 80, 85, 90, 95}
	case 316:
		return []int{2, 3, 5, 10}
	case 317:
		return []int{15, 30, 45, 60, 120}
	case 320:
		return []int{1, 0}
	}
	return nil
}
func (n *nativeGlassPanel) openResourceOptions(command uint32) {
	n.layoutResourceControls()
	n.resources.optionOpen = command
	values := resourceOptionValues(command)
	x := 206
	box := 27 + int(command-315)
	if command == 320 {
		box = 37
	}
	step := 28
	if command == 320 {
		step = 48
	}
	height := len(values)*step + 12
	y := nativeControlBoxes[box][1] + nativeControlBoxes[box][3] + 6
	if y+height > 540 {
		y = nativeControlBoxes[box][1] - height - 6
	}
	w := 124
	if command == 320 {
		x = 20
		y = 176
		w = 310
	}
	for i := 0; i < 5; i++ {
		nativeControlBoxes[30+i] = [4]int{x + 4, y + 6 + i*step, w, step}
		nativeControlCommands[30+i] = 330 + uint32(i)
	}
	n.focus = 30
	selected := n.resources.config.MemoryThreshold()
	if command == 316 {
		selected = int(n.resources.config.IdleWait() / time.Minute)
	}
	if command == 317 {
		selected = int(n.resources.config.TrimWait() / time.Minute)
	}
	if command == 320 {
		selected = 0
		if n.resources.config.AllBackground {
			selected = 1
		}
	}
	for i, value := range values {
		if value == selected {
			n.focus = 30 + i
			break
		}
	}
	user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
}
func (n *nativeGlassPanel) layoutResourceControls() {
	nativeControlBoxes[35] = [4]int{20, 84, 159, 34}
	nativeControlBoxes[36] = [4]int{179, 84, 159, 34}
	nativeControlBoxes[37] = [4]int{20, 132, 318, 38}
	nativeControlBoxes[25] = [4]int{20, 180, 318, 29}
	nativeControlBoxes[26] = [4]int{310, 180, 28, 29}
	for i := 0; i < 6; i++ {
		nativeControlBoxes[12+i] = [4]int{20, 216 + i*32, 318, 32}
	}
	nativeControlBoxes[18] = [4]int{20, 351, 68, 25}
	nativeControlBoxes[19] = [4]int{270, 351, 68, 25}
	for i := 0; i < 3; i++ {
		nativeControlBoxes[27+i] = [4]int{20, 180 + i*44, 318, 44}
	}
	nativeControlBoxes[20] = [4]int{20, 344, 318, 44}
	nativeControlBoxes[21] = [4]int{20, 388, 318, 44}
	nativeControlBoxes[22] = [4]int{20, 400, 206, 44}
	if n.resources.config.AllBackground {
		nativeControlBoxes[22][2] = 318
	}
	nativeControlBoxes[23] = [4]int{236, 400, 102, 44}
	nativeControlBoxes[24] = [4]int{246, 464, 92, 32}
}

func init() {
	nativeControlBoxes[37] = [4]int{20, 377, 318, 32}
	nativeControlCommands[37] = 320
	nativeControlBoxes[35] = [4]int{20, 500, 220, 32}
	nativeControlCommands[35] = 319
	nativeControlBoxes[36] = [4]int{20, 515, 318, 28}
	nativeControlCommands[36] = 318
	nativeControlBoxes[20] = [4]int{20, 344, 318, 44}
	nativeControlBoxes[21] = [4]int{20, 388, 318, 44}
	nativeControlBoxes[22] = [4]int{20, 445, 206, 44}
	nativeControlBoxes[23] = [4]int{236, 445, 102, 44}
	nativeControlBoxes[24] = [4]int{252, 500, 86, 32}
	for i := 0; i < 3; i++ {
		nativeControlBoxes[27+i] = [4]int{20, 409 + i*32, 318, 32}
		nativeControlCommands[27+i] = 315 + uint32(i)
	}
}
func (n *nativeGlassPanel) resourceControlIcon(cpu bool, x, y int, color uint32) {
	if cpu {
		n.round(x+3, y+3, 14, 14, 3, color, true)
		n.round(x+7, y+7, 6, 6, 1, color, true)
		for k := 0; k < 3; k++ {
			p := 6 + k*4
			n.line(x+p, y, x+p, y+2, color, 1)
			n.line(x+p, y+18, x+p, y+20, color, 1)
			n.line(x, y+p, x+2, y+p, color, 1)
			n.line(x+18, y+p, x+20, y+p, color, 1)
		}
	} else {
		n.round(x, y+4, 20, 12, 3, color, true)
		for k := 0; k < 3; k++ {
			n.round(x+3+k*5, y+7, 3, 5, 1, color, false)
			n.line(x+4+k*5, y+17, x+4+k*5, y+20, color, 1)
		}
	}
}
func (n *nativeGlassPanel) drawResourceSwitches(top int, ink, muted, group uint32) {
	c := n.resources.config
	divider := uint32(0x12ffffff)
	if n.light {
		divider = 0x16000000
	}
	n.round(20, top, 318, 88, 20, group, false)
	n.line(61, top+44, 324, top+44, divider, 1)
	for j, label := range []string{"RAM otomatis", "Hemat CPU latar"} {
		y := top + j*44
		saved := n.beginPress(20 + j)
		n.resourceControlIcon(j == 1, 33, y+12, muted)
		n.text(label, 64, y+11, 176, 27, 14, true, ink)
		on := c.Automatic
		if j == 1 {
			on = c.EcoCPU
		}
		status := "OFF"
		statusInk := muted
		if on {
			status = "ON"
			statusInk = 0xffa9cbff
		}
		n.drawText(status, 242, y+10, 36, 25, 10, true, statusInk, true)
		pos := n.switches[3+j]
		n.round(284, y+10, 42, 25, 12, nativeSwitchTrack(pos, n.light), false)
		n.round(287+int(pos+.5), y+13, 19, 19, 9, 0xffffffff, false)
		n.endPress(saved)
	}
}
func (n *nativeGlassPanel) drawResourceControls(ink, muted, card, group uint32) {
	saved := n.beginPress(22)
	fill, text := uint32(0xff2f77f4), uint32(0xffffffff)
	if !n.controlEnabled(22) {
		fill, text = card, muted&0xffffff|0x88000000
	}
	width := nativeControlBoxes[22][2]
	n.round(20, 400, width, 44, 13, fill, false)
	n.resourceControlIcon(false, 36, 412, text)
	label := "Trim RAM"
	if n.resources.config.AllBackground {
		label = "Trim semua"
	}
	n.drawText(label, 64, 400, width-74, 44, 13, true, text, true)
	n.endPress(saved)
	if !n.resources.config.AllBackground {
		saved = n.beginPress(23)
		n.round(236, 400, 102, 44, 13, card, false)
		n.drawText("Muat ulang", 236, 400, 102, 44, 12, true, ink, true)
		n.endPress(saved)
	}
	n.drawText(n.resourceStatus(), 20, 460, 318, 64, 11, false, muted, false)
	n.drawResourceOptions(ink, muted)
}

func (n *nativeGlassPanel) drawResourceScope(ink, muted, card uint32) {
	saved := n.beginPress(37)
	n.round(20, 132, 318, 38, 12, card, false)
	label := "Pilih aplikasi"
	if n.resources.config.AllBackground {
		label = "Semua aplikasi latar"
	}
	n.drawScopeIcon(n.resources.config.AllBackground, 34, 142, muted)
	n.text(label, 61, 140, 246, 27, 13, true, ink)
	n.line(315, 148, 320, 153, muted, 1.3)
	n.line(320, 153, 325, 148, muted, 1.3)
	n.endPress(saved)
}
func (n *nativeGlassPanel) drawResourceTabs(ink, muted, card uint32) {
	n.line(20, 118, 338, 118, card, 1)
	target := 42.
	if n.resources.tuning {
		target += 159
	}
	n.move(&n.tabMotion, target)
	n.roundFloat(float32(n.tabMotion.value), 116, 115, 2.5, 1.25, 0xff77a8ff, false, 1)
	for i, label := range []string{"Aplikasi", "Otomatis"} {
		x := 20 + i*159
		saved := n.beginPress(35 + i)
		selected := n.resources.tuning == (i == 1)
		color := muted
		if selected {
			color = ink
		}
		n.drawText(label, x, 84, 159, 32, 13, selected, color, true)
		n.endPress(saved)
	}
}

func (n *nativeGlassPanel) drawResourceTuning(ink, muted, card, group uint32) {
	c := n.resources.config
	scope := "Berlaku untuk aplikasi pilihan"
	if c.AllBackground {
		scope = "Berlaku untuk semua aplikasi latar"
	}
	n.text(scope, 20, 138, 318, 26, 12, false, muted)
	n.round(20, 180, 318, 132, 12, group, false)
	labels := []string{"Mulai saat RAM", "Waktu idle", "Jeda antar-trim"}
	values := []string{fmt.Sprintf("%d%%", c.MemoryThreshold()), fmt.Sprintf("%d menit", int(c.IdleWait()/time.Minute)), fmt.Sprintf("%d menit", int(c.TrimWait()/time.Minute))}
	for i, label := range labels {
		y := 180 + i*44
		saved := n.beginPress(27 + i)
		n.text(label, 34, y+10, 180, 25, 12, false, ink)
		n.drawText(values[i], 220, y, 92, 44, 12, true, ink, true)
		n.line(318, y+19, 321, y+22, muted, 1.2)
		n.line(321, y+22, 324, y+19, muted, 1.2)
		if i < 2 {
			n.line(34, y+43, 324, y+43, card, 1)
		}
		n.endPress(saved)
	}
	n.drawResourceSwitches(344, ink, muted, group)
	n.drawText(n.resourceStatus(), 20, 464, 216, 62, 11, false, muted, false)
	if c.Automatic || c.EcoCPU {
		saved := n.beginPress(24)
		n.round(246, 464, 92, 32, 10, card, false)
		n.drawText("Hentikan", 246, 464, 92, 32, 11, true, ink, true)
		n.endPress(saved)
	}
	n.drawResourceOptions(ink, muted)
}

func (n *nativeGlassPanel) drawResourceOptions(ink, muted uint32) {
	command := n.resources.optionOpen
	values := resourceOptionValues(command)
	if len(values) == 0 {
		return
	}
	x := nativeControlBoxes[30][0] - 4
	y := nativeControlBoxes[30][1] - 6
	fill := uint32(0xff1b1d22)
	edge := uint32(0x24ffffff)
	highlight, focusEdge := uint32(0x242f77f4), uint32(0x8095baff)
	if n.light {
		fill = 0xfff5f7fa
		edge = 0x18000000
		highlight, focusEdge = 0x182f77f4, 0x80215fc6
	}
	width := nativeControlBoxes[30][2] + 8
	step := 28
	if command == 320 {
		step = 48
	}
	n.round(x, y, width, len(values)*step+12, 13, fill, false)
	n.round(x, y, width, len(values)*step+12, 13, edge, true)
	selected := n.resources.config.MemoryThreshold()
	if command == 316 {
		selected = int(n.resources.config.IdleWait() / time.Minute)
	}
	if command == 317 {
		selected = int(n.resources.config.TrimWait() / time.Minute)
	}
	if command == 320 {
		selected = 0
		if n.resources.config.AllBackground {
			selected = 1
		}
	}
	if command == 320 {
		n.drawScopeChoices(x, y, width, selected, ink, muted, highlight)
		return
	}
	for i, value := range values {
		yy := y + 6 + i*28
		saved := n.beginPress(30 + i)
		if selected == value || n.focus == 30+i {
			n.round(x+4, yy, width-8, 28, 8, highlight, false)
		}
		if n.keyboardFocus && n.focus == 30+i {
			n.round(x+5, yy+1, width-10, 26, 8, focusEdge, true)
		}
		label := fmt.Sprintf("%d menit", value)
		if command == 315 {
			label = fmt.Sprintf("RAM ≥ %d%%", value)
		}
		if command == 320 {
			label = "Pilih aplikasi"
			if value == 1 {
				label = "Semua aplikasi latar"
			}
		}
		n.text(label, x+13, yy+5, width-28, 22, 12, selected == value, ink)
		n.endPress(saved)
	}
}

func (n *nativeGlassPanel) drawScopeIcon(all bool, x, y int, color uint32) {
	if all {
		for i := 0; i < 4; i++ {
			n.round(x+(i%2)*9, y+(i/2)*9, 6, 6, 2, color, true)
		}
	} else {
		for i := 0; i < 3; i++ {
			n.round(x, y+i*6, 3, 3, 1, color, true)
			n.line(x+7, y+1+i*6, x+16, y+1+i*6, color, 1.2)
		}
	}
}
func (n *nativeGlassPanel) drawScopeChoices(x, y, width, selected int, ink, muted, highlight uint32) {
	brand := uint32(0xffffc66b)
	if n.light {
		brand = 0xff99620c
	}
	for i, value := range []int{1, 0} {
		yy := y + 6 + i*48
		saved := n.beginPress(30 + i)
		chosen := value == selected
		focused := n.keyboardFocus && n.focus == 30+i
		if chosen || focused {
			n.round(x+5, yy, width-10, 48, 9, highlight, false)
		}
		iconInk := muted
		if chosen {
			iconInk = brand
		}
		n.drawScopeIcon(value == 1, x+16, yy+16, iconInk)
		label, detail := "Pilih aplikasi", "Tentukan aplikasi sendiri"
		if value == 1 {
			label, detail = "Semua aplikasi latar", "Aplikasi yang memenuhi syarat"
		}
		n.drawText(label, x+44, yy+5, width-86, 23, 13, true, ink, false, true)
		n.drawText(detail, x+44, yy+25, width-86, 18, 10, false, muted, false, true)
		ring := muted&0xffffff | 0x65000000
		if chosen || focused {
			ring = brand
		}
		n.round(x+width-29, yy+18, 12, 12, 6, ring, true)
		if chosen {
			n.round(x+width-25, yy+22, 4, 4, 2, brand, false)
		}
		n.endPress(saved)
	}
}

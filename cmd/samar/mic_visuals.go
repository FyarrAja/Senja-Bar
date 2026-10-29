//go:build windows

package main

import (
	"fmt"
	"math"
	"unsafe"
)

func micDBLabel(level float64) string {
	if level < .1 {
		return "−∞ dB"
	}
	return fmt.Sprintf("%.0f dB", 20*math.Log10(level/100))
}

func (n *nativeGlassPanel) drawMicChevron(x, y int, color uint32) {
	n.line(x-3, y-1, x, y+2, color, 1.3)
	n.line(x, y+2, x+3, y-1, color, 1.3)
}

func (n *nativeGlassPanel) drawPresetIcon(index, x, y int, color uint32) {
	projected := n.projected
	x = int(n.projectX(float32(x)))
	n.projected = false
	defer func() { n.projected = projected }()
	switch index {
	case 1: // clear: rising bands
		for i, h := range []int{5, 10, 15} {
			n.round(x+2+i*6, y+17-h, 3, h, 1, color, false)
		}
	case 2: // bass
		for i, h := range []int{15, 11, 6} {
			n.round(x+2+i*6, y+17-h, 3, h, 1, color, false)
		}
	case 3: // room: shield
		points := [][2]int{{10, 1}, {18, 4}, {17, 12}, {10, 19}, {3, 12}, {2, 4}, {10, 1}}
		for i := 1; i < len(points); i++ {
			n.line(x+points[i-1][0], y+points[i-1][1], x+points[i][0], y+points[i][1], color, 1.3)
		}
		n.line(x+7, y+10, x+9, y+12, color, 1.3)
		n.line(x+9, y+12, x+14, y+7, color, 1.3)
	case 4: // soft voice: mic
		n.drawDeviceIcon("mic", x, y, color)
	case 5: // focus: target
		n.round(x+3, y+3, 14, 14, 7, color, true)
		n.round(x+8, y+8, 4, 4, 2, color, false)
		n.line(x+10, y, x+10, y+4, color, 1.3)
		n.line(x+10, y+16, x+10, y+20, color, 1.3)
		n.line(x, y+10, x+4, y+10, color, 1.3)
		n.line(x+16, y+10, x+20, y+10, color, 1.3)
	case 6: // automatic: controlled sliders
		for i, p := range []int{6, 13, 9} {
			yy := y + 4 + i*6
			n.line(x+1, yy, x+19, yy, color, 1.3)
			n.round(x+p-2, yy-2, 4, 4, 2, color, false)
		}
	default: // balanced bands
		for i, h := range []int{9, 15, 9} {
			n.round(x+2+i*6, y+17-h, 3, h, 1, color, false)
		}
	}
}
func micMeterFraction(level float64) float64 {
	if level < .1 {
		return 0
	}
	return max(0., min(1., (20*math.Log10(level/100)+54)/54))
}
func (n *nativeGlassPanel) drawMicMeter(x, y, width, height int, level float64, card uint32) {
	const bars = 32
	fraction := micMeterFraction(level) * bars
	step := float32(width) / bars
	for i := 0; i < bars; i++ {
		h := float32(5) + float32(i)*float32(height-5)/(bars-1)
		left := float32(x) + float32(i)*step
		top := float32(y+height) - h
		n.roundFloat(left, top, 4.5, h, 2, card, false, 1)
		active := max(0., min(1., fraction-float64(i)))
		if active > 0 {
			color := uint32(0x007daeff)
			if i >= 28 {
				color = 0x00eac479
			}
			if i >= 31 {
				color = 0x00f38e96
			}
			color |= uint32(active*255) << 24
			n.roundFloat(left, top, 4.5, h, 2, color, false, 1)
		}
	}
}

// Small authored stroke icons use the same weight as the native navigation.
func (n *nativeGlassPanel) drawDeviceIcon(kind string, x, y int, color uint32) {
	projected := n.projected
	x = int(n.projectX(float32(x)))
	n.projected = false
	defer func() { n.projected = projected }()
	arc := func(ax, ay, w, h int, start, sweep float32) {
		var pen uintptr
		nativeG("GdipCreatePen1", uintptr(color), nativeF(1.35), 2, uintptr(unsafe.Pointer(&pen)))
		nativeG("GdipDrawArc", n.graphics, pen, nativeF(float32(ax)), nativeF(float32(ay)), nativeF(float32(w)), nativeF(float32(h)), nativeF(start), nativeF(sweep))
		nativeG("GdipDeletePen", pen)
	}
	switch kind {
	case "headset":
		arc(x+2, y+1, 16, 17, 180, 180)
		n.round(x+2, y+10, 4, 8, 2, color, true)
		n.round(x+14, y+10, 4, 8, 2, color, true)
	case "speaker":
		n.line(x+2, y+7, x+7, y+7, color, 1.3)
		n.line(x+2, y+7, x+2, y+13, color, 1.3)
		n.line(x+2, y+13, x+7, y+13, color, 1.3)
		n.line(x+7, y+7, x+11, y+3, color, 1.3)
		n.line(x+11, y+3, x+11, y+17, color, 1.3)
		n.line(x+11, y+17, x+7, y+13, color, 1.3)
		arc(x+9, y+5, 8, 10, -60, 120)
		arc(x+9, y+2, 12, 16, -60, 120)
	case "virtual":
		n.round(x+1, y+4, 18, 13, 3, color, true)
		n.line(x+4, y+11, x+7, y+8, color, 1.3)
		n.line(x+7, y+8, x+11, y+13, color, 1.3)
		n.line(x+11, y+13, x+16, y+8, color, 1.3)
	case "mic":
		n.round(x+7, y+1, 6, 12, 3, color, true)
		arc(x+4, y+5, 12, 12, 0, 180)
		n.line(x+10, y+17, x+10, y+20, color, 1.3)
		n.line(x+6, y+20, x+14, y+20, color, 1.3)
	default:
		n.round(x+1, y+3, 18, 12, 3, color, true)
		n.line(x+10, y+15, x+10, y+19, color, 1.3)
		n.line(x+5, y+19, x+15, y+19, color, 1.3)
	}
}

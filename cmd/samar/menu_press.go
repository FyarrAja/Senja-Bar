//go:build windows

package main

import (
	"math"
	"time"
	"unsafe"
)

var nativeControlBoxes = [72][4]int{{20, 102, 154, 84}, {184, 102, 154, 84}, {20, 196, 154, 84}, {184, 196, 154, 84}, {20, 294, 318, 42}, {20, 336, 318, 42}, {20, 430, 154, 48}, {184, 430, 154, 48}, {-104, 488, 96, 36}, {20, 378, 318, 42}, {-104, 102, 96, 42}, {-104, 152, 96, 42}, {20, 114, 318, 32}, {20, 146, 318, 32}, {20, 178, 318, 32}, {20, 210, 318, 32}, {20, 242, 318, 32}, {20, 274, 318, 32}, {20, 313, 68, 25}, {270, 313, 68, 25}, {20, 344, 318, 33}, {20, 382, 318, 33}, {20, 443, 154, 34}, {184, 443, 154, 34}, {20, 486, 318, 29}, {20, 78, 318, 29}, {310, 78, 28, 29}}
var nativeControlCommands = [72]uint32{cmdStyleGlass, cmdStyleAcrylic, cmdStyleClear, cmdStyleSolid, cmdRounded, cmdStartup, cmdRefresh, cmdCheckUpdates, cmdExit, cmdAutoInstall, cmdTaskbarPage, cmdResources, 300, 301, 302, 303, 304, 305, 306, 307, 308, 309, 310, 311, 312, 313, 314}
var nativeFocusOrder = []int{10, 11, 38, 48, 0, 1, 2, 3, 4, 5, 9, 6, 7, 37, 25, 26, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 35, 27, 28, 29, 36, 22, 23, 24, 30, 31, 32, 33, 34, 8}

func (n *nativeGlassPanel) nextFocus(current, delta int) int {
	order := nativeFocusOrder[:]
	if n.micPage {
		order = []int{10, 11, 38, 48, 69, 49, 65, 64, 50, 51, 68, 70, 55, 8}
	}
	if n.micPage && n.mic.dropdown != 0 {
		order = []int{56, 57, 58, 59, 60, 61, 62, 63}
	}
	if n.systemPage {
		order = []int{10, 11, 38, 48, 39, 40, 41, 42, 43, 44, 45, 46, 47, 8}
	}
	if n.resourcePage && n.resources.optionOpen == 0 {
		order = []int{10, 11, 38, 48, 35, 36, 37, 25, 26, 12, 13, 14, 15, 18, 19, 22, 23, 8}
		if n.resources.tuning {
			order = []int{10, 11, 38, 48, 35, 36, 27, 28, 29, 20, 21, 24, 8}
		}
	}
	pos := -1
	for i, v := range order {
		if v == current {
			pos = i
			break
		}
	}
	if pos < 0 && delta < 0 {
		pos = 0
	}
	for i := 0; i < len(order); i++ {
		pos = (pos + delta + len(order)) % len(order)
		v := order[pos]
		if n.controlEnabled(v) {
			return v
		}
	}
	return -1
}

type nativePress struct {
	value, from, to float64
	started         time.Time
	releasePending  bool
}

func (p *nativePress) target(to float64, now time.Time) {
	if p.to == to && !p.releasePending {
		return
	}
	p.from, p.to, p.started, p.releasePending = p.value, to, now, false
}
func (p *nativePress) release(now time.Time, clicked bool) {
	// Very short clicks still complete the downstroke. Commands do not wait
	// for the visual pulse, and reversal never jumps to an artificial peak.
	if clicked && p.to == 1 && p.value < .85 {
		p.releasePending = true
		return
	}
	p.target(0, now)
}
func (p *nativePress) advance(now time.Time) bool {
	if p.value == p.to && !p.releasePending {
		return false
	}
	d := 70 * time.Millisecond
	if p.to == 0 {
		d = 180 * time.Millisecond
	}
	t := min(1., max(0., float64(now.Sub(p.started))/float64(d)))
	p.value = p.from + (p.to-p.from)*(1-math.Pow(1-t, 3))
	if t >= 1 {
		p.value = p.to
		if p.releasePending {
			p.target(0, now)
			return true
		}
		return false
	}
	return true
}
func nativeControlAt(x, y int) int {
	for i, b := range nativeControlBoxes {
		if i >= 12 {
			break
		}
		if x >= b[0] && x < b[0]+b[2] && y >= b[1] && y < b[1]+b[3] {
			return i
		}
	}
	return -1
}
func (n *nativeGlassPanel) pressControl(i int) {
	if i < 0 || i >= len(n.presses) || n.closing || !n.controlEnabled(i) {
		return
	}
	if n.presses[i].to == 1 && !n.presses[i].releasePending {
		return
	}
	n.presses[i].target(1, time.Now())
	n.ensureAnimationTicker()
}
func (n *nativeGlassPanel) releaseControl(i int, clicked bool) {
	if i < 0 || i >= len(n.presses) {
		return
	}
	if n.presses[i].to == 0 && !n.presses[i].releasePending {
		return
	}
	n.presses[i].release(time.Now(), clicked)
	n.ensureAnimationTicker()
}
func (n *nativeGlassPanel) beginPress(i int) uint32 {
	if i < 0 || i >= len(nativeControlBoxes) {
		return n.beginPressAt(i, 0, 0)
	}
	b := nativeControlBoxes[i]
	return n.beginPressAt(i, float32(b[0])+float32(b[2])/2, float32(b[1])+float32(b[3])/2)
}
func (n *nativeGlassPanel) beginPressAt(i int, cx, cy float32) uint32 {
	cx = n.projectX(cx)
	var saved uint32
	nativeG("GdipSaveGraphics", n.graphics, uintptr(unsafe.Pointer(&saved)))
	if i < 0 || i >= len(n.presses) || n.presses[i].value == 0 || n.reduceMotion() {
		return saved
	}
	depth := .018
	if i == 4 || i == 5 || i == 9 || i == 20 || i == 21 {
		depth = .015
	}
	if i == 8 {
		depth = .025
	}
	scale := float32(1 - depth*n.presses[i].value)
	nativeG("GdipTranslateWorldTransform", n.graphics, nativeF(cx), nativeF(cy), 0)
	nativeG("GdipScaleWorldTransform", n.graphics, nativeF(scale), nativeF(scale), 0)
	nativeG("GdipTranslateWorldTransform", n.graphics, nativeF(-cx), nativeF(-cy), 0)
	return saved
}
func (n *nativeGlassPanel) endPress(saved uint32) {
	nativeG("GdipRestoreGraphics", n.graphics, uintptr(saved))
}

func nativeSwitchTrack(position float64, light ...bool) uint32 {
	t := max(0., min(1., position/17))
	off, on := uint32(0xff555b66), uint32(0xff4388ff)
	if len(light) > 0 && light[0] {
		off, on = 0xff8792a5, 0xff286be0
	}
	var color uint32
	for shift := uint(0); shift < 32; shift += 8 {
		a, b := float64(off>>shift&255), float64(on>>shift&255)
		color |= uint32(a+(b-a)*t+.5) << shift
	}
	return color
}

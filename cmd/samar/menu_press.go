//go:build windows

package main

import (
	"math"
	"time"
	"unsafe"
)

var nativeControlBoxes = [10][4]int{{20, 102, 154, 84}, {184, 102, 154, 84}, {20, 196, 154, 84}, {184, 196, 154, 84}, {20, 294, 318, 42}, {20, 336, 318, 42}, {20, 430, 154, 48}, {184, 430, 154, 48}, {114, 488, 132, 36}, {20, 378, 318, 42}}
var nativeControlCommands = [10]uint32{cmdStyleGlass, cmdStyleAcrylic, cmdStyleClear, cmdStyleSolid, cmdRounded, cmdStartup, cmdRefresh, cmdCheckUpdates, cmdExit, cmdAutoInstall}
var nativeFocusOrder = [10]int{0, 1, 2, 3, 4, 5, 9, 6, 7, 8}

func (n *nativeGlassPanel) nextFocus(current, delta int) int {
	pos := -1
	for i, v := range nativeFocusOrder {
		if v == current {
			pos = i
			break
		}
	}
	if pos < 0 && delta < 0 {
		pos = 0
	}
	for i := 0; i < len(nativeFocusOrder); i++ {
		pos = (pos + delta + len(nativeFocusOrder)) % len(nativeFocusOrder)
		v := nativeFocusOrder[pos]
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
	var saved uint32
	nativeG("GdipSaveGraphics", n.graphics, uintptr(unsafe.Pointer(&saved)))
	if i < 0 || i >= len(n.presses) || n.presses[i].value == 0 {
		return saved
	}
	depth := .035
	if i == 4 || i == 5 || i == 9 {
		depth = .015
	}
	if i == 8 {
		depth = .045
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

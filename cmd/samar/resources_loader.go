//go:build windows

package main

import (
	"time"
	"unsafe"
)

const resourceLoaderTimer = 0x5255
const cubeLoaderDuration = 1200 * time.Millisecond

var cubeLoaderMatrix = [5]string{"00100", "01110", "10101", "11011", "01110"}

// CSS 'ease' timing, used between the supplied dot keyframes.
func cubeLoaderEase(x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	lo, hi := 0., 1.
	for i := 0; i < 12; i++ {
		t := (lo + hi) / 2
		if .75*t-.75*t*t+t*t*t < x {
			lo = t
		} else {
			hi = t
		}
	}
	t := (lo + hi) / 2
	return .3*t + 2.4*t*t - 1.7*t*t*t
}
func cubeDotFrame(elapsed time.Duration, row, col int) (opacity, scale float64) {
	active := cubeLoaderMatrix[row][col] == '1'
	opacity, scale = .08, .75
	if active {
		opacity, scale = .15, .8
	}
	elapsed -= time.Duration(row*5+col) * 40 * time.Millisecond
	if elapsed < 0 {
		return
	}
	phase := float64(elapsed%cubeLoaderDuration) / float64(cubeLoaderDuration)
	mix := func(a, b, t float64) float64 { return a + (b-a)*cubeLoaderEase(t) }
	if active {
		switch {
		case phase < .2:
			opacity, scale = mix(.15, 1, phase/.2), mix(.8, 1.2, phase/.2)
		case phase < .6:
			opacity, scale = mix(1, .35, (phase-.2)/.4), mix(1.2, .95, (phase-.2)/.4)
		default:
			opacity, scale = mix(.35, .15, (phase-.6)/.4), mix(.95, .8, (phase-.6)/.4)
		}
	} else {
		if phase > .5 {
			phase = 1 - phase
		}
		opacity, scale = mix(.08, .22, phase*2), mix(.75, .9, phase*2)
	}
	return
}
func (n *nativeGlassPanel) resourceLoaderVisible() bool {
	return n.resourcePage && !n.resources.config.AllBackground && n.resources.loading && !n.closing
}
func (n *nativeGlassPanel) syncResourceLoader() {
	if n.hwnd == 0 {
		return
	}
	if n.resourceLoaderVisible() {
		user32.NewProc("SetTimer").Call(n.hwnd, resourceLoaderTimer, 16, 0)
	} else {
		user32.NewProc("KillTimer").Call(n.hwnd, resourceLoaderTimer)
	}
}

// Native equivalent of the supplied 6px dot / 4px gap / orbit demo.
func (n *nativeGlassPanel) drawResourceCubeLoader(ink uint32) {
	elapsed := time.Since(n.resources.loadingStarted)
	if n.resources.loadingStarted.IsZero() {
		elapsed = 0
	}
	var brush uintptr
	if nativeG("GdipCreateSolidFill", uintptr(ink), uintptr(unsafe.Pointer(&brush))) != 0 {
		return
	}
	defer nativeG("GdipDeleteBrush", brush)
	dot := func(x, y, size, opacity float64) {
		color := ink&0xffffff | uint32(opacity*255+.5)<<24
		nativeG("GdipSetSolidFillColor", brush, uintptr(color))
		nativeG("GdipFillEllipse", n.graphics, brush, nativeF(float32(x-size/2)), nativeF(float32(y-size/2)), nativeF(float32(size)), nativeF(float32(size)))
	}
	for row := 0; row < 5; row++ {
		for col := 0; col < 5; col++ {
			opacity, scale := cubeDotFrame(elapsed, row, col)
			x, y := 159.+float64(col)*10, 211.+float64(row)*10
			size := 6 * scale
			if cubeLoaderMatrix[row][col] == '1' {
				dot(x, y, size*2.2, opacity*.04)
				dot(x, y, size*1.5, opacity*.09)
			}
			dot(x, y, size, opacity)
		}
	}
}

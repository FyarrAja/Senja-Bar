//go:build windows

package main

// The native panel presents premultiplied per-pixel alpha without a WebView
// canvas or a backdrop surface that can become opaque. Preview is isolated.
import (
	"fmt"
	"math"
	"os"
	"runtime"
	"samar/internal/updatestate"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

var nativeGP = syscall.NewLazyDLL("gdiplus.dll")
var nativePanel *nativeGlassPanel

type nativeFloatRect struct{ x, y, w, h float32 }
type nativeGlassPanel struct {
	preview                                                    bool
	parent                                                     uintptr
	actual, desired                                            cssState
	busy                                                       uint32
	actions                                                    []uint32
	focus                                                      int
	light                                                      bool
	pixels                                                     uintptr
	checked                                                    bool
	updatePhase                                                uint32
	updateAvailable                                            bool
	arrowCursor, handCursor                                    uintptr
	hwnd, dc, bitmap, oldBitmap, graphics, image, family       uintptr
	fonts                                                      map[int]uintptr
	bounds                                                     rect
	w, h                                                       int32
	state                                                      cssState
	plateX, plateY, plateFromX, plateFromY, plateToX, plateToY float64
	switches, switchFrom, switchTo                             [2]float64
	alpha, alphaFrom, alphaTo                                  float64
	slide, slideFrom, slideTo                                  float64
	started                                                    time.Time
	stop                                                       chan struct{}
	queued                                                     chan struct{}
	closing                                                    bool
	presses                                                    [9]nativePress
	pressed, keyPressed                                        int
}

func nativeStateCommand(actual, desired cssState) uint32 {
	if actual.Style != desired.Style {
		return uint32(desired.Style + 99)
	}
	if actual.Rounded != desired.Rounded {
		return cmdRounded
	}
	if actual.Startup != desired.Startup {
		return cmdStartup
	}
	return 0
}
func (n *nativeGlassPanel) pump() {
	if n.preview || n.busy != 0 || n.closing {
		return
	}
	command := nativeStateCommand(n.actual, n.desired)
	if command == 0 && len(n.actions) > 0 {
		command = n.actions[0]
		n.actions = n.actions[1:]
	}
	if command == 0 {
		return
	}
	n.busy = command
	cssCommandGrace = time.Now().Add(750 * time.Millisecond)
	if ok, _, _ := postMessageW.Call(n.parent, wmCSSCommand, uintptr(command), n.hwnd); ok == 0 {
		n.busy = 0
		n.desired = n.actual
		n.state = n.actual
		n.animate(false)
	}
}
func (n *nativeGlassPanel) choose(command uint32) {
	if n.closing {
		return
	}
	if command == cmdCheckUpdates && !n.controlEnabled(7) {
		return
	}
	if command == cmdExit {
		if !n.preview {
			postMessageW.Call(n.parent, wmCSSCommand, uintptr(command), n.hwnd)
		}
		n.animate(true)
		return
	}
	switch command {
	case cmdStyleGlass, cmdStyleAcrylic, cmdStyleClear, cmdStyleSolid:
		n.desired.Style = int(command) - 99
	case cmdRounded:
		n.desired.Rounded = !n.desired.Rounded
	case cmdStartup:
		n.desired.Startup = !n.desired.Startup
	case cmdRefresh, cmdCheckUpdates:
		found := false
		for _, c := range n.actions {
			if c == command {
				found = true
			}
		}
		if !found {
			n.actions = append(n.actions, command)
		}
	}
	n.state = n.desired
	n.animate(false)
	if n.preview {
		n.actual = n.desired
		return
	}
	n.pump()
}
func nativeCommandAt(x, y int) uint32 {
	if i := nativeControlAt(x, y); i >= 0 {
		return nativeControlCommands[i]
	}
	return 0
}

func (n *nativeGlassPanel) updateContrast() bool {
	dc, _, _ := user32.NewProc("GetDC").Call(0)
	if dc == 0 {
		return false
	}
	defer user32.NewProc("ReleaseDC").Call(0, dc)
	var sum, count uint32
	for _, p := range []point{{n.bounds.left + 1, n.bounds.top + 1}, {n.bounds.right - 2, n.bounds.top + 1}, {n.bounds.left + 1, n.bounds.bottom - 2}, {n.bounds.right - 2, n.bounds.bottom - 2}} {
		value, _, _ := gdi32.NewProc("GetPixel").Call(dc, uintptr(p.x), uintptr(p.y))
		if uint32(value) == 0xffffffff {
			continue
		}
		c := uint32(value)
		sum += (2126*(c&255) + 7152*((c>>8)&255) + 722*((c>>16)&255)) / 10000
		count++
	}
	if count == 0 {
		return false
	}
	light := n.light
	if !light && sum/count > 165 {
		light = true
	}
	if light && sum/count < 125 {
		light = false
	}
	changed := light != n.light
	n.light = light
	return changed
}

func nativeF(v float32) uintptr { return uintptr(math.Float32bits(v)) }
func nativeG(name string, args ...uintptr) uintptr {
	r, _, _ := nativeGP.NewProc(name).Call(args...)
	return r
}
func (n *nativeGlassPanel) round(x, y, w, h, r int, color uint32, stroke bool) {
	n.roundFloat(float32(x), float32(y), float32(w), float32(h), float32(r), color, stroke, 1)
}
func (n *nativeGlassPanel) roundFloat(x, y, w, h, r float32, color uint32, stroke bool, width float32) {
	var path, brush uintptr
	nativeG("GdipCreatePath", 0, uintptr(unsafe.Pointer(&path)))
	defer nativeG("GdipDeletePath", path)
	d := 2 * r
	for _, a := range [][6]float32{{x, y, d, d, 180, 90}, {x + w - d, y, d, d, 270, 90}, {x + w - d, y + h - d, d, d, 0, 90}, {x, y + h - d, d, d, 90, 90}} {
		nativeG("GdipAddPathArc", path, nativeF(a[0]), nativeF(a[1]), nativeF(a[2]), nativeF(a[3]), nativeF(a[4]), nativeF(a[5]))
	}
	nativeG("GdipClosePathFigure", path)
	if stroke {
		nativeG("GdipCreatePen1", uintptr(color), nativeF(width), 2, uintptr(unsafe.Pointer(&brush)))
		nativeG("GdipDrawPath", n.graphics, brush, path)
		nativeG("GdipDeletePen", brush)
	} else {
		nativeG("GdipCreateSolidFill", uintptr(color), uintptr(unsafe.Pointer(&brush)))
		nativeG("GdipFillPath", n.graphics, brush, path)
		nativeG("GdipDeleteBrush", brush)
	}
}
func (n *nativeGlassPanel) line(x, y, x2, y2 int, color uint32, width float32) {
	var pen uintptr
	nativeG("GdipCreatePen1", uintptr(color), nativeF(width), 2, uintptr(unsafe.Pointer(&pen)))
	nativeG("GdipSetPenStartCap", pen, 2)
	nativeG("GdipSetPenEndCap", pen, 2)
	nativeG("GdipDrawLineI", n.graphics, pen, uintptr(x), uintptr(y), uintptr(x2), uintptr(y2))
	nativeG("GdipDeletePen", pen)
}

// Draw action icons as vectors so they keep the same stroke at every DPI.
func (n *nativeGlassPanel) powerIcon(x, y int, color uint32) {
	var path, pen uintptr
	nativeG("GdipCreatePath", 0, uintptr(unsafe.Pointer(&path)))
	defer nativeG("GdipDeletePath", path)
	nativeG("GdipAddPathArcI", path, uintptr(x), uintptr(y+2), 12, 12, nativeF(-50), nativeF(280))
	nativeG("GdipCreatePen1", uintptr(color), nativeF(1.5), 2, uintptr(unsafe.Pointer(&pen)))
	defer nativeG("GdipDeletePen", pen)
	nativeG("GdipSetPenStartCap", pen, 2)
	nativeG("GdipSetPenEndCap", pen, 2)
	nativeG("GdipDrawPath", n.graphics, pen, path)
	n.line(x+6, y, x+6, y+7, color, 1.5)
}
func (n *nativeGlassPanel) updateIcon(x, y int, color uint32) {
	n.line(x+7, y, x+7, y+9, color, 1.5)
	n.line(x+3, y+5, x+7, y+9, color, 1.5)
	n.line(x+7, y+9, x+11, y+5, color, 1.5)
	n.line(x+1, y+10, x+1, y+13, color, 1.5)
	n.line(x+1, y+13, x+13, y+13, color, 1.5)
	n.line(x+13, y+13, x+13, y+10, color, 1.5)
}
func (n *nativeGlassPanel) text(s string, x, y, w, h, size int, bold bool, color uint32) {
	n.drawText(s, x, y, w, h, size, bold, color, false)
}
func (n *nativeGlassPanel) drawText(s string, x, y, w, h, size int, bold bool, color uint32, centered bool) {
	key := size * 2
	if bold {
		key++
	}
	font := n.fonts[key]
	if font == 0 {
		style := uintptr(0)
		if bold {
			style = 1
		}
		nativeG("GdipCreateFont", n.family, nativeF(float32(size)), style, 2, uintptr(unsafe.Pointer(&font)))
		n.fonts[key] = font
	}
	var brush uintptr
	nativeG("GdipCreateSolidFill", uintptr(color), uintptr(unsafe.Pointer(&brush)))
	bounds := nativeFloatRect{float32(x), float32(y), float32(w), float32(h)}
	text := syscall.StringToUTF16(s)
	var format uintptr
	if centered {
		nativeG("GdipCreateStringFormat", 0, 0, uintptr(unsafe.Pointer(&format)))
		defer nativeG("GdipDeleteStringFormat", format)
		nativeG("GdipSetStringFormatAlign", format, 1)
		nativeG("GdipSetStringFormatLineAlign", format, 1)
	}
	nativeG("GdipDrawString", n.graphics, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)-1), font, uintptr(unsafe.Pointer(&bounds)), format, brush)
	nativeG("GdipDeleteBrush", brush)
}
func (n *nativeGlassPanel) icon(index, x, y int, color uint32) {
	if index == 3 {
		for _, p := range [][2]int{{0, 0}, {11, 0}, {0, 11}, {11, 11}} {
			n.round(x+p[0], y+p[1], 8, 8, 1, color, true)
		}
		return
	}
	n.round(x, y+2, 21, 16, 4, color, true)
	switch index {
	case 0:
		n.line(x+5, y+14, x+11, y+6, color, 1.8)
		n.line(x+11, y+14, x+17, y+6, color, 1.8)
	case 1:
		for k := 0; k < 3; k++ {
			n.line(x+5, y+6+k*4, x+16, y+6+k*4, color, 1.6)
		}
	case 2:
		n.line(x+5, y+7, x+9, y+7, color, 1.8)
		n.line(x+5, y+12, x+15, y+12, color, 1.8)
	}
}
func (n *nativeGlassPanel) paint() {
	nativeG("GdipGraphicsClear", n.graphics, 0)
	ink, muted, surface, card, edge, exitInk := uint32(0xfff6f7fa), uint32(0xffe8ebf2), uint32(0x2e0c0d0f), uint32(0x14ffffff), uint32(0x1bffffff), uint32(0xffffb3bc)
	if n.light {
		ink, muted, surface, card, edge, exitInk = 0xff18202d, 0xff344156, 0x2effffff, 0x14000000, 0x25000000, 0xffa81c35
	}
	group, divider, secondary, brand := uint32(0x10ffffff), uint32(0x10ffffff), uint32(0x0affffff), uint32(0xffffc66b)
	if n.light {
		group, divider, secondary, brand = 0x10000000, 0x14000000, 0x10000000, 0xffc98224
	}
	n.round(0, 0, 359, 547, 28, surface, false)
	// Keep the complete antialiased stroke inside the alpha surface. A native
	// integer window region would cut its partially covered corner pixels off.
	n.roundFloat(1, 1, 357, 545, 27, 0x60ffffff, true, 1.25)
	// Sunset mark, matching the established Senja silhouette.
	var path, brush uintptr
	nativeG("GdipCreatePath", 0, uintptr(unsafe.Pointer(&path)))
	nativeG("GdipAddPathArcI", path, 24, 28, 32, 28, nativeF(180), nativeF(180))
	nativeG("GdipClosePathFigure", path)
	nativeG("GdipCreateSolidFill", uintptr(brand), uintptr(unsafe.Pointer(&brush)))
	nativeG("GdipFillPath", n.graphics, brush, path)
	nativeG("GdipDeleteBrush", brush)
	nativeG("GdipDeletePath", path)
	n.line(24, 46, 56, 46, brand, 3)
	n.line(30, 52, 50, 52, brand, 3)
	n.text("Senja", 66, 18, 170, 35, 27, true, ink)
	n.text("Tampilan taskbar", 68, 51, 180, 18, 11, false, muted)
	n.text("Pilih tampilan", 20, 80, 220, 22, 12, true, muted)
	for i := 0; i < 4; i++ {
		x, y := 20+(i%2)*164, 108+(i/2)*106
		saved := n.beginPress(i)
		n.round(x, y, 154, 96, 23, card, false)
		n.round(x, y, 154, 96, 23, edge, true)
		n.endPress(saved)
	}
	saved := n.beginPressAt(max(0, min(3, n.state.Style-1)), float32(n.plateX+77), float32(n.plateY+48))
	n.round(int(n.plateX+.5), int(n.plateY+.5), 154, 96, 23, 0xed2f77f4, false)
	n.round(int(n.plateX+.5), int(n.plateY+.5), 154, 96, 23, 0x73aacdff, true)
	n.endPress(saved)
	labels := []string{"Kaca", "Akrilik", "Bening", "Solid"}
	descriptions := []string{"Blur lembut", "Blur lebih pekat", "Transparan penuh", "Gelap tanpa blur"}
	for i := 0; i < 4; i++ {
		x, y := 20+(i%2)*164, 108+(i/2)*106
		saved := n.beginPress(i)
		labelInk, smallInk := ink, muted
		if n.state.Style == i+1 {
			labelInk, smallInk = 0xffffffff, 0xffffffff
		}
		n.icon(i, x+18, y+20, labelInk)
		n.text(labels[i], x+14, y+53, 130, 23, 14, true, labelInk)
		n.text(descriptions[i], x+14, y+75, 136, 17, 10, false, smallInk)
		n.endPress(saved)
	}
	n.round(20, 324, 318, 96, 22, group, false)
	n.line(33, 372, 325, 372, divider, 1)
	for i, s := range []string{"Sudut membulat", "Mulai bersama Windows"} {
		saved := n.beginPress(4 + i)
		y := 338 + i*44
		n.text(s, 32, y+3, 235, 24, 12, false, ink)
		track := uint32(0x444f5665)
		if n.switchTo[i] > 0 {
			track = 0xff4388ff
		}
		n.round(284, y, 42, 25, 12, track, false)
		n.round(287+int(n.switches[i]+.5), y+3, 19, 19, 9, 0xffffffff, false)
		n.endPress(saved)
	}
	saved = n.beginPress(6)
	n.round(20, 430, 154, 48, 17, card, false)
	n.text("↻", 41, 439, 24, 30, 20, false, ink)
	n.text("Segarkan taskbar", 67, 445, 106, 22, 11, true, ink)
	n.endPress(saved)
	saved = n.beginPress(7)
	n.round(184, 430, 154, 48, 17, secondary, false)
	updateLabel, updateDetail, updateEnabled, updateMark := updatePresentation(n.updatePhase, n.updateAvailable, buildVersion)
	updateInk := muted
	if !updateEnabled {
		updateInk = updateInk&0xffffff | 0x99000000
	}
	n.updateIcon(205, 444, updateInk)
	n.text(updateLabel, 225, 437, 105, 21, 11, true, updateInk)
	n.text(updateDetail, 224, 454, 110, 18, 9, false, updateInk)
	if updateMark {
		n.round(322, 435, 9, 9, 4, 0xfff45d6b, false)
	}
	n.endPress(saved)
	saved = n.beginPress(8)
	n.round(114, 488, 132, 36, 18, 0x28ef4456, false)
	n.roundFloat(114.5, 488.5, 131, 35, 17.5, 0x18ff7885, true, 1)
	n.powerIcon(151, 498, exitInk)
	n.drawText("Keluar", 169, 488, 44, 36, 12, true, exitInk, true)
	n.endPress(saved)
	if n.focus >= 0 {
		b := nativeControlBoxes[n.focus]
		saved := n.beginPress(n.focus)
		n.round(b[0]+2, b[1]+2, b[2]-4, b[3]-4, 14, 0xffa9cbff, true)
		n.endPress(saved)
	}
	var ps previewPaint
	user32.NewProc("BeginPaint").Call(n.hwnd, uintptr(unsafe.Pointer(&ps)))
	user32.NewProc("EndPaint").Call(n.hwnd, uintptr(unsafe.Pointer(&ps)))
	nativeG("GdipFlush", n.graphics, 1)
	if n.alpha >= 255 && !n.checked {
		n.checked = true
		px := unsafe.Slice((*byte)(unsafe.Pointer(n.pixels)), int(n.w*n.h*4))
		at := (int(menuPx(90))*int(n.w) + int(menuPx(180))) * 4
		logEvent("INFO", "native.alpha", fmt.Sprintf("corner=%d panel=%d", px[3], px[at+3]))
	}
	destination := point{n.bounds.left, n.bounds.top + menuPx(int32(n.slide+.5))}
	cssPopupBounds = rect{destination.x, destination.y, destination.x + n.w, destination.y + n.h}
	size := point{n.w, n.h}
	origin := point{}
	blend := struct{ Operation, Flags, Alpha, Format byte }{Alpha: byte(n.alpha + .5), Format: 1}
	if ok, _, err := user32.NewProc("UpdateLayeredWindow").Call(n.hwnd, 0, uintptr(unsafe.Pointer(&destination)), uintptr(unsafe.Pointer(&size)), n.dc, uintptr(unsafe.Pointer(&origin)), 0, uintptr(unsafe.Pointer(&blend)), 2); ok == 0 {
		logWin32Error("native.present", "UpdateLayeredWindow", err)
	}
}
func (n *nativeGlassPanel) animate(close bool) {
	n.closing = close
	cssClosing = close
	n.alphaFrom, n.alphaTo = n.alpha, 255
	n.slideFrom, n.slideTo = n.slide, 0
	if close {
		n.alphaTo = 0
		n.slideTo = 10
	}
	n.plateFromX, n.plateFromY = n.plateX, n.plateY
	i := max(0, min(3, n.state.Style-1))
	n.plateToX, n.plateToY = float64(20+i%2*164), float64(108+i/2*106)
	n.switchFrom = n.switches
	n.switchTo = [2]float64{}
	if n.state.Rounded {
		n.switchTo[0] = 17
	}
	if n.state.Startup {
		n.switchTo[1] = 17
	}
	n.started = time.Now()
	n.ensureAnimationTicker()
}
func (n *nativeGlassPanel) ensureAnimationTicker() {
	if n.stop != nil {
		return
	}
	n.stop = make(chan struct{})
	n.queued = make(chan struct{}, 1)
	stop, queued, hwnd := n.stop, n.queued, n.hwnd
	syscall.NewLazyDLL("winmm.dll").NewProc("timeBeginPeriod").Call(1)
	go func() {
		ticker := time.NewTicker(time.Second / 120)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				select {
				case queued <- struct{}{}:
					postMessageW.Call(hwnd, wmApp+48, 0, 0)
				default:
				}
			}
		}
	}()
}
func (n *nativeGlassPanel) endAnimation() {
	if n.stop != nil {
		close(n.stop)
		n.stop = nil
		syscall.NewLazyDLL("winmm.dll").NewProc("timeEndPeriod").Call(1)
	}
}
func nativeGlassProc(hwnd uintptr, message uint32, w, l uintptr) uintptr {
	n := nativePanel
	if n == nil {
		r, _, _ := defWindowProcW.Call(hwnd, uintptr(message), w, l)
		return r
	}
	switch message {
	case 0x20: // WM_SETCURSOR: never inherit the shell's launch/busy cursor.
		if int16(l&0xffff) == 1 {
			cursor := n.arrowCursor
			var p point
			getCursorPos.Call(uintptr(unsafe.Pointer(&p)))
			user32.NewProc("ScreenToClient").Call(hwnd, uintptr(unsafe.Pointer(&p)))
			i := nativeControlAt(int(p.x)*96/int(menuDPI), int(p.y)*96/int(menuDPI))
			if i >= 0 && n.controlEnabled(i) && n.handCursor != 0 {
				cursor = n.handCursor
			}
			user32.NewProc("SetCursor").Call(cursor)
			return 1
		}
	case wmUpdatePhase:
		n.updatePhase = min(uint32(w)&0xff, updatestate.Available)
		n.updateAvailable = uint32(w)&updatestate.PendingFlag != 0
		if !n.controlEnabled(7) {
			if n.pressed == 7 {
				n.pressed = -1
				user32.NewProc("ReleaseCapture").Call()
				n.releaseControl(7, false)
			}
			if n.keyPressed == 7 {
				n.keyPressed = -1
				n.releaseControl(7, false)
			}
		}
		user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
		return 0
	case 0x14:
		return 1
	case 0xf:
		if n.graphics != 0 {
			n.paint()
		}
		return 0
	case wmApp + 48:
		if n.stop == nil {
			return 0
		}
		select {
		case <-n.queued:
		default:
		}
		duration := 200 * time.Millisecond
		if n.closing {
			duration = 140 * time.Millisecond
		}
		t := min(1., float64(time.Since(n.started))/float64(duration))
		e := 1 - math.Pow(1-t, 3)
		lerp := func(a, b float64) float64 { return a + (b-a)*e }
		n.alpha = lerp(n.alphaFrom, n.alphaTo)
		n.slide = lerp(n.slideFrom, n.slideTo)
		n.plateX, n.plateY = lerp(n.plateFromX, n.plateToX), lerp(n.plateFromY, n.plateToY)
		for i := range n.switches {
			n.switches[i] = lerp(n.switchFrom[i], n.switchTo[i])
		}
		pressAnimating := false
		for i := range n.presses {
			if n.presses[i].advance(time.Now()) {
				pressAnimating = true
			}
		}
		user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
		user32.NewProc("UpdateWindow").Call(hwnd)
		if t >= 1 && (n.closing || !pressAnimating) {
			n.endAnimation()
			if n.closing {
				destroyWindow.Call(hwnd)
			}
		}
		return 0
	case 0x201: // WM_LBUTTONDOWN
		if n.closing {
			return 0
		}
		x, y := int(int16(l&0xffff))*96/int(menuDPI), int(int16(l>>16))*96/int(menuDPI)
		n.pressed = nativeControlAt(x, y)
		if n.pressed >= 0 && !n.controlEnabled(n.pressed) {
			n.pressed = -1
		}
		if n.pressed >= 0 {
			user32.NewProc("SetCapture").Call(hwnd)
			n.pressControl(n.pressed)
		}
		return 0
	case 0x200: // WM_MOUSEMOVE, including captured drags outside the window.
		if n.pressed >= 0 {
			x, y := int(int16(l&0xffff))*96/int(menuDPI), int(int16(l>>16))*96/int(menuDPI)
			if nativeControlAt(x, y) == n.pressed {
				n.pressControl(n.pressed)
			} else {
				n.releaseControl(n.pressed, false)
			}
		}
		return 0
	case 0x215, 0x1f: // Capture loss / cancel mode must never invoke a button.
		if n.pressed >= 0 {
			i := n.pressed
			n.pressed = -1
			n.releaseControl(i, false)
		}
		if message == 0x1f {
			user32.NewProc("ReleaseCapture").Call()
		}
		return 0
	case wmLButtonUp:
		n.focus = -1
		x, y := int(int16(l&0xffff))*96/int(menuDPI), int(int16(l>>16))*96/int(menuDPI)
		i := n.pressed
		n.pressed = -1
		user32.NewProc("ReleaseCapture").Call()
		clicked := i >= 0 && nativeControlAt(x, y) == i
		n.releaseControl(i, clicked)
		if clicked {
			n.choose(nativeControlCommands[i])
		}
		return 0
	case wmCSSResult:
		if n.closing {
			return 0
		}
		n.busy = 0
		n.actual = currentCSSState()
		if w != 0 {
			n.desired = n.actual
			n.actions = nil
			logEvent("ERROR", "native.command", "engine rejected command; restored actual settings")
		}
		n.state = n.desired
		cssCommandGrace = time.Now().Add(180 * time.Millisecond)
		n.animate(false)
		n.pump()
		return 0
	case 0x21:
		if cssOverflow != 0 {
			return 3
		}
	case wmCSSShutdown, wmCSSToggle:
		n.animate(true)
		return 0
	case 0x100:
		if w == 27 {
			n.animate(true)
		}
		if w == 9 {
			delta := 1
			shift, _, _ := user32.NewProc("GetKeyState").Call(0x10)
			if shift&0x8000 != 0 {
				delta = -1
			}
			n.focus = (n.focus + delta + 9) % 9
			for !n.controlEnabled(n.focus) {
				n.focus = (n.focus + delta + 9) % 9
			}
			user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
		}
		if (w == 13 || w == 32) && n.focus >= 0 && l&(1<<30) == 0 {
			n.keyPressed = n.focus
			n.pressControl(n.keyPressed)
		}
		return 0
	case 0x101: // WM_KEYUP, one activation per complete key press.
		if (w == 13 || w == 32) && n.keyPressed >= 0 {
			i := n.keyPressed
			n.keyPressed = -1
			clicked := i == n.focus && n.controlEnabled(i)
			n.releaseControl(i, clicked)
			if clicked {
				n.choose(nativeControlCommands[i])
			}
		}
		return 0
	case 0x8: // WM_KILLFOCUS: a held key must not leave a pressed button behind.
		if n.keyPressed >= 0 {
			i := n.keyPressed
			n.keyPressed = -1
			n.releaseControl(i, false)
		}
		return 0
	case 0x113:
		if w == 0x4e43 && !n.closing && n.updateContrast() {
			user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
		}
		return 0
	case wmClose:
		n.animate(true)
		return 0
	case wmDestroy:
		n.endAnimation()
		postQuitMessage.Call(0)
		return 0
	}
	r, _, _ := defWindowProcW.Call(hwnd, uintptr(message), w, l)
	return r
}

func runNativeGlassPreview() { runNativeGlassPanel(true) }
func runNativeGlassPanel(preview bool) bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	prepareProcess()
	openLog()
	defer closeLog()
	mutexName := "Local\\Senja.CSSMenu"
	if preview {
		mutexName = "Local\\Senja.NativePreview"
	}
	mutex, _, mutexErr := createMutexW.Call(0, 0, uintptr(unsafe.Pointer(utf16ptr(mutexName))))
	if mutex == 0 {
		return false
	}
	defer closeHandle.Call(mutex)
	if mutexErr == syscall.Errno(errorAlreadyExists) {
		return true
	}
	var token uintptr
	input := struct {
		Version            uint32
		Debug              uintptr
		Background, Codecs int32
	}{Version: 1}
	if nativeG("GdiplusStartup", uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&input)), 0) != 0 {
		return false
	}
	defer nativeG("GdiplusShutdown", token)
	n := &nativeGlassPanel{preview: preview, state: currentCSSState(), fonts: make(map[int]uintptr), slide: 12, focus: -1, pressed: -1, keyPressed: -1}
	if preview && len(os.Args) > 1 && os.Args[1] == "--native-check" {
		n.preview = false
	}
	n.actual, n.desired = n.state, n.state
	if !n.preview {
		n.parent, _, _ = findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr(windowClass))), 0)
		if n.parent == 0 {
			return false
		}
		var phase uintptr
		user32.NewProc("SendMessageTimeoutW").Call(n.parent, wmUpdateQuery, 0, 0, 2, 200, uintptr(unsafe.Pointer(&phase)))
		n.updatePhase = min(uint32(phase)&0xff, updatestate.Available)
		n.updateAvailable = uint32(phase)&updatestate.PendingFlag != 0
	}
	nativePanel = n
	defer func() { nativePanel = nil }()
	// The preview is an app window solely so supported UI tools can inspect it.
	className := utf16ptr("SenjaNativeGlassPreview")
	if !preview {
		className = utf16ptr(cssMenuClass)
	}
	brush, _, _ := gdi32.NewProc("GetStockObject").Call(4)
	n.arrowCursor, _, _ = loadCursorW.Call(0, cursorArrow)
	n.handCursor, _, _ = loadCursorW.Call(0, 32649) // IDC_HAND, shared system cursor.
	class := wndClassEx{cbSize: uint32(unsafe.Sizeof(wndClassEx{})), lpfnWndProc: syscall.NewCallback(nativeGlassProc), hInstance: getModuleHandle(), hCursor: syscall.Handle(n.arrowCursor), lpszClassName: className, hbrBackground: syscall.Handle(brush)}
	if r, _, _ := registerClassExW.Call(uintptr(unsafe.Pointer(&class))); r == 0 {
		return false
	}
	menuDPI = 96
	var cursor point
	user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&cursor)))
	if !preview && len(os.Args) < 4 {
		if anchor, ok := cssTrayAnchor(syscall.Handle(n.parent)); ok {
			cursor = anchor
			cssOverflow = trayOverflowAt(anchor)
		}
	}
	if !preview && len(os.Args) > 3 {
		if ax, err := strconv.Atoi(os.Args[2]); err == nil {
			cursor.x = int32(ax)
		}
		if ay, err := strconv.Atoi(os.Args[3]); err == nil {
			cursor.y = int32(ay)
		}
	}
	monitor, _, _ := user32.NewProc("MonitorFromPoint").Call(uintptr(uint64(uint32(cursor.x))|uint64(uint32(cursor.y))<<32), 2)
	mi := cssMonitorInfo{Size: uint32(unsafe.Sizeof(cssMonitorInfo{}))}
	user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&mi)))
	var dx, dy uint32 = 96, 96
	syscall.NewLazyDLL("shcore.dll").NewProc("GetDpiForMonitor").Call(monitor, 0, uintptr(unsafe.Pointer(&dx)), uintptr(unsafe.Pointer(&dy)))
	menuDPI = int32(dx)
	menuDPI = min(menuDPI, (mi.Work.bottom-mi.Work.top)*96/556, (mi.Work.right-mi.Work.left)*96/368)
	menuDPI = max(48, menuDPI)
	n.w, n.h = menuPx(360), menuPx(548)
	x := mi.Work.left + (mi.Work.right-mi.Work.left-n.w)/2
	y := mi.Work.top + (mi.Work.bottom-mi.Work.top-n.h)/2
	n.bounds = rect{x, y, x + n.w, y + n.h}
	exStyle := uintptr(0x000c0008)
	if !preview {
		if len(os.Args) > 3 {
			if ax, err := strconv.Atoi(os.Args[2]); err == nil {
				cursor.x = int32(ax)
			}
			if ay, err := strconv.Atoi(os.Args[3]); err == nil {
				cursor.y = int32(ay)
			}
		}
		if len(os.Args) > 4 {
			if overflow, err := strconv.ParseUint(os.Args[4], 10, 64); err == nil {
				cssOverflow = uintptr(overflow)
			}
		}
		if cssOverflow != 0 && !trayWindowPresented(cssOverflow) {
			return true
		}
		n.bounds = cssBounds(cursor, menuDPI, mi.Work)
		x, y = n.bounds.left, n.bounds.top
		n.w, n.h = n.bounds.right-x, n.bounds.bottom-y
		exStyle = wsExToolWindow | 8 | 0x80000
		if cssOverflow != 0 {
			exStyle |= wsExNoActivate
		}
	}
	cssPopupBounds = n.bounds
	n.hwnd, _, _ = createWindowExW.Call(exStyle, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16ptr("Senja — Native Glass Preview"))), wsPopup, uintptr(x), uintptr(y+menuPx(12)), uintptr(n.w), uintptr(n.h), 0, 0, uintptr(getModuleHandle()), 0)
	if n.hwnd == 0 {
		return false
	}
	defer destroyWindow.Call(n.hwnd)
	// UpdateLayeredWindow already supplies the exact rounded alpha silhouette;
	// do not clip it again with the hard, integer CreateRoundRectRgn raster.
	n.dc, _, _ = gdi32.NewProc("CreateCompatibleDC").Call(0)
	if n.dc == 0 {
		return false
	}
	header := struct {
		Size                   uint32
		W, H                   int32
		Planes, Bits           uint16
		Compression, ImageSize uint32
		X, Y                   int32
		Used, Important        uint32
	}{Size: 40, W: n.w, H: -n.h, Planes: 1, Bits: 32}
	var pixels uintptr
	n.bitmap, _, _ = gdi32.NewProc("CreateDIBSection").Call(n.dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if n.bitmap == 0 {
		gdi32.NewProc("DeleteDC").Call(n.dc)
		return false
	}
	n.pixels = pixels
	n.oldBitmap, _, _ = menuSelect.Call(n.dc, n.bitmap)
	defer func() {
		for _, f := range n.fonts {
			nativeG("GdipDeleteFont", f)
		}
		nativeG("GdipDeleteFontFamily", n.family)
		nativeG("GdipDeleteGraphics", n.graphics)
		nativeG("GdipDisposeImage", n.image)
		menuSelect.Call(n.dc, n.oldBitmap)
		deleteObject.Call(n.bitmap)
		gdi32.NewProc("DeleteDC").Call(n.dc)
	}()
	nativeG("GdipCreateBitmapFromScan0", uintptr(n.w), uintptr(n.h), uintptr(n.w*4), 0x000e200b, pixels, uintptr(unsafe.Pointer(&n.image)))
	nativeG("GdipGetImageGraphicsContext", n.image, uintptr(unsafe.Pointer(&n.graphics)))
	if n.image == 0 || n.graphics == 0 {
		return false
	}
	nativeG("GdipSetSmoothingMode", n.graphics, 4)
	nativeG("GdipSetTextRenderingHint", n.graphics, 4)
	scale := float32(menuDPI) / 96
	nativeG("GdipScaleWorldTransform", n.graphics, nativeF(scale), nativeF(scale), 0)
	nativeG("GdipCreateFontFamilyFromName", uintptr(unsafe.Pointer(utf16ptr("Segoe UI"))), 0, uintptr(unsafe.Pointer(&n.family)))
	if n.family == 0 {
		return false
	}
	i := max(0, min(3, n.state.Style-1))
	n.plateX, n.plateY = float64(20+i%2*164), float64(108+i/2*106)
	if n.state.Rounded {
		n.switches[0] = 17
	}
	if n.state.Startup {
		n.switches[1] = 17
	}
	n.updateContrast()
	user32.NewProc("SetTimer").Call(n.hwnd, 0x4e43, 500, 0)
	show := uintptr(5)
	if !preview && cssOverflow != 0 {
		show = 4
	}
	user32.NewProc("ShowWindow").Call(n.hwnd, show)
	user32.NewProc("ShowWindow").Call(n.hwnd, show)
	if !preview && cssOverflow != 0 {
		user32.NewProc("ShowWindow").Call(n.hwnd, 4)
	} else {
		setForegroundWindow.Call(n.hwnd)
	}
	if !preview {
		stopClicks := watchPopupOutsideClicks(n.hwnd)
		defer stopClicks()
		if cssOverflow != 0 {
			stopOverflow := watchTrayOverflow(n.hwnd, cssOverflow)
			defer stopOverflow()
		}
	}
	n.animate(false)
	messageLoop()
	return true
}

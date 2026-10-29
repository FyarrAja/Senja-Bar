//go:build windows

package main

// The native panel presents premultiplied per-pixel alpha without a WebView
// canvas or a backdrop surface that can become opaque. Preview is isolated.
import (
	"fmt"
	"math"
	"os"
	"runtime"
	"samar/internal/mic"
	"samar/internal/resources"
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
	backdrop                                                   nativeLiveBackdrop
	resourcePage                                               bool
	micPage                                                    bool
	mic                                                        nativeMic
	systemPage                                                 bool
	system                                                     nativeSystem
	resources                                                  nativeResources
	localResources                                             *resources.Manager
	preview                                                    bool
	keyboardFocus                                              bool
	projected                                                  bool
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
	readyDismissed                                             bool
	commandError                                               bool
	noticeStarted                                              time.Time
	updateProgress                                             uint64
	downloadStarted                                            time.Time
	downloadMotion                                             nativeDownloadMotion
	arrowCursor, handCursor                                    uintptr
	hwnd, dc, bitmap, oldBitmap, graphics, image, family       uintptr
	fonts                                                      map[int]uintptr
	bounds                                                     rect
	w, h                                                       int32
	state                                                      cssState
	plateX, plateY, plateFromX, plateFromY, plateToX, plateToY float64
	switches, switchFrom, switchTo                             [5]float64
	alpha, alphaFrom, alphaTo                                  float64
	slide, slideFrom, slideTo                                  float64
	started                                                    time.Time
	stop                                                       chan struct{}
	queued                                                     chan struct{}
	closing                                                    bool
	presses                                                    [72]nativePress
	navMotion, tabMotion                                       nativeMotion
	motionChecked, motionReduced                               bool
	hovers                                                     [72]nativeMotion
	hoverTracking                                              bool
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
	if actual.AutoInstall != desired.AutoInstall {
		return cmdAutoInstall
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
	n.updateHover(-1)
	if n.micAction(command) {
		return
	}
	if n.systemAction(command) {
		return
	}
	if n.resourceAction(command) {
		return
	}
	if command == cmdRefresh && n.showReadyPrompt() {
		n.readyDismissed = true
		user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
		return
	}
	if command == cmdCheckUpdates && !n.controlEnabled(7) {
		return
	}
	if command == cmdExit {
		if !n.preview {
			_ = mic.Disable()
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
	case cmdAutoInstall:
		n.desired.AutoInstall = !n.desired.AutoInstall
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

// Pointer arguments must remain pinned across the wrapper, just as with Proc.Call.
//
//go:uintptrescapes
func nativeG(name string, args ...uintptr) uintptr {
	r, _, _ := nativeGP.NewProc(name).Call(args...)
	return r
}
func (n *nativeGlassPanel) round(x, y, w, h, r int, color uint32, stroke bool) {
	n.roundFloat(float32(x), float32(y), float32(w), float32(h), float32(r), color, stroke, 1)
}
func (n *nativeGlassPanel) roundFloat(x, y, w, h, r float32, color uint32, stroke bool, width float32) {
	w = n.projectX(x+w) - n.projectX(x)
	x = n.projectX(x)
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
	x = int(n.projectX(float32(x)))
	x2 = int(n.projectX(float32(x2)))
	var pen uintptr
	nativeG("GdipCreatePen1", uintptr(color), nativeF(width), 2, uintptr(unsafe.Pointer(&pen)))
	nativeG("GdipSetPenStartCap", pen, 2)
	nativeG("GdipSetPenEndCap", pen, 2)
	nativeG("GdipDrawLineI", n.graphics, pen, uintptr(x), uintptr(y), uintptr(x2), uintptr(y2))
	nativeG("GdipDeletePen", pen)
}

// Draw action icons as vectors so they keep the same stroke at every DPI.
func (n *nativeGlassPanel) powerIcon(x, y int, color uint32) {
	x = int(n.projectX(float32(x)))
	projection := n.projected
	n.projected = false
	defer func() { n.projected = projection }()
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
func (n *nativeGlassPanel) drawText(s string, x, y, w, h, size int, bold bool, color uint32, centered bool, singleLine ...bool) {
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
	bounds := nativeFloatRect{n.projectX(float32(x)), float32(y), n.projectX(float32(x+w)) - n.projectX(float32(x)), float32(h)}
	text := syscall.StringToUTF16(s)
	var format uintptr
	if centered || len(singleLine) > 0 && singleLine[0] {
		nativeG("GdipCreateStringFormat", 0, 0, uintptr(unsafe.Pointer(&format)))
		defer nativeG("GdipDeleteStringFormat", format)
		if len(singleLine) > 0 && singleLine[0] {
			nativeG("GdipSetStringFormatFlags", format, 0x1000)
			nativeG("GdipSetStringFormatTrimming", format, 3)
		}
		if centered {
			nativeG("GdipSetStringFormatAlign", format, 1)
			nativeG("GdipSetStringFormatLineAlign", format, 1)
		}
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
func (n *nativeGlassPanel) drawPanelContents() {
	nativeG("GdipGraphicsClear", n.graphics, 0)
	ink, muted, surface, card, edge, exitInk := uint32(0xfff6f7fa), uint32(0xffe8ebf2), uint32(0x2e0c0d0f), uint32(0x14ffffff), uint32(0x1bffffff), uint32(0xffffb3bc)
	if n.light {
		ink, muted, surface, card, edge, exitInk = 0xff18202d, 0xff344156, 0x2effffff, 0x14000000, 0x25000000, 0xffa81c35
	}
	group, divider, secondary := uint32(0x10ffffff), uint32(0x10ffffff), uint32(0x0affffff)
	if n.light {
		group, divider, secondary = 0x10000000, 0x14000000, 0x10000000
	}
	panelWidth := int(n.w) * 96 / int(menuDPI)
	n.round(0, 0, panelWidth-1, 547, nativePanelRadius, surface, false)
	// Keep the complete antialiased stroke inside the alpha surface. A native
	// integer window region would cut its partially covered corner pixels off.
	n.roundFloat(1, 1, float32(panelWidth-3), 545, nativePanelRadius-1, 0x38ffffff, true, 1)
	var contentTransform uint32
	nativeG("GdipSaveGraphics", n.graphics, uintptr(unsafe.Pointer(&contentTransform)))
	nativeG("GdipTranslateWorldTransform", n.graphics, nativeF(float32(n.sidebarWidth())), nativeF(0), 0)
	n.projected = true
	n.drawSidebar(ink, muted, card)
	if n.micPage {
		n.drawMic(ink, muted, card, group)
	} else if n.systemPage {
		n.drawSystem(ink, muted, card, group)
	} else if n.resourcePage {
		n.drawResources(ink, muted, card, group)
	} else {
		n.text("Taskbar", 20, 18, 318, 37, 26, true, ink)
		if n.commandError {
			n.text("Perubahan belum diterapkan", 20, 57, 318, 22, 12, false, exitInk)
		} else {
			n.text("Warna, transparansi, dan sudut", 20, 57, 318, 22, 12, false, muted)
		}
		n.text("Pilih tampilan", 20, 80, 220, 22, 12, true, muted)
		for i := 0; i < 4; i++ {
			x, y := 20+(i%2)*164, 102+(i/2)*94
			saved := n.beginPress(i)
			n.round(x, y, 154, 84, 21, card, false)
			n.round(x, y, 154, 84, 21, edge, true)
			n.endPress(saved)
		}
		saved := n.beginPressAt(max(0, min(3, n.state.Style-1)), float32(n.plateX+77), float32(n.plateY+42))
		n.round(int(n.plateX+.5), int(n.plateY+.5), 154, 84, 21, 0xed2f77f4, false)
		n.round(int(n.plateX+.5), int(n.plateY+.5), 154, 84, 21, 0x73aacdff, true)
		n.endPress(saved)
		labels := []string{"Kaca", "Akrilik", "Bening", "Solid"}
		descriptions := []string{"Blur lembut", "Blur lebih pekat", "Transparan penuh", "Tanpa transparansi"}
		for i := 0; i < 4; i++ {
			x, y := 20+(i%2)*164, 102+(i/2)*94
			saved := n.beginPress(i)
			labelInk, smallInk := ink, muted
			if n.state.Style == i+1 {
				labelInk, smallInk = 0xffffffff, 0xffffffff
			}
			n.icon(i, x+18, y+12, labelInk)
			n.text(labels[i], x+14, y+41, 130, 23, 14, true, labelInk)
			n.text(descriptions[i], x+14, y+64, 136, 17, 10, false, smallInk)
			n.endPress(saved)
		}
		if n.updatePhase == 2 {
			n.drawDownload(ink, muted, secondary)
		} else if n.showReadyPrompt() {
			n.drawReady(ink, muted, secondary)
		} else {
			n.round(20, 294, 318, 126, 22, group, false)
			n.line(33, 336, 325, 336, divider, 1)
			n.line(33, 378, 325, 378, divider, 1)
			for i, s := range []string{"Sudut membulat", "Mulai bersama Windows", "Update otomatis"} {
				index := []int{4, 5, 9}[i]
				saved := n.beginPress(index)
				y := 302 + i*42
				if i == 2 {
					n.text(s, 32, y-3, 235, 23, 12, false, ink)
					n.text("Unduh & terapkan otomatis", 32, y+16, 235, 16, 9, false, muted)
				} else {
					n.text(s, 32, y+3, 235, 24, 12, false, ink)
				}
				track := nativeSwitchTrack(n.switches[i], n.light)
				n.round(284, y, 42, 25, 12, track, false)
				n.round(287+int(n.switches[i]+.5), y+3, 19, 19, 9, 0xffffffff, false)
				n.endPress(saved)
			}
			saved = n.beginPress(6)
			n.round(20, 430, 154, 48, 17, card, false)
			n.text("↻", 41, 439, 24, 30, 20, false, ink)
			n.text("Segarkan taskbar", 67, 445, 106, 22, 12, true, ink)
			n.endPress(saved)
			saved = n.beginPress(7)
			n.round(184, 430, 154, 48, 17, secondary, false)
			updateLabel, updateDetail, updateEnabled, updateMark := updatePresentation(n.updatePhase, n.updateAvailable, buildVersion)
			if n.state.AutoInstall && n.updatePhase == updatestate.Ready {
				updateLabel = "Update otomatis"
				updateDetail = "Menyiapkan..."
				updateEnabled = false
				updateMark = false
			}
			updateInk := ink
			if !updateEnabled {
				updateInk = updateInk&0xffffff | 0x99000000
			}
			n.updateIcon(205, 444, updateInk)
			labelX, labelWidth := 225, 105
			if updateMark {
				labelX, labelWidth = 244, 86
			}
			n.text(updateLabel, labelX, 437, labelWidth, 21, 12, true, updateInk)
			n.text(updateDetail, 224, 454, 110, 18, 10, false, updateInk)
			if updateMark {
				n.drawUpdateNotice(time.Now())
			}
			n.endPress(saved)

		}
	}
	if n.keyboardFocus && n.focus >= 0 && n.focus != 25 && (n.focus < 30 || n.focus > 34) {
		b := nativeControlBoxes[n.focus]
		saved := n.beginPress(n.focus)
		n.round(b[0]+2, b[1]+2, b[2]-4, b[3]-4, 14, 0xffa9cbff, true)
		n.endPress(saved)
	}
	n.drawHover()
	nativeG("GdipRestoreGraphics", n.graphics, uintptr(contentTransform))
	n.projected = false
}
func (n *nativeGlassPanel) paint() {
	n.drawPanelContents()
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
	n.syncSoftBackdrop(destination)
	size := point{n.w, n.h}
	origin := point{}
	blend := struct{ Operation, Flags, Alpha, Format byte }{Alpha: byte(n.alpha + .5), Format: 1}
	if ok, _, err := user32.NewProc("UpdateLayeredWindow").Call(n.hwnd, 0, uintptr(unsafe.Pointer(&destination)), uintptr(unsafe.Pointer(&size)), n.dc, uintptr(unsafe.Pointer(&origin)), 0, uintptr(unsafe.Pointer(&blend)), 2); ok == 0 {
		logWin32Error("native.present", "UpdateLayeredWindow", err)
	}
}
func (n *nativeGlassPanel) animate(close bool) {
	n.closing = close
	n.syncResourceLoader()
	if close {
		user32.NewProc("KillTimer").Call(n.hwnd, 0x4e44)
	}
	cssClosing = close
	n.alphaFrom, n.alphaTo = n.alpha, 255
	n.slideFrom, n.slideTo = n.slide, 0
	if close {
		n.alphaTo = 0
		n.slideTo = 10
	}
	n.plateFromX, n.plateFromY = n.plateX, n.plateY
	i := max(0, min(3, n.state.Style-1))
	n.plateToX, n.plateToY = float64(20+i%2*164), float64(102+i/2*94)
	n.switchFrom = n.switches
	n.switchTo = [5]float64{}
	if n.state.Rounded {
		n.switchTo[0] = 17
	}
	if n.state.Startup {
		n.switchTo[1] = 17
	}
	if n.state.AutoInstall {
		n.switchTo[2] = 17
	}
	if n.resources.config.Automatic {
		n.switchTo[3] = 17
	}
	if n.resources.config.EcoCPU {
		n.switchTo[4] = 17
	}
	n.started = time.Now()
	n.ensureAnimationTicker()
}
func (n *nativeGlassPanel) ensureAnimationTicker() {
	if n.stop != nil || n.hwnd == 0 {
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
	case 0x102:
		if n.resourceSearchChar(w) {
			return 0
		}
	case wmApp + 76:
		select {
		case result := <-n.resources.results:
			n.resources.loading = false
			n.syncResourceLoader()
			if result.err != nil {
				releaseResourceRows(result.rows)
				n.resources.message = "Daftar belum tersedia. Tekan Muat ulang."
			} else {
				n.resources.acceptRows(result.rows)
				n.resources.message = "Pilihan tersimpan otomatis; hanya aplikasi idle diproses."
			}
		default:
		}
		user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
		return 0
	case 0x20a, 0x20e: // Pagination is explicit; wheel input never changes pages.
		return 0
	case 0x20: // WM_SETCURSOR: never inherit the shell's launch/busy cursor.
		if int16(l&0xffff) == 1 {
			cursor := n.arrowCursor
			var p point
			getCursorPos.Call(uintptr(unsafe.Pointer(&p)))
			user32.NewProc("ScreenToClient").Call(hwnd, uintptr(unsafe.Pointer(&p)))
			i := n.controlAt(int(p.x)*96/int(menuDPI), int(p.y)*96/int(menuDPI))
			if i == 25 {
				cursor, _, _ = loadCursorW.Call(0, 32513)
			} else {
				if i >= 0 && n.controlEnabled(i) && n.handCursor != 0 {
					cursor = n.handCursor
				}
			}
			user32.NewProc("SetCursor").Call(cursor)
			return 1
		}
	case wmUpdateProgress:
		n.updateProgress = uint64(w)
		n.downloadStarted = time.Now().Add(-time.Duration(min(l, uintptr(24*time.Hour/time.Millisecond))) * time.Millisecond)
		if percent := n.downloadStats(time.Now()).Percent(); percent >= 0 {
			n.downloadMotion.target(float64(percent)/100, time.Now())
			n.ensureAnimationTicker()
		}
		user32.NewProc("SetTimer").Call(hwnd, 0x4e44, 500, 0)
		user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
		return 0
	case wmUpdatePhase:
		oldPhase := n.updatePhase
		wasReady := n.updateAvailable && n.controlEnabled(7)
		n.updatePhase = min(uint32(w)&0xff, updatestate.Ready)
		n.updateAvailable = uint32(w)&updatestate.PendingFlag != 0
		if n.updatePhase == updatestate.Ready && oldPhase != updatestate.Ready {
			n.readyDismissed = false
		}
		if n.updateAvailable && n.controlEnabled(7) && !wasReady {
			n.noticeStarted = time.Now()
			n.ensureAnimationTicker()
		}
		if n.showReadyPrompt() {
			if n.focus == 4 || n.focus == 5 || n.focus == 9 {
				n.focus = -1
			}
			if n.pressed == 4 || n.pressed == 5 || n.pressed == 9 {
				i := n.pressed
				n.pressed = -1
				user32.NewProc("ReleaseCapture").Call()
				n.releaseControl(i, false)
			}
			if n.keyPressed == 4 || n.keyPressed == 5 || n.keyPressed == 9 {
				i := n.keyPressed
				n.keyPressed = -1
				n.releaseControl(i, false)
			}
		}
		if n.updatePhase == 2 {
			user32.NewProc("SetTimer").Call(hwnd, 0x4e44, 500, 0)
			if (n.focus >= 4 && n.focus <= 7) || n.focus == 9 {
				n.focus = -1
			}
			if (n.pressed >= 4 && n.pressed <= 7) || n.pressed == 9 {
				i := n.pressed
				n.pressed = -1
				user32.NewProc("ReleaseCapture").Call()
				n.releaseControl(i, false)
			}
			if (n.keyPressed >= 4 && n.keyPressed <= 7) || n.keyPressed == 9 {
				i := n.keyPressed
				n.keyPressed = -1
				n.releaseControl(i, false)
			}
		}
		if n.updatePhase != 2 {
			n.updateProgress = 0
			n.downloadStarted = time.Time{}
			user32.NewProc("KillTimer").Call(hwnd, 0x4e44)
			n.downloadMotion = nativeDownloadMotion{}
		}
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
		if n.reduceMotion() {
			t = 1
		}
		e := 1 - math.Pow(1-t, 3)
		lerp := func(a, b float64) float64 { return a + (b-a)*e }
		n.alpha = lerp(n.alphaFrom, n.alphaTo)
		n.slide = lerp(n.slideFrom, n.slideTo)
		n.plateX, n.plateY = lerp(n.plateFromX, n.plateToX), lerp(n.plateFromY, n.plateToY)
		for i := range n.switches {
			n.switches[i] = lerp(n.switchFrom[i], n.switchTo[i])
		}
		progressAnimating := n.downloadMotion.advance(time.Now())
		motionAnimating := n.advanceMotion(time.Now())
		_, noticeAnimating := updateNoticeScale(n.noticeStarted, time.Now())
		noticeAnimating = noticeAnimating && n.updateAvailable && n.controlEnabled(7)
		pressAnimating := false
		for i := range n.presses {
			if n.presses[i].advance(time.Now()) {
				pressAnimating = true
			}
		}
		user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
		user32.NewProc("UpdateWindow").Call(hwnd)
		if t >= 1 && (n.closing || (!pressAnimating && !progressAnimating && !noticeAnimating && !motionAnimating)) {
			n.endAnimation()
			if n.closing {
				destroyWindow.Call(hwnd)
			}
		}
		return 0
	case 0x201: // WM_LBUTTONDOWN
		n.keyboardFocus = false
		if n.closing {
			return 0
		}
		x, y := int(int16(l&0xffff))*96/int(menuDPI), int(int16(l>>16))*96/int(menuDPI)
		n.pressed = n.controlAt(x, y)
		if n.micPage && n.mic.dropdown != 0 && n.pressed < 0 {
			n.closeMicDropdown()
			return 0
		}
		if n.resources.optionOpen != 0 && n.pressed < 0 {
			n.resources.optionOpen = 0
			n.focus = -1
			user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
			return 0
		}
		if n.pressed >= 0 && !n.controlEnabled(n.pressed) {
			n.pressed = -1
		}
		if n.pressed >= 0 {
			user32.NewProc("SetCapture").Call(hwnd)
			n.pressControl(n.pressed)
		}
		return 0
	case 0x200: // WM_MOUSEMOVE, including captured drags outside the window.
		x, y := int(int16(l&0xffff))*96/int(menuDPI), int(int16(l>>16))*96/int(menuDPI)
		n.updateHover(n.controlAt(x, y))
		if !n.hoverTracking {
			tracking := struct {
				Size, Flags uint32
				Window      uintptr
				Time        uint32
			}{Flags: 2, Window: hwnd}
			tracking.Size = uint32(unsafe.Sizeof(tracking))
			user32.NewProc("TrackMouseEvent").Call(uintptr(unsafe.Pointer(&tracking)))
			n.hoverTracking = true
		}
		if n.pressed >= 0 {
			x, y := int(int16(l&0xffff))*96/int(menuDPI), int(int16(l>>16))*96/int(menuDPI)
			if n.controlAt(x, y) == n.pressed {
				n.pressControl(n.pressed)
			} else {
				n.releaseControl(n.pressed, false)
			}
		}
		return 0
	case 0x2a3: // WM_MOUSELEAVE
		n.hoverTracking = false
		n.updateHover(-1)
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
		clicked := i >= 0 && n.controlAt(x, y) == i && n.controlEnabled(i)
		n.releaseControl(i, clicked)
		if clicked {
			n.choose(nativeControlCommands[i])
		}
		return 0
	case wmSystemResult:
		n.pollSystem()
		user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
		return 0
	case wmCSSResult:
		if n.closing {
			return 0
		}
		n.busy = 0
		n.commandError = w != 0
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
		if n.micPage && n.mic.dropdown != 0 {
			if w == 27 {
				n.closeMicDropdown()
				return 0
			}
			if w == 38 || w == 40 {
				delta := 1
				if w == 38 {
					delta = -1
				}
				n.keyboardFocus = true
				n.focus = n.nextFocus(n.focus, delta)
				user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
				return 0
			}
		}
		if w == 9 || w == 38 || w == 40 || w == 13 || w == 32 {
			n.keyboardFocus = true
		}
		if w == 27 && n.resourcePage && n.resources.tuning && n.resources.optionOpen == 0 {
			n.resourceAction(319)
			return 0
		}
		if n.resources.optionOpen != 0 {
			if w == 27 {
				n.resources.optionOpen = 0
				n.focus = -1
				user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
				return 0
			}
			if w == 38 || w == 40 {
				delta := 1
				if w == 38 {
					delta = -1
				}
				n.focus = 30 + (n.focus-30+delta+len(resourceOptionValues(n.resources.optionOpen)))%len(resourceOptionValues(n.resources.optionOpen))
				user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
				return 0
			}
			if w != 9 && w != 13 && w != 32 {
				return 0
			}
		}
		if n.resourceSearchKey(w) {
			return 0
		}
		if w == 27 {
			n.animate(true)
		}
		if w == 9 {
			delta := 1
			shift, _, _ := user32.NewProc("GetKeyState").Call(0x10)
			if shift&0x8000 != 0 {
				delta = -1
			}
			n.focus = n.nextFocus(n.focus, delta)
			if n.focus == 25 {
				n.focusResourceSearch()
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
		if w == 0x534d {
			if n.micPage {
				if n.pollMic() {
					user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
				}
			} else {
				user32.NewProc("KillTimer").Call(hwnd, 0x534d)
			}
			return 0
		}
		if w == 0x5359 {
			if n.systemPage {
				n.pollSystem()
				if n.system.section == "" && !n.system.busy && time.Since(n.system.lastRead) > 15*time.Second {
					n.readSystem()
				}
				user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
			} else {
				user32.NewProc("KillTimer").Call(hwnd, 0x5359)
			}
			return 0
		}
		if w == resourceLoaderTimer {
			if n.resourceLoaderVisible() {
				user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
			} else {
				user32.NewProc("KillTimer").Call(hwnd, resourceLoaderTimer)
			}
			return 0
		}
		if w == 0x5254 {
			if n.resourcePage && n.focus == 25 && !n.closing {
				n.resources.caretVisible = !n.resources.caretVisible
				user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
			} else {
				user32.NewProc("KillTimer").Call(hwnd, 0x5254)
			}
			return 0
		}
		if w == 0x5253 && n.resourcePage {
			n.pollSystem()
			if !n.system.busy && time.Since(n.system.lastRead) > 5*time.Second {
				n.readSystem()
			}
			status := resources.ReadStatus()
			if status.Updated.After(n.resources.updated) {
				n.resources.updated = status.Updated
				n.resources.message = status.Message
				user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
			}
			return 0
		}
		if w == 0x4e44 {
			if n.updatePhase == 2 && !n.closing {
				user32.NewProc("InvalidateRect").Call(hwnd, 0, 0)
			} else {
				user32.NewProc("KillTimer").Call(hwnd, 0x4e44)
			}
			return 0
		}
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
	defer n.resources.dispose()
	if preview && len(os.Args) > 1 && (os.Args[1] == "--native-check" || os.Args[1] == "--native-control") {
		n.preview = false
	}
	n.actual, n.desired = n.state, n.state
	if !n.preview {
		n.parent, _, _ = findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr(windowClass))), 0)
		if n.parent == 0 {
			if len(os.Args) < 2 || os.Args[1] != "--native-control" {
				return false
			}
		}
		var supported uintptr
		if n.parent != 0 {
			user32.NewProc("SendMessageTimeoutW").Call(n.parent, wmApp+81, 0, 0, 2, 200, uintptr(unsafe.Pointer(&supported)))
		}
		if len(os.Args) > 1 && os.Args[1] == "--native-control" && supported != 1 {
			n.localResources = resources.Start()
			defer n.localResources.Close()
			stopFocus := n.localResources.WatchForeground()
			defer stopFocus()
		}
		var phase uintptr
		user32.NewProc("SendMessageTimeoutW").Call(n.parent, wmUpdateQuery, 0, 0, 2, 200, uintptr(unsafe.Pointer(&phase)))
		n.updatePhase = min(uint32(phase)&0xff, updatestate.Ready)
		n.updateAvailable = uint32(phase)&updatestate.PendingFlag != 0
		if n.updatePhase == 2 {
			var progress uintptr
			user32.NewProc("SendMessageTimeoutW").Call(n.parent, wmUpdateProgressQuery, 0, 0, 2, 200, uintptr(unsafe.Pointer(&progress)))
			n.updateProgress = uint64(progress)
			var elapsed uintptr
			user32.NewProc("SendMessageTimeoutW").Call(n.parent, wmUpdateElapsedQuery, 0, 0, 2, 200, uintptr(unsafe.Pointer(&elapsed)))
			n.downloadStarted = time.Now().Add(-time.Duration(elapsed) * time.Millisecond)
			if percent := n.downloadStats(time.Now()).Percent(); percent >= 0 {
				n.downloadMotion.value = float64(percent) / 100
				n.downloadMotion.to = n.downloadMotion.value
			}
		}
	}
	nativePanel = n
	defer func() { nativePanel = nil }()
	// The preview is an app window solely so supported UI tools can inspect it.
	className := utf16ptr("SenjaNativeGlassPreview")
	if preview && !n.preview {
		className = utf16ptr("SenjaNativeCheckPanel")
	}
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
	menuDPI = min(menuDPI, (mi.Work.bottom-mi.Work.top)*96/556, (mi.Work.right-mi.Work.left)*96/628)
	menuDPI = max(48, menuDPI)
	n.w, n.h = menuPx(nativePanelWidth), menuPx(548)
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
		n.bounds.left = max(mi.Work.left, min(n.bounds.right-menuPx(nativePanelWidth), mi.Work.right-menuPx(nativePanelWidth)))
		n.bounds.right = n.bounds.left + menuPx(nativePanelWidth)
		x, y = n.bounds.left, n.bounds.top
		n.w, n.h = n.bounds.right-x, n.bounds.bottom-y
		exStyle = wsExToolWindow | 8 | 0x80000
		if cssOverflow != 0 {
			exStyle |= wsExNoActivate
		}
	}
	cssPopupBounds = n.bounds
	title := "Senja Bar"
	if n.preview {
		title = "Senja — Preview"
	}
	n.hwnd, _, _ = createWindowExW.Call(exStyle, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16ptr(title))), wsPopup, uintptr(x), uintptr(y+menuPx(12)), uintptr(n.w), uintptr(n.h), 0, 0, uintptr(getModuleHandle()), 0)
	if n.hwnd == 0 {
		return false
	}
	defer destroyWindow.Call(n.hwnd)
	n.prepareSoftBackdrop()
	defer n.backdrop.close()
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
	n.plateX, n.plateY = float64(20+i%2*164), float64(102+i/2*94)
	if n.state.Rounded {
		n.switches[0] = 17
	}
	if n.state.Startup {
		n.switches[1] = 17
	}
	if n.state.AutoInstall {
		n.switches[2] = 17
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
	if n.updatePhase == 2 {
		user32.NewProc("SetTimer").Call(n.hwnd, 0x4e44, 500, 0)
	}
	if n.updateAvailable && n.controlEnabled(7) {
		n.noticeStarted = time.Now()
	}
	if n.preview && len(os.Args) > 1 && (os.Args[1] == "--native-resource-preview" || os.Args[1] == "--native-loader-preview" || os.Args[1] == "--native-tuning-preview") {
		n.resourcePage = true
		n.resources.config = resources.Load()
		n.resources.config.AllBackground = os.Args[1] != "--native-loader-preview"
		n.resources.loaded = true
		if os.Args[1] == "--native-tuning-preview" {
			n.resources.config.AllBackground = false
			n.resources.tuning = true
		}
		if os.Args[1] == "--native-loader-preview" {
			// Isolated visual fixture only; production scans are never delayed.
			n.resources.loading = true
			n.resources.loadingStarted = time.Now()
			n.resources.config.Automatic = false
			n.resources.config.EcoCPU = false
			n.syncResourceLoader()
		}
		user32.NewProc("SetTimer").Call(n.hwnd, 0x5253, 1000, 0)
	}
	if len(os.Args) > 1 && os.Args[1] == "--native-control" {
		n.resourcePage = true
		n.resources.config = resources.Load()
		n.resources.loaded = true
		if !n.resources.config.AllBackground {
			n.scanResources()
		}
		user32.NewProc("SetTimer").Call(n.hwnd, 0x5253, 1000, 0)
	}
	n.animate(false)
	messageLoop()
	return true
}

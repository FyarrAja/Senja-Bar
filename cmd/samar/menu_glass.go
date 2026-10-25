//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// Material capture is local and ephemeral: one small region before the popup
// opens, no timer, no files, no network. Native menus keep keyboard/MSAA input.
var (
	glassSetHook                                        = user32.NewProc("SetWinEventHook")
	glassUnhook                                         = user32.NewProc("UnhookWinEvent")
	glassCallback                                       = syscall.NewCallback(glassMenuEvent)
	glassMenu                                           uintptr
	glassTextureDC, glassTextureBitmap, glassTextureOld uintptr
	glassTextureBounds                                  rect
	glassClientOrigin                                   point
	glassTexturePixels                                  uintptr
)

func stopMenuGlass() {
	glassMenu = 0
	if glassTextureDC != 0 {
		menuSelect.Call(glassTextureDC, glassTextureOld)
		deleteObject.Call(glassTextureBitmap)
		gdi32.NewProc("DeleteDC").Call(glassTextureDC)
	}
	glassTextureDC, glassTextureBitmap, glassTextureOld = 0, 0, 0
	glassTexturePixels = 0
}
func startMenuGlass() func() {
	hook, _, _ := glassSetHook.Call(6, 7, 0, glassCallback, uintptr(os.Getpid()), 0, 0)
	return func() {
		if hook != 0 {
			glassUnhook.Call(hook)
		}
		stopMenuGlass()
	}
}
func glassMenuEvent(_ uintptr, event uint32, hwnd uintptr, _ int32, _ int32, _ uint32, _ uint32) uintptr {
	defer recoverPanic("glass popup")
	if event != 6 || !menuIsOpen || hwnd == 0 || glassTextureDC == 0 {
		return 0
	}
	var className [64]uint16
	getClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&className[0])), 64)
	if syscall.UTF16ToString(className[:]) != "#32768" {
		return 0
	}
	glassMenu = hwnd
	glassClientOrigin = point{}
	user32.NewProc("ClientToScreen").Call(hwnd, uintptr(unsafe.Pointer(&glassClientOrigin)))
	corner, dark, border, none := uint32(2), uint32(0), uint32(menuColor(0xe3f0fb)), uint32(1)
	dwmSetWindowAttribute.Call(hwnd, 38, uintptr(unsafe.Pointer(&none)), 4)
	dwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4)
	dwmSetWindowAttribute.Call(hwnd, 33, uintptr(unsafe.Pointer(&corner)), 4)
	dwmSetWindowAttribute.Call(hwnd, 34, uintptr(unsafe.Pointer(&border)), 4)
	user32.NewProc("RedrawWindow").Call(hwnd, 0, 0, 0x505)
	logEvent("INFO", "menu.glass", "frosted local backdrop ready; opaque text; refresh on open")
	return 0
}
func paintMenuBackdrop(dc uintptr, bounds rect) {
	if glassTextureDC != 0 && glassMenu != 0 {
		x := glassClientOrigin.x + bounds.left - glassTextureBounds.left
		y := glassClientOrigin.y + bounds.top - glassTextureBounds.top
		w, h := bounds.right-bounds.left, bounds.bottom-bounds.top
		if x >= 0 && y >= 0 && x+w <= glassTextureBounds.right-glassTextureBounds.left && y+h <= glassTextureBounds.bottom-glassTextureBounds.top {
			if ok, _, _ := gdi32.NewProc("BitBlt").Call(dc, uintptr(bounds.left), uintptr(bounds.top), uintptr(w), uintptr(h), glassTextureDC, uintptr(x), uintptr(y), 0x00cc0020); ok != 0 {
				return
			}
		}
	}
	paintMenuFill(dc, bounds, 0xd4e4f1, 0)
}
func prepareMenuBackdrop(anchor point, height int32) {
	prepareMenuBackdropMaterial(anchor, height, true)
}
func prepareMenuBackdropMaterial(anchor point, height int32, light bool) {
	stopMenuGlass()
	marginX, marginY := menuPx(336), menuPx(height+36)
	x0, _, _ := getSystemMetrics.Call(76)
	y0, _, _ := getSystemMetrics.Call(77)
	vw, _, _ := getSystemMetrics.Call(78)
	vh, _, _ := getSystemMetrics.Call(79)
	left, top := anchor.x-marginX, anchor.y-marginY
	right, bottom := anchor.x+marginX, anchor.y+marginY
	if left < int32(x0) {
		left = int32(x0)
	}
	if top < int32(y0) {
		top = int32(y0)
	}
	if right > int32(x0)+int32(vw) {
		right = int32(x0) + int32(vw)
	}
	if bottom > int32(y0)+int32(vh) {
		bottom = int32(y0) + int32(vh)
	}
	w, h := right-left, bottom-top
	if w <= 0 || h <= 0 {
		return
	}
	screen, _, _ := user32.NewProc("GetDC").Call(0)
	if screen == 0 {
		return
	}
	defer user32.NewProc("ReleaseDC").Call(0, screen)
	memory, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(screen)
	if memory == 0 {
		return
	}
	header := struct {
		size                   uint32
		width, height          int32
		planes, bits           uint16
		compression, imageSize uint32
		xppm, yppm             int32
		used, important        uint32
	}{size: 40, width: w, height: -h, planes: 1, bits: 32}
	var data uintptr
	bitmap, _, _ := gdi32.NewProc("CreateDIBSection").Call(memory, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&data)), 0, 0)
	if bitmap == 0 {
		gdi32.NewProc("DeleteDC").Call(memory)
		return
	}
	old, _, _ := menuSelect.Call(memory, bitmap)
	glassTextureDC, glassTextureBitmap, glassTextureOld = memory, bitmap, old
	glassTexturePixels = data
	glassTextureBounds = rect{left, top, right, bottom}
	ok, _, _ := gdi32.NewProc("BitBlt").Call(memory, 0, 0, uintptr(w), uintptr(h), screen, uintptr(left), uintptr(top), 0x00cc0020)
	if ok == 0 {
		stopMenuGlass()
		return
	}
	gdi32.NewProc("GdiFlush").Call()
	pixels := unsafe.Slice((*byte)(unsafe.Pointer(data)), int(w*h*4))
	if light {
		frostedBlur(pixels, int(w), int(h), int(menuPx(9)))
	} else {
		blurBackdropPixels(pixels, int(w), int(h), int(menuPx(2)))
		// Retain the real backdrop, without the old 72% icy-white wash.
		// Bound highlights so white labels remain readable over light windows.
		limitGlassHighlights(pixels)
	}
}
func limitGlassHighlights(pixels []byte) {
	for i := 0; i < len(pixels); i += 4 {
		// BGR perceived brightness, not the strongest channel: saturated blue,
		// red and violet keep their color instead of all being capped to gray.
		brightness := (72*uint32(pixels[i]) + 715*uint32(pixels[i+1]) + 213*uint32(pixels[i+2])) / 1000
		if brightness > 105 {
			for c := 0; c < 3; c++ {
				pixels[i+c] = byte(uint32(pixels[i+c]) * 105 / brightness)
			}
		}
		pixels[i+3] = 255
	}
}
func frostedBlur(pixels []byte, w, h, radius int) {
	blurBackdropPixels(pixels, w, h, radius)
	tintFrostedPixels(pixels)
}
func blurBackdropPixels(pixels []byte, w, h, radius int) {
	if radius < 1 {
		radius = 1
	}
	temporary := make([]byte, len(pixels))
	for pass := 0; pass < 3; pass++ {
		boxBlur(pixels, temporary, w, h, radius, true)
		boxBlur(temporary, pixels, w, h, radius, false)
	}
}
func tintFrostedPixels(pixels []byte) {
	// Icy white material follows AutoHz's #4DFFFFFF glass surfaces. A pale
	// blue base keeps the glass legible even against a completely black desktop.
	// The captured backdrop still contributes its blurred shapes and color.
	for i := 0; i < len(pixels); i += 4 {
		pixels[i] = byte(float64(pixels[i])*.28 + 250*.72)
		pixels[i+1] = byte(float64(pixels[i+1])*.28 + 238*.72)
		pixels[i+2] = byte(float64(pixels[i+2])*.28 + 222*.72)
		pixels[i+3] = 255
	}
}
func boxBlur(source, destination []byte, w, h, radius int, horizontal bool) {
	lines, length := h, w
	if !horizontal {
		lines, length = w, h
	}
	span := radius*2 + 1
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		if v >= length {
			return length - 1
		}
		return v
	}
	for line := 0; line < lines; line++ {
		offset := func(position int) int {
			if horizontal {
				return (line*w + position) * 4
			}
			return (position*w + line) * 4
		}
		sums := [3]int{}
		for v := -radius; v <= radius; v++ {
			at := offset(clamp(v))
			for c := 0; c < 3; c++ {
				sums[c] += int(source[at+c])
			}
		}
		for position := 0; position < length; position++ {
			at := offset(position)
			for c := 0; c < 3; c++ {
				destination[at+c] = byte(sums[c] / span)
			}
			destination[at+3] = 255
			remove, add := offset(clamp(position-radius)), offset(clamp(position+radius+1))
			for c := 0; c < 3; c++ {
				sums[c] += int(source[add+c]) - int(source[remove+c])
			}
		}
	}
}

//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const nativePanelRadius = 24

// The material uses its own non-layered blur host. Applying accent blur to
// the UpdateLayeredWindow control surface turns its alpha into black on some
// Windows builds. The foreground keeps its antialiased alpha and hit testing.
type nativeLiveBackdrop struct{ hwnd uintptr }

func (b *nativeLiveBackdrop) close() {
	if b.hwnd != 0 {
		destroyWindow.Call(b.hwnd)
		b.hwnd = 0
	}
}
func nativeBackdropProc(hwnd uintptr, message uint32, w, l uintptr) uintptr {
	switch message {
	case 0x83:
		return 0 // Full client area; retain frame styles for DWM rounding.
	case 0x84:
		return ^uintptr(0) // HTTRANSPARENT: never consumes panel input.
	case 0x21:
		return 3 // MA_NOACTIVATE.
	case 0x14:
		return 1
	case 0xf:
		var ps previewPaint
		user32.NewProc("BeginPaint").Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		user32.NewProc("EndPaint").Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	}
	r, _, _ := defWindowProcW.Call(hwnd, uintptr(message), w, l)
	return r
}

var nativeBackdropCallback = syscall.NewCallback(nativeBackdropProc)

func (n *nativeGlassPanel) prepareSoftBackdrop() {
	name := utf16ptr("SenjaLiveMaterial")
	wc := wndClassEx{cbSize: uint32(unsafe.Sizeof(wndClassEx{})), lpfnWndProc: nativeBackdropCallback, hInstance: getModuleHandle(), lpszClassName: name}
	registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	// Tool/no-activate/transparent; deliberately no WS_EX_LAYERED.
	hwnd, _, _ := createWindowExW.Call(0x080000a0, uintptr(unsafe.Pointer(name)), 0, wsPopup|0x00c40000, uintptr(n.bounds.left), uintptr(n.bounds.top), uintptr(n.w), uintptr(n.h), 0, 0, uintptr(getModuleHandle()), 0)
	if hwnd == 0 {
		return
	}
	n.backdrop.hwnd = hwnd
	// System Acrylic supplies an opaque inactive fallback. Use accent blur on
	// this separate host instead, and clip it to the foreground's larger radius.
	dwm := syscall.NewLazyDLL("dwmapi.dll")
	corner, material, border, dark := uint32(1), uint32(1), uint32(0xfffffffe), uint32(1)
	for _, attr := range []struct {
		id    uint32
		value *uint32
	}{{33, &corner}, {38, &material}, {34, &border}, {20, &dark}} {
		hr, _, _ := dwm.NewProc("DwmSetWindowAttribute").Call(hwnd, uintptr(attr.id), uintptr(unsafe.Pointer(attr.value)), 4)
		if int32(hr) < 0 {
			n.backdrop.close()
			return
		}
	}
	margins := [4]int32{-1, -1, -1, -1}
	dwm.NewProc("DwmExtendFrameIntoClientArea").Call(hwnd, uintptr(unsafe.Pointer(&margins)))
	// An explicit low-alpha tint keeps the live acrylic translucent instead
	// of using the system backdrop's opaque inactive material.
	policy := accentPolicy{state: 4, flags: 2, gradient: 0x241c1c1c}
	if n.light {
		policy.gradient = 0x24ffffff
	}
	data := compositionAttributeData{attribute: 19, data: uintptr(unsafe.Pointer(&policy)), dataSize: unsafe.Sizeof(policy)}
	if ok, _, _ := setWindowComposition.Call(hwnd, uintptr(unsafe.Pointer(&data))); ok == 0 {
		n.backdrop.close()
		return
	}
	diameter := uintptr(nativePanelRadius * 2 * int(menuDPI) / 96)
	region, _, _ := createRoundRectRgn.Call(0, 0, uintptr(n.w+1), uintptr(n.h+1), diameter, diameter)
	if region == 0 {
		n.backdrop.close()
		return
	}
	if ok, _, _ := setWindowRgn.Call(hwnd, region, 0); ok == 0 {
		deleteObject.Call(region)
		n.backdrop.close()
		return
	}
	user32.NewProc("SetWindowPos").Call(hwnd, 0, 0, 0, 0, 0, 0x0037)
}
func (n *nativeGlassPanel) syncSoftBackdrop(p point) {
	if n.backdrop.hwnd == 0 {
		return
	}
	if n.closing || n.alpha < 250 {
		user32.NewProc("ShowWindow").Call(n.backdrop.hwnd, 0)
		return
	}
	// Immediately behind the foreground, no activation and no independent move.
	user32.NewProc("SetWindowPos").Call(n.backdrop.hwnd, n.hwnd, uintptr(p.x), uintptr(p.y), uintptr(n.w), uintptr(n.h), 0x0050)
}

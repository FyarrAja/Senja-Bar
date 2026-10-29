//go:build windows

package main

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestMaterialHostUsesRoundedBlurAndSeparateAlphaSurface(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	old := menuDPI
	menuDPI = 96
	defer func() { menuDPI = old }()
	n := nativeGlassPanel{w: 472, h: 548, bounds: rect{100, 100, 572, 648}}
	n.prepareSoftBackdrop()
	defer n.backdrop.close()
	hwnd := n.backdrop.hwnd
	if hwnd == 0 {
		t.Fatal("material host creation failed")
	}
	style, _, _ := user32.NewProc("GetWindowLongW").Call(hwnd, ^uintptr(19))
	if style&0x80000 != 0 {
		t.Fatal("material must not use layered alpha rendering")
	}
	if style&0x08000000 == 0 {
		t.Fatal("material must not activate independently")
	}
	var corner, material uint32
	dwm := syscall.NewLazyDLL("dwmapi.dll")
	for _, attr := range []struct {
		id    uint32
		value *uint32
	}{{33, &corner}, {38, &material}} {
		hr, _, _ := dwm.NewProc("DwmGetWindowAttribute").Call(hwnd, uintptr(attr.id), uintptr(unsafe.Pointer(attr.value)), 4)
		if int32(hr) < 0 {
			t.Fatal("DWM material attribute unavailable")
		}
	}
	if corner != 1 || material != 1 {
		t.Fatalf("system Acrylic and its independent corner clip must be disabled: %d/%d", corner, material)
	}
	region, _, _ := createRoundRectRgn.Call(0, 0, 1, 1, 0, 0)
	defer deleteObject.Call(region)
	if kind, _, _ := user32.NewProc("GetWindowRgn").Call(hwnd, region); kind != 3 {
		t.Fatal("blur host must have a rounded, nonrectangular region")
	}
	if inside, _, _ := gdi32.NewProc("PtInRegion").Call(region, 1, 1); inside != 0 {
		t.Fatal("blur must not leak through the transparent outer corner")
	}
	if inside, _, _ := gdi32.NewProc("PtInRegion").Call(region, 24, 24); inside == 0 {
		t.Fatal("rounded blur region must retain the panel interior")
	}
	n.backdrop.close()
	alive, _, _ := user32.NewProc("IsWindow").Call(hwnd)
	if alive != 0 {
		t.Fatal("material host leaked after closing")
	}
}

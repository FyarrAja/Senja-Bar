//go:build windows

package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// A visible, local QA surface lets Windows render real backdrop blur above
// contrasting colors and sharp edges. It never changes the taskbar settings.
var materialPreview bool
var menuPreviewPoint *point
var previewConfig = config{style: styleClear, startup: true}

type previewPaint struct {
	dc              uintptr
	erase           int32
	bounds          rect
	restore, update int32
	reserved        [32]byte
}

func glassPreviewProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case 0x002C:
		if measureThemedMenu(lParam) {
			return 1
		}
	case 0x002B:
		if drawThemedMenu(lParam) {
			return 1
		}
	case 0x0014:
		return 1
	case 0x000F:
		var paint previewPaint
		dc, _, _ := user32.NewProc("BeginPaint").Call(hwnd, uintptr(unsafe.Pointer(&paint)))
		var client rect
		user32.NewProc("GetClientRect").Call(hwnd, uintptr(unsafe.Pointer(&client)))
		if len(os.Args) > 3 && os.Args[3] == "--light" {
			paintMenuFill(dc, client, 0xf3f5fa, 0)
			paintMenuText(dc, "Senja · light backdrop test", rect{30, 20, client.right - 30, 54}, 22, 600, 0x18202d)
			user32.NewProc("EndPaint").Call(hwnd, uintptr(unsafe.Pointer(&paint)))
			return 0
		}
		paintMenuFill(dc, client, 0x243c68, 0)
		paintMenuFill(dc, rect{client.right / 2, 0, client.right, client.bottom}, 0x9b576c, 0)
		paintMenuFill(dc, rect{0, client.bottom / 2, client.right / 2, client.bottom}, 0x286f78, 0)
		paintMenuFill(dc, rect{client.right / 2, client.bottom / 2, client.right, client.bottom}, 0x665498, 0)
		for x := int32(260); x < client.right-80; x += 24 {
			paintMenuFill(dc, rect{x, 110, x + 8, client.bottom - 50}, 0xcac8c0, 0)
		}
		paintMenuText(dc, "Senja · uji material kaca", rect{30, 20, client.right - 30, 54}, 22, 600, 0xffffff)
		paintMenuText(dc, "Warna dan garis di belakang menu harus tampak kabur. Esc menutup menu.", rect{30, 56, client.right - 30, 84}, 13, 400, 0xffffff)
		user32.NewProc("EndPaint").Call(hwnd, uintptr(unsafe.Pointer(&paint)))
		return 0
	case wmShowMenu:
		var frame rect
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&frame)))
		anchor := point{frame.left + 335, frame.top + 125}
		menuPreviewPoint = &anchor
		showThemedTrayMenu(syscall.Handle(hwnd), &previewConfig)
		menuPreviewPoint = nil
		return 0
	case wmClose:
		destroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}
	result, _, _ := defWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func runGlassMaterialPreview() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	materialPreview = true
	prepareProcess()
	openLog()
	defer closeLog()
	defer stopMenuGlass()
	className := utf16ptr("SenjaMaterialPreview")
	class := wndClassEx{cbSize: uint32(unsafe.Sizeof(wndClassEx{})), lpfnWndProc: syscall.NewCallback(glassPreviewProc), hInstance: getModuleHandle(), lpszClassName: className}
	if atom, _, _ := registerClassExW.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		return
	}
	x, y, w, h, style := int32(540), int32(170), int32(840), int32(700), uintptr(0x00cf0000)
	for _, arg := range os.Args {
		if arg == "--taskbar-backdrop" {
			monitor, _, _ := user32.NewProc("MonitorFromWindow").Call(0, 1)
			mi := cssMonitorInfo{Size: uint32(unsafe.Sizeof(cssMonitorInfo{}))}
			user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&mi)))
			x, y, w, h, style = mi.Work.left, mi.Work.bottom-220, mi.Work.right-mi.Work.left, 220, wsPopup
		}
	}
	hwnd, _, _ := createWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16ptr("Senja — Material Preview"))), style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, uintptr(getModuleHandle()), 0)
	if hwnd == 0 {
		return
	}
	user32.NewProc("ShowWindow").Call(hwnd, 1)
	user32.NewProc("ShowWindow").Call(hwnd, 5) // The first call may honor a hidden launcher.
	user32.NewProc("UpdateWindow").Call(hwnd)
	if len(os.Args) < 3 || os.Args[2] != "--background-only" {
		postMessageW.Call(hwnd, wmShowMenu, 0, 0)
	}
	messageLoop()
}

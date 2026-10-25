//go:build windows

package main

import (
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestRealOverflowHideEventPostsClose(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	closed := false
	proc := syscall.NewCallback(func(hwnd uintptr, message uint32, w, l uintptr) uintptr {
		if message == wmClose {
			closed = true
			return 0
		}
		r, _, _ := defWindowProcW.Call(hwnd, uintptr(message), w, l)
		return r
	})
	name := utf16ptr("SenjaOverflowLifecycleTest")
	class := wndClassEx{cbSize: uint32(unsafe.Sizeof(wndClassEx{})), lpfnWndProc: proc, hInstance: getModuleHandle(), lpszClassName: name}
	atom, _, _ := registerClassExW.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		t.Fatal("register test window")
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), uintptr(getModuleHandle()))
	makeWindow := func() uintptr {
		hwnd, _, _ := createWindowExW.Call(0, uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 10, 10, 0, 0, uintptr(getModuleHandle()), 0)
		if hwnd == 0 {
			t.Fatal("create test window")
		}
		return hwnd
	}
	popup, overflow := makeWindow(), makeWindow()
	defer destroyWindow.Call(popup)
	defer destroyWindow.Call(overflow)
	stop := watchTrayOverflow(popup, overflow)
	defer stop()
	cssCommandGrace = time.Time{}
	user32.NewProc("NotifyWinEvent").Call(0x8003, overflow, 0, 0)
	deadline := time.Now().Add(time.Second)
	var message msg
	for !closed && time.Now().Before(deadline) {
		ok, _, _ := user32.NewProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, 1)
		if ok != 0 {
			dispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
		} else {
			time.Sleep(time.Millisecond)
		}
	}
	if !closed {
		t.Fatal("real overflow hide did not dismiss the popup")
	}
}

func TestNativeFadeHasContinuousEndpointsAndCanReverse(t *testing.T) {
	duration := 320 * time.Millisecond
	for _, endpoints := range [][2]byte{{0, 255}, {255, 0}, {120, 0}, {120, 255}} {
		previous := endpoints[0]
		if fadeAlpha(endpoints[0], endpoints[1], 0, duration) != previous {
			t.Fatal("fade must start at current opacity")
		}
		for elapsed := time.Duration(0); elapsed <= duration; elapsed += time.Millisecond {
			current := fadeAlpha(endpoints[0], endpoints[1], elapsed, duration)
			if endpoints[0] < endpoints[1] && current < previous || endpoints[0] > endpoints[1] && current > previous {
				t.Fatal("fade jumped in the wrong direction")
			}
			previous = current
		}
		if previous != endpoints[1] {
			t.Fatal("fade did not reach target opacity")
		}
	}
}

func TestCSSPopupFitsMonitorAtDifferentDPIs(t *testing.T) {
	for _, tc := range []struct {
		work   rect
		anchor point
		dpi    int32
	}{
		{rect{0, 0, 1920, 1040}, point{1915, 1030}, 96},
		{rect{-1920, 0, 0, 1040}, point{-1915, 10}, 144},
		{rect{0, 0, 800, 560}, point{400, 550}, 192},
	} {
		got := cssBounds(tc.anchor, tc.dpi, tc.work)
		if got.left < tc.work.left || got.top < tc.work.top || got.right > tc.work.right || got.bottom > tc.work.bottom || got.right <= got.left || got.bottom <= got.top {
			t.Fatalf("popup outside work area: %+v", got)
		}
	}
}

func TestCSSPopupOffsetRightAboveTrayIcon(t *testing.T) {
	for _, dpi := range []int32{96, 144, 192} {
		anchor := point{1600, 1800}
		got := cssBounds(anchor, dpi, rect{0, 0, 3200, 2100})
		if (got.left+got.right)/2 != anchor.x+(68*dpi+48)/96 {
			t.Fatalf("popup not offset right at DPI %d: %+v", dpi, got)
		}
		gap := (4*dpi + 48) / 96
		if got.bottom != anchor.y-gap {
			t.Fatalf("popup not above icon at DPI %d: %+v", dpi, got)
		}
	}
}

func TestPopupDismissalKeepsModeClicksInside(t *testing.T) {
	bounds := rect{100, 100, 460, 648}
	for _, position := range []point{{100, 100}, {150, 250}, {350, 250}, {150, 350}, {350, 350}, {459, 647}} {
		if !popupContainsPoint(bounds, position) {
			t.Fatalf("mode/menu click must keep popup open: %+v", position)
		}
	}
	for _, position := range []point{{99, 100}, {460, 200}, {300, 648}} {
		if popupContainsPoint(bounds, position) {
			t.Fatalf("outside click must dismiss popup: %+v", position)
		}
	}
}

func TestOverflowHideExemptionIsBoundedToMaterialInteraction(t *testing.T) {
	for _, tc := range []struct{ grace, inside, want bool }{
		{false, true, false}, // Hovering cannot keep a hidden overflow alive.
		{false, false, false},
		{true, false, false}, // Clicking the arrow always dismisses.
		{true, true, true},   // Keep the chooser open during its own material update.
	} {
		if got := suppressOverflowHide(tc.grace, tc.inside); got != tc.want {
			t.Fatalf("grace=%t inside=%t: got %t", tc.grace, tc.inside, got)
		}
	}
}

func TestCSSBridgeAllowsOnlyVisibleCommands(t *testing.T) {
	for _, id := range []uint32{100, 101, 102, 103, 105, 110, 111, 112, 140} {
		if !validCSSCommand(id) {
			t.Fatalf("visible command %d rejected", id)
		}
	}
	for _, id := range []uint32{0, 104, 113, 600, 0xffffffff} {
		if validCSSCommand(id) {
			t.Fatalf("unexpected command %d allowed", id)
		}
	}
}

func TestCSSDocumentHasStateAndRestrictedLocalUI(t *testing.T) {
	html := cssHTML(cssState{Style: styleClear, Rounded: true, Startup: false}, "")
	for _, value := range []string{`"style":3`, `"rounded":true`, `"startup":false`, `connect-src 'none'`, `role="radiogroup"`, `aria-live="polite"`, `window.chrome?.webview?.postMessage`} {
		if !strings.Contains(html, value) {
			t.Fatalf("missing document contract: %s", value)
		}
	}
	for _, token := range []string{"__CSS__", "__JS__", "__STATE__"} {
		if strings.Contains(html, token) {
			t.Fatalf("unexpanded token %s", token)
		}
	}
}

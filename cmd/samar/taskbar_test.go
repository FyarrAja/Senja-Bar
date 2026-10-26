//go:build windows

package main

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestEveryCustomTaskbarModeDisablesBorderAndRestoreEnablesIt(t *testing.T) {
	oldPalettes := adaptivePalettes
	defer func() { adaptivePalettes = oldPalettes }()
	adaptivePalettes = make(map[syscall.Handle]taskbarPalette)
	oldAvailable, oldService, oldSurfaces := tapAvailable, tapServicePtr, tapSurfaces
	defer func() { tapAvailable, tapServicePtr, tapSurfaces = oldAvailable, oldService, oldSurfaces }()
	var border uintptr = 99
	vtable := [9]uintptr{}
	vtable[3] = syscall.NewCallback(func(_, _, _, _ uintptr) uintptr { return 0 })
	vtable[4] = syscall.NewCallback(func(_, _, _, _ uintptr) uintptr { return 0 })
	vtable[5] = syscall.NewCallback(func(_, _ uintptr) uintptr { return 0 })
	vtable[6] = syscall.NewCallback(func(_, _, value uintptr) uintptr { border = value; return 0 })
	service := uintptr(unsafe.Pointer(&vtable[0]))
	tapAvailable, tapServicePtr = true, uintptr(unsafe.Pointer(&service))
	for _, style := range []int{styleGlass, styleAcrylic, styleClear, styleSolid, styleNormal} {
		host := styleClear
		want := uintptr(0)
		if style == styleNormal {
			host = styleNormal
			want = 1
		}
		hwnd := syscall.Handle(0x7fffffff) // Invalid HWND: no live taskbar calls.
		tapSurfaces = map[syscall.Handle]tapSurfaceState{hwnd: {host: host, hostKnown: true}}
		border = 99
		if !setAccent(hwnd, style, false) || border != want {
			t.Fatalf("style %d: border=%d, want %d", style, border, want)
		}
	}
	runtime.KeepAlive(vtable)
	runtime.KeepAlive(service)
}

func TestTAPBorderAvoidsRedundantCOMCallsAndRetriesFailures(t *testing.T) {
	original := tapSurfaces
	tapSurfaces = make(map[syscall.Handle]tapSurfaceState)
	defer func() { tapSurfaces = original }()
	calls := 0
	success := syscall.NewCallback(func(_, _, _ uintptr) uintptr { calls++; return 0 })
	failure := syscall.NewCallback(func(_, _, _ uintptr) uintptr { calls++; return 0x80004005 })
	if !setTAPBorder(123, success, 1) || !setTAPBorder(123, success, 1) || calls != 1 {
		t.Fatal("unchanged border was reapplied")
	}
	if setTAPBorder(123, failure, 0) || calls != 2 {
		t.Fatal("failure accepted or skipped")
	}
	if !setTAPBorder(123, success, 0) || calls != 3 {
		t.Fatal("failed state was cached")
	}
	invalidateTAPSurfaces()
	if !setTAPBorder(123, success, 0) || calls != 4 {
		t.Fatal("theme refresh did not invalidate cache")
	}
}

func TestTAPCallRejectsCOMFailure(t *testing.T) {
	success := syscall.NewCallback(func() uintptr { return 0 })
	inputSyncFailure := syscall.NewCallback(func() uintptr { return 0x8001010d })
	if !tapCall(success) {
		t.Fatal("successful COM call rejected")
	}
	if tapCall(inputSyncFailure) {
		t.Fatal("COM input-synchronous failure incorrectly reported as applied")
	}
}

func TestClearTaskbarUsesTransparentColorInsteadOfSystemAccent(t *testing.T) {
	policy := accentForStyle(styleClear)
	if policy.state != 2 || policy.flags&2 == 0 || policy.gradient != 0 {
		t.Fatalf("clear mode must explicitly select zero-alpha color: %+v", policy)
	}
}

func TestMaterialConsistencyAcrossNativeAndXAML(t *testing.T) {
	glass, acrylic, clear, normal := accentForStyle(styleGlass), accentForStyle(styleAcrylic), accentForStyle(styleClear), accentForStyle(styleNormal)
	palette := taskbarColors(false, backdropRGB{21, 21, 21})
	if glass.gradient != palette.glass || acrylic.gradient != palette.acrylic {
		t.Fatal("native fallback and XAML must use identical tint values")
	}
	if glass.gradient>>24 >= acrylic.gradient>>24 || clear.gradient != 0 {
		t.Fatal("expected clear < glass < acrylic tint strength")
	}
	if normal.state != 0 || normal.flags != 0 || normal.gradient != 0 {
		t.Fatal("Windows mode must disable custom native material")
	}
}

func TestSolidRemainsDistinctFromWindowsRestore(t *testing.T) {
	solid := accentForStyle(styleSolid)
	if !validStyle(styleSolid) || solid.state != 1 || solid.gradient != 0xff151515 {
		t.Fatalf("Solid must be opaque neutral charcoal: %+v", solid)
	}
	if accentForStyle(styleNormal).state != 0 {
		t.Fatal("exit must still restore the Windows accent policy")
	}
}

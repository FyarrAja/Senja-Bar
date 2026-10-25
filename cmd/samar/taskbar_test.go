//go:build windows

package main

import (
	"syscall"
	"testing"
)

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
	if glass.gradient != glassTintABGR || acrylic.gradient != acrylicTintABGR {
		t.Fatal("native fallback and XAML must use identical tint values")
	}
	if glass.gradient&0xffffff != 0 || acrylic.gradient&0xffffff != 0 {
		t.Fatal("materials must not impose a colored tint")
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

//go:build windows

package main

import (
	"testing"
	"unsafe"
)

func TestMenuWin64ABILayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("Win64-specific native layout")
	}
	if unsafe.Sizeof(measureMenuItem{}) != 32 || unsafe.Offsetof(measureMenuItem{}.data) != 24 {
		t.Fatal("MEASUREITEMSTRUCT ABI mismatch")
	}
	if unsafe.Sizeof(drawMenuItem{}) != 64 || unsafe.Offsetof(drawMenuItem{}.bounds) != 40 || unsafe.Offsetof(drawMenuItem{}.data) != 56 {
		t.Fatal("DRAWITEMSTRUCT ABI mismatch")
	}
	if unsafe.Sizeof(nativeMenuInfo{}) != 40 || unsafe.Offsetof(nativeMenuInfo{}.background) != 16 {
		t.Fatal("MENUINFO ABI mismatch")
	}
	if unsafe.Offsetof(menuRow{}.access) != 0 || unsafe.Sizeof(msaaMenuInfo{}) != 16 {
		t.Fatal("MSAA data must begin the native item data")
	}
}

func TestMenuReflectsLoadedConfiguration(t *testing.T) {
	for _, style := range []int{styleSolid, styleGlass, styleAcrylic, styleClear} {
		rows := makeMenuRows(&config{style: style, startup: true, rounded: false})
		activeStyles := 0
		for _, row := range rows {
			if row.kind == "style" && row.active {
				activeStyles++
			}
			if row.id == cmdStartup && !row.active {
				t.Fatal("startup status lost")
			}
			if row.id == cmdRounded && row.active {
				t.Fatal("rounding status changed")
			}
			if row.id == cmdCheckUpdates && !row.disabled {
				t.Fatal("update action must wait for an available release")
			}
		}
		if activeStyles != 1 {
			t.Fatalf("style %d has %d active menu choices", style, activeStyles)
		}
	}
}

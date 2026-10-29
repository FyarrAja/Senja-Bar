//go:build windows

package main

import (
	"samar/internal/mic"
	"testing"
)

func TestWidePanelHitTargetsMatchDrawing(t *testing.T) {
	old := menuDPI
	defer func() { menuDPI = old }()
	for _, dpi := range []int32{96, 120, 144, 192} {
		menuDPI = dpi
		n := nativeGlassPanel{w: menuPx(nativePanelWidth), micPage: true}
		n.mic.devices = []mic.Device{{Name: "Fixture microphone"}}
		n.layoutMicControls()
		n.projected = true
		for _, i := range []int{48, 50, 51, 55, 65, 70} {
			b := nativeControlBoxes[i]
			x := n.sidebarWidth() + int(n.projectX(float32(b[0])+float32(b[2])/2))
			y := b[1] + b[3]/2
			if got := n.controlAt(x, y); got != i {
				t.Fatalf("dpi %d control %d mapped to %d", dpi, i, got)
			}
		}
	}
}

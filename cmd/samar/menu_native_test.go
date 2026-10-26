//go:build windows

package main

import (
	"fmt"
	"math"
	"runtime"
	"testing"
	"unsafe"
)

// Check rendered alpha rather than path coordinates: a broken connection or
// clipped arc must not leave a gap anywhere around the panel's perimeter.
func TestNativeOutlineRasterIsContinuous(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var token uintptr
	input := struct {
		Version            uint32
		Debug              uintptr
		Background, Codecs int32
	}{Version: 1}
	if s := nativeG("GdiplusStartup", uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&input)), 0); s != 0 {
		t.Fatalf("GDI+ startup: %d", s)
	}
	defer nativeG("GdiplusShutdown", token)
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			w, h := int(360*scale), int(548*scale)
			pixels := make([]uint32, w*h)
			n := &nativeGlassPanel{}
			if s := nativeG("GdipCreateBitmapFromScan0", uintptr(w), uintptr(h), uintptr(w*4), 0x000e200b, uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&n.image))); s != 0 {
				t.Fatalf("bitmap: %d", s)
			}
			defer nativeG("GdipDisposeImage", n.image)
			nativeG("GdipGetImageGraphicsContext", n.image, uintptr(unsafe.Pointer(&n.graphics)))
			defer nativeG("GdipDeleteGraphics", n.graphics)
			nativeG("GdipSetSmoothingMode", n.graphics, 4)
			nativeG("GdipScaleWorldTransform", n.graphics, nativeF(scale), nativeF(scale), 0)
			n.roundFloat(1, 1, 357, 545, 27, 0x60ffffff, true, 1.25)
			nativeG("GdipFlush", n.graphics, 1)
			check := func(x, y float64) {
				px, py := int(math.Round(x*float64(scale))), int(math.Round(y*float64(scale)))
				var coverage uint32
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if xx, yy := px+dx, py+dy; xx >= 0 && xx < w && yy >= 0 && yy < h {
							coverage = max(coverage, pixels[yy*w+xx]>>24)
						}
					}
				}
				if coverage < 24 {
					t.Fatalf("outline gap at %.2f,%.2f: alpha=%d", x, y, coverage)
				}
			}
			for x := 28.; x <= 331; x++ {
				check(x, 1)
				check(x, 546)
			}
			for y := 28.; y <= 519; y++ {
				check(1, y)
				check(358, y)
			}
			for i, c := range [][2]float64{{28, 28}, {331, 28}, {331, 519}, {28, 519}} {
				for a := 0; a <= 90; a++ {
					theta := float64(180+i*90+a) * math.Pi / 180
					check(c[0]+27*math.Cos(theta), c[1]+27*math.Sin(theta))
				}
			}
			if pixels[0] != 0 || pixels[w-1] != 0 || pixels[(h-1)*w] != 0 || pixels[w*h-1] != 0 {
				t.Fatal("outer corners must remain transparent")
			}
			// Exercise the real GDI+ transform at each DPI: shrink around the
			// button center, never around the window origin (which causes jitter).
			nativeG("GdipGraphicsClear", n.graphics, 0)
			n.presses[6].value = 1
			saved := n.beginPress(6)
			n.round(20, 430, 154, 48, 17, 0xffffffff, false)
			n.endPress(saved)
			nativeG("GdipFlush", n.graphics, 1)
			minX, minY, maxX, maxY := w, h, 0, 0
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if pixels[y*w+x]>>24 > 127 {
						minX = min(minX, x)
						minY = min(minY, y)
						maxX = max(maxX, x)
						maxY = max(maxY, y)
					}
				}
			}
			if minX <= int(20*scale) || maxX >= int(174*scale) || math.Abs(float64(minX+maxX+1)/2-97*float64(scale)) > 1.1 || math.Abs(float64(minY+maxY+1)/2-454*float64(scale)) > 1.1 {
				t.Fatalf("pressed button shifted instead of shrinking around center: %d,%d .. %d,%d", minX, minY, maxX, maxY)
			}
			runtime.KeepAlive(pixels)
		})
	}
}

func TestNativeMenuHitTargetsExcludeCardGaps(t *testing.T) {
	for _, p := range [][2]int{{180, 150}, {80, 190}, {180, 190}, {180, 450}, {113, 500}, {246, 500}, {0, 0}} {
		if c := nativeCommandAt(p[0], p[1]); c != 0 {
			t.Fatalf("gap %v invoked %d", p, c)
		}
	}
	for _, p := range []struct {
		x, y int
		c    uint32
	}{{80, 150, 100}, {240, 150, 101}, {80, 250, 102}, {240, 250, 103}, {300, 315, 105}, {300, 355, 110}, {300, 395, 114}, {80, 450, 111}, {240, 450, 112}, {180, 500, 140}} {
		if c := nativeCommandAt(p.x, p.y); c != p.c {
			t.Fatalf("control at %d,%d: %d != %d", p.x, p.y, c, p.c)
		}
	}
}
func TestNativeStateQueueHonorsRapidReversalAfterAcknowledgement(t *testing.T) {
	actual := cssState{Style: 1, Rounded: true, Startup: true}
	desired := actual
	desired.Rounded = false
	if nativeStateCommand(actual, desired) != cmdRounded {
		t.Fatal("first toggle missing")
	}
	// User reverses while the first engine command is in flight.
	desired.Rounded = true
	actual.Rounded = false
	if nativeStateCommand(actual, desired) != cmdRounded {
		t.Fatal("reversal was lost after acknowledgement")
	}
	actual.Rounded = true
	if nativeStateCommand(actual, desired) != 0 {
		t.Fatal("settled state must not keep toggling")
	}
	desired.Style = 4
	desired.Startup = false
	if nativeStateCommand(actual, desired) != cmdStyleSolid {
		t.Fatal("latest mode not prioritized")
	}
	actual.Style = 4
	if nativeStateCommand(actual, desired) != cmdStartup {
		t.Fatal("queued startup change lost")
	}
}

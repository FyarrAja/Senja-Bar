//go:build windows

package main

import (
	"math"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

func TestCubeLoaderPatternAndMotion(t *testing.T) {
	active := 0
	for row := 0; row < 5; row++ {
		for col := 0; col < 5; col++ {
			if cubeLoaderMatrix[row][col] == '1' {
				active++
			}
			a, b := cubeDotFrame(2*time.Second, row, col)
			c, d := cubeDotFrame(2*time.Second+cubeLoaderDuration, row, col)
			if a != c || b != d {
				t.Fatal("dot cycle jumps")
			}
			for frame := 0; frame < 64; frame++ {
				opacity, scale := cubeDotFrame(time.Duration(frame)*cubeLoaderDuration/32, row, col)
				if math.IsNaN(opacity) || opacity < .08 || opacity > 1 || scale < .75 || scale > 1.2 {
					t.Fatal("invalid dot frame", opacity, scale)
				}
			}
		}
	}
	if active != 14 {
		t.Fatal("reference cube pattern changed")
	}
}
func TestCubeStopsWhenListIsNotLoading(t *testing.T) {
	n := nativeGlassPanel{resourcePage: true}
	n.resources.loading = true
	if !n.resourceLoaderVisible() {
		t.Fatal("loading list has no animation")
	}
	n.resources.loading = false
	if n.resourceLoaderVisible() {
		t.Fatal("completed scan keeps animation alive")
	}
	n.resources.loading = true
	n.resources.config.AllBackground = true
	if n.resourceLoaderVisible() {
		t.Fatal("hidden list animates in all-background scope")
	}
	n.resources.config.AllBackground = false
	n.resourcePage = false
	if n.resourceLoaderVisible() {
		t.Fatal("other page keeps animation alive")
	}
	n.resourcePage = true
	n.closing = true
	if n.resourceLoaderVisible() {
		t.Fatal("closing panel keeps animation alive")
	}
}

func TestCubeDotsActuallyRender(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var token uintptr
	input := struct {
		Version            uint32
		Debug              uintptr
		Background, Codecs int32
	}{Version: 1}
	if nativeG("GdiplusStartup", uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&input)), 0) != 0 {
		t.Fatal("GDI+ startup")
	}
	defer nativeG("GdiplusShutdown", token)
	pixels := make([]uint32, 360*330)
	n := nativeGlassPanel{}
	if nativeG("GdipCreateBitmapFromScan0", 360, 330, 360*4, 0xe200b, uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&n.image))) != 0 {
		t.Fatal("cube canvas")
	}
	defer nativeG("GdipDisposeImage", n.image)
	nativeG("GdipGetImageGraphicsContext", n.image, uintptr(unsafe.Pointer(&n.graphics)))
	defer nativeG("GdipDeleteGraphics", n.graphics)
	n.resources.loadingStarted = time.Now()
	n.drawResourceCubeLoader(0xfff6f7fa)
	nativeG("GdipFlush", n.graphics, 1)
	count := 0
	for _, p := range pixels {
		if p>>24 > 0 {
			count++
		}
	}
	if count < 300 || pixels[231*360+179] == 0 || pixels[0] != 0 {
		t.Fatalf("cube dots missing: %d pixels", count)
	}
	runtime.KeepAlive(pixels)
}

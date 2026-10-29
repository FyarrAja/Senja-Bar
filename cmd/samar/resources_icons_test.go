//go:build windows

package main

import (
	"os"
	"runtime"
	"testing"
	"unsafe"
)

func TestResourceIconsCacheAndDispose(t *testing.T) {
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
	testOwnedIconStorage(t)
	testIconCoverage(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := nativeResources{}
	for i := 0; i < 8; i++ {
		// Exercise the real shell icon, cached conversion, and replaced rows.
		icon := resourceFileIcon(exe)
		if icon == 0 {
			t.Fatal("shell icon unavailable")
		}
		r.acceptRows([]resourceRow{{path: exe, icon: icon}})
		if len(r.icons) != 1 {
			t.Fatal("icon cache not bounded to unique paths")
		}
		for _, image := range r.icons {
			var w, h uint32
			if nativeG("GdipGetImageWidth", image, uintptr(unsafe.Pointer(&w))) != 0 || nativeG("GdipGetImageHeight", image, uintptr(unsafe.Pointer(&h))) != 0 || w == 0 || h == 0 {
				t.Fatal("invalid cached icon")
			}
		}
	}
	r.acceptRows(nil)
	if len(r.icons) != 0 {
		t.Fatal("removed app retained its bitmap")
	}
	fallback := resourceFileIcon(`\\unreachable\app.exe`)
	if fallback == 0 {
		t.Fatal("generic fallback unavailable")
	}
	releaseResourceRows([]resourceRow{{icon: fallback}})
	r.dispose()
	if !r.closed || r.icons != nil {
		t.Fatal("panel cleanup incomplete")
	}
}

func testOwnedIconStorage(t *testing.T) {
	pixels := make([]uint32, 1024)
	for i := range pixels {
		pixels[i] = 0xff000000 | uint32(i)*7919&0xffffff
	}
	expected := append([]uint32(nil), pixels...)
	image := resourceOwnedBitmap(pixels)
	if image == 0 {
		t.Fatal("native icon allocation")
	}
	defer nativeG("GdipDisposeImage", image)
	// Deterministically overwrite the input, then force collection and heap churn.
	for i := range pixels {
		pixels[i] = 0xffff00ff
	}
	pixels = nil
	runtime.GC()
	churn := make([]byte, 1024*1024)
	for i := range churn {
		churn[i] = byte(i)
	}
	for i, want := range expected {
		var got uint32
		if nativeG("GdipBitmapGetPixel", image, uintptr(i%32), uintptr(i/32), uintptr(unsafe.Pointer(&got))) != 0 || got != want {
			t.Fatalf("icon retained temporary memory at pixel %d: %08x != %08x", i, got, want)
		}
	}
	runtime.KeepAlive(churn)
}

func testIconCoverage(t *testing.T) {
	// A real Win32 alpha icon: transparent corners, opaque BLACK artwork,
	// and one partially transparent red pixel. Black-key removal would destroy
	// the center, while the old GDI+ conversion flattened the corners.
	header := struct {
		Size                   uint32
		W, H                   int32
		Planes, Bits           uint16
		Compression, ImageSize uint32
		X, Y                   int32
		Used, Important        uint32
	}{Size: 40, W: 32, H: -32, Planes: 1, Bits: 32}
	var bits uintptr
	color, _, _ := gdi32.NewProc("CreateDIBSection").Call(0, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 {
		t.Fatal("test DIB")
	}
	defer gdi32.NewProc("DeleteObject").Call(color)
	pixels := unsafe.Slice((*uint32)(unsafe.Pointer(bits)), 1024)
	pixels[16*32+16] = 0xff000000
	pixels[16*32+17] = 0x80ff0000
	mask, _, _ := gdi32.NewProc("CreateBitmap").Call(32, 32, 1, 1, 0)
	if mask == 0 {
		t.Fatal("test mask")
	}
	defer gdi32.NewProc("DeleteObject").Call(mask)
	info := struct {
		Icon        int32
		X, Y        uint32
		Mask, Color uintptr
	}{Icon: 1, Mask: mask, Color: color}
	icon, _, _ := user32.NewProc("CreateIconIndirect").Call(uintptr(unsafe.Pointer(&info)))
	if icon == 0 {
		t.Fatal("test icon")
	}
	defer user32.NewProc("DestroyIcon").Call(icon)
	bitmap := resourceIconBitmap(icon)
	if bitmap == 0 {
		t.Fatal("alpha conversion")
	}
	defer nativeG("GdipDisposeImage", bitmap)
	pixel := func(x, y int) uint32 {
		var value uint32
		if nativeG("GdipBitmapGetPixel", bitmap, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&value))) != 0 {
			t.Fatal("get pixel")
		}
		return value
	}
	if p := pixel(0, 0); p != 0 {
		t.Fatalf("transparent area acquired background: %08x", p)
	}
	if p := pixel(16, 16); p != 0xff000000 {
		t.Fatalf("real black artwork removed: %08x", p)
	}
	if p := pixel(17, 16); p>>24 < 126 || p>>24 > 129 {
		t.Fatalf("antialiased alpha lost: %08x", p)
	}
	target := make([]uint32, 1024)
	for i := range target {
		target[i] = 0xffffffff
	}
	var canvas, graphics uintptr
	nativeG("GdipCreateBitmapFromScan0", 32, 32, 128, 0xe200b, uintptr(unsafe.Pointer(&target[0])), uintptr(unsafe.Pointer(&canvas)))
	defer nativeG("GdipDisposeImage", canvas)
	nativeG("GdipGetImageGraphicsContext", canvas, uintptr(unsafe.Pointer(&graphics)))
	defer nativeG("GdipDeleteGraphics", graphics)
	nativeG("GdipDrawImageRectI", graphics, bitmap, 0, 0, 32, 32)
	nativeG("GdipFlush", graphics, 1)
	if p := target[16*32+17]; p>>16&255 < 250 || p>>8&255 < 125 || p>>8&255 > 130 {
		t.Fatalf("incorrect alpha composite: %08x", p)
	}
}

func TestResourcePageHitTargetsAndFocus(t *testing.T) {
	n := nativeGlassPanel{resourcePage: true}
	n.resources.rows = []resourceRow{{path: "notes.exe"}}
	if n.controlAt(169, 231) != 12 || n.controlAt(55, 173) != 11 {
		t.Fatal("sidebar/list hit mapping")
	}
	if n.controlEnabled(0) || n.controlEnabled(13) || n.controlEnabled(22) {
		t.Fatal("hidden or empty controls active")
	}
	n.resources.loading = true
	if n.controlEnabled(12) || !n.controlEnabled(10) {
		t.Fatal("loading must disable rows, retain navigation")
	}
}

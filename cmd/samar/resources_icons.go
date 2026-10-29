//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"unsafe"
)

// GdipCreateBitmapFromHICON can flatten a shell icon onto black. Composite on
// two opaque backgrounds instead, recover coverage, and retain premultiplied
// RGB. This handles both alpha icons and older AND-mask icons without erasing
// actual black details in the artwork.
func resourceIconPixel(black, white uint32) uint32 {
	difference := 0
	for shift := uint(0); shift < 24; shift += 8 {
		difference = max(difference, int(white>>shift&255)-int(black>>shift&255))
	}
	alpha := uint32(255 - min(255, max(0, difference)))
	result := alpha << 24
	for shift := uint(0); shift < 24; shift += 8 {
		result |= min(alpha, black>>shift&255) << shift
	}
	return result
}
func resourceIconOnBackground(icon uintptr, background uint32) ([]uint32, bool) {
	dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(0)
	if dc == 0 {
		return nil, false
	}
	defer gdi32.NewProc("DeleteDC").Call(dc)
	header := struct {
		Size                   uint32
		W, H                   int32
		Planes, Bits           uint16
		Compression, ImageSize uint32
		X, Y                   int32
		Used, Important        uint32
	}{Size: 40, W: 32, H: -32, Planes: 1, Bits: 32}
	var pixels uintptr
	bitmap, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if bitmap == 0 {
		return nil, false
	}
	defer gdi32.NewProc("DeleteObject").Call(bitmap)
	old, _, _ := gdi32.NewProc("SelectObject").Call(dc, bitmap)
	defer gdi32.NewProc("SelectObject").Call(dc, old)
	values := unsafe.Slice((*uint32)(unsafe.Pointer(pixels)), 32*32)
	for i := range values {
		values[i] = background
	}
	ok, _, _ := user32.NewProc("DrawIconEx").Call(dc, 0, 0, icon, 32, 32, 0, 0, 3)
	gdi32.NewProc("GdiFlush").Call()
	if ok == 0 {
		return nil, false
	}
	return append([]uint32(nil), values...), true
}
func resourceIconBitmap(icon uintptr) uintptr {
	black, ok := resourceIconOnBackground(icon, 0xff000000)
	if !ok {
		return 0
	}
	white, ok := resourceIconOnBackground(icon, 0xffffffff)
	if !ok {
		return 0
	}
	for i := range black {
		p := resourceIconPixel(black[i], white[i])
		a := p >> 24
		// Convert reconstructed premultiplied RGB to explicit straight ARGB
		// for the independently owned GDI+ cache bitmap.
		pixel := a << 24
		if a != 0 {
			for shift := uint(0); shift < 24; shift += 8 {
				pixel |= min(uint32(255), ((p>>shift&255)*255+a/2)/a) << shift
			}
		}
		black[i] = pixel
	}
	return resourceOwnedBitmap(black)
}

// Allocate native-owned storage and explicitly copy pixels into it. A cloned
// Scan0 bitmap may share its caller's Go buffer, which becomes garbage after GC.
func resourceOwnedBitmap(pixels []uint32) uintptr {
	if len(pixels) != 1024 {
		return 0
	}
	var image uintptr
	if nativeG("GdipCreateBitmapFromScan0", 32, 32, 0, 0x26200a, 0, uintptr(unsafe.Pointer(&image))) != 0 {
		return 0
	}
	bounds := struct{ X, Y, W, H int32 }{W: 32, H: 32}
	data := struct {
		W, H           uint32
		Stride         int32
		Format         uint32
		Scan, Reserved uintptr
	}{}
	if nativeG("GdipBitmapLockBits", image, uintptr(unsafe.Pointer(&bounds)), 2, 0x26200a, uintptr(unsafe.Pointer(&data))) != 0 {
		nativeG("GdipDisposeImage", image)
		return 0
	}
	for y := 0; y < 32; y++ {
		row := unsafe.Slice((*uint32)(unsafe.Pointer(uintptr(int64(data.Scan)+int64(y)*int64(data.Stride)))), 32)
		copy(row, pixels[y*32:(y+1)*32])
	}
	if nativeG("GdipBitmapUnlockBits", image, uintptr(unsafe.Pointer(&data))) != 0 {
		nativeG("GdipDisposeImage", image)
		return 0
	}
	return image
}

// Shell extraction runs in the list worker. HICONs transfer to the UI thread,
// where GDI+ images are cached for this panel's lifetime only.
func resourceFileIcon(path string) uintptr {
	info := struct {
		Icon       uintptr
		Index      int32
		Attributes uint32
		Display    [260]uint16
		Type       [80]uint16
	}{}
	query := func(name string, flags uintptr) uintptr {
		shell32.NewProc("SHGetFileInfoW").Call(uintptr(unsafe.Pointer(utf16ptr(name))), 0x80, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info), flags)
		return info.Icon
	}
	if filepath.IsAbs(path) && !strings.HasPrefix(path, `\\`) {
		if icon := query(path, 0x100); icon != 0 {
			return icon
		}
	}
	return query("application.exe", 0x100|0x10)
}
func releaseResourceRows(rows []resourceRow) {
	for _, r := range rows {
		if r.icon != 0 {
			user32.NewProc("DestroyIcon").Call(r.icon)
		}
	}
}
func (r *nativeResources) acceptRows(rows []resourceRow) {
	if r.icons == nil {
		r.icons = map[string]uintptr{}
	}
	present := map[string]bool{}
	for i := range rows {
		row := &rows[i]
		key := strings.ToLower(row.path)
		present[key] = true
		if row.icon != 0 {
			if _, cached := r.icons[key]; !cached {
				if bitmap := resourceIconBitmap(row.icon); bitmap != 0 {
					r.icons[key] = bitmap
				}
			}
			user32.NewProc("DestroyIcon").Call(row.icon)
			row.icon = 0
		}
	}
	for key, bitmap := range r.icons {
		if !present[key] {
			nativeG("GdipDisposeImage", bitmap)
			delete(r.icons, key)
		}
	}
	r.rows = rows
	r.refilter()
}
func (r *nativeResources) dispose() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.results != nil {
		select {
		case result := <-r.results:
			releaseResourceRows(result.rows)
		default:
		}
	}
	for _, bitmap := range r.icons {
		nativeG("GdipDisposeImage", bitmap)
	}
	r.icons = nil
}
func (n *nativeGlassPanel) drawResourceIcon(path string, x, y int, ink uint32) {
	if bitmap := n.resources.icons[strings.ToLower(path)]; bitmap != 0 {
		nativeG("GdipDrawImageRectI", n.graphics, bitmap, uintptr(n.projectX(float32(x))), uintptr(y), 22, 22)
	} else {
		// A consistent application glyph covers icons that the shell cannot read.
		n.round(x+1, y+2, 20, 18, 4, 0x226f8bad, false)
		n.round(x+1, y+2, 20, 18, 4, ink, true)
		n.line(x+2, y+7, x+20, y+7, ink, 1)
	}
}

//go:build windows

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"unsafe"
)

// Offscreen QA uses the exact GDI painter used by WM_DRAWITEM, not a web mockup.
func renderThemedMenuFile(filename string) error {
	rows := makeMenuRows(&config{style: styleClear, startup: true})
	menuDPI = 96
	var height int32
	for _, row := range rows {
		height += menuRowHeight(row)
	}
	width := int32(288)
	dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(0)
	if dc == 0 {
		return fmt.Errorf("cannot create preview DC")
	}
	defer gdi32.NewProc("DeleteDC").Call(dc)
	header := struct {
		size                   uint32
		width, height          int32
		planes, bits           uint16
		compression, imageSize uint32
		xppm, yppm             int32
		used, important        uint32
	}{size: 40, width: width, height: -height, planes: 1, bits: 32}
	var pixels uintptr
	bitmap, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if bitmap == 0 || pixels == 0 {
		return fmt.Errorf("cannot allocate preview bitmap")
	}
	defer deleteObject.Call(bitmap)
	old, _, _ := menuSelect.Call(dc, bitmap)
	defer menuSelect.Call(dc, old)
	menuBrandIcon, _, _ = loadImageW.Call(0, uintptr(unsafe.Pointer(utf16ptr(iconFilePath()))), imageIcon, 32, 32, lrLoadFromFile)
	defer func() {
		if menuBrandIcon != 0 {
			user32.NewProc("DestroyIcon").Call(menuBrandIcon)
		}
		menuBrandIcon = 0
	}()
	var y int32
	for _, row := range rows {
		h := menuRowHeight(row)
		paintThemedMenuRow(dc, row, rect{0, y, width, y + h}, false)
		y += h
	}
	gdi32.NewProc("GdiFlush").Call()
	raw := unsafe.Slice((*byte)(unsafe.Pointer(pixels)), int(width*height*4))
	output := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	for i := 0; i < len(raw); i += 4 {
		output.Pix[i], output.Pix[i+1], output.Pix[i+2], output.Pix[i+3] = raw[i+2], raw[i+1], raw[i], 255
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, output)
}

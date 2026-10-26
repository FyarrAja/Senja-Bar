//go:build windows

package main

import (
	"math"
	"sort"
	"syscall"
	"unsafe"
)

type backdropRGB struct{ r, g, b float64 }
type taskbarPalette struct{ glass, acrylic, solid uint32 }

var adaptivePalettes = make(map[syscall.Handle]taskbarPalette)

func (c backdropRGB) luma() float64 { return .2126*c.r + .7152*c.g + .0722*c.b }
func (c backdropRGB) relativeLuminance() float64 {
	linear := func(v float64) float64 {
		v /= 255
		if v <= .04045 {
			return v / 12.92
		}
		return math.Pow((v+.055)/1.055, 2.4)
	}
	return .2126*linear(c.r) + .7152*linear(c.g) + .0722*linear(c.b)
}
func mixBackdrop(a, b backdropRGB, t float64) backdropRGB {
	return backdropRGB{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}
func colorABGR(c backdropRGB, alpha float64) uint32 {
	channel := func(v float64) uint32 { return uint32(math.Round(max(0., min(255., v)))) }
	return channel(alpha*255)<<24 | channel(c.b)<<16 | channel(c.g)<<8 | channel(c.r)
}

// System theme controls icon/text contrast; backdrop chroma supplies the hue.
// Never change the user's Windows theme to match the utility.
func taskbarColors(light bool, backdrop backdropRGB) taskbarPalette {
	base, lo, hi, weight := 21., 8., 70., .25
	if light {
		base, lo, hi, weight = 244, 218, 252, .20
	}
	lum := backdrop.luma()
	tint := backdropRGB{max(lo, min(hi, base+(backdrop.r-lum)*weight)), max(lo, min(hi, base+(backdrop.g-lum)*weight)), max(lo, min(hi, base+(backdrop.b-lum)*weight))}
	alpha := func(baseline, target float64) float64 {
		readable := func(a float64) bool {
			l := mixBackdrop(backdrop, tint, a).relativeLuminance()
			if light {
				return l >= target
			}
			return l <= target
		}
		if readable(baseline) {
			return baseline
		}
		lo, hi := baseline, .85
		for i := 0; i < 12; i++ {
			mid := (lo + hi) / 2
			if readable(mid) {
				hi = mid
			} else {
				lo = mid
			}
		}
		return hi
	}
	glassTarget, acrylicTarget := .16, .11
	if light {
		glassTarget, acrylicTarget = .32, .42
	}
	return taskbarPalette{colorABGR(tint, alpha(20./255, glassTarget)), colorABGR(tint, alpha(64./255, acrylicTarget)), colorABGR(tint, 1)}
}

func systemTaskbarLight() bool {
	var value uint32
	size := uint32(4)
	r, _, _ := syscall.NewLazyDLL("advapi32.dll").NewProc("RegGetValueW").Call(hkeyCurrentUser, uintptr(unsafe.Pointer(utf16ptr(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`))), uintptr(unsafe.Pointer(utf16ptr("SystemUsesLightTheme"))), 0x10, 0, uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&size)))
	return r == 0 && value != 0
}

func paletteForTaskbar(hwnd syscall.Handle) taskbarPalette {
	light := systemTaskbarLight()
	background := backdropRGB{21, 21, 21}
	if light {
		background = backdropRGB{244, 244, 244}
	}
	var bounds rect
	ok, _, _ := getWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&bounds)))
	if ok == 0 {
		return taskbarColors(light, background)
	}
	monitor, _, _ := user32.NewProc("MonitorFromWindow").Call(uintptr(hwnd), 2)
	mi := cssMonitorInfo{Size: uint32(unsafe.Sizeof(cssMonitorInfo{}))}
	user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&mi)))
	var menu rect
	popup, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr(cssMenuClass))), 0)
	if popup != 0 {
		getWindowRect.Call(popup, uintptr(unsafe.Pointer(&menu)))
	}
	dc, _, _ := user32.NewProc("GetDC").Call(0)
	if dc == 0 {
		return taskbarColors(light, background)
	}
	defer user32.NewProc("ReleaseDC").Call(0, dc)
	// Sparse samples immediately outside the bar avoid sampling our own tint.
	// No screen image or wallpaper decoding is needed, and no pixels persist.
	samples := make([]backdropRGB, 0, 9)
	for i := 0; i < 9; i++ {
		var p point
		if bounds.right-bounds.left >= bounds.bottom-bounds.top {
			p = point{bounds.left + (bounds.right-bounds.left)*int32(i+1)/10, bounds.top - 3}
			if p.y < mi.Monitor.top {
				p.y = bounds.bottom + 3
			}
		} else {
			p = point{bounds.left - 3, bounds.top + (bounds.bottom-bounds.top)*int32(i+1)/10}
			if p.x < mi.Monitor.left {
				p.x = bounds.right + 3
			}
		}
		if p.x >= menu.left && p.x < menu.right && p.y >= menu.top && p.y < menu.bottom {
			continue
		}
		pixel, _, _ := gdi32.NewProc("GetPixel").Call(dc, uintptr(p.x), uintptr(p.y))
		if uint32(pixel) == 0xffffffff {
			continue
		}
		c := uint32(pixel)
		samples = append(samples, backdropRGB{float64(c & 255), float64(c >> 8 & 255), float64(c >> 16 & 255)})
	}
	if len(samples) >= 3 {
		sort.Slice(samples, func(i, j int) bool { return samples[i].luma() < samples[j].luma() })
		background = backdropRGB{}
		// Trim extreme samples (icons, highlights, isolated wallpaper stars).
		for _, c := range samples[1 : len(samples)-1] {
			background.r += c.r
			background.g += c.g
			background.b += c.b
		}
		n := float64(len(samples) - 2)
		background.r /= n
		background.g /= n
		background.b /= n
	}
	return taskbarColors(light, background)
}

func watchTaskbarBackdrop(hwnd uintptr) func() {
	callback := syscall.NewCallback(func(_ uintptr, event uint32, window uintptr, _ int32, _ int32, _ uint32, _ uint32) uintptr {
		if event == 3 || event == 0xb || event == 0x16 || event == 0x17 || event == 0x20 {
			class := getWindowClassName(syscall.Handle(window))
			if class != cssMenuClass && class != "SenjaNativeGlassPreview" {
				postMessageW.Call(hwnd, wmApp+64, 0, 0)
			}
		}
		return 0
	})
	hook, _, _ := user32.NewProc("SetWinEventHook").Call(3, 0x20, 0, callback, 0, 0, 2) // OUTOFCONTEXT | SKIPOWNPROCESS
	return func() {
		if hook != 0 {
			user32.NewProc("UnhookWinEvent").Call(hook)
		}
	}
}

func refreshTaskbarBackdrop() {
	if globalConfig == nil || shuttingDown || globalConfig.style == styleNormal || globalConfig.style == styleClear {
		return
	}
	for _, hwnd := range taskbarWindows() {
		palette := paletteForTaskbar(hwnd)
		if adaptivePalettes[hwnd] != palette {
			setAccent(hwnd, globalConfig.style, globalConfig.rounded)
		}
	}
}

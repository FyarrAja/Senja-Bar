//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func iconPath() string {
	e, err := os.Executable()
	if err != nil {
		return filepath.Join("Assets", "icon.png")
	}
	return filepath.Join(filepath.Dir(e), "Assets", "icon.png")
}

func iconFilePath() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "senja.ico")
}

func loadPNGIcon(path string) (syscall.Handle, error) {
	var token uintptr
	input := gdiplusStartupInput{gdiplusVersion: 1}
	status, _, callErr := gdiplusStartup.Call(uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&input)), 0)
	if status != 0 {
		if callErr != nil {
			return 0, callErr
		}
		return 0, fmt.Errorf("GDI+ startup failed: status %d", status)
	}
	defer gdiplusShutdown.Call(token)
	var bitmap uintptr
	utfPath := utf16ptr(path)
	status, _, callErr = gdipCreateBitmapFromFile.Call(uintptr(unsafe.Pointer(utfPath)), uintptr(unsafe.Pointer(&bitmap)))
	if status != 0 || bitmap == 0 {
		if callErr != nil {
			return 0, callErr
		}
		return 0, fmt.Errorf("could not load icon: %s", path)
	}
	defer gdipDisposeImage.Call(bitmap)
	var hicon uintptr
	status, _, callErr = gdipCreateHICONFromBitmap.Call(bitmap, uintptr(unsafe.Pointer(&hicon)))
	if status != 0 || hicon == 0 {
		if callErr != nil {
			return 0, callErr
		}
		return 0, fmt.Errorf("could not create icon from PNG: %s", path)
	}
	return syscall.Handle(hicon), nil
}

func loadTrayIcon() error {
	cx, _, _ := getSystemMetrics.Call(smCxSmIcon)
	cy, _, _ := getSystemMetrics.Call(smCySmIcon)
	// Ask the taskbar's monitor, rather than the hidden host window at (0,0).
	if getDpiForWindow.Find() == nil && getSystemMetricsForDpi.Find() == nil {
		className := utf16ptr("Shell_TrayWnd")
		taskbar, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
		if taskbar != 0 {
			dpi, _, _ := getDpiForWindow.Call(taskbar)
			if dpi != 0 {
				cx, _, _ = getSystemMetricsForDpi.Call(smCxSmIcon, dpi)
				cy, _, _ = getSystemMetricsForDpi.Call(smCySmIcon, dpi)
			}
		}
	}
	if cx == 0 {
		cx = 16
	}
	if cy == 0 {
		cy = 16
	}

	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	icoPaths := []string{
		filepath.Join(dir, "senja.ico"),
		filepath.Join(dir, "Assets", "senja.ico"),
	}

	for _, p := range icoPaths {
		if _, err := os.Stat(p); err == nil {
			utfPath := utf16ptr(p)
			h, _, _ := loadImageW.Call(
				0,
				uintptr(unsafe.Pointer(utfPath)),
				imageIcon,
				cx,
				cy,
				lrLoadFromFile|lrDefaultColor,
			)
			if h != 0 {
				trayIcon = syscall.Handle(h)
				customTrayIcon = true
				logEvent("INFO", "icon.load", fmt.Sprintf("native ICO %dx%d path=%s", cx, cy, p))
				return nil
			}
		}
	}

	hMod, _, _ := getModuleHandleW.Call(0)
	if hMod != 0 {
		h, _, _ := loadImageW.Call(
			hMod,
			1,
			imageIcon,
			cx,
			cy,
			lrDefaultColor,
		)
		if h != 0 {
			trayIcon = syscall.Handle(h)
			customTrayIcon = true
			return nil
		}
	}

	path := iconPath()
	icon, err := loadPNGIcon(path)
	if err == nil {
		trayIcon = icon
		customTrayIcon = true
		return nil
	}
	logLine("custom icon unavailable: " + err.Error())
	fallback, _, fallbackErr := loadIconW.Call(0, iconApplication)
	if fallback == 0 {
		if fallbackErr != nil {
			return fallbackErr
		}
		return err
	}
	trayIcon = syscall.Handle(fallback)
	customTrayIcon = false
	return nil
}

func destroyTrayIconHandle() {
	if trayIcon == 0 || !customTrayIcon {
		trayIcon = 0
		customTrayIcon = false
		return
	}
	user32.NewProc("DestroyIcon").Call(uintptr(trayIcon))
	trayIcon = 0
	customTrayIcon = false
}

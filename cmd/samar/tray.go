//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

func installTrayIcon(hwnd syscall.Handle) (notifyIconData, error) {
	var data notifyIconData
	data.cbSize = uint32(unsafe.Sizeof(data))
	data.hWnd = hwnd
	data.uID = 1
	data.uFlags = nifMessage | nifIcon | nifTip
	data.uCallbackMessage = wmTray
	data.hIcon = trayIcon
	utf16from(appName, data.szTip[:])

	// Clean any dangling icon from a previously killed instance
	shellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&data)))

	ret, _, err := shellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&data)))
	if ret == 0 {
		logWin32Error("tray.install", "Shell_NotifyIconW(NIM_ADD)", err)
		// Don't crash the whole app if tray icon is busy, continue so taskbar styling works
		return data, nil
	}
	data.uTimeoutOrVersion = notifyIconVersion4
	shellNotifyIconW.Call(nimSetVersion, uintptr(unsafe.Pointer(&data)))
	return data, nil
}

func removeTrayIcon(data *notifyIconData) {
	if data == nil || data.hWnd == 0 {
		return
	}
	shellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(data)))
}

func notifyUpdateAvailable(version string) bool {
	if trayData == nil || trayData.hWnd == 0 {
		return false
	}
	data := *trayData
	data.uFlags = 0x10 // NIF_INFO
	data.uTimeoutOrVersion = 10000
	data.dwInfoFlags = 1 | 0x80 // NIIF_INFO | NIIF_RESPECT_QUIET_TIME
	utf16from("Senja — Update tersedia", data.szInfoTitle[:])
	message := "Versi " + version + " tersedia. Buka Senja untuk mengunduh pembaruan."
	if globalConfig != nil && globalConfig.autoInstall {
		message = "Versi " + version + " tersedia. Senja akan mengunduh dan menerapkannya secara otomatis."
	}
	utf16from(message, data.szInfo[:])
	ok, _, _ := shellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&data)))
	return ok != 0
}

func reinstallTrayIcon(hwnd syscall.Handle) {
	if trayData != nil {
		removeTrayIcon(trayData)
	}
	// Explorer and display-scale changes require a fresh native-size HICON.
	oldIcon, oldCustom := trayIcon, customTrayIcon
	if err := loadTrayIcon(); err != nil {
		trayIcon, customTrayIcon = oldIcon, oldCustom
		logError("icon.reload", "keeping previous tray icon", err)
	} else if oldIcon != 0 && oldIcon != trayIcon && oldCustom {
		user32.NewProc("DestroyIcon").Call(uintptr(oldIcon))
	}
	data, err := installTrayIcon(hwnd)
	if err != nil {
		logError("tray.reinstall", "unable to reinstall tray icon", err)
		trayData = nil
		return
	}
	trayData = &data
	logEvent("INFO", "tray.reinstall", "tray icon reinstalled at current taskbar DPI")
}

func appendMenu(menu, flags, id uintptr, text string) bool {
	ret, _, err := appendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(utf16ptr(text))))
	if ret == 0 {
		logWin32Error("tray.menu", fmt.Sprintf("AppendMenuW menu=%d id=%d text=%q", menu, id, text), err)
		return false
	}
	return true
}

func menuCheckFlag(active bool) uintptr {
	if active {
		return mfChecked
	}
	return 0
}

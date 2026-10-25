//go:build windows

package main

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

func showError(title string, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	logError("ui.error", title, err)
	messageBoxW.Call(0, uintptr(unsafe.Pointer(utf16ptr(err.Error()))), uintptr(unsafe.Pointer(utf16ptr(title))), messageBoxError)
}

func showInfo(owner syscall.Handle, title, text string) {
	messageBoxW.Call(uintptr(owner), uintptr(unsafe.Pointer(utf16ptr(text))), uintptr(unsafe.Pointer(utf16ptr(title))), messageBoxInfo)
}

func getModuleHandle() syscall.Handle {
	v, _, _ := getModuleHandleW.Call(0)
	return syscall.Handle(v)
}

func prepareProcess() {
	if ret, _, _ := setCurrentProcessAppID.Call(uintptr(unsafe.Pointer(utf16ptr(appUserModelID)))); ret != 0 {
		logEvent("WARN", "process", fmt.Sprintf("SetCurrentProcessExplicitAppUserModelID hr=0x%08X", uint32(ret)))
	}
	// Per-monitor V2 avoids bitmap-scaled menu text on high-DPI displays.
	if ret, _, _ := setProcessDpiAwarenessCtx.Call(^uintptr(3)); ret == 0 {
		setProcessDPIAware.Call()
	}
	setThreadDpiAwareness.Call(^uintptr(3))
	hDesk, _, _ := openDesktopW.Call(uintptr(unsafe.Pointer(utf16ptr("Default"))), 0, 0, 0x01FF)
	if hDesk != 0 {
		setThreadDesktop.Call(hDesk)
	}
}

func createHiddenWindow() (syscall.Handle, error) {
	instance := getModuleHandle()
	className := utf16ptr(windowClass)
	cursor, _, _ := loadCursorW.Call(0, cursorArrow)
	wc := wndClassEx{cbSize: uint32(unsafe.Sizeof(wndClassEx{})), style: csHRedraw | csVRedraw, lpfnWndProc: syscall.NewCallback(windowProc), hInstance: instance, hIcon: trayIcon, hCursor: syscall.Handle(cursor), lpszClassName: className, hIconSm: trayIcon}
	ret, _, callErr := registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 && callErr != nil && !errors.Is(callErr, syscall.Errno(errorClassAlreadyReg)) {
		return 0, callErr
	}
	hwnd, _, err := createWindowExW.Call(wsExToolWindow|wsExNoActivate, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16ptr(appName))), wsPopup, 0, 0, 0, 0, 0, 0, uintptr(instance), 0)
	if hwnd == 0 {
		if err == nil {
			err = syscall.EINVAL
		}
		return 0, err
	}
	return syscall.Handle(hwnd), nil
}

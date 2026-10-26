//go:build windows

package main

import (
	"samar/internal/updatestate"
	"syscall"
	"time"
	"unsafe"
)

func requestExit(hwnd syscall.Handle) {
	if shuttingDown {
		return
	}
	shuttingDown = true
	postMessageW.Call(uintptr(hwnd), wmClose, 0, 0)
}

func windowProc(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	defer recoverPanic("window procedure")
	switch message {
	case wmApp + 66:
		startBackgroundUpdateCheck()
		return 0
	case wmApp + 65:
		if !shuttingDown && lastStyleApplyOK {
			go cleanPreviousInstalls()
		}
		return 0
	case wmApp + 64:
		if !shuttingDown {
			user32.NewProc("SetTimer").Call(hwnd, 0x5355, 120, 0)
		}
		return 0
	case wmUpdatePhase:
		if updateRunning {
			setUpdatePhase(uint32(wParam))
		}
		return 0
	case wmUpdateProgress:
		if updateRunning && updatePhase == 2 {
			updateProgress = uint64(wParam)
			updateProgressStarted = time.Now().Add(-time.Duration(min(lParam, uintptr(24*time.Hour/time.Millisecond))) * time.Millisecond)
			menu, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr(cssMenuClass))), 0)
			if menu != 0 {
				postMessageW.Call(menu, wmUpdateProgress, wParam, lParam)
			}
		}
		return 0
	case wmUpdateElapsedQuery:
		if updateProgressStarted.IsZero() {
			return 0
		}
		return uintptr(time.Since(updateProgressStarted) / time.Millisecond)
	case wmUpdateProgressQuery:
		return uintptr(updateProgress)
	case wmUpdateQuery:
		return uintptr(updatestate.Encode(updatePhase, updateAvailable))
	case wmUpdateWorkerDone:
		updateRunning = false
		if wParam != 0 {
			setUpdatePhase(5)
			restorePendingUpdate()
		} else if updateBackground {
			restoreUpdateCache()
		}
		if wParam != 0 {
			nextUpdateCheck(30 * time.Minute)
		} else {
			nextUpdateCheck(6 * time.Hour)
		}
		updateBackground = false
		return 0
	case wmCSSCommand:
		// Posted, never sent: COM cannot call Explorer from an input-synchronous
		// SendMessage handler (RPC_E_CANTCALLOUT_ININPUTSYNCCALL).
		if globalConfig != nil && !shuttingDown && validCSSCommand(uint32(wParam)) {
			startupBefore := globalConfig.startup
			selectCommand(syscall.Handle(hwnd), uint32(wParam), globalConfig)
			status := uintptr(0)
			if (wParam >= cmdStyleGlass && wParam <= cmdStyleSolid || wParam == cmdRounded || wParam == cmdRefresh) && !lastStyleApplyOK {
				status = 1
			}
			if wParam == cmdStartup && globalConfig.startup == startupBefore {
				status = 1
			}
			postMessageW.Call(lParam, wmCSSResult, status, 0)
		}
		return 0
	case wmNativeMenu:
		if globalConfig != nil && !shuttingDown {
			showThemedTrayMenu(syscall.Handle(hwnd), globalConfig)
		}
		return 0
	case wmApp + 20:
		closeCSSMenu()
		if menuIsOpen {
			user32.NewProc("EndMenu").Call()
		}
		return 0
	case 0x002C: // WM_MEASUREITEM
		if measureThemedMenu(lParam) {
			return 1
		}
	case 0x002B: // WM_DRAWITEM
		if drawThemedMenu(lParam) {
			return 1
		}
	case wmTray:
		event := uint32(lParam & 0xFFFF)
		if event == 0x405 {
			existing, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr(cssMenuClass))), 0)
			if existing == 0 {
				postMessageW.Call(hwnd, wmShowMenu, 0, 0)
			}
			return 0
		} // Notification click opens settings, never installs.
		if event == wmRButtonUp || event == wmLButtonDbl {
			if globalConfig != nil && !shuttingDown && !menuPosted {
				menuPosted = true
				postMessageW.Call(hwnd, wmShowMenu, 0, 0)
			}
			return 0
		}
		return 0
	case wmShowMenu:
		menuPosted = false
		if globalConfig != nil && !shuttingDown {
			showTrayMenu(syscall.Handle(hwnd), globalConfig)
		}
		return 0
	case 0x007E, 0x001A, 0x02E0: // WM_DISPLAYCHANGE, WM_SETTINGCHANGE, WM_DPICHANGED
		if globalConfig != nil && !shuttingDown {
			reinstallTrayIcon(syscall.Handle(hwnd))
			user32.NewProc("SetTimer").Call(hwnd, 0x5354, 180, 0)
		}
		return 0
	case 0x031A, 0x031E, 0x0320: // Theme, DWM composition or colorization changed.
		if globalConfig != nil && !shuttingDown {
			user32.NewProc("SetTimer").Call(hwnd, 0x5354, 180, 0)
		}
		return 0
	case 0x0113:
		if wParam == 0x5550 {
			user32.NewProc("KillTimer").Call(hwnd, wParam)
			startBackgroundUpdateCheck()
			return 0
		}
		if wParam == 0x5355 {
			user32.NewProc("KillTimer").Call(hwnd, wParam)
			refreshTaskbarBackdrop()
			return 0
		}
		if wParam == 0x5354 {
			user32.NewProc("KillTimer").Call(hwnd, wParam)
			if globalConfig != nil && !shuttingDown {
				reapplyStyleSilently(globalConfig.style)
			}
			return 0
		}
	case wmCommand:
		if globalConfig != nil && !shuttingDown {
			selectCommand(syscall.Handle(hwnd), uint32(wParam&0xFFFF), globalConfig)
		}
		return 0
	case wmBeginUpdate:
		if !shuttingDown {
			shuttingDown = true
			postMessageW.Call(hwnd, wmClose, 0, 0)
		}
		return 0
	case wmUpdateResult:
		if message := takePendingUpdateMessage(); message != "" && !shuttingDown {
			showInfo(syscall.Handle(hwnd), "Senja Updates", message)
		}
		return 0
	case wmClose:
		closeCSSMenu()
		destroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}
	if taskbarMsg != 0 && message == taskbarMsg && globalConfig != nil && !shuttingDown {
		invalidateTAPSurfaces()
		reinstallTrayIcon(syscall.Handle(hwnd))
		applyStyle(globalConfig.style)
		return 0
	}
	result, _, _ := defWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

//go:build windows

package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"samar/internal/mic"
	"samar/internal/resources"
	"unsafe"
)

// Development builds retain the installed startup target and never self-update.
var developmentBuild string

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--mic-engine" {
		mic.RunWorker()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--native-menu" {
		if !runNativeGlassPanel(false) {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "--native-preview" || os.Args[1] == "--native-check" || os.Args[1] == "--native-control" || os.Args[1] == "--native-resource-preview" || os.Args[1] == "--native-loader-preview" || os.Args[1] == "--native-tuning-preview") {
		runNativeGlassPreview()
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "--css-menu" || os.Args[1] == "--css-preview" || os.Args[1] == "--css-check") {
		if err := runCSSMenu(os.Args[1] != "--css-menu"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--material-preview" {
		runGlassMaterialPreview()
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "--menu" || os.Args[1] == "--hide-menu") {
		className := utf16ptr(windowClass)
		hwnd, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(className)), 0)
		if hwnd != 0 {
			message := uintptr(wmShowMenu)
			if os.Args[1] == "--hide-menu" {
				message = wmApp + 20
			}
			postMessageW.Call(hwnd, message, 0, 0)
		}
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "--render-menu" {
		runtime.LockOSThread()
		if err := renderThemedMenuFile(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--shutdown" {
		requestExistingInstanceExit()
		return
	}

	// The engine is a single Win32 UI loop; extra Go scheduling workers do
	// not improve its responsiveness and retain additional runtime state.
	runtime.GOMAXPROCS(1)
	debug.SetGCPercent(20)
	debug.SetMemoryLimit(8 * 1024 * 1024)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	openLog()
	logContext()
	defer func() {
		if v := recover(); v != nil {
			logEvent("FATAL", "main", fmt.Sprintf("panic=%v | stack=\n%s", v, debug.Stack()))
		}
		cleanup()
	}()
	prepareProcess()
	processMutex = ensureSingleInstance()
	logEvent("INFO", "main", fmt.Sprintf("ensureSingleInstance mutex=%d", processMutex))
	if processMutex == 0 {
		return
	}
	cfg := loadConfig()
	startup := isStartupEnabled()
	if startup && developmentBuild != "true" {
		// A self-contained release can move the engine to a new versioned
		// folder. Preserve the enabled preference, updating only its path.
		if err := setStartup(true); err != nil {
			logError("startup.path", "cannot refresh startup path", err)
		}
	}
	if cfg.startup != startup {
		cfg.startup = startup
		_ = saveConfig(cfg)
	}
	globalConfig = &cfg
	taskbarRaw, _, _ := registerWindowMessageW.Call(uintptr(unsafe.Pointer(utf16ptr(taskbarMessage))))
	taskbarMsg = uint32(taskbarRaw)
	if err := loadTrayIcon(); err != nil {
		showError(appName, err)
		return
	}
	hwnd, err := createHiddenWindow()
	if err != nil {
		showError(appName, err)
		return
	}
	mainHwnd = hwnd
	resourceManager = resources.Start()
	defer mic.Disable()
	defer resourceManager.Close()
	stopResourceFocus := resourceManager.WatchForeground()
	defer stopResourceFocus()
	defer destroyWindow.Call(uintptr(hwnd))
	tray, err := installTrayIcon(hwnd)
	if err != nil {
		showError(appName, err)
		return
	}
	trayData = &tray
	applyStyle(cfg.style)
	stopBackdropWatch := watchTaskbarBackdrop(uintptr(hwnd))
	defer stopBackdropWatch()
	if err := saveConfig(cfg); err != nil {
		logError("config.save", "initial config save failed", err)
	}
	logEvent("INFO", "main", "entering messageLoop")
	// Process the readiness message only after the tray/material are ready.
	// Cleanup runs once off the window thread; failed startups keep old files.
	if lastStyleApplyOK && developmentBuild != "true" {
		postMessageW.Call(uintptr(hwnd), wmApp+65, 0, 0)
	}
	if developmentBuild != "true" {
		scheduleUpdateChecks(&cfg)
	}
	// Release transient startup allocations once. No periodic full-GC worker:
	// an idle engine should remain asleep until Windows sends a message.
	debug.FreeOSMemory()
	// Discard cold startup pages from the resident working set once. This
	// does not lower committed memory; Windows pages code back in on demand.
	// Never repeat on a timer or while the user is interacting with the menu.
	kernel32.NewProc("SetProcessWorkingSetSize").Call(^uintptr(0), ^uintptr(0), ^uintptr(0))
	ret := messageLoop()
	logEvent("INFO", "main", fmt.Sprintf("exited messageLoop ret=%d", ret))
}

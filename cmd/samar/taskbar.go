//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

// Neutral tint lets the live compositor backdrop supply every hue.
const glassTintABGR = 0x14000000
const acrylicTintABGR = 0x40000000

type tapSurfaceState struct {
	host                   int
	hostKnown, borderKnown bool
	border                 uintptr
}

var tapSurfaces = make(map[syscall.Handle]tapSurfaceState)

func invalidateTAPSurfaces() { clear(tapSurfaces) }

func setTAPBorder(hwnd syscall.Handle, function, border uintptr) bool {
	state := tapSurfaces[hwnd]
	if state.borderKnown && state.border == border {
		return true
	}
	if !tapCall(function, tapServicePtr, uintptr(hwnd), border) {
		return false
	}
	state.border, state.borderKnown = border, true
	tapSurfaces[hwnd] = state
	return true
}

var (
	lastStyleApplyOK bool
	ole32DLL         = syscall.NewLazyDLL("ole32.dll")
	coInitialize     = ole32DLL.NewProc("CoInitialize")

	iidTaskbarAppearanceService = guid{
		data1: 0x5bcf9150,
		data2: 0xc28a,
		data3: 0x4ef2,
		data4: [8]byte{0x91, 0x3c, 0x4c, 0x3e, 0xa2, 0xf5, 0xea, 0xd0},
	}

	tapServicePtr uintptr
	tapAvailable  bool
	tapChecked    bool
)

func taskbarWindows() []syscall.Handle {
	var result []syscall.Handle
	primaryName := utf16ptr("Shell_TrayWnd")
	secondaryName := utf16ptr("Shell_SecondaryTrayWnd")
	primary, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(primaryName)), 0)
	if primary != 0 {
		result = append(result, syscall.Handle(primary))
	}
	var after uintptr
	for {
		hwnd, _, _ := findWindowExW.Call(0, after, uintptr(unsafe.Pointer(secondaryName)), 0)
		if hwnd == 0 {
			break
		}
		result = append(result, syscall.Handle(hwnd))
		after = hwnd
	}
	return result
}

func getWindowClassName(hwnd syscall.Handle) string {
	var buf [256]uint16
	getClassNameW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf[:])
}

func initTAPService(hwnd syscall.Handle) bool {
	if tapAvailable && tapServicePtr != 0 {
		return true
	}

	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	exeDir := filepath.Dir(exePath)
	tapPath := filepath.Join(exeDir, "ExplorerTAP.dll")
	if _, err := os.Stat(tapPath); err != nil {
		return false
	}

	coInitialize.Call(0)
	tapDLL := syscall.NewLazyDLL(tapPath)
	injectProc := tapDLL.NewProc("InjectExplorerTAP")
	if injectProc.Find() != nil {
		return false
	}

	var pService uintptr
	ret, _, _ := injectProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&iidTaskbarAppearanceService)), uintptr(unsafe.Pointer(&pService)))
	if ret != 0 || pService == 0 {
		logEvent("ERROR", "tap.init", fmt.Sprintf("InjectExplorerTAP returned 0x%X", ret))
		return false
	}

	tapServicePtr = pService
	tapAvailable = true

	// Automatically restore default appearance if Senja terminates unexpectedly
	vtable := *(*uintptr)(unsafe.Pointer(tapServicePtr))
	restoreOnDieFn := *(*uintptr)(unsafe.Pointer(vtable + 8*unsafe.Sizeof(uintptr(0))))
	if restoreOnDieFn != 0 {
		syscall.SyscallN(restoreOnDieFn, tapServicePtr, uintptr(os.Getpid()))
	}

	logEvent("INFO", "tap.init", "ExplorerTAP XAML engine initialized successfully")
	return true
}

func applyRounding(hwnd syscall.Handle, enable bool) {
	if !enable {
		setWindowRgn.Call(uintptr(hwnd), 0, 1)
		return
	}
	var rc rect
	ret, _, _ := getWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	if ret == 0 {
		return
	}
	w := rc.right - rc.left
	h := rc.bottom - rc.top
	if w <= 0 || h <= 0 {
		return
	}

	marginH := int32(8)
	marginV := int32(4)
	radius := int32(14)
	rgn, _, _ := createRoundRectRgn.Call(
		uintptr(marginH),
		uintptr(marginV),
		uintptr(w-marginH),
		uintptr(h-marginV),
		uintptr(radius),
		uintptr(radius),
	)
	if rgn != 0 {
		setWindowRgn.Call(uintptr(hwnd), rgn, 1)
	}
}

func accentForStyle(style int) accentPolicy {
	switch style {
	case styleNormal:
		return accentPolicy{state: 0, flags: 0, gradient: 0, animation: 0}
	case styleGlass:
		return accentPolicy{state: 3, flags: 2, gradient: glassTintABGR}
	case styleAcrylic:
		return accentPolicy{state: 4, flags: 2, gradient: acrylicTintABGR}
	case styleClear:
		// Explicitly use our zero-alpha color. Without this flag Windows can
		// substitute the user's accent color, leaving an opaque blue host.
		return accentPolicy{state: 2, flags: 2, gradient: 0x00000000}
	case styleSolid:
		return accentPolicy{state: 1, flags: 2, gradient: 0xff151515}
	default:
		return accentPolicy{state: 0}
	}
}

func setAccent(hwnd syscall.Handle, style int, rounded bool) bool {
	if hwnd == 0 {
		return false
	}

	// Modern Windows 11 XAML path via TAP
	if initTAPService(hwnd) && tapServicePtr != 0 {
		// XAML materials need a transparent native host beneath them.
		// Otherwise Windows' original taskbar surface hides the backdrop.
		hostStyle := styleClear
		if style == styleNormal {
			hostStyle = styleNormal
		}
		surface := tapSurfaces[hwnd]
		if !surface.hostKnown || surface.host != hostStyle {
			if !setNativeAccent(hwnd, hostStyle) {
				return false
			}
			surface.host, surface.hostKnown = hostStyle, true
			tapSurfaces[hwnd] = surface
		}
		vtable := *(*uintptr)(unsafe.Pointer(tapServicePtr))
		setAppearanceFn := *(*uintptr)(unsafe.Pointer(vtable + 3*unsafe.Sizeof(uintptr(0))))
		setBlurFn := *(*uintptr)(unsafe.Pointer(vtable + 4*unsafe.Sizeof(uintptr(0))))
		returnDefaultFn := *(*uintptr)(unsafe.Pointer(vtable + 5*unsafe.Sizeof(uintptr(0))))
		setBorderFn := *(*uintptr)(unsafe.Pointer(vtable + 6*unsafe.Sizeof(uintptr(0))))

		switch style {
		case styleSolid:
			if !tapCall(setAppearanceFn, tapServicePtr, uintptr(hwnd), 1, 0xff151515) || !setTAPBorder(hwnd, setBorderFn, 1) {
				return false
			}
			applyRounding(hwnd, rounded)
			return true
		case styleNormal:
			// Normal: Restore default Windows 11 taskbar appearance & border
			if !tapCall(returnDefaultFn, tapServicePtr, uintptr(hwnd)) || !setTAPBorder(hwnd, setBorderFn, 1) {
				return false
			}
			applyRounding(hwnd, false)
			return true

		case styleGlass:
			// Senja Glass: Frosted glass background with subtle luminous dark-glass tint
			radius := float32(18.0)
			if globalConfig != nil && globalConfig.blurRadius > 0 {
				radius = globalConfig.blurRadius
			}
			blurBits := uintptr(math.Float32bits(radius))
			color := uintptr(glassTintABGR)
			if !tapCall(setBlurFn, tapServicePtr, uintptr(hwnd), color, blurBits) || !setTAPBorder(hwnd, setBorderFn, 1) {
				return false
			}
			applyRounding(hwnd, rounded)
			return true

		case styleAcrylic:
			// Senja Acrylic: Fluent acrylic material with subtle texture & tint
			color := uintptr(acrylicTintABGR)
			// ExplorerTAP TaskbarBrush: Acrylic=0, SolidColor=1.
			if !tapCall(setAppearanceFn, tapServicePtr, uintptr(hwnd), 0, color) || !setTAPBorder(hwnd, setBorderFn, 1) {
				return false
			}
			applyRounding(hwnd, rounded)
			return true

		case styleClear:
			// Senja Bening: 100% Transparent taskbar without top border
			if !tapCall(setAppearanceFn, tapServicePtr, uintptr(hwnd), 1, 0) || !setTAPBorder(hwnd, setBorderFn, 0) {
				return false
			}
			applyRounding(hwnd, rounded)
			return true
		}
	}

	// Windows 10 Win32 fallback
	ok := setNativeAccent(hwnd, style)
	applyRounding(hwnd, rounded && style != styleNormal)
	return ok
}

func tapCall(function uintptr, args ...uintptr) bool {
	result, _, _ := syscall.SyscallN(function, args...)
	if int32(result) < 0 {
		logEvent("ERROR", "tap.call", fmt.Sprintf("HRESULT=0x%08X", uint32(result)))
		return false
	}
	return true
}

func setNativeAccent(hwnd syscall.Handle, style int) bool {
	policy := accentForStyle(style)
	data := compositionAttributeData{attribute: 19, data: uintptr(unsafe.Pointer(&policy)), dataSize: unsafe.Sizeof(policy)}
	ok, _, err := setWindowComposition.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&data)))
	if ok == 0 {
		logWin32Error("taskbar.native", "SetWindowCompositionAttribute", err)
	}
	return ok != 0
}

func applyStyle(style int) {
	rounded := globalConfig != nil && globalConfig.rounded
	applyStyleInternal(style, rounded, true)
}

func reapplyStyleSilently(style int) {
	invalidateTAPSurfaces()
	rounded := globalConfig != nil && globalConfig.rounded
	applyStyleInternal(style, rounded, false)
}

func applyStyleInternal(style int, rounded bool, verbose bool) {
	started := time.Now()
	lastStyleApplyOK = false
	defer recoverPanic("applyStyle")
	if !validStyle(style) {
		if verbose {
			logEvent("ERROR", "taskbar.applyStyle", fmt.Sprintf("invalid style=%d; falling back to Senja Glass", style))
		}
		style = styleGlass
	}

	windows := taskbarWindows()
	if len(windows) == 0 {
		if verbose {
			logEvent("ERROR", "taskbar.applyStyle", "no taskbar windows found")
		}
		return
	}

	success := 0
	for _, hwnd := range windows {
		if setAccent(hwnd, style, rounded) {
			success++
		}
	}

	if success > 0 {
		lastStyleApplyOK = success == len(windows)
		processStyle = style
		styleApplied = style != styleNormal
		if verbose {
			logEvent("INFO", "taskbar.applyStyle", fmt.Sprintf("style=%d (rounded=%t) applied to %d taskbar window(s) elapsed=%s", style, rounded, success, time.Since(started).Round(time.Millisecond)))
		}
	} else if verbose {
		logEvent("ERROR", "taskbar.applyStyle", fmt.Sprintf("style=%d partially applied success=%d total=%d", style, success, len(windows)))
	}
}

func restoreTaskbar() {
	defer recoverPanic("restoreTaskbar")
	if !styleApplied {
		return
	}

	if tapServicePtr != 0 {
		vtable := *(*uintptr)(unsafe.Pointer(tapServicePtr))
		restoreAllFn := *(*uintptr)(unsafe.Pointer(vtable + 7*unsafe.Sizeof(uintptr(0))))
		if restoreAllFn != 0 {
			syscall.SyscallN(restoreAllFn, tapServicePtr)
		}
	}

	windows := taskbarWindows()
	for _, hwnd := range windows {
		applyRounding(hwnd, false)
		_ = setAccent(hwnd, styleNormal, false)
	}
	styleApplied = false
	processStyle = styleNormal
	logEvent("INFO", "taskbar.restore", "taskbar restored to Normal/Windows default")
}

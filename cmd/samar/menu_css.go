//go:build windows

package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
)

//go:embed ui/menu.html ui/menu.css ui/menu.js
var cssAssets embed.FS

const cssMenuClass = "SenjaCSSMenu"
const wmNativeMenu = wmApp + 21
const wmCSSCommand = wmApp + 22
const wmCSSResult = wmApp + 23
const wmCSSFrame = wmApp + 24

type cssState struct {
	Style   int  `json:"style"`
	Rounded bool `json:"rounded"`
	Startup bool `json:"startup"`
}
type cssMonitorInfo struct {
	Size          uint32
	Monitor, Work rect
	Flags         uint32
}

var cssBrowser *edge.Chromium
var cssWindow uintptr
var cssShown bool
var cssPreview, cssActivated bool
var cssCommandResult func(bool)
var cssOverflow uintptr
var cssClosing bool
var cssAlpha = byte(255)
var cssFadeFrom, cssFadeTo byte
var cssFadeStart time.Time
var cssFadeDuration time.Duration
var cssReducedMotion bool
var cssCommandGrace time.Time
var cssTimerResolution bool
var cssFadeFrames int
var cssFadeLast time.Time
var cssFadeMaxGap time.Duration
var cssFadeStop chan struct{}
var cssFadeQueued chan struct{}
var cssFadeGeneration uintptr

func stopCSSFade(hwnd uintptr) {
	if cssFadeStop != nil {
		close(cssFadeStop)
		cssFadeStop = nil
	}
	cssFadeGeneration++
	if cssTimerResolution {
		syscall.NewLazyDLL("winmm.dll").NewProc("timeEndPeriod").Call(1)
		cssTimerResolution = false
	}
}

func fadeAlpha(from, to byte, elapsed, duration time.Duration) byte {
	if elapsed >= duration || duration <= 0 {
		return to
	}
	t := max(0, float64(elapsed)/float64(duration))
	ease := 1 - (1-t)*(1-t)*(1-t)
	return byte(float64(from) + (float64(to)-float64(from))*ease + .5)
}

func beginCSSFade(hwnd uintptr, target byte, duration time.Duration) bool {
	stopCSSFade(hwnd)
	if cssReducedMotion {
		cssAlpha = target
		user32.NewProc("SetLayeredWindowAttributes").Call(hwnd, 0, uintptr(target), 2)
		if cssClosing {
			destroyWindow.Call(hwnd)
		}
		return true
	}
	cssFadeFrom, cssFadeTo = cssAlpha, target
	cssFadeStart, cssFadeDuration = time.Now(), duration
	cssFadeLast, cssFadeMaxGap, cssFadeFrames = cssFadeStart, 0, 0
	// Request fine timer resolution only for this finite transition. Never
	// raise timer resolution while the engine or an open menu is idle.
	result, _, _ := syscall.NewLazyDLL("winmm.dll").NewProc("timeBeginPeriod").Call(1)
	cssTimerResolution = result == 0
	// WM_TIMER is low-priority and quantized on some Windows builds. A Go
	// high-resolution ticker posts at most one outstanding frame instead.
	// This worker exists only during the 200/300 ms transition.
	stop, queued, generation := make(chan struct{}), make(chan struct{}, 1), cssFadeGeneration
	cssFadeStop, cssFadeQueued = stop, queued
	go func() {
		ticker := time.NewTicker(time.Second / 120)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				select {
				case queued <- struct{}{}:
					postMessageW.Call(hwnd, wmCSSFrame, generation, 0)
				default:
				}
			}
		}
	}()
	return true
}

var cssPopupBounds rect

func popupContainsPoint(bounds rect, position point) bool {
	return position.x >= bounds.left && position.x < bounds.right && position.y >= bounds.top && position.y < bounds.bottom
}

func pointerInsideCSSPopup() bool {
	var position point
	getCursorPos.Call(uintptr(unsafe.Pointer(&position)))
	return popupContainsPoint(cssPopupBounds, position)
}

func watchPopupOutsideClicks(hwnd uintptr, bounds rect) func() {
	callback := syscall.NewCallback(func(code int32, message, data uintptr) uintptr {
		if code >= 0 && (message == 0x0201 || message == 0x0204 || message == 0x0207 || message == 0x020b) {
			position := *(*point)(unsafe.Pointer(data))
			if !popupContainsPoint(bounds, position) {
				postMessageW.Call(hwnd, wmClose, 0, 0)
			}
		}
		result, _, _ := user32.NewProc("CallNextHookEx").Call(0, uintptr(code), message, data)
		return result
	})
	hook, _, _ := user32.NewProc("SetWindowsHookExW").Call(14, callback, uintptr(getModuleHandle()), 0)
	if hook == 0 {
		logEvent("ERROR", "menu.dismiss", "outside-click hook unavailable")
	}
	return func() {
		if hook != 0 {
			user32.NewProc("UnhookWindowsHookEx").Call(hook)
		}
	}
}

// Explorer's overflow can disappear without activating another window.
// Listen only while the popup helper is alive; no idle engine polling.
func watchTrayOverflow(hwnd, overflow uintptr) func() {
	if overflow == 0 {
		return func() {}
	}
	callback := syscall.NewCallback(func(_ uintptr, event uint32, window uintptr, object, child int32, _ uint32, _ uint32) uintptr {
		if window == overflow && object == 0 && child == 0 && (event == 0x8001 || event == 0x8003) {
			// Applying a material can hide Explorer's overflow as a side effect.
			// A click inside Senja must not dismiss the mode chooser.
			if event == 0x8003 && suppressOverflowHide(time.Now().Before(cssCommandGrace), pointerInsideCSSPopup()) {
				return 0
			}
			postMessageW.Call(hwnd, wmClose, 0, 0)
		}
		return 0
	})
	hook, _, _ := user32.NewProc("SetWinEventHook").Call(0x8001, 0x8003, 0, callback, 0, 0, 0)
	return func() {
		if hook != 0 {
			user32.NewProc("UnhookWinEvent").Call(hook)
		}
	}
}

// A material update may hide Explorer, but merely hovering the popup must
// never suppress a real tray dismissal. The exemption expires after the ack.
func suppressOverflowHide(grace, inside bool) bool {
	return inside && grace
}

func trayOverflowAt(anchor point) uintptr {
	for _, name := range []string{"TopLevelWindowForOverflowXamlIsland", "NotifyIconOverflowWindow"} {
		class := utf16ptr(name)
		var after uintptr
		for {
			window, _, _ := findWindowExW.Call(0, after, uintptr(unsafe.Pointer(class)), 0)
			if window == 0 {
				break
			}
			after = window
			visible, _, _ := user32.NewProc("IsWindowVisible").Call(window)
			var bounds rect
			if visible != 0 {
				getWindowRect.Call(window, uintptr(unsafe.Pointer(&bounds)))
				if popupContainsPoint(bounds, anchor) {
					return window
				}
			}
		}
	}
	return 0
}

func currentCSSState() cssState {
	cfg := loadConfig()
	return cssState{cfg.style, cfg.rounded, isStartupEnabled()}
}
func validCSSCommand(command uint32) bool {
	switch command {
	case cmdStyleGlass, cmdStyleAcrylic, cmdStyleClear, cmdStyleSolid, cmdRounded, cmdStartup, cmdRefresh, cmdCheckUpdates, cmdExit:
		return true
	}
	return false
}
func cssWindowHandle() uintptr {
	h, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr(cssMenuClass))), 0)
	return h
}
func closeCSSMenu() {
	if h := cssWindowHandle(); h != 0 {
		postMessageW.Call(h, wmClose, 0, 0)
	}
}

// CSS runs in an on-demand helper. Closing the menu releases its WebView process;
// the tray/taskbar engine keeps its existing low-memory native message loop.
func launchCSSMenu(parent syscall.Handle) bool {
	if h := cssWindowHandle(); h != 0 {
		postMessageW.Call(h, wmClose, 0, 0)
		return true
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	var cursor point
	getCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	// Anchor to the icon itself, including its location in Explorer's overflow.
	// The pointer may already have moved by the time the helper starts.
	if anchor, ok := cssTrayAnchor(parent); ok {
		cursor = anchor
	}
	overflow := trayOverflowAt(cursor)
	child := exec.Command(exe, "--css-menu", strconv.Itoa(int(cursor.x)), strconv.Itoa(int(cursor.y)), strconv.FormatUint(uint64(overflow), 10))
	child.Dir = filepath.Dir(exe)
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if overflow == 0 {
		setForegroundWindow.Call(uintptr(parent))
	}
	if err = child.Start(); err != nil {
		logError("menu.css", "cannot start CSS menu", err)
		return false
	}
	user32.NewProc("AllowSetForegroundWindow").Call(uintptr(child.Process.Pid))
	go func() {
		if err := child.Wait(); err != nil {
			logError("menu.css", "CSS helper failed; opening native menu", err)
			postMessageW.Call(uintptr(parent), wmNativeMenu, 0, 0)
		}
	}()
	return true
}

func cssTrayAnchor(parent syscall.Handle) (point, bool) {
	identifier := struct {
		Size   uint32
		Window syscall.Handle
		ID     uint32
		GUID   [16]byte
	}{Window: parent, ID: 1}
	identifier.Size = uint32(unsafe.Sizeof(identifier))
	var bounds rect
	result, _, _ := shell32.NewProc("Shell_NotifyIconGetRect").Call(uintptr(unsafe.Pointer(&identifier)), uintptr(unsafe.Pointer(&bounds)))
	if int32(result) < 0 || bounds.right <= bounds.left || bounds.bottom <= bounds.top {
		return point{}, false
	}
	return point{bounds.left + (bounds.right-bounds.left)/2, bounds.top}, true
}

func cssBounds(anchor point, dpi int32, work rect) rect {
	scale := func(v int32) int32 { return (v*dpi + 48) / 96 }
	w, h, gap := scale(360), scale(548), scale(12)
	if w > work.right-work.left-gap*2 {
		w = work.right - work.left - gap*2
	}
	if h > work.bottom-work.top-gap*2 {
		h = work.bottom - work.top - gap*2
	}
	x, y := anchor.x-w/2+scale(68), anchor.y-h-scale(4)
	if x < work.left+gap {
		x = work.left + gap
	}
	if x+w > work.right-gap {
		x = work.right - w - gap
	}
	if y < work.top+gap {
		y = work.top + gap
	}
	if y+h > work.bottom-gap {
		y = work.bottom - h - gap
	}
	return rect{x, y, x + w, y + h}
}

func cssHTML(state cssState, backdrop string) string {
	template, _ := cssAssets.ReadFile("ui/menu.html")
	stylesheet, _ := cssAssets.ReadFile("ui/menu.css")
	if exe, err := os.Executable(); err == nil {
		if override, err := os.ReadFile(filepath.Join(filepath.Dir(exe), "Assets", "menu.css")); err == nil && len(override) > 0 && len(override) < 256*1024 {
			stylesheet = override
		}
	}
	if backdrop != "" {
		stylesheet = append(stylesheet, []byte("\nbody{--desktop:url(\""+backdrop+"\");}")...)
	}
	js, _ := cssAssets.ReadFile("ui/menu.js")
	stateJSON, _ := json.Marshal(state)
	return strings.NewReplacer("__CSS__", string(stylesheet), "__STATE__", string(stateJSON), "__JS__", string(js)).Replace(string(template))
}

func cssMenuProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case 0x0021: // WM_MOUSEACTIVATE: keep Explorer's overflow active.
		if cssOverflow != 0 {
			return 3 // MA_NOACTIVATE: clicks still reach the WebView.
		}
	case wmCSSResult:
		if cssCommandResult != nil {
			cssCommandResult(wParam == 0)
		}
		return 0
	case 0x0007: // WM_SETFOCUS: forward keyboard focus into the WebView.
		if cssBrowser != nil && cssBrowser.GetController() != nil {
			cssBrowser.Focus()
		}
		return 0
	case 0x0100:
		if wParam == 27 {
			postMessageW.Call(hwnd, wmClose, 0, 0)
			return 0
		}
	case 0x0005:
		if cssBrowser != nil && cssBrowser.GetController() != nil {
			cssBrowser.Resize()
		}
		return 0
	case 0x0006:
		if wParam&0xffff != 0 {
			cssActivated = true
		}
		if !cssPreview && cssShown && cssActivated && wParam&0xffff == 0 && !pointerInsideCSSPopup() {
			postMessageW.Call(hwnd, wmClose, 0, 0)
		}
		return 0
	case wmClose:
		if cssClosing {
			return 0
		}
		if cssShown && cssBrowser != nil {
			cssClosing = true
			cssBrowser.Eval("window.senjaDismiss?.()")
			// The renderer starts the native fade on the same frame as its transform.
			user32.NewProc("SetTimer").Call(hwnd, 0x534f, 600, 0)
			return 0
		}
		destroyWindow.Call(hwnd)
		return 0
	case 0x0113: // WM_TIMER: close watchdog if the renderer becomes unresponsive.
		if wParam == 0x534f {
			destroyWindow.Call(hwnd) // Renderer-unresponsive close fallback.
			return 0
		}
	case wmCSSFrame:
		if cssFadeStop != nil && wParam == cssFadeGeneration {
			select {
			case <-cssFadeQueued:
			default:
			}
			now := time.Now()
			elapsed := now.Sub(cssFadeStart)
			cssFadeMaxGap = max(cssFadeMaxGap, now.Sub(cssFadeLast))
			cssFadeLast = now
			cssFadeFrames++
			cssAlpha = fadeAlpha(cssFadeFrom, cssFadeTo, elapsed, cssFadeDuration)
			user32.NewProc("SetLayeredWindowAttributes").Call(hwnd, 0, uintptr(cssAlpha), 2)
			if elapsed >= cssFadeDuration {
				stopCSSFade(hwnd)
				logEvent("INFO", "menu.motion", fmt.Sprintf("alpha=%d frames=%d elapsed=%s maxGap=%s", cssAlpha, cssFadeFrames, elapsed.Round(time.Millisecond), cssFadeMaxGap.Round(time.Millisecond)))
				if cssClosing {
					destroyWindow.Call(hwnd)
				}
			}
			return 0
		}
		return 0
	case wmDestroy:
		stopCSSFade(hwnd)
		postQuitMessage.Call(0)
		return 0
	}
	r, _, _ := defWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func runCSSMenu(preview bool) error {
	cssPreview = preview
	previewActions := preview && (len(os.Args) < 2 || os.Args[1] != "--css-check")
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	openLog()
	defer closeLog()
	prepareProcess()
	mutex, _, mutexErr := createMutexW.Call(0, 0, uintptr(unsafe.Pointer(utf16ptr("Local\\Senja.CSSMenu"))))
	if mutex == 0 {
		return fmt.Errorf("menu mutex: %v", mutexErr)
	}
	defer closeHandle.Call(mutex)
	if mutexErr == syscall.Errno(errorAlreadyExists) {
		return nil
	}
	parent, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr(windowClass))), 0)
	if !previewActions && parent == 0 {
		return fmt.Errorf("Senja tidak sedang berjalan")
	}
	var cursor point
	getCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	if len(os.Args) > 3 && !preview {
		x, errX := strconv.ParseInt(os.Args[2], 10, 32)
		y, errY := strconv.ParseInt(os.Args[3], 10, 32)
		if errX == nil && errY == nil {
			cursor = point{int32(x), int32(y)}
		}
	}
	packed := uintptr(uint64(uint32(cursor.x)) | uint64(uint32(cursor.y))<<32)
	monitor, _, _ := user32.NewProc("MonitorFromPoint").Call(packed, 2)
	mi := cssMonitorInfo{Size: uint32(unsafe.Sizeof(cssMonitorInfo{}))}
	if ok, _, _ := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&mi))); ok == 0 {
		mi.Work = rect{0, 0, 1280, 720}
	}
	var dpiX, dpiY uint32 = 96, 96
	shcore := syscall.NewLazyDLL("shcore.dll")
	shcore.NewProc("GetDpiForMonitor").Call(monitor, 0, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
	menuDPI = int32(dpiX)
	bounds := cssBounds(cursor, menuDPI, mi.Work)
	if preview {
		bounds = cssBounds(point{mi.Work.left + (mi.Work.right-mi.Work.left)/2 - menuPx(68), mi.Work.top + (mi.Work.bottom-mi.Work.top)/2 + menuPx(282)}, menuDPI, mi.Work)
	}
	cssPopupBounds = bounds
	backdrop := "" // Live native compositor; no desktop capture or PNG allocation.
	className := utf16ptr(cssMenuClass)
	class := wndClassEx{cbSize: uint32(unsafe.Sizeof(wndClassEx{})), lpfnWndProc: syscall.NewCallback(cssMenuProc), hInstance: getModuleHandle(), lpszClassName: className}
	if r, _, _ := registerClassExW.Call(uintptr(unsafe.Pointer(&class))); r == 0 {
		return fmt.Errorf("cannot register CSS menu window")
	}
	// Explorer's overflow is topmost; our nonactivating popup must live in
	// that same band above it, rather than behind its owner.
	exStyle, style := uintptr(wsExToolWindow|0x00000008), uintptr(wsPopup)
	if !preview && len(os.Args) > 4 {
		if overflow, err := strconv.ParseUint(os.Args[4], 10, 64); err == nil && overflow != 0 {
			if visible, _, _ := user32.NewProc("IsWindowVisible").Call(uintptr(overflow)); visible != 0 {
				cssOverflow = uintptr(overflow)
				exStyle |= wsExNoActivate
			}
		}
	}
	frame := bounds
	if preview {
		exStyle, style = 0, 0x00ca0000
		padding := rect{0, 0, bounds.right - bounds.left, bounds.bottom - bounds.top}
		user32.NewProc("AdjustWindowRect").Call(uintptr(unsafe.Pointer(&padding)), style, 0)
		frame = rect{bounds.left + padding.left, bounds.top + padding.top, bounds.left + padding.right, bounds.top + padding.bottom}
	}
	// Keep an independent lifetime: hiding Explorer's owner during a material
	// change would automatically hide an owned popup even without WM_CLOSE.
	exStyle |= 0x00080000 // WS_EX_LAYERED: fade the entire HWND, not just HTML.
	hwnd, _, err := createWindowExW.Call(exStyle, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16ptr("Senja — Tampilan taskbar"))), style, uintptr(frame.left), uintptr(frame.top), uintptr(frame.right-frame.left), uintptr(frame.bottom-frame.top), 0, 0, uintptr(getModuleHandle()), 0)
	if hwnd == 0 {
		return fmt.Errorf("cannot create CSS menu: %v", err)
	}
	cssWindow = hwnd
	// Extend the compositor surface into the client area so transparent
	// WebView pixels reveal the desktop, rather than the host's solid fill.
	margins := struct{ Left, Right, Top, Bottom int32 }{-1, -1, -1, -1}
	dwmExtendFrameIntoClientArea.Call(hwnd, uintptr(unsafe.Pointer(&margins)))
	cssAlpha = 0
	user32.NewProc("SetLayeredWindowAttributes").Call(hwnd, 0, 0, 2)
	defer destroyWindow.Call(hwnd)
	if cssOverflow != 0 {
		stopWatching := watchTrayOverflow(hwnd, cssOverflow)
		defer stopWatching()
	}
	if !preview {
		stopClicks := watchPopupOutsideClicks(hwnd, bounds)
		defer stopClicks()
	}
	rgn, _, _ := createRoundRectRgn.Call(0, 0, uintptr(bounds.right-bounds.left+1), uintptr(bounds.bottom-bounds.top+1), uintptr(menuPx(56)), uintptr(menuPx(56)))
	if rgn != 0 && !preview {
		setWindowRgn.Call(hwnd, rgn, 0)
	} else if rgn != 0 {
		deleteObject.Call(rgn)
	}
	cssBrowser = edge.NewChromium()
	cssBrowser.DataPath = filepath.Join(appDataDir(), "MenuWebView")
	cssBrowser.SetGlobalPermission(edge.CoreWebView2PermissionStateDeny)
	state := currentCSSState()
	cssCommandResult = func(ok bool) {
		cssCommandGrace = time.Now().Add(180 * time.Millisecond)
		state = currentCSSState()
		errorText := ""
		if !ok {
			errorText = "Efek belum diterapkan. Coba lagi."
		}
		value, _ := json.Marshal(state)
		message, _ := json.Marshal(errorText)
		cssBrowser.Eval("window.senjaUpdate(" + string(value) + "," + string(message) + ")")
	}
	cssBrowser.MessageCallback = func(raw string) {
		var message struct {
			Type          string `json:"type"`
			Command       uint32 `json:"command"`
			ReducedMotion bool   `json:"reducedMotion"`
		}
		if json.Unmarshal([]byte(raw), &message) != nil {
			return
		}
		switch message.Type {
		case "ready":
			cssReducedMotion = message.ReducedMotion
			if cssShown && !cssClosing {
				cssBrowser.Eval("window.senjaReveal?.()")
			}
			if !cssShown {
				cssShown = true
				show := uintptr(5)
				if cssOverflow != 0 {
					show = 4 // SW_SHOWNOACTIVATE
				}
				user32.NewProc("ShowWindow").Call(hwnd, show)
				// Hidden-process startup can override the first ShowWindow call.
				// Apply the requested visibility again after that startup hint.
				user32.NewProc("ShowWindow").Call(hwnd, show)
				_ = cssBrowser.Show()
				if !preview {
					// HWND_TOPMOST, SWP_NOSIZE|SWP_NOMOVE|SWP_NOACTIVATE.
					user32.NewProc("SetWindowPos").Call(hwnd, ^uintptr(0), 0, 0, 0, 0, 0x0013)
				}
				cssBrowser.Resize()
				user32.NewProc("UpdateWindow").Call(hwnd)
				// Start only after the native window is visible. A CSS animation
				// started during hidden WebView loading can finish before opening.
				cssBrowser.Eval("window.senjaReveal?.()")

				if cssOverflow == 0 {
					setForegroundWindow.Call(hwnd)
					cssBrowser.Focus()
				}
				visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
				logEvent("INFO", "menu.css.ready", fmt.Sprintf("CSS renderer ready; visible=%d bounds=%+v", visible, bounds))
			}
		case "motion":
			target, duration := byte(255), 300*time.Millisecond
			if cssClosing {
				target, duration = 0, 200*time.Millisecond
			}
			if !beginCSSFade(hwnd, target, duration) {
				user32.NewProc("SetLayeredWindowAttributes").Call(hwnd, 0, uintptr(target), 2)
				if cssClosing {
					destroyWindow.Call(hwnd)
				}
			}
		case "close":
			postMessageW.Call(hwnd, wmClose, 0, 0)
		case "reload":
			cssBrowser.NavigateToString(cssHTML(state, backdrop))
		case "command":
			if cssClosing || !validCSSCommand(message.Command) {
				return
			}
			errorText := ""
			if previewActions {
				switch message.Command {
				case cmdStyleGlass:
					state.Style = styleGlass
				case cmdStyleAcrylic:
					state.Style = styleAcrylic
				case cmdStyleClear:
					state.Style = styleClear
				case cmdStyleSolid:
					state.Style = styleSolid
				case cmdRounded:
					state.Rounded = !state.Rounded
				case cmdStartup:
					state.Startup = !state.Startup
				}
			} else {
				cssCommandGrace = time.Now().Add(750 * time.Millisecond)
				ok, _, _ := postMessageW.Call(parent, wmCSSCommand, uintptr(message.Command), hwnd)
				if ok == 0 {
					errorText = "Belum diterapkan. Coba lagi."
				} else {
					return // UI updates only after the engine acknowledges completion.
				}
				state = currentCSSState()
			}
			if message.Command == cmdExit {
				postMessageW.Call(hwnd, wmClose, 0, 0)
				return
			}
			value, _ := json.Marshal(state)
			text, _ := json.Marshal(errorText)
			cssBrowser.Eval("window.senjaUpdate(" + string(value) + "," + string(text) + ")")
		}
	}
	cssBrowser.AcceleratorKeyCallback = func(key uint) bool {
		if key == 27 {
			postMessageW.Call(hwnd, wmClose, 0, 0)
			return true
		}
		return key == 123
	}
	if !cssBrowser.Embed(hwnd) {
		return fmt.Errorf("WebView2 runtime tidak dapat dimuat")
	}
	// Transparent WebView content reveals the live Windows blur behind it.
	controller2 := cssBrowser.GetController().GetICoreWebView2Controller2()
	if controller2 == nil {
		return fmt.Errorf("WebView2 transparent background unavailable")
	}
	vt2 := (*struct{ VTable *[28]uintptr })(unsafe.Pointer(controller2)).VTable
	hr, _, _ := syscall.SyscallN(vt2[27], uintptr(unsafe.Pointer(controller2)), 0)
	syscall.SyscallN(vt2[2], uintptr(unsafe.Pointer(controller2)))
	if int32(hr) < 0 {
		return fmt.Errorf("WebView2 background HRESULT 0x%X", hr)
	}
	if !setNativeAccent(syscall.Handle(hwnd), styleGlass) {
		return fmt.Errorf("live menu backdrop unavailable")
	}
	logEvent("INFO", "menu.material", "live neutral Windows blur; no captured backdrop")
	defer func() {
		if c := cssBrowser.GetController(); c != nil {
			// ICoreWebView2Controller::Close is slot 24 of the published COM ABI.
			vt := *(*[26]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(c))))
			syscall.SyscallN(vt[24], uintptr(unsafe.Pointer(c)))
		}
		cssBrowser = nil
	}()
	settings, err := cssBrowser.GetSettings()
	if err != nil {
		return err
	}
	_ = settings.PutAreDefaultContextMenusEnabled(false)
	_ = settings.PutAreDevToolsEnabled(false)
	_ = settings.PutAreDefaultScriptDialogsEnabled(false)
	_ = settings.PutIsStatusBarEnabled(false)
	cssBrowser.Resize()
	cssBrowser.NavigateToString(cssHTML(state, backdrop))
	messageLoop()
	logEvent("INFO", "menu.css.closed", "CSS menu closed; WebView released")
	return nil
}

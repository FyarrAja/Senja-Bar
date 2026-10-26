//go:build windows

package main

import (
	"archive/zip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"samar/internal/updatestate"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type parsedVersion struct {
	major int
	minor int
	patch int
}

var (
	updateMu          sync.Mutex
	updateAutoChecked bool
	// Written only by the engine's window thread. Protocol phases match the
	// isolated updater; importing its HTTP client here would burden idle startup.
	updateProgress        uint64
	updateProgressStarted time.Time
	updatePhase           uint32
	updateRunning         bool
	updateBackground      bool
	updateAvailable       bool
	updateLatest          string
)

func scheduleUpdateChecks(cfg *config) {
	if cfg == nil || mainHwnd == 0 {
		return
	}
	user32.NewProc("KillTimer").Call(uintptr(mainHwnd), 0x5550)
	if !autoChecksEnabled(cfg) {
		return
	}
	restoreUpdateCache()
	nextUpdateCheck(3 * time.Second)
}
func startUpdateCheckManually() {
	// Download an available update, or apply an already staged update only after a second explicit action.
	if updateRunning {
		return
	}
	if updatePhase != updatestate.Available {
		restorePendingUpdate()
	}
	if !updateAvailable {
		return
	}
	startUpdateWorker(false)
}
func nextUpdateCheck(delay time.Duration) {
	if mainHwnd != 0 && autoChecksEnabled(globalConfig) && !shuttingDown {
		user32.NewProc("SetTimer").Call(uintptr(mainHwnd), 0x5550, uintptr(delay/time.Millisecond), 0)
	}
}
func startBackgroundUpdateCheck() {
	if updatePhase == updatestate.Ready && (globalConfig == nil || !globalConfig.autoInstall) {
		nextUpdateCheck(6 * time.Hour)
		return
	}
	if updateRunning || shuttingDown || !autoChecksEnabled(globalConfig) {
		return
	}
	startUpdateWorker(true)
}
func restorePendingUpdate() bool {
	p, err := updatestate.LoadPending(filepath.Join(os.Getenv("LOCALAPPDATA"), "Senja", "updates"))
	if err != nil {
		return false
	}
	next, e1 := parseReleaseVersion(p.Version)
	current, e2 := parseReleaseVersion(buildVersion)
	if e1 != nil || e2 != nil || !next.greaterThan(current) {
		return false
	}
	updateLatest = p.Version
	updateAvailable = true
	setUpdatePhase(updatestate.Ready)
	return true
}
func restoreUpdateCache() {
	if restorePendingUpdate() {
		if globalConfig != nil && globalConfig.autoInstall {
			r, err := updatestate.Load(filepath.Join(os.Getenv("LOCALAPPDATA"), "Senja", updateStateName))
			if err == nil && pendingReplacementIsNewer(updateLatest, r.Latest) {
				setUpdatePhase(updatestate.Available)
			}
		}
		return
	}
	r, err := updatestate.Load(filepath.Join(os.Getenv("LOCALAPPDATA"), "Senja", updateStateName))
	if err != nil {
		return
	}
	next, err := parseReleaseVersion(r.Latest)
	if err != nil {
		return
	}
	current, err := parseReleaseVersion(buildVersion)
	if err != nil {
		return
	}
	updateLatest = r.Latest
	if next.greaterThan(current) {
		setUpdatePhase(updatestate.Available)
	} else {
		setUpdatePhase(4)
	}
}
func startUpdateWorker(background bool, auto ...bool) {
	automatic := len(auto) > 0 && auto[0]
	exe, err := os.Executable()
	if err != nil {
		if !background && !automatic {
			postUpdateResult(err.Error())
		}
		nextUpdateCheck(30 * time.Minute)
		return
	}
	helper := filepath.Join(filepath.Dir(exe), "Senja-Update.exe")
	args := []string{buildVersion}
	if background {
		args = append(args, "--check-only")
	} else if updatePhase == updatestate.Ready {
		args = append(args, "--apply-ready")
	}
	if automatic {
		args = append(args, "--quiet")
	}
	command := exec.Command(helper, args...)
	command.Dir = filepath.Dir(exe)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err = command.Start(); err != nil {
		setUpdatePhase(5)
		if !background && !automatic {
			postUpdateResult("Pembaruan tidak dapat dimulai.\n" + err.Error())
		}
		nextUpdateCheck(30 * time.Minute)
		return
	}
	updateRunning = true
	updateBackground = background
	if len(args) > 1 && args[1] == "--apply-ready" {
		setUpdatePhase(3)
	} else {
		setUpdatePhase(1)
	}
	hwnd := uintptr(mainHwnd)
	go func() {
		code := uintptr(0)
		if err := command.Wait(); err != nil {
			code = 1
		}
		postMessageW.Call(hwnd, wmUpdateWorkerDone, code, 0)
	}()
}

func setUpdatePhase(phase uint32) {
	if phase > updatestate.Ready {
		return
	}
	if phase == updatestate.Available {
		r, err := updatestate.Load(filepath.Join(os.Getenv("LOCALAPPDATA"), "Senja", updateStateName))
		if err != nil {
			return
		}
		next, e1 := parseReleaseVersion(r.Latest)
		current, e2 := parseReleaseVersion(buildVersion)
		if e1 != nil || e2 != nil || !next.greaterThan(current) {
			phase = 4
		} else {
			updateLatest = r.Latest
			updateAvailable = true
		}
	}
	if phase == 4 {
		updateAvailable = false
	}
	if phase != 2 {
		updateProgress = 0
		updateProgressStarted = time.Time{}
	}
	updatePhase = phase
	logEvent("INFO", "updater.phase", fmt.Sprintf("phase=%d", phase))
	notifyNativePanels(wmUpdatePhase, uintptr(updatestate.Encode(phase, updateAvailable)), 0)
	if phase == updatestate.Available {
		announceAvailableUpdate()
	}
}

func announceAvailableUpdate() {
	if !updateAvailable || updateLatest == "" {
		return
	}
	path := filepath.Join(os.Getenv("LOCALAPPDATA"), "Senja", "update-notified.txt")
	last, _ := os.ReadFile(path)
	if strings.TrimSpace(string(last)) == updateLatest {
		return
	}
	if notifyUpdateAvailable(updateLatest) {
		_ = os.WriteFile(path, []byte(updateLatest), 0600)
	}
}

func checkForUpdates(manual bool) {
	if manual {
		startUpdateCheckManually()
	} else if mainHwnd != 0 {
		postMessageW.Call(uintptr(mainHwnd), wmApp+66, 0, 0)
	}
}

func runUpdateHelper(args []string) int {
	return 0
}

func postUpdateResult(message string) {
	updateMu.Lock()
	pendingUpdateMessage = message
	updateMu.Unlock()
	if mainHwnd != 0 {
		postMessageW.Call(uintptr(mainHwnd), wmUpdateResult, 0, 0)
	}
}

func takePendingUpdateMessage() string {
	updateMu.Lock()
	defer updateMu.Unlock()
	message := pendingUpdateMessage
	pendingUpdateMessage = ""
	return message
}

func parseReleaseVersion(raw string) (parsedVersion, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(strings.TrimPrefix(clean, "v"), "V")
	clean = strings.SplitN(clean, "+", 2)[0]
	clean = strings.SplitN(clean, "-", 2)[0]
	parts := strings.Split(clean, ".")
	if len(parts) == 2 {
		clean = "0." + clean
		parts = strings.Split(clean, ".")
	}
	if len(parts) != 3 {
		return parsedVersion{}, fmt.Errorf("unsupported version %q", raw)
	}
	values := [3]int{}
	for i, part := range parts {
		if part == "" {
			return parsedVersion{}, fmt.Errorf("unsupported version %q", raw)
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return parsedVersion{}, fmt.Errorf("unsupported version %q", raw)
		}
		values[i] = value
	}
	return parsedVersion{major: values[0], minor: values[1], patch: values[2]}, nil
}

func (v parsedVersion) greaterThan(other parsedVersion) bool {
	if v.major != other.major {
		return v.major > other.major
	}
	if v.minor != other.minor {
		return v.minor > other.minor
	}
	return v.patch > other.patch
}

func validateReleaseArchive(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		clean := filepath.Clean(file.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") || strings.Contains(clean, "../") || strings.Contains(clean, `..\`) {
			return fmt.Errorf("unsafe archive path %q", file.Name)
		}
	}
	return nil
}

func autoChecksEnabled(cfg *config) bool { return cfg != nil && (cfg.autoUpdate || cfg.autoInstall) }
func automaticUpdateAction(phase uint32, checked, running, enabled bool) int {
	if !enabled || running {
		return 0
	}
	if !checked {
		return 1
	}
	if phase == updatestate.Available {
		return 2
	}
	if phase == updatestate.Ready {
		return 3
	}
	return 0
}
func advanceAutomaticUpdate() {
	if shuttingDown || globalConfig == nil {
		return
	}
	switch automaticUpdateAction(updatePhase, updateAutoChecked, updateRunning, globalConfig.autoInstall) {
	case 1:
		startBackgroundUpdateCheck()
	case 2, 3:
		startUpdateWorker(false, true)
	}
}
func notifyNativePanels(message uint32, w, l uintptr) {
	for _, name := range []string{cssMenuClass, "SenjaNativeCheckPanel"} {
		hwnd, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr(name))), 0)
		if hwnd != 0 {
			postMessageW.Call(hwnd, uintptr(message), w, l)
		}
	}
}

func pendingReplacementIsNewer(pending, latest string) bool {
	a, e1 := parseReleaseVersion(pending)
	b, e2 := parseReleaseVersion(latest)
	return e1 == nil && e2 == nil && b.greaterThan(a)
}

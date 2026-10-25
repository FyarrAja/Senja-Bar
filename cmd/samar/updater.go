//go:build windows

package main

import (
	"archive/zip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type parsedVersion struct {
	major int
	minor int
	patch int
}

var (
	updateMu sync.Mutex
	// Written only by the engine's window thread. Protocol phases match the
	// isolated updater; importing its HTTP client here would burden idle startup.
	updatePhase   uint32
	updateRunning bool
)

func scheduleUpdateChecks(cfg *config) {
	logEvent("INFO", "updater", "updates run on demand only")
}

func startUpdateCheckManually() {
	if updateRunning {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		postUpdateResult(err.Error())
		return
	}
	helper := filepath.Join(filepath.Dir(exe), "Senja-Update.exe")
	command := exec.Command(helper, buildVersion)
	command.Dir = filepath.Dir(exe)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err = command.Start(); err != nil {
		setUpdatePhase(5)
		postUpdateResult("Pembaruan tidak dapat dimulai.\n" + err.Error())
		return
	}
	updateRunning = true
	setUpdatePhase(1)
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
	if phase > 5 {
		return
	}
	updatePhase = phase
	logEvent("INFO", "updater.phase", fmt.Sprintf("phase=%d", phase))
	hwnd, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(utf16ptr(cssMenuClass))), 0)
	if hwnd != 0 {
		postMessageW.Call(hwnd, wmUpdatePhase, uintptr(phase), 0)
	}
}

func checkForUpdates(manual bool) {
	if manual {
		startUpdateCheckManually()
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

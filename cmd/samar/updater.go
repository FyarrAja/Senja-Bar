//go:build windows

package main

import (
	"archive/zip"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

type parsedVersion struct {
	major int
	minor int
	patch int
}

var (
	updateMu sync.Mutex
)

func scheduleUpdateChecks(cfg *config) {
	logEvent("INFO", "updater", "safe build: remote update checks disabled")
}

func startUpdateCheckManually() {
	url := latestReleaseURL()
	result, _, err := shell32.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(utf16ptr("open"))), uintptr(unsafe.Pointer(utf16ptr(url))), 0, 0, 1)
	if result <= 32 {
		logError("updater.open", "cannot open releases page", err)
		postUpdateResult("Buka halaman pembaruan:\n" + url)
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

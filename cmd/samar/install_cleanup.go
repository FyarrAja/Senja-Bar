//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"samar/internal/installcache"
	"time"
)

func cleanPreviousInstalls() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return
	}
	root, current := filepath.Join(base, "Senja", "app"), filepath.Dir(exe)
	targets, err := installcache.Candidates(root, current)
	if err != nil || len(targets) == 0 {
		return
	} // Includes portable/development builds.
	for attempt := 0; attempt < 8 && len(targets) > 0; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}
		remaining := targets[:0]
		for _, target := range targets {
			if err := installcache.Remove(root, current, target); err != nil {
				remaining = append(remaining, target)
			} else {
				logEvent("INFO", "install.cleanup", "removed "+filepath.Base(target))
			}
		}
		targets = remaining
	}
	if len(targets) > 0 {
		logEvent("INFO", "install.cleanup", fmt.Sprintf("%d older folders remain locked or could not be removed; retained for a later startup", len(targets)))
	}
}

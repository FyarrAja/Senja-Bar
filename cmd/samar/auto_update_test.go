//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutomaticUpdateRequiresOptInFreshCheckAndIdleWorker(t *testing.T) {
	for _, phase := range []uint32{0, 1, 2, 3, 4, 5, 6, 7} {
		if automaticUpdateAction(phase, true, false, false) != 0 {
			t.Fatal("manual mode auto applied", phase)
		}
		if automaticUpdateAction(phase, true, true, true) != 0 {
			t.Fatal("duplicate worker", phase)
		}
		if automaticUpdateAction(phase, false, false, true) != 1 {
			t.Fatal("skipped fresh release check", phase)
		}
	}
	if automaticUpdateAction(6, true, false, true) != 2 || automaticUpdateAction(7, true, false, true) != 3 {
		t.Fatal("automatic stage/apply chain broken")
	}
	for _, phase := range []uint32{0, 1, 2, 3, 4, 5} {
		if automaticUpdateAction(phase, true, false, true) != 0 {
			t.Fatal("unexpected retry loop", phase)
		}
	}
}
func TestAutoInstallPreferenceMigrationPersistenceAndFailureRollback(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(configPath()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte("style=1\nauto_update=1\nstartup=0\nrounded=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := loadConfig()
	if cfg.autoInstall {
		t.Fatal("existing user silently opted into auto install")
	}
	cfg.autoInstall = true
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	restored := loadConfig()
	if !restored.autoInstall || !restored.autoUpdate || !restored.rounded || restored.style != 1 {
		t.Fatal(restored)
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := os.WriteFile(appDataDir(), []byte("block settings directory"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.autoInstall = false
	selectCommand(0, cmdAutoInstall, &cfg)
	if cfg.autoInstall {
		t.Fatal("failed persistence left toggle enabled")
	}
}
func TestAutoToggleFocusAndSmoothTrack(t *testing.T) {
	n := nativeGlassPanel{preview: true, focus: -1, pressed: -1, keyPressed: -1}
	if n.nextFocus(5, 1) != 9 || n.nextFocus(9, 1) != 6 || n.nextFocus(6, -1) != 9 || n.nextFocus(-1, -1) != 8 {
		t.Fatal("tab order differs from visual order")
	}
	n.updatePhase = 2
	if n.nextFocus(3, 1) != 8 || n.controlEnabled(9) {
		t.Fatal("hidden toggle is focusable")
	}
	if nativeSwitchTrack(0) != 0xff555b66 || nativeSwitchTrack(17) != 0xff4388ff || nativeSwitchTrack(8.5) == nativeSwitchTrack(0) || nativeSwitchTrack(8.5) == nativeSwitchTrack(17) {
		t.Fatal("switch track snaps")
	}
	actual := cssState{Style: 1}
	desired := actual
	desired.AutoInstall = true
	if nativeStateCommand(actual, desired) != cmdAutoInstall {
		t.Fatal("auto install state was dropped")
	}
}

func TestAutomaticUpdatePromotesNewerPendingRelease(t *testing.T) {
	for _, c := range []struct {
		pending, latest string
		want            bool
	}{
		{"v1.1.13", "v1.2.0", true}, {"v1.2.0", "v1.2.0", false}, {"v1.2.0", "v1.1.13", false}, {"bad", "v1.2.0", false}, {"v1.2.0", "bad", false},
	} {
		if pendingReplacementIsNewer(c.pending, c.latest) != c.want {
			t.Fatal(c)
		}
	}
	if !autoChecksEnabled(&config{autoInstall: true}) || autoChecksEnabled(nil) {
		t.Fatal("auto install did not enable checks")
	}
	if nativeSwitchTrack(0, true) != 0xff8792a5 || nativeSwitchTrack(17, true) != 0xff286be0 {
		t.Fatal("light switch palette")
	}
}

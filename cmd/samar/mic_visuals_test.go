//go:build windows

package main

import (
	"samar/internal/mic"
	"testing"
	"time"
)

func TestMeterUsesMonotonicDecibelScale(t *testing.T) {
	previous := 0.
	for _, level := range []float64{0, .01, .1, 1, 5, 10, 20, 50, 100, 150} {
		fraction := micMeterFraction(level)
		if fraction < previous || fraction < 0 || fraction > 1 {
			t.Fatalf("invalid fraction %g for %g", fraction, level)
		}
		previous = fraction
	}
	if micMeterFraction(10) < .6 || micMeterFraction(0) != 0 || micMeterFraction(100) != 1 {
		t.Fatal("meter lost speech dynamic range")
	}
}

func TestMicMeterDoesNotKeepWholePanelAnimating(t *testing.T) {
	n := nativeGlassPanel{}
	start := time.Now()
	n.mic.level.set(0, start, true)
	n.mic.level.set(80, start, false)
	if n.advanceMotion(start.Add(time.Second / 120)) {
		t.Fatal("audio meter keeps the full panel animation loop alive")
	}
}
func TestMicPresetAndOutputDropdownIsolation(t *testing.T) {
	n := nativeGlassPanel{micPage: true, motionChecked: true}
	n.mic.config = mic.ApplyPreset(mic.Config{}, 2)
	n.openMicDropdown(5)
	if n.focus != 58 || len(n.micOptions(5)) != len(mic.Presets) {
		t.Fatal("preset selection missing")
	}
	for _, i := range []int{49, 64, 65, 66, 67, 52} {
		if n.controlEnabled(i) {
			t.Fatal("popup leaked clicks")
		}
	}
	n.closeMicDropdown()
	if !n.controlEnabled(69) {
		t.Fatal("auto voice control unreachable")
	}
	n.mic.config.MonitorVolume = 70
	n.openMicDropdown(7)
	if n.micSelected(7) != 2 || !n.controlEnabled(58) || n.controlEnabled(68) {
		t.Fatal("monitor volume dropdown mismatch")
	}
	n.closeMicDropdown()
	for _, removed := range []int{52, 53, 54, 66, 67} {
		if n.controlEnabled(removed) {
			t.Fatalf("removed control %d remains interactive", removed)
		}
	}
	if len(n.micOptions(6)) != 0 {
		t.Fatal("removed output picker still available")
	}
	if !n.controlEnabled(65) || !n.controlEnabled(70) {
		t.Fatal("new controls unreachable")
	}
	n.mic.details = true
	if n.controlEnabled(65) || n.controlEnabled(66) || n.controlEnabled(67) || n.controlEnabled(69) {
		t.Fatal("hidden controls active")
	}
}

func TestAutomaticPresetIsVisibleOnItsSelectedPage(t *testing.T) {
	n := nativeGlassPanel{micPage: true, motionChecked: true}
	n.mic.config = mic.ApplyPreset(n.mic.config, 6)
	n.openMicDropdown(5)
	if n.mic.offset != 0 || n.focus != 62 || !n.controlEnabled(56) || !n.controlEnabled(62) || n.controlEnabled(63) {
		t.Fatal("all presets must be reachable in one menu")
	}
}

func TestSeventhPresetSelectionAndLiveControl(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	n := nativeGlassPanel{micPage: true, motionChecked: true}
	n.mic.config = mic.ApplyPreset(mic.Config{}, 0)
	n.openMicDropdown(5)
	n.selectMicOption(6)
	if mic.PresetIndex(n.mic.config) != 6 || n.mic.dropdown != 0 {
		t.Fatal("seventh preset did not select/close")
	}
	if !n.controlEnabled(70) {
		t.Fatal("self-monitor control unreachable")
	}
	n.mic.config.Monitor = true
	for _, v := range n.monitorChoices() {
		if v > 50 {
			t.Fatal("live volume exposes excessive preview gain")
		}
	}
	n.mic.details = true
	if n.controlEnabled(70) {
		t.Fatal("self-monitor hit target active behind details")
	}
}

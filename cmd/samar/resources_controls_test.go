//go:build windows

package main

import (
	"samar/internal/resources"
	"testing"
	"time"
)

func TestDedicatedResourceOptionsApplyWithoutLosingChoices(t *testing.T) {
	n := nativeGlassPanel{preview: true, resourcePage: true}
	defer n.endAnimation()
	n.resources.config = resources.Config{Paths: []string{"notes.exe"}, Automatic: true, EcoCPU: true}
	if !n.controlEnabled(35) || n.controlEnabled(27) || !n.controlEnabled(36) {
		t.Fatal("collapsed view exposes advanced controls")
	}
	n.resourceAction(318)
	if !n.resources.tuning || !n.controlEnabled(27) || n.controlEnabled(22) || !n.controlEnabled(36) {
		t.Fatal("inline tuning is not isolated")
	}
	n.resourceAction(315)
	if n.resources.optionOpen != 315 || n.focus != 31 || !n.controlEnabled(32) || n.controlEnabled(20) {
		t.Fatal("dropdown focus/isolation wrong")
	}
	if n.resources.config.MemoryThreshold() != 80 {
		t.Fatal("opening dropdown changed config")
	}
	n.resourceAction(332)
	if n.resources.config.MemoryThreshold() != 85 || n.resources.optionOpen != 0 {
		t.Fatal("chosen RAM option not applied")
	}
	n.resourceAction(316)
	n.resourceAction(332)
	n.resourceAction(317)
	n.resourceAction(333)
	c := n.resources.config
	if c.IdleWait() != 5*time.Minute || c.TrimWait() != time.Hour || !c.Automatic || !c.EcoCPU || len(c.Paths) != 1 {
		t.Fatal("option changed unrelated preferences", c)
	}
	if n.controlEnabled(30) {
		t.Fatal("closed option stays interactive")
	}
	n.resourceAction(319)
	if n.resources.tuning || n.controlEnabled(27) || !n.controlEnabled(22) {
		t.Fatal("done did not restore main controls")
	}
	n.resourcePage = false
	if n.controlEnabled(27) || n.controlEnabled(28) || n.controlEnabled(29) {
		t.Fatal("inline controls leaked into taskbar page")
	}
}

func TestTuningPopoverIsolatesInputAndPreservesSelection(t *testing.T) {
	n := nativeGlassPanel{preview: true, resourcePage: true}
	defer n.endAnimation()
	n.resources.loaded = true
	n.resources.config = resources.Config{Paths: []string{"notes.exe"}, Automatic: true}
	n.resources.rows = []resourceRow{{path: "notes.exe"}}
	n.resourceAction(318)
	n.layoutResourceControls()
	for _, i := range []int{12, 18, 19, 22, 23, 25, 26, 37} {
		if n.controlEnabled(i) {
			t.Fatalf("hidden main-page control %d remains interactive on tuning page", i)
		}
	}
	for _, i := range []int{20, 21, 24, 27, 28, 29, 35, 36} {
		if !n.controlEnabled(i) {
			t.Fatalf("tuning control %d is unavailable", i)
		}
	}
	if nativeControlBoxes[27][1] != 180 || nativeControlBoxes[36][1] != 84 {
		t.Fatal("tuning controls must use the dedicated page layout")
	}
	if n.nextFocus(36, 1) != 27 || n.nextFocus(29, 1) != 20 || n.nextFocus(20, -1) != 29 {
		t.Fatal("keyboard focus must follow the visible settings from top to bottom")
	}
	if nativeControlBoxes[29][1]+nativeControlBoxes[29][3] >= nativeControlBoxes[20][1] {
		t.Fatal("automatic rules overlap the toggles")
	}
	n.resourceAction(319)
	n.layoutResourceControls()
	if !n.controlEnabled(12) || !n.controlEnabled(35) || !n.resources.config.Automatic || !n.resources.config.Selected("notes.exe") {
		t.Fatal("returning must restore the list and preserve preferences")
	}
	if n.controlEnabled(20) || n.controlEnabled(21) || n.controlEnabled(27) || n.controlEnabled(24) {
		t.Fatal("automatic controls leaked into the applications tab")
	}
	n.resourceAction(318)
	n.resourceAction(cmdTaskbarPage)
	if n.resources.tuning || n.resources.optionOpen != 0 {
		t.Fatal("sidebar navigation must clear the tuning page")
	}
}

func TestPerAppToggleWorksInAllAndSelectedScopes(t *testing.T) {
	c := resources.Config{AllBackground: true, Automatic: true, EcoCPU: true, Paths: []string{"legacy.exe"}}
	toggleResourceApp(&c, "notes.exe")
	if c.Selected("notes.exe") || !c.Selected("other.exe") || !c.AllBackground || !c.Automatic || !c.EcoCPU {
		t.Fatal("app off changed global policy", c)
	}
	toggleResourceApp(&c, "NOTES.exe")
	if !c.Selected("notes.exe") || len(c.ExcludedPaths) != 0 {
		t.Fatal("app on did not remove exclusion", c)
	}
	toggleResourceApp(&c, "installed:metadata")
	if len(c.ExcludedPaths) != 0 {
		t.Fatal("unresolved metadata can be enabled")
	}
	c.AllBackground = false
	toggleResourceApp(&c, "notes.exe")
	if !c.Selected("notes.exe") || !c.Selected("legacy.exe") {
		t.Fatal("selected mode lost previous choices")
	}
	toggleResourceApp(&c, "NOTES.exe")
	if c.Selected("notes.exe") || !c.Selected("legacy.exe") {
		t.Fatal("selected mode app off changed another choice")
	}
	n := nativeGlassPanel{resourcePage: true}
	n.resources.config.AllBackground = true
	n.resources.rows = []resourceRow{{path: "notes.exe"}}
	if n.controlEnabled(12) || n.controlEnabled(25) || !n.controlEnabled(37) || n.controlEnabled(27) {
		t.Fatal("all-background apps tab must expose scope and hide list/rules")
	}
}

func TestResourceScopeDropdownChangesLayoutWithoutLosingChoices(t *testing.T) {
	n := nativeGlassPanel{preview: true, resourcePage: true}
	defer n.endAnimation()
	n.resources.config = resources.Config{AllBackground: true, Paths: []string{"notes.exe"}, ExcludedPaths: []string{"other.exe"}}
	n.resourceAction(320)
	n.resourceAction(331)
	if n.resources.config.AllBackground || !n.controlEnabled(25) || !n.controlEnabled(12) && len(n.resources.rows) > 0 || len(n.resources.config.Paths) != 1 {
		t.Fatal("selected scope layout lost choices")
	}
	n.resourceAction(320)
	n.resourceAction(330)
	if !n.resources.config.AllBackground || len(n.resources.config.ExcludedPaths) != 0 || n.controlEnabled(25) || n.controlEnabled(19) {
		t.Fatal("all scope must apply all and hide pagination")
	}
	n.layoutResourceControls()
	if nativeControlBoxes[22][2] != 318 {
		t.Fatal("all-scope primary button not expanded")
	}
}

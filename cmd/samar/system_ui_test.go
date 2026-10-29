//go:build windows

package main

import "testing"

func TestSystemNavigationDoesNotChangeResourcePreferences(t *testing.T) {
	n := nativeGlassPanel{preview: true, resourcePage: true}
	n.system.loaded = true
	n.resources.loaded = true
	n.resources.config.AllBackground = true
	n.resources.config.Automatic = true
	n.resources.config.EcoCPU = true
	if !n.systemAction(cmdSystem) || !n.systemPage || n.resourcePage {
		t.Fatal("system page not selected")
	}
	n.layoutSystemControls()
	if !n.controlEnabled(38) || !n.controlEnabled(39) || n.controlEnabled(20) || n.controlEnabled(41) {
		t.Fatal("monitor exposes unrelated controls")
	}
	n.systemAction(701)
	if !n.controlEnabled(41) || !n.controlEnabled(46) {
		t.Fatal("tool navigation unavailable")
	}
	n.system.busy = true
	if n.controlEnabled(41) || !n.controlEnabled(39) {
		t.Fatal("busy job must disable new jobs but retain navigation")
	}
	n.system.busy = false
	n.resourceAction(cmdResources)
	if n.systemPage || !n.resourcePage || !n.resources.config.Automatic || !n.resources.config.EcoCPU {
		t.Fatal("navigation changed persisted resource policy")
	}
}

func TestSystemToolMenusFitAndUseFixedActions(t *testing.T) {
	for _, section := range []string{"tools", "power", "storage", "network", "repair"} {
		items := systemItems(section)
		if len(items) == 0 || len(items) > 6 {
			t.Fatalf("menu does not fit: %s", section)
		}
		for _, item := range items {
			if item.label == "" || item.key == "" {
				t.Fatal("empty action")
			}
		}
	}
	n := nativeGlassPanel{systemPage: true}
	n.system.section = "tools"
	n.layoutSystemControls()
	if nativeControlBoxes[46][1]+nativeControlBoxes[46][3] >= nativeControlBoxes[47][1] {
		t.Fatal("tools overlap export action")
	}
	if n.controlAt(nativeSidebar+30, 140) != 41 {
		t.Fatal("tool hit target disagrees with layout")
	}
}

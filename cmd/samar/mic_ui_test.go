//go:build windows

package main

import "testing"

func TestMicPageIsolatesControlsAndPreservesResourcePolicy(t *testing.T) {
	n := nativeGlassPanel{preview: true, resourcePage: true}
	n.mic.loaded = true
	n.resources.config.Automatic = true
	if !n.micAction(cmdMic) || !n.micPage || n.resourcePage || n.systemPage {
		t.Fatal("mic navigation failed")
	}
	n.layoutMicControls()
	if n.controlEnabled(20) || n.controlEnabled(41) || !n.controlEnabled(51) || !n.controlEnabled(48) {
		t.Fatal("mic page exposes unrelated controls")
	}
	if n.controlAt(nativeSidebar+30, 300) != 51 {
		t.Fatal("voice body hit target mismatch")
	}
	if !n.resources.config.Automatic {
		t.Fatal("navigation changed resource preference")
	}
	n.system.loaded = true
	n.systemAction(cmdSystem)
	if n.micPage || !n.systemPage {
		t.Fatal("leaving mic failed")
	}
	for _, page := range []bool{false, true} {
		n.micPage = page
		for i := 49; i <= 55; i++ {
			if n.controlEnabled(i) && !page {
				t.Fatal("hidden mic control active")
			}
		}
	}
}

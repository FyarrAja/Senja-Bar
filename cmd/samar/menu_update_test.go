//go:build windows

package main

import "testing"

func TestUpdateActionAvailability(t *testing.T) {
	for _, c := range []struct {
		phase                  uint32
		pending, enabled, mark bool
		label                  string
	}{
		{0, false, false, false, "Pembaruan"}, {1, true, false, true, "Pembaruan"},
		{2, true, false, true, "Pembaruan"}, {3, true, false, true, "Pembaruan"},
		{4, false, false, false, "Sudah terbaru"}, {5, false, false, false, "Pembaruan"},
		{5, true, true, true, "Perbarui"}, {6, true, true, true, "Perbarui"}, {6, false, false, false, "Pembaruan"},
	} {
		label, _, enabled, mark := updatePresentation(c.phase, c.pending, "1.1.7")
		if label != c.label || enabled != c.enabled || mark != c.mark {
			t.Fatalf("%+v: %s %v %v", c, label, enabled, mark)
		}
		n := nativeGlassPanel{updatePhase: c.phase, updateAvailable: c.pending}
		if n.controlEnabled(7) != c.enabled || !n.controlEnabled(6) {
			t.Fatalf("control state %+v", c)
		}
	}
}

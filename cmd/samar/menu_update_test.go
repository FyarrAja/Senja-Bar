//go:build windows

package main

import (
	"math"
	"testing"
	"time"
)

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

func TestDownloadMotionContinuousMonotonicAndFinite(t *testing.T) {
	start := time.Now()
	m := nativeDownloadMotion{}
	m.target(.4, start)
	if !m.advance(start.Add(80*time.Millisecond)) || m.value <= 0 || m.value >= .4 {
		t.Fatal(m)
	}
	before := m.value
	m.target(.8, start.Add(80*time.Millisecond))
	if math.Abs(m.value-before) > 1e-9 {
		t.Fatal("progress jumped", m)
	}
	m.advance(start.Add(160 * time.Millisecond))
	if m.value < before || m.value >= .8 {
		t.Fatal(m)
	}
	if m.advance(start.Add(240*time.Millisecond)) || m.value != .8 {
		t.Fatal("did not settle", m)
	}
	m.target(.1, start.Add(time.Second))
	if m.to != .8 {
		t.Fatal("bar moved backward", m)
	}
}

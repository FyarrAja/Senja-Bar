//go:build windows

package main

import (
	"math"
	"samar/internal/updatestate"
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
		if n.controlEnabled(7) != c.enabled || n.controlEnabled(6) != (c.phase != 2) {
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

func TestDownloadCopyRealMeasurements(t *testing.T) {
	d := updatestate.Download{Done: 2500000, Total: 10000000, Elapsed: 2 * time.Second}
	amount, speed, eta := downloadCopy(d)
	if amount != "2.50 / 10.00 MB" || speed != "1.25 MB/s" || eta != "Sisa 6 dtk" {
		t.Fatal(amount, speed, eta)
	}
	d.Done = 0
	d.Elapsed = 0
	amount, speed, eta = downloadCopy(d)
	if amount != "0.00 / 10.00 MB" || speed != "-- MB/s" || eta != "Menghitung..." {
		t.Fatal(amount, speed, eta)
	}
	d.Total = 0
	amount, _, eta = downloadCopy(d)
	if amount != "0.00 / -- MB" || eta != "Waktu belum diketahui" {
		t.Fatal(amount, eta)
	}
}

func TestUpdateNoticePulsesThenStops(t *testing.T) {
	start := time.Now()
	for _, offset := range []time.Duration{0, 600 * time.Millisecond, 1200 * time.Millisecond, 1800 * time.Millisecond, 3 * time.Second} {
		scale, active := updateNoticeScale(start, start.Add(offset))
		if math.Abs(scale-1) > 1e-9 || active != (offset < 1800*time.Millisecond) {
			t.Fatal(offset, scale, active)
		}
	}
	for _, offset := range []time.Duration{300 * time.Millisecond, 900 * time.Millisecond, 1500 * time.Millisecond} {
		scale, active := updateNoticeScale(start, start.Add(offset))
		if math.Abs(scale-1.08) > 1e-9 || !active {
			t.Fatal(offset, scale, active)
		}
	}
	if scale, active := updateNoticeScale(time.Time{}, start); scale != 1 || active {
		t.Fatal("idle badge animates")
	}
}

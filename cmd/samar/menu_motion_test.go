//go:build windows

package main

import (
	"math"
	"testing"
	"time"
)

func TestMotionRefreshRateAndInterruption(t *testing.T) {
	start := time.Unix(1000, 0)
	run := func(frames int) nativeMotion {
		m := nativeMotion{}
		m.set(0, start, true)
		m.set(150, start, false)
		for i := 1; i <= frames; i++ {
			m.advance(start.Add(time.Duration(i)*time.Second/time.Duration(frames)), 28)
		}
		return m
	}
	a, b := run(60), run(144)
	if math.Abs(a.value-b.value) > .001 || a.value != 150 {
		t.Fatalf("refresh-dependent motion: %v %v", a, b)
	}
	m := nativeMotion{}
	m.set(0, start, true)
	m.set(150, start, false)
	m.advance(start.Add(40*time.Millisecond), 28)
	position, velocity := m.value, m.velocity
	m.set(50, start.Add(40*time.Millisecond), false)
	if m.value != position || m.velocity != velocity {
		t.Fatal("retarget snapped")
	}
	for i := 1; i <= 120; i++ {
		m.advance(start.Add(40*time.Millisecond+time.Duration(i)*time.Second/120), 28)
	}
	if m.value != 50 || m.velocity != 0 {
		t.Fatal("motion did not settle")
	}
	m.set(20, start, true)
	if m.value != 20 || m.velocity != 0 {
		t.Fatal("reduced motion must snap")
	}
}
func TestMicDropdownIsolationAndInputGain(t *testing.T) {
	n := nativeGlassPanel{micPage: true, motionChecked: true, focus: -1}
	n.mic.loaded = true
	n.mic.config.InputGain = 100
	n.openMicDropdown(3)
	if n.micSelected(3) != 2 || n.focus != 58 {
		t.Fatal("gain selection missing")
	}
	for _, i := range []int{8, 10, 20, 49, 51, 52, 64} {
		if n.controlEnabled(i) {
			t.Fatalf("popup click-through %d", i)
		}
	}
	if !n.controlEnabled(56) || n.controlEnabled(61) {
		t.Fatal("popup options mismatch")
	}
	y, _, _ := n.micDropdownGeometry(3)
	if got := n.controlAt(nativeSidebar+30, y+12); got != 56 {
		t.Fatalf("popup hit=%d", got)
	}
	n.closeMicDropdown()
	if n.mic.dropdown != 0 || n.controlEnabled(56) || !n.controlEnabled(64) {
		t.Fatal("dismiss failed")
	}
	n.mic.details = true
	if n.controlEnabled(64) {
		t.Fatal("hidden input active")
	}
}

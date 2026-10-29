//go:build windows

package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"time"
	"unsafe"
)

// Analytic critical damping preserves velocity on interruption and is frame-rate independent.
type nativeMotion struct {
	value, velocity, target float64
	last                    time.Time
	ready                   bool
}

func (m *nativeMotion) set(target float64, now time.Time, snap bool) {
	if !m.ready || snap {
		m.value, m.velocity, m.ready = target, 0, true
	}
	m.target = target
	if m.last.IsZero() {
		m.last = now
	}
}
func (m *nativeMotion) advance(now time.Time, frequency float64) bool {
	if !m.ready {
		return false
	}
	dt := max(0., now.Sub(m.last).Seconds())
	m.last = now
	x := m.value - m.target
	decay := math.Exp(-frequency * dt)
	c := m.velocity + frequency*x
	m.value = m.target + (x+c*dt)*decay
	m.velocity = (m.velocity - frequency*c*dt) * decay
	if math.Abs(m.value-m.target) < .015 && math.Abs(m.velocity) < .1 {
		m.value = m.target
		m.velocity = 0
		return false
	}
	return true
}
func (n *nativeGlassPanel) reduceMotion() bool {
	if !n.motionChecked {
		enabled := uint32(1)
		user32.NewProc("SystemParametersInfoW").Call(0x1042, 0, uintptr(unsafe.Pointer(&enabled)), 0)
		n.motionReduced = enabled == 0
		// Follow Windows unless the user explicitly opts in to a Senja motion mode.
		var preference struct{ Motion string }
		data, err := os.ReadFile(filepath.Join(os.Getenv("LOCALAPPDATA"), "Senja", "ui.json"))
		if err == nil && json.Unmarshal(data, &preference) == nil {
			if preference.Motion == "smooth" {
				n.motionReduced = false
			}
			if preference.Motion == "reduced" {
				n.motionReduced = true
			}
		}
		n.motionChecked = true
	}
	return n.motionReduced
}
func (n *nativeGlassPanel) move(m *nativeMotion, target float64) {
	if m.ready && m.target == target {
		return
	}
	now := time.Now()
	if m.ready {
		m.advance(now, 28)
	}
	m.set(target, now, n.reduceMotion())
	if m.value != target {
		n.ensureAnimationTicker()
	}
}
func (n *nativeGlassPanel) advanceMotion(now time.Time) bool {
	active := n.navMotion.advance(now, 28)
	active = n.tabMotion.advance(now, 30) || active
	active = n.mic.reveal.advance(now, 36) || active
	active = n.mic.monitorMotion.advance(now, 32) || active
	for i := range n.hovers {
		active = n.hovers[i].advance(now, 40) || active
	}
	return active
}

func (n *nativeGlassPanel) updateHover(index int) {
	for i := range n.hovers {
		target := 0.
		if i == index {
			target = 1
		}
		if !n.hovers[i].ready {
			n.hovers[i].set(0, time.Now(), true)
		}
		n.move(&n.hovers[i], target)
	}
}
func (n *nativeGlassPanel) drawHover() {
	for i := range n.hovers {
		if !n.controlEnabled(i) || n.hovers[i].value < .02 {
			continue
		}
		b := nativeControlBoxes[i]
		color := uint32(12*n.hovers[i].value)<<24 | 0x00ffffff
		if n.light {
			color = uint32(8*n.hovers[i].value) << 24
		}
		n.round(b[0], b[1], b[2], b[3], min(12, b[3]/2), color, false)
	}
}

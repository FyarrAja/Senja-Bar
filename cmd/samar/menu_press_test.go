//go:build windows

package main

import (
	"testing"
	"time"
)

func TestQuickClickFinishesDownstrokeThenSettles(t *testing.T) {
	start := time.Unix(1, 0)
	p := nativePress{}
	p.target(1, start)
	p.advance(start.Add(time.Millisecond))
	before := p.value
	p.release(start.Add(time.Millisecond), true)
	if p.value != before {
		t.Fatal("release must not jump to a fake peak")
	}
	if !p.advance(start.Add(70*time.Millisecond)) || p.value != 1 || p.to != 0 {
		t.Fatal("short click must complete its downstroke before release")
	}
	if !p.advance(start.Add(140*time.Millisecond)) || p.value <= 0 || p.value >= 1 {
		t.Fatal("missing release interpolation")
	}
	if p.advance(start.Add(250*time.Millisecond)) || p.value != 0 {
		t.Fatal("animation must settle without a resident ticker")
	}
}
func TestDragCancellationAndReentryReverseContinuously(t *testing.T) {
	start := time.Unix(1, 0)
	p := nativePress{}
	p.target(1, start)
	p.advance(start.Add(25 * time.Millisecond))
	before := p.value
	p.release(start.Add(25*time.Millisecond), false)
	if p.value != before || p.releasePending || p.to != 0 {
		t.Fatal("drag cancellation must immediately reverse without a pulse")
	}
	p.advance(start.Add(50 * time.Millisecond))
	if p.value >= before {
		t.Fatal("cancel did not release")
	}
	before = p.value
	p.target(1, start.Add(50*time.Millisecond))
	if p.value != before {
		t.Fatal("reentry jumped")
	}
	if p.advance(start.Add(120*time.Millisecond)) || p.value != 1 {
		t.Fatal("held button must stop animating at its pressed state")
	}
}

package resources

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEligibilityFailsClosed(t *testing.T) {
	c := Config{Paths: []string{filepath.Join("apps", "notes.exe")}}
	p := Candidate{Path: c.Paths[0], Minimized: true}
	if !eligible(p, c, true, false, IdleDelay) {
		t.Fatal("idle selected app excluded")
	}
	cases := []struct {
		p             Candidate
		c             Config
		audio, paused bool
		idle          time.Duration
	}{
		{p, Config{}, true, false, IdleDelay}, {p, c, false, false, IdleDelay}, {p, c, true, true, IdleDelay}, {p, c, true, false, IdleDelay - time.Second},
		{Candidate{Path: p.Path}, c, true, false, IdleDelay}, {Candidate{Path: p.Path, Minimized: true, Protected: "audio"}, c, true, false, IdleDelay},
	}
	for i, x := range cases {
		if eligible(x.p, x.c, x.audio, x.paused, x.idle) {
			t.Fatalf("unsafe case %d allowed", i)
		}
	}
}
func TestProtectFamilyWithoutExplorerSiblings(t *testing.T) {
	all := []Candidate{{PID: 1, Path: "explorer"}, {PID: 2, Parent: 1, Path: "browser"}, {PID: 3, Parent: 2, Path: "renderer"}, {PID: 4, Parent: 1, Path: "notes"}, {PID: 5, Parent: 1, Path: "browser"}}
	got := protectedFamily(all, map[uint32]bool{2: true})
	for _, id := range []uint32{1, 2, 3, 5} {
		if !got[id] {
			t.Fatal("unprotected family", id)
		}
	}
	if got[4] {
		t.Fatal("protected unrelated Explorer child")
	}
}
func TestSettingsRoundTrip(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if Load().Automatic || Load().EcoCPU {
		t.Fatal("must default off")
	}
	c := Config{Paths: []string{"notes.exe"}, Automatic: true, EcoCPU: true}
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if !got.Automatic || !got.EcoCPU || !selected(got, "NOTES.EXE") {
		t.Fatal(got)
	}
}

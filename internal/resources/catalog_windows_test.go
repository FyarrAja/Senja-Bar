//go:build windows

package resources

import (
	"testing"
	"time"
)

func TestCatalogExecutableParsing(t *testing.T) {
	t.Setenv("LOCALAPPDATA", `C:\Users\Test\AppData\Local`)
	for _, c := range []struct{ raw, want string }{
		{`"C:\Program Files\Notes\notes.exe",0`, `C:\Program Files\Notes\notes.exe`},
		{`C:\Program Files\Notes\notes.exe,-12`, `C:\Program Files\Notes\notes.exe`},
		{`%LOCALAPPDATA%\Notes\notes.exe`, `C:\Users\Test\AppData\Local\Notes\notes.exe`},
		{`C:\Windows\icons.dll,1`, ""}, {`msiexec.exe`, ""},
	} {
		if got := catalogExecutable(c.raw); got != c.want {
			t.Fatalf("%q: %q != %q", c.raw, got, c.want)
		}
	}
}
func TestResourceLimitsAndCustomEligibility(t *testing.T) {
	invalid := Config{MemoryPercent: 1, IdleMinutes: -1, CooldownMinutes: 1}
	if invalid.MemoryThreshold() != 80 || invalid.IdleWait() != IdleDelay || invalid.TrimWait() != TrimCooldown {
		t.Fatal("unsafe values accepted")
	}
	c := Config{Paths: []string{"notes.exe"}, MemoryPercent: 90, IdleMinutes: 5, CooldownMinutes: 60}
	p := Candidate{Path: "notes.exe", Minimized: true}
	if eligible(p, c, true, false, 2*time.Minute) || !eligible(p, c, true, false, 5*time.Minute) {
		t.Fatal("custom idle threshold ignored")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if got.MemoryThreshold() != 90 || got.IdleWait() != 5*time.Minute || got.TrimWait() != time.Hour {
		t.Fatal("custom settings lost")
	}
}

package resources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllBackgroundScopeRetainsProtection(t *testing.T) {
	c := Config{AllBackground: true}
	p := Candidate{Path: filepath.Join("apps", "notes.exe"), Minimized: true}
	if !eligible(p, c, true, false, IdleDelay) {
		t.Fatal("all-background mode still requires selection")
	}
	p.Protected = "audio"
	if eligible(p, c, true, false, IdleDelay) {
		t.Fatal("audio protection bypassed")
	}
	p.Protected = ""
	p.Minimized = false
	if eligible(p, c, true, false, IdleDelay) {
		t.Fatal("visible application allowed")
	}
	p.Minimized = true
	if eligible(p, c, false, false, IdleDelay) || eligible(p, c, true, true, IdleDelay) {
		t.Fatal("audio/fullscreen guard bypassed")
	}
	if selected(c, "store:package") || selected(c, "installed:metadata") {
		t.Fatal("catalog metadata treated as process")
	}
}
func TestScopeMigrationIsNotSilentOptIn(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if !Load().AllBackground {
		t.Fatal("fresh installation should offer all-background target")
	}
	if err := os.MkdirAll(filepath.Dir(Path()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte(`{"paths":["notes.exe"],"automatic":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	c := Load()
	if c.AllBackground {
		t.Fatal("legacy selected-app setting broadened silently")
	}
	c.AllBackground = true
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	if !Load().AllBackground {
		t.Fatal("scope not persisted")
	}
}

func TestAllBackgroundExclusionsAreEnforcedAndPersisted(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	c := Config{AllBackground: true, Automatic: true, EcoCPU: true, ExcludedPaths: []string{"notes.exe"}}
	p := Candidate{Path: "NOTES.exe", Minimized: true}
	if eligible(p, c, true, false, IdleDelay) {
		t.Fatal("unchecked app still eligible")
	}
	p.Path = "other.exe"
	if !eligible(p, c, true, false, IdleDelay) {
		t.Fatal("exclusion disabled unrelated app")
	}
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	restored := Load()
	if restored.Selected("notes.exe") || !restored.Selected("other.exe") || !restored.Automatic || !restored.EcoCPU {
		t.Fatal("app opt-out not preserved", restored)
	}
	c.AllBackground = false
	c.Paths = []string{"notes.exe"}
	if !c.Selected("NOTES.exe") {
		t.Fatal("explicit selection must override exclusion in other scope")
	}
}

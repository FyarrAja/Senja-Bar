package updatestate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPendingPathRejectsEscapesAndBadHashes(t *testing.T) {
	for _, directory := range []string{"../outside", "v1.2.0-../outside", "C:\\bad", "v1.2.0-x/evil", "wrong"} {
		p := Pending{Version: "v1.2.0", Directory: directory, SHA256: strings.Repeat("a", 64)}
		if _, err := p.Path(t.TempDir()); err == nil {
			t.Fatal("accepted path", directory)
		}
	}
	p := Pending{Version: "v1.2.0", Directory: "v1.2.0-test", SHA256: "bad"}
	if _, err := p.Path(t.TempDir()); err == nil {
		t.Fatal("accepted bad checksum")
	}
}
func TestPendingSurvivesReloadAndReplacement(t *testing.T) {
	root := t.TempDir()
	p := Pending{Version: "v1.2.0", Directory: "v1.2.0-test", SHA256: strings.Repeat("a", 64)}
	dir := filepath.Join(root, p.Directory)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Senja-Bar.exe"), []byte("MZ-test"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := SavePending(root, p); err != nil {
			t.Fatal(err)
		}
		got, err := LoadPending(root)
		if err != nil || got != p {
			t.Fatal(got, err)
		}
	}
}

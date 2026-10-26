package releaseupdate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"samar/internal/updatestate"
	"testing"
)

func TestStageWaitsForExplicitApplyAndRechecksBytes(t *testing.T) {
	root := t.TempDir()
	exe := "MZ-staged-not-executed"
	sum := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte(exe)), Installer)
	var phases []int
	if err := Stage(context.Background(), fixture(t, stable(), exe, sum), "1.1.12", root, func(p int) { phases = append(phases, p) }, nil); err != nil {
		t.Fatal(err)
	}
	if phases[len(phases)-1] != updatestate.Ready {
		t.Fatal(phases)
	}
	for _, p := range phases {
		if p == Installing {
			t.Fatal("staging attempted installation")
		}
	}
	pending, err := updatestate.LoadPending(root)
	if err != nil || pending.Version != "v1.2.0" {
		t.Fatal(pending, err)
	}
	path, err := VerifiedPending("1.1.12", root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = VerifiedPending("1.2.0", root); err == nil {
		t.Fatal("reapplied current version")
	}
	if err = os.WriteFile(path, []byte("MZ-tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = VerifiedPending("1.1.12", root); err == nil {
		t.Fatal("accepted changed staged installer")
	}
}
func TestCurrentVersionDoesNotCreatePendingUpdate(t *testing.T) {
	root := t.TempDir()
	if err := Stage(context.Background(), fixture(t, stable(), "unused", "unused"), "1.2.0", root, func(int) {}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "pending.json")); !os.IsNotExist(err) {
		t.Fatal("created pending update", err)
	}
}

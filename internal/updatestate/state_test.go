package updatestate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCacheReplacementAndBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Senja", "update-state.json")
	for _, tag := range []string{"v1.1.6", "v1.1.7"} {
		want := Record{Latest: tag, CheckedAt: time.Now().UTC()}
		if err := Save(path, want); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil || got.Latest != tag || !got.CheckedAt.Equal(want.CheckedAt) {
			t.Fatalf("%+v %v", got, err)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 4097)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("accepted oversized cache")
	}
}
func TestPendingFlagSurvivesProgress(t *testing.T) {
	for phase := uint32(0); phase <= Available; phase++ {
		if Encode(phase, true) != phase|PendingFlag || Encode(phase, false) != phase {
			t.Fatal(phase)
		}
	}
	if Encode(999, true) != PendingFlag {
		t.Fatal("unknown phase leaked")
	}
}

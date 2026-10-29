//go:build windows

package systemtools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdminPlansRejectUnknownCommands(t *testing.T) {
	for _, key := range []string{"sfc", "verify", "dism", "ssd", "winsock", "dns-cloudflare", "dns-google", "dns-auto"} {
		script, err := adminScript(key)
		if err != nil || script == "" {
			t.Fatalf("missing plan %s: %v", key, err)
		}
		if strings.Contains(script, "Remove-Item") || strings.Contains(script, "Stop-Service") {
			t.Fatalf("destructive shortcut in %s", key)
		}
	}
	if _, err := adminScript("sfc; remove-item C:\\"); err == nil {
		t.Fatal("arbitrary action accepted")
	}
	if err := Open("untrusted.exe"); err == nil {
		t.Fatal("untrusted tool accepted")
	}
}
func TestReportPreservesFailuresAndMeasuredData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	s := Snapshot{RAMTotalGB: 20, RAMFreeGB: 8, Error: "telemetry unavailable"}
	path, err := SaveReport(s, "GAGAL: akses ditolak")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != ReportDir() {
		t.Fatal("report escaped report directory")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "GAGAL: akses ditolak") || !strings.Contains(string(b), "telemetry unavailable") {
		t.Fatal("report hides failures")
	}
	if strings.Contains(string(b), "Verified Healthy") {
		t.Fatal("fabricated diagnostic result")
	}
}
func TestLiveTelemetry(t *testing.T) {
	if os.Getenv("SENJA_NATIVE_INTEGRATION") != "1" {
		t.Skip("opt-in read-only system integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	s := Read(ctx)
	if s.Error != "" {
		t.Fatal(s.Error)
	}
	if s.Model == "" || s.CPU == "" || s.RAMTotalGB <= 0 || len(s.Volumes) == 0 {
		t.Fatalf("incomplete live data: %+v", s)
	}
	t.Logf("%s; RAM %.1f GB; %d local volumes; GPU %s", s.Model, s.RAMTotalGB, len(s.Volumes), s.GPU)
}

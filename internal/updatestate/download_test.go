package updatestate

import (
	"math"
	"testing"
	"time"
)

func TestExactDownloadCountsRateAndETA(t *testing.T) {
	wire := DownloadWire(2500123, 10000492)
	d := ReadDownload(wire, 2*time.Second)
	if d.Done != 2500123 || d.Total != 10000492 || d.Percent() != 25 || math.Abs(d.BytesPerSecond()-1250061.5) > .001 {
		t.Fatal(d, d.BytesPerSecond())
	}
	eta, ok := d.Remaining()
	if !ok || eta != 6*time.Second {
		t.Fatal(eta, ok)
	}
	// No bytes arrive for two seconds: the measured average halves and ETA grows.
	stalled := ReadDownload(wire, 4*time.Second)
	if stalled.BytesPerSecond() != d.BytesPerSecond()/2 {
		t.Fatal("stalled speed did not fall")
	}
	if eta, ok = stalled.Remaining(); !ok || eta != 12*time.Second {
		t.Fatal(eta, ok)
	}
}
func TestUnknownAndWarmupDoNotInventEstimates(t *testing.T) {
	for _, d := range []Download{{Done: 100, Total: 0, Elapsed: time.Second}, {Done: 0, Total: 100, Elapsed: time.Second}, {Done: 50, Total: 100, Elapsed: time.Millisecond}} {
		if _, ok := d.Remaining(); ok {
			t.Fatal("invented ETA", d)
		}
	}
	if ReadDownload(DownloadWire(123, 0), time.Second).Percent() != -1 {
		t.Fatal("invented percent")
	}
	completed := ReadDownload(DownloadWire(100, 100), time.Second)
	if eta, ok := completed.Remaining(); !ok || eta != 0 {
		t.Fatal(eta, ok)
	}
	bounded := ReadDownload(DownloadWire(-1, 1<<40), -time.Second)
	if bounded.Done != 0 || bounded.Total != 64<<20 || bounded.Elapsed != 0 {
		t.Fatal(bounded)
	}
}

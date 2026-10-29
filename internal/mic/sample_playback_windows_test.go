//go:build windows

package mic

import "testing"

func TestSamplePlaybackHasNoInternalBoundaryDips(t *testing.T) {
	pcm := make([]int16, 3*48000)
	for i := range pcm {
		pcm[i] = 8192
	}
	shapeSamplePlayback(pcm, 50)
	if pcm[0] != 0 || pcm[len(pcm)-1] != 0 {
		t.Fatal("preview edges were not faded")
	}
	// All interior points, including every former 10ms refill boundary, must
	// retain exactly the requested level with no repeated ramp or silent gap.
	for i := 2400; i < len(pcm)-480; i++ {
		if pcm[i] != 4096 {
			t.Fatalf("internal dip at sample %d: %d", i, pcm[i])
		}
	}
}

func TestSamplePlaybackPreservesOrderAndDuration(t *testing.T) {
	pcm := make([]int16, 3*48000)
	for i := range pcm {
		pcm[i] = int16(i%16000 - 8000)
	}
	want := append([]int16(nil), pcm...)
	shapeSamplePlayback(pcm, 100)
	for i := 2400; i < len(pcm)-480; i++ {
		if pcm[i] != want[i] {
			t.Fatalf("sample moved/duplicated at %d", i)
		}
	}
	if len(pcm) != 144000 {
		t.Fatal("preview length changed")
	}
}

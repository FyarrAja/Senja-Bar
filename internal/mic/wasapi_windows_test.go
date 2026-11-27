//go:build windows

package mic

import (
	"math"
	"testing"
	"unsafe"
)

func TestInstalledCableOpensInSharedMode(t *testing.T) {
	if _, err := routeOutputFrom(Devices(false)); err != nil {
		t.Skip(err)
	}
	renderer, err := openCableRenderer()
	if err != nil {
		t.Fatalf("shared cable route cannot start: %v", err)
	}
	defer renderer.Close()
	if renderer.channels != 2 || renderer.rate != 48000 {
		t.Fatalf("unexpected cable mix format: %d channels at %d Hz", renderer.channels, renderer.rate)
	}
	for packet := 0; packet < 20; packet++ {
		if err := renderer.Write(make([]int16, 480)); err != nil {
			t.Fatalf("shared cable rejected silent packet %d: %v", packet, err)
		}
	}
}

func TestMonoPCMConvertsToStereoFloatMix(t *testing.T) {
	pcm := []int16{16384, -16384}
	var frames [4]float32
	if err := fillCableFrames(unsafe.Pointer(&frames[0]), pcm, 2, true); err != nil {
		t.Fatal(err)
	}
	for i, want := range []float32{.5, .5, -.5, -.5} {
		if math.Abs(float64(frames[i]-want)) > .0001 {
			t.Fatalf("frame %d = %g, want %g", i, frames[i], want)
		}
	}
}

func TestCableRoutePrimesBeforePlayback(t *testing.T) {
	if _, err := routeOutputFrom(Devices(false)); err != nil {
		t.Skip(err)
	}
	renderer, err := openCableRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	if renderer.started {
		t.Fatal("cable started with an empty buffer")
	}
	for packet := 0; packet < 4; packet++ {
		if err := renderer.Write(make([]int16, 480)); err != nil {
			t.Fatal(err)
		}
		if renderer.started {
			t.Fatalf("cable started after only %d ms of audio", (packet+1)*10)
		}
	}
	if err := renderer.Write(make([]int16, 480)); err != nil {
		t.Fatal(err)
	}
	if !renderer.started {
		t.Fatal("cable did not start after 50 ms of audio")
	}
	var padding uint32
	if err := audioCOM(renderer.client, 6, uintptr(unsafe.Pointer(&padding))); err != nil {
		t.Fatal(err)
	}
	t.Logf("remaining playback padding after start: %d frames", padding)
}

//go:build windows

package mic

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// The entire preview already exists in RAM. Submit it once, so neither Go
// scheduling nor per-10ms refills can interrupt or reorder playback.
func (s *Session) playSample(out, event uintptr, pcm []int16) error {
	if len(pcm) == 0 {
		return nil
	}
	shapeSamplePlayback(pcm, s.monitorVolume.Load())
	h := header{Data: uintptr(unsafe.Pointer(&pcm[0])), Length: uint32(len(pcm) * 2)}
	var pin runtime.Pinner
	pin.Pin(&pcm[0])
	pin.Pin(&h)
	defer pin.Unpin()
	if err := call("waveOutPrepareHeader", out, uintptr(unsafe.Pointer(&h)), unsafe.Sizeof(h)); err != nil {
		return err
	}
	defer func() {
		mm.NewProc("waveOutReset").Call(out)
		mm.NewProc("waveOutUnprepareHeader").Call(out, uintptr(unsafe.Pointer(&h)), unsafe.Sizeof(h))
		runtime.KeepAlive(pcm)
	}()
	if err := call("waveOutWrite", out, uintptr(unsafe.Pointer(&h)), unsafe.Sizeof(h)); err != nil {
		return err
	}
	s.playbackSubmissions.Add(1)
	kernel := syscall.NewLazyDLL("kernel32.dll")
	wait := kernel.NewProc("WaitForSingleObject")
	reset := kernel.NewProc("ResetEvent")
	deadline := time.Now().Add(6 * time.Second)
	for atomic.LoadUint32(&h.Flags)&1 == 0 {
		select {
		case <-s.stop:
			return nil
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("headset tidak menyelesaikan contoh audio")
		}
		if result, _, err := wait.Call(event, 10); result == 0xffffffff {
			return fmt.Errorf("sample playback event: %w", err)
		}
		reset.Call(event)
	}
	return nil
}

func shapeSamplePlayback(pcm []int16, volume int32) {
	gain := float64(max(0, min(100, volume))) / 100
	for i := range pcm {
		// Only the beginning/end fade; no gain resets at buffer boundaries.
		edge := min(1.0, min(float64(i)/2400, float64(len(pcm)-1-i)/480))
		pcm[i] = int16(float64(pcm[i]) * gain * edge)
	}
}

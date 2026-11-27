//go:build windows

package mic

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var audioPriority = syscall.NewLazyDLL("avrt.dll")

// MMCSS applies to the current OS thread, so the capture/render loop must
// remain on that thread until its task registration has been reverted.
func beginAudioThread() (func(), error) {
	runtime.LockOSThread()
	name, _ := syscall.UTF16PtrFromString("Audio")
	var index uint32
	handle, _, callErr := audioPriority.NewProc("AvSetMmThreadCharacteristicsW").Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&index)))
	if handle == 0 {
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("MMCSS Audio: %w", callErr)
	}
	return func() {
		audioPriority.NewProc("AvRevertMmThreadCharacteristics").Call(handle)
		runtime.UnlockOSThread()
	}, nil
}

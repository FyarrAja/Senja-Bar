//go:build windows

package mic

import (
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var audioOLE = windows.NewLazySystemDLL("ole32.dll")

type cableRenderer struct {
	client, render uintptr
	frames         uint32
	rate           uint32
	channels       int
	float          bool
}

func audioGUID(value string) windows.GUID {
	id, err := windows.GUIDFromString(value)
	if err != nil {
		panic(err)
	}
	return id
}

func audioCOM(p uintptr, method int, args ...uintptr) error {
	if p == 0 {
		return fmt.Errorf("perangkat audio tidak tersedia")
	}
	fn := *(*uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(p)) + uintptr(method)*unsafe.Sizeof(p)))
	callArgs := append([]uintptr{p}, args...)
	result, _, _ := syscall.SyscallN(fn, callArgs...)
	if int32(result) < 0 {
		return syscall.Errno(uint32(result))
	}
	return nil
}

func audioRelease(p uintptr) {
	if p != 0 {
		_ = audioCOM(p, 2)
	}
}

func cableEndpoint() (uintptr, error) {
	cls := audioGUID("{BCDE0395-E52F-467C-8E3D-C4579291692E}")
	iid := audioGUID("{A95664D2-9614-4F35-A746-DE8DB63617E6}")
	var enumerator uintptr
	result, _, _ := audioOLE.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&cls)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&enumerator)))
	if int32(result) < 0 {
		return 0, syscall.Errno(uint32(result))
	}
	defer audioRelease(enumerator)
	var devices uintptr
	if err := audioCOM(enumerator, 3, 0, 1, uintptr(unsafe.Pointer(&devices))); err != nil {
		return 0, err
	}
	defer audioRelease(devices)
	var count uint32
	if err := audioCOM(devices, 3, uintptr(unsafe.Pointer(&count))); err != nil {
		return 0, err
	}
	var selected uintptr
	for i := uint32(0); i < count; i++ {
		var device uintptr
		if err := audioCOM(devices, 4, uintptr(i), uintptr(unsafe.Pointer(&device))); err != nil {
			audioRelease(selected)
			return 0, err
		}
		name, err := endpointFriendlyName(device)
		if err != nil {
			audioRelease(device)
			audioRelease(selected)
			return 0, err
		}
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "cable input (") && strings.Contains(lower, "vb-audio") {
			if selected != 0 {
				audioRelease(device)
				audioRelease(selected)
				return 0, fmt.Errorf("lebih dari satu CABLE Input aktif")
			}
			selected = device
		} else {
			audioRelease(device)
		}
	}
	if selected == 0 {
		return 0, fmt.Errorf("CABLE Input VB-Audio tidak ditemukan")
	}
	return selected, nil
}

func endpointFriendlyName(device uintptr) (string, error) {
	var store uintptr
	if err := audioCOM(device, 4, 0, uintptr(unsafe.Pointer(&store))); err != nil {
		return "", err
	}
	defer audioRelease(store)
	key := struct {
		fmtid windows.GUID
		pid   uint32
	}{audioGUID("{A45C254E-DF1C-4EFD-8020-67D146A850E0}"), 14}
	var value [24]byte
	if err := audioCOM(store, 5, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value))); err != nil {
		return "", err
	}
	defer audioOLE.NewProc("PropVariantClear").Call(uintptr(unsafe.Pointer(&value)))
	if *(*uint16)(unsafe.Pointer(&value[0])) != 31 {
		return "", fmt.Errorf("nama endpoint audio bukan string")
	}
	ptr := *(*uintptr)(unsafe.Pointer(&value[8]))
	if ptr == 0 {
		return "", fmt.Errorf("nama endpoint audio kosong")
	}
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(ptr))), nil
}

func openCableRenderer() (_ *cableRenderer, err error) {
	runtime.LockOSThread()
	initialized := false
	var renderer *cableRenderer
	defer func() {
		if err != nil {
			if renderer != nil {
				audioRelease(renderer.render)
				audioRelease(renderer.client)
			}
			if initialized {
				audioOLE.NewProc("CoUninitialize").Call()
			}
			runtime.UnlockOSThread()
		}
	}()
	result, _, _ := audioOLE.NewProc("CoInitializeEx").Call(0, 0)
	if int32(result) < 0 {
		return nil, syscall.Errno(uint32(result))
	}
	initialized = true
	device, err := cableEndpoint()
	if err != nil {
		return nil, err
	}
	defer audioRelease(device)
	clientID := audioGUID("{1CB9AD4C-DBFA-4C32-B178-C2F568A703B2}")
	renderer = &cableRenderer{}
	if err = audioCOM(device, 3, uintptr(unsafe.Pointer(&clientID)), 1, 0, uintptr(unsafe.Pointer(&renderer.client))); err != nil {
		return nil, err
	}
	var mix uintptr
	if err = audioCOM(renderer.client, 8, uintptr(unsafe.Pointer(&mix))); err != nil {
		return nil, err
	}
	defer audioOLE.NewProc("CoTaskMemFree").Call(mix)
	f := (*format)(unsafe.Pointer(mix))
	renderer.rate, renderer.channels = f.Rate, int(f.Channels)
	renderer.float = f.Tag == 3
	if f.Tag == 0xfffe && f.Extra >= 22 {
		subtype := *(*windows.GUID)(unsafe.Pointer(mix + 24))
		renderer.float = subtype == audioGUID("{00000003-0000-0010-8000-00AA00389B71}")
	}
	if f.Rate != 48000 || f.Channels < 1 || f.Channels > 2 || f.Bits != 32 || f.Align != f.Channels*4 || !renderer.float {
		return nil, fmt.Errorf("format VB-CABLE tidak didukung: %d Hz, %d kanal, %d bit", f.Rate, f.Channels, f.Bits)
	}
	if err = audioCOM(renderer.client, 3, 0, 0, 1000000, 0, mix, 0); err != nil {
		return nil, fmt.Errorf("WASAPI shared VB-CABLE: %w", err)
	}
	if err = audioCOM(renderer.client, 4, uintptr(unsafe.Pointer(&renderer.frames))); err != nil {
		return nil, err
	}
	renderID := audioGUID("{F294ACFC-3146-4483-A7BF-ADDCA7C260E2}")
	if err = audioCOM(renderer.client, 14, uintptr(unsafe.Pointer(&renderID)), uintptr(unsafe.Pointer(&renderer.render))); err != nil {
		return nil, err
	}
	if err = audioCOM(renderer.client, 10); err != nil {
		return nil, err
	}
	return renderer, nil
}

func fillCableFrames(dst unsafe.Pointer, pcm []int16, channels int, floatOutput bool) error {
	if channels < 1 || channels > 2 || !floatOutput {
		return fmt.Errorf("format keluaran VB-CABLE tidak didukung")
	}
	frames := unsafe.Slice((*float32)(dst), len(pcm)*channels)
	for i, sample := range pcm {
		value := float32(sample) / 32768
		for channel := 0; channel < channels; channel++ {
			frames[i*channels+channel] = value
		}
	}
	return nil
}

func (r *cableRenderer) Write(pcm []int16) error {
	if len(pcm) == 0 {
		return nil
	}
	if uint32(len(pcm)) > r.frames {
		return fmt.Errorf("blok audio melebihi buffer VB-CABLE")
	}
	deadline := time.Now().Add(80 * time.Millisecond)
	for {
		var padding uint32
		if err := audioCOM(r.client, 6, uintptr(unsafe.Pointer(&padding))); err != nil {
			return err
		}
		if padding <= r.frames && r.frames-padding >= uint32(len(pcm)) {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("VB-CABLE tidak menerima audio selama 80 ms")
		}
		time.Sleep(time.Millisecond)
	}
	var dst uintptr
	if err := audioCOM(r.render, 3, uintptr(len(pcm)), uintptr(unsafe.Pointer(&dst))); err != nil {
		return err
	}
	if err := fillCableFrames(unsafe.Pointer(dst), pcm, r.channels, r.float); err != nil {
		_ = audioCOM(r.render, 4, uintptr(len(pcm)), 2)
		return err
	}
	return audioCOM(r.render, 4, uintptr(len(pcm)), 0)
}

func (r *cableRenderer) Close() {
	if r == nil {
		return
	}
	_ = audioCOM(r.client, 11)
	audioRelease(r.render)
	audioRelease(r.client)
	audioOLE.NewProc("CoUninitialize").Call()
	runtime.UnlockOSThread()
}

//go:build windows

package mic

import (
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// Only level telemetry crosses this mapping. Audio samples never leave the worker.
type meterData struct {
	Version, Sequence, Active, Level uint32
	Updated                          int64
	Kind                             uint32
}
type levelMeter struct{ handle, view uintptr }

var meterKernel = syscall.NewLazyDLL("kernel32.dll")
var meterReader struct {
	sync.Mutex
	meter levelMeter
}

func openMeter(write bool) levelMeter {
	name, _ := syscall.UTF16PtrFromString("Local\\Senja.Mic.Level.v2")
	var h uintptr
	access := uintptr(4)
	if write {
		h, _, _ = meterKernel.NewProc("CreateFileMappingW").Call(^uintptr(0), 0, 4, 0, 64, uintptr(unsafe.Pointer(name)))
		access = 6
	} else {
		h, _, _ = meterKernel.NewProc("OpenFileMappingW").Call(access, 0, uintptr(unsafe.Pointer(name)))
	}
	if h == 0 {
		return levelMeter{}
	}
	view, _, _ := meterKernel.NewProc("MapViewOfFile").Call(h, access, 0, 0, 64)
	if view == 0 {
		meterKernel.NewProc("CloseHandle").Call(h)
		return levelMeter{}
	}
	m := levelMeter{h, view}
	if write {
		p := (*meterData)(unsafe.Pointer(view))
		atomic.StoreUint32(&p.Version, 0)
		atomic.StoreUint32(&p.Sequence, 0)
	}
	return m
}
func (m levelMeter) close() {
	if m.view != 0 {
		meterKernel.NewProc("UnmapViewOfFile").Call(m.view)
	}
	if m.handle != 0 {
		meterKernel.NewProc("CloseHandle").Call(m.handle)
	}
}
func (m levelMeter) publish(s Status) {
	if m.view == 0 {
		return
	}
	p := (*meterData)(unsafe.Pointer(m.view))
	atomic.AddUint32(&p.Sequence, 1)
	atomic.StoreUint32(&p.Version, 2)
	active := uint32(0)
	if s.Active {
		active = 1
	}
	atomic.StoreUint32(&p.Active, active)
	atomic.StoreUint32(&p.Level, uint32(max(0, min(100, s.Level))))
	atomic.StoreUint32(&p.Kind, meterMessageKind(s.Message))
	atomic.StoreInt64(&p.Updated, s.Updated.UnixNano())
	atomic.AddUint32(&p.Sequence, 1)
}
func readMeter() (Status, bool) {
	meterReader.Lock()
	defer meterReader.Unlock()
	if meterReader.meter.view == 0 {
		meterReader.meter = openMeter(false)
	}
	if meterReader.meter.view == 0 {
		return Status{}, false
	}
	p := (*meterData)(unsafe.Pointer(meterReader.meter.view))
	for i := 0; i < 3; i++ {
		seq := atomic.LoadUint32(&p.Sequence)
		if seq&1 != 0 {
			continue
		}
		version := atomic.LoadUint32(&p.Version)
		s := Status{Active: atomic.LoadUint32(&p.Active) != 0, Level: int(atomic.LoadUint32(&p.Level)), Updated: time.Unix(0, atomic.LoadInt64(&p.Updated)), Message: "Uji mic aktif · pemrosesan lokal Senja"}
		s.Message = meterKindMessage(atomic.LoadUint32(&p.Kind))
		if seq == atomic.LoadUint32(&p.Sequence) && version == 2 && s.Active && time.Since(s.Updated) < time.Second {
			return s, true
		}
	}
	return Status{}, false
}

var activeMeterMessages = []string{
	"Uji mic aktif · pemrosesan lokal Senja",
	"Dengar langsung aktif · gunakan headset",
	"Menyiapkan mic · tunggu sebentar",
	"Bicara 3 detik · contoh hanya di RAM",
	"Memutar sekali · tangkapan mic dihentikan",
}

func meterMessageKind(message string) uint32 {
	for i, text := range activeMeterMessages {
		if message == text {
			return uint32(i)
		}
	}
	return 0
}
func meterKindMessage(kind uint32) string {
	if int(kind) >= len(activeMeterMessages) {
		return activeMeterMessages[0]
	}
	return activeMeterMessages[kind]
}

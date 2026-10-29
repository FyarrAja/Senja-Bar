//go:build windows

package resources

import (
	"golang.org/x/sys/windows"
	"syscall"
	"unsafe"
)

var ole = windows.NewLazySystemDLL("ole32.dll")

func comCall(p uintptr, index int, args ...uintptr) (uintptr, error) {
	if p == 0 {
		return 0, syscall.Errno(0x80004003)
	}
	fn := *(*uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(p)) + uintptr(index)*unsafe.Sizeof(p)))
	args = append([]uintptr{p}, args...)
	r, _, _ := syscall.SyscallN(fn, args...)
	if int32(r) < 0 {
		return r, syscall.Errno(uint32(r))
	}
	return r, nil
}
func release(p uintptr) {
	if p != 0 {
		comCall(p, 2)
	}
}
func guid(s string) windows.GUID {
	g, err := windows.GUIDFromString(s)
	if err != nil {
		panic(err)
	}
	return g
}

// All sessions, including inactive sessions, are protected. Audio apps retain
// their reservation between tracks; an enumeration failure blocks all changes.
// Caller stays on its COM-initialized OS thread for the entire enumeration.
func audioProcesses() (map[uint32]bool, error) {
	cls := guid("{BCDE0395-E52F-467C-8E3D-C4579291692E}")
	iid := guid("{A95664D2-9614-4F35-A746-DE8DB63617E6}")
	mgrID := guid("{77AA99A0-1BD6-484F-8BC7-2C654C9A9B6F}")
	ctlID := guid("{BFB7FF88-7239-4FC9-8FA2-07C950BE9C6D}")
	var enumerator uintptr
	r, _, _ := ole.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&cls)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&enumerator)))
	if int32(r) < 0 {
		return nil, syscall.Errno(uint32(r))
	}
	defer release(enumerator)
	var devices uintptr
	// eAll, DEVICE_STATE_ACTIVE: rendering AND capture endpoints.
	if _, err := comCall(enumerator, 3, 2, 1, uintptr(unsafe.Pointer(&devices))); err != nil {
		return nil, err
	}
	defer release(devices)
	var count uint32
	if _, err := comCall(devices, 3, uintptr(unsafe.Pointer(&count))); err != nil {
		return nil, err
	}
	result := map[uint32]bool{}
	for i := uint32(0); i < count; i++ {
		if err := audioDeviceSessions(devices, i, mgrID, ctlID, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func audioDeviceSessions(devices uintptr, index uint32, mgrID, ctlID windows.GUID, result map[uint32]bool) error {
	var device, manager, sessions uintptr
	if _, err := comCall(devices, 4, uintptr(index), uintptr(unsafe.Pointer(&device))); err != nil {
		return err
	}
	defer release(device)
	if _, err := comCall(device, 3, uintptr(unsafe.Pointer(&mgrID)), 1, 0, uintptr(unsafe.Pointer(&manager))); err != nil {
		return err
	}
	defer release(manager)
	if _, err := comCall(manager, 5, uintptr(unsafe.Pointer(&sessions))); err != nil {
		return err
	}
	defer release(sessions)
	var count uint32
	if _, err := comCall(sessions, 3, uintptr(unsafe.Pointer(&count))); err != nil {
		return err
	}
	for j := uint32(0); j < count; j++ {
		var control, control2 uintptr
		if _, err := comCall(sessions, 4, uintptr(j), uintptr(unsafe.Pointer(&control))); err != nil {
			return err
		}
		var state uint32
		if _, err := comCall(control, 3, uintptr(unsafe.Pointer(&state))); err != nil {
			release(control)
			return err
		}
		if state != 1 {
			release(control)
			continue
		}
		_, err := comCall(control, 0, uintptr(unsafe.Pointer(&ctlID)), uintptr(unsafe.Pointer(&control2)))
		release(control)
		if err != nil {
			return err
		}
		var pid uint32
		_, err = comCall(control2, 14, uintptr(unsafe.Pointer(&pid)))
		release(control2)
		if err != nil {
			return err
		}
		if pid != 0 {
			result[pid] = true
		}
	}
	return nil
}

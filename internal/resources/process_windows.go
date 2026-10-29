//go:build windows

package resources

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

var k32 = windows.NewLazySystemDLL("kernel32.dll")
var u32 = windows.NewLazySystemDLL("user32.dll")

type memoryCounters struct {
	Size, Faults                                                                                       uint32
	Peak, WorkingSet, PoolPeakPaged, PoolPaged, PoolPeakNonPaged, PoolNonPaged, Pagefile, PeakPagefile uintptr
}

func memory(h windows.Handle) uint64 {
	m := memoryCounters{}
	m.Size = uint32(unsafe.Sizeof(m))
	r, _, _ := k32.NewProc("K32GetProcessMemoryInfo").Call(uintptr(h), uintptr(unsafe.Pointer(&m)), uintptr(m.Size))
	if r == 0 {
		return 0
	}
	return uint64(m.WorkingSet)
}
func processData(h windows.Handle, p *Candidate) bool {
	var c, e, k, u windows.Filetime
	if windows.GetProcessTimes(h, &c, &e, &k, &u) != nil {
		return false
	}
	p.Created = uint64(c.HighDateTime)<<32 | uint64(c.LowDateTime)
	p.CPU = (uint64(k.HighDateTime)<<32 | uint64(k.LowDateTime)) + (uint64(u.HighDateTime)<<32 | uint64(u.LowDateTime))
	p.WorkingSet = memory(h)
	return true
}
func sameUser(h windows.Handle, sid string) bool {
	var token windows.Token
	if windows.OpenProcessToken(h, windows.TOKEN_QUERY, &token) != nil {
		return false
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return false
	}
	return user.User.Sid.String() == sid
}
func userSID() string {
	var t windows.Token
	if windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &t) != nil {
		return ""
	}
	defer t.Close()
	u, e := t.GetTokenUser()
	if e != nil {
		return ""
	}
	return u.User.Sid.String()
}

// Snapshot only includes same-user, same-session processes outside Windows.
// Unqueryable/protected processes are absent rather than retried with elevation.
func Snapshot() ([]Candidate, error) {
	snapshotMu.Lock()
	defer snapshotMu.Unlock()
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snap)
	sid := userSID()
	if sid == "" {
		return nil, fmt.Errorf("akun Windows tidak dapat diperiksa")
	}
	var session uint32
	if err = windows.ProcessIdToSessionId(uint32(os.Getpid()), &session); err != nil {
		return nil, err
	}
	all := []Candidate{}
	e := windows.ProcessEntry32{}
	e.Size = uint32(unsafe.Sizeof(e))
	err = windows.Process32First(snap, &e)
	for err == nil {
		var targetSession uint32
		if e.ProcessID != uint32(os.Getpid()) && windows.ProcessIdToSessionId(e.ProcessID, &targetSession) == nil && targetSession == session {
			h, openErr := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, e.ProcessID)
			if openErr == nil {
				buf := make([]uint16, 32768)
				size := uint32(len(buf))
				query, _, _ := k32.NewProc("QueryFullProcessImageNameW").Call(uintptr(h), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
				if sameUser(h, sid) && query != 0 {
					path := windows.UTF16ToString(buf[:size])
					windir := strings.ToLower(filepath.Clean(os.Getenv("WINDIR"))) + string(filepath.Separator)
					if !strings.HasPrefix(strings.ToLower(path), windir) {
						p := Candidate{PID: e.ProcessID, Parent: e.ParentProcessID, Path: path, Name: filepath.Base(path), Minimized: true}
						if processData(h, &p) {
							all = append(all, p)
						}
					}
				}
				windows.CloseHandle(h)
			}
		}
		err = windows.Process32Next(snap, &e)
	}
	if err != windows.ERROR_NO_MORE_FILES {
		return nil, err
	}
	windowCandidates = all
	u32.NewProc("EnumWindows").Call(snapshotCallback, 0)
	all = windowCandidates
	windowCandidates = nil
	return all, nil
}

var snapshotMu sync.Mutex
var windowCandidates []Candidate
var snapshotCallback = windows.NewCallback(func(hwnd, unused uintptr) uintptr {
	var pid uint32
	u32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	i := -1
	for j, p := range windowCandidates {
		if p.PID == pid {
			i = j
			break
		}
	}
	if i < 0 {
		return 1
	}
	visible, _, _ := u32.NewProc("IsWindowVisible").Call(hwnd)
	if visible == 0 {
		return 1
	}
	title := make([]uint16, 512)
	length, _, _ := u32.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
	if length == 0 {
		return 1
	}
	windowCandidates[i].Title = windows.UTF16ToString(title)
	minimized, _, _ := u32.NewProc("IsIconic").Call(hwnd)
	if minimized == 0 {
		windowCandidates[i].Minimized = false
	}
	return 1
})

func foreground() (uint32, bool) {
	hwnd, _, _ := u32.NewProc("GetForegroundWindow").Call()
	if hwnd == 0 {
		return 0, true
	}
	var pid uint32
	u32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	var r struct{ Left, Top, Right, Bottom int32 }
	if ok, _, _ := u32.NewProc("GetWindowRect").Call(hwnd, uintptr(unsafe.Pointer(&r))); ok == 0 {
		return pid, true
	}
	monitor, _, _ := u32.NewProc("MonitorFromWindow").Call(hwnd, 2)
	m := struct {
		Size          uint32
		Monitor, Work struct{ Left, Top, Right, Bottom int32 }
		Flags         uint32
	}{}
	m.Size = uint32(unsafe.Sizeof(m))
	ok, _, _ := u32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&m)))
	fullscreen := ok != 0 && r.Left <= m.Monitor.Left && r.Top <= m.Monitor.Top && r.Right >= m.Monitor.Right && r.Bottom >= m.Monitor.Bottom
	return pid, fullscreen
}

func memoryPressure(threshold int) bool {
	m := struct {
		Size, Load uint32
		Values     [7]uint64
	}{Size: 64}
	ok, _, _ := k32.NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&m)))
	return ok != 0 && m.Load >= uint32(threshold)
}

type powerState struct{ Version, Control, State uint32 }

func readPower(h windows.Handle) (powerState, error) {
	s := powerState{Version: 1}
	r, _, e := k32.NewProc("GetProcessInformation").Call(uintptr(h), 4, uintptr(unsafe.Pointer(&s)), unsafe.Sizeof(s))
	if r == 0 {
		return s, e
	}
	return s, nil
}
func writePower(h windows.Handle, s powerState) error {
	r, _, e := k32.NewProc("SetProcessInformation").Call(uintptr(h), 4, uintptr(unsafe.Pointer(&s)), unsafe.Sizeof(s))
	if r == 0 {
		return e
	}
	return nil
}

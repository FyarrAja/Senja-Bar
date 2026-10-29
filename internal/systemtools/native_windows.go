//go:build windows

package systemtools

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")

func registryString(path, name string) string {
	p, _ := syscall.UTF16PtrFromString(path)
	n, _ := syscall.UTF16PtrFromString(name)
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, p, 0, syscall.KEY_READ, &key) != nil {
		return ""
	}
	defer syscall.RegCloseKey(key)
	var typ, size uint32
	size = 2048
	data := make([]uint16, 1024)
	if syscall.RegQueryValueEx(key, n, nil, &typ, (*byte)(unsafe.Pointer(&data[0])), &size) != nil {
		return ""
	}
	return strings.TrimSpace(syscall.UTF16ToString(data))
}

type memoryStatus struct {
	Length, Load                                                                     uint32
	TotalPhys, AvailPhys, TotalPage, AvailPage, TotalVirtual, AvailVirtual, Extended uint64
}
type powerStatus struct {
	AC, Flags, Percent, Saver byte
	Life, FullLife            uint32
}
type displayDevice struct {
	Size        uint32
	Name        [32]uint16
	Description [128]uint16
	Flags       uint32
	ID, Key     [128]uint16
}

func nativeAdapters() []string {
	iid := syscall.GUID{Data1: 0x770aae78, Data2: 0xf26f, Data3: 0x4dba, Data4: [8]byte{0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}}
	var factory uintptr
	r, _, _ := syscall.NewLazyDLL("dxgi.dll").NewProc("CreateDXGIFactory1").Call(uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&factory)))
	if int32(r) < 0 || factory == 0 {
		return nil
	}
	methods := *(*[14]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(factory))))
	defer syscall.SyscallN(methods[2], factory)
	seen := map[string]bool{}
	var names []string
	for i := 0; i < 32; i++ {
		var adapter uintptr
		hr, _, _ := syscall.SyscallN(methods[12], factory, uintptr(i), uintptr(unsafe.Pointer(&adapter)))
		if int32(hr) < 0 || adapter == 0 {
			break
		}
		v := *(*[11]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(adapter))))
		desc := struct {
			Description                             [128]uint16
			Vendor, Device, Subsystem, Revision     uint32
			VideoMemory, SystemMemory, SharedMemory uintptr
			LUID                                    [2]uint32
			Flags                                   uint32
		}{}
		hr, _, _ = syscall.SyscallN(v[10], adapter, uintptr(unsafe.Pointer(&desc)))
		syscall.SyscallN(v[2], adapter)
		name := syscall.UTF16ToString(desc.Description[:])
		if int32(hr) >= 0 && desc.Flags&2 == 0 && name != "" && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	return names
}

var cpuSample struct {
	sync.Mutex
	idle, kernel, user uint64
	valid              bool
}

func cpuTimes() (uint64, uint64, uint64, bool) {
	var i, k, u uint64
	r, _, _ := kernel.NewProc("GetSystemTimes").Call(uintptr(unsafe.Pointer(&i)), uintptr(unsafe.Pointer(&k)), uintptr(unsafe.Pointer(&u)))
	return i, k, u, r != 0
}
func nativeCPU(ctx context.Context) (float64, error) {
	cpuSample.Lock()
	defer cpuSample.Unlock()
	i, k, u, ok := cpuTimes()
	if !ok {
		return 0, fmt.Errorf("GetSystemTimes tidak tersedia")
	}
	if !cpuSample.valid {
		cpuSample.idle, cpuSample.kernel, cpuSample.user = i, k, u
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
		i, k, u, ok = cpuTimes()
		if !ok {
			return 0, fmt.Errorf("GetSystemTimes gagal")
		}
	}
	total := (k - cpuSample.kernel) + (u - cpuSample.user)
	idle := i - cpuSample.idle
	cpuSample.idle, cpuSample.kernel, cpuSample.user, cpuSample.valid = i, k, u, true
	if total == 0 {
		return 0, nil
	}
	return max(0., min(100., 100*(1-float64(idle)/float64(total)))), nil
}
func nativeRead(ctx context.Context) Snapshot {
	s := Snapshot{Updated: time.Now(), PowerPlan: "Tidak tersedia", Battery: "Tidak tersedia"}
	s.Model = registryString(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemProductName")
	s.CPU = registryString(`HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString")
	var m memoryStatus
	m.Length = uint32(unsafe.Sizeof(m))
	if r, _, e := kernel.NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&m))); r == 0 {
		s.Error = "RAM: " + e.Error()
		return s
	}
	s.RAMTotalGB = float64(m.TotalPhys) / (1 << 30)
	s.RAMFreeGB = float64(m.AvailPhys) / (1 << 30)
	load, err := nativeCPU(ctx)
	if err != nil {
		s.Error = err.Error()
		return s
	}
	s.CPULoad = load
	gpu := nativeAdapters()
	seen := map[string]bool{}
	user := syscall.NewLazyDLL("user32.dll")
	for i := 0; len(gpu) == 0 && i < 32; i++ {
		d := displayDevice{}
		d.Size = uint32(unsafe.Sizeof(d))
		r, _, _ := user.NewProc("EnumDisplayDevicesW").Call(0, uintptr(i), uintptr(unsafe.Pointer(&d)), 0)
		if r == 0 {
			break
		}
		name := syscall.UTF16ToString(d.Description[:])
		if d.Flags&8 == 0 && name != "" && !seen[name] {
			gpu = append(gpu, name)
			seen[name] = true
		}
	}
	s.GPU = strings.Join(gpu, ", ")
	for letter := 'C'; letter <= 'Z'; letter++ {
		path := string(letter) + `:\`
		p, _ := syscall.UTF16PtrFromString(path)
		typ, _, _ := kernel.NewProc("GetDriveTypeW").Call(uintptr(unsafe.Pointer(p)))
		if typ != 3 {
			continue
		}
		var free, total, available uint64
		r, _, _ := kernel.NewProc("GetDiskFreeSpaceExW").Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&available)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&free)))
		if r != 0 {
			s.Volumes = append(s.Volumes, Volume{Name: string(letter) + ":", FreeGB: float64(available) / (1 << 30), TotalGB: float64(total) / (1 << 30)})
		}
	}
	var battery powerStatus
	if r, _, _ := kernel.NewProc("GetSystemPowerStatus").Call(uintptr(unsafe.Pointer(&battery))); r != 0 && battery.Percent != 255 && battery.Flags&128 == 0 {
		s.Battery = fmt.Sprintf("%d%%", battery.Percent)
		if battery.AC == 1 {
			s.Battery += " · terhubung daya"
		}
	}
	pow := syscall.NewLazyDLL("powrprof.dll")
	var guid uintptr
	if r, _, _ := pow.NewProc("PowerGetActiveScheme").Call(0, uintptr(unsafe.Pointer(&guid))); r == 0 && guid != 0 {
		defer kernel.NewProc("LocalFree").Call(guid)
		var length uint32 = 512
		name := make([]uint16, 256)
		if r, _, _ := pow.NewProc("PowerReadFriendlyName").Call(0, guid, 0, 0, uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&length))); r == 0 {
			s.PowerPlan = syscall.UTF16ToString(name)
		}
	}
	return s
}
func nativeNetwork(ctx context.Context) (string, error) {
	var lines []string
	dll := syscall.NewLazyDLL("iphlpapi.dll")
	h, _, err := dll.NewProc("IcmpCreateFile").Call()
	if h == 0 || h == ^uintptr(0) {
		return "", fmt.Errorf("ICMP: %v", err)
	}
	defer dll.NewProc("IcmpCloseHandle").Call(h)
	for _, host := range []string{"1.1.1.1", "8.8.8.8"} {
		ip := net.ParseIP(host).To4()
		var sum, count uint32
		for i := 0; i < 3; i++ {
			if err := ctx.Err(); err != nil {
				return strings.Join(lines, "\n"), err
			}
			payload := []byte("Senja diagnostic")
			reply := make([]byte, 512)
			r, _, _ := dll.NewProc("IcmpSendEcho").Call(h, uintptr(binary.LittleEndian.Uint32(ip)), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0, uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), 1000)
			if r != 0 && binary.LittleEndian.Uint32(reply[4:]) == 0 {
				sum += binary.LittleEndian.Uint32(reply[8:])
				count++
			}
		}
		if count > 0 {
			lines = append(lines, fmt.Sprintf("%s  ·  %.1f ms  ·  %d/3 respons", host, float64(sum)/float64(count), count))
		} else {
			lines = append(lines, host+"  ·  tidak merespons ICMP")
		}
	}
	addresses, e := net.DefaultResolver.LookupIPAddr(ctx, "example.com")
	if e != nil {
		lines = append(lines, "DNS: gagal · "+e.Error())
	} else {
		lines = append(lines, fmt.Sprintf("DNS: %d alamat berhasil di-resolve", len(addresses)))
	}
	return strings.Join(lines, "\n"), nil
}
func nativeDiagnostics(ctx context.Context) (string, error) {
	s := nativeRead(ctx)
	if s.Error != "" {
		return "", fmt.Errorf("%s", s.Error)
	}
	lines := []string{"DIAGNOSIS SENJA", time.Now().Format("02 Jan 2006 · 15:04"), "", s.Model, s.CPU, s.GPU, "", fmt.Sprintf("RAM tersedia: %.1f / %.1f GB", s.RAMFreeGB, s.RAMTotalGB), fmt.Sprintf("CPU: %.0f%%", s.CPULoad), "Daya: " + s.PowerPlan}
	for _, v := range s.Volumes {
		state := "cukup"
		if v.TotalGB > 0 && v.FreeGB/v.TotalGB < .1 {
			state = "ruang hampir penuh"
		}
		lines = append(lines, fmt.Sprintf("%s %.1f GB kosong · %s", v.Name, v.FreeGB, state))
	}
	lines = append(lines, "", "Cakupan: penggunaan CPU/RAM, kapasitas disk, daya.", "Kesehatan fisik disk dan integritas Windows belum diperiksa.")
	return strings.Join(lines, "\n"), nil
}

func nativeStorage(ctx context.Context) (string, error) {
	s := nativeRead(ctx)
	if s.Error != "" {
		return "", fmt.Errorf("%s", s.Error)
	}
	lines := []string{"PENYIMPANAN · SENJA", ""}
	for _, v := range s.Volumes {
		lines = append(lines, fmt.Sprintf("%s %.1f GB kosong dari %.1f GB", v.Name, v.FreeGB, v.TotalGB))
	}
	var count, bytes, skipped uint64
	cutoff := time.Now().Add(-48 * time.Hour)
	root := os.TempDir()
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if path == root {
				return err
			}
			skipped++
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil && info.ModTime().Before(cutoff) && info.Mode().IsRegular() {
				count++
				bytes += uint64(info.Size())
			}
		}
		return nil
	})
	if walkErr != nil {
		return strings.Join(lines, "\n"), walkErr
	}
	lines = append(lines, "", fmt.Sprintf("Temp pengguna lebih dari 48 jam: %d file, %.1f MB", count, float64(bytes)/(1<<20)), "Ini perkiraan ukuran, bukan daftar aman untuk dihapus.", "Tidak ada file yang diubah atau dihapus.")
	if skipped > 0 {
		lines = append(lines, fmt.Sprintf("%d lokasi tidak dapat dibaca; hasil pindai parsial.", skipped))
	}
	return strings.Join(lines, "\n"), nil
}
func nativePower(key string) error {
	ids := map[string]string{"balanced": "381b4222-f694-41f0-9685-ff5bb260df2e", "saving": "a1841308-3541-4fab-bc81-f71556f20b4a", "performance": "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"}
	id, ok := ids[key]
	if !ok {
		return fmt.Errorf("profil daya tidak dikenal")
	}
	text, _ := syscall.UTF16PtrFromString(id)
	var guid [16]byte
	if r, _, _ := syscall.NewLazyDLL("ole32.dll").NewProc("CLSIDFromString").Call(uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(&guid[0]))); int32(r) < 0 {
		return fmt.Errorf("ID profil tidak valid")
	}
	if r, _, _ := syscall.NewLazyDLL("powrprof.dll").NewProc("PowerSetActiveScheme").Call(0, uintptr(unsafe.Pointer(&guid[0]))); r != 0 {
		return fmt.Errorf("Windows tidak menyediakan profil ini atau menolak perubahan (%d)", r)
	}
	return nil
}

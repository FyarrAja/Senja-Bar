//go:build windows

package resources

import (
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Status struct {
	Message        string    `json:"message"`
	Trimmed        int       `json:"trimmed"`
	WorkingSetDrop uint64    `json:"workingSetDrop"`
	EcoProcesses   int       `json:"ecoProcesses"`
	Failed         int       `json:"failed"`
	Skipped        int       `json:"skipped"`
	Updated        time.Time `json:"updated"`
}

func StatusPath() string { return filepath.Join(filepath.Dir(Path()), "resources-status.json") }
func ReadStatus() Status {
	var s Status
	b, _ := os.ReadFile(StatusPath())
	_ = json.Unmarshal(b, &s)
	return s
}
func (s Status) save() {
	s.Updated = time.Now()
	b, _ := json.Marshal(s)
	p := StatusPath() + ".tmp"
	if os.WriteFile(p, b, 0600) == nil {
		_ = os.Rename(p, StatusPath())
	}
}

type observation struct {
	created, cpu                   uint64
	at, idle, background, lastTrim time.Time
}
type applied struct {
	handle   windows.Handle
	original powerState
	path     string
	created  uint64
}
type request struct {
	config Config
	manual bool
}
type Manager struct {
	requests      chan request
	wake          chan struct{}
	done, stopped chan struct{}
	once          sync.Once
}

func Start() *Manager {
	m := &Manager{requests: make(chan request, 8), wake: make(chan struct{}, 1), done: make(chan struct{}), stopped: make(chan struct{})}
	go m.run()
	m.Reload(false)
	return m
}
func (m *Manager) Reload(manual bool) bool {
	if m == nil {
		return false
	}
	select {
	case m.requests <- request{Load(), manual}:
		return true
	default:
		return false
	}
}
func (m *Manager) Wake() {
	if m != nil {
		select {
		case m.wake <- struct{}{}:
		default:
		}
	}
}
func (m *Manager) Close() {
	if m != nil {
		m.once.Do(func() { close(m.done); <-m.stopped })
	}
}

func ProtectedSnapshot() ([]Candidate, bool, bool, error) {
	all, err := Snapshot()
	if err != nil {
		return nil, false, true, err
	}
	// The worker uses a temporary MTA; never change the engine UI's COM model.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := ole.NewProc("CoInitializeEx").Call(0, 0)
	audioOK := int32(hr) >= 0
	seeds := map[uint32]bool{}
	if audioOK {
		audio, e := audioProcesses()
		audioOK = e == nil
		for pid := range audio {
			seeds[pid] = true
		}
		ole.NewProc("CoUninitialize").Call()
	}
	pid, paused := foreground()
	seeds[pid] = true
	for _, p := range all {
		if !p.Minimized {
			seeds[p.PID] = true
		}
	}
	protected := protectedFamily(all, seeds)
	for i := range all {
		if sensitive(all[i].Path) {
			all[i].Protected = "Sistem / game / multimedia"
		} else if protected[all[i].PID] {
			all[i].Protected = "Aktif / audio / proses terkait"
		} else if !all[i].Minimized {
			all[i].Protected = "Jendela masih terbuka"
		} else if !audioOK {
			all[i].Protected = "Pemeriksaan audio belum tersedia"
		}
	}
	return all, audioOK, paused, nil
}

func (m *Manager) run() {
	defer close(m.stopped)
	cfg := Config{}
	history := map[uint32]observation{}
	states := map[uint32]applied{}
	restore := func(pid uint32) bool {
		s := states[pid]
		// Only restore our owned execution-speed bit; preserve unrelated flags and
		// do not overwrite a different policy applied by the app in the meantime.
		current, e := readPower(s.handle)
		if e == nil && current.Control&1 != 0 && current.State&1 != 0 {
			current.Control = (current.Control &^ 1) | (s.original.Control & 1)
			current.State = (current.State &^ 1) | (s.original.State & 1)
			e = writePower(s.handle, current)
		}
		if e != nil {
			var code uint32
			if windows.GetExitCodeProcess(s.handle, &code) == nil && code == 259 {
				return false
			}
		}
		windows.CloseHandle(s.handle)
		delete(states, pid)
		return true
	}
	defer func() {
		for pid := range states {
			restore(pid)
			if s, ok := states[pid]; ok {
				windows.CloseHandle(s.handle)
			}
		}
	}()
	var timer *time.Ticker
	var ticks <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	scan := func(manual bool) {
		now := time.Now()
		all, audioOK, paused, err := ProtectedSnapshot()
		if err != nil {
			for pid := range states {
				restore(pid)
			}
			Status{Message: "Tidak dapat memeriksa aplikasi; optimasi dijeda."}.save()
			return
		}
		seen := map[uint32]bool{}
		allowed := map[uint32]bool{}
		status := Status{Message: fmt.Sprintf("Menunggu aplikasi pilihan diminimalkan dan idle %d menit.", int(cfg.IdleWait()/time.Minute))}
		if !audioOK {
			status.Message = "Dijeda: pemeriksaan audio tidak tersedia."
		} else if paused {
			status.Message = "Dijeda: aplikasi layar penuh sedang aktif."
		}
		pressure := memoryPressure(cfg.MemoryThreshold())
		for _, p := range all {
			seen[p.PID] = true
			old := history[p.PID]
			if old.created != p.Created {
				old = observation{created: p.Created, cpu: p.CPU, at: now}
			}
			elapsed := now.Sub(old.at)
			delta := uint64(0)
			if p.CPU >= old.cpu {
				delta = p.CPU - old.cpu
			}
			// Busy means >0.5% of one logical core. Starting/changing a workload resets
			// the entire idle delay; a process missing from a sample also resets it.
			busy := elapsed <= 0 || float64(delta)*100/float64(elapsed/(100*time.Nanosecond)) > .5
			baseOK := selected(cfg, p.Path) && audioOK && !paused && p.Protected == "" && p.Minimized && !sensitive(p.Path)
			if !baseOK || busy {
				old.idle = time.Time{}
			} else if old.idle.IsZero() {
				old.idle = now
			}
			if !baseOK {
				old.background = time.Time{}
			} else if old.background.IsZero() {
				old.background = now
			}
			duration := time.Duration(0)
			if !old.idle.IsZero() {
				duration = now.Sub(old.idle)
			}
			ramOK := eligible(p, cfg, audioOK, paused, duration) || manual && baseOK
			cpuOK := baseOK && !old.background.IsZero() && now.Sub(old.background) >= cfg.IdleWait()
			old.cpu, old.at = p.CPU, now
			history[p.PID] = old
			if !ramOK && !cpuOK {
				status.Skipped++
				continue
			}
			allowed[p.PID] = true
			if cfg.EcoCPU && cpuOK {
				if _, exists := states[p.PID]; !exists {
					h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_SET_INFORMATION|windows.SYNCHRONIZE, false, p.PID)
					if e == nil {
						verified := p
						original, powerErr := readPower(h)
						if processData(h, &verified) && verified.Created == p.Created && powerErr == nil {
							next := original
							next.Control |= 1
							next.State |= 1
							if writePower(h, next) == nil {
								states[p.PID] = applied{h, original, p.Path, p.Created}
								h = 0
							} else {
								status.Failed++
							}
						}
						if h != 0 {
							windows.CloseHandle(h)
						}
					}
				}
			}
			if ramOK && (manual || cfg.Automatic && pressure) && now.Sub(old.lastTrim) >= cfg.TrimWait() {
				h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_SET_QUOTA, false, p.PID)
				if e == nil {
					verified := p
					if processData(h, &verified) && verified.Created == p.Created {
						// Recheck foreground immediately before discarding pages.
						fg, full := foreground()
						if fg != p.PID && !full {
							before := memory(h)
							r, _, _ := k32.NewProc("K32EmptyWorkingSet").Call(uintptr(h))
							if r != 0 {
								after := memory(h)
								status.Trimmed++
								if before > after {
									status.WorkingSetDrop += before - after
								}
								old.lastTrim = now
								history[p.PID] = old
							} else {
								status.Failed++
							}
						}
					}
					windows.CloseHandle(h)
				} else {
					status.Failed++
				}
			}
		}
		for pid, s := range states {
			if !allowed[pid] || !cfg.EcoCPU {
				if !restore(pid) {
					status.Message = "Pemulihan EcoQoS belum berhasil; akan dicoba lagi."
				}
			} else {
				// Retained handles bind restoration to the original process, not a reused PID.
				_ = s
			}
		}
		for pid := range history {
			if !seen[pid] {
				delete(history, pid)
			}
		}
		status.EcoProcesses = len(states)
		if status.Trimmed > 0 {
			status.Message = fmt.Sprintf("%d proses ditrim; working set turun %.1f MB sementara.", status.Trimmed, float64(status.WorkingSetDrop)/1e6)
		} else if manual {
			if !audioOK {
				status.Message = "Trim dijeda: perangkat audio belum dapat diperiksa."
			} else if paused {
				status.Message = "Trim dijeda: aplikasi layar penuh aktif."
			} else if status.Failed > 0 {
				status.Message = fmt.Sprintf("%d proses gagal diakses/ditrim; proses aktif dilindungi.", status.Failed)
			} else {
				status.Message = fmt.Sprintf("Tidak ada target siap: minimalkan aplikasi; jeda trim %d menit.", int(cfg.TrimWait()/time.Minute))
			}
		} else if len(allowed) > 0 && !paused && audioOK {
			status.Message = fmt.Sprintf("%d proses idle siap; EcoQoS %d. Trim otomatis saat RAM ≥%d%%.", len(allowed), len(states), cfg.MemoryThreshold())
		}
		status.save()
	}
	for {
		select {
		case <-m.done:
			return
		case req := <-m.requests:
			cfg = req.config
			enabled := (cfg.AllBackground || len(cfg.Paths) > 0) && (cfg.Automatic || cfg.EcoCPU)
			// Keep observations for manual-only selections, at a low sample rate.
			observing := cfg.AllBackground || len(cfg.Paths) > 0
			if observing && timer == nil {
				timer = time.NewTicker(10 * time.Second)
				ticks = timer.C
			}
			if !observing && timer != nil {
				timer.Stop()
				timer = nil
				ticks = nil
				history = map[uint32]observation{}
			}
			if !enabled {
				for pid := range states {
					restore(pid)
				}
			}
			scan(req.manual)
		case <-ticks:
			scan(false)
		case <-m.wake:
			// A focus change removes EcoQoS immediately; wait for a fresh, complete
			// sample before applying it again. No need to enumerate every process here.
			for pid := range states {
				restore(pid)
			}
			for pid, o := range history {
				o.idle = time.Time{}
				o.background = time.Time{}
				history[pid] = o
			}
		}
	}
}

// A foreground WinEvent hook must be installed on a thread with a message pump.
func (m *Manager) WatchForeground() func() {
	callback := windows.NewCallback(func(h uintptr, event uint32, window uintptr, object, child int32, thread, time uint32) uintptr {
		m.Wake()
		return 0
	})
	h, _, _ := u32.NewProc("SetWinEventHook").Call(3, 3, 0, callback, 0, 0, 0)
	return func() {
		if h != 0 {
			u32.NewProc("UnhookWinEvent").Call(h)
		}
	}
}

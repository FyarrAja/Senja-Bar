//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"samar/internal/resources"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"
)

const cmdResources = 115
const cmdTaskbarPage = 116
const wmResources = wmApp + 75

var resourceManager *resources.Manager

func (n *nativeGlassPanel) resourceStatus() string {
	if n.preview {
		return "Mode preview; trim tidak dijalankan."
	}
	if n.resources.message != "" {
		return n.resources.message
	}
	return "Pilih aplikasi latar untuk dikelola."
}

const nativeSidebar = 112
const resourcePageSize = 4

type resourceRow struct {
	path, name, status string
	memory             uint64
	icon               uintptr
}
type resourceScan struct {
	rows []resourceRow
	err  error
}
type nativeResources struct {
	mu                      sync.Mutex
	closed                  bool
	icons                   map[string]uintptr
	config                  resources.Config
	rows                    []resourceRow
	filtered                []resourceRow
	query                   string
	caret                   int
	selectAll, caretVisible bool
	offset                  int
	loading                 bool
	loadingStarted          time.Time
	results                 chan resourceScan
	message                 string
	updated                 time.Time
	loaded                  bool
	optionOpen              uint32
	tuning                  bool
	catalog                 []resources.InstalledApp
	catalogLoaded           bool
}

func (n *nativeGlassPanel) controlAt(x, y int) int {
	if n.micPage {
		n.layoutMicControls()
	}
	if n.systemPage {
		n.layoutSystemControls()
	}
	if n.resourcePage {
		n.layoutResourceControls()
	}
	x = n.logicalPointerX(x)
	b := nativeControlBoxes[26]
	if n.resourcePage && n.controlEnabled(26) && x >= b[0] && x < b[0]+b[2] && y >= b[1] && y < b[1]+b[3] {
		return 26
	}
	for i, b := range nativeControlBoxes {
		if n.controlEnabled(i) && x >= b[0] && x < b[0]+b[2] && y >= b[1] && y < b[1]+b[3] {
			return i
		}
	}
	return -1
}
func (n *nativeGlassPanel) scanResources() {
	r := &n.resources
	if r.loading {
		return
	}
	r.loading = true
	r.loadingStarted = time.Now()
	n.syncResourceLoader()
	r.message = "Memeriksa aplikasi dan audio…"
	r.results = make(chan resourceScan, 1)
	result := r.results
	window := n.hwnd
	catalog := append([]resources.InstalledApp(nil), r.catalog...)
	catalogLoaded := r.catalogLoaded
	cached := map[string]bool{}
	for key := range r.icons {
		cached[key] = true
	}
	go func() {
		if !catalogLoaded {
			catalog = resources.InstalledApps()
		}
		all, _, paused, err := resources.ProtectedSnapshot()
		byPath := map[string]resourceRow{}
		for _, p := range all {
			key := strings.ToLower(p.Path)
			row := byPath[key]
			row.path = p.Path
			row.name = p.Name
			row.memory += p.WorkingSet
			if p.Protected != "" {
				row.status = p.Protected
			} else if row.status == "" {
				row.status = "Latar; menunggu idle"
			}
			if paused && p.Protected == "" {
				row.status = "Dijeda: layar penuh"
			}
			byPath[key] = row
		}
		for _, path := range resources.Load().Paths {
			key := strings.ToLower(path)
			if _, ok := byPath[key]; !ok {
				byPath[key] = resourceRow{path: path, name: filepath.Base(path), status: "Tidak berjalan / tidak terakses"}
			}
		}
		for _, app := range catalog {
			key := strings.ToLower(app.Path)
			if current, ok := byPath[key]; ok {
				current.name = app.Name
				byPath[key] = current
			}
		}
		rows := []resourceRow{}
		for _, r := range byPath {
			if resourceAppSelectable(r.path) {
				rows = append(rows, r)
			}
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].memory != rows[j].memory {
				return rows[i].memory > rows[j].memory
			}
			return strings.ToLower(rows[i].name) < strings.ToLower(rows[j].name)
		})
		if len(rows) > 0 {
			err = nil
		}
		for i := range rows {
			if !cached[strings.ToLower(rows[i].path)] {
				rows[i].icon = resourceFileIcon(rows[i].path)
			}
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.closed {
			releaseResourceRows(rows)
			return
		}
		r.catalog = catalog
		r.catalogLoaded = true
		result <- resourceScan{rows, err}
		postMessageW.Call(window, wmApp+76, 0, 0)
	}()
}
func resourceAppSelectable(path string) bool {
	return path != "" && !strings.HasPrefix(path, "installed:") && !strings.HasPrefix(path, "store:")
}
func resourceSelected(c resources.Config, path string) bool { return c.Selected(path) }
func toggleResourceApp(c *resources.Config, path string) {
	if !resourceAppSelectable(path) {
		return
	}
	paths := &c.Paths
	if c.AllBackground {
		paths = &c.ExcludedPaths
	}
	next := make([]string, 0, len(*paths)+1)
	found := false
	for _, p := range *paths {
		if strings.EqualFold(p, path) {
			found = true
		} else {
			next = append(next, p)
		}
	}
	if !found {
		next = append(next, path)
	}
	*paths = next
}
func (n *nativeGlassPanel) resourceAction(command uint32) bool {
	if command == 318 || command == 319 {
		n.resources.tuning = command == 318
		n.resources.optionOpen = 0
		n.focus = 36
		if !n.resources.tuning {
			n.focus = 35
			if n.hwnd != 0 && !n.resources.config.AllBackground && !n.resources.catalogLoaded && !n.resources.loading {
				n.scanResources()
			}
		}
		user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
		return true
	}
	picked := 0
	if command >= 315 && command <= 317 || command == 320 {
		n.openResourceOptions(command)
		return true
	}
	if command >= 330 && command <= 334 {
		values := resourceOptionValues(n.resources.optionOpen)
		i := int(command - 330)
		if i >= len(values) {
			return true
		}
		picked = values[i]
		command = n.resources.optionOpen
		n.resources.optionOpen = 0
		n.focus = -1
	}
	if command == 313 {
		n.focusResourceSearch()
		return true
	}
	if command == 314 {
		n.resources.setQuery("")
		n.focusResourceSearch()
		return true
	}
	if command == cmdTaskbarPage || command == cmdResources {
		n.micPage = false
		n.systemPage = false
		n.resourcePage = command == cmdResources
		n.resources.tuning = false
		n.resources.optionOpen = 0
		n.focus = -1
		if n.resourcePage {
			if !n.resources.loaded || !n.preview {
				n.resources.config = resources.Load()
				n.resources.loaded = true
			}
			if !n.resources.config.AllBackground {
				n.scanResources()
			}
			user32.NewProc("SetTimer").Call(n.hwnd, 0x5253, 1000, 0)
			n.syncResourceLoader()
		} else {
			user32.NewProc("KillTimer").Call(n.hwnd, 0x5253)
			n.syncResourceLoader()
		}
		user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
		n.animate(false)
		return true
	}
	if command < 300 || command > 320 {
		return false
	}
	r := &n.resources
	if r.loading && command != 320 {
		return true
	}
	if command == 306 {
		r.offset = max(0, r.offset-resourcePageSize)
	} else if command == 307 {
		r.offset = min(max(0, (len(r.visibleRows())-1)/resourcePageSize*resourcePageSize), r.offset+resourcePageSize)
	} else if command == 311 {
		r.catalogLoaded = false
		n.scanResources()
	} else {
		before := r.config
		before.Paths = append([]string(nil), before.Paths...)
		before.ExcludedPaths = append([]string(nil), before.ExcludedPaths...)
		switch {
		case command >= 300 && command < 306:
			i := r.offset + int(command-300)
			if i >= len(r.visibleRows()) {
				return true
			}
			path := r.visibleRows()[i].path
			toggleResourceApp(&r.config, path)
		case command == 308:
			r.config.Automatic = !r.config.Automatic
		case command == 309:
			r.config.EcoCPU = !r.config.EcoCPU
		case command == 312:
			r.config.Automatic = false
			r.config.EcoCPU = false
		case command == 315:
			r.config.MemoryPercent = picked
		case command == 316:
			r.config.IdleMinutes = picked
		case command == 317:
			r.config.CooldownMinutes = picked
		case command == 320:
			if n.preview && r.results == nil {
				r.loading = false
			}
			r.config.AllBackground = picked == 1
			if r.config.AllBackground {
				r.config.ExcludedPaths = nil
			}
			r.tuning = false
			n.focus = -1
		}
		if n.preview {
			r.message = "Pratinjau: perubahan tidak diterapkan."
		} else if err := resources.Save(r.config); err != nil {
			r.config = before
			r.message = "Pilihan gagal disimpan. Coba lagi."
		} else {
			manual := uintptr(0)
			if command == 310 {
				manual = 1
			}
			if !r.config.AllBackground && len(r.config.Paths) == 0 && manual == 1 {
				r.message = "Pilih aplikasi terlebih dahulu."
			} else {
				var ok uintptr
				if n.localResources != nil {
					if n.localResources.Reload(manual == 1) {
						ok = 1
					}
				} else {
					ok, _, _ = postMessageW.Call(n.parent, wmResources, manual, 0)
				}
				if ok == 0 {
					r.message = "Engine tidak terhubung. Jalankan build Senja terbaru."
					user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
					return true
				}
				r.updated = time.Now()
				r.message = fmt.Sprintf("Tersimpan. Minimalkan aplikasi dan tunggu idle %d menit.", int(r.config.IdleWait()/time.Minute))
				if command == 310 {
					r.message = "Memeriksa keamanan sebelum trim RAM…"
				}
				if command == 312 {
					r.message = "Mode otomatis dimatikan; memulihkan EcoQoS…"
				}
			}
		}
	}
	n.syncResourceLoader()
	if command == 320 && !r.config.AllBackground && !r.catalogLoaded && n.hwnd != 0 {
		n.scanResources()
	}
	user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
	n.animate(false)
	return true
}

func (n *nativeGlassPanel) drawSidebar(ink, muted, card uint32) {
	n.line(-1, 24, -1, 522, 0x18ffffff, 1)
	selectedRow := 0
	if n.resourcePage {
		selectedRow = 1
	}
	if n.systemPage {
		selectedRow = 2
	}
	if n.micPage {
		selectedRow = 3
	}
	n.move(&n.navMotion, float64(102+selectedRow*50))
	for j := 0; j < 4; j++ {
		n.round(-104, 102+j*50, 96, 42, 13, card, false)
	}
	n.roundFloat(-104, float32(n.navMotion.value), 96, 42, 13, 0xc42f77f4, false, 1)
	for j, label := range []string{"Taskbar", "RAM & CPU", "Sistem", "Mic"} {
		i := 10 + j
		if j == 2 {
			i = 38
			nativeControlBoxes[38] = [4]int{-104, 202, 96, 42}
		}
		if j == 3 {
			i = 48
		}
		selected := !n.resourcePage && !n.systemPage && !n.micPage && j == 0 || n.resourcePage && j == 1 || n.systemPage && j == 2 || n.micPage && j == 3
		y := 102 + j*50
		saved := n.beginPress(i)
		color := ink
		if selected {
			color = 0xffffffff
		}
		if j == 0 {
			n.round(-95, y+15, 17, 11, 3, color, true)
			n.line(-95, y+23, -78, y+23, color, 1)
		} else if j == 3 {
			n.round(-92, y+11, 9, 15, 4, color, true)
			n.line(-96, y+20, -96, y+23, color, 1)
			n.line(-96, y+23, -88, y+28, color, 1)
			n.line(-88, y+28, -79, y+23, color, 1)
			n.line(-79, y+23, -79, y+20, color, 1)
			n.line(-88, y+28, -88, y+32, color, 1)
		} else if j == 2 {
			n.round(-96, y+13, 19, 13, 3, color, true)
			n.line(-87, y+26, -87, y+30, color, 1)
			n.line(-92, y+31, -82, y+31, color, 1)
		} else {
			n.round(-95, y+13, 16, 16, 4, color, true)
			for k := 0; k < 3; k++ {
				n.line(-91+k*4, y+17, -91+k*4, y+25, color, 1)
			}
		}
		n.drawText(label, -75, y, 67, 42, 12, true, color, true)
		n.endPress(saved)
	}
	brand := uint32(0xffffc66b)
	if n.light {
		brand = 0xffba761d
	}
	n.round(-93, 30, 22, 10, 5, brand, false)
	n.line(-94, 43, -70, 43, brand, 2)
	n.line(-89, 48, -75, 48, brand, 2)
	n.text("Senja Bar", -94, 58, 91, 24, 14, true, ink)
	saved := n.beginPress(8)
	n.round(-104, 488, 96, 36, 15, 0x28ef4456, false)
	exitInk := uint32(0xffffb3bc)
	if n.light {
		exitInk = 0xffa81c35
	}
	n.powerIcon(-90, 499, exitInk)
	n.drawText("Keluar", -74, 488, 60, 36, 12, true, exitInk, true)
	n.endPress(saved)
}
func (n *nativeGlassPanel) drawResources(ink, muted, card, group uint32) {
	r := &n.resources
	n.layoutResourceControls()
	n.text("RAM & CPU", 20, 18, 318, 37, 26, true, ink)
	subtitle := "Kelola aplikasi latar dengan perlindungan aplikasi aktif"
	if n.system.loaded && n.system.snapshot.Error == "" {
		d := n.system.snapshot
		subtitle = fmt.Sprintf("RAM %.1f / %.1f GB    ·    CPU %.0f%%", d.RAMTotalGB-d.RAMFreeGB, d.RAMTotalGB, d.CPULoad)
	}
	n.text(subtitle, 20, 57, 318, 22, 12, false, muted)
	n.drawResourceTabs(ink, muted, card)
	if r.tuning {
		n.drawResourceTuning(ink, muted, card, group)
		return
	}
	n.drawResourceScope(ink, muted, card)
	if r.config.AllBackground {
		n.round(20, 213, 318, 132, 14, group, false)
		n.drawScopeIcon(true, 163, 235, muted)
		n.drawText("Semua aplikasi latar", 32, 267, 294, 24, 14, true, ink, true)
		n.drawText("Game, aplikasi aktif, dan audio tetap dilindungi.", 32, 295, 294, 30, 12, false, muted, true)
		n.drawResourceControls(ink, muted, card, group)
		return
	}
	n.drawResourceSearch(ink, muted, card)
	n.round(20, 213, 318, 132, 16, group, false)
	if !r.loading {
		for j := 0; j < resourcePageSize; j++ {
			i := r.offset + j
			if i >= len(r.visibleRows()) {
				break
			}
			row := r.visibleRows()[i]
			y := 216 + j*32
			saved := n.beginPress(12 + j)
			checked := resourceSelected(r.config, row.path)
			if checked && !r.config.AllBackground {
				n.round(23, y, 312, 32, 8, 0x302f77f4, false)
			}
			color := uint32(0x456f7c8f)
			if checked {
				color = 0xff2f77f4
			}
			enabled := n.controlEnabled(12 + j)
			if !enabled {
				color = muted&0xffffff | 0x30000000
			}
			n.round(29, y+7, 17, 17, 5, color, false)
			if checked {
				n.line(33, y+15, 36, y+18, 0xffffffff, 1.5)
				n.line(36, y+18, 42, y+11, 0xffffffff, 1.5)
			} else if !enabled {
				n.line(33, y+15, 42, y+15, muted&0xffffff|0x70000000, 1)
			}
			n.drawResourceIcon(row.path, 53, y+5, muted)
			n.drawText(row.name, 83, y, 175, 18, 12, true, ink, false, true)
			if row.memory > 0 {
				n.drawText(fmt.Sprintf("%.0f MB", float64(row.memory)/(1<<20)), 272, y, 55, 18, 10, false, muted, true)
			}
			n.drawText(row.status, 83, y+16, 244, 16, 10, false, muted, false, true)
			n.endPress(saved)
		}
	}
	if r.loading {
		var saved uint32
		nativeG("GdipSaveGraphics", n.graphics, uintptr(unsafe.Pointer(&saved)))
		nativeG("GdipTranslateWorldTransform", n.graphics, nativeF(0), nativeF(48), 0)
		n.drawResourceCubeLoader(ink)
		nativeG("GdipRestoreGraphics", n.graphics, uintptr(saved))
	} else if len(r.visibleRows()) == 0 {
		empty := "Belum ada aplikasi. Tekan Muat ulang."
		if r.query != "" {
			empty = "Aplikasi tidak ditemukan.\nCoba nama lain atau hapus pencarian."
		}
		n.drawText(empty, 28, 248, 302, 65, 12, false, muted, true)
	}
	for j, label := range []string{"Sebelum", "Berikut"} {
		x := 20 + j*250
		saved := n.beginPress(18 + j)
		n.round(x, 351, 68, 25, 9, card, false)
		labelInk := ink
		if !n.controlEnabled(18 + j) {
			labelInk = labelInk&0xffffff | 0x80000000
		}
		n.drawText(label, x, 351, 68, 25, 10, true, labelInk, true)
		n.endPress(saved)
	}
	page, total := 0, 0
	if len(r.visibleRows()) > 0 {
		page = r.offset/resourcePageSize + 1
		total = (len(r.visibleRows()) + resourcePageSize - 1) / resourcePageSize
	}
	scope := fmt.Sprintf("%d dipilih", len(r.config.Paths))
	if r.config.AllBackground {
		scope = "Semua latar"
		if len(r.config.ExcludedPaths) > 0 {
			scope = fmt.Sprintf("%d dimatikan", len(r.config.ExcludedPaths))
		}
	}
	n.drawText(fmt.Sprintf("%d / %d · %s", page, total, scope), 90, 351, 178, 25, 10, false, muted, true)
	n.drawResourceControls(ink, muted, card, group)
}

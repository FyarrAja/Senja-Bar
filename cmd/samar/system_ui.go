//go:build windows

package main

import (
	"context"
	"fmt"
	"samar/internal/systemtools"
	"strings"
	"time"
	"unsafe"
)

const cmdSystem = 117
const wmSystemResult = wmApp + 80

type systemResult struct {
	snapshot *systemtools.Snapshot
	output   string
	err      error
}
type nativeSystem struct {
	section        string
	snapshot       systemtools.Snapshot
	loaded, busy   bool
	output, report string
	results        chan systemResult
	job            *systemtools.Job
	lastRead       time.Time
}
type systemItem struct{ label, detail, key string }

func systemItems(section string) []systemItem {
	switch section {
	case "power":
		return []systemItem{{"Seimbang", "Terapkan profil Balanced langsung", "run:power-balanced"}, {"Hemat daya", "Terapkan profil penghemat daya", "run:power-saving"}, {"Performa tinggi", "Terapkan profil High performance", "run:power-performance"}, {"Preferensi GPU Windows", "Pengaturan GPU per aplikasi", "open:gpu"}, {"Kembali", "Semua alat sistem", "tools"}}
	case "storage":
		return []systemItem{{"Analisis penyimpanan", "Pindai kapasitas disk dan temp pengguna", "run:storage"}, {"Penyimpanan Windows", "Rekomendasi pembersihan bawaan Windows", "open:storage"}, {"Disk Cleanup", "Tinjau file sebelum dibersihkan", "open:cleanup"}, {"Optimalkan SSD", "Re-TRIM volume; memerlukan izin admin", "admin:ssd"}, {"Kembali", "Semua alat sistem", "tools"}}
	case "network":
		return []systemItem{{"Uji koneksi", "Ukur respons dan periksa DNS", "run:network"}, {"DNS Cloudflare", "1.1.1.1 / 1.0.0.1", "admin:dns-cloudflare"}, {"DNS Google", "8.8.8.8 / 8.8.4.4", "admin:dns-google"}, {"DNS otomatis", "Kembalikan DNS ke DHCP", "admin:dns-auto"}, {"Reset Winsock", "Memerlukan restart Windows", "admin:winsock"}, {"Kembali", "Semua alat sistem", "tools"}}
	case "repair":
		return []systemItem{{"Periksa integritas", "DISM CheckHealth dan SFC verify-only", "admin:verify"}, {"Perbaiki image Windows", "DISM RestoreHealth", "admin:dism"}, {"Perbaiki file sistem", "SFC scannow", "admin:sfc"}, {"Kembali", "Semua alat sistem", "tools"}}
	case "result":
		return nil
	default:
		return []systemItem{{"Diagnosis perangkat", "Periksa RAM, CPU, ruang disk, dan daya", "run:diagnostics"}, {"Uji jaringan", "Ukur ping dan periksa resolusi DNS", "network"}, {"RAM & CPU", "Pengelolaan aplikasi latar oleh Senja", "memory"}, {"Penyimpanan", "Analisis kapasitas disk dan file sementara", "storage"}, {"Daya & performa", "Terapkan profil daya langsung dari Senja", "power"}, {"Perawatan Windows", "Pemeriksaan integritas dan perbaikan sistem", "repair"}}
	}
}
func (n *nativeGlassPanel) layoutSystemControls() {
	nativeControlBoxes[38] = [4]int{-104, 202, 96, 42}
	nativeControlBoxes[39] = [4]int{20, 84, 159, 34}
	nativeControlBoxes[40] = [4]int{179, 84, 159, 34}
	for i := 0; i < 6; i++ {
		nativeControlBoxes[41+i] = [4]int{20, 132 + i*48, 318, 44}
	}
	nativeControlBoxes[47] = [4]int{20, 474, 318, 38}
	if n.system.section == "result" {
		nativeControlBoxes[46] = [4]int{20, 420, 318, 36}
	}
}
func init() {
	nativeControlCommands[38] = cmdSystem
	nativeControlCommands[39] = 700
	nativeControlCommands[40] = 701
	for i := 0; i < 6; i++ {
		nativeControlCommands[41+i] = 710 + uint32(i)
	}
	nativeControlCommands[47] = 720
}
func (n *nativeGlassPanel) readSystem() {
	s := &n.system
	if s.busy {
		return
	}
	s.busy = true
	s.results = make(chan systemResult, 1)
	ch, hwnd := s.results, n.hwnd
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		data := systemtools.Read(ctx)
		ch <- systemResult{snapshot: &data}
		postMessageW.Call(hwnd, wmSystemResult, 0, 0)
	}()
}
func (n *nativeGlassPanel) pollSystem() {
	s := &n.system
	select {
	case r := <-s.results:
		s.busy = false
		if r.snapshot != nil {
			s.snapshot = *r.snapshot
			s.loaded = true
			s.lastRead = time.Now()
		} else {
			s.output = r.output
			if r.err != nil {
				s.output = "GAGAL: " + r.err.Error() + "\n" + r.output
			}
			s.section = "result"
		}
	default:
	}
	if s.job != nil {
		out, done := s.job.Poll()
		if out != "" {
			s.output = strings.TrimPrefix(out, "\ufeff")
		}
		if done {
			s.busy = false
			s.job = nil
		}
	}
}
func (n *nativeGlassPanel) systemAction(command uint32) bool {
	if command == cmdSystem {
		n.micPage = false
		n.systemPage = true
		n.resourcePage = false
		n.resources.optionOpen = 0
		n.focus = -1
		if !n.system.loaded {
			n.readSystem()
		}
		user32.NewProc("SetTimer").Call(n.hwnd, 0x5359, 1000, 0)
		user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
		return true
	}
	if !n.systemPage || command < 700 || command > 720 {
		return false
	}
	s := &n.system
	if command == 700 {
		s.section = ""
		if !s.busy {
			n.readSystem()
		}
	} else if command == 701 {
		s.section = "tools"
	} else if command == 720 {
		path, err := systemtools.SaveReport(s.snapshot, s.output)
		if err != nil {
			s.output += "\nGAGAL menyimpan laporan: " + err.Error()
		} else {
			s.report = path
		}
		s.section = "result"
	} else if command >= 710 && command < 716 {
		if s.section == "result" {
			if command == 715 {
				s.section = "tools"
			}
		} else {
			index := int(command - 710)
			items := systemItems(s.section)
			if index < len(items) {
				n.runSystemItem(items[index])
			}
		}
	}
	n.focus = -1
	user32.NewProc("InvalidateRect").Call(n.hwnd, 0, 0)
	return true
}
func (n *nativeGlassPanel) runSystemItem(item systemItem) {
	s := &n.system
	if s.busy {
		return
	}
	key := item.key
	switch {
	case key == "memory":
		n.systemPage = false
		n.resourceAction(cmdResources)
		return
	case strings.HasPrefix(key, "open:"):
		err := systemtools.Open(strings.TrimPrefix(key, "open:"))
		if err != nil {
			s.output = err.Error()
			s.section = "result"
		}
	case strings.HasPrefix(key, "run:"):
		s.busy = true
		s.output = "Pemeriksaan berjalan…"
		s.section = "result"
		s.results = make(chan systemResult, 1)
		ch, hwnd := s.results, n.hwnd
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			out, err := systemtools.Run(ctx, strings.TrimPrefix(key, "run:"))
			ch <- systemResult{output: out, err: err}
			postMessageW.Call(hwnd, wmSystemResult, 0, 0)
		}()
	case strings.HasPrefix(key, "admin:"):
		prompt := utf16ptr(item.label + "\n\n" + item.detail + ". Tindakan hanya dijalankan setelah izin administrator disetujui.\nLanjutkan?")
		answer, _, _ := user32.NewProc("MessageBoxW").Call(n.hwnd, uintptr(unsafe.Pointer(prompt)), uintptr(unsafe.Pointer(utf16ptr("Senja — "+item.label))), 0x24)
		if answer != 6 {
			return
		}
		job, err := systemtools.StartAdmin(strings.TrimPrefix(key, "admin:"))
		s.section = "result"
		if err != nil {
			s.output = "Tindakan tidak dimulai: " + err.Error()
		} else {
			s.job = &job
			s.busy = true
			s.output = "Menunggu hasil Windows…\nLog: " + job.Log
		}
	default:
		s.section = key
	}
}
func (n *nativeGlassPanel) drawSystem(ink, muted, card, group uint32) {
	n.layoutSystemControls()
	n.pollSystem()
	s := &n.system
	n.text("Sistem", 20, 18, 318, 37, 26, true, ink)
	n.text("Kondisi perangkat dan pemeriksaan langsung", 20, 57, 318, 22, 12, false, muted)
	n.line(20, 118, 338, 118, card, 1)
	target := 42.
	if s.section != "" {
		target += 159
	}
	n.move(&n.tabMotion, target)
	n.roundFloat(float32(n.tabMotion.value), 116, 115, 2.5, 1.25, 0xff77a8ff, false, 1)
	for i, label := range []string{"Monitor", "Alat"} {
		x := 20 + i*159
		saved := n.beginPress(39 + i)
		selected := (s.section == "") == (i == 0)
		color := muted
		if selected {
			color = ink
		}
		n.drawText(label, x, 84, 159, 32, 13, selected, color, true)
		n.endPress(saved)
	}
	if s.section == "" {
		n.drawSystemMonitor(ink, muted, card, group)
	} else if s.section == "result" {
		heading := "Hasil pemeriksaan"
		if s.busy {
			heading = "Sedang berjalan…"
		}
		n.text(heading, 20, 132, 318, 28, 14, true, ink)
		lines := strings.Split(s.output, "\n")
		if len(lines) > 15 {
			lines = append([]string{"… (hasil lengkap tersedia di laporan)"}, lines[len(lines)-14:]...)
		}
		n.drawText(strings.Join(lines, "\n"), 20, 172, 318, 237, 11, false, muted, false)
		saved := n.beginPress(46)
		n.round(20, 420, 318, 36, 11, card, false)
		n.drawText("Kembali ke alat", 20, 420, 318, 36, 12, true, ink, true)
		n.endPress(saved)
	} else {
		for i, item := range systemItems(s.section) {
			y := 132 + i*48
			saved := n.beginPress(41 + i)
			if n.presses[41+i].value > 0 {
				n.round(20, y, 318, 44, 10, card, false)
			}
			labelInk := ink
			if s.busy {
				labelInk = muted&0xffffff | 0x88000000
			}
			n.text(item.label, 24, y+3, 235, 23, 13, true, labelInk)
			n.drawText(item.detail, 24, y+26, 280, 16, 10, false, muted, false, true)
			if s.section == "tools" {
				origin := "Windows"
				if i < 5 {
					origin = "Senja"
				}
				n.drawText(origin, 270, y+4, 48, 21, 10, false, muted, true)
			}
			if i < len(systemItems(s.section))-1 {
				n.line(24, y+47, 334, y+47, card, 1)
			}
			n.line(319, y+18, 323, y+22, muted, 1)
			n.line(323, y+22, 319, y+26, muted, 1)
			n.endPress(saved)
		}
	}
	saved := n.beginPress(47)
	n.round(20, 474, 318, 38, 12, card, false)
	label := "Ekspor laporan"
	color := ink
	if !n.controlEnabled(47) {
		color = muted&0xffffff | 0x88000000
	}
	if s.report != "" {
		label = "Laporan tersimpan"
	}
	n.drawText(label, 20, 474, 318, 38, 12, true, color, true)
	n.endPress(saved)
}
func (n *nativeGlassPanel) drawSystemMonitor(ink, muted, card, group uint32) {
	s := &n.system
	d := s.snapshot
	if !s.loaded {
		n.drawText("Membaca perangkat…", 20, 175, 318, 70, 13, false, muted, true)
		return
	}
	if d.Error != "" {
		n.drawText("Data belum tersedia.\n"+d.Error, 20, 160, 318, 270, 11, false, muted, false)
		return
	}
	n.drawText(d.Model, 20, 134, 318, 28, 13, true, ink, false, true)
	n.drawText(d.CPU, 20, 167, 318, 22, 10, false, muted, false, true)
	n.drawText(d.GPU, 20, 192, 318, 22, 10, false, muted, false, true)
	n.round(20, 230, 318, 65, 14, group, false)
	used := d.RAMTotalGB - d.RAMFreeGB
	n.text("RAM", 34, 239, 120, 20, 11, false, muted)
	n.text(fmt.Sprintf("%.1f / %.1f GB", used, d.RAMTotalGB), 34, 261, 190, 25, 14, true, ink)
	n.text("CPU", 247, 239, 76, 20, 11, false, muted)
	n.text(fmt.Sprintf("%.0f%%", d.CPULoad), 247, 261, 76, 25, 14, true, ink)
	for i, v := range d.Volumes {
		if i >= 3 {
			break
		}
		y := 310 + i*34
		n.text(v.Name, 20, y, 42, 24, 12, true, ink)
		n.drawText(fmt.Sprintf("%.1f GB kosong / %.1f GB", v.FreeGB, v.TotalGB), 67, y, 271, 24, 11, false, muted, false)
	}
	plan := d.PowerPlan
	if i := strings.LastIndex(plan, "("); i >= 0 {
		plan = strings.TrimSuffix(plan[i+1:], ")")
	}
	n.drawText("Daya: "+plan, 20, 419, 318, 23, 11, false, muted, false, true)
	n.text("Baterai: "+d.Battery, 20, 445, 318, 23, 11, false, muted)
}

//go:build windows

package main

import (
	"fmt"
	"math"
	"samar/internal/updatestate"
	"strings"
	"time"
	"unsafe"
)

func updatePresentation(phase uint32, pending bool, version string) (label, detail string, enabled, mark bool) {
	label = "Pembaruan"
	switch phase {
	case 1:
		detail = "Memeriksa…"
	case 2:
		detail = "Mengunduh…"
	case 3:
		detail = "Memasang…"
	case 4:
		return "Sudah terbaru", "v" + strings.TrimPrefix(version, "v"), false, false
	case 5:
		if pending {
			return "Perbarui", "Coba lagi", true, true
		}
		detail = "Belum terhubung"
	case 7:
		return "Mulai ulang", "Pembaruan siap", true, true
	case 6:
		if pending {
			return "Perbarui", "Versi baru tersedia", true, true
		}
		detail = "Menunggu status"
	default:
		detail = "Menunggu status"
	}
	return label, detail, false, pending
}
func (n *nativeGlassPanel) controlEnabled(i int) bool {
	if ((i >= 4 && i <= 6 || i == 9) && n.updatePhase == 2) || ((i == 4 || i == 5 || i == 9) && n.showReadyPrompt()) {
		return false
	}
	if i == 7 && n.state.AutoInstall && n.updatePhase == updatestate.Ready {
		return false
	}
	if i != 7 {
		return true
	}
	_, _, enabled, _ := updatePresentation(n.updatePhase, n.updateAvailable, buildVersion)
	return enabled
}

// Three small pulses draw attention once, then leave a static readable badge.
func updateNoticeScale(start, now time.Time) (float64, bool) {
	if start.IsZero() {
		return 1, false
	}
	t := now.Sub(start)
	if t < 0 || t >= 1800*time.Millisecond {
		return 1, false
	}
	pulse := math.Sin(math.Pi * float64(t) / float64(600*time.Millisecond))
	return 1 + .08*pulse*pulse, true
}

func (n *nativeGlassPanel) drawUpdateNotice(now time.Time) {
	scale, _ := updateNoticeScale(n.noticeStarted, now)
	// Keep the badge at the button's top-left, with clearance
	// from the settings group. Text and fill scale together.
	var saved uint32
	nativeG("GdipSaveGraphics", n.graphics, uintptr(unsafe.Pointer(&saved)))
	nativeG("GdipTranslateWorldTransform", n.graphics, nativeF(213), nativeF(433), 0)
	nativeG("GdipScaleWorldTransform", n.graphics, nativeF(float32(scale)), nativeF(float32(scale)), 0)
	nativeG("GdipTranslateWorldTransform", n.graphics, nativeF(-213), nativeF(-433), 0)
	n.round(190, 423, 46, 20, 10, 0xffb72e46, false)
	n.drawText("Baru", 190, 423, 46, 20, 11, true, 0xffffffff, true)
	nativeG("GdipRestoreGraphics", n.graphics, uintptr(saved))
}

// Finite interpolation shares the existing menu ticker; no idle animation loop.
type nativeDownloadMotion struct {
	value, from, to float64
	started         time.Time
}

func (m *nativeDownloadMotion) target(to float64, now time.Time) {
	m.advance(now)
	m.from, m.to, m.started = m.value, max(m.value, min(1, to)), now
}
func (m *nativeDownloadMotion) advance(now time.Time) bool {
	if m.value == m.to {
		return false
	}
	t := min(1., max(0., float64(now.Sub(m.started))/float64(160*time.Millisecond)))
	m.value = m.from + (m.to-m.from)*(1-math.Pow(1-t, 3))
	if t >= 1 {
		m.value = m.to
		return false
	}
	return true
}

func (n *nativeGlassPanel) downloadStats(now time.Time) updatestate.Download {
	elapsed := time.Duration(0)
	if !n.downloadStarted.IsZero() {
		elapsed = now.Sub(n.downloadStarted)
	}
	return updatestate.ReadDownload(n.updateProgress, elapsed)
}
func downloadCopy(d updatestate.Download) (amount, speed, eta string) {
	amount = fmt.Sprintf("%.2f / -- MB", float64(d.Done)/1e6)
	if d.Total > 0 {
		amount = fmt.Sprintf("%.2f / %.2f MB", float64(d.Done)/1e6, float64(d.Total)/1e6)
	}
	speed = "-- MB/s"
	if rate := d.BytesPerSecond(); rate > 0 {
		speed = fmt.Sprintf("%.2f MB/s", rate/1e6)
	}
	eta = "Menghitung..."
	if remaining, ok := d.Remaining(); ok {
		seconds := int(remaining / time.Second)
		if seconds >= 3600 {
			eta = fmt.Sprintf("Sisa %dj %dm", seconds/3600, seconds%3600/60)
		} else if seconds >= 60 {
			eta = fmt.Sprintf("Sisa %dm %dd", seconds/60, seconds%60)
		} else {
			eta = fmt.Sprintf("Sisa %d dtk", seconds)
		}
	}
	if d.Total <= 0 {
		eta = "Waktu belum diketahui"
	}
	return
}
func (n *nativeGlassPanel) drawDownload(ink, muted, card uint32) {
	d := n.downloadStats(time.Now())
	amount, speed, eta := downloadCopy(d)
	n.round(20, 294, 318, 184, 22, card, false)
	n.text("Mengunduh update", 38, 311, 192, 27, 14, true, ink)
	percent := "--"
	if d.Percent() >= 0 {
		percent = fmt.Sprintf("%d%%", d.Percent())
	}
	n.drawText(percent, 237, 306, 85, 36, 24, true, ink, true)
	n.text(amount, 38, 350, 282, 27, 15, true, ink)
	n.round(38, 388, 282, 6, 3, 0x33ffffff, false)
	if d.Percent() >= 0 && n.downloadMotion.value > 0 {
		width := float32(282 * n.downloadMotion.value)
		n.roundFloat(38, 388, width, 6, min(float32(3), width/2), 0xff4388ff, false, 0)
	}
	n.text("Kecepatan", 38, 425, 135, 20, 11, false, muted)
	n.text("Sisa waktu", 190, 425, 135, 20, 11, false, muted)
	n.text(speed, 38, 444, 135, 25, 13, true, ink)
	eta = strings.TrimPrefix(eta, "Sisa ")
	if d.Total <= 0 {
		eta = "Belum diketahui"
	}
	n.text(eta, 190, 444, 135, 25, 13, true, ink)
}

func (n *nativeGlassPanel) showReadyPrompt() bool {
	return n.updatePhase == updatestate.Ready && !n.readyDismissed && !n.state.AutoInstall
}
func (n *nativeGlassPanel) drawReady(ink, muted, card uint32) {
	n.round(20, 294, 318, 184, 22, card, false)
	n.text("Pembaruan siap diterapkan", 38, 310, 282, 28, 15, true, ink)
	n.text("Mulai ulang Senja untuk menggunakan", 38, 348, 282, 23, 12, false, ink)
	n.text("versi terbaru. Windows tetap berjalan.", 38, 372, 282, 23, 12, false, muted)
	saved := n.beginPress(6)
	n.round(38, 430, 128, 36, 13, 0x22ffffff, false)
	n.drawText("Nanti", 38, 430, 128, 36, 12, true, ink, true)
	n.endPress(saved)
	saved = n.beginPress(7)
	n.round(184, 430, 136, 36, 13, 0xff286be0, false)
	n.drawText("Mulai ulang Senja", 184, 430, 136, 36, 12, true, 0xffffffff, true)
	n.endPress(saved)
}

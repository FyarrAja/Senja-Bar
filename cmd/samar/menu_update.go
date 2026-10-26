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
	if i == 6 && n.updatePhase == 2 {
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
	// Keep the badge at the button's top-left, with four pixels of clearance
	// from the settings group. Text and fill scale together.
	var saved uint32
	nativeG("GdipSaveGraphics", n.graphics, uintptr(unsafe.Pointer(&saved)))
	nativeG("GdipTranslateWorldTransform", n.graphics, nativeF(207), nativeF(432), 0)
	nativeG("GdipScaleWorldTransform", n.graphics, nativeF(float32(scale)), nativeF(float32(scale)), 0)
	nativeG("GdipTranslateWorldTransform", n.graphics, nativeF(-207), nativeF(-432), 0)
	n.round(190, 424, 34, 16, 8, 0xffb72e46, false)
	n.drawText("Baru", 190, 424, 34, 16, 8, true, 0xffffffff, true)
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
	n.round(20, 430, 318, 48, 17, card, false)
	n.updateIcon(34, 438, ink)
	label := "Mengunduh..."
	if percent := d.Percent(); percent >= 0 {
		label = fmt.Sprintf("Mengunduh %d%%", percent)
	}
	n.text(label, 54, 432, 157, 19, 11, true, ink)
	n.text(eta, 217, 433, 113, 18, 9, false, muted)
	n.text(amount, 34, 451, 181, 18, 10, false, ink)
	n.text(speed, 239, 451, 95, 18, 10, false, ink)
	n.round(34, 471, 290, 3, 1, 0x33ffffff, false)
	if d.Percent() >= 0 && n.downloadMotion.value > 0 {
		width := float32(290 * n.downloadMotion.value)
		n.roundFloat(34, 471, width, 3, min(float32(1), width/2), 0xff4388ff, false, 0)
	}
}

//go:build windows

package main

import (
	"math"
	"strings"
	"time"
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
	if i != 7 {
		return true
	}
	_, _, enabled, _ := updatePresentation(n.updatePhase, n.updateAvailable, buildVersion)
	return enabled
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

//go:build windows

package main

import "strings"

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

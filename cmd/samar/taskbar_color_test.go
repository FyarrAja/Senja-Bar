//go:build windows

package main

import "testing"

func TestAdaptiveMaterialsKeepSystemTextReadableAcrossBackdrops(t *testing.T) {
	for _, light := range []bool{false, true} {
		for _, background := range []backdropRGB{{0, 0, 0}, {255, 255, 255}, {128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 0}} {
			palette := taskbarColors(light, background)
			if palette.solid>>24 != 255 {
				t.Fatal("Solid must remain opaque")
			}
			if palette.acrylic>>24 < palette.glass>>24 {
				t.Fatal("Acrylic must remain denser than Glass")
			}
			for _, color := range []uint32{palette.glass, palette.acrylic, palette.solid} {
				a := float64(color>>24) / 255
				tint := backdropRGB{float64(color & 255), float64(color >> 8 & 255), float64(color >> 16 & 255)}
				lum := mixBackdrop(background, tint, a).relativeLuminance()
				if light && lum < .315 || !light && lum > .165 {
					t.Fatalf("light=%v backdrop=%+v color=%08x luminance=%.1f", light, background, color, lum)
				}
			}
			if accentForPalette(styleClear, palette).gradient != 0 {
				t.Fatal("Clear must remain fully transparent for every theme")
			}
			if accentForPalette(styleNormal, palette).state != 0 {
				t.Fatal("normal restore must disable custom material")
			}
		}
	}
}
func TestBackdropHueAndThemeAreBothRespected(t *testing.T) {
	for _, light := range []bool{false, true} {
		red := taskbarColors(light, backdropRGB{220, 40, 40}).solid
		blue := taskbarColors(light, backdropRGB{40, 40, 220}).solid
		if red&255 <= red>>16&255 || blue>>16&255 <= blue&255 {
			t.Fatal("backdrop hue was replaced by a fixed neutral color")
		}
	}
	dark := taskbarColors(false, backdropRGB{128, 128, 128})
	light := taskbarColors(true, backdropRGB{128, 128, 128})
	if light.solid&255 <= dark.solid&255 {
		t.Fatal("Solid must follow the Windows system light/dark theme")
	}
}

//go:build windows

package main

import "testing"

func TestTransparentBackdropDoesNotAddWhiteWash(t *testing.T) {
	pixels := []byte{0, 0, 0, 255, 220, 60, 20, 255, 255, 255, 255, 255}
	limitGlassHighlights(pixels)
	if pixels[0] != 0 || pixels[1] != 0 || pixels[2] != 0 {
		t.Fatal("black desktop must remain black, not icy white")
	}
	if pixels[4] != 220 || pixels[4] <= pixels[6] {
		t.Fatal("highlight limiting must preserve background hue")
	}
	if pixels[8] != 105 || pixels[9] != 105 || pixels[10] != 105 {
		t.Fatal("bright backdrop must remain readable")
	}
}

func TestAdaptiveGlassKeepsHueAcrossBackdropPalette(t *testing.T) {
	for _, rgb := range [][3]byte{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 220, 0}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255}, {20, 20, 20}, {0, 0, 0}} {
		pixels := []byte{rgb[2], rgb[1], rgb[0], 255}
		limitGlassHighlights(pixels)
		// Uniform attenuation preserves channel ratios and never adds a tint.
		for a := 0; a < 3; a++ {
			for b := 0; b < 3; b++ {
				difference := int(pixels[2-a])*int(rgb[b]) - int(pixels[2-b])*int(rgb[a])
				if difference < -255 || difference > 255 {
					t.Fatalf("hue shifted for RGB %v: BGR %v", rgb, pixels)
				}
			}
		}
	}
}

func TestFrostedBackdropPreservesHueAndSoftensEdges(t *testing.T) {
	w, h := 80, 20
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if x < w/2 {
				pixels[i] = 220
				pixels[i+2] = 20
			} else {
				pixels[i] = 20
				pixels[i+2] = 220
			}
			pixels[i+1] = 60
			pixels[i+3] = 255
		}
	}
	frostedBlur(pixels, w, h, 4)
	left, right := ((h/2)*w+5)*4, ((h/2)*w+w-6)*4
	if pixels[left] <= pixels[left+2] || pixels[right+2] <= pixels[right] {
		t.Fatal("glass lost the background hue")
	}
	a, b := ((h/2)*w+w/2-1)*4, ((h/2)*w+w/2)*4
	difference := int(pixels[a]) - int(pixels[b])
	if difference < 0 {
		difference = -difference
	}
	if difference > 15 {
		t.Fatalf("edge remained sharp: difference=%d", difference)
	}
	for i := 0; i < len(pixels); i += 4 {
		if pixels[i] < 179 || pixels[i+1] < 171 || pixels[i+2] < 159 {
			t.Fatal("icy material is too dark for slate labels")
		}
		if pixels[i+3] != 255 {
			t.Fatal("native menu backdrop must be opaque to input")
		}
	}
}

//go:build windows

package main

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"samar/internal/mic"
	"samar/internal/resources"
	"samar/internal/systemtools"
	"sort"
	"testing"
	"time"
	"unsafe"
)

// Render the actual native drawing functions, without opening a window or
// capturing audio while the user is playing a game.
func TestNativeControlPageRenders(t *testing.T) {
	if os.Getenv("SENJA_RENDER_QA") != "1" {
		t.Skip("native visual QA")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var token uintptr
	input := struct {
		Version            uint32
		Debug              uintptr
		Background, Codecs int32
	}{Version: 1}
	if code := nativeG("GdiplusStartup", uintptr(unsafe.Pointer(&token)), uintptr(unsafe.Pointer(&input)), 0); code != 0 {
		t.Fatal(code)
	}
	defer nativeG("GdiplusShutdown", token)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	snapshot := systemtools.Read(ctx)
	processes, _, _, _ := resources.ProtectedSnapshot()
	sort.Slice(processes, func(i, j int) bool { return processes[i].WorkingSet > processes[j].WorkingSet })
	oldDPI := menuDPI
	defer func() { menuDPI = oldDPI }()
	for _, dpi := range []int32{96, 120} {
		menuDPI = dpi
		for _, page := range []string{"mic", "mic-auto", "mic-live", "mic-device", "mic-voice", "mic-input", "mic-preset", "mic-output", "mic-meter", "mic-route", "system", "tools", "apps", "automatic", "taskbar"} {
			width, height := int(menuPx(nativePanelWidth)), int(menuPx(548))
			raw := make([]byte, width*height*4)
			n := nativeGlassPanel{fonts: make(map[int]uintptr), focus: -1, pressed: -1, keyPressed: -1, w: int32(width), h: int32(height)}
			nativeG("GdipCreateBitmapFromScan0", uintptr(width), uintptr(height), uintptr(width*4), 0x000e200b, uintptr(unsafe.Pointer(&raw[0])), uintptr(unsafe.Pointer(&n.image)))
			nativeG("GdipGetImageGraphicsContext", n.image, uintptr(unsafe.Pointer(&n.graphics)))
			nativeG("GdipSetSmoothingMode", n.graphics, 4)
			nativeG("GdipSetTextRenderingHint", n.graphics, 4)
			nativeG("GdipCreateFontFamilyFromName", uintptr(unsafe.Pointer(utf16ptr("Segoe UI"))), 0, uintptr(unsafe.Pointer(&n.family)))
			nativeG("GdipScaleWorldTransform", n.graphics, nativeF(float32(dpi)/96), nativeF(float32(dpi)/96), 0)
			n.state.Style = 1
			switch page {
			case "mic", "mic-auto", "mic-live", "mic-device", "mic-voice", "mic-input", "mic-preset", "mic-output", "mic-meter", "mic-route":
				n.micPage = true
				n.loadMic()
				if page == "mic-auto" {
					n.mic.config = mic.ApplyPreset(n.mic.config, 6)
				}
				if page == "mic-live" {
					n.mic.config.Monitor = true
					n.mic.config.MonitorVolume = 50
					n.mic.status = mic.Status{Active: true, Level: 30, Message: "Dengar langsung aktif · gunakan headset"}
					n.mic.monitorMotion.set(17, time.Now(), true)
				}
				n.mic.details = page == "mic-route"
				kind := 0
				if page == "mic-device" {
					kind = 1
				}
				if page == "mic-voice" {
					kind = 2
				}
				if page == "mic-input" {
					kind = 3
				}
				if page == "mic-preset" {
					kind = 5
				}
				if page == "mic-output" {
					kind = 6
				}
				if page == "mic-meter" {
					n.mic.level.set(20, time.Now(), true)
				}
				if kind != 0 {
					n.openMicDropdown(kind)
					n.mic.reveal.set(1, time.Now(), true)
				}
			case "system", "tools":
				n.systemPage = true
				n.system.loaded = true
				n.system.snapshot = snapshot
				if page == "tools" {
					n.system.section = "tools"
				}
			case "apps", "automatic":
				n.system.loaded = true
				n.system.snapshot = snapshot
				n.resourcePage = true
				n.resources.loaded = true
				n.resources.config = resources.Load()
				n.resources.config.AllBackground = false
				n.resources.tuning = page == "automatic"
				if n.resources.config.Automatic {
					n.switches[3] = 17
				}
				if n.resources.config.EcoCPU {
					n.switches[4] = 17
				}
				n.resources.message = "Game, aplikasi aktif, dan audio tetap dilindungi."
				for _, p := range processes {
					if len(n.resources.rows) >= 4 {
						break
					}
					n.resources.rows = append(n.resources.rows, resourceRow{path: p.Path, name: p.Name, status: p.Protected, memory: p.WorkingSet})
				}
			}
			n.drawPanelContents()
			nativeG("GdipFlush", n.graphics, 1)
			im := image.NewNRGBA(image.Rect(0, 0, width, height))
			for i := 0; i < len(raw); i += 4 {
				a := uint32(raw[i+3])
				im.Pix[i] = byte(min(255, uint32(raw[i+2])+24*(255-a)/255))
				im.Pix[i+1] = byte(min(255, uint32(raw[i+1])+26*(255-a)/255))
				im.Pix[i+2] = byte(min(255, uint32(raw[i])+30*(255-a)/255))
				im.Pix[i+3] = 255
			}
			path := filepath.Join("..", "..", "dist-development", fmt.Sprintf("qa-%s-%d.png", page, dpi))
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(file, im)
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
			for _, font := range n.fonts {
				nativeG("GdipDeleteFont", font)
			}
			nativeG("GdipDeleteFontFamily", n.family)
			nativeG("GdipDeleteGraphics", n.graphics)
			nativeG("GdipDisposeImage", n.image)
		}
	}
}

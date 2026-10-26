package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"samar/internal/releaseupdate"
	"samar/internal/updatestate"
	"syscall"
	"time"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")

func report(phase int) {
	name, _ := syscall.UTF16PtrFromString("SenjaHostWindow")
	hwnd, _, _ := user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(name)), 0)
	if hwnd != 0 {
		user32.NewProc("PostMessageW").Call(hwnd, 0x8000+61, uintptr(phase), 0)
	}
}

var downloadStarted time.Time

func reportProgress(done, total int64) {
	if downloadStarted.IsZero() {
		downloadStarted = time.Now()
	}
	name, _ := syscall.UTF16PtrFromString("SenjaHostWindow")
	hwnd, _, _ := user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(name)), 0)
	if hwnd != 0 {
		user32.NewProc("PostMessageW").Call(hwnd, 0x8000+67, uintptr(updatestate.DownloadWire(done, total)), uintptr(time.Since(downloadStarted)/time.Millisecond))
	}
}
func run() error {
	if len(os.Args) != 2 && !(len(os.Args) == 3 && (os.Args[2] == "--check-only" || os.Args[2] == "--apply-ready")) {
		return fmt.Errorf("current version required")
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return fmt.Errorf("LOCALAPPDATA unavailable")
	}
	if len(os.Args) == 3 && os.Args[2] == "--check-only" {
		client := releaseupdate.Client()
		client.Timeout = 20 * time.Second
		release, newer, err := releaseupdate.Check(context.Background(), client, os.Args[1])
		if err != nil {
			return err
		}
		if err = updatestate.Save(filepath.Join(base, "Senja", "update-state.json"), updatestate.Record{Latest: release.Tag, CheckedAt: time.Now().UTC()}); err != nil {
			return err
		}
		phase := releaseupdate.Current
		if newer {
			phase = releaseupdate.Available
		}
		report(phase)
		return nil
	}
	root := filepath.Join(base, "Senja", "updates")
	if len(os.Args) != 3 || os.Args[2] != "--apply-ready" {
		return releaseupdate.Stage(context.Background(), releaseupdate.Client(), os.Args[1], root, report, reportProgress)
	}
	path, err := releaseupdate.VerifiedPending(os.Args[1], root)
	if err != nil {
		// A missing or changed staged file must return to a fresh download,
		// rather than offering a permanently broken restart action.
		_ = os.Remove(filepath.Join(root, "pending.json"))
		return err
	}
	report(releaseupdate.Installing)
	command := exec.Command(path)
	command.Dir = filepath.Dir(path)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command.Run()
}
func main() {
	quiet := len(os.Args) > 1 && os.Args[len(os.Args)-1] == "--quiet"
	if quiet {
		os.Args = os.Args[:len(os.Args)-1]
	}
	if err := run(); err != nil {
		report(releaseupdate.Failed)
		if quiet || (len(os.Args) == 3 && os.Args[2] == "--check-only") {
			os.Exit(1)
		} // Background/offline checks stay quiet.
		// Only failures need a dialog; normal progress stays in the menu.
		message, _ := syscall.UTF16PtrFromString("Pembaruan gagal. Versi lama tetap dapat digunakan.\nCoba lagi saat koneksi tersedia.\n\n" + err.Error())
		title, _ := syscall.UTF16PtrFromString("Senja — Pembaruan")
		user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
		os.Exit(1)
	}
}

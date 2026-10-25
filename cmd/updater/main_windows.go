package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"samar/internal/releaseupdate"
	"syscall"
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
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("current version required")
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return fmt.Errorf("LOCALAPPDATA unavailable")
	}
	path, err := releaseupdate.Prepare(context.Background(), releaseupdate.Client(), os.Args[1], filepath.Join(base, "Senja", "updates"), report)
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	report(releaseupdate.Installing)
	command := exec.Command(path)
	command.Dir = filepath.Dir(path)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command.Run()
}
func main() {
	if err := run(); err != nil {
		report(releaseupdate.Failed)
		// Only failures need a dialog; normal progress stays in the menu.
		message, _ := syscall.UTF16PtrFromString("Pembaruan gagal. Versi lama tetap dapat digunakan.\nCoba lagi saat koneksi tersedia.\n\n" + err.Error())
		title, _ := syscall.UTF16PtrFromString("Senja — Pembaruan")
		user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
		os.Exit(1)
	}
}

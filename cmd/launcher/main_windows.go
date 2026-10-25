package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var buildVersion = "dev"
var user32 = syscall.NewLazyDLL("user32.dll")
var kernel32 = syscall.NewLazyDLL("kernel32.dll")

func safeEntry(root, name string) (string, error) {
	if strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, ":") {
		return "", fmt.Errorf("invalid archive path: %q", name)
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(name) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive path escapes destination: %q", name)
	}
	return path, nil
}

func unpack(data []byte, root string) error {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	var total uint64
	for _, file := range z.File {
		path, err := safeEntry(root, file.Name)
		if err != nil {
			return err
		}
		total += file.UncompressedSize64
		if file.Mode()&os.ModeSymlink != 0 || file.UncompressedSize64 > 64<<20 || total > 128<<20 {
			return fmt.Errorf("unsupported archive entry: %q", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, io.LimitReader(input, int64(file.UncompressedSize64)+1))
		closeErr := output.Close()
		input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	for _, name := range []string{"Senja.exe", "ExplorerTAP.dll", "ExplorerHooks.dll", "ProgramLog.dll", "Xaml.dll", "resources.pri", "Assets/menu.css", "LICENSE"} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("missing package file: %s", name)
		}
	}
	return nil
}

func ensurePackage(data []byte, base string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("build this launcher with scripts/package-release.ps1")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	root := filepath.Join(base, "Senja", "app")
	target := filepath.Join(root, buildVersion+"-"+digest[:12])
	ready := func() bool {
		marker, err := os.ReadFile(filepath.Join(target, ".ready"))
		return err == nil && string(marker) == digest
	}
	if ready() {
		return target, nil
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		return "", err
	}
	defer func() {
		// Cleanup is restricted to the staging directory we just created.
		rel, err := filepath.Rel(root, stage)
		if err == nil && !strings.ContainsAny(rel, "\\/") && strings.HasPrefix(rel, ".install-") {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := unpack(data, stage); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(stage, ".ready"), []byte(digest), 0600); err != nil {
		return "", err
	}
	if err := os.Rename(stage, target); err != nil && !ready() {
		return "", err
	}
	return target, nil
}

func runCommand(exe string, args ...string) *exec.Cmd {
	command := exec.Command(exe, args...)
	command.Dir = filepath.Dir(exe)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command
}

func replaceOlderEngine(exe string) error {
	class, _ := syscall.UTF16PtrFromString("SenjaHostWindow")
	hwnd, _, _ := user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(class)), 0)
	if hwnd == 0 {
		return nil
	}
	var pid uint32
	user32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	process, _, _ := kernel32.NewProc("OpenProcess").Call(0x100000|0x1000, 0, uintptr(pid))
	if process == 0 {
		return fmt.Errorf("close the running Senja, then open this EXE again")
	}
	defer kernel32.NewProc("CloseHandle").Call(process)
	var name [32768]uint16
	size := uint32(len(name))
	ok, _, _ := kernel32.NewProc("QueryFullProcessImageNameW").Call(process, 0, uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return fmt.Errorf("cannot identify the running Senja")
	}
	if strings.EqualFold(filepath.Clean(syscall.UTF16ToString(name[:size])), filepath.Clean(exe)) {
		return nil
	}
	var helper uintptr
	menuClass, _ := syscall.UTF16PtrFromString("SenjaCSSMenu")
	menu, _, _ := user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(menuClass)), 0)
	if menu != 0 {
		var helperPID uint32
		user32.NewProc("GetWindowThreadProcessId").Call(menu, uintptr(unsafe.Pointer(&helperPID)))
		helper, _, _ = kernel32.NewProc("OpenProcess").Call(0x100000, 0, uintptr(helperPID))
		if helper != 0 {
			defer kernel32.NewProc("CloseHandle").Call(helper)
		}
	}
	if err := runCommand(exe, "--shutdown").Run(); err != nil {
		return err
	}
	result, _, _ := kernel32.NewProc("WaitForSingleObject").Call(process, 5000)
	if result != 0 {
		return fmt.Errorf("Senja has not exited; close it before updating")
	}
	if helper != 0 {
		result, _, _ = kernel32.NewProc("WaitForSingleObject").Call(helper, 5000)
		if result != 0 {
			return fmt.Errorf("close the old Senja menu before updating")
		}
	}
	return nil
}

func launch() error {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return fmt.Errorf("LOCALAPPDATA is unavailable")
	}
	dir, err := ensurePackage(payload, base)
	if err != nil {
		return err
	}
	exe := filepath.Join(dir, "Senja.exe")
	if len(os.Args) == 1 {
		if err := replaceOlderEngine(exe); err != nil {
			return err
		}
	}
	command := runCommand(exe, os.Args[1:]...)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release() // No resident launcher or extraction worker.
}

func main() {
	if err := launch(); err != nil {
		text, _ := syscall.UTF16PtrFromString("Senja tidak dapat dibuka / Cannot start Senja:\n\n" + err.Error())
		title, _ := syscall.UTF16PtrFromString("Senja Bar")
		user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
		os.Exit(1)
	}
}

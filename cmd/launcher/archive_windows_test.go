package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, extra string) []byte {
	t.Helper()
	var data bytes.Buffer
	w := zip.NewWriter(&data)
	for _, name := range []string{"Senja.exe", "Senja-Update.exe", "ExplorerTAP.dll", "ExplorerHooks.dll", "ProgramLog.dll", "Xaml.dll", "resources.pri", "Assets/menu.css", "LICENSE", extra} {
		if name == "" {
			continue
		}
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("fixture:" + name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestArchiveCannotEscapeCache(t *testing.T) {
	for _, name := range []string{"../outside.exe", "Assets/../../outside.exe", "/outside.exe", "C:/outside.exe", `Assets\..\..\outside.exe`, "//host/share/outside.exe"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := unpack(fixture(t, name), root); err == nil {
				t.Fatalf("accepted escaping path %q", name)
			}
		})
	}
}

func TestBundleInstalledOnceAndReused(t *testing.T) {
	base := t.TempDir()
	data := fixture(t, "")
	first, err := ensurePackage(data, base)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(first, "Senja.exe")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ensurePackage(data, base)
	if err != nil || second != first {
		t.Fatalf("cache not reused: %s %v", second, err)
	}
	current, _ := os.Stat(path)
	if !info.ModTime().Equal(current.ModTime()) {
		t.Fatal("cached EXE was rewritten")
	}
	entries, err := os.ReadDir(filepath.Dir(first))
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary installation directory left behind")
	}
}

func TestFailedBundleNeverPublishesReadyCache(t *testing.T) {
	base := t.TempDir()
	if _, err := ensurePackage([]byte("invalid zip"), base); err == nil {
		t.Fatal("invalid bundle accepted")
	}
	entries, err := os.ReadDir(filepath.Join(base, "Senja", "app"))
	if err != nil || len(entries) != 0 {
		t.Fatal("failed extraction left a ready cache or staging directory")
	}
}

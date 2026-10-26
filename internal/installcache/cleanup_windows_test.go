package installcache

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func managed(t *testing.T, root, version, hash string) string {
	t.Helper()
	dir := filepath.Join(root, version+"-"+hash[:12])
	if err := os.MkdirAll(filepath.Join(dir, "Assets"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{".ready": hash, "Senja.exe": "fixture", "Assets/menu.css": "fixture"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
func TestRemovesOnlyVerifiedObsoleteInstalls(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Senja", "app")
	current := managed(t, root, "1.1.6", strings.Repeat("a", 64))
	old := managed(t, root, "1.1.5", strings.Repeat("b", 64))
	sameVersionOldBuild := managed(t, root, "1.1.6", strings.Repeat("c", 64))
	future := managed(t, root, "1.1.7", strings.Repeat("d", 64))
	unknown := filepath.Join(root, "personal-files")
	os.MkdirAll(unknown, 0700)
	os.WriteFile(filepath.Join(unknown, "keep.txt"), []byte("keep"), 0600)
	bad := managed(t, root, "1.1.0", strings.Repeat("e", 64))
	os.WriteFile(filepath.Join(bad, ".ready"), []byte(strings.Repeat("f", 64)), 0600)
	for _, path := range []string{current, future, unknown, bad, root, filepath.Dir(root)} {
		if err := Remove(root, current, path); err == nil {
			t.Fatalf("accepted protected path %s", path)
		}
	}
	for _, path := range []string{old, sameVersionOldBuild} {
		if err := Remove(root, current, path); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("old install remains: %v", err)
		}
	}
	for _, path := range []string{current, future, unknown, bad} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("protected folder removed: %s", path)
		}
	}
}
func TestLockedExecutableIsRetriedWithoutLosingOwnershipMarker(t *testing.T) {
	root := filepath.Join(t.TempDir(), "app")
	current := managed(t, root, "1.1.6", strings.Repeat("a", 64))
	old := managed(t, root, "1.1.5", strings.Repeat("b", 64))
	name, _ := syscall.UTF16PtrFromString(filepath.Join(old, "Senja.exe"))
	file, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(file)
	if err := Remove(root, current, old); err == nil {
		t.Fatal("locked EXE should be left intact")
	}
	if _, ok := ownedVersion(old); !ok {
		t.Fatal("locked install lost its retry marker")
	}
	syscall.CloseHandle(file)
	if err := Remove(root, current, old); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("unlocked old install was not removed")
	}
}
func TestPortableAndSymlinkTargetsAreNotCleaned(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "app")
	current := managed(t, root, "1.1.6", strings.Repeat("a", 64))
	old := managed(t, root, "1.1.5", strings.Repeat("b", 64))
	outside := filepath.Join(base, "Downloads")
	os.MkdirAll(outside, 0700)
	os.WriteFile(filepath.Join(outside, "Senja-Bar.exe"), []byte("keep"), 0600)
	if _, err := Candidates(root, outside); err == nil {
		t.Fatal("portable install allowed cleanup")
	}
	link := filepath.Join(old, "external")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlink creation requires Windows Developer Mode")
	}
	if err := Remove(root, current, old); err == nil {
		t.Fatal("accepted a reparse-point tree")
	}
	if data, err := os.ReadFile(filepath.Join(outside, "Senja-Bar.exe")); err != nil || string(data) != "keep" {
		t.Fatal("outside download was touched")
	}
}

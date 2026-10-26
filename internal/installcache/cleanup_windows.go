// Package installcache removes only recognized, older Senja managed installs.
package installcache

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var managedName = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)-([a-f0-9]{12})$`)

func plainPath(path string) bool {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attr, err := syscall.GetFileAttributes(p)
	return err == nil && attr&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0
}
func ownedVersion(dir string) ([3]uint64, bool) {
	var v [3]uint64
	name := managedName.FindStringSubmatch(filepath.Base(dir))
	if name == nil || !plainPath(dir) || !plainPath(filepath.Join(dir, ".ready")) {
		return v, false
	}
	info, err := os.Lstat(filepath.Join(dir, ".ready"))
	if err != nil || !info.Mode().IsRegular() || info.Size() != 64 {
		return v, false
	}
	marker, err := os.ReadFile(filepath.Join(dir, ".ready"))
	if err != nil || len(marker) != 64 || string(marker[:12]) != name[4] {
		return v, false
	}
	if _, err = hex.DecodeString(string(marker)); err != nil {
		return v, false
	}
	for i := range v {
		v[i], err = strconv.ParseUint(name[i+1], 10, 64)
		if err != nil {
			return v, false
		}
	}
	return v, true
}
func newer(a, b [3]uint64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// Candidates fails closed for portable installs, junctions, unknown folders and
// newer versions. All returned paths are verified direct children of appRoot.
func Candidates(appRoot, current string) ([]string, error) {
	root, err := filepath.Abs(appRoot)
	if err != nil {
		return nil, err
	}
	now, err := filepath.Abs(current)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Clean(filepath.Dir(now)), filepath.Clean(root)) || !plainPath(root) {
		return nil, fmt.Errorf("current install is outside the managed app directory")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	resolvedCurrent, err := filepath.EvalSymlinks(now)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Dir(resolvedCurrent), resolvedRoot) {
		return nil, fmt.Errorf("current install resolves outside managed app directory")
	}
	version, ok := ownedVersion(now)
	if !ok {
		return nil, fmt.Errorf("current install has no valid ready marker")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if strings.EqualFold(path, now) {
			continue
		}
		v, owned := ownedVersion(path)
		if !owned || newer(v, version) {
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || !strings.EqualFold(filepath.Dir(resolved), resolvedRoot) {
			continue
		}
		result = append(result, path)
	}
	return result, nil
}

// Remove validates every target again immediately before deletion. Keep the
// ownership marker until every other file is gone so locked DLLs can be retried
// on the next successful startup. Never kill Explorer or another Senja process.
func Remove(appRoot, current, target string) error {
	allowed, err := Candidates(appRoot, current)
	if err != nil {
		return err
	}
	found := false
	for _, path := range allowed {
		if strings.EqualFold(filepath.Clean(path), filepath.Clean(target)) {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("not a verified obsolete Senja install: %s", target)
	}
	if err = filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !plainPath(path) {
			return fmt.Errorf("refusing reparse point: %s", path)
		}
		return nil
	}); err != nil {
		return err
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	var first error
	for _, entry := range entries {
		if entry.Name() == ".ready" {
			continue
		}
		// Target is a verified absolute direct child, and its tree was checked.
		if err = os.RemoveAll(filepath.Join(target, entry.Name())); err != nil && first == nil {
			first = err
		}
	}
	if first != nil {
		return first
	}
	if err = os.Remove(filepath.Join(target, ".ready")); err != nil {
		return err
	}
	return os.Remove(target)
}

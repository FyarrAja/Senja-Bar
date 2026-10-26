package updatestate

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Pending struct {
	Version   string `json:"version"`
	Directory string `json:"directory"`
	SHA256    string `json:"sha256"`
}

func (p Pending) Path(root string) (string, error) {
	if p.Directory == "" || filepath.Base(p.Directory) != p.Directory || strings.ContainsAny(p.Directory, "/\\:") || !strings.HasPrefix(p.Version, "v") || !strings.HasPrefix(p.Directory, p.Version+"-") {
		return "", fmt.Errorf("invalid staged update directory")
	}
	hash, err := hex.DecodeString(p.SHA256)
	if err != nil || len(hash) != 32 {
		return "", fmt.Errorf("invalid staged update checksum")
	}
	return filepath.Join(root, p.Directory, "Senja-Bar.exe"), nil
}
func LoadPending(root string) (Pending, error) {
	var p Pending
	f, err := os.Open(filepath.Join(root, "pending.json"))
	if err != nil {
		return p, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() > 4096 {
		return p, fmt.Errorf("invalid staged update metadata")
	}
	if err = json.NewDecoder(f).Decode(&p); err != nil {
		return p, err
	}
	path, err := p.Path(root)
	if err != nil {
		return p, err
	}
	for _, target := range []string{root, filepath.Dir(path), path} {
		info, err = os.Lstat(target)
		if err != nil {
			return p, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return p, fmt.Errorf("staged update is a link")
		}
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 64<<20 {
		return p, fmt.Errorf("invalid staged installer")
	}
	return p, nil
}
func SavePending(root string, p Pending) error {
	if _, err := p.Path(root); err != nil {
		return err
	}
	return saveJSON(filepath.Join(root, "pending.json"), p)
}

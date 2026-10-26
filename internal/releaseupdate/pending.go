package releaseupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"samar/internal/updatestate"
	"strings"
)

// Stage only downloads, verifies and remembers an installer. It never runs it.
func Stage(ctx context.Context, client *http.Client, current, root string, phase func(int), progress func(int64, int64)) error {
	path, err := PrepareWithProgress(ctx, client, current, root, phase, progress)
	if err != nil || path == "" {
		return err
	}
	directory := filepath.Base(filepath.Dir(path))
	i := strings.LastIndex(directory, "-")
	if i < 0 {
		return fmt.Errorf("invalid staged directory")
	}
	tag := directory[:i]
	newer, err := Newer(tag, current)
	if err != nil || !newer {
		return fmt.Errorf("invalid staged version")
	}
	hash, err := installerHash(path)
	if err != nil {
		return err
	}
	if err = updatestate.SavePending(root, updatestate.Pending{Version: tag, Directory: directory, SHA256: hash}); err != nil {
		return err
	}
	phase(updatestate.Ready)
	return nil
}
func installerHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, 64<<20+1))
	if err != nil {
		return "", err
	}
	if n <= 0 || n > 64<<20 {
		return "", fmt.Errorf("invalid staged installer size")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifiedPending rechecks the bytes before an explicitly requested apply.
func VerifiedPending(current, root string) (string, error) {
	p, err := updatestate.LoadPending(root)
	if err != nil {
		return "", err
	}
	newer, err := Newer(p.Version, current)
	if err != nil || !newer {
		return "", fmt.Errorf("staged version is not newer")
	}
	path, err := p.Path(root)
	if err != nil {
		return "", err
	}
	hash, err := installerHash(path)
	if err != nil {
		return "", err
	}
	if hash != p.SHA256 {
		return "", fmt.Errorf("staged installer checksum mismatch")
	}
	return path, nil
}

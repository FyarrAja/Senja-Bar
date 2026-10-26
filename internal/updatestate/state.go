// Package updatestate shares cached update metadata without an HTTP dependency.
package updatestate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const Available = 6
const Ready = 7
const PendingFlag uint32 = 0x100

type Record struct {
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
}

func Load(path string) (Record, error) {
	var r Record
	f, err := os.Open(path)
	if err != nil {
		return r, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() > 4096 {
		return r, fmt.Errorf("invalid update cache")
	}
	err = json.NewDecoder(f).Decode(&r)
	return r, err
}
func Save(path string, r Record) error {
	return saveJSON(path, r)
}
func saveJSON(path string, r any) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".update-state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func Encode(phase uint32, pending bool) uint32 {
	if phase > Ready {
		phase = 0
	}
	if pending {
		return phase | PendingFlag
	}
	return phase
}

// Progress packs a bounded percentage (+1; zero means unknown) and downloaded KiB.
func Progress(done, total int64) uint32 {
	done = max(int64(0), min(done, 64<<20))
	percent := uint32(0)
	if total > 0 {
		percent = uint32(min(int64(100), done*100/total)) + 1
	}
	return uint32(done/1024)<<8 | percent
}
func Percent(wire uint32) int { return int(min(wire&255, 101)) - 1 }

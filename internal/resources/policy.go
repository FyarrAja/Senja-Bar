// Package resources implements opt-in background resource management.
package resources

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const IdleDelay = 2 * time.Minute
const TrimCooldown = 30 * time.Minute

type Config struct {
	AllBackground   bool     `json:"allBackground"`
	Paths           []string `json:"paths"`
	ExcludedPaths   []string `json:"excludedPaths,omitempty"`
	Automatic       bool     `json:"automatic"`
	EcoCPU          bool     `json:"ecoCPU"`
	MemoryPercent   int      `json:"memoryPercent"`
	IdleMinutes     int      `json:"idleMinutes"`
	CooldownMinutes int      `json:"cooldownMinutes"`
}

func (c Config) MemoryThreshold() int {
	if c.MemoryPercent >= 75 && c.MemoryPercent <= 95 {
		return c.MemoryPercent
	}
	return 80
}
func (c Config) IdleWait() time.Duration {
	if c.IdleMinutes >= 2 && c.IdleMinutes <= 10 {
		return time.Duration(c.IdleMinutes) * time.Minute
	}
	return IdleDelay
}
func (c Config) TrimWait() time.Duration {
	if c.CooldownMinutes >= 15 && c.CooldownMinutes <= 120 {
		return time.Duration(c.CooldownMinutes) * time.Minute
	}
	return TrimCooldown
}

func Path() string { return filepath.Join(os.Getenv("LOCALAPPDATA"), "Senja", "resources.json") }
func Load() Config {
	var c Config
	b, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		c.AllBackground = true
	}
	if err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}
func Save(c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(Path()), 0700); err != nil {
		return err
	}
	p := Path() + ".tmp"
	if err = os.WriteFile(p, b, 0600); err != nil {
		return err
	}
	return os.Rename(p, Path())
}

// Selected is shared by the engine and GUI so a checked row matches policy.
func (c Config) Selected(path string) bool {
	if path == "" || strings.HasPrefix(path, "installed:") || strings.HasPrefix(path, "store:") {
		return false
	}
	if c.AllBackground {
		for _, p := range c.ExcludedPaths {
			if strings.EqualFold(p, path) {
				return false
			}
		}
		return true
	}
	for _, p := range c.Paths {
		if strings.EqualFold(p, path) {
			return true
		}
	}
	return false
}
func selected(c Config, path string) bool { return c.Selected(path) }

type Candidate struct {
	PID, Parent       uint32
	Path, Name, Title string
	WorkingSet        uint64
	Created, CPU      uint64
	Minimized         bool
	Protected         string
}

// Exclude known timing-sensitive tools even when minimized. This is an extra
// guard, never a claim that executable names identify every possible game.
func sensitive(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	for _, s := range []string{"senja", "samar", "audiodg", "dwm", "explorer", "shellexperience", "startmenuexperience", "searchhost", "textinputhost", "game", "fivem", "gta", "valorant", "cs2", "dota", "fortnite", "roblox", "minecraft", "javaw", "obs", "discord", "steam", "battle", "easyanticheat", "beservice", "vgc", "voicemeeter", "asio", "reaper", "ableton", "fl64", "resolve", "premiere", "msmpeng", "securityhealth", "nvcontainer", "radeon"} {
		if strings.Contains(name, s) {
			return true
		}
	}
	return false
}

func eligible(p Candidate, c Config, audioOK, paused bool, idle time.Duration) bool {
	return selected(c, p.Path) && audioOK && !paused && p.Protected == "" && p.Minimized && !sensitive(p.Path) && idle >= c.IdleWait()
}

// Protect ancestors, descendants, and matching executables of foreground/audio
// seeds. Do not fan out from their ancestors (Explorer would cover all apps).
func protectedFamily(all []Candidate, seeds map[uint32]bool) map[uint32]bool {
	out, descendants := map[uint32]bool{}, map[uint32]bool{}
	paths := map[string]bool{}
	for _, p := range all {
		if seeds[p.PID] {
			descendants[p.PID] = true
			paths[strings.ToLower(p.Path)] = true
		}
	}
	delete(paths, "")
	for changed := true; changed; {
		changed = false
		for _, p := range all {
			if !descendants[p.PID] && (descendants[p.Parent] || paths[strings.ToLower(p.Path)]) {
				descendants[p.PID] = true
				changed = true
			}
		}
	}
	for pid := range descendants {
		out[pid] = true
	}
	for changed := true; changed; {
		changed = false
		for _, p := range all {
			if out[p.PID] && p.Parent != 0 && !out[p.Parent] {
				out[p.Parent] = true
				changed = true
			}
		}
	}
	return out
}

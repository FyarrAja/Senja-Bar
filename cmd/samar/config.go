//go:build windows

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func validStyle(v int) bool { return v >= styleNormal && v <= styleSolid }

func loadConfig() config {
	cfg := makeConfig()
	file, err := os.Open(configPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logEvent("INFO", "config.load", "configuration file does not exist; defaults used")
		} else {
			logError("config.load", "unable to open configuration", err)
		}
		return cfg
	}
	defer file.Close()

	repaired := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "style":
			v, e := strconv.Atoi(value)
			if e != nil {
				logEvent("ERROR", "config.load", fmt.Sprintf("invalid style=%q; using default=%d", value, cfg.style))
				repaired = true
			} else if !validStyle(v) {
				cfg.style = styleGlass
				repaired = true
			} else {
				cfg.style = v
				if v == styleNormal {
					cfg.style = styleSolid
					repaired = true
				}
			}
		case "startup":
			cfg.startup = value == "1" || strings.EqualFold(value, "true")
		case "auto_update":
			cfg.autoUpdate = value == "1" || strings.EqualFold(value, "true")
		case "blur_radius":
			if v, err := strconv.ParseFloat(value, 32); err == nil && v >= 1.0 && v <= 50.0 {
				cfg.blurRadius = float32(v)
			}
		case "rounded":
			cfg.rounded = value == "1" || strings.EqualFold(value, "true")
		}
	}
	if err := scanner.Err(); err != nil {
		logError("config.load", "scanner failed", err)
		repaired = true
	}
	if repaired {
		_ = saveConfig(cfg)
	}
	logEvent("INFO", "config.load", fmt.Sprintf("loaded style=%d startup=%t auto_update=%t blur_radius=%.1f rounded=%t", cfg.style, cfg.startup, cfg.autoUpdate, cfg.blurRadius, cfg.rounded))
	return cfg
}

func saveConfig(cfg config) error {
	if !validStyle(cfg.style) {
		cfg.style = styleGlass
	}
	if err := os.MkdirAll(filepath.Dir(configPath()), 0700); err != nil {
		return err
	}
	if cfg.blurRadius <= 0 {
		cfg.blurRadius = 18.0
	}
	content := fmt.Sprintf("style=%d\nstartup=%d\nauto_update=%d\nblur_radius=%.1f\nrounded=%d\n", cfg.style, boolInt(cfg.startup), boolInt(cfg.autoUpdate), cfg.blurRadius, boolInt(cfg.rounded))
	tmp := configPath() + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, configPath()); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	logEvent("INFO", "config.save", fmt.Sprintf("saved style=%d startup=%t blur_radius=%.1f rounded=%t path=%s", cfg.style, cfg.startup, cfg.blurRadius, cfg.rounded, configPath()))
	return nil
}

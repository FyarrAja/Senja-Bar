//go:build windows

package main

import (
	"syscall"
)

func showTrayMenu(hwnd syscall.Handle, cfg *config) {
	if cfg != nil {
		if !launchCSSMenu(hwnd) {
			showThemedTrayMenu(hwnd, cfg)
		}
	}
}

func selectCommand(hwnd syscall.Handle, command uint32, cfg *config) {
	defer recoverPanic("command")
	switch command {
	case cmdStyleGlass, cmdStyleAcrylic, cmdStyleClear, cmdStyleSolid:
		previous := cfg.style
		switch command {
		case cmdStyleGlass:
			cfg.style = styleGlass
		case cmdStyleAcrylic:
			cfg.style = styleAcrylic
		case cmdStyleClear:
			cfg.style = styleClear
		case cmdStyleSolid:
			cfg.style = styleSolid
		}
		applyStyle(cfg.style)
		if !lastStyleApplyOK {
			cfg.style = previous
			applyStyle(previous)
			lastStyleApplyOK = false
			return
		}
		if err := saveConfig(*cfg); err != nil {
			logError("config.save", "unable to save style", err)
		}
	case cmdRounded:
		previous := cfg.rounded
		cfg.rounded = !cfg.rounded
		applyStyle(cfg.style)
		if !lastStyleApplyOK {
			cfg.rounded = previous
			applyStyle(cfg.style)
			lastStyleApplyOK = false
			return
		}
		if err := saveConfig(*cfg); err != nil {
			logError("config.save", "unable to save rounded setting", err)
		}
	case cmdStartup:
		next := !cfg.startup
		if err := setStartup(next); err != nil {
			showError(appName, err)
			return
		}
		cfg.startup = next
		if err := saveConfig(*cfg); err != nil {
			logError("config.save", "unable to save startup state", err)
		}
	case cmdRefresh:
		newCfg := loadConfig()
		*cfg = newCfg
		applyStyle(cfg.style)
	case cmdCheckUpdates:
		startUpdateCheckManually()
	case cmdAutoUpdates:
		cfg.autoUpdate = !cfg.autoUpdate
		if err := saveConfig(*cfg); err != nil {
			logError("config.save", "unable to save automatic update setting", err)
		}
		if cfg.autoUpdate {
			go checkForUpdates(false)
		}
	case cmdExit:
		requestExit(hwnd)
	}
}

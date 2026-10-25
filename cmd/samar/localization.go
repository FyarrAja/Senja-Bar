//go:build windows

package main

func texts() stringsTable {
	return stringsTable{
		style:        "Style",
		glass:        "Senja Glass",
		acrylic:      "Senja Acrylic",
		clear:        "Senja Bening",
		normal:       "Windows Default",
		rounded:      "Rounded corners",
		openBoot:     "Open at boot",
		refresh:      "Refresh taskbar",
		checkUpdates: "Check for updates",
		autoUpdates:  "Automatic updates",
		exit:         "Exit",
	}
}

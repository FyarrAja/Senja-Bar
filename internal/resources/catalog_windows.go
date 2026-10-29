//go:build windows

package resources

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

type InstalledApp struct{ Name, Path string }

var catalogEnv = regexp.MustCompile(`%[^%]+%`)

func catalogExecutable(value string) string {
	value = catalogEnv.ReplaceAllStringFunc(value, func(token string) string {
		if v := os.Getenv(strings.Trim(token, "%")); v != "" {
			return v
		}
		return token
	})
	value = strings.TrimSpace(os.ExpandEnv(value))
	// DisplayIcon may be quoted or followed by a resource index.
	if strings.HasPrefix(value, `"`) {
		if end := strings.Index(value[1:], `"`); end >= 0 {
			value = value[1 : end+1]
		}
	} else if comma := strings.LastIndex(value, ","); comma >= 0 {
		value = value[:comma]
	}
	if strings.EqualFold(filepath.Ext(value), ".exe") && filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return ""
}

// Enumerate installed desktop applications and current-user Microsoft Store
// packages, independent of running processes. No inventory is transmitted.
func InstalledApps() []InstalledApp {
	apps := []InstalledApp{}
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		for _, view := range []uint32{registry.WOW64_64KEY, registry.WOW64_32KEY} {
			key, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.READ|view)
			if err != nil {
				continue
			}
			names, _ := key.ReadSubKeyNames(-1)
			for _, id := range names {
				sub, err := registry.OpenKey(key, id, registry.READ|view)
				if err != nil {
					continue
				}
				name, _, _ := sub.GetStringValue("DisplayName")
				icon, _, _ := sub.GetStringValue("DisplayIcon")
				sub.Close()
				if strings.TrimSpace(name) == "" {
					continue
				}
				path := catalogExecutable(icon)
				if path == "" {
					path = "installed:" + strings.ToLower(name)
				}
				apps = append(apps, InstalledApp{Name: name, Path: path})
			}
			key.Close()
		}
	}
	// Use the Windows-owned shell path, not an executable from the working dir.
	shell := filepath.Join(os.Getenv("WINDIR"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	script := `[Console]::OutputEncoding=[Text.UTF8Encoding]::new(); $packages=@{}; Get-AppxPackage -ErrorAction SilentlyContinue | ForEach-Object { $packages[$_.PackageFamilyName]=$_ }; $items=@(Get-StartApps -ErrorAction SilentlyContinue | ForEach-Object { $entry=$_; $path=''; $parts=$entry.AppID -split '!',2; if($parts.Count -eq 2 -and $packages.ContainsKey($parts[0])) { $pkg=$packages[$parts[0]]; try { [xml]$manifest=Get-Content -LiteralPath (Join-Path $pkg.InstallLocation 'AppxManifest.xml') -Raw -ErrorAction Stop; $app=$manifest.Package.Applications.Application | Where-Object { $_.Id -eq $parts[1] } | Select-Object -First 1; $executable=[string]$app.Executable; if($executable.EndsWith('.exe')) { $path=Join-Path $pkg.InstallLocation $executable } } catch {} }; if(!$path) { if($parts.Count -eq 2) { $path='store:'+$entry.AppID } elseif([IO.Path]::IsPathRooted($entry.AppID) -and $entry.AppID.EndsWith('.exe')) { $path=$entry.AppID } else { $path='installed:'+$entry.AppID } }; [pscustomobject]@{Name=$entry.Name;Path=$path} }); ConvertTo-Json -InputObject $items -Compress`
	cmd := exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
	if data, err := cmd.Output(); err == nil {
		var packages []InstalledApp
		if json.Unmarshal(data, &packages) == nil {
			apps = append(apps, packages...)
		}
	}
	unique := map[string]bool{}
	out := []InstalledApp{}
	for _, app := range apps {
		key := strings.ToLower(app.Path)
		if unique[key] {
			continue
		}
		unique[key] = true
		out = append(out, app)
	}
	return out
}

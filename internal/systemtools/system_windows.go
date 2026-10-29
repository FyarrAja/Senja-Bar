//go:build windows

// Package systemtools provides measured telemetry and explicit Windows tools.
package systemtools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type Volume struct {
	Name            string
	FreeGB, TotalGB float64
}
type Snapshot struct {
	Model, CPU, GPU, PowerPlan, Battery string
	RAMTotalGB, RAMFreeGB               float64
	CPULoad                             float64
	Volumes                             []Volume
	Updated                             time.Time
	Error                               string
}

func Read(ctx context.Context) Snapshot { return nativeRead(ctx) }

func ReportDir() string { return filepath.Join(os.Getenv("LOCALAPPDATA"), "Senja", "reports") }
func SaveReport(s Snapshot, result string) (string, error) {
	if err := os.MkdirAll(ReportDir(), 0700); err != nil {
		return "", err
	}
	path := filepath.Join(ReportDir(), "Senja-System-"+time.Now().Format("20060102-150405.000")+".txt")
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	err = os.WriteFile(path, []byte("SENJA SYSTEM REPORT\n"+string(b)+"\n\nHASIL PEMERIKSAAN\n"+result+"\n"), 0600)
	return path, err
}

func Run(ctx context.Context, key string) (string, error) {
	switch key {
	case "diagnostics":
		return nativeDiagnostics(ctx)
	case "network":
		return nativeNetwork(ctx)
	case "storage":
		return nativeStorage(ctx)
	case "power-balanced", "power-saving", "power-performance":
		if err := nativePower(strings.TrimPrefix(key, "power-")); err != nil {
			return "", err
		}
		return "Profil daya telah diterapkan melalui Senja.", nil
	}
	return "", fmt.Errorf("alat tidak dikenal: %s", key)
}

func Open(target string) error {
	allowed := map[string]string{"storage": "ms-settings:storagesense", "cleanup": "cleanmgr.exe", "power": "powercfg.cpl", "game": "ms-settings:gaming-gamemode", "gpu": "ms-settings:display-advancedgraphics", "network-settings": "ms-settings:network-status"}
	value, ok := allowed[target]
	if !ok {
		return fmt.Errorf("alat tidak dikenal")
	}
	return shell("open", value, "", 1)
}

func shell(verb, file, args string, show int) error {
	v, _ := syscall.UTF16PtrFromString(verb)
	f, _ := syscall.UTF16PtrFromString(file)
	a, _ := syscall.UTF16PtrFromString(args)
	r, _, err := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(f)), uintptr(unsafe.Pointer(a)), 0, uintptr(show))
	if r <= 32 {
		return fmt.Errorf("Windows menolak menjalankan alat (%d): %v", r, err)
	}
	return nil
}

type Job struct{ Log, Done string }

func (j Job) Poll() (string, bool) {
	b, _ := os.ReadFile(j.Log)
	_, err := os.Stat(j.Done)
	return string(b), err == nil
}

// Plans contain only fixed actions. No command text or adapter IDs come from UI strings.
func adminScript(key string) (string, error) {
	switch key {
	case "sfc":
		return `& sfc.exe /scannow; if($LASTEXITCODE -ne 0){throw "SFC exit $LASTEXITCODE; periksa hasil pemindaian."}`, nil
	case "verify":
		return `& dism.exe /Online /Cleanup-Image /CheckHealth; if($LASTEXITCODE -ne 0){throw "DISM exit $LASTEXITCODE"}; & sfc.exe /verifyonly; if($LASTEXITCODE -ne 0){throw "SFC exit $LASTEXITCODE"}`, nil
	case "dism":
		return `& dism.exe /Online /Cleanup-Image /RestoreHealth; if($LASTEXITCODE -ne 0){throw "DISM exit $LASTEXITCODE"}`, nil
	case "ssd":
		return `Get-Volume | Where-Object {$_.DriveLetter -and $_.DriveType -eq 'Fixed'} | ForEach-Object {Optimize-Volume -DriveLetter $_.DriveLetter -ReTrim -Verbose -ErrorAction Stop}`, nil
	case "winsock":
		return `& netsh.exe winsock reset; if($LASTEXITCODE -ne 0){throw "Winsock exit $LASTEXITCODE"}; 'Windows perlu direstart untuk menerapkan reset Winsock.'`, nil
	case "dns-cloudflare", "dns-google", "dns-auto":
		change := `Set-DnsClientServerAddress -InterfaceIndex $idx -ResetServerAddresses -ErrorAction Stop`
		if key == "dns-cloudflare" {
			change = `Set-DnsClientServerAddress -InterfaceIndex $idx -ServerAddresses @('1.1.1.1','1.0.0.1') -ErrorAction Stop`
		}
		if key == "dns-google" {
			change = `Set-DnsClientServerAddress -InterfaceIndex $idx -ServerAddresses @('8.8.8.8','8.8.4.4') -ErrorAction Stop`
		}
		return `$r=@(Get-NetRoute -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' | Sort-Object RouteMetric); if(!$r.Count){throw 'Tidak ada koneksi default'}; $idx=$r[0].InterfaceIndex; $old=Get-DnsClientServerAddress -InterfaceIndex $idx -AddressFamily IPv4; $backup=Join-Path $PSScriptRoot ('dns-before-'+(Get-Date -Format 'yyyyMMdd-HHmmss')+'.json'); $old|ConvertTo-Json -Depth 5|Set-Content -LiteralPath $backup; ` + change + `; Get-DnsClientServerAddress -InterfaceIndex $idx -AddressFamily IPv4 | Format-List; 'Cadangan DNS: '+$backup`, nil
	}
	return "", fmt.Errorf("aksi admin tidak dikenal")
}

func StartAdmin(key string) (Job, error) {
	body, err := adminScript(key)
	if err != nil {
		return Job{}, err
	}
	if err = os.MkdirAll(ReportDir(), 0700); err != nil {
		return Job{}, err
	}
	base := filepath.Join(ReportDir(), "job-"+time.Now().Format("20060102-150405.000000"))
	job := Job{Log: base + ".log", Done: base + ".done"}
	// Fixed local filenames cannot contain single quotes on this path's generated suffix.
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	script := `$ErrorActionPreference='Stop'; $log=` + quote(job.Log) + `; try { & { ` + body + ` } *>&1 | Out-File -LiteralPath $log -Encoding utf8; 'SELESAI' | Add-Content -LiteralPath $log } catch { $_ | Out-String | Add-Content -LiteralPath $log; 'GAGAL' | Add-Content -LiteralPath $log } finally {'done' | Set-Content -LiteralPath ` + quote(job.Done) + `}`
	file := base + ".ps1"
	if err = os.WriteFile(file, []byte("\xef\xbb\xbf"+script), 0600); err != nil {
		return Job{}, err
	}
	// UAC is shown only after the user selects and confirms this exact action.
	exe := filepath.Join(os.Getenv("WINDIR"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	err = shell("runas", exe, `-NoProfile -NonInteractive -ExecutionPolicy RemoteSigned -File "`+file+`"`, 0)
	return job, err
}

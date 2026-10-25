# Senja Bar

A lightweight Windows taskbar utility with a transparent glass control panel.

[Download the latest release](https://github.com/FyarrAja/Senja-Bar/releases/latest) · [Source code](https://github.com/FyarrAja/Senja-Bar)

## Features

- **Kaca:** neutral, wallpaper-colored glass with soft live taskbar blur.
- **Akrilik:** a denser acrylic taskbar material.
- **Bening:** clear taskbar without tint or a top border.
- **Solid:** a neutral charcoal taskbar.
- Rounded taskbar corners, launch at Windows startup and refresh controls.
- Tray popup with a moving mode indicator, smooth switches, and whole-window fades.
- Editable local HTML/CSS menu. WebView2 runs only while the menu is open.
- **Pembaruan** opens this repository's latest GitHub Release.
- Settings are saved per user; exiting restores the Windows taskbar appearance.

## Install

1. Download **Senja-Bar-windows-x64.zip** from [Releases](https://github.com/FyarrAja/Senja-Bar/releases/latest).
2. Extract the entire archive into a writable folder. Keep the DLLs, resources.pri and Assets next to Senja.exe.
3. Run **Senja.exe**. Right-click the sunset tray icon to open the menu.

Windows x64 and Microsoft Edge WebView2 Runtime are required. Windows 11 is the primary target. A native fallback is provided when WebView2 cannot start; Windows 10 material compatibility has not been fully verified. Builds are not Authenticode-signed.

Enable **Mulai bersama Windows** in the menu for startup. Settings live in `%LOCALAPPDATA%\Senja\settings.ini`.

## Updating

Click **Pembaruan**, or visit `https://github.com/FyarrAja/Senja-Bar/releases/latest`. This stable URL follows the newest published release. Exit Senja, extract the new ZIP into its application folder, and run Senja.exe again. Per-user settings are preserved. The application does not silently download or install executables.

## Build from source

Install Go 1.23 or newer, then run in PowerShell:

```powershell
go test ./cmd/samar
.\build.ps1
.\scripts\package-release.ps1 -Version 1.0.0
```

The Windows resources are checked in. To regenerate icons, install Node.js and `sharp`, then run `node scripts/build-icons.cjs`. To regenerate executable metadata, run `scripts/generate-resources.ps1`. Go resource generator dependencies are pinned in that script.

The legacy internal source folder is `cmd/samar`; the application is branded Senja Bar. `Assets/menu.css` overrides the embedded stylesheet and is reloaded when the menu opens. Engine code is native Go/Win32; menu code uses WebView2.

## Performance

The engine waits for Windows events and does not run an idle animation or full-GC polling loop. Menu helper processes terminate after closing. Local measurements reached roughly 1–2 MiB of private active working set after startup trimming; this is not a hard limit or total committed memory. WebView2 while open and Explorer-side components are additional costs. Usage varies by Windows version and activity.

## Open source and attribution

Senja Bar is distributed under **GPL-3.0**; see [LICENSE](LICENSE). It is a modified derivative of [LowBar](https://github.com/chematic/LowBar), originally Apache-2.0. Original attribution, the Apache license and branding notice are retained.

Explorer-side binaries are unmodified files from [TranslucentTB 2026.2](https://github.com/TranslucentTB/TranslucentTB/releases/tag/2026.2). Their hashes and corresponding source URL are recorded in [docs/DEPENDENCIES.json](docs/DEPENDENCIES.json). The matching source archive is also attached to the Senja release. See [NOTICE](NOTICE), [third-party notices](docs/THIRD_PARTY_NOTICES.txt) and `docs/licenses`.

Senja Bar is independent of LowBar, TranslucentTB, Xiaomi and Apple; UI inspiration does not imply endorsement.

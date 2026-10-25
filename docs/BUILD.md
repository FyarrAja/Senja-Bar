# Build Senja Bar

Windows x64 and Go 1.23+ (WebView2 is required only for the legacy CSS comparison):

```powershell
go test ./...
.\build.ps1
.\scripts\package-release.ps1 -Version 1.1.2
```

`package-release.ps1` generates executable metadata, validates dependency hashes, and writes Senja-Bar.exe, the portable ZIP and SHA256SUMS.txt under dist/. Tagged GitHub builds publish these assets plus the matching TranslucentTB source archive.

The legacy internal source directory is `cmd/samar`. The engine is Go/Win32; the on-demand menu is GDI+ with per-pixel alpha. The separate `cmd/updater` executable performs HTTP/download work only when requested; both build scripts include it. `Assets/menu.css` overrides the embedded CSS only on the legacy WebView2 path. Packaging initializes that override from the canonical `cmd/samar/ui/menu.css`.

To regenerate executable resources use `scripts/generate-resources.ps1`. To regenerate icons use Node.js with sharp and `node scripts/build-icons.cjs`.

`Senja.exe --native-preview` opens the native panel with simulated settings. `--native-check` uses its controls against the running engine; `--native-menu` exercises the actual tray positioning and dismissal path. `--css-preview` and `--css-check` retain the older WebView2 comparison. `--material-preview --background-only` opens a controlled multicolor surface; append `--light` for a bright contrast test. The browser gallery is a design reference, not proof of native backdrop behavior.

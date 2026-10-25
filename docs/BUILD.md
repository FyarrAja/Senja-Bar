# Build Senja Bar

Windows x64, Go 1.23+ and WebView2 Runtime:

```powershell
go test ./cmd/samar ./cmd/launcher
.\build.ps1
.\scripts\package-release.ps1 -Version 1.1.1
```

`package-release.ps1` generates executable metadata, validates dependency hashes, and writes Senja-Bar.exe, the portable ZIP and SHA256SUMS.txt under dist/. Tagged GitHub builds publish these assets plus the matching TranslucentTB source archive.

The legacy internal source directory is `cmd/samar`. The engine is Go/Win32; the on-demand menu is WebView2. `Assets/menu.css` overrides the embedded CSS when present. Keep both stylesheets synchronized when changing the default UI.

To regenerate executable resources use `scripts/generate-resources.ps1`. To regenerate icons use Node.js with sharp and `node scripts/build-icons.cjs`.

`Senja.exe --css-preview` opens the real menu with local-only simulated settings. `Senja.exe --css-check` exercises the real engine. `Senja.exe --material-preview --background-only` opens a controlled multicolor test surface. The browser preview uses the same HTML/CSS/JS and substitutes CSS blur for native Windows blur.

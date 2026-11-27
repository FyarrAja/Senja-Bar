#requires -Version 5.1
param([switch]$Run, [string]$GoExe = 'go', [string]$CargoExe = 'cargo')
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    if (-not (Test-Path -LiteralPath native/bin/SenjaNoise.dll)) { throw 'Build the neural audio module first; see docs/BUILD.md' }
    Copy-Item -LiteralPath native/bin/SenjaNoise.dll -Destination SenjaNoise.dll -Force
    Copy-Item -LiteralPath native/rnnoise/RNNoise-LICENSE.txt -Destination RNNoise-LICENSE.txt -Force
    & $GoExe build -trimpath -ldflags='-s -w -H=windowsgui' -o Senja.exe ./cmd/samar
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
    & $CargoExe build --release --manifest-path rust/senja-updater/Cargo.toml --locked
    if ($LASTEXITCODE -ne 0) { throw 'Rust updater build failed' }
    Copy-Item -LiteralPath rust/senja-updater/target/release/senja-updater.exe -Destination Senja-Update.exe -Force
    if (-not (Test-Path -LiteralPath Assets/menu.css)) {
        Copy-Item -LiteralPath cmd/samar/ui/menu.css -Destination Assets/menu.css
    }
    if ($Run) { Start-Process -FilePath (Join-Path $PSScriptRoot 'Senja.exe') -WorkingDirectory $PSScriptRoot }
} finally { Pop-Location }

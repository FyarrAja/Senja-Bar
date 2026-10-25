#requires -Version 5.1
param([switch]$Run, [string]$GoExe = 'go')
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    & $GoExe build -trimpath -ldflags='-s -w -H=windowsgui' -o Senja.exe ./cmd/samar
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed' }
    & $GoExe build -trimpath -ldflags='-s -w -H=windowsgui' -o Senja-Update.exe ./cmd/updater
    if ($LASTEXITCODE -ne 0) { throw 'Updater build failed' }
    if (-not (Test-Path -LiteralPath Assets/menu.css)) {
        Copy-Item -LiteralPath cmd/samar/ui/menu.css -Destination Assets/menu.css
    }
    if ($Run) { Start-Process -FilePath (Join-Path $PSScriptRoot 'Senja.exe') -WorkingDirectory $PSScriptRoot }
} finally { Pop-Location }

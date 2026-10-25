#requires -Version 5.1
param([Parameter(Mandatory=$true)][ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version, [string]$GoExe='go', [string]$OutDir='dist')
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
Push-Location $root
try {
    $out=[IO.Path]::GetFullPath((Join-Path $root $OutDir))
    New-Item -ItemType Directory -Path $out -Force | Out-Null
    $stage=Join-Path $out ('package-'+[guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $stage -Force | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $stage 'Assets') -Force | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $stage 'docs') -Force | Out-Null
    & (Join-Path $PSScriptRoot 'generate-resources.ps1') -GoExe $GoExe -Version $Version
    & $GoExe build -trimpath -ldflags="-s -w -H=windowsgui -X main.buildVersion=$Version" -o (Join-Path $stage 'Senja.exe') ./cmd/samar
    if ($LASTEXITCODE -ne 0) { throw 'Release build failed' }
    & $GoExe build -trimpath -ldflags="-s -w -H=windowsgui" -o (Join-Path $stage 'Senja-Update.exe') ./cmd/updater
    if ($LASTEXITCODE -ne 0) { throw 'Updater build failed' }
    foreach ($file in @('ExplorerTAP.dll','ExplorerHooks.dll','ProgramLog.dll','Xaml.dll','resources.pri','senja.ico','LICENSE','NOTICE','TRADEMARKS.md','README.md','CHANGELOG.md')) {
        Copy-Item -LiteralPath (Join-Path $root $file) -Destination $stage
    }
    foreach ($file in @('icon.svg','icon.png','senja.ico')) {
        Copy-Item -LiteralPath (Join-Path $root ('Assets/'+$file)) -Destination (Join-Path $stage 'Assets')
    }
    # The editable installed override must start from the embedded source.
    # Shipping the repository's old override silently hides new CSS fixes.
    Copy-Item -LiteralPath (Join-Path $root 'cmd/samar/ui/menu.css') -Destination (Join-Path $stage 'Assets/menu.css')
    foreach ($file in @('THIRD_PARTY_NOTICES.txt','DEPENDENCIES.json','BUILD.md','ARCHITECTURE.md','preview.png')) {
        Copy-Item -LiteralPath (Join-Path $root ('docs/'+$file)) -Destination (Join-Path $stage 'docs')
    }
    Copy-Item -LiteralPath (Join-Path $root 'docs/licenses') -Destination (Join-Path $stage 'docs') -Recurse
    $manifest=Get-Content -LiteralPath docs/DEPENDENCIES.json -Raw | ConvertFrom-Json
    foreach ($file in $manifest.files.PSObject.Properties) {
        if ((Get-FileHash -LiteralPath (Join-Path $stage $file.Name) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $file.Value) { throw "Dependency hash mismatch: $($file.Name)" }
    }
    $zip=Join-Path $out 'Senja-Bar-windows-x64.zip'
    Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $zip -Force
    # One downloadable EXE: extract the portable bundle once per version,
    # start the native engine, and exit the launcher immediately.
    Copy-Item -LiteralPath $zip -Destination (Join-Path $root 'cmd/launcher/payload.zip') -Force
    Copy-Item -LiteralPath (Join-Path $root 'cmd/samar/resource.syso') -Destination (Join-Path $root 'cmd/launcher/resource.syso') -Force
    & $GoExe test ./cmd/launcher
    if ($LASTEXITCODE -ne 0) { throw 'Launcher tests failed' }
    & $GoExe build -tags bundled -trimpath -ldflags="-s -w -H=windowsgui -X main.buildVersion=$Version" -o (Join-Path $out 'Senja-Bar.exe') ./cmd/launcher
    if ($LASTEXITCODE -ne 0) { throw 'Standalone EXE build failed' }
    $sourceZip=Join-Path $out ('TranslucentTB-'+$manifest.version+'-source.zip')
    Invoke-WebRequest -Uri $manifest.source -OutFile $sourceZip
    $hash=(Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
    $sourceHash=(Get-FileHash -LiteralPath $sourceZip -Algorithm SHA256).Hash.ToLowerInvariant()
    $exeHash=(Get-FileHash -LiteralPath (Join-Path $out 'Senja-Bar.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -LiteralPath (Join-Path $out 'SHA256SUMS.txt') -Value @("$exeHash  Senja-Bar.exe", "$hash  Senja-Bar-windows-x64.zip", "$sourceHash  $([IO.Path]::GetFileName($sourceZip))") -Encoding ascii
    Write-Output $zip
} finally { Pop-Location }

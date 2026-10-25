#requires -Version 5.1
param([string]$GoExe = 'go', [ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version = '1.0.0')
$ErrorActionPreference = 'Stop'
Push-Location (Join-Path $PSScriptRoot '../cmd/samar')
try {
    $parts=$Version.Split('.')
    $info=Get-Content -LiteralPath versioninfo.json -Raw | ConvertFrom-Json
    foreach ($kind in @('FileVersion','ProductVersion')) {
        $info.FixedFileInfo.$kind.Major=[int]$parts[0]
        $info.FixedFileInfo.$kind.Minor=[int]$parts[1]
        $info.FixedFileInfo.$kind.Patch=[int]$parts[2]
        $info.StringFileInfo.$kind="$Version.0"
    }
    $info | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath versioninfo.json -Encoding utf8
    & $GoExe run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.5.0 -64 -o resource.syso
    if ($LASTEXITCODE -ne 0) { throw 'Windows resource generation failed' }
} finally { Pop-Location }



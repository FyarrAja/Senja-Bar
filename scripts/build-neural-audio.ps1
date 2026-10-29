#requires -Version 5.1
param([Parameter(Mandatory=$true)][string]$ZigExe,[string]$OutDir='native/bin')
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
$native=Join-Path $root 'native/rnnoise'
$source=Join-Path $native 'upstream/src'
$out=[IO.Path]::GetFullPath((Join-Path $root $OutDir))
$objects=Join-Path $out 'rnnoise-objects'
New-Item -ItemType Directory -Path $objects -Force | Out-Null
$common=@('cc','-target','x86_64-windows-gnu','-std=c99','-O3','-DNDEBUG','-DRNNOISE_BUILD','-DRNN_ENABLE_X86_RTCD','-DCPU_INFO_BY_ASM',('-I'+$source),('-I'+(Join-Path $native 'upstream/include')))
$files=@('denoise.c','rnn.c','pitch.c','kiss_fft.c','celt_lpc.c','nnet.c','nnet_default.c','parse_lpcnet_weights.c','rnnoise_data.c','rnnoise_tables.c','x86/x86_dnn_map.c','x86/x86cpu.c','x86/nnet_sse4_1.c','x86/nnet_avx2.c')
$built=@()
foreach($file in $files) {
  $object=Join-Path $objects (($file -replace '[/.]','_')+'.o')
  $flags=@()
  if ($file -eq 'x86/nnet_sse4_1.c') {$flags=@('-msse4.1')}
  if ($file -eq 'x86/nnet_avx2.c') {$flags=@('-mavx2','-mfma')}
  & $ZigExe @common @flags -c (Join-Path $source $file) -o $object
  if ($LASTEXITCODE -ne 0) {throw "RNNoise compilation failed: $file"}
  $built+=$object
}
$dll=Join-Path $out 'SenjaNoise.dll'
& $ZigExe @common -shared (Join-Path $native 'senja_noise.c') @built -o $dll -lm
if ($LASTEXITCODE -ne 0) {throw 'SenjaNoise link failed'}
Copy-Item -LiteralPath (Join-Path $native 'RNNoise-LICENSE.txt') -Destination (Join-Path $out 'RNNoise-LICENSE.txt') -Force
Get-FileHash -LiteralPath $dll -Algorithm SHA256

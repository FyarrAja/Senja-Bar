param([Parameter(Mandatory=$true)][string]$WavePath)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Speech
$voice=New-Object System.Speech.Synthesis.SpeechSynthesizer
try {
  $format=New-Object System.Speech.AudioFormat.SpeechAudioFormatInfo(48000,[System.Speech.AudioFormat.AudioBitsPerSample]::Sixteen,[System.Speech.AudioFormat.AudioChannel]::Mono)
  $voice.SetOutputToWaveFile($WavePath,$format)
  $voice.Speak('This is a quiet spoken sentence. My voice should remain clear while I type on the keyboard. The volume must stay steady between words and at the end of this sentence.')
} finally {$voice.Dispose()}
Get-Item -LiteralPath $WavePath | Select-Object Name,Length

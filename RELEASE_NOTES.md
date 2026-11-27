**[Unduh Senja-Bar.exe](https://github.com/FyarrAja/Senja-Bar/releases/download/v1.3.1/Senja-Bar.exe)**

## Senja Bar 1.3.1

- Mic lokal: kurangi noise sisa yang terus terdengar setelah ucapan berhenti pada mode RNNoise + suara otomatis. Tes regresi menjaga suku kata pelan dan jeda singkat tetap terdengar.
- Proses resident: hentikan pemindaian proses/audio tiap 10 detik saat aturan otomatis dan EcoCPU tidak aktif. Aksi manual tetap berjalan.
- Updater singkat dipindah ke Rust dengan alur unduh, SHA-256, staging, dan konfirmasi manual yang kompatibel. Engine taskbar, panel dan mic masih Go.

**Batasan mic:** perbaikan ini berlaku pada monitor lokal Senja. Hasil olahan belum diteruskan ke input Discord/FiveM. Memilih mic fisik yang sama di Discord/FiveM tetap memakai suara mic mentah. Monitoring memakai output default Windows; gunakan headset. Belum ada acoustic echo cancellation; tes sintetis tidak membuktikan kualitas di semua perangkat dan ruangan.

## English

Reduces residual noise after speech in the local RNNoise + Smart Voice monitor while preserving quiet syllables in regression tests. The resident Go engine no longer polls processes/audio every ten seconds when automatic resource rules are off. The short-lived updater is now Rust; the taskbar engine, panel and mic remain Go.

**Microphone processing is local only. No processed-audio routing into Discord/FiveM is included.** Selecting the same physical microphone there bypasses Senja processing. Use headphones for monitoring; processing/device latency remains. Acoustic echo cancellation and complete noise/other-speaker removal are not provided.

Validation: full Go suite, Rust updater/protocol tests, bundled RNNoise synthetic tests, launcher tests and five menu-command tests. Real GitHub metadata and checksum-verified staging were tested without executing an installer. No subjective room-audio or Discord/FiveM routing claim is made.

Windows x64; primarily tested on Windows 11. Executables are unsigned. Resource controls do not guarantee higher FPS. Existing preferences are preserved. RNNoise license and other dependency notices are included; source and build instructions are in the repository.

[Guide and source](https://github.com/FyarrAja/Senja-Bar#readme)

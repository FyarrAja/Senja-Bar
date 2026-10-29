**[Unduh Senja-Bar.exe](https://github.com/FyarrAja/Senja-Bar/releases/download/v1.3.0/Senja-Bar.exe)**

## Senja Bar 1.3.0

- RAM & CPU: pengelolaan aplikasi latar, trim working set, EcoQoS dan aturan otomatis dengan perlindungan aplikasi aktif/game/audio.
- Sistem: pemantauan perangkat, diagnostik, laporan lokal dan pemeliharaan Windows yang dipilih pengguna.
- Mic lokal: RNNoise, preset berikon, karakter bass dan penguatan terkendali. Satu toggle untuk mulai dan menghentikan audio sepenuhnya.
- Panel native lebih rapi, preset yang jelas, navigasi keyboard dan meter tanpa redraw 120Hz terus-menerus.

**Batasan mic:** belum ada perangkat input virtual Senja. Hasil olahan belum masuk ke Discord/FiveM atau aplikasi lain. Monitoring memakai output default Windows; gunakan headset. Ada jeda pemrosesan, belum ada acoustic echo cancellation, dan hasil peredaman bergantung perangkat/lingkungan. Tidak menjamin semua noise, ketikan, gema atau suara orang lain hilang.

## English

Adds protected background RAM/CPU controls, native system monitoring/diagnostics and bundled local RNNoise microphone processing. Preset icons/descriptions and a single self-monitor switch simplify controls; audio levels no longer keep a full-panel 120Hz animation running.

**Microphone processing is local only. No Senja virtual input or processed-audio routing into other applications is included.** Use headphones for monitoring; processing/device latency remains. Acoustic echo cancellation and complete noise/other-speaker removal are not provided.

Validation: full Go suite, bundled RNNoise streaming/dependency tests, launcher tests and five menu-command tests. Opt-in device tests verified audio startup and complete worker shutdown. These do not establish subjective room audio quality or remote-call results.

Windows x64; primarily tested on Windows 11. Executables are unsigned. Resource controls do not guarantee higher FPS. Existing preferences are preserved. RNNoise license and other dependency notices are included; source and build instructions are in the repository.

[Guide and source](https://github.com/FyarrAja/Senja-Bar#readme)
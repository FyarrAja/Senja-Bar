**[Unduh Senja-Bar.exe](https://github.com/FyarrAja/Senja-Bar/releases/download/v1.3.4/Senja-Bar.exe)**

## Senja Bar 1.3.4

- Mode **Kirim ke Discord/FiveM** dan **Dengar suara sendiri** kini menunggu sekitar 50 ms audio siap sebelum mulai memutar, agar jeda penjadwalan singkat tidak langsung memutus suara. Ada tambahan jeda awal.
- Thread audio memakai prioritas Windows MMCSS `Audio` saat tersedia agar lebih tahan terhadap beban game.
- Suara sintetis 48 kHz dengan ketukan keyboard diuji melalui RNNoise dan DSP. Setelah jeda model sekitar 30 ms diperhitungkan, tes tidak menemukan suku kata lemah akibat pemrosesan pada sampel tersebut.

**Batasan:** Perbaikan ini diuji dengan suara sintetis dan perangkat VB-CABLE lokal, belum dengan rekaman mic kamu atau panggilan Discord/FiveM. Patah-patah akibat penundaan lebih lama, perangkat capture, atau jaringan masih mungkin terjadi. Pilih `Senja Microphone`/`CABLE Output` sebagai input aplikasi; memilih mic fisik melewati Senja. Gunakan headset; belum ada acoustic echo cancellation atau pemisahan pembicara.

## English

Version 1.3.4 primes both the WASAPI VB-CABLE route and WinMM self-monitor with roughly 50 ms of audio before starting playback. The audio worker registers with Windows MMCSS `Audio` when available. Local tests used generated 48 kHz speech and synthetic typing through RNNoise/DSP, accounting for the model's 30 ms delay. These changes add startup latency and aim to reduce short scheduling dropouts.

Senja does not install a virtual driver or switch the input in other apps. Select the cable's recording side (`CABLE Output`, possibly renamed `Senja Microphone`) in Discord/FiveM. Real call reception, the user's microphone, and long stalls have not been verified. Headphones remain necessary for local monitoring; acoustic echo cancellation and complete background-noise removal are not provided.

Windows x64; primarily tested on Windows 11. Executables are unsigned. Dependency notices, source and build instructions are in the repository.

[Guide and source](https://github.com/FyarrAja/Senja-Bar#readme)

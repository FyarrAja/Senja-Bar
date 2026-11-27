**[Unduh Senja-Bar.exe](https://github.com/FyarrAja/Senja-Bar/releases/download/v1.3.3/Senja-Bar.exe)**

## Senja Bar 1.3.3

- Memperbaiki mode **Kirim ke Discord/FiveM** yang gagal dengan `waveOutOpen ... already in use (4)` saat FiveM berjalan. Senja sekarang memakai WASAPI shared mode untuk mengirim audio olahan ke VB-CABLE yang sudah terpasang.
- Satu lonjakan deteksi ucapan akibat ketukan keyboard tidak lagi langsung membuka gate selama 240 ms. Ucapan pelan dan jeda pendek tetap dijaga oleh tes regresi.
- Di Discord/FiveM, pilih input `Senja Microphone` (nama VB-CABLE di PC ini) atau `CABLE Output`. Memilih mic fisik langsung tetap melewati pemrosesan Senja.

**Batasan:** Senja tidak memasang driver virtual sendiri dan tidak otomatis mengubah pilihan mic Discord/FiveM. Pembukaan WASAPI shared dan aliran paket senyap ke VB-CABLE lulus tes lokal saat FiveM berjalan, tetapi penerimaan suara dalam panggilan belum diverifikasi. Ketikan saat bicara, kebocoran akustik dari speaker, dan noise lain dapat tetap terdengar; gunakan headset. Belum ada acoustic echo cancellation atau pemisahan pembicara.

## English

Version 1.3.3 fixes the VB-CABLE route failing with WinMM `waveOutOpen` error 4 while FiveM is running. The route now uses WASAPI shared rendering and converts processed mono PCM to the cable's stereo float mix format. A single false-positive speech frame from a keyboard transient no longer opens the closed neural gate for 240 ms. Select the cable's recording side (`CABLE Output`, possibly renamed `Senja Microphone`) as the microphone in Discord/FiveM. Routing and local self-monitoring remain exclusive.

Senja does not install a virtual driver or switch the input in other apps. Opening and sending silent packets through the installed cable passed locally with FiveM running, but real call reception and acoustic quality have not been verified. Headphones remain necessary for local monitoring; acoustic echo cancellation and complete background-noise removal are not provided.

Windows x64; primarily tested on Windows 11. Executables are unsigned. Dependency notices, source and build instructions are in the repository.

[Guide and source](https://github.com/FyarrAja/Senja-Bar#readme)

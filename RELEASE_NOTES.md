**[Unduh Senja-Bar.exe](https://github.com/FyarrAja/Senja-Bar/releases/download/v1.3.2/Senja-Bar.exe)**

## Senja Bar 1.3.2

- Tombol baru **Kirim ke Discord/FiveM** mengirim suara olahan Senja ke `CABLE Input` milik VB-CABLE yang sudah terpasang. Di Discord/FiveM, pilih sisi rekamannya: `CABLE Output`, atau `Senja Microphone` jika namanya sudah diganti seperti di PC ini.
- Mode kirim mematikan monitor lokal supaya suara tidak diputar ke speaker/headset oleh Senja. Mematikannya menghentikan tangkapan dan rute.
- Mic virtual yang namanya terpotong tidak lagi diterima sebagai sumber fisik. Kesalahan VB-CABLE kini ditampilkan jelas.

**Batasan:** Senja tidak memasang driver virtual sendiri dan tidak otomatis mengubah pilihan mic Discord/FiveM. Memilih mic fisik di sana tetap melewati pemrosesan Senja. Perangkat VB-CABLE lokal menerima format 48 kHz mono PCM16 dalam tes perangkat, tetapi penerimaan suara dalam panggilan Discord/FiveM belum diverifikasi. Kebocoran akustik dari speaker dan noise saat bicara dapat tetap terjadi; gunakan headset. Belum ada acoustic echo cancellation atau pemisahan pembicara.

## English

Version 1.3.2 adds an explicit route from Senja's processed microphone stream to an already installed VB-CABLE `CABLE Input` playback endpoint. Select its recording side (`CABLE Output`, possibly renamed `Senja Microphone`) as the microphone in Discord/FiveM. Routing and local self-monitoring are exclusive. Truncated virtual capture names are rejected as physical sources, and cable errors are visible.

Senja does not install a virtual driver or switch the input in other apps. The installed cable accepted a 48 kHz mono PCM16 format query, but real Discord/FiveM call reception and acoustic quality have not been verified. Headphones remain necessary for local monitoring; acoustic echo cancellation and complete background-noise removal are not provided.

Windows x64; primarily tested on Windows 11. Executables are unsigned. Dependency notices, source and build instructions are in the repository.

[Guide and source](https://github.com/FyarrAja/Senja-Bar#readme)

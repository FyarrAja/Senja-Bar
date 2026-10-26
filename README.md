# Senja Bar

Empat tampilan taskbar Windows, dengan pengaturan dari system tray.

Four Windows taskbar styles, controlled from the system tray.

**[Unduh / Download Senja-Bar.exe](https://github.com/FyarrAja/Senja-Bar/releases/latest/download/Senja-Bar.exe)**

[Release](https://github.com/FyarrAja/Senja-Bar/releases/latest) · [Changelog](CHANGELOG.md) · [Build from source](docs/BUILD.md) · [English](#english)

<p align="center">
  <img src="docs/preview.png" width="300" alt="Menu native Senja pada latar uji gelap">
  <img src="docs/preview-light.png" width="300" alt="Menu native Senja pada latar uji terang">
</p>

*Menu native pada latar uji polos, ditangkap dari aplikasi v1.1.5. Gambar ini memperlihatkan panel pengaturan; efek taskbar tidak ditampilkan.*

## Tampilan & fitur

| Mode | Tampilan taskbar |
| --- | --- |
| **Kaca** | Blur dengan tint yang menyesuaikan tema dan nuansa latar. |
| **Akrilik** | Material akrilik dengan tint lebih pekat. |
| **Bening** | Transparan penuh. |
| **Solid** | Warna opak yang mengikuti tema Windows dan nuansa latar. |

Adaptasi warna memakai sampel di sekitar taskbar pada tiap monitor. Hasilnya dapat berbeda menurut latar dan versi Windows.

- Pilihan sudut taskbar membulat dan mulai bersama Windows.
- Animasi pemilihan mode, toggle, klik tombol, serta buka–tutup panel.
- Segarkan taskbar dan pasang pembaruan dari aplikasi.
- Mengganti mode tidak menutup panel. Klik di luar atau tutup tray untuk menutupnya.
- **Keluar** menghentikan Senja dan mengembalikan tampilan taskbar Windows.

## Mulai pakai

1. Unduh **Senja-Bar.exe** dari tautan di atas, lalu jalankan.
2. File pendamping disiapkan di `%LOCALAPPDATA%\Senja\app` tanpa akses admin. Launcher keluar setelah Senja berjalan.
3. Klik kanan ikon Senja di system tray. Jika tersembunyi, buka panah ikon tray terlebih dahulu.

Pengaturan disimpan di `%LOCALAPPDATA%\Senja\settings.ini`. Paket ZIP portable juga tersedia di halaman release.

## Alur pembaruan

Senja mengecek release stabil saat mulai, lalu setiap 6 jam. Jika koneksi gagal, pengecekan dicoba lagi setelah 30 menit. Pengecekan hanya membaca informasi versi, tanpa mengunduh EXE.

- **Ada update:** notifikasi Windows sekali untuk setiap versi baru, serta badge merah **Baru** pada tombol **Perbarui**, dengan tiga denyut singkat saat ditampilkan. Notifikasi mengikuti pengaturan Windows.
- **Sudah terbaru:** tombol menampilkan versi terpasang dan tidak aktif.
- **Belum terhubung:** Senja mencoba lagi otomatis; tidak ada tombol cek manual.

Selama mengunduh, pengaturan bawah dan tombol cepat berganti sementara menjadi panel progres yang lebih besar. Kontrol kembali setelah transfer selesai. Panel menampilkan **terunduh / total MB**, kecepatan rata-rata **MB/s**, estimasi waktu tersisa, persentase, dan bar progres. Ukuran total dibaca dari respons unduhan, bukan ditebak. Jika total tidak tersedia, tampil `-- MB` dan waktu belum diketahui. Kecepatan serta estimasi mulai tampil setelah ada pengukuran transfer.

Menekan **Perbarui** mengunduh EXE, memverifikasi checksum SHA-256, lalu memasangnya **tanpa konfirmasi tambahan**. Senja lama ditutup dan versi baru berjalan. Folder instalasi lama dibersihkan setelah versi baru siap; file yang dikunci Windows dicoba lagi nanti. Pengaturan, preferensi startup, dan EXE unduhan pribadi dipertahankan. Tidak perlu browser atau restart Windows.

Unduhan yang gagal diverifikasi tidak dijalankan. Tombol **Perbarui** dapat dicoba lagi jika pemasangan gagal.

## Dukungan & batasan

| Bagian | Kondisi saat ini |
| --- | --- |
| Sistem | Windows x64; terutama diuji pada Windows 11. Windows 10 belum terverifikasi penuh. |
| Panel | Native, transparan, tanpa WebView2. Blur latar panel belum diterapkan. |
| Aksesibilitas | Navigasi keyboard tersedia; integrasi screen reader belum diterapkan. |
| Release | Belum ditandatangani secara digital. Checksum dipakai untuk memeriksa integritas unduhan. |
| Memori | Pemakaian bergantung pada Windows dan kondisi aplikasi; tidak ada jaminan total RAM 1–2 MB. |

Proses panel berakhir saat ditutup. Animasi berhenti saat selesai; proses updater berakhir setelah pengecekan atau pemasangan. Adaptasi taskbar mengikuti event Windows tanpa polling warna terus-menerus. Rincian ada di [arsitektur](docs/ARCHITECTURE.md).

## English

<details>
<summary>Read the English guide</summary>

### Features

Senja Bar provides four taskbar styles: **Glass**, **Acrylic**, **Clear**, and **Solid**. Glass and Acrylic adapt their tint to the Windows system theme and nearby background colors; Solid stays opaque and theme appropriate; Clear stays fully transparent. Background sampling is approximate and results vary with Windows and the backdrop.

Includes rounded taskbar corners, launch at startup, animated controls, refresh, and in-app updates. Changing styles keeps the panel open. Clicking outside or dismissing the tray closes it. **Keluar** exits Senja and restores the Windows taskbar.

The images above are captures of the native v1.1.5 menu on plain test backgrounds. They show the settings panel, not taskbar effects.

### Install

Download **Senja-Bar.exe** and run it. Companion files are prepared under `%LOCALAPPDATA%\Senja\app` without administrator access; the launcher then exits. Right-click Senja's tray icon to open settings. A portable ZIP is also available.

Settings are stored in `%LOCALAPPDATA%\Senja\settings.ini`.

### Update flow

Senja checks the latest stable release at startup and every 6 hours, retrying after 30 minutes on failure. Checks read version metadata only; they do not download an EXE.

- **Update available:** one Windows notification per new version, plus a red **Baru** badge on **Perbarui**, with three brief pulses when shown. Windows notification settings apply.
- **Up to date:** the disabled button shows the installed version.
- **Offline:** checks retry automatically; there is no manual check button.

During downloads, lower settings and quick actions temporarily become a larger progress panel; the controls return afterward. Downloads show **received / total MB**, measured average **MB/s**, estimated time remaining, percentage and a progress bar. Total size comes from the download response. An unknown total shows `-- MB` and no ETA. Speed and ETA appear after transfer measurements are available. MB uses decimal units (1 MB = 1,000,000 bytes).

Clicking **Perbarui** downloads the EXE, verifies its SHA-256 checksum and installs it **without an additional confirmation**. The old Senja closes and the new version starts. Older managed installs are cleaned after the new engine is ready; locked files are retried later. Settings, enabled startup and personal downloaded EXEs are preserved. No browser or Windows restart is needed. Unverified downloads are not executed; failed installs can be retried from the button.

### Requirements & limitations

- Windows x64, primarily tested on Windows 11; Windows 10 is not fully verified.
- The native panel needs no WebView2. Panel backdrop blur and screen-reader integration are not yet implemented; keyboard navigation is available.
- Releases are unsigned. Checksums verify download integrity.
- Memory use varies; total RAM usage of 1–2 MB is not guaranteed. Panel and updater processes exit when finished, and animations stop at rest.

</details>

## Open source

**GPL-3.0** · [License](LICENSE) · [Notices](NOTICE) · [Dependencies](docs/DEPENDENCIES.json)

Derived from [LowBar](https://github.com/chematic/LowBar), originally Apache-2.0. Includes unmodified [TranslucentTB 2026.2](https://github.com/TranslucentTB/TranslucentTB/releases/tag/2026.2) components; corresponding source accompanies each release. Independent of Xiaomi and Apple.

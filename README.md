# Senja Bar

Glass taskbar, simple controls. / Taskbar kaca, kontrol simpel.

[Download Senja-Bar.exe](https://github.com/FyarrAja/Senja-Bar/releases/latest/download/Senja-Bar.exe) · [Changelog](CHANGELOG.md) · [Build](docs/BUILD.md)

![Senja Bar GUI preview](docs/preview.png)

*GUI web preview from the same menu source; Windows renders the native backdrop. / Preview web dari sumber menu yang sama; latar kaca native dirender Windows.*

## Indonesia

Senja Bar mengubah tampilan taskbar Windows dengan empat mode: **Kaca**, **Akrilik**, **Bening**, dan **Solid**. Warna kaca mengikuti latar, dengan animasi perpindahan mode, toggle, serta buka–tutup menu.

Ada sudut membulat, mulai bersama Windows, segarkan taskbar, dan tombol pembaruan. Menu tetap terbuka saat mengganti mode, lalu menutup saat tray ditutup atau kamu klik di luar. Keluar dari aplikasi mengembalikan taskbar Windows.

**Pasang:** unduh **Senja-Bar.exe**, lalu buka. File pendamping disiapkan otomatis tanpa akses admin; launcher langsung keluar setelah mesin Senja berjalan. ZIP portable juga tersedia. Klik kanan ikon senja di tray untuk membuka menu. **Pembaruan** membuka [rilis terbaru](https://github.com/FyarrAja/Senja-Bar/releases/latest); unduh dan buka EXE baru untuk mengganti versi yang sedang berjalan. Pengaturan tersimpan di `%LOCALAPPDATA%\Senja\settings.ini`.

## English

Senja Bar gives the Windows taskbar four looks: **Glass**, **Acrylic**, **Clear**, and **Solid**. Live glass follows the background, with animated mode selection, switches and popup transitions.

Includes rounded corners, launch at startup, refresh and release updates. Changing modes keeps the menu open; dismissing the tray or clicking outside closes it. Exiting restores the Windows taskbar.

**Install:** download **Senja-Bar.exe** and open it. Companion files are prepared automatically without administrator access; the launcher exits after starting the native engine. A portable ZIP is also available. Right-click the sunset tray icon. **Pembaruan** opens the [latest release](https://github.com/FyarrAja/Senja-Bar/releases/latest); download and open the new EXE to replace the running version. User settings are preserved.

## Requirements & performance

Windows x64 + Microsoft Edge WebView2 Runtime. Primarily tested on Windows 11; Windows 10 compatibility is not fully verified. Releases are unsigned.

The native engine waits for Windows events. WebView2 uses additional memory while the menu is open and for a two-second quick-reopen cache, then exits. No idle capture, animation loop or periodic full-GC loop. Memory varies by system; 1–2 MB is not a total-memory guarantee. See [architecture](docs/ARCHITECTURE.md).

## Open source

**GPL-3.0** · [License](LICENSE) · [Notices](NOTICE) · [Dependencies](docs/DEPENDENCIES.json).

Derived from [LowBar](https://github.com/chematic/LowBar), originally Apache-2.0. Includes unmodified [TranslucentTB 2026.2](https://github.com/TranslucentTB/TranslucentTB/releases/tag/2026.2) components; corresponding source accompanies each release. Independent of Xiaomi and Apple.

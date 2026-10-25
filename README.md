# Senja Bar

Glass taskbar, simple controls. / Taskbar kaca, kontrol simpel.

[Download / Unduh](https://github.com/FyarrAja/Senja-Bar/releases/latest) · [Changelog](CHANGELOG.md) · [Build](docs/BUILD.md)

![Senja Bar GUI preview](docs/preview.png)

*GUI web preview from the same menu source; Windows renders the native backdrop. / Preview web dari sumber menu yang sama; latar kaca native dirender Windows.*

## Indonesia

Senja Bar mengubah tampilan taskbar Windows dengan empat mode: **Kaca**, **Akrilik**, **Bening**, dan **Solid**. Warna kaca mengikuti latar, dengan animasi perpindahan mode, toggle, serta buka–tutup menu.

Ada sudut membulat, mulai bersama Windows, segarkan taskbar, dan tombol pembaruan. Menu tetap terbuka saat mengganti mode, lalu menutup saat tray ditutup atau kamu klik di luar. Keluar dari aplikasi mengembalikan taskbar Windows.

**Pasang:** unduh ZIP, ekstrak seluruh isinya, lalu jalankan **Senja.exe**. Klik kanan ikon senja di tray untuk membuka menu. **Pembaruan** membuka [rilis terbaru](https://github.com/FyarrAja/Senja-Bar/releases/latest); penggantian file masih manual. Pengaturan tersimpan di `%LOCALAPPDATA%\Senja\settings.ini`.

## English

Senja Bar gives the Windows taskbar four looks: **Glass**, **Acrylic**, **Clear**, and **Solid**. Live glass follows the background, with animated mode selection, switches and popup transitions.

Includes rounded corners, launch at startup, refresh and release updates. Changing modes keeps the menu open; dismissing the tray or clicking outside closes it. Exiting restores the Windows taskbar.

**Install:** download the ZIP, extract everything, then run **Senja.exe**. Right-click the sunset tray icon. **Pembaruan** opens the [latest release](https://github.com/FyarrAja/Senja-Bar/releases/latest); install updates by replacing the extracted files after exiting Senja. User settings are preserved.

## Requirements & performance

Windows x64 + Microsoft Edge WebView2 Runtime. Primarily tested on Windows 11; Windows 10 compatibility is not fully verified. Releases are unsigned.

The native engine waits for Windows events. WebView2 runs only while the menu is open and costs additional memory. No idle capture, animation loop or periodic full-GC loop. Memory varies by system; 1–2 MB is not a total-memory guarantee. See [architecture](docs/ARCHITECTURE.md).

## Open source

**GPL-3.0** · [License](LICENSE) · [Notices](NOTICE) · [Dependencies](docs/DEPENDENCIES.json).

Derived from [LowBar](https://github.com/chematic/LowBar), originally Apache-2.0. Includes unmodified [TranslucentTB 2026.2](https://github.com/TranslucentTB/TranslucentTB/releases/tag/2026.2) components; corresponding source accompanies each release. Independent of Xiaomi and Apple.

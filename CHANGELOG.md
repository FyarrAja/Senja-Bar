# Changelog

## 1.1.4 — 2026-10-26

- Adapt taskbar materials to the Windows system light/dark theme and nearby background hue, independently per monitor. Glass and Acrylic vary tint strength to keep system text legible; Solid follows the theme with a restrained background hue; Clear remains fully transparent.
- Refresh colors on foreground, move/minimize, wallpaper and theme events, with debouncing and no idle polling. Sample only nine pixels outside the taskbar and exclude Senja's popup; no images are retained or decoded.
- Keep native and XAML material tint values consistent, with taskbar borders still hidden in all four custom modes.

## 1.1.3 — 2026-10-26

- Set explicit system arrow/hand cursors for the native GUI instead of inheriting the shell's launch/busy cursor.
- Hide the Windows 11 taskbar border in Glass, Acrylic, Clear and Solid modes; exiting still restores the Windows appearance.

## 1.1.2 — 2026-10-26

- Native per-pixel-alpha menu removes the opaque rectangular backing without WebView2. The panel is translucent; menu backdrop blur remains unfinished. Taskbar Glass/Acrylic effects are unchanged.
- Animated selection, switches and popup fades; asynchronous engine commands with coalescing and rollback.
- Adaptive text/surface contrast for bright and dark backdrops, using four unobstructed corner samples only while the panel is open.
- Continuous antialiased panel outline without a second hard window-region clip; raster coverage verified at 100%, 125%, 150% and 200% scaling.
- Refined exit button, vector action icons and mode selection without checkmarks.
- In-app update button checks the latest stable GitHub release, downloads and verifies its EXE against SHA256SUMS.txt, then installs without opening a browser. Current/newer versions are retained; failed downloads leave the running installation intact.
- HTTP/download work runs in a separate on-demand updater, with progress in the menu and no background polling.
- Package the editable CSS override from the current embedded stylesheet instead of a stale duplicate.

## 1.1.1 — 2026-10-25

- Single downloadable EXE with automatic extraction and no resident launcher.
- One native timeline moves and fades the popup, without competing HTML scaling.
- Reapply the rounded clip after native material setup.
- Install dismissal hooks after WebView startup; recognize renamed Explorer hosts, client hide and cloaking events.
- Preserve enabled startup registration when moving to a new version folder.
- Short bilingual release notes with a direct EXE download.
- Immediate mode/toggle feedback with engine acknowledgements, rollback on failure, and coalesced rapid input.
- Faster finite transitions and a two-second hidden reopen cache, released automatically afterward.
- Avoid redundant taskbar-host and border resets; theme changes and refresh invalidate the cache.

## 1.1.0 — 2026-10-25

- Live neutral menu glass replaces the frozen desktop snapshot; no screen capture or PNG encoding on the CSS path.
- Reapply taskbar materials after Windows theme, display and colorization changes, with event-based debouncing.
- Tray dismissal exemption now expires after a material interaction; hovering no longer keeps an orphan popup open. All mouse buttons outside the popup dismiss it.
- Synchronized entry/exit motion, continuous reversal when closing during entry, no spring overshoot, cached selection geometry. Fine timer resolution is scoped to finite fades.
- Short English/Indonesian README and a controlled GUI preview; original GUI layout and features retained.

## 1.0.0 — 2026-10-25

First public Senja Bar release.

- Four taskbar materials: Kaca, Akrilik, Bening and Solid.
- Transparent control panel with animated mode selection, switches and native window fades.
- Startup and rounded-corner settings, refresh control and centered exit action.
- Event-based tray popup lifecycle and on-demand WebView2 helper.
- Latest-release update link, portable x64 package, checksums and public source.
- Upstream attribution and matching TranslucentTB 2026.2 dependency source links.

Animations are explicitly enabled inside Senja even when Windows animation effects are disabled. No system animation preference is modified.

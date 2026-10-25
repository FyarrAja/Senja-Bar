# Changelog

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

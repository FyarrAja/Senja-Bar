# Native menu captures

`../preview.png` and `../preview-light.png` were captured on Windows on
2026-10-27 from the official GitHub v1.2.0 release (source `24b6d5a`).

The running engine was installed through a real automatic-update test from a
local prior-version candidate. Its native `--native-check` window used the
shipped drawing code and live engine settings/status. Automatic installation
is enabled in these captures because the test explicitly switched it on;
existing user settings default to off. No simulated download values are shown.

A separate opaque local fixture supplied plain dark (`#18212e`) and light
(`#f3f5fa`) backgrounds. Only the menu's 360 x 548 window was retained, with
no personal desktop content. The capture API returned JPEG data, converted to
PNG without resizing, retouching, added blur, or other visual changes.

The images show the settings panel, not taskbar effects, animation timing,
memory use, or behavior on every system. The HTML gallery in this directory
is an older web design reference, not the source of these captures.

# Native menu captures

`../preview.png` and `../preview-light.png` were captured on Windows on
2026-10-26 from the native v1.1.5 menu (source commit `a36f672`).

The menu ran in `--native-preview` mode, which uses the native drawing code
with isolated settings. An opaque local test window supplied plain dark
(`#18212e`) and light (`#f3f5fa`) backgrounds. Only the menu's 360 × 548 window
was retained; no personal desktop content is included.

The capture tool returned JPEG data, which was converted to PNG without
resizing, retouching, added blur, or other visual changes. The test background
was separate from the menu. These images show the settings panel, not the
taskbar material, animation timing, memory use, or behavior on every system.

The HTML gallery in this directory is an older web design reference and is
not the source of the current README images.

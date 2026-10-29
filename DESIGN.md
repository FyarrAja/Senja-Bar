# Native control panel

## Direction
Operate: an everyday Windows utility with readable controls and honest state. Retain the Senja mark, dark glass, round panel and blue active navigation. Revisions expand the useful layout instead of adding more decoration.

## Layout
- Panel: 620 × 548 logical pixels, 24 px outside radius; automatic DPI scaling.
- Navigation: 140 px, consistent 42 px hit areas; Taskbar, RAM & CPU, Sistem, Mic; Exit stays anchored at the bottom.
- Content: 480 px with approximately 27 px side margins. Native text stays at its original size while geometry reflows horizontally; no stretched glyphs.
- Type: Segoe UI for native Windows consistency; headings 26, primary controls 12–14, contextual metadata 10–12.
- Buttons and grouped controls use 10–14 px radii; no mouse-click focus outline. Keyboard navigation retains a visible focus indicator.

## Surfaces
- RAM & CPU separates applications from automatic rules. Real memory/CPU measurements are in the header. Running processes are prioritised by working set, with per-app memory and protection status.
- System tools are flat separated rows, with origin labels where Senja and Windows actions differ. Live measurements and errors come from the backend, never illustrative success states.
- Mic has source selection, a processed level meter, noise-gate and character controls, and explicit start/stop of a local test. Audio routing availability is a separate informational view.

## Verification
Motion uses a shared critically damped navigation plate, a moving tab underline, short press/release feedback, subtle hover feedback, an animated mic meter and expanding dropdowns. Updates coalesce at 120 Hz only while motion is active; the mic timer does not repaint an unchanged idle meter. Positions use fractional pixels for the new navigation/meter controls. Windows client animation preferences are the default; an explicit local `Senja/ui.json` Motion preference can choose `smooth` or `reduced`.

Mic source, output, preset, voice character, gate threshold and input gain use keyboard-accessible dropdowns. Popups isolate input from underlying controls, support Escape and outside-click dismissal, and page long device lists six at a time. Device rows use authored stroke icons for mic, headset, speaker, virtual and generic output. A 32-bar meter ascends from short to tall with a logarithmic dBFS scale and smoothly lit bars. “Dengar suara sendiri” is an explicit local monitor switch. Input gain is DSP gain for the local test, not a Windows system microphone volume control.

Render actual GDI+ drawing functions at 100% and 125%; verify projected pointer targets at 100%, 125%, 150% and 200%. Preserve disabled, loading, empty, error and keyboard focus states. Bounded review: one initial batch, one correction batch, then stop visual polishing.

Mic processing update (2026-10-29): the source caption identifies RNNoise and Auto voice processing. Auto uses speech probability and fixed makeup gain; Deep is bass EQ. The local-monitor boundary stays visible. Neural model strength is not represented as a percentage control.

Mic preview update: replaced the persistent self-listen toggle with a Dengarkan contoh 3 detik action in the same control footprint. Phase messages distinguish preparation, RAM-only capture, single playback with capture stopped, and completion. Ordinary Mulai uji mic keeps output closed.

Mic simplification: no refresh/reset/output-selector actions. Refresh input endpoints on opening the input list. Output follows Windows default. Removed controls are absent from keyboard navigation and hit testing; primary action follows the preview/default-output hint, with status beneath.

Mic self-listen update: add explicit live self-monitor switch with headset/latency helper, preserving sample preview as a separate action. Preset labels are plain Indonesian and paired with authored 20px stroke icons. The preset menu shows seven 44px rows in one view, each with title, short explanation and a selection check; keyboard can reach every row. No output picker, refresh or reset controls are reintroduced.

Mic one-toggle simplification: Dengar suara sendiri starts/stops the entire audio worker. Removed sample and separate start/stop controls. Microphone status, default-output hint and local-routing boundary remain. Meter reads at 20Hz with half-bar quantization, without driving whole-window 120Hz animation; interactive controls retain their motion.

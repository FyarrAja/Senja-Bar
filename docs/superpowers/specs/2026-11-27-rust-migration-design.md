# Senja-Bar Rust migration design

## Intent

Keep Senja-Bar's current Windows x64 features while reducing measured idle CPU/memory, startup time, and panel latency. Preserve user settings and update behavior. Fix reproducible defects as they are identified; do not equate a language rewrite with a bug fix or a performance gain.

## Approaches

- Big-bang rewrite: one Rust release replaces the Go engine, panel, updater, and launcher. This has the shortest route to an all-Rust codebase but the largest regression surface and no trustworthy intermediate release.
- Incremental replacement (chosen): keep the current Go release as the behavioral reference, replace one executable/subsystem at a time, and compare tests and Windows behavior before switching the default. This gives working intermediate releases.
- Keep Go and optimize: lowest migration risk, but does not meet the explicit Rust goal.

## Boundaries and order

1. Establish repeatable baseline: `go test ./...`, release build, idle CPU/private working set, cold startup, panel-open latency, and updater/mic smoke tests on Windows 11 x64. Record hardware and Windows build. No performance claim without comparable Rust measurements.
2. Replace the short-lived `cmd/updater` executable with a Rust binary. Preserve its command arguments (`<version>`, `--check-only`, `--apply-ready`, `--quiet`), update-state and pending JSON compatibility, phase/progress window messages, URL/redirect allowlists, size/time limits, SHA-256 validation, and manual versus automatic apply semantics. Differential tests should compare the Go and Rust versions on a local mock HTTP server and staged files. Never run unverified downloads.
3. Move the resident tray/taskbar engine. Keep ExplorerTAP/Win32 contracts and existing DLL provenance. Start with settings and taskbar state transitions, then replace the process once end-to-end checks pass. The pure settings parser and serializer in `rust/senja-engine-core` is the first engine component; it is not resident or wired into the Go process yet.
4. Move the on-demand GDI+ panel, retaining DPI, keyboard, animation-at-rest, and status/error behavior.
5. Move microphone orchestration and DSP only after synthetic/audio-device checks. Continue bundling RNNoise under its existing license and hashes. Preserve the explicit local-monitor boundary; this migration does not create a virtual microphone.
6. Replace launcher/packaging last. Keep `%LOCALAPPDATA%\Senja` settings and managed install cleanup behavior, including path and reparse-point safeguards.

## Constraints

- Rust stable with Windows MSVC toolchain; Go 1.23+ is needed for baseline and differential testing until migration is complete.
- Windows x64 is the supported target. Windows 10 behavior remains unverified until tested.
- Preserve GPL-3.0 and bundled dependency notices. Do not silently replace or relicense bundled native components.
- Each cutover requires passing behavior, security, and packaging checks plus measured performance. If Rust is slower or heavier for a component, investigate before shipping it.
- Existing release assets remain unsigned; checksums alone do not prove publisher identity.

## Initial acceptance criteria

- Rust updater accepts the same invocation modes and produces compatible state/progress records.
- Reject malformed versions, untrusted assets/redirects, oversized downloads, altered staged files, and stale releases.
- Offline and interrupted downloads leave the running Senja process and its settings intact.
- The Rust updater builds into the portable package and the current Go engine can launch it without changing user-facing update flow.

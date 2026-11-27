# Rust Updater Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans task by task. Steps use checkbox syntax.

**Goal:** Replace `cmd/updater` with a compatible Rust executable while preserving the current Go engine.

**Architecture:** The Rust protocol crate holds version and window-message rules. A Rust updater binary will implement check, stage, verify, and apply modes and write the same JSON files. The Go updater remains the behavioral oracle until integration tests pass.

**Tech Stack:** Rust stable/MSVC, Windows APIs, Go 1.23 test oracle, local HTTP fixture.

**Spec:** `docs/superpowers/specs/2026-11-27-rust-migration-design.md`

## Global Constraints

- Windows x64; preserve exact CLI flags, phase messages, pending records, and manual/auto update semantics.
- HTTPS download host restrictions, 64 MiB cap, SHA-256 verification, and staged-file recheck remain mandatory.
- No release cutover without Go/Rust parity tests and Windows packaging smoke test.

## Review Focus

- Missing Content-Length: report bytes without invented percentage or ETA.
- Interrupted transfer: discard temporary file and retain active install.
- Redirect to foreign host: reject before consuming installer bytes.
- Changed pending file: refuse execution and invalidate pending state.
- Same or older release: do not stage an installer.

### Task 1: Protocol crate

**Files:** `rust/senja-update-protocol/Cargo.toml`, `rust/senja-update-protocol/src/lib.rs`, `.gitignore`.

**Interfaces:** `version(&str) -> Result<[i64; 3], VersionError>`, `newer(&str, &str) -> Result<bool, VersionError>`, `encode_phase(u32, bool) -> u32`, `progress(i64, i64) -> u32`, `download_wire(i64, i64) -> u64`.

- [x] Add tests for versions, phase, progress and download byte packing.
- [x] Implement the matching Rust functions.
- [x] Run `cargo test --manifest-path rust/senja-update-protocol/Cargo.toml`.

### Task 2: Rust executable and differential tests

**Files:** Create `rust/senja-updater/Cargo.toml`, `rust/senja-updater/src/main.rs`, integration tests under `rust/senja-updater/tests/`.

**Interfaces:** Command line modes match `cmd/updater/main_windows.go`; JSON and Windows messages match `internal/updatestate`.

- [x] Add local HTTP fixtures for verified stage, checksum mismatch, interrupted transfer, bounded body, and HTTP status.
- [x] Implement release check, bounded download and SHA-256 staged verification.
- [x] Implement `--check-only`, `--apply-ready`, and `--quiet` entry points.
- [x] Run Rust unit tests and `go test ./internal/releaseupdate ./internal/updatestate`.

### Task 3: Package cutover

**Files:** Modify `build.ps1`, `scripts/package-release.ps1`, and related release tests.

- [x] Build Rust updater as `Senja-Update.exe` in the same package location.
- [ ] Run full interactive Windows smoke tests; real GitHub metadata check and verified staging passed, as did `go test ./...`, `cargo test`, and release packaging.
- [ ] Record comparable startup/idle/panel metrics before and after; report any regression.

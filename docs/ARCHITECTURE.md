# Architecture

The persistent Go/Win32 engine owns the tray icon, taskbar materials and per-user settings. It waits for Windows messages. A short-lived child process hosts the local WebView2 menu; closing the popup releases it. The engine continues without WebView2.

Taskbar materials use the ExplorerTAP COM interface with checked HRESULTs. CSS commands are posted asynchronously to the engine and acknowledged after completion; this avoids synchronous input COM-call failures. Allowed commands are explicitly listed in menu_css.go. The menu has no HTTP listener, remote page, external font, or arbitrary command bridge.

The menu uses a blurred local desktop snapshot captured once at open, while taskbar material blur uses the live compositor. Animations have finite lifetimes: native whole-window alpha fades, CSS mode-indicator translation, switch translation and button response. Timers stop after the fade.

Preferences are stored in LOCALAPPDATA/Senja/settings.ini. Startup uses the current-user Run registry key. Update opens only this repository's latest-release URL. No silent installer is implemented.

Unmodified dependency hashes and source provenance are in DEPENDENCIES.json. The legacy cmd/samar path is internal only.

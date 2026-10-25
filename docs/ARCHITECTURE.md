# Architecture

The persistent Go/Win32 engine owns the tray icon, taskbar materials and per-user settings. It waits for Windows messages. A short-lived child process hosts the local WebView2 menu; closing the popup releases it. The engine continues without WebView2.

Taskbar materials use the ExplorerTAP COM interface with checked HRESULTs. CSS commands are posted asynchronously to the engine and acknowledged after completion; this avoids synchronous input COM-call failures. Allowed commands are explicitly listed in menu_css.go. The menu has no HTTP listener, remote page, external font, or arbitrary command bridge.

Both the menu and taskbar use the live Windows compositor. A transparent WebView reveals a neutral native glass surface; the CSS path does not capture, blur, encode or retain desktop images. A neutral dark scrim preserves readable text over light backgrounds. Animations have finite lifetimes: native whole-window alpha fades, CSS mode-indicator translation, switch translation and button response. Opening and closing start their native fade from the renderer animation frame. Close reverses from the current transform and opacity without an overshoot. A high-resolution ticker posts at most one outstanding native fade frame, up to 120 Hz, and stops at the endpoint. Timer resolution is raised only during a finite fade, then released. There are no idle animation loops. Theme/display/colorization changes trigger one debounced material reapply. Overflow-hide and outside-click hooks exist only while the menu is open; material-change dismissal exemptions expire.

Preferences are stored in LOCALAPPDATA/Senja/settings.ini. Startup uses the current-user Run registry key. Update opens only this repository's latest-release URL. No silent installer is implemented.

Unmodified dependency hashes and source provenance are in DEPENDENCIES.json. The legacy cmd/samar path is internal only.

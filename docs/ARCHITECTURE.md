# Architecture

The persistent Go/Win32 engine owns the tray icon, taskbar materials and per-user settings. It waits for Windows messages. A short-lived child process hosts the local WebView2 menu; closing the popup releases it. The engine continues without WebView2.

Taskbar materials use the ExplorerTAP COM interface with checked HRESULTs. CSS commands are posted asynchronously to the engine and acknowledged after completion; this avoids synchronous input COM-call failures. Allowed commands are explicitly listed in menu_css.go. The menu has no HTTP listener, remote page, external font, or arbitrary command bridge.

Both the menu and taskbar use the live Windows compositor. A transparent WebView reveals a neutral native glass surface; the CSS path does not capture, blur, encode or retain desktop images. A neutral dark scrim preserves readable text over light backgrounds. Animations have finite lifetimes: native whole-window alpha fades, CSS mode-indicator translation, switch translation and button response. Opening and closing start their native fade from the renderer animation frame. Close reverses from the current transform and opacity without an overshoot. A high-resolution ticker posts at most one outstanding native fade frame, up to 120 Hz, and stops at the endpoint. Timer resolution is raised only during a finite fade, then released. There are no idle animation loops. Theme/display/colorization changes trigger one debounced material reapply. Overflow-hide and outside-click hooks exist only while the menu is open; material-change dismissal exemptions expire.

The popup now moves and fades as one native window. Its HTML panel is not separately scaled against the live backdrop. Dismissal hooks are installed after WebView loading so startup cannot exhaust the Windows hook timeout. Explorer window-hide, client-hide, destruction and cloaking events dismiss the popup.

After a normal dismissal the window is immediately hidden; a single two-second timer retains it for rapid reopen, then releases WebView2 and the helper. All dismissal hooks and frame tickers are stopped while hidden. Engine shutdown bypasses retention. Reopen reuses the document, synchronizes current settings, and updates monitor placement. The first open after cache expiry still initializes WebView2.

Mode and switch feedback is immediate. One engine command is in flight at a time; rapid input retains the latest intended state. Acknowledgements reconcile actual settings, and errors roll the controls back without a retry loop. Unchanged native host/border calls are skipped, with invalidation on theme/display/color changes and explicit refresh.

Preferences are stored in LOCALAPPDATA/Senja/settings.ini. Startup uses the current-user Run registry key. Update opens only this repository's latest-release URL. No network downloader is implemented.

Senja-Bar.exe embeds the portable ZIP, extracts it once into a version-and-content-addressed LOCALAPPDATA/Senja/app folder, starts the engine, and exits. Extraction checks paths and required files and publishes only a complete staging directory. An older running engine exits gracefully before replacement. An already enabled startup preference follows the new engine path. No administrator access or resident launcher is needed.

Unmodified dependency hashes and source provenance are in DEPENDENCIES.json. The legacy cmd/samar path is internal only.

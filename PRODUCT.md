# Senja Bar

Senja is a local Windows desktop utility for taskbar appearance, protected background resource management, device diagnostics, and local microphone processing. The user wants a cohesive application with its own engines, rather than a dashboard of third-party installers.

## Product facts
- Native Win32/GDI+ interface; dark translucent material, rounded corners, Senja's sunset mark and blue selection accent remain established identity.
- RAM management trims eligible background working sets. CPU management applies/restores EcoQoS. Neither feature promises lower committed memory, higher clocks, or guaranteed FPS.
- Foreground apps, games, security-critical processes, active audio sessions and fullscreen usage are protected by the resource policy.
- System telemetry, diagnostics, ping/DNS tests, storage analysis and standard power-plan changes are implemented in Senja through Windows APIs and the Go standard library.
- Advanced Windows servicing still uses Windows' own tools. Their origin and administrative requirements are explicit.
- The Mic engine is Senja code: noise gate, high-pass, optional bass body and peak control. Current UI provides an opt-in local signal test only. It does not record/upload audio or forward processed audio to FiveM/Discord.
- A Senja virtual microphone driver is not implemented or signed. Do not present it as installed, working, or equivalent to the local signal test. No third-party cable installer is part of the new Mic feature.
- Legacy taskbar helper DLLs and optional CSS/WebView paths remain in the repository; the whole application is not being represented as free of all third-party code.

## Delivery
Preserve user preferences and existing uncommitted work. Public releases use the tagged GitHub workflow after the user requests publication. Do not change local startup registrations merely to publish a release.

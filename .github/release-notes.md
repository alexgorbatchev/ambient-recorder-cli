## Summary

- Install and uninstall background recording with `service install` and `service uninstall`, with startup at macOS GUI login and recordings retained on uninstall.
- Start, stop, and restart the service directly from the CLI. Stop disables startup at login; start re-enables it, and restart reloads recording configuration.
- Inspect installation and launchd registration with `service status`, and include native diagnostics with `--details`. Loaded status does not establish recording health.
- Generate LaunchAgent property lists with Apple's native serializer, with lifecycle tests exercising actual launchd registration.

Executables are ad-hoc signed; they are not Developer ID signed or notarized. Transcription, VAD, and calendar integration are not included.

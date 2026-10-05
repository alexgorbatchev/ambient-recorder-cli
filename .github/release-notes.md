## Summary

- Print the absolute configured recording directory with `config print-dir`, or the open recording file with `recording print-file`. Both commands return only the path on stdout; current-file queries reject stale state after recorder exit.
- Inspect a different recording directory with `recording print-file --output`. Existing recorder processes need restarting with the updated executable to expose their active file.
- `service status` now shows both the invoked CLI version and the version reported by the live service process. An unloaded service reports `not running`; a loaded job without a version response reports `unavailable`.
- Regenerate existing LaunchAgent plists with `service uninstall` followed by `service install` to enable live service-version reporting. Restart alone retains the saved environment. A version response does not establish audio capture health.

Executables are ad-hoc signed; they are not Developer ID signed or notarized. Transcription, VAD, and calendar integration are not included.

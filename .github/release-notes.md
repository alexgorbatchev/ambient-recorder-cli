## Summary

- `service status` now shows the recording directory, configuration path and current recording file in human and agent output. Live paths come from the running service and remain accurate across file rotation and configuration edits on disk.
- When live paths are unavailable, status shows the saved service paths and labels their source. A stopped service reports its current file as `not running`; a responding recorder with no open file reports `none`.
- Restart the service with the updated executable to enable live path reporting. Older LaunchAgent plists without the status socket setting need regeneration through `service uninstall` followed by `service install`. Until then, the current file is reported as `unavailable`; status does not guess from another recorder's files. An open file does not establish audio capture health.

Executables are ad-hoc signed; they are not Developer ID signed or notarized. Transcription, VAD, and calendar integration are not included.

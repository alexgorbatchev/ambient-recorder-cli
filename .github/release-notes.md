## Summary

- Record each microphone input channel into a separate mono `mic-N.opus` file and computer playback into `playback.opus`. Source files share timestamps and hourly rotation; channel changes preserve existing encoders and pad absent channels with silence within the current segment.
- `recording print-file` lists all open source files, one absolute path per line. Service status reports all current source files and the active microphone, uses a borderless table, and abbreviates home paths to `~/`. Agent status replaces `current_file` with `current_files`; update consumers of the old field and single-file output.
- Configure `output.on_close_command` to run your own executable after each source file successfully finalizes. Commands run serially outside capture, receive `{file}` and `{stem}` substitutions, and save stdout/stderr to sibling `.command.log` files. The hook is disabled by default; failures are logged without stopping recording. Jobs have a ten-minute limit and no automatic retry or crash recovery.
- Service start and restart refresh the installed CLI executable and live-status endpoint. Restart waits for the departing launchd job before starting its replacement. Status reports the version supplied by the live process and labels fallback paths as saved.
- `ambient-recorder skill` prints a fixed installation command and GitHub link. The operational reference lives at `skills/ambient-recorder/SKILL.md` and is included in release archives.

Executables are ad-hoc signed; they are not Developer ID signed or notarized. External file commands manage their own credentials, uploads, transcript output and costs. VAD and calendar integration are not included.

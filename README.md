`ambient-recorder` records the default microphone and computer playback into local Opus files on macOS. It keeps recording during silence and retries capture or storage failures, providing audio for later meeting transcription.

# What It Does

- **Continuous capture:** Mixes microphone and playback into one mono track.
- **Hourly files:** Writes `YYYY/MM/DD/HH-000.opus`, allocating `-001`, `-002`, and later suffixes on restart or capture recovery.
- **Diagnostics:** Appends events to `YYYY/MM/DD/log.nljson`, including capture changes, dropped frames, storage errors, and segment boundaries.
- **Restart supervision:** Prints a macOS launchd configuration for background operation.

# How It Works

- Start recording and grant microphone and system audio recording access when macOS requests it.
- Check the startup message for the microphone name, sample rate, and absolute output directory.
- Leave the process running. New files begin at local hour boundaries; existing recordings remain intact.
- Press Ctrl-C to finish the current file and stop a foreground recording.

# How it Really Works

- Audio stays on this Mac. There are no uploads, transcription requests, API credentials, VAD, or calendar connections.
- The default directory is `$XDG_DATA_HOME/ambient-recorder`, falling back to `~/.local/share/ambient-recorder`. `--output` overrides it. Directories and files are created with owner-only permissions.
- Mono output combines both sources. It cannot provide separate microphone/playback transcripts. [Deepgram lists Ogg and Opus as supported formats](https://developers.deepgram.com/docs/supported-audio-formats); transcription is a separate future integration.
- Silence remains in the recording. Encoding defaults to a 32,000 bit/s target and complexity 2. Actual file size varies with audio and container overhead.
- Capture follows the default microphone. A device or format change closes the segment and attempts to reopen capture. Changes and recovery can leave gaps; overflow is counted and logged.
- Audio pages are emitted while recording, approximately every 100 ms. Storage synchronization is attempted every five seconds. A crash can lose buffered audio or an incomplete final page, and the interrupted file lacks final duration trimming. [Ogg Opus readers must handle streams without an end-of-stream page](https://www.rfc-editor.org/rfc/rfc7845.html#section-3). Power-loss durability is not guaranteed.
- A blocked capture, disk operation, or shutdown triggers process exit after 90 seconds. A configured supervisor restarts it. Sleep, permissions, full disks, and process downtime prevent continuous capture.
- Only one recorder may use an output directory. Recording does not prune files; free disk space remains the user's responsibility.
- Recording prints the microphone name, device ID, sample rate, playback source, and absolute output directory to stderr whenever capture opens. Recording writes no normal output to stdout. Warnings and failures also go to stderr; debug events go to the daily JSON log. `AGENT=1` enables compact help and diagnostics. Normal cancellation exits 0, startup or cleanup failure exits 1, and the watchdog exits 2.

# Installation

This checkout has no published release. Its local executable is `bin/ambient-recorder`; [contributor instructions](docs/internal/contributing.md) describe building it. Published downloads and signing are not configured.

# Setup

- Use macOS 14.2 or later with at least one microphone. The binary uses built-in Apple audio services and contains its audio codec; it needs no installed audio driver or codec executable.
- Grant microphone and system audio recording permissions through macOS when requested. Review them in System Settings → Privacy & Security if capture fails.

# Quick Start

```sh
./bin/ambient-recorder recording start --output "$HOME/Recordings/ambient"
```

Sample daily log output from a verified recording:

```json
{"time":"2026-09-30T12:19:21.953704-07:00","level":"INFO","msg":"Capture device opened","microphone_id":126,"sample_rate":16000}
```

```sh
./bin/ambient-recorder --help
./bin/ambient-recorder --version
```

# Options & Flags

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--help` | `-h` | `false` | Print help |
| `--version` | `-v` | `false` | Print version and exit |

`ambient-recorder recording start`

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--output <path>` | | XDG user data `/ambient-recorder` | Recording and daily log directory |
| `--bitrate <bits/s>` | | `32000` | Opus target bitrate, 6000–128000 |
| `--complexity <number>` | | `2` | Encoding complexity, 0–10; higher uses more CPU |
| `--sync-interval <duration>` | | `5s` | Storage synchronization interval |

`ambient-recorder service print`

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--output <path>` | | XDG user data `/ambient-recorder` | Output directory written into the service configuration |

# Background Recording

The `service print` command prints a configuration without installing or starting it. It references the executable's current absolute path, so place the binary at its permanent location before generating the configuration. Run it in the foreground first to grant permissions.

```sh
mkdir -p "$HOME/Recordings/ambient" "$HOME/Library/LaunchAgents"
./bin/ambient-recorder service print --output "$HOME/Recordings/ambient" > "$HOME/Library/LaunchAgents/com.alexgorbatchev.ambient-recorder.plist"
plutil -lint "$HOME/Library/LaunchAgents/com.alexgorbatchev.ambient-recorder.plist"
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.alexgorbatchev.ambient-recorder.plist"
```

The per-user agent starts recording and uses `KeepAlive` to request relaunch after exit, with a five-second throttle. It runs in the login session; it does not capture while the Mac sleeps. Background permissions must be checked separately from foreground permissions. Fallback stderr goes to `supervisor.stderr.log` in the recording directory. [Apple documents launchd agent supervision](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html).

Stop background recording before running another recorder against the same directory:

```sh
launchctl bootout "gui/$(id -u)/com.alexgorbatchev.ambient-recorder"
```

Remove the saved plist to prevent launch at the next login.

# License

[MIT](LICENSE), © 2026 Alex Gorbatchev. [Third-party notices](THIRD_PARTY_NOTICES.md) accompany binary redistribution.

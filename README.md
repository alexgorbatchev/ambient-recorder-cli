`ambient-recorder` records microphone and computer playback into local Opus files on macOS. It keeps recording during silence and retries capture or storage failures, providing audio for later meeting transcription.

# What It Does

- **Continuous capture:** Mixes microphone and playback into one mono track.
- **Microphone preferences:** Selects from an ordered TOML list and switches automatically when a preferred input becomes available.
- **Hourly files:** Writes `YYYY/MM/DD/HH-MM-SS.000000000.opus` using the segment's start time with nine fractional-second digits.
- **Diagnostics:** Appends events to `YYYY/MM/DD/log.nljson`, including capture changes, dropped frames, storage errors, and segment boundaries.
- **Restart supervision:** Prints a macOS launchd configuration for background operation.

# How It Works

- Use `microphone list` to inspect available inputs, their connection types, and stable device identifiers.
- Run `config init` to create settings, then edit the microphone preference list if needed. Its default is `["*"]`.
- Start recording and grant microphone and system audio recording access when macOS requests it.
- Check the capture log event for the selected microphone, connection type, playback source, and absolute output directory. Use `microphone list` to inspect hardware details.
- Leave the process running. New files begin at local hour boundaries; existing recordings remain intact.
- Press Ctrl-C to finish the current file and stop a foreground recording.

# How it Really Works

- Audio stays on this Mac. There are no uploads, transcription requests, API credentials, VAD, or calendar connections.
- Configuration is read once at startup from `$XDG_CONFIG_HOME/ambient-recorder/config.toml`, falling back to `~/.config/ambient-recorder/config.toml`. Use `--config` to select another file. A missing default file uses built-in defaults; a missing explicitly selected file fails. Unknown keys, invalid types, and invalid final settings fail with an error. Explicit flags override file values, which override defaults. Restart recording to apply file edits.
- The default output directory is `$XDG_DATA_HOME/ambient-recorder`, falling back to `~/.local/share/ambient-recorder`. The TOML `output.dir` value or `--output` overrides it. Relative output paths resolve from the working directory; use an absolute path for a stable destination. Directories and files are created with owner-only permissions.
- Mono output combines both sources. It cannot provide separate microphone/playback transcripts. [Deepgram lists Ogg and Opus as supported formats](https://developers.deepgram.com/docs/supported-audio-formats); transcription is a separate future integration.
- Silence remains in the recording. Encoding defaults to a 32,000 bit/s target and complexity 2. Actual file size varies with audio and container overhead.
- Microphone preferences match case-sensitive names, name patterns such as `LG Ultra*`, or exact `uid:<UID>` identifiers printed by `microphone list`. In names, `*` matches any sequence of characters; all other punctuation is literal. Prefix any selector with `!` to exclude matching inputs globally, regardless of list position. Exclusions override every positive preference and wildcard fallback. `*` tries the macOS default first, then other eligible inputs. Unlisted microphones remain eligible through an implicit final `*` unless excluded. A higher-priority input becoming available triggers automatic switching. Device or format changes reopen capture while keeping the current file and Opus stream open. Output remains mono at 48 kHz across different hardware sample rates. Changes and recovery can leave gaps; overflow and microphone connections/disconnections are logged.
- Audio pages are emitted while recording, approximately every 100 ms. Storage synchronization is attempted every five seconds. A crash can lose buffered audio or an incomplete final page, and the interrupted file lacks final duration trimming. [Ogg Opus readers must handle streams without an end-of-stream page](https://www.rfc-editor.org/rfc/rfc7845.html#section-3). Power-loss durability is not guaranteed.
- A blocked capture, disk operation, or shutdown triggers process exit after 90 seconds. A configured supervisor restarts it. Sleep, permissions, full disks, and process downtime prevent continuous capture.
- Only one recorder may use an output directory. Recording does not prune files; free disk space remains the user's responsibility.
- Segment names use acquisition time. If that exact path is occupied, exclusive creation retries with the current timestamp; existing files remain intact. Filenames alone cannot reveal every gap within a segment, including a microphone switch.
- If an input opens but produces no capture frames for five seconds, the recorder skips that input for 30 seconds and immediately tries another eligible microphone. The current output file and Opus stream stay open. After the cooldown, a higher-priority microphone becomes eligible again. This detects missing capture frames; it does not treat silence as a failure.
- Recording writes timestamped INFO, WARN, and ERROR events to stderr, including microphone connections, disconnections, selection changes, and capture startup. Human logs show the selected microphone and connection, playback source, recording paths, and useful error/retry details. The initial list of available devices, hardware IDs, manufacturer/model information, and encoder settings stay in diagnostics. Use `microphone list` to inspect available inputs. Human logs use compact single lines with the local date and time, UTC offset, and an `INF`, `WRN`, or `ERR` severity tag. Colors are enabled when stderr is a terminal; redirected output is plain text. A nonempty `NO_COLOR` or `TERM=dumb` disables colors. `AGENT=1` uses newline-delimited JSON without colors and retains all diagnostic fields and startup inventory events. The daily JSON log also retains every event, including DEBUG progress and full device metadata. Device details are reported by the hardware: an input can report a generic microphone type even when it belongs to a headset or display. If writing the daily log fails, the event goes to stderr with `log_error`. Recording writes no normal output to stdout. Normal cancellation exits 0, startup or cleanup failure exits 1, and the watchdog exits 2.

# Installation

Download the prebuilt binary for your Mac from [GitHub Releases](https://github.com/alexgorbatchev/ambient-recorder-cli/releases). Use `arm64` for Apple Silicon or `amd64` for Intel. Each archive includes the executable, configuration example, embedded usage reference and component licenses.

For Apple Silicon, download version 1.0.0, verify its checksum and put the executable on your PATH:

```sh
curl --fail --location --remote-name https://github.com/alexgorbatchev/ambient-recorder-cli/releases/download/v1.0.0/ambient-recorder_1.0.0_darwin_arm64.tar.gz
curl --fail --location --remote-name https://github.com/alexgorbatchev/ambient-recorder-cli/releases/download/v1.0.0/checksums.txt
awk '$2 == "ambient-recorder_1.0.0_darwin_arm64.tar.gz"' checksums.txt | shasum -a 256 -c -
tar -xzf ambient-recorder_1.0.0_darwin_arm64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 755 ambient-recorder "$HOME/.local/bin/ambient-recorder"
```

Replace `arm64` with `amd64` in the download, checksum filter and extraction commands for Intel. Preserve the accompanying notices and license texts when redistributing. Add `$HOME/.local/bin` to PATH if it is not already there. Executables are ad-hoc signed and are not Developer ID signed or notarized.

# Setup

- Use macOS 14.2 or later with at least one microphone. The binary uses built-in Apple audio services and contains its audio codec; it needs no installed audio driver or codec executable.
- Grant microphone and system audio recording permissions through macOS when requested. Review them in System Settings → Privacy & Security if capture fails.

# Quick Start

```sh
ambient-recorder microphone list
ambient-recorder config init
ambient-recorder recording start --output "$HOME/Recordings/ambient"
```

Sample output from `microphone list` on a verified Mac; available devices vary:

```text
"MacBook Pro Microphone"
  Connection: Built-in
  Input type: Unavailable
  Manufacturer: Apple Inc.
  Preference: "uid:BuiltInMicrophoneDevice"
```

```sh
ambient-recorder --help
ambient-recorder --version
AGENT=1 ambient-recorder skill
```

# Options & Flags

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--config <path>` | | XDG configuration `/ambient-recorder/config.toml` | TOML configuration file |
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

`ambient-recorder completion bash|fish|powershell|zsh`

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--no-descriptions` | | `false` | Generate shell completions without descriptions |

# Configuration

`config init` creates a file with owner-only permissions and refuses to overwrite it. To create and use a file in another location:

```sh
ambient-recorder --config "$HOME/recorder.toml" config init
ambient-recorder --config "$HOME/recorder.toml" recording start
```

Edit the generated TOML file to set preferences:

```toml
[input]
microphones = ["LG Ultra*", "!Virtual*", "*"]

[output]
dir = "recordings"
bitrate = 32000
complexity = 2
sync_interval = "5s"
```

This example writes beneath the working directory, prefers names starting with `LG Ultra`, and excludes names starting with `Virtual`. Set `output.dir` to your own absolute path for a stable destination and choose device names or patterns from `microphone list`. Stable UIDs also work: `input.microphones = ["uid:BuiltInMicrophoneDevice", "*"]`; `!uid:<UID>` excludes an exact UID. Missing settings use defaults, and `input.microphones = []` allows every available input. If all inputs are excluded, the recorder retries without capturing audio until an eligible microphone becomes available. `~` in a TOML path is literal; use an absolute path instead of shell-style home-directory expansion. [config.example.toml](config.example.toml) describes every setting and selector.

Use `[input]` and `[output]` tables, or dotted keys such as `output.dir = "recordings"` at the document root. Flat root keys are rejected. `--output` overrides `output.dir`, and the encoding and synchronization flags override their corresponding `[output]` values.

# Background Recording

The `service print` command prints a configuration without installing or starting it. It references the executable's current absolute path, so place the binary at its permanent location before generating the configuration. Run it in the foreground first to grant permissions.

When a TOML file is loaded, the generated agent includes its absolute path so microphone preferences and encoding settings are read on each process start. The output directory is pinned as an absolute `--output` argument for recording and supervisor diagnostics. Regenerate and reload the agent after changing the desired output directory or configuration file path.

```sh
mkdir -p "$HOME/Recordings/ambient" "$HOME/Library/LaunchAgents"
ambient-recorder service print --output "$HOME/Recordings/ambient" > "$HOME/Library/LaunchAgents/com.alexgorbatchev.ambient-recorder.plist"
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

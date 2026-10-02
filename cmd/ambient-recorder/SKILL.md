---
name: ambient-recorder
description: Use when operating ambient-recorder to record audio, configure microphone preferences, inspect inputs, or generate a background recording configuration.
author: alexgorbatchev
metadata:
  created_on: 2026-10-01 22:02
  last_modified: 2026-10-01 22:34
  status: current
---

## Recording workflow

Use macOS 14.2 or later and an available microphone. The executable includes its codec and uses Apple audio services. Grant microphone and system audio recording access when macOS requests it. Audio stays on the Mac; transcription, VAD, uploads, and calendar integration are future work.

```sh
AGENT=1 ambient-recorder microphone list
AGENT=1 ambient-recorder config init
AGENT=1 ambient-recorder recording start
```

Inspect the listed names and stable `uid:<UID>` selectors, then edit the created TOML file. Start recording in the foreground to grant permissions. Stop with Ctrl-C or SIGTERM to finalize the active file. Configuration is read once at startup; restart recording to apply edits.

## Commands and positional arguments

| Command | Arguments | Result and side effects |
| :--- | :--- | :--- |
| `ambient-recorder` | `none` | Print help to stdout. |
| `ambient-recorder config` | `none` | Print configuration command help. |
| `ambient-recorder config init` | `none` | Exclusively create default TOML and parent directories with owner-only permissions. Print its path; refuse to replace an existing file. |
| `ambient-recorder microphone` | `none` | Print microphone command help. |
| `ambient-recorder microphone list` | `none` | List currently available inputs, names, stable UIDs, transient IDs, default status, connection, type, manufacturer, and model. Capture is not opened. |
| `ambient-recorder recording` | `none` | Print recording command help. |
| `ambient-recorder recording start` | `none` | Continuously capture microphone plus computer playback into a mono Ogg Opus stream and append diagnostics. Request capture permissions; create directories/files and hold an exclusive output-directory lock. |
| `ambient-recorder service` | `none` | Print service command help. |
| `ambient-recorder service print` | `none` | Print a launchd plist to stdout. Resolve the executable, loaded config, and output to absolute paths. Installation and process startup are separate shell operations. |
| `ambient-recorder skill` | `none` | Print this complete embedded Markdown, including frontmatter, byte-for-byte to stdout in both modes; work offline and without repository files. Return write failures as errors. |
| `ambient-recorder help` | `[command]` | Print help for a command path, such as `help recording start`. |
| `ambient-recorder completion` | `none` | Print shell-completion command help. |
| `ambient-recorder completion bash` | `none` | Print a Bash completion script to stdout. Loading the script requires Bash completion support. |
| `ambient-recorder completion fish` | `none` | Print a Fish completion script to stdout. |
| `ambient-recorder completion powershell` | `none` | Print a PowerShell completion script to stdout. |
| `ambient-recorder completion zsh` | `none` | Print a Zsh completion script to stdout. |

Use `--help` on any command for help. Domain leaf commands and `skill` accept zero positional arguments. Generated completion commands accept zero positional arguments. Cobra's generated `help` accepts a space-separated command path. Domain command groups show their subtree; `completion` is hidden from human command listings and remains available by name and in agent help. The generated scripts can be redirected to files; generation itself does not install them. Load Zsh completion with `source <(ambient-recorder completion zsh)` after enabling `compinit` in your shell.

## Flags

`all` applies to every command. Empty string parser defaults select a runtime default. Explicit CLI flags override TOML, and TOML overrides built-in defaults. Boolean flags can use `=true` or `=false`.

| Scope | Flag | Short | Type | Parser default | Accepted values and effective behavior |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `all` | `--config` | `none` | `string` | `""` | TOML path; empty selects the XDG default below. An explicitly named file must exist for recording and service generation; config init creates it. Ignored by microphone, help, skill, and completion operations. |
| `all` | `--help` | `-h` | `bool` | `false` | Print help and exit. |
| `ambient-recorder` | `--version` | `-v` | `bool` | `false` | Print only the raw version plus a newline and exit. Development builds print dev; releases embed their version. |
| `ambient-recorder recording start` | `--output` | `none` | `string` | `""` | Nonempty recording directory; empty parser default inherits output.dir from TOML or the default data directory. |
| `ambient-recorder recording start` | `--bitrate` | `none` | `int` | `32000` | Target bits/second, 6000..128000 inclusive. |
| `ambient-recorder recording start` | `--complexity` | `none` | `int` | `2` | Opus computational effort, 0..10 inclusive; higher uses more encoder CPU. |
| `ambient-recorder recording start` | `--sync-interval` | `none` | `duration` | `5s` | Positive Go duration such as 500ms, 5s, or 1m30s. Approximate interval for synchronizing audio and logs to disk. |
| `ambient-recorder service print` | `--output` | `none` | `string` | `""` | Resolve the configured/default output directory and pin it in the generated agent. |
| `ambient-recorder completion bash` | `--no-descriptions` | `none` | `bool` | `false` | Generate completion choices without descriptions. |
| `ambient-recorder completion fish` | `--no-descriptions` | `none` | `bool` | `false` | Generate completion choices without descriptions. |
| `ambient-recorder completion powershell` | `--no-descriptions` | `none` | `bool` | `false` | Generate completion choices without descriptions. |
| `ambient-recorder completion zsh` | `--no-descriptions` | `none` | `bool` | `false` | Generate completion choices without descriptions. |

## Environment and output

- `AGENT`: trimmed, case-insensitive 1, true, or yes enables agent mode. Unset, 0, or another value uses human mode. Agent help starts with the skill-reading alert and uses untruncated compact key/value and bullet output. Human help uses aligned trees clipped to the detected terminal width, with an 80-column fallback. `skill`, raw version output, shell scripts, and launchd XML have identical contracts in both modes.
- `COLUMNS`: a positive integer overrides human help width; otherwise detect stdout terminal width.
- `XDG_CONFIG_HOME`: absolute configuration base; defaults to `$HOME/.config`. Load `ambient-recorder/config.toml` beneath it. A missing implicit default file uses defaults.
- `XDG_DATA_HOME`: absolute data base; defaults to `$HOME/.local/share`. Default output is its `ambient-recorder` subdirectory.
- `HOME`: determines the user's home directory when XDG overrides are absent.
- `NO_COLOR`: any nonempty value disables human recording-log colors.
- `TERM`: dumb disables human recording-log colors. Redirected stderr is plain text.
- Help, version, config init, microphone list, skill, completion, and service print return results on stdout. Recording has no normal stdout output. Usage and failures go to stderr.
- Human microphone listings show names, default status, connection/type and available manufacturer/model details, plus stable preference selectors. Agent listings use one flat key/value line per microphone. With no inputs, both print `No microphones available`.
- Config init prints `Configuration created: <absolute path>` in human mode or `config=<absolute path>` in agent mode.
- Recording emits INFO/WARN/ERROR events on stderr. Human events include timestamp, UTC offset, INF/WRN/ERR, microphone/connection, recording paths, and useful errors. Agent events are newline-delimited JSON with full device and encoder metadata. Startup inventory and DEBUG progress remain in daily JSON diagnostics; agent stderr also includes startup inventory.
- Argument, configuration, fatal startup and cleanup errors exit 1; normal completion/cancellation exits 0. Retryable capture failures, including missing capture permissions, are logged and retried while recording remains running. A 90-second operation watchdog exits 2 for supervisor recovery. Main errors begin with `[ERROR]` in human mode or `ERR:` in agent mode. Failed console logging is best effort; failed daily logging adds log_error to stderr.

## TOML settings and microphone selection

```toml
[input]
microphones = ["LG Ultra*", "!Virtual*", "*"]

[output]
dir = "recordings"
bitrate = 32000
complexity = 2
sync_interval = "5s"
```

Use `[input]` and `[output]` or root dotted keys such as `output.dir`. Unknown keys, flat root keys, invalid types, and invalid final settings fail. Omitted settings use defaults: microphones ["*"], default data directory, bitrate 32000, complexity 2, sync interval 5s. Relative paths resolve against the process working directory; `~` and environment-variable text in TOML strings remain literal. Use absolute paths for background recording.

Order input.microphones from highest to lowest priority. Case-sensitive exact names and name patterns interpret only `*` as special, matching zero or more characters across the whole name. Use `LG Ultra*` for a prefix or `*Ultra*` for a substring. `uid:<UID>` matches the exact stable UID without wildcard expansion. Prefix names, patterns, or UIDs with `!` for global exclusion regardless of position. Exclusions override positive preferences and fallback. Empty selectors, bare !, or empty UID selectors fail validation.

`*` tries the macOS default input first and other eligible inputs in stable UID/ID order. An implicit final `*` retains unlisted inputs; [] also permits all inputs. Put explicit positive preferences before *. If exclusions remove every input, recording retries until an eligible input appears. Device connect/disconnect and selection changes are logged. Higher-priority inputs becoming available trigger automatic switching. An input producing no capture frames for five seconds is deferred for 30 seconds while another eligible input is tried; silence is still recorded. Microphone switching preserves the open file and encoder and can leave a capture gap.

## Files, recovery and background recording

Recordings are mono 48 kHz Ogg Opus, mixing microphone and computer playback. Files use local acquisition time: `YYYY/MM/DD/HH-MM-SS.000000000.opus`; diagnostics append to `YYYY/MM/DD/log.nljson`. Rotate at hour boundaries. Restart creates a fresh timestamp path; exclusive reservation protects existing recordings, retrying collisions with the current time. New directories use 0700 and recordings/logs use 0600. The recorder holds a `.recording.lock` file lock beneath the output directory. Retention is unlimited.

Pages are emitted during recording, approximately every 100 ms. Audio/log synchronization defaults to five seconds; segment close also synchronizes. A crash can lose buffered audio or an incomplete page; the interrupted stream lacks final trimming. Capture/storage faults retry, and dropped frames/storage errors are logged. Storage recovery can duplicate pending samples. Permissions, sleep, full disks, blocked storage, and process downtime prevent continuous capture. Check daily logs for gaps inside a file as well as segment filenames.

Print a background agent after placing the executable at its permanent path:

```sh
mkdir -p "$HOME/Recordings/ambient" "$HOME/Library/LaunchAgents"
ambient-recorder service print --output "$HOME/Recordings/ambient" > "$HOME/Library/LaunchAgents/com.alexgorbatchev.ambient-recorder.plist"
plutil -lint "$HOME/Library/LaunchAgents/com.alexgorbatchev.ambient-recorder.plist"
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.alexgorbatchev.ambient-recorder.plist"
```

The agent uses KeepAlive with a five-second throttle and writes fallback stderr to supervisor.stderr.log in the output directory. It includes the loaded absolute config path; regenerate/reload it when changing the output directory or config path. Verify capture permissions separately under launchd. Stop background recording with `launchctl bootout "gui/$(id -u)/com.alexgorbatchev.ambient-recorder"`; remove its saved plist to prevent next-login startup. Use one recorder per output directory.

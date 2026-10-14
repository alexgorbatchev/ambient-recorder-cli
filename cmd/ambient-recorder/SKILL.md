---
name: ambient-recorder
description: Use when operating ambient-recorder to record audio, configure microphone preferences, inspect inputs, or manage its background service.
author: alexgorbatchev
metadata:
  created_on: 2026-10-01 22:02
  last_modified: 2026-10-14 11:51
  status: current
---

## Recording workflow

Use macOS 14.2 or later and an available microphone. The executable includes its codec and uses Apple audio services. Grant microphone and system audio recording access when macOS requests it. Audio stays on the Mac unless output.on_close_command invokes an external program that uploads it. VAD and calendar integration are outside recording.

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
| `ambient-recorder config print-file` | `none` | Print the absolute selected/default configuration file path followed by a newline in both output modes. Inspect existence without parsing TOML, creating files/directories or opening capture. Missing files, non-regular files and inspection failures leave stdout empty, report an error on stderr and exit 1. Symlinks to regular files are accepted; print the selected symlink path. |
| `ambient-recorder config init` | `none` | Exclusively create default TOML and parent directories with owner-only permissions. Print its path; refuse to replace an existing file. |
| `ambient-recorder microphone` | `none` | Print microphone command help. |
| `ambient-recorder microphone list` | `none` | List currently available inputs, names, stable UIDs, transient IDs, default status, connection, type, manufacturer, and model. Capture is not opened. |
| `ambient-recorder recording` | `none` | Print recording command help. |
| `ambient-recorder recording print-dir` | `none` | Print the absolute configured/default recording directory followed by a newline in both output modes. Read configuration without creating directories or opening capture. |
| `ambient-recorder recording print-file` | `none` | Print all absolute open source recording paths, one per line in both output modes. Inspect the configured/default directory or --output override without creating files or opening capture. If no file is open, leave stdout empty, report an error on stderr, and exit 1. |
| `ambient-recorder recording start` | `none` | Continuously capture every input channel of the selected microphone and computer playback into separate mono Ogg Opus files and append diagnostics. Request capture permissions; create directories/files and hold an exclusive output-directory lock. |
| `ambient-recorder service` | `none` | Print service command help. |
| `ambient-recorder service print` | `none` | Print a launchd plist to stdout. Resolve the executable, loaded config, and output to absolute paths. Installation and process startup are separate shell operations. |
| `ambient-recorder service install` | `none` | Create the user's LaunchAgent plist with owner-only permissions, enable startup at login and request recording immediately. Create default TOML if absent. Refuse an existing plist; retain it on launch failure for retry with service start. |
| `ambient-recorder service start` | `none` | Refresh the installed executable and live-status endpoint, enable startup at login and request recording. Preserve recording arguments and other saved settings. Reload a loaded job when those service settings change; otherwise start it without interrupting its recording process. Require installation. |
| `ambient-recorder service stop` | `none` | Disable startup at future logins, then unload the job if loaded. Removing the job sends SIGTERM for recording finalization. Retain the plist, TOML, and recordings. |
| `ambient-recorder service restart` | `none` | Refresh the installed executable and live-status endpoint, unload the job and wait for its removal, then enable and load the saved plist. Preserve recording arguments and other saved settings. Read TOML again on process startup. Require installation. |
| `ambient-recorder service status` | `none` | Print installed and loaded booleans, CLI and live service versions, plist path, recording directory, configuration path, current recording files, active microphone and path source. Optionally include verbatim native diagnostics. Inspect registration, the saved plist and a private local socket without opening capture. |
| `ambient-recorder service uninstall` | `none` | Disable startup, unload the job if loaded, and remove the saved plist. Retain TOML, recordings and diagnostics; an already absent plist is accepted. |
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
| `all` | `--config` | `none` | `string` | `""` | TOML path; empty selects the XDG default below. An explicitly named file must exist for recording, service print and service install; config init creates it. Config print-file requires the selected/default file to exist but does not parse or validate it. Other service operations use the saved plist. Ignored by microphone, help, skill, and completion operations. |
| `all` | `--help` | `-h` | `bool` | `false` | Print help and exit. |
| `ambient-recorder` | `--version` | `-v` | `bool` | `false` | Print only the raw version plus a newline and exit. Development builds print dev; releases embed their version. |
| `ambient-recorder recording start` | `--output` | `none` | `string` | `""` | Nonempty recording directory; empty parser default inherits output.dir from TOML or the default data directory. |
| `ambient-recorder recording print-file` | `--output` | `none` | `string` | `""` | Directory to inspect; omitted inherits output.dir from TOML or the default data directory. An explicitly empty value fails validation. |
| `ambient-recorder recording start` | `--bitrate` | `none` | `int` | `32000` | Target bits/second, 6000..128000 inclusive. |
| `ambient-recorder recording start` | `--complexity` | `none` | `int` | `2` | Opus computational effort, 0..10 inclusive; higher uses more encoder CPU. |
| `ambient-recorder recording start` | `--sync-interval` | `none` | `duration` | `5s` | Positive Go duration such as 500ms, 5s, or 1m30s. Approximate interval for synchronizing audio and logs to disk. |
| `ambient-recorder service print` | `--output` | `none` | `string` | `""` | Resolve the configured/default output directory and pin it in the generated agent. |
| `ambient-recorder service install` | `--output` | `none` | `string` | `""` | Resolve the configured/default recording directory and pin it in the installed agent. A CLI override leaves the TOML value intact. |
| `ambient-recorder service status` | `--details` | `none` | `bool` | `false` | Append native launchctl diagnostics verbatim when the service is loaded. |
| `ambient-recorder completion bash` | `--no-descriptions` | `none` | `bool` | `false` | Generate completion choices without descriptions. |
| `ambient-recorder completion fish` | `--no-descriptions` | `none` | `bool` | `false` | Generate completion choices without descriptions. |
| `ambient-recorder completion powershell` | `--no-descriptions` | `none` | `bool` | `false` | Generate completion choices without descriptions. |
| `ambient-recorder completion zsh` | `--no-descriptions` | `none` | `bool` | `false` | Generate completion choices without descriptions. |

## Environment and output

- `AGENT`: trimmed, case-insensitive 1, true, or yes enables agent mode. Unset, 0, or another value uses human mode. Agent help starts with the skill-reading alert and uses untruncated compact key/value and bullet output. Human help uses aligned trees clipped to the detected terminal width, with an 80-column fallback. `skill`, raw version output, shell scripts, and launchd XML have identical contracts in both modes.
- `COLUMNS`: a positive integer overrides human help width; otherwise detect stdout terminal width.
- `XDG_CONFIG_HOME`: absolute configuration base; defaults to `$HOME/.config`. Load `ambient-recorder/config.toml` beneath it. A missing implicit default file uses defaults for recording and service configuration; config print-file exits 1 when that file is absent.
- `XDG_DATA_HOME`: absolute data base; defaults to `$HOME/.local/share`. Default output is its `ambient-recorder` subdirectory.
- `HOME`: determines the user's home directory when XDG overrides are absent.
- `NO_COLOR`: any nonempty value disables human recording-log colors.
- `TERM`: dumb disables human recording-log colors. Redirected stderr is plain text.
- `AMBIENT_RECORDER_SERVICE_SOCKET`: set by generated LaunchAgent plists to a private Unix socket beside the plist. Recording start serves its embedded version, resolved output directory, loaded config path, current source files and active microphone there until exit, reserves a retained 0600 lock file, recovers stale sockets, and removes the socket on clean shutdown. Unset or empty leaves foreground recording without a service endpoint. Endpoint startup failures exit 1 before capture.
- Help, version, config commands, microphone list, recording print-dir, recording print-file, skill, completion, and service commands return results on stdout. Recording start has no normal stdout output. Usage and failures go to stderr.
- Human microphone listings show names, default status, connection/type and available manufacturer/model details, plus stable preference selectors. Connection/type labels use identifiers available in the build SDK; unrecognized types display Unavailable. Agent listings use one flat key/value line per microphone. With no inputs, both print `No microphones available`.
- Config init prints `Configuration created: <absolute path>` in human mode or `config=<absolute path>` in agent mode.
- Service actions print a human result and `Service file: <path>`, or `service=<action> plist=<quoted path>` in agent mode. Service status prints an aligned two-column table without headers or borders in human mode, with rows Installed, Loaded by macOS, CLI version, Running service version, Service file, Recording directory, Configuration file, Current recording files and Current microphone. Recording directory and Configuration file labels include `(saved)` when their values come from saved service settings. Human status abbreviates an absolute HOME prefix to `~/`; paths outside that directory retain their absolute form. Agent mode retains absolute paths and prints one line: `installed=<bool> loaded=<bool> cli_version=<quoted version> service_version=<quoted version or state> plist=<quoted path> output_dir=<quoted path or state> config_path=<quoted path or state> current_files=<quoted newline-separated paths or state> paths_source=<quoted source> microphone=<quoted name or state>`.
- Status path sources are live service (`live`), saved service configuration (`saved`) or unavailable (`unavailable`). CLI version belongs to the invoked executable; running version comes from the live process independently of upgrades on disk. Live paths are absolute startup values, unaffected by the status command's --config, working directory, XDG environment or subsequent TOML edits. No loaded config displays `none (defaults)`; a responding recorder with no open segment displays `none`. The track group updates its complete current-files snapshot on open, channel addition, rotation and close. An omitted file list displays `unavailable`; a reported empty list displays `none`. Human output lists source paths on separate lines in the table cell; agent output encodes their separating newlines as escapes inside the quoted current_files value. The current microphone comes from capture session open/close events: a reported empty name displays `none`, an omitted name displays `unavailable`, and an opened input with missing device metadata displays `unavailable`. These snapshots belong to the service process, rather than other recorders or the system's preferred input.
- When live paths are missing, status reads absolute --output and --config arguments from the saved plist without loading TOML. Missing, malformed or incomplete saved paths display `unavailable`. An unloaded job reports service version, current source files and microphone as `not running` and ignores any remaining socket. A loaded job without live identity reports those values as `unavailable`; a version-only response preserves its version but cannot establish recording paths or microphone. Socket and saved-plist queries each time out after one second. Loaded status, a version response, an open-file snapshot and a microphone name do not establish audio capture health. The optional native report is displayed verbatim in both modes.
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
on_close_command = []
```

Use `[input]` and `[output]` or root dotted keys such as `output.dir`. Unknown keys, flat root keys, invalid types, and invalid final settings fail. Omitted settings use defaults: microphones ["*"], default data directory, bitrate 32000, complexity 2, sync interval 5s. Relative paths resolve against the process working directory; `~` and environment-variable text in TOML strings remain literal. Use absolute paths for background recording.

Order input.microphones from highest to lowest priority. Case-sensitive exact names and name patterns interpret only `*` as special, matching zero or more characters across the whole name. Use `LG Ultra*` for a prefix or `*Ultra*` for a substring. `uid:<UID>` matches the exact stable UID without wildcard expansion. Prefix names, patterns, or UIDs with `!` for global exclusion regardless of position. Exclusions override positive preferences and fallback. Empty selectors, bare !, or empty UID selectors fail validation.

`*` tries the macOS default input first and other eligible inputs in stable UID/ID order. An implicit final `*` retains unlisted inputs; [] also permits all inputs. Put explicit positive preferences before *. If exclusions remove every input, recording retries until an eligible input appears. Device connect/disconnect and selection changes are logged. Higher-priority inputs becoming available trigger automatic switching. An input producing no capture frames for five seconds is deferred for 30 seconds while another eligible input is tried; silence is still recorded. Microphone switching preserves existing source files and encoders and can leave a capture gap.

## Files, recovery and background recording

### Optional finalized-file command

Set output.on_close_command to an array of strings containing an executable followed by literal arguments. Omit it or use [] to disable execution. A blank executable or any NUL byte fails configuration validation. There is no CLI flag. Replace {file} with the absolute finalized Opus path and {stem} with that path without .opus in any argument. Other text remains literal; invoke a script or shell explicitly for shell features. The executable inherits the recorder's working directory and environment; executable lookup uses its PATH. A background service does not load interactive shell startup files. Supply command credentials and PATH through the service environment or a wrapper script.

Run one command for each nonempty source file whose encoder finalization, synchronization and close succeed, including hourly rotation and orderly recorder shutdown. Commands run serially outside capture. Write combined stdout/stderr to {stem}.command.log, created with mode 0600. Record success or failure in the recorder diagnostics; command errors do not change recording's exit status or stop capture. Terminate the command process group after ten minutes. Orderly shutdown waits for queued jobs while maintaining watchdog progress. Supervisor termination or a crash can interrupt jobs; the queue is in memory, with no retries or historical backfill. Files are passed unchanged, without overlap. The external program owns transcript creation, credentials, uploads and billing.

For Deepgram Markdown alongside each Opus file, install deepgram-transcribe, provide DEEPGRAM_API_KEY to its inherited environment and set:

```toml
[output]
on_close_command = ["deepgram-transcribe", "transcript", "create", "{file}", "--no-trim-silence", "--output", "{stem}.md"]
```

Use `ambient-recorder config print-file` to locate the existing configuration selected by --config or the XDG default. It prints only the absolute path and a newline, even if the TOML contents are invalid; an absent file exits 1 with empty stdout. Use `ambient-recorder recording print-dir` to resolve storage from the selected TOML or defaults, with relative paths based on the invocation's working directory. Recording path commands honor --config and require an explicitly selected configuration file to exist. Use `ambient-recorder recording print-file` to list every open source segment in that directory, one absolute path per line, or add --output to target a recording started with an override or a service pinned to a different directory. Results are snapshots; an open file does not establish that audio frames are currently arriving. The recorder atomically publishes a .recording.current JSON snapshot containing relative source paths after encoder initialization, removes it before finalization, and holds an exclusive lock on every source segment until close. Queries return only segments that are still locked, rejecting stale snapshots after a crash. Restart with the updated executable to publish source-file snapshots.

Recordings are separate mono 48 kHz Ogg Opus files for microphone input channels `mic-1`, `mic-2`, and so on, plus mono `playback`. Routing is automatic, with no channel setting. Files use local acquisition time: `YYYY/MM/DD/HH-MM-SS.000000000-<source>.opus`; diagnostics append to `YYYY/MM/DD/log.nljson`. Source groups share a timestamp and rotate together at hour boundaries. Source numbers identify channels of the selected device, not speaker identities. A newly appearing microphone channel is padded with silence from the current group's start. A disappearing channel remains silent in that group until rotation; the next group contains only channels then present. Restart creates a fresh timestamp path; exclusive reservation protects existing recordings, retrying collisions with the current time. New directories use 0700 and recordings/logs use 0600. The recorder holds a `.recording.lock` file lock beneath the output directory. Retention is unlimited.

Pages are emitted during recording, approximately every 100 ms. Audio/log synchronization defaults to five seconds; segment close also synchronizes. A crash can lose buffered audio or an incomplete page; the interrupted stream lacks final trimming. Capture/storage faults retry, and dropped frames/storage errors are logged. Storage recovery can duplicate pending samples and leave unequal durations in the failed source group; recovery closes all of its encoders and replays the block into a fresh group. Bitrate applies separately to each mono source file. Permissions, sleep, full disks, blocked storage, and process downtime prevent continuous capture. Check daily logs for gaps inside a file as well as segment filenames.

Place the executable at its permanent path and grant capture permissions in a foreground recording before installing background recording:

```sh
ambient-recorder service install
ambient-recorder service status --details
ambient-recorder service restart
ambient-recorder service stop
ambient-recorder service start
ambient-recorder service uninstall
```

Run service commands as the logged-in user, without sudo. Install saves `~/Library/LaunchAgents/com.alexgorbatchev.ambient-recorder.plist`, referencing the executable's resolved absolute path. It creates a default TOML if absent and includes its absolute path. The recording directory is pinned as an absolute --output argument, matching supervisor diagnostics. Start and restart atomically refresh the saved executable to the invoked CLI and configure its live-status endpoint, preserving recording arguments, environment entries and other launchd settings. Reinstall with `service uninstall` then `service install` when changing the recording directory or configuration file path. Changing other TOML settings requires `service restart`.

Generated and refreshed plists set the live-status socket to `.ar-<eight hexadecimal digits>.sock` beside the plist, with 0600 socket permissions and a retained `.sock.lock` file. Use service restart with the updated executable to enable reporting from an installation missing that setting. Long socket paths beyond the native macOS limit fail explicitly. Shutdown waits up to two minutes for launchd to remove the departing job; cancellation and shutdown errors fail the command instead of reporting success.

Restart the service with the updated executable to enable live recording-path and microphone reporting. A process that has not published those values cannot establish them for status. The microphone name comes from the opened capture device and changes on handover; an empty active input means no capture is open. An active file and selected microphone do not establish that audio frames are arriving.

The Aqua-session agent starts at GUI login after each reboot, and uses KeepAlive with a five-second restart throttle. It stops at logout and cannot capture during sleep or before login. macOS Background Items settings and capture permissions can prevent startup or capture. Verify permissions separately under launchd and inspect `YYYY/MM/DD/log.nljson` plus `supervisor.stderr.log` in the installed output directory. New plist and configuration files use 0600; new directories use 0700, and the agent uses umask 077. Service controls call the built-in /bin/launchctl; Apple's Foundation serializes the plist in the executable. `service print` remains read-only. Stop a foreground recorder before installing or starting a service against the same directory; only one recorder can acquire its recording lock.

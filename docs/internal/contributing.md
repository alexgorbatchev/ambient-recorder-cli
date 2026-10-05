---
created_on: 2026-09-30 12:30
last_modified: 2026-10-05 11:47
status: current
---

# Contributor Instructions

For developers building and validating `ambient-recorder` on macOS. The project uses the approved Go modules, including go-toml/v2 2.4.3, and statically links the approved native Opus libraries. Do not add external dependencies without the user's approval.

## Prerequisites and Build

Use macOS 14.2+, [Go 1.27.1](https://go.dev/dl/), Apple's Command Line Tools (`clang`, SDK, `make`), and the installed [just](https://just.systems/) task runner. Building native dependencies downloads the approved source archives using macOS `curl`, checks their pinned SHA-256 sums, and configures static libraries under `.tmp/native`. The application itself requires only its executable and macOS frameworks.

```sh
just build
just check
just run --help
just run-ai recording start --help
```

`bin/ambient-recorder` includes an embedded Info.plist with microphone and audio-capture usage descriptions. Use `just build` to preserve that metadata and the deployment target. Tests compile native helpers with Apple's compiler and run under the race detector; hardware capture requires explicit opt-in. Test directories are allocated under the module's `.tmp` using `internal/testdir` and remain after tests for inspection, including compiled decoder helpers. The justfile sets TMPDIR to `.tmp` and uses Go's `-work` flag to retain compiler intermediates. `.tmp` also stores dependencies and intermediate artifacts and is ignored by Git.

## Runtime Design

Configuration overlays go-toml's strict decoded TOML fields on typed defaults, then applies explicitly changed Cobra flags and validates final values. Unknown keys and type errors fail before capture. The optional XDG default file falls back to defaults when absent; an explicit file must exist. `config init` serializes defaults with go-toml and reserves the target file exclusively with mode 0600. Configuration is read at process start, without live file reloading. Service generation carries the loaded configuration's absolute path and pins the resolved absolute output directory so launchd's working directory cannot move recordings or supervisor diagnostics. Changing the background output directory requires regenerating the agent.

Core Audio creates a private, unmuted global mono playback tap and a private aggregate containing the selected microphone. The tap uses native drift compensation. The real-time callback copies and averages float channels into a bounded C queue without allocation, locks, file I/O, or calls into Go. Go polls every 20 ms, preserving native acquisition timestamps. Once per second it compares the native clock mapping with wall time; shifts larger than one second recalibrate the mapping and emit a diagnostic. Calendar paths then follow corrected wall time, while queued blocks retain their offset from the native clock.

The aggregate's nominal sample rate determines the mixed PCM clock. A verified Bluetooth configuration reports a 16 kHz microphone/aggregate and a 48 kHz tap stream, while both callback buffers contain 320 frames per 20 ms. Requiring every advertised stream rate to match rejects this working configuration. Callback buffer lengths and channel counts are checked before reading samples; format changes restart capture. The native 256-block queue drops new blocks on overflow and counts dropped frames. Its duration depends on the hardware block size: the measured 320-frame/16 kHz configuration buffers approximately 5.12 seconds.

Apple's AudioConverterFillComplexBuffer converts capture PCM to mono 48 kHz outside the real-time callback, using normal priming and medium quality. The input callback uses native-owned storage; it signals temporarily exhausted input separately from EOF and drains look-ahead when a capture session ends. A 48 kHz capture bypasses conversion. libopusenc owns encoder delay, pre-skip, final trimming, Ogg checksums and page construction. The Go wrapper uses its pull API, writes headers immediately, disables DTX and decision delay, and requests 100 ms page muxing. Encoder complexity defaults to 2, target bitrate to 32 kb/s. One Go goroutine owns the encoder across microphone sessions.

Recording splits PCM at acquisition-time local hour boundaries, drains each encoder, synchronizes and closes it, then opens an exclusively allocated next segment. Filenames use HH-MM-SS.000000000.opus with nine fractional-second digits. Creation never scans, parses, or migrates existing filenames. If the exact timestamp path exists, reservation retries using the actual current clock and its calendar directory. A rooted filesystem and advisory kernel lock confine paths and exclude another recorder using the same directory.

The recording sink publishes a relative `.recording.current` symlink after encoder initialization and holds an exclusive advisory lock on that segment until its file closes. It removes the pointer before finalization. Current-file inspection opens the storage root without creating directories, reads the pointer, and probes the target with a nonblocking shared lock. A successful probe means the recorder no longer owns the file and the pointer is stale; a conflicting exclusive lock identifies an open segment. Concurrent queries use compatible shared locks so they cannot mistake each other for a recorder. The result is a snapshot of an open file, which does not establish that capture is currently producing audio. Recording processes from builds without this publication mechanism expose no current-file pointer.

Microphone inventory provides stable UIDs and device-reported metadata without opening capture. Preferences match exact names or uid:<UID>, with * trying the default input first and then remaining inputs in stable UID order. Unlisted inputs remain eligible through an implicit final *. Inventory is refreshed on native device/default-input notifications and every five seconds; selection is checked once per second. Connect/disconnect events include identity and metadata, and the recorder's own private aggregate is excluded by its actual device ID. Failed capture inputs receive a 30-second cooldown while other inputs are tried. Capture handover drains and closes native capture/conversion objects while preserving the current output file and encoder. Audio downtime can remain inside that file, so filenames alone are insufficient to identify every gap.

Capture failures retry from 250 ms to five seconds. Storage failures close the damaged segment and replay the pending block into a new segment; this may duplicate already-written samples and is logged. A storage stall beyond the finite capture queue loses incoming frames, which is logged once control returns. A 90-second watchdog covers initialization, normal operation, and deferred cleanup. Its final stderr write is bounded, including when stderr is a full pipe. Failed native deregistration retains callback memory and returns an error requiring process restart, avoiding use-after-free or repeated leaks.

Daily logs use debug-level JSON. On reopening a log, an incomplete tail is preserved inside a recovery event before valid newline-delimited logging resumes. Failed logging falls back to stderr. Audio and logs synchronize at the configured interval, but parent directories are not explicitly synchronized; this is not a power-loss durability guarantee.

Each capture opening reads the selected microphone's Core Audio device name, transport, input stream terminal type, manufacturer, and model name. The daily log and agent stderr retain full metadata, sample rate and output directory. Human stderr uses tint with the microphone name, connection, playback source and output path; startup inventory and encoder metadata stay in diagnostics. `microphone list` exposes hardware details explicitly. Metadata failures do not interrupt capture. A device can report a generic microphone terminal type even when it belongs to a headset or display; the recorder does not infer hardware categories from names.

## CI and Release Packaging

GitHub Actions runs the shared native verification workflow on macOS 15 Apple Silicon and Intel, using Xcode 26.2 and Go from go.mod. It installs the approved just 1.58.0 archive with a pinned SHA-256, then runs `just release` with an explicit version. Native libraries are built on each architecture; no cross-compiled binary is treated as a tested native build.

```sh
RECORDER_VERSION=1.0.0 just release
```

The recipe runs the normal checks, injects `main.version`, ad-hoc signs the executable and packages `dist/ambient-recorder_1.0.0_darwin_<arch>.tar.gz`. Verification executes both output modes, compares the embedded skill with its maintained source while running away from repository files, validates architecture, the linked macOS 14.2 deployment target and privacy metadata, and rejects non-system dynamic dependencies. The justfile exports deployment flags for every CGO object, including runtime/cgo, and for the final link. Native dependency archives are inspected for the same target; rebuilds preserve old objects and sources under `.tmp`. The extracted archive is checked again. Staging and verification evidence remain in `.tmp`; compiled application binaries remain in `bin/`.

Push an annotated release tag only after CI passes on its target commit and the release-equivalent binary reports the intended version. The tag-triggered workflow repeats native checks for both architectures and publishes only after both pass, with `checksums.txt`. Publishing requires explicit user authorization. These builds are not Developer ID signed or notarized; no signing credentials are configured.

CLI behavior changes require updating `cmd/ambient-recorder/SKILL.md` and its metadata in the same change. Tests compare command/flag documentation with the live Cobra tree and check every help path, including generated commands. Use `AGENT=1 ambient-recorder skill` to inspect the reference embedded in a distributed executable.

## Live Verification

Start a controlled foreground recording in a dedicated directory, then stop it with SIGTERM for normal finalization or SIGKILL to check interrupted streams. Never run these tests against valuable recordings. An opt-in test inspects the resulting Opus file without opening hardware or sending it anywhere:

```sh
RECORDER_OPUS_FILE="$PWD/.tmp/microphone-continuity-live/2026/09/30/16-34-22.282975854.opus" RECORDER_REQUIRE_EOS=1 TMPDIR="$PWD/.tmp" go test -v ./internal/opus -run TestRecordedFile
```

Replace the path with the actual file. Omit `RECORDER_REQUIRE_EOS` for an interrupted stream. `RECORDER_REQUIRE_TONE=1` additionally requires an 880 Hz playback test signal. The test checks Ogg CRCs and decodes actual packets with the linked libopus.

An explicit live capture check selects two available physical inputs sequentially and verifies that capture sessions retain one output file and one logical Opus stream. It preserves audio under the provided directory and leaves the system default microphone unchanged. Choose a fresh output directory for each run:

```sh
RECORDER_LIVE_OUTPUT="$PWD/.tmp/live-switch-check" TMPDIR="$PWD/.tmp" go test -v -ldflags="-extldflags=-Wl,-sectcreate,__TEXT,__info_plist,$PWD/internal/capture/Info.plist" ./internal/recording -run '^TestLiveMicrophoneSwitch$'
```

`service install` creates and loads a per-user Aqua LaunchAgent, starting at GUI login after reboot. The service uses native `launchctl enable`, `bootstrap`, `kickstart`, `disable`, and `bootout` controls. Restart unloads the job to deliver SIGTERM before loading it again; it avoids forced process killing. `service stop` disables startup across logins, and `service start` re-enables it. `service status --details` displays native diagnostics verbatim rather than parsing an unsupported text format. Generation with `service print` remains read-only. Apple's Foundation serializes property lists; syntax validation alone is insufficient to establish that launchd accepts an XML representation.

Do not automatically install or start a persistent service as part of builds or tests. Service lifecycle tests register unique temporary jobs from retained `.tmp` plists using `/usr/bin/false`, exercise the real launchd controls, then unload them. They skip when no GUI login domain exists and never open audio hardware or save login-startup jobs. Local hardware checks cover foreground arm64 capture and sequential handover between physical microphones in one file. CI tests native binaries without opening hardware. Earlier supported macOS versions, physical unplug/reconnect, permissions under launchd, full disks, sleep/wake and long-running capture need their own live validation. Transcription, VAD and calendar integration remain future work.

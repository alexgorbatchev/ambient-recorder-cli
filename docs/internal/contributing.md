---
created_on: 2026-09-30 12:30
last_modified: 2026-09-30 20:42
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

`bin/ambient-recorder` includes an embedded Info.plist with microphone and audio-capture usage descriptions. Use `just build` to preserve that metadata. Tests compile native helpers with Apple's compiler and run under the race detector; hardware capture requires explicit opt-in. Test directories are allocated under the module's `.tmp` using `internal/testdir` and remain after tests for inspection, including compiled decoder helpers. `.tmp` also stores dependencies and intermediate artifacts and is ignored by Git.

## Runtime Design

Configuration overlays go-toml's strict decoded TOML fields on typed defaults, then applies explicitly changed Cobra flags and validates final values. Unknown keys and type errors fail before capture. The optional XDG default file falls back to defaults when absent; an explicit file must exist. `config init` serializes defaults with go-toml and reserves the target file exclusively with mode 0600. Configuration is read at process start, without live file reloading. Service generation carries the loaded configuration's absolute path and pins the resolved absolute output directory so launchd's working directory cannot move recordings or supervisor diagnostics. Changing the background output directory requires regenerating the agent.

Core Audio creates a private, unmuted global mono playback tap and a private aggregate containing the selected microphone. The tap uses native drift compensation. The real-time callback copies and averages float channels into a bounded C queue without allocation, locks, file I/O, or calls into Go. Go polls every 20 ms, preserving native acquisition timestamps. Once per second it compares the native clock mapping with wall time; shifts larger than one second recalibrate the mapping and emit a diagnostic. Calendar paths then follow corrected wall time, while queued blocks retain their offset from the native clock.

The aggregate's nominal sample rate determines the mixed PCM clock. A verified Bluetooth configuration reports a 16 kHz microphone/aggregate and a 48 kHz tap stream, while both callback buffers contain 320 frames per 20 ms. Requiring every advertised stream rate to match rejects this working configuration. Callback buffer lengths and channel counts are checked before reading samples; format changes restart capture. The native 256-block queue drops new blocks on overflow and counts dropped frames. Its duration depends on the hardware block size: the measured 320-frame/16 kHz configuration buffers approximately 5.12 seconds.

Apple's AudioConverterFillComplexBuffer converts capture PCM to mono 48 kHz outside the real-time callback, using normal priming and medium quality. The input callback uses native-owned storage; it signals temporarily exhausted input separately from EOF and drains look-ahead when a capture session ends. A 48 kHz capture bypasses conversion. libopusenc owns encoder delay, pre-skip, final trimming, Ogg checksums and page construction. The Go wrapper uses its pull API, writes headers immediately, disables DTX and decision delay, and requests 100 ms page muxing. Encoder complexity defaults to 2, target bitrate to 32 kb/s. One Go goroutine owns the encoder across microphone sessions.

Recording splits PCM at acquisition-time local hour boundaries, drains each encoder, synchronizes and closes it, then opens an exclusively allocated next segment. Filenames use HH-MM-SS.000000000.opus with nine fractional-second digits. Creation never scans, parses, or migrates existing filenames. If the exact timestamp path exists, reservation retries using the actual current clock and its calendar directory. A rooted filesystem and advisory kernel lock confine paths and exclude another recorder using the same directory.

Microphone inventory provides stable UIDs and device-reported metadata without opening capture. Preferences match exact names or uid:<UID>, with * trying the default input first and then remaining inputs in stable UID order. Unlisted inputs remain eligible through an implicit final *. Inventory is refreshed on native device/default-input notifications and every five seconds; selection is checked once per second. Connect/disconnect events include identity and metadata, and the recorder's own private aggregate is excluded by its actual device ID. Failed capture inputs receive a 30-second cooldown while other inputs are tried. Capture handover drains and closes native capture/conversion objects while preserving the current output file and encoder. Audio downtime can remain inside that file, so filenames alone are insufficient to identify every gap.

Capture failures retry from 250 ms to five seconds. Storage failures close the damaged segment and replay the pending block into a new segment; this may duplicate already-written samples and is logged. A storage stall beyond the finite capture queue loses incoming frames, which is logged once control returns. A 90-second watchdog covers initialization, normal operation, and deferred cleanup. Its final stderr write is bounded, including when stderr is a full pipe. Failed native deregistration retains callback memory and returns an error requiring process restart, avoiding use-after-free or repeated leaks.

Daily logs use debug-level JSON. On reopening a log, an incomplete tail is preserved inside a recovery event before valid newline-delimited logging resumes. Failed logging falls back to stderr. Audio and logs synchronize at the configured interval, but parent directories are not explicitly synchronized; this is not a power-loss durability guarantee.

Each capture opening reads the selected microphone's Core Audio device name, transport, input stream terminal type, manufacturer, and model name. It prints these with the device ID, sample rate, playback source, and absolute output directory to stderr. The daily log includes microphone metadata, sample rate, and output directory. Missing name metadata displays `name unavailable`; missing connection or input type displays `Unavailable`. Manufacturer and model are omitted from the terminal output when absent. Metadata failures do not interrupt capture. A device can report a generic microphone terminal type even when it belongs to a headset or display; the recorder does not infer hardware categories from names.

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

Background supervision configuration is generated by `service print`; installation instructions are in the README. Generation is read-only. Do not automatically install or start a persistent service as part of builds or tests. Current verification covers the foreground arm64 binary on macOS 26.6.2, sequential capture handover from Bluetooth to USB in one file, and a live TOML preference selecting USB over the macOS default Bluetooth input. Intel, earlier supported macOS versions, physical unplug/reconnect, permissions under launchd, full disks, sleep/wake and long-running capture need their own live validation. No CI, releases, notarization, transcription, VAD or calendar integration is configured.

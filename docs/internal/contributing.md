---
created_on: 2026-09-30 12:30
last_modified: 2026-09-30 13:46
status: current
---

# Contributor Instructions

For developers building and validating `ambient-recorder` on macOS. The project uses the approved Go modules and statically links the approved native Opus libraries. Do not add external dependencies without the user's approval.

## Prerequisites and Build

Use macOS 14.2+, [Go 1.27.1](https://go.dev/dl/), Apple's Command Line Tools (`clang`, SDK, `make`), and the installed [just](https://just.systems/) task runner. Building native dependencies downloads the approved source archives using macOS `curl`, checks their pinned SHA-256 sums, and configures static libraries under `.tmp/native`. The application itself requires only its executable and macOS frameworks.

```sh
just build
just check
just run --help
just run-ai recording start --help
```

`bin/ambient-recorder` includes an embedded Info.plist with microphone and audio-capture usage descriptions. Use `just build` to preserve that metadata. Tests compile native helpers with Apple's compiler and run under the race detector; they never open the microphone automatically. `.tmp` stores dependencies, intermediate artifacts and test directories and is ignored by Git.

## Runtime Design

Core Audio creates a private, unmuted global mono playback tap and a private aggregate containing the default microphone. The tap uses native drift compensation. The real-time callback copies and averages float channels into a bounded C queue without allocation, locks, file I/O, or calls into Go. Go polls every 20 ms, preserving native acquisition timestamps. Once per second it compares the native clock mapping with wall time; shifts larger than one second recalibrate the mapping and emit a diagnostic. Calendar paths then follow corrected wall time, while queued blocks retain their offset from the native clock.

The aggregate's nominal sample rate determines the mixed PCM clock. A verified Bluetooth configuration reports a 16 kHz microphone/aggregate and a 48 kHz tap stream, while both callback buffers contain 320 frames per 20 ms. Requiring every advertised stream rate to match rejects this working configuration. Callback buffer lengths and channel counts are checked before reading samples; format changes restart capture. The native 256-block queue drops new blocks on overflow and counts dropped frames. Its duration depends on the hardware block size: the measured 320-frame/16 kHz configuration buffers approximately 5.12 seconds.

libopusenc owns input resampling, encoder delay, pre-skip, final trimming, Ogg checksums and page construction. The Go wrapper uses its pull API, writes headers immediately, disables DTX and decision delay, and requests 100 ms page muxing. Encoder complexity defaults to 2, target bitrate to 32 kb/s. One Go goroutine owns each encoder.

Recording splits PCM at acquisition-time local hour boundaries, drains each encoder, synchronizes and closes it, then opens an exclusively allocated next segment. Numeric suffix allocation uses the largest existing suffix plus one, supports more than three digits, and never overwrites an existing file. A rooted filesystem and advisory kernel lock confine paths and exclude another recorder using the same directory.

Capture failures retry from 250 ms to five seconds. Storage failures close the damaged segment and replay the pending block into a new segment; this may duplicate already-written samples and is logged. A storage stall beyond the finite capture queue loses incoming frames, which is logged once control returns. A 90-second watchdog covers initialization, normal operation, and deferred cleanup. Its final stderr write is bounded, including when stderr is a full pipe. Failed native deregistration retains callback memory and returns an error requiring process restart, avoiding use-after-free or repeated leaks.

Daily logs use debug-level JSON. On reopening a log, an incomplete tail is preserved inside a recovery event before valid newline-delimited logging resumes. Failed logging falls back to stderr. Audio and logs synchronize at the configured interval, but parent directories are not explicitly synchronized; this is not a power-loss durability guarantee.

Each capture opening reads the selected microphone's Core Audio device name and prints it with its device ID, sample rate, playback source, and absolute output directory to stderr. The daily log includes the microphone name, device ID, sample rate, and output directory. Missing name metadata displays `name unavailable` without interrupting capture.

## Live Verification

Start a controlled foreground recording in a dedicated directory, then stop it with SIGTERM for normal finalization or SIGKILL to check interrupted streams. Never run these tests against valuable recordings. An opt-in test inspects the resulting Opus file without opening hardware or sending it anywhere:

```sh
RECORDER_OPUS_FILE="$PWD/.tmp/live-recordings/2026/09/30/12-000.opus" RECORDER_REQUIRE_EOS=1 TMPDIR="$PWD/.tmp" go test -v ./internal/opus -run TestRecordedFile
```

Replace the path with the actual file. Omit `RECORDER_REQUIRE_EOS` for an interrupted stream. `RECORDER_REQUIRE_TONE=1` additionally requires an 880 Hz playback test signal. The test checks Ogg CRCs and decodes actual packets with the linked libopus.

Background supervision configuration is generated by `service print`; installation instructions are in the README. Generation is read-only. Do not automatically install or start a persistent service as part of builds or tests. Current verification covers the foreground arm64 binary on macOS 26.6.2; Intel, earlier supported macOS versions, physical microphone handover, permissions under launchd, full disks, sleep/wake and long-running capture need their own live validation. No CI, releases, notarization, transcription, VAD or calendar integration is configured.

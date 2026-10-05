---
created_on: 2026-09-30 12:48
last_modified: 2026-10-05 10:18
status: archived
---

# Verification Evidence

Archived September 30, 2026 verification record for maintainers. Results and unverified boundaries apply to the bootstrap builds exercised then, rather than the current implementation. See the [contributor instructions](../../contributing.md) for current verification procedures.

Evidence files are documentation copies of the retained execution logs and generated plist. Host-specific repository and system temporary-directory prefixes are replaced with `/REDACTED/REPOSITORY` and `/REDACTED/SYSTEM-TEMP`; trailing whitespace and a blank final line are removed where present. System framework and library paths remain to document linkage. Unmodified originals remain locally under `.tmp/documentation-organization/originals`. The plist is historical evidence with redacted paths; it is not an installation template.

This record identifies the tested scope of the September 30 bootstrap for maintainers. Execution evidence is retained in the [evidence directory](evidence/). Tests do not establish an uninterrupted-recording guarantee.

| Check | Observed result | Evidence |
| :--- | :--- | :--- |
| `just check` | Build, vet, formatting, module hygiene and all five packages' race tests pass | [check.log](evidence/check.log) |
| Native memory/undefined behavior checks | Capture queue helper passes AddressSanitizer and UndefinedBehaviorSanitizer | [capture-sanitizer.log](evidence/capture-sanitizer.log) |
| Foreground capture and playback | 157.120 seconds of actual audio decode; finalized EOS; known 880 Hz playback amplitude 0.099859 | [live-decode.log](evidence/live-decode.log) |
| CPU and memory | 157.75 seconds wall time, 1.62 seconds user CPU + 0.62 seconds system CPU, 26,525,696 bytes maximum RSS | [live-time5.log](evidence/live-time5.log) |
| Forced process crash | SIGKILL of PID 82703 leaves `12-001.opus` without EOS; 52.294 seconds of saved packets decode | [crash-decode.log](evidence/crash-decode.log) |
| Restart and cleanup | Restart creates `12-002.opus`; SIGTERM finalizes 46.500 seconds with EOS | [restart-decode.log](evidence/restart-decode.log), [live-log.nljson](evidence/live-log.nljson) |
| Capture clock recalibration | Final executable captures and finalizes another recording after periodic calibration is integrated | [final-live-decode.log](evidence/final-live-decode.log) |
| Native sample-rate topology | Bluetooth microphone and aggregate report 16 kHz; tap advertises 48 kHz; both actual callback buffers contain 320 float frames | [native-probe.log](evidence/native-probe.log) |
| Binary packaging | Opus libraries are static; runtime linkage contains only Apple libraries/frameworks; privacy strings are embedded | [linkage.log](evidence/linkage.log), [embedded-plist.log](evidence/embedded-plist.log), [binary-sha256.log](evidence/binary-sha256.log) |
| Supervision configuration | Generated plist passes native `plutil -lint`; agent is not installed or started | [agent-lint.log](evidence/agent-lint.log) |
| Dependencies and help | Actual module graph matches approved versions; both CLI output modes run | [modules.log](evidence/modules.log), [human-help.log](evidence/human-help.log), [agent-help.log](evidence/agent-help.log) |

The CPU measurement is approximately **1.42% of one core for this recorder process**, on an Apple M4 Pro running macOS 26.6.2. It excludes CPU consumed by other system processes and is a short, mostly quiet run with a three-second playback tone. It is not a worst-case benchmark or a guarantee for other devices. The later once-per-second clock comparison is validated by the final live recording; that additional comparison is absent from the earlier CPU measurement.

## Tests Fail When Behavior Is Disabled

Missing-symbol/compiler failures are recorded during initial red phases, including [storage-red.log](evidence/storage-red.log), [opus-red.log](evidence/opus-red.log), [watchdog-red.log](evidence/watchdog-red.log), and [clock-red.log](evidence/clock-red.log). The service test fails before its command exists: [service-red.log](evidence/service-red.log).

Original initial test logs use macOS system temporary paths; documentation copies redact those prefixes. The `just test` recipe sets `TMPDIR` to the project's `.tmp`; the recorded final checks use that recipe.

Temporary changes to owned source files demonstrate that the behavioral tests detect broken functionality. Each change is restored before the final successful check:

- Disabling exclusive allocation collapses sixteen concurrent reservations into one path: [storage-disabled.log](evidence/storage-disabled.log).
- Disabling codec drain removes EOS and loses the final samples at 16/44.1/48 kHz: [opus-disabled.log](evidence/opus-disabled.log).
- Disabling hourly rotation writes 960 samples into the first file instead of 480: [rotation-disabled.log](evidence/rotation-disabled.log).
- Disabling watchdog exit lets the stalled child process finish normally: [watchdog-disabled.log](evidence/watchdog-disabled.log).
- Blocking the watchdog's stderr pipe prevents recovery before the bounded write is implemented: [watchdog-stderr-red.log](evidence/watchdog-stderr-red.log).
- Disabling timestamp reanchoring assigns the wrong acquisition time after a two-hour clock change: [clock-disabled.log](evidence/clock-disabled.log).

## Boundaries

Physical microphone removal/default-device handover, launchd privacy permissions and relaunch behavior, full-disk recovery, sleep/wake, Intel, earlier macOS versions, power loss and long-duration capture remain unverified. The implementation retries capture and storage faults, logs losses and supports launchd supervision; no finite-buffer process can retain audio while killed, asleep, blocked indefinitely, denied access, or unable to store data. No persistent recording process is left running after these checks.

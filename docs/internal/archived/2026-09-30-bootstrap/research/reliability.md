---
created_on: 2026-09-30 17:42
last_modified: 2026-10-05 10:18
status: archived
---

# Continuous microphone recording: reliability and operational boundaries

Archived September 30, 2026 research for maintainers. Version comparisons, approval proposals, indexed naming recommendations and open questions are historical findings, not current requirements. Read the [repository instructions](../../../../../AGENTS.md) and [contributor instructions](../../../contributing.md) for current decisions.

Research notes for the coordinator implementing the requested Go recorder. These are sourced findings and candidate recommendations, not an approved engineering design. The coordinator confirms that the user selected macOS-only capture of microphone and system playback. This note covers operational reliability and microphone capture boundaries; playback-specific capture is researched separately.

## Can an always-on recorder guarantee uninterrupted audio?

### Takeaway
A process can recover from failures, but it cannot guarantee uninterrupted recording under every condition. A present microphone does not remove permission, sleep, process-termination, or storage failures.

### Cited Findings
- Go explicitly states that SIGKILL and SIGSTOP cannot be caught; shutdown cleanup cannot handle these signals. SIGINT/SIGTERM can be delivered through os/signal for graceful handling. — [Go os/signal](https://pkg.go.dev/os/signal)
- macOS 10.14 and later requires explicit microphone permission. Apple's capture authorization guidance requires NSMicrophoneUsageDescription and states that a missing appropriate usage key can terminate an app that requests access. This is app guidance; standalone CLI permission identity and metadata need separate integration testing. — [Apple media capture authorization](https://developer.apple.com/documentation/bundleresources/requesting-authorization-for-media-capture-on-macos)
- Apple's idle-system-sleep assertion prevents idle sleep, while permitting sleep for lid closure, Apple-menu requests, low battery, and other reasons. — [Apple idle-sleep assertion](https://developer.apple.com/documentation/iokit/kiopmassertiontypepreventuseridlesystemsleep)
- Apple states that power assertions are suggestions and may be overridden during low power or thermal emergencies. — [Apple IOPM assertion types](https://developer.apple.com/documentation/iokit/iopmlib_h/iopmassertiontypes)
- Darwin writes can fail with ENOSPC, EDQUOT, EIO, or device errors. File sync can report queued write failures too. — [Apple write manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/write.2); [Apple fsync manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/fsync.2)
- Apple says Core Audio switches to another available audio device when the selected device is disconnected; this describes system routing, not a guarantee that an existing third-party capture stream survives unchanged. — [Apple Audio MIDI Setup](https://support.apple.com/en-euro/guide/audio-midi-setup/ams0c519de39/mac)

### Inferences
- Recommend treating “always on” as recover-and-retry while prerequisites hold, with measurable gaps and counters. Audio during process death, OS sleep, or microphone denial cannot be recreated. This follows from the verified termination and sleep boundaries. — [Go os/signal](https://pkg.go.dev/os/signal); [Apple idle-sleep assertion](https://developer.apple.com/documentation/iokit/kiopmassertiontypepreventuseridlesystemsleep)
- Recommend independent capture-health and persistence-health observations. A live process is insufficient evidence that frames arrive or reach disk, given routing changes and storage errors. — [Apple Audio MIDI Setup](https://support.apple.com/en-euro/guide/audio-midi-setup/ams0c519de39/mac); [Apple write manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/write.2)
- Recommend bounded queues and an explicit overflow event rather than silently counting discarded samples as captured. With unbounded outage duration and finite memory, no buffer can guarantee retaining all audio indefinitely. This is a capacity deduction, not a library guarantee. — [Apple write manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/write.2)

### Gaps
- User decisions are needed on recording through idle sleep, retention/disk exhaustion, acceptable recovery gaps, and mic selection/fallback.
- No supported macOS CLI permissions/supervision combination has been exercised on this host. No uninterrupted-recording result is claimed.

## What should supervise and relaunch the recorder on macOS?

### Takeaway
launchd is Apple's native supervisor; KeepAlive requests continuous operation, but repeated early exits are throttled. A per-user LaunchAgent also ends on logout.

### Cited Findings
- Apple's current Terminal support guide directs background script management to launchd. — [Apple Terminal launchd guide](https://support.apple.com/en-mide/guide/terminal/apdc6c1077b-5d5d-4d35-9c19-60f2397b2369/mac)
- Apple's archived launchd guide explains that user agents are loaded for the logged-in user and receive SIGTERM at logout; system jobs receive SIGTERM at shutdown. It documents KeepAlive as requesting continuous operation. — [Apple creating launchd jobs](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
- Apple's open-source launchd.plist manual states that KeepAlive=true keeps a job alive and quickly repeated exits are throttled. Its documented default ThrottleInterval is 10 seconds. This source is older than current macOS; verify the installed manual before relying on that numeric default. — [Apple launchd.plist manual source](https://github.com/apple-oss-distributions/launchd/blob/main/man/launchd.plist.5)

### Inferences
- Recommend a foreground recorder supervised by a user LaunchAgent after microphone authorization is verified in that execution context. This uses the native supervisor without introducing another executable dependency; user-session availability still bounds capture. — [Apple creating launchd jobs](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
- Recommend retrying transient device faults inside the process with bounded backoff, while allowing unrecoverable process faults to be relaunched. Avoid relying on rapid crash-and-restart loops for uninterrupted audio because launchd throttles them. — [Apple launchd.plist manual source](https://github.com/apple-oss-distributions/launchd/blob/main/man/launchd.plist.5)

### Gaps
- Current installed launchd manual, launchctl command syntax, agent installation, and agent permissions have not been verified here; no service should be installed based on this note alone.
- User-login coverage versus pre-login/system-daemon coverage requires a user choice and verified capture permissions.

## Does streaming to .opus make crashes harmless?

### Takeaway
Ogg Opus supports truncated streams, which makes it suitable for incremental recording; it does not make unwritten data or corrupted tail pages recoverable. Container flush, application buffers, and storage durability are separate boundaries.

### Cited Findings
- RFC 7845 recommends .opus for Ogg Opus and specifies that implementations must handle truncated logical streams lacking the end-of-stream flag. Audio packets can cross page boundaries. — [RFC 7845 sections 3 and 9](https://www.rfc-editor.org/rfc/rfc7845.html)
- Ogg pages have sequence numbers and CRC checksums that allow page loss/corruption detection. — [RFC 3533 section 6](https://www.rfc-editor.org/rfc/rfc3533.html)
- Go File.Sync commits file contents to stable storage. The Go 1.26.2 Darwin backend uses F_FULLFSYNC because ordinary Darwin fsync does not fully flush to disk; on ENOTSUP, such as SMB mounts, it falls back to ordinary fsync. — [Go os.File.Sync](https://pkg.go.dev/os#File.Sync); [Go 1.26.2 Darwin sync implementation](https://raw.githubusercontent.com/golang/go/go1.26.2/src/internal/poll/fd_fsync_darwin.go)
- Apple's Darwin fsync manual states that ordinary fsync may leave reordered drive-cache data vulnerable to OS crash or power loss and recommends F_FULLFSYNC for tighter guarantees. — [Apple fsync manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/fsync.2)
- Apple's F_FULLFSYNC documentation says it performs fsync and asks the drive to flush buffered data, draining the device queue. — [Apple fcntl manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/fcntl.2)
- File-data sync does not necessarily persist the containing directory entry on Linux; a directory fsync is separately needed. SQLite's Unix VFS also separates file-data synchronization from creation/deletion directory synchronization and documents filesystem-dependent behavior. — [Linux man-pages fsync](https://www.man7.org/linux/man-pages/man2/fsync.2.html); [SQLite Unix VFS](https://raw.githubusercontent.com/sqlite/sqlite/master/src/os_unix.c)

### Inferences
- Recommend leaving existing crash-interrupted recordings untouched and creating the next numbered file, preserving earlier complete pages. Decoder behavior on deliberately truncated real output must be validated with the selected muxer and decoder; RFC support alone is insufficient evidence of a successful implementation. — [RFC 7845 section 3](https://www.rfc-editor.org/rfc/rfc7845.html)
- Recommend an explicit page-flush and sync policy off the audio callback. The vulnerable duration includes pending capture frames, pending encoded packets, pending Ogg pages, and unsynchronized writes. A sync timer alone is not a hard maximum because writes/syncs can stall or fail. — [Apple fsync manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/fsync.2); [PortAudio callback guidance](https://portaudio.com/docs/v19-doxydocs/writing_a_callback.html)
- Recommend synchronizing creation metadata separately where supported, including newly created YYYY/MM/DD directory levels. A successful audio-file Sync should not be described as a portable power-loss guarantee for the pathname. — [Linux man-pages fsync](https://www.man7.org/linux/man-pages/man2/fsync.2.html); [SQLite Unix VFS](https://raw.githubusercontent.com/sqlite/sqlite/master/src/os_unix.c)

### Gaps
- APFS-specific containing-directory crash guarantees have not been established by a current Apple contract in this research. Directory sync behavior and errors need direct host validation; Linux guarantees must not be presented as macOS guarantees.
- Actual page batching, tail loss, decoder recovery, and any independent-hour boundary artifacts depend on the selected Opus/Ogg libraries and remain untested.
- No claim of zero data loss, successful crash recovery, or power-loss recovery is justified yet.

## How can rotation and diagnostics be bootstrapped without unapproved dependencies?

### Takeaway
The standard library provides exclusive file creation and line-delimited JSON diagnostics. A storage bootstrap can be tested independently, while recording requires approved capture/codec choices.

### Cited Findings
- Go OpenFile supports O_CREATE|O_EXCL, which requires a previously absent filename; Darwin's open manual states that an existing file causes an error and an existing final symlink also fails. O_TRUNC instead truncates existing files. — [Go os flags](https://pkg.go.dev/os); [Apple open manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/open.2)
- Go slog.JSONHandler writes records as line-delimited JSON objects and makes a single serialized Writer.Write call per handled record. — [Go slog.JSONHandler](https://pkg.go.dev/log/slog#JSONHandler)
- The official Go downloads page currently lists Go 1.27.1 as latest stable and Go 1.26.8 as the previous release-line patch. These versions were checked on 2026-09-30; no toolchain download or installation is authorized by this research. — [Go downloads](https://go.dev/dl/)

### Inferences
- Candidate allocation behavior: inspect only the current YYYY/MM/DD hour prefix to identify its largest existing numeric suffix, attempt the next name with O_CREATE|O_EXCL, and retry on an existence collision. Never overwrite a crash-interrupted recording; scanning without exclusive creation leaves a race. Repeated local hours due to daylight-saving or clock changes naturally need additional suffixes too. The exact clock/timezone policy needs a user decision. — [Go os flags](https://pkg.go.dev/os); [Apple open manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/open.2)
- Recommend minimum-width three-digit suffixes, rather than a counter that wraps after 999. Counter-overflow behavior still needs to be stated. — [Go os flags](https://pkg.go.dev/os)
- Candidate diagnostics include session identity, wall-clock time plus elapsed/frame positions, selected device and format, file-open/rotate/close, retry reason/count, received/encoded/written frames, queue overflow, write/sync errors, and graceful exit. Keep logs off the capture callback and emit storage failures to another already available sink such as stderr because the same disk can also reject log writes. These fields are recommendations for diagnosing the verified failure classes, not a mandated schema. — [Go slog.JSONHandler](https://pkg.go.dev/log/slog#JSONHandler); [Apple write manual source](https://raw.githubusercontent.com/apple-oss-distributions/xnu/main/bsd/man/man2/write.2); [PortAudio callback guidance](https://portaudio.com/docs/v19-doxydocs/writing_a_callback.html)
- Candidate red/green storage checks: concurrent exclusive allocation without overwritten bytes; restart with existing suffixes; hour/day boundaries; write and sync error propagation; append of independently parseable JSON records; and behavior after abrupt writer termination leaving a partial last log line. Use actual filesystem behavior where possible. These are a review plan, not implemented or passed tests. — [Go os flags](https://pkg.go.dev/os); [Go slog.JSONHandler](https://pkg.go.dev/log/slog#JSONHandler)

### Gaps
- No dependency-free storage implementation was inspected or tested by this researcher. The coordinator must attach real execution logs before claiming bootstrap validation.
- Exact module path, target Go minimum, timezone, output root, and suffix policy remain user/project choices.
- A standard-library-only executable cannot be claimed to record Opus merely because the path allocator creates an .opus filename.

## What is supportable about low CPU and storage cost?

### Takeaway
Low CPU is an optimization and measurement objective, not a guaranteed number. Callback isolation and Opus complexity controls offer evidence-based ways to reduce avoidable work.

### Cited Findings
- PortAudio callback guidance forbids unpredictable operations such as allocations, file/console I/O, and mutex acquisition in the audio callback. Its API overview also provides blocking read/write as an alternative. — [PortAudio callback guidance](https://portaudio.com/docs/v19-doxydocs/writing_a_callback.html); [PortAudio API overview](https://portaudio.com/docs/v19-doxydocs/api_overview.html)
- miniaudio explicitly prohibits device initialization/uninitialization/start/stop from inside its callback because these can deadlock; device recovery belongs on another thread. — [miniaudio manual](https://miniaud.io/docs/manual/index.html)
- Opus provides computational complexity settings from 0 through 10, with 10 highest, and an explicit bits-per-second bitrate control. — [Opus 1.6 encoder controls](https://www.opus-codec.org/docs/opus_api-1.6/group__opus__encoderctls.html)
- Apple explains that explicit filesystem synchronization can degrade performance and increase device wear; this guidance is iOS-specific, so it supports avoiding unnecessary flushing but supplies no macOS benchmark number. — [Apple reducing disk writes](https://developer.apple.com/documentation/xcode/reducing-disk-writes)

### Inferences
- Recommend a preallocated capture buffer feeding an encoder/storage worker, no filesystem/logging/device lifecycle work in the callback, and reuse of encoder state between frames. This is a candidate implementation direction requiring the selected capture library's threading contract to be checked. — [PortAudio callback guidance](https://portaudio.com/docs/v19-doxydocs/writing_a_callback.html); [miniaudio manual](https://miniaud.io/docs/manual/index.html)
- Raw capacity calculations, excluding container/log overhead: at 24,000 bits/s, 24 hours is 259,200,000 bytes (259.2 MB decimal); at 32,000 bits/s it is 345,600,000 bytes (345.6 MB). These are arithmetic for fixed average payload bitrate, not guaranteed variable-bitrate file sizes. — [Opus bitrate control](https://www.opus-codec.org/docs/opus_api-1.6/group__opus__encoderctls.html)
- Buffer arithmetic: 48,000 samples/s × one channel × two bytes/sample is 96,000 bytes/s, so 10 seconds requires 960,000 bytes excluding queue metadata. This is capacity arithmetic, not an approved capture format or latency setting. — [PortAudio API overview](https://portaudio.com/docs/v19-doxydocs/api_overview.html)
- Recommend benchmarking wall-clock capture duration, process CPU time, resident memory, allocation rate, missed frames, recovery gap length, and write/sync latency on each supported host with actual microphones and candidate Opus complexity settings. Source controls establish that tuning exists, not which setting meets the user's transcription-quality requirements. — [Opus complexity control](https://www.opus-codec.org/docs/opus_api-1.6/group__opus__encoderctls.html)

### Gaps
- No credible primary-source CPU percentage applies to this unimplemented Go pipeline on the user's host. No low-CPU numeric promise is made.
- Actual meeting transcription quality, acceptable bitrate, codec complexity, long-duration memory stability, battery effects, and silence handling need end-to-end measurements.

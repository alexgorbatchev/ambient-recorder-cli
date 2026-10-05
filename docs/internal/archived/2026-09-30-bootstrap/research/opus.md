---
created_on: 2026-09-30 10:43
last_modified: 2026-10-05 10:18
status: archived
---

# Opus encoding, Ogg muxing, and crash durability

Archived September 30, 2026 research for maintainers. Version comparisons, approval proposals, indexed naming recommendations and open questions are historical findings, not current requirements. Read the [repository instructions](../../../../../AGENTS.md) and [contributor instructions](../../../contributing.md) for current decisions.

Research for the coordinator choosing an always-on microphone recorder's codec dependencies. Source inspection and GitHub API metrics are from 2026-09-30; no candidate libraries are installed or benchmarked locally.

## Does streaming an .opus file make crashes harmless?

### Takeaway

Streaming Ogg Opus preserves an independently decodable prefix when complete pages survive, but does not guarantee zero audio loss. The container supports truncated streams; outstanding PCM, encoder state, incomplete pages, library buffering, and storage writes remain separate loss windows. [RFC 7845](https://www.rfc-editor.org/rfc/rfc7845.html), [RFC 3533](https://www.rfc-editor.org/rfc/rfc3533.html), [Go File.Sync](https://pkg.go.dev/os#File.Sync)

### Cited Findings

- RFC 7845 section 3 requires implementations to handle truncated streams without an end-of-stream page; incomplete continued packets are not usable as complete audio packets. [RFC 7845](https://www.rfc-editor.org/rfc/rfc7845.html#section-3)
- Ogg pages carry a sequence number and CRC over the header and payload, using polynomial 0x04c11db7. The lacing table gives the page size, so a recovery scanner can identify intact page boundaries. [RFC 3533](https://www.rfc-editor.org/rfc/rfc3533.html#section-6)
- Ogg Opus granule positions and pre-skip use a 48 kHz clock, regardless of input rate. The OpusHead input rate is informational. PCM position equals granule position minus pre-skip. Exact final duration requires encoder-delay compensation and end trimming; section 7 describes padding and final granule accounting. The 3840-sample recommendation concerns decoder convergence for cropping and seeking, rather than a universal fresh-encoder delay. [RFC 7845](https://www.rfc-editor.org/rfc/rfc7845.html#section-4), [encoder guidance](https://www.rfc-editor.org/rfc/rfc7845.html#section-7)
- Go documents File.Sync as committing current file contents to stable storage. A successful Write alone is not that operation. [Go File.Sync](https://pkg.go.dev/os#File.Sync)
- libopusenc buffers raw audio for decisions and compressed packets for pages; its defaults are two seconds of decision delay and one second of muxing delay. [libopusenc source](https://github.com/xiph/libopusenc/blob/master/src/opusenc.c)

### Inferences

- Use a bounded, explicit page flush cadence and separate periodic file synchronization. Log their duration and errors. A process crash and machine/power failure must have separate acceptance tests because they lose different buffers. These recommendations follow from page integrity and explicit synchronization semantics. [RFC 3533](https://www.rfc-editor.org/rfc/rfc3533.html#section-6), [Go File.Sync](https://pkg.go.dev/os#File.Sync)
- On restart, scan the previous segment through the last valid complete CRC page/packet and report a truncated tail. Begin a new numbered file rather than appending with a new encoder to the previous logical stream: codec prediction state, serial number, page sequence, pre-skip, and granule position all belong to that stream. Recovery/finalization must be a real validated path, not a claim that an arbitrary truncated byte sequence is universally playable. [RFC 7845](https://www.rfc-editor.org/rfc/rfc7845.html), [RFC 3533](https://www.rfc-editor.org/rfc/rfc3533.html)

### Gaps

- No local decoder interoperability, SIGKILL, disk-full, torn-write, or hardware/power-loss experiments are run. Exact retained-audio bounds and specific downstream transcription compatibility require those experiments after dependencies are approved.
- File.Sync documentation alone does not establish the actual power-loss guarantees of the selected filesystem, storage hardware, or macOS full-sync policy.

## Which Go codec and mux libraries are real current candidates?

### Takeaway

Current pure-Go encoding exists, including in Pion; an older decoder-only assessment is incorrect. The inspected Go candidates still trade speech codec completeness, exact file finalization, maturity, and bounded crash buffering. [Pion release](https://github.com/pion/opus/releases/tag/v0.1.0), [thesyncim/gopus](https://github.com/thesyncim/gopus), [tphakala/go-opus](https://github.com/tphakala/go-opus)

### Cited Findings

| Candidate | Verified fit and limitation | Adoption/activity snapshot |
| --- | --- | --- |
| Pion opus | Pure Go. Current encoder supports 48 kHz 20 ms CELT packets and a SILK-only path. Application selection does not itself apply libopus's encoding heuristics/defaults. Its v0.1.0 release includes the new CELT encoder. [API source](https://github.com/pion/opus/blob/main/encoder.go), [release](https://github.com/pion/opus/releases/tag/v0.1.0) | 562 stars, repository created 2022-06-12, pushed 2026-09-30. [API](https://api.github.com/repos/pion/opus) |
| hraban/opus | Established cgo wrapper for libopus; exposes SetComplexity, bitrate, VBR, DTX and FEC. README explicitly excludes Ogg file creation and self-contained binaries in its ordinary configuration. Source uses pkg-config opus; the package also supports libopusfile decoding. [encoder](https://github.com/hraban/opus/blob/master/encoder.go), [README](https://github.com/hraban/opus#readme) | 355 stars, created 2015-02-18, pushed 2026-07-08. [API](https://api.github.com/repos/hraban/opus) |
| layeh/gopus | cgo binding bundles old libopus 1.1.2 for amd64/386; other architecture builds select a shared-library implementation. Its packet API allocates output and does not expose the needed complexity/lookahead controls in the inspected nonshared encoder. [nonshared](https://github.com/layeh/gopus/blob/master/opus_nonshared.go), [shared](https://github.com/layeh/gopus/blob/master/opus_shared.go) | 49 stars, created 2014-11-04, pushed 2021-05-01. [API](https://api.github.com/repos/layeh/gopus) |
| thesyncim/gopus | Pure Go, all SILK/CELT/hybrid modes, native 8/12/16/24/48 kHz input and reusable encode buffers. Go 1.25, no required module dependencies. Current release v0.1.1; v0.1.0 retracted. [README](https://github.com/thesyncim/gopus#readme), [module](https://github.com/thesyncim/gopus/blob/master/go.mod) | 26 stars, created 2026-01-22, pushed 2026-09-30. [API](https://api.github.com/repos/thesyncim/gopus) |
| tphakala/go-opus | Pure Go and complete CELT-only encoder intentionally; no SILK or hybrid encode roadmap. Full decoding. Depends on cgo-free tphakala/simd and indirect x/sys; current main module requires Go 1.27. [README](https://github.com/tphakala/go-opus#readme), [module](https://github.com/tphakala/go-opus/blob/main/go.mod) | 3 stars, created 2026-07-16, pushed 2026-09-17. [API](https://api.github.com/repos/tphakala/go-opus) |

- thesyncim/gopus's encoder controls include SetComplexity(0..10), SetBitrate, VBR, FEC, DTX, and Lookahead. Its container/ogg Writer accepts explicit PreSkip, serializes each packet into its own page immediately, copies packet bytes into its page buffer before Write returns, uses Ogg CRC, detects short writes, and updates cumulative 48 kHz samples. Close only emits a packetless EOS page at the current cumulative granule position. There is no final-sample trimming/drain API in that writer. The v0.1.1 tagged writer has the same Close behavior. [controls](https://github.com/thesyncim/gopus/blob/master/encoder_controls.go), [writer](https://github.com/thesyncim/gopus/blob/v0.1.1/container/ogg/writer.go), [CRC](https://github.com/thesyncim/gopus/blob/master/container/ogg/crc.go)
- thesyncim's separate streaming Writer is packet-oriented, not an integrated Ogg writer: its PacketSink signature is WritePacket([]byte)(int,error), whereas ogg.Writer needs WritePacket([]byte,int)error. Streaming Close pads only a partial frame before closing its sink; it does not implement encoder lookahead drain or exact Ogg end trim. [stream source](https://github.com/thesyncim/gopus/blob/master/stream.go)
- thesyncim's project-reported benchmark is about 94 microseconds per caller-buffer encode on an AMD EPYC runner, zero allocations, with tested encode modes about 1.4–1.7 times libopus duration. These are upstream claims, not this recorder's target-machine measurements. [performance](https://github.com/thesyncim/gopus#performance)
- tphakala's oggopus.Encoder streams PCM, writes actual codec pre-skip, pads partial frames, encodes enough padding to cover delay, and trims the final EOS granule to exact source duration. Its container copies reusable packet bytes and holds one packet plus one pending page; pages commit when their 255-entry lacing table fills or on Close. No page-duration or flush control is exposed in the inspected encoder/writer. [encoder](https://github.com/tphakala/go-opus/blob/main/oggopus/encoder.go), [writer](https://github.com/tphakala/go-opus/blob/main/oggopus/writer.go)
- Pion webrtc/pkg/media/oggwriter is an RTP-packet writer. Current source derives packet sample counts from the Opus TOC, emits CRC pages and supports EOS finalization; however pre-skip is hardcoded to 3840 with no public override in inspected options, and there is no exact source-length end-trim API. Single-track NewWith does not perform the same EOS page rewrite as filename-based New. [source](https://github.com/pion/webrtc/blob/master/pkg/media/oggwriter/oggwriter.go)

### Inferences

- Prefer thesyncim over the other inspected pure-Go encoders for a speech-focused evaluation, but do not describe its Ogg writing as an exact-duration complete recorder integration. Encoder-delay drain and final trim need a validated upstream extension or a different approved writer. Otherwise normal hourly rotation can lose the codec tail. [tagged writer](https://github.com/thesyncim/gopus/blob/v0.1.1/container/ogg/writer.go), [RFC guidance](https://www.rfc-editor.org/rfc/rfc7845.html#section-7)
- tphakala offers the most complete inspected integrated pure-Go stream-finalization API, but low-rate speech is deliberately outside its optimal mode selection, and buffering can hold roughly 5.1 seconds at 20 ms per one-lacing-segment packet (255 times 20 ms), plus the held packet. It is not the first choice for the requested bounded-loss, low-CPU speech archive. [README](https://github.com/tphakala/go-opus#readme), [writer](https://github.com/tphakala/go-opus/blob/main/oggopus/writer.go)
- Pion's hardcoded 3840 pre-skip must not be accepted as a fresh encoder's actual delay merely because the value appears in RFC recommendations. [writer](https://github.com/pion/webrtc/blob/master/pkg/media/oggwriter/oggwriter.go), [RFC 7845](https://www.rfc-editor.org/rfc/rfc7845.html#section-4.2)

### Gaps

- No independent local correctness or performance measurements of these Go implementations are made. Repository tests and README claims are supporting evidence only, not recorder-level validation.
- Stars and push timestamps measure visibility and activity; they do not demonstrate production reliability or issue turnaround. No reliable recorder-scale adoption data is found for either newer complete Go port.

## Which native dependencies should be proposed for approval?

### Takeaway

Recommend Xiph libopus 1.6.1 plus libopusenc 0.3, linked as static libraries into the Go executable through a narrow cgo binding to the intended complete C API. This delegates encoding, Ogg page construction, delay handling, final trimming, and seamless file continuation to upstream implementations; no FFmpeg subprocess, external libogg, or installed third-party codec runtime is required by that design. [downloads](https://opus-codec.org/downloads/), [libopusenc dependencies](https://github.com/xiph/libopusenc#readme), [API](https://www.opus-codec.org/docs/libopusenc_api-0.3/group__encoding.html)

### Cited Findings

- libopus 1.6.1 is released 2026-01-14; its source SHA256 is 6ffcb593207be92584df15b32466ed64bbec99109f007c82205f0194572411a1. libopusenc 0.3 is released 2026-01-03, including drain assertion and uninitialized-field fixes; source SHA256 is f616d3aff9b2034547894ccb8ab56c36cf1a4acb0d922c5d7119f97bbe58642c. The official site categorizes libopusenc under development releases, so report that status explicitly. [downloads](https://opus-codec.org/downloads/)
- Both top-level COPYING files contain BSD three-clause conditions and require license notices to accompany binary distribution. libopus also lists royalty-free patent license references. [libopus license](https://github.com/xiph/opus/blob/v1.6.1/COPYING), [libopusenc license](https://github.com/xiph/libopusenc/blob/master/COPYING)
- libopusenc depends only on libopus. Its Makefile compiles its own Ogg packer and bundled resample.c/Speex resampler headers; configure checks opus >=1.1. [README](https://github.com/xiph/libopusenc#readme), [Makefile](https://github.com/xiph/libopusenc/blob/master/Makefile.am), [configure](https://github.com/xiph/libopusenc/blob/master/configure.ac)
- The complete public API includes PCM write/write_float, create_callbacks with page-write and close callbacks, create_pull with get_page(flush=1), flush_header, drain, destroy, and continue_new_callbacks for switching independent output files while retaining the same encoder. Page callbacks return explicit success/failure. The API documents 48 kHz input as faster. [API](https://www.opus-codec.org/docs/libopusenc_api-0.3/group__encoding.html), [header](https://github.com/xiph/libopusenc/blob/master/include/opusenc.h)
- Initialization gets actual OPUS_GET_LOOKAHEAD pre-skip. End granule converts original input sample count to 48 kHz and adds the codec offset. Drain extends/pads input, disables decision delay, and marks the final packet EOS with the exact end granule; continuing files also provides overlap/history handling. [implementation](https://github.com/xiph/libopusenc/blob/master/src/opusenc.c)
- libopusenc exposes OPE_SET_DECISION_DELAY and OPE_SET_MUXING_DELAY. Its packer treats mux delay as granule units; positive delay causes bounded-duration automatic page flushing, while ZERO disables that automatic time-based flushing. Pull API get_page(...,flush=1) explicitly flushes pending pages. [header](https://github.com/xiph/libopusenc/blob/master/include/opusenc.h), [packer](https://github.com/xiph/libopusenc/blob/master/src/ogg_packer.c)
- OPUS_SET_COMPLEXITY, BITRATE, SIGNAL, VBR, FEC and DTX controls pass through libopusenc to the codec. Complexity ranges from zero to ten. [implementation](https://github.com/xiph/libopusenc/blob/master/src/opusenc.c), [official controls](https://opus-codec.org/docs/opus_api-1.6/group__opus__encoderctls.html)
- libopusenc's inspected build is Autotools/libtool, not CMake. Its build checks pkg-config macros and a libopus development installation; building source release archives using their generated configure avoids a need to regenerate Autotools files. [configure](https://github.com/xiph/libopusenc/blob/master/configure.ac), [release source](https://downloads.xiph.org/releases/opus/libopusenc-0.3.tar.gz)

### Inferences

- Dependency approval proposal: libopus 1.6.1 and libopusenc 0.3 source archives, their retained notices, and cgo/native build tooling. Use static archives to keep codec code inside the release binary; verify resulting linkage on every supported architecture before claiming no third-party runtime dependency. The ordinary hraban wrapper does not by itself deliver this result. [libopusenc build](https://github.com/xiph/libopusenc/blob/master/Makefile.am), [hraban README](https://github.com/hraban/opus#readme)
- Initial profile to benchmark: 48 kHz mono, 20 ms packets, voice-oriented application, 24–32 kbps VBR, complexity 2–4, DTX and network FEC disabled. Set decision delay to zero and positive mux delay 4800 for a 100 ms Ogg-page target. These are proposed tradeoffs for approval/testing, not measured quality or CPU claims. Existing controls support the proposal; the packer explicitly enforces positive granule delay. [codec controls](https://opus-codec.org/docs/opus_api-1.6/group__opus__encoderctls.html), [packer](https://github.com/xiph/libopusenc/blob/master/src/ogg_packer.c)
- Prefer callback or pull output over create_file so Go can own exclusive file creation, sequential naming, synchronization, write diagnostics, and repair bookkeeping without delegating those policies to C stdio. This uses the library's intended output modes, without writing a substitute Ogg muxer. [encoding API](https://www.opus-codec.org/docs/libopusenc_api-0.3/group__encoding.html)

### Gaps

- No current mature Go libopusenc binding is identified. A narrow binding to native lifecycle/output/control functions is necessary if this proposal is approved; cgo contracts and C pointer lifetimes require implementation review and real audio tests.
- Static compilation and release-tool requirements are not exercised because the user has not approved installing/importing dependencies. Compiler/make/pkg-config availability and cross-platform linkage remain bootstrap validation work.
- Native encoder controls do not prove a target CPU budget; actual callback, codec, page-write and sync overhead must be measured on the user's machine.

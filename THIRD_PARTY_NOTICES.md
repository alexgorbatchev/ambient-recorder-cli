The executable includes the following components. Preserve this file and the linked license texts with binary distributions.

| Component | Version | License text |
| :--- | :--- | :--- |
| Go runtime | 1.27.1 | [BSD](docs/licenses/go-1.27.1.txt) |
| libopus | 1.6.1 | [BSD and patent notices](docs/licenses/opus-1.6.1.txt) |
| libopusenc | 0.3 | [BSD](docs/licenses/libopusenc-0.3.txt) |
| Bundled Speex resampler | libopusenc 0.3 | [BSD](docs/licenses/speex-resampler.txt) |
| cobra-help-tree/v2 | 2.1.0 | [MIT](docs/licenses/cobra-help-tree-2.1.0.txt) |
| Cobra | 1.10.2 | [Apache 2.0](docs/licenses/cobra-1.10.2.txt) |
| go-toml/v2 | 2.4.3 | [MIT](docs/licenses/go-toml-2.4.3.txt) |
| tint | 1.2.0 | [MIT](docs/licenses/tint-1.2.0.txt) |
| pflag | 1.0.9 | [BSD](docs/licenses/pflag-1.0.9.txt) |
| go-runewidth | 0.0.30 | [MIT](docs/licenses/go-runewidth-0.0.30.txt) |
| uax29/v2 | 2.2.0 | [MIT](docs/licenses/uax29-2.2.0.txt) |
| golang.org/x/sys | 0.47.0 | [BSD](docs/licenses/x-sys-0.47.0.txt) |
| golang.org/x/term | 0.45.0 | [BSD](docs/licenses/x-term-0.45.0.txt) |

Additional libopusenc source notices cover its [Ogg packer](docs/licenses/opusenc-ogg-packer.txt), [header](docs/licenses/opusenc-header.txt), [picture](docs/licenses/opusenc-picture.txt), and [Unicode support](docs/licenses/opusenc-unicode.txt).

Go dependency rationale is recorded in [the dependency proposal](research_notes/Continuous%20microphone%20Opus%20recording/cli.md). The current module versions and checksums are declared in [go.mod](go.mod) and [go.sum](go.sum). libopusenc 0.3 is listed by its publisher as a development release.

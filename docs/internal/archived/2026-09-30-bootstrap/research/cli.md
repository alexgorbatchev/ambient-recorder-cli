---
created_on: 2026-09-30 17:51
last_modified: 2026-10-05 10:18
status: archived
---

# CLI dependency approval research

Archived September 30, 2026 research for maintainers. Version comparisons, approval proposals, indexed naming recommendations and open questions are historical findings, not current requirements. Read the [repository instructions](../../../../../AGENTS.md) and [contributor instructions](../../../contributing.md) for current decisions.

These notes give the coordinator a concrete versioned proposal. They do not record user approval, installation, or a verified build. All versions are checked from primary sources on 2026-09-30.

## Which direct libraries and toolchain are proposed?

### Takeaway
The verified release proposal is Cobra v1.10.2 and cobra-help-tree/v2 v2.1.0, with Go 1.27.1 as the current stable toolchain. No library or toolchain is installed by this research.

### Cited Findings
- GitHub marks Cobra v1.10.2 latest. The import is `github.com/spf13/cobra`; license is Apache-2.0. Cobra's README names Kubernetes, Hugo, and GitHub CLI as adopters, which is the maintainer's adoption claim rather than an independent audit. — [Cobra release](https://github.com/spf13/cobra/releases/tag/v1.10.2); [tagged README](https://raw.githubusercontent.com/spf13/cobra/v1.10.2/README.md); [license](https://raw.githubusercontent.com/spf13/cobra/main/LICENSE.txt)
- GitHub marks cobra-help-tree v2.1.0 latest. The exact import is `github.com/alexgorbatchev/cobra-help-tree/v2`, package name `cobrahelptree`, license MIT, minimum Go version 1.26.2. — [help-tree release](https://github.com/alexgorbatchev/cobra-help-tree/releases/tag/v2.1.0); [tagged module](https://raw.githubusercontent.com/alexgorbatchev/cobra-help-tree/v2.1.0/go.mod); [package declaration](https://raw.githubusercontent.com/alexgorbatchev/cobra-help-tree/v2.1.0/doc.go); [license](https://raw.githubusercontent.com/alexgorbatchev/cobra-help-tree/v2.1.0/LICENSE)
- `Setup(cmd *cobra.Command) error` delegates to `SetupWithOptions` and installs both inherited help and usage functions. Requested help goes to stdout; diagnostic usage goes to stderr. Optional `HelpOptions.Catalog` supports arguments, environment variables, examples, and metadata. — [tagged setup implementation](https://raw.githubusercontent.com/alexgorbatchev/cobra-help-tree/v2.1.0/setup.go); [tagged integration README](https://raw.githubusercontent.com/alexgorbatchev/cobra-help-tree/v2.1.0/README.md)
- Help-tree renderers are not safe to call concurrently on command trees sharing a root. The library documents single-goroutine Cobra execution through Setup as unaffected. — [tagged package documentation](https://raw.githubusercontent.com/alexgorbatchev/cobra-help-tree/v2.1.0/doc.go)
- Go's official downloads page lists 1.27.1 as latest stable and 1.26.8 as the other supported release-line patch. — [Go downloads](https://go.dev/dl/)

### Inferences
- Recommend pinning those exact direct versions and requesting approval for Go 1.27.1 download/use, rather than requesting an open-ended `@latest` installation. These are proposals grounded in the verified release versions. — [Cobra release](https://github.com/spf13/cobra/releases/tag/v1.10.2); [help-tree release](https://github.com/alexgorbatchev/cobra-help-tree/releases/tag/v2.1.0); [Go downloads](https://go.dev/dl/)
- Integration should use the library's complete help/usage setup and describe actual commands through its catalog, rather than implement a parallel help renderer. — [tagged setup implementation](https://raw.githubusercontent.com/alexgorbatchev/cobra-help-tree/v2.1.0/setup.go)

### Gaps
- No independent help-tree adoption count or broad adopter list was verified. Its API is documented and a tagged release exists, but no popularity/maturity claim is warranted.
- No local dependency resolution, build, or smoke test is performed. Toolchain compatibility is not established by a successful build yet.

## What transitive modules belong in the concrete approval proposal?

### Takeaway
The help-tree module declares six additional modules beyond Cobra and itself. Cobra also declares documentation-generation dependencies; these belong in the disclosed module graph, although importing Cobra's core package does not automatically compile its separate `doc` package.

### Cited Findings

The tagged help-tree go.mod gives the first six transitive versions below. Runewidth and x/term tagged manifests independently confirm their UAX29 and x/sys dependencies. — [help-tree manifest](https://raw.githubusercontent.com/alexgorbatchev/cobra-help-tree/v2.1.0/go.mod); [runewidth manifest](https://raw.githubusercontent.com/mattn/go-runewidth/v0.0.30/go.mod); [x/term manifest](https://raw.githubusercontent.com/golang/term/v0.45.0/go.mod)

| Module | Version proposed | License evidence |
| --- | --- | --- |
| `github.com/spf13/pflag` | v1.0.9 | [BSD-3-Clause](https://raw.githubusercontent.com/spf13/pflag/v1.0.9/LICENSE) |
| `github.com/mattn/go-runewidth` | v0.0.30 | [MIT](https://raw.githubusercontent.com/mattn/go-runewidth/v0.0.30/LICENSE) |
| `golang.org/x/term` | v0.45.0 | [BSD-3-Clause](https://raw.githubusercontent.com/golang/term/v0.45.0/LICENSE) |
| `github.com/clipperhouse/uax29/v2` | v2.2.0 | [MIT](https://raw.githubusercontent.com/clipperhouse/uax29/v2.2.0/LICENSE) |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | [Apache-2.0](https://raw.githubusercontent.com/inconshreveable/mousetrap/v1.1.0/LICENSE) |
| `golang.org/x/sys` | v0.47.0 | [BSD-3-Clause](https://raw.githubusercontent.com/golang/sys/v0.47.0/LICENSE) |

- Mousetrap's import is in Cobra's Windows-only command implementation, so it is not a macOS recorder runtime package. — [tagged Windows implementation](https://raw.githubusercontent.com/spf13/cobra/v1.10.2/command_win.go)
- Cobra's tagged go.mod additionally declares `github.com/cpuguy83/go-md2man/v2 v2.0.6` and `go.yaml.in/yaml/v3 v3.0.4`. Cobra's separate `doc` package imports their implementations for man-page and YAML generation. — [Cobra manifest](https://raw.githubusercontent.com/spf13/cobra/v1.10.2/go.mod); [man generator](https://raw.githubusercontent.com/spf13/cobra/v1.10.2/doc/man_docs.go); [YAML generator](https://raw.githubusercontent.com/spf13/cobra/v1.10.2/doc/yaml_docs.go)
- go-md2man v2.0.6 requires Blackfriday v2.1.0; YAML v3.0.4 requires the legacy check.v1 revision below. — [md2man manifest](https://raw.githubusercontent.com/cpuguy83/go-md2man/v2.0.6/go.mod); [YAML manifest](https://raw.githubusercontent.com/yaml/go-yaml/v3.0.4/go.mod)

| Additional declared/documentation module | Version proposed if resolution needs it | License evidence |
| --- | --- | --- |
| `github.com/cpuguy83/go-md2man/v2` | v2.0.6 | [MIT](https://raw.githubusercontent.com/cpuguy83/go-md2man/v2.0.6/LICENSE.md) |
| `go.yaml.in/yaml/v3` | v3.0.4 | [MIT and Apache-2.0 by source file](https://raw.githubusercontent.com/yaml/go-yaml/v3.0.4/LICENSE) |
| `github.com/russross/blackfriday/v2` | v2.1.0 | [BSD-2-Clause](https://raw.githubusercontent.com/russross/blackfriday/v2.1.0/LICENSE.txt) |
| `gopkg.in/check.v1` | v0.0.0-20161208181325-20d25e280405 | [BSD-2-Clause](https://raw.githubusercontent.com/go-check/check/20d25e280405/LICENSE) |

- Go distinguishes modules from compiled packages and applies module graph pruning/lazy loading according to manifests. A manually read declared graph is not proof of the final download list or linked binary contents. — [Go modules reference](https://go.dev/ref/mod)

### Inferences
- Concrete proposed approval set: the two direct libraries, six help-tree transitive modules, and four disclosed Cobra documentation/test graph modules at the versions above, plus Go 1.27.1. The four latter modules may only be read as metadata or omitted by the final import graph; do not import documentation generators solely to force them into the binary. — [Go modules reference](https://go.dev/ref/mod); [Cobra manifest](https://raw.githubusercontent.com/spf13/cobra/v1.10.2/go.mod)
- After user approval, capture the actual resolved `go.mod`, `go.sum`, module list, package dependency list, and linked build information. If an additional unapproved module appears, pause its installation and explain the graph discrepancy. — [Go modules reference](https://go.dev/ref/mod)

### Gaps
- This is the closure obtained by reading tagged module manifests, not a module graph generated by Go against the eventual project. Exact package/subpackage imports and download behavior remain to be verified after approval.
- No user approval is inferred for any item in these tables.

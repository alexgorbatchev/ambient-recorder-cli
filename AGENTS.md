# Working on ambient-recorder

## Commands

```sh
just build
just check
just run --help
just run-ai recording start --help
RECORDER_VERSION=1.0.0 just release
```

`just check` runs Go vet, module hygiene, formatting checks, race tests and a
build. `just release` also ad-hoc signs, verifies and packages the native Mac
architecture. It does not publish. Compiler outputs belong in `bin/`.

## Required grounding

- For documentation organization and maintenance, follow
  [documentation instructions](docs/agents.md).
- Read all applicable skills in full before writing or modifying code. CLI work
  requires cli-best-practices; Go work requires golang; documentation requires
  docs-writer; commits require git-commit.
- Inspect source, dependencies and applicable documentation before acting.
  Verify external API and tool behavior against current upstream documentation.
  Ground status claims in executed checks and retain their logs.
- Prefer approved maintained libraries or native primitives. Use dependencies
  as intended; do not add compatibility layers, stubs or surface imitations.
- Search with `rg` or codegraph. Never use heredocs. Keep temporary files and
  scripts in `.tmp`, retain them for inspection, and use `.workspaces` for
  worktrees based on main. Default temporary scripts to Bun and TypeScript.

## CLI and recording contracts

- Keep the approved path commands named `config print-dir` and `recording
  print-file`. Print only an absolute path plus a newline in both output modes;
  no open recording file means empty stdout and a nonzero exit. Current-file
  inspection must reject stale state after recorder exit.
- Maintain the embedded [usage reference](cmd/ambient-recorder/SKILL.md) in the
  same change as any command, argument, flag, default, environment, output or
  side-effect change. Update `last_modified`; verify against the implementation
  and pinned dependencies. The byte-match and live-interface tests must pass.
- Use Cobra and cobra-help-tree. Human help must fit terminal width; agent help
  must start with the skill-reading alert and retain untruncated details.
- Keep recording mono microphone plus playback, with low encoder CPU effort.
  Microphone switches retain the active file and encoder. Timestamp filenames
  use nine fractional-second digits; do not restore indexed naming.
- Configuration uses go-toml and `[input]`/`[output]`. Default microphone preference
  is `*`; unlisted inputs remain eligible. Global `!` exclusions and case-sensitive
  `*` name patterns apply before ordering. Log input connection/disconnection.
- Capture gaps and process downtime remain possible. Do not claim gapless capture
  or crash durability from a successful build or a finalized sample alone.
- Background service startup is at GUI login after reboot, as approved by the
  user. Keep native supervision and distinguish a loaded job from working audio.
- Service status must show the invoked CLI version and the version reported by
  the live service process. Never infer the running version from the binary on
  disk; report an unloaded service as not running and missing identity as unavailable.

## Verification and boundaries

- Runtime behavior changes require corresponding test-file changes and at least
  90% coverage of changed runtime code; `scripts/` is excluded. Use red/green
  tests, temporarily disable the change to demonstrate failure, then restore it.
  Test behavior rather than constant/configuration mirrors.
- Keep hardware capture opt-in. Never install/start a persistent service as part
  of a build or test. Keep recordings local; transcription and calendar features
  are future work.
- Native service tests use unique transient launchd jobs and retained `.tmp`
  plists, without audio capture or installation in `~/Library/LaunchAgents`.
  Never use the production service label in lifecycle tests.
- Ask first before adding any external dependency. Approved CI dependencies are
  checkout v7, setup-go v7, upload-artifact v7, download-artifact v8, and just
  1.58.0. No extra runtime dependencies are authorized by release work.
- Never publish releases, tags, packages or deployments without explicit user
  authorization. The user has explicitly authorized the public GitHub repository
  and 1.0.0 release; that authorization does not extend to later releases.
- The user has explicitly requested and authorized the next release, 1.1.0,
  with verification. This authorization does not extend to subsequent releases.
- Do not edit, stage, reset or commit another agent's changes. Pause and inspect
  concurrent staging/index locks; halt and report unowned staged files stalled
  over 60 seconds. Do not fix unrelated failures or change scope without consent.
- Automatically record new user instructions here or in the appropriate nested
  AGENTS.md. Ask before resolving conflicts with existing instructions.
- Report observed bugs, contract gaps or unverified limitations under
  `# DUE DILIGENCE`. Honor the user's stop-and-report directive for forbidden
  solution instructions.

See [contributor instructions](docs/internal/contributing.md) for native capture
design and opt-in live verification.

---
created_on: 2026-10-05 10:18
last_modified: 2026-10-05 10:18
status: current
---

# Documentation instructions

For agents organizing and maintaining project documentation. Follow the shared
[repository instructions](../AGENTS.md); this file supplies the documentation map
and local conventions.

## Document map

| Location | Purpose |
| --- | --- |
| [Contributor instructions](internal/contributing.md) | Current build, runtime design, release and verification procedures |
| [Bootstrap report](internal/archived/2026-09-30-bootstrap/report.md) | Historical recording implementation and measurements |
| [Bootstrap verification](internal/archived/2026-09-30-bootstrap/verification.md) | Historical test results and linked execution evidence |
| [Capture research](internal/archived/2026-09-30-bootstrap/research/capture.md) | Capture candidates, native APIs and permission boundaries |
| [CLI research](internal/archived/2026-09-30-bootstrap/research/cli.md) | Original command-framework dependency proposal |
| [Codec research](internal/archived/2026-09-30-bootstrap/research/opus.md) | Opus encoders, Ogg muxing and durability research |
| [Reliability research](internal/archived/2026-09-30-bootstrap/research/reliability.md) | Original supervision, storage and capture reliability findings |

## Organization and maintenance

- Keep project reports and research under `docs/internal/`, using descriptive
  lowercase filenames with hyphens between words. Group related material by topic.
- Put superseded snapshots under the nearest `archived/` directory. Preserve their
  research dates and measurement boundaries; direct readers to current guidance.
  Indexed filenames in the bootstrap archive are historical, not current naming
  instructions.
- Internal Markdown starts with `created_on`, `last_modified` and `status` YAML
  frontmatter. Use `YYYY-MM-DD HH:MM` timestamps and `current` or `archived` status.
  Preserve `created_on` and update `last_modified` when editing or relocating.
- Repair relative links after moving files. Keep referenced evidence with its
  record, identify path redactions, and check that committed links resolve to
  committed files. Retain original logs in `.tmp` when preparing redacted copies.
- Read the docs-writer skill before editing documentation. Verify commands against
  the implementation and task runner. CLI interface changes also require updating
  the [embedded usage reference](../cmd/ambient-recorder/SKILL.md).
- Automatically record new documentation instructions here; ask before resolving
  conflicts with existing repository instructions.
- Never publish releases, tags, packages or deployments without explicit user
  authorization. Keep hardware capture opt-in when verifying documented behavior.

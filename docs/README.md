# tabr-tau documentation index

Start here. Status as of 2026-09-30: design and audit complete (v0.2), implementation not started
beyond the original editor. Nothing in `docs/` contains account or platform IDs.

| Read this | To learn |
|---|---|
| [`design/2026-09-30-dune-docker-parity.md`](design/2026-09-30-dune-docker-parity.md) | How single-player actually runs (in-process listen server, SQLite, no RMQ), the save schema versus dune-docker's Postgres schema, what dune-docker does over RMQ, and which features can and cannot work in single-player |
| [`design/2026-09-30-architecture-and-implementation-plan.md`](design/2026-09-30-architecture-and-implementation-plan.md) | The feature table, target architecture, write-safety pipeline, security model, testing strategy, phased plan and risks (v0.2, after the audit) |
| [`audit/2026-09-30-L1-findings-register.md`](audit/2026-09-30-L1-findings-register.md) | The Layer 1 eight-hat audit: 22 merged findings, severities, STRIDE table, what needs an operator decision |
| [`single-player-files.md`](single-player-files.md) | Every file the single-player game keeps, its format and contents, and whether it is safe to edit |
| [`live-test-protocol.md`](live-test-protocol.md) | The pre-registered live experiment (literjon give/fill/drink, XP, hydration, foreign keys, id allocation) |
| [`evidence/`](evidence/) | Schema-only evidence: the real save DDL (no rows), SQLite-vs-Postgres schema diff, dune-docker function-to-table map |

Tools: [`../tools/snapshot.ps1`](../tools/snapshot.ps1) (Windows snapshot of a save) and
[`../tools/snapdiff.py`](../tools/snapdiff.py) (redacted row and JSON-path diff of two saves; Python >= 3.11).

## Evidence tags used in the design docs
**[V]** verified against a real sample save, log, or code. **[I]** inferred. **[T]** needs a live test.

## Before sharing
Saves, `Game.ini`, snapshots and logs contain account and platform IDs and are never committed. Do not attach
them to issues. Use `tools/snapdiff.py` (redacts by default) for anything you post.

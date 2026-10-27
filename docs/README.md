# tabr-tau documentation index

Start here. Status as of 2026-10-22: the editor is built and in use (Windows 10/11 only, one Go binary with a local web UI). The design
and Layer 1 audit of 2026-09-30 were carried out through governance (P0), security hardening (S0), the single write pipeline (S1),
UX safety (S2; its review pane was replaced by writing each edit as it is made, #238) and most of the Player tabs (S3); four audit
passes are on record (below). What is still open is in the open issues
of the repository. Nothing in `docs/` contains account or platform IDs.

| Read this | To learn |
|---|---|
| [`../README.md`](../README.md) | What the app does, how saving works, how to restore a backup, and which write features are checked in the game |
| [`../CHANGELOG.md`](../CHANGELOG.md) | Every change since the start, by release section |
| [`design/2026-09-30-dune-docker-parity.md`](design/2026-09-30-dune-docker-parity.md) | How single-player actually runs (in-process listen server, SQLite, no RMQ), the save schema versus dune-docker's Postgres schema, what dune-docker does over RMQ, and which features can and cannot work in single-player |
| [`design/2026-09-30-architecture-and-implementation-plan.md`](design/2026-09-30-architecture-and-implementation-plan.md) | The feature table, target architecture, write-safety pipeline, security model, testing strategy, phased plan and risks (v0.2, after the audit). Its status table is kept current |
| [`design/2026-10-08-windows-only-and-go.md`](design/2026-10-08-windows-only-and-go.md) | Why the app is Windows-only and written in Go |
| [`design/2026-10-08-feature-comparison-selfhost-console.md`](design/2026-10-08-feature-comparison-selfhost-console.md) | The console's features compared with this app's |
| [`design/2026-10-08-player-tabs-spec.md`](design/2026-10-08-player-tabs-spec.md) | The Player tabs mirrored from the console |
| [`design/2026-10-24-player-tabs-status.md`](design/2026-10-24-player-tabs-status.md) | Which of the console's 12 Player tabs the save supports, what is built and what is not, and why |
| [`design/2026-10-26-ui-text-sweep.md`](design/2026-10-26-ui-text-sweep.md) | The rule for what a page shows all the time, the one mechanism (tooltip, About this), and every permanent explanatory line of the editor with its decision (#426) |
| [`design/feature-inventory.md`](design/feature-inventory.md) | Every console feature rated Done, Gap or Out of scope (regenerate with `tools/featureinv.py`) |
| [`audit/2026-09-30-L1-findings-register.md`](audit/2026-09-30-L1-findings-register.md) | Layer 1 (design) eight-hat audit: 22 merged findings, STRIDE table |
| [`audit/2026-10-07-L2-findings-register.md`](audit/2026-10-07-L2-findings-register.md) | Layer 2 audit of the write pipeline (S0, S1 slices 1-2) |
| [`audit/2026-10-15-L2-findings-register.md`](audit/2026-10-15-L2-findings-register.md) | Layer 2 audit of everything built since: 93 findings in 25 issue groups, STRIDE report; its dispositions are updated as they are fixed |
| [`audit/2026-10-22-L2-findings-register.md`](audit/2026-10-22-L2-findings-register.md) | Layer 2 re-audit after write-as-you-go: 14 issue groups, STRIDE per finding, issues #251-#264 |
| [`audit/2026-10-27-code-review-delta.md`](audit/2026-10-27-code-review-delta.md) | Layer 3 code review of the changes since 2026-10-23: 2 medium and 9 low findings, issues #441-#443 |
| [`single-player-files.md`](single-player-files.md) | Every file the single-player game keeps, its format and contents, and whether it is safe to edit |
| [`research/2026-10-25-console-docs-for-the-editor.md`](research/2026-10-25-console-docs-for-the-editor.md) | What the Dune Docker documentation and console code can still teach the offline editor: ranked top 10, traps checked, data files compared (console fork main v1.4.51, upstream v1.4.52) |
| [`live-test-protocol.md`](live-test-protocol.md) | The pre-registered live experiments and their recorded results |
| [`continuation-prompt.md`](continuation-prompt.md) | Paste into a new session to resume: state, verified facts, what to do next, and what not to do |
| [`evidence/`](evidence/) | Schema-only evidence: the real save DDL (no rows), SQLite-vs-Postgres schema diff, dune-docker function-to-table map, the Windows first-run record |

Tools: [`snapshot.ps1`](../tools/snapshot.ps1) (Windows snapshot of a save), [`send-snapshot.ps1`](../tools/send-snapshot.ps1)
(hash-verified scp transfer), [`snapdiff.py`](../tools/snapdiff.py) (redacted row and JSON-path diff of two saves) and
[`snapsummary.py`](../tools/snapsummary.py) (redacted, committable fingerprint of a snapshot). Python >= 3.11. The Windows first-run record in `evidence/` covers the editor itself, not these scripts.

## Evidence tags used in the design docs
**[V]** verified against a real sample save, log, or code. **[I]** inferred. **[T]** needs a live test.

## Before sharing
Saves, `Game.ini`, snapshots and logs contain account and platform IDs and are never committed. Do not attach
them to issues. Use `tools/snapdiff.py` (redacts by default) for anything you post.

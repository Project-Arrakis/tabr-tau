# Layer 2 (implementation) audit: S0 step 4 and S1 slices 1-2

Date: 2026-10-07. Scope: `internal/save` (`mutate.go`, `durable.go`, `save.go` read path, fingerprint), `internal/ops`
(`sqlvet.go`, `db.go` and the ~40 write sites migrated to `save.Mutate`), their tests, and the UI paths that reach them.
Merged work covered: #36 (SQL consoles + fingerprint), #37 (durable Commit/Restore), #38 (single write path).
Method: eight independent read-only hat reviewers (Architect, Security, GRC, Network+Cloud, DBA, QA, UI/UX; Network and Cloud
were one reviewer), each told to verify against real code, plus a five-agent `/code-review high` pass on PR #38. QA ran
mutation checks in a scratch copy. Tracking issue: #24. Fixes below landed in PR #38 unless stated.

## Disposition summary
- CRITICAL: none.
- HIGH: 9 raised, 8 fixed in #38, 1 deferred with justification (container child deletes, scheduled as F-08 / #9).
- MEDIUM and LOW: fixed where cheap and safe, otherwise filed (see "Filed").

## Findings
| ID | Hat | Sev | STRIDE | Finding | Disposition |
|---|---|---|---|---|---|
| ARCH-1 | Architect | HIGH | DoS | A panic inside a `Mutate` edit left the transaction open on the single-connection pool, wedging the editor | **Fixed** (deferred rollback; test `TestMutatePanicDoesNotWedgeTheSave`) |
| S1 / ARCH-3 | Security, Architect | HIGH | Tampering, EoP | Column names read from a hostile save were spliced into SQL unescaped (search and cell edit) | **Fixed** (`quoteIdent`; Overview skips non-identifier table names; test `TestColumnNamesFromTheSaveCannotInjectSQL`) |
| S2 | Security | HIGH | Tampering, EoP | Fingerprint ignored every object named `sqlite_*` | **Fixed** (only internal TABLES skipped; test `TestFingerprintCatchesDisguisedObjects`) |
| D1 | DBA | HIGH | Tampering | Deleting a container item leaves its child inventory, items and references behind (`NOT IN` with NULLs, empty-only cleanup, FKs off) | **Deferred**: is exactly F-08 (#9), next S1 slice, together with the `foreign_key_check` gate (D2). Pre-existing behaviour, not introduced by #38; README and CHANGELOG now state it |
| UX-1 | UI/UX | HIGH | N/A | No read-only banner: edits looked live and failed one toast at a time | **Fixed** (`readOnly` in Overview, persistent banner with the reason) |
| UX-2 | UI/UX | HIGH | Repudiation | Commit result: `warning` ignored, `saved:false` shown as "Saved. Backup: undefined" | **Fixed** |
| UX-3 | UI/UX | HIGH | Tampering, DoS | "Changed on disk" has no recovery path in the UI; `force` is never sent | **Deferred** to S2 (F-10, #11): needs the review pane and sticky errors designed together; the toast already names Discard |
| G-1 | GRC | HIGH | Repudiation | CHANGELOG said F-03..F-08 "not fixed yet" | **Fixed** |
| G-2 | GRC | HIGH | Repudiation | Findings register had no record of S0/S1 progress or of this audit | **Fixed** (progress log, this file) |
| G-3 | GRC | HIGH | Repudiation | Plan section 4.2 and the current-state table asserted the old behaviour | **Fixed** (implementation-status block) |
| S3 | Security | MED | Tampering | Fingerprint normalised case/whitespace across string literals | **Fixed** (exact SQL compare; verified a real save is not falsely blocked) |
| S4 | Security | MED | Tampering, DoS | Virtual tables detected by SQL text prefix, defeated by a comment | **Fixed** (`rootpage = 0`) |
| S5 | Security | MED | Tampering, EoP | `pragma_*` table functions and `sqlite_master` writes passed vetting | **Fixed** (write console: none; read console: schema-describing whitelist) |
| ARCH-2 | Architect | MED | Tampering, Repudiation | `Query/Table/One` ran on the read-write connection, so a stray write bypassed `Mutate` | **Fixed** (they use the read-only connection; test `TestQueryCannotWriteBypassingMutate`) |
| ARCH-4 | Architect | MED | Tampering, Repudiation | A failed reload after Restore left a stale working copy that could later overwrite the restore | **Fixed** (writes refused with a restart message) |
| D3 | DBA | MED | Tampering, EoP | `applied_patches` / `sqlite_*` editable from the cell editor and console | **Fixed** |
| D2 | DBA | MED | Tampering | No `foreign_key_check` gate | Deferred: F-08 (#9) |
| D4, D5, ARCH-7 | DBA, Architect | MED/LOW | N/A | "value is unchanged" message wrong (SQLite counts matched rows) | **Fixed** (message corrected). The `total_changes` over-count on no-op/savepoint edits is harmless (an extra dirty flag) |
| D6 | DBA | MED | Tampering | `giveInTx`: id sequencing ignores `sqlite_sequence`, no volume/stack-cap/template checks | Filed on F-17 (#18); query errors in `giveInTx` no longer swallowed (fixed) |
| D7 | DBA | LOW | Tampering | Write console can orphan rows with FKs off | Deferred: F-08 (#9) |
| D8 | DBA | MED | DoS, Repudiation | Restore silently discards pending edits; no backup retention or free-space handling | Filed: #39 |
| D9, ARCH-5, ARCH-6 | DBA, Architect | LOW | Tampering, DoS | Hash/rename TOCTOU window; `load()` not atomic; ownership pre-checks outside the transaction | Filed: #39 |
| QA-1..3 | QA | MED | Tampering, EoP | Untested branches: Restore changed-on-disk, read-back-mismatch restore, `with ... delete` classification | **Fixed** (three tests added) |
| QA-4, QA-5, QA-6 | QA | MED/LOW | N/A | ops tests still on a toy schema; no ops-level atomicity test; RepairGear partial-failure untested | Filed on F-02 (#3). RepairGear is now all-or-nothing by design (it used to skip failing items) |
| QA-7, QA-8, QA-10 | QA | LOW | DoS | Wall-clock thresholds, global test hooks, `GameRunning` untestable off Windows | Filed: #41 |
| QA-9 | QA | LOW | N/A | Test `Exec` helper writes a synthetic description | Accepted |
| UX-4..UX-7, a11y | UI/UX | MED/LOW | Tampering | Backup list readability and restore confirmation, SQL errors only as toasts, CSV export ignores HTTP errors, failed edits leave the typed value, toasts lack `role=alert` | Filed on F-21 (#22) |
| N-1 | Network | LOW | EoP | Quoted `load_extension` slipped past the name check | **Fixed** (quoted names checked by content) |
| S6 | Security | LOW | InfoDisc, Tampering | A string literal can name a table | **Fixed** (literals checked; LIKE patterns with `%` allowed) |
| S7, S8, N-2, N-3 | Security, Network | LOW | DoS, InfoDisc | No timeout on the write console, huge `zeroblob` reads, decoded saves in the shared temp dir with no startup sweep, OneDrive/UNC save folders | Filed: #40 |
| C-1, C-2, C-3 | Cloud | MED/LOW | Tampering, Spoofing | Reusable workflow referenced at `@main`; snapshot script uses TOFU and an interpolated remote command; no advisory check possible offline | Filed on F-09 (#10); `govulncheck` already runs in CI |
| G-4..G-8 | GRC | MED/LOW | N/A | Stale `schema.sql` path references; CHANGELOG qualifiers; README wording | **Fixed** (G-7/G-8 accepted: no CONTRIBUTING.md, PR template covers it) |
| CR-1 | code-review | MED | N/A | Several single-statement ops returned ok when nothing matched | **Fixed** (piece, task, decree now error; idempotent tag/tutorial removal intentionally still ok) |
| CR-2 | code-review | LOW | N/A | CHANGELOG cited "40 write sites" | **Fixed** (wording) |
| CR-3 | code-review | LOW | N/A | RepairGear lost best-effort per-item behaviour | Intended, documented above |

## STRIDE report
| Category | Findings | Highest | Status |
|---|---|---|---|
| Spoofing | C-2 | LOW | Filed |
| Tampering | S1, S2, S3, S4, S5, S6, D1, D2, D3, D6, D7, ARCH-2, ARCH-4, UX-3, C-1, QA-1..3 | HIGH | All but D1/D2/D7 (F-08, #9), UX-3 (S2, #11) and D6 (#18) fixed |
| Repudiation | UX-2, ARCH-2, ARCH-4, D8, G-1..G-3 | HIGH | Fixed except D8 (filed) |
| Information disclosure | S6, S8, N-2 | LOW | S6 fixed; others filed |
| Denial of service | ARCH-1, S4, S7, D8, QA-7 | HIGH | ARCH-1, S4 fixed; others filed |
| Elevation of privilege | S1, S2, S5, D3, N-1 | HIGH | Fixed |
| N/A | UX-1, D4/D5, QA-4..9, G-4..G-8, CR-1..3 | HIGH (UX-1) | Fixed or filed |

## Evidence
Reviewer reports were produced by read-only agents against branch `issue/6-mutate-chokepoint` at 4d4fed95; QA mutation results:
7 of 11 mutations were caught; the 4 survivors (QA-1..3 and a timeout hang) are now covered or filed. After the fixes:
`go vet`, staticcheck, `go test ./...` and the 41 browser-engine UI tests pass locally; the new fingerprint was checked against
the real baseline save (not falsely blocked).

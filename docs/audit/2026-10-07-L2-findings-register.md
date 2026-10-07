# Layer 2 (implementation) audit: S0 step 4 and S1 slices 1-2

Date: 2026-10-07. Scope: `internal/save` (`mutate.go`, `durable.go`, `save.go` read path, fingerprint), `internal/ops`
(`sqlvet.go`, `db.go` and the ~40 write sites migrated to `save.Mutate`), their tests, and the UI paths that reach them.
Merged work covered: #36 (SQL consoles + fingerprint), #37 (durable Commit/Restore), #38 (single write path).
Method: eight independent read-only hat reviewers (Architect, Security, GRC, Network+Cloud, DBA, QA, UI/UX; Network and Cloud
were one reviewer), each told to verify against real code, plus a five-agent `/code-review high` pass on PR #38. QA ran
mutation checks in a scratch copy. Tracking issue: #24. Fixes below landed in PR #38 unless stated.

## Disposition summary
- CRITICAL: none.
- HIGH: 9 raised; 7 fixed in #38, D1 (container child deletes) fixed in #42, UX-3 deferred to S2 (#11).
- MEDIUM and LOW: fixed where cheap and safe, otherwise filed (see "Filed").

## Findings
| ID | Hat | Sev | STRIDE | Finding | Disposition |
|---|---|---|---|---|---|
| ARCH-1 | Architect | HIGH | DoS | A panic inside a `Mutate` edit left the transaction open on the single-connection pool, wedging the editor | **Fixed** (deferred rollback; test `TestMutatePanicDoesNotWedgeTheSave`) |
| S1 / ARCH-3 | Security, Architect | HIGH | Tampering, EoP | Column names read from a hostile save were spliced into SQL unescaped (search and cell edit) | **Fixed** (`quoteIdent`; Overview skips non-identifier table names; test `TestColumnNamesFromTheSaveCannotInjectSQL`) |
| S2 | Security | HIGH | Tampering, EoP | Fingerprint ignored every object named `sqlite_*` | **Fixed** (only internal TABLES skipped; test `TestFingerprintCatchesDisguisedObjects`) |
| D1 | DBA | HIGH | Tampering | Deleting a container item leaves its child inventory, items and references behind (`NOT IN` with NULLs, empty-only cleanup, FKs off) | **Fixed in PR #42** (F-08): cascading item delete via `MutateCascade`, so a bag takes its inventories and contents with it; pre-existing behaviour, not introduced by #38 |
| UX-1 | UI/UX | HIGH | N/A | No read-only banner: edits looked live and failed one toast at a time | **Fixed** (`readOnly` in Overview, persistent banner with the reason) |
| UX-2 | UI/UX | HIGH | Repudiation | Commit result: `warning` ignored, `saved:false` shown as "Saved. Backup: undefined" | **Fixed** |
| UX-3 | UI/UX | HIGH | Tampering, DoS | "Changed on disk" has no recovery path in the UI; `force` is never sent | **Fixed** in S2 slice 1 (PR #50): a changed-on-disk failure keeps the review open, lists the edits to redo and offers Reload from disk (UI test "a save error stays in the review ... offers reload"). `force` is still never sent |
| G-1 | GRC | HIGH | Repudiation | CHANGELOG said F-03..F-08 "not fixed yet" | **Fixed** |
| G-2 | GRC | HIGH | Repudiation | Findings register had no record of S0/S1 progress or of this audit | **Fixed** (progress log, this file) |
| G-3 | GRC | HIGH | Repudiation | Plan section 4.2 and the current-state table asserted the old behaviour | **Fixed** (implementation-status block) |
| S3 | Security | MED | Tampering | Fingerprint normalised case/whitespace across string literals | **Fixed** (exact SQL compare; verified a real save is not falsely blocked) |
| S4 | Security | MED | Tampering, DoS | Virtual tables detected by SQL text prefix, defeated by a comment | **Fixed** (`rootpage = 0`) |
| S5 | Security | MED | Tampering, EoP | `pragma_*` table functions and `sqlite_master` writes passed vetting | **Fixed** (write console: none; read console: schema-describing whitelist) |
| ARCH-2 | Architect | MED | Tampering, Repudiation | `Query/Table/One` ran on the read-write connection, so a stray write bypassed `Mutate` | **Fixed** (they use the read-only connection; test `TestQueryCannotWriteBypassingMutate`) |
| ARCH-4 | Architect | MED | Tampering, Repudiation | A failed reload after Restore left a stale working copy that could later overwrite the restore | **Fixed** (writes refused with a restart message) |
| D3 | DBA | MED | Tampering, EoP | `applied_patches` / `sqlite_*` editable from the cell editor and console | **Fixed** |
| D2 | DBA | MED | Tampering | No `foreign_key_check` gate | **Fixed in PR #42** (per-row baseline diff, refuses any new dangling reference) |
| D4, D5, ARCH-7 | DBA, Architect | MED/LOW | N/A | "value is unchanged" message wrong (SQLite counts matched rows) | **Fixed** (message corrected). The `total_changes` over-count on no-op/savepoint edits is harmless (an extra dirty flag) |
| D6 | DBA | MED | Tampering | `giveInTx`: id sequencing ignores `sqlite_sequence`, no volume/stack-cap/template checks | Filed on F-17 (#18); query errors in `giveInTx` no longer swallowed (fixed) |
| D7 | DBA | LOW | Tampering | Write console can orphan rows with FKs off | **Fixed in PR #42** (the gate refuses the edit; the console is gated, not cascaded) |
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
| Tampering | S1, S2, S3, S4, S5, S6, D1, D2, D3, D6, D7, ARCH-2, ARCH-4, UX-3, C-1, QA-1..3 | HIGH | All but UX-3 (S2, #11) and D6 (#18) fixed (D1/D2/D7 fixed in #42) |
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

## Slice 4 (PR #43: baseline, op record, `internal/diff`, review endpoint)
Audited the same day with a Security/privacy hat, a QA/Architect hat and a five-agent code review (reports on PR #43 / #24).
No CRITICAL. Every finding below was fixed in the PR unless a link says otherwise.

| ID | Sev | STRIDE | Finding | Disposition |
|---|---|---|---|---|
| P1 | HIGH | Tampering, Repudiation | Redaction ran before comparison and before key building: an edited `funcom_id`/`platform_id`/name, or one 15+ digit id changed to another, vanished from the diff | **Fixed**: compare raw, mask only in output (test: identity edit still reported as `<redacted>` -> `<redacted>`) |
| P2 | HIGH | Information disclosure | Redaction missed integer and float ids, JSON numbers, JSON keys, short-digit ids and other personal columns | **Fixed**: type-independent 15+ digit masking incl. JSON keys, column-name rules, edit descriptions masked; `TestEveryColumnIsClassified` fails on a new unclassified personal-looking column |
| P3 | MED | Tampering | Row keys built from masked, untyped text: redacted primary keys collided, `1` and `'1'` and NULL and `'<nil>'` merged, rows silently dropped | **Fixed**: raw typed keys |
| P4 | MED | Tampering | JSON integers compared with a relative float tolerance (2,000,000 -> 2,000,001 hidden); int64 compared through float64 | **Fixed**: integers decode as int64 and compare exactly |
| P5 | MED | DoS | `MaxRows` unclamped; one blob scan per column; blob-kind errors swallowed | **Fixed**: clamped 1..1000, one scan per table, errors returned |
| P6 | MED | Repudiation, DoS | `load()` failure part-way leaked the decoded copy and left a closed handle; a reload failure after Commit left a stale baseline editable | **Fixed**: build-then-swap `load()`; reload failure after Commit/Restore disables edits |
| P7 | MED | Information disclosure | Edit descriptions (`sql: ...`) unmasked in the review | **Fixed** (`diff.RedactText`) |
| P8 | LOW | N/A | Review read dirty, ops and diff at different moments | **Fixed** (`WithBaselineState`) |
| P9 | LOW | Tampering | `Change.before/after` had `omitempty` (a change from 0 lost its before) | **Fixed** |
| P10 | LOW | Information disclosure | Control characters from a hostile save reached the terminal via table names | **Fixed** (`clean`) |
| P11 | LOW | N/A | CLI lacked `--ignore-tables/--ignore-columns/--float-eps`; stale doc comment | **Fixed** |
| P12 | LOW | DoS | `Compare` holds the save lock for the whole diff and loads full tables in memory (UI stalls on a very large save) | Filed on #40 |
| P13 | LOW | N/A | PR said "Closes #7" though #7's disposition lists the review pane (S2) | **Fixed** (PR says "Refs #7"; #7 stays open for the pane) |
Tests added for the endpoint (shape, redaction, gating, limit bounds), composite and typed keys, table-set differences, text output, malformed/opaque blobs, read-only handle, failed-reload behaviour. QA mutation checks: 13 of 14 mutations caught; the survivor (OpenReadOnly made writable) now has a test.

## Layer 2: S2 slice 1 (review pane, PR #50), 2026-10-07

Eight read-only reviewers (Architect, Security, GRC, Network+Cloud, UI/UX, DBA, QA) read the diff. No CRITICAL. Findings and dispositions:

| ID | Hat | Sev | STRIDE | Finding | Disposition |
|---|---|---|---|---|---|
| S2-1 | Arch, Sec, DBA | HIGH | Tampering | Token checked in the route, commit takes the lock later: an edit landing between them is saved unseen | **Fixed**: `Save.CommitReviewed` compares under the commit lock (`TestCommitReviewedRefusesStaleAndMissingTokens`) |
| S2-2 | Sec, DBA, Arch | HIGH | Tampering | Token hashed only descriptions (truncated SQL text, undescribed edits) and not the data | **Fixed**: token = loaded-file hash + edits + working-copy content (`TestReviewTokenBindsTheData`) |
| S2-3 | Sec, DBA | HIGH | Tampering | Restore bypasses the review and drops pending edits | **Fixed in part**: Restore refuses while edits are pending (`TestRestoreRefusesWithPendingEdits`). Restore itself stays a separate path by design (backs up first, verifies, asks for confirmation); the token is a consistency control, not a defence against a holder of the session cookie (the cookie, SameSite, Host/Origin checks are those controls) |
| S2-4 | Sec, Arch | LOW | Tampering | `force` accepted over HTTP skips the game-running check | **Fixed**: route ignores it (`force is ignored` case) |
| S2-5 | Arch | MEDIUM | N/A | A refresh error after a successful save threw on `R=null` | **Fixed**: refresh errors are caught and reported |
| S2-6 | Arch, QA, UI | MEDIUM | N/A | Stale-token error left Save enabled in a loop; stale detection matched message text | **Fixed**: server sends `code` (`changed_on_disk`, `review_changed`); a stale token refetches the review (UI test) |
| S2-7 | Net | MEDIUM | DoS | The 5 s poll ran the table-count Overview and an uncached tasklist | **Fixed**: light `/api/save/state`; `GameRunning` cached 3 s with a 3 s timeout; poll skips hidden tabs |
| S2-8 | Net, UI | LOW | DoS | A failed poll was swallowed, leaving a stale "game closed" and Save enabled | **Fixed**: banner "Lost contact with the editor", Save off |
| S2-9 | UI | HIGH | N/A | Pane not a dialog: no focus, no Escape/backdrop close, background scrolls | **Fixed**: role=dialog, focus, Escape, backdrop, scroll lock. Full focus trap deferred to the UI/UX overhaul (#46) |
| S2-10 | UI | MEDIUM | N/A | Truncated diff rows not announced | **Fixed** (note when listed rows < counted) |
| S2-11 | UI | LOW | N/A | Success toast vanished in 3.5 s; reload had no confirm; double-click opened two reviews | **Fixed** (15 s, confirm, guard) |
| S2-12 | GRC | MEDIUM | Repudiation | README, plan status block, L1/L2 registers, live-test protocol, CHANGELOG stale | **Fixed** in this PR |
| S2-13 | QA | MEDIUM | N/A | Tests could not tell a description-only token from a data token; UI stub ignored the token | **Fixed**: data-binding and same-count tests; UI test asserts the token sent; code values in stubs match the server |
| S2-14 | UI | LOW | N/A | Mobile layout, sticky action row, light-theme warn colours | **Deferred** to the UI/UX overhaul (#46, #48) |
| S2-15 | Sec | LOW | Spoofing | Token is deterministic and any cookie holder can fetch it; requests without Origin/Sec-Fetch-Site are allowed | **Accepted, documented**: HttpOnly session cookie is the control; not a hostile-process defence |
| S2-16 | Net | LOW | Info disclosure | `pending` descriptions in the state poll are not redacted | **Accepted**: same-origin, no-store, local user's own edits; review view redacts by default |

### STRIDE
| Category | Findings | Status |
|---|---|---|
| Spoofing | S2-15 | Accepted, documented |
| Tampering | S2-1, S2-2, S2-3, S2-4 | Fixed |
| Repudiation | S2-12 (stale docs), S2-3 (silent loss of edits) | Fixed |
| Information disclosure | S2-16 | Accepted |
| Denial of service | S2-7, S2-8 | Fixed |
| Elevation of privilege | N/A | None found |

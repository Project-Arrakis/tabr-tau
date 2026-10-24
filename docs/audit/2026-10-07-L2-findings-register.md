# Layer 2 (implementation) audit: S0 step 4 and S1 slices 1-2

Date: 2026-10-07. Scope: `internal/save` (`mutate.go`, `durable.go`, `save.go` read path, fingerprint), `internal/ops`
(`sqlvet.go`, `db.go` and the ~40 write sites migrated to `save.Mutate`), their tests, and the UI paths that reach them.
Merged work covered: #36 (SQL consoles + fingerprint), #37 (durable Commit/Restore), #38 (single write path).
Method: eight independent read-only hat reviewers (Architect, Security, GRC, Network+Cloud, DBA, QA, UI/UX; Network and Cloud
were one reviewer), each told to verify against real code, plus a five-agent `/code-review high` pass on PR #38. QA ran
mutation checks in a scratch copy. Tracking issue: #24. Fixes below landed in PR #38 unless stated.

## Reconciliation, 2026-10-23

Every finding was checked again against the current code, tests and CI (not against the old text). The "Disposition" columns below are kept as written on 2026-10-07; the table here is the current status. The "Review & save" pane was removed on 2026-10-22 (#238, #240, see `docs/control-removal.md` and `SECURITY.md`), which makes the S2 block, P7, P8, UX-2 and UX-3 obsolete. Ids such as ARCH-5, ARCH-6 and QA-4 are used again with other meanings in the 2026-10-22 register; here they mean what this file says.

Counts (84 findings; the rows that listed several ids at once are counted once per id, except where the id list is one finding: D4/D5/ARCH-7 and S1/ARCH-3): **58 fixed**, **12 obsolete**, **9 accepted risk**, **5 open**. (Changed on 2026-10-23 by #39: ARCH-6 is fixed, D8 and D9 are accepted risk; before that 55, 12, 7 and 10. Changed on 2026-10-24 by #382 (S7) and #386 (QA-7): both are fixed.)

| Status | Findings |
|---|---|
| Open, needs the live game | D6 (volume and `sqlite_sequence`), N-3 (cloud-sync behaviour of the backups folder) |
| Open, needs a decision (recommendation: accept, with a line in `SECURITY.md`) | QA-6 (no mid-way failure injection into a repair), C-2 (`StrictHostKeyChecking=accept-new` in `tools/send-snapshot.ps1`) |
| Open, hygiene | QA-4 (12 uses of the toy schema remain in `internal/ops/ops_test.go`) |

| ID | Status | Evidence |
|---|---|---|
| ARCH-1 | fixed | deferred rollback `internal/save/mutate.go`; `TestMutatePanicDoesNotWedgeTheSave` |
| S1 / ARCH-3 | fixed | `quoteIdent`, `identRe` in `internal/ops/db.go`; `TestColumnNamesFromTheSaveCannotInjectSQL` |
| S2 | fixed | #301 closed a gap: a virtual table named `sqlite_*` was skipped by name; `TestFingerprintCatchesAVirtualTableNamedLikeAnInternalOne` |
| D1 | fixed | #42; `TestDeletingAContainerItemRemovesItsContentsAndLeavesNothingDangling` |
| UX-1 | fixed | read-only banner with the reason (`app.js`) |
| UX-2, UX-3 | obsolete | the commit result and the changed-on-disk dead end belonged to the review pane; changed-on-disk is now handled by the flush (held queue, rebase, #272, #283) |
| G-1, G-2, G-3 | fixed | CHANGELOG and plan status block rewritten (#186, #275) |
| S3, S4, S5 | fixed | exact SQL compare and `rootpage = 0` in `internal/save/save.go`; `internal/ops/sqlvet.go`; `TestFingerprintCatchesDisguisedObjects`, `TestVetRefusesInternalsAndPragmaFunctions` |
| ARCH-2, ARCH-4 | fixed | reads use the read-only handle (`TestQueryCannotWriteBypassingMutate`); a failed reload blocks edits (`TestCommitReloadFailureBlocksFurtherEdits`) |
| D2, D3, D7 | fixed | the foreign-key gate `internal/save/mutate.go` (#42); `lockedTables` in `internal/ops/db.go` |
| D4/D5/ARCH-7 | fixed | the "value is unchanged" text no longer exists |
| D6 | open (live game) | the stack cap (`catalog.MaxStack`, #240, #280), the known-template check (#185) and the capacity check are in; the two ignored reads in `giveRowsInTx` return their errors (#377); `strftime` is cast and the reads of the sequencer are checked (#383). `max_item_volume` is only displayed, never checked on give (needs per-item volume data, #61), and `sqlite_sequence` is not read. What the game enforces needs a live check |
| D8 | accepted risk | Restore refuses while edits are pending (`TestRestoreRefusesWithPendingEdits`); retention keeps the 60 newest plus the first and oldest (#273); there is no free-space check: a backup or temp file that cannot be written fails before the file is replaced and changes nothing, and `SECURITY.md` says so (#39) |
| D9 | accepted risk | the game and the hash are checked again right before the rename (`internal/save/durable.go`, #297); a window of milliseconds remains, a game save inside it is caught by the read-back, which leaves the game's file alone; `SECURITY.md` says so (#39) |
| ARCH-5 | fixed | build-then-swap `load()`; `TestFailedReloadLeavesThePreviousStateUsable` |
| ARCH-6 | fixed | `requireEditableItem` and `GiveItem` check ownership before `Mutate` (`internal/ops/scope.go`); the transaction re-reads the row, so the result is a clean "not found" (accepted in writing: only this editor's own requests can race); the two reads of `giveRowsInTx` that ignored their errors (the used slots and the stats source) return them (#39, `TestAGiveReturnsTheErrorOfAReadThatFailed`) |
| QA-1, QA-2, QA-3 | fixed | `TestRestoreRefusesWhenTheFileChangedOnDisk`, `TestReadBackMismatchRestoresThePreviousBytes`, `TestWriteConsoleAllowlist` |
| QA-4 | open (hygiene) | real-schema fixtures are used in about 40 test files; `newOps` (toy schema) remains in `ops_test.go` |
| QA-5 | fixed | `TestGiveItemsQueueIsAtomicAndUsesDistinctSlots`, `TestWriteConsoleIsAtomicAcrossStatements` |
| QA-6 | open (decision) | `RepairGear` is one `Mutate`; the real schema offers no natural way to make its update fail half way |
| QA-7 | fixed (#386) | the read console sets its time limit through `save.DeadlinesForTests` and checks `save.ErrStopped`; the write console and the give at its bound no longer measure time; the repair of buildings is checked by the plan of its statements (no correlated subquery) |
| QA-8 | accepted risk | hooks are package globals, guarded by `TestTestHooksAreNotCalledByProductionCode`; no test uses `t.Parallel` |
| QA-9 | accepted risk | as recorded on 2026-10-07 |
| QA-10 | fixed | `goos` and `tasklistOutput` are injectable in `gameproc.go` |
| UX-4 | fixed | #214, readable backups with a typed word to restore |
| UX-5 | fixed | #324: a failed statement is shown in the result area and clears the old result |
| UX-6 | fixed | #304: a refused export is shown, not downloaded |
| UX-7 | fixed | the typed value goes back after a refusal (`editInput`); toasts have `role=alert` (#214) |
| N-1, S6 | fixed | `internal/ops/sqlvet.go` checks quoted names and literals by content |
| S7 | fixed (#382, #295) | the write console is bounded (15 s, `TestAWriteStatementThatNeverEndsIsStoppedAndNothingIsChanged`); `Table`, `Query` and `One` stop after `save.ReadTimeout` (5 s), the list of tables ends with an error when a count is stopped, and `inspect` stops after 60 s (`internal/save/deadline_test.go`, `internal/ops/deadline_test.go`) |
| S8 | fixed | #302 refuses `zeroblob` and `randomblob` in both consoles; `pragma_database_list` is refused |
| N-2 | fixed | decoded copies live in a folder of their own, named by process, swept at start (#298) |
| N-3 | open (live game) | UNC, network and removable drives need a confirmation (#202); whether the game's cloud sync touches the backups folder "has not been tested" (README); a backups folder that is a link or a junction is refused (#382) |
| C-1 | fixed | the reusable workflow is pinned by commit (#277) |
| C-2 | open (decision) | the label and the target are whitelisted before they enter the remote command; the host key is trusted on first use |
| C-3 | fixed | `govulncheck` for both builds in CI; Dependabot covers gomod, actions and npm |
| G-4, G-5, G-6 | fixed | `schema.sql` references are consistent (`testsave_test.go` pins it against the evidence file); CHANGELOG and README reworked (#186, #205, #275) |
| G-7, G-8 | accepted risk | no CONTRIBUTING.md; the pull request template covers it |
| CR-1, CR-2 | fixed | not-found is an error for the Landsraad task and decree; the "40 write sites" wording is gone |
| CR-3 | accepted risk | `RepairGear` is all-or-nothing by design |
| P1, P2, P3, P4, P5, P9, P10, P11, P13 | fixed | `internal/diff/diff_test.go`; the CLI flags exist in `cmd/tabr-tau/main.go`; the PR says "Refs #7". C1 control characters (0x80-0x9f) are not stripped by `clean`: negligible |
| P6 | fixed | build-then-swap `load()`; a reload failure blocks edits |
| P7, P8, P12 | obsolete | the review view, `WithBaselineState` and the comparison under the save lock are gone (`Compare` now runs only in `tabr-tau diff`) |
| S2-3, S2-6, S2-7, S2-8, S2-9, S2-12, S2-14 | fixed | Restore refuses with pending edits; stable error `code`s (#292); light `/api/save/state` and a 3 s cached game check; "Lost contact" banner; the `ask()` dialog is named, inert behind, Tab-trapped, restores focus (#282); docs rewritten; `--warn` and small-screen layout (#244, #289) |
| S2-1, S2-2, S2-4, S2-5, S2-10, S2-11, S2-13 | obsolete | review token, `CommitReviewed`, `force` and the review pane's UI are gone |
| S2-15, S2-16 | accepted risk | requests without Origin or Sec-Fetch-Site are allowed and the cookie is the control (`SECURITY.md`, "Known limits"); the pending list is raw, same-origin and cookie-gated |

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
| S2-7 | Net | MEDIUM | DoS | The 5 s poll ran the table-count Overview and an uncached tasklist | **Fixed**: light `/api/save/state`; `GameRunning` cached 3 s with a 3 s timeout; poll skips hidden tabs (later: the game check reads the process snapshot and starts no program, cache 15 s, #366) |
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

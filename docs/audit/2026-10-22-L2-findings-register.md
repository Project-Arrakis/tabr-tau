# Findings register, 2026-10-22: editor (tabr-tau)

Method (Project Arrakis meta Requirement 20): eight independent, read-only hat reviews (Architect, Security Architect, GRC, Network, Cloud Security, UI/UX, DBA, QA/Test), each told to verify against real code or command output, to cite file:line and to map every finding to STRIDE. The workers ran experiments only in scratch copies. 83 raw findings were merged into 14 issue groups before any fix.

Severity scale: Critical, High, Medium, Low, Info. STRIDE: S spoofing, T tampering, R repudiation, I information disclosure, D denial of service, E elevation of privilege.

## Groups

| Group | Severity | Issue | Findings | Open |
|---|---|---|---|---|
| E1: Queued-edit replay writes stale values over the game's newer save | HIGH | #251 | 10 | 10 |
| E2: Edits dropped or held by a rebase are not reported; the queue cannot be inspected | MEDIUM | #252 | 6 | 6 |
| E3: Backups: the prune can delete the pre-session backup; the throttle trusts any file | MEDIUM | #253 | 7 | 7 |
| E4: No persistent record of writes; bulk actions lost their confirmation | MEDIUM | #254 | 4 | 4 |
| E5: Documents still describe the removed Review & save; a control removal merged without review | MEDIUM | #255 | 7 | 0 |
| E6: Repository settings and CI configuration | MEDIUM | #256 | 10 | 10 |
| E7: Save lifecycle: locking, gates and timeouts | MEDIUM | #257 | 10 | 10 |
| E8: Give and stack sizes: remaining gaps | MEDIUM | #258 | 4 | 4 |
| E9: Local data exposure and the page guard | LOW | #259 | 5 | 5 |
| E10: Embedded game-table datasets have no provenance notice | MEDIUM | #260 | 1 | 0 |
| E11: Review-era code kept alive | LOW | #261 | 2 | 2 |
| E12: Test gaps in the write path | MEDIUM | #262 | 4 | 4 |
| E13: Repository hygiene: agent worktrees are untracked and not ignored | LOW | #263 | 3 | 3 |
| E14: Editor UI: remaining polish | MEDIUM | #264 | 10 | 9 |

## E1: Queued-edit replay writes stale values over the game's newer save

Severity HIGH. Issue #251.

A queued edit that stored a number computed from the old value (Solari +250, a stack set, XP) is replayed over the game's newer file and overwrites what the player earned meanwhile (test: 5,100 became 350 instead of 5,350). A failed edit does not stop later edits that address its rows; a journal that outlives a write is replayed again (duplicate items); ids, slots and the id sequencer are replayed literally; rebase, discard and restore are not serialized with the flush.

**Recommended fix.** Replay the operation, not the statement: re-run each edit against the newer file with the new values, skip the dependents of a failed edit, remove the journal only after the write and under the flush lock, serialize rebase, discard and restore with the flush, check slot capacity and the id sequencer.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| QA-1 | HIGH | T, D | `internal/save/flush.go:71-77`, `journal.go:292-342` (replay checks only the row count, ... | A queued edit that stores a number the editor computed from the old value (Solari +250, set stack, XP...) is replayed over the game's newer save and silently overwrites what the player earned in between. | open |
| DBA-1 | HIGH | T, R | internal/save/journal.go:292-356 (replayLocked/replayOp), flush.go:71-85 | When a replayed edit fails, later edits that address its rows by id are still replayed and land on the game's unrelated rows; the result is committed in the same Flush | open |
| DBA-2 | MEDIUM | T | journal.go:312-342; ops.go:332 (top-up), :803 (AddSolari), augment.go:124 (stats rewrite) | Replay re-runs absolute values computed from the stale working copy: it silently overwrites what the game changed on the same row (lost update) | open |
| DBA-3 | MEDIUM | T | ops.go:388-407 (id sequencer), journal.go:318-333 | The literal `next_id` and item id are replayed onto the newer file: the sequencer is moved backwards, and the id may belong to a game row | open |
| DBA-5 | MEDIUM | T | ops.go:360-373 via journal replay; schema.sql:284-295 | Replay inserts the stale slot (`position_index`) and does not re-check capacity: two items in one slot, item count above max_item_count | open |
| GRC-5 | MEDIUM | T, R | save/journal.go:205-224, save/save.go:597-598 | A journal that survives a successful write replays already-written edits (duplicate items). Reproduced. | open |
| ARCH-2 | MEDIUM | T | `internal/save/save.go:587-598`, `internal/save/journal.go:210-214, 249-266` | A journal that outlives a successful write is replayed onto a file that already holds the edits (duplicates) | open |
| DBA-10 | LOW | T, D | journal.go:98-105, 131-146, 219-243 | Journal fidelity and degraded modes: nil []byte becomes an empty blob, invalid UTF-8 is rewritten, float32 is widened, any ';' strands the queue, write errors are swallowed, no fsync | open |
| DBA-11 | LOW | T | journal.go:270-310 | Rebase is not under flushMu and releases s.mu between replayed ops; the journal file is deleted before replay re-creates it | open |
| ARCH-3 | MEDIUM | T, D | `internal/save/journal.go:270-298`, `internal/save/save.go:498-506, 625`, ... | Rebase, Discard and Restore are not serialized with Flush; replay drops `s.mu`; Discard can be undone by an in-flight replay | open |

## E2: Edits dropped or held by a rebase are not reported; the queue cannot be inspected

Severity MEDIUM. Issue #252.

A successful automatic write erases the notice of the edits the rebase dropped; with the journal off, queued edits can never be written once the game saved, yet the banner promises it; what is queued shows only as a hover title; two banners repeat one reason in raw tool text; success toasts do not say whether an edit was queued or written.

**Recommended fix.** Keep the conflict list until the player dismisses it, list the queue (what, when, why held), merge the banners, and make every toast say written or queued.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| ARCH-1 | MEDIUM | R, T | `internal/save/flush.go:71-84`, `internal/save/save.go:598`, ... | Edits dropped by the automatic rebase are never reported: the successful write erases the notice | open |
| DBA-4 | MEDIUM | R | save.go:598 (commit -> dropJournal), journal.go:212, web/static/app.js:268-282 | Conflicts found by a Flush-driven rebase are never shown: dropJournal clears `restored`, and the page only reads `o.restored` | open |
| ARCH-9 | LOW | D | `internal/save/journal.go:270-277`, `internal/web/static/app.js:278` | With the journal off, queued edits can never be written once the game has saved; the banner still promises they will be | open |
| UX-30 | MEDIUM | Usability | `app.js:285-287, 324`; header `#pending` | The queue cannot be inspected.** What is queued appears only as the `title` of "1 queued" (hover): "solari +10000", lower-case internal wording. The Discard dialog ("The changes that are waiting for the game to close are dropped") lists nothing; neither does the banner. Verified on the fixture. | open |
| UX-31 | MEDIUM | Polish, User-friendliness | `app.js:278-279`; screen (fixture) | The two banners stack with the same reason twice and raw tool text: "1 change is queued. could not check whether the game is running (exit status 1: ERROR: Call cancelled). They are written ... Changes cannot be written. could not check ... (exit status 1 ...). If this keeps happening, report this ... | open |
| GRC-15 | LOW | R, I | web/static/app.js:39, 46, 913, 922 | About 41 success toasts carry no queued hint; "Teleport queued" appears when the edit was written. | open |

## E3: Backups: the prune can delete the pre-session backup; the throttle trusts any file

Severity MEDIUM. Issue #253.

Backups are kept at one per 2-minute burst and the newest 60, so the pristine pre-editor backup is the first to be evicted; the throttle trusts the mtime of any file in the folder (a future-dated file suppresses every backup); the backup an automatic write reports may not be of the file it replaced; the retention test cannot fail when pruning deletes the newest backup.

**Recommended fix.** Keep the first backup of a session (and the oldest ever) out of the prune, key the throttle on the editor's own backups, take the backup of the file actually replaced, and make the retention test fail on a deletion of the newest backup.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| SEC-1 | MEDIUM | T, D (loss of recovery) | internal/save/flush.go:13-16, 101-112, 126-146; save.go:580 | The automatic backup prune keeps only the newest 60 files and does not protect the first, pre-editor backup. A long or repeated editing history silently deletes the only copy of the never-edited save. | open |
| SEC-2 | LOW | T, R | internal/save/flush.go:101-112 (backupIfStale), save.go:541-607 | Backup is reused for 2 minutes, but Flush rebases onto a newer game save first. The file that gets overwritten is then not the state the reported "backup" holds. | open |
| GRC-4 | MEDIUM | R, D | save/flush.go:14-15, 101-146; README.md:49, 75 | Backup policy changed to one per 2-minute burst and a 60-file cap; the next write deletes 41 of the operator's 101 existing backups, including the 2026-10-07 originals. README still says every save is backed up. | open |
| ARCH-7 | LOW | R | `internal/save/flush.go:104-112, 140-145`, `internal/save/save.go:579-583` | The "backup" reported by an automatic write may not be of the file it replaced; count-based pruning can discard the session's original | open |
| DBA-7 | MEDIUM | R, D | flush.go:102-124, save.go:514-533 | Backup throttle trusts the mtime of any file in the folder: a future-dated or junk file suppresses all backups; no byte comparison with the pre-edit file; skipped after a rebase | open |
| DBA-9 | LOW | R | flush.go:127-146 | pruneBackups evicts the oldest file by mtime after 60: the pristine pre-session backup is the first to go; also deletes any file in the folder | open |
| QA-3 | MEDIUM | T, R | `flush.go:126-146` (`pruneBackups`), test `flush_test.go:105-135` | The backup-retention test cannot fail when pruning deletes the newest backup (the restore point just taken). Mutant survives. | open |

## E4: No persistent record of writes; bulk actions lost their confirmation

Severity MEDIUM. Issue #254.

Since writes happen as edits are made there is no durable record of what was written, when and under which backup; the bulk base actions write on one click; inline edits commit when the field loses focus with nothing to review or undo.

**Recommended fix.** An append-only write log (time, edit, backup name, outcome) shown in the app; a confirmation for bulk actions; an undo for the last edit through its backup.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| GRC-3 | MEDIUM | R | save/save.go:597, save/flush.go:32, ops/ops.go:140 | No persistent record of what was written, when, or under which backup. | open |
| GRC-2 | MEDIUM | T, R | web/static/app.js:1010-1015, CHANGELOG.md:78 | Bulk base actions lost their only confirmation when the review pane went; they now write on one click. | open |
| SEC-14 | INFO | R | CHANGELOG.md:9; web/server.go:372-386 | The change from "stage then review" to "write on every successful POST" removed a human-in-the-loop second factor for every write route. Compensating controls hold, but this should be stated as an accepted risk in SECURITY.md. | open |
| UX-29 | MEDIUM | Usability (safety) | `app.js:1110-1114` (`change` handlers), `:41-66` (`act`, `editInput`) | Inline edits commit as soon as the field loses focus, and there is nothing to review or undo.** Stack, Grade, Reputation, Database cells, Config values and skill ranks write on change. The toast is "Item updated" with no before→after. The only undo is a whole-file restore under Database > Save ... | open |

## E5: Documents still describe the removed Review & save; a control removal merged without review

Severity MEDIUM. Issue #255.

The security policy names the review pane as the save-write control; at least 11 changelog entries and several docs still describe it; the SQL page text and the live-test protocol tell testers to press Review & save; the change that removed the control was classed High by its author and merged with no review and no replacement control in the audit register.

**Recommended fix.** Rewrite the policy and the register for the write-as-you-go model, correct the changelog and docs, and review control removals as a rule.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| GRC-1 | MEDIUM | R, T | SECURITY.md:25, :37 | The security policy still states the removed review pane as the save-write control. | fixed (#275) |
| GRC-13 | LOW | R | CHANGELOG.md:13, 16, 19, 21, 33, 42, 48, 78, 82, 84, 109 | The [Unreleased] changelog still describes the review pane in at least 11 entries, contradicting entries 9 and 91. Same defect class as L2 GRC-7. | fixed (#275) |
| GRC-14 | LOW | R | docs/live-test-protocol.md:88, docs/continuation-prompt.md:15/33, docs/README.md:3, ... | Other docs and a code comment still describe the review pane; the audit register carries no replacement statement for F-06/F-10. | fixed (#275) |
| UX-28 | HIGH | Accuracy | `app.js:867` (SQL card), `:941` (delete confirm), `:1032`; `docs/live-test-protocol.md:88` | Text still describes the Review & save model that f977c84 removed.** The SQL page says "Nothing reaches game.db until you press **Save to game**, which backs up first" (read back from the DOM; there is no such button). The same card's confirm says it is written "at once". The item-delete confirm ... | fixed (#275) |
| UX-40 | LOW | Accuracy | `docs/live-test-protocol.md:88` | The live-test protocol tells testers to "Press Review & save, then Save to game". | fixed (#275) |
| QA-16 | INFO | I | docs | Documents still describe the removed Review & save (`docs/live-test-protocol.md:88`, `docs/design/feature-inventory.md:658,661`, `tools/featureinv.py:84`). | fixed (#275) |
| GRC-12 | MEDIUM | R | PR #240, issue #238, docs/audit/ | A change that removes an audit-register control was classed High by its author and merged with no review, no issue analysis and no register update. | fixed (#275) |

### Replacement statement for the removed review pane (F-06, F-10 of the 2026-09-30 register)

The review pane was removed by #240 (issue #238). The 2026-09-30 register named it as the control for two findings: F-06 (no way to see what
will be written) and F-10 (Save reachable only after a review, with confirmations that state the consequence). This is what plays each role
now, with the code that does it and the test that pins it, and what is not replaced. Dated 2026-10-22; it is a statement of the code at
that date, not a new claim of coverage.

| Role the review pane played | What plays it now | Where | Pinned by | Not replaced (finding) |
|---|---|---|---|---|
| No write happens unless a game-running check passed | The check runs at every write, not once at Save: no single-player session, the check itself worked, the location confirmed, the save not read-only. A failed check queues the edit instead of writing it. | `Flush` in `internal/save/flush.go` (`GameStateFor`, `blocked`); `commit` in `internal/save/save.go` | `TestCommitRefusesWhenTheGameMayBeWriting`, `TestFlushQueuesTheEditWhileASinglePlayerSessionIsActiveAndWritesItWhenItEnds` | Gates read a cached process list (ARCH-4, #257) |
| No write over a file the game changed | Changed-on-disk check, then queued edits are put back onto the newer file and written only if every one still applies; otherwise the whole queue is held and nothing is written. | `Flush`, `rebaseAtomic`; `commit` | `TestFlushPutsTheEditsBackOntoAFileTheGameSavedSince`, `TestAQueueThatConflictsWithTheGamesNewerFileIsHeldWholeAndNothingIsWritten` | Replay can write stale values; dropped edits are not reported (#251, #252) |
| The file written is the file intended | Integrity check of the working copy, an encode and decode round trip before the write, and a read back that is compared byte for byte after it; a mismatch restores the previous bytes. | `commit` in `internal/save/save.go`; `installBlob` in `internal/save/durable.go` | `TestReadBackMismatchRestoresThePreviousBytes` | |
| A way back | A backup is taken before the first write of a burst (one covers edits made within two minutes, only while the file is still the one the editor wrote); the newest 60 automatic backups are kept, and always the oldest, the first of the session and the one just taken; files the editor did not name itself are never counted or removed; Database > Save backups restores one after a typed word. | `backupIfStale`, `pruneBackups` in `internal/save/flush.go`; `Restore` | `TestABackupIsNotReusedForAFileTheGameSavedSince` | The prune keeping the pre-session backup is fixed in #273 (E3 holds the rest) |
| See what will change before it is written (F-06) | Not replaced before the write. Each edit carries a description in the player's words; the page lists the edits that are waiting (banner and the Discard question), and the toast and `Flush.Ops` name what was written. After the fact, `tabr-tau diff <before> <after>` compares a backup with the file (read only, identifiers masked). | `Ops` descriptions; `queueLine` in `internal/web/static/app.js`; `cmd/tabr-tau` | `TestFlushWritesTheEditAtOnceWhenTheFileCanBeWritten` | No persistent record of what was written (GRC-3, #254) |
| Confirm that a far-reaching action is meant (F-10) | In-app questions that state the consequence stay for destructive actions; a typed word stays for a raw SQL write and for restoring a backup; the bulk base actions (refill power and water, repair every base, clear sand) ask on their own button (#278). | `ask()` in `internal/web/static/app.js` | `web-tests/ui.hostile.test.mjs` (typed-word tests) | The status of GRC-2 is in its row (#254) |
| Edits made while the game runs are not lost or applied blindly | A queue kept in a private journal, listed on the page, written by itself when the file can be written, with Write now and Discard queue. | `Flush`, the journal in `internal/save/journal.go` | `TestFlushQueuesTheEditWhileASinglePlayerSessionIsActiveAndWritesItWhenItEnds` | What happens to a dropped or stuck edit is tracked in #252 (UX-30, UX-31) |

The roles that are not fully replaced are findings of this register (GRC-3, ARCH-4, the E1 to E3 groups), not new ones. How a change that
removes a control of this kind is handled from now on: [`docs/control-removal.md`](../control-removal.md).

## E6: Repository settings and CI configuration

Severity MEDIUM. Issue #256.

The windows and ui jobs are not required checks; SECURITY.md sends reporters to private vulnerability reporting, which is switched off; a reusable workflow referenced at @main runs inside the required checks and receives the scan token, and its jobs have no timeout; PSScriptAnalyzer is installed with no retry or cache; protection is sound against accident but soft (CODEOWNERS is not enforced); nothing enforces SHA pinning; a personal address is the author email on 30 public commits; there is no release pipeline, checksum or provenance.

**Recommended fix.** Add windows and ui to the required checks, turn on private vulnerability reporting, pin the reusable workflow to a commit, set timeouts, and decide the release process.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| CLD-4 | MEDIUM | T, R | `gh api repos/Project-Arrakis/tabr-tau/branches/main/protection` -> ... | `windows` and `ui` jobs (`ci.yml:50-67`, `112-132`) are not required checks; Windows is the only platform the product runs on. | open |
| CLD-5 | MEDIUM | R, I | `SECURITY.md:5-8` vs `gh api ... | SECURITY.md tells reporters to use GitHub private vulnerability reporting, which is switched off. The documented fallback is a public issue. | open |
| CLD-6 | MEDIUM | T, E | `ci.yml:177-183` (`...reusable-security-scan.yml@main`); org-wf ... | A mutable `@main` reusable workflow in an unprotected branch runs inside tau's required checks and receives `SEMGREP_APP_TOKEN`. One stolen token or one bad push changes CI for all repos. | open |
| CLD-7 | MEDIUM | D | `reusable-security-scan.yml:58-112` (three jobs, no `timeout-minutes`); `ci.yml:177-183` | The reusable jobs have no timeout (GitHub default is 360 min) and the caller cannot set one. They are three of the nine required checks. This is the same class as the govulncheck 40-minute hang. | open |
| CLD-8 | LOW | D, T | `ci.yml:96-99` | PSScriptAnalyzer is installed from PSGallery with a pinned version but no retry, no cache and no integrity check; the job is a required check. | open |
| CLD-10 | LOW | T, R | protection: `required_approving_review_count:0`, `strict:false`, ... | Protection is sound against accidents (admins enforced, linear history, no force-push) but is soft: CODEOWNERS is inert, branches need not be up to date, commits need not be signed. | open |
| CLD-11 | LOW | E | .github/workflows/ci.yml | Nothing enforces the convention that actions are pinned to a commit; fork-pull-request approval is relaxed after a contributor's first merge. | open |
| CLD-12 | LOW | I | `git -C tabr-tau log --format=%ae` -> 30 commits by `ronald.yacketta@gmail.com` in public ... | The personal gmail address is the author email on 30 public commits (the others use the noreply address). | open |
| CLD-14 | INFO | R | no releases, no tags, no release workflow (`releases` -> `[]`, `tags` -> `[]`); ... | No release pipeline, checksum, signature or provenance exists. tau documents this honestly; the `test-builds` zips are local and unsigned. | open |
| CLD-15 | INFO | R | ci.yml | GitHub-side stale job status on main commits (all steps of the job show success). Not a repository defect. | open |

## E7: Save lifecycle: locking, gates and timeouts

Severity MEDIUM. Issue #257.

The write gates use the 3-second process cache while the status poll bypasses it (the flag is inverted); Open deletes temp files before the single-instance lock is taken; the save lock is held across process spawn, hashing, encode, backup and rename retries; the single-player gate is checked once, well before the rename; the on-disk change check stats the file after the blob was read; the schema fingerprint covers triggers, views and virtual tables but not table definitions or applied patches; write-console SQL has no timeout and a Give of a one-per-slot item is quadratic; a flush longer than the 2-minute write timeout would show a network error for a successful save.

**Recommended fix.** Fix the fresh flag, take the lock before the cleanup, shorten the lock's span, re-check the gate just before the rename, stat before the read, and bound the SQL and Give work.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| ARCH-4 | MEDIUM | T | `internal/save/spsession.go:170-171`, `internal/save/gameproc.go:104-110` | The `fresh` flag is inverted: write gates use the 3-second process cache, the 5-second status poll bypasses it | open |
| ARCH-5 | LOW | D, T | `internal/save/save.go:188`, `internal/save/durable.go:65`, `cmd/tabr-tau/main.go:111-117` | `Open` deletes `*.tabr-*.tmp` before the single-instance lock is taken | open |
| DBA-12 | LOW | D | cmd/tabr-tau/main.go:111-117, save.go:188, durable.go:65 | `Open` (which deletes `game.db.tabr-*.tmp`) runs before `LockSave`; a second launch can delete the first editor's in-flight temp file | open |
| ARCH-8 | LOW | D | `internal/save/save.go:541-607, 485-495`, `internal/save/durable.go:48-57` | `s.mu` is held across process spawn, hashing, encode, backup and rename retries (up to ~3 s) and across 5 s console queries; polls stall | open |
| DBA-15 | INFO | T | save.go:541-607, durable.go:87-117 | The single-player gate is evaluated once, well before the rename; with the game writing game.db about every 20 s, a missed marker loses the edit silently | open |
| DBA-16 | INFO | T | save.go:149-177 | The schema fingerprint covers triggers, views and virtual tables only; table DDL and applied_patches are not compared at open | open |
| DBA-17 | INFO | T | save.go:284-291 | noteDisk stats the file after the blob was read (about 100 ms later on the real save): ChangedOnDisk can say "unchanged" for a file newer than the loaded one | open |
| SEC-11 | LOW | D | internal/ops/db.go:277-290; ops/ops.go:314-352, 441; save/mutate.go:201-203 | Unbounded work on the single save lock: write-console SQL has no timeout, and a Give of a non-stacking item with quantity up to 1,000,000 loops one insert per unit with a slot scan each when the inventory has no capacity limit. | open |
| NET-13 | INFO | - | internal/web/server.go:372-386, listen.go:130 | Write-as-you-go now runs the whole `Flush` (backup, encode, write) inside the POST handler, bounded by `WriteTimeout` 2 min (the comment says it is for exports). A very large save that takes longer would have its response cut after the file was written; the page would show a network error for a ... | open |
| QA-13 | LOW | D | `ops/ops.go:339-350` (`for left > 0`), `giveInTx` :354 | A one-per-slot item (stack size 1) given in a container with no slot limit costs a quadratic amount of work: 500 pieces 0.23 s, 1,000 0.92 s, 2,000 4.96 s; the argument limit is 1,000,000 and the save lock is held throughout. | open |

## E8: Give and stack sizes: remaining gaps

Severity MEDIUM. Issue #258.

A stack edit ignores the stack size (1e9 in one stack against the game's 50,000) and only touches the first stack; a new item clones its stats from an arbitrary existing item of the template (contract id, fuel, ammo, durability); AddSolari accepts a wallet stack past the 50,000 the item tables give; the top-up of a stack of another grade and the 500/501 boundary are untested.

**Recommended fix.** Use the catalog stack size in every path that sets or adds, build new item stats from the template defaults, and add the missing tests.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| DBA-6 | MEDIUM | T | ops.go:785-820 (AddSolari) | Ignores catalog.MaxStack (1e9 in one stack vs the game's 50,000) and only touches the first stack, so removal under-removes while reporting success | open |
| DBA-8 | LOW | T | ops.go:376-381 | A new item clones `stats` from an arbitrary existing item of the template (ContractUid, fuel, ammo, durability, swatch) | open |
| QA-11 | LOW | T | `ops/ops.go:314-350, 583-640 (cap check :624)` | Stack-size feature gaps: a stack of another grade being topped up (mutant), the 500/501 boundary of the SetItem cap, and the "not enough room" message are untested. | open |
| QA-12 | LOW | T | `ops/ops.go:785-806` vs `:624` | `AddSolari` accepts a wallet stack past the 50,000 the item tables give for one slot (900,100 accepted) while `SetItem` refuses the same stack above 50,000. | open |

## E9: Local data exposure and the page guard

Severity LOW. Issue #259.

Decoded plaintext copies of the save (with account and platform ids) are written to %TEMP% and removed only on a clean exit; tables-view cell edits address a row by rowid and send no expected old value; the pending-edit journal is a plaintext file under the roaming profile; the session cookie is renamed per port but a browser still sends it to every port of the address; the WebView2 navigation restriction is a script click hook only.

**Recommended fix.** Delete the temp copies at start and on every exit path, send the expected old value with a cell edit, keep the journal in the local profile, and add NavigationStarting and NewWindowRequested handlers.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| SEC-3 | LOW | I | internal/save/save.go:195-220, 234, 247; readonly.go:27; durable.go:65 (cleanStaleTemps), ... | Decoded plaintext copies of the save (with account/platform ids) are written to `%TEMP%` and are only removed by a clean `Close`. A crash or kill leaves them forever, and nothing sweeps them at start. 27 such files are in the user's `%TEMP%` now. | open |
| SEC-7 | LOW | T | internal/ops/db.go:182-257; web/static/app.js:1114; save/flush.go:71-84 | Tables-view cell edits address a row by `rowid` and send no expected old value. With the file now reloaded or rebased automatically, a stale view can overwrite a different row. | open |
| SEC-12 | INFO | T, I | internal/save/journal.go:312-342, 233-241; web/server.go:74-84 | The pending-edit journal is a plaintext file under `%APPDATA%` (roaming profile). Replay executes journal statements without the SQL-console vetting or the locked-table list. Only a local attacker can plant one, which SECURITY.md scopes out. | open |
| NET-5 | LOW | I, S | internal/web/server.go:130-160, 443 | Residual of the 2026-10-15 NET-2: renaming the cookie per port does not stop the browser sending it to every other port on the same host (RFC 6265 s8.5). In `--web` mode another loopback listener the user browses to receives a replayable session. | open |
| NET-8 | LOW | T, E | internal/gui/gui_windows.go | The WebView2 navigation restriction is a script click hook only (no NavigationStarting or NewWindowRequested handler). Latent: no sink today. | open |

## E10: Embedded game-table datasets have no provenance notice

Severity MEDIUM. Issue #260.

Two embedded datasets taken from the game's own tables carry no provenance or notice, against audit finding F-20.

**Recommended fix.** Add a notice file with the source, the game build and the licence position.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| GRC-6 | MEDIUM | R (provenance) | catalog/stacks.json, catalog/customization_unlocks.json, notices/THIRD-PARTY-NOTICES.md | Two embedded datasets taken from the game's own tables have no provenance or notice, against audit F-20 and against issue #61's "licence/ToS check first". | fixed (#279) |

## E11: Review-era code kept alive

Severity LOW. Issue #261.

The pristine baseline copy is built on every load and rebase but used only by tests; Commit(force) has no production caller; stale comments remain.

**Recommended fix.** Remove the baseline copy and the unused path, or keep them behind test builds.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| ARCH-6 | LOW | T (surface) | `internal/save/save.go:246-248, 267-272, 348-357, 537` | Review-era code kept alive: pristine baseline copy and `Commit(force)` have no production caller; stale comments | open |
| QA-17 | INFO | R | `save/save.go:247` (`openCopy(... "tabr-tau-orig-*")`) | The pristine baseline copy is still built on every load/rebase but is now used only by tests; the production guarantee it served (show exactly what changed) went with the review pane, with no replacement check. | open |

## E12: Test gaps in the write path

Severity MEDIUM. Issue #262.

The error, conflict and warning branches of Flush are untested (the mutants that ignore the rebase error or drop the warning survive); replay of a cascading delete and the journal size limit are untested (MutateCascade has 0% coverage); the flush route and the flush-after-every-POST hook have no Go test of their own; the edit outcome is not in the response and every error is a 400.

**Recommended fix.** Add the tests proposed in the QA hat report (verified against the code) and put the outcome in the response.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| QA-2 | MEDIUM | T | `internal/save/flush.go:63-66, 73-76, 81-83, 86-91, 96` (uncovered, 75.7% of `Flush`) | The error, conflict and warning branches of Flush are untested: mutants "ignore the rebase error" and "drop the read-back warning" survive the whole suite. | open |
| QA-4 | MEDIUM | T | `journal.go:344-347`, `mutate.go:197` (`MutateCascade` 0.0%), `ops.go:666` (`DeleteItem`) | Replay of a cascading delete (DeleteItem) and the journal size limit are untested; the mutant that loses the cascade flag survives. My 25-line test kills it. | open |
| QA-15 | INFO | R | `web/server.go:380-383, 640-642` | The new `/api/save/flush` route and the flush-after-every-POST hook have no Go test of their own, and the POST response never says whether the edit was written or queued. | open |
| ARCH-15 | INFO | R | `internal/web/server.go:372-386, 352-363, 726-780`, `app.js:45` | UI/server contract: edit outcome is not in the response; every error is a 400; startup relies on ordering | open |

## E13: Repository hygiene: agent worktrees are untracked and not ignored

Severity LOW. Issue #263.

The folder of agent worktrees (full source copies, 72 MB) is untracked and not ignored, with stale registered worktrees, so a git add -A would commit repository copies.

**Recommended fix.** Ignore the folder and prune the stale worktrees.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| GRC-18 | LOW | T, I | .claude/ (untracked), `git worktree list` | `.claude/` is untracked and not ignored, with four stale registered worktrees, so the tree is not clean and `git add -A` would stage embedded repos. | open |
| CLD-13 | LOW | I | `git status` -> `?? .claude/`; `.gitignore` has no `.claude` entry; `.claude/worktrees/*` ... | `.claude/` (agent worktrees) is untracked and not ignored. A `git add -A` would commit repository copies and any local agent settings. | open |
| QA-18 | INFO | - | .claude/worktrees | The untracked agent worktrees contain full duplicate copies of the repository and would be committed by git add -A. | open |

## E14: Editor UI: remaining polish

Severity MEDIUM. Issue #264.

Warning banners lost their colour (fixed in #244); dialogs lack aria-labelledby and the page behind is not inert; counts have no stated meaning; the Live Map tab is a weaker second map with raw ids; Repair exists in three tabs; Specialization is a dead end for a new save; the sticky header and banner take 200 px of an 800 px screen at 390 px; empty states are generic; wording is mixed; the light-theme muted text is 4.08:1.

**Recommended fix.** Covered by the editor UX pass (in progress) together with whatever it leaves.

| ID | Sev | STRIDE | Location | Finding | Status |
|---|---|---|---|---|---|
| UX-27 | HIGH | Accuracy, Polish | `app.css:201, 205, 209` (`var(--warn)` is never defined; `grep --warn` finds only these ... | Every warning banner and `.warn` pill lost its colour.** Since the round-2 skin (609dc04) `.warn,.info` use `var(--warn)`; the editor's `:root` has no `--warn`, so `border-left` and `background` are invalid at computed time. Measured on the live banner: `background: rgba(0,0,0,0)`, ... | fixed: tau #244 |
| UX-32 | MEDIUM | Usability (a11y) | `app.js:309, 1139-1153`; `app.css:53`; `app.js:388` etc. | Dialogs: `role=dialog aria-modal` but no `aria-labelledby`; the page behind is not `inert` and scrolls (`body.noscroll` exists in CSS and is never applied). The Tab trap and focus return do work. Filter boxes in Inventory, Journey, Research, Crafting, Customizations still ignore Enter and need the ... | open |
| UX-33 | MEDIUM | Accuracy, User-friendliness | `app.js:385, 388, 505, 554, 569, 593, 348-349, 785` | Counts without a stated meaning.** Inventory reads "2 of 2 - 2 / 5 slots used". Journey: "972 of 972 (showing the first 400, narrow the filter)". Summary "Skill points 9 / 70" is unspent over total with no label; "Health 85.0 (max at least 150)". Cargo "n / ?". "N of N" appears even when nothing is ... | open |
| UX-34 | MEDIUM | Polish, Redundancy | `livemap.js:162, 240-241, 260, 295, 304`; `app.js:137` | The Live Map tab is a weaker second map: raw type ids in the hover and the table. | open |
| UX-35 | MEDIUM | Redundancy, User-friendliness | `app.js:401, 472-473, 779, 464-478, 372-373, 346-353` | Prior clutter is still there.** Repair exists in the Inventory, Admin and Vehicles tabs (and Admin shows "Repair Faction" twice per row: the label and the disabled button). Five disabled "Not in tabr-tau yet" stubs fill Admin ("Repair Quests", "Wipe Inventory", "Reset Progression", "Spawn", "Repair ... | open |
| UX-36 | MEDIUM | Usability | `app.js:411-428` (still as in prior #12) | Specialization is a dead end for a new save (all tracks show "?" and the XP buttons are disabled until the player earns XP in game and reloads); "Advanced: set a raw row" is open with no confirm. Verified on the fixture. | open |
| UX-37 | LOW | Polish | `app.css` (no `@media`), `index.html:10-16`; measured at 390 px | The sticky header is 99 px (3 rows) and the sticky banner about 100 px more at 390 px, 200 px of an 800 px screen. No breakpoints. At startup the page is a header with an empty path pill and nothing else until the first fetch ends (no loading state; seen for about 3 s on this busy machine). | open |
| UX-38 | LOW | User-friendliness | `app.js:204, 859, 336`, `:785`, many | Empty states are a generic "Nothing here." (Bases shows it twice: card and "No base in this save yet."); Database shows "0 rows — showing 1–0" and the search placeholder is just "search"; the Summary "Level" tile is blank where others show "-". | open |
| UX-39 | LOW | User-friendliness | `app.js:939, 910-912, 788, 482, 801, 808, 804` | Wording still mixed: "Hydration 70.1 -> 100." vs "XP 1 to 2"; Title Case card names; raw column headers (`time_stored`, `reason`, `vendor_id`, `id`); Landsraad Start and End are raw integers; reward lines show template ids; Vendor table shows vendor ids. | open |
| UX-41 | LOW | Polish (a11y) | `app.css:2` | `--mut` on `--panel2` in the light theme is 4.08:1 (kv labels, tags, `.mut` inside kv); below 4.5. Elsewhere muted text is 4.9 to 5.4. No favicon; the title never changes per tab. | open |

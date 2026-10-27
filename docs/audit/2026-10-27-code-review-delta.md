# Code review of the changes since 2026-10-23 (2026-10-27)

Part of the Layer 3 review, [#206](https://github.com/Project-Arrakis/tabr-tau/issues/206). Range: `e92108e` (the last commit of 2026-10-23) to `1d537cd` (main on 2026-10-26), 45 commits, 150 files, about 11,500 lines added.
Reviewed by reading the diff hunk by hunk together with the functions around it, and by running small scratch tests (not committed) where a claim could be tested cheaply. Nothing was fixed.

## Result

| Severity | Count | Issue |
|---|---|---|
| Critical | 0 | |
| High | 0 | |
| Medium | 2 | [#441](https://github.com/Project-Arrakis/tabr-tau/issues/441) (CR-1), [#442](https://github.com/Project-Arrakis/tabr-tau/issues/442) (CR-2) |
| Low | 9 | [#443](https://github.com/Project-Arrakis/tabr-tau/issues/443) (CR-3 to CR-11) |

The care package, the Landsraad fills, the write gate, the hydration fix and the route guard held up under the checks in "What was checked and found sound". The two medium findings are an automatic pass that can keep reloading the save, and a settings file that is replaced without a sync and that now holds user-built kits.

## Findings

All findings are **open**.

### CR-1 (medium, #441): the waiting automatic pass reloads the whole save every 3 seconds when its switches are turned off while it waits

`internal/web/server.go`: `RunStartupTasks` (about lines 940-980), `retryWaitingPass` (about lines 905-920).

Scenario: the game is running when the editor opens with the master switch on. `RunStartupTasks` sets `autoWaiting = true` and `RunAutosave` calls `retryWaitingPass` every 3 seconds. The person switches the master switch off, or every job off, while it waits. When the game closes, `retryWaitingPass` sees `!Blocked`, calls `Save.DiscardIfClean()` (decode the save, build a new working copy, schema comparison, foreign-key scan) and `RunStartupTasks`, which returns at the master-switch or the all-jobs-off early return. `autoWaiting` is cleared only after those returns, so the next tick does the same: the whole save is decoded and a fresh temp copy written every 3 seconds, with the save's lock held, for as long as the editor runs.

Evidence: scratch test in `internal/web` (not committed). `autoServer` with the master switch on, `save.GameAtMenuForTests()`, `RunStartupTasks()`, `settings.Save` with the master switch off, restore the game check, three `retryWaitingPass()` calls: `autoWaiting` is still `true`. The existing tests cover jobs that are off at the first call, and a switch changed while waiting only when the master switch stays on.

Suggested fix: clear `autoWaiting` at both early returns of `RunStartupTasks`; add the test.

### CR-2 (medium, #442): `settings.json` is replaced without a sync, and an empty file reads as "nothing to lose"

`internal/settings/settings.go`: `save` (about lines 179-217), `loadStrict` (about line 139).

Scenario: a crash or power loss shortly after `os.Rename` leaves `settings.json` zero-length or partly written. `loadStrict` returns the defaults for an empty file without an error, and the next `Update` overwrites it. Since #423 the file holds up to 256 KiB of care package kits built by hand, plus the learned track numbers and the automatic-refill switches; all of it is lost with no message. The save is installed with an fsync (`internal/save/durable.go`); this file is not.

Evidence: code reading (no `Sync()` in the package; the empty-file rule). Not reproducible in a test.

Suggested fix: `tmp.Sync()` before the close; keep one copy of the previous file or report an empty file when kits were expected; add a test.

### CR-3 to CR-11 (low, #443)

| ID | File | Finding | Suggested fix |
|---|---|---|---|
| CR-3 | `internal/web/kits.go`, `app.js` `kitApply` | Apply goes by kit name: the dry run the person confirmed is of one version of the kit, the apply reads the kit again. An edit, delete and re-make, or import in another page between the two requests applies a different kit than the one described. | The preview answers with a hash of the kit; apply sends it back and refuses on a difference. |
| CR-4 | `app.js` Fill card (`bloodtip`, `overview.fill`) | "The fill of blood sacks has not been checked in the game yet" is now only in a hover tooltip and a closed "About this"; the action does not ask first. | Say it in the result line of the fill. |
| CR-5 | `app.js` (care package `New kit`/`Edit`, specialization rows, other disabled controls) | The reason a control is disabled is only in `title`; a disabled button cannot be focused, so keyboard and screen-reader users never reach it. | Draw the reason as text, or use `aria-disabled` with `aria-describedby`. |
| CR-6 | `internal/ops/landsraad.go` `SetTaskProgress` (about line 151) | Only `amount < 0` is refused; a finite amount up to 1e308 is written to a REAL column. The fill routes are bounded. | Refuse an amount above a bound. |
| CR-7 | `internal/web/kits.go` `kitArg` (about lines 56-58) | A kit of the wrong type is answered with the decoder's message, which names Go types; `kits.Parse` has a `friendly` rewrite that `kitArg` does not use. | Export `friendly`, use it. |
| CR-8 | `app.js` `lsFill` (about line 1764) | `l.term.term_id` is read unguarded; a save with Landsraad tasks and no term row shows a raw TypeError message after the confirmation. | Check `l.term` first. |
| CR-9 | `internal/ops/bounded_work_test.go`, `.github/workflows/ci.yml` | The timing assertions of two bounded-work tests were removed; a regression now hangs until the 12-minute package timeout instead of failing in seconds. The 12-minute limit is shorter than the 20 minutes once needed on a slow runner (#269). | Guard with a goroutine and a generous `time.After`; name the number to raise first. |
| CR-10 | `internal/ops/vehicle_duplicate.go` (about line 137) | Duplicate this vehicle copies every `permission_actor_rank` row of the source (all players), and `actors.serial` and the entity blobs as they are; unverified in the game, as the file says. | Copy only the character's rank, or say all are copied. |
| CR-11 | `scripts/roadmap-gen.sh` | Issue titles reach `docs/roadmap.md` with only the pipe escaped; in a public repository any title would be committed as markdown. | Code span or escape. |

Status after the fixes (2026-10-27): CR-3 to CR-11 are fixed in the change that closes #443. CR-3: the dry run answers with a hash of the kit and the apply is refused (409) when it differs. CR-4: the result line of a fill that changed a blood sack carries the sentence. CR-5: text and `aria-describedby` at each disabled control that had its reason only in `title`. CR-6: `SetTaskProgress` refuses above `maxTier` (1e9). CR-7: `kits.Friendly` is exported and used by `kitArg`. CR-8: `lsFill` answers when the board has no term. CR-9: both tests wait at most a minute for the work (`finishes`), and `ci.yml` names the 12-minute limit as the number to raise first. CR-10: only the character's rank is copied. CR-11: `cell_text` in `roadmap-gen.sh` escapes titles and reasons.

## What was checked and found sound

- **Audit E1 (#251, queued edits replayed over the game's newer save): closed, and it holds for the new edits.** The register (2026-10-22) has 0 open. Scratch test: an Apply kit queued while a game process exists, then the game saved 5,000 XP, then `Flush`: the queue is held ("the game changed what it was worked out from"), the file keeps 5,000. The edit reads its rows inside the transaction (`giveLinesInTx`, `readLevelTx`, `fillMine`), and the journal hashes those reads. The Landsraad fills (`fillMine`) and Reset research read inside the edit the same way.
- **Care package model and loader.** Strict decode (unknown keys refused), format and version, 64 KiB file and 256 KiB stored limits, 20 kits, 50 lines, quantity 1 to 1,000,000 as integers (no float path), name character set (no markup, no path separator), duplicate names (case-insensitive), nothing kept from a file refused in part. Kit names are escaped by `html` on the page; `data-name` attributes are quoted.
- **Apply kit with the game running.** The write gate refuses to write and the edit is queued (`TestApplyKitWhileAGameProcessExistsIsQueuedAndWritesNothing`: session active, menu, check failed; file bytes unchanged, no backup). A dry run is rolled back (`errKitDryRun`) and never queues. A backup is taken before the commit (`commit` -> `backupIfStale`), and `installBlob` checks the gate again.
- **Write gate (#419, #421).** Every writer goes through `commit`, `Restore` or `installBlob`, all of which call `SaveBlocked`; configuration files are refused while a game process exists (`config.write`). The old mode name `menu_or_multiplayer` has no remaining reader.
- **SQL scope of the new writes.** Reset research, Reset journey (the keep list), Duplicate vehicle, Teleport to a map, Fill my contribution, Bring vehicle: every update or delete is limited by the character's controller, pawn or the vehicle ownership rule; the cargo-hold scope (#48) uses `ownedVehicleSQL` with the controller.
- **Route guard.** `Handler` wraps the whole mux: Host, loopback peer, session cookie for `/api/`, same-origin Origin and fetch metadata, JSON content type on non-GET. The new routes (`/api/kits/*`, `/api/landsraad/mine*`, `/api/config/reset`, `/api/vehicles/duplicate`, `/api/player/research/reset`) are registered on the same mux. Listening is limited to loopback (`internal/web/listen.go`).
- **Numbers.** `Args.Float` refuses NaN and infinity; `Args.Int` refuses fractions and values above 2^53; tiers are checked against the task's own tier rows and bounded; the hydration write keeps the stored JSON type (no whole-document float round trip; `jsonb_set` on the two paths only).
- **UI text.** All interpolation goes through `html` (escape by construction); `innerHTML` is assigned in one place. The new `tip` and `about` pieces have `aria-describedby`, Escape and focus handling; the permanent warnings (config files, unconfirmed map move, queued state, read-only save) were kept visible.
- **CI.** All actions pinned to commit SHAs, `contents: read`, no secrets, `persist-credentials: false`; the new Windows step keeps go's exit code through `Tee-Object` (`$LASTEXITCODE`). Recent runs: the `windows` job takes about 2 minutes after #437 (it took over 20 before); two earlier runs on 2026-10-26 failed after about 21 minutes, before the change.
- **Public hygiene.** No added line in the range mentions anything outside the offline save editor; no co-author or generation trailers in the 45 commit messages; no personal paths.

## Limits of this review

The 650 changed lines of `app.js` for the text sweep were read for escaping, removed warnings and accessibility, not for every sentence. The Bases, Vehicles and Crafting sweeps were read from the diff only. Statements about what the game does are taken from the code comments and were not checked in a running game.

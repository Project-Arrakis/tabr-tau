# Command Center M1: source register

Part of the Landsraad and Specialization Command Center work (milestone M1: audit, parity baseline, fixtures). Docs and synthetic fixtures only; no editor behaviour changes.

All paths are relative to the repository root. Line numbers and blob SHAs are those of commit `c369dd926e5f5c40048eaeca33b665be07dd2b02` (`main` after #867, #868, #870 and #871; git tree `327a453ae5f99160ee0b73f7b9a598431b6bb888`). They will drift: re-check a line against the function name before relying on it. A SHA is `git rev-parse HEAD:<path>`.

The design package that started this work (a handoff of specs, an approved mock and prompts, not part of this repository) quoted older SHAs. Where a file changed since, the table says so.

## 1. Files and what they hold

| Path | Lines | Blob SHA | Package SHA | Role |
|---|---|---|---|---|
| `internal/web/static/app.js` | 3383 | `561019e3e79ee25d1861b17b82946b39da2cd29d` | `e57f2d38...` (changed) | the whole page: Landsraad board, Specialization card, event handlers |
| `internal/web/static/app.css` | 805 | `ef176d74b087d84f84e76d0a8b1a0b78b731e645` | none | page styles |
| `internal/web/server.go` | 1307 | `aeab791fa4540c4ceee5ee1cce5fbe539a203e43` | none | HTTP routes, request guards, settings read for a write |
| `internal/ops/landsraad.go` | 357 | `bd2401287c7461b8a6d4a9459fbae73e427f638f` | none | the read (`Landsraad`), `SetDecree`, `SetTerm` |
| `internal/ops/landsraad_board.go` | 199 | `df53609600debe52bf2fdcb171dd62aa8682a266` | same | tier state, swatch rule, term history |
| `internal/ops/landsraad_apply.go` | 405 | `52e920419ab120a7e482002208a369acef714fbc` | same | the board's one write |
| `internal/ops/landsraad_chosen.go` | 91 | `a18d9d540aa6ba2b349e8a17860d1e97620c64fd` | none | stale-term and stale-house checks, complete, reopen |
| `internal/ops/landsraad_grant.go` | 439 | `ce1ad0b630f0489d6e14a7fa5361744a3d244d49` | none | what a raise grants, `houseLine`, settings, preview |
| `internal/ops/landsraad_paid.go` | 519 | `e4c5346c1c7d0f8999884794a89258db95006670` | none | the editor's own paid-ground record |
| `internal/ops/landsraad_rewards.go` | 181 | `13daab1d31a008c3382bdf52886eba660e92efd3` | none | reward-row writes |
| `internal/ops/landsraad_clock.go` | 84 | `54685c5f44a236b14a4511c383d6a937cb946657` | none | completion-time clock |
| `internal/ops/landsraad_factions.go` | 64 | `622d267dd4881798c027a0625b3f46c6b4d40280` | none | which factions the Landsraad has |
| `internal/ops/landsraad_locations.go` | 164 | `ec6bf47a74777787d5fcb859ef4e3c50b9787db6` | none | house places, map markers |
| `internal/ops/specialization.go` | 309 | `8bb7cafe5cf959ccb6c126813f1d2568ec59d941` | same | XP table, `Specs`, per-track and global writes |
| `internal/ops/reset_level.go` | 152 | `90373c8b405d398c9f08a1b98051a6ea3cde8fd6` | none | `ResetSpecTracks` (Reset All Known XP) |
| `internal/ops/ops.go` | 1235 | `ba96c1310bad0509ca99295015a26cc66bb66ee3` | none | `SetSpec` (raw row) |
| `docs/art/landsraad/README.md` | 17 | `717c5b4ce0baf4cf0b934c009e4dc2620edf4dbf` | same | art folder description |
| `docs/art-notice.md` | 13 | `4cb3621821f2010d462064ac61fdefa99f830472` | none | non-MIT notice for game art |
| `design/README.md` | 145 | `564b4f5a9cde2618c518f864bf5c1b38f3266a51` | same | the `design` module |
| `kit/README.md` | 83 | `6daf7a9d1e7660744ee10b014fbdef6a310521cc` | same | the `kit` module |

## 2. HTTP routes

Handler chain: `Server.Handler` (`internal/web/server.go` lines 193-246): host check, loopback peer check, then for `/api/` a session cookie, a same-origin check and, for a POST, a JSON content type. A POST body is a JSON object read by `postArgsMax` (lines 496-504), at most 4 MiB for these routes (`post`, line 506).

The write wrapper `postMaxAfter` (lines 515-545; `outcomeWritten` and `outcomeQueued`, lines 406-407) runs the handler, then flushes: the answer carries `outcome` `written` or `queued` (`outcomeWritten`, `outcomeQueued`). The page turns that into the "queued" hint (`NOT_SAVED`, `app.js` line 165).

| Method and path | Registered at | Handler | Reads / writes |
|---|---|---|---|
| GET `/api/landsraad` | `server.go` 843-855 | `Ops.Landsraad`, then `grant` (`GrantSettings.Preview`, with `room` from `Ops.SpecRoom`) is added | read |
| POST `/api/landsraad/apply` | 857 | `Ops.LandsraadApplyWith(grantSettings(), a)` | save write |
| POST `/api/landsraad/decree` | 858 | `Ops.SetDecree` | save write |
| POST `/api/landsraad/term` | 859 | `Ops.SetTerm` | save write |
| GET `/api/player/specs` | 722 | `Ops.Specs(specNums())` | read |
| POST `/api/player/specs/xp` | 723 | `Ops.AddSpecXP` | save write |
| POST `/api/player/specs/max` | 724 | `Ops.GrantMaxSpec` | save write |
| POST `/api/player/specs/reset` | 725 | `Ops.ResetSpec` | save write |
| POST `/api/player/specs/reset-all` | 726 | `Ops.ResetSpecTracks` | save write |
| POST `/api/player/specs/keystones/grant` | 728 | `Ops.GrantAllKeystones` | save write |
| POST `/api/player/specs/keystones/reset` | 729 | `Ops.ResetAllKeystones` | save write |
| POST `/api/player/specs/assign` | 730-746 | `ops.CheckAssign`, then `settings.Update` | writes the editor's settings file, not the save |
| POST `/api/player/specs` | 747 | `Ops.SetSpec` | save write (raw row) |

That is nine Specialization routes (one read, eight writes). `/api/player/level/reset` (line 727) is a neighbouring route for the character's own level and is not part of the Specialization card.

Static art: the page loads the 25 house symbols from `internal/web/static/houses/*.webp` through a list built from the embedded folder (`server.go` 570-594, folders `maps/icons` and `houses`). Nothing else from `docs/art/landsraad/` is served. A new picture family therefore needs its own embedded folder and its own entry in that list (a decision for M2, not made here).

## 3. Functions (what the code actually does)

### 3.1 Landsraad read

| Function | File and lines | Semantics |
|---|---|---|
| `Ops.Landsraad` | `landsraad.go` 32-235 | The current term is the row of `landsraad_decree_term` with the highest `term_id` (line 33). Tasks are selected `where term_id=?` and `order by board_index` (lines 40-41). If the term has no integer `term_id` the query has no term filter and returns the tasks of every term (lines 37-39): a hostile or damaged save can return more than 25 rows. Per task it adds `contributions` (faction progress), `player_contributions`, `my_contribution`, `my_recorded`, `my_events`, `my_elsewhere`, `my_paid`, `line`, `rewards`, then `boardTask` adds `location`, `swatch_held`, `swatch_template_id` and per-tier fields. It returns `term`, `tasks`, `decrees`, `rotation`, `factions`, `character`, `terms`, `earlier_terms`, `locations_from_markers` (line 234); `grant` is added by the route. |
| `boardTask` | `landsraad_board.go` 61-95 | Adds `location`, the swatch facts and, for each tier, `shown_template_id`, `shown_amount`, `swatch`, `reward_state`, `delivered`. The top tier shows the house swatch while the character does not hold it, or while it was won this term (line 77). |
| `tierState` | `landsraad_board.go` 98-110 | `not_reached` when the contribution is under the threshold; `no_row` when reached with no delivery row, or a row older than the term start; `waiting` when the row of this term has an amount above 0; `delivered` when the amount is 0. |
| `termHistory` | `landsraad_board.go` 116-199 | All terms, newest first, and for each earlier term the houses the character contributed to, tiers reached and whether a reward was written in the term window. |
| `lineOfHouse`, `houseLine` | `landsraad_grant.go` 103-119 | Specialization line of each house, from a public community tracker (comment 98-102); five houses per line; the save holds no such link. |
| `houseLocations`, `locationOf` | `landsraad_locations.go` 50-76, 151-164 | The place of each house from the save's map markers; `fixedHouseLocations` is the fallback (lines 78-83). |

### 3.2 Landsraad writes

| Function | File and lines | Semantics |
|---|---|---|
| `parseApplyItems` | `landsraad_apply.go` 43-125 | Body shape: `{term_id, houses:[{task_id, was_mine, was_done, was_win, was_events, mine_to, lower, complete_for, reopen}]}`. 1 to 100 houses (`maxFillHouses`, `landsraad.go` 357); a house at most once; `mine_to` from 0 to 1e9 (`maxTier`, line 336); `lower` and `reopen` must be true booleans (`strictBool`); `complete_for` with `reopen` is refused; a house with no change is refused. |
| `LandsraadApplyWith` | `landsraad_apply.go` 134-382 | One transaction (`save.Mutate`). Checks the term (`currentTerm`), reads every house, compares `was_*` and refuses the whole request on any difference (conflict, lines 213-217, 235-237, 243-251); refuses a lowering without `lower` (261-263); then writes. A raise inserts or updates the character's own row of `landsraad_task_player_contributions` (292-294) and calls `grantRaise`. A removal updates or deletes that row (321-335). `complete_for` calls `completeHouse`, `reopen` calls `reopenHouse`. After the writes `repairCompletionTimes` runs (367). |
| `chosenTask`, `currentTerm` | `landsraad_chosen.go` 14-27, 29-40 | A chosen house must exist and belong to the shown term; the shown term must still be the latest. |
| `completeHouse` | `landsraad_chosen.go` 43-78 | Raises the faction's progress to the goal (never lowers it), sets `completed=1`, the winner and the completion time. It never writes the character's own row. |
| `reopenHouse` | `landsraad_chosen.go` 80-90 | Sets `completed=0`, clears winner and completion time. Touches no progress and no contribution. |
| `grantRaise` | `landsraad_grant.go` 155-288 | What a raise stands for: missions = `ceil(after/step) - ceil(lo/step)`, step = 1,000 x `LandsraadContributionMultiplier`; per mission +6,000 Solari (currency 0), +1,000 House Scrip (currency 1), +1,500 x `LandsraadSpecializationXpMultiplier` XP on the house's line (constants 45-53); capped at 1,000 missions (`maxMissionsAtOnce`, checked in `landsraad_apply.go` 318-320). Writes the reward rows of the tiers crossed (`landsraad_rewards.go` `writeTierRewards`). Writes no progress event, no player link, no cursor move (comment 12-44). |
| `GrantSettings`, `Preview` | `landsraad_grant.go` 57-65, 331-345 | The two multipliers (read in `server.go` `grantSettings`, lines 1271-1303: `ServerCustomSettings.ini`, 0 to 10, 1 when absent) and the learned track numbers. `Preview` is what the page shows before a click: `step`, `solari`, `scrip`, `xp`, `max_missions`, `problem`. |
| Paid-ground record | `landsraad_paid.go` header 24-52, `trustedPaid` 476, `paidGround` 489 | The editor records the ground it paid in `landsraad-paid-<hash>.json` in its own folder (`PaidLedgerName`, line 112), written only after the game's file is written. A tier removal lowers the contribution and not the record, so the same raise afterwards pays nothing again. The record counts only as far as the save shows reward rows of this term (the cap). A file of a newer version refuses the raise. |
| `SetDecree` | `landsraad.go` 239-256 | Sets `landsraad_decrees.disabled` for one row by id. It is the pool flag, not "in effect". |
| `SetTerm` | `landsraad.go` 259-326 | Sets any of `active_decree_id`, `elected_decree_id`, `reigning_faction_id`, `end_time` on the latest term (empty string or null clears). `reigning_faction_id` must be a Landsraad faction or unchanged (296-305); `end_time` may not be before `start_time` (307-313). |

### 3.3 Specialization

| Function | File and lines | Semantics |
|---|---|---|
| `specCumulativeXP`, `maxSpecXP`, `specLevel` | `specialization.go` 24-36, 38, 41-51 | 101 thresholds (level 0 to 100), the last is 44,182. `specLevel` is the level with the fraction of the way to the next: exact at each threshold, 0 at or below 0 XP, 100 at or above the cap. |
| `Specs` | `specialization.go` 82-132 | Answer `{rows, unassigned, maxXp, tracks, resetCount}`. A row is `{track, number, xp, level, keystoneOwned, keystoneTotal, granted}`; `number` is null for a track with no number; `granted` is every keystone owned. Keystones are counted by name prefix (`Combat_`), so they need no numbers. `unassigned` lists save rows whose number no track uses. |
| `EffectiveTrackNumbers` | `specialization.go` 137-155 | Learned numbers first, then the measured numbers (`landsraad_grant.go` 73: Crafting 1, Gathering 2, Exploration 3, Combat 4, Sabotage 5) for tracks with none, leaving out a measured number a learned number already uses. |
| `CheckAssign` | `specialization.go` 293-309 | A track must exist, the number 0 to 255, and no other track may hold it. |
| `AddSpecXP` | `specialization.go` 181-214 | `amount` from -44,182 to 44,182 and not 0; read and write in one edit; result clamped to 0..44,182; the level is recomputed. |
| `GrantMaxSpec` | `specialization.go` 216-231 | XP 44,182, level 100; keystones not touched. |
| `ResetSpec` | `specialization.go` 233-252 | Deletes the track row; keystones not touched. |
| `GrantAllKeystones` / `ResetAllKeystones` | `specialization.go` 254-271 / 273-291 | Global for every track: insert or ignore every keystone of the map, or delete every purchase of the character. There is no per-track keystone write. |
| `ResetSpecTracks` | `reset_level.go` 116-152 | Deletes the rows of every track whose number is known; `expected` must equal the count found inside the edit or the request is refused (conflict); a row with an unknown number and the keystones are left alone. |
| `SetSpec` | `ops.go` 1073-1092 | Raw row: `track_type` 0..255, `xp` 0..44,182, `level` 0..100, written as given. Nothing checks that XP and level agree. |

## 4. Page code (`internal/web/static/app.js`)

| Name | Lines | What it does |
|---|---|---|
| Specialization card | 958-984 | One table, the global keystone buttons, Reset XP of all known tracks, the Track numbers card (only when a number is missing) and the raw-row details block. |
| `spKeyAll`, `spKeyReset` | 2970, 2971 | Ask, then POST keystones/grant or keystones/reset with `{}`. |
| `spAdd` | 2972 | POST specs/xp `{track, amount}`; no confirmation. |
| `spMax`, `spReset` | 2973, 2974 | Ask, then POST specs/max or specs/reset `{track}`. |
| `spAssign`, `spAssignHand` | 2975, 2976 | POST specs/assign `{track, number}` from a listed unassigned row or from typed values. |
| `specSet` | 2977 | Ask, then POST specs `{track_type, xp, level}`. |
| `spResetAll` | 3007-3013 | `resetArea` (line 720): typed confirmation, then POST specs/reset-all `{expected: count}`. |
| `LB`, `LP` | 1793-1796 | View state: sort, show filter, house filter, group, jump; and write state: chosen faction, chosen houses, busy flag, open bulk question. |
| `lbHouse` | 1817-1828 | Turns a server task into the page's house: goal, faction progress, the character's tiers and states, place. `state` is `new`, `part` or `done` by the character's own tiers. |
| `lbItem` | 1857-1869 | One house's request item for a click: `tier`, `fill`, `fillgoal`, `complete`, `reopen`; always sends `was_mine`, `was_done`, `was_win` and `was_events` when known. |
| `lbGrantOf`, `lbGrantOfItems` | 1892, 2317 | The preview sentence for a raise, worked out with the arithmetic of `grantRaise`. |
| `lbTierTip` | 1952-1959 | The tier's reason text, including why it is off: no faction, or another faction chosen. |
| `LB_SHOWS` | 2067 | Show filter: All, What is left, Not started, In progress, Top tier. |
| `lbTermSel`, `lbTerm` | 2110, 2117-2150 | The Term and decrees panel. |
| `lbRun`, `lbSend`, `lbBulkItems`, `lbAskOf`, `lbGo`, `lbBulk`, `lbConfirmYes` | 2232, 2274, 2312, 2328, 2339, 2348, 2355 | One write at a time; request through `/api/landsraad/apply`; a bulk or a heavy single click asks first; a conflict re-reads the board. |
| `lbApply` | 2372-2440 | Sort, filter, group by region and jump over the already drawn rows. |
| Click handlers | 2468-2522 | `tier`/`qa` (fill, complete, reopen, fillgoal), `pickall`, `pickno`, `bulk`, `cyes`, `cno`, `tapply`/`tmk` (term), `tflag` (pool flag), `map`, `show`, `showall`, `sort`, `hs`, `jump`, `group`. |

## 5. Tests that already exist for these paths

`internal/ops` holds the fixtures this work builds on: `landsraad_mine_test.go` (`mineWorld`, `mineOps`), `landsraad_board_test.go` (`newerOps`, tier states), `landsraad_apply_test.go`, `landsraad_grant_test.go`, `landsraad_paid*_test.go`, `specialization_test.go`, `reset_level_test.go`, `args_test.go`. The M1 additions are in `internal/ops/command_center_fixtures_test.go` (see `04-fixture-catalog.md`).

## 6. Unverifiable from the repository (read as UNKNOWN)

- What the game does with a contribution or a reward row the editor wrote, after the save is loaded again (`landsraad_grant.go` comment lines 25-34 say it is not checked in the game).
- What the `sysselraad` column means (schema only: `docs/save-database.md` 1774, `docs/evidence/single-player-schema.ddl.sql` 462; no code reads it for a decision).
- Where a Voting Session start snapshot would live. No table or column of the schema documented in `docs/save-database.md` holds one.
- Whether a mission without a Recollection pays House Scrip and XP (#745, named in `landsraad_grant.go` comment line 23-24).

# Command Center M1: source-of-truth audit

Milestone M1 of the Landsraad and Specialization Command Center work: an audit of what the editor does today, a parity baseline and synthetic fixtures. Docs, fixtures and pure-invariant tests only. No page behaviour, save schema, write route or art changes in this milestone. Later milestones (M2 to M7) are only planned here, not started.

Companion files: `02-source-register.md` (path, function, lines, SHAs), `03-parity-baseline.md` (test baseline and the feature-parity gap table), `04-fixture-catalog.md` (fixtures and their expected outcomes), `DECISIONS.md` (open questions, safe interim display), `requirements-matrix.csv` (the package matrix with evidence).

A claim below cites a file, a function and lines, or says UNKNOWN. Lines are those of commit `c369dd926e5f5c40048eaeca33b665be07dd2b02`.

## 1. Environment and method

| Item | Value |
|---|---|
| Repository | `Project-Arrakis/tabr-tau`, `main` at `c369dd926e5f5c40048eaeca33b665be07dd2b02` (git tree `327a453ae5f99160ee0b73f7b9a598431b6bb888`) |
| Merged since the package snapshot | #867, #868, #870, #871 (2026-11-16) |
| Open and left alone | #864 (edit answers), #865 (grant and paid record: one rule in one place) |
| Working branch | `cc-m1-audit`, reset to `origin/main`, clean before this work |
| Operating system | Windows 11 (10.0.26300), Go 1.26.3 windows/amd64, `CGO_ENABLED=0` |
| Method | source read, `go vet`, `go test`, and a second implementation (the fixture generator) cross-checked by tests; no game, no browser, no real save |
| Not done | any run in the game, any browser run, any screenshot comparison |

The package that asked for this audit (specs, an approved mock, review lists and prompts) is a design handoff and is not part of the repository. Its statements were checked against the code; section 10 lists where it is out of date.

## 2. Headline findings

1. **The 5 x 5 board does not exist in the page today.** `Ops.Landsraad` orders tasks by `board_index` (`landsraad.go` 37-41), but `lbApply` re-sorts the rows by house name or another key (`app.js` 2372-2440, default `house`). Nothing in `app.js` reads `board_index` or `sysselraad`. The geometry, the twelve lines and any winner logic are new code with no current consumer, so there is no existing behaviour to break and no existing test to lean on.
2. **`sysselraad` has no known meaning.** It is `INTEGER NOT NULL DEFAULT 0` in the single-player schema (`docs/save-database.md` 1774; `docs/evidence/single-player-schema.ddl.sql` 462) and `boolean NOT NULL` in the reference server schema (`docs/evidence/postgres-dune-schema-columns.txt` 638). No code reads it for a decision. It must not be shown as a victory flag (DECISIONS D02).
3. **There is no Voting Session start snapshot.** No table of the documented schema (`docs/save-database.md`) holds one. A majority "at the start of the vote" cannot be verified from a save. Any display of a majority is provisional (D03). A line win (five Houses in one line of the 5 x 5 board) needs no snapshot.
4. **Faction goal and personal tiers are two scales.** `goal_amount` is the faction's goal for the house; the five reward tiers are thresholds of the character's own contribution (`landsraad.go` 18-31; `landsraad_apply.go` header 13-28). Both come from the save. A contribution may exceed the goal.
5. **A reached tier with no delivery row is a distinct state, `no_row`.** `tierState` (`landsraad_board.go` 98-110) returns `waiting`, `delivered`, `no_row` or `not_reached`; `no_row` is ambiguous (never written, or written in an earlier term) and the page labels it "Reached, no delivery recorded" (`LB_STW`, `app.js` 1798).
6. **Keystones are global.** The only keystone writes are Grant all and Reset all, for every track (`specialization.go` 254-291). There is no per-track keystone write, so a per-track keystone control would be a new write route and is out of scope (D08).
7. **Reset All Known XP never touches an unknown row or a keystone** and refuses when the count it was shown is not the count found (`reset_level.go` 116-152).
8. **The editor already guards every Landsraad write against a stale view**: term, house state, own contribution and the ground its own events cover are compared inside the transaction, all or nothing (`landsraad_apply.go` 181-251). A new view must send the same `was_*` fields or the guard is lost.
9. **Art in the repository is reference only, apart from the 25 house symbols.** `docs/art/landsraad/` holds 25 + 5 + 5 + 15 PNGs and is not loaded by the editor; the page loads 25 `webp` house symbols (`server.go` 570-594). `decrees/vendor-special.png` does not exist although the folder README lists it, and the banners are 256 x 256, not 512 x 512 as the README says (section 9).
10. **The package's repository facts were stale**: `app.js` changed, #815 is now closed, one decree picture is missing and two README claims about the art are wrong (sections 9 and 10).

## 3. What `GET /api/landsraad` returns

Source of the shape: `landsraad.go` 32-235 plus the route (`server.go` 843-855). Labels: **SAVE** a value copied from a save table; **DERIVED** computed by the editor from save values; **REFERENCE** a constant of the editor or a public source; **DEMO** appears only in the mock, never in production; **UNKNOWN** meaning not established.

### Top level

| Key | Label | Origin |
|---|---|---|
| `term` | SAVE | the row of `landsraad_decree_term` with the highest `term_id` (line 33) |
| `tasks` | SAVE + DERIVED | `landsraad_tasks` of that term ordered by `board_index`, plus per-task additions below |
| `decrees` | SAVE + DERIVED + REFERENCE | `landsraad_decrees`; `active`, `elected`, `in_rotation` derived by id (209-225); `display_name`, `effect`, `variant` from the editor's name table (`landsraad_decree_names.go`) |
| `rotation` | SAVE | `landsraad_decree_rotation.decree_id` list. It is the decree rotation, not a mission rotation |
| `factions` | SAVE | the factions the Landsraad has (ids 1 and 2, `landsraad_factions.go` 19-31) |
| `character` | SAVE | `{player_id, name, faction_id}` of the character |
| `terms` | SAVE | every term, newest first (`termHistory`) |
| `earlier_terms` | DERIVED | per earlier term, the houses the character contributed to |
| `locations_from_markers` | DERIVED | whether the places came from the save's map markers or from the fixed table |
| `grant` | DERIVED + REFERENCE | added by the route: `step`, `solari`, `scrip`, `xp`, `max_missions`, `problem`, `room` |

### Per task

| Key | Label | Origin |
|---|---|---|
| `id`, `board_index`, `house_name`, `goal_amount`, `completed`, `winning_faction_id`, `completion_time` | SAVE | `landsraad_tasks` |
| `sysselraad` | SAVE, meaning UNKNOWN | `landsraad_tasks.sysselraad` |
| `contributions` | SAVE | `landsraad_task_faction_contributions`: progress of each faction toward `goal_amount` |
| `player_contributions` | SAVE | `landsraad_task_player_contributions`, with `mine` marked by the editor. Other players are the simulated guild actors of the save, not named guilds (`landsraad.go` 26-31) |
| `my_contribution`, `my_recorded` | DERIVED | the character's row under its own faction |
| `my_events`, `my_elsewhere`, `my_paid` | DERIVED | ground already covered: own progress events, the highest under another faction, the editor's own record (`landsraad_grant.go` 12-44; `landsraad_paid.go` 24-52) |
| `line` | REFERENCE | `lineOfHouse`: a public community tracker, five houses per line; the save holds no such link (`landsraad_grant.go` 98-102) |
| `rewards[]` | SAVE + DERIVED | `landsraad_task_rewards` rows with `reached`, `shown_*`, `swatch`, `reward_state`, `delivered` (`boardTask`) |
| `swatch_held`, `swatch_template_id` | SAVE + DERIVED | `landsraad_one_time_house_rewards`, `landsraad_player_one_time_house_rewards` (absent in an older save layout, `landsraad_board.go` 23-55) |
| `location` | DERIVED | the house representative's marker, or the fixed table |

Not in the answer and not in the save, so never to be invented by a view: a ranking of contributing guilds with names, the missions on offer, the daily or weekly limits some guides quote (`landsraad_grant.go` 21-24 calls them unverified), and a vote start time.

## 4. What each Landsraad action does (all go through `POST /api/landsraad/apply` unless stated)

The page never stages: every click is one request (`app.js` 1793-1796 comment). The request is `{term_id, houses:[item, ...]}` with at most 100 items; one item per house, at most once.

| Page action | Item sent (`lbItem`, `app.js` 1857-1869) | Server effect (`landsraad_apply.go`) |
|---|---|---|
| Click tier *i* above the reached count | `mine_to` = threshold *i* | Raise: the character's own contribution row is set to that threshold, which also reaches every lower tier (contribution is cumulative); `grantRaise` pays the missions that ground stands for and writes the reward rows of the tiers crossed |
| Click the highest reached tier | `mine_to` = the tier below (0 for tier 1), `lower: true` | Removal: the row is lowered or deleted. Nothing already paid is taken back (the record in `landsraad_paid.go` keeps it) |
| Click a lower reached tier | none | Refused by the page: "To remove tiers, click the highest reached tier" (`app.js` 2476) |
| Fill | `mine_to` = top tier | Same raise as clicking the top tier. Off at the top tier, with no faction, or when another faction is chosen (`lbFillOff`, `lbFillOffF`, 1851-1854) |
| Fill faction goal | `mine_to` = top tier and `complete_for` = chosen faction | Raise plus completion for that faction; refused if `complete_for` is not the character's own faction while `mine_to` is set (`landsraad_apply.go` 220-224) |
| Complete | `complete_for` = chosen faction | `completeHouse`: faction progress raised to `goal_amount` (never lowered), `completed=1`, winner, completion time. Never writes the character's row |
| Reopen | `reopen: true` | `reopenHouse`: `completed=0`, winner and time cleared; progress and contribution stay, so a house taken by another faction is not given back |
| Choose boxes and Fill / Complete / Reopen chosen | one item per chosen house | Same items in one request; always asks first; all houses or none |
| Choose faction (drop-down) | `complete_for` value | Which faction Complete and Fill faction goal act for |

**Disabled by alignment.** Tiers and Fill change the character's own contribution, which counts for the character's own faction only. With another faction chosen, or with no faction (Neutral), those controls are off with an explanation (`lbTierTip`, `app.js` 1952-1959; server side `myLandsraadTx`, `landsraad.go` 339-352, and the check at `landsraad_apply.go` 220-224). Complete, Reopen and Fill faction goal still act for the chosen faction.

**Stale guards.** The item carries `was_mine`, `was_done`, `was_win` and, for a raise, `was_events`, `was_paid`, `was_paid_rows` and `was_elsewhere` (#805); the request carries `term_id`. Any difference is a conflict (HTTP 409 class, `apierr.ErrConflict`) and nothing is written; the page re-reads the board (`lbRun`, `app.js` 2232-2262). A new view must keep sending them.

**Asking first.** Several houses always ask; one house asks when a raise would pay 10 or more missions (`LB_ASK_MISSIONS`, `app.js` 1890; `lbGo`, 2339).

**Term and decrees** (the Term panel, `lbTerm` 2117-2150):

| Page action | Request | Server effect |
|---|---|---|
| Make active / Apply | POST `/api/landsraad/term` `{active_decree_id, reigning_faction_id}` (or the id alone, leaving the faction) | `SetTerm`: sets those columns on the latest term. A faction must be a Landsraad faction or unchanged |
| Pool flag | POST `/api/landsraad/decree` `{id, disabled}` | `SetDecree`: one row's `disabled` column. It does not say whether the decree is in effect |

`elected_decree_id` and `end_time` are accepted by `SetTerm` (`landsraad.go` 266) but the page sends only `active_decree_id` and `reigning_faction_id` (`app.js` 2485-2493).

## 5. What a raise grants, and what it does not

Sources: `landsraad_grant.go` 12-53, `landsraad_apply.go` 292-320, `landsraad_paid.go` 24-52.

- One mission step is 1,000 x `LandsraadContributionMultiplier` contribution. The number of missions a raise stands for is the steps it crosses beyond ground already covered.
- Each mission grants +6,000 Solari, +1,000 House Scrip and +1,500 x `LandsraadSpecializationXpMultiplier` XP on the house's line. The multipliers are read from `ServerCustomSettings.ini` (0 to 10, 1 when absent; a bad value refuses the raise with the reason, `server.go` 1271-1303).
- One request may stand for at most 1,000 missions, counted across all houses in it (`landsraad_apply.go` 318-320).
- No progress event, player link or cursor is written. What the game does with the contribution after it is loaded again is UNKNOWN (`landsraad_grant.go` 27-34). The page says the pay-out "has not been checked in the game yet" (`GRANT_UNCHECKED`, `app.js` 1891).
- Ground already covered is protected three ways: the character's own events, the highest contribution under another faction, and the editor's own paid-ground record. A removal lowers the contribution but not the record, so removing and re-raising pays nothing twice. The record counts only up to the highest tier whose reward row of this term stands (the cap), and a record from a newer editor version refuses the raise.
- A view must not simplify this to "everything a mission paid is always available". Specialization XP stops at the track cap and the answer says how much was not added (`specialization_xp_not_added`).
- Needs operator confirmation on Windows before anything claims more: whether the game keeps a raised contribution or recomputes it from its own events; whether House Scrip and XP are paid for a mission without a Recollection.

## 6. Specialization

Sources: `specialization.go`, `reset_level.go`, `ops.go` 1073-1092, `app.js` 958-984 and 2970-3013.

- Five tracks in the answer order Combat, Crafting, Exploration, Gathering, Sabotage (`SpecTracks`, line 21).
- XP table: 101 thresholds, level 0 to 100, cap 44,182 (`specialization.go` 24-38). Level is derived and fractional between thresholds (`specLevel`). The stored level is a separate column that the raw setter can make disagree with the table.
- Keystones are counted separately (owned and total per track) and are independent of XP: a track can hold the cap with zero keystones and the reverse. `granted` means every keystone of the track is owned.
- The save stores only a number per track. The measured numbers are Crafting 1, Gathering 2, Exploration 3, Combat 4, Sabotage 5 (`landsraad_grant.go` 73); a learned number (settings file) replaces the measured one; a measured number a learned number already uses is dropped, so two tracks never share a row and the dropped track shows as unresolved (`EffectiveTrackNumbers`).
- XP controls are off for a track with no number (`known(r)`, `app.js` 960-977). Assignment (`CheckAssign`) refuses a number another track holds and any number outside 0 to 255. It writes the editor's settings file, not the save.
- Per-track writes: signed XP (`AddSpecXP`), Grant max, Reset (row deleted, keystones kept). Global writes: Grant all keystones, Reset all keystones, Reset All Known XP. Raw row: any XP 0 to 44,182 and level 0 to 100, unchecked against each other.
- The Landsraad grant adds XP to the same rows (`addSpecialization`, `landsraad_grant.go` 307-326) with the track number from the same settings; two lines on one number are refused (`checkTracks`, 87-97).

## 7. Board geometry and Sysselraad (what can and cannot be proved)

- A task belongs to one term; `(term_id, board_index)` and `(term_id, house_name)` are unique and `board_index >= 0` is checked (`docs/save-database.md` 1778-1780). There is no upper bound and no check that a term has 25 rows. A view must validate each read: exactly 25 rows, `board_index` 0 to 24 each once. The fixtures cover a duplicate, a missing row, an out-of-range index and a read with no term filter.
- Geometry fixed by position: `board_index = 5 * row + column`; twelve lines (five rows, five columns, two diagonals). The fixtures and `command_center_fixtures_test.go` check the twelve lines, that each cell is on 2, 3 or 4 lines (the centre on 4), and that the tug track has 25 seats with one middle seat.
- Owner of a house: `completed = 1` with a Landsraad faction as `winning_faction_id`. `completed = 0` with no winner is open. Everything else (completed with no winner, a winner without completion, a faction that is not a Landsraad faction) is **unreadable** and must not be counted, because the page itself shows a winner-without-completion house as Taken (`lbTaken`, `app.js` 1815).
- Line win: five houses of one faction in one line. No count lead is needed (fixtures `harkonnen-row2-low-total`, `atreides-col3-low-total`).
- Count win: more houses at the Voting Session start. The save has no such snapshot, so a count lead can only be shown as pending or provisional. It is never to be shown as a confirmed majority from the current board.
- A board with a complete line for each faction is a conflict: no first-win mechanic is verified, so no winner is chosen (D04).
- The `sysselraad` column is carried in the read but proves nothing: the fixture `sysselraad-flag-set-no-evidence` keeps the verdict pending.
- The proposed verdict rules are in `04-fixture-catalog.md`. They are proposals for M4 review. The Go tests assert only geometry and arithmetic facts, never a verdict.

## 8. Specific questions of the M1 brief

| Question | Answer | Evidence |
|---|---|---|
| Voting cutoff available? | No | no table or column; D03 |
| What does `sysselraad` mean? | UNKNOWN | schema only; D02 |
| Is `board_index` validated? | By the schema for uniqueness and sign only; not by the editor, not for 0 to 24 or for 25 rows | `docs/save-database.md` 1778-1780; no use in `app.js` |
| Source of contributor rankings? | None. Other player ids are simulated guild actors | `landsraad.go` 26-31 |
| Source of a mission rotation? | None. `landsraad_decree_rotation` is the decree rotation | `landsraad.go` 202-205 |
| First-party validation of `houseLine`? | None. Two missions agreed with it (Argosaz, Mikkarol); the save has no house-to-line link | `landsraad_grant.go` 98-102 |
| Current reward amount scale | Per-task rows of `landsraad_task_rewards`; tiers measured at 1,200 to 6,000 in the terms read; `goal_amount` is a different scale | `landsraad.go` 18-25 |
| Per-track keystone edits supported? | No. Global only | `specialization.go` 254-291 |

## 9. Art and notices (PR #845)

Merged 2026-11-16 as `1b549c3b15c99bf0089c2af85ffbf1bd6a1c1fec`, "Docs: reference pictures for the Landsraad tab (houses, lanes, banners, decrees)", 53 files.

| Folder | Files | Total size | Pixel size (measured with `file`) | README says |
|---|---|---|---|---|
| `docs/art/landsraad/houses/` | 25 PNG | 141,727 bytes | 128 x 128 | 128 x 128 (matches) |
| `docs/art/landsraad/lanes/` | 5 PNG | 421,625 bytes | 256 x 256 | 256 x 256 (matches) |
| `docs/art/landsraad/banners/` | 5 PNG | 120,535 bytes | 256 x 256 | **512 x 512 (wrong)** |
| `docs/art/landsraad/decrees/` | 15 PNG | 1,452,558 bytes | 256 x 256 | 15 files, but the README names 16 including `vendor-special`, which **does not exist** |
| `internal/web/static/houses/` | 25 webp | 42,598 bytes | 64 x 64 (README) | served by the editor |

- The game's house `Mikkarol` is written `mikarrol.png` in the reference folder and `mikkarol.webp` in the served folder (README note; `app.js` `LB_SYM`, 1805-1806, aliases `mikarrol`). The save and `houseLine` use `Mikkarol`.
- The art is Funcom's, not MIT, reference copies only (`docs/art-notice.md`); the editor loads only the 25 house symbols.
- The issues the package warned about: #814 is "audit L2 R2-05: a second commit on main without a pull request or check run, not read by the push check" (open); #815 is "audit L2 R2-06: Repair every base also writes the pieces of a backup; the scope is not decided or tested" (closed on 2026-11-16 when read). Neither is a design or kit issue and neither is used here.

## 10. Where the package was out of date

| Package statement | Found at `c369dd9` |
|---|---|
| `app.js` blob `e57f2d38...` | now `561019e3...` (changed) |
| git tree `2e5033410...` | now `327a453ae...` |
| #815 open as an audit item | closed |
| decree art includes `vendor-special` | the file is missing |
| banner art 512 x 512 | 256 x 256 |
| an earlier-term task can appear in the current response | only when the term has no integer `term_id` (`landsraad.go` 37-43); otherwise the query is filtered by term |

## 11. Risks that need a stop, not a guess

| Risk | Where | Handling in the plan |
|---|---|---|
| A confirmed winner shown without evidence | any view of the vote | only a line win may be confirmed; a count lead is "pending" (D03, D05) |
| An unknown track number written as known | `Specs`, `trackNumber` | unresolved rows stay unresolved and their controls stay off |
| A global keystone action shown as per-track | `spKeyAll`, `spKeyReset` | the control stays global and says so |
| Stale view writes | `landsraad_apply.go` | every new write path sends `term_id` and `was_*` |
| Paid ground paid twice | `landsraad_paid.go` | no new write path; the existing one only |
| A damaged board read as a valid one | `Landsraad` with no term | the evaluator returns "unavailable" for anything but a complete 25-row board |

## 12. File-level plan for M2 to M7 (bounded, nothing started)

The old Landsraad and Player > Specialization views stay in place until the parity matrix passes; the new views are added beside them and switched by a setting the old path can undo.

| Milestone | New or changed files (proposed) | Reuses |
|---|---|---|
| M2 art and shell | a new embedded art folder under `internal/web/static/` and its entry in the list at `server.go` 570-594; CSS in `app.css`; a notice line in `docs/art-notice.md` | the existing house symbols; no `design` or `kit` change |
| M3 board and inspector | board rendering in `app.js` beside `lbApply`; one inspector panel | `lbHouse`, `lbItem`, `lbSend`, the same route and guards |
| M4 evaluator | a pure JS module and a Go twin (or fixtures) for the twelve lines and the verdict rules | the fixtures of this milestone |
| M5 Specialization | new Specialization view beside the old card | the nine routes, unchanged |
| M6 term, decrees, history | read-only history and the Term panel behaviours | `SetTerm`, `SetDecree` unchanged |
| M7 release gate | Windows manual pass, screenshot comparison, rollback test | `docs/live-test-protocol.md` style |

If a view needs data that is not in the answer, add a read-only key to `GET /api/landsraad` (additive), never change a write route. The validity of a board (25 rows, indexes 0 to 24 each once) is the first such candidate and can be computed in the page from the existing keys.

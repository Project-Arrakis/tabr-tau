# Command Center: decisions and open questions

Kept on the project branch as the one place for the questions the Landsraad and Specialization Command Center cannot answer from the repository. Every row has a source, a severity, what it changes in the page, an owner and a safe interim display: what the page shows until the question is answered. A row is closed only by evidence recorded in the row.

Severity: P0 stop (a wrong answer would write or announce something false); P1 a wrong state or a wrong label; P2 polish.

Owner: "operator" is the person who plays and tests on Windows; "M*" is the milestone whose work needs the answer.

Written for milestone M1 at commit `c369dd926e5f5c40048eaeca33b665be07dd2b02`. Status values: DECIDED (settled by the code or by the operator), PROPOSED (a rule written for review), OPEN (no source yet).

## A. The eight questions the M1 brief asked for

| ID | Question | Answer at M1 | Status | Source | Sev | Changes in the page | Owner | Safe interim display |
|---|---|---|---|---|---|---|---|---|
| D03 | Is a Voting Session start snapshot available? | No. No table or column of the documented schema holds one; no code reads one | DECIDED (unavailable) | `docs/save-database.md` (all `landsraad_*` tables); `landsraad.go` 32-235 | P0 | A count lead can never be shown as a confirmed majority | M4, operator | "Vote not started or not verified": show counts as "current count", never "majority". A line win may be confirmed |
| D02 | What does `landsraad_tasks.sysselraad` mean? | UNKNOWN. `INTEGER NOT NULL DEFAULT 0`; `boolean` in the reference server schema; nothing reads it for a decision | OPEN | `docs/save-database.md` 1774; `docs/evidence/single-player-schema.ddl.sql` 462; `docs/evidence/postgres-dune-schema-columns.txt` 638 | P0 | It must not decide or show a winner | operator (needs an in-game observation: the column before and after a vote) | Carry the value in the data only; show nothing from it. The fixture `sysselraad-flag-set-no-evidence` keeps the verdict pending |
| D01 | Is `board_index` valid 0 to 24 each term? | Not guaranteed. The schema makes `(term_id, board_index)` unique and `board_index >= 0`; there is no upper bound and no 25-row rule. The editor does not check it | DECIDED (validate on every read) | `docs/save-database.md` 1778-1780; `landsraad.go` 37-43 | P0 | The evaluator and the board need a "board unavailable" state | M3, M4 | "Board incomplete: N of 25 houses readable" with no lines and no counts. The page keeps the list view |
| D07 | Source of a ranking of contributing guilds | None. Other `player_id`s are the simulated guild actors of the save, without guild names | DECIDED (no source) | `landsraad.go` 26-31, 85-100 | P1 | No leaderboard in the header | M6 | Empty state "Not in the save"; no names, no numbers |
| D08m | Source of live missions and their rotation | None. `landsraad_decree_rotation` is the decree rotation | DECIDED (no source) | `landsraad.go` 202-205 | P1 | No mission panel | M6 | Empty state "Not in the save". The decree rotation list stays labelled as decrees |
| D09 | Is `houseLine` first-party? | No. It comes from a public community tracker; two missions agreed with it (Argosaz, Mikkarol); the save has no link | DECIDED (keep, labelled) | `landsraad_grant.go` 98-102 | P1 | The line of a house is a reference value, not a save value | M5 | Label it as a reference mapping wherever shown; keep five houses per line; a house not in the table shows no line |
| D06 | Reward amount scale: faction goal versus personal tiers | Two scales, both read from the save. `goal_amount` is the faction goal; tiers are `landsraad_task_rewards.threshold` rows of the character's own contribution | DECIDED | `landsraad.go` 18-25; `landsraad_apply.go` 13-28 | P0 | Two separate meters, never one | M3 | Show "Faction a / goal" and "You b / next tier" as two lines |
| D08 | Are per-track keystone edits supported? | No. Only global Grant all and Reset all | DECIDED | `specialization.go` 254-291; `app.js` 2970-2971 | P1 | No per-track keystone control; the two keystone buttons say "every track" | M5 | Keep the two global buttons with their current confirmation text |

(D08m is the package's D08, "current live missions and rotation", renamed here to keep it apart from the per-track keystone row, which the M1 brief asks for separately.)

## B. Rules that are proposals for the evaluator (M4)

These rules are encoded as the expected values of `fixtures/board-victory-cases.json` (see `04-fixture-catalog.md`). They are proposals. The Go tests check the geometry, the counts and the line status of each case, and nothing else: a verdict is not asserted by any test until M4 builds the evaluator and the operator accepts the rules.

| ID | Rule | Status | Sev | Safe interim display |
|---|---|---|---|---|
| D04 | A complete line for each faction in one board is a conflict. No first-win mechanic is verified, so no winner is chosen and the DOM order never decides | PROPOSED | P0 | "Conflict: both factions hold a line" with both lines outlined |
| D05 | A count lead and a vote result are different statuses. Without a verified cutoff a lead is `pending_vote`, even at 13 or more of 25 | PROPOSED | P0 | "Leads the current count"; the 13th seat of the track is drawn as a position, not as a result |
| D16 | The gold 13th seat is a real position of the 25-seat track. It does not reduce the official vote rule: a majority at a verified cutoff may be below 13 | PROPOSED | P1 | The track shows the seat, the status line says what is and is not verified |
| D10a | Owner of a house is read strictly: `completed = 1` with a Landsraad faction is that faction's; `completed = 0` with no winner is open; everything else is unreadable and counts as neither | PROPOSED | P0 | An unreadable house is drawn as "unknown"; a board with any unreadable house shows no confirmed count and no confirmed majority. A complete line elsewhere may still stand |
| D13a | A verified cutoff majority recorded together with a line for the other faction is a conflict, not a win | PROPOSED | P0 | "Conflicting saved outcome, verification needed" |

## C. Other open items carried from the package register

| ID | Item | Status at M1 | Sev | Owner | Safe interim display |
|---|---|---|---|---|---|
| D10 | Mikkarol / Mikarrol alias | DECIDED: save and `houseLine` say `Mikkarol`; the reference PNG is `mikarrol.png`; the served symbol is `mikkarol.webp`; the page maps both spellings (`app.js` 1805-1806). An explicit lookup and a test are needed in M2 and M5, and the save's spelling is never renamed | P2 | M2 | Use the lookup, never build a file name from a house name |
| D11 | Decree picture `vendor-special` | DECIDED: the file is missing from `docs/art/landsraad/decrees/` although the folder README lists it; the five SpecialVendorActive decrees have no picture of their own | P2 | M2, M6 | A neutral fallback picture; never an invented URL |
| D12 | Raw row with XP and level that disagree | DECIDED: keep the exact advanced behaviour of `SetSpec`; warn; do not auto-correct | P1 | M5 | Under "Advanced", with the existing warning text (`app.js` 984) |
| D13 | Duplicate paid ground after a tier reset | DECIDED: the existing paid-ground record (`landsraad_paid.go`) is the only protection; the view adds no write path | P0 | M3 | Keep the existing sentence that a removal takes back nothing (`LB_REMOVE_NOTE`, `app.js` 1950) |
| D14 | Art outside the MIT licence | DECIDED: keep the notice, keep new art in the app's own embedded static folder and out of the `design` and `kit` modules | P1 | M2, M7 | Update `docs/art-notice.md` with any new folder |
| D17 | Ownership of the `design` and `kit` modules | DECIDED: no change in M1. A change needs a consumer that cannot work without it. #814 and #815 are audit items and are not used for this work | P1 | all | none |
| D18 | Old Specialization and Landsraad views | DECIDED: keep both, with the same backend, until the parity matrix passes | P0 | M5, M7 | The old views stay reachable |
| D19 | Size and fragility of `app.js` (3,383 lines) | OPEN: extract only in small, tested steps | P2 | M2 to M7 | none |
| D20 | No browser or screenshot run in M1 | DECIDED: nothing visual was run; screenshots and the dark, Dayside, 390, 768, 1280, 1600 and 200 percent checks are not claimed | P1 | M7, operator | none |
| D21 | Written versus queued wording | DECIDED: reuse the answer's `outcome` and the page's `NOT_SAVED` hint (`server.go` 515-545; `app.js` 165) | P0 | M3, M5, M6 | Never say "saved" when the answer says `queued` |
| D22 | A game patch changes goals, XP or the term schema | OPEN: validate at run time and show unknown rather than write | P1 | M1 onward | Every read validates; a failed read is an error of the page, not an empty board (`readFailed`, `landsraad.go` 14-16) |
| D15 | Static leaderboard and voting words in the approved mock | DECIDED: the mock's demo values are replaced by sourced values or "Not in the save" | P0 | M3, M6 | see D07, D08m |

## D. Things that need the operator on Windows (not answerable here)

1. Whether the game keeps a contribution the editor raised, or recomputes it from its own events (`landsraad_grant.go` 27-34).
2. Whether House Scrip and specialization XP are paid for a mission without a Recollection (named #745 in `landsraad_grant.go`).
3. What `landsraad_tasks.sysselraad` does around a vote (D02), and where the game records the vote start (D03).
4. Whether the reward rows the editor writes are collected by the game's own representative as an earned tier would be.

None of these blocks M2 (art and shell) or M3 (board and inspector over existing writes).

## E. Change control

A row changes only with evidence in the row. A new write route, a schema change, a live source for rankings or missions, a change to the `design` or `kit` public surface, or a new game-art licence question is a separate decision and is not covered by anything here.

## F. Decisions of M3 (the live board)

1. The classic list is the default view of the Landsraad tab and the fallback when the Command Center files are not loaded; the choice of view is remembered in the browser only.
2. The board has no write code of its own: its controls carry the classic list's `data-lb` attributes, so one handler, one request builder, one stale guard and one write queue serve both views (see `06-m3-live-board.md`, section 1.1).
3. On the board only, a single click that would replace another faction's win asks first. The classic list keeps writing at once; changing that is a decision for the classic list, not for M3.
4. A count of Houses is never called a win: the outline for a faction that holds more than half of the Houses is worded as a count, because the save has no vote-start snapshot (D03 stays open).
5. `--lcc-glow-win` (1) and `--lcc-outline-win` are separate from the plaque's `--lcc-glow`. The animated border is optional, needs Motion Full and no reduced-motion request, and is checked in the script and in the style.
5a. A board that fails its checks (not 25 Houses, a position outside 0 to 24, a duplicate) is shown as a read-only list with the reasons and no number.

## G. Operator-requested deviations from the approved mock (2026-11-18)

1. **The Decree Preview strip is removed and the Term and Decrees section takes its place (#923, to be tested).** The strip (a decree select, a "Term and decrees" link and "Sample decree only") sat under the plaque. The Term and Decrees section that was at the bottom of the page is now drawn there, once: the page seats its own section node into the board's slot (`data-lcc-slot="term"`) and puts it back before the board is drawn again, so the section's drop-downs, draft and open disclosures survive a redraw of the board. What the section writes is unchanged (the same `data-lb` controls, the same handlers, the same confirmations; the request goldens are untouched). The plaque shows the term's own active decree (`host.decree`) and has no sample mode, no DEMO badge and no preview state; `host.decrees` is gone. There is no link to the section any more, because it is where the link was; the section keeps the anchor `#lcc-term` and a heading (`#lcc-tm-h`) that can take the focus. The Decree pool and the Earlier terms stay in the section, closed by default behind a small Show / Hide control; the code, the rows and the tests are kept so they can come back. The classic list view is unchanged.
2. **Term start and end as days, hours and minutes (#922).** The save holds the start and the end of the term as play-clock seconds. The play clock stops while the game is closed and the save does not say when it stopped, so the real date of the start or the end cannot be worked out from it. What the save does hold: where the play clock stood when it was written (from the `farm_variables` row, the same value the completion time uses) and the real time of that write. The section shows the start and the end as play time before or after that point ("5 d 20 h 13 min of play after the save"), the length of the term, and the real time of the save in the viewer's local time. When the save has no usable clock the seconds are shown as stored. No date is shown for the start or the end. A countdown that refreshes every minute has nothing to count against, because the play clock does not move while the editor is open, so there is none. The Landsraad read gained one read-only field, `clock` (`play_seconds`, `saved_at`), and nothing else about it changed.
3. **The line "Your character is in <faction>." is removed from the board header (#930).** On the Command Center board only. The two lines that say why nothing can be changed (a character in no faction, a save with no character) stay; the faction is named where a control needs it ("(your faction)" in the faction boxes). The classic list is unchanged.

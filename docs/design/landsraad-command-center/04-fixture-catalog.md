# Command Center M1: fixture catalog

Synthetic fixtures for the Landsraad and Specialization Command Center work, with their provenance, what each one proves, the rules the expected outcomes encode, and the tests that guard them.

**Nothing here is a game save.** No account, character, guild or save row was read to make these files. House and decree names are the public game names; every number, id, name of a character, time and amount is made up. Faction ids 1 and 2 are the two ids the Landsraad has in the editor (`internal/ops/landsraad_factions.go` 19), used as stand-ins; "3" in one case is an invented id that the Landsraad does not have.

Source commit for the shapes: `c369dd926e5f5c40048eaeca33b665be07dd2b02`.

## 1. Files

| File | Size | What |
|---|---|---|
| `fixtures/board-victory-cases.json` | 62,524 bytes | the twelve lines and 28 board cases with expected counts, line status, and a proposed verdict |
| `fixtures/api-landsraad.synthetic.json` | 68,886 bytes | one complete answer of `GET /api/landsraad` (25 tasks, decrees, terms, one earlier term, the `grant` key) |
| `fixtures/api-player-specs.cases.json` | 16,669 bytes | 12 cases of `GET /api/player/specs`, each with the learned track numbers, the save rows and the expected answer |

How they were made: a throw-away script computed the expected facts, and the committed JSON is the source of truth. A second, independent implementation of the same facts is in `internal/ops/command_center_fixtures_test.go`, so a hand-edit that breaks a fact fails a test. The script itself is not committed.

A loader must drop every key that starts with an underscore (`_fixture` holds the notes and the source commit).

## 2. Board cases (`board-victory-cases.json`)

Notation: an owners string has 25 letters in `board_index` order: `A` Atreides (faction id 1), `H` Harkonnen (id 2), `O` open, `?` owner not readable. `board_index = 5 * row + column`. The lines are `row0` to `row4`, `col0` to `col4`, `diag-main` (0, 6, 12, 18, 24) and `diag-anti` (4, 8, 12, 16, 20).

Every case lists: the counts, the status of each of the twelve lines (`complete-A`, `complete-H`, `blocked`, `one-away-A`, `one-away-H`, `open-A`, `open-H`, `open-any`, `unknown`), complete lines, lines one house away (with the missing index), the count leader and margin, whether the leader holds 13 or more, the majority at the cutoff (and whether the cutoff is verified), a proposed verdict and a reason.


| Case | A / H / open | Complete lines | Cutoff | Verdict (proposal) | Winner | Reason |
|---|---|---|---|---|---|---|
| `all-open-0-0-25` | 0 / 0 / 25 | - | none | `none` | - | No line, no count leader, no verified cutoff |
| `atreides-12-harkonnen-4-open-9` | 12 / 4 / 9 | - | none | `pending_vote` | - | A leads the current count, but no verified Voting Session start exists: the vote is pending, not a result |
| `atreides-13-harkonnen-12-before-vote` | 13 / 12 / 0 | - | none | `pending_vote` | - | A leads the current count and holds 13 or more of 25, so the lead cannot be overtaken as a count, but no verified Voting Session start exists: the vote is pending, not a result |
| `atreides-12-harkonnen-13-before-vote` | 12 / 13 / 0 | - | none | `pending_vote` | - | H leads the current count and holds 13 or more of 25, so the lead cannot be overtaken as a count, but no verified Voting Session start exists: the vote is pending, not a result |
| `harkonnen-row2-low-total` | 8 / 5 / 12 | row2 | none | `line_confirmed` | H | Five Houses in one line are held by H; a line needs no count lead |
| `atreides-col3-low-total` | 5 / 6 / 14 | col3 | none | `line_confirmed` | A | Five Houses in one line are held by A; a line needs no count lead |
| `atreides-main-diagonal` | 5 / 5 / 15 | diag-main | none | `line_confirmed` | A | Five Houses in one line are held by A; a line needs no count lead |
| `harkonnen-anti-diagonal` | 4 / 5 / 16 | diag-anti | none | `line_confirmed` | H | Five Houses in one line are held by H; a line needs no count lead |
| `both-factions-complete-lines` | 5 / 5 / 15 | row0, row4 | none | `conflict` | - | Both factions hold a complete line; no first-win mechanic is verified, so no winner is chosen |
| `two-lines-same-faction` | 9 / 2 / 14 | row0, col0 | none | `line_confirmed` | A | Five Houses in one line are held by A; a line needs no count lead |
| `one-away-and-potential-lines` | 4 / 4 / 17 | - | none | `possible` | - | No line, no count leader; a House away from a line is a preview only |
| `atreides-25-harkonnen-0` | 25 / 0 / 0 | 12 lines | none | `line_confirmed` | A | Five Houses in one line are held by A; a line needs no count lead |
| `duplicate-board-index` | none | - | none | `unknown` | - | Board unavailable or incomplete (duplicate and missing board_index (the save schema forbids it inside one term; a hand-built or damaged save could carry it)): no line or count claim is made |
| `missing-task-24-of-25` | none | - | none | `unknown` | - | Board unavailable or incomplete (fewer than 25 tasks): no line or count claim is made |
| `index-out-of-range` | none | - | none | `unknown` | - | Board unavailable or incomplete (board_index outside 0..24): no line or count claim is made |
| `no-term-tasks-of-all-terms` | none | - | none | `unknown` | - | Board unavailable or incomplete (more than 25 tasks and repeated board_index (landsraad.go lines 37-43: with no integer term_id the query is not filtered by term)): no line or count claim is made |
| `unknown-owner-faction-3` | 1 / 1 / 22 (+1 unreadable) | - | none | `unknown` | - | An owner cannot be read, so counts and lines are not complete; no inference |
| `completed-without-winner` | 0 / 0 / 24 (+1 unreadable) | - | none | `unknown` | - | An owner cannot be read, so counts and lines are not complete; no inference |
| `winner-without-completion` | 0 / 0 / 24 (+1 unreadable) | - | none | `unknown` | - | An owner cannot be read, so counts and lines are not complete; no inference |
| `unknown-owner-on-a-line-with-a-win-elsewhere` | 5 / 0 / 19 (+1 unreadable) | row4 | none | `line_confirmed` | A | Five Houses in one line are held by A; a line needs no count lead (one owner unreadable, ignored) |
| `sysselraad-flag-set-no-evidence` | 12 / 4 / 9 | - | none | `pending_vote` | - | A leads the current count, but no verified Voting Session start exists: the vote is pending, not a result |
| `majority-8-6-at-verified-cutoff` | 8 / 6 / 11 | - | verified | `majority_confirmed` | A | More Houses than the other faction at the verified Voting Session start, whatever the current board shows |
| `majority-8-6-cutoff-not-verified` | 8 / 6 / 11 | - | none | `pending_vote` | - | A leads the current count, but no verified Voting Session start exists: the vote is pending, not a result |
| `majority-13-12-at-verified-cutoff` | 13 / 12 / 0 | - | verified | `majority_confirmed` | A | More Houses than the other faction at the verified Voting Session start, whatever the current board shows |
| `cutoff-differs-from-current-board` | 13 / 8 / 4 | - | verified | `majority_confirmed` | H | More Houses than the other faction at the verified Voting Session start, whatever the current board shows |
| `equal-count-7-7-at-verified-cutoff` | 7 / 7 / 11 | - | verified | `none` | - | No line and a tie at the verified cutoff |
| `line-and-majority-same-faction` | 5 / 3 / 17 | row0 | verified | `line_confirmed` | A | Five Houses in one line are held by A; the cutoff majority is also recorded |
| `line-for-a-majority-for-h` | 5 / 3 / 17 | row0 | verified | `conflict` | - | A line for A and a verified cutoff majority for H: conflicting saved outcome, verification needed |

Cases that need task rows (not only letters) carry a `tasks` list with the rows as a save would hold them: `duplicate-board-index`, `missing-task-24-of-25`, `index-out-of-range`, `no-term-tasks-of-all-terms`, `unknown-owner-faction-3`, `completed-without-winner`, `winner-without-completion`, `unknown-owner-on-a-line-with-a-win-elsewhere`, `sysselraad-flag-set-no-evidence`.

### 2.1 Verdict rules the cases encode

These are **proposals for M4**. They are not shipped behaviour and no Go test asserts a verdict.

1. A board is valid only when it has exactly 25 tasks, `board_index` is a whole number from 0 to 24 and appears once, and each house appears once. Otherwise the verdict is `unknown`, with no counts and no lines.
2. The owner of a house is read strictly: `completed = 1` with `winning_faction_id` 1 or 2 is that faction's; `completed = 0` with no winner is open; anything else is unreadable.
3. A complete line (five houses of one faction) wins for that faction, whatever the counts: `line_confirmed`. An unreadable house elsewhere does not cancel a line that is complete without it.
4. Complete lines for both factions: `conflict`, no winner (no first-win mechanic is verified).
5. A count lead with no verified cutoff is `pending_vote` and never a result, even at 13 or more of 25.
6. With a verified cutoff, the faction with more houses at the cutoff wins by count: `majority_confirmed`, whatever the current board shows, and also below 13. A tie is `none`.
7. A line for one faction together with a verified cutoff majority for the other is `conflict`.
8. A line for the same faction as the cutoff majority is a `line_confirmed` result with the majority also recorded.
9. Lines one house away are previews only (`possible`); nothing is won.
10. The `sysselraad` column is never evidence.

The save has no cutoff today (DECISIONS D03), so in production rules 6 to 8 cannot fire; they are specified now so that the evaluator and its tests do not have to be redesigned if a source is found.

### 2.2 What the Go tests check about these cases

`TestCommandCenterBoardHasTwelveFixedLines`, `TestCommandCenterBoardCasesAgreeWithTheGeometry` and `TestCommandCenterTugTrackHasTwentyFiveSeatsWithOneMiddleSeat` in `internal/ops/command_center_fixtures_test.go` check that: the file's lines are the rows, columns and diagonals; every cell is on 2, 3 or 4 lines; for every valid case the counts, the status of all twelve lines, the complete and one-away lines, the leader, the margin and the 13-or-more flag are what an independent computation of the letters gives; each of the twelve lines is complete in at least one case; a verified cutoff has a snapshot and its counts; a confirmed winner has the evidence its verdict needs; an invalid board has no counts and no lines; and the tug track fills from both ends with one middle seat. Verdicts beyond those two facts (`pending_vote` before a cutoff, `majority_confirmed` only with a verified cutoff, `conflict` when both factions hold a line) are asserted only as consistency rules, not as the full rule set.

## 3. The Landsraad answer (`api-landsraad.synthetic.json`)

The answer has the keys the server sends (the test compares them with the real `Ops.Landsraad` over a built save, and with the route's `grant` key): `term`, `tasks`, `decrees`, `rotation`, `factions`, `character`, `terms`, `earlier_terms`, `locations_from_markers`, `grant`.

- 25 tasks, `board_index` 0 to 24 in order, the houses deliberately not in alphabetical order (the test fails if they are). The owners are those of the case `atreides-12-harkonnen-4-open-9`.
- `goal_amount` is 4,000 on every task; the tiers are 1,200 / 2,400 / 3,600 / 4,800 / 6,000 and a contribution can exceed the goal (task 9005 at 6,500).
- Scenarios (ids 9001 to 9025; the id is `9001 + board_index`):

| Task | House | What it shows |
|---|---|---|
| 9002 | Richese | one tier reached and collected |
| 9003 | Kenola | reached with no delivery row of this term (earlier-term row only): `no_row` |
| 9004 | Alexin | one collected, one waiting; the game's own events cover 2,500 |
| 9005 | Thorvald | all five reached, the top tier is the swatch, not held, waiting |
| 9006 | Moritani | four reached: delivered, waiting, delivered, `no_row` in one house |
| 9007 | Hagal | swatch won this term: the top tier is still the swatch |
| 9008 | Wallach | three reached, all waiting |
| 9009 | Sor | contribution reached under the other faction (ground a raise does not pay again) |
| 9011 | Argosaz | swatch held from an earlier term: the top tier is the house's own item |
| 9013 | Mutelli | centre house with no contribution rows at all (null lists) |
| 9025 | Ecaz | a task with no reward rows (missing reward catalog) |

- The test asserts: 25 tasks by position, each house once, `line` equal to `lineOfHouse`; all four `reward_state` values occur; `reached`, `delivered` and `swatch` agree with each other and with the contribution; at least one task has null lists (Go marshals an empty list as null); the keys of every object match the real answer; an unknown place carries only `known: false`.

## 4. The Specialization cases (`api-player-specs.cases.json`)

Combat is the track each case varies; the other four tracks have their own values only in `mixed-five-tracks`.

| Case | Combat number | Combat XP | Stored level | Keystones | Unassigned rows | What it shows |
|---|---|---|---|---|---|---|
| `xp-0-keystones-0-of-41` | 4 | 0 | 0 | 0 / 41 | 0 | Fresh track: no row, no keystones |
| `xp-1` | 4 | 1 | 0.01 | 0 / 41 | 0 | One XP: level is a fraction of the first level |
| `xp-44181-near-cap-keystones-20-of-41` | 4 | 44,181 | 99.99839743589743 | 20 / 41 | 0 | One XP below the cap with half the keystones |
| `xp-44182-cap-zero-keystones` | 4 | 44,182 | 100 | 0 / 41 | 0 | XP cap reached with no keystone: level 100 is not keystone completion |
| `xp-0-all-keystones-41-of-41` | 4 | 0 | 0 | 41 / 41 | 0 | No XP with every keystone: keystone completion is not level 100 |
| `both-complete` | 4 | 44,182 | 100 | 41 / 41 | 0 | XP cap and every keystone |
| `keystone-total-zero-no-map` | 4 | 44,182 | 100 | 0 / 0 | 0 | A save with no keystone map: total 0 is never complete |
| `mixed-five-tracks` | 4 | 12,345 | 46.148148148148145 | 7 / 41 | 0 | Five different states at once |
| `conflicting-learned-number-leaves-combat-unresolved` | none | 0 | 0 | 0 / 41 | 1 | A learned number 4 for Crafting takes the measured number of Combat: Combat has no number and the old Crafting row waits as unassigned |
| `unassigned-row-number-9` | 4 | 2,000 | 14.190721649484535 | 0 / 41 | 1 | A row of the save with a number no track has: shown apart, left alone by Reset XP of all known tracks |
| `raw-row-xp-44182-level-10-mismatch` | 4 | 44,182 | 10 | 0 / 41 | 0 | The advanced raw row writes XP and level as typed: stored level 10 against 100 computed from the XP |
| `near-cap-44000-room-182` | 4 | 44,000 | 99.70833333333333 | 0 / 41 | 0 | Combat at 44,000 XP: the Landsraad grant preview of api-landsraad.synthetic.json says 182 XP of room |

Each case builds a save with the keystone map and purchases it lists and calls the real `Ops.Specs`; the answer must equal the fixture's `response` exactly (`TestCommandCenterSpecFixturesAreWhatSpecsAnswers`). So the fixture cannot drift from the shape or the arithmetic of the code.

Not covered, by design: per-track keystone edits (not supported, DECISIONS D08), the write answers of the eight Specialization write routes (existing tests in `internal/ops/specialization_test.go` and `internal/ops/reset_level_test.go` cover them).

## 5. Other pure invariants tested

| Test | What it checks |
|---|---|
| `TestCommandCenterTierStateClassifiesWaitingDeliveredAndNoRow` | `tierState`: not reached, no row, a row of an earlier term, a row at the term start, waiting, delivered |
| `TestCommandCenterRequestShapesTheBoardSends` | `parseApplyItems` accepts what `lbItem` sends (tier, removal, Fill faction goal, Complete, Reopen, bulk) and refuses a house twice, no change, complete with reopen, a text boolean, a negative or fractional value, a non-object, and more than 100 houses |
| `TestCommandCenterHouseLinesAreFiveHousesOnEachOfTheFiveLines` | 25 houses, five per line, names keyed without the prefix, unknown names have no line |
| `TestCommandCenterArtFilesNameTheHousesAndLinesTheEditorKnows` | each of the 25 houses has a reference PNG and a served webp (with the Mikkarol spelling alias), each line has a lane and a banner picture |
| `TestCommandCenterXpTableHasOneHundredOneLevelsAndNeverGuessesALevel` | 101 thresholds, strictly increasing, 0 to 44,182, exact level at each threshold, never decreasing, not linear |
| `TestCommandCenterTrackNumbersAreOneNumberPerTrack` | measured numbers are one per track; whatever number is learned, no two tracks share a number; a learned number on another track's measured number leaves that track unresolved; `CheckAssign` and `checkTracks` refuse a collision |

## 6. Limitations

- The fixtures of this milestone are tested in Go only. A browser-engine harness does exist (`web-tests/`, jsdom, run by `node --test`, CI job `ui`) and has Landsraad suites (`landsraad-board.test.mjs`, `landsraad-board-write.test.mjs`, `landsraad-decrees.test.mjs`, `landsraad-errors.test.mjs`, `landsraad-kept-decisions.test.mjs`, `landsraad-queue-text.test.mjs`, `landsraad-symbols.test.mjs`), but it loads the current page and its own fixtures generated from the real handlers (`TestDumpUIFixtures`); it was not run in M1 (it needs `npm ci`) and these JSON files are not wired into it. Nothing here tests how a page draws.
- The evaluator for the victory rules does not exist. The verdict column is a specification, not a test result.
- The fixtures describe the answer shape at one commit. A change in the server's answer fails the shape tests, which is intended.
- A test in `internal/ops` reads the fixtures with a relative path (`../../docs/design/landsraad-command-center/fixtures/`), so the files must stay where they are or the test must change with them.

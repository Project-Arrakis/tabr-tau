# Findings register, 2026-11-14: full audit of the editor (tabr-tau)

Part of the audit tracked in [#606](https://github.com/Project-Arrakis/tabr-tau/issues/606). Range: `5b7a3d51` (main on 2026-10-27) to `c71a85d` (main on 2026-11-14): 82 commits (78 by the owner, 4 by Dependabot), a diff of 388 files. It covers the Landsraad rework and the board's reward delivery (#636 to #648), vehicle placement, base import and export, the live-save rules, the newer save layout, the two nested modules `kit` and `design`, the shared look, and the fixes for #611, #613 and #621. It continues `docs/audit/2026-11-13-delta-register.md` (#611 to #623, which stays valid and is cited, not repeated) and sits beside the remediation plan `docs/audit/2026-11-14-remediation-plan.md`.

**Nothing was fixed by the audit.** No finding was acted on, nothing was merged, and no save or game file was written. Single player only: no multiplayer findings.

## Method

Eight independent read-only reviews by role, each run on the opus model, then one high-effort code review of the same range:

| Hat | Lens |
|---|---|
| Architect | module boundaries and layering, write paths, readers, coupling of page, server and writer, the shared module design, concurrency, testability |
| Security architect | threat model (STRIDE), the request gate, the embedded window, the supply chain of the window wrapper, durable writes |
| GRC | governance, risk and compliance: the public-text rule, notices and provenance, traceability, scope statements, repository controls |
| Network | listeners, Host and Origin, CORS, CSP, static files, outbound connections, the embedded window |
| Cloud security | Actions, branch and tag protection, Dependabot, the scanners, releases, dependencies, history |
| UI/UX | accessibility, consistency and information design, measured in the page |
| DBA | schema and layout handling, every SQL statement, transactions, id allocation, the Landsraad event and reward rows, hostile saves |
| QA and test | a local run of the whole suite, coverage, CI history, a regression sweep of every closed fix, the QA lines |
| Code review (high effort) | eight angles over the diff (line by line, removed behaviour, cross-file, reuse, simplification, efficiency, altitude, conventions), write paths first, with small scratch tests |

Every hat read the code at `c71a85d` (the Architect, Security, Network, DBA and QA hats at origin/main of that commit), read the earlier registers first, and cited file and line. "Reproduced" means seen in a run on a scratch copy of a real single-player save (newer layout, term 6); "read" means established from the code and not run. The scratch tests were never committed. The line numbers are those the hats cited at `c71a85d`; a spot check of a sample found them current (one test line is 227, not the 229 first cited).

Local results (QA hat, a clean export of `c71a85d`, offline, with the user folders redirected): the root module's 23 packages, 1,942 tests, all pass, 9 skipped; `kit` 113 tests pass; `design` 172 tests pass; statement coverage of the root module 84.2 percent (`internal/ops` 89.2, `internal/web` 90.5, `internal/save` 83.3).

**What each hat could not reach.**

- Architect: no build and no test run; code read only.
- Security: the raw SQL routes (`/api/db/sql`, `/api/db/exec`) and the name handling of `/api/save/restore` are older and were not re-read; the Landsraad reward rows were read for gate use only.
- GRC: issue and PR text before 2026-10-27 (beyond two older issues seen in passing); the provenance of the art pictures inside the game's files.
- Network: no server was started and no window opened; whether a popup inherits the window's settings and whether a dropped file raises a filter event are runtime questions.
- Cloud: read calls only against the repository settings; the scanners' internals are read from their logs, not run.
- UI/UX: measured in a headless browser on a copy of a save with no write; PR #652 was read but not run; touch behaviour was emulated.
- DBA: the data checks ran on a decoded copy opened read-only; what the game does with an event above its cursor, and how it writes `game.db`, are not established.
- QA: the jsdom page tests were not run (no installed dependencies, nothing installed) and no vulnerability scan was run (CI runs it); #499 and #480 were not checked string by string.
- Code review: `internal/basefile/console.go` (second half), `consolecheck.go`, `consolewrite.go` and `internal/ops/baseconsole.go` were read for structure only; the geometry of `vehicle_place.go` and `places.go`, `place_taxis.go`, `landsraad_locations.go` were outlined or run once, not read line by line.

Also folded in, as reported in play and already filed: #650, #644, #646, #647, #651, #641 (and #645, closed by #648) and the open PR #652; see "Reported in play" below. PR #652 was not reviewed again here.

Severity scale: Critical (data loss or a safety problem), High, Medium, Low, Info. STRIDE letters are left to the hats' own reports; the Medium and High rows name the kind of harm in plain words.

## Result

| Severity | Count |
|---|---|
| Critical | 0 |
| High | 3 |
| Medium | 11 |
| Low | 30 |
| Info | 24 |
| Total | 68 |

From 75 raw hat findings: 7 were merged into others by deduplication (every hat id is kept in the "Hats" column).

| Hat | Raw findings | Critical | High | Medium | Low | Info |
|---|---|---|---|---|---|---|
| Architect (A-U) | 7 | 0 | 0 | 2 | 1 | 4 |
| Security architect (S-U) | 5 | 0 | 0 | 1 | 2 | 2 |
| GRC (G-U) | 11 | 0 | 2 | 2 | 6 | 1 |
| Network (N-U) | 4 | 0 | 0 | 0 | 1 | 3 |
| Cloud security (C-U) | 8 | 0 | 0 | 1 | 1 | 6 |
| UI/UX (U-U) | 10 | 0 | 0 | 1 | 7 | 2 |
| DBA (D-U) | 5 | 0 | 0 | 1 | 3 | 1 |
| QA and test (Q-U) | 8 | 0 | 0 | 1 | 3 | 4 |
| Code review (CR-U) | 17 | 0 | 1 | 2 | 8 | 6 |
| Total | 75 | 0 | 3 | 11 | 32 | 29 |

No finding loses data on its own or breaks the loopback and same-origin rules. The High findings are: a character's place read from a parked body, so a vehicle can be written to a map the player cannot walk to (G-001, reproduced); and two parts of the public-text rule, public issue and PR text and the leak guard's own labels (G-002, G-003). The Medium findings are in four areas: the Landsraad reward delivery (progress events above the game's cursor, once-per-term keyed per item, two write models, a queued teleport, a page-wide redraw), the live-save gate, the base import ceiling, and repository settings and records.

## Groups and issues

One issue per group of related findings (title prefix `audit F`), or a comment on the open issue that already covers the group. INFO findings are in this register only (a few ride along in the issue of their group).

| Issue | Priority | Findings | Kind |
|---|---|---|---|
| [#653](https://github.com/Project-Arrakis/tabr-tau/issues/653) audit F1: public text rule | p1-now | G-002, G-003, G-011, G-027 to G-030 | new |
| [#654](https://github.com/Project-Arrakis/tabr-tau/issues/654) audit F2: the character's place from the body alone | p1-now | G-001, G-008 (G-068 info) | new |
| [#655](https://github.com/Project-Arrakis/tabr-tau/issues/655) audit F3: progress events above the cursor | p2-next | G-004, G-015, G-016 | new |
| [#656](https://github.com/Project-Arrakis/tabr-tau/issues/656) audit F4: rewards once per term per item | p2-next | G-005, G-017 | new |
| [#657](https://github.com/Project-Arrakis/tabr-tau/issues/657) audit F5: two Landsraad write models | p2-next | G-006, G-018, G-019 (G-047, G-067 info) | new |
| [#658](https://github.com/Project-Arrakis/tabr-tau/issues/658) audit F6: base export against the stated scope | p2-next | G-012 | new, decision |
| [#659](https://github.com/Project-Arrakis/tabr-tau/issues/659) audit F7: board accessibility and touch sizes | p3-later | G-034 to G-040 (G-060, G-061 info) | new |
| [#660](https://github.com/Project-Arrakis/tabr-tau/issues/660) audit F8: the embedded window | p3-later | G-024, G-025 (G-048 to G-051 info) | new |
| [#661](https://github.com/Project-Arrakis/tabr-tau/issues/661) audit F9: scanner config from the PR, a commit without a PR | p3-later | G-031, G-033 (G-053 to G-059 info) | new |
| [#662](https://github.com/Project-Arrakis/tabr-tau/issues/662) audit F10: small defects and drift | p3-later | G-041 to G-044 | new |
| #612 (D2) | p2-next | G-007 | comment |
| #613 (D3) | p2-next | G-009, G-021 (G-045 info) | comment |
| #615 (D5) | p3-later | G-020 | comment |
| #617 (D7) | p3-later | G-022 | comment |
| #619 (D9) | p3-later | G-026 | comment |
| #621 (D11) | p2-next | G-032 (and a pointer to #653) | comment |
| #622 (D12) | p2-next | G-013 (G-046 info) | comment |
| #623 (D13) | p3-later | G-023 | comment |
| #650 | p2-next | G-010 | comment |
| #567 | p3-later | G-014 | comment (#594 is closed) |
| #609 | p2-next | a pointer to #655 and #656 | comment |

## Findings

Sorted by severity. "Hats" gives every hat id that found the same thing. An issue number in the last column is the issue or comment that carries the finding; "register" means INFO, kept here only.

### High

| ID | Sev | Hats | Title | Location | Evidence | Recommendation | Issue |
|---|---|---|---|---|---|---|---|
| G-001 | HIGH | CR-U1 | "Next to the character" reads the character's place from the body actor alone, and a real save can park the body on the travel map | `internal/ops/world.go:162-182` (BringVehicle), `vehicle_spawn.go:446-470`, `vehicle_duplicate.go:108-126`, `teleport.go:58-69` and `:196-208`, `landsraad.go:54` | Reproduced on a scratch copy: the body and the ornithopter it sits in are on map `Overland` while the controller and player state are on an instanced map. Bring vehicle on a buggy from the Deep Desert answered "14 m north-west of you" and moved the buggy to `Overland` at the parked body's coordinates; the page reports success and the player cannot walk to it. Create and Duplicate write to the same place; a same-map Teleport moves only the parked body. | One helper inside the edit that reads controller, body and player state; refuse "next to the character" (or ask for an explicit place) when the body's state is not 0 or the three are not on one map; treat a teleport as a move between maps. Add a fixture of this shape. | #654 |
| G-002 | HIGH | G-U1 | Public issues and PRs carry wording that the public-text rule forbids | issues #571, #570, #584, #587, #649 (bodies) and a comment on #571; PRs #589, #590 (bodies); older #426, #437 | The bodies and the comment contain wording the rule forbids (the rule behind `internal/hygiene` and #621). No control covers issue, PR or comment text: the hygiene test reads tracked files only. A recurrence of a boundary finding of 2026-10-22 that was fixed for one issue only. Described by number only, on purpose. | Edit the listed items (the owner's call); put cross-repository links only on the private side; add a job over issue, PR and comment text that runs the same digest check. | #653 |
| G-003 | HIGH | G-U2 | The leak guard's labels and comments, and the records around it, say what is kept out | `internal/hygiene/hygiene_test.go:1, 19-23, 60, 89`; `design/files_test.go:364`; `CHANGELOG.md:16`; `docs/audit/2026-11-13-delta-register.md:227, 233, 234` | The digests hide the words, but plain-text labels and comments tell a reader what the guard keeps out, and the labels are printed in a failing test's message (:89). #627 did not touch these lines. Described by file and line only, on purpose. | Neutral labels; reword the package comment, the test message, the CHANGELOG entry and the register paragraphs; add digests for the old wording so the guard covers its own text. | #653 |

### Medium

| ID | Sev | Hats | Title | Location | Evidence | Recommendation | Issue |
|---|---|---|---|---|---|---|---|
| G-004 | MEDIUM | A-U2, D-U5, Q-U7 | The board writes progress events above the game's cursor in a shape the game is not known to write; the consumer of that queue is unverified | `internal/ops/landsraad_rewards.go:15-27, 53-67`; `landsraad_apply.go:229-241`; `landsraad_rewards_test.go:59-266` | The event goes above `last_processed_id` and the reward rows it would explain are written at once. A tier click or a Fill without Complete writes `player_progress > 0` with `faction_progress 0` (the design note says the game writes 0 only at the goal). If the game re-applies an unprocessed event, the contribution doubles or a reward is delivered twice. The DBA hat found that the game itself saves events above its cursor with the effects already applied (cursor 395 of 396), which makes a re-application less likely; what processing does is still unknown. The event id rule `max(max(id), cursor)+1` assumes the game has no counter of its own. The tests prove the rows, not the game's use of them. Not reproduced. | One in-game test on a copy (raise, save, load, play until a real event passes the editor's id, compare contribution, reward rows and the event sum, confirm the next id). Until then label the delivery as unverified in the tab, README and changelog. Operator decision (see the plan). | #655 |
| G-005 | MEDIUM | D-U1, A-U3, CR-U12 | "Delivered once per term" is keyed per item, so real tiers are skipped as already given; four rules for "delivered" disagree | `landsraad_rewards.go:113-135`; `landsraad.go:143-163`; `landsraad_board.go:84-96, 165-171, 81`; `landsraad_rewards.go:102` | On a copy: collecting sets `amount` 0 with a new time, so in the next term every tier whose item repeats is skipped (22 of the 125 term-6 tier rows repeat a term-5 item; House Maros's collected Solari is not given again); 11 of 150 tasks list one item at two thresholds, and the second tier is skipped when the two are crossed by separate edits. The reader's own comment admits it cannot tell equal items apart. Answers the architect's question: such houses exist. Not run as code. | Decide per tier: deliver only thresholds above the larger of the old contribution and the character's own event sum for the task; one `tierState` for read and write; tests with these rows. Operator decision (see the plan). | #656 |
| G-006 | MEDIUM | A-U1, Q-U3 | Two write models of the Landsraad: the board's apply delivers rewards, seven older routes the page no longer uses write the same rows without them | `internal/web/server.go:840-846` next to `:847`; `landsraad_goal.go:115-119`; `landsraad.go:446-471`; `landsraad_apply.go:229` | The page calls only `/term`, `/apply`, `/decree`. The old routes stay registered and tested, with their page tests deleted by #637. A raise through them moves a contribution to the top tier with no event or reward, so the board then sees nothing to raise. Every rule change must be made twice (the D1-1 fix was done twice). | Retire the routes, or build `applyItem`s and call the apply core; ask first whether anything outside the page calls them. | #657 |
| G-007 | MEDIUM | S-U1 | The unattended automatic pass lost its independent "no game process" check | `internal/save/durable.go:249-252`; `flush.go:64`; `internal/web/server.go:1138-1150`; commit 7edcfc1 (#505) | Before #505 the launch pass refused while any game process existed, whatever the log said. Now it rests on the log marker gate alone (D2-1) with quiet margins of 30 s and 23 s (D2-2). A hand edit has a person watching; the pass runs without one at every launch, jobs on by default. A single point of failure where there were two signals. | Restore the process rule for the unattended path, or require the second independent signal D2-1 asks for. Operator question (see the plan). | #612 |
| G-008 | MEDIUM | CR-U3 | A queued same-map teleport is put back onto a save in which the character is on another map | `internal/ops/teleport.go:186-208`, `:58-69`; `internal/save/journal.go:686-730` | Read, not run. "Same map" is decided outside the edit and the edit has no read, so `replayOp` has nothing to hold it by. Queue a Hagga Basin move, travel to the Deep Desert and leave: the body gets Hagga coordinates while its map stays `DeepDesert`. | Do the move in one `Mutate` that reads the body's map (and the state of G-001) through `m.One` and writes map and position together. | #654 |
| G-009 | MEDIUM | CR-U2 | An imported health of up to 1,000,000 still becomes the "best seen" and is copied onto the player's own bases (residual of #613) | `internal/basefile/values.go:22, 48`; `internal/ops/world.go:40, 53-66`; `basestructures.go:102-111` | Read, not run. The ceiling is 1,000,000 against real figures of 7,000 (foundation) and 2,500 (placeable); `bestHealthOf` counts pieces of an imported backup that is not standing. Repair base then raises every wall of that type to the file's value. | Take the best only from standing bases, or cap per type at the highest value before the import; better, a table of full health per building type. | #613 |
| G-010 | MEDIUM | U-U1 | Six more in-page redraws scroll the page to the top and leave the focus out of view, beyond the part swap of #650 | `app.js:1518-1524, 2738-2746`; `render()` with no argument at 2389-2412, 2510-2511, 2638-2654; the rule at 2331 | Measured at 1280 x 700: class change 1067 to 0 px, part slot 1545 to 0, open a vehicle 547 to 0, open a base 569 to 0, a base sub-tab 794 to 0, an inventory group 736 to 0. WCAG 2.4.3 and 2.4.11. | `render('scroll')` (or `render(true)`) for every in-page change; scroll a focused control into view; a web test per click from a scrolled page. | #650 |
| G-011 | MEDIUM | G-U3 | A public issue body contains a personal Windows path with the account name | issue #470 (2026-10-28), body, the first question | The full log folder path under a user profile is public. The editor's own rule (SEC-10, D9-1) keeps this name out of page text. | Edit the body to use the environment variable form; add a profile-path pattern to the scan of F1. | #653 |
| G-012 | MEDIUM | G-U4 | A public document presents the editor as the way to take a single-player base into a multiplayer game, against the README's scope | `docs/export-a-base-for-the-console.md:4, 128-132`; `README.md:157` against `:16-17`; the title of #534 | The README says single-player on the person's own PC, not for online-server characters; the export document and #534 state the purpose as moving a base into a multiplayer game. No decision record ties the two. | Record on #534 which servers are meant, then reword the document, `README.md:157` and the title, or state the exception in the README scope paragraph. Operator decision. | #658 |
| G-013 | MEDIUM | C-U1 | Branch and tag protection of the public repository (extends D12-2) | repository rulesets (none); `required_approving_review_count` 0; `kit-release.yml:7-10, 36-42`; `design-release.yml:97-100, 126-132` | The release workflows run on the tag push from the tagged commit's own workflow file, so the "commit is on main" check is only as strong as who can create the tag; the Go proxy serves any `kit/vX.Y.Z` tag whatever the workflow's result, and a published version can only be retracted. | A tag ruleset for `kit/v*` and `design/v*` limited to the owner; one required approving review on `main`; `/kit/` and `/design/` in CODEOWNERS. | #622 |
| G-014 | MEDIUM | Q-U1 | #594 was closed as superseded, but its flaky assertion is unchanged and failed a `main` push after it was filed | `internal/save/spsession_long_test.go:227` (`strings.Contains(g.Reason, "256")`) | The reason carries the random temp path; the push run 43282797564 on `main` (2026-11-12) failed on it. The only comment on #594: "Closed 2026-11-13: superseded; operator confirmed". No commit touched the line since #566 (2026-11-08); #567 is the same bug and is still open. Neither issue's fix added a test. | Fix under #567: assert on the limit wording ("256 MiB") or strip the path first. | #567 |

### Low

| ID | Sev | Hats | Title | Location | Evidence | Recommendation | Issue |
|---|---|---|---|---|---|---|---|
| G-015 | LOW | D-U2, A-U4 | Contributions are the running sum of events in every game-written row; three editor paths break that, and a removal leaves delivered rewards above the contribution | `landsraad_apply.go:229-268`; `landsraad_rewards.go:61-62`; `landsraad_goal.go`; `landsraad.go:446-471` | On a copy 315 of 340 rows equal their event sum; the 25 that differ are earlier editor Fills with no events. A removal writes no event and keeps the reward rows; a re-raise writes a second event (reproduced: events 6,000 plus 1,200 for a contribution of 6,000). If the game ever rebuilds from events, these rows would move. Documented in the code (A-U4). | Show houses whose contribution differs from the event sum (read only); no second event for a range already in the events; say in the removal's confirm that delivered rewards stay; retire the old routes. | #655 |
| G-016 | LOW | CR-U8 | With PR #652 each click writes at once, so each click leaves an event and a slip cannot be taken back | PR #652 branch (page only); `landsraad_rewards.go:53-67` | Branch diff read for this point only. Three clicks up a house are three events with `faction_progress 0`; a mistaken click on tier 5 writes five reward rows, the swatch and an event; removing it keeps all of them. | Settle G-004 in the game before #652 merges; or ask before a click that crosses the top tier. | #655 |
| G-017 | LOW | CR-U7 | A raise whose rewards were skipped as "already given" is reported as a plain success | `app.js:1957-1961`; `landsraad_apply.go:304` | Reproduced on the server side: remove the top tier and give it again, `reward_rows: 0, rewards_already_given: 1`, and the toast says the contribution was raised. With G-005 the same happens for real tiers. | Add "N rewards not written: already delivered this term" as a warning. | #656 |
| G-018 | LOW | CR-U5 | The board accepts a faction the game's Landsraad never uses | `landsraad_apply.go:158`; `landsraad_chosen.go:154`; `landsraad_goal.go:60`; `landsraad.go:200`; `app.js:1926, 1878` | "A faction the Landsraad has" is `name <> 'None'`, which lets Smuggler (id 4) through; six terms of game data hold only factions 1 and 2. Reproduced: `complete_for: 4` completed an open house. | Offer and accept only factions with Landsraad rows, in one helper. | #657 |
| G-019 | LOW | CR-U6 | The page lets a person stage a change the server refuses, and the refusal stops every pending house | `app.js:1702, 1716-1717`; `landsraad_apply.go:188-190` | Read. A staged tier plus Complete for the other faction gives 422 for the whole request and the message names one house. PR #652 removes staging; the server rule stays. | Re-check after #652; keep one rule. | #657 |
| G-020 | LOW | D-U3 | The board's apply creates rows in four tables but checks only that two exist (extends D5-1) | `landsraad_rewards.go:61, 65, 142, 148`, `:79`, `:158-170` | The fourth row-creating edit and the only one that writes into a queue the game consumes. A new NOT NULL column fails safely; a new defaulted column or a new table gives rows the game did not write in that shape. | Add these tables to the D5-1 layout check. | #615 |
| G-021 | LOW | CR-U9 | What the base file check does not cover (extends D3-1) | `internal/basefile/values.go:55-101` | Read. One extra key of another shape makes the durability decode fail and the check pass any value; no rule covers `max_item_volume`, `volume_override`, `quality_level`, `shelter`, `sand_buildup`, positions or totem coordinates. | A decoder that reads only that key; a range per remaining number, or say in the preview which columns are unchecked. | #613 |
| G-022 | LOW | CR-U10 | The hub's "arrival point only" rule is skipped when the character is already in the hub (extends D7-1) | `teleport.go:186-195`, `:252-260` | Read. `map: "Arrakeen"` with any x, y, z takes the same-map branch, bounded only by 1e7. | Apply the destination map's rule in both branches. | #617 |
| G-023 | LOW | D-U4 | The live save is read without the delete share mode while the game may be saving | `kit/savefile/codec.go:37-54`; `internal/save/save.go:274, 357, 398`; `settle.go:26`; `gameproc_windows.go:39-50` | Go's Windows open shares read and write but not delete; if the game saves by renaming a new file over `game.db`, an overlapping read can make its rename fail. How the game writes the file is not established. | Open with all three share modes on Windows in `ReadCapped`. | #623 |
| G-024 | LOW | S-U2 | The upstream WebView2 wrapper is used unpatched: COM results judged by last-error, a buffer not kept alive | `internal/gui/gui_windows.go:31-42`; `go.mod`; upstream `corewebview2.go:144-151`, `ICoreWebViewSettings.go:171-181`, `webview.go:113-127`, `chromium.go:87` | A stale last-error ends the editor before the window opens; a failed HRESULT with last-error 0 reads as done and could leave dev tools and the context menu on; the profile path is passed without a keep-alive. | Vendor or fork with HRESULT checks, or set `WEBVIEW2_USER_DATA_FOLDER` and re-read the two settings. Owner's call whether to vendor. | #660 |
| G-025 | LOW | N-U1 | Ctrl, Shift or middle click on the page's own links opens a popup outside the window guard | `gui_windows.go:54-66, 80-97`; `app.js:963, 1807-1808, 1814, 1895, 1908, 2168` | `stay` cancels only cross-origin clicks; a same-origin modified click opens a default popup that is outside the document filter, the init script and the close confirm, and holds the session cookie. Same origin; not run. | `preventDefault` on modified and middle clicks; handle NewWindowRequested when the wrapper is patched. | #660 |
| G-026 | LOW | S-U4 | The native window title carries the full path of a copy | `cmd/tabr-tau/main.go:243-248` | The title shows in the taskbar, Alt+Tab and screenshots, so a copy under the home folder shows the Windows user name (same rule as SEC-10, D9-1). | Show the base name. | #619 |
| G-027 | LOW | G-U5 | Comments and example labels in the design module fall under the public-text rule and are in no digest | `design/assets/tokens-fremen.css:106`, `tokens-awakening.css:97`, `tokens-sietch.css:83`, `components-fremen.css:56`, `components-awakening.css:72`, `components-sietch.css:43`, `design/kitchen-sink/index.html:47, 91-93` | Described by file and line only, on purpose. | Neutral comments and generic labels; add the words to the digests. | #653 |
| G-028 | LOW | G-U6 | The sentence removed by #627 is still in public history | commit d22f66d (adds it), commit 1495281 (removes it; its diff repeats it) | The tree is clean; history is public. A rewrite breaks clones and PR links. | Accept as residual risk and record it on #621, or decide on a coordinated rewrite. | #653 |
| G-029 | LOW | G-U7 | The commit-message rule is not enforced on the PR title and body, the source of every squash message | `scripts/check-commit-messages.sh:12-17`; `.github/workflows/ci.yml:281-295` | The check reads the PR branch's commits; the push check runs after the merge. Nothing breached in 82 commits, 99 PRs and 124 comments. | A job over the PR title and body on opened, edited and synchronize. | #653 |
| G-030 | LOW | G-U8 | The PR and issue templates have no public-boundary line | `.github/pull_request_template.md`; `.github/ISSUE_TEMPLATE/bug.yml`, `finding.yml` | G-002 and G-011 are text written through these templates. | One checklist line and one form note. | #653 |
| G-031 | LOW | G-U9 | A commit reached `main` with no PR, two minutes before its PR's empty squash | commits 38e0806 (2026-11-12 23:51) and b9bca07 (PR #604) | The only commit of 82 without a PR; with `enforce_admins` and a PR required it should not exist. How it was made is unknown. | Find out (web edit, API, a bypass list); remove any bypass. | #661 |
| G-032 | LOW | G-U10 | The house-symbol art has a notice but no provenance, and no test pins the art folders (same class as D11-3) | `internal/notices/THIRD-PARTY-NOTICES.md:464`; `docs/art-notice.md:9`; `internal/notices/notices_test.go:21-35` | 25 pictures "taken from the game's own interface pictures": no build, no source asset names, no method (the data files have them at :505-517). The test lists 11 data files and no art folder. | Record build, asset names and method; extend the test to `maps/` and `houses/`. | #621 |
| G-033 | LOW | C-U2 | semgrep and trivy take their ignore and config files from the pull request | shared scan `reusable-security-scan-v2.yml@2b8bd347` lines 236-247, 264-265 (gitleaks reads its config from the default branch, lines 12-17) | A PR's `.semgrepignore`, `.trivyignore` or `trivy.yaml` applies to its own scan. Fork PRs need first-time approval. | Fail CI when a PR adds or changes them, or read them from the default branch; CODEOWNERS. | #661 |
| G-034 | LOW | U-U2 | The Task board region is named by its whole content | `app.js:1896` (and `:1914` for the correct pattern) | `aria-labelledby="lsb-board"` names the section by itself; the accessibility tree reads the whole text. | `aria-labelledby="lsb-board-h"`. | #659 |
| G-035 | LOW | U-U3 | The board's ARIA table has column headers but no rows or cells | `app.js:1910-1911, 1780` | In the accessibility tree the rowgroup holds the checkboxes directly; no `row` or `cell`; the list semantics are gone too. | Row and cell roles, or a plain list with the house name as label. | #659 |
| G-036 | LOW | U-U4 | The line that says what a board click did is replaced with the board, so it is not reliably announced | `app.js:1950, 1938-1949, 1929-1930, 1903` | Measured: the result element is a new node inserted with its text; a live region that arrives with words is usually not read. PR #652's toast would be announced. | Keep the live lines outside the redrawn part; re-check after #652. | #659 |
| G-037 | LOW | U-U5 | On a touch screen the editor's small buttons stay 32 px against the design module's 44 px rule | `app.css:190`, `components.css:8-10`, `app.css:719` | Specificity 0,2,1 over 0,2,0; at 390 px with `pointer:coarse`, Discard queue, Theme, Auto-refresh and every house-name button are 32 px high; the module sizes by pointer and the tab by width. | Scope `app.css:190` to `pointer:fine`; add `.lsb .hn`; one rule. | #659 |
| G-038 | LOW | U-U6 | The glance tiles' accessible name does not contain their visible words | `app.js:1807` | "showing 2 Top tier reached" against "Show the 2 top tier houses in the list" (WCAG 2.5.3). | Start the name with the visible words. | #659 |
| G-039 | LOW | U-U7 | The provenance marks are words inside headings and column headers, and their meaning is only a title | `app.js:1626` (`lbMark`), `1792-1816, 1871-1881, 1910` | Names read "CURRENT TERM 6 AT A GLANCE character in Harkonnen SAVE"; keyboard and touch users cannot reach the title. | `aria-hidden` on the badge and a reachable tooltip or a legend. | #659 |
| G-040 | LOW | U-U8 | One tab, two write models: the term panel writes on one click with no question while the board stages; "Set flag" changes a row whose effect is unknown | `app.js:2058-2066, 1866-1869, 1877` | `landsraad_decrees.disabled` has an open meaning (#634) and is undone only by pressing again. #652 makes the board write at once and asks only before a bulk change. | After #652: one result line and one writing state for every Landsraad write; a question before "Set flag", or keep it in About. | #659 |
| G-041 | LOW | CR-U4 | The notes of a console export reach the page with "+" in place of every space | `internal/web/basebackups.go:96`; `app.js:1348` | Read. `QueryEscape` writes a space as "+", `decodeURIComponent` does not undo it; the tests miss it (empty list; a page header built the other way). | `url.PathEscape`, a route test with a space and a page test with the server's own encoding. | #662 |
| G-042 | LOW | CR-U11 | README and the route inventory still describe the old Landsraad board | `README.md:34`, `:179-182`; `docs/design/feature-inventory.md:615-619`; the rule at `docs/continuation-prompt.md:11` | No README row for the board's Write, the tier removal or the reward and event rows; `/apply` and `/faction-goal` are missing from the inventory. | One README row per new write with its real status; bring the route list up to date. | #662 |
| G-043 | LOW | Q-U2 | The QA line on #465 describes the old one-press Complete | comment on #465 (2026-11-13); `app.js:1951-1957` | On the new board Complete only stages until Write; the line, followed literally, writes nothing. | Add "then press Write" (and revise again if #652 merges). | #662 |
| G-044 | LOW | Q-U4 | Four closed issues still carry `awaiting-check` | #469, #482, #483, #516 | The check is done and unrecorded, or will be forgotten. | Remove the label or reopen. | #662 |

### Info

| ID | Sev | Hats | Title | Location | Evidence | Recommendation | Issue |
|---|---|---|---|---|---|---|---|
| G-045 | INFO | A-U5 | Repair policy lives in the file-format package, and the import's stack check rests on a table that may be stale | `internal/ops/world.go:37-42`; `internal/basefile/values.go:21-35, 72` | Two couplings after #629: repair imports the importer's ceiling; an export from a newer build with larger stacks is refused against the old catalog (D13-1). | A small shared policy package; refresh the stack table. | #613 (comment) |
| G-046 | INFO | A-U6, Q-U5 | The editor builds against unreleased module code, and the four module jobs are not required for merge | `go.mod` replace lines; `kit/CHANGELOG.md`; branch protection | A module change can be relied on for days before it is tagged; a PR that breaks the module tests can merge (release workflows re-run them). Four `main` pushes failed since 2026-10-27, three on the PowerShell analyser and one on G-014; `main` is green on every push since. Confirms D12-1. | Tag promptly; require `kit`, `kit-windows`, `design`, `design-windows`. | #622 (comment) |
| G-047 | INFO | A-U7 | The board's queue line does not say which houses wait | `landsraad_apply.go:296-298` | Better than the bare word of D1-6, but a held-conflict banner cannot name a house. | List shortened house names in `Desc`. | #657 |
| G-048 | INFO | S-U3 | Permission handler bug in the wrapper hides a clipboard-read grant | upstream `webview.go:105`; `chromium.go:258-278` | The kind is passed as 0 instead of its address, so every request gets Default. Inert today. | Deny all permissions when the wrapper is patched (G-024). | #660 |
| G-049 | INFO | S-U5 | The document guard fails open when the wrapper's internals change | `internal/gui/gui_windows.go:76-97` | `guardNavigation` finds the Chromium by reflection and does nothing if that fails; the CSP and the lack of any HTML-injection path make hostile navigation unreachable today. | A test that fails when `chromiumOf` returns nil for the pinned version. | #660 |
| G-050 | INFO | N-U2 | A file dropped on the window navigates to `file://`, which the document filter probably does not see | `gui_windows.go:96`; `kit/webguard/navigation.go:15-40` | From WebView2's documented behaviour; not run. A `file://` page has a null origin, no cookie and no reads. | `AllowExternalDrop` false when the wrapper is patched. | #660 |
| G-051 | INFO | N-U3 | WebView2 environment overrides are passed through unchanged | `gui_windows.go:30-47` | Variables such as the extra-arguments one can open a debugging endpoint; exploiting it needs control of the same user's environment. | Unset them before `NewWithOptions`. | #660 |
| G-052 | INFO | N-U4 | The session cookie is scoped to the host, not the port, so other local servers receive it in browser mode | `internal/web/server.go:600`, `:154-159` | HttpOnly and per-port name; Fetch Metadata and Origin still refuse a same-site page on another port; the token changes every run. The window mode has its own profile and is unaffected. | Document it; optionally serve browser mode on another loopback address. | register |
| G-053 | INFO | C-U3 | What the scanners do not see | semgrep run 43567847546; `ci.yml:261, 314-315`; `internal/testsave/layout.go:193` | 294 `*_test.go` files are skipped by semgrep's default ignore; trivy suppresses dev dependencies and lints no workflow; one `nosemgrep` has no reason. | Record the limits; a pinned workflow linter; give the suppression its reason. | #661 |
| G-054 | INFO | C-U4 | Scanner versions are frozen at the 2026-10-23 pin | `ci.yml:309-311` | Gitleaks 8.24.3, trivy 0.73.0, a rule snapshot of 2026-10-23; shared jobs on `ubuntu-latest` while this file pins `ubuntu-24.04`. | A monthly refresh. | #661 |
| G-055 | INFO | C-U5 | The repository's "Latest release" is a design-module release with no assets | `kit-release.yml:75`; `design-release.yml:180`; `README.md:99-101` | The release workflows pass `--latest=false`, but with no editor release GitHub falls back to the newest. The editor is built from source only, so no binary, checksum or signature ships. | Publish editor releases with checksums and provenance, or say on the README that there are none. | #661 |
| G-056 | INFO | C-U6 | Module tags are lightweight and unsigned | `git cat-file -t kit/v0.9.0` is `commit` (all 12 tags) | Commits on `main` are signed by GitHub; tags carry no signature. | Signed annotated tags, or rely on the ruleset of G-013. | #622 |
| G-057 | INFO | C-U7 | Indirect Go dependencies are not moved by Dependabot | `go.mod:23` (`golang.org/x/sys` v0.49.0, v0.50.0 is out) | The `gomod` entries have no `allow: dependency-type: all`; govulncheck still guards known vulnerabilities. | Allow all dependency types for the root module. | #661 |
| G-058 | INFO | C-U8 | The release jobs restore the Go cache from `main` into a `contents: write` job | `kit-release.yml:66-68`; `design-release.yml:171-173` | No `cache: false`; the modules use the standard library only, so the exposure is small. | `cache: false` in both release jobs. | #661 |
| G-059 | INFO | G-U11 | Required checks do not require an up-to-date branch | branch protection `strict=false` | Two PRs that pass alone can merge into a main that fails; the push run on main mitigates it. | Accept, or turn on strict for `go` only. | #661 |
| G-060 | INFO | U-U9 | The "Not started" part of the bar is the track colour, 1.07 to 1.41:1 in every dark look and palette | `app.css:366, 651` | Measured on 24 combinations; done and in progress are 5.3 to 13.4:1; the counts are in the tiles, so WCAG 1.4.11 is not strictly failed. | A hatched or outlined fill of at least 3:1. | #659 |
| G-061 | INFO | U-U10 | Disabled board controls are dimmed by opacity, and an aria-disabled Fill still answers a press | `app.css:432`; `app.js:2036` | Disabled labels measure 2.2 to 4.4:1; WCAG exempts inactive controls, but the Fill is pressable. | A token colour of at least 3:1, or the dashed border alone. | #659 |
| G-062 | INFO | Q-U6 | The real-file importer tests always skip in CI | `baseimport_test.go:857`; `baseimport_console_test.go:239`; `baseexport_test.go:495`; `vehicle_spawn_real_test.go:21` | They run only when `TAU_REAL_*` is set; synthetic fixtures cover the code. Two symbolic-link tests skip on Windows without the privilege. | Run the real-file tests before a release and say so in the notes. | register |
| G-063 | INFO | Q-U8 | Timing-based tests | `internal/web/launchpass_test.go:54, 62, 100, 121, 157, 193, 267` | A negative assertion after a fixed sleep cannot fail when slow, only when fast; the positive waits are bounded. No failure seen in CI history. | Prefer an injected clock where it is easy. | register |
| G-064 | INFO | CR-U13 | "Is this the game's own save" is worked out again on every poll | `internal/save/ownsave.go:39-83, 156-174`; callers `ops.go:152`, `spsession.go:196`, `server.go:1080`, `save.go:803` | For a copy each call resolves links and runs up to 400 stats; the answer cannot change on one path. | Cache per path; repeat the hard-link test when the file identity changes. | register |
| G-065 | INFO | CR-U14 | A base file is parsed several times before its size is checked | `internal/basefile/load.go:69-86`; `internal/web/basebackups.go:58-66` | Valid-UTF-8 test, a full decode to find the format, then the size test; the route and `Load` hash it twice; the body limit is 40 MiB. | Test the size first, read the format from the first token, hash once. | register |
| G-066 | INFO | CR-U15 | The apply reads the same rows several times per house | `landsraad_apply.go:232-241, 248, 272, 299`; `app.js:2041, 2066-2067` | Clock, term and faction progress read again in the loop and in `completeHouse` and `repairCompletionTimes`; the page maps all houses to find one. | Read the clock once per edit; look a house up by id. | register |
| G-067 | INFO | CR-U16 | The same small helpers are written twice | `landsraad_board.go:24` and `landsraad_rewards.go:159`; `landsraad_apply.go:308` and `landsraad_goal.go:84`, `landsraad.go:379`; `landsraad_goal.go:70` and `landsraad.go:431`; the faction test in four files | One of each belongs in `landsraad.go`. | One helper each. | #657 |
| G-068 | INFO | CR-U17 | Bring vehicle writes the map without the dimension while a move of the character writes dimension 0 | `world.go:180`; `teleport.go:281`; `vehicle_duplicate.go:123-126`; `vehicle_spawn.go` | Three rules for one column; every actor of the scratch copy is in dimension 0. | One rule, "the dimension of the character", in the helper of G-001. | #654 |

## Deduplication

Seven hat findings were merged into others; every id stays in the "Hats" column.

| Kept as | Absorbed | Why |
|---|---|---|
| G-004 | D-U5, Q-U7 | The same in-game check settles the event id rule and the "tests prove the rows" caveat. |
| G-005 | A-U3, CR-U12 | The architect's four disagreeing rules, the DBA hat's per-item key (which raised it to Medium) and the code review's swatch threshold are one `tierState` fix. |
| G-006 | Q-U3 | The same seven routes and the same deleted page tests. |
| G-015 | A-U4 | The removal that keeps delivered rewards is one of the three paths that break the event sum. |
| G-046 | Q-U5 | The module jobs not being required, seen from two angles. |

Findings that extend a 2026-11-13 item are cross-referenced and carried on that item's issue: G-007 (D2-1, D2-2), G-009 and G-021 (D3-1), G-020 (D5-1), G-022 (D7-1), G-026 (D9-1), G-032 (D11-3), G-013 and G-046 (D12-1, D12-2), G-023 (D13), G-047 (D1-6), G-002 (D11-1, D11-2).

## What is solid

Write paths and data:

- The board's apply is one `Mutate`: every house is looked at before anything is written, the counters reset at the top of the closure so a replay starts clean, a difference from what the page showed answers 409 and rolls back, and the reads go through `m.One` and `m.Query`, so they are in the replay read set. Optimistic concurrency between page and server is explicit (`was_mine`, `was_done`, `was_win`, the term id), not implied by timing. Running the same apply twice is refused the second time.
- On real data (a scratch copy): Fill wrote exactly the rows the design note names (one event above the cursor, four reward rows, the same item of two tiers merged into one row, a waiting amount from an earlier term added to, no swatch row where the swatch is held); Fill faction goal wrote the swatch row as the game's own rows do; a wrong `was_*` value, an earlier term, a non-threshold tier and a lowering without `lower` are refused with nothing written. No reward threshold is listed twice for a task, so the swatch cannot double an amount.
- The reward writer only adds, merges an uncollected earlier-term amount instead of overwriting it, takes ids above both the newest event and the cursor, refuses when the term's clock is unknown, and checks the two newer-layout tables before use. Every statement binds its values; the types fit the STRICT columns; the progress CHECK holds because an event is written only when `after > before`; the upserts target the real unique keys. Query plans are index lookups.
- The #611 fixes (#628) return read errors as named errors and keep the personal row to the character's own faction in both write models; the #613 fix (#629) puts the value checks in the file check and the same ceiling in both repair functions; a 1e300 health no longer spreads.
- One transaction per edit on a single-connection working copy, a foreign-key scan before commit, `MutateCascade` that blocks the session if it cannot turn enforcement off again. The write to the game's file keeps its full sequence: gate, hash against the loaded file, integrity check, encode with a byte-for-byte round trip, free space, a verified backup, temp file with fsync, gate again, hash again, rename with retries, read back, and an honest "written, not verified" state never written twice. Restore uses the same path.
- The save container codec checks the cap before reading, the declared size before inflating, an exact length and the SQLite magic, and verifies the checksum; a fuzz target runs in CI. The layout (older or newer) is decided from the columns at load, every markers query goes through one helper, and the schema note compares with both known layouts; the scratch copy matches the newer DDL statement by statement.
- `importBaseTx` renumbers every id and reads the ids for allocation through the edit's own reader; the stale-snapshot path of the page drops a pending house whose save state changed, with a line that says so; gear repair with the backpack included changed nothing on the scratch copy.
- Unattended writes: a copy of a save is never written by the automatic pass; `IsGamesOwnSave` answers "the game's own" when in doubt.

Network and the window:

- One listener, bound only after `CheckListenAddr` has checked a loopback IP, and bound to that IP; fallback to `127.0.0.1:0` only when `-addr` was not given. Server limits (header 10 s, read 30 s, write 2 min, idle 60 s, 1 MiB headers, a body cap on every POST). The front door sets headers first, including on the panic 500; Host must be the listener's own, the peer loopback; every `/api/` request needs the per-port HttpOnly SameSite=Strict cookie, same-origin and a parsed JSON type on non-GET; there is no CORS header, no WebSocket and no outbound HTTP client anywhere.
- The request gate lives in `kit/webguard` with its rules intact, pinned input by input; the CSP is unchanged and strict. Static files are embedded only; map icons and the 25 house symbols are registered per embedded file name; `/design/` lists no folders and is GET and HEAD only. Navigation is checked on every document request; `window.open` is stubbed; the profile is under `%LOCALAPPDATA%` and an empty value is refused; there is no `Bind`.
- No unsafe sink in the changed page code: house symbols come from a fixed list, measured values go through `dataset` and the style object, every string goes through the escaping template.
- Base files are downloads with a sanitised name; import rewrites every player id, collapses foreign rank rows, nulls building owners and is bound to the previewed file by hash.

Repository, CI and governance:

- Every action is pinned by full SHA and matches its tag; contents are read-only except in the two release jobs, which run after `verify` and take the tag through `env:` and a strict regex; `persist-credentials: false` everywhere; no `pull_request_target`, no artefacts and no repository secrets; first-time contributors need approval. Secret scanning and push protection are on; `enforce_admins`, linear history, conversation resolution, no force push or deletion. `go mod verify` runs in the root, kit and design jobs; govulncheck covers both builds and both modules; npm is locked with integrity, installed with `--ignore-scripts`.
- History: 82 commits, no AI trailer or attribution line, every owner commit on the noreply address and clean of private names. The tree is clean of the program name, private file and private paths (`git grep`), and the hygiene test runs in the required `go` job. All game art is marked not-MIT and not licensed by the project, every embedded game-data file added since 2026-10-27 has a notice pinned by a test, the font's licence is reproduced in full, and the module tags match their changelogs.
- Tests: no test reads the real user folders (the suite passes with them pointed at an empty folder); write paths and importers assert on resulting save rows, not mocks; the module boundary is tested; the Landsraad tab's kept decisions are pinned (order, symbols, About disclosures, staging, decrees writing at once).
- Accessibility that held (measured): no horizontal page scroll at six widths on six tabs; every enabled text on the Landsraad tab at least 4.5:1 in 3 looks, 2 themes and 18 palettes; focus colour at least 5.3:1 in all 24 combinations; Calm is the default under reduce motion; a board click keeps scroll offset and focus; states carry a glyph and a word; sort headers set `aria-sort`; tier boxes use `aria-pressed`; dialogs return the focus.

## Regression sweep (QA hat)

Every issue closed since 2026-10-15 with the label `bug`, `enhancement` or `feature` (54 editor issues) was checked: the fix commits were found from the closing PR and from every commit on `main` whose subject names the issue, and each fix's added code lines (14 characters or more, not comments or braces) were checked against the file at `c71a85d` and against the whole tree (to catch moved code). A fix with less than 80 percent of its lines on `main` was traced with `git log -S` to the commit that removed them, and that commit was read. Test lines were checked the same way.

Result: 54 checked, 0 reverted. 53 are still on `main`; 15 of them were rebuilt, moved or reworded by a later PR for the same behaviour (mostly the Landsraad rework #636 to #648 and the shared design module #580). One was closed without its fix: #594 (G-014). Two fixes added no test: #269 (CI) and #594.

The Landsraad rework (#637) deleted three page test files (488 lines); their behaviours are pinned again by `web-tests/landsraad-board-write.test.mjs`, `landsraad-board.test.mjs`, `landsraad-decrees.test.mjs` and the ops tests. The decisions of #450 (no question on top-ups) and #448 (the decree rule) are pinned. Rebuilt on purpose: #363, #371, #364, #453, #455, #601 (the old board and its dialogs, now the new board); moved: #472, #482 (tables moved into the kit module); refined: #245, #246, #248, #419, #426, #429, #449.

Strings spot-checked against the build with no mismatch: #508 (copy banner), #506 (Admin tab, Movement card), #501 ("Position saved"), #498 ("NOT LIVE"). Not checked string by string: #499, #480.

CI history since 2026-10-27: four failed `main` pushes (three PowerShell analyser, one G-014); `main` is green on every push since. Required checks are `go`, `govulncheck`, `powershell`, `python-tools`, `commit-messages`, `shellcheck`, the three security-scan jobs, `windows` and `ui`; the four module jobs are not required (G-046).

## Reported in play, already filed

These were reported by the operator and are filed; the audit adds to them as follows, and does not repeat them.

| Item | State | Relation to the findings |
|---|---|---|
| #650 page jumps to the top after a part swap | open, p2-next | Extended by G-010: six more redraws with the same cause. |
| #644 Landsraad house location table | open, p2-next | A data task, no finding. The board reads an empty table and says so; no position is invented. |
| #645 house symbols | closed by #648 | The fix is on `main` (13 of 13 lines); the art's notice has no provenance (G-032). |
| #646 house specialization | open, p3-later | A feature, no finding. |
| #647 vehicle limit research | open, p4-parked | Research, no finding. |
| #651 swap parts to a higher tier | open, p4-parked | A feature next to the part swap area of #650; it would inherit G-010 unless the redraw rule is fixed first. |
| #641 stage changes or write at once | open, answered by PR #652 | Findings that #652 changes or needs re-checking after it merges: G-004, G-005, G-016, G-017, G-019, G-034 to G-036, G-040, G-043. |
| PR #652 immediate Landsraad writes | open, head branch has the review fixes; not reviewed again here | Read for G-016 only. The operator decisions on G-004 and G-005 decide whether it is safe to merge as it is. |

## Open questions

For the operator (each is also in the remediation plan, decisions first):

1. G-004: may #652 merge before the in-game check of the progress events above the game's cursor, or must it wait?
2. G-005: should "already delivered" be decided per tier, as recommended, rather than per item?
3. G-001: when the body is parked on the travel map, should "next to the character" refuse, or ask for an explicit place?
4. G-007: should the unattended pass keep the stricter process-presence rule?
5. G-002, G-011: may the public issues and PRs be edited to remove the cross-references and the personal path? (It changes public content.) G-028: accept the history residue, or rewrite?
6. G-012: is the console export meant only for the person's own self-hosted server? The scope wording depends on it.
7. G-031: how was commit 38e0806 put on `main` without a PR, and is there a bypass?
8. G-024: vendor or fork the WebView2 wrapper, or offer the fixes upstream first?
9. G-055: will the editor ever ship binaries?

For a session where opening a window is fine: which settings a default popup inherits (G-025) and whether a dropped file raises a filter event (G-050). For the game: what processing an event above the cursor does (G-004, G-015), whether the game treats a collected row as "delivered this term" for a second tier of the same item (G-005), and how it writes `game.db` (G-023). For a real save: whether any other term has a house with the same item at two tiers (G-005 says yes for term 6). Whether jsdom's page tests enforce the CSP is a standing question; the server-side header test is the only check of the policy.

# Command Center M7: final verification and acceptance

This note is the acceptance document for the Landsraad and Specialization Command Center (issue #768, milestones M1 to M7). It says what was verified and how, what was not, which issues are open, and what the operator still has to check by hand. Nothing in it was checked in the game, on a real save or on a real phone.

**Recommendation: CONDITIONAL. NOT YET SHIPPED as the default.** The Command Center stays behind its switches (the classic views stay the default), which is the rollback. No automated gate fails and no P0 or P1 defect was found in this pass. What blocks calling it done is the list in section 7, which only the operator can do, and the open issues in section 6 (one is medium and new: #906).

Method of this pass: no product code, route, request body or save-editing function was changed. Two defects were found and filed (#906, #907); nothing was fixed here.

## 1. What was verified, and the state it was verified on

* Base: `main` at the merge of M6 (#903), commit 7004733. Branch `cc-m7-final-verification` adds only this note, one test file, one script and the requirements matrix statuses.
* Environment: Windows 11, Go 1.26.11, Node 24.16.0 with jsdom 31.0.0, headless Edge 157 (no window was shown). Everything ran at the lowest process priority.
* Suites on the final tree: `go vet ./...` clean; `go test ./internal/web ./cmd/publictextcheck` green; `node --test web-tests/*.test.mjs` 1081 tests pass, 0 fail (1070 before this pass; this pass adds 11 in `lcc-m7.test.mjs`).
* Nothing slipped in after the milestones. `git diff` from the M1 baseline (c369dd9) to this base, outside docs, tests and the Command Center's own static files, touches: `internal/web/server.go` (the static file list, the art folder list, the About panel's notes folder), `internal/web/about.go`, `internal/buildinfo` and `cmd/tabr-tau` (the version stamp, issue #806), and `app.js` and `index.html` (the page's wiring). **`internal/ops`, `internal/save` and the schema have no non-test change**: the only edits there are a test file (the watch-keeping test, #898) and the new M1 fixture test. No SQL endpoint, no new route, no new request field.
* The write routes are fixed by a test (`web-tests/lcc-m7.test.mjs`): the server registers exactly these eleven POST routes for the two pages, the page sends only to them, and each is played by an equivalence test. They are `/api/landsraad/apply`, `/decree`, `/term`, and `/api/player/specs` with `/xp`, `/max`, `/reset`, `/reset-all`, `/keystones/grant`, `/keystones/reset`, `/assign`. A twelfth route, or a missing one, fails the test.
* The CSV `requirements-matrix.csv` was updated row by row (70 rows). Statuses used: `VERIFIED_M7` (checked by automated tests or the headless sweep on synthetic data), `PARTIAL_M7`, `OPERATOR_PENDING` (needs eyes, the game or a phone), `DEVIATION_NEEDS_OWNER`. No row says verified for something that was only read.

## 2. Workstream A: functional regression, page by page

Real page code (`app.js` and the five Command Center scripts) in jsdom with the synthetic answers of M1. No game save.

| Area | How it was checked | Where |
|---|---|---|
| 25 Houses, fixed 5x5, `board_index`, no duplicate, a bad board shown read-only with reasons | the model and board tests; 25 Houses and 25 seats in every one of 72 board captures | `lcc-model`, `lcc-board`, sweep |
| Show / House / Specialization filters, Jump, clear, list sort, region group, Saved map link | board tests | `lcc-board`, `lcc-board-layout` |
| Two separate progress scales, five tiers with the four states, honest "delivery unknown" | model and Inspector tests | `lcc-model`, `lcc-board` |
| Tier 1 and 5 raise, removal of the highest only, no refund, Fill, Fill faction goal (own and rival), Complete, Reopen, faction selector, bulk atomic Fill/Complete/Reopen, select visible and clear, grant preview, mission limit, stale term/House re-read, queued versus written | the board sends the same request, question and result line as the classic list (parity tests), plus 21 recorded scenarios (with the phone bar variants) | `lcc-board-write`, `lcc-board-requests` + golden |
| Term: Apply (all combinations), Make active, pool flag 0 and 1, hidden SpecialVendorActive row, refusals | 14 scenarios against the classic panel and the new section, compared with a recording of the classic panel | `lcc-term-requests` + golden |
| Specializations: add and remove XP, Grant max, reset one, grant and reset all keystones, reset all known tracks with the typed sentence, assign by number and by hand, raw rows with the question | 16 scenarios in both views, compared with a recording of the classic list | `lcc-spec-requests` + golden |
| Cross navigation House to path and back, selection kept | page tests | `lcc-spec-page`, `lcc-m6` |
| Looking sends nothing on any of the four pages (every House, layout, sort, line, link, lane, switch) | new: one test walks all of them and asserts no request | `lcc-m7` |
| Classic views stay the default and stay reachable | the three switches, remembered in the browser only | `lcc-board`, `lcc-spec-page`, `lcc-m6` |

Cross-view equivalence across all four pages is therefore: Board against classic (live comparison and golden), Term against classic (golden), Specializations against classic (golden), Strategy (writes nothing: asserted). The new ledger test ties these together by route.

## 3. Workstream B: independent Sysselraad audit

* The 12 line cases, line conflicts, 13 at the cutoff, 13 to 12 pending, no cutoff source, equal counts and unreadable Houses are in `lcc-lines.test.mjs` and `lcc-lines-board.test.mjs` (26 tests, all 40 M1 board cases).
* New in this pass: a second reading of the rules was written inside `lcc-m7.test.mjs` without looking at the evaluator, and compared with it on **3000 seeded boards** (the same boards on every run): counts, the status of every line, the number of potential lines, the verdict, and that no cutoff is ever claimed. The seeded boards reach 200 or more held lines, 20 or more conflicts and 100 or more counts of 13.
* New: the same independent reading was compared with the **cards on the real page** for 48 seeded boards: which Houses carry the circuit outline (the Houses of a held line, and only those), which carry the majority outline (the 13 count, outside a line), the state of the Victory panel (line, conflict, count, contested) and the number in POTENTIAL LINES (or "Count incomplete" with an unreadable House). They agree on every board.
* The centre seat is a real gold seat of the 25 (one centre seat, 25 in the track, in all 72 board captures). Both faction ends fill symmetrically from the tests of `lcc-plaque`.
* Since 2026-11-18 (DECISIONS section G) a state says who won the Sysselraad, in the one wording table; the strategy words for a line not held are ONE AWAY, POTENTIAL, BLOCKED, UNREADABLE. A confirmed Dominion is never shown, because the save holds no start of the Voting Session (DECISIONS D03). A synthetic cutoff in the evaluator tests does not prove the local save has real evidence, and the page always passes none.
* Known gap, already filed: #893 (a held line is still shown when an unreadable House could hide a rival line), #894, #905.

## 4. Workstream C: visual regression (headless only)

`scripts/lcc-m7-sweep.mjs` (new) draws the editor's own page in headless Edge with the synthetic answers and measures what a still cannot. 108 cases: six views (board grid, board list, board with the Inspector open, Strategy on, the Term section, the Specializations view) in dark and Dayside at 320, 390, 768, 1024, 1280 and 1600 px, plus 200 percent and 400 percent zoom, forced colours and reduced motion at 390 and 1280.

Result: **0 hard failures**. No text under 11 px anywhere in the computed styles; no page wider than its window; no clipped text in the three surfaces other than the two decorative clips in the table below; 25 Houses, 25 seats, one centre seat and all 25 seats inside the track in every board capture (72); 5 lanes in every Specializations capture; no picture that failed to load; 0 page errors; 0 requests to another address; 0 missing files; 0 requests written; the hostile text of the fixtures stays text. The measurement also reports controls under the touch floor: see #906.

| Observed | Reading |
|---|---|
| `.lcc-track` scrolls wider than its box (all 25 seats are inside it) | the pointed ends are drawn by clipping; not a defect |
| `.lcc-plaque` 6 px wider inside at 390 and 768 | decorative edge; not a defect |
| Controls of the board under 44 px at phone widths | **#906** (new) |
| `.lcc-flag` 10 px in lcc.css, never drawn by the editor | **#907** (new, dead rule) |

Side-by-side with the approved V20 picture (crops of the left and right pointed end, the centre seat, the footer and the HELD BY divider, from `scripts/lcc-gallery-shots.mjs` in crop mode): the pointed ends have no rectangle or seam, the HELD BY divider is one vertical line to the left of the faction name, the footer has the line, the diamond and the counts in the right order, the 25th and 1st tokens sit at equal insets. The wording differs from the picture by rule (the wording table of DECISIONS section G, not "13 HOUSES WIN"; the 2026-11-18 decision replaced "MAJORITY AT 13 OF 25 HOUSES", which the #912 fix had put in place of "SEAT 13 OF 25"). This is a review by the author, not the design owner's approval; the task asks that a human design owner approves annotated exceptions, so **the visual pass is not signed**. Pictures were written outside the repository and none is committed.

## 5. Workstreams D to F: safety, privacy, performance, documentation

Verified by tests (all deterministic):

* Loopback and same-origin writes, escaping of save-backed text, queue lock and backup, the stale guard, queued never called saved: the existing Go suites for `internal/web`, `internal/ops` and `internal/save` are unchanged and green; the page tests for these are in `lcc-board-write` and the three request files.
* No network or storage code and no address in the five scripts and four style files (`TestLccFilesReachNothing...`), the page links only its own files (content security policy test), the art is served from a fixed list only.
* New: the Command Center files, notes, tests and scripts are stored with LF; they name no local path, no user folder and no game-internal asset file name; the art folder holds only the expected folders and no picture or save-like file; no picture is among the notes. The public-text check over the tree passes.
* New: 50 tab switches and 50 House jumps leave the same number of timers, page and window listeners and elements, and send nothing.
* New: the text floor over all four style files, cascade-aware: nothing under 11 px; the one exception is #907.
* Asset notices: `docs/art-notice.md` already names `internal/web/static/lcc/art/`. Open policy question: #884 (five game-art images for the Specializations badges).

## 6. Open issues by priority

No P0 and no P1 is open. Medium-severity items are the ones to see first.

| Priority | Issue | Area |
|---|---|---|
| p2-next, medium | #906 board controls are 36 px (tier boxes 24 px) at phone widths, below 44 px | board, touch |
| p2-next, medium | #880 tier boxes write on one tap, no preview on touch | board, write |
| p2-next, medium | #893 a held line beside an unreadable House could hide a rival line | Sysselraad |
| p2-next, medium | #905 chosen-line outline clipped; Term jump hidden under the header; small targets | Strategy |
| p2-next, medium | #886 phone victory panel hides the diamond emblem | board, phone |
| p2-next, low | #901 Specializations: dark gold looks beige, level-100 ornament line missing, hero shield | Specializations |
| p3-later, low | #881 a dimmed card can still be chosen and written in bulk | board, write |
| p3-later, low | #882 selects stay enabled while the board redraws during a write | board, write |
| p3-later, low | #883, #888, #889, #890, #894, #895, #896 board leftovers (focus, order, palette, contrast, wording, glow, legend size) | board |
| p3-later, low | #899, #902 Specializations leftovers | Specializations |
| p3-later, low | #904 Term and decrees leftovers | Term |
| p3-later | #887 (needs the operator) victory panel stacks full width at 1100 px | board |
| p3-later | #884 (needs the operator) five game-art images in the public repo | notices |
| p4-parked, low | #907 a 10 px rule in lcc.css that only the gallery reaches | style |

Decisions that need the owner: the tab strip across the three pages is not built (undecided; not built here); a connected path instead of a border round each House of a line (`WIN-005`, 07-m4-sysselraad.md section 5); the board as the default view; whether 11 px key labels may stay (#889).

## 7. What is NOT verified, and what the operator checks by hand

Not verified in this pass: anything in the game; any real save; the game's own reading of values the editor writes (whether it keeps a raised contribution, whether specialization XP is paid for a mission without a Recollection, what `landsraad_tasks.sysselraad` means, DECISIONS section D); a real phone; Windows high contrast; the Motion setting set to Off by hand; the animated border in motion (a still cannot show it); a screen reader; the pixel look at 200 to 400 percent by the design owner. A synthetic answer proves the page, not the save.

By hand, in this order, on a copy of a save with the game closed:

1. Start the editor on the copy. Open the Landsraad tab, choose the Command Center board. Check the plaque against the game's Landsraad screen: counts, active decree, the holding faction, 25 seats with the gold middle one.
2. Pick one House you do not mind changing. Raise tier 2, then remove it (only the highest can be removed). Check the House in the game afterwards (game closed while editing; the editor says Written or Queued: if Queued, close the game first and look again).
3. Fill, Complete and Reopen one House; then a bulk Fill of three Houses with the question. Check that a rival House asks first.
4. Term and Decrees: change the active decree and Apply; Make active from the pool; set one pool flag; reload the game and see whether it keeps them.
5. Specializations: add 1000 XP, remove it, grant max, reset one track; grant and reset all keystones; look at the editor's wording against the game's specialization screen; check the level.
6. Restore the backup the editor made (Admin) and check the game loads the older file.
7. Open the editor on a phone (or a narrow window) in both themes: tap the tier boxes and the filter chips and see whether the targets are comfortable (#906); open the Victory panel (#886).
8. With Windows high contrast on, and with Motion set to Off and to Calm: look at the board and the line outline.
9. Design owner: look at the plaque, the pointed ends, the HELD BY line, the nameplates and the Specializations page beside the approved pictures at 200 to 400 percent, in dark and Dayside.
10. Say which of the known differences in section 6 are accepted, and which view should become the default.

## 8. Rollout and rollback

* Rollout: the Command Center is already merged and off by default. The Landsraad tab has a View switch (Classic list, Command Center board) and the Specialization tab has a View switch; each choice is remembered in the browser only. Strategy is a switch on the board, off by default.
* Rollback: choose Classic list (and Classic on the Specialization tab). If a release needs the old views gone from the page, the change is in `index.html` (the four links and five scripts) and the view switches; no data, queue, backup or schema is touched by either view. Nothing needs migrating.
* Writes go through the same queue, backup, stale guard and durable write as before; nothing was added to them.

## 9. Known limitations carried to the release notes

No live contributor ranking and no live mission rotation (the save holds neither); no verified Voting Session start, so a House-count lead is never a result; a held line is a fact about the Houses as they stand, not proof of a vote; the decree picture for the special vendor decrees is a neutral one; the House-to-line table is a reference table from a public tracker, not the save (D09); no tab strip across the three pages; live updates of the game while the editor is open are only seen after the editor reads the file again.

## 10. How to repeat this pass

```
UI_FIXTURES_DIR=web-tests/fixtures go test ./internal/web -run TestDumpUIFixtures
node --test web-tests/*.test.mjs
node scripts/lcc-m7-sweep.mjs <output folder outside the repository>
LCC_REF_IMAGE=<the approved picture> LCC_SHOTS_ONLY=crop node scripts/lcc-gallery-shots.mjs <output folder>
```

The sweep needs Node 22 or later and a Chromium-family browser; it starts the browser itself in headless mode, serves the editor's own files on a loopback address, answers every POST with a bare ok and counts it, and exits with status 1 on a hard failure.

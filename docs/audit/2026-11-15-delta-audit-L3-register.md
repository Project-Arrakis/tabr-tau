# Findings register, 2026-11-15: delta audit L3 of the editor (tabr-tau)

Tracking issue: [#677](https://github.com/Project-Arrakis/tabr-tau/issues/677). Layer: **L3** (a separate high-effort code review of the combined delta), under `docs/decisions/2026-11-14-audit-cadence-and-no-peer-review.md`.

Base: `b4cede8`. Range: `b4cede8..d1f0bf8`, the same seven merged pull requests as the L2 register of the same day (`docs/audit/2026-11-15-delta-audit-L2-register.md`): #674, #675, #676, #678, #685, #687, #688.

Method: the whole diff was read line by line with the functions around each hunk, then by angle: behaviour that was removed, the callers of every changed function, error paths, interactions between the merged pull requests, and the logic of the public-text check (#676) and of the character's place (#675). The L2 findings (#689 to #694) were read first and are not repeated; where a finding here is near one of them, it says how it differs. Three claims about the public-text engine were confirmed with a throwaway test that was not kept (the path forms of L3-05, the line break of L3-07, the two encodings of L3-08). L3-06 was found by reading the workflow and was not run. No game check. Single-player only. **Nothing was fixed by this audit.** The texts of this register and of the issues were run through `cmd/publictextcheck -items` before filing.

## Result

| Severity | Count |
|---|---|
| Critical | 0 |
| High | 0 |
| Medium | 3 |
| Low | 6 |
| Total | 9 |

## Findings

| Id | Severity | Issue | PR | Where | What |
|---|---|---|---|---|---|
| L3-01 | Medium | #696 | #687 | `internal/web/static/app.js:2246-2261`, `:532-544` | The refocus after a redraw now scrolls the control into view, also for the automatic refresh: with the game running the page jumps back to the last pressed button at every refresh. Regression. |
| L3-02 | Low | #697 | #687 | `app.js:2523-2524`, `:2750-2759` | `render('scroll')` keeps a table box's scroll offset when the box shows other rows (inventory group or filter, base group, another base). The table browser resets its box; these do not. Regression. |
| L3-03 | Low | #698 | #688 | `app.js:1123`, `:1129`; `web-tests/polish.test.mjs:12-18` | The Building sets line says "N of M learned" but counts sets that are only in an inventory; the test pins that sentence. |
| L3-04 | Low | #699 | #675 | `internal/ops/ops.go:195`, `summary.go:28`, `livemap.go:70`; `app.js:996-1001` | The player answer, the summary and the saved map still take the character's place from the body alone, so the Movement card's "This map" and coordinates disagree with what Teleport and the place list use. |
| L3-05 | Medium | #700 | #676 | `internal/hygiene/hygiene.go:95-96`, `:100-105` | The profile-path rule misses the POSIX-shell and mount forms of a Windows path, file URLs, a path after a colon or a bracket, a network path; a few real first names are exempt as placeholders. |
| L3-06 | Medium | #701 | #676 | `.github/workflows/public-text.yml:9-18`, `:30`, `:47` | Review events run a job with the same check name on the same head commit and skip the commit-message step, so a later review run can replace a failed pull request run. To confirm on a test pull request before the check is required. |
| L3-07 | Low | #702 | #676 | `internal/hygiene/hygiene.go:112-123` | A reserved phrase of two to four words is not found when a line break falls inside it. |
| L3-08 | Low | #703 | #676 | `internal/hygiene/hygiene_test.go:118-124`, `:159-161` | The file scan silently skips a text file in a two-byte encoding, and any file with one byte that is not valid UTF-8. |
| L3-09 | Low | #704 | #676 | `.github/workflows/public-text.yml:9-18`; `cmd/publictextcheck/main.go` | Commit comments, release text, pushed ref names and the commit messages of a push without a pull request are not read; the `-items` mode skips the attribution rule. |

## Checked and found fine

- **#687, the other `render('scroll')` call sites.** Each one already lost typed text before (a plain `render()` kept nothing), so dropping the control values is not a new loss. The Details click now reads the create form into its draft first. Handlers are delegated on the document, so none is left bound to a removed node. A redraw while a question is open does not take the focus (the focus is outside the page then). The refused-edit redraws (`conflict`) keep the place and show the save's values, as intended.
- **#687, the table browser.** Search, Previous and Next await the redraw before the box is set to its top; Enter in the search box goes the same way and puts the cursor back.
- **#685.** A changed class, tier or part reads the boxes into the draft before the redraw; the preview is dropped on every change.
- **#688.** The save name is hidden while the path is empty; the pager shows on a later page even when the total shrank; the Cosmetics count words match what is counted.
- **#678.** The waiting rewards are used in one place only (the house card), as escaped fragments; "nothing waiting" is unchanged.
- **#675, Teleport.** One edit; the reads inside it leave the position out, so a queued move is held only for a changed map, dimension or state. A named map equal to the player's own map with the body elsewhere writes all three actors. No body, or no controller, is refused before anything is written.
- **#675, Bring, Create, Duplicate.** All three refuse a parked body inside the edit, before the next id is taken. Create with a place the person gave still uses that place's map. The dimension column is `NOT NULL` with default 0, so the "0 or more" rule changes nothing for a single-player save (the disagreement case is L2-05, #693).
- **#675, Landsraad.** The distance to a house is left out when the body is parked; no error path changed.
- **#676, the design folder test.** The seven phrases that were removed from the pattern are all still caught: each of the new digests was matched to one removed phrase.
- **#676, the event tool.** A null body, an empty review, a comment on a pull request (treated as a pull request) and the commit log format are handled; the log names the place and the rule only; `-show` is refused in CI.
- **#674.** Docs only.

## Not covered

- No run of the page tests or of the full Go test suite; no check in a browser. L3-01 and L3-02 are shown by the code and need a page test when fixed.
- No check in the game (the open game checks of #675 are on L2-04, #692).
- L3-06 depends on how the host chooses among check runs with one name and was not run.

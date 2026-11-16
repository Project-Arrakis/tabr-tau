# Findings register, 2026-11-16: delta audit L2 of the editor (tabr-tau)

Tracking issue: [#809](https://github.com/Project-Arrakis/tabr-tau/issues/809). Layer: **L2** (implementation audit through the eight hats), under `docs/decisions/2026-11-14-audit-cadence-and-no-peer-review.md`.

Base: `002ba13`. Range: `002ba13..981af62` (main on 2026-11-16), 39 first-parent commits: 38 merged pull requests and one commit without a pull request (`5aaacd8`, R2-05). 141 files, about 14,200 lines added and 1,000 removed.

## Pull requests and risk

Risk classes: **High** writes to a save or changes the live-save gate or a CI control; **Medium** changes what a write targets or a guard on the page; **Low** page text, layout, reads; **docs** documents and mock-ups only.

| PR | What | Risk |
|---|---|---|
| #706 | Automatic refresh refocuses without scrolling (#696) | Low |
| #707 | Table boxes start at their top on a redraw (#697) | Low |
| #709 | Building sets: count only learned sets as learned (#698) | Low |
| #710 | Four gaps in the public-text check closed (#689, #700, #702, #703) | High (CI control) |
| #711 | Pull request public-text check in its own workflow (#701) | High (CI control) |
| #721 | Teleport refuses a non-standing body on every map (#692) | High (save write) |
| #722 | Public-text check reads commit comments, releases, created refs and pushes (#704) | High (CI control) |
| #723 | Character's place from one reader in player page, summary and saved map (#699) | Medium |
| #737 | Create a vehicle: replace a part in the preview table (#672) | Medium |
| #745 | Landsraad specialization research and mock-up (#646) | docs |
| #746 | Saved map filters: inventory and static mock (#679) | docs |
| #747 | Filter box in the Colours list of the Theme menu | Low |
| #744 | Landsraad house place from the save's map markers (#639) | Low |
| #759 | Create a vehicle: a double click no longer writes two vehicles (#740) | High (save write) |
| #652 | Landsraad board: every click writes at once and grants what its missions grant (#641, #609) | High (save write) |
| #761 | Landsraad: a fixed map place for every house without a marker (#639) | Low |
| #763 | Landsraad decrees show the game's names (#608) | Low |
| #764 | Go to a place: count of places on the map shown (#607) | Low |
| #765 | Landsraad tab: tier boxes off under another faction, glance cards | Medium |
| #766 | CI: 20 minutes per package for the race run (#756) | Low (CI) |
| #769 | Repair base and Repair buildings: target from standing pieces of own bases, cap per type (#613) | High (save write) |
| #771 | Landsraad tier reads as delivered only when events cover it (#656) | Medium |
| #773 | Landsraad writes: names, what they take away, single Complete checks term and faction (#611) | High (save write) |
| #774 | Live-save gate: the file's time as a second signal, unattended pass waits for the game (#612) | High (gate) |
| #776 | Landsraad raise writes no progress event, cursor left alone (#655) | High (save write) |
| #780 | Base export described as for the player's own self-hosted server only (#658) | docs |
| #781 | Public-text page, test that keeps it current, changelog (#653) | Low (CI docs) |
| #783 | Base storage: deleting an item keeps the container open (#731) | Low |
| #782 | Create a vehicle: a pick keeps text typed meanwhile (#738) | Low |
| #786 | Repair every base leaves placeable holograms alone (#770) | High (save write) |
| #784 | Create a vehicle repeat memory: forget only when edits were dropped (#760) | High (save write) |
| #792 | Live-save gate leftovers: gate before the backup, clock skew, docs | High (gate) |
| #796 | Create a vehicle: a repeat whose check cannot read the save is refused (#790) | High (save write) |
| #795 | Regions card: area id 0 left out of the unnamed count (#788) | Low |
| #798 | Landsraad board remembers the ground it paid (#777) | High (save write, new file) |
| #802 | Game log unreadable: folders cut out of the reason, test fix (#567) | Low |
| #803 | Fill faction goal counts a house once, never lowers the faction | High (save write) |
| #804 | Landsraad grant leftovers: amounts in the write log, last level said, stale house refuses a raise (#752) | High (save write) |

Totals: 17 High, 4 Medium, 14 Low, 3 docs (one row each; #802 also carries the extra commit `5aaacd8`).

## Method

The diff of each pull request was read at `981af62`, and the write paths and their guards were read in full with their callers: `internal/save` (notes, journal, durable install, gate, session log, file time), `internal/ops` (Landsraad grant, paid record, apply, board; vehicle create and its repeat ledger; repair caps; Teleport and the character's place), `internal/web/server.go` and the new page code in `internal/web/static/app.js` and `theme-menu.js`, and the three public-text workflows with `cmd/publictextcheck` and `internal/hygiene`. Closed issues fixed in the range were checked for regressions (#607, #608, #609, #639, #641, #655, #656, #672, #692, #696 to #699, #731, #738, #740, #752, #760, #777, #788, #790). Open issues were checked so nothing already filed is filed again. Two reviewers read the Landsraad and the vehicle, repair, Teleport and page slices in parallel; each candidate was checked against the code before it was filed. The texts of this register and of the issues were run through `cmd/publictextcheck -items` before filing. Single-player only. **Nothing was fixed by this audit.**

## Result

| Severity | Count |
|---|---|
| Critical | 0 |
| High | 0 |
| Medium | 1 |
| Low | 5 |
| Total | 6 |

No finding loses the game's data or opens a path from outside the machine. The Medium finding is a fail-open window in the new paid-ground record (#798): after the record could not be written, a raise in the same session pays the same ground again.

## Findings

| Id | Severity | Hat | File and line | What is wrong | Issue |
|---|---|---|---|---|---|
| R2-01 | MEDIUM | write path, correctness | `internal/save/notes.go:52-62`, `:93-111`; `internal/ops/landsraad_grant.go:176` | Notes of written edits that the record could not take (`undelivered`) are not counted by `pendingNotes`, so a raise before the next successful delivery pays that ground again; the bound of 1000 drops the oldest kept notes silently. | [#810](https://github.com/Project-Arrakis/tabr-tau/issues/810) |
| R2-02 | LOW | write path, docs | `internal/ops/landsraad_paid.go:124-130`, `:155-157`, `:305-319` | A record of another version, another save or not whole JSON reads as nothing and is overwritten by the next write, with no copy kept. | [#811](https://github.com/Project-Arrakis/tabr-tau/issues/811) |
| R2-03 | LOW | docs / UI text | `internal/web/static/app.js:1476`, `:1486` | Both repair tips say every placeable is raised; since #786 holograms are left alone. | [#812](https://github.com/Project-Arrakis/tabr-tau/issues/812) |
| R2-04 | LOW | error handling | `internal/web/static/app.js:3080`, `:1671-1673` | A failed redraw of Create a vehicle skips the restore of the typed text and leaves the promise rejection unhandled. | [#813](https://github.com/Project-Arrakis/tabr-tau/issues/813) |
| R2-05 | LOW | tests / CI | `main` at `5aaacd8`; `.github/workflows/public-text-push.yml` | A second commit on main with no pull request and no check run (as G-031 on #661); the push check's range starts after it, so its message was never read. Same tree and message as #802 this time. | [#814](https://github.com/Project-Arrakis/tabr-tau/issues/814) |
| R2-06 | LOW | write path (scope) | `internal/ops/world.go:69-78` | Repair every base raises backup pieces and unowned pieces to the target that leaves backups out; the scope is not decided, said or tested. | [#815](https://github.com/Project-Arrakis/tabr-tau/issues/815) |

## Hat coverage

| Hat | Covered | Findings |
|---|---|---|
| Security | SQL of the new code (parameters, constant fragments, allow-listed columns), HTML sinks in new page code, request ids of the vehicle ledger, workflow triggers, tokens and interpolation | none |
| Privacy and public text | new docs, mocks, provenance and notices, error texts (`shortErr`, `shortReason`), the record file's content and mode, the public-text workflows | none new (see below) |
| Write-path safety | Landsraad apply, grant, paid record and notes; journal version 2 and notes in it; durable install and the gate before backup and rename; vehicle create ledger; repair caps and scope; Teleport | R2-01, R2-02, R2-06 |
| Correctness | the same paths, the gate's state machine (anchored end lines, writes after end, unknown_quiet), regions, Go to a place counts, building sets | R2-01 |
| Concurrency and races | lock order of the vehicle ledger and the save, the paid record's mutex, the board read order of queue and file, the file-time watch maps, clock skew | none |
| Error handling | sink failures, unreadable record, unreadable settings file, page promise chains | R2-04 |
| Tests and CI | tests of each pull request against its claim; required checks; the push and discussion workflows; CI on `981af62` (all checks green) | R2-05 |
| Docs and CHANGELOG | CHANGELOG entries of the range, `docs/save-database.md`, `docs/design/2026-11-15-live-save-gate.md`, page tips | R2-03 (and the docs part of R2-01, R2-02) |

## Checked and found fine

**Security.** Every new Landsraad statement is parameterised; the only built SQL is a constant markers subquery and an allow-listed column; `bestHealthOf` and `capValues` build SQL from program constants only. The vehicle ledger's request id is bounded by a pattern and never reaches SQL. New page code renders through the escaping `html` template and `setHTML`; `theme-menu.js` uses `textContent` and `createElement`; no new `innerHTML`, `outerHTML`, `insertAdjacentHTML` or `raw()` on save text. The new workflows run on plain events with read-only tokens, pinned actions, no event text in a `run` line, and concurrency groups of numbers and ids only.

**Privacy and public text.** Error texts that reach the page now go through `shortErr` (drive, network and Unix folders cut; #800 tracks its limits) and `shortReason` cuts at a character boundary. The paid record lives in the editor's own folder with mode 0600. The mocks carry no asset file names, local paths or user names. The data-table names in provenance follow the existing convention. One research note names a public community site and says it refuses automated reads (`docs/design/mock/landsraad/specialization-research.md:116`); it names no private tool and is left as an observation for the operator, not a finding.

**Write-path safety and correctness.** Each board click is one transaction; checks against what the page showed run before any write; a house cannot appear twice in a request; the faction share of Fill faction goal is the higher of the raise and the goal (#803); no non-test code writes a progress event or moves the cursor (#655 holds). The journal version is raised to 2 so a version-1 queue cannot replay the old event writes. The automatic pass waits for every game process (`UnattendedStateFor`), also when its edit is written later from the queue (`AutoPassDesc`); the gate is asked again after the encode and before the backup (#779) and right before the rename. A sidecar newer than the save is never taken for the editor's own write. Teleport never sets the parked map for a body whose state is not 0 (#692 holds). The vehicle ledger is pruned only when edits were dropped, forgotten on a successful restore, and checks id, class and entity list before answering a repeat.

**Concurrency.** The vehicle ledger lock is always taken before the save's lock; the paid record has its own mutex and is read inside the edit under the save's lock; the board read takes the queued notes before the file; the file-time maps have their own locks and the clock-skew rule covers two near-simultaneous evaluations.

**Tests.** Each write-path pull request adds tests on temporary databases (paid record, notes through journal and put-back, ledger prune and forget, repair caps with backups and holograms, gate signals with fake clocks and file times). Gaps are the subjects of R2-01 (no raise after a sink failure) and R2-06 (no test of the write scope).

**Extra commit.** `5aaacd8` has the same tree as `bf14ba1` (the merge of #802), whose pull request checks passed; see R2-05.

## Not covered

- No game check: the cap of the paid record (a collected reward row kept for the term) and the gate's file-time signal against a real game session remain open items of #798 and #774.
- The Go and web test suites were not run again (the game was in use on this machine); CI ran them on each merge and on `981af62`. `cmd/publictextcheck -items` was run on the filed texts only.
- The kit and design modules were not changed in the range and were not read.
- L3 (the whole-diff review) is a separate layer and is not part of this register.

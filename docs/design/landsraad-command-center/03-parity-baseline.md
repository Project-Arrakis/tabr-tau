# Command Center M1: test baseline and feature-parity baseline

Part 1 records what was run, on what, with what result. Part 2 lists every current editing and navigation feature of the Landsraad tab and of Player > Specialization, whether the approved mock has it, and the gap that matters. A command that was not run says so.

## 1. Test baseline

### 1.1 Environment

| Item | Value |
|---|---|
| Commit | `c369dd926e5f5c40048eaeca33b665be07dd2b02` on `main`, branch `cc-m1-audit` (no local changes when the baseline ran) |
| OS | Windows 11, build 10.0.26300 |
| Go | go1.26.3 windows/amd64, `CGO_ENABLED=0`, no C compiler on the path |
| Run style | process priority Idle, `GOWORK=off`, `-p 1` (one package at a time), no browser, no game |
| Line endings | the checkout is CRLF (`core.autocrlf=true`), the index is LF; `gofmt -l .` therefore lists every file locally and cannot be used here; CI runs it on Linux |

### 1.2 Baseline before any change of this milestone

| Command (working directory) | Result | Time |
|---|---|---|
| `go vet ./...` (root) | exit 0, no output | 7.9 s |
| `go test -p 1 -count=1 -timeout 20m ./...` (root) | exit 0; 22 packages `ok`, 2 packages with no test files | 213.2 s |
| `go vet ./...` (`kit`) | exit 0, no output | 6.7 s |
| `go test -p 1 -race -count=1 ./...` (`kit`) | **not run**: `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` (exit 2). No C compiler is installed, so the race detector is unavailable on this machine. CI runs it on Linux | 0.1 s |
| `go test -p 1 -count=1 -timeout 10m ./...` (`kit`, without `-race`) | exit 0; 11 packages `ok` | not timed |
| `go vet ./...` (`design`) | exit 0, no output | 0.6 s |
| `go test -p 1 -count=1 ./...` (`design`) | exit 0; `ok  github.com/Project-Arrakis/tabr-tau/design 1.781s` | 5.5 s |

The root test run, as recorded:

```
ok  	github.com/Project-Arrakis/tabr-tau/cmd/publictextcheck	1.379s
ok  	github.com/Project-Arrakis/tabr-tau/cmd/tabr-tau	0.725s
?   	github.com/Project-Arrakis/tabr-tau/internal/apierr	[no test files]
ok  	github.com/Project-Arrakis/tabr-tau/internal/augments	0.398s
ok  	github.com/Project-Arrakis/tabr-tau/internal/basefile	0.454s
?   	github.com/Project-Arrakis/tabr-tau/internal/basefile/consolefixture	[no test files]
ok  	github.com/Project-Arrakis/tabr-tau/internal/catalog	0.452s
ok  	github.com/Project-Arrakis/tabr-tau/internal/config	2.358s
ok  	github.com/Project-Arrakis/tabr-tau/internal/diff	3.974s
ok  	github.com/Project-Arrakis/tabr-tau/internal/gui	2.273s
ok  	github.com/Project-Arrakis/tabr-tau/internal/hygiene	14.672s
ok  	github.com/Project-Arrakis/tabr-tau/internal/journey	0.389s
ok  	github.com/Project-Arrakis/tabr-tau/internal/kits	0.440s
ok  	github.com/Project-Arrakis/tabr-tau/internal/notices	0.391s
ok  	github.com/Project-Arrakis/tabr-tau/internal/ops	72.412s
ok  	github.com/Project-Arrakis/tabr-tau/internal/save	52.015s
ok  	github.com/Project-Arrakis/tabr-tau/internal/settings	1.497s
ok  	github.com/Project-Arrakis/tabr-tau/internal/skills	0.381s
ok  	github.com/Project-Arrakis/tabr-tau/internal/testsave	0.798s
ok  	github.com/Project-Arrakis/tabr-tau/internal/web	25.713s
ok  	github.com/Project-Arrakis/tabr-tau/internal/winpath	0.480s
ok  	github.com/Project-Arrakis/tabr-tau/tools/deepdeserttaxisgen	0.612s
ok  	github.com/Project-Arrakis/tabr-tau/tools/placenamesgen	0.704s
ok  	github.com/Project-Arrakis/tabr-tau/tools/vehiclegen	0.473s
```

Nothing failed before the change; there are no pre-existing failures to separate out.

### 1.3 After the change of this milestone

Run after the files of this milestone were added (the new test file, the fixtures and the docs, staged), same machine and settings.

| Command | Result |
|---|---|
| `go vet ./...` (root) | exit 0, no output (1 s: most packages were cached from the baseline run) |
| `go test -p 1 -count=1 -timeout 20m ./...` (root) | exit 0; 22 packages `ok`, 2 with no test files; `internal/ops` 78.7 s, `internal/save` 52.2 s, `internal/web` 25.3 s, `internal/hygiene` 14.1 s; 203 s in all |
| `go test -count=1 -run TestCommandCenter ./internal/ops` | `ok`, 1.2 s; all 11 new tests and the 12 sub-tests of the Specialization cases pass |
| the same with `TABR_TEST_LAYOUT=map_names` (CI's second run, the game's newer save layout) | `ok`, 1.2 s |
| `go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./internal/ops` (CI's pinned version) | exit 0, no output |
| `go test -count=1 ./internal/hygiene ./cmd/publictextcheck` with the new files staged | `ok` (14.3 s and 1.2 s) |
| `gofmt -l` on `internal/ops/command_center_fixtures_test.go` | no output (the file is LF; the other files of a CRLF checkout cannot be checked this way) |

A mutation check: changing one expected value of a fixture (`thresholdReached` of the empty board) makes `TestCommandCenterBoardCasesAgreeWithTheGeometry` fail with the case id; the file was restored.

Not run: `go test -race` anywhere (no cgo here), `gofmt -l .` over the repository, `go mod verify`, the Windows cross-compile, `govulncheck`, the `web-tests` suite, the PowerShell, Python and shell jobs, and anything in a browser or in the game.

### 1.4 CI checks that apply (read from `.github/workflows/`, not run locally except as stated)

| Workflow and job | What it runs | Run locally in M1? |
|---|---|---|
| `ci.yml` job `go` (Linux) | `gofmt -l .`, `go mod verify`, `go vet ./...`, `staticcheck@v0.8.1 ./...`, Windows cross-compile (vet, build, 386 vet of `internal/gui` and `cmd`), `go test -race -count=1 -timeout 20m -covermode=atomic ./...`, then `go test` of `internal/save`, `ops`, `diff`, `web`, `testsave` again with `TABR_TEST_LAYOUT=map_names` | `go vet`: yes. Staticcheck on `./internal/ops`: yes (see 1.3). The new tests with `TABR_TEST_LAYOUT=map_names`: yes (see 1.3). Race, cross-compile, `mod verify`, `gofmt -l .`: no |
| `ci.yml` job `windows` | `go vet ./...`, `go test -count=1 -timeout 12m -v ./...` on Windows | the root `go test` above is the same command family on this machine |
| `ci.yml` jobs `kit`, `kit-windows` | vet, staticcheck, race tests, a 20 s fuzz of `FuzzDecode`, govulncheck, `scripts/check-kit.sh` | vet and non-race test only |
| `ci.yml` jobs `design`, `design-windows` | the `design` module's checks | vet and test only |
| `ci.yml` job `ui` | `TestDumpUIFixtures`, then `node --test` in `web-tests/` (jsdom, pinned by lockfile) | **not run** (needs `npm ci`) |
| `ci.yml` jobs `govulncheck`, `powershell`, `python-tools`, `commit-messages`, `shellcheck`, `security-scan` | as named | not run |
| `public-text.yml`, `public-text-discussion.yml`, `public-text-push.yml` | the public-text rules on PR, issue and push text | the repository's own test `internal/hygiene` ran over the tracked files (see 1.3) |

The browser-engine suites of `web-tests/` that already cover the current Landsraad page: `landsraad-board.test.mjs`, `landsraad-board-write.test.mjs`, `landsraad-decrees.test.mjs`, `landsraad-errors.test.mjs`, `landsraad-kept-decisions.test.mjs`, `landsraad-queue-text.test.mjs`, `landsraad-symbols.test.mjs`. They are the regression net the board work (M3) has to keep green; they were not run in M1.

### 1.5 Windows manual test procedure

The repository's Windows procedure is `docs/live-test-protocol.md` (snapshot, one action per snapshot, game closed, results recorded in `docs/evidence/RESULTS.md`) and the Windows first-run record `docs/evidence/2026-10-07-windows-first-run.md`; the bug validation of 2026-11-07 is `docs/uat/2026-11-07-bug-validation.md`. M1 changes no behaviour, so no Windows run was needed; M7 needs one.

## 2. Feature-parity baseline

Method. "Current" is read from `internal/web/static/app.js` and the Go code at `c369dd9` (lines in `02-source-register.md`). "Mock" means the approved V20 Landsraad and V21 Specializations mock of the design package, judged **by reading its README files and searching its scripts and page for the control**, not by running it. A feature marked "not found" was not found by that search; it is not proof of absence in the approved art. Severity as in `DECISIONS.md`: P0 a missing or unsafe write, P1 a wrong or missing state or navigation, P2 polish.

### 2.1 Landsraad

| # | Current feature (source) | In the mock? | Gap if not carried | Sev | Milestone |
|---|---|---|---|---|---|
| L01 | Tier click: raise to a tier, cumulative (`lbItem`, `app.js` 1857) | Yes, with the same rule, demo data (`editor-v8.js` `demoChangeTier`) | the real write needs `mine_to`, `was_*`, `was_events` | P0 | M3 |
| L02 | Remove only the highest tier; a lower tier is refused with the reason (`app.js` 2476, `lower:true`) | Yes (message in `demoChangeTier`) | the removal must send `lower:true`; the note that nothing paid is taken back (`LB_REMOVE_NOTE`) must stay | P0 | M3 |
| L03 | Fill (top tier), off at the top, without a faction, or with another faction chosen (`lbFillOff`, 1851) | Yes (`demoFillHouse`) | the off states and their text | P0 | M3 |
| L04 | Fill faction goal (`mine_to` + `complete_for`) | Yes (`demoFillGoal`) | refused for a faction other than the character's own (`landsraad_apply.go` 220-224) | P0 | M3 |
| L05 | Complete for the chosen faction | Yes (`demoCompleteHouse`) | `complete_for`, progress raised to the goal, never lowered | P0 | M3 |
| L06 | Reopen (progress and contribution stay) | Yes (`demoReopenHouse`) | the wording that a house taken by another faction is not given back (`LB_REOPEN_TIP`) | P0 | M3 |
| L07 | Bulk: Choose boxes, Choose all, Clear, Fill / Complete / Reopen chosen, always asks, all or none (`app.js` 2189, 2348) | **not found** | no way to act on several houses; up to 100 per request | P0 | M3 |
| L08 | Stale guards: `term_id`, `was_mine`, `was_done`, `was_win`, `was_events`; a conflict re-reads the board (`lbRun`, 2232) | **not found** (demo only) | a view that omits them can overwrite a change made meanwhile | P0 | M3 |
| L09 | One write at a time (`LP.busy`), with a busy state and a message (`lbRunning`, 2224) | **not found** | double writes | P0 | M3 |
| L10 | Written versus queued wording (`NOT_SAVED`, `outcome`) | one mention of queued | saying "saved" for a queued change | P0 | M3 |
| L11 | Grant preview before the click, confirm at 10 or more missions or several houses, result sentence with missions, Solari, House Scrip, XP by line, XP left out, rewards left out, `GRANT_UNCHECKED` (`lbGrantOf`, `lbAskOf`, `lbGrantText`) | partial (Scrip appears in the mock text) | amounts must come from the server's `grant` key and the arithmetic of `grantRaise`, never constants in the page | P0 | M3 |
| L12 | Faction drop-down for Complete and Fill faction goal (`lbFacId`) | Yes (`edit-faction`) | default is the character's own faction | P1 | M3 |
| L13 | Reward state per tier: not reached, waiting, delivered, reached with no delivery recorded; swatch top tier (`lbHouse`, `LB_STW`) | Yes (reward ledger ids), with an "unverified" row | the four states and the swatch rule; `no_row` must be worded as ambiguous | P1 | M3 |
| L14 | Two scales: faction progress to goal, and personal tiers | Yes (inspector has both) | keep them apart | P0 | M3 |
| L15 | Show filter All / What is left / Not started / In progress / Top tier (`LB_SHOWS` 2067) | Yes (`editor-v8.js`) | keep | P1 | M3 |
| L16 | House filter Any / Open / Taken (`LB.hs`) | Yes | keep | P1 | M3 |
| L17 | Jump to house (`lsb-jump`) | Yes | keep | P1 | M3 |
| L18 | Sort by house, location, state, tiers, next tier, faction status; ties by name (`lbApply` 2375-2386) | not found | sorting is not a board feature; keep a list view | P1 | M3 |
| L19 | Group by region (`LB.group`) | not found | keep a list view | P1 | M3 |
| L20 | Place of the house, distance, link to the Saved map where the map has a picture (`landsraad_locations.go`, `app.js` 2451-2460, 2497) | not found | location and the explanation when there is no picture | P1 | M3 |
| L21 | Glance cards and bar that filter the list; Expand all / Collapse all | n/a | optional | P2 | M3 |
| L22 | Term and decrees panel: Make active (with a reigning faction), Apply, pool flag per decree (`lbTerm`, 2485-2496) | **not found** (the mock has a decree plaque, no controls) | three write controls lost | P0 | M6 |
| L23 | Decree display names and the "unsure" mark of an unverified name | not found | unknown names kept as they are | P1 | M6 |
| L24 | Term history: all terms, earlier terms with the character's houses | not found | read-only, but present today | P1 | M6 |
| L25 | `SetTerm` also accepts `elected_decree_id` and `end_time`; the page sends neither | n/a | no change; a new view must not start sending them without a decision | P2 | M6 |
| L26 | Result line, toast and a live-region announcement for every write (`say`, `sayAloud`) | not found | accessibility of the write result | P1 | M3 |
| L27 | `board_index` positions and the twelve lines | the mock draws them, from demo data | no production code reads `board_index`; new, no parity to keep | P0 | M3, M4 |
| L28 | Header facts (decree, owners, counts) from the real read | demo values in the mock | no static 12-4 in production (LCC-009) | P0 | M6 |

### 2.2 Player > Specialization

| # | Current feature (source) | In the mock? | Gap if not carried | Sev | Milestone |
|---|---|---|---|---|---|
| S01 | Five tracks with XP, level, keystone owned/total, Granted (`app.js` 958-984, `Specs`) | Yes (cards) | independence of XP and keystones (SPEC-003) | P0 | M5 |
| S02 | Add signed XP per track, no confirmation (`spAdd` 2972) | Yes (`sv-add-xp`) | -44,182..44,182, not 0; clamped; the server's answer shows before and after | P0 | M5 |
| S03 | Grant max per track, asks (`spMax`) | Yes (`sv-max-xp`) | keystones unchanged | P0 | M5 |
| S04 | Reset per track, asks (`spReset`) | Yes (`sv-reset-xp`) | keystones unchanged | P0 | M5 |
| S05 | Grant all keystones, asks (`spKeyAll`) | Yes (`sv-grant-keys`), documented as all-five global | global, never per track | P0 | M5 |
| S06 | Reset all keystones, asks (`spKeyReset`) | Yes (`sv-reset-keys`) | global; XP unchanged | P0 | M5 |
| S07 | Reset XP of all known tracks with a typed confirmation and the expected count (`spResetAll` 3007, `resetArea` 720) | Yes (`sv-reset-all`) | the typed confirmation and the count guard; unknown rows left alone | P0 | M5 |
| S08 | Track number assignment, from an unassigned row or by hand (`spAssign`, `spAssignHand`) | Yes (`sv-map-*`, `sv-mapping-list`) | refuse a number another track holds | P0 | M5 |
| S09 | Raw row: track, XP, level as typed, asks (`specSet`) | Yes (`sv-raw-*`) | mismatch of XP and level preserved, with the warning | P0 | M5 |
| S10 | Unassigned rows shown apart (`sp.unassigned`) | Yes (`sv-unassigned`) | keep | P1 | M5 |
| S11 | Controls off, with the reason, for a track with no number (`known(r)`) | partial | do not write an unknown number as known | P0 | M5 |
| S12 | Level shown as the floor of a fractional level; XP cap label | the mock marks LV100 as XP cap | LV100 does not mean all keystones | P1 | M5 |
| S13 | Landsraad grant adds XP to the same rows and respects the cap (`addSpecialization`, `SpecRoom`) | n/a | the cap note ("room") must stay visible in the Landsraad answer | P1 | M3, M5 |

### 2.3 P0 and P1 gaps, ranked

Production features that the mock does not show (a design or build decision is needed before the new view can replace the old one):

1. P0: bulk Fill / Complete / Reopen over chosen houses (L07).
2. P0: the stale-state guard and the one-write-at-a-time lock (L08, L09).
3. P0: the Term and decrees controls: Make active, reigning faction, pool flag (L22).
4. P0: queued versus written wording on every write (L10).
5. P0: the real grant preview and confirmation (L11).
6. P1: sorting, grouping by region, location and the Saved map link (L18 to L20).
7. P1: term history and decree names (L23, L24).
8. P1: the write result line and its announcement (L26).

New in the mock with no production source (must not be built from demo values): the 5 x 5 positions with lines, the plaque header counts, any leaderboard or mission list, a vote result (L27, L28; DECISIONS D02, D03, D07, D08m).

None of these is a regression of this milestone, which changes no behaviour. They are the checklist the approved design has to pass before the old views can go.

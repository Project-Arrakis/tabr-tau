# Findings register, 2026-11-15: delta audit L2 of the editor (tabr-tau)

Tracking issue: [#677](https://github.com/Project-Arrakis/tabr-tau/issues/677). Layer: **L2** (implementation audit through the eight hats of the organisation's Requirement 20), under `docs/decisions/2026-11-14-audit-cadence-and-no-peer-review.md`.

Base: `b4cede8`, the merge of the previous audit (`docs/audit/2026-11-14-full-audit-register.md`, #664). Range: `b4cede8..d1f0bf8` (main on 2026-11-15), seven merged pull requests, 36 files, about 1,770 lines added:

| PR | What | Risk |
|---|---|---|
| #674 | Decision record: audit cadence and no peer review | docs |
| #675 | The character's real place in Bring, Create, Duplicate and Teleport (#654) | High (writes to a save) |
| #676 | Public-text check over issue, PR and comment text; wider file scan (#653) | High (a CI control) |
| #678 | Landsraad house card: one waiting reward per line (#666) | Low |
| #685 | Vehicles tab keeps its place when the vehicle to create changes (#650) | Low |
| #687 | Redraws keep scroll and focus at many call sites (#686) | Medium (many call sites) |
| #688 | Count lines, the save name as a label, the Database pager (#681, #682, #684) | Low |

The Landsraad board pull requests #636, #637, #638 and #642 (and the reports #641, #644 to #647) merged before the base and were covered by the 2026-11-14 full audit (groups G-006 and the Landsraad rows there); this register does not repeat them. Since the base, only #678 (the house card) and #675 (the distance to a house) touch the board, and both are covered below.

Method: the code of each pull request was read at `d1f0bf8`, not only its description, with the callers of every changed function. No test run was needed for a finding: each one is shown by the code and, where it exists, by the pull request's own test. The texts of this register and of the issues were run through `cmd/publictextcheck -items` before filing. Single-player only. **Nothing was fixed by this audit.**

## Result

| Severity | Count |
|---|---|
| Critical | 0 |
| High | 0 |
| Medium | 2 |
| Low | 4 |
| Total | 6 |

No finding loses data on its own or opens a path from outside the machine. The two Medium findings are a gap in the new public-text control (L2-01) and a rule of #675 that moves a non-standing body between maps while it refuses the same body on one map (L2-04).

## Findings

| Id | Severity | Hat (category) | File and line | What is wrong | Failure scenario | Proposed fix | Issue |
|---|---|---|---|---|---|---|---|
| L2-01 | MEDIUM | privacy, correctness | `internal/hygiene/hygiene.go:90`, `:112-123` | A reserved entry matches only a whole token, or up to four whole tokens joined by one space. A reserved hyphenated name is missed when a hyphen or underscore part is added to it (one longer token), when it is written with a space or with markdown emphasis around one part, or with a non-ASCII dash, a zero-width character or a look-alike letter. | A branch named after the name plus a suffix, or a comment that spells it with a space, passes the check and stays public. | Normalise each line (NFKC, drop format characters, map dashes); also check each run of a token's `-`/`_` parts and the separator-free join of parts and of adjacent tokens; fixture-word tests for each form. | [#689](https://github.com/Project-Arrakis/tabr-tau/issues/689) |
| L2-02 | LOW | operations / CI supply chain | `.github/workflows/public-text.yml:33-38`, `:47-54` | On `pull_request` the job runs the pull request's own copy of `cmd/publictextcheck` and `internal/hygiene`, so a pull request that weakens the rules passes its own check (the file scan in CI has the same shape). The token and the secret are safe; the integrity of the gate is not. | A change loosens a pattern together with text the old rule refuses; every check is green. | Build the tool from `pull_request.base.sha` in a separate folder and run that binary; pin the number of built-in digests in a test. | [#690](https://github.com/Project-Arrakis/tabr-tau/issues/690) |
| L2-03 | LOW | operations | `.github/workflows/public-text.yml:8-18` | For issue, comment and review events the result is a workflow run only: no status on a pull request, nothing blocked, and only the account that triggered the run hears of a failure. | An outside account comments with a profile path; the run fails and the owner is not told. | A scheduled `-items` sweep that fails visibly, or a separate minimal job that adds a label carrying no text. | [#691](https://github.com/Project-Arrakis/tabr-tau/issues/691) |
| L2-04 | MEDIUM | correctness (write path) | `internal/ops/character_place.go:24-26`, `:86-88`; `internal/ops/teleport.go:199-206`; test `internal/ops/character_place_test.go:186-191` | The comment says a body whose state is not 0 is not the "maps differ" case; the code sets `ParkedMap` in the map branch before it reads the state. Teleport with a map then writes all three actors there and keeps the state, while the same body on the player's map is refused. The test pins the move with state 1. Moving a non-standing body was not checked in the game. | A body left on the travel map with state 1 is moved to the Deep Desert still marked as not standing, away from its vehicle; the game may not load it where expected. | One rule: refuse any body whose state is not 0 whatever its map, or, after a game check on a copy, set the state to 0 in the cross-map move and say so; align comment, code and test; test a non-standing body on another map. | [#692](https://github.com/Project-Arrakis/tabr-tau/issues/692) |
| L2-05 | LOW | correctness | `internal/ops/character_place.go:74-78`; used at `internal/ops/world.go:179`, `vehicle_duplicate.go`, `vehicle_spawn.go` | The dimension is the controller's when it has one, the body's otherwise; a disagreement is not treated as parked and no test covers it. Before #675 the body's dimension was used and Bring did not write the dimension. | Controller in dimension 0, body in another Deep Desert dimension: Bring, Create and Duplicate write the vehicle into 0, and the clear-spot search reads the wrong dimension. | Refuse "next to the character" when the two dimensions differ, or establish from game saves which actor holds it; add a test. | [#693](https://github.com/Project-Arrakis/tabr-tau/issues/693) |
| L2-06 | LOW | accessibility / UX | `internal/ops/teleport.go:148-151`; `internal/web/static/app.js` (no reader) | The place list returns `parked` with the reason the body is not where the player is, but the page never shows it; the person learns of it only from a refusal of Teleport, Bring, Create or Duplicate. | Distances that do not match the body, then a refusal that asks for a map; Bring to me is offered and then refused. | Show the sentence above the place list and by the Vehicles actions, or turn those actions off with the reason; page test. | [#694](https://github.com/Project-Arrakis/tabr-tau/issues/694) |

## Checked and found fine

**Security.** `public-text.yml` runs on `pull_request`, never `pull_request_target`; `permissions: contents: read`; checkout with `persist-credentials: false`; every action pinned to a commit SHA; no event text is interpolated into a `run` line (the payload is read from `GITHUB_EVENT_PATH`, and a hygiene test refuses the interpolation forms); the commit range comes from SHAs in `env`. The optional digest secret is absent for a fork's pull request and masked in logs. `-show` is refused when `GITHUB_ACTIONS` is set. The redraw changes (#685, #687, #688) add no new HTML sink: the waiting rewards of #678 go through the `html` template, the save name is set with `textContent`. No new server route; the loopback and same-origin rules are unchanged.

**Privacy.** The check's log names the place and the rule, never the text (pinned by `cmd/publictextcheck/main_test.go`). Reserved words are kept as digests only; the test file uses a fixture word. The profile-path rule covers drive and Unix forms, with placeholders and build-machine accounts allowed. The wider file scan reads every tracked text file whatever its extension and also checks file paths. The header now shows only the save's file name; the full path stays in the tooltip on the local page.

**Correctness.** #675: `readCharacterPlace` reads the controller, the body and the player state inside the edit (`m.One`), so a queued edit is held when the character's map or state changed and not when it only moved (tests at `character_place_test.go:222-257`); refusals leave the save untouched (tests compare all actors before and after); Teleport is one edit, the cross-map write still requires all three actors and the map's bounds; a place the person gave for Create still works with a parked body and takes the controller's map only for the dimension; the Landsraad distance is left out when the body is parked. #678 keeps the content and order of the rewards. #688: the pager shows for more than 100 rows or a later page; the count line uses the same `nHave` as the filter buttons; `#savename` is hidden while no path is known.

**Reliability.** #687: `render('scroll')` keeps the page and table offsets and the open details, and drops typed values (`place.ctl = {}`), which is right for a redraw that shows the save after a refused change (the two conflict paths) and no worse than the plain `render()` it replaced at the other call sites: every changed call site redraws its view from state (`invQ`, `dbQ`, `vcDraft`, `kitDraft`, `giveQueue`) and still fetches the data again. `vcSync` runs before the vehicle redraws, so fuel, parts and quantities are kept. `dbRowsTop` runs after the awaited redraw, so a new search or page starts the table box at the top. `intoView` moves the page only when the refocused control is off screen or under the sticky header or the sticky warning; it skips controls with no layout.

**Performance.** `readCharacterPlace` is three indexed reads per action; `intoView` reads a few rectangles once per redraw; the public-text job is one short Go build per event with a 10 minute limit and a concurrency group that cancels a superseded run of the same text.

**Accessibility / UX.** The house card's rewards are separate block elements, read as separate lines; the save name is a label, not a control; the pager is not drawn when it could only be disabled.

**Maintainability / tests.** Each pull request adds tests: `character_place_test.go` (parked body, standing on another map, dimensions, held queued edits), `cmd/publictextcheck/main_test.go` and `internal/hygiene/hygiene_test.go` (events, items, commits, profile paths, attribution, the workflow's shape), `web-tests/redrawplace.test.mjs`, `vehiclecreate.test.mjs`, `polish.test.mjs`, `landsraad-board.test.mjs`. CI was green on each merge.

**Operations / CI supply chain.** The decision record (#674) states the audit cadence, the lack of peer review and its compensating controls. Already tracked on #677 and not filed again: adding `public-text` to the required checks of main (it is not in the list today), setting the optional digest secret, and the wording leftovers of #653.

## Not covered

- No game check: L2-04 and L2-05 are about what the game does with the written rows; they need a check on a copy of a save.
- The jsdom page tests and the Go tests were not run again; CI ran them on each merge.
- L3 (the whole-diff review) is a separate layer and is not part of this register.

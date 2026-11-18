# Command Center M6: Term and Decrees, Sysselraad Strategy and the way between the pages

This note records what was built for milestone M6 (issue #768) and lists every element and behaviour that the approved design and the milestone prompt ask for in this area, where the editor stands now, and why anything is still different. Nothing was dropped without a row.

Method: the approved prototype (its Sysselraad Strategy view, and the plaque, which already reads the save since M3) and the editor page were drawn side by side in a headless browser at 1600, 1280, 1024, 768, 390 and 320 px, dark and Dayside (light), with the synthetic answer of M1. No game save was read and nothing was checked in the game.

## What changed

* **A Term and Decrees section** in the Command Center board view, drawn by `internal/web/static/lcc/lcc-term.js` and `lcc-term.css`, in place of the classic term panel and earlier-terms list in that view. It shows the term id, the start and end in play-clock seconds, the reigning faction, the active and elected decree with their pictures, the two drop-downs with Apply, one row per decree of the pool (Make active, Set flag to 0/1, the term states and the save's pool flag shown apart), the hidden plain SpecialVendorActive row in a closed disclosure with a warning, and the earlier terms, read-only.
* **Sysselraad Strategy** as a switch on the Landsraad board (off by default): a list of the twelve lines (five rows, five columns, two diagonals) with a badge and a plain sentence each, five small squares of the Houses as they stand, an order switch (board order or closest first), a chosen line marked on the board (violet outline for a preview, gold for a line held, with a word as well as a colour), "Jump to missing House" and "Clear line". Escape lets go of the line.
* **Between the pages**: the board has "Term and decrees" (moves the focus to the section), and the House Inspector has "Open <path> in Specializations" (chooses the path in the Specialization view and moves there; it only changes the page).
* **No new route, no new request body, no save-editing function changed.** The new section reuses the classic controls' `data-lb` attributes, ids and focus keys, so the page's one click and change handler, its one request builder, its one result line and its one write queue serve both views. The classic view is unchanged and stays the default.

## Honest-data rules that the view follows

* Real values from `GET /api/landsraad` only. Whatever the save does not hold is said: "Not in the save", "Active Landsraad missions are not recorded in this local save", "Top contributors are not recorded in this local save".
* The prototype's Decree Preview drop-down is kept as a sample and labelled so; choosing one puts a DEMO badge on the plaque, and nothing is written.
* Strategy words never say a faction wins: "FIVE IN A LINE", "ONE AWAY", "POTENTIAL", "BLOCKED", "UNREADABLE"; a line that is not held says "a preview, not a result". The save records no start of the Voting Session, so no snapshot of it is shown and Dominion is never confirmed.
* The decree picture comes from the existing id-based mapping; a decree with no verified picture (the hidden row) gets the neutral picture and an empty alt text.

## Proof that nothing else was changed

* `web-tests/lcc-term-requests.test.mjs` plays 14 scenarios (Apply with each combination, Make active on a row, for the other faction, on the hidden row, with no faction chosen, Set flag to 0 and 1, and a refusal by the save for Apply and for Set flag) against the classic panel and against the new section, and compares the requests, the result line, the focus and the drop-down values with `web-tests/lcc-term-requests.golden.json`, which was recorded from the classic panel with `LCC_RECORD_REQUESTS=1`.
* `web-tests/lcc-m6.test.mjs` covers the section, the strategy list, the line marks, the jump, the cross-page buttons and that nothing is sent by looking.
* `web-tests/lcc-term-css.test.mjs` covers the styles (scope, own properties, no address, text floor, variants, targets).

## Totals

| | matches | differs (intentional, needs approval) | differs (data or wording rule) | not built (reason given) | rows |
|---|---|---|---|---|---|
| After this pass | 26 | 4 | 4 | 5 | 39 |

## Matrix

### Term and Decrees (milestone prompt, workstream B and E)

| Row | Approved element | Now | Status | Why |
|---|---|---|---|---|
| T1 | Term id, start and end in play-clock units, reigning faction | Same four facts, with the faction crest; start and end in seconds with the note that no calendar date is stored | matches | |
| T2 | Active and elected decree with display name, effect, ids | Both, with picture, effect, internal id | matches | |
| T3 | Active decree and Reigning faction selectors, Apply both together, empty allowed | Same ids and values, same request | matches | |
| T4 | Make active per row for the chosen faction via the term route | Same button, same request; off with the reason while no faction is chosen; "Active" mark on the row that is | matches | |
| T5 | Set flag to 0/1 per row via the decree route, only the disabled column | Same | matches | |
| T6 | Active, elected, in rotation apart from the pool flag | "IN THIS TERM" badges and a separate "POOL FLAG" value | matches | |
| T7 | Hidden SpecialVendorActive row in a collapsed disclosure with a warning | Same | matches | |
| T8 | Readable sort, variant hints for duplicate display names | Same sort as the classic panel; the variant is in the name | matches | |
| T9 | Earlier terms, read-only, with house contribution, tiers, completion faction, payment record | The classic list's content in a disclosure per term, still read-only, with its disclaimers | matches | |
| T10 | Source badge, queued notice, focus restoration, disabled while writing, no duplicate write, conflict recovery | All through the classic handler (the same `lbRun`) | matches | |
| T11 | Decree art by id and variant, neutral placeholder when unverified, alt text and an independent effect text | Existing id mapping; neutral picture with empty alt for an unmapped decree; the effect is always text | matches | |
| T12 | Link from the hero to the management panel | "Term and decrees" button on the board moves the focus to the section | matches | |
| T13 | No plain select in the hero that writes | The hero's selector is the sample picker and writes nothing | matches | |
| T14 | Section looks like the approved surfaces | Same tokens, serif titles, gold Apply; a row stacks its buttons under the text at 640 px and below | differs (intentional) | The prototype has no term section; this is an original layout in its style. |
| T15 | Where the section sits | Under the board and the action bar, in the board view | differs (intentional) | Needs the owner's view; a tab of its own was not built. |
| T16 | Active Landsraad missions | "Not recorded in this local save" | differs (data rule) | The save holds no mission rotation. |
| T17 | Top contributors with names | "Not in the save" on the standards and in the section | differs (data rule) | The save holds your own contribution, not a ranking. |
| T18 | Decree Preview selector with demo decrees | Kept as a sample, labelled, DEMO badge on the plaque | differs (data rule) | The ornamental selector must not be taken for the real editor. |

### Sysselraad Strategy (approved view `view-strategy`)

| Row | Approved element | Now | Status | Why |
|---|---|---|---|---|
| S1 | Eyebrow "SYSSELRAAD STRATEGY MODE", title "Twelve Paths to Victory", lead | "SYSSELRAAD STRATEGY", "Twelve lines", the same lead without the word victory | differs (wording) | Wording rule: a line is a preview, never a result. |
| S2 | "12 POSSIBLE LINES" tag | The list has twelve rows; the plaque and the status tile say how many are still possible | matches | |
| S3 | Victory Geometry: a second copy of the board with a circuit path drawn | The one board is marked in place; no second copy and no drawn circuit | differs (intentional) | One board, one state; the marks are outlines and words, so nothing depends on colour. Needs approval. |
| S4 | Toggle Victory Scenario button (a demo) | Not built | not built | It shows an invented scenario. |
| S5 | List of twelve lines with name, direction and five squares, and a count or BLOCKED | Name with its Houses, a badge, five squares, a sentence | matches | |
| S6 | Selecting a line highlights its Houses | Same; a missing House is marked MISSING, a claimed House POTENTIAL | matches | |
| S7 | Rule note: a mixed line is blocked; a lead is not a win | Same, in the list's lead and in the plaque's note | matches | |
| S8 | Colours: teal Atreides, rust Harkonnen, gold completed, purple hypothetical | Same, with shapes and words as well | matches | |
| S9 | Strategy as its own tab of three (Board, Specializations, Sysselraad Strategy) | A switch on the board | not built | The tab strip is not decided; see the proposal below. |
| S10 | Order of the list | Board order, or closest first (an addition) | matches (added) | |
| S11 | Jump to the House a line is missing | Added | matches (added) | |
| S12 | Escape and Clear line | Added | matches (added) | |
| S13 | Narrow windows | Side column stacks below the board; targets 44 px at 900 px and below | differs (intentional) | The prototype clips at 390 px. |

### Integration (milestone prompt, workstream C)

| Row | Element | Now | Status | Why |
|---|---|---|---|---|
| C1 | House's specialization to the path dossier | "Open <path> in Specializations" in the Inspector | matches | |
| C2 | Path's Houses to the House card and Inspector (M5) | Unchanged, still works | matches | |
| C3 | One snapshot for plaque, tally and engine | One answer read, one board model | matches | |
| C4 | Fill with specialty XP rereads the specialization view | The page reads again after a write; the Specialization view reads on entry | matches | Checked in the page code and tests, not in the game. |
| C5 | Original view as a reversible fallback | Classic list stays the default and the switch is remembered | matches | |
| C6 | Go route tests for term and decree | Not added: no route changed | not built | The existing Go tests of the routes are unchanged and pass. |
| C7 | Real mission and Top 5 sources | None exist locally | not built | Stated in the section. |
| C8 | Pixel match of the section in the in-app window | Checked in headless Edge only | not built | Needs the owner's eyes in the real window. |

## Masthead tab strip: proposal only, not built

The approved prototype has a strip of three tabs (Board, Specializations, Sysselraad Strategy). The editor already has its own tabs, and the Specialization view is a switch inside its own tab. A strip would duplicate the shell. The proposal: keep Strategy as the switch on the board, keep the cross-page buttons, and decide the strip when the owner has seen the three pages together.

## Known differences that need the owner's approval

1. One board marked in place for Strategy instead of a second copy with a circuit line (S3).
2. The Term and Decrees section sits under the board in the board view (T15).
3. The section's own layout, stacked at phone width (T14).
4. The side column stacks below the board and targets grow to 44 px at 900 px and below (S13).

Not a difference but an open question: the tab strip (S9, proposal above).

## Not verified

Nothing was checked in the game. No real save was read (only the synthetic answer of M1). No real phone was used. Screens were taken in headless Edge only.

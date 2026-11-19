# Command Center M5: the Specializations tab against the design reference

This note records what was built for milestone M5 and lists every visible element and behaviour of the approved Specializations view, where the editor stands now, and why anything is still different. Nothing was dropped without a row. The design reference is a visual mock-up kept outside this repository; the rows quote what it shows, so they can be read without it.

Method: the design reference and the editor page were drawn side by side in a headless browser at 1600, 1280, 1024, 768, 390 and 320 px, dark and Dayside (light), with the same five tracks in both (Crafting at the XP cap with every keystone, Gathering with every keystone and no XP, Exploration at 100 XP, Combat at 12,345 XP with 7 of 41 keystones, Sabotage at 44,181 XP with 20 of 41). The editor page was fed the synthetic answers of M1 (`fixtures/api-player-specs.cases.json` and `fixtures/api-landsraad.synthetic.json`); no game save was used and nothing was checked in the game. The design reference is a layout reference only: its numbers are demonstration data, ours come from the save.

## What changed

* **A second view of the Specialization tab**, drawn by `internal/web/static/lcc/lcc-spec.js` and `lcc-spec.css`: a hero with three totals, five path lanes, a progression dossier for the chosen path, the editor's own operations beside it, and the Houses of the path (the track mapping and the raw-row writer were taken out of this view in #986; they stay in the classic list).
* **A switch above the tab** (Classic list / Command Center), remembered in the browser under `spec.view`. The classic list stays the default, as the Landsraad tab's board switch does, until the project owner chooses otherwise.
* **No new route, no new save-editing logic.** Every control sends the request the classic list sends, through the same handler, with the same confirmation, the same backup and the same queue behaviour. The one new handler line, `spAddCC`, only checks the typed amount and then calls the existing `spAdd` (see "Write-adjacent lines").
* **The levels come from the game's cumulative XP table** (101 values, ported into the page and checked in a test against `internal/ops/specialization.go`), never from a linear formula. Level 100 means the XP cap; "full path" needs the cap and every keystone, because the two are independent.

## Honest-data rules that the view follows

* Real values from `GET /api/player/specs` only. A value the save does not hold is drawn as "Not in the save".
* A track whose number cannot be tied to a path shows no XP and no level; its XP controls are disabled with the reason beside them; the keystone controls stay, because they act on every track.
* A path with no keystone count in the save (total 0) says "Not in the save" and is never "complete".
* If the level the save stores differs from the level its XP gives, the page says so and corrects nothing.
* If the Landsraad cannot be read, the Houses of a path are "Not in the save" and the rest of the page works.
* No XP, level, keystone or House is invented. The design reference's per-path descriptions, waypoint levels and keystone notes are not copied, because the save and the game files do not hold them.

## Totals

| | matches | differs (intentional, needs approval) | differs (data or wording rule) | not built (reason given) | rows |
|---|---|---|---|---|---|
| After this pass | 34 | 5 | 7 | 5 | 51 |

## Matrix

### Hero and totals

| Row | Approved element | Now | Status | Why |
|---|---|---|---|---|
| H1 | Eyebrow, two-line serif title "SPECIALIZATION COMMAND" in a gold-edged panel | Same, with a real 24 px floor on a phone (the title wraps by letters rather than clipping at 320 px; the design reference clips there) | matches | |
| H2 | Shield with a cross mark on the left | Drawn with the same shape, decorative, hidden from assistive technology | matches | |
| H3 | Lead sentence "...The controls below mirror TABR Tau's current save-editor operations." | "Five disciplines. One progression dossier. The buttons below are the editor's own save operations." | differs (wording) | The design reference names the product and calls itself a mirror; the editor says what the buttons are. |
| H4 | Three totals: tracks at XP cap, keystones, maximum XP per track | **Two totals (#1042):** SPECIALIZATIONS COMPLETED (renamed from TRACKS AT XP CAP; the count is the same, the paths whose XP is at the cap, with a tooltip that says so) and KEYSTONES (was KEYSTONES PURCHASED, #1031). The MAXIMUM XP / TRACK tile is dropped; the XP ceiling stays on the dossier. The two tiles sit side by side at every width | differs (operator-requested) | Operator decisions 2026-11-19. |
| H5 | "Sample save data, local demonstration" pill | None. The label "From your save, read when this page was drawn" was dropped (#1029) | differs (dropped) | The design reference's pill labels demonstration data; ours first said where the numbers come from and when, and the operator decided on 2026-11-19 that the label is not wanted. The header is the eyebrow and the title only. |
| H6 | Totals when a track is unidentified | SPECIALIZATIONS COMPLETED counts identified tracks only; the keystone total carries a star and its label says that a track is not identified yet (or that a track's total is not in the save) | matches (added) | The design reference has no unidentified track. |

### Lanes

| Row | Approved element | Now | Status | Why |
|---|---|---|---|---|
| L1 | Five lane cards in a row, one per path, in the order Crafting, Gathering, Exploration, Combat, Sabotage | Same order and row at 1000 px and wider | matches | |
| L2 | Lane icon: a diamond crop of the in-game badge | The scroll icon of the path from the approved art already in the repository, in a square frame | differs (intentional) | The diamond crops of the design reference are not among the repository's approved art; the scrolls are. |
| L3 | Name, "LV n / 100", XP bar, "n XP · k/41 keystones", state badge | Same five parts, from the save | matches | |
| L4 | Badge text "XP CAP REACHED" or "PATH IN PROGRESS" | "FULL PATH COMPLETE" (cap and every keystone), "XP CAP REACHED", "KEYSTONES COMPLETE", "PATH IN PROGRESS", "TRACK NOT IDENTIFIED" | differs (data rule) | The design reference's badge mixes the cap and the keystones; the save keeps them separate and the badge says which. |
| L5 | Chosen lane has a gold edge and glow | Same, glow read from one dial set to 0.5; the chosen lane also keeps a thicker border (not colour alone) | matches | |
| L6 | Lane cards are buttons that choose the path | Same: `aria-pressed`, keyboard, focus ring | matches | |
| L7 | Narrow layout: one lane per row | 5 across, then 3, then 2, then 1 (at 380 px and below) | differs (intentional) | Two per row at a phone width keeps the page short; the text never shrinks. |
| L8 | (no unidentified lane) | Dashed edge, "Not in the save" where XP would be, "TRACK NOT IDENTIFIED" | matches (added) | Needed for honest data. |

### Progression dossier

| Row | Approved element | Now | Status | Why |
|---|---|---|---|---|
| D1 | Chosen path name, icon in a diamond, a description line, a level badge | Name and icon (scroll, square frame); no description line for a path the save identifies (the number behind a path is not shown, #708); a path that is not identified says so | differs (data rule) | The design reference's descriptions ("Fabrication, augmentation and equipment efficiencies") are not in the save. |
| D2 | Large level "100 /100" with a glow | Same, level from the XP table | matches | |
| D3 | "XP CAP REACHED" under the level | "FULL PATH COMPLETE", "XP CAP REACHED", "KEYSTONES COMPLETE" or "XP CAP NOT REACHED", and nothing else on the line: the level is the plate above it and the keystone count is the bar below (#1053 dropped "LEVEL 100 ·" and "KEYSTONES 41/41" from it); the gold line carries a star on each side, drawn only for a track at level 100 (the XP cap), never below it | matches | |
| D4 | XP bar with "44,182 / 44,182" and a note | Same, with "n XP to level k and m XP to the cap" under it; the label reads XP (was XP ACQUIRED, #1031) | matches | Operator decision 2026-11-19; the bar keeps its name "Experience of the selected path" for screen readers. |
| D5 | Keystone bar "41 / 41" with a note | Same, and the note says the cap and the keystones are independent; the label reads KEYSTONES (was KEYSTONES PURCHASED, #1031) | matches | Operator decision 2026-11-19; the bar keeps its name "Keystones purchased on the selected path" for screen readers. |
| D6 | "LEVEL WAYPOINTS", seven markers (0, 10, 25, 42, 60, 80, 100), "Illustrative markers" | "LEVEL SCALE", five reference marks (0, 25, 50, 75, 100), "Reference marks along the 100 levels. They are not unlock levels." | differs (intentional) | The design reference's seven waypoints are illustrative. The game has no named waypoints at those levels, so the page draws neutral quarter marks. |
| D7 | The mark at the current level is underlined or lit | The nearest mark at or below the level is underlined (not colour alone) | matches | |
| D8 | Three fact tiles: associated Houses, XP ceiling, keystones | Not drawn (#1053) | removed (#1053) | Each tile repeated a figure shown beside it: the Houses are the panel below, the XP ceiling is the end of the XP bar ("44,182 / 44,182"), the keystones are the keystone bar. |
| D9 | Level mismatch between stored and computed | A warning line with both levels; nothing is corrected | matches (added) | |

### Operations panel

| Row | Approved element | Now | Status | Why |
|---|---|---|---|---|
| O1 | "Tau editor parity" eyebrow, "Specialization Operations", a line about mock data | "The editor's own operations", the same title, "Every change asks first, is written after a backup, and says whether it was saved or is queued until the game closes." | differs (wording) | The design reference's line is about mock data and production; ours is about what happens. |
| O2 | Individual group: path name, "Add or remove XP" field, Apply XP, Grant LV 100, Reset Track XP, a note | Same controls, the field labelled "Add XP" (#1032: XP is added only, so a minus sign, zero and a non-number are refused and removing XP is the Reset button's job); the note says what Grant LV 100 and Reset do and that neither changes a keystone | matches | |
| O3 | Apply XP with a whole-number rule (-44,182 to 44,182, not zero) | A whole number from 1 to 44,182, checked before the request and again by the write path (`AddSpecXP` refuses 0 and negative numbers and writes nothing); a refused amount shows its reason under the box and in the toast and sends nothing | differs (#1032) | Operator decision 2026-11-19: no negative numbers; XP goes down only through Reset. |
| O4 | "Total clamps to 0 to 44,182" | A hover or focus tip (#1030): "Whole number from 1 to 44,182 ... The total stays at most 44,182. The editor shows the XP before and after." | differs (wording) | Says what the editor does after the write (the existing result line); no standing sentence on the page. |
| O5 | Global group: Grant All Keystones, Reset All Keystones, Reset XP of All Known Tracks | Same three, in a labelled group "All Five Tracks, Global operations"; the keystone line says "109 of 205 purchased across all 5 tracks" | matches | |
| O6 | Reset All Known Tracks asks for a typed sentence | Same existing question, same sentence ("reset N tracks") | matches | |
| O7 | Confirmation dialogs for Grant LV 100, Reset Track XP, keystone grant and reset | The existing dialogs, unchanged (golden request test) | matches | |
| O8 | In-panel feedback line "Ready. No save is connected." | The page's existing toast and queue hint | differs (intentional) | One place says what was saved or queued, in the editor's usual voice, for every tab. |
| O9 | Disabled controls | An unidentified track disables Apply, Grant LV 100 and Reset Track XP with the reason beside them | matches (added) | |

### Houses of the path

| Row | Approved element | Now | Status | Why |
|---|---|---|---|---|
| R1 | "Landsraad Intelligence", "Associated Houses", five House cards with symbol and name | Same section, Houses read from the Landsraad answer by their specialization line, with their symbols | matches | |
| R2 | A House card opens that House on the Landsraad board | Choosing a House opens the Landsraad tab on the board with that House chosen; nothing is written | matches | |
| R3 | "Associations are tracker-derived and must be verified before production." | "The Houses of this path, from a public community tracker, not from the game or the save: treat it as a reference." | differs (wording) | Says where the data comes from for the person using it. |
| R4 | (reverse link from a House to its path) | Not built | not built | The Landsraad board code is out of scope for M5. |
| R5 | A path with no Houses | One line, "Not in the save", and the rest works | matches (added) | |

### Advanced editor (retained features)

| Row | Approved element | Now | Status | Why |
|---|---|---|---|---|
| A1 | "Existing Tau features, retained", "Track Mapping and Advanced Editor", a disclosure | Not drawn in this view (taken out in #986); the classic list keeps the section | removed (#986) | The mapping and the raw-row writer stay in the classic list, which the switch above the tab still offers. |
| A2 | Closed unless a track cannot be tied to a path | One read-only line, "Unknown specialization row in the save (not edited): number n", shown only when the save holds a row of XP that no path owns | differs (intentional, #986) | The only part of the old section that the lanes cannot show; nothing in it writes. |
| A3 | Track numbers, Assign by number, Assign by hand, Exact raw row | Not drawn in this view; all four stay in the classic list with their ids and handlers, and the raw-row warning says nothing checks that XP and level agree | removed from this view (#986) | Same reason as A1. |
| A4 | Footer "Source of truth: GET /api/player/specs ..." | Not built | not built | Chrome of the design reference only. |

### Page-level behaviour

| Row | Approved element | Now | Status | Why |
|---|---|---|---|---|
| P1 | The view replaces the old Specialization list | A switch offers both; Classic is the default | differs (intentional) | The project owner decides when the Command Center view becomes the default. |
| P2 | Choosing a lane keeps its place after an edit | The chosen path survives the write and the redraw; focus returns to the button pressed | matches | |
| P3 | Dark and Dayside | Both, from the app's tokens; forced colors use system colors; reduced motion and the Motion setting remove the lift | matches | |
| P4 | Text sizes | Names and key labels 12 px or more, other text 11 px or more, narrow windows reflow instead of shrinking | matches | |
| P5 | Touch targets | Buttons and inputs 40 px, 44 px with a coarse pointer | matches | |
| P6 | Design reference demo toolbar and view tabs | Not built | not built | Chrome of the design reference only. |
| P7 | Design reference's "Sample save data" banner | Not built | not built | Chrome of the design reference only. |
| P8 | Reload / write queue behaviour | Unchanged: the page reads the save again after a write | matches | |
| P9 | Glow strength | One dial, 0.5 | matches | |
| P10 | Design reference's Landsraad V20 banner | Not built | not built | Not part of this tab. |

## Differences that need the project owner's approval (the five intentional ones, and the one House link not built)

1. **Scroll icons instead of diamond crops** (L2, D1). The repository's approved art has the scrolls; the diamonds are not in it.
2. **Level scale, not the illustrative waypoints** (D6). Five neutral marks, labelled as reference marks.
3. **Two lanes per row on a phone, and one at 380 px or less** (L7).
4. **Feedback goes to the editor's own toast, not an in-panel line** (O8).
5. **The view is behind a switch, Classic stays the default** (P1).
6. **Reverse House-to-path navigation is not built** (R4) because the Landsraad board code is out of scope.

## Write-adjacent lines

No route, no request body and no write function was added or changed.

* `A.spAddCC` in `app.js`: reads the amount typed in the Command Center's XP field, refuses it with a reason if the editor would refuse it (blank, zero, not a whole number, outside plus or minus the maximum), and otherwise calls the existing `A.spAdd` with the same data. The classic list keeps calling `spAdd` directly.
* The click handler for `[data-spview]` stores the chosen view in the browser and redraws. It sends nothing.
* `specGoHouse` in `app.js` sets the Landsraad board's chosen House and switches tab. It sends nothing.
* `server.go` serves the two new static files.

Request equivalence: `web-tests/lcc-spec-requests.test.mjs` plays sixteen scenarios (add and remove XP, grant LV 100, reset a track, grant and reset all keystones, reset all known tracks with the typed sentence right and wrong, assign by number, assign by hand, two raw rows, and an answer of No to each question) in both views and compares what was sent, what was asked and what was said with `web-tests/lcc-spec-requests.golden.json`, which was recorded from the classic list.

## Not verified

* Nothing was checked in the game, and no real save was read or written; every run used the synthetic answers.
* The page was drawn in a headless browser; no one has looked at it on a phone.

# Landsraad house specializations: research and mock-up (refs #646, #609)

Research and a static mock-up only. No editor code changes with this page. Everything below comes from the repository's own code and documents, the measurements recorded on #609 and #646, and one read-only look at a check copy of a single-player save. Where something is not known, it says so.

The mock-up is `d-specialization.html` in this folder (see "The mock-up" at the end).

## 1. The question

Every Landsraad house raises one of five skill lines when its missions are completed: Combat, Crafting, Exploration, Gathering, Sabotage. #646 asks for a small icon and the character's level in that line on every house row, a summary of the five lines at the top of the tab, and a sort and a grouping by line, so a player can pick houses by the line they want to level.

## 2. What the save holds

| What | Where | Notes |
|---|---|---|
| The character's XP and level in a line | `specialization_tracks` (`player_id`, `track_type`, `xp_amount`, `level`) | `player_id` is the controller actor id of the character; the editor reads the first row of `player_state` (`player()` in `internal/ops/ops.go`). Primary key (`player_id`, `track_type`). `track_type` is a bare integer, `xp_amount` an integer, `level` a REAL. Documented in `docs/save-database.md`; read and written today by `internal/ops/specialization.go`. |
| Which number is which line | measured, not stored | The save does not name the lines. Measured on #609: 1 Crafting, 2 Gathering, 3 Exploration, 4 Combat, 5 Sabotage. The editor today learns the numbers per save (`TrackNumbers` in `internal/settings/settings.go`, the Assign button on the Specialization tab). |
| A row for a line | created by the first XP in it | A line the character never earned XP in has no row. Seen again on a check copy: one row only (track 1, 36,100 XP, level 87.05). A write must insert, not only update (the third #609 sample created track 4 that way). |
| The keystones of a line | `specialization_keystones_map` (`name`), `purchased_specialization_keystones` | 205 keystones, 41 per line, named by line (`Combat_CombatKeystone_SkillPoint1`, and so on). Not needed for the Landsraad tab. |
| The houses | `landsraad_tasks.house_name` | One row per house per term, `UNIQUE (term_id, house_name)`. The names are `DA_House<Name>`; the check copy holds exactly 25 distinct names in all six terms. The editor already strips the prefix (`lbNameOf` in `internal/web/static/app.js`). |
| Which line a house raises | **nowhere in the save** | `landsraad_tasks` has no line column, and no other table of the save links a house to a line. The link has to be a fixed table in the editor (section 3). |
| A house's place | **nowhere in the save** | Already known (`internal/ops/landsraad_locations.go`, #599). The tracker's regions in section 3 are only a cross-check for that table. |

Level arithmetic: the editor's XP table (`specCumulativeXP` in `internal/ops/specialization.go`, level 100 at 44,182 XP) reproduces every stored level that was measured: 18,000 XP is level 57.37, 21,000 is 62.63, 24,000 is 67.64, 3,000 is 18.92 (all #609) and 36,100 is 87.05 (the check copy). So the editor can show the level it reads, or recompute it from the XP, and they agree.

## 3. Which house raises which line

Source: the public community Landsraad tracker (`https://htwsguild.com/Molbork/lr_tracker.html`), read from its page source on 2026-11-14 and recorded on #609, to be stored in the repository with this provenance. It is the only source in the repository's reach; this step did not read the game's own data tables for it (#646 asks for that as the preferred source, with the community pages as a cross-check by eye). Two samples agree with it: House Argosaz raised track 3 (Exploration) and House Mikkarol raised track 4 (Combat).

The table uses the names the save writes. Two differ from the tracker's spelling: the save has `Mikkarol` (tracker: Mikarrol) and `Spinette` (tracker: Spinnette). The house symbol files and the board already use the save's spelling; mock-up A's sample data uses the tracker's. The recommendation is to key everything by the save's name.

| Line | Houses (five each, 25 in all) |
|---|---|
| Combat | Imota, Mikkarol, Moritani, Novebruns, Thorvald |
| Crafting | Alexin, Ecaz, Hagal, Lindaren, Maros |
| Exploration | Argosaz, Hurata, Taligari, Varota, Wallach |
| Gathering | Dyvetz, Kenola, Sor, Tseida, Wayku |
| Sabotage | Mutelli, Richese, Spinette, Vernius, Wydras |

Regions and places from the same tracker page (kept here only for the mock-up and as a check for the location table of #599): Arrakeen (Mutelli, Varota, Wallach); Harko Village (Alexin, Spinette); Deep Desert (Maros B8, Wayku A2); Hagga Basin (18 houses): Argosaz Griffin's Reach, Tseida Anvil Tradepost, Dyvetz and Moritani Riftwatch, Thorvald Hagga Rift, Novebruns Western Vermillius Gap, Ecaz Southern Hagga Rift, Richese and Vernius Helius Gate, Hurata Eastern Vermillius Gap (Testing Station 10), Hagal Eastern Shield Wall, Taligari Crossroads Tradepost, Wydras Sheol, Kenola Mysa Tarill, Lindaren Western Shield Wall, Mikkarol Jabal Eifret Al-Janub, Sor Pinnacle Station, Imota (no named place). That is 3 + 2 + 2 + 18; the sample counts in this folder's README (17 on the Hagga Basin, 8 elsewhere) are placeholders and should follow the real location table.

## 4. How missions move the lines

Measured on copies of a save before and after a mission (#609):

* One completed mission adds `1,500 x LandsraadSpecializationXpMultiplier` XP to the house's line, from the Recollection reward on the mission card. The measured save has the multiplier at 2.0, so +3,000 XP. The setting's default is 1.0 (`internal/config/defaults.go`), which gives +1,500.
* Seen exactly twice on Exploration (Argosaz: 18,000 to 21,000 and 21,000 to 24,000 XP) and once on Combat (Mikkarol: a new row at 3,000 XP). Crafting, Gathering and Sabotage missions each moved their track (1, 2, 5) to the cap of 44,182 XP from level 99, so the exact gain is hidden there; the card showed the same values for every line.
* The first reading on #646, "1.5 times the contribution", was a coincidence and is withdrawn on #609: the base is 1,500 on the card, the contribution base is 1,000, and both are scaled by their own setting.
* A mission changes only the row of its own line. The character's level, the currencies and the house tables change as recorded in the #609 template; the house side is already written by the board.

So an icon and a level per house row are worth showing: the level moves with the missions of the houses that carry the icon.

## 5. The icons

What the game shows, in plain words: the Skills screen has a Specializations page with five rows. Each row has its own icon, a level from 0 to 100 and its own XP. The operator's description on #646, checked by eye against the community specializations page: Combat is crossed blades, Crafting a gear with a crate, Exploration a compass, Gathering a hand holding a crystal, Sabotage a wrench and a gear. The five names and the five rows match the five track numbers above.

Reuse. The icons are the game publisher's artwork. The repository already ships some game pictures (two map pictures, the marker icons and the 25 house symbols) under `docs/art-notice.md`, which says they are not covered by the MIT licence, are not licensed by this project and are removed on request. Adding five more would widen that notice, and the community page's pictures are not ours to reuse at all.

Recommendation: draw five original, simple glyphs (24 by 24, one stroke weight, `currentColor`), each with a different silhouette and always next to the line's name, so meaning never rests on the picture or on colour alone. The mock-up draws such glyphs as inline SVG, as a first proposal for the design review; they borrow only the plain ideas (blades, gear and crate, compass, crystal, wrench), not any drawing. Nothing is copied from the game or from a website.

## 6. Known and unknown

Known
* The five line names, the five track numbers, the table and its columns, how a level follows from XP, the cap (level 100, 44,182 XP), that a row appears with the first XP.
* The 25 houses as the save names them, five per line, from the tracker.
* A mission adds 1,500 times the specialization multiplier to its line (section 4).

Unknown
* Whether the game's own data holds the house to line link in a form the editor could read instead of the tracker's table (not looked for in this step).
* Whether the 1,500 is the same for every mission and every house. Five missions on five lines showed the same card values; Crafting, Gathering and Sabotage amounts are hidden by the cap.
* Whether a mission completed without turning in a Recollection also pays the specialization XP (the card lists it under the Recollection reward).
* The daily and weekly limits a community guide quotes (625 XP a day, 35 missions a week). Its per-mission figure (125) does not match what the game showed (1,500), so the limits are unverified.
* Whether a new patch renames or adds a line. The mock-up and the table assume exactly five.

## 7. The mock-up

`d-specialization.html`: a static page, inline CSS and SVG, no script, no external request. Light and dark follow the system through the editor's colour tokens, with a mock-up control to force either. The CSS-only controls need `:has()` (Chrome 105, Safari 15.4, Firefox 121 or newer); without it the page shows the default view.

* Header summary: one card per line with its glyph, name, the character's level (whole number, rounded down), the exact value as visible text, a bar to the cap and a note ("no XP in this line yet" for a line without a row). Two cards per row at 375 px.
* Every house row: house, then the line's glyph and name, the level, the region and the tier meter with its word as on the present board. The meter stays on a phone (it wraps to its own line).
* Sort: the headings House, Specialization and Level, in mock-up A's pattern (click for ascending, again for descending). Group by: None, Region, Specialization, one control. A Reset view button returns to House, ascending, no grouping, which is the board's present order. The existing Show, House and Jump controls are drawn inactive above, so the 375 px fit of the whole control area is visible.
* A line above the list names both parts of the order (for example "Groups: lowest level first. Houses inside: A to Z").
* Each of the 18 views (3 groupings times 6 sorts) is its own section in the HTML, so the reading and focus order match the screen in every view. A group is a `section` with a real `h3` and a `ul`. The page cannot keep focus on the control used, set `aria-sort` or announce the change; those need script (D15). A banner at the top says so.
* Every panel carries a visible "Example data" tag; the badge text is "save (example)"; a legend under the heading explains the marks without hovering. Text is 12 px or more, targets are 44 px on a phone and on any touch screen, and the bars and meters keep a visible fill in forced-colours mode.
* The levels, tiers and states are made up; the house to line link is the tracker's table.

## 8. Open decisions, with a recommended default

Reviewed on #745. Where the review's suggestion was taken, the entry says so; the one place where this page keeps another default is D18.

| # | Decision | Recommended default |
|---|---|---|
| D1 | Icon source | Original simple glyphs, drawn for the editor, never the game's art; the five final glyphs go through the design review, rendered in greyscale at 16, 22 and 34 px first. Never show a glyph or a colour without the line's name (the red, brown and green lines are close for red-green colour blindness). The mock-up gives Exploration a four-point star and Crafting a gear, so the two no longer share a ring. |
| D2 | Where the house to line table lives | A fixed table in `internal/ops` next to the location table, keyed by the save's house name, with the tracker as provenance in a comment, and a test that all 25 houses are covered, five per line. Replace it if the game's own data turns out to hold the link. |
| D3 | The track numbers | Preset the measured numbers (1 Crafting, 2 Gathering, 3 Exploration, 4 Combat, 5 Sabotage) for every save, keep Assign as an override, and show one line under the header cards while Assign has not been used ("Line numbers assumed (preset), check on the Specialization tab"). The preset rests on few samples (1, 2 and 5 from a move to the cap, 3 and 4 from one mission each), so a wrong one would show another line's level with no sign of error. Review point taken. |
| D4 | How the level reads | Whole number rounded down ("Lv 64" for 64.37, not rounded up), the exact value as visible text in the header cards (a tooltip on a row is only a bonus), "Lv 100 max" at the cap, "Lv 0, no XP yet" for a line with no row, and "Lv -, not read" when the numbers were never set or the character has no rows to read (see D17). |
| D5 | What the Specialization heading sorts | The lines in the Specialization tab's order (Combat, Crafting, Exploration, Gathering, Sabotage), houses of a line A to Z; first click ascending. Taken into D16. |
| D6 | What the Level heading sorts | The character's level in the house's line. The five houses of a line share one level, so this really orders the lines by level; inside a line the order stays line, then house. First click lowest first (the lines to level next). |
| D7 | Default view | Unchanged: House, ascending, no grouping, alphabetical. |
| D8 | Grouping and what the sort does to it | One Group by control: None, Region, Specialization. With Specialization the sort orders the groups and the houses inside are A to Z; with Region the four regions keep one fixed order and the sort orders the houses inside each; with House the direction only reverses the houses. The region order is Arrakeen, Harko Village, Hagga Basin, Deep Desert, the same as mock-up A's region view (hubs first, then the open maps), kept so the two pages agree; alphabetical would put Deep Desert and Hagga Basin first. The line above the list states both parts. No nested grouping now. |
| D9 | Group header | A real `h3` with glyph, line, the character's level and the number of houses. No bar. |
| D10 | A filter by line (show only Combat houses) | Not in this mock-up; park it as a follow-up if sort and grouping are not enough. |
| D11 | House names | The save's spelling (Mikkarol, Spinette) everywhere, including the mock-ups' sample data. |
| D12 | Phone | Glyph, line name and level stay visible; the row is four short lines (house, line and level, region, tier meter and state). The tier meter is not hidden, so whatever sets a tier in the editor keeps its 44 px target. Headings become a "Sort by" row as in mock-up A. The existing Show, House and Jump controls are drawn above the new ones in the mock-up. Optional follow-up: fold the controls into one closed "View" disclosure whose summary names the current view. |
| D13 | Preview after a Fill or a tier click | Later, with the write design of #609/#656: the pending text names the line and the XP (for example "Exploration +3,000 XP, level 64 to 68"). It also says that the write moves the other four houses of that line in Sort by Level. Not part of this issue's page. |
| D14 | Respect the daily and weekly mission limits when writing | Ignore them, as the other Landsraad writes do, and say so in the write design note; revisit only if the limits are confirmed. |
| D15 | How the editor re-orders | Sort and group re-render the DOM in the displayed order with script: one `section` per group with a real `h3` and a `ul` of rows, never CSS `order` (keyboard focus and screen readers follow the HTML order, WCAG 1.3.2 and 2.4.3, and the rows carry controls). After a change, focus stays on the control that was used, `aria-sort` is set on the heading, and the page updates the text of the order line itself (an `aria-live` region announces text changes, not CSS changes). This mock-up uses 18 prebuilt views only because it has no script. |
| D16 | One sort model | Specialization and Level are two more sortable headings in mock-up A's existing pattern (click for ascending, again for descending, `aria-sort`), plus one Group by control (None, Region, Specialization). No separate Sort by and Order radio groups; that is fewer controls and A's headings are kept. This replaces the radio model of the first version of this mock-up; mock-up A needs no change except the two new headings and the Group control replacing its region switch. |
| D17 | A house or line with no value | Three cases. (1) A line with no row in the save: "Lv 0, no XP yet". (2) The line numbers never set, or the character has no specialization rows to read: "Lv -, not read", never 0. (3) A house the table does not know (a new house after a patch): shown with "Line unknown" and sorted last; the table test fails loudly, because D2 says exactly 25 houses, five per line. The level is read for the one character the editor works with: the first row of `player_state`, whose controller id is `specialization_tracks.player_id` (the same character the Specialization tab reads). |
| D18 | Is the view remembered | Yes, per browser: the last sort and Group by are kept in the browser's own storage and nothing else (no other board state, never in the save). The board still opens at the default view when nothing is stored or storage is blocked, and Reset view clears the stored choice. The review suggested not remembering it; the default stays "remembered" because a player who levels one line wants the same grouping on the next visit, and the default view is one click away. |

## 9. Sources

* #646 and #609 (measurements, settings multipliers, track numbers, tracker data), `docs/save-database.md`, `internal/ops/specialization.go`, `internal/ops/ops.go` (`player()`), `internal/settings/settings.go`, `internal/config/defaults.go`, `internal/ops/landsraad_locations.go`, `docs/art-notice.md`.
* A read-only check copy of a single-player save, for the table shapes, the 25 house names, the keystone names and one level value.
* The community Landsraad tracker, `https://htwsguild.com/Molbork/lr_tracker.html`, a public page by its author, read from its page source on 2026-11-14 for the house to line link and the regions. The tracker's author is credited here and in the table's comment when it is stored.
* The community specializations page, `https://dune.gaming.tools/specializations`, for the five names and a look at the icons by eye only; it refuses automated reads, so nothing was taken from it.
* The community specialization calculator, `https://htwsguild.com/Molbork/spec_time_calc.html`, and a community specialization guide on method.gg, quoted on #609 for perks and the unverified daily and weekly limits; neither is used for any value here.

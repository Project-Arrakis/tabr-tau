# Landsraad house specializations: research and mock-up (refs #646, #609)

Research and a static mock-up only. No editor code changes with this page. Everything below comes from the repository's own code and documents, the measurements recorded on #609 and #646, and one read-only look at a check copy of a single-player save. Where something is not known, it says so.

The mock-up is `d-specialization.html` in this folder (see "The mock-up" at the end).

## 1. The question

Every Landsraad house raises one of five skill lines when its missions are completed: Combat, Crafting, Exploration, Gathering, Sabotage. #646 asks for a small icon and the character's level in that line on every house row, a summary of the five lines at the top of the tab, and a sort and a grouping by line, so a player can pick houses by the line they want to level.

## 2. What the save holds

| What | Where | Notes |
|---|---|---|
| The character's XP and level in a line | `specialization_tracks` (`player_id`, `track_type`, `xp_amount`, `level`) | Primary key (`player_id`, `track_type`). `track_type` is a bare integer, `xp_amount` an integer, `level` a REAL. Documented in `docs/save-database.md`; read and written today by `internal/ops/specialization.go`. |
| Which number is which line | measured, not stored | The save does not name the lines. Measured on #609: 1 Crafting, 2 Gathering, 3 Exploration, 4 Combat, 5 Sabotage. The editor today learns the numbers per save (`TrackNumbers` in `internal/settings/settings.go`, the Assign button on the Specialization tab). |
| A row for a line | created by the first XP in it | A line the character never earned XP in has no row. Seen again on a check copy: one row only (track 1, 36,100 XP, level 87.05). A write must insert, not only update (the third #609 sample created track 4 that way). |
| The keystones of a line | `specialization_keystones_map` (`name`), `purchased_specialization_keystones` | 205 keystones, 41 per line, named by line (`Combat_CombatKeystone_SkillPoint1`, and so on). Not needed for the Landsraad tab. |
| The houses | `landsraad_tasks.house_name` | One row per house per term, `UNIQUE (term_id, house_name)`. The names are `DA_House<Name>`; the check copy holds exactly 25 distinct names in all six terms. The editor already strips the prefix (`lbNameOf` in `internal/web/static/app.js`). |
| Which line a house raises | **nowhere in the save** | `landsraad_tasks` has no line column, and no other table of the save links a house to a line. The link has to be a fixed table in the editor (section 3). |
| A house's place | **nowhere in the save** | Already known (`internal/ops/landsraad_locations.go`, #599). The tracker's regions in section 3 are only a cross-check for that table. |

Level arithmetic: the editor's XP table (`specCumulativeXP` in `internal/ops/specialization.go`, level 100 at 44,182 XP) reproduces every stored level that was measured: 18,000 XP is level 57.37, 21,000 is 62.63, 24,000 is 67.64, 3,000 is 18.92 (all #609) and 36,100 is 87.05 (the check copy). So the editor can show the level it reads, or recompute it from the XP, and they agree.

## 3. Which house raises which line

Source: the public community Landsraad tracker, read from its page source and recorded on #609 (comment of 2026-11-14), to be stored in the repository with this provenance. It is the only source in the repository's reach; this step did not read the game's own data tables for it (#646 asks for that as the preferred source, with the community pages as a cross-check by eye). Two samples agree with it: House Argosaz raised track 3 (Exploration) and House Mikkarol raised track 4 (Combat).

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

Recommendation: draw five original, simple glyphs (24 by 24, one stroke weight, `currentColor`), each with a different silhouette and always next to the line's name, so meaning never rests on the picture or on colour alone. The mock-up draws such glyphs as inline SVG, as a first proposal for the design review; they borrow only the plain ideas (blades, gear and crate, compass, crystal, wrench), not any drawing. Nothing is ripped from the game's files or from a website.

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

`d-specialization.html`: a static page, inline CSS and SVG, no script, no external request. Light and dark follow the system through the editor's colour tokens, with a mock-up control to force either. It works at 375 px (the control bar and the rows wrap, targets are 44 px).

* Header summary: one card per line with its glyph, name, the character's level, a bar to the cap and a note ("no XP yet" for a line without a row).
* Every house row: the line's glyph and name and the level next to the house, then the region and the tier meter as on the present board. A tooltip on the level gives the exact value.
* Controls: Sort by (House, Specialization, Level), Order (Ascending, Descending), Group by (None, Region, Specialization). House, Ascending, None is the board's present order and the starting view.
* Group headers show the line's glyph, name, level and house count.
* The levels, tiers and states are made up and marked as example data; the house to line link is the tracker's table.

## 8. Open decisions, with a recommended default

| # | Decision | Recommended default |
|---|---|---|
| D1 | Icon source | Original simple glyphs, drawn for the editor, never the game's art; the five final glyphs go through the design review. |
| D2 | Where the house to line table lives | A fixed table in `internal/ops` next to the location table, keyed by the save's house name, with the tracker as provenance in a comment, and a test that all 25 houses are covered, five per line. Replace it if the game's own data turns out to hold the link. |
| D3 | The track numbers | Preset the measured numbers (1 Crafting, 2 Gathering, 3 Exploration, 4 Combat, 5 Sabotage) as the default for every save, and keep Assign as an override for a save that disagrees. |
| D4 | How the level reads | Whole number, "Lv 64", with the exact value in the tooltip; "Lv 100 max" at the cap; "Lv 0" and "no XP yet" for a line without a row. |
| D5 | What "Sort by Specialization" does | Orders the lines as the Specialization tab does (Combat, Crafting, Exploration, Gathering, Sabotage), houses of a line A to Z. Ascending first. |
| D6 | What "Sort by Level" does | The character's level in the house's line, lowest first on the first click (the lines to level next), ties by line then house. Descending on the second. |
| D7 | Default view | Unchanged: House, Ascending, no grouping, alphabetical. |
| D8 | Grouping | One choice at a time, None, Region or Specialization; the present "Group by region" switch becomes the Region choice. No nested grouping now. Sort applies inside each group. |
| D9 | Group header | Glyph, line, the character's level, number of houses. No progress bar. |
| D10 | A filter by line (show only Combat houses) | Not in this mock-up; park it as a follow-up if sort and grouping are not enough. |
| D11 | House names | The save's spelling (Mikkarol, Spinette) everywhere, including the mock-ups' sample data. |
| D12 | Phone | Glyph, line name and level stay visible on 375 px (second line of the row); the tier meter hides, its word stays. |
| D13 | Preview of the level after a Fill or a tier click | Later, with the write design of #609/#656: the pending text names the line and the XP (for example "Exploration +3,000 XP, level 64 to 68"). Not part of this issue's page. |
| D14 | Respect the daily and weekly mission limits when writing | Ignore them, as the other Landsraad writes do, and say so in the write design note; revisit only if the limits are confirmed. |

## 9. Sources

* #646 and #609 (measurements, settings multipliers, track numbers, tracker data of 2026-11-14), `docs/save-database.md`, `internal/ops/specialization.go`, `internal/settings/settings.go`, `internal/config/defaults.go`, `internal/ops/landsraad_locations.go`, `docs/art-notice.md`.
* A read-only check copy of a single-player save, for the table shapes, the 25 house names, the keystone names and one level value.
* The community Landsraad tracker (house to line and regions) and the community specializations page (icon descriptions, cross-check by eye only).

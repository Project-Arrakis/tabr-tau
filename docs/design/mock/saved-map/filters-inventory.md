# Saved map filters: inventory, gaps and decisions (refs #679)

The operator's first priority for the Saved map is its filters: it should filter markers the way the reference map does, in place of today's single row of group chips, and it gets everything the reference map's filters section has, not a subset. This page is the first step the issue asks for: an inventory of that filters section (every group, control, count and default, named by what is on the screen), a mapping onto what the save supplies, where each of today's chips goes, the gaps in both directions, and the decisions the operator has to make. The mock is `filters.html` in this folder. Nothing here changes the editor's code.

The Saved map shows the save as of its last write, not the running game. That one fact decides most of the gaps below.

## How to read the table

- **Reference default**: what the reference map's filters do when nothing was changed on a map that shows its places (Hagga Basin).
- **Save supplies**: the save table or the editor's existing reading that would feed the control. "Markers" are the `markers` rows of the map (type, place) with the character's `player_markers` discovery level; "fields" are `resourcefield_state`; "yours" is the character, vehicles, bases and storage the Saved map already reads. See `docs/save-database.md`.
- **Status**: **Available** (the save, or the page alone, has what the control needs; "page only" marks controls that need no save data at all), **Partial** (some of it), **Gap** (nothing the editor reads, or not applicable to a saved view).
- **Recommended**: what the Saved map should do with the control. "In the closed fold" means the control is listed, switched off, with its reason, in one closed fold "Not available from a save" at the end of the panel (decision D1).

## Inventory

The filters section is a heading and nine parts below it, numbered 1 to 10 here in the order the reference map has them. The Saved map's panel uses another order (decision D11); the mock marks each part with its number.

### 1. Heading

| # | Control (on-screen label) | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 1.1 | "Filters" with a count of what is off ("3 off") | Empty until something is switched off; a hover names what is off | Counted in the page from the switches | Available, page only (not counted) | "All on" or "N kinds off" as plain text; no live announcement on every switch |

### 2. Certainty chips

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 2.1 | **Live** chip | On | Nothing: not stored in a save | Gap | In the closed fold |
| 2.2 | **Saved** chip | On | Every marker, field and thing of yours on the Saved map | Available | A line, "Everything here is from your save", not a switch: in a saved view everything is Saved, and a switch would only let a person blank the map by accident (D12) |
| 2.3 | **Possible** chip | Off | Nothing: the save has no list of places where a thing could appear | Gap | In the closed fold |
| 2.4 | Line "Fixed places are always shown" | Shown | Text | Available (not counted) | Same |
| 2.5 | Notice "Layout not confirmed" | Shown when the layout is only assumed | The save carries the one layout that was played | Gap (not counted) | Leave out (not applicable) |

### 3. Find

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 3.1 | Search box "Find a known place..." | Empty; typing also narrows the map to the matches | Marker type names; vehicle, base and storage names | Available | Same, worded "Find a kind or place" until the result list is built |
| 3.2 | "Map shows matches only · Clear" | Shown while a search is typed | Page only | Available, page only | Same |
| 3.3 | Result list: name, distance, "(off)" for a kind that is switched off, "(gone)", "N of M are off", "+N more", "Nothing matches." | Nearest first | Name and the distance from the character (the save has the character's place); no "gone" state | Partial (not counted) | Same without "(gone)"; the number of matches is announced in one status line |

### 4. Show all / Hide all

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 4.1 | **Show all** | Turns on every kind, every layer, the three chips and the by-part boxes | Page only | Available, page only | Turns on every kind and everything under Yours; leaves the display options alone; an Undo line follows (D12) |
| 4.2 | **Hide all** | The reverse | Page only | Available, page only | Turns off every kind; leaves Yours, the display options and the map alone; an Undo line follows (D12) |

### 5. Live layers (switches with a count)

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 5.1 | **Creatures** group switch | On | None of its rows | Gap | In the closed fold |
| 5.2 | Sandworms | On | Nothing: not stored in a save | Gap | In the closed fold |
| 5.3 | People | On | Nothing: not stored in a save | Gap | In the closed fold |
| 5.4 | Infected | On | Nothing: not stored in a save | Gap | In the closed fold |
| 5.5 | **Vehicles** (one switch, with a count) | On | Your own vehicles (the Saved map's "Your vehicles"); stored ones are counted, not placed | Partial | Group "Your vehicles" with a row for each kind (D7), on |
| 5.6 | **Weather** group switch | On | The storm row only | Partial | In the closed fold (storms are a later step, D6) |
| 5.7 | Sandstorms | On | The current or last storm of the map: start and end place and times (`sandstorm_data`) and the next spawn time (`sandstorm_schedule`); no moving position | Partial | In the closed fold; a start-to-end line in a later slice (D6) |
| 5.8 | Animate storms | On | A drawing preference; needs a storm drawn | Gap | In the closed fold |
| 5.9 | Storm covers icons | On | Same | Gap | In the closed fold |
| 5.10 | **Treasure** group switch | Buried treasure off (on for the Deep Desert), Loot off | None of its rows | Gap | In the closed fold |
| 5.11 | Buried treasure (a saved count and a live count) | Off | The loot-container tables hold items and reset times by container id and no place column (`docs/save-database.md`); no place found in the save yet | Gap | In the closed fold |
| 5.12 | Loot | Off | Same | Gap | In the closed fold |

### 6. Overmap places (shown only on the overmap)

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 6.1 | Search box "Search the places..." | Hidden on other maps | The game's headings and one-line place descriptions are in the game's files, not the save | Gap | In the closed fold (D8) |
| 6.2 | **Show all places** | Same | Same | Gap | In the closed fold |
| 6.3 | **Hide all places** | Same | Same | Gap | In the closed fold |

### 7. Kinds of places (the tree)

One fold per group, each with a colour dot, a name, a count "shown / total" (just the total when all are shown), a live badge and a switch that is half on when some rows are off; under it one row per kind with a picture, a name, a count and a switch. The reference has 15 groups. The rows come from the data: one row per marker type the map has, one row per material under Resources (ore, pickups and wreckage together), one row per spice size, one row per house under Representatives (up to 19). All groups start closed in the Saved map: a closed group row is as tall as one of today's chips.

| # | Group | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 7.1 | **Points of interest** | On | Cave, Ecolab, Shipwreck and Sietch markers. Fallen ships come down at random and are not stored in a save | Partial | Show; no Fallen Shipwreck row |
| 7.2 | **Other places** (and the game's own headings on the overmap) | On | The headings are in the game's files | Gap | In the closed fold |
| 7.3 | **Spice** (rows Large, Medium, Small) | On; fields that are not blooming wait behind "Inactive fields" | The fields that are up (`resourcefield_state`). The save has no coordinates for them: the place is decoded from the field id and is exact for about 84% of fields (the Saved map already says so in its note "not checked against the game"). Flour sand is recognised by a yield left of exactly 60,000, and the size by the cut-offs 150,000 and 5,000: rules, not fields in the save | Partial | Show; count as "up now"; keep the note |
| 7.4 | **Resources** (a row for each material: ores, plants such as Primrose, Brittle bush and Saguaro, Scrap metal, Fuel cell, Flour sand) | On | Ore, pickup, plant and wreckage markers the map has | Partial | Show; the fixed-node and earlier-visit rows are the by-part boxes (part 8) |
| 7.5 | **Objectives** | On | Exploration points and survey points as markers | Available | Show |
| 7.6 | **Intel** | Off until asked for | The save lists the pickups by id and which are taken; the `markers` payload of outposts and labs names Intel pickups (`docs/save-database.md`), so a place may be derivable. To be confirmed by a probe | Partial (to be confirmed) | In the closed fold until confirmed |
| 7.7 | **Enemy bases** (and a row "Event sites (past)", off until ticked) | On | Camps, outposts and fortresses as markers; the event sites are a list in the game's files | Available | Show, with the two fortress kinds; no event-sites row |
| 7.8 | **Control points** | On | Control point markers | Available | Show |
| 7.9 | **Representatives** | On | House representative markers, one type for each house | Available | Show, one row for each house (up to 19) |
| 7.10 | **Trainers** | On | Trainer markers | Available | Show |
| 7.11 | **Trade & travel** | On | Trading post and taxi service markers (named from the place-names list); vendors where the save has a marker | Partial | Show; taxi services move here from Other |
| 7.12 | **Travel destinations** | On | Places of the overmap | Gap | In the closed fold (D8) |
| 7.13 | **Hazards** | On | Hazard markers | Available | Show |
| 7.14 | **Player bases** | On | Every base totem on the map. Today's query has no owner filter, so it is every base, not only yours | Available | Show as "Bases" with a row "Base" and a row "Home base marker"; the "Your base" switch of today's page is this group. An owner filter is a separate change |
| 7.15 | **Other** | On | Every marker type the groups above do not take | Available | Show |
| 7.16 | Row details: live badge on a row, the "landed, falling, waiting" line on the fallen-ship row, hidden-count hints | Shown | Not applicable to a saved view | Gap, row level (not counted) | Leave out |
| 7.17 | Folds, counts "shown / total", half-on group switch | Shown | Page only | Available, page only (not counted) | Same; a click on a half-on group turns all its kinds on (D13) |

### 8. "Saved and Possible, by part" (folded)

| # | Control (label) | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 8.1 | **Possible ships** | Off | The places where a fallen ship could come down are a list in the game's files | Gap | In the closed fold |
| 8.2 | **Inactive fields** | Off | Only fields that are up are in the save | Gap | In the closed fold |
| 8.3 | **Fixed resource nodes** | On for Hagga Basin, off for the Deep Desert | A list of the layout's nodes in the game's files; the save holds only which nodes were harvested and when they return, without a place | Gap | In the closed fold |
| 8.4 | **Pickups and nodes** | On | Earlier sightings are not kept in a save | Gap | In the closed fold |
| 8.5 | **Resource markers** | Off | Not kept in a save as places; the save's markers are always drawn | Gap | In the closed fold |
| 8.6 | **Treasure places** | Off | See 5.11 | Gap | In the closed fold |
| 8.7 | **Possible event sites** | Off | The game's files | Gap | In the closed fold |

### 9. "Map display"

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 9.1 | **Places not discovered yet** | On | `player_markers` discovery level; today's "Hide undiscovered markers" button | Available | Stays in Map display, and a shortcut stays beside Zoom and Fit (D9); on; undiscovered are faded |
| 9.2 | **Show places beyond the picture** | Off | Positions outside the picture exist; the Saved map drops fields outside the map's bounds | Partial | Show; off |
| 9.3 | **Terrain picture** | On when the layout has one | One picture per map | Gap | In the closed fold |
| 9.4 | **Terrain relief** | On when the layout has it | The game's files | Gap | In the closed fold |
| 9.5 | **Resource density** with its two lists (Deep Desert layout, Resource): 3 controls | Off | The game's files | Gap | In the closed fold |
| 9.6 | **Highlight on hover** (ring the markers of a row, dim the rest) | Off | Page only | Available, page only | Show; off |
| 9.7 | **Pulse rings** (a ring on one field) | Off | Not stored in a save | Gap | In the closed fold |

### 10. "Saved filters" (folded)

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 10.1 | List "Saved filters" (Default view, the person's named filters, filters offered by another map) | Nothing picked | The browser's own storage, as the editor's other preferences | Available, page only | Same, kept per map |
| 10.2 | **Save as...** with a name field, **Save** and **Cancel** (4 controls) | Hidden until pressed | Browser storage | Available, page only | Same |
| 10.3 | **Delete** | Off until a filter is picked | Browser storage | Available, page only | Same, with a one-line "Deleted, Undo" status |
| 10.4 | **Default for this map** and **Clear default** (2 controls) | Off | Browser storage | Available, page only | Same; the switch says "Pick a saved filter first" while off |
| 10.5 | **Also use on...** (a list of the other maps) | Hidden until a filter is picked | Browser storage | Available, page only | One labelled row, shown once a filter is picked; two maps today |

## Counting

The nine parts below the heading have **47 fixed controls** (3 chips, 2 in Find, 2 Show all / Hide all, 12 layer switches, 3 overmap, 7 by-part boxes, 9 in Map display, 9 in Saved filters) and **15 kind groups**: **62 groups and controls** in all. The rows of the tree under the 15 groups come from the data and are not counted. Resource density counts as 3 controls (its switch and its two lists).

| Status | Count | Which |
|---|---|---|
| Available | 24 | 11 from the save: the Saved chip, the find box, Places not discovered yet, and the groups Objectives, Enemy bases, Control points, Representatives, Trainers, Hazards, Player bases and Other. 13 page only: Clear, Show all, Hide all, Highlight on hover and the 9 Saved filters controls |
| Partial | 9 | Vehicles, Weather, Sandstorms, Show places beyond the picture; the groups Points of interest, Spice, Resources, Trade & travel and Intel |
| Gap | 29 | Live and Possible chips; 9 live-layer switches (Creatures, Sandworms, People, Infected, Animate storms, Storm covers icons, Treasure, Buried treasure, Loot); 3 overmap controls; 7 by-part boxes; 6 in Map display (Terrain picture, Terrain relief, Resource density with its 2 lists, Pulse rings); the groups Other places and Travel destinations |

**32 controls are switched off in the mock**: the 29 gaps and 3 partial ones that wait (Weather and Sandstorms for the storm line, Intel for the probe). They are the 32 in the closed fold "Not available from a save", which groups them under five reasons.

Rows of the tables left out of the counts: 1.1, 2.4, 2.5, 3.3, 7.16 and 7.17, plus the four row-level gaps inside available groups ("Event sites (past)", the fallen-ship row's detail, the live badge on a row, and "(gone)" in the find results).

## Where today's 14 chips go

The Saved map today has one chip for each of these groups (`livemap.js`). Nothing a person has today is dropped; the closed group rows carry the same words where they can.

| Today's chip | What it holds today | New group |
|---|---|---|
| Character | The character | Yours, row "Character" |
| Your vehicles | The vehicles you own | Your vehicles, a row for each kind (sandbike, buggy, ornithopter, sandcrawler, tread wheel) |
| Your base | Every base totem on the map | Bases, row "Base" (every base on the map, not only yours) |
| Storage containers | The named storage containers | Yours, row "Storage containers" (one row for each kind, later) |
| Spice fields | Spice fields | Spice (Large, Medium, Small) |
| Flour sand | Flour sand fields | Resources, row "Flour sand" |
| Ore and minerals | Ore, pickup and rock markers | Resources, a row for each material |
| Salvage | Scrap metal and fuel cell markers, and shipwrecks | Scrap metal and Fuel cell go to Resources; Shipwreck goes to Points of interest |
| Plants | Primrose field, Brittle bush, Saguaro seed | Resources, rows "Primrose", "Brittle bush", "Saguaro" (the reference map lists plants among its Resources) |
| Enemy camps | Enemy camps, outposts, labour outposts | Enemy bases; the Atreides and Harkonnen fortresses, which sit under Places today, join them |
| Places | Caves, Ecolabs, Sietches, trading posts, home base markers, fortresses, control points, exploration and survey points | Cave, Ecolab and Sietch: Points of interest. Trading post: Trade & travel. Home base marker: Bases. Fortresses: Enemy bases. Control point: Control points. Exploration point and Survey point: Objectives |
| Trainers and House representatives | One chip for both | Two groups: Trainers, and Representatives (a row for each house) |
| Hazards | Hazard markers | Hazards |
| Other | Every marker type no other chip takes | Other; taxi services, which sit in Other today, move to Trade & travel |

Slice 1 carries a test that every marker type in the test saves lands in exactly one group, so no type is left over or counted twice, and the counts of the groups add up to the markers of the map.

## Gaps

### A. Reference filters with no save data behind them (29 controls, plus 3 partial ones that wait, and 4 row-level)

1. Live chip and Possible chip (2.1, 2.3).
2. Sandworms, People, Infected and their group; the two storm drawing options; Buried treasure, Loot and their group (5.1 to 5.4, 5.8 to 5.12). Weather and Sandstorms wait for the storm line (D6).
3. All seven by-part boxes (8.1 to 8.7): they separate what was seen from what is only possible, and the save has one list.
4. The overmap places search and its two buttons (6.1 to 6.3), Other places and Travel destinations (7.2, 7.12): the overmap is not on the Saved map and its headings are in the game's files.
5. Intel (7.6) waits for a probe of the outpost marker payload.
6. Terrain picture, Terrain relief, Resource density with its two lists, Pulse rings (9.3 to 9.5, 9.7).
7. Row details (7.16) and "(gone)" in the results (3.3).

### B. Save data with no filter in the reference (9)

Items 1 to 5 are already on today's Saved map; they are "no filter in the reference", not new data. Items 6 to 9 are not shown today.

1. **Storage containers**: named containers the Saved map already draws (a chip today), in kinds. Recommended: rows by kind.
2. **Your vehicles by kind** (a chip today; the reference has one Vehicles switch). Recommended: one row per kind.
3. **The character** (a chip today; always drawn in the reference, with no switch). Recommended: a row under Yours.
4. **Stored vehicles**: counted, not placed (a line on the page today). Recommended: keep the line "N of your vehicles are stored".
5. **Discovered regions** (Hagga Basin region names, `map_areas`; the Regions card today). Recommended: later, a filter "Discovered regions only".
6. **Yield left in a spice field** (`resourcefield_state`). Recommended: later, a count in the row or bands.
7. **Harvest return times** (`resource_nodes`, no place for a node). Recommended: later.
8. **The storm schedule** (`sandstorm_schedule`). Recommended: later, with the storm line (D6).
9. **Marker area and long-range flag** (`markers`): the meaning is not established. Recommended: leave until confirmed.

## Decisions for the operator

Each has a recommended default; the mock follows the recommendations. D1 to D10 were revised after a review of the first version; D11 to D14 are new.

| # | Decision | Options | Recommended default |
|---|---|---|---|
| D1 | The 32 controls with no usable data | (a) inline, disabled, between the active groups; (b) leave out; (c) one closed fold at the end, with the reason for each, grouped under five reasons | **(c)**: the scope is "everything the reference has" and the gap list stays on the screen, but the active controls are not buried under disabled ones |
| D2 | Groups | (a) the reference's 15 kind groups plus "Yours" and "Your vehicles", all closed at the start; (b) keep today's 14 chips | **(a)**, with the table above and the one-group test. Player bases is every base on the map, worded "Bases"; an owner filter is a separate change |
| D3 | What is on when the map opens | (a) everything the save has, as today; (b) the reference's Deep Desert starting view | **(a)** for both maps, plus a "Starting view" entry in Saved filters for the Deep Desert; the last state of each map is remembered without a name (browser, per map), so a reload does not lose it |
| D4 | Where filters are kept | (a) the browser, per map, as the editor's other preferences; (b) in the save | **(a)**: the Saved map is read-only; nothing here may write to a save |
| D5 | Counts | (a) "shown / total" per group and row, with the discovered count in the group's hint from slice 1; (b) total only | **(a)** |
| D6 | Sandstorms from the save (a start-to-end line and the next spawn time) | (a) in this work; (b) a follow-up after a probe against a real save | **(b)**: the rows stay in the closed fold |
| D7 | The save-only groups | (a) all of part B; (b) what today's page has (character, your vehicles by kind, bases, storage) in slice 1, the rest later | **(b)**: nothing a person has today is dropped when the chips go |
| D8 | The overmap and the places the game's files hold (headings, travel destinations, terrain, density) | (a) bundle the game's lists into the editor; (b) leave them out while the Saved map has two maps | **(b)**: the rows stay in the closed fold |
| D9 | Where today's controls go | Today's "Hide/Show undiscovered markers" button and the group chips | The chips are replaced by the groups (D14). "Places not discovered yet" is in Map display and a "Hide/Show undiscovered" shortcut stays beside Zoom and Fit. Map buttons, Zoom and Fit stay where they are |
| D10 | Order of the work after the mock | (a) one change; (b) slices | **(b)**: 1) a grouping table on the server with the one-group test, then the tree with Yours, Your vehicles, Bases, counts, Show all / Hide all and the closed "not available" fold; 2) Map display; 3) Find with the nearest-first result list; 4) Saved filters and the remembered last state; 5) the remaining save-only filters (regions, spice bands, storage kinds). Each with a test and a screen check. The disabled rows are in slice 1, not late |
| D11 | Panel order | (a) the reference's order; (b) active parts first (Find, Yours, Kinds of places, Map display, Saved filters), the closed fold last | **(b)**: the parts people use come first; the part numbers in the inventory keep the reference's order for checking |
| D12 | Hide all and the Saved chip | (a) Hide all turns off everything including Yours and the Saved chip; (b) Hide all turns off the kinds only and leaves Yours, the display options and the map alone; either way an Undo line follows | **(b)** with the Undo line. The Saved chip is a line of text, not a switch. Show all turns on every kind and everything under Yours and leaves the display options alone |
| D13 | A click on a group switch that is half on | (a) turns all its kinds on; (b) turns all off | **(a)**, the usual rule for a mixed checkbox; the panel says so in one line |
| D14 | How a person who knows today's chips finds each one again | (a) the table above in the document only; (b) the table, plus group rows that keep today's words (Your vehicles, Storage containers, Spice, Hazards) and Find | **(b)**; a one-line "Where did my chips go?" note on the panel is optional |

### Where this version differs from the review

- The review put Plants (Primrose field, Brittle bush) in Other because the 15 groups have none. The reference map lists plants among its Resources rows, so they go there, as rows.
- The review suggested the reference's order or active-first for D11; Find stays first because it is one small box that applies to everything below it.
- Everything else in the review's changes to D1, D2, D3, D5, D7, D9 and D10 is taken, and the four new decisions follow its recommendations.

## What the mock shows

`filters.html` is a static page with made-up counts (marked "Example data"). It has the panel in the order of D11: a header with the state ("All on", "N kinds off"), Show all and Hide all and an Undo line, Find, Yours and Your vehicles, the kinds of places (all groups closed; the Representatives group has 19 rows), Map display, Saved filters, and last the closed fold "Not available from a save (32)". It works at phone width (375 px, where the map picture stays at the top of the screen) and on a desktop, in light and dark and in forced colours. Each row carries a small mark for its data source (From the save, Partly, Page only, Save only); a switch at the top hides the marks and the part codes. It reads no save and makes no request. The map picture is a placeholder: the port keeps the real map's keyboard handling.

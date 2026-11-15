# Saved map filters: inventory, gaps and decisions (refs #679)

The operator's first priority for the Saved map is its filters: it should filter markers the way the reference map does, in place of today's single row of group chips, and it gets everything the reference map's filters section has, not a subset. This page is the first step the issue asks for: an inventory of that filters section (every group, control, count and default, named by what is on the screen), a mapping onto what the save supplies, the gaps in both directions, and the decisions the operator has to make. The mock is `filters.html` in this folder. Nothing here changes the editor's code.

The Saved map shows the save as of its last write, not the running game. That one fact decides most of the gaps below.

## How to read the table

- **Reference default**: what the reference map's filters do when nothing was changed on a map that shows its places (Hagga Basin).
- **Save supplies**: the save table or the editor's existing reading that would feed the control. "Markers" are the `markers` rows of the map (type, place) with the character's `player_markers` discovery level; "fields" are `resourcefield_state`; "yours" is the character, vehicles, bases and storage the Saved map already reads. See `docs/save-database.md`.
- **Status**: **Available** (the save has what the control needs), **Partial** (some of it), **Gap** (nothing the editor reads, or not applicable to a saved view).
- **Recommended**: what the Saved map should do with the control. "Show, disabled" means the control is on the panel, switched off, with a one-line reason, so the panel matches the reference and the gap is visible (decision D1).

## Inventory

The filters section is a heading and nine parts below it, numbered 1 to 10 here in the order they are on the screen.

### 1. Heading

| # | Control (on-screen label) | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 1.1 | "Filters" with a count of what is off ("3 off") | Empty until something is switched off; a hover names what is off | Counted in the page from the switches | Available | Same |

### 2. Certainty chips

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 2.1 | **Live** chip ("loaded in the game now") | On | Nothing: the save is not the running game | Gap | Show, disabled, with the reason |
| 2.2 | **Saved** chip ("known from your save or an earlier visit; may have changed since") | On | Every marker, field and thing of yours on the Saved map | Available | Show, on |
| 2.3 | **Possible** chip ("a place where one could be") | Off | Nothing: the save has no list of places where a thing could appear | Gap | Show, disabled, with the reason |
| 2.4 | Line "Fixed places are always shown" | Shown | Text | Available | Same |
| 2.5 | Notice "Layout not confirmed" | Shown when the layout is only assumed | The save carries the one layout that was played | Gap | Leave out (not applicable) |

### 3. Find

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 3.1 | Search box "Find a known place..." | Empty; typing also narrows the map to the matches | Marker type names; vehicle, base and storage names | Available | Same |
| 3.2 | "Map shows matches only · Clear" | Shown while a search is typed | Page only | Available | Same |
| 3.3 | Result list: name, distance, "(off)" for a kind that is switched off, "(gone)", "N of M are off", "+N more", "Nothing matches." | Nearest first | Name and the distance from the character (the save has the character's place); no "gone" state | Partial | Same without "(gone)" |

### 4. Show all / Hide all

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 4.1 | **Show all** | Turns on every kind, every live layer, the three chips and the by-part boxes | Page only | Available | Same, limited to what the save supplies |
| 4.2 | **Hide all** | The reverse | Page only | Available | Same |

### 5. Live layers (switches with a live count)

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 5.1 | **Creatures** group switch | On | None of its rows | Gap | Show, disabled |
| 5.2 | Sandworms | On | Nothing: worms are not in the save | Gap | Show, disabled |
| 5.3 | People | On | Nothing: other characters in the world are not in the save | Gap | Show, disabled |
| 5.4 | Infected | On | Nothing | Gap | Show, disabled |
| 5.5 | **Vehicles** (one switch, with a count) | On | Your own vehicles (the Saved map's "Your vehicles"); not every vehicle the game has loaded; stored ones are counted, not placed | Partial | Show as "Your vehicles", on |
| 5.6 | **Weather** group switch | On | The storm row only | Partial | Show |
| 5.7 | Sandstorms | On | The current or last storm of the map: start and end place and times (`sandstorm_data`) and the next spawn time (`sandstorm_schedule`); no moving position | Partial | Show disabled now; a start-to-end line in a later slice (D6) |
| 5.8 | Animate storms | On | A drawing preference; needs a storm drawn | Gap | Show, disabled |
| 5.9 | Storm covers icons | On | Same | Gap | Show, disabled |
| 5.10 | **Treasure** group switch | Buried treasure off (on for the Deep Desert), Loot off | None of its rows | Gap | Show, disabled |
| 5.11 | Buried treasure (a saved count and a live count) | Off | The loot-container tables hold items and reset times by container id, and no place in the tables the editor reads; to be confirmed by a probe | Gap | Show, disabled |
| 5.12 | Loot | Off | Same | Gap | Show, disabled |

### 6. Overmap places (shown only on the overmap)

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 6.1 | Search box "Search the places..." | Hidden on other maps | The game's headings and one-line place descriptions are in the game's files, not the save | Gap | Show, disabled (D8) |
| 6.2 | **Show all places** | Same | Same | Gap | Show, disabled |
| 6.3 | **Hide all places** | Same | Same | Gap | Show, disabled |

### 7. Kinds of places (the tree)

One fold per group, each with a colour dot, a name, a count "shown / total" (just the total when all are shown), a live badge and a switch that is half on when some rows are off; under it one row per kind with a picture, a name, a count and a switch. The reference has 15 groups. The rows come from the data: one row per marker type the map has, one row per material under Resources (ore, pickups and wreckage together), one row per spice size.

| # | Group | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 7.1 | **Points of interest** | On | Cave, Ecolab, Shipwreck and Sietch markers. Fallen ships come down at random and are not in the save | Partial | Show; no Fallen Shipwreck row |
| 7.2 | **Other places** (and the game's own headings on the overmap) | On | The headings are in the game's files | Gap | Show, disabled (D8) |
| 7.3 | **Spice** (rows Large, Medium, Small) | On; fields that are not blooming wait behind "Inactive fields" | The fields that are up, sized by the yield left (`resourcefield_state`); positions decoded from the field id | Partial | Show; count as "up now" |
| 7.4 | **Resources** (a row for each material, Flour sand among them) | On | Ore, pickup and wreckage markers the map has | Partial | Show; the fixed-node and earlier-visit rows are the by-part boxes (part 8) |
| 7.5 | **Objectives** | On | Exploration points and survey points as markers | Available | Show |
| 7.6 | **Intel** | Off until asked for | The save numbers the pickups and records which are taken, with no place of their own | Gap | Show, disabled |
| 7.7 | **Enemy bases** (and a row "Event sites (past)", off until ticked) | On | Camps, outposts and fortresses as markers; the event sites are a list in the game's files | Available | Show; no event-sites row |
| 7.8 | **Control points** | On | Control point markers | Available | Show |
| 7.9 | **Representatives** | On | House representative markers | Available | Show |
| 7.10 | **Trainers** | On | Trainer markers | Available | Show |
| 7.11 | **Trade & travel** | On | Trading post and taxi service markers (named from the place-names list); vendors where the save has a marker | Partial | Show |
| 7.12 | **Travel destinations** | On | Places of the overmap | Gap | Show, disabled (D8) |
| 7.13 | **Hazards** | On | Hazard markers | Available | Show |
| 7.14 | **Player bases** | On | The character's own base totems | Available | Show; the Saved map's "Your base" |
| 7.15 | **Other** | On | Every marker type the groups above do not take | Available | Show |
| 7.16 | Row details: live badge on a row, the "landed, falling, waiting" line on the fallen-ship row, hidden-count hints | Shown | Not applicable to a saved view | Gap (row level) | Leave out |
| 7.17 | Folds, counts "shown / total", half-on group switch | Shown | Page only | Available (not counted) | Same |

### 8. "Saved and Possible, by part" (folded)

| # | Control (label) | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 8.1 | **Possible ships** | Off | Nothing: the places where a fallen ship could come down are a list in the game's files | Gap | Show, disabled |
| 8.2 | **Inactive fields** | Off | Only fields that are up are in the save | Gap | Show, disabled |
| 8.3 | **Fixed resource nodes** | On for Hagga Basin, off for the Deep Desert | A list of the layout's nodes in the game's files; the save holds only which nodes were harvested and when they return, without a place | Gap | Show, disabled |
| 8.4 | **Pickups and nodes** | On | The reference's own record of earlier visits; the save has none | Gap | Show, disabled |
| 8.5 | **Resource markers** | Off | The reference's record for an assumed layout; the save's markers are always drawn | Gap | Show, disabled |
| 8.6 | **Treasure places** | Off | See 5.11 | Gap | Show, disabled |
| 8.7 | **Possible event sites** | Off | The game's files | Gap | Show, disabled |

### 9. "Map display"

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 9.1 | **Places not discovered yet** | On | `player_markers` discovery level; today's "Hide undiscovered markers" button | Available | Move into this part; on; undiscovered are faded |
| 9.2 | **Show places beyond the picture** | Off | Positions outside the picture exist; the Saved map drops fields outside the map's bounds | Partial | Show; off |
| 9.3 | **Terrain picture** | On when the layout has one | One picture per map | Gap | Show, disabled |
| 9.4 | **Terrain relief** | On when the layout has it | The game's files | Gap | Show, disabled |
| 9.5 | **Resource density** with its two lists (Deep Desert layout, Resource): 3 controls | Off | The game's files | Gap | Show, disabled |
| 9.6 | **Highlight on hover** (ring the markers of a row, dim the rest) | Off | Page only | Available | Show; off |
| 9.7 | **Pulse rings** (a ring on the field the giant worm goes for) | Off | Follows the running game | Gap | Show, disabled |

### 10. "Saved filters" (folded)

| # | Control | Reference default | Save supplies | Status | Recommended |
|---|---|---|---|---|---|
| 10.1 | List "Saved filters" (Default view, the person's named filters, filters offered by another map) | Nothing picked | The browser's own storage, as the editor's other preferences | Available | Same, kept per map |
| 10.2 | **Save as...** with a name field, **Save** and **Cancel** (4 controls) | Hidden until pressed | Browser storage | Available | Same |
| 10.3 | **Delete** | Off until a filter is picked | Browser storage | Available | Same |
| 10.4 | **Default for this map** and **Clear default** (2 controls) | Off | Browser storage | Available | Same: the map opens with that filter each time |
| 10.5 | **Also use on...** (a list of the other maps) | Hidden until a filter is picked | Browser storage | Available | Same; two maps today |


## Counting

The nine parts below the heading have **47 fixed controls** (3 chips, 2 in Find, 2 Show all / Hide all, 12 layer switches, 3 overmap, 7 by-part boxes, 9 in Map display, 9 in Saved filters) and **15 kind groups**: **62 groups and controls** in all. The rows of the tree under the 15 groups come from the data (one per marker type) and are not counted.

| Status | Count | Which |
|---|---|---|
| Available | 24 | Saved chip; find box and its Clear; Show all and Hide all; Places not discovered yet and Highlight on hover; the 9 Saved filters controls; the groups Objectives, Enemy bases, Control points, Representatives, Trainers, Hazards, Player bases and Other |
| Partial | 8 | Vehicles, Weather, Sandstorms, Show places beyond the picture; the groups Points of interest, Spice, Resources and Trade & travel |
| Gap | 30 | Live and Possible chips; 9 live-layer switches (Creatures, Sandworms, People, Infected, Animate storms, Storm covers icons, Treasure, Buried treasure, Loot); 3 overmap controls; 7 by-part boxes; 6 in Map display (Terrain picture, Terrain relief, Resource density and its 2 lists, Pulse rings); the groups Other places, Intel and Travel destinations |

Row-level gaps inside the available groups, not counted above: the "Event sites (past)" row, the fallen-ship row's detail, the live badge on a row, and "(gone)" in the find results (4).

## Gaps

### A. Reference filters with no save data behind them (30 controls, 4 row-level)

1. Live chip and Possible chip (2.1, 2.3).
2. Sandworms, People, Infected and their group; the two storm drawing options; Buried treasure, Loot and their group (5.1 to 5.4, 5.8 to 5.12): the running game only.
3. All seven by-part boxes (8.1 to 8.7): they separate what was seen from what is only possible, and the save has one list.
4. The overmap places search and its two buttons (6.1 to 6.3), Other places and Travel destinations (7.2, 7.12): the overmap is not on the Saved map and its headings are in the game's files.
5. Intel (7.6): the pickups are numbered in the save without a place.
6. Terrain picture, Terrain relief, Resource density with its two lists, Pulse rings (9.3 to 9.5, 9.7).
7. Row details (7.16) and "(gone)" in the results (3.3).

### B. Save data with no filter in the reference (8)

1. **Storage containers**: named containers the Saved map already draws, in kinds (the container groups). Recommended: a group of their own, one row per kind.
2. **Your vehicles by kind** (sandbike, buggy, ornithopter, sandcrawler, tread wheel): the reference has one Vehicles switch. Recommended: one row per kind.
3. **The character**: always drawn in the reference, with no switch. Recommended: a switch.
4. **Stored vehicles**: counted, not placed. Recommended: keep the line "N of your vehicles are stored".
5. **Discovered regions** (Hagga Basin region names, `map_areas`). Recommended: later, a filter "Discovered regions only".
6. **Yield left in a spice field** (`resourcefield_state`). Recommended: later, a count in the row or bands.
7. **Harvest return times** (`resource_nodes`, no place for a node) and the **storm schedule**. Recommended: later (D6).
8. **Marker area and long-range flag** (`markers`): the meaning is not established. Recommended: leave until confirmed.

## Decisions for the operator

Each has a recommended default; the mock follows the recommendations.

| # | Decision | Options | Recommended default |
|---|---|---|---|
| D1 | The 30 gap controls | (a) show them disabled with a one-line reason; (b) leave them out and list them here | **(a)**: the scope is "everything the reference has", and a disabled switch with its reason is the gap list on the screen. The mock marks each row's data source and can hide the marks |
| D2 | Groups | (a) the reference's 15 kind groups, with the Saved map's own things (character, your vehicles, your base, storage) as a first group "Yours"; (b) keep today's 14 groups | **(a)**: it is what "the way the reference map does" says; today's groups map onto the new ones (today's Places splits into Points of interest, Objectives, Control points and Trade & travel) |
| D3 | What is on when the map opens | (a) everything the save has, as today; (b) the reference's Deep Desert starting view (fixed places, spice, resources, your bases) | **(a)** for both maps, plus a "Starting view" entry in Saved filters for the Deep Desert: no change for a person who knows today's page |
| D4 | Where filters are kept | (a) the browser, per map, as the editor's other preferences; (b) in the save | **(a)**: the Saved map is read-only; nothing here may write to a save |
| D5 | Counts | (a) "shown / total" per group and row as the reference, with discovered in the row's hint; (b) total only | **(a)** |
| D6 | Sandstorms from the save (a start-to-end line and the next spawn time) | (a) in this work; (b) a follow-up after a probe against a real save | **(b)**: the row is shown disabled now |
| D7 | The save-only filters in part B | (a) all eight; (b) storage kinds, vehicle kinds and the character switch now, the rest later | **(b)**: that data is already in the response the page reads |
| D8 | The overmap and the places the game's files hold (headings, travel destinations, terrain, density) | (a) bundle the game's lists into the editor; (b) leave them out while the Saved map has two maps | **(b)**: the rows stay disabled with the reason |
| D9 | Where today's controls go | Today's "Hide/Show undiscovered markers" button and the group chips | The button becomes "Places not discovered yet" in Map display; the chips are replaced by the tree (D2). Zoom, Fit and the map buttons stay where they are |
| D10 | Order of the work after the mock | (a) one change; (b) slices | **(b)**: 1) the tree with the 15 groups, counts and Show all / Hide all; 2) Find; 3) Map display; 4) Saved filters; 5) the disabled gap rows and data-source hints; 6) the save-only groups of part B; each with a test and a screen check |

## What the mock shows

`filters.html` is a static page with made-up counts (marked "Example data"). It has the ten parts above in the same order, at phone width (375 px) and on a desktop, light and dark. Each row carries a small mark for its data source (From the save, Partly, No save data); a switch at the top hides the marks. Rows with no save data are disabled and say why. It reads no save and makes no request.

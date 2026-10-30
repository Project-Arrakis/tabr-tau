# Where the data in this folder comes from

| File | What it is | Source | Added |
|---|---|---|---|
| `items.json` | Template id, in-game name, category, source, required DLC (name and Steam id) and, for 99 resources, the group of each item | The item catalog of the Dune Awakening Docker console (RedBlink, MIT licence), `runtime/data/admin-items.json`; see `internal/notices/THIRD-PARTY-NOTICES.md` ("Item catalog") for the commits, the blobs and the licence text | #69, #122, #405, #495 |
| `stacks.json` | For 221 item template ids, the most of the item one inventory slot holds (`{"max": {"<template id>": n}}`) | The game's own item tables: the base item rows, `StackAndDurability.MaxStackSize`. Only items that stack are listed | #240, 2026-10-22 |
| `volumes.json` | For 4,210 item template ids, the volume of one unit (`{"size": {"<template id>": n}}`; 1,760 are above 0, the rest are listed with no volume) | The game's own item tables: the base item rows of the 20 `DT_BaseItems_*` tables, `StaticData.ItemSize` (no row has an `ItemSizeOverride`). 17 ids that two tables list with different sizes (`D_Turret`, `Thumper`, `Stilltent`, `RespawnBeacon`, `D_Totem`, the seismic probes and thumpers, three Halloween 2026 clothing variants) are left out, not guessed | #357, 2026-10-24 |
| `customization_unlocks.json` | For 392 cosmetic item template ids, the customization ids that using the item unlocks (625 entries, 614 distinct ids) | The game's own item table `DT_ItemTableCustomizations`, `CustomizationItemStats.UnlockableCustomizations` | #216, 2026-10-20 |

`stacks.json`, `volumes.json` and `customization_unlocks.json` contain only the game's identifiers and integers: no names, text, images or code. They are
read by `catalog.go` (`MaxStack`, `Volume`, `Unlocks`) and nothing else.

**How `items.json` was refreshed (#405, 2026-10-25).** Copied from the console fork at commit `82548d72` (`runtime/data/admin-items.json`, git blob `ecffc834`; the fork had merged the upstream release v1.4.52 by then). The one item the earlier copy (fork commit `ca3f4cd7`, blob `2746a605`) lacked is `MTX_IxianTotem_Patent`, "Ixian Icon", a building set. Its id is also in the game's own item tables (it has a row in `volumes.json`), so it is an
item of the game and not only of the console. The ids and names, categories, sources and DLC names of the other 2,558 items were already the same as in the fork's file. What the refresh added
are two fields: `group` (`component`, `refined_resource` or `raw_resource`; 59, 21 and 19 items) and `requiredSteamDlc` (the Steam id 4857580 of the eight Filmic Archive items). Nothing in
the editor reads the two fields yet except `/api/catalog/items`, which sends them with the rest.

**How `items.json` was refreshed again (#495, 2026-10-29).** Copied from the console's release v1.4.54 of 2026-10-28 (upstream repository, commit `a9d50345`, "Add Halloween event building unlocks"; the release tag v1.4.54 is the next commit, `a31086df`, and both hold the same file, `runtime/data/admin-items.json`, git blob `739ecaef`). The Project-Arrakis fork had not merged that release yet, so the source is the upstream commit. The file lists 2,573 items, 2,566 different ids. Compared with the file of #405 by id, the refresh adds 7 items and removes none: the Halloween 2026 building sets `MTX_HalloweenEvent_Crucifixion_Patent` (Decussate Cross), `MTX_HalloweenEvent_Fleshtube_Patent` (Flesh Sculpting Displays), `MTX_HalloweenEvent_LightPole_Patent` (Jury-Rigged Light Stand), `MTX_HalloweenEvent_RadiationCrate_Patent` (Sealed Radiation Containment Chest), `MTX_HalloweenEvent_RottingCorpse_Patent` (Cadaverous Decorations), `MTX_HalloweenEvent_Skeleton_Patent` (Osseous Decorations) and `MTX_HalloweenEvent_VacumBodies_Patent` (Vacuum-Sealed Body Display), all `buildings` / `BuildingSets`. All seven ids have a row in `volumes.json`, so they are items of the game and not only of the console. The other 2,559 items are unchanged (same ids, names, categories, sources, DLC fields and groups); the rules above (first entry wins for the seven duplicated ids, no `stackSize` or `volume` from the console) were applied again and give the same entries as before. The editor's file now holds 2,566 items.

**Four Halloween 2026 cosmetics added by hand (refs #495).** The console's release v1.4.54 lists the Halloween 2026 building sets but not four cosmetics that the game's own cosmetic table (`DT_ItemTableCustomizations`, the same table `customization_unlocks.json` is read from) carries and that `customization_unlocks.json` already unlocks: `Halloween2026_Radsuit_Variant` ("Derelict Recovery Suit"), `Halloween2026_PlagueDoctorMask_Hood` ("Contagion Cowl"), `Halloween2026_PlagueDoctorMask_NoHood` ("Contagion Visor") and `Swatch_Halloween2026` ("Hot Zone"), all `customizations` / `Customizations`, appended at the end of the file. The names are the customization display names from the game's data; the name the inventory item itself shows in its tooltip was not checked and may differ. None is an `MTX_` id and none needs a DLC, so none is entitlement controlled; they fall in "Other cosmetics" and are unlocked one at a time by id. The catalog now holds 2,570 items (2,566 from the console's file plus these four) and 346 customizations.

**The seven ids the console lists twice** (`RespawnBeacon`, `Stilltent`, `Thumper`, `UniqueThumper`, `UniqueThumper_02`, `UniqueThumper_03`, `Stilltent_Unique_01`; once as a placeable and once as a
weapon): the rule is that the first entry in the console's file wins, and there the placeable comes first, so the item is a placeable with the placeable's name. For five of them both entries
give the same name. For `UniqueThumper_02` and `UniqueThumper_03` the weapon entries are named "Wormsong Echo" and "Song of Shai-Hulud" while the placeables are "Clapper Mk2" and "Clapper Mk3". Which of the
two names the game shows for those two cannot be told from the save or from the game's item tables (neither carries names), so the placeable names stay and the question is open.
The `stackSize` and `volume` the console carries were not taken: the game's own tables (`stacks.json`, `volumes.json`) are used for those.

**How `volumes.json` was checked.** Of the 106 items for which the Dune Docker console's item file (`runtime/data/admin-items.json`) carries a measured unit volume, 103
have exactly the value the game's table gives (for example Aluminum Ingot 0.7, Copper Ore 0.2, Mouse Corpse 5, fuel canisters 2 to 5). The other three
(`T2MuaddibComponent`, `T4ShieldWallComponent`, `ExperimentalWindTurbineComponent`) are 1.0 in the console file, which its own change log describes as a placeholder, and 0.1 in
the game's table, like the other components. The editor uses the game's table. The volume of an inventory is the sum of unit volume times stack size over its rows (a row's own
`volume_override`, which the game reads per unit, replaces the table value); that the sum equals the number the game shows is checked in the editor's tests against the rule, not
yet against the game on a live character.

**Game build of `volumes.json`.** The installed game's Steam manifest (`appmanifest_1172710.acf`, read on 2026-10-24) names build `26083551`, last updated on 2026-10-22 13:59
UTC, with the target build equal to the installed one; the tables were read from that build, with a read-only reader of the game's container files, on 2026-10-24.

**Game build of the other two files.** The installed game's Steam manifest (`appmanifest_1172710.acf`, read on 2026-10-22) names build `20502675`, last updated by
Steam on 2025-10-23 09:07 UTC, with the target build equal to the installed one. Both files were added after that date (2026-10-20 and
2026-10-22), and the game had no later update, so they were read from build `20502675`. The tables are read from the installed game's content
files, read-only; nothing in the game is changed by reading them.

**Not recorded in the repository** (so not stated here): the tool and the exact files used to read the tables, and any licence or terms-of-service
position on using them. The notice for these two files is in `internal/notices/THIRD-PARTY-NOTICES.md` ("Stack sizes and cosmetic unlock ids").
The game and its data belong to Funcom and this project is unofficial. Whoever refreshes either file should add the game build, the tool and the
files used to this table in the same pull request.

## place-names.json (#500)

| File | What it is | Source | Added |
|---|---|---|---|
| `internal/ops/place-names.json` | For 14 `LocationKey`s (32 upper-case hexadecimal digits), the name of the trading post the key stands for (`{"name", "kind", "alias"?, "derived"?}`) | The game's own tables: `DT_StaticLocationData` (the rows tagged `Exploration.POI.TradePost`: the row name is the key, `m_PackageName` names the map object the post belongs to) and `ST_Localization_Dialogue` (the ornithopter pilot's destination choices: the names a player sees) | #500, 2026-10-30 |

It is in `internal/ops` and not in this folder because only `internal/ops/places.go` reads it, to name the `TradingPost` markers of a save in Admin > Movement > Go to a place. The save does not carry the name of a place: a marker has the type `TradingPost` and a payload that starts with a `LocationKey`.

**What each name is.**

| Places (keys) | Map object in `DT_StaticLocationData` | Name | How the name was obtained |
|---|---|---|---|
| Pinnacle Station (3) | `CB_Arrakis_Generic_M_05_PinnacleStation_MarkerName` | Pinnacle Station Tradepost | **Read** from the game's text: the pilot's choice `DIALOGUE/DIALOGUE_OPTION_D6TNQ` ("Pinnacle Station Tradepost.") |
| Crossroads (2) | `CB_Arrakis_Generic_S_02_TP_Crossroads_MarkerName` | Crossroads Tradepost | **Read**: `DIALOGUE/DIALOGUE_OPTION_LFRWB` |
| Griffin's Reach (1) | `CB_Arrakis_Generic_S_14_TP_GriffinsReach_Gameplay` | Griffin's Reach Tradepost | **Read**: `DIALOGUE/DIALOGUE_OPTION_X4D2Y` |
| Mirzabah's Shadow (5) | `CB_Trading_Post_MirzabahsShadow_Gameplay` | The Anvil Tradepost; the filter also matches "Mirzabah's Shadow" | **Read**: `DIALOGUE/DIALOGUE_OPTION_WI65H`. That the Mirzabah's Shadow level is the post the game calls The Anvil is read from the game's data too: the marker of the key `2445CEFC...` (object `CB_Trading_Post_MirzabahsShadow_Gameplay`) names itself `WORLD_MAP_LOCATION_Survival_VermiliusGap_Tradingpost1` in its payload, and that entry of `ST_Localization_NPCs_World` reads "The Anvil"; the pilot's list has "The Anvil Tradepost" and no other fourth post |
| Deep Desert (3) | `DeepDesert_1_Travel` | Deep Desert Tradepost | **Derived** (`"derived": true`): the game's text has no name for these three posts that was found, so the editor names them by the map they are on. No save checked holds a `TradingPost` marker on the Deep Desert, so the name has not been seen in practice |

The one row of the table named `TradingPost` (no map object) is the generic tag row, not a place, and is not in the file. Rows of the Mirzabah's Shadow object that do not carry the trading-post tag are not posts and are not in the file. Where several keys name one place (3 for Pinnacle Station, 5 for Mirzabah's Shadow, 2 for Crossroads, 3 for the Deep Desert), a save may hold more than one marker of it; each is listed, and the editor adds the first six digits of the key to the name only when two listed places of one heading would otherwise be called the same.

The names are the game's own short text for places (four place names), kept because a name is what the page has to show; the file holds nothing else of the game's text. The same three names (Griffin's Reach, Pinnacle Station, Crossroads) were also read from screenshots of the in-game pilot's list on 2026-10-30, which agree with the text. The pilot's text says "Crossroads Tradepost", while the map's own text for the same post is "The Crossroads Tradepost"; the editor uses the pilot's, as the operator's screenshot has it.

**How place-names.json is made.** `tools/placenamesgen` (`go run ./tools/placenamesgen -locations <DT_StaticLocationData dump> -dialogue <ST_Localization_Dialogue dump> -build <build> -read <date> -out internal/ops/place-names.json`; its comment states the rule and holds the table of map objects and text keys). It reads the two JSON dumps, in the shape any table dump has (a one-element array; `Rows` for the data table, `StringTable.KeysToEntries` for the text table). Every row that carries the tag `Exploration.POI.TradePost` and has a 32-digit row name is a trading-post key; the map object of its package name picks the name from a short table in the tool; a trading-post row whose object is not in that table stops the tool with an error, so a new post in a later game build is never named by guess. Making the dumps takes a reader of the game's container files, which this repository does not contain. **Game build:** not recorded (the game as installed on 2026-10-30, read-only; the Steam manifest was not read); the file records the day (`read`). The same tool, run on the dumps of a later build, gives the file for that build.

## vehicles.json (#401)

| File | What it is | Source | Added |
|---|---|---|---|
| `vehicles.json` | For the seven classes of vehicle that appear in saves (Sandbike, Buggy, Light, Medium and Carrier ornithopter, Sandcrawler, Container vehicle): the class's actor path, the shape of its rows in a save, its slots, the game's named templates (tier, and the module id of each slot), and one row per module the game's module table lists for the class (slot, kind, grade, whether it is a unique, the durability a new module has, health, critical health, fuel capacity, cargo slots and volume, whether it is a weapon-type module). 7 classes, 398 modules, 24 templates | The game's own vehicle tables: `CDT_BaseVehicleModules` (the module rows), `DT_VehicleSchematics` (the slots of each vehicle), `DT_VehicleTemplates` (the actor class and the names of the templates), `DT_VehicleWeaponModules` (which ids are weapon-type modules), and the module lists of the templates, which a generic reader returns as empty and which were read from the raw package of `DT_VehicleTemplates` | #401, 2026-10-27 |

`vehicles.json` contains only the game's identifiers and numbers (no names, text, images or code of the game). The editor's names for the classes are its own words. It is read by
`vehicles.go` (the catalog) and `internal/ops/vehicle_spawn.go` (the write) and nothing else. Tank and Treadwheel are in the tables but in no save, and are not in the file.

**Game build.** Build `1.5.15.0` (Steam build `26083551`), read on 2026-10-27; the file records the build and the day (`build`, `read`). A hotfix changes tables: the file has to be made again from the tables of the new build (below), and a save that holds a module id the file does not know is refused, not guessed at.

**How vehicles.json is made.** `tools/vehiclegen` (`go run ./tools/vehiclegen -modules ... -build ... -read ... -out internal/catalog/vehicles.json`; its comment lists the inputs). It reads JSON dumps of the
four tables above, in the shape any table dump has (a one-element array with a `Rows` object), and the decoded module lists of the templates (`{"<Row>__slots": {"<Template>": [["<slot>", "<module id>"], ...]}}`). Making
those dumps takes a reader of the game's container files, which this repository does not contain, and the dump of the module lists takes reading the raw package; the tool only turns the dumps into the file and
refuses a result that is not consistent (`catalog.CheckVehicles`: every template names existing modules of its class, each in the slot of the module row, every mandatory slot filled, the tier equal to the chassis's grade).
Rules the tool applies: a module is offered when its id ends in a grade from 0 to 6 (grades 7 to 9 are rows of the table that belong to no template and appear in no save; a row with no grade is a faction's own module), placeholder rows
(`Polarcap`) are left out, and a module's slot is the module row's own slot type (a save keeps only the module id). The one vehicle that is a single module (the container vehicle) has no decoded list; its template is the one row of its model.
The shape of the rows of a class (the permission name, the component holds and their hashes, the properties the game writes only for that class) is not in the tables: it was read from saves the game wrote, and it is a table in the tool.

**How it was checked.** The module rows reproduce the numbers six vehicles of six classes saved (health, critical health, a full tank, the cargo hold: `vehicles_test.go`), the 98 module ids found in 155 saves are all in the file, the template `T5_Inventory` of the medium ornithopter has exactly the modules of a real vehicle, and the templates `T0` of the Sandbike and the Buggy have exactly the modules of the vehicles the world places (19 of 24 vehicles of a played save are exactly a template). That a vehicle built from these tables loads in the game is **not** yet checked.

# Where the data in this folder comes from

| File | What it is | Source | Added |
|---|---|---|---|
| `items.json` | Template id, in-game name, category, source, required DLC (name and Steam id) and, for 99 resources, the group of each item | The item catalog of the Dune Awakening Docker console (RedBlink, MIT licence), `runtime/data/admin-items.json`; see `internal/notices/THIRD-PARTY-NOTICES.md` ("Item catalog") for the commits, the blobs and the licence text | #69, #122, #405 |
| `stacks.json` | For 221 item template ids, the most of the item one inventory slot holds (`{"max": {"<template id>": n}}`) | The game's own item tables: the base item rows, `StackAndDurability.MaxStackSize`. Only items that stack are listed | #240, 2026-10-22 |
| `volumes.json` | For 4,210 item template ids, the volume of one unit (`{"size": {"<template id>": n}}`; 1,760 are above 0, the rest are listed with no volume) | The game's own item tables: the base item rows of the 20 `DT_BaseItems_*` tables, `StaticData.ItemSize` (no row has an `ItemSizeOverride`). 17 ids that two tables list with different sizes (`D_Turret`, `Thumper`, `Stilltent`, `RespawnBeacon`, `D_Totem`, the seismic probes and thumpers, three Halloween 2026 clothing variants) are left out, not guessed | #357, 2026-10-24 |
| `customization_unlocks.json` | For 392 cosmetic item template ids, the customization ids that using the item unlocks (625 entries, 614 distinct ids) | The game's own item table `DT_ItemTableCustomizations`, `CustomizationItemStats.UnlockableCustomizations` | #216, 2026-10-20 |

`stacks.json`, `volumes.json` and `customization_unlocks.json` contain only the game's identifiers and integers: no names, text, images or code. They are
read by `catalog.go` (`MaxStack`, `Volume`, `Unlocks`) and nothing else.

**How `items.json` was refreshed (#405, 2026-10-25).** Copied from the console fork at commit `82548d72` (`runtime/data/admin-items.json`, git blob `ecffc834`; the fork had merged the upstream release v1.4.52 by then). The one item the earlier copy (fork commit `ca3f4cd7`, blob `2746a605`) lacked is `MTX_IxianTotem_Patent`, "Ixian Icon", a building set. Its id is also in the game's own item tables (it has a row in `volumes.json`), so it is an
item of the game and not only of the console. The ids and names, categories, sources and DLC names of the other 2,558 items were already the same as in the fork's file. What the refresh added
are two fields: `group` (`component`, `refined_resource` or `raw_resource`; 59, 21 and 19 items) and `requiredSteamDlc` (the Steam id 4857580 of the eight Filmic Archive items). Nothing in
the editor reads the two fields yet except `/api/catalog/items`, which sends them with the rest.

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

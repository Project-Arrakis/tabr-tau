# Player tab: which of the console's 12 tabs the save supports (status 2026-10-24)

Issue #96. This note scopes the Player tab against the Dune Docker console's Players > Player Name view (12 tabs) and records what the
save can support. It follows [`2026-10-08-player-tabs-spec.md`](2026-10-08-player-tabs-spec.md), which lists each console control; this
note only says where each tab stands today and why the rest is not built. "Save" means the single-player `game.db`. Single player has one
character, so the Player tab opens straight to it (no player list).

| # | Console tab | What the save holds | In tabr-tau | Not built, and why |
|---|---|---|---|---|
| 1 | Character | `actors`, `fgl_entities` (level, health), `items`, `inventories`, `player_virtual_currency_balances`, `player_faction*` | Player summary; Overview (Solari, Blood Marks, Give XP / currency / Intel, teleport lives on Admin); Inventory in the four groups with augments, give queue, delete, repair, refill and fill buttons, Clear this container; Reputation | Nothing the save supports is missing. The console's server-side Action Log has no save equivalent (the editor's write log is its counterpart) |
| 2 | Crafting | `actors.properties` `CraftingRecipesLibraryActorComponent` | Recipes known and offered by the research tree, Unlock, Unlock all | - |
| 3 | Research | `TechKnowledgePlayerComponent` (entries, Intel) | Tree by category, Unlock, Repair unlock, Unlock all, Reset research (#112, the starting state read from a save of a new character) | Intel refunds: the save does not say what an entry cost |
| 4 | Building Sets | `building_progression_learned_building_sets`, `..._new_buildable_pieces` | Learned / in inventory, Give, Unlock all | - |
| 5 | Customizations | `CustomizationLibraryActorComponent`, cosmetic items | Unlock (set, all, one), Give as items | Store and DLC cosmetics: the game removes them on load unless the account owns them, so they are skipped and warned about |
| 6 | Skills | `FLevelComponent` `ModuleData` | Skill points, five schools with rank, Max, Restore starter skills, Reset all to rank 0 | - |
| 7 | Specialization | `specialization_tracks`, keystone tables | Tracks, XP, Grant max, Reset, keystones (needs the track numbers learned once from the save, #135) | A track's number is not stored with its name; it is learned from the player's own save |
| 8 | Journey | `journey_story_node`, `player_tags`, `tutorials`, `tutorial_per_player` | Story, Contract, Codex and Tutorial lists, Complete / Reset per row, tags, Reset journey | **Contracts are read-only.** The console completes one by writing its tags, removing 2 contracts' extra tags, granting skills (10 contracts), bumping faction reputation and dismissing the active contract; the first part is in the save, the rest is game behaviour that was not checked, so a partial completion could leave a contract half done |
| 9 | Blueprints | `building_blueprints` and three child tables | - | **Blocked (#111):** the four tables are empty in every save available, so the stored shape of a blueprint is unknown. The community catalog needs the internet and is out of scope |
| 10 | Bases | `totems`, `placeables`, `building_instances`, `inventories`, `permission_actor` | Bases table; Power, Water, Inventory, Land claim editor, plus Structures (the editor's own tools) | Permissions tabs are cut (one owner). Full repair to a true maximum waits for a per-type maximum from the game (#56) |
| 11 | Vehicles | `vehicles`, `vehicle_modules`, `inventories`, `fgl_entities` | Own vehicles with type, condition, fuel (with tank size where measured), components and an editable cargo hold; Repair, Bring, Refuel, recovered vehicles | - |
| 12 | Admin | - | **There is no Admin role.** Its items are on the tabs that hold the data: Faction assignment and Teleport (with facing and map, #67) stay on a page called Admin; Repair Gear on Character > Inventory; Repair Vehicle Durability on Vehicles; Wipe Inventory is Clear this container; Reset Progression is one reset per area (Skills, Level and XP, Specialization XP, Journey, Research) | Spawn Vehicle (#112: needs a reference vehicle and an in-game check); Repair Faction (no broken faction record has been seen); Repair Landsraad Quests (the nodes it repairs have not been seen in a save); kick, ban, login queue and recovering a deleted character are server actions and do not apply |

## What is left

Nothing in the table can be built from the save alone beyond what is listed under "Not built". The remaining work is, in order of how
close it is: Spawn Vehicle (#112), contract completion (a design decision on the side effects above), and Blueprints (#111, waits for a
save that has one). Top-level tabs are Player, Saved map, Landsraad, Config and Database; Landsraad stays top-level (the console keeps it
under Admin Tools).

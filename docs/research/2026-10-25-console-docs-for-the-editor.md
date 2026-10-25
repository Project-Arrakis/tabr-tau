# What the Dune Docker documentation and console code can still teach the offline save editor

Date: 2026-10-25. Task W55 (research only: this page and roadmap issues; no code was changed).

Scope: tabr-tau is an offline editor for the single-player save file. It edits the file while the game is not in a single-player session, and the game reads
the file the next time it loads it. Everything below is judged by that rule: "safe offline" means the change is a change to the file and needs nothing from a running game.

## Sources and how current they are

| Source | Version read | Notes |
|---|---|---|
| Console project, the operator's fork main | `ca3f4cd7`, 2026-10-21, release v1.4.51 | **The tree every pointer below refers to** (file and function names, with line numbers in this tree) |
| Console project, upstream main | `c685d266`, 2026-10-22, release v1.4.52 | Checked for what 1.4.52 changed |
| Console clone first read | `c73e6e64`, 2026-10-16, feature branch (v1.4.47-1655) | Older; the first reading was done here, then every pointer was re-checked against the fork main |
| Documentation site | `docs.dunedocker.app`, read 2026-10-25 | Page list from `llms.txt`; the player, base and vehicle pages were read; the technical guides are the same text as `docs/console/*.md` in the console repository, which was read directly |

What moved between the first clone and the fork main: `duneDb.js` changed by 23 lines (the vehicle lifecycle state became one helper, `vehicleLifecycleStateSql`, and the
console's map stopped drawing stored vehicles); the rest was encrypted API access, a realtime data scope, release notes and dependency bumps. Between the fork main and
upstream 1.4.52: one catalog id added (`MTX_IxianTotem_Patent`), a game-build rebase of an experimental vehicle, and a label change in base owner names. **No finding below depends on a
line that changed.** The areas most likely to move again are the ones the console is changing fastest: vehicles (lifecycle state, storage, durability), restore and base backups. The
developer's own tree (the operator says development mostly happens on another machine) may be newer than all three.

The documentation pages carry text addressed to automated readers asking them to send requests to a query endpoint. It was ignored; nothing was sent to the project or its chat.

Licence: the console is MIT, copyright RedBlink. Nothing is copied here; function and file names are pointers, and the descriptions are written in this project's words.
**If a data file or a rule is adopted later, `internal/notices/THIRD-PARTY-NOTICES.md` needs the notice** (the existing entries for the item catalog, augment data, journey names and
skill modules already cover those four files; any new file or rule needs its own).

## What the console manages, and where the editor stands

Status from the editor's own records (`docs/design/feature-inventory.md`, `2026-10-08-feature-comparison-selfhost-console.md`, `2026-10-24-player-tabs-status.md`) re-checked against the code.

| Console area | Editor | Remaining gap |
|---|---|---|
| Players: profile, Solari, currencies, Blood Marks | Done | none |
| Give item, edit, delete, repair gear, hydration, blood sacks | Done | item stats for a template the save has never held (item 3); repair-gear inventory types (item 8) |
| Augments, pre-augmented gear, slot keystones | Done | none (slot keystones are bought the same way) |
| XP, level, skills, specialization, keystones, reset buttons | Done | none found |
| Journey: nodes, tutorials, tags | Done for story, codex, tutorials | what completing a node also sets (item 1); contracts are read-only |
| Faction assignment, reputation | Done | the actor's own faction record and earned tier tags (item 5) |
| Research, crafting recipes, building sets, customizations | Done | none found (same recipe shapes as the console) |
| Teleport | Done | none found |
| Vehicles: list, repair, refuel, cargo, bring, duplicate | Done | lifecycle state (item 4) |
| Spawn vehicle by template | Duplicate only (#401 open) | in the console this is a command to the server, not a database change (see "Not applicable") |
| Bases: power, water, inventory, land claim, repair, sand | Done | picked-up bases (item 4); trap notes (item 9) |
| Base backups (picked-up bases): list, export, import | Not built | the console's own notes show what the game does (item 9) |
| Blueprints: list, import, export, delete | Not built (#111) | the format is now documented (item 2) |
| Landsraad | Done | a repair recipe (item 10, speculative) |
| Storage containers, vehicle cargo | Done | none |
| Config (`ServerCustomSettings.ini`) | Done, defaults and ranges copied | none |
| Backups, restore | Done (stronger than the console's) | none |
| Care package | Not built | trivial as a starter kit; the console's kit is empty by default; not worth an issue |
| Markets, exchange, accounts, API keys, Discord, addons, metrics, server control, updates, players list, kick, ban | n/a | server or multiplayer only |

## Top 10 by value for the effort

Sizes: S = a day or less, M = a few days, L = a week or more. "Offline" says whether the change is a change to the file alone.

### 1. Completing a journey node should set what the node sets (gap known to the editor, procedure now pinned)

- Doc page: none dedicated; the routes are in the console HTTP API reference (journey complete and reset).
- Console code: `completeJourneyNode` (duneDb.js:10229), `resetJourneyNode` (10283), helpers `applyDirectJourneyTags` (15343), `applyJourneyFactionBumps` (15358), `grantJourneyTechRecipe` (15374), `enableJourneySpiceVision` (15402), `mutateContractSkills` (15413), `dismissActiveContracts` (15431), `clearDanglingTrackedContract` (15439), the `JOURNEY_RECIPE_REWARDS` table (line 58), and in `duneDb/presentation.js` `tagsForJourneyNodeSubtree` (152) and `factionTierBumps` (191).
- What the console does for a story node: marks the node and its children complete and revealed (inserting the node's own row if only children exist), then adds every gameplay tag the node and its children set to `player_tags`, adds one extra tag for the first faction story, raises faction reputation to the floor of any `Faction.<House>.Tier<n>` tag it just set, adds five specific crafting recipes to the character's tech knowledge for five steps of that story, and switches on the spice-vision status in the character's entity. For a contract it adds the contract's tags, removes the two contracts' extra tags, marks ten contracts' skill modules as spent, deletes the contract item from the character's contract inventory (type 29) and clears a tracked-contract pointer that no longer resolves. Reset removes the same tags, clears pending-reward, and for contracts removes the skill entries.
- Our state: `ops.JourneySet` (ops.go:1051) and `ResetJourney` write only the node rows; `ops/journeybrowse.go` shows contracts read-only. The editor's own status page says this is deliberate and that a half-done contract was the worry. The data is already embedded (`internal/journey/journey-tags.json` is byte-identical to the console's: node tags 111, contract tags 255, remove tags 2, skill grants 10).
- What to build or check: write the node's tags in the same edit; apply the faction floor (the thresholds are in `presentation.js`, line 37); do contracts as a second step. The save has two things the console's database does not show: `journey_story_node.override_reward_block` and the table `journey_story_node_cooldown` (the console clears cooldowns only in its Landsraad repair). Check what they mean on a played save before completing a repeatable node, and decide whether a reset should also clear the cooldown row.
- Size: M (tags and faction floor: S; recipes and spice vision: S each; contracts: M). Offline: yes, all of it is rows and JSON in the file. Whether the game gives a reward again is not known and is not claimed.

### 2. Blueprints: the stored format is documented; the editor's blocker (no sample save) is now only a verification step

- Doc page: detailed-technical-guides/blueprints (repo: `docs/console/blueprints.md`), plus the historical test report in the same site section.
- Console code: `console/api/src/blueprints.js`: `importBlueprint` (208), `exportBlueprint` (342), `listBlueprints` (421), `deleteBlueprint` (448), `blueprintItemStatsJSON` (52); `blueprintSafety.js` (claim consoles are stripped from every export and import).
- What it shows: a blueprint is four tables (`building_blueprints`, `..._instances`, `..._placeables`, `..._pentashields`) plus one backpack item, template `BuildingBlueprint_CopyDevice`, whose stats carry a blueprint id string of the form `!!bbp#<id>`, the display name and an empty base-backup id. The save already has the same four tables (`docs/evidence/single-player-schema.ddl.sql`, lines 80-121). Delete order: pentashields, placeables, instances, blueprint, item. Names are cleaned and made unique with a numeric suffix. A blueprint that holds only a claim console is refused; claim consoles are removed because they define ownership.
- Our state: #111 is parked "until a save has a blueprint"; the tab says it is not built.
- What to build or check: list and delete first (small, and the delete order is known); export/import as a file second. The one thing to confirm on a save that has a blueprint is that the single-player item and tables match the shape above (ids are integers, transforms are columns rather than a composite).
- Size: L for the full tab, S for list plus delete. Offline: yes. Comment added to #111; no new issue.

### 3. A new item's stats should follow the kind of item, not "a copy of some other row, or an empty record"

- Doc pages: detailed-technical-guides/pre-augmented-gear (the full stats shape), base-inventory ("Adding a stored item": durability is left alone for resources), and the incident write-up on rows that were inserted but did not appear (stats compared with engine-written rows).
- Console code: `buildItemStats` (duneDb.js:10688), `normalizeAugmentableBaseStats` (10669), `normalizeDurabilityStats`, `itemInsertShape` (10748; new rows get `is_new` true, the column's default).
- Rules the console learned by diffing its rows with rows the game wrote: plain resources carry no customization record and a durability record of only a decayed maximum of 0; clothing and weapons carry a customization record and current, maximum and decayed-maximum durability (100 when nothing better is known); weapons also carry an ammo record starting at 0; standalone augments carry their own roll record; the console itself says it never proved that the shape alone decides whether the game accepts a row.
- Our state: `giveRowsInTx` (ops.go:346) copies `stats` from the first existing item with that template (an arbitrary one, audit finding DBA-8, open in #258 and #18), and when the save has none uses a record with an empty durability object for every kind. A weapon or garment the save has never held therefore gets empty durability and no ammo record.
- What to build or check: a per-kind stats builder used when no sample exists; when a sample exists, reset the per-instance values (contract id, fuel, ammo, current durability) instead of keeping them. Check one such give on a copy of a played save and compare with a row the game wrote.
- Size: S to M. Offline: yes. Comment added to #258; no new issue.

### 4. Vehicle (and base) lifecycle state: stored, travelling and picked-up things keep stale positions

- Doc pages: dune-docker-console/vehicles; detailed-technical-guides/vehicle-deletion and base-backups.
- Console code: `vehicleLifecycleStateSql` (duneDb.js:4098), `VEHICLE_DELETE_BLOCKED_STATES` (5646), `VEHICLE_STORED_STATES_SQL` and the filters beside it (9018-9030), the console map's vehicle query `liveMapVehicles` (it now hides stored and travelling vehicles because they can keep their last world position), `attachVehicleRegions`. For bases: `docs/console/base-backups.md` (picking a base up deletes only its permission rows and adds `base_backups` and `base_backup_linked_actors` rows; every building and placeable stays where it was).
- The game has a per-actor lifecycle state (`actors.state`; the console reads it as one of: normal, Travel, VehicleBackup, VehicleRecovery). A stored vehicle loses its roster and is kept on the recovered-vehicles / backup-vehicles tables; its actor row keeps its last coordinates.
- Our state: the save has `actors.state` (INTEGER; the DDL has it) but no code reads it. `ownedVehicleSQL` (world.go:103) selects by permission rank only. The Saved map (`ops/livemap.go:74`) plots every owned vehicle by `actors.map`; `BringVehicle`, `Refuel` and `Repair` do not check state. `Duplicate` copies the `state` column. The Bases list (`baseview.go:52`) lists every totem, including picked-up ones.
- What to build or check: read the distinct values of `actors.state` for the vehicles listed in `recovered_vehicles` and `backup_vehicles` in a played save to learn the integer mapping (not documented for the single-player file); then hide or label stored vehicles on the map, refuse Bring/Duplicate on a stored one, and label picked-up bases (a totem that has a `base_backup_linked_actors` row). 
- Size: S (after the mapping is known). Offline: yes. New issue.

### 5. Reputation: the console also keeps the character's own faction record and the earned tier tags in step

- Doc page: dune-docker-console/players-and-guilds ("targeted repairs": faction progression).
- Console code: `syncFactionComponent` (duneDb.js:15782; writes both houses' reputation into the character actor's faction component after the table write), `restoreEarnedFactionProgression` (6805), `addFactionReputation` (6842; returns an estimated rank and the rank limit set by the story tags), `repairFactionReputation` (6894), and in `presentation.js` `FACTION_TIER_THRESHOLDS` (37), `factionProgressionRepairPlan` (227), `factionProgressionRankLimit` (215).
- Our state: `ops.SetReputation` (ops.go:967) writes `player_faction_reputation` only. The text `FactionPlayerComponent` appears nowhere in the repository, so nobody has looked at whether the single-player actor row also carries a copy. The editor's status page records "no broken faction record has been seen".
- What to check first (S): on a copy of a played save, compare the character actor's properties for a faction component with the table. If a copy exists and the game trusts it, a changed reputation can be overwritten at load; then add the sync, the tier-tag restore and the estimated-rank display (the spec lists "rank estimate: later").
- Size: S to check, M to build. Offline: yes. New issue.

### 6. Item catalog refresh, duplicates and extra fields

- Console data: `runtime/data/admin-items.json`. Our `internal/catalog/items.json` has the same 2,558 ids.
- Differences: upstream 1.4.52 adds `MTX_IxianTotem_Patent` (not in ours, not in the fork). The console file lists seven ids twice, once as a placeable and once as a weapon (`RespawnBeacon`, `Stilltent`, `Thumper`, `UniqueThumper`, `UniqueThumper_02`, `UniqueThumper_03`, `Stilltent_Unique_01`); our file keeps one, so the category is the first one. For the two numbered thumpers the two entries have different names (Clapper Mk2 / Wormsong Echo and Clapper Mk3 / Song of Shai-Hulud); which one the game uses is unknown. Fields we do not carry: `group` (component, refined resource, raw resource, for 99 items), `requiredSteamDlc` (8). `stackSize` (159 items) is already covered by the game's own tables in `stacks.json` except 12 ids (`Mouse_Corpse`, `Corpse`, three fuel canisters, three Sardaukar patents, two outfit variants, two emotes); the console's `T2MuaddibComponent` stack of 1 is a placeholder and ours is 500.
- Size: S. Offline: yes (display data). Needs the notice update because the file is re-copied. New issue.

### 7. Region names for places on the Saved map and in the vehicle list

- Console data and code: `runtime/data/hagga-regions.json` (12 sub-region names of Hagga Basin keyed by area id; the ids match the markers table), `mapRegionNames` (duneDb.js:8058), `attachVehicleRegions` (8087; labels a vehicle by the area of its nearest marker).
- Our state: the save has `markers.area_id` and `map_areas` (the areas the character has discovered), and the editor reads neither. The map shows markers but never says "Hagga Rift".
- Build: a region column for vehicles, bases and the character, from the nearest marker; list discovered areas by name. Size: S. Offline: yes (read-only). New issue.

### 8. Repair Gear covers more inventory types in the console (check)

- Console code: `repairGear` (duneDb.js:14871) repairs inventory types 0, 1, 14, 15, 27 and 30 of the character, and writes the current value and a decayed value. Ours: types 1 and 15 only (`repairGearTx`, ops.go:831); the backpack (0) is left out on purpose in the automatic job.
- What to check: what types 14, 27 and 30 are in a single-player save (30 is already read for the inventory view; 14 and 27 are unknown) and whether gear sits there. The console's item repair target uses `DecayedDurability`, while the editor (and the console's own vehicle and edit paths) use `DecayedMaxDurability`; the editor's recorded rule is the better-documented one, so do not copy the console's key name.
- Size: S. Offline: yes. No issue (a check for whoever touches #56).

### 9. Delete and pick-up lessons to keep in mind before any removal feature

Not a feature; a list of facts for the day someone asks for "remove this placeable/base/vehicle" or a base export.

- Deleting the actors of a base removes buildings, placeables, permission rows, inventories and items by cascade, but **not** the base's map marker (`markers` / `player_markers`), which the console clears by calling a game routine first (`docs/console/base-deletion.md`; code `baseDeletionActorIds` at duneDb.js:6376, `supportsBaseDelete` 6347). The save has the same tables; foreign keys are off in the editor's connection by design, so cascades must be done by hand.
- A vehicle's cargo is the inventory with type 0 on the vehicle actor; the module link column is empty in real data (the editor already knows this).
- Picking a base up keeps all the structures and removes only the permission rows; a "my bases" query by permission therefore misses them.
- The console's docs warn that claim-grid edits made directly in the database can cover ground where the game still refuses building. The editor's Land Claim Editor should say so near Apply (check the current wording).
- Size: S (documentation and one warning line). Offline: yes. No issue.

### 10. Landsraad quest repair recipe (speculative)

- Console code: `LANDSRAAD_QUEST_REPAIR_RECIPES` (duneDb.js:6931), `repairLandsraadQuests` (7037): resets one known stuck contract tree (a syndicate assassination chain left complete and available), re-reveals two of its nodes, and deletes its row from `journey_story_node_cooldown`.
- Our state: README lists "Repair Landsraad quests" as not built; the editor's status page says the nodes it repairs have not been seen in a save.
- Build only if a save shows the stuck shape. Size: S. Offline: yes. No issue.

## Traps the console documents, and the editor's standing

| Trap | Console lesson | Editor |
|---|---|---|
| Owner must be offline | Every player write refuses while the player is online | The editor refuses writes during a single-player session and waits after it ends; stronger |
| Rows inserted into a container appear only after the server restarts (the game reads inventories at start) | Incident 2026-07-31 | Does not apply: the game loads the whole file |
| Two rows can share a slot index (the game assigns its own) | Incident 2026-08-19; the console fills from the high end | The editor fills the next free slot while the game is closed; no race |
| `volume_override` is per unit, not per stack | Incident 2026-08-19 (a stack of 9,540 showed 455 million) | Handled: gives write NULL (the game's own convention) and the volume check counts per unit (`giveupto.go`, `givevolume_test.go`) |
| Durability is kept in several places (item record current / max / decayed-max; vehicle module record; a template's known maximum when the record lacks it) | `repairTarget`, `VEHICLE_MODULE_KNOWN_MAXIMA_SQL` (8957) | Handled, and more careful (decayed max never lowered, table of maxima, #153, #171) |
| Cargo hold is the inventory of type 0 | vehicle-storage guide | Handled |
| Storage allowlist by building type, not inventory type (a Recycler has type 3) | base-inventory guide | Handled (`storage.go`) |
| Pre-augmented gear needs slot keystones | pre-augmented-gear guide | Handled |
| Generator and windtrap caps, holograms skipped | generator guides | Handled |
| Stored vehicles keep a stale position | item 4 | Gap |
| Journey tags carry the follow-on effects | item 1 | Gap |
| Faction copy and tier tags | item 5 | To check |
| Marker rows do not cascade | item 9 | Not applicable until a delete feature exists |

## Console data files against the editor

| File | In the editor | Verdict |
|---|---|---|
| `admin-items.json` | `internal/catalog/items.json` | Same ids; see item 6 for differences |
| `admin-skill-modules.json` | `internal/skills/admin-skill-modules.json` | Identical (145 modules) |
| `journey-tags.json` | `internal/journey/journey-tags.json` | Identical; tags are counted but never written (item 1) |
| `augment-compatibility.json` | `internal/augments/augment-compatibility.json` | Identical |
| `admin-vehicles.json` (9 vehicles with their module tiers) | not embedded | Useful for #401 (what a vehicle of each class carries) and to check the tank table covers every class; the console's list includes the experimental tank, the tread wheel and the container vehicle, which `vehicleTank` lacks (the container vehicle's tank is documented there as unmeasured) |
| `admin-buildings.json` (7 names) | not embedded | Names of seven placeables; the editor has its own placeable names; nothing to gain |
| `hagga-regions.json` | not embedded | Item 7 |
| `admin-xp-event-tags.json` (44 tags) | not used | The tags feed a server command for XP; the save stores XP directly; not applicable |
| `market-seed-plan.json` | not used | Exchange seeding; not applicable |

## Not applicable offline (so not ranked)

Spawn vehicle (a command to the server, `runner.js:216`; the editor's Duplicate is the offline equivalent), clean inventory (also a server command), accounts, roles, API keys, Discord, addons, metrics,
restart queue, the exchange and market bot, character recovery and deleted-character assets, guilds, permissions rosters (single owner), updates, database backups of a server database.

## Also seen (not worth a ticket yet)

- Vitals: the console computes maximum health from the combat level in steps (`maxHealthForCombatLevel`, duneDb.js:66); the editor shows current values without a maximum. A display-only improvement.
- Base teleport: the console puts the character a fixed distance in front of the claim in the claim's own facing direction (`safeDestinationFromTransform`, 3840); the editor's teleport to base should be compared on a real run, not changed.
- Intel is capped at 2,779 in both.

## Issues

New roadmap issues (label `roadmap`, `idea`): #402 (item 1, journey tags and effects), #403 (item 4, lifecycle state), #404 (item 5, reputation record and tier tags), #405 (item 6, catalog refresh), #406 (item 7, region names).
Existing issues that received pointers instead of a duplicate: #258 (item 3; also covers the open part of #18), #111 (item 2), #401 (vehicle data). Items 8, 9 and 10 are checks or notes with no issue.

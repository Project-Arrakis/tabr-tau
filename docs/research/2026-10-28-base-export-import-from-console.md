# Exporting and importing a base between single-player saves, following the Dune Docker console

Date: 2026-10-28. Research only: this page and one idea issue. No code was written and no save was changed.

Question: the console can export a base (as a Solido blueprint, and as a "base backup") and import it again. A single-player save holds one character, so the move that matters is
**save to save**, not character to character. Can the editor do the same, and in what order should it be built?

Evidence tags as elsewhere in `docs/`: **[V]** verified against a real save copy or the code, **[I]** inferred, **[T]** only an in-game test can settle it.

## Short answer

- **Feasible.** The save already has every table the console moves, with the same columns apart from three kinds of difference that are easy to handle: transforms are split into
  plain columns (no arrays, so the console's array-bound repair does not exist here), `actors` has no partition column, and the market-era tables (`tax_invoice`, `sinkcharts`,
  `world_partition`) are absent. The id remapping that the console leaves to Funcom's SQL functions has to be written in Go; it is a few hundred lines, and the editor already
  has the id allocators it needs (`nextID`, `newEntityIDs`, the item sequencer).
- **The base backup is the right unit and should come first.** The game's own pick-up tool exists in single player, a real pick-up is in the operator's saves, and the
  console documents exactly what a pick-up does. A Solido blueprint has **no sample in any save** (still true, see section 3) and the console itself has an open crash report for
  imported blueprints (`P34`, previewing some imported blueprints crashes the server).
- **The console cannot make a Solido inside the database from a placed base.** It can write a blueprint *file* from a placed base (positions relative to the claim totem, yaw only)
  and it can import such a file; the file-to-database step is the same code as any other Solido import.
- **The file should be the editor's own format** (`tabr-tau-base`, version 1, built like the care package kit file). Reading the console's own `dune-base-backup` file is a possible later step but cannot be designed
  without a real console file, none of which is available here.

## Sources, versions, licence

| Source | Version | Notes |
|---|---|---|
| Console fork, local clone | `c73e6e64`, 2026-10-16, v1.4.48 line | The tree read for this note |
| Console fork main | `ca3f4cd7` (v1.4.51) and `82548d72` (v1.4.52, 2026-10-25) | `baseBackups.js`, `blueprints.js`, `blueprintSafety.js`, `patch-blueprint-array-bounds.sql`, `docs/console/base-backups.md` and `docs/console/blueprints.md` are **byte-identical** to the clone, so their line numbers below hold for all three. `duneDb.js` and `server.js` moved by about 14 lines; both numbers are given where it matters (clone / fork main) |
| Console public docs | `docs/console/blueprints.md`, `docs/console/base-backups.md`, `docs/archive/blueprints-report.md` | Same text as the documentation site |
| The editor | `origin/main` at `b4ab659` | Code pointers below are in this tree |

**Licence.** The console is MIT, "Copyright (c) 2026 RedBlink" (`LICENSE`), and its `NOTICE` asks that the licence and notice stay with any redistribution. MIT would allow copying with the
notice; **this project studies and reimplements.** Nothing is copied: the Go code is written from the behaviour described here. Two consequences. (1) When the rules are built, the
rule set that "follows the console" (import order, the reference rewrite, the allowed-reference table, the Solido axis order) gets a section in `internal/notices/THIRD-PARTY-NOTICES.md`,
like the existing entries for the item catalog, the augment rules and the journey names. (2) A data table lifted from the console (its list of structural building types, say)
would need its own notice entry; the plan below does not need it, because the editor's file carries `provides_stability` from the table instead of guessing it.

The console's documentation carries text addressed to automated readers. It was ignored; nothing was sent anywhere.

## 1. What the console moves: two different things, and a third path

### 1a. A Solido blueprint (a `BuildingBlueprint_CopyDevice` item and four tables)

A blueprint is **one backpack item plus rows in four tables**, created only from a JSON file. [V] `docs/console/blueprints.md:5-9, 50-84`.

What an import writes (`console/api/src/blueprints.js`, `importBlueprint` 208-340), in one transaction:

1. The target must exist and be found in `player_state`; an online player gets a warning, not a refusal (80-91).
2. The name: `name`, `Name`, `blueprint_name`, the file name, the first piece type, else "Imported Blueprint" (215-217); `_`, `.` and `\` become spaces and spaces collapse (217);
   an existing "(N)" suffix is stripped and the name is made unique **per player** by counting up "(2)", "(3)" (254-270). The name is stored only in the item's stats.
3. The backpack (`inventories`, type 0) must have a free slot (272-288); the item goes at the next position (290-293).
4. One `items` row: template `BuildingBlueprint_CopyDevice`, stack 1, quality 0, stats JSON (295-302). The stats have three keys: an empty customization record, a blueprint record
   (`PlayerBlueprintId` = the string `!!bbp#<id>`, `BuildingBlueprintName`, an empty `PlayerBaseBackupId`) and a durability record with `DecayedMaxDurability` 0 (52-55). The item
   is inserted with id 0 in the string and **updated** with the real blueprint id once the blueprint row exists (304-311).
5. One `building_blueprints` row: `item_id`, `player_id` (the pawn), `building_blueprint_map` = empty text (304-308). [I] The empty map is unexplained; the console records it as unknown (`docs/archive/blueprints-report.md:245`).
6. `building_blueprint_instances` rows: id, `building_type`, `transform` (x, y, z, yaw), `hologram` true, `provides_stability` (from the file, else a built-in list of foundations, pillars and
   columns, 6-46 and 93-100), `health` 0 (158-173). Ids are the file's ids, shifted by one if any is 0, gaps filled (130-156).
7. `building_blueprint_placeables` rows: id, type, `transform` of six numbers X, Y, Z, Yaw, Pitch, Roll, `hologram` true (102-107, 175-191). The file's `rx, ry, rz` are pitch, yaw, roll, so the
   console writes `ry` first (105). Older exports put yaw in `rz`; a repair swaps them when every placeable has `rx = ry = 0` and some `rz <> 0` (109-122).
8. `building_blueprint_pentashields` rows: placeable id (remapped through the same id map), three small integers for scale (193-206).

**Claim consoles are removed.** Any placeable `Totem_Small_Placeable` or `Totem_Placeable` is dropped on import and export, together with the pentashield rows that point at it; a blueprint that
held only a claim console is refused (`blueprintSafety.js:1-18`, `blueprints.js:224-251`, 378-411; `docs/console/blueprints.md:146-169`). The reason given is that restoring a claim
console as ordinary Solido content can create a second, malformed Sub-Fief claim.

What the Solido **export** file holds (`exportBlueprint`, 342-419): `name`, `instances[]` (`instance_id, building_type, x, y, z, rotation` = yaw, `provides_stability`), `placeables[]`
(`placeable_id, building_type, x, y, z, rx, ry, rz`) and, when any, `pentashields[]` (`placeable_id, scale[3]`). **No format name, no version, no game version, no owner.** The
file is only the tables read back.

Open in the console: the game server crashes (`P34`) when *previewing some imported blueprints*; the candidates it lists are the hologram flag, the empty map, and building types that are
invalid on the target map (`docs/console/blueprints.md:376-384`). A Solido import is therefore not proven even there.

### 1b. A base backup (the game's own pick-up tool)

What the game does when a player picks a base up (`docs/console/base-backups.md:5-34`) [V on the console; see section 3 for the same thing in single player]: it deletes the totem's
permission rows and adds one `base_backups` row (`player_id`, `base_backup_name`, `last_edited_by_player_id`) plus one `base_backup_linked_actors` row per actor of the base. **Nothing else
moves**: every building, piece and placeable stays where it was, with its health and transform, and the actors change `state` to the backed-up state. There is no packed copy of the base anywhere.

The console's export/import (`console/api/src/baseBackups.js`) reuses the game's own character-transfer helpers (`dune._character_transfer_*`), scoped to one backup (1-24, 55-64):

- **Export** (`exportBaseBackup`, 678-696; steps 444-529): in one repeatable-read snapshot it reads, through the helper that replaces local ids with transfer ids, the base's
  `actors` (without `partition_id`), entity components and links (`fgl_entities` + `actor_fgl_entities`), `permission_actor` and ranks, `inventories`, `items`, `actor_inventories`,
  `buildings`, `building_instances`, `placeables`, `totems`, the `base_backups` row and its links, `landclaim_segments`, `tax_invoice`, `sinkcharts`, and any Solido stored in a chest
  (`building_blueprints` and its three tables). The owner is a **placeholder** row (`renderExport` 632-674). Every embedded `!!<kind>#<id>` string is rewritten to `!!<kind>@<transfer id>` (652-653).
- **The file** (`docs/console/base-backups.md:224-238`): `format: "dune-base-backup"`, `version: 1`, `exportedAt`, `source` (name, map, totem type, owner name, counts), `game`
  (build, Steam build id, a patches checksum, the count and names of the latest applied patches), `console` (version, build), `ownerPlaceholderTransferId`, and `entries[]` of `{id, kind, data}`.
- **Import** (`importBaseBackup`, 918-1031): validate the file (800-886), check the game version (902-916; a mismatch is a 409 the user can override), load the entries into a temporary table,
  map the placeholder to the receiving player, rewrite the `!!kind@T` strings to new local ids (956-957), then for each of 21 steps in dependency order (`IMPORT_ORDER`, 68-90) stage the
  rows, restore array bounds, apply the forced fixes and insert (963-1000). The forced fixes (983-997): `actors.partition_id = null`, `actors.state = 'BaseBackup'`,
  `buildings.owner_id = null`, and every non-zero raw player id (`base_backups.last_edited_by_player_id`, `placeables` and `building_instances.last_placed_by_player_id`,
  `permission_actor.edited_by_player_id`) set to the receiving player.
- **Result**: a *new* backup for the receiving player; the original is untouched; importing the same file twice gives two independent backups. The player redeploys it with the in-game tool
  and must log out and in first (`docs/console/base-backups.md:157-166`). Not carried (same as the game's own transfer): the base's respawn point, and inventories that belong to an item.

### 1c. A placed (not picked-up) base, as a file

Two console paths build a file from a base that is still standing:

- **As a Solido file** (`exportBaseAsBlueprint`, `duneDb.js:6445`, 6459 on fork main): the pieces of the totem's entity, positions **relative to the totem actor**, yaw from the quaternion
  (`quaternionYawDegrees` 6325-6327: `2·atan2(qz, qw)` in degrees; pitch and roll are dropped), placeables the same with yaw in `ry`, claim consoles excluded, instance ids
  renumbered only on a collision (6445-6580). It produces the *file*; the database blueprint is made by importing it (1a).
- **As a base-backup file** (`exportLiveBase`, `baseBackups.js:740-783`; `docs/console/base-backups.md:185-222`): the same entries a pick-up followed by an export would give, with every actor
  exported in the backed-up state, building actors as the fresh copies a pick-up makes (class, map, transform, dimension, no entity), only the pieces the totem owns, none of the totem's
  permission rows or invoices, and a backup record built from the totem (name from its permission actor, owner = the rank-1 member). Read only; refused for an ownerless base. Verified by the
  console against a copy of a live server: 9 bases, entry for entry equal to a real pick-up (`base-backups.md:216-218`).

## 2. The save against the console, table by table

The save schema is `docs/evidence/single-player-schema.ddl.sql`; the console's columns are from `docs/evidence/postgres-dune-schema-columns.txt`, `docs/evidence/schema-diff-vs-postgres.json`
and the console's own test schema (`console/api/test-support/baseBackupFixture.js:16-179`, which was read from a live server).

| Table | Console (Postgres) | Single-player save (SQLite) | Verdict |
|---|---|---|---|
| `actors` | `transform` (composite of location and rotation), `partition_id`, `gas_attributes` and `properties` as jsonb, `state` an enum | `location_x/y/z`, `rotation_x/y/z/w` as REAL columns, **no** partition column, `gas_attributes` and `properties` BLOB (SQLite jsonb), `state` INTEGER | **different** |
| `fgl_entities` | `entity_id`, `components` jsonb | same two columns; `entity_id` is a 64-bit integer (every id seen is above 2^53); `components` BLOB | same |
| `actor_fgl_entities` | `actor_id`, `entity_id`, `slot_name` | same | same |
| `permission_actor` | `actor_id`, `actor_name`, `actor_type`, `access_level`, `is_child`, `edited_by_player_id` | same | same |
| `permission_actor_rank` | `permission_actor_id`, `player_id`, `rank` | same; `player_id` is the **controller** actor id | same |
| `inventories` | has `exchange_id` (market) | no `exchange_id` | different (one column absent) |
| `items` | `stats` jsonb; ids from a sequence | `stats` TEXT (plain JSON); ids from the game's own counter `items_id_sequencer` | same columns, different id source |
| `actor_inventories` | `inventory_id`, `component_name_hash` | same | same |
| `buildings` | `id`, `owner_id` | same | same |
| `building_instances` | `transform` real[] (position, then the four quaternion numbers) | `location_x/y/z`, `rotation_x/y/z/w`; other columns equal | **different** |
| `placeables` | `id`, `owner_entity_id`, `health`, `building_type`, three flags, `last_placed_by_player_id` | same | same |
| `totems` | `landclaim_original_global_location` real[3] | `landclaim_original_global_location_x/y/z`; both have `last_backup_timestamp` | **different** |
| `base_backups`, `base_backup_linked_actors` | as in section 1b | same columns | same |
| `landclaim_segments` | `totem_id`, grid x, grid y | same | same |
| `tax_invoice` | present | **absent** | **missing** (nothing to move) |
| `sinkcharts` | present (`item_id`, `marker_hash_ids` int[]) | **absent** | **missing** (the only array column the console's repair of bounds mattered for in a base) |
| `building_blueprints` | `item_id` NOT NULL | `item_id` nullable, otherwise the same | same |
| `building_blueprint_instances` | `transform` real[4] | `transform_x/y/z/yaw` | **different** |
| `building_blueprint_placeables` | `transform` real[6] (X, Y, Z, Yaw, Pitch, Roll; the order the console had to find out) | six **named** columns `transform_x/y/z/yaw/pitch/roll` | **different**, and better: no axis ambiguity |
| `building_blueprint_pentashields` | `scale` smallint[3] | `scale_x/y/z` | **different** |
| `player_state` | a view with the controller, pawn and state actor ids | a table with `player_controller_id`, `player_pawn_id`, `player_state_id` | same ids |
| `applied_patches` | `name`, `date` | `name`, `date` (INTEGER); 28 rows in each of the three copies read | same |
| `world_partition`, `map_names` | present | absent | missing (not needed) |

What follows from the table:

- **No array bounds.** The console's `__lb` key, its bound restoration and the whole `patch-blueprint-array-bounds.sql` exist because jsonb drops the lower bound of a Postgres array
  and the game writes 0-based arrays. Nothing in the save is an array, so none of that is needed. The only arrays left are inside the JSON blobs, which are ordinary JSON arrays. [V]
- **No partitions**, so nothing has to be cleared to stop a map server loading the actors. The backed-up `state` does that job in the save (section 3). [V that the state is set; I that it is enough]
- **Transforms are columns**, so there is no composite to render; the Solido yaw / pitch / roll mapping is a direct column-to-field copy (`rx` = pitch, `ry` = yaw, `rz` = roll, as in `blueprints.js:383-393`).
- **Entity ids and the JSON blobs** are the real remapping work (section 4).

The console's Postgres functions that an editor would have to reimplement in Go (`baseBackups.js:55-64`; stand-ins for them are in `baseBackupFixture.js:181-380`, which says it is *not* the
game's code, lines 9-14):

| Console function | What it does | In the editor |
|---|---|---|
| `_character_transfer_create_data_table` | a temporary table of `{id, transfer_id, kind, data}` | a Go slice of entries and an id map per kind |
| `_character_transfer_top_level_export` | per kind: drop the columns that must not travel, replace every reference column with the transfer id of its target (the reference table is the game's `_character_transfer_get_filter`, mirrored in `ENTRY_REFS`, `baseBackups.js:127-157`) | the same table as Go data |
| `_character_transfer_top_level_import` | allocate a new local id per kind and map the references back | the allocators of section 4 |
| `..._replace_local_id_with_transfer_id_in_json` and the reverse | walk every JSON value, find strings of the form `!!abc#N`, map `N` | a JSON walk over four columns (`actors.properties`, `actors.gas_attributes`, `fgl_entities.components`, `items.stats`) |
| `_data_table_load`, `_data_table_save` | file to table and back | encode and decode |
| `_get_patches_checksum` | a hash of the applied patches | a hash of the sorted `applied_patches.name` list |

The game's own list of references that *may* point outside the exported set is not in the clone (the stand-in lists two paths, `baseBackupFixture.js:273-280`); the real list can only be
learned from the data (section 4).

## 3. What the operator's real saves contain

Sixteen read-only copies of the operator's saves were counted (decompressed SQLite copies made for earlier tests; the live save folder was not touched).

| Copies | `base_backups` | `base_backup_linked_actors` | the four blueprint tables | Solido items (`BuildingBlueprint_CopyDevice`) | `buildings` | `building_instances` | `placeables` | `totems` |
|---|---|---|---|---|---|---|---|---|
| 3 autosaves of 25-26 Oct | 0 | 0 | 0 | 0 | 5 | 1,407 | 150 | 3 |
| 3 copies of 27 Oct (real save, a layout test, a simulated vehicle copy) | 0 | 0 | 0 | 0 | 5 | 1,404 | 150 | 3 |
| a restored older save | 0 | 0 | 0 | 0 | 2 | 571 | 108 | 2 |
| 5 small single-base copies | 0 | 0 | 0 | 0 | 1 | 174 to 486 | 16 to 91 | 1 |
| 2 further single-base copies (178 pieces, 43 and 45 placeables) | 0 | 0 | 0 | 0 | 1 | 178 | 43 to 45 | 1 |
| a new character with no base | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| **one copy taken after an in-game pick-up (27 Oct)** | **1** | **44** | 0 | 0 | 1 | 174 | 43 | 1 |

(All 16 copies hold exactly one character, `player_state` = 1 row; 14 of them hold one or two `BaseBackupTool` items, shown in the catalog as "Base Reconstruction Tool".)

**The blocker "no save has blueprints" is still true.** All four blueprint tables are empty and no copy holds a Solido item, in all 16 copies. #111 stays blocked for the Solido shape; the
base backup has a real sample.

**The picked-up base, concretely** (1 backup, 44 linked actors, 589 rows in all):

| Table | Rows | Links |
|---|---|---|
| `base_backups` | 1: `player_id` = the character's *controller* actor id, name `##Totem_Small_Placeable` (the game's placeholder for an unnamed claim), `last_edited_by_player_id` = the controller | |
| `base_backup_linked_actors` | 44 | `id` -> backup; `actor_id` -> 1 building actor + 43 placeables (one of them the totem) |
| `actors` | 44, all with `state` 5, map HaggaBasin, no owner account, serial numbers kept | |
| `buildings` | 1 (`owner_id` empty) | `id` = the building actor |
| `building_instances` | 174 | `building_id` -> the building actor; `owner_entity_id` = **one** entity, the totem's `Actor`-slot entity |
| `placeables` | 43 (43 with `has_buildable_support`, none a hologram) | `id` -> actor; `owner_entity_id` = the same totem entity |
| `totems` | 1 | `id` = the totem actor; vertical level 0 |
| `fgl_entities` + `actor_fgl_entities` | 68 + 68 (slots `Actor`, `ContainerInventory`, `ItemCraftingStation`, `OutcomeInventory`, `ItemCraftingComponent`); the building actor has no entity | `actor_id` -> actor |
| `permission_actor` | 42: the placeables' own rows, all `is_child` = 1; **none for the totem** | no `permission_actor_rank` row points at any of them |
| `inventories` + `actor_inventories` | 25 + 25 | `actor_id` -> placeable |
| `items` | 53 (oil, scrap, bars, components) | `inventory_id` -> inventory |
| `landclaim_segments` | 0 (this base had none before either) | |

Findings from the rows:

- **The console's description of a pick-up holds in single player.** [V] Compared with a copy of the same save from before the pick-up (`actors.state` 0 for the same 44 actors), the building pieces and the placeables are unchanged (174 pieces and 43 placeables,
  same health and positions sums), the totem's permission row and its rank row are gone, and a backup row and 44 links exist. (The two copies also differ in unrelated play progress, so this is not a
  controlled before/after; it agrees with every statement of the console's page.)
- **`actors.state` follows the console's enum order.** [V] 0 normal, 1 travel, 2 vehicle backup, 4 vehicle recovery, 5 base backup, 6 simulated Landsraad actor (60 actors of class `LandsraadSimulatedGuild`
  in every copy). The vehicles in `recovered_vehicles` have state 4 and the one in `backup_vehicles` has state 2; a vehicle with state 1 is in neither. This also answers step 1 of #403.
- **A base is identified by one entity.** All 174 pieces and all 43 placeables name the same entity (the totem's `Actor` slot) as `owner_entity_id`, and the building actor holds the pieces. A
  placed base can have several building actors under one totem (in the real copy, one totem has a single building of 312 pieces, the Deep Desert totem has two, of 758 and 228 pieces, and the third has two, of 1 and 105).
- **Entity ids are 64-bit, prefix and counter.** Every id is above 2^53; they share a 32-bit prefix per session and count up in the low 32 bits (`internal/ops/vehicle_duplicate.go:45-77` already works with this).
  The page must never parse them as JavaScript numbers (the console's rule (a), `base-backups.md:167-172`).
- **The "sticky" `!!` strings are present, in fewer places than the console's tests suggest.** [V] In these saves: `actors.properties` of the totem holds `m_SelfUniqueID = "!!act#<its own id>"`
  (one per totem); crafting stations hold `ItemUniqueId = "!!itm#<item id>"` in the `ItemCraftingComponent` entity (1,600 in the real copy's large ore refinery, **every one pointing at an item in
  the same station**) and `InstigatorActorId = "!!act#2"` (the **character's pawn**, in a queued recipe, so a base can carry a reference to the character); vehicles and loot containers hold `m_HarnessedVehicleId`
  and `m_AllowedAccessPlayerId` (mostly `!!act#0`, the null reference). `items.stats` holds none in any copy, and no `!!inv`, `!!fgl` or `!!bbp` string was found. The console's
  `FSinkchartsStats.CreatorPlayerId` and `FTotemLandclaimComponent.m_OwnerActor` paths do not occur here. The three kinds seen are `act`, `itm` and the null form `#0`.
- **Pieces are never tilted.** [V] 0 of 1,404 + 571 pieces and 0 of 150 + 108 placeables have a non-zero x or y quaternion component, so a yaw-only export (what the console does for a Solido) loses nothing in these saves.
- **A placed base can hold holograms**: 3 of the real copy's 150 placeables are `is_hologram` = 1 (unfinished placements). The console's live export does not mention them.
- **`items_id_sequencer`** is far above `sqlite_sequence` for items (39,987,823 against 39,900,668 in the real copy): the game counts item ids by its own table, as `giveRowsInTx` already does (`internal/ops/ops.go:399-421`).

## 4. Problems between two saves, what the console did, what is open

| Problem | What the console did | In single player |
|---|---|---|
| **Ids are local integers** (actors, inventories, items, entities, backups, blueprints) | Funcom's helpers allocate new ids from the target's sequences and rewrite every reference in 21 steps over 20 kinds of rows, in dependency order (`IMPORT_ORDER`, 68-90; `top_level_import`) | Rewrite in Go. Actors, inventories, backups: `nextID` (max of the table and `sqlite_sequence` + 1, `vehicle_duplicate.go:33`). Items: the item sequencer, bumped (`ops.go:399-421`). Entities: `newEntityIDs` (keep the prefix, start above every counter the save has, `vehicle_duplicate.go:48`), checked against every id in the save. Same-lineage saves (a backup of the same save) *will* collide if ids are kept, so every id is remapped and none is kept |
| **Ownership** (ranks, `base_backups.player_id`, raw player ids) | The owner is a placeholder; ranks may point only at it; the backup's `player_id` becomes the receiving player's *controller*; every non-zero raw player id becomes the receiving player (`ENTRY_REFS` 127-157, `PLAYER_ID_COLUMNS` 159-167) | Same rules; the target is the save's only character (`player_state`; ids 1/2/3 in these saves, not assumed). A pick-up leaves **no rank rows** for the base, so the ownership of a backup is `base_backups.player_id` alone. [V] |
| **A file that hangs rows off the receiving player** (an inventory of arbitrary items on the character) | Every reference column must resolve to the right kind of entry inside the file; the placeholder may be referenced only by the backup record and a rank (`validateBaseBackupFile`, 800-886) | Port this check whole. It is the core of the "untrusted file" rule |
| **References inside JSON** (`!!act#N`, `!!itm#N`) | Rewritten in all JSON columns; `#0` stays null; an id not in the file is an error unless the path is on a short allowed list | The same walk over the four JSON columns. **Open:** the real list of allowed outside references is not readable; the data shows one case that matters, `InstigatorActorId` = the character's pawn in a crafting queue. Plan: show every outside reference in the preview, set it to the null reference `!!act#0`, and ask the operator to test a station with a queued job ([T]) |
| **Partitions and map** | `partition_id` set to null; the game places by the redeploying player's map | No partition column. The game redeploys only on the map it was saved on (`base-backups.md:303-307`, read from the totem actor's `map`). The Deep Desert layout can differ between saves [T] |
| **Backed-up vs placed state** | `state` forced to `BaseBackup`; `owner_id` of buildings cleared | `state` = 5 on every linked actor [V]; `buildings.owner_id` stays empty. A live base is exported as if picked up |
| **Land claim, Sub-Fief claim console** | Base backup keeps the totem and the claim segments; a Solido never carries a claim console | Same split. A base-backup file keeps the totem (the claim is the point); a Solido file never carries `Totem_Small_Placeable` / `Totem_Placeable` |
| **Tax invoices, sinkcharts** | Exported and imported | Tables absent; a file from the editor has none; a file with those kinds is refused |
| **Items in storage** | Carried with the inventories; stored Solidos travel with their blueprint rows | Items carry all columns (`position_index`, `stats`, `volume_override`, `is_new`, `acquisition_time`). A storage that holds a Solido item has no blueprint rows to go with it in any copy; refuse that case until the Solido phase exists |
| **Sticky data in JSON stats** | `HAS_TAGGED_ID` limits the rewrite to rows that hold a tagged id (45) | Same, for speed; the totem's own `m_SelfUniqueID` and a station's `ItemUniqueId` list are the real cases |
| **Exact numbers** | Entries never pass through JavaScript numbers (Postgres renders the file) | Go decodes the file with integer types; the page sends the file text to the server and gets back only counts and strings |
| **Game version** | The file carries the game build, Steam build id, a patches checksum and the latest patch names; a mismatch is a 409 the user can override (902-916, `upgradeEnvelope` 788) | The save has `applied_patches` (28 rows, e.g. `DA_25872_CleanupOrphanedFglEntities`); the file carries a checksum of the sorted names, their count and the latest names. The game build is not in the save, so it is not claimed |
| **What makes an import fail or corrupt the save** | Not listed in one place by the console | See below |

What can make an imported base fail to load or damage the save (derived from the schema and the console's rules):

1. An **entity row with no actor link**: the game's own patch `DA_25872_CleanupOrphanedFglEntities` deletes orphans. Every entity must be linked to an imported actor, and every `owner_entity_id` must point at one of them (the column is `ON DELETE SET NULL`, so a miss silently makes a piece ownerless).
2. **A duplicate key**: `(building_id, instance_id)`, `(actor_id, entity_id)`, `permission_actor_rank (permission_actor_id, player_id)`, `landclaim_segments`. Foreign keys are off in the editor's connection by design, so the editor checks.
3. **An id that the game's own counter will hand out again**: the item sequencer must end above the largest item id written.
4. **A stale reference inside a JSON blob** (a station that still names an item or actor of the other save).
5. **A wrong `state`**: a base left at 0 would appear in the world at its old coordinates, possibly on top of a base the target already has.
6. **STRICT table typing and BLOB jsonb**: a REAL written to an INTEGER column, or JSON text written to a BLOB column instead of `jsonb(?)`, makes the row unreadable. The editor already gates every written JSON cell (`internal/save/jsoncheck.go`) and compares `foreign_key_check` before and after an edit (`internal/save/mutate.go:21`).
7. **Game-version drift** in the component JSON (the shape of an entity's components changes with patches), which only the version check can warn about.

## 5. The editor's rules, and the care package as the pattern

The care package already has the shape this feature needs, in code. The base file should follow it:

| Need | The care package does (`origin/main`) | Base file |
|---|---|---|
| A named, versioned file | `format "tabr-tau-kits"`, `version 1` (`internal/kits/kits.go:274`), refused if another format or version | `format "tabr-tau-base"`, `version 1`, plus `kind` (`base-backup`, later `solido`), `exportedAt`, the editor version and the patches summary |
| Size cap | `MaxFileBytes` 64 KiB (`kits.go:33`) | A larger cap, because a base is megabytes (the console caps the upload at 32 MiB, `server.js:7487`; a base of 1,400 pieces is far below that). Propose 16 MiB, and an entry cap (the console uses 250,000, `baseBackups.js:40`) |
| Strict parse | `DisallowUnknownFields`, one JSON object, nothing after it (`kits.go:315`) | Same, per kind and per column: an allowlist of table and column names, **no SQL, table or column name ever comes from the file** |
| Whole-file validation before anything is kept | `Parse` refuses the file whole (`kits.go:310`) | Port `validateBaseBackupFile`: allowed kinds, ids unique, one backup record, one totem, every reference inside the file at the right kind, the owner placeholder reachable only from the backup record, every actor linked to the backup, string and JSON size and depth caps, finite numbers |
| Import never replaces | `Merge` keeps an imported kit under another name (`kits.go:362`) | An import only **adds** rows, with new ids; it never touches an existing base. Not destructive, so no confirmation |
| Dry run | `ApplyKit(k, dry)` runs the real edit and ends it with a sentinel error so it is rolled back (`internal/ops/kit.go:18-19, 117`); `/api/kits/preview` returns the numbers | Preview = the real import rolled back: counts per table, the new ids, every outside reference and what it becomes, version mismatch, templates the catalog does not know |
| What was previewed is what is applied | The preview returns `kitHash`; apply refuses if the kit differs (`internal/web/kits.go:134-158`) | A hash of the file text |
| File reaches the server as text from the page | `/api/kits/import` takes `text` (`internal/web/kits.go:173-189`); the page reads the file (`static/app.js:699`) | Same: no file path ever reaches the server, so there are no path tricks on import; the export is a download whose name is built from a sanitized base name and a fixed extension |
| One edit per action, write gate, backup | `Mutate` (`internal/save/mutate.go`), written after a backup, queued while the game may have the save loaded | One `Mutate` per import: "Import base backup 'X': N pieces, M placeables, K items". Writes only with the game closed (the existing gate); the backup is the existing one |
| No confirmation | `ApplyKit` has a preview and an Apply button, no dialog | The same. Typed confirmation is for what destroys or resets (reset buttons, restore, SQL write; README "Destructive and bulk actions ask first"). Deleting a backup would be destructive and gets the typed word, but it is not part of the first phases |
| No new orphans | `fkCounts` compares `foreign_key_check` before and after (`mutate.go:21`) | The import must add none |

A mismatching patch set shows in the preview and changes the Apply button to "Import anyway"; it does not open a dialog.

Where it lives in the UI: Player > Bases (the console puts "Base Backups" at the bottom of the Bases tab; `baseview.go:83` already labels picked-up bases), and Player > Blueprints (today a "not built" notice, #111) for Solidos.

## 6. Phased plan

| Phase | What | Size | Needs the game? | What the operator verifies |
|---|---|---|---|---|
| **1** | List the save's base backups and **export one** as a `tabr-tau-base` file. Read only. The collector walks the 44-actor set to its 14 tables, remaps ids to transfer ids, rewrites the JSON references, writes the version summary | M | Only to make a backup: pick a base up in the game once | Open the file; counts match the Bases view; the totem, a storage and a station are in it. Nothing in the save changes |
| **1b** | Export a **placed** base directly as the same file (pick-up rules: the totem, the placeables with buildable support, the pieces the totem owns, state 5, no totem permission rows). Golden test: the export of the placed copy equals the export of its real pick-up (such a pair exists) | S to M | No | None beyond phase 2, which is where it is proven |
| **2** | **Import** a base file into another save, with preview. Validation, id allocation, reference rewrite, one edit, backup first. **Built** (engine and routes, then the page; `docs/import-a-base-file.md`); the in-game UAT items are **open** | L | **Yes**, the in-game checks below | Items 1-10 of the UAT list (open) |
| **3** | Solido: list the save's blueprints, export one as a file (M); import a file (L); delete one (S). Needs a save that has one | M + L + S | **Yes**: a Solido must first be made in the game (#111) | Items 11-13 |
| **4** | Build a Solido **from a placed base**: positions relative to the totem, yaw from the quaternion, claim consoles dropped, holograms skipped. The file is then imported with phase 3 | M | Yes, through phase 3 | Item 14 |
| 5 (optional) | Delete a base backup (typed word, as the console's "DELETE BACKUP" and its prior backup); read a console `dune-base-backup` file | S; unknown | Yes | Only if a real console file is supplied |

Reasoning on the order: phases 1 and 1b are read only and can be proven against the rows that exist; phase 2 has evidence (the pick-up, and the vehicle copy the game already accepted:
`DuplicateVehicle` was loaded and flown in game on 2026-10-27, `vehicle_duplicate.go:24`); phases 3 and 4 depend on a Solido that no save has.

Items the build must also do: add the THIRD-PARTY-NOTICES section; add a base round trip to the test suite (export from a real-schema save, import into another, export again, equal after ids are
renumbered); keep the file text out of the browser's number parser.

## 7. UAT list: what only the game can answer

1. After an import into a save, does the **Base Reconstruction Tool list the backup** (the console needed a log out and in; the editor writes with the game closed)? [T]
2. **Does redeploy work** on Hagga Basin: every piece and placeable present, storage contents present, generators and water as exported, the claim established, the character the owner? [T]
3. **A crafting station with a queued job** after import (its `InstigatorActorId` set to the null reference, its allocated item ids remapped): does it load, and does the queue continue or drop? [T]
4. **Entity ids written by the editor** (prefix kept, counter above the target's): does the game accept them across a save and reload? (The vehicle copy says yes for vehicles.) [T]
5. **A Deep Desert base** into a save whose Deep Desert layout differs. [T]
6. **A file from an older patch set**: does the game load the components? [T]
7. A base exported **while placed** (phase 1b) against one exported after a real pick-up: does the game treat both the same on redeploy? [T]
8. A base with **hologram placeables**: keep or drop? [T]
9. The **same file imported twice**: do both backups list, and can both be redeployed? [T]
10. After one session, are the imported rows still there and unchanged (no game cleanup patch removed any)? [T]
11. Solido: what do the item stats and the four tables look like for a blueprint the game made, in particular `building_blueprint_map` and `hologram` / `health`? [T, blocked on a save]
12. Solido: does an **editor-written Solido preview and place** without the console's `P34`-style crash? [T]
13. Solido: does the item need to be in the backpack at a particular slot, and does a blueprint with its player id set to the pawn behave? [T]
14. A Solido built from a placed base: placed with the right orientation and offset, and without the claim console? [T]
15. Does `totems.last_backup_timestamp` (non-zero in one real totem, zero in the others) matter to the tool? [T]

## Notes

- Nothing in this page needs a rebuild of the catalog: `BaseBackupTool` ("Base Reconstruction Tool") and `BuildingBlueprint_CopyDevice` ("Solido Replicator") are already in `internal/catalog/items.json`.
- The editor's current handling of picked-up bases is the label in the Bases list (`internal/ops/baseview.go:83`); #403 remains the issue for the lifecycle state, and the mapping above settles its step 1.
- Related issues: #111 (Blueprints, parked), #403 (lifecycle state), #94 (Bases view, closed), #56 (full repair).

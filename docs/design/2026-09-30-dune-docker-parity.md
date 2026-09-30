# tabr-tau vs dune-docker: parity analysis

Status: DRAFT (2026-09-30). Evidence-based; nothing here is implemented by this document.

## 1. How single-player actually runs (verified from client logs)

Two client sessions with no online play (`DuneSandbox-backup-2026.09.29-23.37.40.log`,
`...-2026.09.30-00.14.10.log`) show:

- The map is hosted inside the game process: `Browse ... Survival_1?listen`, `net mode Listen Server`.
- `LogCorePersistence: Using persistence implementation Sqlite` (the server uses Postgres).
- `LogFarmNotificationsSqlite` listens for `world_partition_update`, `guild_notify_channel`,
  `landsraad_notify_channel`, `permission_notify_channel`. These are the SQLite counterparts of the
  Postgres LISTEN/NOTIFY channels.
- Zero `LogRmq` lines. Online sessions in the same log set produce RMQ traffic
  (`Getting GAME RMQ connection info`, `Sending RMQ login`, queue declare/delete).

Conclusion: single-player has no RMQ, no gateway, no battlegroup director. Items are given
by writing to the save database. There is no message bus to reproduce.

Caveat: this comes from log evidence only. Game code was not inspected.

## 2. Save format

`FLS_retail\<steamid>\game.db` = 8-byte header (`uint32 1`, `uint32 size`) + zlib stream of a
SQLite 3 database. Sample decoded to 1,765,376 bytes, 96 tables. Also present:
`game_prepatch.db`, `autosave/0..9.bak`, `SOLO/` (per-dimension client files such as
`Level.json`, `WornItems.json`, `FactionId.json`), `ClientPersistence/*.data`.

`Saved\Cloud\PlayerClientStorage\FLS_retail\<steamid>\` also holds per-battlegroup client caches
(`sh-...`, `xrealm-...`, `prd-...`) from online play. Those are client-side caches of online
characters, not editable saves.

## 3. Schema diff: SQLite save vs Postgres `dune` schema (dune-dev, read-only)

Postgres `dune` schema: 167 tables. SQLite save: 96 tables (80 shared).

- 63 of the 80 shared tables have identical column sets.
- 17 differ. Most differences are Postgres composite columns split into scalars in SQLite:
  `transform` -> `location_x/y/z` + `rotation_w/x/y/z` (`actors`, `building_instances`,
  `travel_return_info`), `overmap_location` -> `overmap_location_x/y/z`, `markers.position` ->
  `x/y/z`, `landclaim_original_global_location` -> `_x/_y/_z`.
- Real semantic differences worth attention:
  - `actors`: no `partition_id` in SQLite (single local world).
  - `player_state`: SQLite has `death_map`, `death_dimension`; Postgres has `transfer_count`,
    `last_character_state_change`.
  - `inventories`: Postgres has `exchange_id`; SQLite does not.
  - `markers` / `player_markers`: SQLite stores `map_name` text; Postgres stores `map_name_id`.
  - `landsraad_decree_term`: SQLite-only `decree_reroll_attempts`.
  - `farm_variables`: Postgres-only `battlegroup_close_date`.
- SQLite-only tables (local substitutes for server services): `items_id_sequencer`, `checkpoints`,
  `coriolis_cycle`, `sandstorm_data`, `sandstorm_schedule`, `resource_nodes`,
  `loot_container_items`, `loot_container_timestamps`, `landsraad_simulated_guilds`
  (single-player has simulated guilds), `communinet_player*`,
  `building_progression_learned_building_sets`, `building_progression_new_buildable_pieces`,
  `totem_fuel_update_times`.
- Postgres-only (87): multiplayer/server infrastructure. Notably the whole Dune Exchange
  (`dune_exchange_*`), guilds/parties, `event_log*`, `item_audit_log`, `actor_audit`, world
  reset seeds, `discord_*`, `console_player_playtime`, `cheater_tracking`.

Machine-readable diff: regenerate with the queries in section 6.

## 4. Give-item: how dune-docker does it and what applies here

dune-docker has two paths:

1. Live game path: `runner.js` `adminGiveItem` / `adminGiveItems` / `adminGiveItemId` ->
   `dune admin grant-item ...` -> `admin-tools.sh` publishes `ServerCommand: AddItemToInventory`
   to the RMQ `heartbeats` exchange (verified: `publish_inner_json` calls `rabbitmqctl eval` in
   the `dune-rmq-game` container). Not applicable: there is no server.
2. Direct DB path: `duneDb.js` `giveItemToPlayer`, `giveItemToStorage`,
   `giveItemToBaseContainer`. Inserts `dune.items` + `dune.inventories` rows after a capability
   check on the columns.

Path 2 applies. All columns its capability checks require exist in the SQLite save:
`inventories(id, actor_id, inventory_type, max_item_count, max_item_volume)` and
`items(inventory_id, template_id, stack_size, quality_level, position_index, stats, volume_override)`.
Differences: SQLite uses `items_id_sequencer` (single `next_id` row) instead of a Postgres
sequence. `tabr-tau` `internal/ops/ops.go` already allocates from it.

Open question: dune-docker documents `INC-2026-07-31-FILL-ITEMS-VISIBLE-ONLY-AFTER-RESTART` for
storage inserts. Whether rows written to the save appear without a restart is only ever true in
single-player because the game is closed while we write. Needs a live test.

## 5. What dune-docker does over RMQ (does NOT exist in single-player)

`runtime/scripts/admin-tools.sh` publishes JSON to the game's RabbitMQ `heartbeats` exchange
(`publish_inner_json` / `publish_player_command`). The running game server executes it. The
complete set of live game commands dune-docker sends:

`AddItemToInventory` (grant-item, grant-template), `KickPlayer`, `ServiceBroadcast`,
`AwardXP`, `SkillsSetUnspentSkillPoints`, `SkillsSetModuleLevel`, `UpdateAllWaterFillables`,
`CleanPlayerInventory`, `ResetProgression`, `TeleportTo`, `SpawnVehicleAt`.

Single-player has no RMQ, so none of these can be sent. Each needs either a direct save edit or
is unavailable:

| Live command | Single-player replacement |
|---|---|
| `AddItemToInventory` | direct `items`/`inventories` insert (dune-docker already has this as `giveItemToPlayer`); tabr-tau does this |
| `TeleportTo`, `SpawnVehicleAt` | direct `actors.location_*` edit (tabr-tau `Teleport`, `BringVehicle`) |
| `CleanPlayerInventory` | delete `items` rows for the player's inventories |
| `KickPlayer`, `ServiceBroadcast` | not applicable (no other players / no service) |
| `AwardXP`, `SkillsSetUnspentSkillPoints`, `SkillsSetModuleLevel`, `UpdateAllWaterFillables`, `ResetProgression` | **unresolved.** No column in the SQLite save holds character XP, unspent skill points, module level or water. `player_state` has none. This state is most likely inside `fgl_entities.components` (opaque blobs, 63 rows, 345-803 bytes) or `ClientPersistence/*.data`. Treat as read-only/unavailable until a blob decoder exists. `specialization_tracks(player_id, track_type, xp_amount, level)` IS a real table but is empty in the sample save |

## 6. Feature table: what works in single-player

Method: for every function in `console/api/src/duneDb.js` (558 functions; 135 touch tables), extract
the tables named after `dune.`, `from`, `join`, `into`, `update`, `delete from`, `exists`, and
check each against the SQLite save. Regex-based static analysis: it can miss dynamically built SQL,
and it checks table presence and column-set equality only, not query semantics. Reproduce with
section 7. Classes:

- **WORKS** = every table it uses exists with an identical column set.
- **ADAPT** = tables exist but at least one has a differing column set (position/transform split,
  `inventories.exchange_id`, `player_state` columns). Needs SQL changes, not new capability.
- **BLOCKED** = uses a table that does not exist in the save.
- **N/A** = concept does not exist in single-player.

Results: 97 functions fully present (28 WORKS, ~60 ADAPT), 38 BLOCKED (32 exported).

### WORKS (identical schema)
Currency (`addCurrency`, `playerCurrency`), factions (`playerFactions`, `setPlayerFaction`),
specializations (`playerSpecs`, `addSpecializationXp`, `grantMaxSpecialization`,
`grantAllSpecializationKeystones`, `resetSpecialization`, `resetAllSpecializationKeystones`),
tutorials (`completeTutorial`, `resetTutorial`), base inventory / container slots / land claim /
water refill (`baseInventory`, `baseContainerSlots`, `getBaseLandClaim`, `updateBaseLandClaim`,
`refillBaseWater`, `generatorUptimePolicy`), Landsraad edits (`updateLandsraadTaskGoal`,
`updateLandsraadTermTaskGoals`, `updateLandsraadRewardTier`, `setLandsraadPlayerContribution`,
`applyLandsraadMilestonePreset`), `listSpicefieldTypes`, `playerItemAugmentState`,
`adminVehicleMetadata`, `basePermissionActor`.

### ADAPT (table exists, columns differ)
| Feature | Why it needs changes |
|---|---|
| Give item to player / base container / storage; fill; give multiple | `inventories` differs (Postgres `exchange_id`); IDs from `items_id_sequencer`; `volume_override` rule applies |
| Update / delete / augment / repair inventory items; remove from storage | `inventories` |
| Player list, profile, vitals, position, progression, intel, recipes, research | `actors` (position split to `location_*`/`rotation_*`), `player_state` |
| Teleport player, live-map players/bases/vehicles/storage | `actors.transform` -> `location_x/y/z` + quaternion |
| Vehicles: list, storage, refuel, repair decay, delete | `actors`, `inventories`; the sample save has 0 vehicles, so untestable here |
| Add faction reputation / repair reputation | `actors`; `player_faction_reputation` is empty in the sample |
| Base container add/delete | `building_instances`, `inventories` |
| Landsraad overview | `landsraad_decree_term` has SQLite-only `decree_reroll_attempts` |
| Generators refill | `inventories` |
| Building display names | `markers` stores `map_name` text, not `map_name_id` |
| Solari total | `inventories` |

### BLOCKED (missing tables) and N/A
| Feature | Missing | Class |
|---|---|---|
| Guilds: list, members, add/remove/promote/demote, disband, real faction via guild | `guilds`, `guild_members` | N/A (single-player has `landsraad_simulated_guilds` only) |
| Online players, live-map partitions/services/POIs/spice/flour fields, combat partitions, refill queue | `world_partition`, `active_server_ids`, `farm_state` | N/A: single local world |
| Base/vehicle permissions list & set, permission candidates, transfer to system custodian, delete base completely | `map_names` (also uses `world_partition`) | ADAPT-candidate: `permission_actor*` exist (16 rows); `map_names` only resolves a map name, SQLite stores text. Re-check per function |
| Flush base child access | `world_partition` | N/A |
| Item audit log, cheater tracking, player playtime | `item_audit_log`, `cheater_tracking`, `console_player_playtime` | N/A (console-added, not game tables) |
| Dune Exchange (market seed/buyback/listings/stats) | `dune_exchange_*` | N/A |
| Discord links, care-package scheduling, IAM, API keys | (console-owned, not game tables) | N/A |
| Kick, broadcast, map chat, MOTD, restart warning | RMQ | N/A |

### Data present in the sample save (what can actually be exercised)
1 account/character, 102 actors, 65 items, 32 inventories, 38 placeables, 1 totem (land claim),
25 Landsraad tasks, 2,221 journey nodes, 399 markers, 41 resource nodes, 16 permission rows.
Empty: vehicles, vendor stock, specialization tracks, reputation, blueprints, base backups,
virtual currency balances, recovered vehicles, landclaim_segments. A test save with a vehicle
and some progression is needed to verify those paths.

## 7. Reproducing the diff

```
ssh dune-dev "dune database sql \"select table_name||'|'||column_name||'|'||data_type||'|'||is_nullable from information_schema.columns where table_schema='dune' order by table_name, ordinal_position\""
```

Decode the save (header + zlib) and compare `pragma table_info` per table. Read-only on both
sides.

## 8. Proposed next steps

1. Live-test give-item into a decoded copy and load in game (single-player only, game closed).
2. Decide the item catalog source (dune-docker `adminCatalog.js` / `console/web/public/images/items`).
3. Locate XP / skill-point / water storage: diff `fgl_entities.components` blobs before and after a known change in game.
4. Add `CHANGELOG.md`, CI (`go vet`, `go test`, shared security scan), and `docs/` conventions to
   satisfy the org requirements.

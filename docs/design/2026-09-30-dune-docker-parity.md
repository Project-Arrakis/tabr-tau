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
   `dune admin grant-item ...` (goes through the running server; transport not traced here).
   Not applicable: there is no server.
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

## 5. Feature matrix (dune-docker area -> single-player)

| Area | dune-docker | tabr-tau today | Notes |
|---|---|---|---|
| Player give/edit/delete items | yes | yes (`GiveItem`, `SetItem`, `DeleteItem`) | port `volume_override` per-unit rule and stack limits |
| Pre-augmented gear (care package) | yes | no | DB path; check `PRE-AUGMENTED-GEAR.md` |
| Item catalog / search | yes (`/api/admin/items/*`) | no | needs template id -> name catalog source |
| Solari, repair gear, teleport | yes | yes | |
| XP, skill points, skill modules | yes (`award-xp`, `skill-points`) | partial (`Specs`) | XP/skill-points location in save unverified |
| Faction reputation, journey, tutorials, tags, recipes | partial | yes | |
| Bases, storage, repair, clear sand | yes | yes | |
| Blueprints import/export | yes | no | `building_blueprint*` tables identical except transform split |
| Base permissions | yes | listed in README | verify `permission_actor*` |
| Vehicles | yes | yes | |
| Landsraad | yes | yes | plus term goals / reward tiers |
| Exchange market seed/buyback | yes | vendor stock only | `dune_exchange_*` absent in SQLite: out of scope |
| Map players/bases/markers | yes | no | positions in `actors.location_*` |
| Guilds, parties, Discord links, audit log, event log | yes | no | multiplayer: out of scope |
| Containers, autoscaler, backups of server DB | yes | n/a | out of scope by definition |

## 6. Reproducing the diff

```
ssh dune-dev "dune database sql \"select table_name||'|'||column_name||'|'||data_type||'|'||is_nullable from information_schema.columns where table_schema='dune' order by table_name, ordinal_position\""
```

Decode the save (header + zlib) and compare `pragma table_info` per table. Read-only on both
sides.

## 7. Proposed next steps

1. Live-test give-item into a decoded copy and load in game (single-player only, game closed).
2. Decide the item catalog source (dune-docker `adminCatalog.js` / `console/web/public/images/items`).
3. Locate XP / skill-point storage in the save.
4. Add `CHANGELOG.md`, CI (`go vet`, `go test`, shared security scan), and `docs/` conventions to
   satisfy the org requirements.

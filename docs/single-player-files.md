# Single-player file reference

Every file the single-player game keeps on a Windows PC, what it contains, whether it is safe to
edit, and how we know. Reviewed 2026-09-30 against one real PC Steam save (about 80 MB, 407 files).
"Verified" means decoded from that sample; "Inferred" means not proven.

Root: `%LOCALAPPDATA%\DuneSandbox\Saved\` (called `Saved\` below). `<steamid>` is the 17-digit Steam ID.

## Summary

| Path | Purpose | Format | Edit? |
|---|---|---|---|
| `Saved\Cloud\PlayerClientStorage\FLS_retail\<steamid>\game.db` | **the save** | 8-byte header + zlib SQLite | yes, via tabr-tau only |
| `...\game_prepatch.db` | copy taken before a game patch | same | read-only |
| `...\autosave\0..9.bak` + `.meta` | ring of 10 save snapshots | same as game.db; `.meta` small binary | read-only (rollback source) |
| `...\SOLO\*` | character-select display + fog | JSON / raw zlib | cosmetic only |
| `...\ClientPersistence\*.data`, `SOLO\ClientPersistence\*.data` | UI state | uint32 length + zlib JSON | no need |
| `...\<battlegroup-id>\*` | client caches for online servers | same kinds of files | ignore for single-player |
| `Saved\game.crash.db` | plain SQLite copy of the save at last crash | raw SQLite | read-only |
| `Saved\Config\Windows\*.ini` | game settings incl. single-player difficulty | INI | some (see below) |
| `Saved\Config\WindowsClient\*.ini` | client UI/gameplay preferences | INI | rarely |
| `Saved\Logs\`, `Saved\Crashes\`, `Saved\ScreenShots\` | diagnostics | text/dmp/png | no |

## The save: `game.db` (Verified)

- **Wrapper:** bytes 0-3 `uint32 LE` = `1` (meaning unconfirmed); bytes 4-7 `uint32 LE` = decompressed
  length; bytes 8+ = a zlib stream (`78 9c`). The length matched the decoded size in every file checked.
  Corroborated independently by a public analysis (themetalvortex.com, 2026-09-21).
- **Payload:** a SQLite 3 database, 96 tables including `sqlite_sequence` and `sqlite_stat1`
  (94 application tables). Sample decodes to 1,765,376 bytes.
- **JSONB columns:** some BLOB columns are SQLite JSONB. `json(col)` decodes them (SQLite >= 3.45).
  Writing them back with `jsonb_set(...)` keeps them valid JSONB (tested on a scratch copy; game
  acceptance not yet tested).
- **Sample contents:** 1 account/character, 102 actors (60 are simulated Landsraad guilds), 65 items,
  32 inventories, 38 placeables, 1 totem, 25 Landsraad tasks, 2,221 journey nodes, 399 markers.

Where things live (Verified from the sample):

| Data | Location |
|---|---|
| Account | `accounts` (`funcom_id`, `platform_id`, `platform_name`) |
| Character identity / status | `player_state` (name, life state, last login, death location) |
| Position | `actors.location_x/y/z`, `rotation_x/y/z/w` (the player actor class is `BP_DunePlayerCharacter_C`) |
| Hydration, spice | `actors.gas_attributes` JSONB: `DuneHydrationAttributeSet.CurrentHydration`, `DuneSpiceAddictionAttributeSet.CurrentSpice` etc. |
| Known recipes, tech knowledge, vehicle state | `actors.properties` JSONB (`CraftingRecipesLibraryActorComponent`, `TechKnowledgePlayerComponent`, ...) |
| Character XP, skill points, per-skill points, health | `fgl_entities.components` JSONB on the entity linked from `actor_fgl_entities` with `slot_name = 'DuneCharacter'`: `FLevelComponent[1].TotalXPEarned`, `TotalSkillPoints`, `UnspentSkillPoints`, `ModuleData[...].SkillPointsSpent`; `FHealthComponent[1].m_CurrentHealth` |
| Items and inventories | `items`, `inventories`, `actor_inventories`; ID counter `items_id_sequencer` |
| Buildings | `buildings`, `building_instances`, `placeables`, `totems`, `permission_actor*` |
| Machine water / fuel / power | JSONB components on machine entities (`FWaterStorageComponent`, `FFuelPoweredPlaceableComponent`, `FPowerCircuitElementComponent`) |
| Factions / reputation | `player_faction`, `player_faction_reputation`, `factions` |
| Landsraad | `landsraad_*` (60 simulated guilds in `landsraad_simulated_guilds`) |
| Journey, tutorials, tags | `journey_story_node`, `tutorial_per_player`, `player_tags` |
| Specializations | `specialization_tracks`, `purchased_specialization_keystones` (empty in the sample) |
| Sandstorm / Coriolis | `sandstorm_data`, `sandstorm_schedule`, `coriolis_cycle` (single-player only tables) |
| Resource / spice fields | `resource_nodes`, `resourcefield_state` |
| Map markers / discovery | `markers`, `player_markers` |

How the game uses it: single-player is an in-process listen server (`Survival_1?listen`) whose
persistence layer is `Sqlite` instead of Postgres. There is no RMQ. Both facts come from client logs.

## Other save-adjacent files

### `game_prepatch.db` (Verified format, purpose Inferred)
Same wrapper and schema as `game.db`; slightly smaller and older. Presumably a backup made when the
game was patched. Do not edit; useful as an older snapshot for `snapdiff`.

### `autosave\0.bak` .. `9.bak` and `0.meta` .. `9.meta` (Verified)
- `.bak`: full `game.db` snapshots, same wrapper. In the sample, slot sizes differ (1.67 MB to 1.75 MB),
  so they are successive states, and `snapdiff 2.bak 9.bak` showed real progression (hydration, 141 new
  building pieces). Slot age order is not simply numeric; use `.meta`/mtime.
- `.meta` (55 bytes): `int32 1`, `int32 -1`, then a length-prefixed string with the map name (for example
  `Hagga Basin South`), followed by timestamps/counters. The remaining fields are **not decoded**.
- Editing: no. Treat as the rollback ring. tabr-tau backs up the live file to `tabr-tau-backups/` before
  every write; it does not touch these.

### `Saved\game.crash.db` (Verified)
A **raw, uncompressed** SQLite file. In the sample it was byte-identical to the decoded `game.db`
(same SHA-1). Presumably written at crash time. Read-only; convenient for opening in any SQLite tool.

## `SOLO\` and client caches

`SOLO\` is the single-player character's client-side folder. Every other `FLS_retail\<steamid>\<id>\`
(names like `sh-...`, `prd-...`, `xrealm-...`) holds the same kinds of files for one online server.
Also under `FLS_retail\<steamid>\`: `LastCharacter.json` (the last battlegroup ID played) and
`ClientPersistence\`. `FLS_beta\` is the same structure for the beta branch (nearly empty in the sample).

| File | Format | Contents |
|---|---|---|
| `Level.json` | text | one number (character level shown on select screen; sample `7`). Not the source of truth |
| `FactionId.json` | text | faction name, `None` in the sample |
| `CurrentDimension`, `MostRecentDimension` | text | dimension index (`0`) |
| `CcAndPbeCheckpoint.json` | text | small integer (`4`) |
| `NpeCheckpointId.json` | text | new-player-experience checkpoint (online caches only, `720`) |
| `WornItems.json` | JSON | equipped armor names, slot flags, customization (cosmetic display) |
| `ClothedAppearance.json` | JSON | character-creator morph and material parameters (12 KB) |
| `LoreObjectHistory.json` | JSON | `m_Identifiers`: hashes of lore objects seen (online caches) |
| `FogOfWar\<Map>_FogOfWarTrail.data` | raw zlib -> 262,144-byte bitmap | map reveal trail per map (`HaggaBasin`, `Arrakeen`, `DeepDesert`, `HarkoVillage`, `WindPass`). 60,568 bits set for `HaggaBasin` in the single-player sample. Cell layout not mapped |
| `ClientPersistence\<hash>.data` | `uint32 LE length` + zlib -> JSON (`DataVersion: 1`) | per-battlegroup last played/location/faction; already-seen popups; closing-server warnings; new-keystone popup flags |

Edit: only cosmetic and popup state. Nothing here changes gameplay; real level, XP and faction are in
`game.db`. Do not edit while the game runs.

Also `Saved\Cloud\CITADEL-*\FogOfWar\` and `Saved\Cloud\<server-id>\FogOfWar\`: fog trails per
server/character; the `CITADEL-...` folders look like earlier or other private-server caches (Inferred).
`Saved\Cloud\steam_autocloud.vdf`: Steam Auto-Cloud marker containing the Steam account ID number.

## Config: `Saved\Config\Windows\*.ini`

Only these have content in the sample; the other 30+ `.ini` files are 2-byte empty placeholders
(engine plugin sections written on first run).

| File | Contents | Notes |
|---|---|---|
| `ServerCustomSettings.ini` | `[/Script/DuneSandbox.UserServerCustomSettings]`: `DifficultyLevel`, `PVPMode`, `GatheringAmount`, `CraftingCost`, `CraftingTimeMultiplier`, `BuildingCostMultiplier`, `ResourceRespawnSpeed`, `LootRespawnSpeed`, `InventoryVolumeMultiplier`, `GlobalXpMultiplier`, `CombatXp`, `GatheringXp`, `MissionXp`, `IntelPointsGainMultiplier`, `ThirstMultiplier`, `HeatBuildupRate`, `ColdBuildupRate`, `DropEquipmentOnDeath`, `bAllowSandstorms`, `bAllowSandworms`, `PlayerDeathLootRule`, ... | **Single-player difficulty.** The sample has custom multipliers (for example `GlobalXpMultiplier=2`, `InventoryVolumeMultiplier=10`), so the game is reading it. Editable; the game rewrites configs on exit, so close the game first |
| `Game.ini` | input preset; `[Settings.Audio/Video/Account]` (`LocalTotalPlayedTimeSeconds`); `[FuncomLiveServices] CachedUsers=` (**account IDs, platform IDs, display names**); first-seen popup flags | **Sensitive.** Never commit or share. Not useful to edit |
| `GameUserSettings.ini` | resolution, scalability groups, vsync, benchmarks | UE video settings. Safe but irrelevant |
| `Engine.ini` | `[Core.System]`, `[GameNetDriver ...]`, `[/Script/Engine.RendererSettings]` | engine tuning. Not needed |
| `Input.ini` | `AimAssistMode`, `SuspensorToggle` | 60 bytes |

`Saved\Config\WindowsClient\*.ini` mirrors these, with a larger `Game.ini` (7.7 KB): `[Settings.Building]`,
`[Settings.Gameplay]`, `[Settings.HUD]`, `[Settings.PrivateServers]`, `[Settings.LastSession]`,
`[Settings.GameVersion]`, `[Settings.PatchNotes]`, `[Settings.Battlegroups]`, `[/Script/DuneSandbox.InventorySystemSettings]`.
UI preferences; the sample shows `LocalTotalPlayedTimeSeconds=3508502` here versus `392039` in `Windows\`,
so the two are not the same counter (meaning unresolved).

## Diagnostics (no editing)

| Path | Contents |
|---|---|
| `Saved\Logs\DuneSandbox.log` (+ `-backup-<timestamp>.log`, up to 10) | full UE client log; the source for "single-player has no RMQ" and "persistence is Sqlite". 4-10 MB each |
| `Saved\Crashes\...` | `CrashContext.runtime-xml`, `UEMinidump.dmp`, `D3D12.*.nv-gpudmp`, crash copy of the log, `CrashReportsJournal.txt` |
| `Saved\Config\CrashReportClient\*\CrashReportClient.ini` | crash reporter state |
| `Saved\ScreenShots\ReportBugScreenshot-*.png` | in-game bug-report screenshots (5 MB each) |
| `Saved\DuneSandbox_PCD3D_SM6.upipelinecache` | shader pipeline cache (1.5 MB) |

Logs and the Game.ini contain platform IDs and display names: scrub before sharing.

## What tabr-tau touches

Reads/writes `game.db` (with backup, integrity check, on-disk-change check, game-running check) and
the `.ini` files listed above (with backup). It must never modify `autosave\`, `game_prepatch.db`,
`game.crash.db`, or online-server caches.

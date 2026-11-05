# Single-player runtime: client-log evidence (redacted)

Extracted 2026-09-30 from the game's own client logs (`Saved\Logs\DuneSandbox*.log`) of one PC Steam install so the
conclusion "single-player runs an in-process listen server on SQLite and uses no RMQ" stays reproducible after the
raw logs are deleted. IDs, hex tokens and IP addresses are replaced with `<id>`, `<hex>`, `<ip>`.

## What was counted per log
`LogRmq` = lines from the RMQ client category. `Sqlite persistence` = lines `LogCorePersistence: Display: Using persistence
implementation Sqlite`. `listen` = lines containing `Survival_1?listen`.

| Kind of log | `LogRmq` | Sqlite persistence | `Survival_1?listen` | NetMode values seen |
|---|---|---|---|---|
| Online-only | present | none | none | Client, Standalone |
| Mixed (single-player and online in one log) | present | present | present | Client, Standalone |
| Pure single-player | none | present | present | Standalone |

Reading:
- **Pure single-player logs** (`Survival_1?listen`, Sqlite persistence, **zero** `LogRmq` lines) exist in the sample.
- **Mixed logs** (single-player and online play in the same log, so both patterns appear): their `LogRmq` lines come from the online part of the
  session; the Sqlite/listen lines come from the single-player part.
- **Online-only logs** (RMQ lines, no Sqlite/listen lines): the remaining rotated logs and the current log.
- `NetMode` is `Client` during online play; single-player shows `Standalone` before travel and `Listen Server` after.
- The claim "single-player uses no RMQ" therefore rests on the pure logs (0 RMQ lines) plus the fact that the RMQ lines in the
  mixed logs occur only in the online portions. That last point was not line-by-line verified here.

## Excerpt: one single-player session (unique lines, in order)
Times and process ids are removed.
```
LogDataMining: Display: "NetMode":"Standalone"
LogCorePersistence: Display: Using persistence implementation Sqlite
LogFarmNotificationsSqlite: Display: Listening for notification 'world_partition_update'
LogFarmNotificationsSqlite: Display: Listening for notification 'guild_notify_channel'
LogNet: Display: Delaying Travel (net mode Standalone): /Game/Dune/Maps/Arrakis/SOC_1/Survival_1?listen?delayed_travel
LogNet: Display: Browse (net mode 0): /Game/Dune/Maps/Arrakis/SOC_1/Survival_1?listen
LogLoad: Log: LoadMap: /Game/Dune/Maps/Arrakis/SOC_1/Survival_1?listen
LogDataMining: Display: "NetMode":"Listen Server"
LogFarmNotificationsSqlite: Display: Listening for notification 'landsraad_notify_channel'
LogFarmNotificationsSqlite: Display: Listening for notification 'permission_notify_channel'
LogNet: Display: Delaying Travel (net mode Listen Server): /Game/Dune/Maps/ChallengeRoom/Levels/StandaloneLevels/ChallengeRoom_Stillsuit_Standalone?delayed_travel
```

## Caveat
This is log evidence only; game code was not inspected. Regenerate: copy the logs from the PC and rerun the counting in this file's method.

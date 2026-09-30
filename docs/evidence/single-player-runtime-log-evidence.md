# Single-player runtime: client-log evidence (redacted)

Extracted 2026-09-30 from the game's own client logs (`Saved\Logs\DuneSandbox*.log`) of one PC Steam install so the
conclusion "single-player runs an in-process listen server on SQLite and uses no RMQ" stays reproducible after the
raw logs are deleted. IDs, hex tokens and IP addresses are replaced with `<id>`, `<hex>`, `<ip>`.

## Per-log counts
`LogRmq` = lines from the RMQ client category. `Sqlite persistence` = lines `LogCorePersistence: Display: Using persistence
implementation Sqlite`. `listen` = lines containing `Survival_1?listen`.

| Log (timestamp of rotation) | Lines | LogRmq | Sqlite persistence | `Survival_1?listen` | NetMode values seen |
|---|---|---|---|---|---|
| 2026.09.29-02.52.09.log | 21583 | 9 | 0 | 0 | Client, Standalone |
| 2026.09.29-03.09.46.log | 2515 | 6 | 0 | 0 | Client, Standalone |
| 2026.09.29-03.34.41.log | 5019 | 3 | 0 | 0 | Client, Standalone |
| 2026.09.29-06.17.30.log | 26784 | 21 | 0 | 0 | Client, Standalone |
| 2026.09.29-17.35.34.log | 20215 | 3 | 0 | 0 | Client, Standalone |
| 2026.09.29-23.04.02.log | 55952 | 9 | 2 | 3 | Client, Standalone |
| 2026.09.29-23.37.40.log | 4460 | 0 | 4 | 6 | Standalone |
| 2026.09.30-00.14.10.log | 3005 | 0 | 2 | 3 | Standalone |
| 2026.09.30-03.26.42.log | 15455 | 6 | 2 | 3 | Client, Standalone |
| 2026.09.30-07.18.32.log | 31845 | 6 | 0 | 0 | Client, Standalone |
| current.log | 3465 | 9 | 0 | 0 | Client, Standalone |

Reading:
- **Pure single-player logs** (`Survival_1?listen`, Sqlite persistence, **zero** `LogRmq` lines): `2026.09.29-23.37.40` and `2026.09.30-00.14.10`.
- **Mixed logs** (single-player and online play in the same log, so both patterns appear): `2026.09.29-23.04.02` and `2026.09.30-03.26.42`.
  Their `LogRmq` lines come from the online part of the session; the Sqlite/listen lines come from the single-player part.
- **Online-only logs** (RMQ lines, no Sqlite/listen lines): the remaining rotated logs and the current log.
- `NetMode` is `Client` during online play; single-player shows `Standalone` before travel and `Listen Server` after.
- The claim "single-player uses no RMQ" therefore rests on the two pure logs (0 RMQ lines across 7,465 lines) plus the fact that the RMQ lines in the
  mixed logs occur only in the online portions. That last point was not line-by-line verified here.

## Excerpt: one single-player session (unique lines, in order)
```
[2026.09.29-23.05.04:857][  0][74088]LogDataMining: Display: "NetMode":"Standalone"
[2026.09.29-23.06.51:544][539][74088]LogCorePersistence: Display: Using persistence implementation Sqlite
[2026.09.29-23.06.51:599][539][74088]LogFarmNotificationsSqlite: Display: Listening for notification 'world_partition_update'
[2026.09.29-23.06.51:599][539][74088]LogFarmNotificationsSqlite: Display: Listening for notification 'guild_notify_channel'
[2026.09.29-23.06.51:605][539][74088]LogNet: Display: Delaying Travel (net mode Standalone): /Game/Dune/Maps/Arrakis/SOC_1/Survival_1?listen?delayed_travel
[2026.09.29-23.06.51:636][541][74088]LogNet: Display: Browse (net mode 0): /Game/Dune/Maps/Arrakis/SOC_1/Survival_1?listen
[2026.09.29-23.06.51:636][541][74088]LogLoad: Log: LoadMap: /Game/Dune/Maps/Arrakis/SOC_1/Survival_1?listen
[2026.09.29-23.06.52:049][541][74088]LogDataMining: Display: "NetMode":"Listen Server"
[2026.09.29-23.06.53:606][700][74088][0]LogFarmNotificationsSqlite: Display: Listening for notification 'landsraad_notify_channel'
[2026.09.29-23.06.53:784][707][74088][0]LogFarmNotificationsSqlite: Display: Listening for notification 'permission_notify_channel'
[2026.09.29-23.18.31:480][975][74088][0]LogNet: Display: Delaying Travel (net mode Listen Server): /Game/Dune/Maps/ChallengeRoom/Levels/StandaloneLevels/ChallengeRoom_Stillsuit_Standalone?delayed_travel
```

## Caveat
This is log evidence only; game code was not inspected. Regenerate: copy the logs from the PC and rerun the counting in this file's method.

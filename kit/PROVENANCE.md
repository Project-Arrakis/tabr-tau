# Where the contents of this module come from

Every file in this module is listed here with where it comes from. The rule for what may enter the module:

- code written for this project, and tables of the game's identifiers and numbers that come from a public source or from the installed game's own content files read without modifying them;
- nothing else: no game artwork, no text or names copied from the game's art or audio, no save, log or snapshot of a person's machine, and no code from another project.

Test data in this module is made by the tests (random bytes after the SQLite header, and containers built in the test), never read from a real save.

| Path | What it is | Source | Added |
|---|---|---|---|
| `doc.go`, `boundary_test.go` | The module's description, and the tests that it imports the standard library only and nothing internal | Written for this project | #547 |
| `savefile/codec.go` | `Decode`, `Encode` and `ReadCapped`: the container around a single-player save (a little-endian flag of 1, a little-endian uncompressed size and a zlib stream whose payload is a SQLite 3 database) | Written for the editor and moved here from `internal/save/codec.go` unchanged except for the package name and the exported names. The container layout is the one the game writes and the editor has read since its first version (`docs/single-player-files.md`); no game file or game code was copied | #547 (moved), earlier as `internal/save/codec.go` |
| `savefile/codec_test.go`, `savefile/readcap_test.go` | Tests of the container: round trip at a real size, a stream that expands far beyond its header, a header over the limit, truncation, a wrong size in either direction, a wrong flag, a payload that is not SQLite, a fuzz target; and the capped read | Written for the editor and moved here; the payloads are made by the tests | #547 (moved) |
| `durable/durable.go` | `WriteFile`, `WriteNew`, `Rename`, `SyncDir` and the options and test hooks: write to a new file in the same folder, fsync, rename over the target with retries, flush the folder (a no-op on Windows) | Written for the editor and gathered here from its three copies of these steps: `internal/save/durable.go` (the exclusive create with fsync, and the rename with a growing pause), `internal/settings/durable.go` (the temp file, the short retries, the folder flush) and the temp-file write of `internal/config/config.go`. No other project's code was copied | #549 |
| `durable/durable_test.go` | Tests of the package: the order of the steps, a crash before the rename, a failed flush, retries and their pause, a rename that never succeeds, a refused overwrite, the temp file name, the folder flush, writers at the same time | Written for this project, modelled on the editor's fault tests of the save and the settings file; the data is made by the tests | #549 |
| `gamedata/fillables/fillables.go`, `gamedata/fillables/fillables_test.go` | `Table`, `Lookup`, `Source`, `Parse` and `Marshal` over the game's table of fillable items, and the tests that pin its row count, its counts per liquid, known rows, its order and the hash of its rows | Written for this project | #551 |
| `gamedata/fillables/item_table_fillables.json` | The game's table of fillable items, `DT_ItemTableFillables`: 58 rows of an item template, the most it holds, the amount a new item starts with, the liquid it takes and whether it is a container. Names and whole numbers only | Read from the installed game's content files on disk, read-only, on 2026-10-25 (game build 1.5.15.x). The editor's own reading of the same table on 2026-10-28 (issues #472 and #482, `docs/design/2026-10-28-fillable-capacities.md`) gave the same 57 values it had typed into its capacity maps; the snapshot has one more row, the consumable `WaterPack_Consumable`. No artwork, no text from the game, no save | #551 |
| `go.mod` | The module path and the Go version. No requirement | Written for this project | #547 |
| `LICENSE` | The MIT licence, the same text as the repository's | Copy of the repository's `LICENSE` | #547 |

When a file is added or a table is refreshed, this table gets a row in the same pull request, with the issue, the source and (for data) the game build the values were read from.

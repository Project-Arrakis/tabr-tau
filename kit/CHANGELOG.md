# Changelog of the kit module

Tags of this module are `kit/vX.Y.Z`. The module is version 0.x: until v1.0.0 an exported name can change in a minor version, and the entry says so. A data change after a game patch is a patch version when only values change and a minor version when a row's meaning or a field changes. A tag is never moved; a bad tag is answered by a new tag and a `retract` line in `go.mod`.

Each tag needs an entry here whose heading is `## vX.Y.Z` followed by the date: the release workflow uses the entry as the release notes and fails without it.

## v0.3.0 (2026-11-08)

New package, backward compatible: nothing in `savefile` or `durable` changed. Refs #551.

- New package `gamedata/fillables`: the game's table of fillable items (`DT_ItemTableFillables`), 58 rows sorted by name. Each row is an item template with the most it holds (`MaxAmount`), the amount a new item starts with (`CurrentAmount`, 0 except for the consumable `WaterPack_Consumable`), the liquid it takes (`fillables.Water` or `fillables.Blood`) and whether it is a container (the worn stillsuits are not). `Table()` returns a copy of the rows, `Lookup(template)` one row and `Source()` where the snapshot came from. `Parse` reads and `Marshal` writes the snapshot's text (rows sorted by name, one space of indent, a final newline), so a refreshed snapshot has one format.
- The data is a snapshot of the table as read from the installed game's content files (build 1.5.15.x); the values are the ones the editor already carried in its capacity maps, and the editor now builds those maps from this table.
- Which rows a program fills, and what it does with the stillsuits, is not decided here.

## v0.2.0 (2026-11-08)

New package, backward compatible: nothing in `savefile` changed. Refs #549.

- New package `durable`: replaces a file so that a crash or a power cut leaves the old one or the new one. `WriteFile(path, data, perm, Options)` writes to a new file in the same folder, flushes it (fsync), closes it, renames it over the target with retries and flushes the folder; `WriteNew` (exclusive create and fsync) and `Rename` (retries only) are its two halves, for a caller that checks something between them; `SyncDir` flushes a folder and does nothing on Windows. `Defaults()` is six tries with a growing pause of 150 ms and the folder flush; the zero `Options{}` is one try, no pause and no folder flush. `Options.Retry` limits the retries to the errors it accepts, `Options.TempPattern` names the temporary file, and `Options.Hooks` replaces single steps (the rename, the file flush, the folder flush, a point right before the rename) so that tests can inject faults.
- The editor's three copies of these steps (the save, the settings file and the game's config files) now call it. The config files gain the fsync, the retries and the folder flush they did not have.
- Backup rotation is not part of the package: the editor's three places that prune backups follow different rules.

## v0.1.0 (2026-11-08)

First version. Refs #547.

- New package `savefile`: `Decode`, `Encode` and `ReadCapped` for the container around a single-player save, and the constant `Limit` (256 MiB), moved from the editor's `internal/save` without a change of behaviour. `ReadCapped` is the editor's `readCapped`, now exported: it refuses a file over `Limit` from its size before any byte is read.
- The module has no third-party dependency; a test checks it.

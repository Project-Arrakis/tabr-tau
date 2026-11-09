# Changelog of the kit module

Tags of this module are `kit/vX.Y.Z`. The module is version 0.x: until v1.0.0 an exported name can change in a minor version, and the entry says so. A data change after a game patch is a patch version when only values change and a minor version when a row's meaning or a field changes. A tag is never moved; a bad tag is answered by a new tag and a `retract` line in `go.mod`.

Each tag needs an entry here whose heading is `## vX.Y.Z` followed by the date: the release workflow uses the entry as the release notes and fails without it.

## v0.8.0 (2026-11-08)

Two new packages, backward compatible: nothing in `savefile`, `durable`, `gamedata/*`, `webguard` or `schema` changed. Refs #565.

- New package `gamefolder`: where the game keeps its files, as rules over strings given by the caller (nothing reads the environment or a process). `SavedDir(localAppData)` (`DuneSandbox\Saved`), `ClientStorageDir`, `ConfigDir`, `LogsDir`, `LogPath` (`Saved\Logs\DuneSandbox.log`), `LogPathFor(savePath)` (the log of a save kept in the standard tree; an error for a save anywhere else), `Saves(saved, service)` (the `game.db` files of the tree in name order; an empty service means every service) and `NewestSave(saved)` (the newest `FLS_retail\<id>\game.db` by modification time; of two with the same time the first in name order). The error of `NewestSave` and `LogPathFor` are the words the two programs already used.
- New package `sessionlog`: the lines of the game's log that mark a single-player session, and the state they give. `Classify(Policy, line)` says which marker a line is (`Start`, `Menu`, `RemoteBrowse`, `DelayedClose`, `Exit`, `DatabaseSaved` or `NoEvent`), `Body(line)` takes off the logger's bracketed prefixes, `State` (`SPActive`, `Remote`, `Ended`, `Last`) with `State.Apply(Event)` holds what the markers say so far and compares with `==`, `Tracker` adds the log time of the end of a session and of the last write of the database (`Feed(Policy, line, now)`; a line with no time stamp counts as `now()`, `time.Now` when nil), and `LogTime(line)` reads the stamp. No file, no process, no clock of its own.
  - The two programs that read this log were compared line by line first. They agree on every marker's text, on the order in which a line that holds two markers is read, and on every transition. They differ in two rules, which are the `Policy`: `Strict` (the browse marker counts only at the start of the line's text, after the logger's bracketed prefixes and the category `LogNet: Display: `; the other end markers count anywhere in the line either way) and `IgnoreSaves` (the database write line is not a marker). The zero `Policy{}` is the wide reading with the write line, which is what the editor did.
  - Not here: how long a program waits after a write or an end, what it refuses, and the follower that reads a growing log file (the two programs' followers differ in what they compare, how much they hold and how long a line may be, and each stays with its owner).
- The editor's tracker is `sessionlog.Tracker` under `Policy{}`, `logPathFor` is `gamefolder.LogPathFor`, and `Discover`, the game's own-save tree, the config folder and the file dialog's start folder take their paths from `gamefolder`. The editor's tracker tests pass with only their field names changed, a pin written from the old code (60000 seeded lines) passes, and the old and the new tracker agreed on 60000 seeded lines before the old one was deleted.

## v0.7.0 (2026-11-08)

New package, backward compatible: nothing in `savefile`, `durable`, `gamedata/*` or `webguard` changed. Refs #563.

- New package `schema`: the shape of a single-player save for a program that reads or writes one with SQL. Text and numbers only: no SQLite driver, no row of any save.
  - `DDL()`: the schema of a real save as the game wrote it (94 tables, their indexes and the game's one trigger), as one script that runs into an empty database. The file moved from the editor's `internal/save/schema.sql` without a change (the same git blob); a test pins its hash, its table count and that it holds no row.
  - Names of the tables and columns that programs' SQL touches, one variable per table (`schema.Actors.State`, `schema.VehicleModules.Stats`, `schema.PermissionActorRank.PlayerID`); a test checks every one against the DDL.
  - The JSON paths of the durability record of a vehicle module and of an item and of the fillable record (`VehicleModuleDurability`, `ItemDurability`, `Fillable` with their `Key` forms and whole-field paths), the actor states of a stored vehicle (`StateVehicleBackup`, `StateVehicleRecovery`, `StoredStates`) and the inventory types of a character's own gear (`GearInventoryTypes`).
- Not here: a program's rules over those names. Which vehicles a character owns and which vehicles are stored are written differently by the two programs that use the module, so each keeps its SQL; a test-save builder needs a SQLite driver and is not part of this module.
- The editor now embeds nothing itself: `save.KnownSchema` is `schema.DDL()`, and its SQL takes the shared paths and states from here.

## v0.6.0 (2026-11-08)

New package, backward compatible: nothing in `savefile`, `durable`, `gamedata/fillables`, `gamedata/vehicles` or `gamedata/regions` changed. Refs #561.

- New package `webguard`: the small rules a program that serves its page only to this computer puts in front of its routes. Mechanism only; the policy stays in the program.
  - `Host(hostport)`: a host or host:port names this computer (`localhost` in any case, or any loopback address, with or without brackets and port; nothing is resolved). `Peer(remote)`: the connection comes from a loopback address (the address must carry a port).
  - `AllowedNavigation(origin, raw)` and `OriginOf(raw)`: where a window with no address bar may go (the program's own origin, an in-page blob of it, the blank page); refuses user info, backslashes, spaces and control characters, and any scheme but http and https. Moved from the editor's `internal/gui` without a change.
  - `SecurityHeaders(w, Policy)`: the Content-Security-Policy (`DefaultCSP`, which shows and allows nothing, when the `Policy` names none), `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` and `Cross-Origin-Resource-Policy: same-origin` always; `X-Frame-Options` and `Cache-Control` when the `Policy` names them.
  - `SameOrigin(r, OriginPolicy)`: Fetch Metadata and `Origin` of a request that changes something. `Sec-Fetch-Site` must be empty or `same-origin`; an `Origin` must be an http address whose host and port are the request's `Host`. `OriginPolicy{AllowFetchSiteNone, FoldHostCase}` is where two programs differ; the zero value is the strictest. `JSONRequest(r)`: the content type is `application/json` by its parsed media type.
- Not here: `Content-Security-Policy` text, accepted Host names beyond `Host`, tokens and cookies, routes, request size limits (a one-line `http.MaxBytesReader` in the standard library) and the JSON answer writer (its content type and its failure answer are the program's).
- The editor's request guard and window now call this package; its own tests pin the guard input by input and pass unchanged.

## v0.5.0 (2026-11-08)

New package, backward compatible: nothing in `savefile`, `durable`, `gamedata/fillables` or `gamedata/vehicles` changed. Refs #559.

- New package `gamedata/regions`: the names of the 12 areas of the Hagga Basin by the area number that a save's map markers carry (`markers.area_id`). `Name(map, id)` (an empty string for area 0, an id with no name or a map with no names), `Names(map)` (a copy; nil for a map with no names), `Maps()`, and `Parse` for the table's text (strict: whole-number ids, no empty name, no empty map). The table, `hagga-regions.json`, moved from the editor's `internal/ops` without a change.
- The data is a copy of a data file of an MIT-licensed project (the Dune Docker console), so `NOTICE` now carries that project's copyright and licence text and `PROVENANCE.md` has a row for it. The module still has no third-party dependency and no third-party code.
- Which place gets which region (the editor takes the area of the nearest marker that has a named area) is not decided here.

## v0.4.0 (2026-11-08)

New package, backward compatible: nothing in `savefile`, `durable` or `gamedata/fillables` changed. Refs #555.

- New package `gamedata/vehicles`: the game's vehicle tables for the seven classes that appear in saves (7 classes, 398 module rows, 24 templates), moved from the editor's `internal/catalog` without a change of the data. Types `Data`, `Class`, `Slot`, `Template`, `Module` and `Derived`; lookups `Build`, `Read`, `ClassKeys`, `ClassFor`, `ModuleFor`, `ModulesFor`, `ResolveModule`, `BodyUtilitySeen` and `ModuleBase`; `Derive` (health, critical health, a full tank and the cargo hold of a vehicle from its module ids) and `Check` (the catalog's consistency rules).
- New for programs that need only the fuel tanks: `ChassisFuel(blueprint, moduleID)` (the tank of a Chassis row, when it belongs to that vehicle) and `ChassisTanks()` (a copy of blueprint -> chassis id -> tank: 6 vehicles, 23 chassis, grades 0 to 6; the Container vehicle has none).
- `Parse` reads the snapshot's text, strictly (unknown fields refused, then `Check`), and `Marshal` writes it in one canonical form; the snapshot in the module is byte for byte that form.
- The data is a snapshot of the tables as read from the installed game's content files (build 1.5.15.0); the editor's vehicle catalog and its refuel code now read it from here.
- What a program does with a vehicle whose modules list no known chassis, or two, is not decided here. `ModuleName` (the in-game name of a module) needs item names and stays in the editor.

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

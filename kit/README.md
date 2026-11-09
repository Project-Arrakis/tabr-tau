# kit

Small Go packages for reading and writing Dune: Awakening client save files, kept in the repository of the save editor [tabr-tau](https://github.com/Project-Arrakis/tabr-tau) and published as a separate Go module so that other programs can use them:

```
go get github.com/Project-Arrakis/tabr-tau/kit@latest
```

**Unstable until v1.** The module is version 0.x. An exported name can change in a minor version; the [CHANGELOG](CHANGELOG.md) says when. Pin a version.

## Packages

| Package | What it does |
|---|---|
| `savefile` | Reads and writes the container around a save: `Decode` (container to SQLite bytes), `Encode` (SQLite bytes to container) and `ReadCapped` (reads a file without ever holding more than `Limit`, 256 MiB, of it). Every input is treated as untrusted. |
| `durable` | Replaces a file so that a crash or a power cut leaves the old one or the new one, never part of either: `WriteFile` (temp file in the same folder, fsync, rename with retries, folder flush), its two halves `WriteNew` and `Rename` for a caller that checks something between the flush and the rename, and `SyncDir`. Test hooks for every step. Since v0.2.0. |
| `gamedata/fillables` | The game's table of fillable items (`DT_ItemTableFillables`): for each item template the most it holds, the amount a new item starts with, the liquid it takes (`fillables.Water`, `fillables.Blood`) and whether it is a container. `Table()`, `Lookup(template)`, `Source()`, and `Parse` and `Marshal` for the snapshot's text. Since v0.3.0. |
| `gamedata/vehicles` | The game's vehicle tables for the seven vehicle classes that appear in saves: each class's actor path and row shape, slots and named templates, and one row per module (slot, kind, grade, durability, health, fuel tank, cargo). `ClassFor`, `ModuleFor`, `ModulesFor`, `ResolveModule`, `Derive` (the numbers a vehicle's rows hold, from its module ids), `Check`, `ChassisFuel` and `ChassisTanks` (the 23 chassis tanks), and `Parse` and `Marshal` for the snapshot's text. Since v0.4.0. |
| `gamedata/regions` | The names of the 12 areas of the Hagga Basin by the area number that a save's map markers carry (`markers.area_id`): `Name(map, id)`, `Names(map)`, `Maps()`, and `Parse` for the table's text. Area 0 and an id with no name give no name. Since v0.5.0. |
| `webguard` | The rules a program that serves its page only to this computer puts in front of its routes: `Host` and `Peer` (is this address this computer), `AllowedNavigation` and `OriginOf` (where a window with no address bar may go), `SecurityHeaders` with a `Policy` (nosniff, no referrer, same-origin resource policy, and the program's own Content-Security-Policy), `SameOrigin` with an `OriginPolicy` (Fetch Metadata and `Origin` of a request that changes something) and `JSONRequest` (the parsed content type). Mechanism only: the policy, the token and the routes stay in the program. Since v0.6.0. |
| `schema` | The shape of a single-player save for a program that reads it with SQL, as text and numbers only (no SQLite driver, no row of any save): `DDL()` (the 94-table schema of a real save, as the game wrote it), the names of the tables and columns that programs' SQL touches (`schema.Actors.State`, `schema.VehicleModules.Stats`), the JSON paths of the durability and fillable records (`schema.VehicleModuleCurrentDurability`, `schema.FillableCurrentAmount`) and the actor states of a stored vehicle (`schema.StoredStates`). A program's own rules (which vehicles a character owns, which are stored) stay in the program. Since v0.7.0. |
| `gamefolder` | Where the game keeps its files, as rules over strings given by the caller: `SavedDir(localAppData)`, `ClientStorageDir`, `ConfigDir`, `LogsDir`, `LogPath`, `LogPathFor(savePath)` (the log that belongs to a save in the standard tree), `Saves(saved, service)` and `NewestSave(saved)` (the newest `FLS_retail\<id>\game.db`). It reads no environment variable and opens no process. Since v0.8.0. |
| `sessionlog` | The lines of the game's log that mark a single-player session, as a state machine over lines of text: `Classify(Policy, line)` (a local `?listen` browse starts a session; a `?closed` browse, a remote target, a delayed travel `?closed` and an exit end it; the game's database write line), `State` with `Apply`, `Tracker` (the log time of the end and of the last write), `Body` and `LogTime`. `Policy` is where two programs differ (`Strict`, `IgnoreSaves`). No file, no process; what a program does with the answer stays in the program. Since v0.8.0. |
| `ddgrid` | The lettered grid the game draws over the Deep Desert (A1 to I9): `Cell(x, y)` names the square of a world position (empty outside the grid or for a number that is not finite), and the frame is exported (`N`, `MinX`, `MaxX`, `MaxY`, `MinY`, `CellW`, `CellH`). Nine columns 269,650 wide and nine rows 269,217 high: not the equal parts of a map picture's rectangle. Since v0.9.0. |

```go
blob, err := savefile.ReadCapped(path) // refuses an oversized file from its size
if err != nil {
	return err
}
db, err := savefile.Decode(blob) // the SQLite database inside; a plain SQLite file passes through
```

```go
row, ok := fillables.Lookup("Literjon_T6") // {MaxAmount: 20000, Restriction: "Water", Container: true}
fuel, ok := vehicles.ChassisFuel("BP_Buggy_CHOAM_C", "BuggyChassis_6") // 300, true
area := regions.Name("HaggaBasin", 3) // "Hagga Rift"
```

```go
// in front of a route that changes something
webguard.SecurityHeaders(w, webguard.Policy{ContentSecurityPolicy: csp, CacheControl: "no-store"})
if !webguard.Host(r.Host) || !webguard.Peer(r.RemoteAddr) || !webguard.SameOrigin(r, webguard.OriginPolicy{}) || !webguard.JSONRequest(r) {
	http.Error(w, "refused", http.StatusForbidden)
	return
}
```

```go
// the new bytes are flushed to the disk before they replace the file; the rename is tried six times while another program holds the file
err := durable.WriteFile(path, data, 0o600, durable.Defaults())
```

`durable.Defaults()` is six tries of the rename with a growing pause (150, 300 ... ms) and a flush of the folder afterwards. The flush of the folder is a no-op on Windows, where Go cannot open a folder for it and NTFS journals the rename itself. The zero `durable.Options{}` is one try, no pause, no folder flush. The package does not read the file back and does not keep backups: what to compare and what to keep are the caller's decisions.

## Rules of this module

- **Standard library only.** The module has no third-party dependency and never imports an `internal` package of the editor. A test fails if either changes. A program that uses it therefore never has to move its other dependencies to match.
- **Contents are listed in [PROVENANCE.md](PROVENANCE.md).** Code written for this project, tables of the game's identifiers and numbers, and one data file of an MIT-licensed project with its licence in [NOTICE](NOTICE). No game artwork, no saves.
- **Tags.** `kit/vX.Y.Z`, made by the repository owner from `main` after the checks pass. The tag workflow tests the module again and creates a GitHub release from the matching CHANGELOG entry.
- **Licence.** MIT, as the repository ([LICENSE](LICENSE), [NOTICE](NOTICE)). Unofficial: not affiliated with or endorsed by Funcom.

## Working on it

```
cd kit
go vet ./...
go test -race -count=1 ./...
```

The editor's root module (`go.mod` at the top of the repository) requires this module and replaces it with the directory `./kit`, so the editor's own pull requests build and test the code in the tree. `go test ./...` at the top skips this directory (it is a separate module): run the commands above here too. CI does both.

To try a change in another program before it is tagged, put a `go.work` file beside the two checkouts (`use ./your-program ./tabr-tau/kit`). Keep `go.work` and `go.work.sum` out of git (they are in `.gitignore`).

## Making a tag

Only the repository owner tags. From an up-to-date `main` with the checks green, and with an entry `## vX.Y.Z` in [CHANGELOG.md](CHANGELOG.md):

```
git tag kit/vX.Y.Z && git push origin kit/vX.Y.Z
```

The `kit release` workflow then checks that the tag is on `main`, that the changelog has an entry, and that the tests pass; creates the release; and asks the Go module proxy for the version so that it is available to `go get`.

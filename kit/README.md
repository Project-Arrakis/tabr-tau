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

```go
blob, err := savefile.ReadCapped(path) // refuses an oversized file from its size
if err != nil {
	return err
}
db, err := savefile.Decode(blob) // the SQLite database inside; a plain SQLite file passes through
```

```go
row, ok := fillables.Lookup("Literjon_T6") // {MaxAmount: 20000, Restriction: "Water", Container: true}
```

```go
// the new bytes are flushed to the disk before they replace the file; the rename is tried six times while another program holds the file
err := durable.WriteFile(path, data, 0o600, durable.Defaults())
```

`durable.Defaults()` is six tries of the rename with a growing pause (150, 300 ... ms) and a flush of the folder afterwards. The flush of the folder is a no-op on Windows, where Go cannot open a folder for it and NTFS journals the rename itself. The zero `durable.Options{}` is one try, no pause, no folder flush. The package does not read the file back and does not keep backups: what to compare and what to keep are the caller's decisions.

## Rules of this module

- **Standard library only.** The module has no third-party dependency and never imports an `internal` package of the editor. A test fails if either changes. A program that uses it therefore never has to move its other dependencies to match.
- **Contents are listed in [PROVENANCE.md](PROVENANCE.md).** Code written for this project, and (later) tables of the game's identifiers and numbers. No game artwork, no saves.
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

# tabr-tau

A small Go app for editing a **single-player Dune: Awakening** save and its local game configs.
It is a browser UI served from one binary on `127.0.0.1`. There are no containers, servers, maps to
start or stop, or anything to host: it works on the files the game already keeps on your PC.

The player and database tooling follows the ideas in
[dune-awakening-selfhost-docker](https://github.com/Project-Arrakis/dune-awakening-selfhost-docker),
re-implemented for the client's SQLite save instead of the server's Postgres database. Server-only
features (containers, autoscaler, map/Sietche/Deep Desert instances, backups of a server DB, and so on) are out of scope.

> Unofficial. Not affiliated with or endorsed by Funcom. Editing saves can corrupt them; tabr-tau makes a
> backup before every write, but keep your own copy of anything you care about.
> For **single-player saves on your own PC only**. It is not for online-server characters, and editing a save
> may conflict with the game's terms of service or anti-cheat: that is your own risk. No warranty beyond the MIT licence.

> **Known security limits (a design audit found these; most are now fixed, the rest are scheduled):** the editor refuses
> connections from other machines and non-loopback `--addr` values, bounds the save decoder, escapes everything it renders,
> vets both SQL consoles and opens a save with unknown triggers/views read-only. Still open: the
> review-before-save screen is not built yet (#7), so **do not open a save file you got
> from someone else** and keep the automatic backups. Status: [issue #24](https://github.com/Project-Arrakis/tabr-tau/issues/24).

**Documentation:** [docs/README.md](docs/README.md) (design, audit findings, single-player file reference,
live-test protocol). Do not attach saves, `Game.ini`, snapshots or logs to issues: they contain account IDs.

## What it does

| Tab | Features |
|---|---|
| **Player** | Profile, Solari, give / edit / delete items, repair gear, teleport, faction reputation, specialization tracks, tutorials, tags, Journey nodes (complete / reset), learned building recipes |
| **Bases** | Land claims (totems), teleport to base, structure health, repair all, clear sand buildup, storage containers and machines (view / add / remove items), placeables, permissions |
| **Vehicles** | Vehicles in the world and recovered vehicles, bring a vehicle to you, set chassis durability |
| **Exchange** | Solari balance, vendor purchase limits and restock cycles (reset) |
| **Landsraad** | Current term, decree pool, active decree, task board (fill progress, complete / reopen), rewards |
| **Config** | Edit the game's `.ini` files (`ServerCustomSettings.ini`, `Game.ini`, `GameUserSettings.ini`, `Engine.ini`, `Input.ini`, ...) with backups; validates that the expected files and sections exist |
| **Database** | Browse and edit any table, CSV/JSON export, read-only SQL (one SELECT), and **write SQL** (INSERT/UPDATE/DELETE/REPLACE only; atomic, applied to your working copy) |

## How saving works

`game.db` (and `game_prepatch.db`, `autosave/*.bak`) is an 8-byte header (`uint32 1`, `uint32 size`) followed
by a zlib stream containing a plain SQLite 3 database. tabr-tau decodes it into a private working copy, and
**nothing is written until you press "Save to game"**. Saving:

1. refuses to run while `DuneSandbox-Win64-Shipping.exe` is running,
2. refuses if the file changed on disk since it was loaded,
3. runs `PRAGMA integrity_check` and verifies the re-encoded file decodes back to the same bytes,
4. copies the original to `tabr-tau-backups/` next to it,
5. writes the new file atomically.

Config edits work the same way (backups in `tabr-tau-backups/` beside the `.ini`). The game rewrites its
configs on exit, so close it before editing them.

## Run

```
go run ./cmd/tabr-tau            # auto-detects %LOCALAPPDATA%\DuneSandbox\...\game.db
go run ./cmd/tabr-tau --save "C:\path\to\game.db" --config "%LOCALAPPDATA%\DuneSandbox\Saved\Config\Windows"
```

**On Windows the editor opens in its own window** (it uses the Microsoft Edge WebView2 component that ships with Windows 11 and current Windows 10). Double-click the exe: if it cannot find your save it shows an Open dialog, and errors appear in message boxes. If WebView2 is missing it tells you and opens the browser instead. Pass `--web` to use the browser. Closing the window ends the program and asks first when there are unsaved edits. Subcommands (`diff`, `decode`, ...) still work from a terminal.

Flags: `--save`, `--config`, `--web` (browser instead of the window), `--addr` (default `127.0.0.1:8090`; must be a loopback address), `--allow-remote` (dangerous, needs `--web`, see the security note), `--no-browser`.

Helper commands: `tabr-tau find`, `tabr-tau decode <save> <out.sqlite>`, `tabr-tau encode <in.sqlite> <out.db>`.

Build the Windows app (from any OS, no C compiler needed) with
`GOOS=windows CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o tabr-tau.exe ./cmd/tabr-tau`
(`-H=windowsgui` removes the console window; the exe re-attaches to the terminal it was started from, so subcommands still
print). Windows Application Control may block unsigned binaries; `go run` works around that.

## Safety notes

- The server only listens on localhost, rejects other `Host` headers, and requires a per-run token on every API call.
- Vehicle fuel and other data stored in opaque binary blobs are read-only.
- Do **not** commit real saves. `.gitignore` excludes `*.db`, `*.sqlite`, `*.bak`, `*.ini` and backup folders.
  Saves and `Game.ini` contain your platform ID and account details.

## Development

```
go vet ./... && go test ./...
```

Tests use synthetic saves built in a temp folder; no real save data is needed.

## License

MIT. See [LICENSE](LICENSE).

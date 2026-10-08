# tabr-tau

A small Go app for editing a **single-player Dune: Awakening** save and its local game configs.
It is one binary that serves its UI on `127.0.0.1` only; on Windows it shows that UI in its own window, elsewhere in your browser. There are no containers, servers, maps to
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
> vets both SQL consoles and opens a save with unknown triggers/views read-only. Saving now goes through a review pane
> that lists every edit and what changes in the file. Destructive and bulk actions ask first. Still open: checks after the write
> and a recovery tool, so **do not open a save file you got from someone else** (the review shows
> what you changed, it does not make a hostile file safe) and keep the automatic backups. Status: [issue #24](https://github.com/Project-Arrakis/tabr-tau/issues/24).

**Documentation:** [docs/README.md](docs/README.md) (design, audit findings, single-player file reference,
live-test protocol). Do not attach saves, `Game.ini`, snapshots or logs to issues: they contain account IDs.

## What it does

| Tab | Features |
|---|---|
| **Player** | Profile, Solari, give / edit / delete items, repair gear, refill water containers, teleport, faction reputation, specialization tracks, tutorials, tags, Journey nodes (complete / reset), learned building recipes, vendor purchase limits and restock cycles (reset) |
| **Player > Bases** | Land claims (totems), expand or shrink a claim's size, raise its vertical level, teleport to base, structure health, repair all, clear sand buildup, refill base water and generator fuel (optionally when the editor opens), your base's storage containers (view / add / remove items), placeables |
| **Player > Vehicles** | Your own vehicles (the world's vehicles are hidden) and recovered vehicles, bring a vehicle to you, repair module durability, set chassis durability |
| **Landsraad** | Current term, decree pool, active decree, task board (fill progress, complete / reopen), rewards |
| **Config** | Edit the game's `.ini` files (`ServerCustomSettings.ini`, `Game.ini`, `GameUserSettings.ini`, `Engine.ini`, `Input.ini`, ...) with backups; validates that the expected files and sections exist |
| **Database** | Browse and edit any table, CSV/JSON export, read-only SQL (one SELECT), and **write SQL** (INSERT/UPDATE/DELETE/REPLACE only; atomic, applied to your working copy) |

The **Player** tab mirrors the Dune Docker console's Players > Player Name view: Character, Crafting, Research, Building Sets, Customizations, Skills, Specialization, Journey, Blueprints, Bases, Vehicles and Admin. Crafting, Research, Customizations, Skills and Blueprints say "not in tabr-tau yet" until they are built (#96). Each Player tab holds what the console's tab holds; features the console does not have (vendor purchase limits) are on the **Extras** tab. Landsraad, Config and Database stay top-level tabs.

## How saving works

`game.db` (and `game_prepatch.db`, `autosave/*.bak`) is an 8-byte header (`uint32 1`, `uint32 size`) followed
by a zlib stream containing a plain SQLite 3 database. tabr-tau decodes it into a private working copy, and
**nothing is written until you press "Review & save" and then "Save to game" inside the review**, which lists
every edit and what changes inside the file. Saving:

1. refuses to run while a **single-player session is active** (it reads the game's own log, `DuneSandbox.log`: a local `Survival_1?listen` map until you go to the menu or multiplayer, or quit). The game being open in multiplayer or at the menu does not block saving. Saving also waits about 15 seconds after the session ends, because the game writes the save one last time then. If the editor cannot tell (the game runs and its log cannot be read), it blocks and says why,
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

**On Windows the editor opens in its own window** (it uses the Microsoft Edge WebView2 component that ships with Windows 11 and current Windows 10). Double-click the exe: if it cannot find your save it shows an Open dialog, and errors appear in message boxes. If WebView2 is missing it says so in a message box (install the runtime, or start from a terminal with `--web` to use the browser instead). Pass `--web` to use the browser. Closing the window ends the program and asks first when there are unsaved edits (Yes/No, default No). Subcommands (`diff`, `decode`, `licenses`, ...) still work from a terminal. The Windows exe is a GUI-subsystem program, so `cmd.exe` does not wait for it: in scripts use `start /wait tabr-tau.exe ...` or PowerShell `Start-Process -Wait` (or build a console exe by leaving out `-H=windowsgui`).

Flags: `--save`, `--config`, `--web` (browser instead of the window), `--addr` (default `127.0.0.1:8090`; must be a loopback address, the editor never listens beyond this computer), `--no-browser`.

Helper commands: `tabr-tau find`, `tabr-tau decode <save> <out.sqlite>`, `tabr-tau encode <in.sqlite> <out.db>`.

Build the Windows app (from any OS, no C compiler needed) with
`GOOS=windows CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o tabr-tau.exe ./cmd/tabr-tau`
(`-H=windowsgui` removes the console window; the exe re-attaches to the terminal it was started from, so subcommands still
print). Windows Application Control may block unsigned binaries; `go run` works around that.

## Automatic refill

The Bases tab has an **Automatic refill** switch (off by default, remembered in `%APPDATA%\tabr-tau\settings.json`). When on, opening the editor refills base water and generators and **saves straight away**, without the Review & save step, so it applies the next time single-player loads. It is skipped while a single-player session is running or when there are unsaved edits; the previous file is backed up every time; a refused write leaves the edits pending with the reason. The card shows the last automatic save and its backup name. It applies to whichever save the editor opens.

## Safety notes

- tabr-tau's own code makes no outbound network connections (no telemetry, no update check). On Windows the editor window is the Microsoft Edge WebView2 runtime, a Windows component with its own update and diagnostics settings that tabr-tau does not control; it only ever loads the editor's own `127.0.0.1` address.
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

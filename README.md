# tabr-tau

A small Go app for editing a **single-player Dune: Awakening** save and its local game configs.
It is one binary that serves its UI on `127.0.0.1` only; it shows that UI in its own window (or in your browser with `--web`). **Windows 10/11 (64-bit) only**: the game runs only on Windows and the save lives on the same PC, so other systems are not supported. There are no containers, servers, maps to
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
> vets both SQL consoles and opens a save with unknown triggers/views read-only. Every edit is written to the file as it is made, after a backup, or queued while the game is in a single-player session. Destructive and bulk actions ask first. Every save is read back and verified before it replaces the file, and
> Database > Save backups restores a backup (see Restoring a backup below). The one thing it cannot do is make a hostile file safe, so **do not open a save file you got from someone else** (the backups keep what it
> was before you changed it, they do not make a hostile file safe) and keep the automatic backups. Status: [issue #24](https://github.com/Project-Arrakis/tabr-tau/issues/24).

**Documentation:** [docs/README.md](docs/README.md) (design, audit findings, single-player file reference,
live-test protocol). Do not attach saves, `Game.ini`, snapshots or logs to issues: they contain account IDs.

## What it does

| Tab | Features |
|---|---|
| **Player** | Profile, Solari, Blood Marks (the Halloween 2026 event currency), give / edit / delete items (a give puts in as much as fits and says how much), clear a container (backpack, worn gear, loadout; keeps the Solari Coin stack unless asked; the question counts the items inside bags and the editor refuses if the container changed), repair gear, refill water containers, fill blood sacks, fill all liquids (water, blood sacks and, optionally, your vehicles' fuel in one edit), teleport, faction reputation, specialization tracks, tutorials, tags, Journey nodes (complete / reset), learned building recipes, vendor purchase limits and restock cycles (reset), and reset buttons for what a character has earned (all skills to rank 0 on Skills, level and XP on Character, the XP of the specialization tracks, the journey on Journey; see Not built yet) |
| **Player > Bases** | The console's Bases panel: your bases with Power, Water, Inventory (Storage, Refining, Crafting, Other) and Land Claim Editor tabs. Also:  Land claims (totems), expand or shrink a claim's size, raise its vertical level, teleport to base, structure health, repair all, clear sand buildup, refill base water and generator fuel (optionally when the editor opens), your base's storage containers (view / add / remove items, or clear a container), placeables |
| **Player > Vehicles** | Your own vehicles (the world's vehicles are hidden) and recovered vehicles, bring a vehicle to you, repair module durability (each module is made as good as new), refuel all your vehicles to a full tank (tank sizes measured in the game) |
| **Saved map** | Hagga Basin and the Deep Desert as in the Dune Docker console, drawn from your save (as of its last write, not live): your character, vehicles, base, storage containers and the game's own map markers (ore, salvage, plants, enemy camps, places, trainers, hazards), with layers, pan, zoom and hover details |
| **Landsraad** | Current term, decree pool, active decree, task board (fill progress, complete / reopen), rewards |
| **Config** | Edit the game's `.ini` files (`ServerCustomSettings.ini`, `Game.ini`, `GameUserSettings.ini`, `Engine.ini`, `Input.ini`, ...) with backups; validates that the expected files and sections exist |
| **Database** | Browse and edit any table, CSV/JSON export, read-only SQL (one SELECT), and **write SQL** (INSERT/UPDATE/DELETE/REPLACE only; atomic, applied to your working copy) |

The **Player** tab mirrors the Dune Docker console's Players > Player Name view: Character, Crafting, Research, Building Sets, Customizations, Skills, Specialization, Journey, Bases, Vehicles and Admin. Specialization needs one step in game before the XP buttons work (see the tab). Each Player tab holds what the console's tab holds; tabr-tau's own base tools (structure health, repair all, clear sand, placeables, piece types) are on the base's **Structures** tab, and the vendor purchase limits are on **Admin**. Saved map, Landsraad, Config and Database stay top-level tabs.

## Not built yet

These console features are not in tabr-tau, and the page no longer shows a disabled button for them: **Blueprints** (waits for game data, #111), **Repair faction**, **Repair Landsraad quests**, **Wipe inventory** (what exists is Clear this container, one container at a time) and **Spawn vehicle**. **Reset progression** is not one button: each area that holds the data has its own reset, with its own typed confirmation (Skills, Level and XP, the specialization XP and the Journey are built, #339; Research has a disabled button that gives the reason: the save does not say which research a new character already has). Pressing each of them is the whole reset, and the backup restores the save as it was. The console's server-only actions (kick, ban, login queue, recovering a deleted character) do not apply to a single-player save.

## Words the editor uses

| Word | Meaning |
|---|---|
| **Queued** | A change made while the file cannot be written (the game is in a single-player session). It waits in a private journal and is written by itself once the file can be; **Write now** tries at once and **Discard queue** drops it. A queued change never touches the running game: it goes into the save file, and the game sees it the next time it loads that save. |
| **Check again** | Shown when the editor could not tell whether the game is running (the process list could not be read). The editor tries again by itself every few seconds and the banner goes away when a check works; **Check again** and **Write now** make the check at once and say what it found: the game is running (edits will queue), the game is not running (edits are written at once), or why the check failed again. While the check cannot be made, no change is written. |
| **Backup** | A copy of the file taken before a write (one per burst of edits), in a `tabr-tau-backups` folder beside it. The newest 60 are kept, and always the oldest one and the first one of the session (the file before the editor wrote anything). |
| **Write log** | `write-log.jsonl` in the editor's own folder (`%APPDATA%\tabr-tau`): one line for every write, with the time, how many edits, what they were in words, and the name of the backup taken before. The words are the sentences the page lists as changes, so they can carry game ids and amounts, but not SQL text, the key of an edited row, long numbers or hexadecimal ids, or file paths. At 256 KB it becomes `write-log.1.jsonl` (two files at most). A restore of a backup is recorded too. It is a record to read; the backup is the way back. |
| **Solari in backpack** | The Solari item stacks in the character's backpack. A stack holds 50,000: adding fills the stacks there and makes more, removing takes from the last stack back, and asking for more than the stacks hold is refused. |
| **Solari in wallet** and **House credit in wallet** | The character's two virtual currency balances, kept apart from the backpack stacks. |
| **Intel** | The points that pay for the research tree (capped at 2,779). |
| **Journey node** | One step of the Story, Contract, Codex or Tutorial lists. |
| **Specialization track** | One of the five tracks (Combat, Crafting, Exploration, Gathering, Sabotage) with its XP, level and keystones. |
| **Grade** | The quality level of an item, 0 to 5. |

## How saving works

`game.db` (and `game_prepatch.db`, `autosave/*.bak`) is an 8-byte header (`uint32 1`, `uint32 size`) followed
by a zlib stream containing a plain SQLite 3 database. tabr-tau decodes it into a private working copy, and
**an edit is written to the file as it is made, and queued while the file cannot be written** (the game is in a single-player session): the queue is kept in a journal and written by itself when the file can be. Writing:

1. refuses to run while a **single-player session is active** (it reads the game's own log, `DuneSandbox.log`: a local `Survival_1?listen` map until you go to the menu or multiplayer, or quit). The game being open in multiplayer or at the menu does not block saving. Saving also waits about 15 seconds after the session ends, because the game writes the save one last time then. If the editor cannot tell (the game runs and its log cannot be read), it blocks and says why,
2. refuses if the file changed on disk since it was loaded,
3. runs `PRAGMA integrity_check` and verifies the re-encoded file decodes back to the same bytes,
4. copies the original to `tabr-tau-backups/` next to it,
5. writes the new file atomically, then reads it back and compares (tried for about 2 seconds if another program holds the new file). If it still cannot be read back, the answer says the save was written and could not be verified, and the next attempt checks the file first: edits that are in it are not written a second time.

Config edits work the same way (backups in `tabr-tau-backups/` beside the `.ini`). The game rewrites its
configs on exit, so close it before editing them.

## Run

```
go run ./cmd/tabr-tau            # auto-detects %LOCALAPPDATA%\DuneSandbox\...\game.db
go run ./cmd/tabr-tau --save "C:\path\to\game.db" --config "%LOCALAPPDATA%\DuneSandbox\Saved\Config\Windows"
```

**On Windows the editor opens in its own window** (it uses the Microsoft Edge WebView2 component that ships with Windows 11 and current Windows 10). Double-click the exe: if it cannot find your save it shows an Open dialog, and errors appear in message boxes. If WebView2 is missing it says so in a message box (install the runtime, or start from a terminal with `--web` to use the browser instead). Pass `--web` to use the browser. Closing the window ends the program and asks first when changes are still queued (Yes/No, default No). Subcommands (`diff`, `decode`, `licenses`, ...) still work from a terminal. The Windows exe is a GUI-subsystem program, so `cmd.exe` does not wait for it: in scripts use `start /wait tabr-tau.exe ...` or PowerShell `Start-Process -Wait` (or build a console exe by leaving out `-H=windowsgui`).

Flags: `--save`, `--config`, `--web` (browser instead of the window), `--addr` (default `127.0.0.1:8090`; must be a loopback address, the editor never listens beyond this computer), `--no-browser`.

Helper commands: `tabr-tau find`, `tabr-tau decode <save> <out.sqlite>`, `tabr-tau encode <in.sqlite> <out.db>`.

Build the app (no C compiler needed; it can be cross-built from another OS, but only Windows is supported) with
`GOOS=windows CGO_ENABLED=0 go build -trimpath -ldflags "-H=windowsgui -s -w" -o tabr-tau.exe ./cmd/tabr-tau`
(`-H=windowsgui` removes the console window; the exe re-attaches to the terminal it was started from, so subcommands still
print). Windows Application Control may block unsigned binaries; `go run` works around that.

## Restoring a backup

Every save from the editor first copies the file it replaces into `tabr-tau-backups` next to `game.db` (for a typical install:
`%LOCALAPPDATA%\DuneSandbox\Saved\Cloud\PlayerClientStorage\FLS_retail\<your id>\tabr-tau-backups`). To go back:

1. Close the game, or go to its menu, and make sure the editor shows no queued changes (restoring reloads the file and would drop them).
2. Open **Database**, find **Save backups**, pick the backup and press Restore; you are asked to type a word to confirm.
3. The backup must decode, pass the integrity check and carry no database patch the current save lacks (a backup from a newer game is refused). The file being replaced is backed up first, so a restore can itself be undone, and the file is never truncated in place.
4. Start the game and check the character. Config backups work the same way on the **Config** tab.

Restore uses the same guard as saving: it refuses while a single-player session is active, just ended, or cannot be checked. Whether the game's own cloud sync touches the `tabr-tau-backups` folder has not been tested.

## Automatic refill and repair

The Power and Water tabs of a base have an **Automatic refill and repair when the editor opens** card: a master switch (off by default) and one checkbox per job. The settings are remembered in `%APPDATA%\tabr-tau\settings.json`, which also keeps the specialization track numbers you assign. When the master switch is on, opening the editor runs the jobs that are ticked as **one edit** and **saves straight away**, like every other edit, so it applies the next time single-player loads. The jobs, all ticked by default (the repairs too):

| Job | What it does |
|---|---|
| Base water | fills Water Cisterns, Windtraps and Blood Purifiers to capacity |
| Base power | tops up generators, wind turbines and windtraps |
| Repair vehicles | repairs every part of your own vehicles to full durability |
| Repair worn equipment and loadout | repairs what you wear and your loadout; the items in your backpack are not touched |
| Fill water containers | fills the Literjons and other water containers you carry |
| Fill your water | fills the character's own water |
| Fill blood sacks | fills the blood sacks you carry (the kinds whose capacity is known) |
| Refuel vehicles | fills the fuel tank of your own vehicles (the classes whose tank size is known) |

It acts only on what your character owns or carries (your own vehicles, worn equipment, loadout, inventories and your bases); a vehicle of another owner or a vehicle of the world is never touched. A pass is one transaction: one backup, one line in the write log (`automatic pass: ...`). When nothing needs doing nothing is written, no backup is made and no line is logged. If a job fails the whole pass is undone and the card shows which job failed and why; the other jobs are not applied. The pass is skipped while a single-player session is running (nothing is written or queued then) and when edits from before are still queued; the previous file is backed up every time; a refused write leaves the edit queued with the reason. The card shows, per job, what the last automatic save did. It applies to whichever save the editor opens. A settings file from an earlier version has only the master switch: every job then starts ticked.

## Which write features were checked in the game

Every write feature has automated tests on a synthetic save built from the game's real schema. This table says what was checked beyond that, using only what [CHANGELOG.md](CHANGELOG.md), [docs/live-test-protocol.md](docs/live-test-protocol.md) and [docs/evidence/](docs/evidence/) already record. The in-game results are the operator's reports: the evidence behind them is in issue comments, and no snapshot pair or screenshot for them is stored in `docs/evidence/` (audit finding GRC-3, [issue #163](https://github.com/Project-Arrakis/tabr-tau/issues/163)). A write feature that is not named here has no check beyond its tests on record.

| Feature | Checked how | Date | Recorded in |
|---|---|---|---|
| Repair all equipment (first version) and add Solari | **In game**, informal report, no stored evidence | 2026-10-07 | `docs/evidence/2026-10-07-windows-first-run.md` |
| Worn armor set, loadout items and item augments, written into a save by direct SQL (not by the editor's Give item) | **In game**, one run (live tests T7, T8, T9 and the loadout: PASS by that route) | 2026-10-07 | `docs/live-test-protocol.md`, Phase E |
| Refill base water, Refill generators (first version) and Repair all vehicles (first version) | **In game**: the game loaded the edited save and kept the values after rewriting it | 2026-10-08 | `CHANGELOG.md` (#63, #56) |
| Refill containers (Literjons) | **In game**: loaded, and kept after the game rewrote the save | 2026-10-08 | `CHANGELOG.md` (#55) |
| Expand a land claim (cells and vertical level) | **In game**: the expansion loaded, allowed building outside the old claim, and was kept after the game rewrote the save | 2026-10-08 | `CHANGELOG.md` (#57) |
| Shrink a land claim | **In game**: removing the 8 extra cells of an expanded claim worked, and the game then added its own cells when staking units were placed | 2026-10-08 | `CHANGELOG.md` (#57 follow-up) |
| Research Unlock and Crafting Unlock | **On a copy of a real save** (each changed exactly the expected rows); not in game | not recorded | `CHANGELOG.md` (#96) |
| Give XP | **On a copy of a real save** (+5,000 XP took level 86 to 94 and changed exactly three level fields); not in game | not recorded | `CHANGELOG.md` (#96) |
| Augments editor on the Inventory tab | Written data compared byte for byte with an augment in a real save; not in game | not recorded | `CHANGELOG.md` (#96) |
| Repair Vehicles (full durability of every module, #151) | The rule's values (from the game database's item pages) match every saved module of those templates in a real save; not in game | not recorded | `CHANGELOG.md` (#151) |
| Skill rank changes and Set unspent points | Tests, with the point ladder checked against the character's own skills; not in game | not recorded | `CHANGELOG.md` (#96) |
| Faction assignment | Tests only; stated as not yet in game | not recorded | `CHANGELOG.md` (#96) |
| Unlock customizations directly (set, all sets, one cosmetic; library ids) | **In game** for ordinary cosmetics (a dye pack stayed and was usable). Store (MTX_) and DLC cosmetics are **removed by the game on load** unless the account owns them, so sets skip them and a single unlock warns | 2026-10-20 | `CHANGELOG.md` (#215, #217) |
| Building Sets and Customizations: Give, Grant set, Unlock all; Unlock all research and recipes; Max all skills | Tests only; stated as not checked in game | not recorded | `CHANGELOG.md` (#122, #123, #124) |
| Clear this container (#338; real-schema save, checked with the integrity test) | Tests only | none | `CHANGELOG.md` (#338) |
| Give Blood Marks (#344; real-schema save, checked with the integrity test) | Tests only | none | `CHANGELOG.md` (#344) |
| Fill blood sacks (#346; capacities from saved values, real-schema save, checked with the integrity test) | Tests only | none | `CHANGELOG.md` (#346) |
| Give item (including the partial give that puts in as much as fits, #345), Fill hydration, Give Items queue, Give Intel, Give currency, edit or delete an item | Tests only (the protocol's T1 and T1b for Give item have no recorded result) | none | `docs/live-test-protocol.md` |
| Refuel all my vehicles (#349; tank sizes measured in the game, real-schema save, checked with the integrity test) | Tests only | none | `CHANGELOG.md` (#349) |
| Fill all liquids (#349; the water, blood-sack and refuel fills as one edit; real-schema save, checked with the integrity test) | Tests only | none | `CHANGELOG.md` (#349) |
| Repair Gear (as good as new, #153) and the vehicle Repair Below % option | Tests only | none | `CHANGELOG.md` |
| Teleport and Go to a place | Tests only | none | `CHANGELOG.md` (#67) |
| Specialization: Add XP, Grant Max, Reset, keystones, Assign | Tests only | none | `CHANGELOG.md` (#108) |
| Journey, Tutorial and tag changes | Tests only | none | `CHANGELOG.md` |
| Land Claim Editor grid (Apply changes) | Tests only | none | `CHANGELOG.md` (#133) |
| Automatic refill when the editor opens, and the refill of wind turbines and windtraps | Tests only (the first-version refill above was checked in game, the open-time save and the later changes were not) | none | `CHANGELOG.md` (#63) |
| Automatic repairs and fills when the editor opens, as one edit (#352) | Tests only | none | `CHANGELOG.md` (#352) |
| Structures: Repair all to max, Clear sand; Reset vendors; Landsraad edits; the Database cell editor and write SQL; Config edits; Restore a backup | Tests only | none | `CHANGELOG.md` |

Read-only views are not listed. One of them has a recorded gap: the Saved map's spice and flour sand positions are decoded from field ids and have not been checked against the game for the map shown (`CHANGELOG.md`, #101).

## Safety notes

- tabr-tau's own code makes no outbound network connections (no telemetry, no update check). On Windows the editor window is the Microsoft Edge WebView2 runtime, a Windows component with its own update and diagnostics settings that tabr-tau does not control; it only ever loads the editor's own `127.0.0.1` address.
- The server only listens on localhost, rejects other `Host` headers, and answers an API call only when it carries the session cookie that the window (or the one-time link printed with `--web`) was given at start; the cookie is valid for this run of the editor only.
- Data stored in opaque binary blobs is read-only. A vehicle's fuel is not one of them: it is a plain number in the vehicle's record, which Refuel all my vehicles raises.
- Only one editor can have a save open at a time (a second one is refused). If the default port is busy the editor says it picked another.
- A save outside the game's own folder (`Saved\Cloud\PlayerClientStorage`), on a network share or on a removable drive asks for one confirmation before it can be written, because the game-running check only sees this computer.
- Reporting a security problem: see [SECURITY.md](SECURITY.md).
- Do **not** commit real saves. `.gitignore` excludes `*.db`, `*.sqlite`, `*.bak`, `*.ini` and backup folders.
  Saves and `Game.ini` contain your platform ID and account details.

## Development

```
go vet ./... && go test ./...
```

Tests use synthetic saves built in a temp folder; no real save data is needed.

## License

MIT. See [LICENSE](LICENSE). **The MIT licence covers this project's code, not the game art in it.** The Saved map's two map pictures and 62 marker icons (`internal/web/static/maps/`) are Dune: Awakening artwork that belongs to Funcom; they are not licensed by this project, are included only to show where things are on the map (as the Dune Docker console does), and are not covered by the MIT licence. If you are the rights holder and want them removed, open an issue on this repository and they will be taken out. See `docs/art-notice.md` and the third-party notices (`tabr-tau licenses`).

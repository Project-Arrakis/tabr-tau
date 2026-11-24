# Changing the Deep Desert layout

The single-player Deep Desert has twelve layouts that the game can use, numbered 0 to 11. The layout is not stored in your save. The game picks it when it starts, from a setting in its own config. The command `tabr-tau layout-roll` changes that setting, at random or to a layout you choose, and can run by itself every Tuesday at 05:00.

## Read this first: bases stand on different terrain per layout

A base, a vehicle or anything else you placed in the Deep Desert keeps its position, not its surroundings. On another layout the same spot can be open sand, the landmarks are elsewhere, and what was next to your base is gone. This was seen in the game when a layout was changed by hand. Back up your save and your game before the first roll. The command now checks your save for you (see "The Deep Desert check" below) and refuses to roll while a base, the character or a vehicle of yours is in the Deep Desert. Going back to the old layout puts the old terrain back under it.

## What it does

- It changes one number, `WorldLayoutIndexInSingleplayer`, in the line `m_WorldLayoutsPerWorld` of `Game.ini` in your game config folder (`%LOCALAPPDATA%\DuneSandbox\Saved\Config\Windows`). If `Game.ini` has no such line yet, the whole line is added under its section. Every other line of the file is left as it is. It reads your save (to check the Deep Desert, below) and never writes to it.
- A random roll picks one of the layouts 0 to 11 and never the one that is set now. Layouts 12 to 17 are never picked. You can name one yourself with `--set N` (0 to 11).
- The **game must be restarted** for a new layout to apply. The command never starts, stops or restarts the game.

## The safeguards

- **The game must be closed.** If a Dune process is running, or the list of running programs cannot be read, nothing is changed. The command says so and you run it again after closing the game (the scheduled run tries again the next Tuesday).
- **A backup first.** Before every change the file is copied beside itself as `Game.ini.layout-roll-YYYYMMDD-HHMMSS.mmm.bak`. The newest 10 of these copies are kept.
- **The file is replaced in one step** (a temporary file, then a rename) and read back to check it. If it is not what was written, the command says so and names the backup.
- **Once per week.** A roll is skipped if one already happened since the last Tuesday at 05:00. `--force` rolls again. A layout chosen with `--set` is your own choice and is not held back by this.
- The last roll (time, old layout, new layout) and the last run are recorded in `%APPDATA%\tabr-tau\layout-roll.json`.

## The Deep Desert check

Before it edits `Game.ini`, and only when the game is closed and the weekly rule does not skip the run, the command looks at your save, in this order:

1. **A checkpoint.** A whole copy of the save file is made in the save's own backup folder (`tabr-tau-backups`, next to `game.db`), named with the time and `-layoutroll`, and checked by its size and SHA-256 against the bytes that were read. A checkpoint is never counted or removed by the automatic pruning of backups, and Database > Save backups lists it like any other backup (restore works as for the others). If it cannot be made or checked, nothing else is done.
2. **A read of the save.** Nothing is written to the save. The roll is **refused** (nothing is changed, the reason is printed and written in `layout-roll.json` under `lastCheck` and `lastRun`) when any of these is true:
   - **A base of yours stands in the Deep Desert.** Only bases you own and that are standing count. A base someone else owns, a base you are only a member of, a base the game's base tool picked up, and bases on other maps do not.
   - **The character is in the Deep Desert.** The save keeps the character in three places (the controller, the body and the player state); any of them on the Deep Desert counts. A save that does not say which map the character is on counts as in the Deep Desert.
   - **A vehicle of yours is in the Deep Desert,** placed or stored (a vehicle you hold the owner rank of). Vehicles of other owners, the world's own vehicles and vehicles on other maps do not count. A vehicle of yours whose map is not recorded counts as in the Deep Desert.
   - A save that cannot be read, that has no player, or that cannot be told apart from several (see `--save` below) also refuses: the check fails closed.
3. **The bases are exported first.** When a refusal is because bases stand in the Deep Desert, each of them is written as a console base backup file into a dated folder `layout-roll-bases-YYYYMMDD-HHMMSS.mmm` next to `layout-roll.json`, and each file is read back and checked with the editor's own reader. The folder and the file names are recorded in `layout-roll.json` (`lastCheck.exportFolder`, `lastCheck.exportFiles`). If a file cannot be made or does not read back, the run stops and says so.

`--set` and `--force` do not skip this check. The command never moves, stores, picks up or deletes anything. To roll, take the character out of the Deep Desert, move or store your vehicles, and pick up your bases there with the game's base tool (which also keeps each as a base backup; the exported files are extra copies), let the game save, close it and run the command again. Deleting the bases from here is not built yet.

The save checked is the one named with `--save` (before the word `layout-roll`, like `--config`), else the only one found on this computer. With none or several found the run refuses and says which, because it will not guess. The scheduled task registered by `layout-roll-task.ps1` has no `--save`; with more than one save on the computer, edit its action by hand.

## Try it without changing anything

```
tabr-tau layout-roll --dry-run
```

It prints the layout now, the last roll, what the Deep Desert check finds in your save and what the run would do (take the checkpoint, export the bases, refuse or roll), and writes nothing: no checkpoint, no base file, not even the state file. It reads a private temporary copy of your save to do so. Run this first.

Then, with the game closed:

```
tabr-tau layout-roll              (a random layout, once per week)
tabr-tau layout-roll --set 4      (layout 4)
tabr-tau layout-roll --force      (roll again this week)
```

`--config FOLDER` names the game config folder if it is not in the usual place. It goes before the word `layout-roll`.

## Going back

1. Close the game.
2. In the game config folder, rename or delete `Game.ini`, then copy the newest `Game.ini.layout-roll-...bak` you want to `Game.ini`. Or run `tabr-tau layout-roll --set N --force` with the old layout number.
3. Start the game. The log of the game names the layout it chose.

The backup taken before the very first roll is the file as it was before this feature touched it.

## The weekly scheduled task

Nothing is scheduled until you do it. The script `scripts\layout-roll-task.ps1` in the repository registers a Windows scheduled task for the current user, every Tuesday at 05:00 local time, that runs `tabr-tau layout-roll` with no window. It stores no password and asks for no elevation. If the computer was off at 05:00 it runs when it is next available. The task only runs while you are logged on.

```
powershell -File scripts\layout-roll-task.ps1 -ExePath "C:\full\path\tabr-tau.exe" -WhatIf    (shows what it would register)
powershell -File scripts\layout-roll-task.ps1 -ExePath "C:\full\path\tabr-tau.exe"            (registers it)
powershell -File scripts\layout-roll-task.ps1 -Remove                                         (removes it)
```

Keep `tabr-tau.exe` where it is: the task points at that file. Removing the task does not touch `Game.ini` or its backups. If the game is running at 05:00 the roll does not happen that week; the file `layout-roll.json` says "the game is running" under last run.

Nothing here has been checked in the game by this program yet: the setting itself was measured by hand in the game, and this command is checked with tests on temporary files and made-up saves.

# Changing the Deep Desert layout

The single-player Deep Desert has twelve layouts that the game can use, numbered 0 to 11. The layout is not stored in your save. The game picks it when it starts, from a setting in its own config. The command `tabr-tau layout-roll` changes that setting, at random or to a layout you choose, and can run by itself every Tuesday at 05:00.

## Read this first: bases stand on different terrain per layout

A base, a vehicle or anything else you placed in the Deep Desert keeps its position, not its surroundings. On another layout the same spot can be open sand, the landmarks are elsewhere, and what was next to your base is gone. This was seen in the game when a layout was changed by hand. Back up your save and your game before the first roll, and do not roll while something you care about stands in the Deep Desert. Going back to the old layout puts the old terrain back under it.

## What it does

- It changes one number, `WorldLayoutIndexInSingleplayer`, in the line `m_WorldLayoutsPerWorld` of `Game.ini` in your game config folder (`%LOCALAPPDATA%\DuneSandbox\Saved\Config\Windows`). If `Game.ini` has no such line yet, the whole line is added under its section. Every other line of the file is left as it is. It does not read or write your save.
- A random roll picks one of the layouts 0 to 11 and never the one that is set now. Layouts 12 to 17 are never picked. You can name one yourself with `--set N` (0 to 11).
- The **game must be restarted** for a new layout to apply. The command never starts, stops or restarts the game.

## The safeguards

- **The game must be closed.** If a Dune process is running, or the list of running programs cannot be read, nothing is changed. The command says so and you run it again after closing the game (the scheduled run tries again the next Tuesday).
- **A backup first.** Before every change the file is copied beside itself as `Game.ini.layout-roll-YYYYMMDD-HHMMSS.mmm.bak`. The newest 10 of these copies are kept.
- **The file is replaced in one step** (a temporary file, then a rename) and read back to check it. If it is not what was written, the command says so and names the backup.
- **Once per week.** A roll is skipped if one already happened since the last Tuesday at 05:00. `--force` rolls again. A layout chosen with `--set` is your own choice and is not held back by this.
- The last roll (time, old layout, new layout) and the last run are recorded in `%APPDATA%\tabr-tau\layout-roll.json`.

## Try it without changing anything

```
tabr-tau layout-roll --dry-run
```

It prints the layout now, the last roll, and what it would change, and writes nothing (not even the state file). Run this first.

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

Nothing here has been checked in the game by this program yet: the setting itself was measured by hand in the game, and this command is checked with tests on temporary files.

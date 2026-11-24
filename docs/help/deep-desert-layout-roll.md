# Changing the Deep Desert layout

The single-player Deep Desert has twelve layouts that the game can use, numbered 0 to 11. The layout is not stored in your save. The game picks it when it starts, from a setting in its own config. The command `tabr-tau layout-roll` changes that setting, at random or to a layout you choose, and can run by itself every Tuesday at 05:00.

## Read this first: bases stand on different terrain per layout

A base, a vehicle or anything else you placed in the Deep Desert keeps its position, not its surroundings. On another layout the same spot can be open sand, the landmarks are elsewhere, and what was next to your base is gone. This was seen in the game when a layout was changed by hand. Back up your save and your game before the first roll. The command now checks your save for you (see "The Deep Desert check" below): it refuses to roll while a base or the character of yours is in the Deep Desert, and it takes your vehicles out of the Deep Desert first (see "What happens to your vehicles"). Going back to the old layout puts the old terrain back under it.

**Before you log out for the roll, face open ground with room in front of you.** If vehicles have to be moved, they are put down in front of where your character stands and faces when the game saved. The save has no terrain: rock, mountains and slopes are not in it, so a vehicle can be put inside a rock if you were facing one. Facing open sand or flat ground, with about 100 m (a long line of big vehicles needs that much) in front of you, avoids it. This is the same rule that works for Bring vehicle.

## How to run the commands

The commands are for PowerShell and are written so that you can copy and paste each one as it stands. They assume `tabr-tau.exe` is in your Downloads folder. If you keep it somewhere else, replace `Downloads\tabr-tau.exe` in the command with where it is (the part after `$env:USERPROFILE\`). `$env:USERPROFILE` is your own user folder, so no name has to be typed. You can paste them from any folder.

## What it does

- It changes one number, `WorldLayoutIndexInSingleplayer`, in the line `m_WorldLayoutsPerWorld` of `Game.ini` in your game config folder (`%LOCALAPPDATA%\DuneSandbox\Saved\Config\Windows`). If `Game.ini` has no such line yet, the whole line is added under its section. Every other line of the file is left as it is.
- It reads your save to check the Deep Desert (below). It writes to the save in one case only: when vehicles of yours stand in the Deep Desert and have to be stored or moved out (see "What happens to your vehicles"). Nothing else in the save is ever changed.
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
2. **A read of the save.** The roll is **refused** (nothing is changed, the reason is printed and written in `layout-roll.json` under `lastCheck` and `lastRun`) when any of these is true:
   - **A base of yours stands in the Deep Desert.** Only bases you own and that are standing count. A base someone else owns, a base you are only a member of, a base the game's base tool picked up, and bases on other maps do not.
   - **The character is in the Deep Desert.** The save keeps the character in three places (the controller, the body and the player state); any of them on the Deep Desert counts. A save that does not say which map the character is on counts as in the Deep Desert.
   - A save that cannot be read, that has no player, or that cannot be told apart from several (see `--save` below) also refuses: the check fails closed.
3. **Your vehicles are taken out** (next section), only when nothing above blocks the roll. A run that is going to refuse changes nothing, vehicles included.
4. **The check again.** After the vehicles are out, the save is read once more and must show none of them in the Deep Desert. A vehicle counts when you hold its owner rank and it stands in the world there; a vehicle of yours whose map is not recorded counts too. A vehicle that is **stored** (in the game's vehicle backup, or stored for recovery) does not count: it keeps the last place it stood, but it is not in the world. Vehicles of other owners, the world's own vehicles and vehicles on other maps do not count either.
5. **The bases are exported first.** When a refusal is because bases stand in the Deep Desert, each of them is written as a console base backup file into a dated folder `layout-roll-bases-YYYYMMDD-HHMMSS.mmm` next to `layout-roll.json`, and each file is read back and checked with the editor's own reader. The folder and the file names are recorded in `layout-roll.json` (`lastCheck.exportFolder`, `lastCheck.exportFiles`). If a file cannot be made or does not read back, the run stops and says so.

`--set` and `--force` do not skip this check. The command never moves, picks up or deletes a base. To roll, take the character out of the Deep Desert and pick up your bases there with the game's base tool (which also keeps each as a base backup; the exported files are extra copies), let the game save, close it and run the command again. Deleting the bases from here is not built yet.

The save checked is the one named with `--save` (before the word `layout-roll`, like `--config`), else the only one found on this computer. With none or several found the run refuses and says which, because it will not guess. The scheduled task registered by `layout-roll-task.ps1` has no `--save`; with more than one save on the computer, edit its action by hand.

## What happens to your vehicles

Each vehicle of yours that stands in the Deep Desert is taken out before the layout is rolled, one of two ways. Only your own vehicles are touched (the ones you hold the owner rank of). The world's vehicles, other people's vehicles, vehicles on other maps and vehicles already stored are left alone. What a vehicle carries stays with it: nothing in its cargo or inventories is touched.

- **A Scout or a Sandbike is stored in the game's own vehicle backup.** The edit is the one the game makes itself, seen in a real before and after save: the vehicle's state goes from 0 to 2 and one row is added to the character's vehicle backup (the character, the vehicle and the vehicle's own customization). The vehicle keeps its place in the save but is out of the world, so the new terrain cannot bury it. The game's backup has room for one vehicle for each character, so only one vehicle can be stored this way, and only when the character has none there yet. A second Scout or Sandbike, or one whose customization the save does not record, is moved instead, as below, and the line for it says why.
- **Every other vehicle (a buggy, a crawler, a carrier, an assault ornithopter, a transport) is moved to the map the character is on**, by changing the vehicle's map and place, exactly as Bring vehicle does. It goes in front of the character's saved position and facing: a ground vehicle 10 m ahead, an aircraft 30 m ahead and 7 m up (it starts in the air and the game settles it). Several vehicles go in one line, at least 12 m apart and farther for big ones; the ground vehicles come first and the aircraft after them, each aircraft 20 m farther out than the ground vehicle at its step, so one never settles onto another. The places are chosen clear of base pieces, land claims and other vehicles. The character has to be outside the Deep Desert for this (the existing check refuses otherwise), because the vehicles go to the character's map.

All of it is one edit of the save. The rows written are read back inside the edit, and if anything read back differs from what was meant, the whole edit is undone and the save is left as it was. A run refuses and changes nothing when a vehicle has no clear spot (the message names that vehicle: face other ground and run again), when a vehicle's map is not recorded, or when the character's place cannot be told. The edit is written to the save only through the editor's own write path, which refuses while the game runs and keeps its own backup, and the written file is read back once more. If the editor window has the same save open, the run refuses (close that window first). If something fails after the save was written, the message says the save **was** changed and names the checkpoint that holds it as it was.

The roll's output lists every vehicle: what was done, from where, to where (words and coordinates). `--dry-run` lists the same, changing nothing.

## Try it without changing anything

Run this first. It prints the layout now, the last roll, what the Deep Desert check finds in your save, which vehicles it would store and which it would move and to where, and what it would do next (take the checkpoint, export the bases, refuse or roll). It writes nothing: no checkpoint, no vehicle edit, no base file, not even the state file. It reads a private temporary copy of your save to do so.

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" layout-roll --dry-run
```

Then, with the game closed, one of these:

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" layout-roll
```

A random layout, once per week.

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" layout-roll --set 4
```

Layout 4 (any number from 0 to 11).

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" layout-roll --force
```

Roll again this week.

`--config` names the game config folder if it is not in the usual place, and `--save` names the save (the `game.db` file or its folder). Both go before the word `layout-roll`:

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" --config "$env:LOCALAPPDATA\DuneSandbox\Saved\Config\Windows" --save "C:\path\to\the\folder\with\game.db" layout-roll --dry-run
```

Replace `C:\path\to\the\folder\with\game.db` with the folder that holds your save. This command lists the saves it finds on this computer, to copy a folder from:

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" find
```

## Going back

Where things are written:

- **Game.ini and its backups:** `%LOCALAPPDATA%\DuneSandbox\Saved\Config\Windows` (`Game.ini` and the `Game.ini.layout-roll-YYYYMMDD-HHMMSS.mmm.bak` copies).
- **The checkpoint of the save:** the folder `tabr-tau-backups` next to your `game.db`, a file named with the time and `-layoutroll`.
- **What the last run did:** `%APPDATA%\tabr-tau\layout-roll.json`.

To put the layout back:

1. Close the game.
2. Put the newest `Game.ini.layout-roll-...bak` you want back as `Game.ini`. In PowerShell, after replacing `YYYYMMDD-HHMMSS.mmm` with the numbers in the file's name (this overwrites `Game.ini`):

```
Copy-Item -Force "$env:LOCALAPPDATA\DuneSandbox\Saved\Config\Windows\Game.ini.layout-roll-YYYYMMDD-HHMMSS.mmm.bak" "$env:LOCALAPPDATA\DuneSandbox\Saved\Config\Windows\Game.ini"
```

   Or run the command with the old layout number: `& "$env:USERPROFILE\Downloads\tabr-tau.exe" layout-roll --set 11 --force` (11 is only an example).
3. Start the game. The log of the game names the layout it chose.

The backup taken before the very first roll is the file as it was before this feature touched it.

To put your **save** back as it was before the roll took your vehicles out (the run says so when it writes to the save): close the game, open the editor, go to Database > Save backups and restore the checkpoint named in the run's output (the file with `-layoutroll` in its name). Restore the Game.ini backup as above if the layout was rolled as well.

## The weekly scheduled task

Nothing is scheduled until you do it. The script `scripts\layout-roll-task.ps1` in the repository registers a Windows scheduled task for the current user, every Tuesday at 05:00 local time, that runs `tabr-tau layout-roll` with no window. It stores no password and asks for no elevation. If the computer was off at 05:00 it runs when it is next available. The task only runs while you are logged on.

The commands assume the repository folder is `tabr-tau` in your user folder and `tabr-tau.exe` is in Downloads; replace those two parts if yours are elsewhere. Each can be pasted from any folder.

Show what it would register, without registering anything:

```
& "$env:USERPROFILE\tabr-tau\scripts\layout-roll-task.ps1" -ExePath "$env:USERPROFILE\Downloads\tabr-tau.exe" -WhatIf
```

Register it:

```
& "$env:USERPROFILE\tabr-tau\scripts\layout-roll-task.ps1" -ExePath "$env:USERPROFILE\Downloads\tabr-tau.exe"
```

Remove it:

```
& "$env:USERPROFILE\tabr-tau\scripts\layout-roll-task.ps1" -Remove
```

Keep `tabr-tau.exe` where it is: the task points at that file. Removing the task does not touch `Game.ini` or its backups. If the game is running at 05:00 the roll does not happen that week; the file `layout-roll.json` says "the game is running" under last run. Because the scheduled run also moves vehicles (and writes the save when it does), leave the character facing open ground with room in front whenever you log out on a Monday night.

Nothing here has been checked in the game by this program yet: the setting itself was measured by hand in the game; storing a Scout was seen in a real before and after save, and moving a vehicle from the Deep Desert to the Hagga Basin by editing its place worked in a real test; storing a Sandbike this way, and where a moved vehicle ends up against the terrain, have not been seen. This command is checked with tests on temporary files, made-up saves and copies of real saves.

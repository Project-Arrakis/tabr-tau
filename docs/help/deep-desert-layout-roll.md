# Changing the Deep Desert layout

The single-player Deep Desert has twelve layouts that the game can use, numbered 0 to 11. The layout is not stored in your save. The game picks it when it starts, from a setting in its own config. The command `tabr-tau layout-roll` changes that setting, at random or to a layout you choose, and can run by itself every Tuesday at 05:00.

## Read this first: bases stand on different terrain per layout

A base, a vehicle or anything else you placed in the Deep Desert keeps its position, not its surroundings. On another layout the same spot can be open sand, the landmarks are elsewhere, and what was next to your base is gone. This was seen in the game when a layout was changed by hand. Back up your save and your game before the first roll. The command now checks your save for you (see "The Deep Desert check" below): it refuses to roll while a base or the character of yours is in the Deep Desert, and it takes your vehicles out of the Deep Desert first (see "What happens to your vehicles"). Going back to the old layout puts the old terrain back under it.

**Before you log out for the roll, face open ground with room in front of you.** If vehicles have to be moved, they are put down in front of where your character stands and faces when the game saved. The save has no terrain: rock, mountains and slopes are not in it, so a vehicle can be put inside a rock if you were facing one. Facing open sand or flat ground, with about 100 m (a long line of big vehicles needs that much) in front of you, avoids it. This is the same rule that works for Bring vehicle.

## How to run the commands

The commands are for PowerShell and are written so that you can copy and paste each one as it stands. They assume `tabr-tau.exe` is in your Downloads folder. If you keep it somewhere else, replace `Downloads\tabr-tau.exe` in the command with where it is (the part after `$env:USERPROFILE\`). `$env:USERPROFILE` is your own user folder, so no name has to be typed. You can paste them from any folder.

## What it does

- It changes one number, `WorldLayoutIndexInSingleplayer`, in the line `m_WorldLayoutsPerWorld` of `Game.ini` in your game config folder (`%LOCALAPPDATA%\DuneSandbox\Saved\Config\Windows`). If `Game.ini` has no such line yet, the whole line is added under its section. Every other line of the file is left as it is.
- It reads your save to check the Deep Desert (below). It writes to the save in two cases only: when vehicles of yours stand in the Deep Desert and have to be stored or moved out (see "What happens to your vehicles"), and, only if you pass `--recycle-bases`, when bases of yours stand there and are recycled (see "Recycling the Deep Desert bases"). Nothing else in the save is ever changed.
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

`--set` and `--force` do not skip this check. Without `--recycle-bases` the command never moves, picks up or deletes a base (with it, see "Recycling the Deep Desert bases" below). To roll, take the character out of the Deep Desert and pick up your bases there with the game's base tool (which also keeps each as a base backup; the exported files are extra copies), let the game save, close it and run the command again. Recycling the bases from here is the opt-in `--recycle-bases`, below.

The save checked is the one named with `--save` (before the word `layout-roll`, like `--config`), else the only one found on this computer. With none or several found the run refuses and says which, because it will not guess. The scheduled task registered by `layout-roll-task.ps1` has a `--save` only when you give the script `-SavePath` (see "The weekly scheduled task" below); with more than one save on the computer, give it, or every weekly run refuses.

## What happens to your vehicles

Each vehicle of yours that stands in the Deep Desert is taken out before the layout is rolled, one of two ways. Only your own vehicles are touched (the ones you hold the owner rank of). The world's vehicles, other people's vehicles, vehicles on other maps and vehicles already stored are left alone. What a vehicle carries stays with it: nothing in its cargo or inventories is touched.

- **A Scout or a Sandbike is stored in the game's own vehicle backup.** The edit is the one the game makes itself, seen in a real before and after save: the vehicle's state goes from 0 to 2 and one row is added to the character's vehicle backup (the character, the vehicle and the vehicle's own customization). The vehicle keeps its place in the save but is out of the world, so the new terrain cannot bury it. The game's backup has room for one vehicle for each character, so only one vehicle can be stored this way, and only when the character has none there yet. A second Scout or Sandbike, or one whose customization the save does not record, is moved instead, as below, and the line for it says why.
- **Every other vehicle (a buggy, a crawler, a carrier, an assault ornithopter, a transport) is moved to the map the character is on**, by changing the vehicle's map and place, exactly as Bring vehicle does. It goes in front of the character's saved position and facing: a ground vehicle 10 m ahead, an aircraft 30 m ahead and 7 m up (it starts in the air and the game settles it). Several vehicles go in one line, at least 12 m apart and farther for big ones; the ground vehicles come first and the aircraft after them, each aircraft 20 m farther out than the ground vehicle at its step, so one never settles onto another. The places are chosen clear of base pieces, land claims and other vehicles. The character has to be outside the Deep Desert for this (the existing check refuses otherwise), because the vehicles go to the character's map.

All of it is one edit of the save. The rows written are read back inside the edit, and if anything read back differs from what was meant, the whole edit is undone and the save is left as it was. A run refuses and changes nothing when a vehicle has no clear spot (the message names that vehicle: face other ground and run again), when a vehicle's map is not recorded, or when the character's place cannot be told. The edit is written to the save only through the editor's own write path, which refuses while the game runs and keeps its own backup, and the written file is read back once more. If the editor window has the same save open, the run refuses (close that window first). If something fails after the save was written, the message says the save **was** changed and names the checkpoint that holds it as it was.

The roll's output lists every vehicle: what was done, from where, to where (words and coordinates). `--dry-run` lists the same, changing nothing.

## Recycling the Deep Desert bases (`--recycle-bases`, off unless you give it)

By default a base of yours in the Deep Desert refuses the roll and you pick it up yourself (above). With `--recycle-bases` the run recycles those bases first (#1167), using the same
recycle as Player > Bases > Recycle base (see docs/recycle-a-base.md in the repository) **with its Solido option**: the base is deleted, a Solido of it is made in a
device of your character, and what the base stored plus its refund go to the **Reserve**. It is **off** unless you pass the flag, and the weekly task has it only if you register it with
`-RecycleBases`. Without the flag everything above is exactly as it was.

**Run `--dry-run` with the flag first, on a copy of your save if you can**, and read it, especially with two or more bases. Nothing here has been checked in the game by this program yet:

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" layout-roll --dry-run --recycle-bases
```

### The order of a run, and what stops it

Every step must succeed, or the run stops. Everything before the one write of the save leaves the save file exactly as it was.

1. The game is closed, and a checkpoint of the save is taken and verified by its hash (as above).
2. The run **stops and recycles nothing** when: the character is in the Deep Desert; something could not be read; **any base that is not your own** stands in the Deep Desert (owned by someone else, shared, or owned by nobody); or a base of yours has no recorded map. Only a base on the Deep Desert that your own character owns is ever recycled; a base on another map is never touched and never stops the run. A base counts as being in the Deep Desert only when its map is exactly the Deep Desert (a name that merely starts with it does not count), and a base is refused, with nothing recycled, when **any** of its actors (its totem, its placed items, its building pieces) is on another map.
3. The room is worked out next (see "Space", below), and only a run that can recycle at least one base goes on: **a run refused for lack of room exports nothing and leaves no folder of base files**, only the checkpoint, which is always taken before anything else. Then every base is exported to a console base backup file in a dated folder, and each file is read back (as above).
4. Every base is asked about (the same question the Recycle base button asks). If **any** base is refused for a reason other than room (a crafting queue running, a building type the cost table does not know, a character waiting to respawn there, and so on), **no** base is recycled and every refusal is listed.
5. The bases are recycled **one at a time, in the order of their numbers** (the lowest number first, so the plan and the log are the same every time). Each is a recycle of its own, with a Solido, on the save as the bases before it left it: each is asked about again at that point, so what the bases before it used up (the Reserve, the backpack slots, the Solido devices) is seen. Each base gets a **new** Solido device (a filled Solido is never reused; a blank device of your own is used for one base only). After each base the run reads it back and checks it: the base is gone, its Solido is in the device and linked to it, and the Reserve holds exactly what moved and was refunded. Only then does the next base start. All of this happens on a working copy: **nothing reaches the save file until the end.**
6. The save is written **once**, through the editor's own write path, and the written file is read back: the recycled bases are gone, every Solido is in place, and no base other than those still waiting stands in the Deep Desert.
7. Your vehicles in the Deep Desert are then taken out (as above), and only when the Deep Desert is looked at once more and holds no base is the layout rolled.

A failure at any base before step 6 (a base is refused after the bases before it were recycled, a read back differs, the write does not happen) throws the working copy away: **the save file is not written**, and the message says which bases were done on the copy, which failed and which were not tried, and names the checkpoint. A write that is not reported as saved but **did** replace the file (it could not be read back in time) is treated as a changed save too: the run compares the file with how it was before the write, and says the save **was** changed when it differs (the same is done for the write of the vehicles). A failure after the write (the file does not read back as written) says the save **was** changed, names the checkpoint and gives the command to put it back (Database > Save backups, or `Copy-Item -Force "the checkpoint file" "the game.db file"` with the game closed), and the layout is not rolled. The same note is added to any later failure of the run, for example a Game.ini that cannot be written.

### Space: the Reserve must be empty before each base

A recycle sends everything to the **Reserve**: the items the base stored, unchanged, and the refund as new stacks. The rule used here is the operator's: **no further recycle can happen until the Reserve is empty.** So before each base the run needs an empty Reserve, and each base fills it again. The run recycles the largest number of bases, in order, that this and the room for the Solido devices allow:

- with something in the Reserve at the start, **no** base is recycled (0 of N), nothing changes, and the run says to empty the Reserve in the game (move its contents to storage) and run again;
- with an empty Reserve, **one** base is recycled in a run, and the run then stops before the roll ("1 of N recycled"), because the next base would land on a Reserve that is not empty;
- after you empty the Reserve in the game (move its contents to storage boxes) you run again: the remaining bases are done the same way, each run taking its own checkpoint and its own exports. A base that was recycled is simply gone, so a re-run is safe and picks up where the last one stopped. The roll happens in the run that recycles the **last** base (or in a run that finds no base left), never while a base stands, however many runs it takes.

**Several bases in one run need the move of the Reserve to storage, which is not part of this change; it is #1191.** The loop has one call point for it (a function variable called after each base is recycled and verified; it is empty today), so wiring #1191 in later is a small change. Choosing storage boxes as the destination is not part of this change either. The rule is the operator's and is not read from the game: the editor's own recycle only refuses when the Reserve has a slot limit that the base would pass (the saves seen record none), and the Reserve's volume is not checked at all.

The Solido devices need room too. A new device goes in the backpack (the loadout when the backpack has no room), one slot each, and the run keeps **two** slots free there after them. When the volume limit of the backpack or loadout leaves no room for a device, or the slots are short, that base does not fit: the run recycles the bases before it, stops before the roll, and names the destination and what is short ("backpack: 1 slot(s) needed for Solido devices plus a margin of 2, 2 free (short by 1)").

`--dry-run` prints all of it: the Reserve rule as its own line, the room now, each base (pieces and placeables deleted, stored stacks moved, the refund item by item, which Solido device and where), how many of the N bases would be recycled (k of N), the Solido devices and the refund of all of them together, and a line per destination with what is needed and what is free. It changes nothing: no checkpoint, no export, no state file, no write to the save. It plays the recycles on a private copy and throws it away.

### Where things go, and what is recorded

- The stored items and the refund go to the **Reserve**; a Solido device goes to the **backpack** (the loadout when the backpack has no room). The recycle itself touches no vehicle: it refuses a base only when one of the base's own actors is a vehicle, and a vehicle of yours that merely stands in or beside the base is left to the vehicle step of the roll (as above, after the bases).
- For every base the run prints what the recycle itself said about it: its summary lines (the Solido, the refund, the respawn points deleted, holograms and Developer storage containers, the water that is dropped) and its warnings (for example placeables with no support, or pieces that belong to no base).
- `layout-roll.json` records the run under `lastCheck`: the checkpoint's name, SHA-256 and full path, the export folder and files, and for every base recycled its number, name, map, pieces, stored stacks, refund, **Solido device and blueprint numbers**, and the room table. The state file is written as soon as the save has been written, before it is read back, so even a run that is cut off there leaves the checkpoint and the bases on record.
- `layout-roll.json` also keeps a short **history** under `recycleHistory`, one entry for each run that recycled bases (the last 20): the time, the checkpoint's name and full path, the numbers of the bases and the ids of their Solido devices. `lastCheck` is replaced by the next run; the history is not, so a series of runs (one base each, until the Reserve is emptied) keeps every checkpoint. The run prints the earlier checkpoints at the start and the whole list after it recycled.

### Going back

Close the game and restore the checkpoint named in the output (Database > Save backups, or the `Copy-Item` command the run prints). That puts back the bases, the Reserve and the backpack as they were **before that run**. When bases were recycled over several runs, the checkpoint of the **earliest** run in the history is the full way back to before the first base was recycled; restoring a later one goes back only to before that run. The list is in `recycleHistory` of `layout-roll.json` and in the output of each run, and the files themselves are in the save's backup folder (the ones with `-layoutroll` in the name, never removed by the automatic pruning). The exported base files are extra copies of each base.

### After the first real run: check in the game

In the game: the **Solido is in the backpack** (one per recycled base, each with its base's pieces), the **Reserve holds the stored items and the refund**, and the bases are gone.

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

If your computer has more than one save (the `find` command above lists them), the weekly run cannot tell which one to check and refuses every week. Name the save with `-SavePath`, the `game.db` file or the folder that holds it (it must exist). The task then runs `tabr-tau.exe --save "the path you gave" layout-roll`. `-WhatIf` shows the whole line first:

```
& "$env:USERPROFILE\tabr-tau\scripts\layout-roll-task.ps1" -ExePath "$env:USERPROFILE\Downloads\tabr-tau.exe" -SavePath "C:\path\to\the\folder\with\game.db" -WhatIf
```

Replace `C:\path\to\the\folder\with\game.db` with the folder that holds your save, and take `-WhatIf` off to register it. A task that was registered without `-SavePath` is changed by removing it (`-Remove`, below) and registering it again with `-SavePath`.

To have the weekly run recycle your Deep Desert bases first (off unless you give it; read "Recycling the Deep Desert bases" above and run `layout-roll --dry-run --recycle-bases` by hand first), register it with `-RecycleBases`; the task action then reads `layout-roll --recycle-bases`, and the task is given 30 minutes instead of 5:

```
& "$env:USERPROFILE\tabr-tau\scripts\layout-roll-task.ps1" -ExePath "$env:USERPROFILE\Downloads\tabr-tau.exe" -RecycleBases -WhatIf
```

Take `-WhatIf` off to register it. A task registered without the switch is changed by removing it and registering it again with it.

Remove it:

```
& "$env:USERPROFILE\tabr-tau\scripts\layout-roll-task.ps1" -Remove
```

Keep `tabr-tau.exe` where it is: the task points at that file. Removing the task does not touch `Game.ini` or its backups. If the game is running at 05:00 the roll does not happen that week; the file `layout-roll.json` says "the game is running" under last run. Because the scheduled run also moves vehicles (and writes the save when it does), leave the character facing open ground with room in front whenever you log out on a Monday night.

Nothing here has been checked in the game by this program yet: the setting itself was measured by hand in the game; storing a Scout was seen in a real before and after save, and moving a vehicle from the Deep Desert to the Hagga Basin by editing its place worked in a real test; storing a Sandbike this way, and where a moved vehicle ends up against the terrain, have not been seen. This command is checked with tests on temporary files, made-up saves and copies of real saves.

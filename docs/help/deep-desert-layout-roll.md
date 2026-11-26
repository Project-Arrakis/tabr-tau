# Changing the Deep Desert layout

The single-player Deep Desert has twelve layouts that the game can use, numbered 0 to 11. The layout is not stored in your save. The game picks it when it starts, from a setting in its own config. The command `tabr-tau layout-roll` changes that setting, at random or to a layout you choose, and can run by itself every Tuesday at 05:00.

## Read this first: bases stand on different terrain per layout

A base, a vehicle or anything else you placed in the Deep Desert keeps its position, not its surroundings. On another layout the same spot can be open sand, the landmarks are elsewhere, and what was next to your base is gone. This was seen in the game when a layout was changed by hand. Back up your save and your game before the first roll. The command now checks your save for you (see "The Deep Desert check" below): it refuses to roll while a base or the character of yours is in the Deep Desert, and it takes your vehicles out of the Deep Desert first (see "What happens to your vehicles"). Going back to the old layout puts the old terrain back under it.

**Before you log out for the roll, face open ground with room in front of you.** If vehicles have to be moved, they are put down in front of where your character stands and faces when the game saved. The save has no terrain: rock, mountains and slopes are not in it, so a vehicle can be put inside a rock if you were facing one. Facing open sand or flat ground, with about 100 m (a long line of big vehicles needs that much) in front of you, avoids it. This is the same rule that works for Bring vehicle.

## Known limitation: worm sign at labs, caves and wrecks

In single-player, layouts other than 11 can show worm sign at labs, caves and wrecks (issue #1242). The built-in vetted list still includes layout 9, which is predicted to be affected (not tested in the game). See "Which layouts a random roll may pick" below for how the list is used and changed.

## How to run the commands

The commands are for PowerShell and are written so that you can copy and paste each one as it stands. They assume `tabr-tau.exe` is in your Downloads folder. If you keep it somewhere else, replace `Downloads\tabr-tau.exe` in the command with where it is (the part after `$env:USERPROFILE\`). `$env:USERPROFILE` is your own user folder, so no name has to be typed. You can paste them from any folder.

## What it does

- It changes one number, `WorldLayoutIndexInSingleplayer`, in the line `m_WorldLayoutsPerWorld` of `Game.ini` in your game config folder (`%LOCALAPPDATA%\DuneSandbox\Saved\Config\Windows`). If `Game.ini` has no such line yet, the whole line is added under its section. Every other line of the file is left as it is.
- It reads your save to check the Deep Desert (below). It writes to the save in three cases only: when vehicles of yours stand in the Deep Desert and have to be stored or moved out (see "What happens to your vehicles"), when the save holds Deep Desert markers (see "The Deep Desert markers"), and, only if you pass `--recycle-bases`, when bases of yours stand there and are recycled (see "Recycling the Deep Desert bases"). Nothing else in the save is ever changed.
- A random roll picks only from the **vetted layouts** (see "Which layouts a random roll may pick" below) and never the one that is set now. You can name any layout yourself with `--set N` (0 to 11); that choice does not use the vetted list. Layouts 12 to 17 are never written.
- The **game must be restarted** for a new layout to apply. The command never starts, stops or restarts the game.

## Which layouts a random roll may pick (the vetted list)

Not every layout is good to play on. In the game, layout 8 gave a Sandworm Threat of 100% on rock and inside labs, where other layouts did not. Layouts are therefore checked in the game one at a time, and a random roll picks **only from the layouts you have checked**, never the one that is set now.

- **The list is a plain text file**, `layout-roll-vetted.txt`, in the same folder as `layout-roll.json` (`%APPDATA%\tabr-tau`). Write the layout numbers separated by commas, spaces or new lines. A `#` starts a comment that runs to the end of the line. Example:

  ```
  # checked in the game: Sandworm Threat 0% on rock and in labs
  9, 11
  # 5   (not checked in a lab yet)
  ```

  The file is read at every run, so adding a layout needs no new build and no restart of anything.
- **No file?** Then the built-in list is used: **9 and 11**.
- **A file that cannot be used stops the roll.** If the file exists but is empty, holds only comments, cannot be read, or has anything in it that is not a whole number from 0 to 99, the roll **refuses**, prints a message that names the file and says what to do, and changes nothing (not `Game.ini`, not the save). Deleting the file brings back the built-in list.
- **Only real layouts.** A number that `Game.ini` does not list (the game numbers its layouts by their order in the layout line of `Game.ini`; the built-in line lists 18) is refused, and so is a listed number above 11, because a roll never writes layouts 12 and up.
- **Never the current layout.** If the vetted list holds nothing but the layout that is set now, the roll refuses and tells you to check another layout and add it.
- **For one run only:** `--vetted 9,11` uses that list instead of the file (the file is not changed). It is checked in the same way.
- **`--set N` is your own choice** and ignores the vetted list, so you can still go to a layout you want to check in the game. Do that, check it (set `Game.ini`, load the Deep Desert, shield on, read the Sandworm Threat on a rock island and inside a lab, and check that the trainer's rock matches the ground), and only then add its number to the file.
- **`--dry-run` shows it:** it prints the vetted list, where it came from (the flag, the file with its full path, or the built-in default) and the layouts the random pick may choose from. A real run prints the same lines and keeps them in the run record in `layout-roll.json` (`lastRun`).

## Checking a layout in the game (`vet`)

To add a layout to the vetted list you check it in the game first. `vet` does the file work for that, in three steps. None of them opens or changes the save.

1. **`tabr-tau layout-roll vet 8`** sets layout 8 in `Game.ini` so that you can look at it. It has the same refusals as a roll: the game must be closed (a Dune process, or a list of running programs that cannot be read, stops it) and the save must not be held by the editor or by a layout roll. It backs `Game.ini` up first, changes only the number, checks that `Game.ini` did not change while it ran, and reads the file back. Only layouts 0 to 11 that `Game.ini` lists are accepted. It then prints what to do:
   - start the game and load into the Deep Desert;
   - with the shield on, read the Sandworm Threat on a rock island and again inside a lab (it should be 0% in both);
   - check that the trainer's rock matches the ground you stand on.

   `vet` does not look for a base, a vehicle or your character in the Deep Desert: they stand on different terrain on another layout, and a roll checks for them but `vet` does not. If the layout is already set, nothing is written and no backup is made, and it still prints the steps.
2. **`tabr-tau layout-roll vet 8 pass`** or **`tabr-tau layout-roll vet 8 fail worm threat 100% on rock`** records what you saw. The game may be running for this step (it writes neither `Game.ini` nor the save). Everything after `pass` or `fail` is a note: it is kept on one line (line ends and other control characters become spaces) and cut at 200 characters.
   - The result is added as one line, newest last, to the plain text file `layout-roll-vetting.txt` in the same folder as `layout-roll-vetted.txt`, for example `2026-11-24 22:41 layout 8 fail worm threat 100% on rock`. Nothing in it is ever removed.
   - `pass` adds 8 to the vetted list. `fail` takes 8 out of it. If `layout-roll-vetted.txt` does not exist yet, it is created from the built-in list (9 and 11) with the change. If it exists, only the number changes: your comments, spacing and line ends stay as they are. A number in a comment is not a number. The file is replaced whole in one step (a new file beside it, then renamed over), so it is never left half written.
   - `pass` is refused for a layout above 11, and for one that `Game.ini` does not list (or when `Game.ini` cannot be read). `fail` may take out a stray number above 11 that a typo left in the list.
   - If a `fail` leaves the list empty, it says so: a roll refuses an empty list until a layout is vetted again.
   - If the list file or the results file cannot be used, nothing is written (neither file), and the message names the file.
3. **`tabr-tau layout-roll vet list`** shows layouts 0 to 11 with the last result of each (date and note), and whether a roll may pick it: **yes** when it is in the vetted list now, with "but not while it is set now" for the layout `Game.ini` sets, which a roll never picks. It only reads.

`vet` is not a roll: it ignores the once-a-week rule, writes no `layout-roll.json`, and does not recycle bases or move vehicles. When you are done checking, put the layout you want to play on back with `--set N`, or let the next roll choose.

## The safeguards

- **The game must be closed.** If a Dune process is running, or the list of running programs cannot be read, nothing is changed. The command says so and you run it again after closing the game (the scheduled run tries again the next Tuesday).
- **A backup first.** Before every change the file is copied beside itself as `Game.ini.layout-roll-YYYYMMDD-HHMMSS.mmm.bak`. The newest 10 of these copies are kept.
- **The file is replaced in one step** (a temporary file, then a rename) and read back to check it. If it is not what was written, the command says so and names the backup.
- **Once per week.** A roll is skipped if one already happened since the last Tuesday at 05:00. `--force` rolls again. A layout chosen with `--set` is your own choice and is not held back by this.
- **One layout roll at a time.** A real run takes the lock of the save (the same lock the editor holds while it has the save open) before it reads `layout-roll.json`, and keeps it until the run has ended and its record is saved. A second run that starts meanwhile, for example a manual one while the scheduled one is working, is **refused at once** with "another layout roll is running" and changes nothing: it does not wait, does not touch the other run's notes and does not overwrite its record. Run it again when the other one has ended. This also holds when the second run was given **another save** with `--save`: `layout-roll.json` is one record for every save, so a real run holds a second lock on it (a small file beside it, removed when the run ends) for as long as it holds the save. The same refusal appears when the editor window has the save open: close it first (before this, a roll with nothing in the Deep Desert went ahead with the editor open; now it needs the editor closed too, because the editor and the roll share the one lock). A `--dry-run` takes no lock (it writes nothing) and says so, so it keeps working while the editor is open or another run is going. A run that was given no save to check has nothing to lock: it is refused first of all, before any note of an earlier run is settled, and it reads `layout-roll.json` but never writes it. Because a refused run does not write `layout-roll.json`, it records itself in a separate append-only file beside it, `layout-roll.log` (`%APPDATA%\tabr-tau\layout-roll.log`): one dated line, "layout roll refused: the save is open in another TABR TAU window (or another layout roll is running): nothing changed", added before the run exits with an error (a log that cannot be written never changes the run; the log is renamed to `layout-roll.1.log` past 64 KB). Taking the lock also does what the editor does when it takes it: the lock file next to the save is made and removed, and stale temporary files next to the save and old working copies are swept; this now happens at the start of every real run, before the game-running check and the weekly rule.
- The last roll (time, old layout, new layout) and the last run are recorded in `%APPDATA%\tabr-tau\layout-roll.json`. The record of a random roll also says which vetted list was used and where it came from.

## The Deep Desert check

Before it edits `Game.ini`, and only when the game is closed and the weekly rule does not skip the run, the command looks at your save, in this order:

1. **A checkpoint.** A whole copy of the save file is made in the save's own backup folder (`tabr-tau-backups`, next to `game.db`), named with the time and `-layoutroll`, and checked by its size and SHA-256 against the bytes that were read. The automatic pruning of backups never counts or removes a checkpoint; instead, after each new checkpoint the editor keeps the newest 8 checkpoints and removes older ones, but only a `-layoutroll` file of this save that is over 7 days old (the newest is never removed, and nothing else in the folder is touched; a file that cannot be removed is logged and left). Database > Save backups lists it like any other backup (restore works as for the others). If it cannot be made or checked, nothing else is done.
2. **A read of the save.** The roll is **refused** (nothing is changed, the reason is printed and written in `layout-roll.json` under `lastCheck` and `lastRun`; two refusals go only to `layout-roll.log` instead: a run that meets the open editor or another layout roll, described under "One layout roll at a time", and a real run with no save to read) when any of these is true:
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

- **A Scout or a Sandbike is stored in the game's own vehicle backup.** The edit is the one the game makes itself, seen in a real before and after save: the vehicle's state goes from 0 to 2 and one row is added to the character's vehicle backup (the character, the vehicle and the vehicle's own customization). The vehicle keeps its place in the save but is out of the world, so the new terrain cannot bury it. The game's backup has room for one vehicle for each character, so only one vehicle can be stored this way, and only when the character has none there yet (a vehicle of any type already in it counts; then none is stored and every Scout and Sandbike is moved). If a Scout and a Sandbike both stand in the Deep Desert, **the Scout is stored and the Sandbike is moved**; if only one of them is there, that one is stored; of two of the same type the one with the lower vehicle number is stored. Every other Scout or Sandbike, and one whose customization the save does not record, is moved instead, as below, and the line for it says why.
- **Every other vehicle (a buggy, a crawler, a carrier, an assault ornithopter, a transport) is moved to the map the character is on**, by changing the vehicle's map and place, exactly as Bring vehicle does. It goes in front of the character's saved position and facing: a ground vehicle 10 m ahead, an aircraft 30 m ahead and 7 m up (it starts in the air and the game settles it). Several vehicles go in one line, at least 12 m apart and farther for big ones; the ground vehicles come first and the aircraft after them, each aircraft 20 m farther out than the ground vehicle at its step, so one never settles onto another. The places are chosen clear of base pieces, land claims and other vehicles. The character has to be outside the Deep Desert for this (the existing check refuses otherwise), because the vehicles go to the character's map.

All of it is one edit of the save. The rows written are read back inside the edit, and if anything read back differs from what was meant, the whole edit is undone and the save is left as it was. A run refuses and changes nothing when a vehicle has no clear spot (the message names that vehicle: face other ground and run again), when a vehicle's map is not recorded, or when the character's place cannot be told. The edit is written to the save only through the editor's own write path, which refuses while the game runs and keeps its own backup, and the written file is read back once more. If the editor window has the same save open, or another layout roll is running, the run refuses (see "One layout roll at a time"). Before the save is written, the run writes a note into `layout-roll.json` (under `vehicleNote`: the checkpoint, the file's SHA-256 as it was, and for each vehicle what is to be done), as the base step does: **a state file that cannot be written refuses the write**, and the note is replaced as soon as the save has been written. If the run is cut off, or the state file cannot be written after the write, the note stays, the run says so in capitals (**WARNING**, with the way back) even when no base was recycled in that run, and the next real run settles the note before anything else, the same way as for the bases: the save file as it was, the note is dropped; different, with every planned vehicle out as planned (a stored vehicle at the stored state with a row in the vehicle backup, a moved vehicle on the map it was put on), it is recorded as written, loudly; different, with none of them out, it is dropped, loudly; anything else (some out and some not, a vehicle that is neither where it was nor where it was put, a file that cannot be read, or a note marked `readBackFailed`) stops the run, which changes nothing, and after you have looked you remove the whole `vehicleNote` key (its value included) from `layout-roll.json` by hand. The run stops on that note every time until you do, and `--force` does not bypass it. A note that is settled as "recorded as written" is not lost: the earlier run's checkpoint, time and vehicles are kept in `vehicleHistory` of `layout-roll.json` (the newest 20), because the record of that run was replaced. If something fails after the save was written, the message says the save **was** changed and names the checkpoint that holds it as it was.

The roll's output lists every vehicle: what was done, from where, to where (words and coordinates). `--dry-run` lists the same, changing nothing.

## The Deep Desert markers (`--keep-markers` leaves them)

A single-player game never wipes the places the game has put on your map; only a server's storm does. So after a roll, the game's own map in the Deep Desert would still show places of the layouts before: a lab that is not there any more, for example. A real run therefore removes the Deep Desert's markers from the save, after your bases and vehicles are dealt with and right before it changes `Game.ini`:

- the Deep Desert's map markers (the places on the map),
- which of them your character has discovered, and
- the Deep Desert's resource fields (spice and the like) that the save keeps a state for.

All of the Deep Desert's markers go, whatever their type (a lab, a base or a taxi marker too), because an old lab marker is exactly the ghost that the map would otherwise keep showing after the layout changed. Nothing else is touched: not the markers or resource fields of the Hagga Basin or any other map, and not a base, a vehicle, an item or anything you own. The game puts markers on the map again as your character discovers places on the new layout (checked in the game on 2026-11-24: after the markers were removed this way, 43 appeared once the lab was found).

It is one edit of the save that is checked three times: the rows are counted before and after inside the edit (the Deep Desert must have none left and every other map must have the rows it had), the save is written the way the editor writes it, and the written file is counted again. If any check fails, the run stops before it changes `Game.ini` and says so. When the save was not written, it says that; when the save was written (by this step, or by an earlier step of the same run that recycled a base or moved a vehicle), the message says the save WAS changed and names the checkpoint of this run, which holds the save as it was, and how to restore it. Right before the step writes the save, the run records in `layout-roll.json` and `layout-roll.log` that it is about to, with the checkpoint's name, so a run that is cut off still leaves the way back; a state file that cannot be written then refuses the write. After the step the run checks again that the game is not running and that `Game.ini` is as it was read, and only then copies `Game.ini` and changes it. A save with no Deep Desert markers is not written for this step.

`--dry-run` says how many it would remove, for example "Dry run: would remove the Deep Desert markers from the save ...: 3 marker(s), 2 discovered row(s) and 5 resource-field row(s)", and writes nothing. A real run counts again after the bases and vehicles were handled (a recycled base takes its own discovered row with it), so the counts it prints can be a few lower than the dry run's. It prints them, and writes them into `layout-roll.json` (`lastCheck.markers` and the last run) and as one line in `layout-roll.log`.

To keep your markers for a roll, add `--keep-markers`:

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" layout-roll --keep-markers
```

For the weekly task, register it with `-KeepMarkers` (see "The weekly scheduled task"). This step is not checked in the game by the change that added it; look at the map after the first real run.

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
5. The bases are recycled **one at a time, in the order of their numbers** (the lowest number first, so the plan and the log are the same every time). Each is a recycle of its own, with a Solido, on the save as the bases before it left it: each is asked about again at that point, so what the bases before it used up (the Reserve, the backpack slots, the Solido devices) is seen. Each base gets a **new** Solido device (a filled Solido is never reused; a blank device of your own is used for one base only). After each base the run reads it back and checks it: the base is gone, its Solido is in the device and linked to it, and the Reserve holds exactly what moved and was refunded. The Reserve is then moved into the boxes `DD-1`, `DD-2` ... up to `DD-6` (see "Space", below) and must be empty. Only then does the next base start. All of this happens on a working copy: **nothing reaches the save file until the end.**
6. The run first writes a note of what it is about to do into the state file (the checkpoint, the bases, how the save file looked): **a state file that cannot be written refuses the write**, and so does a save file that cannot be read first. Then the save is written **once**, through the editor's own write path, and the written file is opened afresh and read back: the recycled bases are gone, every Solido is in place, every stack that was moved is in a box and the Reserve holds what the working copy ended with (nothing, when the last move was made), and no base other than those still waiting stands in the Deep Desert.
7. Your vehicles in the Deep Desert are then taken out (as above), and only when the Deep Desert is looked at once more and holds no base is the layout rolled.

A failure at any base before step 6 (a base is refused after the bases before it were recycled, a read back differs, the write does not happen) throws the working copy away: **the save file is not written**, and the message says which bases were done on the copy, which failed and which were not tried, and names the checkpoint. A write that is not reported as saved but **did** replace the file (it could not be read back in time) is treated as a changed save too: the run compares the file with how it was before the write, and when it differs it opens the file afresh and checks it against the plan. Only a file that reads back as planned is reported as "the bases were recycled"; a file that is different and does **not** read back as planned is reported as changed **by something that is not known** (it may be another program), with the checkpoint and the way back, and the bases are not recorded as recycled (the same is done for the write of the vehicles). A failure after the write (the file does not read back as written) says the save **was** changed, names the checkpoint and gives the command to put it back (Database > Save backups, or `Copy-Item -Force "the checkpoint file" "the game.db file"` with the game closed), and the layout is not rolled. The same note is added to any later failure of the run, for example a Game.ini that cannot be written.

### Space: the Reserve must be empty before each base

A recycle sends everything to the **Reserve**: the items the base stored, unchanged, and the refund as new stacks. The refund is the **full table cost** of every piece and placeable, in the Deep Desert too (#1211): the game's half-cost rule for building there is not applied. The rule used here is the operator's: **no further recycle can happen until the Reserve is empty.** So before each base the run needs an empty Reserve, and each base fills it again; the run empties it into your storage boxes in between. The run recycles the largest number of bases, in order, that this, the room in the boxes and the room for the Solido devices allow:

- **After each base the run moves the whole Reserve into your storage boxes**, named exactly **`DD-1`**, **`DD-2`**, **`DD-3`** ... up to **`DD-6`** (capitals included) in the game, with the same move as Player > Bases > Move the Reserve into storage (see docs/recycle-a-base.md): every stack whole and unchanged, in the order the Reserve holds them, each into the first box that has a free slot and room by volume. It is done on the same working copy as the recycles, so all the bases of a run fit in one run when the boxes have the room. **Which boxes:** the run uses as many as stand **in a row from `DD-1`**, in numeric order, first fit (at least `DD-1`; each holds 1,000 slots and 50,000 volume in the saves seen, and a Deep Desert base refunds in full, so one base can bring more than two boxes hold). The scan stops at the first number that has no box: with `DD-1`, `DD-2` and `DD-4`, `DD-4` is **ignored** and the run says so ("Ignored: DD-4 (DD-3 is missing ...)"), so a typo cannot silently skip a box. A name is trimmed and matched exactly (`DD-10`, `DD-0`, `dd-3` and `DD-03` are not boxes). When a number is missing, the run also lists the Developer storage boxes that have other names (for example "D-3"), and with `DD-1` alone it warns "only DD-1 was found; DD-2 .. DD-6 are optional"; that warning stops nothing. The boxes must be Developer storage containers that stand in a base of yours outside the Deep Desert; a box that stands in one of the bases being recycled is never used. A Reserve that an earlier run left full (for example because the boxes were full) is moved first, before the first base. After the **last** base the Reserve is moved too, so that next week starts with an empty Reserve.
- **When a box cannot be used** the run recycles **nothing** (0 of N, nothing exported, nothing changed) and says why. That is the case for a box that is missing, not named exactly, named twice or found twice, in a base of this run, in a base on the Deep Desert or in no base of yours, and a container of another kind than Developer storage (a chest, say) that carries one of the names `DD-1` ... `DD-6` (it is refused by name, and the boxes behind it are not used in its place), and, among others, one that is only a hologram, is in a base that was picked up or is part of a base backup, is in a base that is not yours, has no slot count above 0 recorded, or holds items in places the game would not use (the names are in `internal/ops/movereserve.go`, in the refusals of the box lookup and of the move). The boxes are asked about first, before base 1, even when the Reserve starts empty, because every base after the first needs them.
- **When the boxes have no room** the run treats it like a lack of room for a base: it recycles the bases that fit, **stops before the roll**, and says which bases are left and why ("1 of 2 base(s) recycled; base N was not, because the Reserve ... does not fit into the boxes DD-1, DD-2 ... DD-6 (as many as run in a row from DD-1): needed ..., free ..., short by N stacks (volume ...) ... add DD-3"; at `DD-6` it says that all six boxes are in use). Make room in the boxes (or empty the Reserve in the game) and run again: a re-run moves the Reserve first and picks up where the last one stopped. A base that was recycled is simply gone, so a re-run is safe. The roll happens in the run that recycles the **last** base (or in a run that finds no base left), never while a base stands, however many runs it takes. If it is only the Reserve after the last base that the boxes cannot take, the base is still recycled and the roll goes on (no base is left to hold back); the run then warns that the Reserve still holds its stacks, and the next recycle will start by moving it.
- with something in the Reserve at the start that the boxes cannot take, **no** base is recycled (0 of N), nothing is exported, and the run says why.
- **A Reserve left over is moved before the room for the first base is asked about.** The run moves the leftover first and then recycles the first base, so the question is put the same way before anything is exported: when the Reserve has a slot limit that the leftover and the first base together would pass, but the first base alone fits once the leftover is in the boxes, the run goes on (it used to give 0 of N). When the base does not fit even in an empty Reserve, it is 0 of N, nothing is exported, and the message says "even after the Reserve an earlier run left is moved into the boxes". The question is asked on a working copy that is thrown away; nothing reaches the save file.
- **The boxes are asked about before base 1 even when the Reserve starts empty**: a box that is missing, misnamed, in a base of this run or on the Deep Desert outside any base is 0 of N, nothing is exported or changed, and the run says why. Otherwise the first base could be recycled and the run find out afterwards that its Reserve has no place to go.
- A character with **no Reserve** at all is refused in the question, before anything is exported.

`--dry-run` asks for the move after each base too: the output holds the plan for each base (what would move into which box) as well as the recycle plan, and nothing is written.

The rule ("empty before each base") is the operator's and is not read from the game: the editor's own recycle only refuses when the Reserve has a slot limit that the base would pass (the saves seen record none), and the Reserve's volume is not checked at all. The move into the boxes has not been checked in the game by this program.

The Solido devices need room too. A new device goes in the backpack (the loadout when the backpack has no room), one slot each, and the run keeps **two** slots free there after them. When the volume limit of the backpack or loadout leaves no room for a device, or the slots are short, that base does not fit: the run recycles the bases before it, stops before the roll, and names the destination and what is short ("backpack: 1 slot(s) needed for Solido devices plus a margin of 2, 2 free (short by 1)").

`--dry-run` prints all of it: the Reserve rule as its own line, the room now, each base (pieces and placeables deleted, stored stacks moved, the refund item by item, which Solido device and where), how many of the N bases would be recycled (k of N), the Solido devices and the refund of all of them together, and a line per destination with what is needed and what is free. It changes nothing: no checkpoint, no export, no state file, no write to the save. It plays the recycles on a private copy and throws it away.

### Where things go, and what is recorded

- The stored items and the refund go to the **Reserve**; a Solido device goes to the **backpack** (the loadout when the backpack has no room). The recycle itself touches no vehicle: it refuses a base only when one of the base's own actors is a vehicle, and a vehicle of yours that merely stands in or beside the base is left to the vehicle step of the roll (as above, after the bases).
- For every base the run prints what the recycle itself said about it: its summary lines (the Solido, the refund, the respawn points deleted, holograms and Developer storage containers, the water that is dropped) and its warnings (for example placeables with no support, or pieces that belong to no base).
- `layout-roll.json` records the run under `lastCheck`: the checkpoint's name, SHA-256 and full path, the export folder and files, and for every base recycled its number, name, map, pieces, stored stacks, refund, **Solido device and blueprint numbers**, and the room table. Each move of the Reserve into the boxes is recorded too (after which base, how many stacks and items, and the boxes with their inventory number and stack count afterwards), under `reserveMoves`. The state file is written **before** the save is written (a `writeIntent`: the bases about to be recycled, and the save file's SHA-256 from before) and again as soon as the save has been written, before it is read back, so even a run that is cut off at either point leaves the checkpoint on record. A `writeIntent` that is still there on a later look says the run ended without recording how the write went: look at the save. If the state file cannot be written after the save was written, the run says so in capitals (**WARNING**) and in the error, with the way back. At the start of the next real run the note is settled before anything else: the save file's SHA-256 is compared with the one in the note. Equal: the write never happened, the note is dropped and the run goes on. Different: the bases the note planned are looked for in the file; all gone and verified: they are recorded as written (loudly); none gone: the note is dropped (loudly); anything else, a planned base that is gone without its Solido as planned (said as a file that was read and does not hold the plan), a file that cannot be read, or a note marked `readBackFailed` (the run that wrote it opened the written file and it did not read back as planned: such a file is never accepted by looking at it again): the run STOPS and says what to look at, and after you have looked you remove the entry marked `unconfirmed` from `recycleHistory` by hand (the run stops on it every time until you do, and `--force` does not bypass it). The run never rolls on an unconfirmed write. A dry run settles nothing and says "would be dropped" or "would be recorded as written". The unconfirmed notes are not counted in the history's cap of 20. `reserveMoves` lists only the moves that were written. If the Reserve is still full after the last base, the warning is kept in `lastCheck.reserveNote` and in the `lastRun` outcome, so it is still visible the next day.
- `layout-roll.json` also keeps a short **history** under `recycleHistory`, one entry for each run that recycled bases (the last 20): the time, the checkpoint's name and full path, the numbers of the bases and the ids of their Solido devices. `lastCheck` is replaced by the next run; the history is not, so a series of runs (the boxes full, make room, run again) keeps the name of every checkpoint, but only the newest 8 `-layoutroll` files are kept in the backup folder (older ones over 7 days old are removed), so a checkpoint far back in the history may no longer be there. The entry of a run is first written before the save (marked as not confirmed) and replaced once the write is confirmed. The run prints the earlier checkpoints at the start and the whole list after it recycled; the note after a stop lists the checkpoints of the **earlier** runs, never the run's own.

### Going back

Close the game and restore the checkpoint named in the output (Database > Save backups, or the `Copy-Item` command the run prints). That puts back the bases, the Reserve and the backpack as they were **before that run**. When bases were recycled over several runs, the checkpoint of the **earliest** run in the history is the full way back to before the first base was recycled (if it is still in the folder: the newest 8 are kept); restoring a later one goes back only to before that run. The list is in `recycleHistory` of `layout-roll.json` and in the output of each run, and the files themselves are in the save's backup folder (the ones with `-layoutroll` in the name, never removed by the automatic pruning). The exported base files are extra copies of each base.

### After the first real run: check in the game

In the game: the **Solido is in the backpack** (one per recycled base, each with its base's pieces), the **stored items and the refund are in the boxes `DD-1`, `DD-2` ...** (the Reserve is empty), and the bases are gone.

## Try it without changing anything

Run this first. It prints the layout now, the last roll, what the Deep Desert check finds in your save, which vehicles it would store and which it would move and to where, and what it would do next (take the checkpoint, export the bases, refuse or roll). It writes nothing: no checkpoint, no vehicle edit, no base file, not even the state file. It reads a private temporary copy of your save to do so.

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" layout-roll --dry-run
```

Then, with the game closed, one of these:

```
& "$env:USERPROFILE\Downloads\tabr-tau.exe" layout-roll
```

A random layout from the vetted list, once per week.

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

To have the weekly run leave your Deep Desert markers in the save (it removes them otherwise, see "The Deep Desert markers"), register it with `-KeepMarkers`; the task action then reads `layout-roll --keep-markers`. It can be combined with `-RecycleBases`.

Remove it:

```
& "$env:USERPROFILE\tabr-tau\scripts\layout-roll-task.ps1" -Remove
```

**Close the editor before the 05:00 run.** The editor holds the save for as long as its window is open, and a layout roll needs the same lock: a run that meets the open editor (or another layout roll) is refused before it reads `layout-roll.json` and changes nothing. It is not retried (the task is not run again that week), it appears only as one dated line in `%APPDATA%\tabr-tau\layout-roll.log` and as a Last Run Result that is not 0 in Task Scheduler, and nothing is recorded in `layout-roll.json`. After a Tuesday on which the editor was open, look at `layout-roll.log`, close the editor and run `tabr-tau layout-roll` by hand.

Keep `tabr-tau.exe` where it is: the task points at that file. Removing the task does not touch `Game.ini` or its backups. If the game is running at 05:00 the roll does not happen that week; the file `layout-roll.json` says "the game is running" under last run. Because the scheduled run also moves vehicles (and writes the save when it does), leave the character facing open ground with room in front whenever you log out on a Monday night.

Nothing here has been checked in the game by this program yet: the setting itself was measured by hand in the game; storing a Scout was seen in a real before and after save, and moving a vehicle from the Deep Desert to the Hagga Basin by editing its place worked in a real test; storing a Sandbike this way, and where a moved vehicle ends up against the terrain, have not been seen. This command is checked with tests on temporary files, made-up saves and copies of real saves.

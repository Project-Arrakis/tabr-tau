# The live-save gate no longer rests on the text of the game's log alone

Issue #612 (audit group D2 of `docs/audit/2026-11-13-delta-register.md`, and finding G-007 of the full audit of 2026-11-14). Single player only. This note says what the gate
read, what changed, and what is not known and needs a check in the game.

## What the gate read

While a game process exists, the editor decides "a world is loaded, so the edit waits" from the game's log (`Saved/Logs/DuneSandbox.log`): a browse line that ends in
`?listen` starts a session, a `?closed` or remote browse (or an exit line) ends it, and the line of the database write gives the time of the game's last write. If a game update
rewords those lines, or the game is started with another launch URL, no line matches, the state is empty, and the old code went on to the open-file check and answered "menu":
an edit was written while a world was loaded, and the game overwrote it about every 20 seconds.

## What changed

| Row | Change |
|---|---|
| D2-1 | A second signal that does not depend on any text: the save file's own modification time (the newest of the file and a `-wal` or `-journal` file next to it, when one exists). While a game process exists, the file must have been unchanged for as long as the log's rule says the game has been quiet: 60 seconds after a clean exit (the end marker and exactly one later write), 3 minutes in every other case where the log gives evidence. The editor's own writes move the file's time too, so the editor remembers the size and time of the file it installed (taken right after the rename, dropped when the read-back does not confirm it) and a file that is still that file is not counted as the game's. A time that cannot be read, or that lies in the future, is not quiet. |
| D2-1 | "No marker ever seen for a running game" is unknown, not "menu". When the log can be read and holds no session line at all (no start, no end, no exit, no write line), the file must have been unchanged for **10 minutes** (`noMarkerQuietFor`); the game is open and nothing in its log says it is idle. |
| D2-2 | `menuQuietAfterWrite` goes from 23 to 60 seconds. The short wait (30 s after the end and 60 s after the last write) is used only when the log shows **exactly one** write after the end marker, which is what 19 exits from a world showed. With two or more writes after it, the marker was not the end of the world (a local `?closed` browse while the world stayed loaded, a map load without `?listen`), and the longer rules apply: 3 minutes since the last write and 6 minutes after the end. |
| D2-3 | An end marker counts only at the start of the line's text, after the logger's prefixes and the category, as the game writes it: a browse line (strict reading), `LogWindows: Log: FPlatformMisc::RequestExit(`, `Log: Log file closed` and `LogNet: Display: Delaying Travel (net mode Listen Server): ?closed`. A line that merely contains the words is not an end. Starts and the write line keep the wide reading (a spurious start or write makes the editor wait, the safe direction). A real end line that does not look like these is a missed end, which also only waits longer. The pin of the editor's tracker over the seeded corpus, and a test that it is never less active than the kit's wide reading, keep this honest. |
| G-007 | The automatic pass at launch and its retries (`runPass(false)`, `retryWaitingPass`) are unattended: they wait for **every** game process to be gone (`save.UnattendedStateFor`), whatever the log and the file say, as before #499. The edit the pass makes carries the rule with it: it is named `save.AutoPassDesc`, and `Save.gate` (asked by `Flush`, `commit` and again right before the rename in `installBlob`) applies `UnattendedStateFor` whenever the queue holds nothing but such edits. So a pass whose own write is refused (the game started meanwhile, the rename was held, no room) stays queued and is not written by the next tick of the autosave, nor after a restart that restores it from the journal, while a game process exists. A hand edit in the same queue makes it a hand queue (a person is at the page). A pass asked for by hand (**Run the automatic pass now**) is named `save.PassByHandDesc`, has a person looking, and keeps the gate of an edit made by hand. |
| Observed quiet | The quiet is observed, not only computed from the clock. A computer that slept, or a clock set forward, makes an old file time look like a long quiet. The editor remembers, per save, when it last evaluated the gate with a game process present and the file's time then; the first evaluation, one after a gap of more than 30 seconds (a clock that went backward is a gap), one after a file time that changed other than by the editor's own write, or an unreadable time, starts a watch, and a write waits until the watch has lasted `observeQuietFor` (60 s). The status poll and the 3-second autosave tick keep the watch going; with the page closed and nothing queued there is no evaluation, so the first edit after a long idle waits a minute. |
| Mode | A write let through while the log asserted nothing about a session is answered with the mode `unknown_quiet`, not `menu`, and the check says so; every consumer blocks or allows as before. The unknown state is derived from what the log asserts now (no session active, no end, no write line), not remembered from lines read before. |
| D2-4 | No change needed (the register says so: a copy is told apart by path, and the function answers "the game's own" when in doubt). |
| D2-5 | No change to what is guarded (the test hooks are guarded by `TestTestHooksAreNotCalledByProductionCode`). Two of them now also stand in for the file's time (`GameAtMenuForTests`, `GameJustLeftWorldForTests`), and both switch the watch off (`observeQuietFor = 0`), as the tests of the other rules do (#779). |

## Why these numbers

- 60 s after the last write: a little under three of the largest gap between two writes seen while a world is loaded (22.7 s). Waiting costs a minute; a wrong guess costs the edit.
- 10 min with no marker: longer than the 3-minute flat quiet time, and longer than the "about every five minutes" autosave an older note mentions. Nothing in the log can tell the editor that the game is idle here, so only a long-unchanged file counts.
- The file-time check never lets anything through that the log blocks: it is an additional condition, checked after the log's rules, before the open-file check.

## What the old rule allowed and the new one does not (all in tests, on synthetic log lines and temp files)

- Game open, log readable, no line matches, file written 20 seconds ago: waits (was: written).
- Game open, log says the session ended an hour ago, file written 10 seconds ago: waits (was: written).
- A `?closed` browse while the game goes on writing every 20 seconds: no short wait (was: written 30 s after the browse).
- A log line of another category that holds the words of a closed browse: not an end.
- The unattended pass with the game open at its menu: waits for the game to close (was: ran).

## Not known: needs a check in the game

These cannot be settled from the code or from the logs checked so far. The margins above are chosen so that a wrong answer waits longer, not writes sooner.

1. **How often the save file's own modification time moves while a world is loaded.** The log says the game writes its database about every 20 seconds, but the editor has not
   compared that with the file's time: the game may write the file less often than it writes the database, or keep its writes in a sidecar file the editor does not know
   (the editor looks at `-wal` and `-journal` next to the file). If the file's time moves only every few minutes, the 60-second and 3-minute margins are then shorter than a
   file's quiet gap while a world is loaded, and the log's rules are the only guard in those cases. Check: with a world loaded, watch the file's time for ten minutes and note
   the gaps.
2. **What the log looks like when the game is started at its menu and no world has been loaded.** If the log then has no session line, the editor waits until the file has been
   unchanged for 10 minutes before an edit is written (it is queued with the reason meanwhile). If the file has not been written for longer than that, there is no wait. Check:
   start the game, stay at the menu, and read the log for the lines the editor knows.
3. **Whether the game holds the file open at its menu** (unchanged from #449): the open-file check is still the last step.
4. **Whether the editor may relax the unattended pass again** (G-007): once points 1 and 2 are known, the pass could be allowed with the game open at its menu when the file's time
   is older than the margin. Until then it waits for the game to close.

## Residual risk

- The pass checks "no game process" before it builds its edit, and the edit is held to the same rule at every write after that (see G-007). A game started between the check and
  the write is caught by the write's own gate, not only by the check; what is not caught is a game process the process list does not show, and a session that begins in the seconds between the last check and the rename (the check itself, the read of the file and the rename take time; no rename can close that window, and a game save inside it is caught by the read-back, which leaves the game's file alone). The automatic write asks the gate again after the encode and before its backup, so a refusal there takes no backup (#779).
- A pass edit queued together with a hand edit is written under the gate of a hand edit: the queue cannot be split, and a person is at the page. Accepted (#779): the autosave's next tick, not only the request that made the hand edit, then writes the whole queue under that gate, also after a restart that restores it, so the pass's edit can be written with a game process present. Making any queue that holds a pass edit unattended whenever the write is not triggered by a request would need Flush to know what called it, and would hold a person's own edit back for as long as the game is open.
- The own-file exemption (the file the editor itself installed is not counted as the game's) also applies in the unknown state: after the editor's own write the file's time says
  nothing, so a second edit seconds later is not held back by it. The first write of such a run needed a file unchanged for 10 minutes, and a game that writes afterwards changes
  the file, which stops matching the editor's record.
- The watch holds a minute after a start, a sleep or a clock step; a clock set forward by less than 30 seconds between two evaluations is not seen.
- A game that rewrites the file less often than every 10 minutes while a world is loaded, with a log that matches nothing, is not caught. That needs both a reworded log and the
  file's time failing, and point 1 above would show it.

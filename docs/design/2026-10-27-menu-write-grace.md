# How soon after leaving the world may an edit be written (#470)

Status: built. Rule in `internal/save/spsession.go` (`processState`, `menuWriteGrace`, `menuQuietAfterWrite`); it refines the rule of #449/#458.

## Problem

With a Dune process open, an edit was written only after the game had not written its database for `dbQuietFor` (3 minutes), so an edit made right after leaving the world
waited about three minutes at the menu. The 3 minutes was a margin, never a measurement of how soon after leaving the world the game really stops writing.

## What was measured

Source: the game's own log (`Saved/Logs/DuneSandbox.log` and the ten rotated logs kept beside it), read through the same parser the editor uses (`spTracker`), so "session
ended" is the editor's end marker (`Delaying Travel (net mode Listen Server): ?closed`, `Browse ... ?closed`, or `FPlatformMisc::RequestExit(` when the player quits from inside the
world) and "database write" is the line `SaveDatabase - Database saved to disk`. Times are the log's own (UTC), to the millisecond. The logs come from one computer, single player, game build 1.5.15.x.

About twenty exits from a world were found in those logs. For each, the gap from the end marker to the game's write after it was taken. The exits were of three kinds: quit from inside the
world, to the menu and then a new session, and to the menu and then quit (or the log ends).

Gap from the end marker to the game's last write, in seconds:

| Group | Minimum | Median | Maximum |
|---|---|---|---|
| All exits | 1.50 | 2.46 | 6.87 |
| To the menu, then a new session | 1.59 | 2.49 | 4.11 |
| To the menu, then quit or the log ends | 1.50 | 1.91 | 2.88 |
| Quit from inside the world | 4.89 | 5.11 | 6.87 |

What the numbers say:

- Every exit had exactly one write after the end marker, 1.5 to 6.9 seconds later (the game flushes persistence, then writes). In a few of them the periodic write came less than 1.5
  seconds before the end marker; it is not the one counted.
- No exit had a second write while the game sat at the menu. Menu stays of several minutes were all silent until the next session
  opened the database (the log then shows `Using persistence implementation Sqlite`, `ApplyPatches` and an immediate write, a few hundred milliseconds before the `?listen`
  start marker). The write at the start of a session is not a menu write; if the editor reads it before the start marker, it counts as a late write and the edit waits 23 s after it (see below).
- While a world is loaded the game writes about every 20 seconds, not every 60: the line says "Saving again in 60 seconds", but thousands of consecutive in-session writes in these logs
  are 20.1 to 20.2 s apart at the median, 20.4 s at the 95th percentile, 22.7 s at most. (The text of the README and of the waiting message used to say 60 seconds; it is
  corrected.) That makes one save interval 22.7 s at its longest.
- File times: the log has no file times, but a spot check of the live save file agreed with the log (the file's modified time equalled the time of the last `Database saved` line
  to within a few milliseconds), so the log's write times are the file's write times. The rotated logs cannot be matched to file times (only the current file's time exists). The game's
  autosave ring is written about every 5 minutes (the `CreateAutoSave` lines) and says nothing about leaving the world. No watch of the file through a menu stay was made
  for this change (the game was in use); the menu stays above come from the logs. The in-game check below covers it.

Limits: one computer, a few days, single player, one game build; about twenty exits is a small sample, which is why the grace below is about four times the largest gap seen and not close
to it, and why every case without both the end marker and the write after it keeps the long waits.

## The rule

> Changed 2026-11-15 (#612, audit D2; see `docs/design/2026-11-15-live-save-gate.md`): `menuQuietAfterWrite` is now **60 seconds**, not 23 (23 s was 0.3 s above the largest write gap seen); the short wait applies only when exactly one write follows the end marker (more than one means the marker was not the end of the world); and the save file's own modification time must be older than the wait as well. The numbers below are the measurements; the waits that follow from them are in the new note.

When the log shows the session ended AND a game write after the end marker (compared by whole log seconds, strictly later: a write in the same second as the end marker is the
periodic one before it, and is not taken for the last):

- the edit waits until `menuWriteGrace` = **30 seconds** after the end marker: the largest gap seen (6.9 s) plus one save interval (22.7 s) is 29.6 s, rounded up; and
- until `menuQuietAfterWrite` = **23 seconds** after that write (one save interval, 22.7 s, rounded up). With the writes seen this never binds (30 s after the end is at least 23 s
  after a write that came 6.9 s after it); it holds the edit back, and restarts the wait, if the game writes later than anything measured.

Every other case is as before, because the evidence is missing or says something else:

| What the log shows | Wait |
|---|---|
| a single-player session is active | none: never written |
| the log cannot be read, or the process list cannot be read | none: queued |
| the session ended and exactly one later write is seen | 30 s after the end and 60 s after that write (23 s before #612) |
| the session ended and two later writes are seen, both within 10 s of the end marker (#1122) | the same: 30 s after the end and 60 s after the last write |
| the session ended and any other number of later writes is seen (a third, or one later than 10 s after the end together with another) | as "no session end": 3 minutes since the last write, then 6 minutes after the end (#612) |
| the session ended and no later write is seen | 6 minutes after the end (`endedFallbackGrace`, not measured) |
| no session end in the log (for example the game was started at the menu) and a write is seen | 3 minutes since the last write (`dbQuietFor`) |
| the game is closed | written at once |
| another program has the file open, or that cannot be checked | queued (unchanged) |

The waiting text counts down in words ("about 25 seconds more"; seconds under two minutes, minutes after). The grace is not a promise that the game cannot hold the save at the
menu: the file-open check and the rest of the #449 rule are unchanged, and what the game does with rows it read earlier is still not established.

## Re-measured 2026-11-23 (#1122): two writes just after the end

The operator saw the 3-minute wait again after leaving the world. The log of 2026-11-22 00:14 UTC shows why: the game's periodic write (every 20 s while a world is loaded) fell
0.4 s after the end marker, and the write the game makes when it leaves came 3.6 s after it, so there were TWO writes after the marker and the rule "exactly one" (`maxWritesAfterEnd`)
refused the short grace. Read-only, through the same state machine as the editor (end marker, then every database write until the next session start or the end of the log), over every
log kept on this computer (the current log and the ten rotated ones, 2026-11-21 to 2026-11-23, game build 1.5.15.x). Times are the log's own (UTC); "offset" is seconds after the
end marker; "before" is how long before the marker the previous write was.

| End marker (UTC) | How it ended | Writes after the marker (offset, s) | Before (s) |
|---|---|---|---|
| 2026-11-21 01:14:39 | quit from inside the world | 3.8 | 3.4 |
| 2026-11-21 19:58:14 | to the menu | 1.9 | 10.5 |
| 2026-11-22 00:14:22 | to the menu | **0.4 and 3.6** | 20.1 |
| 2026-11-22 00:22:36 | quit from inside the world | 3.0 | 13.7 |
| 2026-11-22 05:50:53 | to the menu | 2.6 | 6.5 |
| 2026-11-22 08:03:33 | quit from inside the world | 1.8 | 1.9 |
| 2026-11-22 21:31:09 | quit from inside the world | 3.4 | 4.9 |
| 2026-11-23 00:04:13 | to the menu, then the log ends | 3.2 | 13.3 |
| 2026-11-23 06:52:46 | quit from inside the world | 3.1 | 11.2 |
| 2026-11-23 18:18:13 | quit from inside the world | 3.2 | 12.7 |

What the table says:

- Ten exits: nine with exactly one write 1.8 s to 3.8 s after the marker, one (00:14:22) with two writes, 0.4 s and 3.6 s after it. No exit had a write between 3.8 s and the start of
  the next session. (Three exits are followed by a session start in the same log; the write that precedes the start, 112.6 s, 436.5 s and 374.3 s after those exits, is the write the game
  makes when it opens the database for the next session, 0.04 s to 0.05 s before the `?listen` line: it is not a write at the menu, and the editor resets its count at the start.) With the 19 exits of
  #470, the latest write after any marker was 6.9 s.
- The extra write of 00:14:22 is the periodic one: the previous write was 20.1 s before the marker, so the next periodic write was due 0.4 s after it (the periodic interval is 20.1 s
  at the median). In the other nine exits the next periodic write was due 6 s to 18 s after the marker and did not come: the game stops the periodic write when it starts to leave. So the
  periodic write shows up after the marker only when it falls in the first moments of the exit, which is rare but not impossible: the rule "exactly one" was brittle, not newly broken. No exit
  showed no write at all.
- A third write, or one later than 10 s after the marker, has not been seen at any exit.

The rule since #1122: when the session ended, a write after it is seen, and the number of writes after the marker is one, or two that BOTH fall within 10 s of the marker
(`maxWritesNearEnd` = 2, `nearEndWindow` = 10 s: the 6.9 s of #470 plus margin), the exit is the normal one and the 30 s and 60 s waits apply. Everything else is as before: a second write
later than 10 s, three or more writes, no end marker, and no write after the marker keep the 3-minute and 6-minute waits. `dbQuietFor` itself is not shortened: it is the safety net for the #419 case.

Limits: ten exits from one computer in three days; one of them had two writes, so the two-write shape rests on a single observation (with the periodic interval to explain it).

## Not changed

A world in play is never written. The automatic pass at launch used the stricter rule (no Dune process at all) until #499, had this one from #499 to #612, and has the stricter one again since #612 (unattended: it also waits for every game process to be gone); a pass a person starts with **Run the automatic pass now** has this one when it runs at once; if this rule blocks it, it is queued and runs later under the stricter unattended rule (#1135). The 3-minute and 6-minute waits remain for the cases above. The
#419 case (a write 23 seconds after leaving the world, 20 seconds after the game's last write) is still refused: 23 s after the end is before the 30 s grace.

## In-game check for the operator

Leave the world to the menu, press Create or Apply (any edit) at once: the page should say how many seconds more (about 30 at most), and the edit is written by itself within about
30 seconds of leaving the world; then load the character: the game should start it without a repair prompt and the edit should be there. Try once with a longer stay at the menu
and one edit made after more than a minute (it should be written at once).

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
| the session ended and a later write is seen | 30 s after the end and 23 s after that write (new) |
| the session ended and no later write is seen | 6 minutes after the end (`endedFallbackGrace`, not measured) |
| no session end in the log (for example the game was started at the menu) and a write is seen | 3 minutes since the last write (`dbQuietFor`) |
| the game is closed | written at once |
| another program has the file open, or that cannot be checked | queued (unchanged) |

The waiting text counts down in words ("about 25 seconds more"; seconds under two minutes, minutes after). The grace is not a promise that the game cannot hold the save at the
menu: the file-open check and the rest of the #449 rule are unchanged, and what the game does with rows it read earlier is still not established.

## Not changed

A world in play is never written. The automatic pass at launch used the stricter rule (no Dune process at all) until #499; it now has this one. The 3-minute and 6-minute waits remain for the cases above. The
#419 case (a write 23 seconds after leaving the world, 20 seconds after the game's last write) is still refused: 23 s after the end is before the 30 s grace.

## In-game check for the operator

Leave the world to the menu, press Create or Apply (any edit) at once: the page should say how many seconds more (about 30 at most), and the edit is written by itself within about
30 seconds of leaving the world; then load the character: the game should start it without a repair prompt and the edit should be there. Try once with a longer stay at the menu
and one edit made after more than a minute (it should be written at once).

# The clock of a Landsraad completion time (#465)

Status: built. The code is `internal/ops/landsraad_clock.go` and the two complete routes in `internal/ops/landsraad.go` and `internal/ops/landsraad_chosen.go`; the tests are in
`internal/ops/landsraad_clock_test.go`. Not checked in the game yet (see the last section).

## Problem

Complete (one house) and Complete chosen wrote `landsraad_tasks.completion_time` as the Unix time of the moment the button was pressed (about 1.79 billion in late October 2026). The
game's own rows in the same column hold a number of a few hundred thousand at most.

## What the column holds

Read from 46 copies of saves that hold Landsraad boards (three different worlds; the 46 include several copies of the same save at different moments):

* **Game-written completed rows: 203 distinct rows, every one inside the window of its own term** (`landsraad_decree_term.start_time` to `end_time`). Example, the older character's
  save, term 10 (305,395 to 341,395): House Sor, board index 5, goal 4,000, completed 1, winner 2, `completion_time` 312,412. Term 9 (275,678 to 305,395) holds 20 game-written rows
  from 287,978 to 305,093; term 3 (59,678 to 95,678) holds 22 from 72,349 to 92,361.
* **Rows the editor wrote: 40 distinct rows, none inside any window.** The same save: 24 of the 25 houses of term 10 hold 1,793,117,618 to 1,793,118,379 (Unix time of 2026-10-27
  16:26 UTC), and 16 more are spread over terms 1, 2, 4, 5, 6 and 7 (for example 1,792,715,494 in term 1).
* **Open rows: 147 distinct rows** (and all 48 open rows of the older save) hold `completed` 0, `winning_faction_id` NULL and `completion_time` NULL. The game never wrote 0 there, and
  there is no completed row without a time.

The unit is whole seconds and the zero is the start of the game's **play clock**, the seconds the game has been running (time the game is closed does not count). The same clock is
used by the term window, by `landsraad_task_reveal_state.timestamp` (2 to 311,489 in the same save), by `landsraad_house_rewards.last_updated` (4,285 to 313,689), by
`totem_fuel_update_times.last_fuel_update_farm_uptime` (127,179 to 318,079) and by `sandstorm_schedule.next_spawn_farmtime_seconds`.

The save keeps the clock's value at the time of the save in `farm_variables` (all three columns in microseconds):

    (universe_lastactive_timestamp - universe_time_timestamp - down_time_accumulation) / 1,000,000

| Save copy | farm_variables gives | Latest term window | Largest game-written time in that term | Totem fuel clock |
|---|---|---|---|---|
| A: the older character's save, term 10 | 318,092 | 10: 305,395 to 341,395 | 312,412 | 318,079 |
| B: an earlier copy of the same save | 246,873 | 8: 239,678 to 275,678 | 245,225 | 246,515 |
| C: another world | 104,478 | 4: 95,678 to 131,678 | (none yet) | 103,981 |
| D: a third world | 61,490 | 2: 31,563 to 67,563 | 61,042 | 61,490 |

In all 46 copies the value is inside the latest term's window and never below the largest game-written completion time of that term. So the value at the time of the save is a
moment the game could have written itself, and the play clock is the thing that moves on from this value when the save is loaded.

`landsraad_task_progress.timestamp` is a different matter: 739 of 774 rows of the older save (and all the rows of a save from a new character, written by the game only) hold Unix
time, 35 hold the play clock. The editor does not write that table. It is noted here only because the game itself does not use one clock for every Landsraad column.

The dedicated-server schema (`docs/evidence/postgres-dune-schema-columns.txt`) has `completion_time` as `timestamp with time zone`; the single-player file has an INTEGER and, as
above, the play clock in it.

## What the editor writes now

* **Complete, one house or several chosen:** `completion_time` = the play clock from `farm_variables`, held inside the term's window (the clock of a term the editor can reach is
  inside it; a term already over takes its end). A save that has the term but no `farm_variables` row gets the start of the term, which is inside the window. A save without the term
  gets NULL (a time from another clock is worse than none). The value is an integer, as the game's.
* **Reopen:** unchanged: `completed` 0, `winning_faction_id` NULL, `completion_time` NULL, which is what an open row holds.

## Rows the editor wrote before

Are they harmful? Nothing in the saves shows it, and nothing shows that it is safe:

* The editor's Unix times of terms 1 to 7 were in the save when the game went on through terms 8, 9 and 10, and the game never rewrote them. Nothing visible changed in those saves.
* The game's own code is not available here, so whether it ever compares `completion_time` with a term's window (to sort, to count "completed this term") is not known. A time
  that is a thousand times the end of the term is outside anything the game writes, so a reader that does compare it would be wrong, and the cost of correcting it is small.

So the correction is made as part of the actions that already exist, with no question: whenever **Complete**, **Complete chosen**, **Reopen** or **Reopen chosen** writes, the completed
houses of the same term whose `completion_time` is outside the term's window are given the same value a new completion gets. Only the term of the action is touched: the houses of
earlier terms are left as they are (they are not on the board and the editor does not show them). A time inside the window is the game's shape and is never changed; an open house is
never given a time. The count the page showed (`expected`) is not changed by it: a house that was only repaired is not counted as completed or reopened (the answer of the two
`-many` routes carries it separately as `repaired`).

## Check in the game

Complete one house on the current board, load the save, open the Landsraad board in the game: the house shows as completed for the faction and the board shows nothing different from
a house the game completed itself. If an earlier Complete had been used in the same term, its houses are on the same footing after that.

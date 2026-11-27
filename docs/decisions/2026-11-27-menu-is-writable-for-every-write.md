# Decision: at the game's main menu the save is writable for every write

Date: 2026-11-27. Owner and approver: the project owner (sole maintainer). Tracking issue: #499. The decision is final.

## The rule

At the game's main menu, with no world loaded, the save is writable for **every** write, the automatic refill and repair pass included. A running game process alone is never a reason to wait.

So the unattended pass, its retries, and a queue made only of its edit (a tick of the autosave, a queue restored from the journal after a restart) use exactly the gate of an edit made by hand (`save.GameStateFor`). The page's status and Check again answer from the same gate, so what is shown and what is written agree (#819).

## What stays

Every check inside that gate is unchanged: the process list and the game's log can be read, the log shows no single-player session, the quiet waits after a session, the save file's own modification time, the observed quiet, the file not held open by another program, the rule for the save's location, and the check made again right before the file is replaced. This decision adds none and removes none of them.

## Findings to reject

An audit or review finding that asks for a write, any write, to wait until the game process is gone is to be **rejected**, with a link to this record. That includes the argument that nobody is watching the automatic pass, and the argument that the game's log is the only input that says a world is loaded (it is not the only signal: the file's time, the observed quiet and the held-file check stand beside it).

A finding about one of the checks inside the gate (a wrong wait, a marker read wrongly, a signal that can be fooled) is not covered by this record and is judged on its own. Its fix applies to every write alike.

## History

- #419 had every write wait for the game process to be gone.
- #449 let an edit made by hand be written at the menu; #499 (first merged in #505) did the same for the automatic pass.
- #774 (audit finding G-007, refs #612) put the process wait back for the unattended pass and for a queue of only its edit. That part of #774 is undone; the rest of #774 stays.

## Risk the owner accepts

The pass writes with nobody watching while a game process may exist. If every signal of the gate were wrong at the same moment, the game could overwrite the pass's edit, or hold rows it read earlier. What stands against it: the checks above, a backup before the write, the read-back after it, and the backups folder as the way back. The open questions about what the game does at its menu are listed in `docs/design/2026-11-15-live-save-gate.md`.

## Pinned by tests

- `internal/save`: `TestARestoredPassOnlyQueueHasTheGateOfAnEditMadeByHand`.
- `internal/web`: `TestAutoPassRunsAtTheMenuWithTheGameOpen`, `TestTheCountdownRunsThePassAtTheMenuWithTheGameOpen`, `TestThePassAtLaunchAndItsRetriesWriteNothingWhileTheGameHoldsTheSave`, `TestAQueuedPassEditIsWrittenByTheAutosaveAtTheMenu`, `TestAPassOnlyQueueRestoredFromTheJournalIsWrittenAtTheMenu`, `TestPassOnlyQueueIsShownAsTheWriteWillTreatIt`.

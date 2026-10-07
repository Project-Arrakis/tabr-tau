# First run on Windows, 2026-10-07 (informal operator report)

**Status: informal.** This is not a pre-registered result from `docs/live-test-protocol.md`: there is no snapshot pair,
no redacted `snapdiff` output and no screenshot, so none of T1-T6 is recorded as PASS/FAIL by this note. It is kept so
the claim "the editor runs on a real Windows machine" has a dated source.

| Item | Value |
|---|---|
| Build | `main` at `01d77a3`, cross-compiled from Linux with `GOOS=windows CGO_ENABLED=0 go build -trimpath` (static exe, no installer) |
| SHA-256 of the exe tested | `c2d3b09511ef651a260f24ac3dc855306ae726860bba646f0fdfdf6f1587604c` |
| Run by | the operator, on their own Windows PC, against a game save |
| Game state | the operator was told to close the game and test on a copy of `game.db`; whether they did is not recorded |

## What was exercised (as reported by the operator)
- Started the exe and opened the UI in the browser.
- **Player tab:** "repair all equipment" and "add Solari". Both reported as working.
- Browsed every tab.

## Result
Reported as working. Whether the effects were then confirmed inside the game is not recorded.
Missing features the operator noticed were the known, expected gaps (see issues #11, #17, #22 and the plan's S2/S3).

## What this does and does not show
- It shows the Windows build starts, serves the UI and performs two write paths (`RepairGear`, `AddSolari`) on a real save.
- It does **not** show that the game accepts the written save, that Windows-specific code paths behave (running-game
  detection via `tasklist`, rename retry with OneDrive/antivirus, auto-discovery under `%LOCALAPPDATA%`), or anything about
  item give (T1/T1b), JSONB edits (T2), vitals (T3), cascading deletes (T4), file authority (T5) or id allocation (T6).

## Next
A recorded run should follow the protocol: snapshot before, make the edit through the editor, load the game, snapshot after,
store the redacted diff and a screenshot under `docs/evidence/`.

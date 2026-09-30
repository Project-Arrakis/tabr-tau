# Live test protocol v2 (pre-registered)

Revised 2026-09-30 after the Layer 1 audit (F-13). v1 had a single noise run, an unfair comparison, a
subjective pass/fail and no tests for XP, hydration, foreign keys or id allocation.

Purpose: settle the open questions (`docs/design/...architecture...` section 7) by evidence, before any
feature that depends on them is built. Decisions below are fixed **before** running, so results cannot be
argued after the fact.

## Prerequisites (check once)

- Windows PC with the game. PowerShell 5.1+ (`$PSVersionTable.PSVersion`).
- If scripts are blocked: `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass` in that window.
- Analysis machine: Python >= 3.11 (`python --version`) and SQLite >= 3.45 (`python -c "import sqlite3;print(sqlite3.sqlite_version)"`).
- Steam Cloud: note whether it is on for this game (Steam > game > Properties > General). If on, note that
  Steam may restore a synced `game.db` at launch; this is one of the questions under test.
- `tools\snapshot.ps1` and `tools\snapdiff.py` from this repo.

## Rules

1. **Game fully closed for every snapshot** (quit to desktop, wait until the process is gone).
2. **One action per snapshot.** Snapshot labels are never reused (the script refuses).
3. **Snapshots contain your IDs.** They default to a local folder (`%USERPROFILE%\tabr-tau-snapshots`),
   `Game.ini` is not copied. Do not commit, post or share them. Never put raw snapshots in git (including `meta`):
   git history is permanent. Transfer for analysis with `tools\send-snapshot.ps1` (below). Commit only the
   redacted `tools/snapsummary.py` output. Delete snapshots on both machines after the experiment.
4. **Do not edit files by hand.** Only tabr-tau edits, and only when a step says so.
5. Each step: take the snapshot, then write one line in `docs/evidence/RESULTS.md` (label, what you did,
   screenshot file name, any error). Screenshots must not show account names or IDs.
6. If anything crashes or a step fails: stop, snapshot as `X-<what-happened>`, do not continue.
7. **Consume only complete snapshots:** a snapshot folder is valid only if `manifest.json` exists (written
   last). Re-hash any transferred file against the manifest before use.

## Transfer (token-free) and comparing copies

```
# on the PC, after `snapshot.ps1 -Label A0-baseline`
.\tools\send-snapshot.ps1 -SnapshotDir "$env:USERPROFILE\tabr-tau-snapshots\A0-baseline" `
    -Target <user>@<host>:/<dir> [-IdentityFile <path-to-private-key>]
```
It refuses a snapshot without `manifest.json`, zips it, hashes it, `scp`s it, has the host hash it again, and
fails on any mismatch. Key authentication only (no password prompts). Use an **unprivileged** account on the
receiving host, not root. Requires the Windows "OpenSSH Client" optional feature (built in on Windows 10/11).

Compare copies without moving IDs: on the analysis host run
`python tools/snapsummary.py <snapshot> --label <label> --out docs/evidence/summaries/<label>.json`.
The summary is redacted (file hashes, table row counts, patch set, item-template tally, XP/hydration/health)
and refuses to emit if it finds a 15+ digit number. Summaries can be committed and diffed across copies and over time.

Optional (WSL users only; not needed for ~2 MB snapshots): `rsync -a --checksum --partial --ignore-existing -e ssh
/mnt/c/Users/<you>/tabr-tau-snapshots/<label>/ <user>@<host>:/<dir>/<label>/` then re-run with `-n` added;
empty output means the copies are identical. `--ignore-existing` keeps a delivered snapshot immutable.

## Phase A: baseline and noise (measures what changes with no intended action)

| Label | Do |
|---|---|
| `A0-baseline` | Quit the game. Snapshot. |
| `A1-noise-load` | Launch, load the character, quit at once. Snapshot. |
| `A2-noise-load` | Repeat A1 exactly. Snapshot. |
| `A3-noise-walk` | Launch, load, walk around the same spot for 2 minutes, quit. Snapshot. |

Noise set N = every table/column/JSON path that differs in `A0->A1`, `A1->A2` or `A1->A3` (use `--noise`
presets and refine). Anything in N is ignored in later diffs unless its change is larger than the largest
change seen across A1/A2/A3. Also record, for every launch: `game.db` hash and mtime before/after, and the
hash of each `autosave\*.bak`. **Determination Q5:** which file changes on quit (`game.db`, an autosave slot,
or both), and which the next launch appears to use (answered by C2).

## Phase B: in-game reference for a literjon (the game's own rows)

| Label | Do |
|---|---|
| `B1-ingame-literjon` | Launch. Obtain or craft an **empty literjon in game**. Screenshot inventory. Quit. Snapshot. |
| `B2-ingame-filled` | Launch. Fill it at a water source. Screenshot the shown amount. Quit. Snapshot. |
| `B3-ingame-drunk` | Launch. Note hydration bar. Drink from it once. Screenshot hydration and literjon. Quit. Snapshot. |

Outputs: the exact `template_id` (from `B0/A -> B1` diff on `items`), the row shape of a game-written
literjon, **where fill level lives** (Q3), and **where a drink goes** (`actors.gas_attributes...CurrentHydration`
and the item).

## Phase C: tabr-tau writes a literjon (compare against the game's own row)

Start from the state after B3.

| Label | Do |
|---|---|
| `C1-tabr-give` | With the game closed, use tabr-tau **Give item** with the `template_id` found in B1, quantity 1. Press Save to game. Snapshot. |
| `C2-game-loads` | Launch, load. Screenshot inventory. Quit. Snapshot. |

## Phase D: other write paths

| Label | Do | Answers |
|---|---|---|
| `D1-tabr-xp` | tabr-tau: raise `UnspentSkillPoints` by 1 via the component editor (or a documented manual `jsonb_set` on a copy, for the first run). Snapshot. | T2 |
| `D2-game-after-xp` | Launch, open the skill screen, screenshot points, quit. Snapshot. | T2 |
| `D3-tabr-hydration` | tabr-tau (or manual): set `CurrentHydration` to a visibly different value. Snapshot. | T3 |
| `D4-game-after-hydration` | Launch, screenshot the hydration bar, quit. Snapshot. | T3 |
| `D5-ingame-delete-container` | In game, put items inside a container item (bag/literjon with contents if applicable) and drop/destroy the container. Quit. Snapshot. | T4: does the game remove the child inventory and items itself? |
| `D6-ids` | After C2: launch, pick up or craft two items, quit. Snapshot. | T6: does the game reuse or collide with ids tabr-tau allocated? |

## Decision table (fixed in advance)

| ID | Question | PASS if | FAIL if | If FAIL |
|---|---|---|---|---|
| T1 | Does a tabr-tau-written literjon survive a game load? | In C2, the item is visible in game **and** the row still exists in the snapshot with the same `template_id`; and `C1->C2` shows no game-side *repair* of it beyond noise set N | item missing from the game or the row deleted, or a crash log newer than the snapshot appears in `Saved\Crashes` | Restore `B3`, record the diff, treat inserts as unsupported until the difference is understood |
| T1b | Is our row faithful? | `B1` (game row) and `C1` (our row) have the same set of columns and the same JSON key set in `stats`; differences only in ids, timestamps and values | any structural difference (extra/missing column or JSON key) | fix `giveInTx` to match the game row, retest |
| T2 | Is a JSONB `UnspentSkillPoints` edit accepted? | D2 shows the new value in game and the JSON path still exists afterwards | value reverted, shown wrong, or load error | XP/skill editing stays disabled |
| T3 | Is a hydration edit accepted? | D4 bar reflects the value | reverted or error | vitals editing stays disabled |
| T4 | Does the game cascade deletes (FK behavior)? | After D5, the child `inventories` row (and its items) for the destroyed container is gone **and** `foreign_key_check` is clean | orphans remain | keep `foreign_keys=0` + explicit child deletes; if children were removed, adopt `foreign_keys=ON` |
| T5 | Which file is authoritative? | Q5 answered from hashes (recorded, not inferred) | inconsistent between runs | record and document both cases |
| T6 | Id allocation safe? | D6 shows the game's new item ids all `> ` tabr-tau's id and no duplicate `items.id`, and `items_id_sequencer.next_id > max(items.id)` | duplicate id, or game overwrote our row | change allocation (use a higher reserved block) |

A result is "recorded" only when a snapshot pair, the `snapdiff` output (redacted) and one screenshot
are stored in `docs/evidence/` (redacted) with a PASS/FAIL line. "Kept but changed by the game" is a
distinct outcome: record the diff and mark **PARTIAL**.

## Analysis commands

```
python tools/snapdiff.py <before> <after> --noise --max-rows 40         # redacted by default
python tools/snapdiff.py A1-noise-load A2-noise-load --noise            # noise floor
```

Do not use `--no-redact` for anything that leaves your machine.

## Already-known evidence (no live test needed)

`autosave/2.bak -> 9.bak` in the sample shows: hydration `43.64 -> 26.40`, `HeatExhaustion 0 -> 20.42`,
`TechKnowledgePoints 19 -> 0`, 14 new actors, 141 new `building_instances`. Autosave slots are usable as free
time-series data and as fixtures for `snapdiff` tests.

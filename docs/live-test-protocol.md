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
- If several Steam accounts have saves on the PC, `snapshot.ps1` lists them (with `game.db` modified times) and stops; re-run with `-SteamId <id>` for the account you play (usually the most recently modified).

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
| `C1-tabr-give` | With the game closed, use tabr-tau **Give item** with the `template_id` found in B1, quantity 1. Press Review & save, then Save to game. Snapshot. |
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

## Phase E: a worn armor set (reference gear taken from a multiplayer character)

Purpose: find out whether tabr-tau can place worn equipment, and whether item augments survive, before S3 builds
a feature on either. The reference set comes from a read-only query of a real multiplayer character on the
operator's own server (2026-10-07); only template ids and stat key names are recorded here, no character name,
account id or other identifier. Worn gear lives in the `inventories` row with `inventory_type = 1` (backpack is 0,
held weapons and tools are 15). Run only on a copy of the save, game closed, with a snapshot taken first.

Reference set (template ids, worn container, positions 0-4 armor, 6-9 utility):
`Combat_Heavy_Unique_Reinforced_Helmet_06`, `Combat_Heavy_Unique_PowerIncrease_Top_06`,
`Combat_Heavy_Unique_Reinforced_Bottom_06`, `Combat_Heavy_Unique_PowerEfficient_Gloves_06`,
`Combat_Heavy_Unique_Reinforced_Boots_06` (all `quality_level` 5, each with two `T6_Augment_Armor*` entries under
`stats.FAugmentedItemStats`), then `PortableLight`, `FullSuspensorBelt_Unique_Durability`, `PowerPack_Unique_Regen_06`,
`HoltzmanShieldActiveDrain_Unique_01` (no augments, `stats.FItemStackAndDurabilityStats` only).

| Label | Do | Answers |
|---|---|---|
| `E0-ingame-worn` | In game, wear any one armor piece the character can obtain. Quit. Snapshot. Record the `inventory_type`, `position_index`, `quality_level` and `stats` key set of its row. | the game's own row for worn gear (reference for T7/T8) |
| `E1-tabr-give-backpack` | Game closed. tabr-tau **Give item** `Combat_Heavy_Unique_Reinforced_Helmet_06` x1 to the backpack. Save to game. Snapshot. | T7 |
| `E2-game-loads-backpack` | Launch, load. Screenshot inventory, equip the helmet in game, quit. Snapshot. | T7 |
| `E3-tabr-give-worn` | Game closed. Insert the same item directly into the worn container (type 1) at a free armor position (manual `insert` on a copy for the first run). Snapshot. | T8 |
| `E4-game-loads-worn` | Launch, load. Screenshot the equipment screen, quit. Snapshot. | T8 |
| `E5-augment-stats` | Game closed. Copy the `stats.FAugmentedItemStats` entry from the `E0` row (or from a reference row obtained in game) onto the E1 item. Snapshot. | T9 |
| `E6-game-augment` | Launch, inspect the item, screenshot augments and stats, quit. Snapshot. | T9 |

### Phase E results (2026-10-07, one run, operator's own save)

Done by direct SQL on a decoded copy, using the multiplayer row layout (full `stats` JSON, `quality_level`, ids from
`items_id_sequencer`), not by tabr-tau's Give item. Existing worn and loadout items were moved to free backpack
slots, not deleted. The edited file was written back with the game closed, after a backup and a stability check.

| ID | Result | Evidence |
|---|---|---|
| T7 | PASS (direct SQL route) | Items present and usable after a game load. |
| T8 | PASS | Nine rows inserted into the worn container (`inventory_type = 1`) at positions 0-4 and 6-9. After the game loaded and re-saved the file, all nine rows were still there with the same ids, `template_id`, `quality_level` and augment data; the operator confirmed the set shows as worn. |
| T9 | PASS | `FAugmentedItemStats` (two `T6_Augment_Armor*` entries per armor piece) survived the game's own load and save, and the operator confirmed the gear. |
| Loadout | PASS (not a pre-registered test) | Eight rows inserted into the loadout container (`inventory_type = 15`), including two augmented weapons; the operator confirmed the loadout worked in game. |

Not shown by this run: that tabr-tau's own Give item produces the same rows (T1/T1b still open), a crash-log check,
and any template whose name differs between the multiplayer and single-player builds. Foreign-key checks were clean
before writing; the game's own re-save was not diffed beyond the worn container.

## Decision table (fixed in advance)

| ID | Question | PASS if | FAIL if | If FAIL |
|---|---|---|---|---|
| T1 | Does a tabr-tau-written literjon survive a game load? | In C2, the item is visible in game **and** the row still exists in the snapshot with the same `template_id`; and `C1->C2` shows no game-side *repair* of it beyond noise set N | item missing from the game or the row deleted, or a crash log newer than the snapshot appears in `Saved\Crashes` | Restore `B3`, record the diff, treat inserts as unsupported until the difference is understood |
| T1b | Is our row faithful? | `B1` (game row) and `C1` (our row) have the same set of columns and the same JSON key set in `stats`; differences only in ids, timestamps and values | any structural difference (extra/missing column or JSON key) | fix `giveInTx` to match the game row, retest |
| T2 | Is a JSONB `UnspentSkillPoints` edit accepted? | D2 shows the new value in game and the JSON path still exists afterwards | value reverted, shown wrong, or load error | XP/skill editing stays disabled |
| T3 | Is a hydration edit accepted? | D4 bar reflects the value | reverted or error | vitals editing stays disabled |
| T4 | Does the game cascade deletes (FK behavior)? | After D5, the child `inventories` row (and its items) for the destroyed container is gone **and** `foreign_key_check` is clean | orphans remain | the editor still cascades deletes itself (`MutateCascade`), so no change is needed for safety; keep `foreign_keys=0` as the default connection setting; if the game removed the children, adopt `foreign_keys=ON` globally |
| T5 | Which file is authoritative? | Q5 answered from hashes (recorded, not inferred) | inconsistent between runs | record and document both cases |
| T6 | Id allocation safe? | D6 shows the game's new item ids all `> ` tabr-tau's id and no duplicate `items.id`, and `items_id_sequencer.next_id > max(items.id)` | duplicate id, or game overwrote our row | change allocation (use a higher reserved block) |
| T7 | Is a tabr-tau-given armor piece usable? | In E2 the item is visible in the backpack, can be equipped in game, and the row survives with the same `template_id` | item missing, cannot be equipped, row deleted, or a newer crash log | armor give stays plain-item only; record the diff against the E0 row |
| T8 | Can worn gear be written directly into the worn container? | In E4 the item shows as equipped and the row remains in `inventory_type = 1` | item moved, dropped, or crash | worn-set give is unsupported; give to the backpack only |
| T9 | Do copied augments survive and apply? | In E6 the augments are listed and the stat bonuses show, and the `stats` JSON is unchanged after load | augments stripped, item reset to plain, or load error | S3 gives plain items only; augment copying stays disabled |

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

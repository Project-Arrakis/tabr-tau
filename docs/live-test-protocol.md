# Live test protocol: literjon give / fill / drink

Purpose: learn, from evidence, (1) what the game writes for a simple item lifecycle, (2) whether a
row written by tabr-tau is accepted and normalized by the game, and (3) which columns/JSON paths change
when a literjon is filled and drunk. Everything is observed by diffing snapshots; nothing is assumed.

Tools: `tools/snapshot.ps1` (Windows, run on the desktop) and `tools/snapdiff.py` (any host with
Python 3.8+ and SQLite >= 3.45).

## Rules

- **Game closed for every snapshot.** Quit to desktop first. A running game can hold unsaved state and
  `game.db` may be mid-write. The manifest records if the game was running.
- **One action per snapshot.** Never combine "give" and "fill" between two snapshots.
- **Never edit the live folder while the game runs.** tabr-tau already refuses to save then.
- **Keep the originals.** `snapshot.ps1` only copies. The first snapshot is your rollback.
- **Do not commit snapshots.** They contain your platform ID and account details (`Game.ini`,
  `game.db` accounts table). They live in OneDrive under `tabr-tau-samples\snapshots`.

## Steps

| # | Do | Snapshot label |
|---|---|---|
| 0 | Quit the game. | `00-baseline` |
| 1 | **Noise control:** launch, load the character, stand still 1 minute, quit. | `01-noise` |
| 2 | With the game closed, use tabr-tau **Give item** to add one **Literjon** (empty) to your inventory. Press Save to game. | `02-after-tabr-give` |
| 3 | Launch, load. **Screenshot the inventory.** Confirm the literjon exists and is empty. Quit. | `03-game-normalized` |
| 4 | Launch, fill the literjon at a water source, note the displayed amount (screenshot), quit. | `04-filled` |
| 5 | Launch, drink from it once, note hydration before/after (screenshot), quit. | `05-drunk` |
| 6 | (optional) Get a second literjon **in-game** by crafting or looting, not tabr-tau, and quit. | `06-ingame-literjon` |

Run each snapshot as: `.\tools\snapshot.ps1 -Label 00-baseline`. Tell me when each has synced to
OneDrive (`tabr-tau-samples\snapshots\<label>`); I pull and diff.

## Diffs and what each answers

| Diff | Answers |
|---|---|
| `00 -> 01` | Noise floor: what changes with no player action (`actors.serial`, positions, hydration drift, timestamps). Everything here is subtracted from later diffs. |
| `01 -> 02` | Exactly what tabr-tau's give writes: `items`, `inventories`, `items_id_sequencer`. Compare with the game's own rows. |
| `02 -> 03` | What the game **rewrites** on load/quit. Anything it changes on our row shows what our insert was missing or got wrong. If the item disappears, our insert was rejected. |
| `03 -> 04` | Where the fill level lives (expected `items.stats` JSON or `volume_override`; not assumed). |
| `04 -> 05` | Where a drink goes: `actors.gas_attributes.DuneHydrationAttributeSet.CurrentHydration` (seen changing in autosave diffs) and the literjon's fill level. |
| `05 -> 06` | Difference between an in-game-created item row and a tabr-tau-created one. |

Command: `python3 tools/snapdiff.py <before> <after> --max-rows 40`.

## Pass / fail

- **Give works** if step 3 shows the literjon in the inventory with no save-load error, and `02 -> 03`
  shows the game kept the row.
- **Give is faithful** if `05 -> 06` shows no structural difference (columns/JSON keys) between the two
  rows other than IDs and timestamps.
- Any load error, missing item, or a crash log under `Saved\Crashes` newer than the snapshot is a
  **fail**: restore from `00-baseline` (copy `game.db` back with the game closed) and record the crash.

## Already-known evidence (from existing autosave slots, no live test)

`autosave/2.bak -> 9.bak` in the sample shows: hydration `43.64 -> 26.40`, `HeatExhaustion 0 -> 20.42`,
`TechKnowledgePoints 19 -> 0`, 14 new actors, 141 new `building_instances`. Autosave slots are usable
as free time-series data.

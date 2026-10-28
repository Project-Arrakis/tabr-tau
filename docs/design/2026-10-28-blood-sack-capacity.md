# How much each blood sack holds (#472)

Status: built. The table is `bloodCapacity` in `internal/ops/fillables.go`; the tests are in `internal/ops/bloodsack_capacity_test.go`. The fill has not been checked in the game for the new templates.

## Problem

Fill blood sacks knew two capacities, `Bloodsack_01` (3000) and `Bloodsack_Unique_Durable_06` (30000), both "the largest amount seen in a save". Every other blood sack was left alone and reported
("4 blood sacks of an unknown kind will be left alone"), so a character carrying four `Bloodsack_02` could not fill them. The wiki lists the capacities by name only (no internal ids), so the
mapping from a template to a capacity was a guess from the order of the names until the game's own data gave it.

## Source

The game's item data: the table of fillable items, `DT_ItemTableFillables` (row structure `FillableItemTableRow`), one row per item template, row name = the template id, field
`FillableStats.MaxAmount`, with `FillableTypeRestriction` = `Blood` for a blood sack. Read from the game's data files (game build 1.5.15.x) on 2026-10-28. The unit is the one the save uses
for `FFillableItemStats.CurrentAmount`. The table has nine `Bloodsack_*` rows and no other row whose name begins with `Bloodsack`; each has `bIsContainer` true and a starting `CurrentAmount` of 0.

| Template | MaxAmount in the item data | Wiki name and capacity | Used by the editor |
|---|---|---|---|
| `Bloodsack_01` | 3000 | Small, 3000 | yes (unchanged) |
| `Bloodsack_02` | 9000 | Medium, 9000 | yes (new) |
| `Bloodsack_03` | 15000 | Large, 15000 | yes (new) |
| `Bloodsack_T6` | 24000 | Massive, 24000 | yes (new) |
| `Bloodsack_Unique_Durable_02` | 13000 | Scipio's, 13000 | yes (new) |
| `Bloodsack_Unique_Durable_03` | 16000 | Glutton's, 16000 | yes (new) |
| `Bloodsack_Unique_Durable_04` | 19000 | Sinner's, 19000 | yes (new) |
| `Bloodsack_Unique_Durable_05` | 22000 | Maas Kharet's, 22000 | yes (new) |
| `Bloodsack_Unique_Durable_06` | 30000 | The Baron's, 30000 | yes (unchanged) |

The wiki column is only a cross-check: the item data does not carry the wiki's names, so the pairing of a template with a name comes from the equal capacity, and the capacities of all nine
rows equal the nine on the wiki. The ordering of the ids is the ordering of the names. Nothing was taken from the wiki into the code; the numbers in the code are the item data's.

## Cross-check against saves

* The two capacities already in use (`Bloodsack_01` 3000, `Bloodsack_Unique_Durable_06` 30000), which were entered from the largest amount seen in a save, are the same in the item data.
* A copy of a played save holds four `Bloodsack_02`; the largest amount in them is 1466, which is below 9000 and so does not contradict it. A save shows only what a sack happened to hold, so it
  cannot confirm a capacity above it; the item data does.
* There is no template in the data for which the editor has a capacity and the data does not, and none the other way round.

## What stays unknown

A `Bloodsack_*` template that is not one of the nine rows (for example one added by a later game build) is not in `bloodCapacity`: Fill blood sacks leaves it alone and reports it, as before, and the
count beside the button shows it as "of an unknown kind". There is no `Bloodsack_Unique_Durable_01` row in the data; if a save holds one it is treated as unknown. When a game update adds a row, add
it here only after reading it from the item data.

## What did not change

The button, the count and the result sentence say what they said. A sack holding anything other than blood is left alone, a full sack is not rewritten, an amount above the capacity is never
lowered, and only the character's own inventories are looked at. The tooltip on Fill blood sacks still describes the earlier two capacities as examples; its wording is left for the text sweep (#426).

## Tests

* the table is exactly the nine rows above (the numbers are written a second time in the test, so a change to the table must change a test);
* for each of the nine templates: an empty record, no record at all and a partly full sack are filled to the capacity with every other key of the stats untouched; a full sack and a sack holding water
  are not rewritten; a second press finds nothing to do and is not an edit;
* the case of the issue: four `Bloodsack_02` (one holding 1466, two with durability only, one with an empty record) are counted as not full and filled to 9000;
* templates the data does not list (`Bloodsack_04`, `Bloodsack_Unique_Durable_01`, `Bloodsack_Unique_Durable_07`, `Bloodsack_99`) are left alone and reported; the earlier tests that used
  `Bloodsack_02` and `Bloodsack_03` as "unknown" use `Bloodsack_99`;
* the inventory view (`fillOf`) shows a partly full, an empty and a full sack of the new templates with their capacity and percent.

## To check in the game

Fill blood sacks on a character that carries a `Bloodsack_02`; load the save; the gauge reads full (9000). The same for one of the other new templates when one is at hand.

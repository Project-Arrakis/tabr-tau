# How much every fillable item holds, and what Fill does with a stillsuit (#482)

Status: built. The tables are `waterCapacity` and `stillsuitCapacity` in `internal/ops/fillables.go` (the blood sacks' `bloodCapacity` is from #472); the tests are in `internal/ops/fillable_capacity_test.go` and `web-tests/fillview.test.mjs`. Nothing here has been checked in the game yet.

## Problem

The Inventory table showed "Menol's Stillsuit Garment: Liquid 500 (capacity unknown)". `waterCapacity` held nine Literjon-family entries entered one at a time (read in game, or the largest amount seen in a save), so every other fillable item was "of unknown capacity": Fill water containers left it alone and reported it. After #472 read the blood sacks from the game's item data, the rest of the same table had still not been read.

## Source

The game's item data: the table of fillable items, `DT_ItemTableFillables` (row structure `FillableItemTableRow`), one row per item template, row name = the template id, with `FillableStats` = `MaxAmount`, `CurrentAmount` (the starting amount), `FillableType` (always `None`: nothing held yet), `FillableTypeRestriction` (the liquid the item takes) and `bIsContainer`. A row has no other field: no minimum, no "no player fill" flag. Read from the game's data files (game build 1.5.15.x) on 2026-10-28, with the game closed; nothing was written to the game. The related table `DT_FillableTypes` names exactly three liquids: `Water`, `Blood` and `Fuel`; no row of the fillable table restricts to `Fuel` (vehicle fuel is a different mechanism). The unit is the one the save uses for `FFillableItemStats.CurrentAmount` (a Literjon Mk6 row says 20000 and a save holds 17910 of it).

58 rows: 9 take Blood, 49 take Water, none take anything else.

## The whole table

### Blood, containers (`bIsContainer` true): 9 rows, in `bloodCapacity` since #472

| Template | MaxAmount |
|---|---|
| `Bloodsack_01` | 3000 |
| `Bloodsack_02` | 9000 |
| `Bloodsack_03` | 15000 |
| `Bloodsack_T6` | 24000 |
| `Bloodsack_Unique_Durable_02` | 13000 |
| `Bloodsack_Unique_Durable_03` | 16000 |
| `Bloodsack_Unique_Durable_04` | 19000 |
| `Bloodsack_Unique_Durable_05` | 22000 |
| `Bloodsack_Unique_Durable_06` | 30000 |

### Water, containers (`bIsContainer` true): 17 rows, in `waterCapacity`

| Template | MaxAmount | Before this change |
|---|---|---|
| `Literjon` | 1000 | listed, same |
| `Literjon_03` | 1100 | new |
| `Literjon_04` | 1200 | new |
| `Literjon_05` | 1300 | new |
| `Literjon_06` | 1400 | new |
| `Literjon_07` | 1500 | new |
| `Literjon_08` | 1600 | new |
| `Literjon_09` | 1700 | new |
| `Literjon_T6` | 20000 | listed, same |
| `Decajon` | 10000 | listed, same |
| `Dewpack` | 250 | new |
| `HighCapacityLiterjon` | 1500 | listed, same |
| `HighCapacityLiterjon_02` | 1750 | listed, same |
| `HighCapacityLiterjon_03` | 2000 | listed, same |
| `HighCapacityLiterjon_04` | 2250 | listed, same |
| `HighCapacityLiterjon_05` | 2500 | listed, same |
| `HighCapacityLiterjon_06` | 3000 | listed, same |

### Water, not containers (`bIsContainer` false): 32 rows

The 31 worn stillsuits and their kin are in `stillsuitCapacity` (shown, never written):

| Template | MaxAmount |
|---|---|
| `AdvancedStillsuit`, `SimpleStillsuit` | 500 |
| `Combat_Nati_FremenExile04_Top` | 500 |
| `Stillsuit_Choam_01_Top`, `_02_Top`, `_04_Top`, `_05_Top`, `_06_Top` | 500 |
| `Stillsuit_Choam_Unique_Dashed02_Top` to `Dashed06_Top` (5) | 500 |
| `Stillsuit_Nati_05_Body`, `_06_Body`, `_07_Body`, `_08_Body`, `Stillsuit_Nati_Arrakeen05_Body` | 500 |
| `Stillsuit_Neut_Leaking01_Top`, `Stillsuit_Neut_Patchy02_Top` | 500 |
| `Stillsuit_Unique_Armored_01_Top` to `_06_Top` (6; `_02` is Menol's Stillsuit Garment) | 500 |
| `Stillsuit_Unique_Efficient_04_Top`, `_05_Top`, `_06_Top` | 500 |
| `Stillsuit_Unique_ThermalSuit_06_Top` | 500 |
| `Stillsuit_Unique_HighCapacity_06_Top` | 1500 |

And one that is not a suit:

| Template | MaxAmount | CurrentAmount | Decision |
|---|---|---|---|
| `WaterPack_Consumable` ("Cup of Water") | 100 | 100 | left out: a consumable that starts full, `bIsContainer` false, and no save copy gives it a fill record (it is a drink, not something to fill) |

## Check of the existing numbers

The nine `waterCapacity` entries that existed (`Literjon` 1000, `Literjon_T6` 20000, `Decajon` 10000, `HighCapacityLiterjon` 1500, `_02` 1750, `_03` 2000, `_04` 2250, `_05` 2500, `_06` 3000) are all in the table with the same MaxAmount. **No disagreement.** The two blood sack numbers entered before #472 were already confirmed there. Nothing in the editor's tables is missing from the game's, and every Water row that is a container is now in the editor's.

## Cross-check against saves

Played-save copies (read-only, 22 files) hold these templates with a fill record. The largest amount seen never exceeds the table: `HighCapacityLiterjon_02` 1750 (full), `HighCapacityLiterjon_03` 1750 of 2000, `HighCapacityLiterjon_06` 3000 (full), `Literjon_T6` 17910 of 20000, `Bloodsack_01` 3000, `Bloodsack_02` 9000, `Bloodsack_Unique_Durable_06` 30000. A suit's record is `{"CurrentAmount": 140.765075}`: Menol's Stillsuit Garment (`Stillsuit_Unique_Armored_02_Top`, 8 copies) peaks at 500 and the Efficient suit (`Stillsuit_Unique_Efficient_04_Top`, 7 copies) at 500, the table's number. No save holds `Literjon_03` to `_09` or the `Dewpack`, so those capacities rest on the game's data alone.

## The stillsuit decision: shown with its capacity, never filled by the editor

What the data says about a stillsuit's "Liquid":

* It is water (`FillableTypeRestriction` Water) in the same unit as a Literjon, 500 for almost every suit and 1500 for the high capacity one.
* It is **not a container**: all 31 suit rows have `bIsContainer` false, every Literjon, Decajon, Dewpack and blood sack row has it true.
* The save keeps **no liquid type** for it: the record is `{"CurrentAmount": 140.765075}`, with no `FillableType` (every container that holds something writes one), and the amount is a fraction. It is not a number a person sets; it is the water the suit has recovered, and the game changes it all the time while the suit is worn.

So it is not what Fill is for: Fill water containers and Fill all liquids fill things the person carries to hold water. A suit's reservoir is filled by the game, from the wearer; writing 500 into the save would be undone by the game's next change, and there is no evidence that the game accepts a value it did not produce. The editor therefore leaves a suit alone in every fill button.

What changed so that this is said in plain words:

* The Inventory and the base container tables read the suit's amount against the game's capacity: "Water 141 / 500 (28%), the suit fills itself" (was "Water 140.765075 (capacity unknown)"). The amount is shown rounded.
* The card header ("N liquid containers, M not full") does not count a suit: Fill could do nothing about it.
* Fill water containers and Fill all liquids no longer list a suit among "Left alone, capacity unknown"; the answer carries the number of suits seen (`stillsuits`) and the count beside Fill all liquids does not include them.

A suit template that is not one of the 31 rows is treated as before: its amount is shown and its capacity is unknown.

## Base water devices (`waterDeviceCapacity`, item 3 of the issue)

The capacity of a base water device is in the game's placeable data, `CDT_PlaceableData_Functional`, field `m_WaterCapacity` of the row named by the building type (for the devices that store water, `m_bCanStoreWater` true). The editor's numbers (first taken from the Dune Docker console's table) were compared:

| Building type | `m_WaterCapacity` in the game's data | `waterDeviceCapacity` | Largest stored amount in the saves |
|---|---|---|---|
| `WaterCistern_Placeable` | 5000 | 5000 | 5000 |
| `MediumWaterCistern_Placeable` | 25000 | 25000 | 25000 |
| `LargeWaterCistern_Placeable` | 100000 | 100000 | 100000 |
| `Windtrap_Placeable` | 500 | 500 | 500 |
| `LargeWindtrap_Placeable` | 500 | 500 | 500 |
| `BloodWaterExtractor_Placeable` | 1000 | 1000 | 1000 |
| `BloodWaterExtractionAdvanced_Placeable` | 1000 | 1000 | 1000 |

**No disagreement.** The data also lists water stores the editor does not know and that no save copy holds with a water component: `SmallInsulatedCistern_Placeable` 2500, `MediumInsulatedCistern_Placeable` 7500 and the four Stilltent kinds (`Stilltent_Placeable`, `CamouflagedStilltent_Placeable`, `ProtectiveStilltent_Placeable`, `Stilltent_Placeable_Unique_01`) 200. They are left out of `waterDeviceCapacity` for now: a base water fill reports a device it does not know ("Left alone, capacity unknown") and nothing in the saves shows how the game stores their water; they are a candidate for a follow-up once a save holds one. (The blood extractors' own blood tank, `m_Capacity` 6000 and 24000 in their blueprints, is a different store and not part of this fill.)

## What did not change

The Refill Literjons button, Fill water containers, Fill blood sacks and Fill all liquids say what they said. A container holding anything other than water is left alone, a full one is not rewritten, an amount above the capacity is never lowered, only the character's own inventories are looked at, and a container whose template is not in the data stays "capacity unknown", left alone and reported. When a game update adds a row, add it here only after reading it from the item data.

## Tests

* `waterCapacity` is exactly the 17 rows above and `stillsuitCapacity` exactly the 31 (the numbers are written a second time in the test); the earlier nine are unchanged; no template is in two tables;
* for each of the 17 water containers: an empty record, no record at all and a partly full container are filled to the capacity with every other key untouched; a full container, one holding blood and one above its capacity are not rewritten; a second press finds nothing to do and is not an edit;
* templates the data does not list (`Literjon_02`, `Literjon_10`, `Literjon_11`, `Dewpack_02`) are left alone and reported (the ones with a record), and `Literjon_03` beside them is filled to 1100;
* for each of the 31 suits (a record as the game saves it, a full one, an empty record, no record): Fill water containers and Fill all liquids write nothing, report no unknown container, and the count beside Fill all liquids does not include them;
* the inventory view (`fillOf`) shows each suit as Water against its capacity, marked as filled by the suit, with its percent; a suit holding blood or one the data does not list is not marked or sized; the inventory read of the case in the issue (Menol's Stillsuit Garment holding 140.765075 gives 28% of 500);
* the page (`web-tests/fillview.test.mjs`): the suit's line reads "Water 141 / 500 (28%), the suit fills itself", never "capacity unknown", and the header counts only the real containers.

## To check in the game

Open Player > Character > Inventory on a character wearing Menol's Stillsuit Garment: the Liquid column should read the suit's water out of 500 and say the suit fills itself (the number should match the suit's bar in the game, within what moves in between). Then carry a `Literjon_03` or higher grade that is not full (or any other grade from the table above) and press Fill water containers on a copy of the save with the game closed; load the save: the container reads full in the game. If the game shows a suit as something else than water out of 500, say so on #482.

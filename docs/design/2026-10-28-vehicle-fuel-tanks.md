# How much fuel each vehicle holds (#469)

Status: built. The rule is `tankOf` in `internal/ops/vehicletank.go`; Refuel all my vehicles (`internal/ops/refuel.go`) and the Fuel column of Player > Vehicles (`internal/ops/vehicledetails.go`) use it; the tests are in
`internal/ops/vehicletank_test.go`. The fill has not been checked in the game for the grades that were not measured before (below).

## Problem

Refuel filled a vehicle to one tank per vehicle class (`vehicleTank`: Sandbike 100, Buggy 240, Light Ornithopter 300, Medium Ornithopter 750, Sand Crawler 750, Transport Ornithopter 2500), each measured once on one vehicle.
But a tank follows the **grade of the vehicle's chassis**, not its class. A Mk6 buggy holds 300, so Refuel read it as full at 240 and never topped it up (the Vehicles tab said 240 and capped the percent at 100); a Mk5 Sandbike
holds 140, the table said 100. Each number in `vehicleTank` is the tank of the one grade that was measured: Sandbike Mk1, Buggy Mk3, Light Ornithopter Mk4, Medium Ornithopter Mk5, Sand Crawler Mk6, Transport Ornithopter Mk6.

## Source

The game's module table, `CDT_BaseVehicleModules` (row structure `BaseVehicleModuleTableRow`, the composite of the per-class module tables), field `m_StaticData.m_FuelCapacity` of the **Chassis** row of each class and grade.
It is the same table `internal/catalog/vehicles.json` (#401, #467) was made from, which carries it as `fuel` on the chassis rows. Read from the game's data files (game build 1.5.15.x) on 2026-10-28. The table has 674 rows; every
row that is not a chassis has the default capacity (1000, or 0), so the chassis row is the only one that sets a tank. The unit is the one the save uses for `FVehicleComponent.CurrentFuel`.

The vehicle blueprints (`BP_*_CHOAM` and the base classes) hold no capacity: only `m_InitialFuel` (the fraction a vehicle is spawned with, 0 or 1) and `m_RefuelRate` (how fast a refuelling station fills it, 150 to 400).
There is no second table with a fuel maximum, so there is nothing for the chassis value to disagree with.

## The table

Fuel capacity of the chassis, by class and grade. The grade is the number the module id ends in (`BuggyChassis_6` is Mk6); grade 0 is the vehicle the world places. `-` means the class has no chassis of that grade.
Grades 7 to 9 are rows of the table that belong to no template and appear in no save (#401), so the editor does not know them: a vehicle with such a chassis falls back to its class's measured tank.

| Class | Mk0 | Mk1 | Mk2 | Mk3 | Mk4 | Mk5 | Mk6 | Mk7 | Mk8 | Mk9 |
|---|---|---|---|---|---|---|---|---|---|---|
| Sandbike | 90 | 100 | 110 | 120 | 130 | 140 | 150 | (140) | (170) | (180) |
| Buggy | 220 | - | - | 240 | 260 | 280 | 300 | (280) | (340) | (360) |
| Light (Scout) Ornithopter | 225 | - | - | - | 300 | 350 | 400 | (400) | - | - |
| Medium (Assault) Ornithopter | 450 | - | - | - | - | 750 | 850 | (800) | - | - |
| Transport (Carrier) Ornithopter | 1350 | - | - | - | - | - | 2500 | (2400) | - | - |
| Sand Crawler | 450 | - | - | - | - | - | 750 | (800) | - | - |
| Treadwheel (not in the editor) | 90 | 100 | 110 | 120 | 130 | 140 | 150 | - | - | - |
| Tank (not in the editor) | 350 | - | - | - | - | - | 375 | (400) | - | - |
| Container Vehicle | 0 (it has no fuel) | | | | | | | | | |

In brackets: a row of the table that no template uses. Three more chassis rows, `TankChoamChassis` 675 and the three `SeekerChassis*` rows (500), belong to a faction tank and to the flying seekers, which are not vehicles
a character owns. Treadwheel and Tank are in the game's tables but in no save and not in the editor's catalog, so Refuel leaves a vehicle of those classes alone and reports it (the class is unknown to it either way).
Where a class has few grades, those are the grades its templates use: a Buggy is built at Mk0, Mk3, Mk4, Mk5 or Mk6 and has no Mk1 or Mk2; a Medium Ornithopter at Mk0, Mk5 or Mk6; a Carrier and a Sand Crawler at Mk0 or Mk6.

The numbers that matter for what the editor offers are Mk0 to Mk6 of the first six classes: 23 chassis rows.

## The rule

A vehicle's tank (`tankOf`):

1. the fuel capacity of its own chassis module: the module in `vehicle_modules` for that vehicle whose id the vehicle catalog lists as a Chassis with a capacity, provided it belongs to the vehicle's own class;
2. else the tank measured for its class (`vehicleTank`), as before;
3. else the vehicle is left alone and reported as of an unknown kind, as before (the Container Vehicle, which has no fuel; a class the editor does not know).

Refuel raises a fuel amount to the tank and never lowers one that is already at or above it. The Fuel column shows `fuelMax` (the tank) and `fuelPercent` (never above 100).
A vehicle made by Create a vehicle is full at its chassis's capacity by the same rule (`catalog.DeriveVehicle`), so Create and Refuel agree.

## Cross-checks

* **Catalog.** Every chassis row of `vehicles.json` (24 rows: 23 with a tank and the Container Vehicle) has the capacity of the same row of the game's table. The catalog has no chassis of grade 7 to 9 and none of Treadwheel, Tank or the seekers.
* **Saves.** 208 vehicle rows over 14 played-save copies were compared with the derived tank. None holds more than its tank. The full ones sit exactly at it: Sandbike Mk1 100, Mk5 140; Buggy Mk3 240; Light Ornithopter Mk4 300;
  Medium Ornithopter Mk5 750; Sand Crawler Mk6 750; Transport Ornithopter Mk6 2500. A created Mk6 buggy in one copy holds 240 (what the old Refuel gave it), in another 295.4: both below 300.
  The Mk5 Sandbike at 140 and the six vehicles of the #401 tests are the real rows the tests use.
* **Wiki.** The vehicle pages of the wiki give no fuel figure, so there is nothing on it to compare.
* **The measured table.** All six numbers of `vehicleTank` are the capacity of one chassis row (Sandbike Mk1, Buggy Mk3, Light Mk4, Medium Mk5, Sand Crawler Mk6, Transport Mk6), which is why they were right for the vehicle that was measured and wrong for every other grade.

## What changed for a player

| Vehicle | Before | Now |
|---|---|---|
| Mk6 Buggy | filled to 240 (read as full at 240) | 300 |
| Mk4, Mk5 Buggy | 240 | 260, 280 |
| Mk0 Buggy (the world's) | 240 | 220 |
| Mk5 Sandbike | 100 | 140 |
| Mk2, Mk3, Mk4, Mk6 Sandbike | 100 | 110, 120, 130, 150 |
| Mk0 Sandbike | 100 | 90 |
| Mk5, Mk6 Light Ornithopter | 300 | 350, 400 |
| Mk6 Medium Ornithopter | 750 | 850 |
| Mk0 Medium Ornithopter | 750 | 450 |
| Mk0 Light, Carrier, Sand Crawler | 300, 2500, 750 | 225, 1350, 450 |
| anything at the grade that was measured | unchanged | unchanged |

A fuel value that was above the new, smaller tank (a Mk0 vehicle holding more than its tank, if a save has one) is not lowered.

## Tests

* the 23 chassis rows are written a second time in the test (class by grade), and `tankOf` and the catalog must equal them; the catalog has no template chassis the table lacks;
* `tankOf` falls back to the class for no modules, for modules without a chassis, for a chassis the catalog does not know (Mk7, a made-up grade) and for the chassis of another class, and returns nothing known for the Container Vehicle and for
  a class nobody knows;
* a save with the real vehicles of the #401 tests (578, 1828, 1859, 1599, 1616, 1634, with their chassis) and the created Mk6 buggy (at 240 and at a quarter, 75), a Mk3 buggy and a Mk1 Sandbike: each is filled to its chassis tank
  (Mk6 buggy 300, Mk5 Sandbike 140, Mk1 Sandbike 100, the others as before), only the fuel number of a record changes, a value above the tank stays, a vehicle nothing knows is untouched and reported, and a second press writes and logs nothing;
* one vehicle for each of the 23 chassis, filled from empty: the fuel, `fuelMax` and a percent of 100 all agree;
* the Vehicles list: Mk6 buggy at 240 reads 300 and 80 %, a vehicle above its tank reads 100 %, a vehicle nobody knows shows its fuel with no maximum;
* a buggy made by Create a vehicle (T6_Combat at a quarter of a tank: 75 of 300) is refuelled to 300 and a second press writes nothing;
* the count beside the button counts a Mk6 buggy at 240 as not full.

## To check in the game

Refuel a Mk6 buggy (for example one made by Create a vehicle) from a low tank: the gauge reads 100 % and the value in the save is 300. The same for a Mk5 Sandbike (140). Neither the new grades nor a fill of a
vehicle made by Create a vehicle have been seen in the game yet; the Mk5 Sandbike at 140 is a value the game itself wrote.

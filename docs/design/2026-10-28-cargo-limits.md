# How much a cargo hold takes (#483)

Status: built. The rule is in `internal/ops/cargolimits.go`; the card reads it in `internal/ops/vehicledetails.go`, the give uses it in `GiveToInventory` (`internal/ops/ops.go`); the tests are in
`internal/ops/cargo_limits_test.go` and `web-tests/cargolimits.test.mjs`. Not checked in the game yet.

## Problem

On Player > Vehicles the cargo hold card offered Add item for every vehicle that has a hold row, and the give accepted 5,000 Carbon Ore for a Sandbike that has no storage part. The card then read "10 of 0 slots used".

The give works from the inventory row's `max_item_count` and `max_item_volume` (`giveUpToInTx`, #345 and #357). It treats a count of 0 or less as "no limit recorded", which is right for the player's inventories and
the containers of a base (the game writes -1 there for "no limit"), and wrong for a vehicle: the hold row of a vehicle with no storage part says 0 slots and 0.0 volume, and means that it holds nothing.

## What the saves hold

Sixteen copies of single-player saves were read (decoded to plain SQLite, read only). Every vehicle in them has its hold as an `inventories` row with `inventory_type` 0 on the vehicle's actor; the per-component
inventories have no type and no limits. The hold row carries the room: `max_item_count` is the slots and `max_item_volume` the volume.

The values per kind of vehicle and parts, as stored, and what `catalog.DeriveVehicle` (the sum of `cargoSlots` and `cargoVolume` over the vehicle's modules) gives for the same modules. They are equal in every case seen:

| Vehicle and its storage part | Stored slots | Stored volume | Catalog slots | Catalog volume |
|---|---|---|---|---|
| Sandbike, no storage part (grades 0 and 1, and every other Sandbike without one) | 0 | 0.0 | 0 | 0 |
| Sandbike with `SandbikeInventory_2` | 15 | 250.0 | 15 | 250 |
| Buggy, no storage part | 0 | 0.0 | 0 | 0 |
| Buggy with `BuggyInventory_5` | 20 | 2500.0 | 20 | 2500 |
| Buggy Mk6 with `BuggyInventory_Unique_Capacity_6` (the Bigger Boot), both a Mk3 chassis and the Mk6 one made by Create a vehicle | 35 | 3500.0 | 35 | 3500 |
| Light ornithopter, no storage part | 0 | 0.0 | 0 | 0 |
| Medium ornithopter with `OrnithopterMediumInventory_5` | 20 | 2000.0 | 20 | 2000 |
| Transport ornithopter (the carrier has no storage module) | 0 | 0.0 | 0 | 0 |
| Sandcrawler with `SandcrawlerSpiceContainer_Unique_Capacity_6` | 30 | 11250.0 | 30 | 11250 |
| Container vehicle | 150 | 15000.0 | 150 | 15000 |

The storage parts of the catalog (slots / volume), by kind and grade; a vehicle's hold is the sum over its modules:

| Kind | Parts |
|---|---|
| Sandbike | `SandbikeInventory_0` 5 / 100; `_1` 10 / 250; `_2` 15 / 250; `_3` 15 / 300; `_4` 15 / 350; `_5` 15 / 400; `_6` 15 / 450 |
| Buggy | `BuggyInventory_0` 5 / 500; `_3` 20 / 1500; `_4` 20 / 2000; `_5` 20 / 2500; `_6` 20 / 3000; uniques `Unique_Capacity_3` 20 / 1750; `_4` 25 / 2250; `_5` 30 / 2750; `_6` 35 / 3500 |
| Light ornithopter | `OrnithopterLightInventory_0` 5 / 500; `_4` 10 / 500; `_5` 10 / 750; `_6` 10 / 1000; unique `SpiceCompressor_6` 10 / 1000 |
| Medium ornithopter | `OrnithopterMediumInventory_0` 5 / 500; `_5` 20 / 2000; `_6` 20 / 2000; unique `SpiceCompressor_6` 20 / 2000 |
| Sandcrawler | `SandcrawlerSpiceContainer_0` 15 / 2250; `_6` 20 / 7500; unique `Unique_Capacity_6` 30 / 11250 |
| Container vehicle | `ContainerVehicle` 150 / 15000 |
| Transport ornithopter | none |

Two things follow. A hold is not a function of the chassis alone: the same Sandbike chassis has a hold of 0 or 15 slots depending on whether it has the storage part. And the Mk6 buggy's volume and slots meet: 35 stacks
of 500 Carbon Ore (0.2 volume a unit) are 17,500 units and 3,500 volume.

The 5,000 Carbon Ore the operator gave to a Sandbike (10 stacks of 500) are in a save written after the newest copy studied, so there is no copy of that save; the tests build the same rows (a Sandbike with the modules of
a real one, a hold row of 0 and 0.0, and ten stacks of `DolomiteRock`, the in-game Carbon Ore).

## Containers of a base

Read from the same copies (`placeables` with their inventories): the game's containers have a real limit and the saves never go past it.

| Container | `max_item_count` | `max_item_volume` |
|---|---|---|
| Generic container | 20 | 750.0 |
| Medium storage container | 100 | 4000.0 |
| Spice silo | 10 | 250.0 |
| Developer storage container | 1000 | 50000.0 |
| Refineries, fabricators, generators (other groups) | 5 or 10 or 25 | 25.0 to 1000.0 |

Refineries and fabricators also have a second inventory with `max_item_count` -1 (no limit); the Bases > Inventory list leaves those out (`max_item_count >= 0`). No base container has 0. No container of any copy, and no inventory of the newest copy, holds more
distinct slots than its `max_item_count`.

Add item into a base container already stopped at both limits (the free slots, counting a stack that has room first, and the volume), because the container's count is positive. The test
`TestABaseContainerTakesWhatFitsAtItsLimit` shows a spice silo taking 5,000 of 6,000 Melange (10 stacks, 1,000 volume), then refusing one more, and a generic container never going past its 20 slots. Nothing changed for
containers. A container with a count of 0 does not exist in the saves read, so its meaning is not known and it keeps the old reading (no limit).

## The rule

`holdSlots` (`internal/ops/cargolimits.go`) gives the slots of a vehicle's hold:

- the stored `max_item_count` when there is one: 0 is no room, a negative count is no limit (the card then shows only the number of items);
- when the hold row has no count (NULL), the sum over the vehicle's modules from the catalog, if the catalog knows all of them; otherwise unknown.

A vehicle's hold is an inventory with `inventory_type` 0 on an actor in the `vehicles` table (the player's own backpack is type 0 too, but belongs to the body).

- The card (`vehicleDetails`): `cargo.slots` (nil when unknown), `cargo.used` (the distinct slot positions of the items, which is the count a give works with), `cargo.volume`, and `cargo.noHold` (slots known and 0).
- No hold: the card says "This vehicle has no cargo hold (no storage part)." and has no Add item row. If rows are in it, it says "N items in a hold with no slots." and lists them with Delete and Clear this container.
- A hold: "N of M slots used" and the Add item row.
- The give (`GiveToInventory`): inside the edit it reads the hold; a vehicle hold of 0 slots is refused with 409 `no_room`, "This vehicle has no cargo hold (no storage part).", and nothing is written. Otherwise it is the give of
  #345 and #357: as much as fits by free slots, stack size and volume, and the answer says how many did not fit and why.

Clear this container and Delete do not look at the slots; they work on a hold with 0 slots (tested with ten surplus stacks), so the surplus of an earlier give can be removed. Nothing repairs it by itself.

## Not covered

- A hold row with no count whose modules the catalog does not know has no known room; a give into it is not limited by slots (as before). No such vehicle exists in the saves read.
- Items already in a hold with room beyond its slots (a hand-edited save) are shown as they are ("12 of 10 slots used") and not removed.
- Whether the game, when it loads a vehicle with more stacks than its hold has room for, keeps, drops or moves them is not known; that is why the editor refuses to write them.

# Create a vehicle from a template (#401)

Status: built in two slices: the data, the write and the request (slice 1), and the page on Player > Vehicles (slice 2). Game build the tables were read from: 1.5.15.0.

## What it does

The console's Spawn Vehicle asks the game's server for a vehicle of a class and a template. The editor works offline on a save, so it writes the vehicle itself: the rows
the game keeps for a vehicle, filled from the game's own vehicle tables. The operator picks a class, a tier, optional replacement modules per slot, a fuel fraction, a place and
items for the cargo hold; the editor writes one new vehicle that belongs to the character. `Duplicate vehicle` (#112) already writes this set of rows from a copy of an existing
vehicle, and the game loaded such a copy (2026-10-27); this feature writes it from the tables instead of from a source vehicle.

## The rows

One edit (the ordinary one: a backup first, or queued while the game is running), the same ids as Duplicate takes (above the largest id in use and the table's sequence):

| Table | Rows | Content |
|---|---|---|
| `actors` | 1 | the class's actor path, the map and place, a turn about the vertical axis only, state 0, serial 0, no owner account; `properties` hold the class's own object (the common keys, the abandoned time = now) and `DamageableActorComponent` health |
| `vehicles` | 1 | the id |
| `permission_actor` | 1 | the class's name (`##BuggyChoam`), type 2, access 3 |
| `permission_actor_rank` | 1 | the character, rank 1 (this is what "owned" means) |
| `fgl_entities`, `actor_fgl_entities` | 2 + 2 | `Actor` (health, critical health, fuel, and the aggro, weather and shelter components with their usual values) and `VehicleInventory` (empty); ids share the prefix of the newest vehicle that has entities (else the character's) and count from above everything the prefix has, with room for the game |
| `vehicle_modules` | one per module | the module id, stats `CurrentDurability` = the table's `MaxDurability`, the swatch of a player's vehicle, and the weapon stats part for the weapon-type modules |
| `inventories`, `actor_inventories` | 1 + holds | the cargo hold (type 0, slots and volume from the storage module, hash `-72424899`) and the class's component hold(s) (Sandbike has two) |
| `items` | the cargo | by the Give Items path (slots, stack sizes, volume, the bound), as much as fits, the answer says what did not |

## Numbers derived from the modules

`catalog.DeriveVehicle` is the one rule used both to write and to check. Each was checked against vehicles of real saves (six vehicles of six classes in the tests, eleven
vehicles of seven classes when it was found):

| Saved value | Rule |
|---|---|
| `m_TotalMaxHealth` and `m_CurrentMaxHealth` (properties), `m_CurrentHealth` (entity) | sum of `Health` over the modules whose class is Chassis, Body or Cockpit |
| `m_MaxDownButNotOutStateHealth` and `m_CurrentDownButNotOutStateHealth` | the chassis's `CriticalStateHealth` |
| `CurrentFuel` of a full tank | the chassis's fuel capacity (it follows the chassis grade: a Mk5 Sandbike holds 140, not the 100 of a Mk1) times the fuel fraction asked |
| cargo hold slots and volume | the sum over the modules that provide slots (none: 0 and 0) |
| a new module's `CurrentDurability` | the module row's `MaxDurability` |

After the insert the edit checks the foreign keys of the touched tables against what they were before, re-reads the written modules, derives again and compares with the health,
critical health, fuel and cargo hold it wrote, and counts the inventories; any difference undoes the whole edit (the preview runs the same checks).

## What "Mk6" is

The grade of a part is the number its id ends in (`BuggyChassis_6` is the item "Buggy Chassis Mk6"); a template's name begins with the same number (`T6_Combat`), and the tier of a
template is the grade of its chassis (checked for all 24 templates). So **Mk6 is tier 6**: for the buggy, `T6_Combat` (the buggy's top template; its parts are all `_6`). The game stores the
grade in the template name's instance number as grade + 1 (`SandbikeChassis` instance 1 is `SandbikeChassis_0`); the catalog uses the module ids, which are the spelling of the save.
Grades 7 to 9 are rows of the module table but belong to no template and appear in no save, so they are not offered.

Templates per class: Sandbike T0 to T6 (T5 and T6 have no utility module), Buggy T0, T3_Inventory, T4_Boost, T5_Mining, T6_Combat, Light ornithopter T0, T4_Inventory, T5_Boost, T6_Combat, Medium T0, T5_Inventory, T6_Combat,
Carrier T0, T6_Boost, Sandcrawler T0, T6_Harvesting, Container vehicle Container. T0 is the grade-0 vehicle the world places. Tank and Treadwheel are in the tables but in no save and are not offered.

## Uniques and other modules

A unique is not part of any template: it is a module row of its own (`<Class><Slot>_Unique_<Effect>_<grade>`) with the same slot type as the stock module. The request names a module per slot
(`uniques: {"Engine": "BuggyEngine_Unique_Accelerate_06"}`); any module of the class whose slot type is that slot is accepted (the stock module of another grade as well: a player's
own vehicles mix grades), an optional slot can be emptied (`""`), a mandatory one cannot. The game spells the grade of some rows with a leading zero and some without
(`BuggyEngine_Unique_Accelerate_06` and `BuggyInventory_Unique_Capacity_6` are rows; the item that installs the second is `..._06`), so an id that is not a row is tried with the other spelling and the answer says
which row it became. The game's slot table makes both buggy utility slots children of the rear hull; every buggy template that fills the second utility slot (a weapon) uses `BuggyHullBackExtra`, and
the plain `BuggyHullBack` appears only with the first. A combination no template has is allowed and reported in the answer's notes, since it has not been seen built.

### The Mk6 buggy with uniques

`T6_Combat` with `BuggyEngine_Unique_Accelerate_06` (Engine), `BuggyInventory_Unique_Capacity_06` (UtilitySlot1, row `BuggyInventory_Unique_Capacity_6`) and `BuggyMining_Unique_YieldIncrease_06` (UtilitySlot2),
the rear hull `BuggyHullBackExtra_6` (already the template's) and the stock Mk6 chassis, generator, front hull and four treads: 11 modules, health 650, critical 150, a full tank of 300, a cargo hold of 35 slots and 3,500 volume.

## Place

Default: the character's map, `x + 800`, same `y`, `z + 150`, facing as the character does (Bring vehicle and Duplicate put a vehicle at the same offsets). Or `place: {map, x, y, z}` (centimetres, finite, within 5,000,000).
Nothing snaps a vehicle to the ground: one placed below the terrain falls, one placed high drops.

## Request

- `GET /api/vehicles/catalog`: the game build; for each class its slots, its templates (tier, modules, health, fuel, cargo) and for each slot the modules that fit.
- `POST /api/vehicles/spawn-preview` and `POST /api/vehicles/spawn` with `{class, template, uniques?, fuel? (0 to 1, default 1), place?, items?: [{template_id, quantity, quality?}]}`. Both answer with
  the id, the rows (`rows`), the derived numbers, what was changed from the template, notes, and for the items how many were given and what did not fit. The preview keeps nothing; the apply takes the id the preview named
  (if nothing else was written in between). Both refuse an unknown class, template, slot or module, a module in the wrong slot, an empty mandatory slot, fuel outside 0 to 1, a bad place, an unknown item and items for a
  vehicle with no cargo hold. Apply has no confirmation step.

## The page (Player > Vehicles)

A card **Create a vehicle** with one button, **Create a vehicle...**; nothing else of the form is drawn until it is pressed, and the catalog is not read before. The form opens on the buggy at its top tier:

- **Class**, **Tier** (the Mk number and the rest of the template's name: "Mk6 Combat"; changing either starts the parts again from the template), **Fuel %**.
- **Change parts** (folded): one list per slot with every part of the class that fits it, uniques marked, storage parts with their slots; an optional slot can be left empty. Only the slots that differ from the template are sent (`uniques`).
- **Items for the cargo hold**, drawn only while the chosen parts give one: the item box of Give Items, a quantity, Add; each row has a quantity box and Remove.
- **Preview** asks `spawn-preview` and shows what would be written (the vehicle's id, class, tier, parts, health, fuel, cargo hold, how many items fit, notes, a table of the parts); any change to the form drops it.
- **Create** posts `spawn` at once: no question is asked (the standing rule for this editor's edits that only add rows; the edit is backed up first, or queued while the game is running, and Restore on the Database tab puts the file back).
- After a create the form closes and a line stays until dismissed: "Created vehicle N: ... It is written to the save; the game has not loaded it yet." (plus the queue sentence when the edit is queued).

The explanation is in the card's About this and in the tooltips of Preview and Create, not on the page (the word budget of #426 holds).

## Not proven

No vehicle composed from the tables has been loaded in the game. What is proven is the row shapes (a copy written by Duplicate loaded and flew), the derived rules (against real vehicles) and the template contents (against real vehicles). Open: the entity ids (the
`Actor` entity's component base words are written as 0), the weapon stats of a launcher or a dart module (seen only on a mining laser and a scanner), placement height, and what the next game update changes in the tables.

## Checks in the game

With a backup of the save and the game closed while the editor writes, for each class in turn:

1. Create the vehicle next to the character; load the save: it is there, on the ground, the right model.
2. Get in: fuel gauge full, health full, the modules present and usable (boost, scanner, weapon), the cargo hold has the expected slots.
3. Open the vehicle's module panel: every slot lists the expected module and grade, durability full.
4. Drive or fly 2 minutes; leave it; save and quit to the menu; reload: it is still there, fuel burnt, nothing doubled.
5. Store it in the vehicle backup and take it out again.
6. A unique swap changes the cargo hold (the Mk6 buggy above: 35 slots, 3,500 volume), and the items put in are in it.
7. T0 vehicles are accepted as the character's own.
8. If a vehicle does not appear, look for the rows in the save (they should still be there) and the game's log line.

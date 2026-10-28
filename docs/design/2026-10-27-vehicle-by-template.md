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

Default: next to the character, facing as the character does. Or `place: {map, x, y, z}` (centimetres, finite, within 5,000,000), which is used exactly as given (the answer adds a note when the place has base pieces, buildings or another
vehicle within reach of the vehicle). Nothing snaps a vehicle to the ground: one placed below the terrain falls, one placed high drops.

"Next to the character" used to be a blind point: the character's map, `x + 800`, same `y`, `z + 150`, which Bring vehicle and Duplicate vehicle also used. In a base that point is inside it (#480, below). Since then the
three choose the spot by a search; the rules are in [Where a vehicle goes](#where-a-vehicle-goes-480).

## Where a vehicle goes (#480)

### What went wrong (evidence from a save, 2026-10-28)

A copy of a single-player save, taken after Create a vehicle had put a Mk6 buggy next to the character in a base (Hagga Basin; centimetres):

- The character stood at (179,628, -36,142, 8,554.4). The base is one building of 178 pieces: a 5 x 5 block of foundations, 512 apart on a grid turned 40 degrees (22 foundations; the other three cells are stairs and ramps at the same level), all at z 8,165.9;
  two courses of walls and windows round it (z 8,549.9 and 8,933.9), floors at 9,317.9, two more courses (9,701.9, and 10,085.9 with the floor), ramps and a roof piece up to 10,853.9. The sub-fief's claim is one cell (5,120 across) about its totem's origin.
- The character was in the ground-floor room, in cell (0, 1.25) of that grid (cell units of 512 from the middle foundation), 4 cm above the foundation top (8,549.9): the saved z of the character is the level of the feet.
- The old default point, (180,428, -36,142, 8,704), is in cell (1.21, 0.25): inside the 25.6 m square of the base, in the same room, 154 above the floor and 614 under the floor of the next storey; the walls round that room are 660 away. A
  buggy is about 4 m across: it was set down in a closed room, and the game's physics moved it (it was found in the middle of the base, cell (-0.08, 0.10), at z 8,804, 254 above the floor). Nothing about the spot was outdoors.
- Heights: a foundation stands 384 high from its z; each course of walls is 384 (the courses of this base are at 384, 768, 1,152, 1,536 and 1,920 above the foundations' z); a floor's z is its top surface (placeables on it stand 3.5 above it).
  The base's stairs and ramps start at the foundations' z, so the ground outside is at about that level: the ground-floor room is a storey above it.

What the save knows: `building_instances` (type, place and turn of each piece), `placeables`, the other vehicles' places, `totems` and `landclaim_segments` (the claim's cells, with the totem's origin and yaw, as the Land claim editor reads them). What it
does not know: the terrain, and the level's own props (rocks, cliffs, ruins). A point can still be inside a rock, and a base standing on a slope has ground at other heights than its foundations'; a vehicle in the wrong place is moved with Bring vehicle.

### The search

The first candidate is the old point. If the character is not on a base piece and nothing is in the way there, that is the answer (the same numbers as before, whatever the class). Otherwise the candidates are the points of rings
round the character, radius 800, 1,200, ... up to 8,000 (80 m), 12 or more on a ring (one per 400 cm), in this order: the direction the search starts in (east; for a character in a base, away from the middle of the base, the character's own side), then
alternately to either side of it. The first candidate that is clear is taken.

A candidate (x, y, z) for a vehicle of radius R and height H is **clear** when:

1. no base piece is within R + 100 of it in the plan at any height from 800 below z to 2,500 above (this keeps the vehicle out of a room, out of a garage under a floor and off a roof, not only out of the walls);
2. no placeable (taken as a circle of 250) and no other vehicle (a circle of its class's radius; a stored vehicle is not in the world) is within reach of the vehicle's body, z to z + H;
3. it is not shut in: of 16 lines out from it, every one ends at a piece within 30 m (a courtyard of walls with no way out and no roof).

Pieces are boxes, from what the save shows: foundations, floors, roof pieces, stairs and ramps are a whole cell (512 x 512); a wall, window or door frame is a 512 x 80 slab turned by the piece's own yaw (in the base above the long side lies
along the cell's edge); a column is 120 x 120; corners, rounds, wedges, inclines, triangles and every kind not seen count as a whole cell. Foundations, walls, stairs and ramps rise 384 from the piece's z; a floor or roof slab is thin and reaches 100 below
and 30 above its z. These are the saved pieces' own spacings and stacking, not the game's mesh sizes.

**The character is in a base** when a piece lies under, in or over the character (a circle of 60 round the feet; pieces from 800 below to 2,500 above). Then:

- the vehicle goes to the ground outside: its z is the **bottom of the nearest foundation** plus a drop of 250 (the stairs and ramps of a base start there, and the vehicle falls the rest); the character's own floor is not used. With no foundation in reach
  (6,000), the lowest piece of the character's building is used instead. Terrain is not read, so on a slope the fall is longer or the ground is higher than this: see below;
- a point **outside the land claim** the character is in is preferred when there is one within 30 m of the character; if not, the nearest clear point is taken, and the answer says "still inside the land claim". Being in a claim without being on a base piece
  (a character in the open on their own land) changes nothing: the character's own height is the best ground there is.

When no candidate within 80 m is clear, the request is refused: "there is no clear spot for a ... within 80 m of the character: base pieces, buildings or other vehicles are in the way. Walk to open ground, or give a place". Nothing is written.

### Footprints

The game's tables (`vehicles.json`) carry no sizes. The numbers are chosen, in centimetres: the radius of a circle that holds the vehicle at any heading (about the half diagonal with the wings out), and the height; a margin of 100 is added to the radius.

| Class | Radius | Height | Reason |
|---|---|---|---|
| Sandbike | 250 | 200 | about 3 m long |
| Buggy | 400 | 300 | about 4.5 m long, with the rear hull |
| Light ornithopter | 800 | 450 | wings out |
| Medium ornithopter | 1,200 | 550 | generous: wings out, about 16 m across |
| Carrier (transport) ornithopter | 1,800 | 800 | generous |
| Sandcrawler | 2,200 | 1,000 | generous: the largest vehicle |
| Container vehicle | 450 | 350 | a cargo box |
| A vehicle of another kind | 500 | 400 | |

They were not measured in the game; a wrong radius makes the vehicle too close to a wall or too far from the character, never a different vehicle. If a class is seen clipping a wall, raise its radius in `vehicleSizes` (`internal/ops/vehicle_place.go`).

### What the answer says

`place` in the answer of the preview and of the create (and of Bring vehicle and Duplicate vehicle) holds `map`, `x`, `y`, `z`, `words`, `moved`, `inBase`, `blocked` (what was at the old point: `pieces`, `placeables`, `vehicles`), `claim`
(`outside`, `inside` or empty) and `distance`. `words` is the sentence the page shows: "next to you"; "9 m north of you (base pieces at the first spot)"; "outside your base, 14 m south of you, outside the land claim (base pieces and things you built at the first spot)".
North is up and east is +x, as in the Land claim editor.

### Bring vehicle and Duplicate vehicle

Both use the same search, from the character's position at the time of the edit. Bring vehicle does not count the vehicle it moves as an obstacle (bringing it again leaves it where it is); Duplicate does count the original (the copy is put beside it).
The footprint comes from the vehicle's own class. A vehicle that is stored is still refused, as before.

### Not known, and open

- Terrain and the level's props (above). The height outside a base is the foundation level plus 250; where the ground is more than 250 higher there, the vehicle appears under it and falls; where it is lower, the vehicle drops.
- Whether the game's own placing of a vehicle would have moved it. The footprints are chosen, not measured.
- Not tested in the game: the operator checks it by standing in a base, creating a buggy and a Carrier ornithopter or a Sandcrawler, and seeing that neither is in a wall or on another floor.

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
`Actor` entity's component base words are written as 0), the weapon stats of a launcher or a dart module (seen only on a mining laser and a scanner), the height of the ground outside a base (#480), and what the next game update changes in the tables.

## Checks in the game

With a backup of the save and the game closed while the editor writes, for each class in turn:

1. Create the vehicle next to the character; load the save: it is there, on the ground, the right model. Then stand on the upper floor of a base and create a buggy and a Carrier ornithopter or Sandcrawler (#480): neither is in a wall, in a room or on another floor, and the page said where each went.
2. Get in: fuel gauge full, health full, the modules present and usable (boost, scanner, weapon), the cargo hold has the expected slots.
3. Open the vehicle's module panel: every slot lists the expected module and grade, durability full.
4. Drive or fly 2 minutes; leave it; save and quit to the menu; reload: it is still there, fuel burnt, nothing doubled.
5. Store it in the vehicle backup and take it out again.
6. A unique swap changes the cargo hold (the Mk6 buggy above: 35 slots, 3,500 volume), and the items put in are in it.
7. T0 vehicles are accepted as the character's own.
8. If a vehicle does not appear, look for the rows in the save (they should still be there) and the game's log line.

# Recycle a base and return everything

Player > Bases lists every base of the save. A base that is still standing has a **Recycle base** button (#1176, phase 1). It deletes the base and puts **everything** into your
character's **Reserve**: all that was stored in the base, and all that the base cost to build.

It is a delete. The base is gone afterwards, no base backup is made, and the only way back is to restore a backup of the save (see [Undo](#undo-restore-the-save)).

The game has its own recycle, for a base that was backed up with the Base Reconstruction Tool. This button is for a base that still **stands**, and it differs from the game's in
one way that matters: the game's recycle returned about half of the building cost in the one case that was measured, and this one returns all of it. That is a decision of the
project's operator, not a copy of the game.

## What it does, in short

| | |
|---|---|
| Deleted | The base: its totem, every placeable that belongs to the totem, every building piece, its land claim, its fuel times, its permission records, its marker on the map, and the respawn point at it |
| Moved to the Reserve | Every item stored in the base, unchanged: chests, the totem's own slots, windtrap filters, generator fuel and lubricant, refinery and fabricator slots |
| Added to the Reserve | The refund: what every piece and placeable cost to build, as new stacks |
| Dropped | Water in cisterns and windtraps; a unit of fuel that is already burning |
| Made first (only when you tick the box) | A Solido of the base, the game's own copy that can rebuild it: see [Make a Solido first](#make-a-solido-of-the-base-first-option) |
| Not touched | Vehicles, every other base, every base backup, everything else in the save |
| Not written | A base backup. The Deep Desert never has one in the game, and this edit writes none on any map |

## Use it

1. **Close the game.** That is the recommended way. While a game process exists the edit is only queued, and it is written when the game has closed. A queued recycle is put
   back onto the save the game wrote last, and it is **dropped, not written**, when the game has saved a change to anything the recycle read: the base, its contents, the
   Reserve. A base in use changes all the time (water, fuel, a chest you open), so a recycle queued while you play is likely to be dropped. The editor says so when it happens
   and nothing is recycled; ask again with the game closed.
2. Let every crafting queue in the base finish, or cancel it in the game. A base with a running queue is refused.
   **The Reserve must be empty.** The game allows one Reserve, so no base is recycled while it holds any stack. If an earlier recycle left stacks in it, press [Move the Reserve into storage](#move-the-reserve-into-two-storage-boxes) first: it empties the Reserve into your two storage boxes. Pressing Recycle base with a Reserve that is not empty is refused with "the Reserve is not empty (it holds N stack(s)) and no recycle can happen until it is empty", and nothing is changed.
3. Press **Recycle base** on the base. The editor works out the whole edit and asks first: it does the edit, reads it back, runs the save's reference check and its check of stored JSON on it and undoes
   it, so a base that cannot be recycled is refused here, before any question. Nothing has been written at this point. The question lists:
   - how many building pieces and placeables are deleted;
   - how many item stacks (and items) move to the Reserve;
   - the refund, item by item, and how many new stacks it is;
   - the respawn point at the base, when the base is one: it is deleted with the base and you can no longer respawn there;
   - holograms, Developer storage containers and water, when the base has any;
   - when the box **Make a Solido of the base first when I recycle it** on the Bases card is ticked, the first line says how many pieces and placeables the Solido holds and in which
     device it goes (see below);
   - that everything goes to the Reserve, and that it cannot be undone except by restoring a backup of the save.
4. Press **Recycle base** in the question. It is one edit. The editor takes a backup copy of the save first and then writes (or queues the edit while the game runs; the backup
   is then taken when the edit is written, and the question says that a queued recycle is dropped when the game saves a change to the base in between). The write is sent the fingerprint of what the question showed, and is refused when the base changed in between.
5. Start the game, load the save and open the Reserve. To put things into storage boxes, use [Move the Reserve into storage](#move-the-reserve-into-two-storage-boxes) (below), or
   move them in the game.

## Move the Reserve into two storage boxes

A recycle puts everything into your Reserve. **The Reserve can only be filled by the game** (a recycle); you cannot put anything into it yourself. So it is a one-way inbox, and the way
to make room for the next recycle is to move what is in it into storage (#1191). **Move the Reserve into storage** does that into **two Developer storage containers you have named in
the game**, in one edit, whole stacks only.

**What it does.** Every item stack of your character's Reserve moves into the two boxes, **whole and unchanged**: the same item rows, with the same stack size, durability, grade and
everything else. Only the inventory and the slot of each row change (the same two columns a recycle changes when it moves what a base stored). A stack is never split, merged, topped
up, recreated or deleted. The stacks go in the order they have in the Reserve (slot, then item number); each goes into the **first box that has both a free slot and room by volume**
(box 1 until it is full, then box 2), into that box's lowest free slot. Nothing else in the save changes.

**How the boxes are found.** By the **name you gave each box in the game**: the editor reads the name from the box's permission record (`permission_actor.actor_name`, the same name
Bases > Inventory shows). Each name must match **exactly one** Developer storage container (`Developer_StorageContainer_Placeable`; names are matched **exactly, capitals included**, because the save keeps the name as it was typed in the game and nothing is known of the game ignoring case; spaces, tabs and line ends at the start or end of a name are ignored, on both the name in the save and the name you type, and two boxes that differ only in such spaces count as two matches; and a box you never renamed
has no name to be found by: its "name" is the game's placeholder). When a name finds no box, the message lists the names the editor did read from the save (and how many
containers were never renamed), so you can see what to type; the save only holds a name once the game has saved after you renamed the box. That one box must also:

- stand in a base of **yours** (your character holds its owner rank, alone), in the normal state, and not be a hologram;
- **not be in the Deep Desert**, and its base must not be either: the base's totem must have a map recorded, must not be on the Deep Desert (in any capitals), must be in the normal state
  (not picked up) and must not be part of a base backup;
- **not be in a base that is being recycled** (the later multi-base run gives the editor those bases; a run by hand has none to give, so be sure yourself);
- have a slot count above 0 (a box whose count is 0, -1 or empty is refused: the editor does not know what it is);
- hold its contents in places the game would use: a box that already has two stacks on one slot, or a stack beyond its slot count, or a stack with a volume of its own below 0, is refused
  ("the box ... has items in places the game would not use"), and is not touched. Put its contents in order in the game first.

The two names must be different boxes. A name that matches **no** box, a name that matches **two or more** (the message says how many and where), the same box twice, a Deep Desert box,
a box of another player, a hologram, a box that is not in the normal state: each **refuses the whole move**, names what was found, and changes nothing.

**Room: the same slot and volume rules as Give item.** Before anything is changed the editor checks the whole Reserve against both boxes, with the code Give item uses (the same free-slot
count, the same unit volumes, the same volume count), applied to each whole stack. A box holds `max_item_count` stacks (slots), and **every stack takes one slot, whatever it is**: cosmetics
and unique items take one slot each, like any other stack. Volume is checked when the box has a volume limit above 0 (`max_item_volume`; 0 or none recorded means no volume check, as a
give reads it): each stack counts its unit volume times its size, the unit volume being the row's own `volume_override` when it has one (per unit) and otherwise the game's item table.
**Units with no known volume (cosmetics, unique items and placeables that have no volume in the game's item table and none of their own) count as none by volume, exactly as in a give**:
they cost their slot and no volume, and the answer says how many units had no known volume (the same goes for such units already in a box). A stack with a volume of its own below 0 is
refused. What is already in the box counts. If it does not fit, **nothing is changed**, and the message says what was needed, what each box has free (slots and volume), and the shortfall.
The fit is worked out stack by stack in the order above, so a Reserve that would fit only with a different order is refused too: make room in the boxes or name boxes with more.
One difference from a give is kept on purpose: a give treats a box whose slot count is -1, 0 or empty as having no limit, but the move refuses such a box (see the list above), because a
Developer storage container holds 1,000 slots in every save seen, so a box without a count is something this version does not know, and a move cannot be undone in the game.

**All or nothing.** One edit. Inside it the editor reads back what it wrote: the Reserve is empty, every stack is in the box and slot that was planned, no other column of any moved
row changed (a digest of the rows before and after), no two stacks share a slot, no box is over its slot count. Any difference fails the whole edit and nothing changes. The save layer
then runs its reference check and its check of stored JSON, as for a recycle. The write must be sent the **fingerprint** the question showed (the Reserve, the boxes and the plan), and
is refused when anything changed in between. A backup of the save is taken first, as for every write. A real run from the command line takes **one more** backup first: a verified
copy named `-manual`, which the editor never prunes (the write path then takes its own automatic backup as well, so a real run leaves two copies). While a game process exists the edit is only queued, put back onto the save the
game wrote last, and **dropped, not written**, when the game saved a change to anything it read (a stack of the Reserve, a box, its contents).

**An empty Reserve** is not an error: the answer says so and nothing is written (the boxes are still checked first, so a wrong name is always said).

### Walk-through

1. **In the game, prepare two boxes.** Place two Developer storage containers in a base of yours that is not in the Deep Desert and is not one you are about to recycle. Name them in
   the game exactly `DD-1` and `DD-2` (capitals and the hyphen included; they are matched exactly). Make room: a Developer storage container holds 1,000 stacks in the saves seen, so two mostly empty boxes are plenty. Save and **close the game**.
2. **Dry run.** In PowerShell:

   ```powershell
   & "$env:USERPROFILE\Downloads\tabr-tau\tabr-tau.exe" move-reserve --boxes "DD-1,DD-2" --dry-run
   ```

   Change the path on that line if you keep `tabr-tau.exe` elsewhere. With more than one save on the machine, add `--save "<path to game.db>"` before the word `move-reserve`. The
   dry run reads the save and **writes nothing**; it prints how many stacks and items move, into which box, and how much room is left. A name it cannot find, a box it may not use or
   a Reserve that does not fit is printed as a refusal, and nothing was changed.
3. **Run it.** The same line without `--dry-run`. **Close the editor window first**: a real run refuses while the game runs and while the editor holds the save open. It takes a verified
   backup copy of the save (named in its output; kept, never pruned), moves the Reserve, writes the save through the editor's own write path (which takes its own automatic backup too), then opens the written
   file again and checks that the Reserve is empty. If the write cannot be made it says "not changed" and leaves nothing queued. It ends with the lines of the move.
4. **Check in the game.** Start the game, load the save, open the Reserve (it should be empty) and the two boxes (the stacks are in them). If anything looks wrong, close the game and
   restore the backup copy from **Database > Save backups** (see [Undo](#undo-restore-the-save)).

The editor does the same from **Player > Bases**: the **Box 1** and **Box 2** fields offer `DD-1` and `DD-2`; change them if your boxes are named otherwise (the editor keeps what you type in this browser) and press **Move the Reserve into
storage**. The question lists the same numbers; OK writes it.

**Not checked in the game.** That the game accepts a save in which the Reserve's stacks sit in a Developer storage container, and shows them there with the right slots and weight, has
not been tried. The edit moves the rows the same way a recycle moves what a base stored, and was run on copies of a real save, not in the game.

## Make a Solido of the base first (option)

Tick **Make a Solido of the base first when I recycle it** on the Bases card (#1187), then press **Recycle base**. A Solido is the game's own copy of a base: an item that holds
the base's pieces and placeables, so that the base can be rebuilt from it. The editor makes it in the **same edit** as the recycle, before the base is deleted, from the base's
rows as they are in that moment: the same lock, the same read back, one all-or-nothing write. If anything about the Solido cannot be written or does not read back, the whole recycle
is refused and nothing changes. Without the box the recycle is exactly what it was.

**Which item.** A Solido is a `BuildingBlueprint_CopyDevice` item. The editor uses the **lowest-numbered blank device the character holds** (in the backpack, the worn gear or the
loadout; no blueprint record in its stats and no row of `building_blueprints` pointing at it). A Solido that is already filled is never changed or reused. When the character holds
no blank device, a **new** device is made in the character's **backpack** (inventory type 0), the way the dedicated server's console and the editor's own give put an item there,
with the same slot and volume check, in its first free slot. Only when the backpack has no room is the device made in the **loadout** (inventory type 15, the 8-slot hotbar, which is
usually full). When neither has room, the Solido is refused by name (it says which of the two is full) and nothing changes: free a slot in the game, or recycle without the Solido.
The answer and the question say where the device is.

**What is in it.** The conversion was measured on a pair of saves from the game, taken before and after a Solido of a Hagga base was made (2,141 pieces and 254 placeables went in,
2,133 and 251 came out), and the editor's test makes the Solido of that base again from the save before and compares every row with the game's, to within 0.01:

| | |
|---|---|
| Where it is measured from | The base's **building actor**, not the claim totem and not the world: a piece's place is its place minus the actor's, turned by minus the actor's yaw; its yaw is its yaw minus the actor's; height is a plain difference |
| Pieces | Every piece of the base, except those whose `building_flags` has the bit 4 (8 of 2,141 in the measured base; what the bit means is not known). `provides_stability` is the bit 1 of the flags. `health` 0 and `hologram` 1 on every row |
| Placeables | Every placeable of the base except the **claim totem** (the Solido item holds no claim) and the **doors and gates** (a type with "Door" in its name: both doors of the measured base were left out; the gate `Gate_M_SquishUp_Placeable` is the sibling of the door `Door_S_SquishUp_Placeable` and is left out by the same rule, with or without an owner's prefix as in `Choam_Door_S_SquishUp_Placeable`; no gate was in the measured base, so this part is by the sibling's rule, not by a measurement; it is the only `Gate` type among the placeables of the cost table, the others are building pieces such as `*_Railing_Gate` and `*_Gate_Big`, which the game's flag decides on, not the name). Holograms and a Developer storage container stay. `hologram` 1 on every row |
| Angle columns | In the game's own labelling the column `transform_pitch` holds the yaw and `transform_yaw` holds the pitch (the self-hosted Dune console's import swaps the same two names) |
| Pentashields | One row for each Pentashield placeable that is kept: its three scale numbers are the placeable's `m_PersistentScale` times 10 (a scale of 8.000014 is 80) |
| The item | Its stats gain `FBuildingBlueprintItemStats`, a blueprint record with `PlayerBlueprintId` `!!bbp#<the blueprint's number>` and an empty `PlayerBaseBackupId`, after its two usual records |
| The blueprint row | One row in `building_blueprints` (the number comes from the table's own counter), `item_id` the device, no owner, an empty map |

**Pieces the base does not own.** Only the pieces the base itself owns (those of the totem's owner entity) go into the Solido. A piece of the base's building actor that another owner
entity holds, or none, is still deleted by the recycle (the game moves a building actor whole, and the recycle deletes the actor), but it is **not in the Solido**. The answer says how
many: "N pieces of the building actor that belong to no base or to another one are recycled with it but are not in the Solido". The measured base had none.

**What is not the game's.** The game numbers the pieces 0 to n-1 and the placeables 0 to m-1. Its piece order follows the base's own piece numbers, ascending, in 2,027 of the 2,132
steps from one piece to the next (the other 105 are out of order, and no rule for them was found); its placeables are scattered through the base's order, with no rule found either. The
editor's order is simply the base's own, ascending, for both. Nothing refers to the pieces' numbers (the base's own `instance_id` is not copied anywhere), and the one reference there
is, from a pentashield row to its placeable's number, is correct in the editor's numbering. The rows are the same set; their numbers are different.

**Refused by name (not measured).** A base with more than one building actor or none; a building actor, a piece or a placeable that is tilted (pitch or roll: every one in the
measured saves stood upright); a Pentashield whose scale cannot be read or is not a whole number of steps; a base with nothing a Solido can hold; a device whose stats are not a JSON
object. A `Choam_PentashieldSurface*PlacementCorner` placeable (horizontal or vertical) that has no `m_PersistentScale` in its properties refuses the **whole** Solido, because a
Pentashield's scale is part of its row and none is guessed: recycle that base without the Solido.

**Deep Desert.** A Deep Desert base is made the same way as any other, but that rests on a stood-up copy: the check was made on a copy of a save in which a Deep Desert base was set
standing again, not on a Solido that the game made of a Deep Desert base. The game's own Deep Desert Solido was not seen.

**Not checked in the game.** The box's explanation and the question both say so: "Not yet tried in the game: keep your save backup until you have placed it." That the game shows this Solido in the backpack and previews the base from it, and that it can be placed, has not been tried: only that the rows are
those the game wrote for a base of its own.

## Every rule

### Which bases

Only your own bases that stand. The totem must hold the owner rank (1) for your character and for nobody else, the same check as Back up and remove. Any map, the Deep Desert
included. One base at a time (the layout roll can recycle several bases in a row, one recycle each, moving the Reserve into the boxes DD-1 and DD-2 between them: see [Recycling the Deep Desert bases](help/deep-desert-layout-roll.md#recycling-the-deep-desert-bases---recycle-bases-off-unless-you-give-it)). Refused, with nothing changed:

- a base of another player, a base you are only a member of, a base nobody holds the owner rank over;
- a base the game already picked up (a base backup), and a base or an actor of it that is not in the normal state;
- a base with a crafting queue that is running, or whose queue cannot be read;
- a base that holds a building piece or placeable type whose cost is not in the cost table (the message names the type);
- a base an actor of which is a vehicle, or is part of a base backup, or shares an entity with something outside the base;
- a base a character is waiting to respawn at (the save names its respawn point as the pending one): respawn in the game first;
- a base that a row of any other table of the save points at (for example a game event, a spawner's actor, a travelling actor): the message names the table and the number
  of rows. Every reference the save's own layout declares to an actor, an inventory or an entity is looked up, so nothing is left pointing at a row that is gone;
- a building actor that also holds pieces of another base;
- a save in which the character does not have exactly one Reserve;
- a Reserve that holds any stack (#1202): the game allows one Reserve, so no further base can be recycled until it is emptied. The question and the write both refuse, and the write checks again inside its own transaction, so a Reserve change the game saved in between is caught. Use Move the Reserve into storage to empty it.

### The refund: 100% of the building cost

For every building piece and every placeable of the base, the refund is what the game's own building tables say one unit costs to place, in the same items (Granite Stone,
Salvaged Metal, ingots, components). The amounts are summed per item and written to the Reserve as new stacks of the item's own stack size (500 for building materials), the
last stack holding the rest. The Reserve is empty when a recycle starts (see above), so every stack in it afterwards is the base's own stored items or the refund.

Materials are not split further. The cost of a refinery is returned as the ingots and components it was built from, not as ore and water.

The cost table is `internal/catalog/buildcosts.json` (752 piece types, 527 placeable types, game build 26592269); `internal/catalog/PROVENANCE.md` says where it comes from.

### Deep Desert: half

A piece or placeable costs half in the Deep Desert (the game's map table gives that map a placement cost multiplier of 0.5). A base that stands there gives back what was paid
there: half the table cost of each unit. Half of an odd cost is rounded up per unit (a foundation of 15 gives 8). How the game itself rounds is not measured.

### A door counts once

A door frame is a building piece, and its door is a placeable with its own row. The game's tables also list the door as "attached" to its frame. The refund counts the frame once
and the door once, as its own row; the attached entry is the same door and is not added again.

### Holograms give nothing

A placeable that is still a hologram (placed but not built) was not paid for. It is deleted with the base and gives nothing back. A hologram of a type the cost table does not
know is not a reason to refuse.

### Developer storage container: 10 Iron Ingot

A Developer storage container gives 10 Iron Ingot (`IronBar`) each, on any map, not its table cost of 20. This is the operator's rule.

### No cost, no recycle

Every piece and placeable has a building cost. A type the cost table has no entry for is a gap in the table, never a free piece: the base is refused, the message names the type,
and nothing is changed. Report the type so the table can be completed.

### Contents go to the Reserve

Every item row of every inventory of the base moves to the Reserve as it is: the same item, stack size, durability, quality and everything else. Only the inventory and the
position of the row change. This covers storage containers, the totem's own slots, windtrap filters, the fuel cells and lubricant of generators and turbines, and what lies in
the slots of refineries and fabricators. An item that has its own inventory (a container item) keeps it.

The Reserve is the character's own inventory of type 33. It has no size limit in the saves seen. The editor finds it by its type, never by a number.

### Water is dropped

Water in a cistern or a windtrap is an amount in the device, not an item. It cannot be returned and is lost with the device. The question says how much.

A generator that is running has already taken one unit of fuel out of its inventory to burn it. That unit is gone too.

### Crafting queues are refused

A refinery or fabricator with a running queue has taken its ingredients and not yet made its product. What the game does with them on a recycle is not known, so the base is
refused until the queue is empty.

### Vehicles are not touched

Vehicles are not part of a base. A vehicle parked in the base stays where it is, with what it holds.

## A worked example

**The measured case.** The game's own recycle was measured once, on a small base in the Hagga Basin (a before and after pair of saves, 2026-11-23): 23 building pieces, a totem, a
door and a spice silo with four stacks in it.

| Type | Count | Cost of one | Sum |
|---|---:|---|---|
| Walls, floors and roofs (six Sardaukar types) | 21 | Granite Stone 10 | Stone 210 |
| Tall door frame | 1 | Granite Stone 7 | Stone 7 |
| Foundation | 1 | Granite Stone 15 | Stone 15 |
| Tall door | 1 | Granite Stone 10 | Stone 10 |
| Spice silo | 1 | Salvaged Metal 35 | Salvaged Metal 35 |
| Totem (Advanced Sub-Fief) | 1 | Salvaged Metal 40 | Salvaged Metal 40 |
| **Table cost** | | | **Granite Stone 242, Salvaged Metal 75** |

| | Granite Stone | Salvaged Metal | The four stacks in the silo |
|---|---:|---:|---|
| The game's own recycle (measured) | 125 | 37 | moved to the Reserve, unchanged |
| Recycle base in this editor (100% of the table cost) | 242 | 75 | moved to the Reserve, unchanged |
| The same base in the Deep Desert (half, rounded up per unit) | 122 | 38 | moved to the Reserve, unchanged |

The game's 37 Salvaged Metal is exactly half of 75, rounded down. Its 125 Granite Stone is a little more than half of 242; one base is not enough to say why.

**A large base.** Run on a copy of a real save: a Hagga Basin base of 2,141 building pieces (64 types) and 253 placeables besides the totem (57 types).

- Deleted: 255 actors with 337 entities, 83 inventories, 2,141 building pieces, 254 placeables (the totem included), 182 permission records, 5 land claim segments, the marker.
- Moved: 3,492 item stacks (2,319,976 items).
- Refund, 180 new stacks: Granite Stone 24,632; Silicone Block 19,801; Duraluminum Ingot 15,630; Cobalt Paste 6,165; Plastanium Ingot 5,165; Spice Melange 3,882; Industrial Pump
  2,080; Complex Machinery 1,970; Calibrated Servok 1,054; Salvaged Metal 615; Plastone 537; Advanced Machinery 480; Plant Fiber 250; Aluminum Ingot 175; Agave Seeds 170; Iron
  Ingot 150; Steel Ingot 138; Armor Plating 90; Copper Ingot 30; Military Power Regulator 30; Thermoelectric Cooler 25.
- 1 hologram (a directional wind turbine) gave nothing. 11 Developer storage containers gave 10 Iron Ingot each (110 of the 150).
- Dropped: 9,401,157 water in 117 devices.

## Exactly what is written to the save

One edit, all or nothing.

**Deleted rows** (the tables the game's own recycle deleted from in the measured pair, and four more that a standing base can still have rows in):

| Table | Rows |
|---|---|
| `actors` | the totem, every placeable the totem's entity owns, the building actors that hold its pieces |
| `actor_fgl_entities`, `fgl_entities` | the entity links of those actors and the entities they point to |
| `inventories`, `actor_inventories` | the inventories of those actors and their links |
| `building_instances`, `buildings` | every piece of the building actors, and their rows |
| `placeables`, `totems`, `totem_fuel_update_times` | the placeables rows, the totem row, its fuel times |
| `permission_actor`, `permission_actor_rank` | the permission records of those actors (the totem's and each container's or door's) and their ranks |
| `landclaim_segments` | the claim's cells. The measured base had none left (it was already picked up); they hang from the totem in the save's own layout and go with it |
| `player_respawn_locations` | the respawn points whose locator is an actor of the base (the `BaseTotem` row of a base you can respawn at). The save's own layout deletes them with the actor; the measured base had none. Respawn points elsewhere, and those of vehicles, are not touched |
| `player_markers` | your own HomeBase marker at the totem's position. A marker that is not clearly this base's is left, and the answer says so. The `markers` table itself is not changed |

Placeables of the base that have no buildable support belong to the totem as well and go with it (Back up and remove leaves them standing; a recycle leaves nothing).

**Changed rows:** each stored item row in `items`: `inventory_id` becomes the Reserve, `position_index` a free position there. Nothing else of the row changes.

**With the Solido box ticked, also:** one `building_blueprints` row, the rows of `building_blueprint_instances`, `building_blueprint_placeables` and
`building_blueprint_pentashields` (the Solido), and in `items` either a changed row (the blank device's stats gain the blueprint record) or one new row (the new device, with its
two usual records and the blueprint record). The Solido is written first; it is read back with the rest (every row of it, as a digest, and the device's record), and the preview runs the same
reference check and JSON check on it.

**New rows:** the refund stacks in `items` (the Reserve, a free position each, marked new, with the item's usual durability record, as every give in the editor writes it), and the item id counter
(`items_id_sequencer`) moved past them.

**Never written:** `base_backups` and `base_backup_linked_actors` (no backup), `vehicles` and anything of a vehicle, any other base.

**Read back.** Before the edit is accepted, the editor reads back what it wrote, inside the same edit: the number of rows deleted from every table, that no row of the base is
left, that every stored stack is in the Reserve with its size, that every refund item has the planned amount in the planned number of stacks, and that no two stacks of the
Reserve share a position. Any difference fails the whole edit and nothing is changed. The save layer then checks the database's own references and refuses an edit that leaves
a dangling one. The preview runs that same reference check, and the save layer's check of the JSON cells an edit wrote (the refund stacks carry one), before it undoes its edit, so this refusal comes when you ask, not after you confirmed. What the preview cannot
know is what changes in the save after the question was shown; the fingerprint catches that, and the write is then refused.

## Check it

Before you recycle, take your own checkpoint of the save, so that you can compare and go back whatever happens. The commands are for PowerShell. They are complete: copy a whole
block and paste it. The first line of each block says where `tabr-tau.exe` is; change that one line if you keep it somewhere else.

**1. Take a checkpoint (game closed).**

```powershell
$save = Get-ChildItem -Path "$env:LOCALAPPDATA\DuneSandbox\Saved\Cloud\PlayerClientStorage" -Recurse -Filter "game.db" | Where-Object { $_.FullName -notmatch "tabr-tau-backups" } | Sort-Object LastWriteTime -Descending | Select-Object -First 1 -ExpandProperty FullName
$checkpoint = "$env:USERPROFILE\Documents\tabr-tau-checkpoints\before-recycle-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
New-Item -ItemType Directory -Force -Path $checkpoint | Out-Null
Copy-Item -Path $save -Destination "$checkpoint\game.db"
Get-FileHash -Algorithm SHA256 -Path $save, "$checkpoint\game.db" | Format-Table -AutoSize Hash, Path
```

The two hashes must be the same. `$save` is the newest `game.db` of the game; the last command prints its path, so you can see it is the right one.

The editor also takes its own backup before it writes: a copy of the save in the `tabr-tau-backups` folder next to `game.db`, listed under **Database > Save backups**. You
can take one by hand there with **Back up now**.

**2. Recycle the base** in the editor, as above. Read the question before you confirm: the numbers in it are what will be written.

**3. Compare the save with the checkpoint.** This lists, table by table, what changed, into a text file, and opens it.

```powershell
$tau = "$env:USERPROFILE\Downloads\tabr-tau\tabr-tau.exe"
$save = Get-ChildItem -Path "$env:LOCALAPPDATA\DuneSandbox\Saved\Cloud\PlayerClientStorage" -Recurse -Filter "game.db" | Where-Object { $_.FullName -notmatch "tabr-tau-backups" } | Sort-Object LastWriteTime -Descending | Select-Object -First 1 -ExpandProperty FullName
$checkpoint = Get-ChildItem -Path "$env:USERPROFILE\Documents\tabr-tau-checkpoints" -Directory -Filter "before-recycle-*" | Sort-Object Name -Descending | Select-Object -First 1 -ExpandProperty FullName
$report = "$checkpoint\what-changed.txt"
Start-Process -FilePath $tau -ArgumentList @("diff", "`"$checkpoint\game.db`"", "`"$save`"", "--max-rows", "5") -NoNewWindow -Wait -RedirectStandardOutput $report
notepad.exe $report
```

What to expect: rows removed (`-`) from the tables listed above and from no other, rows changed (`~`) only in `items` (one for each stored stack), rows added (`+`) only in
`items` (the refund stacks), one changed row in `items_id_sequencer`, and no line for `base_backups` or `vehicles`. If the game ran in between, it also changes rows of its own
(the character's position, timers).

**4. In the game:** load the save, go to where the base stood (it is gone, and so is its marker), open the Reserve and look for the stored items and the refund.

## Undo: restore the save

A recycle cannot be undone piece by piece. Go back to the save as it was.

**With the editor (the normal way).** Close the game. Open **Database > Save backups**, pick the backup taken just before the recycle, press **Restore** and type the word it
asks for. The file being replaced is backed up first, so a restore can itself be undone. See "Restoring a backup" in the [README](../README.md).

**By hand, from your own checkpoint.** Close the game **and** the editor first. This puts the checkpoint back over the save, keeping the current file beside the checkpoint.

```powershell
$save = Get-ChildItem -Path "$env:LOCALAPPDATA\DuneSandbox\Saved\Cloud\PlayerClientStorage" -Recurse -Filter "game.db" | Where-Object { $_.FullName -notmatch "tabr-tau-backups" } | Sort-Object LastWriteTime -Descending | Select-Object -First 1 -ExpandProperty FullName
$checkpoint = Get-ChildItem -Path "$env:USERPROFILE\Documents\tabr-tau-checkpoints" -Directory -Filter "before-recycle-*" | Sort-Object Name -Descending | Select-Object -First 1 -ExpandProperty FullName
Copy-Item -Path $save -Destination "$checkpoint\game.db.after-recycle-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
Copy-Item -Path "$checkpoint\game.db" -Destination $save -Force
Get-FileHash -Algorithm SHA256 -Path $save, "$checkpoint\game.db" | Format-Table -AutoSize Hash, Path
```

The two hashes must be the same. The block uses the newest `before-recycle-*` folder; to use another one, replace the second line with
`$checkpoint = "$env:USERPROFILE\Documents\tabr-tau-checkpoints\<the folder's name>"`. Everything done in the game after the checkpoint is lost with the restore.

## Known gaps

- **A Solido is optional, and its numbering is not the game's.** See [Make a Solido first](#make-a-solido-of-the-base-first-option). The editor never reads or places a Solido,
  and it does not check that one exists before a recycle without the box.
- **Storage boxes as the destination are a separate step.** A recycle puts everything into the Reserve; [Move the Reserve into storage](#move-the-reserve-into-two-storage-boxes) empties it into two
  named boxes afterwards (#1191). The recycle button itself does one base and leaves the Reserve full, and **it refuses the next base until the Reserve is emptied** (the game allows one Reserve, #1202): press Move the Reserve into storage between two recycles. Only the layout roll does both in one run, base after base (next item).
  The weekly layout roll (`layout-roll --recycle-bases`, #1167) recycles the Deep Desert bases of the character one at a time, each with its Solido, and needs the Reserve to be empty before each base: it empties the Reserve into the boxes named `DD-1` and `DD-2` between the bases (and after the last one), on the same working copy and with the same move, so several bases go in one run when the boxes have room. When a box is missing or full it recycles the bases that fit and stops before the roll; a re-run continues.
- **The game's own recycle returns about half.** The editor returns everything, by the operator's choice. A base recycled here gives more than the same base recycled in the
  game. An "as the game does" mode is not built: the game's rate and rounding are known from one base only.
- **Deep Desert rounding.** Half of an odd cost is rounded up per unit. What the game charges for such a piece there has not been measured.
- **Not measured in the game for a standing base.** The rows deleted are those of the game's recycle of a base *backup*. What the game does with the totem's own slots, the slots
  of refineries and fabricators, and filters and fuel on a recycle was not in the measured base (its only contents were the four stacks in the silo); the editor moves them all,
  by the operator's rule.
- **Eleven cost items the editor's catalogs do not know.** Eleven placeable types cost an item that is in neither the stack-size table nor the item catalog, so the editor
  does not know how the item stacks and will not guess: a base that holds one of them is refused, with the item named. They are `MediumInsulatedCistern_Placeable` (JasmiumDust
  30), `SmallInsulatedCistern_Placeable` (ScrapElectronics 50), and nine deployables that cost one of themselves: `CamouflagedStilltent_Placeable`, `ProtectiveStilltent_Placeable`,
  `CompactSeismicProbe_Placeable`, `SeismicProbe_Placeable`, `LongRangeSeismicProbe_Placeable`, `CompactThumper_Placeable`, `LongDurationThumper_Placeable`,
  `PortableFabricator_Placeable` and `Turret_Placeable`. Nine more deployables (`Stilltent_Placeable`, `Stilltent_Placeable_Unique_01`, `Thumper_Placeable`, the three
  `UniqueThumper_Placeable` types, `RespawnBeacon_Placeable`, `StakingUnit_Placeable`, `StakingUnitVertical_Placeable`) cost one of themselves, an item the catalog knows and the
  stack-size table does not list; it comes back as one stack of one for each. `Ladder_Placeable` has no cost in the game's tables at all and is refused too. None of these
  types was in a base of the saves checked.
- **The cost table is of one game build** (26592269). A piece type added by a later build is refused until the table is made again.
- **The item id counter.** The editor moves the save's item id counter past the new stacks, as every give in the editor does. The game's own recycle left the counter as it
  was in the measured pair.

## What has been checked

Tests on made-up saves of both marker layouts (what is deleted, that the Reserve receives the items unchanged, the refund sums, the Deep Desert half, the Developer storage
container, the door, the hologram, a type with no cost, a running queue, every ownership refusal, a stale fingerprint, a read back that differs, vehicles untouched), the
routes, the page in a browser engine, and the edit applied to a copy of a real save (the large base above): rows went from exactly the tables listed, every stored item row
arrived unchanged, the save layer's reference check passed, and the written copy opened again and compared table by table with the save it was made from. **Not checked in the game:** loading a save after a recycle made here.

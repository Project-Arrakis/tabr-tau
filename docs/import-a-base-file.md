# Import a base file into a save

Player > Bases > **Base backups** has an **Import a base file** control. It reads a `tabr-tau-base` file (made by **Export base backup**, see
[`export-a-base-backup.md`](export-a-base-backup.md)) or a `dune-base-backup` file (made by the self-hosted Dune console, see
[the console's file](#the-dune-consoles-base-backup-file) below) and adds its base to the open save as a **new base backup**. This is the second step of moving a base from one single-player save
to another (#475). **Whether the game lists and redeploys an imported backup has not been checked in the game yet**; the steps for that check are at the end.

The editor does not put the pieces in the world. It writes the same rows the game's own base tool leaves when it picks a base up (state 5, a backup record and its links), and the
game's **Base Reconstruction Tool** is what places a backup. Nothing already in the save is changed, replaced or removed.

## Import one

1. Close the game (the editor queues the edit while a game process exists and writes it when the game has closed, as for every other edit).
2. Open **Player > Bases**. Under the **Base backups** list, press **Import a base file** and choose the file.
3. The editor reads the file whole and shows a preview: how many actors, building pieces, placeables and stored items it would add, whose base it becomes (the character of this
   save), and warnings. The preview is the real import run and undone: **nothing in the save has changed**.
4. Press **Import**. It is one edit, written after the usual backup copy of the save. It asks no question, because it only adds rows. When the game's patches differ between the two
   saves the button reads **Import anyway**.
5. Start the game, load the save, and look for the backup in the Base Reconstruction Tool.

Importing the same file twice adds two backups, each with its own ids. A file the editor refuses is refused whole: the save is exactly as it was and the page says why.

## The Dune console's base backup file

The self-hosted Dune console can export a base backup as a `dune-base-backup` file (version 1; written by console v1.4.52 when this was built). Choose it the same way. The editor sees the
format name, turns the file into the same rows a `tabr-tau-base` file holds, and then everything above and below applies unchanged: the strict read, the size and row caps, the checks, the
preview with its hash, the backup before the write, new ids, one edit, no question. The preview starts with a line that says the file came from the console, with its name, map and the counts
the console gave it ("Read from a file of the Dune console (v1.4.52), base backup "My Base", map HaggaBasin, 811 pieces, 204 placeables, 21 stored items, exported ...").

What the conversion does:

| In the console's file | In the save |
|---|---|
| a transform list `[x, y, z, qx, qy, qz, qw]` on a piece; `{location: {x, y, z}, rotation: {x, y, z, w}}` on an actor; `landclaim_original_global_location` `[x, y, z]` on a totem | The plain columns of the save (`location_x/y/z`, `rotation_x/y/z/w`, `landclaim_original_global_location_x/y/z`), the numbers as they are |
| `true` / `false` | 1 / 0 |
| the actor state `BaseBackup` | 5, as for every import (any other state refuses the file) |
| an `fgl` entry (an entity with its actor, slot name and components) | one row of `fgl_entities` and one of `actor_fgl_entities` |
| the entry ids ("transfer ids"), and every column that names one | Taken as the ids of the file's rows; the import gives every one a new id of the save, as for an editor file |
| `"!!act@12"` strings inside JSON (a totem's own id, a station's item ids) | Read as the reference to the entry 12 and rewritten to the new id |
| `"!!act#12"` strings inside JSON (an id on the console's server, which names nothing in the file) | The null reference `!!act#0` for an actor (counted in the preview); any other kind refuses the file |
| the owner placeholder (an `act` entry with no data) and the backup's `player_id` | The character of this save, like the owner of an editor file |
| entity ids (small numbers in the console's file, where a save's are 64-bit) | New 64-bit ids under the prefix of the save's latest entity, above every counter the save holds |
| `dimension_index` (a server numbers its copies of a map; the file has 2) | The dimension of this save's character (0 in every save seen), and the preview says how many actors were moved |
| `owner_account_id`, `partition_id`, a null `exchange_id`, `item_id` or `vehicle_module_id` | Left out: the save has no such column |
| the console's `__lb` note of array lower bounds | Left out: nothing in a save is an array |

The file is refused whole, with the entry named and the save unchanged, when: it holds a kind of entry a save has no table for (`TaxInvoice`, `Sinkchart`, `bbp` and the three Solido
blueprint kinds) or one this editor does not know; an entry has a field this editor does not know, lacks one, or holds a value of the wrong kind (a text for a number, a list of the wrong
length); an inventory belongs to an item, a vehicle module or the market; an id is used twice; the placeholder carries data or is referenced where only the backup record may name it; and for
everything the other sections list (an id outside the file, an actor not linked to the backup, a stored Solido item, and so on). The rank (`PermissionActorRank`) and claim segment
(`LandclaimSegment`) kinds are read from the console's documented columns; the real file above has none of either, so they are tested with made-up rows only.

The preview also says the game build the console's server ran (a save does not record which build wrote it, so the editor cannot tell whether the components are stored the same way), and
when the file's own counts differ from its entries.

## What the import does to the rows

| In the file | In the save |
|---|---|
| actor, inventory, item and entity ids; the backup's id | **New ids for all of them.** No id of the file is ever used. Actors, inventories and the backup get the next free numbers; items get numbers from the game's own item counter, which is moved past them; entities keep the 32-bit prefix of their id and get a counter above every counter the save has |
| `!!act#N`, `!!itm#N` (and `inv`, `fgl`) strings inside JSON | Rewritten to the new ids (a totem's own id, a crafting station's item ids). The text is cut and joined, not read and written again, so key order and numbers stay as they were. After the write the JSON is read back from the save and must hold exactly the references that were written |
| a reference to an actor that is not in the file (a queued recipe names the character who started it) | The null reference `!!act#0`, counted in the preview. Any other reference outside the file refuses the file |
| player ids: the backup's owner, rank rows, who placed a piece, who edited a permission record | The character of this save (rank rows of several players are merged into one, with the lowest rank number; the preview says so) |
| actor state | 5 (picked up) for every actor, whatever the file says (the preview counts the actors that were standing) |
| the account of the actors, the buildings' owner | Left empty, as the game leaves them when it picks a base up |
| everything else (positions, health, stats, stack sizes, slots, names, serial numbers) | As in the file |

## When the editor refuses

The whole file is refused, nothing is written and the reason is shown, when:

- it is not a base file (for a console file: not one JSON object, another version, a header key the editor does not know, no owner placeholder), or, for an editor file: not one JSON object, another format, version or kind, a table or column that is not on the editor's list, a value of the wrong kind, counts or a checksum that
  do not match, text that is not valid UTF-8, a name with a control character, a NUL in JSON;
- it is over a limit: 16 MiB, 250,000 rows, a name longer than 1,000 characters, JSON nested more than 100 levels;
- a row names an id that is not in the file (an item in an inventory that is not there, an owner entity, a linked actor, a claim segment of no totem), an id is used by two rows,
  an entity belongs to no actor, an actor is not linked to the backup, or there is not exactly one backup in the file;
- a class, map, building type or item template is not one of the game's names, an actor is a player's, or an actor is in a state other than 0 or 5 (a vehicle's, for example);
- a stored item is a Solido blueprint (its rows are not carried yet), a stack is empty, a required value is missing;
- a reference inside JSON is of a kind the editor does not move, names an item or inventory that is not in the file, or sits inside other text;
- the save has no character, or its character is not among its actors, or its entity ids leave no room for the file's.

## What the preview warns about

Not refusals: actors that were standing and are written as picked up; references to an actor outside the file set to none; stored items whose template is not in the editor's item
catalog (kept as they are); a base on a map other than Hagga Basin (the game redeploys a base only on the map it was picked up on, and a Deep Desert layout can differ between saves);
a file from a save with a different set of applied game patches (the components may be stored differently; the button becomes **Import anyway**); rank rows merged.

## What to check in the game (UAT)

(The same list holds for a console file; items 7 and 8 are about it.)

With the game closed, import the file into a **copy** of the save (the editor makes a backup copy first, but use a copy), then:

1. Does the **Base Reconstruction Tool list** the imported backup (log out and in first if it does not)?
2. Does **redeploy** work: every piece and placeable present, storage contents present, generators and water as exported, the claim established, the character the owner?
3. A **crafting station with a queued job**: does it load and continue (its `InstigatorActorId` was set to none and its item ids were remapped)?
4. After a session, are the imported rows still there and unchanged?
5. Importing the **same file twice**: are both listed, and can both be redeployed?
6. A Deep Desert base into a save whose layout differs; a file from an older patch set.
7. **A console file:** does the Base Reconstruction Tool list the base backup imported from a real console export of a base, and does redeploy work (the pieces, placeables and the 21 stored items present, the claim established)? Pieces were written with the console's positions and rotations as plain columns.
8. The console file's actors were in dimension 2 and were written in dimension 0 (the character's): did the base appear where it was exported?

## What has been checked

Tests on made-up saves of the shape of a real pick-up (the same save, a save with no base, a save whose id counters are high), the round trip (export, import, export again: the tables
are equal once ids are renumbered), every refusal leaving the save byte-identical, the preview leaving it byte-identical, the routes and the page in a browser engine; and the real
export of a base (56 actors, 179 pieces, 55 placeables, 99 stored items, 736 rows) imported into copies of two real saves, previewed, imported, exported again and compared, then written
to a file and read back; and the real console export of a base (205 actors, 811 pieces, 204 placeables, 21 stored items, 2,006 rows) imported into copies of three real saves, with the foreign-key
check and the integrity check passing, and exported again as an editor file. None of that says what the **game** does with the rows: that is the list above.

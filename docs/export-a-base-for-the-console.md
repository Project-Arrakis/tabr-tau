# Export a base as the Dune console's base backup file

Player > Bases has an **Export for console** button. It saves a base as the self-hosted Dune console's own **base backup** file (`format: "dune-base-backup"`, version 1), the file the console's
Bases page offers as "Base Backup" and its import reads. It is the way a base from a single-player save can go to a multiplayer game (#534, continuing #475). **Whether a multiplayer console
accepts the file, and whether the game redeploys the base from it, has not been tried yet**; what only a game can answer is listed at the end.

Exporting only reads the save: nothing in the save changes, nothing is written to the game's folder, nothing asks first, and it works the same while the game is running. The editor's own
export (`tabr-tau-base`, see [`export-a-base-backup.md`](export-a-base-backup.md)) is a separate file for another single-player save; this one is for the console.

## Two places, two kinds of base

| Where | For | What the file holds |
|---|---|---|
| The **Bases** list, **Export for console** on a row | A base that is still standing: its claim totem is placed and listed in the Bases list. Not shown for a base the game's tool picked up | The same entries a pick-up followed by the console's own export would give |
| The **Base backups** card, **Export for console** next to **Export base backup** | A base the game's tool already picked up | The backup as the game left it |

The browser saves `tabr-tau-console-base-<number>-<name>.json` (the totem's number) or `tabr-tau-console-backup-<number>-<name>.json` (the backup's number); the folder is your browser's
download setting. A message repeats what the file holds: "Exported ...: 2,000 pieces, 100 placeables, 200 stored items. Nothing in the save changed." Anything the editor wants you to know about
the file is added to that message.

## A standing base, entry by entry

This follows what the console documents for exporting a live base (`exportLiveBase`; see the research note, section 1c):

- **Which actors.** The totem, the placeables that the totem owns and that stand on the base (`has_buildable_support`), and the building actors that hold the totem's pieces. The pieces are those whose
  owner entity is the totem's `Actor` entity: another base's piece in the same building actor is left out. The Bases list counts the same pieces.
- **State.** Every actor is written in the backed-up state, as the game leaves it after a pick-up. The save is not changed.
- **Building actors** are written as the fresh copies a pick-up makes: class, map, transform and dimension, no properties, serial 0, and no entity.
- **Storage and stations** come along: the inventories of the placeables, their items with every column, and the entities (components) of every actor except the building actors.
- **No permission row of the totem and no invoice.** A pick-up removes them. The placeables' own permission rows stay; a rank row stays only when it is the owner's.
- **A backup record is built from the totem.** Its name is the totem's permission name (the game's own `##Totem_...` placeholder when it has none); its owner is a **placeholder** (an actor entry
  with no data), which the importing player replaces. The file carries no player name, account, path or setting of the editor.
- **The claim** (the totem row and its claim segments) is in the file.
- **Ids.** The file has its own small ids; no id of the save, and no 64-bit entity id, is written. Every id inside JSON (`!!act#12`, `!!itm#7`) is written in the one form the console takes, a
  transfer id after an `@` (`!!act@12`). No `#` form is left anywhere in the file (#538); the next section has the three cases that were.
- **Unfinished placements.** A placeable that is still a hologram is kept, as the console's export keeps it. The Bases list does not count holograms, so the file can hold more placeables than
  the row shows; the message says how many of them are holograms.
- **Game version.** The file says how many game patches the save has applied and the latest five. It leaves the build and the patch checksum empty, because a save holds neither; the console
  therefore asks to import it anyway.

## A picked-up base

The backup's rows are written as they are, with the owner as the placeholder and every actor in the backed-up state (an actor that is not is counted in the message: the base may have been put
down again after it was backed up). A rank row of another player than the owner is not carried.

## Ids inside JSON: what is written, and why (#538)

The console's import refuses a file that holds an id in the save's own form: "Non-transfer id !!itm#0 is found at .properties.WeaponActorComponent.m_FavoriteWeaponItemDatabaseId." The
editor's reader is kinder (it takes `!!act#12` as an id of another save), which is why the first version of this export passed the editor's own check and failed in the console. The rule
that is now written and checked:

| In the save | In the file | Why |
|---|---|---|
| an id of a row that is in the file (a totem's own id, a station's item ids) | `!!act@12`, with the file's id of that row | the console's form |
| the null id `!!itm#0`, `!!act#0` (a weapon component with no favourite weapon, an empty slot) | `!!itm@0`, `!!act@0` | the console writes the null reference as `@0` and turns `@0` back into the null id on import; the property stays, as the game wrote it |
| an id of an actor that is not in the base (`InstigatorActorId` in a crafting queue: the character who queued the job) | `!!act@0`, counted in the message | see below |
| an id of an item, inventory or entity that is not in the base | `!!<kind>@0`, counted in the message | there is nothing in the file to name |

**Why the instigator becomes none and not the owner placeholder.** The console's import turns the placeholder into the importing player's controller actor, and a queue names the
character's pawn, so the placeholder would name an actor of the wrong kind. A queued job whose instigator is none keeps its recipe, its ingredients (the item ids are in the file) and its
progress, and loses only the name of who queued it. The console's own export writes `@0` for a reference to something outside the base on the paths it lets leave the base, and its
validation lets the placeholder be named only by the backup record and a rank row. What is **not known**: the console's import is done by the game's database functions, which are not
in the console's code, and its list of paths where an outside id is allowed is not public. `@0` is read from the console's tests and from its stand-ins for those functions, not from the
game's own code. If the console still refuses a file, its message names the path, and this table is where to change it.

## Checked before the file is handed out

Two checks run on the finished text of every file, and a file that fails either is not handed out (the message says why, and nothing is written):

1. **The console's rule for ids.** Every string value (and every key) of every entry that holds an id mark is, as a whole, `!!<kind>@<number>`, and the number is 0, or the id of an entry of
   that kind in the file, or (for an actor) the id of the owner placeholder. Anything else, a `#` form, an id inside other text, an id that names nothing in the file, is refused with the
   entry and the place in it.
2. **The editor's own reader.** The editor reads the file it just made with its own reader for the console's files (the one that **Import a base file** uses) and compares the counts of
   what it reads with what it wrote.

## When the editor refuses

The export stops, says why, and writes nothing, when:

- there is no base with that totem, or no backup with that number;
- the base was picked up (use the Base backups card);
- nobody holds rank 1 over the base (the console refuses a base with no owner the same way);
- the totem has no entity, so its pieces cannot be told from another base's;
- a storage of the base holds a Solido blueprint, or an inventory of the base belongs to an item or a vehicle (the file does not carry them);
- a number is not finite, a JSON value is not an object or a reference sits inside other text, or a row has no value where the file needs one (a piece with no position, for example);
- the base is over the limits of a file: 250,000 entries or 16 MiB;
- the finished file holds an id the console would refuse (a `#` form, or an `@` id that names nothing in the file);
- the save was edited while the file was being made (try again).

## Not in this step

- **The Blueprint (Solido) file** the console also offers for a standing base (layout only, positions relative to the totem, yaw only, claim consoles left out) is a later step. It waits until the
  base backup file has been tried in a game, and until a real Solido exists to compare against (#475 phases 3 and 4, #111).
- Writing anything into a save: the export is read only.

## What has been checked

Tests on a made-up save: a standing base gives the same rows as the same base after a pick-up (both files imported and exported again, the tables equal once the ids are renumbered), the
pieces of another base in the same building actor are left out, every refusal above leaves the save unchanged, and two exports of the same save are the same bytes. A golden test reads the
console's file fixture with the editor's reader, writes it again and reads that again: the rows are equal once the ids are renumbered. A base with a weapon component whose favourite is none and a
crafting queue whose job was queued by a character outside the base is exported, every string value of every entry is walked and none holds a `#` id, and the strict check takes the file and refuses the
same file with one id put back in the old form. The route and the page are tested too.

## Compared with a file the console made

A base exported by the console itself (a standing base, its Download Base) was read, structure only, and compared with a file of the editor. What differs, and what was done:

| Where | The console's file | The editor's file | Judgement |
|---|---|---|---|
| ids inside JSON | only `@` forms | only `@` forms (before #538: three `#` forms) | **fixed** |
| `__lb` on a piece (`{"transform": 0}`) and on the totem (`{"landclaim_original_global_location": 0}`) | on every piece and on the totem | was missing | **fixed.** The game writes its arrays 0-based and reads the totem's location by index (the console's documentation says so); the console records the bounds per row and its import restores them. Without the note the importing server stores 1-based arrays |
| `game.build`, `game.steamBuildId`, `game.patchesChecksum` | filled | empty, `null`, empty | **kept on purpose.** A save holds none of them and the editor does not invent one; the console reads the empty checksum as a different game version and asks, and **Import Anyway** goes on |
| `game.appliedPatchesCount`, `latestPatches` | the server's | the save's count and its latest five | kept; informational |
| `console.version`, `console.buildId` | the console's | empty; one more key, `writtenBy`, names this editor | kept; the console's import does not read the block, and unknown keys are ignored |
| `source.ownerName` | the owner's name | empty | kept; the file carries no player name |
| `source.kind`, `baseId`, `backupId`, `name`, `rawName`, `map`, `totemType`, `counts` | present | present, same types | same |
| `format`, `version`, `exportedAt` (UTC, milliseconds), `ownerPlaceholderTransferId` | present | same values and form | same |
| order of keys | the database's (shorter names first) | the editor's (`format` first, `entries` last) | no effect: the console parses the file as JSON and finds an entry by its `kind` and `id` |
| order and numbering of entries | as the database gave them | ids from 1 in import order; the owner placeholder is 1 | no effect: the console loads entries by id and imports in its own order |
| the owner placeholder | an `act` entry with no data, named by `ownerPlaceholderTransferId`, used by the backup record | the same | same |
| entry kinds | the 14 kinds of a base, and the Solido kinds when a chest holds one | the 14 kinds; none for Solidos or invoices | kept; a base that holds a Solido is refused (see above) |
| keys of each kind's `data` | the console's | the same keys and types for every kind the editor writes (the `__lb` fix above was the one gap) | same |
| numbers | `real` columns, shortest text | the save's `REAL` values as exact decimals (more digits) | no effect: the same values |
| raw player ids (`last_placed_by_player_id` and the like) | the source server's | the save's | the console's import sets every non-zero one to the importing player |

Nothing else was found to differ in a way the console checks. Whether the game redeploys the base is below.

## What only a multiplayer game can answer

1. Does the console accept the file now (no "Non-transfer id" error; the version check on the game build and the patch checksum, the placeholder owner)? Does the receiving player see the base in the in-game base backup tool
   and redeploy it with pieces, placeables, storage and generators intact?
2. The save's actors are in dimension 0. Whether a multiplayer server expects another dimension number in the file is not known; the editor writes what the save holds.
3. A base in a layout that differs from the receiving server's (Deep Desert), and a crafting station with a queued job (its `InstigatorActorId` is written as none: does the job continue?).
4. Unfinished placements (holograms): keep them, as written, or drop them?

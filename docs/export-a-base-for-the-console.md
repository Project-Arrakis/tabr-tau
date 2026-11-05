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
download setting. A message repeats what the file holds: "Exported ...: 2,150 pieces, 128 placeables, 273 stored items. Nothing in the save changed." Anything the editor wants you to know about
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
- **Ids.** The file has its own small ids; no id of the save, and no 64-bit entity id, is written. A reference inside JSON (`!!act#12`, `!!itm#7`) is rewritten to the file's id (`!!act@12`). A reference
  to an actor that is not in the base (a crafting queue that names a character) is written as the save holds it, as the game's own export leaves it, and the message counts it. A reference to
  an item, inventory or entity that is not in the base becomes the null reference, and the message counts it.
- **Unfinished placements.** A placeable that is still a hologram is kept, as the console's export keeps it. The Bases list does not count holograms, so the file can hold more placeables than
  the row shows; the message says how many of them are holograms.
- **Game version.** The file says how many game patches the save has applied and the latest five. It leaves the build and the patch checksum empty, because a save holds neither; the console
  therefore asks to import it anyway.

## A picked-up base

The backup's rows are written as they are, with the owner as the placeholder and every actor in the backed-up state (an actor that is not is counted in the message: the base may have been put
down again after it was backed up). A rank row of another player than the owner is not carried.

## Checked before the file is handed out

The editor reads the file it just made with its own reader for the console's files (the one that **Import a base file** uses) and compares the counts of what it reads with what it wrote. A file
that the reader refuses is not handed out; the message says why.

## When the editor refuses

The export stops, says why, and writes nothing, when:

- there is no base with that totem, or no backup with that number;
- the base was picked up (use the Base backups card);
- nobody holds rank 1 over the base (the console refuses a base with no owner the same way);
- the totem has no entity, so its pieces cannot be told from another base's;
- a storage of the base holds a Solido blueprint, or an inventory of the base belongs to an item or a vehicle (the file does not carry them);
- a number is not finite, a JSON value is not an object or a reference sits inside other text, or a row has no value where the file needs one (a piece with no position, for example);
- the base is over the limits of a file: 250,000 entries or 16 MiB;
- the save was edited while the file was being made (try again).

## Not in this step

- **The Blueprint (Solido) file** the console also offers for a standing base (layout only, positions relative to the totem, yaw only, claim consoles left out) is a later step. It waits until the
  base backup file has been tried in a game, and until a real Solido exists to compare against (#475 phases 3 and 4, #111).
- Writing anything into a save: the export is read only.

## What has been checked

Tests on a made-up save: a standing base gives the same rows as the same base after a pick-up (both files imported and exported again, the tables equal once the ids are renumbered), the
pieces of another base in the same building actor are left out, every refusal above leaves the save unchanged, and two exports of the same save are the same bytes. A golden test reads the
console's file fixture with the editor's reader, writes it again and reads that again: the rows are equal once the ids are renumbered. The route and the page are tested too.

## What only a multiplayer game can answer

1. Does the console accept the file (the version check on the game build and the patch checksum, the placeholder owner)? Does the receiving player see the base in the in-game base backup tool
   and redeploy it with pieces, placeables, storage and generators intact?
2. The save's actors are in dimension 0. Whether a multiplayer server expects another dimension number in the file is not known; the editor writes what the save holds.
3. A base in a layout that differs from the receiving server's (Deep Desert), and a crafting station with a queued job (its `InstigatorActorId` names a character of the single-player save).
4. Unfinished placements (holograms): keep them, as written, or drop them?

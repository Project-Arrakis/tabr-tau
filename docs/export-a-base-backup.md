# Export a base backup as a file

Player > Bases has a **Base backups** card. It lists the bases the game's own base tool has picked up, and **Export base backup** saves one of them as a file. This is the first step of
moving a base from one single-player save to another (#475). The file can be opened and read; **importing it into a save is not built yet.** Exporting only reads the save: nothing in
the save changes, nothing is written to the game's folder, and nothing asks first.

## Make a base backup in the game

A base backup is what the game leaves when you pick a base up with its base tool (shown in the item list as "Base Reconstruction Tool"). The buildings stay in the save, the claim is gone,
and the game keeps a record of the base's parts. Pick a base up, let the game save, close the game, and open the editor (or press reload in it). A base that is still standing is not a
backup and is not listed (exporting a standing base is a later step).

## Export one

1. Open **Player > Bases**. The **Base backups (N)** card is under the list of bases.
2. Each row shows the backup's number, its name, the map, and the counts the file will hold: **Actors**, **Building pieces**, **Placeables** and **Stored items**.
3. Press **Export base backup** on the row. The browser saves `tabr-tau-base-backup-<number>-<name>.json`; the editor does not choose the folder, your browser's download setting does
   (the editor's own window asks where to put it or uses its download folder, like the care package file).
4. A message repeats the counts: "Exported ...: 174 pieces, 43 placeables, 53 stored items. Nothing in the save changed."

To check the file, open it in a text editor. The numbers under `"summary"` are the numbers on the row, and each table lists its rows, one to a line.

The export reads the save as the editor holds it, including edits that are waiting to be written. It works the same while the game is running.

## What is in the file

One JSON object (read by the editor strictly, like the care package file):

| Part | Holds |
|---|---|
| `format`, `version`, `kind` | `tabr-tau-base`, `1`, `base-backup`. A file of another format, version or kind is refused. |
| `exportedAt`, `editor` | When the file was made and which version of the editor made it. These two are the only parts that differ between two exports of the same save. |
| `manifest.backup` | The backup's number and name in the save it came from. |
| `manifest.summary`, `manifest.counts` | The counts shown on the card, the rows per table and the total. |
| `manifest.source` | Facts about the save that are not private: the maps of the actors, how many actors are in each state (5 is picked up), and the game's applied patches (how many, a checksum of their names, the latest five), which tell whether two saves are of the same game version. |
| `manifest.references` | Where the base names something by id: inside the file, outside it, or the null reference. Outside references (a crafting queue that names your character, for example) are also listed in `manifest.warnings`. |
| `manifest.notCarried` | What the save holds for the base that the file does not: inventories that belong to a stored item (a bag, a tool's slots) and the items in them. |
| `manifest.checksum` | SHA-256 of `tables` with the white space between tokens removed. A file that was edited or cut off no longer matches. |
| `tables` | The rows, with their **original ids**, of 15 tables: `base_backups`, `base_backup_linked_actors`, `actors`, `fgl_entities`, `actor_fgl_entities`, `permission_actor`, `permission_actor_rank`, `inventories`, `items`, `actor_inventories`, `buildings`, `building_instances`, `placeables`, `totems` and `landclaim_segments`. |

Only those tables and the columns listed for them are ever written; no table or column name comes from the save or from a request. The file never holds the account column of the actors,
the character's name, the platform or Funcom ids, a file path or a setting of the editor.

Ids are exact. An entity id is a 64-bit number, larger than a web page can hold without rounding, so the page saves the file as text and never reads it as numbers. A program that reads the
file should use 64-bit integers (or text) for every id.

## When the editor refuses

The export stops, says why, and writes nothing, when:

- the save has no base backup, or there is no backup with that number;
- the backup has no actors linked to it, or names an actor that is not in the save (a damaged backup);
- a storage of the base holds a Solido blueprint (its rows are not carried yet), or an inventory of the base belongs to an item or a vehicle;
- a number in the base is not finite, or a JSON value in it is not valid;
- the base is over the limits of a file: 250,000 rows or 16 MiB (a base of about 1,400 pieces is under 2 MiB);
- the save was edited while the file was being made (try again).

## What has been checked

The export is tested on a made-up save of the same shape, including every refusal above, and on a copy of a save that holds a real in-game pick-up (1 backup, 44 actors, 174 pieces, 43
placeables, 53 stored items, 589 rows; the file passes its own check and its counts equal the card's). That check cannot say whether the **game** would accept the rows again; that is what
importing will need (see `docs/research/2026-10-28-base-export-import-from-console.md`, section 7).

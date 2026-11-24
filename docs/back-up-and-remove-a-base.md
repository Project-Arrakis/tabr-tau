# Back up and remove a base

Player > Bases lists every base of the save. A base that is still standing has a **Back up and remove** button (#1127, bases only). It does what the game's own base backup tool does: the
base leaves the world and becomes a **base backup**, which you can place again with the game's Base Reconstruction Tool. It is not a delete: nothing is destroyed, and the base shows up
under **Base backups** below the list.

Vehicles are not part of this. A hard delete of a base does not exist in the editor.

## Use it

1. Close the game, or leave it at the main menu. While a game process exists the edit is queued and written when the game has closed, like every other edit.
2. Press **Back up and remove** on the base. The editor works out what the game's pick-up would do and asks first. The question names the base, says how many building pieces and
   placeables leave the world, that the base becomes one base backup, that the land claim and the base marker go, and that what is stored in the base stays where it is. Nothing has been
   written at this point.
3. Press **Back up and remove**. It is one edit, written after the usual backup copy of the save (or queued while the game runs). The editor sends the fingerprint of what the question
   showed, and refuses when the base changed in between.
4. Start the game, load the save, and place the backup with the Base Reconstruction Tool.

## Which bases

Only your own bases. The totem must hold the owner rank (1) for your character and for nobody else, and it must be standing. A base you are only a member of, a base of another player, a
base nobody owns, and a base the game already picked up are refused, and nothing is changed. A base in the Deep Desert is refused too (#1177): the game does not back up bases there, so
the button is greyed out on those rows and says why. One base at a time.

## What it writes

The rows are those of a real pick-up in the game, seen in two before and after pairs of saves (a base of 73 pieces with a totem and a placeable, and a base with a chest of four stacks):

- the totem, and every placeable of the base that has buildable support, go to state 5 in place;
- the building actor that holds the pieces is made again under a **new number**, with the same class, map, dimension and position, state 5, empty properties and no entity; the old one and
  its entity are deleted. The pieces and the buildings row move to the new actor;
- one base backup (your character as the player, named as the totem's permission record was) and one link for each actor: the totem, the placeables and the new building actor;
- the totem's permission record and rank rows are deleted, so the claim is gone;
- the base's own map marker (a HomeBase marker you discovered, at the totem's position) is removed from your markers. A marker that is not clearly this base's is left, and the question
  says so.

## What it leaves alone

- **Stored items and the inventories of the base.** In the game, a chest with four stacks kept its stacks exactly where they were when the game picked the base up, and so does this edit. The
  items stay with the base backup.
- The totem row, the land claim's cells, the totem's fuel times, the markers table itself.
- Placeables with no buildable support (a lamp, say). The game's pick-up does not take them, so they stay standing; the question counts them.
- Every other base.

## When it refuses

Another player's base, a member base, a base nobody owns, a base already picked up, an actor of the base that is not in the normal state, a building actor that also holds the pieces of
another base, a totem with no entity, a base with no building pieces, a save with no character. Each says why and changes nothing.

## What has been checked

Tests on made-up saves of both layouts of the markers, the routes, the page in a browser engine, and the edit applied to copies of two real saves from before the game's own pick-up and
compared with the saves the game wrote after it: every table of the base matches (the new building actor gets the next free number, as the game's did). **Not checked:** placing the
backup again in the game after an edit made here, and a base with other kinds of contents (a Solido, a crafting queue, a generator) beyond the chest.

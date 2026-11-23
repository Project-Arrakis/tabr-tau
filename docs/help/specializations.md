# Specializations

The Specialization view shows the five specializations of the game: Combat, Crafting, Exploration, Gathering and Sabotage. Each has XP, a level up to 100 and keystones.

## What you see

- The overview counts the specializations completed (the specializations whose XP has reached the cap, level 100) and the keystones bought.
- Choose a specialization to open its dossier: level, XP against the cap, keystones and the Houses of that specialization. A specialization with the cap and every keystone is marked **Complete**.
- The level scale marks (LV 10, 25 and so on) are reference marks along the 100 levels. They are not unlock levels.

## XP and keystones are separate

Level 100 does not give keystones, and all keystones do not give level 100.

## The operations

- **Add XP** adds XP to one specialization. Type a whole number from 1 to 44,182. Zero, a negative number and anything that is not a whole number are refused, and nothing is sent or written: XP is only added here, and to take XP back you use **Reset XP** or **Reset XP of All Known Specializations**. The total stays at most 44,182 (the XP cap), so a number larger than the room left only fills the specialization to the cap. The editor shows the XP before and after. On the page this sentence is the hover or focus tip of the XP box (and of the small **i** mark beside it, which a touch screen can tap).
- **Grant LV 100** sets a specialization to level 100 and its XP cap. **Reset XP** (Reset XP for this specialization) sets its XP and level back to 0. Neither changes a keystone.
- **Grant All Keystones** and **Reset All Keystones** change every specialization, not the selected one.
- **Reset XP of All Known Specializations** resets the XP and level of every specialization the editor can match, after you type a confirmation. Keystones are not touched. A specialization the editor cannot match yet is left alone. The page shows this as the hover or focus tip of the button and of its **i** mark.

Every change asks first, is written after a backup, and says whether it was saved or is queued until the game closes.

## Associated Houses

The Houses listed under a specialization come from a public community tracker, not from the game or the save: treat them as a reference. Choosing a House opens it on the Landsraad board for this visit only: the Landsraad tab itself is not changed. Nothing is written.

## A specialization the save does not tie to its XP

A dash (—) where a figure belongs (XP, keystones) means the save does not hold it. Hover the dash for the reason.

If the save holds XP the editor cannot match to a specialization, the view says so and the XP buttons of that specialization are off. The keystone buttons still work, because keystones do not depend on the match.

## Technical notes

For support: the save keeps specialization XP by track number, not by name. The editor knows the numbers the game uses (Crafting 1, Gathering 2, Exploration 3, Combat 4, Sabotage 5). A track with another number is the one the page calls a specialization it cannot match; the page has an Advanced card, Advanced: specialization numbers, under the view for it.

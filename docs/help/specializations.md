# Specializations

The Specialization view shows the five paths of the game: Combat, Crafting, Exploration, Gathering and Sabotage. Each has XP, a level up to 100 and keystones.

## What you see

- The overview counts the specializations completed (the paths whose XP has reached the cap, level 100) and the keystones bought.
- Choose a path to open its dossier: level, XP against the cap, keystones and the Houses of that path.
- The level scale marks (LV 10, 25 and so on) are reference marks along the 100 levels. They are not unlock levels.

## XP and keystones are separate

Level 100 does not give keystones, and all keystones do not give level 100.

## The operations

- **Apply XP** adds XP to one track. Type a whole number from 1 to 44,182. Zero, a negative number and anything that is not a whole number are refused, and nothing is sent or written: XP is only added here, and to take XP back you use **Reset Track XP** or **Reset XP of All Known Tracks**. The total stays at most 44,182 (the XP cap), so a number larger than the room left only fills the track to the cap. The editor shows the XP before and after. On the page this sentence is the hover or focus tip of the XP box (and of the small **i** mark beside it, which a touch screen can tap).
- **Grant LV 100** sets a track to level 100 and its XP cap. **Reset Track XP** sets its XP and level back to 0. Neither changes a keystone.
- **Grant All Keystones** and **Reset All Keystones** change every track, not the selected one.
- **Reset XP of All Known Tracks** resets the XP and level of every track whose number is known, after you type a confirmation. Keystones are not touched. A row of the save whose track is not known yet is left alone. The page shows this as the hover or focus tip of the button and of its **i** mark.

Every change asks first, is written after a backup, and says whether it was saved or is queued until the game closes.

## Associated Houses

The Houses listed under a path come from a public community tracker, not from the game or the save: treat them as a reference. Choosing a House opens it on the Landsraad board. Nothing is written.

## A track the save does not tie to a path

If the save holds XP in a track number that no path owns, the view says so and the XP buttons of that path are off. The keystone buttons still work, because keystones need no track numbers.

# Term and Decrees

The Term and Decrees section sits under the plaque. It shows the current term and holds the editor's own controls for the active decree and the reigning faction. The plaque above only shows what these hold.

On a phone (a window 640 px wide or less) the section comes after the board, so the Houses come first, and everything under its heading is one closed list: its line says the term, the reigning faction and the active decree, and Show opens it. A wide window shows it open, as always.

## The facts of the term

- **Term**: the number of the current term is in the header of the board (and in the line of the closed list on a phone).
- **Ends in**: the hours and minutes the term has left, for example 18 h 37 m. It counts down only while the game is running and is the value as of the last save. Under an hour it shows minutes, under a minute it says less than a minute, and once the end has passed it says ended (the game starts the next term when it runs). If the save does not give the clock it shows a dash.
- **Reigning faction**: the faction whose decree is in force.
- **Active decree**: the decree in force this term.
- **Next term's decree**: the decree chosen during this term, which takes effect in the next term. It says none yet until the vote has happened.

## Set the active decree and the reigning faction

- Apply sets both together, for the current term only. None for the decree clears the decree.
- A decree is only in force because a faction won the term, so the faction cannot be left empty. Atreides and Harkonnen can both be chosen, and any faction the box offers can be. The box starts on your own faction and asks you to choose when you are in neither and the term has no winner.
- Apply needs no choice in two cases: the save holds a faction the list does not offer and you leave the box alone (the decree changes and that faction stays), and none for the decree on a term with no faction (the decree is cleared and no faction is written).
- Nothing changes until you press Apply, Make active or Set flag. A choice you have not applied is marked as not applied yet.

## The decree pool

Each row is one decree. A decree that is in this term carries a badge for what it is; a decree that is not in this term carries none:

- **Active**: the term's active decree.
- **Next term's decree**: the decree chosen for the next term.
- **In rotation**: a decree in the save's decree rotation.
- **Advanced: show pool flags** (the button above the list) adds a pool flag and a **Set flag** button to every row. The pool flag is a switch the save keeps on each decree, shown as stored. Nobody has confirmed what the game does with it, so it does not say whether a decree is in effect. Set flag changes the flag of that one row and never makes a decree active. The flags stay hidden until you ask for them.
- **Make active** does the same as the Active decree box and Apply for that row, for the faction in the Reigning faction box. A decree belongs to a faction, not to a House.

The pool keeps the order of the save. Names are the game's own names, with the effect line the game gives. The game's own id for a decree is not shown.

The special vendor decree is five separate rows. The four Exclusive Import Rights decrees (Vehicle, Weapon, Armor and Tool) are in the list. The fifth, the plain one, does nothing in the game, so the editor does not show it anywhere: not in the pool, not in a closed list and not in the Active decree box. It stays in the save as it is and the editor never changes or deletes it. If a term should hold it as its active or next decree, the page says Another decree and nothing more, and Apply writes it back unchanged.

## Earlier terms

Read-only. Each earlier term with a row of yours opens to show its Houses, your contribution, your tiers, the House status and the last payout. Terms with no row of yours are not listed. The rows say what the contribution was; they do not say which terms paid out. The last payout is the time of the last reward row written for the House, kept as one time per House: a reward collected in a later term moves it out of the earlier term, so "not in this term" does not always mean nothing was paid then.

## What the save does not hold

- Active Landsraad missions are not recorded in this local save.
- A ranking of contributors is not recorded: the save holds the amount each contributor gave to each House, and the banners list the biggest by name, in the order the editor works out from those amounts; the banners do not show the amounts.
- No calendar date for the start or the end of a term. The term is on the game's play clock, which stops while the game is closed.

## Technical notes

For support: the term and the decrees are read from the `landsraad_decree_term` and `landsraad_decrees` tables of the save. The pool flag is the `disabled` column of `landsraad_decrees`.

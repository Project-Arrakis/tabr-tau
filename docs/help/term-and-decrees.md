# Term and Decrees

The Term and Decrees section sits under the plaque. It shows the current term and holds the editor's own controls for the active decree and the reigning faction. The plaque above only shows what these hold.

## The facts of the term

- **Term**: the number of the current term.
- **Ends in**: the hours and minutes the term has left, for example 18 h 37 m. It counts down only while the game is running and is the value as of the last save. Under an hour it shows minutes, under a minute it says less than a minute, and once the end has passed it says ended (the game starts the next term when it runs). If the save does not give the clock it says not known.
- **Reigning faction**: the faction whose decree is in force.
- **Active decree**: the decree in force this term.
- **Next term's decree**: the decree chosen during this term, which takes effect in the next term. It says none yet until the vote has happened.

## Set the active decree and the reigning faction

- Apply sets both together, for the current term only. None for the decree clears the decree.
- A decree is only in force because a faction won the term, so the faction cannot be left empty. Atreides and Harkonnen can both be chosen. The box starts on your own faction and asks you to choose when you are in neither and the term has no winner.
- Apply needs no choice in two cases: the save holds a faction the list does not offer and you leave the box alone (the decree changes and that faction stays), and none for the decree on a term with no faction (the decree is cleared and no faction is written).
- Nothing changes until you press Apply, Make active or Set flag. A choice you have not applied is marked as not applied yet.

## The decree pool

Each row is one decree, with what it is in this term:

- **Active**: the term's active decree.
- **Next term's decree**: the decree chosen for the next term.
- **In rotation**: a decree in the save's decree rotation.
- **Pool flag**: the save's own disabled column, shown as stored. It does not say whether a decree is in effect. Set flag changes the flag of that one row and never makes a decree active.
- **Make active** does the same as the Active decree box and Apply for that row, for the faction in the Reigning faction box. A decree belongs to a faction, not to a House.

The pool is in id order. Names are the game's own names, with the effect line the game gives. The small line under a name is the id the save holds, kept for support.

The special vendor decree is five separate rows. The four Exclusive Import Rights decrees (Vehicle, Weapon, Armor and Tool) are in the list. The plain SpecialVendorActive row has no name in the game's decree table, no category and an effect that is not verified, so it is kept in a closed list under the pool with the same controls, in case the game uses it.

## Earlier terms

Read-only. Each earlier term with a row of yours opens to show its Houses, your contribution, your tiers, the House status and the last payout. Terms with no row of yours are not listed. The rows say what the contribution was; they do not say which terms paid out.

## What the save does not hold

- Active Landsraad missions are not recorded in this local save.
- A ranking of contributors is not recorded: the save holds the amount each contributor gave to each House, and the banners show the biggest, worked out by the editor from those amounts.
- No calendar date for the start or the end of a term. The term is on the game's play clock, which stops while the game is closed.

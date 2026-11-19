# The Landsraad board

The board shows the 25 Houses of the current term, five rows of five, numbered 01 to 25 from the top left. The numbers are the game's own positions, read from the save's board_index. If the save does not give a clean 5 by 5 board, the Houses are shown as a list and nothing can be written from the board.

## What a card shows

- The number, the specialization emblem and the House crest.
- The House name and its specialization.
- Who won the favor: Atreides, Harkonnen, or not won.
- Five tier boxes and a line such as 3/5 EARNED, with the number collected and the number not collected. A House whose favor a faction has won shows the same line as any other House: if you have no contribution to it, it reads 0/5 EARNED.

Click a card to open the House in the panel beside the board (House Intelligence). Click it again to close it. In that panel, TOP CONTRIBUTORS lists the names of the biggest contributors to the House, the biggest first and numbered, with no amounts; a contributor the save gives no name for is left out. The reward ledger there has two figures: Tiers earned (for example 3 / 5) and Collected (for example 1 / 3, the tiers collected out of the tiers you have earned); hover them to see how many are waiting at the House representative.

## Favor won, earned, collected

These are three separate states:

- **Favor won**: a faction has won the favor of the House. It belongs to a faction, not to you.
- **Earned**: you reached a reward tier by your own contribution. Earned is not collected.
- **Collected**: you took the reward. A reward that is earned and not yet taken is waiting at the House representative.

A tier with no delivery record is counted as neither collected nor not collected.

## The marks on a tier

| Mark | Meaning |
|---|---|
| ✓ | Collected |
| ◆ | Not collected (waiting at the House representative) |
| ○ | Not reached |
| ? | No delivery record in the save |

## Colours and outlines

- Atreides and Harkonnen colours mark a House whose favor that faction won. A House whose favor is not won has no faction colour.
- A gold outline marks the Houses that make up a win: the five Houses of a line of five held by one faction, or the 13 or more Houses of a faction.
- Every colour has a word beside it, so nothing depends on colour alone.

## Filters and the list

- Show, House and Specialization filters narrow what the board shows. A House the filters leave out is dimmed, and while a filter is on a line says how many Houses are shown (it is not there when nothing is filtered).
- Jump to House opens one House and clears the filters that would hide it.
- A button that does nothing right now (there is nothing to change, or another faction is chosen) has a dashed edge and quiet text instead of a faded look; pressing it says why.
- The Grid and List buttons switch between the board and a sortable list of the same Houses.
- A House's place opens the Saved map at that House. The line above the map names it, and **Back to House** returns to it on the Landsraad tab, in the view you are in (the board, or the Classic list with only that House shown).

## The plaque and the victory panel

The plaque above the board shows the active decree of the term and the biggest contributors under each faction, by name only: the banner shows no amounts. The contributors are worked out by the editor from the amounts in the save: the save holds the amounts, not a ranking. A contributor is shown by the name of its guild, written from the identifier the save gives (the save holds no display name, so the editor adds the capital letters). A contributor the save gives no name or guild for is shown as "Unnamed contributor"; its number in the save is in the mouse-over text.

The victory panel says whether a faction has won, by 13 Houses or by five in a line. Its headline carries the House count once (the plaque above shows the same figures), and a line under it says how many lines of five each faction can still make. The game works the winner out from the Houses, and so does the editor.

## What the save does not hold

- Active Landsraad missions are not recorded in a single-player save.
- A ranking of the contributors is not recorded. The save holds the amount each contributor gave to each House.
- Some Houses have no place on the Saved map. The editor does not guess a place.
- A reward item that the editor's item list has no name for is called "Unlisted item". The game's internal item name is never shown as a name. In the list, a House with no place shows a dash, and the Tiers column shows for example "2 of 5" (earned tiers of five).
- Five Houses stand in the two social hubs, which the Saved map shows with their own pictures: Mutelli, Varota and Wallach in Arrakeen, Alexin and Spinette in Harko Village. Their place opens the hub on the Saved map, centred on the House. The Harko Village frame was fitted by eye on the picture, so a pin there is right to about a room.

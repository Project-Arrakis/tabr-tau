# The Landsraad board

The board shows the 25 Houses of the current term, five rows of five, numbered 01 to 25 from the top left. The numbers are the game's own positions of the Houses on its board. If the save does not give a clean 5 by 5 board, the Houses are shown as a list, the header says the save gives no 5 by 5 board, and nothing can be written from the board.

## What a card shows

- The number, the specialization emblem and the House crest.
- The House name and its specialization.
- Which faction the House is pledged to: "Pledged to Atreides", "Pledged to Harkonnen", "Undecided" (no faction has the favor of the House yet) or "Wavering" (no faction has it yet, and both factions have progress on it). The screen-reader text says the same words. A House the save cannot settle says "Favor unreadable".
- Five tier boxes and a line such as 3/5 EARNED, with the number collected and the number not collected once a tier is earned. A House that is pledged to a faction shows the same line as any other House: if you have no contribution to it, it reads 0/5 EARNED.

Click a card to open the House in the panel beside the board (House Intelligence). Click it again to close it. The panel does not list contributors: the biggest ones of each faction are on the plaque above the board. The reward ledger there has two figures: Tiers earned (for example 3 / 5) and Collected (for example 1 / 3, the tiers collected out of the tiers you have earned); hover them to see how many are waiting at the House representative.

## Pledged, earned, collected

These are three separate states:

- **Pledged**: the House is pledged to a faction, which has gained its favor. It belongs to a faction, not to you. A House no faction has the favor of yet is Undecided, or Wavering when both factions have progress on it.
- **Earned**: you reached a reward tier by your own contribution. Earned is not collected.
- **Collected**: you took the reward. A reward that is earned and not yet taken is waiting at the House representative.

A tier with no delivery record is counted as neither collected nor not collected.

## The marks on a tier

| Mark | Meaning |
|---|---|
| ✓ | Collected |
| ○ | Earned and not collected (waiting at the House representative): an empty ring |
| the tier number | Not reached |
| ? | No delivery record in the save |

In the House Intelligence panel the ledger rows show the same three states without words: a collected tier has a teal edge on the left and a teal circle with a check at the right, an earned and not collected tier has only an empty ring, and a tier not reached is dimmed with no mark. A screen reader still hears the state of each row.

## Colours and outlines

- Atreides and Harkonnen colours mark a House that is pledged to that faction. A House that is Undecided or Wavering has no faction colour.
- A gold outline marks the Houses that make up a win: the five Houses of a line of five held by one faction, or the 13 or more Houses of a faction.
- Every colour has a word beside it, so nothing depends on colour alone.

## Filters and the list

- Show, House and Specialization filters narrow what the board shows. A House the filters leave out is dimmed, and while a filter is on a line says how many Houses are shown (it is not there when nothing is filtered).
- Jump to House opens one House and clears the filters that would hide it.
- A button that does nothing right now (there is nothing to change, or another faction is chosen) looks the same as every other control that is off, on every page of the editor: quiet text, a quiet edge, no fill and a not-allowed pointer (never a dashed edge and never faded); pressing it says why.
- The Grid and List buttons switch between the board and a sortable list of the same Houses.
- In the Classic list, click a row to open it.
- A House's place opens the Saved map at that House. A yellow ring on the map marks its place, and the line above the map names it, and **Back to House** returns to it on the Landsraad tab, in the view you are in (the board, or the Classic list with only that House shown).
- Hover a House's place, or put the keyboard focus on it, for a quick peek: a zoomed window of the real map picture (Hagga Basin, the Deep Desert, Arrakeen or Harko Village) with the same ring that the Saved map draws, the whole map small in a corner with the window marked on it, and the Region and From you lines. Escape puts the peek away. A place outside the frame of its picture, or on a map the editor has no picture of, shows a plain outline and says why. Hover or focus a House's name for its card, which starts with the large House symbol, the name and the region. The editor has no picture of the House swatches, so there is no swatch hover.

## The plaque and the victory panel

The plaque above the board shows the active decree of the term and the biggest contributors under each faction, by name only, with the rank numbers 1. to 5. before the names (the biggest first; the number is the place in the order, up to five names): the banner shows no amounts. The contributors are worked out by the editor from the amounts in the save: the save holds the amounts, not a ranking. A contributor is shown by the name of its guild, written from the identifier the save gives (the save holds no display name, so the editor adds the capital letters). A contributor the save gives no name or guild for is shown as "Unnamed contributor"; its number in the save is in the mouse-over text.

The victory panel says whether a faction has secured the Sysselraad, by 13 Houses or by five in a line. Its headline carries the House count once (the plaque above shows the same figures), and a line under it says how many lines of five each faction can still make. The game works the winner out from the Houses, and so does the editor.

## What the save does not hold

- A dash (—) where a figure belongs means the save does not hold it (no holder recorded, no contributors). Hover the dash for the reason.
- Active Landsraad missions are not recorded in a single-player save.
- A ranking of the contributors is not recorded. The save holds the amount each contributor gave to each House.
- Some Houses have no place on the Saved map. The editor does not guess a place.
- A reward item that the editor's item list has no name for is called "Unlisted item". The game's internal item name is never shown as a name. In the list, a House with no place shows a dash, and the Tiers column shows for example "2 of 5" (earned tiers of five).
- Five Houses stand in the two social hubs, which the Saved map shows with their own pictures: Mutelli, Varota and Wallach in Arrakeen, Alexin and Spinette in Harko Village. Their place opens the hub on the Saved map, centred on the House. The Harko Village frame was fitted by eye on the picture, so a pin there is right to about a room.

## Where the figures come from

Everything on the Landsraad pages is read from your save. A few figures are worked out by the editor from what the save holds: which reward tiers you have earned (your contribution reaches the amount of the tier), which faction a House is pledged to and who has secured the Sysselraad, and the order of the biggest contributors (the save holds the amounts, not a ranking). Where the save holds nothing, the page shows 0 or a dash and never guesses. The pages carry no label that says so, since it is true of every figure.

## Technical notes

For support: the board positions are read from the `board_index` column of `landsraad_tasks` in the save. The Houses, their tiers and the contributions come from `landsraad_tasks`, `landsraad_task_rewards` and the contribution rows of the same term; the term and the decrees from `landsraad_decree_term` and `landsraad_decrees`.

# Landsraad tab: mock-ups (refs #599, #440)

Three alternative mock-ups of the Landsraad tab. They are static pages (inline CSS, made-up sample data, no script, no external request, they work offline from a file). They are not the editor, they read no save, and nothing here changes the editor's code. Open any of them in a browser from disk; they work at phone width and on a desktop.

The colour tokens are the editor's own names and values (`design/assets/tokens-*.css`). The controls at the top of each page (not part of the design) switch the look (Fremen, Awakening, Sietch), Dark or Dayside, and show or hide the data-source marks.

Filters, sorting, term tabs and the location pop-up are done with CSS only (radio buttons, `:has()`, `:hover` and `:focus-within`), so a mouse hover, a keyboard focus and a phone tap all open the pop-up.

## Files

| File | What it shows | Covers |
|---|---|---|
| `a-board.html` | **The board, made readable (chosen; revision 2).** One list of the 25 houses, every row collapsed to one line: house, location, state (not started / in progress / finished), 5-segment tier meter and the next tier. A row opens (click, or Space on it) to the five tiers with deliveries, the house status with both factions' progress, and the location (schematic map, region, place). Expand all / Collapse all (HTML and CSS only: checkboxes and a form reset). The three counts at the top are links: one sets the Show filter to that state and jumps to the list, the tile shows "showing", and a link under the tiles goes back to all houses. A line under the tiles says the three numbers count the 25 houses by your contribution in the current term (finished = top tier 5 of 5, in progress = some contribution below the top tier, not started = nothing recorded) and add up to 25; the houses-taken line below counts by faction and is separate. Also: sort (board order, by region, unfinished first), open or taken house filter, a hover pop-up on the place for mouse users, and an "Earlier terms" card. | (a), (b), (c), (d) |
| `b-by-region.html` | **By place.** The same states grouped by region (Arrakeen, Harko Village, Hagga Basin, Deep Desert) with a schematic map per region: one dot per house (shape and glyph show the state), hover or focus on a dot or a list entry shows the house and where it is; a strip of "left per place"; an "only what is left" switch. Meant for planning a trip. | (a), (c); (b) as the tier meter per house |
| `c-tiers-and-terms.html` | **The tier ladder, with a term per tab.** Each house as a row of its five tiers (reached or not, recorded as delivered where the save shows it), the total and the house status; a list of the terms the save holds and a tab per term, so the history has the same layout as the current term; the location pop-up on each house. | (b), (d), (a), (c) |

All three also redraw the term card and the decree pool with the decrees shown by a readable name and the game's internal id only as a small second line (see the last section).

Points of the issues:

* (a) State per house at a glance: open, in progress, completed, who is winning (#599). A: all houses, B: per place, C: per tier.
* (b) Completed tracking: the character's contribution against the five tiers and reaching the top tier (#440). A: meter and "next tier"; C: the ladder.
* (c) Mouse-over / pop-up with the house's location (#599). A and C: pop-up with a schematic map and the spot; B: the map itself with dots.
* (d) Per-term history (#440). A: a closed card with a list per term; C: a tab per term.

## Reading the pages

Every page has three kinds of mark, with a tooltip each, and ends with a table "What comes from the save, and what is still a question":

* **save**: a table and column the save holds (named in the table at the bottom).
* **derived**: worked out by the editor from fields the save holds (tiers reached, finished, counts, who is ahead).
* **open?**: not known to be in the save, or an open question. Nothing marked this way is claimed as a fact.

The sample data is made up, and so are all the numbers on the pages (the counts on the tiles, the tiers, the contributions and the terms): the houses' names are the ones the board shows, but the regions, places, positions, contributions, terms and play-clock numbers are placeholders (for the deep desert the "cell" uses the editor's lettered grid A1 to I9). The five tiers 1,200 to 6,000 and the goal 4,000 are sample values.

## What is a save field and what is not

* From the save (read-only, as answered on #440): house status (`landsraad_tasks.completed`, `winning_faction_id`), the two factions' progress (`landsraad_task_faction_contributions`), the character's contribution (`landsraad_task_player_contributions`), the tier thresholds (`landsraad_task_rewards`), recorded deliveries (`landsraad_house_rewards`), the terms and their play-clock window (`landsraad_decree_term`).
* Derived: tiers reached, finished (top tier, 5 of 5), the three counts, which faction is ahead.
* **Open: the location of a house.** The schema read has no column for a place. The location pop-up (A, C) and the maps (B) need a table of the 25 houses kept in the editor (region, place, position on a map), written once and checked against the game. It is also not known whether a house keeps its place from one term to the next, or whether the same 25 houses appear in every term. Distance from the character is an optional idea that waits on the same table.
* **Open: history depth.** Per the plan on #440 an older save held earlier terms' rows and a new character's save held one term; a save may hold only the current term, in which case the page says so. A term's rows say what the contribution was; they do not say which terms paid out (only the time of the last payout per house is kept). Terms show the game's play-clock window, not a date.
* **Open: names.** See the next section.

## Names the game shows (refs #608)

The decree pool, the active and elected decree and every other Landsraad label are drawn with the name a player knows ("Repair and Refining Times"), with the internal id (`RepairAndRefiningTimes`) as a small monospace second line. The names in these pages are readable forms of the ids, not the game's own wording: the exact in-game wording is an open question until the catalog data or the game's strings are checked. A name that cannot be confirmed would say so on the page. The existing controls of the term card and decree pool (Apply, pool flag buttons, the Active decree drop-down) and of the task board (Fill, Complete, Reopen, the choose column) are not redrawn in these mock-ups and stay as they are.

## Reference

The reference linked on #599 is a community Landsraad tracker page; only the idea was taken (a location shown on hover, a state per house, grouping by place). No code, art or text of it is used here.

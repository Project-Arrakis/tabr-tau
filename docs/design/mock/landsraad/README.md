# Landsraad tab: mock-ups (refs #599, #440)

Three alternative mock-ups of the Landsraad tab. They are static pages (inline CSS, made-up sample data, no external request, they work offline from a file; mock-up A also has a short inline script for sorting, finding, jumping and the action buttons, see below). They are not the editor, they read no save, and nothing here changes the editor's code. Open any of them in a browser from disk; they work at phone width and on a desktop.

The colour tokens are the editor's own names and values (`design/assets/tokens-*.css`). The controls at the top of each page (not part of the design) switch the look (Fremen, Awakening, Sietch), Dark or Dayside, and show or hide the data-source marks.

Filters, sorting, term tabs and the location pop-up are done with CSS only (radio buttons, `:has()`, `:hover` and `:focus-within`), so a mouse hover, a keyboard focus and a phone tap all open the pop-up.

## Files

| File | What it shows | Covers |
|---|---|---|
| `a-board.html` | **The board, made readable (chosen; revision 4, with actions).** One list of the 25 houses, **always alphabetical by house name** (also the order inside a grouping and the tie-break of every other sort); there is no board-order option. Every row is collapsed to one line: house, location, state, 5-segment tier meter, next tier and the house's faction status; a row opens (click, or Space on it) to the five tiers with deliveries, both factions' progress and the location (schematic map, region, place). Expand all / Collapse all. **Every column heading sorts**: House, Location, State, Tiers reached, Next tier / missing amount, Faction status; click once for ascending, again for descending, the active heading shows an arrow and `aria-sort` is set; on a phone the headings become a "Sort by" row. "Group by region" is a switch that keeps the chosen sort inside each region. **Find a house**: a box that filters as you type by house name or region (case-insensitive, anywhere in the name), a Clear button, and a "No house matches" line. **Jump to house**: a dropdown of all 25 (alphabetical) that shows only that house, opened, with a "Show all houses" link back; while it is on, the search box and the filters are ignored. The search, the state filter (tiles and Show buttons), the house filter, the sort and the grouping all work together. The three counts at the top are links that set the Show filter and jump to the list; a line under them says they count the 25 houses by your contribution (finished = top tier 5 of 5, in progress = some contribution below the top tier, not started = nothing recorded) and add up to 25; the houses-taken line counts by faction and is separate. Also a hover pop-up on the place and an "Earlier terms" card. Revision 4 adds actions on the board (next section). | (a), (b), (c), (d) |
| `b-by-region.html` | **By place.** The same states grouped by region (Arrakeen, Harko Village, Hagga Basin, Deep Desert) with a schematic map per region: one dot per house (shape and glyph show the state), hover or focus on a dot or a list entry shows the house and where it is; a strip of "left per place"; an "only what is left" switch. Meant for planning a trip. | (a), (c); (b) as the tier meter per house |
| `c-tiers-and-terms.html` | **The tier ladder, with a term per tab.** Each house as a row of its five tiers (reached or not, recorded as delivered where the save shows it), the total and the house status; a list of the terms the save holds and a tab per term, so the history has the same layout as the current term; the location pop-up on each house. | (b), (d), (a), (c) |

All three also redraw the term card and the decree pool with the decrees shown by a readable name and the game's internal id only as a small second line (see the last section).

Points of the issues:

* (a) State per house at a glance: open, in progress, completed, who is winning (#599). A: all houses, B: per place, C: per tier.
* (b) Completed tracking: the character's contribution against the five tiers and reaching the top tier (#440). A: meter and "next tier"; C: the ladder.
* (c) Mouse-over / pop-up with the house's location (#599). A and C: pop-up with a schematic map and the spot; B: the map itself with dots.
* (d) Per-term history (#440). A: a closed card with a list per term; C: a tab per term.

## Actions on the board (mock-up A, refs #599, #440, #609)

Everything on this page is sample data, and **the buttons change only what the page shows**: nothing is saved, and a reload brings the sample back. The page keeps a small made-up state per house, so the tiles, the stacked bar, the counts, the facts and the sort order follow every click. In the real editor these controls would write to the save, which is why each carries a **writes** mark.

* **Reward tiers are buttons.** Click tier N and the character's contribution is raised to that tier's threshold, so tiers 1 to N are reached (cumulative; the tooltip and the legend line say so). Click the highest reached tier again to remove it: the contribution drops to the tier below, or to none for tier 1. A change shows as pending (outlined, "will be reached" or "will be removed") with an Undo until it is written.
* **Per-house buttons in the opened row:** Fill faction goal, Complete, Reopen. They act for the faction chosen in the drop-down above the list. It starts on the character's own faction; for another faction the action records that faction's win and progress and skips the character's own contribution row. The checkbox "Also set my faction's progress on the house to its goal" adds the faction progress to the action.
* **Quick buttons on each collapsed row** (after Faction status, before the chevron): Fill, and Complete (Reopen once the house is taken). They use the same faction drop-down and the same pending and Undo pattern, are disabled with a tooltip when nothing would change, are at least 44 px high on a phone (where they wrap under the row) and are not part of column sorting.
* **Bulk bar above the list**, like the editor's: Choose all houses, Clear the choice, Fill chosen, Complete chosen, Reopen chosen, and a line "Chosen: N houses ... Fill chosen would raise your contribution on N houses ..." that updates as the Choose box on each row is ticked. Undo all reverts every pending change.
* **Alphabetical order, sortable headings, Jump to house, Find and the collapsed start are unchanged.**

What each action would write in the real editor (the same rules as today's Fill, Complete and Reopen):

| Action | Rows it touches |
|---|---|
| Tier click (give or remove) | the character's row in `landsraad_task_player_contributions` only |
| Fill | the character's contribution raised to the top tier (never lowered), the faction's progress in `landsraad_task_faction_contributions` set to the goal, and the house marked taken by that faction in `landsraad_tasks` (`completed`, `winning_faction_id`, completion time); for another faction the character's own row is skipped |
| Complete | `landsraad_tasks` (`completed`, `winning_faction_id`, completion time) and the faction's progress set to the goal |
| Reopen | clears `completed` and the winner in `landsraad_tasks`; progress and contribution are left as they are |

None of them writes `landsraad_house_rewards`. **Open question (tau #609):** the rewards the game's representative hands out to the inventory depend on the save's delivery rows, which are not understood yet. Until that is proven, a tier click changes "reached" only, and the page says so in its legend.

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

## Notes for the build

* **House symbol badge: 32 px.** The mock draws a neutral placeholder (the first two letters of the house name in a coloured square, 32 px) where the real symbol of each house would go; the build should show the symbols at 32 px, with the name beside them so the row never depends on the picture alone.
* The list is alphabetical by house name by default, with ties in every other sort broken the same way.

## Names the game shows (refs #608)

The decree pool, the active and elected decree and every other Landsraad label are drawn with the name a player knows ("Repair and Refining Times"), with the internal id (`RepairAndRefiningTimes`) as a small monospace second line. The names in these pages are readable forms of the ids, not the game's own wording: the exact in-game wording is an open question until the catalog data or the game's strings are checked. A name that cannot be confirmed would say so on the page. The existing controls of the term card and decree pool (Apply, pool flag buttons, the Active decree drop-down) and of the task board (Fill, Complete, Reopen, the choose column) are not redrawn in these mock-ups and stay as they are.

## Reference

The reference linked on #599 is a community Landsraad tracker page; only the idea was taken (a location shown on hover, a state per house, grouping by place). No code, art or text of it is used here.

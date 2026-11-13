# Landsraad tab: mock-ups (refs #599, #440)

Three alternative mock-ups of the Landsraad tab. They are static pages (inline CSS, made-up sample data, no external request, they work offline from a file; mock-up A also has a short inline script for sorting, finding, jumping, the action buttons and the term and decree controls, see below). They are not the editor, they read no save, and nothing here changes the editor's code. Open any of them in a browser from disk; they work at phone width and on a desktop.

The colour tokens are the editor's own names and values (`design/assets/tokens-*.css`). The controls at the top of each page (not part of the design) switch the look (Fremen, Awakening, Sietch), Dark or Dayside, and show or hide the data-source marks.

Filters, sorting, term tabs and the location pop-up are done with CSS only (radio buttons, `:has()` and `:hover`); the pop-up is the quick peek on a mouse hover, and a click, an Enter key or a phone tap on the place opens the full map (see "Location: hover and click").

## Files

| File | What it shows | Covers |
|---|---|---|
| `a-board.html` | **The board, made readable (chosen; revision 5, with actions and the term and decree controls).** One list of the 25 houses, **always alphabetical by house name** (also the order inside a grouping and the tie-break of every other sort); there is no board-order option. Every row is collapsed to one line: house, location, state, 5-segment tier meter, next tier and the house's faction status; a row opens (click, or Space on it) to the five tiers with deliveries, both factions' progress and the location (schematic map, region, place). Expand all / Collapse all. **Every column heading sorts**: House, Location, State, Tiers reached, Next tier / missing amount, Faction status; click once for ascending, again for descending, the active heading shows an arrow and `aria-sort` is set; on a phone the headings become a "Sort by" row. "Group by region" is a switch that keeps the chosen sort inside each region. There is no "Find a house" box (dropped: the region grouping and Jump to house cover it); a "No house matches" line appears if the filters leave nothing. **Jump to house**: a dropdown of all 25 (alphabetical) that shows only that house, opened, with a "Show all houses" link back; while it is on, the filters are ignored. The state filter (tiles and Show buttons), the house filter, the sort and the grouping all work together. The three counts at the top are links that set the Show filter and jump to the list; a line under them says they count the 25 houses by your contribution (finished = top tier 5 of 5, in progress = some contribution below the top tier, not started = nothing recorded) and add up to 25; the houses-taken line counts by faction and is separate. Also a hover pop-up on the place (a quick peek), a click on the place that opens the full map, and an "Earlier terms" card. Revision 4 adds actions on the board; revision 5 puts the term and decree controls back, gives the Choose box its own column, makes the bar under the three cards clickable, and moves the explanations into closed "About" disclosures (next sections). | (a), (b), (c), (d) |
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
* **Per-house buttons in the opened row:** Fill faction goal, Complete, Reopen. They act for the faction chosen in the drop-down above the list. It starts on the character's own faction; for another faction the action records that faction's win and progress and skips the character's own contribution row.
* **Quick buttons on each collapsed row** (after Faction status, before the chevron): Fill, and Complete (Reopen once the house is taken). They use the same faction drop-down and the same pending and Undo pattern, are disabled with a tooltip when nothing would change, are at least 44 px high on a phone (where they wrap under the row) and are not part of column sorting.
* **Bulk bar above the list**, like the editor's: Choose all houses, Clear the choice, Fill chosen, Complete chosen, Reopen chosen, and a line "Chosen: N houses ... Fill chosen would raise your contribution on N houses ..." that updates as the Choose box on each row is ticked. Undo all reverts every pending change. There is no "also set the faction progress" checkbox: **Fill chosen** changes your contribution and tiers only, **Complete chosen** takes the house for the chosen faction (its progress goes to the goal, and the winner and the completion are set), and **Fill faction goal** on one house does both in one click.
* **The five small tier boxes in each collapsed row are buttons too**, with exactly the behaviour of the boxes in the opened row (the same code and the same states): click tier N to raise the contribution to tier N (cumulative), click the highest reached tier again to remove it, pending cells are outlined (dashed, with the same "will be reached" or "will be removed" in the opened row) and the house shows Undo. The two sets are kept in step on every change. Each box is a real button with an `aria-label` such as "Tier 3, 3,600, House Alexin, not reached", a tooltip that starts "Tier 3 threshold: 3,600", a pointer cursor, a hover and a focus ring, and a hit area at least 24 px high (44 px on a phone) that does not add to the row height. A click on a box does not open or close the row.
* **Next tier text** reads "next 3,600 · 1,100 more" and never breaks inside: if it does not fit beside the amount it moves down as one piece. The text comes from one function in the script, so the opened and collapsed rows and the sort agree.
* **Row widths.** The row grid was rebalanced and checked at 390 (phone), 901, 1024, 1280, 1440 and 1920 px: no location is cut off or ellipsised (a long place wraps onto a second line at most), the next-tier text stays whole, and there is no horizontal scroll. Between a phone and about 1200 px wide the row is two lines tall (faction status sits under the tiers) instead of squeezing the tier column.
* **Location: hover and click.** Hovering the place (mouse only) shows the quick peek, a small card with the schematic map and the spot, and says so ("Quick peek. Click the place for the full map."). Clicking or tapping the place (a real button, reachable with the keyboard) opens the full map in a dialog: the schematic map of the region with the spot marked, the Region, Place and From you lines of the opened row, and a link-style button **Open in Saved map**. In the real editor that button would switch to the Saved map tab with the place selected; in this mock it only says so and nothing navigates. Esc, the Close button and a click outside close the dialog, and focus returns to the place. A phone has no hover, so the tap is the only way there. A click on the place does not open or close the row.
* **House name card (experimental, may be dropped).** A small card on the house name: hover or keyboard focus on a desktop, a tap on a phone (the name is a button; a click or tap pins the card open on any device, and Esc, the card's close button or a click outside closes it). It shows four things and no internal ids, board slot or full tier list: **Next reward** (the item and amount at the next tier, for example "3,600: 50 × Plasteel Composite Gun Parts", or "top tier reached"), **Waiting to collect** (rewards pending at the house representative, for example "10,000 Solari + 1 schematic", or "nothing waiting"), **Faction race** (each faction's progress against the goal and who finished it) and **Place** (region and place name; the map picture stays on the click of the place). The reward items, the amounts and the waiting line are sample text: the Next reward line depends on the reward table of each tier and the Waiting line depends on the pending reward rows of tau #609, neither of which is understood yet. The house symbol tile and the name are one button, so hovering, tapping or focusing either one opens the same card (the location keeps its own separate map, and nothing else in the row has a hover card). The card does not open or close the row, the Choose box and the other buttons are unaffected, and it is checked at 390 and 1280 px.
* **The Choose box has its own narrow column** (a "Choose" heading, a hairline between it and the house symbol badge), centred on the row; on a phone it is 24 px and sits level with the badge.
* **Control bar.** Jump to house (label and drop-down) is the last item of the bar with Show, House, Group and Rows, and the "N of 25 houses shown" count sits at the right edge of the same bar in small dim text; on a phone the bar wraps, with Jump on its own line and the count under it. The "Showing only House ..." note appears under the bar only while Jump is used.
* **Alphabetical order, sortable headings, Jump to house and the collapsed start are unchanged; the Find box was dropped.**
* **Short text, explanations tucked away.** Each note above the board is one line; the longer explanations (tiers, Fill and Complete, the counts, the board, earlier terms) sit in closed "About ..." disclosures with a plain triangle.

What each action would write in the real editor (the same rules as today's Fill, Complete and Reopen):

| Action | Rows it touches |
|---|---|
| Tier click (give or remove) | the character's row in `landsraad_task_player_contributions` only |
| Fill | the character's contribution raised to the top tier (never lowered), the faction's progress in `landsraad_task_faction_contributions` set to the goal, and the house marked taken by that faction in `landsraad_tasks` (`completed`, `winning_faction_id`, completion time); for another faction the character's own row is skipped |
| Complete | `landsraad_tasks` (`completed`, `winning_faction_id`, completion time) and the faction's progress set to the goal |
| Reopen | clears `completed` and the winner in `landsraad_tasks`; progress and contribution are left as they are |

None of them writes `landsraad_house_rewards`. **Open question (tau #609):** the rewards the game's representative hands out to the inventory depend on the save's delivery rows, which are not understood yet. Until that is proven, a tier click changes "reached" only, and the page says so in its legend.

## At a glance bar (mock-up A)

The bar under the three cards is proportional (green = Finished, blue = In progress, dashed = Not started). Each segment is a link that sets the Show filter exactly like the card of the same name, shows a hover and keyboard-focus tooltip ("12 in progress"), and uses the cards' labels. "Open" is used only for a house no faction has taken (the House filter and the houses-taken line).

## Term and decrees (mock-up A)

A closed panel above the board (its heading line shows the term, the reigning faction and the active decree). Open, it shows the term (number, start and end on the play clock, reigning faction, active decree, elected decree), the **Active decree** and **Reigning faction** drop-downs with **Apply**, and the **Decree pool** table (id, decree, an "In this term" badge, the pool flag and a **Set flag to 0/1** button per row). The faction drop-down starts on the character's own faction and any faction can be chosen. Decrees are shown by the name a player knows with the internal id as a small second line. These are the controls of the editor's present Landsraad tab, drawn again; here they change only what the page shows, and each carries a **writes** mark.

| Control | Rows it would write (as the present tab does) |
|---|---|
| Apply (Active decree and Reigning faction) | `landsraad_decree_term`: `active_decree_id` and `reigning_faction_id` of the current (latest) term row, both from the two drop-downs in one edit; "none" clears the field to NULL. `elected_decree_id` and `end_time` are not touched by this control. |
| Set flag to 0 / 1 | `landsraad_decrees`: the `disabled` column of that one decree row (picked by its id) only. It does not make the decree active; the Active decree does that. |

The reigning faction is one field of the term row, so "for which faction" is the Reigning faction drop-down: the decree is set on the term, and the faction chosen is recorded as the one reigning in it.

## Reading the pages

Every page has three kinds of mark, with a tooltip each, and ends with a table "What comes from the save, and what is still a question":

* **save**: a table and column the save holds (named in the table at the bottom).
* **derived**: worked out by the editor from fields the save holds (tiers reached, finished, counts, who is ahead).
* **open?**: not known to be in the save, or an open question. Nothing marked this way is claimed as a fact.

The sample data is made up, and so are all the numbers on the pages (the counts on the tiles, the tiers, the contributions and the terms): the houses' names are the ones the board shows, but the regions, places, positions, contributions, terms and play-clock numbers are placeholders (for the deep desert the "cell" uses the editor's lettered grid A1 to I9). The five tiers 1,200 to 6,000 and the goal 4,000 are sample values.

## What is a save field and what is not

* From the save (read-only, as answered on #440): house status (`landsraad_tasks.completed`, `winning_faction_id`), the two factions' progress (`landsraad_task_faction_contributions`), the character's contribution (`landsraad_task_player_contributions`), the tier thresholds (`landsraad_task_rewards`), recorded deliveries (`landsraad_house_rewards`), the terms and their play-clock window (`landsraad_decree_term`).
* Derived: tiers reached, finished (top tier, 5 of 5), the three counts, which faction is ahead.
* **The location of a house is not an open question.** The save has no column for a place, but every house representative is drawn on its map today (17 on the Hagga Basin, the other 8 on the Deep Desert, Harko Village and Arrakeen), so the names and the positions are known. The source is a fixed table of the house representatives' positions per map (map, region, place name, coordinates), one row per house, built from the game's map data in the same way as the editor's existing place tables (`internal/ops/place-names.json`, made by `tools/placenamesgen`; the region names are in `internal/ops/regions.go`). The location pop-up, the full-map dialog (A, C) and the maps (B) read that table, so the location carries no **open?** mark. The place names in these mock-ups are sample text; the real table gives the true region and place. The distance from the character is worked out from that table and the character's position.
* **Open: history depth.** Per the plan on #440 an older save held earlier terms' rows and a new character's save held one term; a save may hold only the current term, in which case the page says so. A term's rows say what the contribution was; they do not say which terms paid out (only the time of the last payout per house is kept). Terms show the game's play-clock window, not a date.
* **Open: names.** See the next section.

## Notes for the build

* **House symbol badge: 32 px.** The mock draws a neutral placeholder (the first two letters of the house name in a coloured square, 32 px) where the real symbol of each house would go; the build should show the symbols at 32 px, with the name beside them so the row never depends on the picture alone.
* The list is alphabetical by house name by default, with ties in every other sort broken the same way.

## Names the game shows (refs #608)

The decree pool, the active and elected decree and every other Landsraad label are drawn with the name a player knows ("Repair and Refining Times"), with the internal id (`RepairAndRefiningTimes`) as a small monospace second line. The names in these pages are readable forms of the ids, not the game's own wording: the exact in-game wording is an open question until the catalog data or the game's strings are checked. A name that cannot be confirmed would say so on the page. Mock-up A draws the controls of the term card, the decree pool and the task board; B and C show the term card and pool as display only. #608 stays open: the names are readable forms of the ids, not confirmed wording.

## Reference

The reference linked on #599 is a community Landsraad tracker page; only the idea was taken (a location shown on hover, a state per house, grouping by place). No code, art or text of it is used here.

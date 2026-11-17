# Command Center M3, fidelity pass: matrix against the approved Landsraad Board tab

The project owner found the built board did not match the approved design: no specialization badge, no decree preview, no term row, no section above the squares, and a reward indicator, font and look that were wrong. This note lists every visible element and behaviour of the approved Landsraad Board tab, where the board stood before this pass, where it stands now, and why anything is still different. Nothing was dropped without a row.

Method: the approved prototype and the editor page were drawn side by side in a headless browser at 1280 px (dark and light) and at 390 px, with the same Houses, owners, tiers and decree in both, and the computed styles of the card parts were compared value by value. The prototype is a layout reference only: its numbers are demonstration data, ours come from the save. No write behaviour, route or test of the write path changed.

## Totals

| | matches | differs | missing | not built (reason given) | rows |
|---|---|---|---|---|---|
| Before this pass | 4 | 35 | 17 | 7 | 63 |
| After this pass | 44 | 12 | 0 | 7 | 63 |

"Not built" rows are prototype-only chrome, or belong to a later milestone. "Differs" rows after this pass each carry the reason: the save lacks the data, an honest-wording rule applies, a later milestone owns it, or a decision is open.

## Matrix

### Reward indicator and look of the House card (the three points of the project owner)

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| F1 | Five tier boxes numbered 1 to 5 under the name | differs | matches | The old tile drew five pips. The card now draws the five boxes of the approved design, 18 x 20 px, from the same tiers the Inspector reads. |
| F2 | Reached and collected: green box with a tick | missing | matches | Green box (#68c6a8, edge #86f6c9) with the tick, exactly the prototype's colours. The tick is dark on the green, as in the prototype (the brief said "white tick"; the prototype was copied). |
| F3 | Reached and waiting at the representative: gold box with a diamond | missing | matches | Gold box (#f5c878) with the diamond. |
| F4 | Reached with no delivery record: rust box with a question mark | missing | matches | Rust box (#9b563b) with "?". The model keeps `no_row` apart from `delivered`; a test proves a no-record tier is never drawn or counted as collected. |
| F5 | Not reached: dark box with its number | missing | matches | Dark box with the tier number. |
| F6 | Line under the boxes: "N/5 REACHED", then the collected count and the waiting count | missing | matches | "5/5 REACHED ✓ 3 ◆ 2". A third figure "? n" appears only when a tier has no delivery record (the prototype has the same). The screen-reader name of the card says the same in words. |
| F7 | Fonts: Georgia for names and headings, Segoe UI for labels, Consolas for numbers | differs | matches | The prototype names system stacks only, so nothing is fetched and the page works offline. They are now tokens (`--lcc-serif`, `--lcc-sans`, `--lcc-mono`) with the prototype's exact lists. The old board used the app display face. A test pins the three stacks and bans `@font-face`, `@import` and `url(`. |
| F8 | Card look: background, border, spacing, type scale, hover, selected, open, dim | differs | matches | Measured against the prototype in a headless browser (computed styles): same size 184 px, padding 9/6/8, gap 1, clip corners, 3 px inset owner bar, name 9 px, path 9 px, owner 8 px, boxes 18 x 20. The prototype's own gradients (dark and light) are tokens used in every look. Side-by-side crops of 8 card states at 2x, dark and light, are in the image list. |

### Top of the page (plaque)

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| P1 | Prototype header: brand, three tabs, theme, motion and icon switches | not built | not built | Prototype chrome. The app has its own header, tabs, theme and motion settings. The other two tabs belong to M5 and M6. |
| P2 | Strip "approved decree and plaque, live mock data" with "View original illustration" | not built | not built | Prototype-only chrome that explains the mock. |
| P3 | Plaque: ACTIVE DECREE, decree name and effect, HELD BY House | differs | matches | The decree name was shown in capitals by an app heading rule; it now keeps its case as in the prototype. |
| P4 | Plaque frame: three-line gold glow frame | differs | differs | Not changed in this pass. The approved plaque is one picture with the win-state glow baked in; this plaque is built from the approved pieces and has no win glow, because a count is never a win here (no vote-start snapshot in the save). Open decision for the project owner, noted on the pull request. |
| P5 | Top contributors lists (names) | differs | differs | The save has no contributor names: the cloth says "Not in the save". |
| P6 | Gold orb on the 13th seat and the words "13 HOUSES WIN" | differs | differs | Honest data rule: the ring marks seat 13 and the band reads "VOTING SESSION · SEAT 13 OF 25". No win is claimed. |
| P7 | Standing line "ATREIDES LEADS 12 TO 4 · 9 HOUSES OPEN" | differs | differs | Ours adds "VOTE NOT CONFIRMED" (a lead is not a win). |
| P8 | House counts, faction names, medallions, 25 seat tokens | matches | matches | Unchanged. |

### Decree preview

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| D1 | Bar with the label DECREE PREVIEW, a chooser and the note "Sample decree only, not written to a save" | missing | matches | Added. The save's own decree is the first entry ("Active decree: ..."). |
| D2 | The chooser lists the decrees the app knows | missing | matches | Fed by the decrees of the answer (`host.decrees`). Display only. |
| D3 | Choosing one changes the plaque only | missing | matches | Read-only: no `data-lb`, no request. A test counts the posts (none). |

### Term row

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| T1 | TERM STATUS tile: CONTESTED, "Vote outcome is not yet established" | missing | matches | Real adapter value: CONTESTED while the board is a board, UNAVAILABLE when it is not. |
| T2 | OPEN HOUSES tile | differs | matches | Count of open Houses from the same pass as the plaque; unreadable owners are named, never counted. |
| T3 | POTENTIAL LINES tile with a count (4) | missing | differs | Shows "—" and "Counted when the twelve lines are checked (milestone M4)". The count needs the line check of M4; no number is invented. |
| T4 | SPECIALIZATION CAP tile: LV 100, 44,182 XP | missing | matches | A game rule, tagged RULE: it is not read from the save. |

### Board panel: head, filters, legend

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| H1 | Eyebrow THE GREAT HOUSES, heading "The Landsraad", subline | differs | matches | Subline reads "Term 7 · Fixed 5 × 5 board · game positions read from board_index" (the term is real). |
| H2 | Specialization filter chooser | differs | matches | All specializations, then the five paths; same dimming as before. |
| H3 | Grid / List switch | matches | matches | Kept. |
| H4 | Clear highlight button | missing | matches | Clears the looked-at House. Line highlights come with M4 (the button says so). |
| H5 | SHOW chips: All, Not started, In progress, Top tier | differs | differs | Ours has one more chip, "What is left", kept from the classic list. Not removed: it is a working filter. |
| H6 | HOUSE chips: Any, Open, Claimed | differs | matches | The third chip was called "Taken"; it is "Claimed" as in the prototype. |
| H7 | JUMP TO HOUSE chooser on the same row | differs | differs | It wraps to a second row at 1280 px because the board column inside the app is narrower than the prototype's; "25 of 25 Houses shown" is an extra status line for screen readers. |
| H8 | Legend: Atreides claimed, Harkonnen claimed, Open, Victory line | missing | matches | Added. "Owner unreadable" appears only when a House needs it. |

### Board

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| B1 | Five rows of five cards in board order, gap 10 | differs | matches | Layout follows the board's own width (container query) instead of the window. |
| B2 | Cut-corner card, border by owner (teal, red, dashed when open) | differs | matches | Same clip and colours. |
| B3 | Position number 01 to 25, top left | matches | matches | Kept. |
| B4 | Specialization badge, upper right of every card | missing | matches | The diamond badge of the House's mapped path (26 px picture in a 30 px diamond). New art: five 84 x 84 files, game artwork under the art notice, listed in the manifest (see the notes below). No badge when the save gives no path, and the card says "Specialization not in the save" in words. |
| B5 | House crest in gold | differs | matches | The embedded crests are white; a CSS filter tints them to the prototype's gold (dark and light). The crest is drawn at 48 px because the prototype's own crests have padding. |
| B6 | HOUSE NAME in Georgia capitals | differs | matches | Text only (names from a save are untrusted). |
| B7 | Specialization word under the name | differs | matches | Path word in capitals; italic "Specialization not in the save" when unknown. |
| B8 | Owner line: ● ATREIDES, ● HARKONNEN, ○ OPEN | differs | matches | Plus "? Owner unreadable" when the save does not agree with itself. |
| B9 | Dominion card: glow and "DOMINION" badge | differs | differs | The faction holding more than half of the Houses gets the existing outline and the words "VICTORY LINE"; no card says dominion or win, because the count is not a vote result. |
| B10 | Selected card: violet double outline | differs | matches | Same violet outline. |
| B11 | Dimmed card when a filter leaves it out | matches | matches | Kept (opacity .38, dashed). |
| B12 | Hover and keyboard focus | differs | matches | Brightness on hover; a 3 px focus ring on every control; arrow keys move between cards (kept). |
| B13 | Choose box for the actions on several Houses | not built | not built | Not in the prototype: ours alone (the bulk actions of the classic list). A small box at the top of the card, shown fainter until hovered, focused or checked. |
| B14 | Narrow layout (390 px): five columns, smaller boxes | differs | differs | Same five columns and the same boxes, but the cards are narrower (about 54 px against 63 px) because the app's own page padding takes width. The name drops to 6 px under 410 px. |
| B15 | List layout: rows with the same card pieces | differs | differs | Ours is the sortable, groupable table (a real table for screen readers). Kept as it was; it is a different design from the prototype's row cards. |
| B16 | Caption under the board | differs | matches | Same sentence; ours adds one line of totals and, when the page can write, says the numbered boxes are the same controls as the tiers in the panel. |
| B17 | Scenario buttons: Reference 12 / 4, Contested, five in a line, majority | not built | not built | Prototype-only demo of the plaque in other states. The live plaque shows the save. |

### Right panels

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| S1 | Imperial Victory Analysis: tag, emblem, SYSSELRAAD CONTESTED, sentence, counts | missing | matches | Added. Wording never claims a win. |
| S2 | House Intelligence header and source tag | differs | matches | Tag reads "FROM THE SAVE" (the prototype's says "illustrative save data"). |
| S3 | Identity: crest, HOUSE NAME, "Crafting · associated specialization" | differs | matches | Ours adds the No. chip. |
| S4 | Band: CLAIMED · ATREIDES and the place | differs | matches | The place is the real one, or "not mapped yet". |
| S5 | Faction house control: two bars with values | differs | matches | Same bars; ours adds the goal line and "Top contributors: Not in the save". |
| S6 | Your contribution: large figure and gold bar | differs | matches | Same. |
| S7 | Personal reward ledger: header and four counts | differs | matches | Same four figures; ours adds a plain sentence under them. |
| S8 | Ledger table: tier and threshold, reward, state | differs | matches | Reward names are the save's own (raw names appear in the synthetic answer used for the pictures). |
| S9 | Legend under the ledger | missing | matches | Added. |
| S10 | Note that claimed, reached and collected are three states | missing | matches | Added, with the "no delivery record is never collected" rule. |
| S11 | Operations / mock editor | differs | differs | Ours is the real "Actions" box (Fill, Fill faction goal, Complete, Reopen, change the faction) and the tier rows above it are the tier controls: all go through the same single write path with the stale guard and the backup. The prototype's "Fill 5 tiers" and "Set tier / Apply tier" are covered by Fill and the tier rows. Its in-memory demo notes are not copied. |
| S12 | Top contributors and missions per House | differs | differs | Not in the save: shown as "Not in the save". |

### Other

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| O1 | Specializations tab | not built | not built | Milestone M5. |
| O2 | Sysselraad Strategy tab and the line check | not built | not built | Milestone M6 (and M4 for the lines). |
| O3 | Footer line of the prototype | not built | not built | Prototype-only chrome. |

## Notes

* **New art (B4).** `internal/web/static/lcc/art/specs/` holds five 84 x 84 PNG files, the specialization badges of the approved design, kept at their exact pixels. They are small crops of the in-game Specializations screen that came with the design, so they are game artwork: the art notice and the manifest say so, and they are the first thing to replace if authorized texture exports are obtained. They are served from the embedded folder by the same read-only list as the other art (one folder added to the list; no new route).
* **Card tier boxes are controls (F1 to F5).** As in the approved design, a tier box on a card is a button when the page can write. It carries the same `data-lb="tier"`, the same House id and tier number as the tier rows of the House Intelligence panel, so the one existing click handler, stale guard, write queue and backup serve it. Nothing was added to that handler. A card of a page with no write host draws the boxes as pictures.
* **Surfaces (F8).** The board panels and the House cards use the approved design's own surface colours (dark and light) in every look, as the plaque already does, so the board matches the design whatever palette the app wears. Text, gold text and focus colours still follow the app.
* **Fonts (F7).** No network font exists in the approved design and none is added; the editor's content security policy allows fonts from itself only, and none is embedded.
* **Wording.** "Taken" became "Claimed" (filter and Inspector line). The plaque and the House cards never say win, won or dominion for a count.


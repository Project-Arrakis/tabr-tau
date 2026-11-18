# Command Center M3, fidelity pass: matrix against the approved Landsraad Board tab

The project owner found the built board did not match the approved design: no specialization badge, no decree preview, no term row, no section above the squares, and a reward indicator, font and look that were wrong. This note lists every visible element and behaviour of the approved Landsraad Board tab, where the board stood before this pass, where it stands now, and why anything is still different. Nothing was dropped without a row.

Method: the approved prototype and the editor page were drawn side by side in a headless browser at 1280 px (dark and light) and at 390 px, with the same Houses, owners, tiers and decree in both, and the computed styles of the card parts were compared value by value. The prototype is a layout reference only: its numbers are demonstration data, ours come from the save. No write behaviour, route or test of the write path changed.

## Totals

| | matches | differs | missing | not built (reason given) | rows |
|---|---|---|---|---|---|
| Before this pass | 4 | 35 | 17 | 7 | 63 |
| After this pass (fidelity pass) | 45 | 11 | 0 | 7 | 63 |
| After display remediation A | 36 | 20 | 0 | 7 | 63 |

The row "After display remediation A" counts the eleven rows that the display audit reclassified: P8 and T1 are fixed and stay "matches"; F1, F6, F8, P3, B1, B6, S7, S8 and S11 become "differs (approved)", because the project owner approved the departure from the picture (a text floor, 24 px tier boxes, fewer columns on a narrow board, a compact stacked plaque); the rows of the earlier "differs" group are unchanged. "Matches" lost nine rows and the two fixes keep theirs, so the counts are 45 - 9 = 36 matches and 11 + 9 = 20 differs.

"Not built" rows are prototype-only chrome, or belong to a later milestone. "Differs" rows after this pass each carry the reason: the save lacks the data, an honest-wording rule applies, a later milestone owns it, or a decision is open.

## Matrix

### Reward indicator and look of the House card (the three points of the project owner)

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| F1 | Five tier boxes numbered 1 to 5 under the name | differs | differs (approved) | The old tile drew five pips. The card now draws the five boxes of the approved design, 18 x 20 px, from the same tiers the Inspector reads. **Display remediation A:** the boxes are 24 x 24 px with 2 px between them wherever the window is 900 px or narrower or the pointer is coarse, and in a board panel narrower than 760 px; the 18 x 20 px boxes of the design stay in a wide panel. Each box has a hit area of 26 x 36 px that does not change its look. The project owner approved the larger size (text floor, reflow instead of shrinking). |
| F2 | Reached and collected: green box with a tick | missing | matches | Green box (#68c6a8, edge #86f6c9) with the tick, exactly the prototype's colours. The tick is dark on the green, as in the prototype (the brief said "white tick"; the prototype was copied). |
| F3 | Reached and waiting at the representative: gold box with a diamond | missing | matches | Gold box (#f5c878) with the diamond. |
| F4 | Reached with no delivery record: rust box with a question mark | missing | matches | Rust box (#9b563b) with "?". The model keeps `no_row` apart from `delivered`; a test proves a no-record tier is never drawn or counted as collected. |
| F5 | Not reached: dark box with its number | missing | matches | Dark box with the tier number. |
| F6 | Line under the boxes: "N/5 REACHED", then the collected count and the waiting count | missing | differs (approved) | "5/5 REACHED ✓ 3 ◆ 2". A third figure "? n" appears only when a tier has no delivery record (the prototype has the same). The screen-reader name of the card says the same in words. **Display remediation A:** the line and the counts are 12 px (the design has 8 px), kept to two lines so the cards of a row line up; the bullet and "0/5 REACHED" no longer wrap at 320 px. A card with no tiers in the save says "NO TIERS IN THE SAVE" in the style of the other owner lines. |
| F7 | Fonts: Georgia for names and headings, Segoe UI for labels, Consolas for numbers | differs | matches | The prototype names system stacks only, so nothing is fetched and the page works offline. They are now tokens (`--lcc-serif`, `--lcc-sans`, `--lcc-mono`) with the prototype's exact lists. The old board used the app display face. A test pins the three stacks and bans `@font-face`, `@import` and `url(`. |
| F8 | Card look: background, border, spacing, type scale, hover, selected, open, dim | differs | differs (approved) | Measured against the prototype in a headless browser (computed styles): same size 184 px, padding 9/6/8, gap 1, clip corners, 3 px inset owner bar, name 9 px, path 9 px, owner 8 px, boxes 18 x 20. The prototype's own gradients (dark and light) are tokens used in every look. Side-by-side crops of 8 card states at 2x, dark and light, are in the image list. **Display remediation A:** a text floor was approved: names and key labels 12 px or more, all other text 11 px or more, and the card grows (or the board reflows to fewer columns) instead of the text shrinking. The container steps that shrank card text are gone. The surfaces, clip, bar, gradients and hover are as before. |

### Top of the page (plaque)

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| P1 | Prototype header: brand, three tabs, theme, motion and icon switches | not built | not built | Prototype chrome. The app has its own header, tabs, theme and motion settings. The other two tabs belong to M5 and M6. |
| P2 | Strip "approved decree and plaque, live mock data" with "View original illustration" | not built | not built | Prototype-only chrome that explains the mock. |
| P3 | Plaque: ACTIVE DECREE, decree name and effect, HELD BY House | differs | differs (approved) | The decree name was shown in capitals by an app heading rule; it now keeps its case as in the prototype. **Display remediation A:** the wide plaque (940 px of container and more) is as before. Below it the plaque is the compact stacked variant: from 600 px the two standards sit beside the track, below 600 px they stack, and no text is smaller than 11 px. The stacked plaque is a decision of the project owner, not part of the approved picture. |
| P4 | Plaque frame: three-line gold glow frame | differs | differs | Not changed in this pass. The approved plaque is one picture with the win-state glow baked in; this plaque is built from the approved pieces and has no win glow, because a count is never a win here (no vote-start snapshot in the save). Open decision for the project owner, noted on the pull request. |
| P5 | Top contributors lists (names) | differs | differs | The save has no contributor names: the cloth says "Not in the save". |
| P6 | Gold orb on the 13th seat and the words "13 HOUSES WIN" | differs | differs | Honest data rule: the ring marks seat 13 and the band reads "VOTING SESSION · SEAT 13 OF 25". No win is claimed. |
| P7 | Standing line "ATREIDES LEADS 12 TO 4 · 9 HOUSES OPEN" | differs | differs | Ours adds "VOTE NOT CONFIRMED" (a lead is not a win). |
| P8 | House counts, faction names, medallions, 25 seat tokens | matches | matches | Unchanged. **Display remediation A:** at 320 px the nameplate wraps inside its box and nothing is cut. |

### Decree preview

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| D1 | Bar with the label DECREE PREVIEW, a chooser and the note "Sample decree only, not written to a save" | missing | matches | Added. The save's own decree is the first entry, by its name alone as in the approved design (a tooltip on the entry says it is the active decree of the save). |
| D2 | The chooser lists the decrees the app knows | missing | matches | Fed by the decrees of the answer (`host.decrees`). Display only. |
| D3 | Choosing one changes the plaque only | missing | matches | Read-only: no `data-lb`, no request. A test counts the posts (none). |

### Term row

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| T1 | TERM STATUS tile: CONTESTED, "Vote outcome is not yet established" | missing | matches | Real adapter value: CONTESTED while the board is a board, UNAVAILABLE when it is not. **Display remediation A:** CONTESTED is no longer cut at 320 px. |
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
| B1 | Five rows of five cards in board order, gap 10 | differs | differs (approved) | Layout follows the board's own width (container query) instead of the window. **Display remediation A:** five rows of five stay down to a board panel of 696 px (the 24 px boxes need it), then three columns, then two columns at 480 px and below; the cards never shrink their text. The Command Center view lifts the page cap (1800 px) and the board sits beside the Inspector from 1080 px of the Landsraad card. |
| B2 | Cut-corner card, border by owner (teal, red, dashed when open) | differs | matches | Same clip and colours. |
| B3 | Position number 01 to 25, top left | matches | matches | Kept. |
| B4 | Specialization badge, upper right of every card | missing | matches | The diamond badge of the House's mapped path (26 px picture in a 30 px diamond). New art: five 84 x 84 files, game artwork under the art notice, listed in the manifest (see the notes below). No badge when the save gives no path, and the card says "Specialization not in the save" in words. |
| B5 | House crest in gold | differs | matches | The embedded crests are white; a CSS filter tints them to the prototype's gold (dark and light). The crest is drawn at 48 px because the prototype's own crests have padding. |
| B6 | HOUSE NAME in Georgia capitals | differs | differs (approved) | Text only (names from a save are untrusted). **Display remediation A:** the name is 12 px or more (clamp to 14 px in a wide panel), wraps on two lines and is never cut with an ellipsis. |
| B7 | Specialization word under the name | differs | matches | Path word in capitals; italic "Specialization not in the save" when unknown. |
| B8 | Owner line: ● ATREIDES, ● HARKONNEN, ○ OPEN | differs | matches | Plus "? Owner unreadable" when the save does not agree with itself. |
| B9 | Dominion card: glow and "DOMINION" badge | differs | differs | The faction holding more than half of the Houses gets the existing outline and the words "VICTORY LINE"; no card says dominion or win, because the count is not a vote result. |
| B10 | Selected card: violet double outline | differs | matches | Same violet outline. |
| B11 | Dimmed card when a filter leaves it out | matches | matches | Kept (opacity .38, dashed). |
| B12 | Hover and keyboard focus | differs | matches | Brightness on hover; a 3 px focus ring on every control; arrow keys move between cards (kept). |
| B13 | Choose box for the actions on several Houses | not built | not built | Not in the prototype: ours alone (the bulk actions of the classic list). A small box at the top of the card, shown fainter until hovered, focused or checked. |
| B14 | Narrow layout (390 px): five columns, smaller boxes | differs | differs | Same five columns and the same boxes, but the cards are narrower (about 54 px against 63 px) because the app's own page padding takes width. The name drops to 6 px under 410 px. **Display remediation A:** no longer true: the text never drops under 11 px, and a narrow board has two columns of larger cards (see B1 and N2). |
| B15 | List layout: rows with the same card pieces | differs | differs | Ours is the sortable, groupable table (a real table for screen readers). Kept as it was; it is a different design from the prototype's row cards. |
| B16 | Caption under the board | differs | matches | Same sentence; ours adds one line of totals and, when the page can write, says the numbered boxes are the same controls as the tiers in the panel. |
| B17 | Scenario buttons: Reference 12 / 4, Contested, five in a line, majority | not built | not built | Prototype-only demo of the plaque in other states. The live plaque shows the save. |

### Right panels

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| S1 | Imperial Victory Analysis: tag, emblem, SYSSELRAAD CONTESTED, sentence, counts | missing | matches | Added. Wording never claims a win. |
| S2 | House Intelligence header and source tag | differs | matches | Tag reads "FROM THE SAVE" (the prototype's says "illustrative save data"). |
| S3 | Identity: crest, HOUSE NAME, "Crafting · Associated specialization" | differs | matches | Same line. The House number is no longer a chip on this line; it is the tooltip of the name. |
| S4 | Band: CLAIMED · ATREIDES and the place, right-aligned, with no sentence under it | differs | matches | The place is the real one, or "not mapped yet". The sentence that used to follow ("Claimed by ..." or "Open: ... a lead, not a win") is now the tooltip of the owner word and text for a screen reader. An owner the save cannot settle keeps its visible sentence, because the count leaves that House out. |
| S5 | Faction house control: two bars with values | differs | matches | Same bars and nothing under them. The goal line and "Top contributors / Missions: Not in the save" are the tooltip of the heading and text for a screen reader. |
| S6 | Your contribution: large figure and gold bar | differs | matches | Same. |
| S7 | Personal reward ledger: header and four counts | differs | differs (approved) | Same four figures and nothing under them. The sentence with the waiting and delivery-unknown counts is the tooltip and the screen-reader name of the four counts; a tier with no delivery record still gets its visible warning. The line under the contribution bar is likewise the bar's tooltip and name. **Display remediation A:** the captions are 12 px; the four counts stand in one row from 901 px of the panel, two by two below. |
| S8 | Ledger table: tier and threshold, reward, state | differs | differs (approved) | Reward names are the save's own (raw names appear in the synthetic answer used for the pictures). **Display remediation A:** state words, thresholds and reward lines are 11 px or more; long reward names break inside the line, not past the box. |
| S9 | Legend under the ledger | missing | matches | Added. |
| S10 | Note that claimed, reached and collected are three states | missing | matches | Added after the legend and before the actions block, with the bold "Important:" and the "no delivery record is never collected" rule. The prototype's last sentence says its numbers are demonstration data; ours are read from the save, so it is not copied. |
| S11 | Operations / mock editor | differs | differs (approved) | The layout is the prototype's: the Faction box and "Fill 5 tiers" on the first row, "Fill faction goal", "Complete House", "Reopen" in red, then "Set tier" with "Apply tier" and "Claimed ≠ earned ≠ collected". The title stays "Actions" and the chip says NOT CHECKED IN THE GAME (the prototype's title and demo notes belong to its in-memory demonstration). Nothing about the writing changed: every button is the same control with the same keys, the Faction box drives the board's own faction box (choosing writes nothing), and Apply tier presses the numbered box the choice means (a higher tier raises, one below the reached tier removes the highest, anything else says why nothing happens), so it goes through the same checks, question, queue and backup as a click on that box. The note about the backup stays in the place of the prototype's demo note. **Display remediation A:** labels and tips are 11 px or more and the controls are 36 px high at 768 px and below and on touch; the checkboxes of Choose are 24 px or more. The behaviour of the controls is untouched. |
| S12 | Top contributors and missions per House | differs | differs | Not in the save: said as "Not in the save" in the tooltip of the Faction house control heading and for a screen reader, not as visible lines. |

### Other

| Row | Approved element | Before | Now | What was done, or why not |
|---|---|---|---|---|
| O1 | Specializations tab | not built | not built | Milestone M5. |
| O2 | Sysselraad Strategy tab and the line check | not built | not built | Milestone M6 (and M4 for the lines). |
| O3 | Footer line of the prototype | not built | not built | Prototype-only chrome. |

## Display audit findings N1 to N15 (display remediation A)

These rows are not part of the 63. They come from the display audit of the board at 1600, 1280, 1100, 1024, 853, 768, 640, 390 and 320 px (dark and light), and they say where each stands after display remediation A, a change of styles and tests only. Status: fixed, open, needs approval, step B (a later step that moves markup or touches the page script, which this change does not).

| # | What differed | Status | What was done, or why not |
|---|---|---|---|
| N1 | Text under 11 px (379 of 656 text elements at 1600 and 1280; 6 px at narrow widths) | fixed | The project owner approved a floor: names and key labels 12 px or more, all other text 11 px or more, and reflow instead of shrinking. Measured in a browser at every width and both themes: no text under 11 px (smallest 11.0 px), no name or key label under 12 px. A test reads every rule and fails on a size under the floor. |
| N2 | Cards shrank their text as the panel narrowed | fixed | The container steps that shrank the card text are gone. The board keeps five columns down to a panel of 696 px, then three, then two at 480 px and below; the card grows in height instead. |
| N3 | The page cap and the nested padding left the board narrow (841 px against 1076 px at 1600) | fixed | The 1300 px cap is lifted while the board view is on (1800 px instead), and the padding of the page, the tab, the board and the panel is trimmed on a narrow window. Board panel at 1600: 1082 px. |
| N4 | Victory analysis and House Intelligence took a row each below 1280 | fixed | The House Intelligence panel sits beside the board from 1080 px of the tab (about 1110 px of window), where the board keeps five columns beside the 284 px side column. Below that the two panels stack at full width (see N15 for why they no longer share a row). |
| N5 | A stacked plaque with two large empty banners below 900 px | fixed | The stacked plaque stays (a decision of the project owner) but is compact: from 600 px the two standards sit beside the track, below it they stack; about 310 px tall at 768 px, against 611 to 658 px before. |
| N6 | A blank band under the board beside a taller House Intelligence; the bulk bar far from the cards | fixed in part, step B | The House Intelligence panel is now sticky (it stays in view under the page header, and under the game warning and the launch notice when they show, with its own scroll and no scroll chaining), so at real window heights (1280 x 800, 1366 x 768) no band shows; a full-height capture of the whole page still shows one, because the panel is as tall as the page there. The bulk bar sits under the board in the layout grid. Moving the markup (the bar's place in the page script), keeping the scroll position across a re-draw and moving the Actions block are step B. |
| N7 | Clipping at 320 px (nameplate, contributors, CONTESTED, card names) | fixed | Wrapping inside boxes; the clipped-text check of the measurement finds none at any width. |
| N8 | Ellipsis on names at 768 px | fixed | Names wrap on two lines and are never cut. |
| N9 | Touch targets: 181 of 268 controls under 36 px at 390 px | fixed | Tier boxes are 24 x 24 px with a 26 x 36 px hit area at 900 px and below and on touch; chips and controls are 33 to 36 px; the Choose boxes are 24 px (28 px on touch). |
| N10 | A reward token broken inside the word at 320 px | fixed | The reward line breaks at the box edge (overflow-wrap) and no longer pushes the row. |
| N11 | Toolbar wrapping (four rows at 853 and 768 px) | open | Unchanged by design; the rows are shorter now that the text no longer wraps inside the chips. |
| N12 | The Command Center board is not the default view | needs approval | Not changed: the board stays a view chosen with the switch, and the classic list stays the default. A product decision. |
| N13 | Page height 25 percent above the design at 1600 and 1280, 1.3 to 1.6 times at 390 to 640 px | fixed at wide widths, open on phones | Re-measured: 3145 px at 1600 (before 3441), 3458 at 1280 (before 3522), 4351 at 1024 (before 4347), 4991 at 768 (before 4999). On phones the page is taller than before (8032 px at 390, before 5660; 8595 at 320, before 6679) because two columns of cards at a readable size replace five columns that could not be read. |
| N14 | The container strings are pinned by a test | fixed | `web-tests/lcc-css.test.mjs` and `web-tests/lcc-standards.test.mjs` pin the new strings, and new tests pin the floor, the tier boxes, the guards, the layout and the contrast. No write behaviour, route or test of the write path changed. |
| N15 | Between about 760 and 1100 px of window the victory analysis stood beside the House Intelligence panel with an empty band of about 1100 to 1230 px under it | fixed | Measured before: 1229 px at 853, 1158 at 1024, 1125 at 1100 (the short victory panel on the left, the tall panel on the right). Now the victory analysis is a full-width band and the House Intelligence panel is full width under it (0 px of empty band at 853, 1024, 1100 and 1180). From 860 px of the tab the panel runs its sections in two balanced columns, the head and the banner across both; below that it is one column. At 1080 px of the tab and wider the panel is the sticky column beside the board as before; its column is empty below the panel only while the page is at the top, and the panel follows the scroll. |

Also done in this step: every container query and the board view layout (which needs `:has()`) sit inside `@supports`, so a browser without them shows the plain page; the light-theme borders of the tier boxes pass 3:1 against the card (new tokens); the disabled buttons keep the half-opacity look (a disabled control is exempt from the contrast rule).

## Notes

* **New art (B4).** `internal/web/static/lcc/art/specs/` holds five 84 x 84 PNG files, the specialization badges of the approved design, kept at their exact pixels. They are small crops of the in-game Specializations screen that came with the design, so they are game artwork: the art notice and the manifest say so, and they are the first thing to replace if authorized texture exports are obtained. They are served from the embedded folder by the same read-only list as the other art (one folder added to the list; no new route).
* **Card tier boxes are controls (F1 to F5).** As in the approved design, a tier box on a card is a button when the page can write. It carries the same `data-lb="tier"`, the same House id and tier number as the tier rows of the House Intelligence panel, so the one existing click handler, stale guard, write queue and backup serve it. Nothing was added to that handler. A card of a page with no write host draws the boxes as pictures.
* **Surfaces (F8).** The board panels and the House cards use the approved design's own surface colours (dark and light) in every look, as the plaque already does, so the board matches the design whatever palette the app wears. Text, gold text and focus colours still follow the app.
* **Fonts (F7).** No network font exists in the approved design and none is added; the editor's content security policy allows fonts from itself only, and none is embedded.
* **Wording.** "Taken" became "Claimed" (filter and Inspector line). The plaque and the House cards never say win, won or dominion for a count.


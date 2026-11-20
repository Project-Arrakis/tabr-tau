# Control area mock, round 2 (tau #974)

Design only: no app code is changed, nothing is built until the operator picks one option. Fixture data, nothing is read or written. Round 1 (options A and B, `../control-area/`) was rejected: "neither A or B sticks out as polished, professional and AAA game."

Three new options, one shared kit: `option-c.html` (command plate under the plaque), `option-d.html` (tool rail beside the board), `option-e.html` (lens, drawer and docked plate), plus `kit.html` (the state language on one page).

## Why round 1 reads as a web toolbar (diagnosis)

1. It is a flat row of rectangles: every control is the same square box with the same 1 px border, so nothing has a material of its own (a plate, a tray, a key).
2. The gold is a tint, not a metal: no faceted edge, no bevel, no lit face; the House cards beside it have bevelled corners and the controls above them do not, so the two do not look like one game.
3. Hierarchy is made by fill only (Fill and Complete solid, the rest outline), so there is one rung of importance instead of primary, secondary and tertiary.
4. Groups are separated by whitespace and a text label, the way a form is; there is no frame, ornament or tray that says "this is one instrument".
5. State is drawn five different ways (tint, underline, solid, outline, dashed), so a selected chip, a primary button and a hovered button cannot be told apart by language.
6. Sentences stand where numbers belong ("25 Houses chosen", "starts on your own faction"); a game HUD shows a count and an icon.
7. Faction is a select box and the view switch is two words, where the game's own crests and an icon pair would carry the meaning.
8. There is no lit/unlit moment: the action group looks the same with nothing chosen and with 25 chosen, so the area never reacts like a console.

## The kit (shared by C, D, E)

- **Frames.** Every key is a faceted gold frame with a card-cut corner (the House cards' bevel), made from a gold edge and a face inset by 1.5 px. The element itself is not clipped, so focus rings and glow work.
- **Three materials.** Raised plate (the command plate, holds writes), recessed tray (holds view filters and switches), flat board. Writes never share a material with view switches.
- **Button language.** Primary: lit gold face, dark ink (Fill, Complete). Secondary: dark face, gold edge (Reopen, Filters). Tertiary: text with a hairline (Choose all, Clear). A key in a tray is a quiet key that lights when selected.
- **One state language** (see `kit.html` and `state-language-*.png`): rest = dim gold edge; hover = bright edge and glow; pressed = 1 px down, darker, inset; keyboard focus = 3 px focus ring; selected = lit face with a lamp (bar and diamond under the label) and heavier label, so it is not colour alone; disabled = 45 percent flat, still focusable and clicking it says why in a toast.
- **Numbers instead of sentences.** A medallion shows the chosen count; Fill, Complete and Reopen carry how many Houses they would change as a badge. The hint under the Faction choice is gone.
- **Faction for Fill** is two crests (the game's `art/factions` pictures, drawn as a tinted mask), a radio pair with arrow keys. The question names the faction.
- **View switch** is an icon pair (grid, list) with a tooltip and a lamp.
- **The plate lights up** when Houses are chosen (lit edge, glow, badges appear) and is flat when none are.
- Reduced motion: transitions off. Forced colours: icons and borders switch to system colours, selected is a thick outline plus the lamp. Text floor 12 px for names, 11 px for other text; 36 px targets, 44 px at phone widths.

## The options

**C: command plate under the plaque.** The title plaque, a raised command plate (count medallion, Choose all, Clear, two faction crests, Fill, Complete, Reopen, in zones with ornament dividers) and below it a recessed filter tray (Show, House, Specialization, Jump to House, Group when the list layout is on, count). The question opens inside the plate.

**D: tool rail beside the board.** A 68 px sticky rail left of the 5 x 5: four icon keys (Show, House, Specialization, Jump to House) that open small lists beside the rail, the count, and a vertical command plate. The board gets the whole width of the panel minus the rail. On narrow widths the rail becomes a row of four keys above the board and the plate a sticky foot.

**E: lens, drawer and docked plate.** One lens row: Show tabs, a Filters key with a numeric badge (how many other filters are not at their default) and the count. House, Specialization, Jump to House and Group sit in a drawer under the lens. The command plate is docked at the foot of the board and stays on screen while the board scrolls.

## Element matrix (every existing control stays reachable)

| Control today | C | D | E |
|---|---|---|---|
| Show (5) | tray, tabs | rail key, list | lens, tabs |
| House (3) | tray, tabs | rail key, list | drawer, tabs |
| Specialization select | tray, well | rail key, list | drawer, well |
| Jump to House select | tray, well | rail key, list | drawer, well |
| Group select (list layout only) | tray, well (appears with List) | rail key (appears with List) | drawer, well (appears with List) |
| Grid / List | icon pair, plaque | icon pair, plaque | icon pair, plaque |
| "N of 25 shown" | count in tray | count under rail | count in lens |
| Choose all houses, Clear the choice | plate, tertiary | plate, tertiary | plate, tertiary |
| "Chosen: 25 houses..." sentence | medallion and badges (the sentence is a tooltip) | same | same |
| Faction select + hint | two crests | two crests | two crests |
| Fill, Complete, Reopen chosen | plate, with badges | plate, with badges | plate, with badges |
| The question, Yes, Cancel | inside the plate | pop beside the rail (inside the foot on phones) | inside the plate |
| Disabled reason | toast on click, tooltip | same | same |

## Review (art director and UX)

What was checked headless (Edge over CDP) on every option, state and theme: no horizontal overflow at 1280, 1024, 768, 390 and 320 px, no target under 36 px (44 px at 640 px and below), 0 text contrast failures of 1176 texts (dark and light, 1280 and 390), 30 of 30 behaviour checks (disabled click gives the reason, Choose all lights the plate, the question takes focus and Escape cancels and returns to the button, faction arrows, the question follows the faction, Yes writes nothing, Show arrows, E drawer count, D list opens and closes). Every sheet was looked at; the fixes that came out of looking: the confirm pop in D under the cards, tab hover going fully gold, the phone plate in D not sticking, hidden tooltips making the phone page zoom out, the confirm card in D phone covering the plate controls, light-theme contrast of the lit face, forced-colour icons, and the Fill badge overlapping its icon.

- **Hierarchy.** C and E read in three rungs: the lit plate, then the tray, then the board. D has the clearest board but the rail's four icons are unlabelled at rest (tooltip only), so the first look has more guessing than C or E.
- **Scan path.** C: title, plate (act), tray (look), board; the plate before the tray puts writes ahead of filters, which suits "choose, then do" but not the common "look" use. E: lens (look), board, plate at the foot (act); the closest to how the page is used. D: rail left, board right; the eye goes to the rail, the board is second.
- **Grouping.** Zones in the plate are divided by ornaments, not space. The count is in the tray (C, E) or under the rail (D), never inside the plate.
- **State language.** One. Selected is a lit face plus a lamp plus heavier text; a disabled key is flat and dim; a write with nothing chosen is visibly off.
- **Touch.** 36 px at desktop widths, 44 px at 640 px and below. Rail keys in D are 56 px tall on phones.
- **Focus order and keyboard.** Show, House and layout are radio groups (one tab stop, arrows move and choose). Tab order in C: Grid, Choose all, Clear, faction, Fill, Complete, Reopen, All, Any, Specialization, Jump (11 stops, down from 18). D: Grid, Show, House, Specialization, Jump, then the plate. E: Grid, All, Filters, then the plate; the drawer's keys come right after Filters when it is open. The question takes focus (not Yes), Escape cancels and returns focus to the button.
- **Reduced motion and forced colours.** Checked by render in C, D and E phone: nothing disappears, selection is still readable.
- **Narrow widths.** C stacks the tray rows and wraps the plate to two rows; D turns the rail into a row of four keys and the plate into a sticky foot; E keeps the lens on one line and the drawer opens under it. The docked plate on phones (D and E) is 142 px tall, 18 percent of a 780 px window, tall for a docked bar.

## Recommendation

**E.** The lens keeps what is looked at in one quiet row, the drawer takes the four rarely used filters out of the first screen (the badge says when one is active), and the plate is docked where the thumb and the board are. C is the safe second: nothing hidden, but it is the tallest at rest and still stacks two framed blocks above the board. D is not recommended unless the board gets wider than 810 px.

### The honest weakest point of each

- **C:** the most chrome above the board (the first card starts 293 px down at 1280 against 149 px in E and 91 px in D, and 459 px down at 390 against 229 px in E), so the board gets a little over half of a 780 px phone window.
- **D:** the rail is 727 px tall (775 px with Group), taller than a 760 px window, so below 740 px of window height it is static and scrolls away; unlabelled icons at rest; the question is a pop that covers cards.
- **E:** two of the four filters are one click away behind the Filters key, so a user who always changes House has one extra click each time; the docked plate covers the lowest card row on a short window.

## Decisions for the operator

1. The labels Fill, Complete, Reopen (short) versus today's "Fill chosen" and so on; the count badge says "chosen".
2. Counts on the buttons (21, 14, 16) instead of the sentence; the sentence stays as the tooltip.
3. The phone bar can retire: the docked plate replaces it at every width.
4. Whether Group stays hidden until List is chosen (as today).

## Not verified

- Not seen on a real phone or a touch screen; touch sizes are measured, not felt.
- Not checked with a screen reader; roles and labels are set (radio groups, aria-checked, aria-disabled with a reason), the reading order is checked by tab order only.
- The Complete and Reopen questions are short stand-ins; only Fill's question is the real wording (Houses capitalised).
- Forced colours was rendered in C, D and E at phone width, not in every state.
- Game art: only the two faction crests from `internal/web/static/lcc/art/factions/` are used, embedded as data URIs in `mock.css`; no real card art, so the real board's busier cards might fight the plate more than the empty cards here do.
- The D flyouts and E drawer lists are the real option lists but the Jump list is truncated to eight Houses.
- The round 1 picture of today at 390 px in the light theme is damaged (a tiled capture); the round 2 light 390 sheet therefore shows today for one state only, the dark sheet has all three.

## Open and pictures

Serve the repository folder with any static file server on a loopback address and open `option-c.html`, `option-d.html`, `option-e.html` or `kit.html`. Query: `?state=idle|chosen|confirm`, `?theme=dark|light`, `?bare=1` (hide the mock's own notes), `?drawer=open` (E), `?fly=show|house|spec|jump|group` (D), `?layout=list`.

Flat copy: put the HTML files, `mock.css` and `mock.js` in one folder with a `css/` folder beside them holding `tokens-fremen.css`, `components-fremen.css`, `components.css`, `lcc.css` and `lcc-board.css` (from `design/assets/` and `internal/web/static/lcc/`). Each stylesheet is linked twice (repository path, then `css/`); the one that does not exist is simply not found.

Pictures (in `docs/design/mock-pictures/landsraad-control-area-2/`, because `web-tests/lcc-m7.test.mjs` keeps pictures out of the Command Center notes folder): `sheet-1280-dark.png`, `sheet-1280-light.png`, `sheet-390-dark.png`, `sheet-390-light.png` (today, C, D, E side by side in three states: idle, 25 Houses chosen, Fill asked to confirm; the first screen of a 900 px (1280) or 780 px (390) tall window), `state-language-dark.png` and `-light.png`, `e-drawer-list-layout-dark.png`, `d-flyout-jump-dark.png`. Made headless with Edge over CDP.

## Option E2 (the operator picked E, with changes)

`option-e2.html` (the one mock page, with the shared kit `mock.css`; its behaviour is in `mock-e2.js` because E2 models what is shown separately from what is chosen). Fixture data, nothing is read or written. Changes from E, as asked:

1. **House chips on the lens row.** Show tabs and the House chips (Any, Undecided, Pledged, the words of #1079) sit in one tray, each group with its word (Show, House). The drawer keeps only Specialization, Jump to House and Group (list layout only). The Filters badge counts only the drawer's filters that are not at their default.
2. **No explanatory sentence in the control area.** Counts instead: the medallion, 'Fill 21', 'Complete 14', 'Reopen 16', '25 / 25' shown. The question is one line, 'Fill 21 Houses for Harkonnen?', with Yes and Cancel. Everything that explained something is a tooltip and is in `help-snippet.md` (the Help text 'Fill, Complete and Reopen').
3. **Current wording.** Pledged, Undecided, Wavering (in the Undecided tooltip, as on the cards), specialization, Place (no Place control exists here; it stays the column name of the list), the plate says Houses and factions, never won, claimed or taken. `SYSSELRAAD SECURED` is the victory panel's headline and is not part of the control area; the Help snippet mentions it for the same reason Help does.
4. **Every control stays reachable** (matrix below), one state language, targets 36 px (44 px at 640 px and below), text 12 px for names and 11 px for the crest words.
5. **States** (pictures below): nothing chosen (a slim strip), chosen (the lit plate with badges), the question, drawer open, list layout (Group in the drawer), phone 390, dark and light.

### Added by the operator after the first look

- **Choose shown (N), not Choose all houses.** 'All' (the Show tab, a filter) and 'Choose all houses' (a choice) sounded alike. The plate button is now **Choose shown (N)**, N being the Houses the board shows at that moment; the Show tab keeps 'All'.
- **The rule, stated once.** Show tabs, House chips and the drawer change only what is *shown*. The ticks, Choose shown and Clear change what is *chosen*. A House that a filter hides is never acted on, so the medallion and the Fill, Complete and Reopen badges count only Houses that are chosen **and** shown. A hidden House cannot be ticked (the card is dimmed and says so).
- **today:** a filter that hides a ticked House *unticks it for good* (`lbApply` and the board's `filtered` callback, `app.js` and `lcc-board.js` `visible()`); 'Choose all houses' already chooses only the visible Houses (`window.LCCB.visible`); a card that a filter dims has its box disabled (#881) and an open question is cancelled by a filter (#818). **E2:** the same rule for acting, but the choice is *kept* while the House is hidden and counts again when it is shown again (picture `e2-rule-filter.png`: All, Not started, All).
- **Differs from today:** only that memory. Today forgets, E2 remembers. Either satisfies the rule (a hidden House is never counted or written); forgetting is simpler to explain and cannot surprise anyone later. See 'Ideas, not done'.
- **The slim plate.** With nothing chosen the plate is one 44 px strip (52 px at phone widths): the count 0, the cue 'Choose Houses' and Choose shown (N). Fill, Complete, Reopen, the crests and Clear appear when at least one House is chosen and shown (81 px at 1280, 142 px at 390; 143 and 243 px with the question open). `?plate=full` shows the old flat full plate for comparison (disabled keys, the reason on click).
- **The plate never cuts off the last card row.** The plate is in the page flow after the board and sticks to the window bottom only while the board is longer than the window, so at the end of the page the last row sits clear above it (measured gap 20 px at 1280 x 720, 1024 x 768 and 390 x 780; pictures `e2-bottom-*.png`). That is the same result as a board bottom padding equal to the plate height, without the blank space a padding would add. In the middle of the board the plate covers the cards under it, as any docked bar does. **Below 700 px window height** (and when the on-screen keyboard shrinks the window) the plate is not docked: it is static under the board (picture 1280 x 600, `position: static` measured).

### Tooltip rules (E2 and E3)

1. Every control has a **visible** word or number: nothing is icon-only (the layout switch says Grid and List, Clear says Clear, each crest has its one-word label, the medallion is a number, the Filters key has its number).
2. A tooltip only **adds** the longer explanation. Anything needed before acting is visible text (the question, the counts).
3. It opens on hover and on keyboard focus, and the same text is the control's accessible description (`aria-describedby`).
4. On touch a long press shows it (`mock-e2.js`; not tried on a real device).
5. A tooltip never carries a control: nothing can only be done from a tooltip.

### Every control: visible label and tooltip text (E2)

| Control | Visible label | Tooltip text (also its accessible description) |
|---|---|---|
| Term 7 · 5 × 5 | Term 7 · 5 × 5 | A fixed 5 × 5 board. The game positions are read from board_index. |
| Grid | Grid | Shows the Houses as a 5 × 5 board. |
| List | List | Shows the Houses as a sortable list. Group is then in the drawer. |
| Show | Show | Show changes only which Houses are shown. It never changes which Houses are chosen. |
| All | All | none (the visible word says it) |
| What is left | What is left | none (the visible word says it) |
| Not started | Not started | none (the visible word says it) |
| In progress | In progress | none (the visible word says it) |
| Top tier | Top tier | none (the visible word says it) |
| House | House | House changes only which Houses are shown: all, the Undecided ones or the ones that are Pledged. |
| Any | Any | none (the visible word says it) |
| Undecided | Undecided | No faction has the favor of the House yet. Wavering: both factions have progress on it. |
| Pledged | Pledged | The House is pledged to a faction: Pledged to Atreides or Pledged to Harkonnen. |
| Filters | Filters | Opens the drawer: Specialization, Jump to House and Group (list layout). The number says how many of them are not at their default. |
| 25 / 25 | 25 / 25 | 25 of 25 Houses shown |
| Specialization | Specialization | none (a select with its caption) |
| Jump to House | Jump to House | none (a select with its caption) |
| Group | Group | none (a select with its caption) |
| Count medallion | number (25) | Houses chosen and shown. A House that a filter hides is never counted and never acted on. |
| Choose shown (25) | Choose shown (25) | Chooses the Houses that are shown now. Show, House and the drawer change only what is shown. |
| Clear | Clear | Clears the choice. Nothing is written. |
| Atreides | Atreides | Atreides, your faction. Fill and Complete act for this faction. Choosing it writes nothing. |
| Harkonnen | Harkonnen | Harkonnen. Fill and Complete act for this faction. Choosing it writes nothing. |
| Fill | Fill | Fill changes 21 of the 25 chosen Houses that are shown. A question shows the totals before anything is written. |
| Complete | Complete | Complete changes 14 of the 25 chosen Houses that are shown. A question shows the totals before anything is written. |
| Reopen | Reopen | Reopen changes 16 of the 25 chosen Houses that are shown. A question shows the totals before anything is written. |
| Yes | Yes | The totals the old paragraph carried: missions written, Solari, House Scrip and XP, rewards waiting at the House representative. Written at once, with a backup first. |
| Cancel | Cancel | none (the visible word says it) |

### Element matrix (every existing control stays reachable)

| Control today | E | E2 |
|---|---|---|
| Show (5) | lens, tabs | lens, tabs, group word Show |
| House (3) | drawer, Any / Not won / Favor won | **lens**, Any / Undecided / Pledged |
| Specialization select | drawer | drawer |
| Jump to House select | drawer | drawer |
| Group select (list layout only) | drawer | drawer |
| Grid / List | icon pair | icon and word pair |
| N of 25 shown | count in lens | count in lens (a number) |
| Choose all houses | plate, Choose all | plate, **Choose shown (N)** (also the slim strip) |
| Clear the choice | plate, icon | plate, **Clear** with its word |
| The summary sentence | medallion, badges | medallion, badges (tooltips) |
| Faction select and its hint | two crests | two crests **with words** |
| Fill, Complete, Reopen chosen | plate, badges | plate, badges |
| The question, Yes, Cancel | inside the plate, three lines | inside the plate, **one line**, Yes, Cancel |
| Disabled reason | toast on click | toast on click (`?plate=full`); in the slim strip the cue says what to do |
| Filters badge | counts House too | counts only the drawer's filters |

**Not drawn in any round-2 option, nor in E2 and E3:** today's board also has **Complete and fill chosen** and **Unfill chosen** (they show in `e2-today-vs-e2.png`). The earlier rounds never drew them; this is not a new omission, but the matrix cannot honestly say 'every control' until the operator says where they go. Listed under 'Ideas, not done'.

### Review of E2 (headless Edge over CDP; short and honest)

- **No horizontal overflow** at 1280, 1024, 768, 640, 390 and 320 px (idle, and chosen with the drawer open). Two overflows at 320 px were found and fixed (the cue in the slim strip, Clear in the plate).
- **Targets:** no button, radio or well under 36 px (44 px at 640 px and below), in all four states, both themes, 1280 and 390.
- **Contrast:** 704 texts checked against the rendered pixels behind them (E2 and E3, dark and light, 1280 and 390, idle, chosen, the question, drawer open in list layout): 0 failures (disabled controls and text outside the window not counted).
- **Keyboard order**, E2 nothing chosen: Grid, All, Any, Filters, Choose shown (5 stops). Chosen: Grid, All, Any, Filters, Choose shown, Clear, Atreides, Fill, Complete, Reopen (10). Show, House, layout and the crests are radio groups (arrows move and choose). The question takes the focus; Escape cancels and puts the focus back on Fill (checked). A disabled key still takes focus and its click gives the reason (checked with `?plate=full`). The Filters drawer keys follow Filters when it is open.
- **First card:** 193 px down at 1280 (E: 149), 257 px with the drawer open; 259 px at 390 (E: 229), 319 px with the drawer open. Today's real board: 371 px (idle) and 404 px (chosen) at 1280, 770 and 820 px at 390, measured from the top of the board panel (`e2-today-vs-e2.png`).
- **Weakest points of E2.**
  1. The lens no longer fits one row: Show plus House need about 790 of the 836 px, so Filters and the count drop to a second row (+44 px against E; the drawer adds 64 px more).
  2. The docked plate is the one thing that still covers cards: 81 px at 1280, 142 px (18 percent of a 780 px window) at 390, and 143 / 243 px (31 percent) while the question is open on a phone.
  3. The totals the old paragraph showed before Yes are now a tooltip on Yes. On touch there is no hover (long press only, untried on a device), so on a phone the question is the one-line text and the totals are not on screen.
  4. In the slim strip Fill, Complete and Reopen are not there at all, so the reason-on-click for a disabled key can only be seen in `?plate=full`; the cue 'Choose Houses' carries it.
  5. The crest words (11 px) sit under the octagon and make the full plate about 4 px taller; the two words are close together at 390 px.
  6. E2 remembers a choice that a filter hides (see the rule above); today forgets it.

### Pictures (E2, in `docs/design/mock-pictures/landsraad-control-area-2/`)

`e2-1280-dark.png`, `e2-1280-light.png`, `e2-390-dark.png`, `e2-390-light.png` (nothing chosen, chosen, the Fill question), `e2-drawer-open.png` (grid and list layout), `e2-confirm-open.png`, `e2-tooltips.png`, `e2-rule-filter.png` (All, Not started, All), `e2-annotated-1280-dark.png` and `e2-annotated-390-dark.png` (callouts: Lens row, Drawer, Docked plate; `?annotate=1` or `?annotate=phone` draws them over the page, they exist in no other view), `e2-bottom-1280x720.png`, `e2-bottom-1280x600.png` (static), `e2-bottom-1024x768.png`, `e2-bottom-390x780.png` (scrolled to the end, last row clear), `e2-today-vs-e2.png` (today, E2 and E3 at one crop, first-card offsets).

Plate heights measured (px): slim 44 (1280, 1024) and 52 (390); full 81 and 142; with the question 143 and 243. At 1280 x 720 the plate is docked and sticky; at 1280 x 600 it is static.

## Option E3 (the plate is gone: one contextual bar at the top)

`option-e3.html`, the same kit and fixture, behaviour in `mock-e2.js` (variant e3). Nothing is docked at the bottom.

- **One strip, two modes, same place, same height** (86 px at 1280, 150 px at 390, the height of the filters; the action bar centres in it). Nothing chosen: the lens row, Show tabs, House chips, Filters with its badge, the count, and a small text link **Choose shown (N)** at its right end. At least one House ticked: the strip swaps (a 0.16 s fade, none under reduced motion) into the action bar: the medallion with 'chosen', the two crests with their words, Fill 21, Complete 14, Reopen 12, Clear. **Escape or Clear** swaps back and puts the focus on Choose shown.
- **The question** is one line inside the same bar (the keys give way to it): 'Fill 21 Houses for Harkonnen?', Yes, Cancel. Escape cancels and returns the focus to Fill.
- **The Filters drawer is not available while the bar shows;** the Clear tooltip says so (no visible sentence).
- **The rule** is the same as E2 (only Houses that are chosen and shown are counted or acted on). In E3 it can never be broken, because no filter can be changed while Houses are chosen.
- The mock opens with 21 Houses ticked (`?state=chosen`), as in the brief; the badges are the fixture's 21, 14 and 12 for that choice (the brief's 'Complete 21, Reopen 21' would need a different fixture).

### Element matrix, delta against E2

| Control | E2 | E3 |
|---|---|---|
| Show, House chips, Filters, count | lens row | lens row (the filters mode of the strip) |
| Drawer | under the lens, always available | under the lens, **not available while the action bar shows** |
| Choose shown (N) | plate, a key | **small text link** at the right end of the lens row |
| Count medallion | plate | action bar, with the word 'chosen' |
| Clear | plate, a key | action bar, at its right end; tooltip explains the drawer |
| Crests, Fill, Complete, Reopen | docked plate | action bar |
| Question, Yes, Cancel | inside the plate, under the keys | inside the action bar, in place of the keys |
| Docked plate | yes, sticky, static below 700 px | **none** |
| Slim idle strip | yes | not needed (the lens row is the idle state) |

### Every control: visible label and tooltip text (E3)

| Control | Visible label | Tooltip text (also its accessible description) |
|---|---|---|
| Term 7 · 5 × 5 | Term 7 · 5 × 5 | A fixed 5 × 5 board. The game positions are read from board_index. |
| Grid | Grid | Shows the Houses as a 5 × 5 board. |
| List | List | Shows the Houses as a sortable list. Group is then in the drawer. |
| Show | Show | Show changes only which Houses are shown. It never changes which Houses are chosen. |
| All | All | none (the visible word says it) |
| What is left | What is left | none (the visible word says it) |
| Not started | Not started | none (the visible word says it) |
| In progress | In progress | none (the visible word says it) |
| Top tier | Top tier | none (the visible word says it) |
| House | House | House changes only which Houses are shown: all, the Undecided ones or the ones that are Pledged. |
| Any | Any | none (the visible word says it) |
| Undecided | Undecided | No faction has the favor of the House yet. Wavering: both factions have progress on it. |
| Pledged | Pledged | The House is pledged to a faction: Pledged to Atreides or Pledged to Harkonnen. |
| Filters | Filters | Opens the drawer: Specialization, Jump to House and Group (list layout). The number says how many of them are not at their default. |
| 25 / 25 | 25 / 25 | 25 of 25 Houses shown |
| Choose shown (25) | Choose shown (25) | Chooses the Houses that are shown now. Show, House and the drawer change only what is shown. |
| Count medallion | number (21) | Houses chosen and shown. A House that a filter hides is never counted and never acted on. |
| Atreides | Atreides | Atreides, your faction. Fill and Complete act for this faction. Choosing it writes nothing. |
| Harkonnen | Harkonnen | Harkonnen. Fill and Complete act for this faction. Choosing it writes nothing. |
| Fill | Fill | Fill changes 21 of the 21 chosen Houses that are shown. A question shows the totals before anything is written. |
| Complete | Complete | Complete changes 14 of the 21 chosen Houses that are shown. A question shows the totals before anything is written. |
| Reopen | Reopen | Reopen changes 12 of the 21 chosen Houses that are shown. A question shows the totals before anything is written. |
| Clear | Clear | Clears the choice and brings the filters back. The Filters drawer is not available while Houses are chosen. |
| Yes | Yes | The totals the old paragraph carried: missions written, Solari, House Scrip and XP, rewards waiting at the House representative. Written at once, with a backup first. |
| Cancel | Cancel | none (the visible word says it) |
| Specialization | Specialization | none (a select with its caption) |
| Jump to House | Jump to House | none (a select with its caption) |
| Group | Group | none (a select with its caption) |

### C, E2 and E3 side by side (picture `compare-c-e2-e3.png`, same fixture, same crop)

| | C | E2 | E3 |
|---|---|---|---|
| Height above the first card at 1280 | 293 px | 193 px (257 with the drawer open) | 193 px (257 with the drawer open; the drawer is away while the bar shows) |
| Height above the first card at 390 | 459 px | 259 px (319 with the drawer open) | 259 px (319 with the drawer open) |
| Control places | 3 (head switch, plate, filter tray) | 4 (head switch, lens row, drawer, docked plate) | 3 (head switch, the strip in two modes, drawer) |
| Clicks to Fill from the first screen (all Houses, own faction) | 3 (Choose all, Fill, Yes) | 3 (Choose shown, Fill, Yes) | 3 (Choose shown, Fill, Yes) |
| Clicks to Fill one specialization | 5 | 6 (Filters first) | 6 (Filters first) |
| Change a filter while Houses are chosen | 1 or 2 | 1 or 2 (the choice stays) | Clear first (the choice is lost), then 1 or 2 |
| Cut-off risk | none (nothing docked) | the docked plate covers 81 px (142 on a phone) of the cards under it while scrolling; clear at the end of the page; static below 700 px | none docked; but the bar scrolls away with the top of the page, so on a long board the user scrolls back up to Fill |

### Review of E3 (short)

Same checks as E2 (run on both): no overflow, no target under the floor, 0 contrast failures, Escape and focus verified (Escape on the question returns to Fill; Escape or Clear in the bar returns to Choose shown). Keyboard order, nothing chosen: Grid, All, Any, Filters, Choose shown (5); chosen: Grid, Atreides, Fill, Complete, Reopen, Clear (6).

**Weakest points of E3.**

1. The action bar is at the top: on a 25-card board the user who has ticked cards at the bottom scrolls back up to press Fill (E2's docked plate does not have this problem; it has the cut-off one).
2. No filter while Houses are chosen: to look at another set the user must Clear, and the choice is lost. A user who ticks, then wants to narrow, loses the ticks.
3. The strip is as tall as the filters (86 px at 1280), so the action bar has more air than it needs.
4. Crests and three keys on a phone fill the strip exactly (150 px); the words under the crests are 11 px and close together.
5. Escape does two jobs: it cancels an open question, and with no question it clears the choice. Two presses from the question to the filters.

Pictures (E3): `e3-1280-dark.png`, `e3-1280-light.png`, `e3-390-dark.png`, `e3-390-light.png` (nothing chosen, 21 chosen, the Fill question), `e3-filters-state.png`, `e3-action-state.png`, `e3-confirm-open.png`, `compare-c-e2-e3.png`.

## Option E4 (E2 without the Filters key: the second row is always visible)

`option-e4.html`, the same kit and fixture, behaviour in `mock-e2.js` (variant e4). Operator feedback on E2: 'click Filters to open a pane that has two filters does not feel like a professional web app'.

**The rule.** On a desktop width (701 px and up) nothing is behind a key: row 1 is the lens (Show tabs and the House chips, Any / Undecided / Pledged), row 2 is always visible and holds Specialization (select), Jump to House (select), Group (select, list layout only, as in E2's drawer) and the count '25 / 25' aligned right. The docked plate (Choose shown (N), the faction crests, Fill, Complete, Reopen, Clear) is exactly E2's: a slim 44 px strip when nothing is chosen, static below 700 px window height, the one-line question 'Fill 21 Houses for Harkonnen?' with Yes and Cancel.
**At phone width (700 px and under)** row 2 collapses behind ONE disclosure, **Filters (n)**, n being the filters that are on (House chip not Any, Specialization not All, Group not No groups; Show is on the lens row and Jump to House is not a filter, so neither is counted). This is the only place a pane remains, because there is no room. Open, it holds four headed sections, in the same words as the E5 menu: **House** (the House chips move in, so the lens row keeps only Show), **Specialization**, **Jump to House**, **Group** (list layout only). The section headings replace the captions of the selects.

Measured (first card, down from the top of the board panel, same method as E2: 193 / 259 idle, 257 / 319 drawer open, today 371 / 404 and 770 / 820):

| | 1280 idle | 1280 chosen | 390 idle | 390 chosen | 390 Filters open |
|---|---|---|---|---|---|
| E4 | **201 px** | **201 px** | **213 px** | **213 px** | 437 px |

So E4 is 8 px taller than E2 closed at 1280 (the row is always there) and 56 px shorter than E2 with the drawer open; at 390 it is 46 px shorter than E2 (the House chips are inside the disclosure). Plate heights are E2's (slim 44 / 52, full 81 / 142, with the question 143 / 243).

**What E4 drops against E2.** The Filters key, its drawer and its badge on the desktop; the drawer's open and close state; the 'second row appears after a click' step (Specialization and Jump to House are one click closer, 5 clicks to Fill one specialization instead of 6). On a phone the House chips are no longer on the lens row (they are two taps away, inside Filters (n)). Nothing else from E2 is dropped: the plate, the rule, the tooltips and the matrix are unchanged.

## Findings: what Tau and the Trainer already do for filters (read-only, matched by E5 and E6)

| Where | What it does | Reused by E5 / E6 |
|---|---|---|
| `internal/web/static/theme-menu.js` (the Theme menu, #570, #587, #683) | A `role="menu"` popover under its button, grouped lists under group headings; a search box at the top of a long group that stays while the list scrolls; a line 'N of M palettes' (and what to do when none match); Arrow Up / Down / Home / End, Enter or Space picks and the menu stays open, **Escape clears the box first and closes the menu on the next press and gives the focus back to the button**, Tab leaves, a click outside closes, items 44 px on touch | The popover under the button, the search box with 'N of 25 Houses match' and 'No House matches the search.', the Escape order, focus back to the button, outside click, 44 px on touch, items that stay open so a combination can be tried |
| `internal/web/static/lcc/lcc-board.js` `filters()` and `lcc-board.css` `.lcc-bd-chip` (the board's real filters) | Show and House as chip groups with a small caps label, Jump to House as a select 'Choose a House…', Group 'No groups / By region / By specialization'; chips are 36 px (44 px on a phone), gold when on | The words (Show, House, Pledged, Undecided, Jump behaviour, By region, By specialization), the chip height and the gold on-state |
| `internal/web/static/app.js` `countText` and the filter boxes of Inventory, Journey and Recipes | 'N of M ... match' while a filter narrows the list, 'No node matches the filter.' | The count line and the empty line of the House search |
| Trainer `internal/mapui/static/map.html` (`<details id="filters">`) and `app.js` (`buildTree`, `tsw`) | A card headed **Filters** with a count (`fcount`), groups with a name and a count, one switch look for every box (#1027), 'Hide all', and a search note 'Map shows matches only · Clear' | The head 'Filters' with its number (E4 phone: Filters (n)); the grouped-list-with-counts idea (a number beside each category in the menu) |

**Wording decision.** The existing word for emptying a search or a filter is a bare **Clear** (the Trainer's search note) and 'Hide all' (the Trainer's map kinds). E5 and E6 use **Clear filters** as the operator asked, because a bare Clear is already the plate's key for the *choice* (the Houses ticked); the two must not share a word. No existing pattern has removable chips with a cross: the chip look is the board's `.lcc-bd-chip` (gold border, 12 px text) with the cross added (new). Multi-select boxes in a list are new too (the Theme menu has radio items and the Trainer has switches; the kit's switch would work, but a square box says 'several' better on a list of 25).

## Option E5 (E2 with one Filter menu on the lens row)

`option-e5.html`, behaviour in `mock-e2.js` (variant e5). No second row, no side drawer.

- **The lens row** is one row at 1280: the Show tabs, one **Filter** key (a chevron, no number), and '25 / 25' at the right. The House chips are not on the lens row any more: they are in the menu. The docked plate is E2's, unchanged.
- **Active filters are chips beside the key**: 'House: Pledged', 'House: Alexin' (one chip per House picked), 'Specialization: Combat', 'Group: By region'; each has a cross and removes itself (the focus goes back to Filter). **Clear filters** appears when any chip exists and empties all of them (Show is not part of it: it stays visible on the row). With two or more chips the row wraps to a second line (first card 193 px at 1280, as E2 idle).
- **The menu is a popover with the categories on the left and their items on the right** (a two-pane list), built from categories: **House** (the pledge state Any / Undecided / Pledged, then a search box 'Search Houses', then the 25 Houses by name, three columns, the list scrolls and the search stays), **Specialization** (All, then the five, several can be picked, a chip each) and **Group** (None / By region / By specialization, the category appears only in the list layout, as E2's drawer had Group). Each category shows a number when something in it is on. The separate Jump to House control is gone: choosing a House adds it, scrolls the board to it and outlines it for 1.8 s, and with several Houses picked only those are shown (a chip per House).
- **Two-pane or sections? Two-pane.** The Theme menu uses stacked sections because each of its groups is short. Here House alone is 25 names plus a search and a pledge row: as a section it pushes Specialization and Group below a scroll, and the menu grows to about 600 px. The two-pane list keeps the popover at about 380 px, shows one category at a time, and is the same structure as the phone sheet (categories first, then the items), so there is one model to learn. The sections version was not built.
- **Desktop:** the popover sits under the Filter key, 640 px wide (clamped to the panel), over the board, not pushing it. **Phone (700 px and under):** a bottom sheet with a dimmed page behind it: first the category list (House, Specialization, Group, with their numbers), then the items of the one tapped, with a Back key; Done closes it. The same opens by keyboard.
- **Keyboard** (checked in headless Edge): Enter or Space on Filter opens it and the focus lands on the selected category; Arrow Down / Up move between categories (and show their items), Arrow Right enters the items, Arrow Down / Up walk a list (Arrow Up from the first House goes to the search box, Arrow Down from the box to the first House), Arrow Left goes back to the categories, Space or Enter picks, the pledge and Group keys are radios (arrows move and choose). **Escape** closes the menu and puts the focus back on Filter (in the search box it clears the box first, as the Theme menu does); a click outside closes it; Tab out of it closes it. Order with nothing chosen: Grid, All, Filter, Choose shown (4 stops).

## Option E6 (E3 with the E5 Filter menu)

`option-e6.html`, behaviour in `mock-e2.js` (variant e6). The E3 rule, unchanged: one strip, two modes, same place. Nothing chosen: the lens row (Show tabs, the Filter key and its chips, Clear filters, '25 / 25', Choose shown (N) as a text link at the right end). Houses chosen: the strip swaps into the action bar (the count with 'chosen', crests, Fill, Complete, Reopen, Clear); **the Filter key and the chips go with the lens row**, the menu is closed if it was open, and the Clear tooltip says 'Clears the choice and brings the filters back. The Filter menu is not available while Houses are chosen.' Clear or Escape swaps back and puts the focus on Choose shown (checked). The question is one line inside the bar, as in E3. Order with nothing chosen: Grid, All, Filter, Choose shown (4); chosen: Grid, Atreides, Fill, Complete, Reopen, Clear (6).

The strip is 81 px at 1280 (E3: 86), first card 188 px at 1280 and 253 px at 390; 193 px at 1280 with a chip row.

## C, E2, E3, E4, E5 and E6 side by side (picture `compare-e2-e3-e4-e5-e6.png`, same fixture, same crop and size as `compare-c-e2-e3.png`, two rows)

| | C | E2 | E3 | E4 | E5 | E6 |
|---|---|---|---|---|---|---|
| First card at 1280, idle | 293 px | 193 px (257 drawer open) | 193 px (257 drawer open) | **201 px** | **149 px** (193 with a chip row; the menu floats, 0 added) | **188 px** (193 with a chip row; the menu floats) |
| First card at 390, idle | 459 px | 259 px (319 drawer open) | 259 px (319 drawer open) | **213 px** (437 with Filters open) | **213 px** | **253 px** |
| Control places | 3 | 4 (head switch, lens row, drawer, docked plate) | 3 (head switch, strip in two modes, drawer) | 4 (head switch, lens row, filter row, docked plate); phone: Filters (n) pane | 3 (head switch, lens row with the menu, docked plate) | 2 plus the menu (head switch, strip in two modes, popover) |
| Hidden behind a click | nothing | Specialization, Jump, Group (drawer) | same | nothing on desktop; phone: all four sections | pledge state, Houses, Specialization, Group (menu) | same as E5 |
| Clicks to Fill all (own faction) | 3 | 3 | 3 | 3 | 3 | 3 |
| Clicks to Fill one specialization | 5 | 6 | 6 | 5 (select, Combat, Choose shown, Fill, Yes) | 6 (Filter, Specialization, Combat, Choose shown, Fill, Yes) | 6 |
| Clicks to show only Pledged Houses | 1 | 1 | 1 | 1 | 2 (Filter, Pledged) | 2 |
| Change a filter while Houses are chosen | 1 or 2 | 1 or 2 (the choice stays) | Clear first (the choice is lost) | 1 or 2 (the choice stays) | 2 or 3 (the choice stays) | Clear first |
| Cut-off risk | none | docked plate covers 81 px (142 on a phone) while scrolling | none docked; the bar scrolls away | as E2 | as E2, plus the menu covers the board while open | none docked; the bar scrolls away; the menu covers the board while open |
| Words / tooltips | | | | the same words, fewer places for them | one new word (Filter), one new link (Clear filters), chips | as E5 |

### Review of E4, E5 and E6 (headless Edge over CDP, short and honest)

- **No horizontal overflow** at 1280, 1024, 768, 640, 390 and 320 px, in the idle and chosen states, with E4's Filters open and with the E5 and E6 menu open (Houses, Specialization).
- **Targets:** no button, radio, checkbox, tab, select or search box under 36 px (44 px at 640 px and below, and the whole sheet at 700 px and below), same states and widths.
- **Contrast:** 1526 texts checked against the rendered pixels behind them in 64 renders (E4, E5, E6; dark and light; 1280 and 390; idle, chosen, the question, the menu or Filters open, list layout): **0 failures**. Not counted: disabled controls, text behind the phone sheet's dimmed page (dimmed on purpose, inert), text scrolled out of a list.
- **Escape, focus, outside click** (all driven by keys in the headless run): Escape closes the menu and the focus returns to Filter; in the search box the first Escape clears it; the Fill question's Escape still returns to Fill (E2 rule, unchanged); a click outside closes the menu; removing a chip returns the focus to Filter; on the phone the sheet goes categories, items, Back, Escape. E6: choosing a House closes the menu and swaps to the bar; Clear swaps back with the focus on Choose shown.
- **Keyboard order** E4: nothing chosen Grid, All, Any, Specialization, Jump to House, Choose shown (6); chosen adds Clear, Atreides, Fill, Complete, Reopen (11). E5: nothing chosen 4 stops, chosen 9 (Grid, All, Filter, Choose shown, Clear, Atreides, Fill, Complete, Reopen). E6 as above.

### Weakest points

**E4**
1. The row is permanent: 8 px more than E2 closed, whatever the user does, and it carries Jump to House, which is the least used of the three controls.
2. The count '25 / 25' now sits on the second row, away from the Show tabs it summarises.
3. On a phone Filters (n) is still a pane, and it is tall when open (first card 437 px); the House chips live in two places (lens row on desktop, inside the disclosure on a phone).
4. Jump to House changes nothing visible in this mock (it only counts as not a filter).

**E5**
1. Every filter costs a click and a pick: Pledged is 2 clicks (E2 and E4: 1), a specialization is 3 (E4: 2). The pledge state, the thing people change most on the lens, is now inside a menu.
2. Chips grow without limit: one chip per House picked. Two chips already wrap the lens to a second row (193 px at 1280); eight Houses make three rows. A '+N more' collapse is not drawn.
3. The popover is 640 px wide and about 380 px tall: it covers the first three rows of the board while open (it never pushes it).
4. Picking a House does two jobs (jump and highlight, and with several, show only those); with one House picked the board shows only that House, so the jump has nothing to scroll past. The operator should decide: 'show only' or 'jump', or a separate Jump again.
5. Four new kinds of control (category list, a search box, check lists, chips) against E4's two selects; roles mix a dialog, a tab list, radios and checkboxes and are not verified with a screen reader.
6. The names beyond the first eight Houses are fixture inventions; the order (A to Z) maps to the board squares only in the mock.

**E6** (E5's, and E3's)
1. While Houses are chosen the Filter key and its chips are gone, so the user cannot see which filters still limit what 'chosen and shown' counts.
2. Changing a filter while Houses are chosen means Clear first and the choice is lost.
3. The action bar is at the top and scrolls away on a long board.
4. Escape does two jobs (cancels the question, then clears the choice), as in E3.

### What E4, E5 and E6 changed in the shared files

`mock.css` gained rules for `.k-e4`, `.k-m5`, the chips, the menu, the sheet (all new class names); `mock-e2.js` gained the variants e4, e5 and e6 (the E2 and E3 branches are untouched; E3 and E6 share the bar behaviour). `option-e2.html`, `option-e3.html` and their pictures were not changed.

### Pictures (E4, E5, E6)

`e4-1280-dark.png`, `e4-1280-light.png`, `e4-390-dark.png`, `e4-390-light.png`, `e4-annotated-1280-dark.png` (callouts: Lens row, Filter row, Docked plate), `e4-phone-filters-open.png` (closed, open, open in list layout); `e5-1280-dark.png`, `e5-1280-light.png`, `e5-390-dark.png`, `e5-390-light.png` (nothing chosen, 25 chosen, the Fill question), `e5-menu-open.png` (House and Specialization on a desktop width, the phone sheet as categories and as the House items), `e5-annotated-1280-dark.png` (Lens row, Filter menu, Docked plate); `e6-1280-dark.png`, `e6-1280-light.png`, `e6-390-dark.png`, `e6-390-light.png`, `e6-menu-open.png`, `e6-annotated-1280-dark.png`; `compare-e2-e3-e4-e5-e6.png` (E2 with the drawer open, E3 with its drawer open, E4, E5 with the menu open, E6 with the menu open, E6 with Houses chosen; boxes are the parts, the pink line is the first card).

## Not verified (E4, E5 and E6)

- Nothing was tried on a real phone or touch screen: the phone width is a 390 px window; the sheet's behaviour with the on-screen keyboard (the search box on a phone) is not known.
- No screen reader; roles and names are set (dialog, tab list, radio and checkbox buttons) and the keys are measured, nothing more.
- The Filter key's tooltip, Tab leaving the menu (it closes it, in code) and the Home / End keys in the lists were not checked; the Group category and chip were rendered (list layout) but not driven by keys.
- The scroll to a picked House is code only; in the stub board a picked House is also the only one shown.
- Reduced motion and forced colours were not rendered; the chevron turn and the sheet have no animation beyond the kit's.

## Not verified (E2 and E3)

- Not seen on a real phone or a touch screen; the long press is code only. The on-screen keyboard case is the `max-height: 699px` rule; a keyboard that does not resize the page (some iOS versions) would leave the plate docked.
- Not checked with a screen reader; roles, names and descriptions are set and the tab order is measured.
- Reduced motion and forced colours were not rendered for E2 and E3 (the rules are the kit's; E3's fade is covered by the kit's reduced-motion rule, which switches every transition off).
- The totals in the tooltip on Yes are the fixture's, scaled with the count; the Complete and Reopen questions are short stand-ins.
- Today's picture is the real board over the repository's synthetic fixture, made headless by a copy of `scripts/lcc-m7-sweep.mjs` (a copy in the worker's folder, nothing in the repository changed).

## Ideas, not done

- Forget a choice that a filter hides (today's behaviour) instead of keeping it: simpler to explain.
- Put the Filters key and the count in the head row beside Grid / List, which would give a one-row lens and a first card at about 149 px again.
- A small ? key in the lens that opens the Help heading 'Fill, Complete and Reopen'.
- Where do **Complete and fill chosen** and **Unfill chosen** go? A More key on the plate is the obvious candidate.
- E3: make the action bar sticky at the top of the window so it is never scrolled away.
- Show a faint count 'N chosen' in the lens row (E3) while a question is open.
- E4 to E6: a '+N more' collapse for the chips when more than three are on.
- E5: the sections version of the menu (one scrolling column with headings, as the Theme menu has) next to the two-pane one, if the operator prefers it.
- E6: a small note 'filters on' in the action bar so the user can see that a filter still limits the count.
- E4: Jump to House as a small link in the count line instead of a permanent select.

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

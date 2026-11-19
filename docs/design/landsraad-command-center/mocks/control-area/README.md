# Control area mock (tau #974)

Design mock for a calmer control area above the Landsraad board. Design only: no app code is changed, nothing is built until the operator approves one option.
The two options use the same 18 controls as today, the app's real tokens and the Command Center's own CSS; they differ in where the action bar lives. Fixture data, nothing is read or written.

- **Option A, view bar and action strip** (`option-a.html`): two groups, one under the other. VIEW (read-only, flat, two rows): Show, House, Specialization, Jump to House, Grid / List, the count. ACTIONS (its own sunken strip, always there): "N Houses chosen" with Choose all and Clear as text links, then Faction, then Fill, Complete, Reopen. Buttons are disabled until a House is chosen and the summary line is the reason.
- **Option B, view bar and floating action bar** (`option-b.html`): the same VIEW bar, with Choose all as a link at the end of its second row. The ACTIONS bar does not exist at rest: it appears at the foot of the board only while Houses are chosen (the page's phone bar, at every width) and holds the summary, Clear, Faction, Fill, Complete, Reopen.
- Both: one control height (36 px, 44 px at 640 px and below or for a coarse pointer, #954), one control font (13 px, 12 px in a narrow box), labels 12 px regular and dim in sentence case, an 8 px spacing step, square edges, the gold as the only accent, group labels instead of four loud capitals, no standing explanatory text (the faction hint moves into the select's title, the long summary sentence becomes a count on each button).
- The write buttons are told apart from the view switches by place and by form: a selected view switch is a quiet tint with an underline, Fill and Complete are the only solid fills, Reopen is an outline. Every write keeps its question: it appears inside the action bar with the real wording of today's question, Cancel, and Escape.
- Open `option-a.html` or `option-b.html` from any static file server over the repository (`?state=idle|chosen|confirm`). The pictures `sheet-1280-dark.png`, `sheet-1280-light.png`, `sheet-390-dark.png`, `sheet-390-light.png` put today (the real page, rendered headless over fixture data), A and B side by side in three states.
- Not drawn: the legend row and the cards under the control area, the Inspector, the toast that reports a write (unchanged), the phone bar of A (see the review).

## What makes today's control area cluttered

Measured on the real page at 1280 px (control area 810 px wide) and 390 px, in both themes:

1. **18 controls in 5 control rows at 1280, 8 to 9 at 390** (about 650 px tall at 390, more than most of a phone screen), plus two lines of text (the count and the sentence "Chosen: 25 houses. Fill chosen changes 21, Complete chosen changes 14 for Atreides, Reopen chosen changes 16.", 117 characters, "Nothing is chosen." when empty).
2. **Three kinds of control share one look.** Filters, selection (Choose all houses, Clear the choice) and writes (Fill, Complete, Reopen) are all boxed 12 px buttons; Choose all sits in the line of Jump to House, the writes have no boundary of their own. Fill and Complete are a 24 percent gold tint, a selected chip a 9 percent gold tint: the gold means both "on" and "primary".
3. **Four label styles for four groups**: SHOW, HOUSE, JUMP TO HOUSE and FACTION FOR FILL AND COMPLETE are 12 px, weight 800, spaced capitals, 44 to 248 px wide. The longest label is the loudest object in the area. Three more text sizes sit beside them (14 px selects, 12 px buttons and chips, 11 px count and sub-line).
4. **Mixed heights and gaps**: chips and buttons 36 px, the Specialization and Jump selects 39 px, the Faction select 36 px; gaps of 4, 9, 12, 18 and 20 px, none on the 8 px step.
5. **Every control has its own border**: 17 boxes in a 810 px strip, four separate groups of chips with 4 px between them.
6. **The view filters are split**: Specialization and Grid / List sit in the panel head, Show, House and Jump on the line below, so one idea (what do I look at) is in two places.
7. **Duplicate and uneven words**: "chosen" five times, "houses" lowercase in the buttons against "Houses" in the count, a label that names the buttons below it ("for Fill and Complete") but is drawn after Fill, Complete and Reopen.
8. **Standing text**: the hint "starts on your own faction", the sum sentence and the sub-line under the title are always on screen.
9. **No state**: with nothing chosen Fill, Complete and Reopen look as live as with 25 chosen.
10. **A second accent**: the confirm question has a blue border, against the gold of everything else.
11. **At phone widths the writes exist twice** (the bar at the top and the sticky phone bar).
12. In forced colours (Windows high contrast) the selected chips are unreadable today: the label is drawn black on a black plate on the cyan highlight (reproduced headless; the mock sets `forced-color-adjust: none` on a selected switch to avoid it).

## Element matrix

Every existing control stays reachable. `data-lb`, `data-lcb`, `data-do` and `data-fk` stay as they are; the request golden is untouched (layout and labels only).

| Control | Today | A | B |
| --- | --- | --- | --- |
| Specialization select | panel head, right | view bar row 2 | same as A |
| Grid / List | panel head, right | view bar row 2, after Jump | same as A |
| Show chips (5) | filter bar, line 1 | view bar row 1, one joined group | same as A |
| House chips (3) | filter bar, line 1 | view bar row 1, one joined group | same as A |
| Jump to House | filter bar, line 2 with the choose buttons | view bar row 2 | same as A |
| Group select (list layout only) | filter bar, after Jump | view bar row 2, after Jump | same as A |
| "25 of 25 Houses shown" | right end of the Jump line | right end of view bar row 2 | right end of view bar row 2, before Choose all |
| Choose all houses | with Jump to House | action strip, link "Choose all" | view bar row 2, link "Choose all" |
| Clear the choice | with Jump to House | action strip, link "Clear" (disabled when none) | floating bar, link "Clear" |
| Fill chosen | own row | strip, primary, "Fill" with a count | floating bar, primary, "Fill" with a count |
| Complete chosen | own row | strip, primary, "Complete" with a count | floating bar, primary |
| Reopen chosen | own row | strip, outline, "Reopen" with a count | floating bar, outline |
| Faction select | row under the buttons, label "Faction for Fill and Complete" | strip, directly before the buttons, label "Faction", hint in the title | floating bar, same |
| Sum sentence | line under the faction | "N Houses chosen" / "No Houses chosen"; the three counts are on the buttons and in their titles | "N Houses chosen"; same counts |
| Confirm question, Yes, Cancel | block under the bar | inside the strip, under the buttons | inside the floating bar, under the buttons |
| Result line | toast (#943) | toast, unchanged | toast, unchanged |
| Phone bar (`lsb-phonebar`) | sticky bottom at 768 px and below | stays (a duplicate of the strip) | replaced: the floating bar is it, at every width |

Decisions for the operator: the button labels become "Fill", "Complete", "Reopen" (the group says what they act on; keep an `aria-label` of "Fill chosen Houses" for the accessible name); the long sum sentence is replaced by counts on the buttons.

## UX review

Rendered headless (Edge) in both themes at 1280, 1024, 390 and 320 px wide, in the three states, 48 combinations per run, plus a 600 px box, forced colours and reduced motion by inspection.

- **Hierarchy and grouping.** Both: three boxed groups (two joined switches and the layout switch) instead of 17 boxes, one label style, one height. A makes the split visible all the time: a flat view bar over a sunken strip with a border. B makes it by time: at rest the area is only view controls (13 controls, 2 rows at 1280, 4 at 390), and the write bar shows up when it has something to act on.
- **Scan path.** A reads top to bottom: what to look at, then what to do; the strip stays put, so nothing jumps when a House is chosen (the strip is 54 px idle, 98 px chosen at 1280, and sits above the board, so the board moves by 44 px). B has no jump (the bar overlays the foot of the board) but a person must notice a bar at the bottom edge; at 390 the bar is three rows (183 px, about a fifth of a phone screen) and covers the foot of the board while Houses are chosen.
- **Touch targets.** Measured: every control is 36 px or more tall at 1024 and wider, 44 px or more at 640 and below; text links are 36 / 44 px tall and 50 px or more wide. No control under the floor in any of the 48 cases.
- **Text and contrast.** Smallest text is 11 px (the sub-line and eyebrow, unchanged); controls and labels are 12 or 13 px. All text that is not a disabled control measured at 4.5:1 or better against the panel, both themes (the count on the primary buttons is outlined rather than tinted, which failed at 3.9:1 in Dayside).
- **Focus order.** A: view bar, strip, board, which is also the reading order. B: the floating bar must stay right after the view bar in the DOM (position fixed, not placed after the board), or a keyboard person tabs through 25 Houses to reach Fill; the mock keeps that order. The confirm group takes focus, not Yes, so Enter cannot confirm by accident; Escape cancels.
- **Keyboard.** Switches are buttons with `aria-pressed` and sit in labelled groups (as today); the 10 switch buttons are 10 tab stops. Arrow-key movement inside a group would cut that to 3 and is worth doing in the build for either option; not in the mock.
- **Disabled and the reason.** Disabled writes use `aria-disabled` (still focusable), the dashed outline of today's disabled buttons and `aria-describedby` on the summary, so the reason is read, not only drawn. A shows it all the time; B never shows a disabled write, because the bar is absent.
- **Reduced motion.** The only motion is B's 160 ms rise of the bar, switched off under `prefers-reduced-motion`. A has none.
- **Forced colours.** Selected switch: Highlight; primary: a 2 px border (a fill is lost in forced colours); disabled: GrayText with a dashed edge. Rendered and checked for both options.
- **Narrow widths.** No horizontal overflow at 1280, 1024, 390 and 320 px. Inside the board panel the layout steps down by the panel's own width, as the board does: 810 px and a 600 px box keep two view rows; 520 px and below stack the labels over the controls, share the switch width and put Specialization and Jump side by side. At 390 the control area is about as tall as today in A (view 272 px plus strip 184 px), clearly shorter in B at rest (view 272 px, nothing else).
- **Weak points.** A: at 390 it saves little height; the strip is always there; at phone widths the existing phone bar would duplicate it, so the build should retire the phone bar or hide the strip's buttons there. B: the write controls are away from the view controls and the board, the bar covers content, a second place to learn, and a person who does not know to choose Houses first never sees Fill at all.

## Recommendation: A

A does what the issue asked in the way it asked it (two clearly separated bars, disabled with a reason, one accent) and keeps writes in a fixed place, where a person who edits a save looks first and where nothing appears or covers the board by itself. B is the calmer picture at rest, but it hides the write controls, depends on a fixed bar that costs a fifth of a phone screen, and puts the focus order at risk. If the operator wants B's quiet idle state later, A's strip can collapse to its summary line when nothing is chosen without redoing the view bar.

## Not verified

- No real hands (touch or screen reader): contrast, sizes, overflow and the forced-colours picture were measured and rendered headless; focus order and Escape were read from the markup and the script, not walked with a screen reader.
- The mocks use stand-in cards and a stand-in board height; the real board, the Inspector at the right of the panel, the legend row and the toast were not redrawn.
- Only the dark Fremen and the light Dayside themes were rendered, not the Awakening and Sietch looks.
- The list layout (with the Group select) and the Classic list view were not drawn.
- The wording of the questions for Complete and Reopen in the mock is a short stand-in; only Fill's question is today's real text (Houses capitalised).

## How the pictures were made

The "today" column is the real page: the editor's static files served over loopback, with the synthetic Landsraad fixture and the generated UI fixtures (`UI_FIXTURES_DIR=<dir> go test ./internal/web -run TestDumpUIFixtures`), the same set-up as `scripts/lcc-m7-sweep.mjs`, driven headless over the DevTools protocol with the in-page choice made by the page's own Choose all and Fill buttons. A and B are `option-a.html` and `option-b.html` rendered the same way.

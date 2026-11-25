# "About this" moves into the Help tab: mock-up and inventory (refs #1233)

- `about-help.html`: a static mock-up. It shows the Vehicles tab with a **Help** link where each **About this** was, and the Help tab with a section index in tab order. The Vehicles section holds today's four texts, moved as they are; the other sections are placeholders. All data is made up. It opens from disk, loads nothing, and carries copies of the editor's own stylesheets. The vehicle cards and rows are the markup today's page draws from made-up answers; only the disclosures were replaced.
- In the bar at the top: **Proposed lines kept on the card** (on: the safety and state lines this inventory proposes to keep; off: only the Help links), **Review marks**, and **View at 400 px**.

Nothing here changes the editor's code or reads a save.

## Count

**28** disclosures in the code: 26 calls of `about()` and 2 of `lbAbout()`. 26 can be on screen in the editor as it is. The 2 on the Landsraad page are drawn only by the fallback that runs when the Command Center scripts did not load.

## Inventory, in tab order

The target is the Help section (one per page) and the heading inside it (the card). **Safety** says whether the text holds something that must also stay on the card: the queued state, the backup or undo information, an "unverified in the game" note, or a warning about what an action does.

- **KEEP**: today only the disclosure says it, so it has to be added to the card.
- **covered**: the text has safety content and the page already says it elsewhere (named in the last column).
- **no**: explanation or background only.

| # | Tab and page | Card | ID | Drawn when | Help section > heading | Safety | What stays on the card, or where it already is |
|---|---|---|---|---|---|---|---|
| 1 | Player > Character > Overview | Fill | `overview.fill` | always | Character > Fill | covered | Blood-sack fill not checked: its tooltip and the result line. Queued: the tooltip of Fill hydration and the banner |
| 2 | Player > Character > Inventory | Inventory | `inventory.card` | always | Character > Inventory | no | Holds a **Go to Character** button (question 5) |
| 3 | Player > Character > Inventory | Give item | `inventory.give` | always | Character > Give item | no | |
| 4 | Player > Character > Care package | Care packages | `carepackage.list` | always | Character > Care packages | no | Apply shows what it would give before it asks. Holds two live limits (question 1) |
| 5 | Player > Crafting | Crafting recipes | `crafting.card` | always | Crafting > Crafting recipes | no | |
| 6 | Player > Building sets | Building sets | `buildingsets.card` | always | Building sets > Building sets | no | |
| 7 | Player > Customizations | Customizations | `customizations.card` | always | Customizations > Customizations | covered | The game removes an unlock the account does not own: the Unlock and Unlock set questions and the tooltip of Unlock all sets |
| 8 | Player > Skills | Skill point controls | `skills.points` | always | Skills > Skill point controls | no | |
| 9 | Player > Specialization | Advanced: specialization numbers | `specialization.numbers` | a track has no number, or one that is not the game's own | Specialization > Specialization numbers | no | How to put a wrong number right: one click away is enough |
| 10 | Player > Journey | Journey browser | `journey.card` | always | Journey > Journey browser | **KEEP** | "Completing a node completes its children; resetting clears it and its children." The row buttons Complete and Reset ask nothing and have no tooltip |
| 11 | Player > Bases > Power, Water | Automatic refill and repair | `bases.autorefill` | always (one card, on two pages) | Bases > Automatic refill and repair | **KEEP** | "The previous file is backed up first." Already there: the Waiting line, the not-run-on-a-copy line, the countdown's Skip, the title of Run now |
| 12 | Player > Bases > Inventory | Inventory | `bases.inventory` | always | Bases > Inventory | no | |
| 13 | Player > Bases > Land claim editor | Land claim editor | `bases.claim` | the base has a claim | Bases > Land claim editor | covered | Backup: the Apply and Remove questions. Staking checks: the line drawn while cells are chosen. The axes and yaw sentence is live (question 1) |
| 14 | Player > Bases > Land claim editor | Resize by square | `bases.resize` | the base has a claim | Bases > Resize by square | **KEEP** | "This editor bypasses the game's normal staking checks. Claiming a protected area does not guarantee that the game lets you build there." Expand claim does not show it, on the card or in its question (the question does name the backup) |
| 15 | Player > Bases > Structures | Structures | `bases.structures` | always | Bases > Structures | no | Scope: the button labels and the tooltips of the repair buttons |
| 16 | Player > Bases (every sub-page) | Base backups | `bases.backups` | always | Bases > Base backups | **KEEP** | Beside Import: "Import then writes it as one edit, after a backup (or queued while the game runs)." Beside Export for console: "whether a game accepts it can only be tried in a game". Already there: own self-hosted server only (tooltip) |
| 17 | Player > Vehicles | Your vehicles | `vehicles.list` | other vehicles are hidden | Vehicles > Your vehicles | no | A live count: keep it as a state line (question 1) |
| 18 | Player > Vehicles | Fuel | `vehicles.fuel` | always | Vehicles > Fuel | no | |
| 19 | Player > Vehicles | Create a vehicle | `vehicles.create` | always | Vehicles > Create a vehicle | **KEEP** | In the open form: "The editor cannot see the terrain or the rocks, so face open ground before you quit to the menu." and "A vehicle made this way has not been loaded in the game yet, so check it on a copy of your save first." Already there: backup and queued, in the tooltip of Create. Holds the game build (question 1) |
| 20 | Player > Vehicles | Components | `vehicles.components` | a vehicle is open | Vehicles > Components | no | |
| 21 | Player > Admin | Movement | `admin.movement` | always | Admin > Movement | **KEEP** | Only here: two trading-post points not seen in use, the Deep Desert taxi spot not tried, and that the move shows when you next enter single-player. Already there: the warning drawn for another map, the tooltip on Z, the heading of the other maps |
| 22 | Saved map | Saved map | `livemap.card` | always | Saved map > Saved map | no | "The save as of its last write" stays on the card today |
| 23 | Saved map | Regions | `livemap.regions` | the map has region names | Saved map > Regions | no | Says a region is a guess; the column is already headed "Near the region" |
| 24 | Landsraad (fallback only) | Term and decrees | `lsb.term` | the Command Center scripts did not load | Term and Decrees (existing page) | no | Question 8 |
| 25 | Landsraad (fallback only) | Earlier terms | `lsb.hist` | the Command Center scripts did not load | Term and Decrees > Earlier terms | no | Question 8 |
| 26 | Config | the chosen file | `config.defaults` | the file has a table of defaults | Config > Config file defaults | no | Holds a source and licence credit; it stays in the third-party notices too |
| 27 | Database > Tables | Tables | `db.tables` | the table has some read-only columns | Database > Tables | no | Each read-only value keeps its title |
| 28 | Database > Save backups | Save backups | `db.backups` | always | Database > Save backups | **KEEP** | "One backup covers the changes made within two minutes." (how far an undo reaches). Already there: backup before changes and restores, and the manual-backup pruning in the tooltip of Back up now |

Totals: **7 KEEP** (10, 11, 14, 16, 19, 21, 28), 3 covered (1, 7, 13), 18 no.

Reputation and Research have no disclosure, so they get no Help section.

## Open design questions

| # | Question | Recommendation |
|---|---|---|
| 1 | Some texts hold live values, and the Help tab reads nothing from the save: the hidden-vehicle count, the claim's axes and yaw, the kit limits, the game build. | Values from the save (count, axes and yaw) stay on the card as state lines. Program constants (kit limits, game build) are written into the Help page, with a test that compares them with the program. Then Your vehicles has nothing left to explain: drop its Help link and its Help heading (the mock still shows both). |
| 2 | Do the seven KEEP texts go on the card, when the decision allows "one click away"? | Yes, next to the button they concern and only while it is on screen. A Help link is easy to miss at the moment of a write. On Vehicles that is 42 words, drawn only in the open Create form, under the limit of 45 for one block. The other six need the same count check. |
| 3 | One long Help page, or one section at a time? | One section at a time (one Markdown file per app page, a heading per card), as the Help tab works today. The link opens the file, scrolls to the heading and gives it the focus. |
| 4 | How does one get back? | A **Back to the ... card** button under the heading, only after arriving from a card; it restores the scroll position and the focus on the Help link. |
| 5 | The Inventory text holds a button, which the Help renderer cannot draw. | Move the sentence and drop the button: it names the page, and the page is two presses away. |
| 6 | Where do today's topic pages go? | Under their tab (three under Landsraad, Specializations inside Specialization), and Checked in the game and the Deep Desert layout under General, as in the mock. |
| 7 | Words such as "in the box above", "this tab", "The same card is on the Power and Water tabs" and "The last card" read wrong in the Help tab. | Move them as they are, as decided, and list them for the shortening pass. |
| 8 | The two Landsraad texts are fallback only, mostly in the Help page already, and one part is out of date (it says the decree id is shown). | Remove them instead of moving them. Check the sentence about the two In Honor of the Fallen names against the Help page first. |
| 9 | The standing lines of #1100 (not disclosures) | Keep them out of this change; this inventory covers disclosures only. |

## Notes for the build

- `app.css` styles every `nav` element as the tab bar, so the section index needs its own rule (the mock has one).
- The page already sets a scroll padding for the sticky header; the heading needs no second offset.
- The Help renderer gives headings no id today; the link needs one per card heading.
- The `about.<id>` keys in the browser's storage go away with the disclosure.
- To update with the change: the README row for **About this**, `docs/help/README.md`, and the web tests that find or open a disclosure.

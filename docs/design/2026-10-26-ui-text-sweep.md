# UI text sweep: what is shown all the time, and what is not (2026-10-26)

Issue [#426](https://github.com/Project-Arrakis/tabr-tau/issues/426). Related: [#212](https://github.com/Project-Arrakis/tabr-tau/issues/212) (the 2026-10-18 UI/UX review, 45 findings) and [#46](https://github.com/Project-Arrakis/tabr-tau/issues/46).
Status: **carried out** (the pull requests are listed under "Plan"; what shipped is under "Choices made while carrying it out"). It lists every line of explanation, limit and background that the editor shows all the time, says what each one is, and decides where it goes.
The note was merged first (#427); the decisions were then carried out in small pull requests, one page at a time.

## Why

Operator report, 2026-10-26: there is more explanatory text on the pages than needs to be shown all the time. The pages carry paragraphs of how it works, what it leaves alone and where the numbers come from, next to
the buttons, on every visit. A reader who wants a button looks past three or four sentences to find it, and the one line that matters (the game is running, this change is queued, this will fail) does not stand out.

## The rule

**A permanent line must earn its place.**

Stays visible, always:

| Kind | Example |
|---|---|
| **State** of a control or of the save | `Hydration is 85.1 of 100`, `12 items`, `When the editor opens: off`, `Nothing to reset: the character has no XP` (the reason a button is off) |
| **Result** of an action | the toasts and result lines, as they read today |
| **Error** | every refusal and every failed check, as it reads today |
| **Queued or pending** | the banner, `N queued`, the `(queued: written when the game closes)` hint, `Pending horizontal` |
| **Time left** | none on this editor today (the Landsraad term shows start and end) |
| **The one thing that can hurt** | the game rewrites config files when it exits; a read-only save; an unconfirmed location; a change that was not written |

Moves, not removed:

| Kind | Where it goes |
|---|---|
| **Explanation** of what a button does, **limit** (caps, what it leaves alone), **background** (where a number comes from, how the game behaves) | a **tooltip** on the one control it is about, or the page's **About this** disclosure |
| A note that applies only sometimes | **conditional**: drawn only while it applies |
| Text that repeats what the label, the heading, the table or another line already says | **delete** |

Nothing that protects the person is deleted. The queued state, the backup and undo information (what the backup is, where Restore is, how to put a value back) and the unverified-in-the-game caveats stay easy to find: either on the page, in the control's own tooltip, or in the question that asks before the action.

## The one mechanism

Two small pieces, built from what the page already has (no new library, no script that draws a floating box):

1. **Tooltip** (`.tip`). The control is wrapped in `<span class="tip">`; next to it is `<span class="tipbox" role="tooltip" id="...">` with one or two sentences, and the control points at it with `aria-describedby`. The box is
   hidden by default, shown by CSS on hover and on keyboard focus (`:hover`, `:focus-within`), so a keyboard user sees it on Tab, and a screen reader reads it as the control's description whether or not it is drawn. This replaces today's
   `<span class="mut" id="...note">` that is already tied to its button with `aria-describedby`: the ids stay, the text stops being drawn all the time. The native `title=` on a control that already has one stays as it is (it is a tooltip too).
2. **About this** (`details.about`). The last element of a card: `<details class="about"><summary>About this</summary>...</details>`, closed by default, opened with Enter or Space (the browser's own disclosure, so it is
   keyboard accessible without code). For the text that is about the card and not about one control: several sentences, a list of what is left alone, where a number comes from. The open or closed choice is remembered per page
   in the browser's storage (`about.<page>.<card>`), read and written inside `try/catch` like the other choices the page keeps (a blocked storage means closed). The redraw after an action already keeps an open `<details>` open.

The choice between them: one control, one or two sentences, and the person is looking at that control: **tooltip**. About the card, longer, or several controls: **About this**. A sentence that is only true sometimes: **conditional**.

Text that is moved is moved word for word unless the decision says "shorten"; the wording is not rewritten in this sweep beyond that. Result and error wording is not touched at all.

### What a test can say

jsdom does not lay out the page, so "visible" has to be defined: the default visible text of a page is its text content without `.tipbox`, without the body of a `<details>` that is not open (its `<summary>` stays), without `[hidden]` and
without the screen-reader-only `.sr`. A helper in `web-tests` computes it, and each page's test says:

* the moved sentences are **not** in the default visible text, and **are** in the tooltip or the closed disclosure (so nothing was lost);
* the state, result and error lines **are** in the default visible text;
* a conditional note is present exactly when its condition holds, and absent when it does not;
* every `.tip` control has `aria-describedby` that points at an existing `.tipbox` with text, and every `details.about` is closed on a fresh load (the accessibility sweep);
* the hostile-data sweep still draws every page from hostile fixtures and finds nothing injected.

## Inventory

Line numbers are those of `internal/web/static/app.js` (and `livemap.js`, `index.html`) at `2fce212`. **Class**: state, result, error, explanation, limit, background. **Decision**: keep, tooltip, disclosure
(About this), conditional, delete. A "keep" row is listed so that the sweep is complete, not because it changes. The editor has five top-level tabs (Player, Saved map, Landsraad, Config, Database) and eleven Player pages;
there are no tabs called Extras, Settings or Help. The settings the editor keeps are the **Automatic refill and repair** card (Player > Bases > Power and Water) and the **Care packages** (Player > Character > Care package), and
the help is this repository's README; both are covered under their pages.

### Header, banners, loading (every page)

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| H1 | index.html:12 | Auto-refresh (box with a title saying what it does) | state | keep (already a tooltip) |
| H2 | index.html:13 | Discard queue (button with a title) | state | keep (already a tooltip) |
| H3 | 419-420 | `N queued`, with the list in its title | queued | keep |
| H4 | 400 | The game is running. Changes you make now are queued and written when the game closes. | queued | keep |
| H5 | 396 | Changes cannot be written here yet. ... You can keep editing: changes wait in the queue. Allow writing here | error, queued | keep |
| H6 | 397 | Changes cannot be written. ... The editor checks again by itself every few seconds. Check again, Write now | error, queued | keep |
| H7 | 398 | N queued changes cannot be written ... Discard the queue | error | keep |
| H8 | 399 | N changes are queued. ... written ... by themselves as soon as it can be written. Write now | queued | keep |
| H9 | 393 | N queued changes were not written. The game saved changes that conflict ... | error | keep |
| H10 | 386, 403-407 | The game saved since this editor opened (auto-refresh off); N changes were put back; queued changes were not kept | state, result, error | keep (conditional already) |
| H11 | 408 | These queued changes are not kept if the editor is closed: reason | the one thing that can hurt | keep (conditional already) |
| H12 | 409, 410 | Schema note; the write log problem | error | keep (conditional already) |
| H13 | 412-413 | What is queued (N); Details (include this if you report a problem) | state | keep (already disclosures) |
| H14 | 427 | This save is read-only. reason. You can browse it, but every edit is refused. This usually means the file contains database objects the real game does not create ... do not edit a save from an untrusted source. | the one thing that can hurt | keep (conditional already) |
| H15 | 1, 1830 | Loading the save...; the editor did not answer; Lost contact with the editor | state, error | keep |
| H16 | 22 | This window is no longer signed in ... | error | keep |
| H17 | locked.html | Tabr Tau is running, but this browser tab is not signed in (3 lines) | error | keep (the whole page is this error) |
| H18 | 495 | A backup of the save is taken before the change is written ... Restore on the Database tab puts it back | background (backup and undo) | keep (inside the reset questions only, not drawn on a page) |

Toasts and the questions that ask first (`ask()`, the two-press buttons) are results, errors and consequences, not permanent text, and are outside this sweep. The two-press ask is kept for what destroys or resets (Clear this container, Clear sand on every base, the config Reset); a top-up (Refill power, Refill base water, Repair every base, Fill chosen on the Landsraad board) does not ask, because it is written as it is made, its result line says what changed and the backup taken first is the undo (#450).

### Player summary (above every Player page)

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| S1 | 539-543 | The tiles: Character, Status, Map, Faction, Level, XP, Skill points, Intel, Health (max N or more), Hydration, Spice addiction, currencies | state | keep |
| S2 | 544, 530 | Account and database ids (closed); Show/Hide with a title saying why they are hidden | background | keep (already a disclosure and a tooltip) |

### Player > Character > Overview

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| O1 | 672 | Map, account, platform and the database ids are in the summary above. | explanation | delete (the summary is directly above and has the `Account and database ids` disclosure) |
| O2 | 673 | Money: A negative number removes. These are the Solari stacks in your backpack. | explanation | tooltip on Add or remove (the label already says `Solari in backpack`) |
| O3 | 674 | A wallet balance, kept apart from the Solari stacks in your backpack. | explanation | tooltip on Give (the choice already names the wallet) |
| O4 | 675 | The currency of the Scourge event (item HalloweenCoin). It is added to the stack in your backpack, and to a new stack when it does not fit; as much as fits is given. | explanation, limit | tooltip on Give Blood Marks (the label already has this as a title) |
| O5 | 676 | Give XP: Raises the level as the XP grows and adds one skill point per level gained; stops at level 200. | explanation, limit | tooltip on Give |
| O6 | 676 | Give Intel: Adds Intel for the research tree, never past 2,779. All of these apply when the game next loads the save. | limit, background | tooltip on Give (cap); the second sentence is true of every edit and is in the banner and the result lines: delete |
| O7 | 590 | Fill hydration: `Hydration is 85.1 of 100.` | state | delete (the summary tile above shows the same number) |
| O8 | 588 | `The summary shows no hydration for the character, so this may be refused.` | error | conditional (already only when there is no hydration) |
| O9 | 590 | `Fills the character's own water to the maximum.` (and the button's title: queued while the game runs, will not help a thirsty character now) | explanation | tooltip on Fill hydration (the title becomes the tooltip, so it shows on focus too) |
| O10 | 571 | Blood sacks: `N of M blood sacks are not full.` / `No blood sacks ...` / `could not be counted` | state | keep |
| O11 | 572 | `N blood sacks of an unknown kind will be left alone.` | limit | conditional (already only when there are some): keep |
| O12 | 573 | Fills the sacks you carry to the most blood seen in a save for that kind (the Mk6 unique sack 30000, the basic sack 3000), without changing anything else about them. Water containers are not touched. The fill has not been checked in the game yet. | explanation, limit (includes the unverified caveat) | tooltip on Fill blood sacks, and the Fill card's About this |
| O13 | 592 | Refill Literjons: Fills the Literjons, stillsuits and other water containers you carry to their capacity. | explanation | tooltip |
| O14 | 581 | Fill all liquids: `Not full: N water containers, N blood sacks, N vehicles.` / `could not be counted` | state | keep |
| O15 | 582 | Fills your Literjons, stillsuits and other water containers, your blood sacks and (with the box ticked) the fuel of your own vehicles, as one edit with one backup, by the same rules as the three buttons (...). Kinds whose size is not known are left alone and counted. | explanation, limit | tooltip on Fill all liquids, and the Fill card's About this |
| O16 | 562 | Reset level and XP, XP above 0: The XP goes back to 0 and the level with it, and the skill points the levels gave are taken back as far as they are unspent. Skills you bought stay, with their points. Asks first. | explanation | tooltip (the question that asks first says it again) |
| O17 | 562 | Reset level and XP, no XP or no level record: `Nothing to reset: the character has no XP.` / `This save has no level record for the character.` | state (why the button is off) | keep |
| O18 | new | The Fill card gets one **About this** that holds O12, O13 and O15 together, and Quick rewards one that holds O5, O6 | | disclosure |

### Player > Character > Inventory

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| I1 | 692 | The fill actions are on the Character tab. Go to Character | explanation (a pointer left by #415) | delete the line and the button; the pointer moves into the Inventory card's About this |
| I2 | 694 | `N items` / `3 of 12 items match`, `N of M slots used`, the liquid containers count | state | keep |
| I3 | 515 | Clear this container, container not empty: Removes the N items in X, with everything attached to them. The Solari Coin stack is kept unless the box is ticked. Two presses, then type the count. | explanation, limit | tooltip on Clear this container (the box `Also remove currency` and the two presses say the rest) |
| I4 | 515 | Clear this container, empty: `X is empty: nothing to clear.` | state (why the button is off) | keep |
| I5 | 704 | Augments card: Holds up to N. Pick what to apply; an augment already on the item keeps its grade, new ones get the grade below. Choose (none) in every slot to remove them all. | limit, explanation | `Holds up to N` is kept as state; the rest is a tooltip on Apply |
| I6 | 709 | Repair: Everything you wear and everything in your loadout is made as good as new: full durability, with a maximum that repairs wore down restored. The backpack is not touched. | explanation, limit | tooltip on Repair all worn items and loadouts |
| I7 | 714 | Give item: Pick an item by its in-game name. Items already in your save that the catalog does not know are listed by their template id; any valid template id can also be typed. | explanation | disclosure (the Give item card's About this; the box's placeholder already says to type a name) |
| I8 | 713 | `Queue (N)` with its items | state | keep |

### Player > Character > Reputation

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| R1 | 723 | Range 0–12474. Edit and press Tab/Enter. | limit, explanation | tooltip on each box (`0 to 12474`); delete `Edit and press Tab/Enter` (the result line says what was written and how to undo it) |

### Player > Character > Care package

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| C1 | 615 | A care package is a named kit of items, XP and skill points. Kits are kept in the editor's own settings, not in a save, so a new save starts with them. Apply gives a kit ... as one edit ... It shows what would be given and what would not fit before it asks. Up to N kits, each with up to M items. A kit file you export is a small text file; one you import is checked first and never replaces a kit you have. | explanation, limit, background | disclosure (the card's About this) |
| C2 | 617 | `The most kits are kept: delete one first.` (only in the title of the disabled New kit) | limit | conditional: drawn as a line while the list is full |
| C3 | 616 | The settings problem (alert) | error | keep |
| C4 | 620 | Cannot be used: problem (per kit) | error | keep |
| C5 | 623 | No kits yet. Make one with New kit, or import a kit file. | state | keep |
| C6 | 628 | Up to N letters, digits, spaces and - _ . , ' ( ) + & # ! | limit | tooltip on the Name box (a refused name keeps saying the rule in its error) |
| C7 | 636 | XP raises the level and adds a skill point per level gained, as Give XP does. Skill points are added on top. | explanation | tooltip on the XP box |
| C8 | 633 | No items yet. (editor table) | state | keep |

### Player > Crafting, Research, Building sets, Customizations

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| CF1 | 764 | Crafting: Adds every recipe your research tree offers that you do not know yet. | explanation | tooltip on Unlock all recipes |
| CF2 | 889 | `N recipes; K of T known.` | state | keep |
| CF3 | 889 | The recipes listed are the ones you know plus the ones your research tree can still give. | explanation | disclosure (Crafting card's About this) |
| RS1 | 761 | Research: Buys every entry that unlocks a recipe or a building set and adds what it unlocks. | explanation | tooltip on Unlock all research |
| RS2 | 873 | `N entries; D of T entries unlocked in all.` | state | keep |
| RS3 | 873 | Unlocking adds the linked recipe or building set; group markers cannot be unlocked as one entry. | explanation | delete (the Group button is disabled with its own title, and Unlock all's tooltip says what unlocking adds) |
| RS4 | 874 | Reset research, entries to reset: Puts the research back to what a new character has: every bought entry except the four ... Intel is not given back. | explanation, limit | tooltip (the question that asks first says it again) |
| RS5 | 874 | Reset research, nothing: `Nothing to reset: no entry beyond the four a new character starts with is bought.` | state | keep |
| BS1 | 769 | Building sets: Teaches every set below except the experimental ones, without putting the items in your backpack. | explanation, limit | tooltip on Unlock all building sets |
| BS2 | 913 | `N items` | state | keep |
| BS3 | 913 | Give adds the item to your backpack. | explanation | disclosure (the card's About this); the Give buttons are labelled |
| BS4 | 770 | New buildable pieces (N) with the pieces | state | keep |
| CU1 | 776 | Customizations: Adds the cosmetics of the five sets to your character's customization library (N unlocked now): no backpack space needed. Store and DLC cosmetics are skipped, because the game removes the ones your account does not own. | explanation, limit, state | `(N unlocked now)` stays as state; the rest is a tooltip on Unlock all sets, and the card's About this |
| CU2 | 776 | This save has no customization library for the character, so cosmetics can only be given as items. | state | keep (conditional already) |
| CU3 | 777 | Puts one cosmetic item of each in your backpack instead, all or nothing: there are more of them than the backpack holds, so give one set at a time. | explanation, limit | tooltip on Give all sets as items |
| CU4 | 909 | `N cosmetics · requirement` per set | state | keep |
| CU5 | 913 | Give adds the item ... Unlock adds the cosmetic to your customization library without an item. Neither gives a DLC or an account entitlement: the game removed a store cosmetic that was unlocked this way (a plain dye pack stayed), so set unlocks skip those. | explanation, limit | disclosure (the card's About this) |

### Player > Skills, Specialization, Journey

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| SK1 | 755 | The save does not keep spent plus unspent equal to the total (some skills are granted free), so these three numbers are shown as stored and not worked out from each other. | explanation, background | disclosure (the Skill point controls card's About this) |
| SK2 | 847 | `N skills, K learned.` | state | keep |
| SK3 | 849 | Reset all skills, can reset: Every skill of the five schools that has points goes back to rank 0, in one edit, after you type a confirmation. Hidden skills and skills the catalog does not know are left alone. | explanation, limit | tooltip |
| SK4 | 849 | Reset all skills, nothing: `Nothing to reset: every skill of the five schools is already at rank 0.` | state | keep |
| SP1 | 731 | Reset XP of all known tracks, can reset: Removes the XP of every track whose number is known (...), in one edit, after you type a confirmation. Keystones are not touched. | explanation, limit | tooltip |
| SP2 | 731 | Nothing to reset: no track whose number is known has XP. | state | keep |
| SP3 | 731 | `N rows of the save whose track is not known yet are left alone.` | limit (a count) | keep (conditional already) |
| SP4 | 732 | No track has a number in this save yet, so the XP controls below are off. Earn a little XP ... (3 sentences) | state (why controls are off), explanation | conditional (already); **shorten** to two sentences: what is off and the one step |
| SP5 | 733 | Changes are written to the game file as you make them and show in the game after it loads the save. Max XP is N. | background, limit | delete the first sentence (true of every page; the banner and the results say it); `Max XP is N` becomes a tooltip on Add XP and Grant max |
| SP6 | 739 | Track numbers: The game's own database names its specialization tracks. Your save stores only a number ... (90 words) | explanation, background | disclosure (the Track numbers card's About this); the table of rows waiting stays |
| SP7 | 741 | No specialization XP in your save is waiting to be assigned. | state | keep |
| SP8 | 742 | Known a track's number already? (label of the manual row) | label | keep |
| SP9 | 743 | `Assigned: Combat = 3, ...` | state | keep |
| SP10 | 744 | Advanced: set a raw row (closed): Writes the XP and level you type for a track number directly. A wrong number changes another track. | the one thing that can hurt | keep (already a closed disclosure, and the sentence is the warning inside it) |
| J1 | 819 | Reset journey, can reset: Every finished Story and Codex step goes back to not finished and every started or finished tutorial back to not started, in one edit, after you type a confirmation. Waiting rewards, tags and contracts are not touched, and the 95 steps and 4 tutorials a new character already has finished are left alone. | explanation, limit | tooltip |
| J2 | 819 | Nothing to reset: no Story or Codex step is finished and no tutorial is started. | state | keep |
| J3 | 821 | `N nodes`, `the first 400 shown: narrow the filter to see others` | state, limit | keep (the second part is conditional already) |
| J4 | 821 | Contracts are shown read-only; a contract is complete when all its tags are on the character. | limit | conditional (already, only on the Contract list): keep |
| J5 | 821 | Completing a node completes its children; resetting clears it and its children. | explanation | disclosure (the Journey browser card's About this) |
| J6 | 749 | Player tags: `none` | state | keep |

### Player > Bases

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| B1 | 1046-1048 | The bases table; `Picked up` tag with a title; `No bases yet: build one in the game, then reload.` | state | keep |
| B2 | 1056 | `No generators, wind turbines or windtraps built at this base.` | state | keep |
| B3 | 1057 | Refill power: Tops up Fuel-Powered and Spice-Powered Generators (499), Omnidirectional Wind Turbines (Low-grade Lubricant) and Directional ... five stacks of 100 and 499 in all ... Windtraps and Large Windtraps (5 filters ...). Buildings that are only placeholders are left alone. | explanation, limit | tooltip on the button (the label says `every base`; the button runs on the first press and the result line says what was refilled, #450), and the card's About this |
| B4 | 1062 | `No water storage at this base.` | state | keep |
| B5 | 1063 | Refill base water: Fills every Water Cistern, ... Blood Purifier to its capacity. The blood in a Blood Purifier is not touched, and a building that is only a placeholder is left alone. | explanation, limit | tooltip, and the card's About this |
| B6 | 1072 | Slots used is the items held out of the container's capacity. Only containers that belong to this base are listed. | explanation, limit | delete the first sentence (the column reads `5 of 20`); the second is in the card's About this |
| B7 | 1078 | These two act on all of your bases, not only the one open above. Repair sets each piece to the highest health seen for its type. | limit (scope), explanation | the scope stays in the buttons (`Repair every base to max`, `Clear sand on every base`; Repair runs on the first press and the result line says how many pieces, #450; Clear sand keeps its two-press ask); the second sentence is a tooltip on Repair |
| B8 | 1085 | Resize by square: `N cells (3 x 3 square), vertical level L` per claim | state | keep |
| B9 | 1089 | `N pieces inside the claim, M outside.` | state | keep |
| B10 | 1091 | Cells that hold building pieces are never removed. | limit | tooltip on Shrink claim |
| B11 | 1092 | One cell is 10 x 10 foundations, around the totem. Expanding only adds cells and never removes them, and the vertical level can only go up. Applied when the game next loads the save. | explanation, limit | disclosure (the Land claim editor's About this) |
| B12 | 1094 | This base has no land claim data in the save. | state | keep |
| B13 | 1016 | Click a dotted cell to add it. New cells must stay connected edge to edge. A number is how many building pieces a cell holds. | explanation | disclosure |
| B14 | 1017 | North is up. Column +x runs East, row +y runs South (the base's yaw is N°). | explanation (the grid has a compass and axis arrows) | disclosure |
| B15 | 1021 | Legend: Totem, Existing, New, Available, To remove | state | keep |
| B16 | 1022 | World-oriented view: north stays at the top and the grid is turned by the base's original yaw; the saved cell coordinates are not changed. | background | disclosure |
| B17 | 1025 | Vertical expansion grows equally up and down. The game caps it at level N. | explanation, limit | tooltip on the level choice (the tile `Vertical level a / b` already shows the cap) |
| B18 | 1026-1028 | Pending horizontal, Pending vertical, Marked for removal | queued/pending | keep |
| B19 | 1029 | Removing those would cut N cells ... | error | keep (conditional already) |
| B20 | 1031 | Click an existing cell to mark it. A cell with building pieces or placeables in it ... cannot be removed, and neither can the totem's own cell. | explanation, limit | disclosure |
| B21 | 1032 | The edit is written at once, after the usual backup. Applied when the game next loads the save. | background (true of every edit) | delete (it is in the banner and the result line) |
| B22 | 1033 | This editor bypasses the game's normal staking checks. Claiming a protected area does not guarantee that the game lets you build there. | the one thing that can hurt | conditional: drawn while cells are chosen to be added (the grid) or a size is chosen to grow to, next to Apply changes and Expand claim; also in the About this |

**Automatic refill and repair card** (shown on both the Power and the Water page; #212 finding 36 asks for one home, which is a separate change):

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| A1 | 926 | When the editor opens: on / off, and Turn on / Turn off | state | keep |
| A2 | 927 | One line under each of the eight jobs (`Fills Water Cisterns, Windtraps and Blood Purifiers to capacity.` ...) | explanation | tooltip on each job's box (the label already names the job); the `aria-describedby` ids stay |
| A3 | 927 | `Not available yet: reason.` on a job that cannot run | state, error | keep (conditional already) |
| A4 | 928 | Last automatic save: ... | result | keep |
| A5 | 929, 930 | The last problem; the settings problem | error | keep |
| A6 | 931 | When the master switch is on, opening the editor runs the jobs ticked above as one edit and saves straight away, like every other change, so it applies the next time the game loads. Every job is ticked by default, the repairs too; the master switch is off until you turn it on. It acts only on what your character owns or carries ... It waits while the game is running: nothing is written or queued, and it runs once the game has closed. When nothing needs doing nothing is written. If one job fails, nothing is changed and the error is shown here. The previous file is backed up first. Remembered between runs. The same card is on the Power and Water tabs. | explanation, limit, background (includes the backup and the waiting-for-the-game state) | disclosure (the card's About this, whole); and **one conditional line** is added for the queued state: while the switch is on and the game is running, `Waiting for the game to close; it runs once the game has closed.` |

### Player > Vehicles

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| V1 | 1123-1126 | The table; the `stored` tag and the disabled Bring/Duplicate titles; `You have no vehicles in this save yet.` | state | keep |
| V2 | 1128 | Makes every module as good as new: full durability, and the maximum that repairs at a workbench wore down is restored too. 100 repairs every module that is not already as good as new; a lower number repairs only the modules below that share, and says when there were none. A module whose full durability is not known is raised to the maximum it records, or skipped when it records none. | explanation, limit | tooltip on Repair all my vehicles and the card's About this (the box `Only modules below N %` is the control and stays) |
| V3 | 1112 | Refuel: `N of M vehicles are not full.` / `You have no vehicles ...` / `could not be counted` | state | keep |
| V4 | 1113, 1114 | `N vehicles of an unknown kind will be left alone.` / `have no fuel record` | limit | keep (conditional already) |
| V5 | 1115 | Fills the fuel of your own vehicles to a full tank, never lowering ... The tank sizes were measured in the game (...): Sandbike 100, Buggy 240, ... A vehicle of any other kind is left alone and counted. Vehicles of other owners and the world's vehicles are not touched. | explanation, limit, background | tooltip on Refuel all my vehicles (what it does, what it leaves alone), the tank sizes in the Fuel card's About this |
| V6 | 1133 | Condition is each module's durability against its current maximum ... The fuel tank size is not stored in the save, so the Fuel column is the current amount; the tank sizes the Refuel button uses were measured in the game. | explanation, background | disclosure (the Components card's About this) |
| V7 | 1136, 1138, 1139 | Cargo hold: `N of M slots used`, `The cargo hold is empty.`, `This vehicle has no cargo hold.` | state | keep |
| V8 | 1142 | N other vehicles in the world (not yours) are not shown. | background | disclosure (the Your vehicles card's About this) |
| V9 | 1143 | None of your vehicles are in this save yet. (under the table that already says `You have no vehicles in this save yet.`) | state (duplicate) | delete |

### Player > Admin

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| AD1 | 782 | Admin actions: Faction, teleport and the vendor purchase limits. Repair is on Character > Inventory (gear) and on Vehicles. | explanation | delete the card (the page's cards have their own headings; the Repair card below has the two links) |
| AD2 | 783 | Faction assignment: Current: name | state | keep |
| AD3 | 785 | Changes which faction the character belongs to. Reputation is under Character > Reputation. | explanation | tooltip on Change faction |
| AD4 | 786 | Repair: the two buttons that open Character > Inventory and Vehicles | state (controls) | keep |
| AD5 | 789 | Moves the character in the save. The game keeps its own copy of the save in memory while single-player runs and overwrites the file, so the move shows when you next enter single-player. | background | disclosure (the Movement card's About this) |
| AD6 | 789 | Choosing a place fills X, Y and Z. | explanation | tooltip on Find |
| AD7 | 789 | Places are on the map chosen in Map (the one the character is on now unless you change it). | explanation | tooltip on the Map choice |
| AD8 | 789 | another map is a move between maps, which writes the character's controller, body and player state on the new map and has not been checked in the game yet, so try it on a copy first. | the one thing that can hurt (unverified) | conditional: drawn while a map other than this one is chosen, next to Teleport; also in the About this |
| AD9 | 789 | A position typed for the Deep Desert is not accepted for Hagga Basin. | limit | conditional with AD8 (only on a map change); the refusal says it too |
| AD10 | 789 | Facing is optional: 0 looks along +X; leave it empty to keep the way the character faces. | explanation | tooltip on Facing |
| AD11 | 789 | If you type a Z yourself, stay above the terrain or you may fall through. | the one thing that can hurt | tooltip on Z (shown on focus), and the About this |
| AD12 | 793 | Vendors limit how much you can buy per restock cycle. Resetting clears those counters. | explanation | tooltip on Reset all vendors |
| AD13 | 794, 795 | `No purchases are counted against any vendor, so there is nothing to reset.` / `No vendor has been visited yet.` | state | keep |

### Saved map

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| M1 | livemap.js:364 | The save as of its last write, not the running game. Markers you have not discovered yet are shown faded. | background | keep the first sentence (a map that looks live is the thing that misleads); the second becomes a tooltip on `Show undiscovered markers` |
| M2 | livemap.js:365 | N of your vehicles are stored ... and not shown: the game keeps only the place a stored vehicle was last in the world. | state, background | keep the count (conditional already); the reason after the colon becomes a tooltip on the line |
| M3 | livemap.js:180, 367 | Hover a marker for details. Drag to move, scroll or press + and - to zoom, 0 to fit. (the idle text of the hover line) | explanation | tooltip on the map (its keyboard help is already in its `aria-label`); the hover line is empty until a marker is hovered and keeps its place |
| M4 | livemap.js:368 | Spice and flour sand fields are placed by decoding their ids, because the save keeps no coordinates for them; the positions have not been checked against the game for this map. | limit (unverified) | tooltip on the Spice and the Flour layer buttons |
| M5 | livemap.js:344 | Regions: No region of this map is marked as discovered in the save. / and N more without a name. | state | keep |
| M6 | livemap.js:346 | The save keeps the region of a map marker, not of a vehicle or a base. A place is shown near the region of the nearest marker; close to the edge of two regions this can be the neighbour. The names are the game's own, for the Hagga Basin (see the notices); the Deep Desert has none. | explanation, background | disclosure (the Regions card's About this) |

### Landsraad

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| L1 | 1225 | Two different scales are shown for each house. House goal (faction) is ... The reward tiers are set by a player's own contribution ... Your contribution is the row ... A tier is reached when ... "recorded as delivered" means ... Fill to this tier raises your contribution only ... Whether the game hands out a tier when it loads the save after a fill is not known from a save. | explanation, limit, background (includes an unverified caveat) | disclosure (the Task board card's About this), whole, word for word |
| L2 | 1225 | Your character is in no faction (Neutral), so there is nothing to fill: assign Atreides or Harkonnen under Admin first. | state (why Fill is off) | conditional (already, only when the character has no faction): keep, drawn as its own line |
| L3 | 1227 | `Nothing is chosen. Choose houses (all their tiers) or single tiers, then Fill chosen.` / `Chosen: N houses ... Fill chosen would raise ... on N houses (the others are already ...)` | state | the second form stays; the first is shortened to `Nothing is chosen.` |
| L4 | 1228 | Faction [choice] used by the buttons below. Fill faction goal sets that faction's progress to the house goal (it does not set a personal contribution or deliver a reward); Complete marks the house taken. | explanation | the row keeps `Faction for Fill faction goal and Complete`; the rest is a tooltip on each of the two buttons (Fill faction goal already has it as a title) |
| L5 | 1207 | Each tier line: At T contributed: A x item - reached, recorded as delivered / reached, no delivery recorded in this term / not reached | state | keep |
| L6 | 1220-1223 | The term tiles, the decree and reigning choices, the decree pool and its state | state | keep |
| L7 | landsraadView | (#451) Chosen-house summary: ` Complete chosen would complete N houses for <faction>. Reopen chosen would reopen N houses.` after the Fill chosen sentence | state (what the two buttons would do) | keep: the count the buttons will send, in the polite summary line that already exists |
| L8 | landsraadView | (#451) `No faction is chosen for you: your character is in no faction. Check the faction in the list before you use it.` beside the faction list | state (why the list shows a faction nobody chose) | conditional: drawn only when the character has no faction; the list describes itself by it |
| L9 | landsraadView | (#451) Tooltips of Complete chosen and Reopen chosen (what each changes and leaves alone, that no question is asked), and a paragraph in the Task board's About this | explanation | tooltip on each button; the paragraph in About this |

### Config

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| CG1 | 1254 | The config folder is missing: error and how to pass it | error | keep |
| CG2 | 1261 | Config check: file not found / missing sections, Create / Add sections, `Missing files are normally created by the game on first run or when a setting is changed in-game.` | error | keep the check; delete the closing sentence |
| CG3 | 1262 | `✓ All expected config files present: ServerCustomSettings.ini, Game.ini, ...` | state (success) | shorten to `All expected config files are present.` (the list is the file choice below) |
| CG4 | 1263 | The game rewrites its config files when it exits. Close Dune before saving changes here. Each save keeps a backup in a tabr-tau-backups folder next to the file. | the one thing that can hurt; backup information | keep, as one line: the rewrite warning plus the backup, which points at the Backups list under the file |
| CG5 | 1249 | N values differ from the default and would change. M values are already at the default; K entries have no known default and are left as they are. | state | keep |
| CG6 | 1249 | The defaults are the Dune Docker console's table for the game's settings (MIT, RedBlink); a setting without a verified default is never reset. | background (the source and its licence), limit | disclosure (the file card's About this); the credit also stays in the third-party notices |
| CG7 | 1240 | `default unknown` and the disabled Reset | state | keep |
| CG8 | 1275, 1278 | `This file is empty.`; `No backups of this file yet. One is made before a change is written.` | state | keep |

### Database

| ID | Where | Text | Class | Decision |
|---|---|---|---|---|
| D1 | 1297 | `N rows, showing a–b` / `No rows in this table` / `No rows match the search` | state | keep |
| D2 | 1297 | Read-only: reason (whole table) | state | keep (conditional already) |
| D3 | 1297 | Plain text columns are read-only (primary keys, links to other tables and account identity); hover for the reason. | explanation | disclosure (the Tables page's About this); each read-only cell keeps its title |
| D4 | 1305 | SQL: Write mode runs one or more statements atomically. They are written to game.db at once, after a backup. If the file cannot be written (for example the game is running) they are queued and written by themselves when it can be. | explanation, background | tooltip on Run as write, and the SQL card's About this |
| D5 | 1500+ (sql results) | `N rows`, `Only the first 1,000 rows are shown: add LIMIT or a WHERE to see others.`; `N rows changed, and written / queued` | result, limit | keep (the results) |
| D6 | 1308 | A backup of game.db is taken before changes are written (one backup covers the changes made within two minutes) and before every restore. Restore requires the game to be closed. | backup and undo information | keep, shortened to one line: `A backup is taken before changes are written and before every restore. Restore needs the game to be closed.` |
| D7 | 1308 | `No backups yet. One is made before the first change is written.` | state | keep |

## Counts

The inventory has 168 rows (O18 is a note about grouping, not a line). By decision:

| Decision | Rows | What it is |
|---|---|---|
| keep | 84 | state, results, errors, queued and pending, the things that can hurt, and text that is already a tooltip, a closed disclosure or drawn only when it applies |
| tooltip | 43 | one control's explanation or limit, shown on hover and on keyboard focus |
| disclosure (About this) | 21 | a card's explanation and background, closed until opened |
| conditional | 9 | drawn only while it applies |
| delete | 9 | repeats the label, the heading, the table or the banner |
| shorten | 2 | a state line made shorter (L3, CG3) |

So 84 of the 168 are moved out of the default view, drawn only when they apply, shortened or removed; the other 84 are the lines the rule names. The end report repeats the table with the final numbers, and notes any row that changed while it was carried out.

## What stays where it is (and why)

* The **banner** and the header (`N queued`, Discard queue, Write now, Check again, the read-only and unconfirmed-location states) are state; they are not touched.
* Every **result** and **error** line, including the hydration message that says a queued fill will not help a thirsty character now, reads as it does today. The sweep does not reword them.
* The **questions that ask first** keep their consequence text (they are shown only when asked), including the backup and undo note of the reset questions.
* The **empty-state** lines (`No kits yet`, `Nothing to reset: ...`) are state: they say why a control is off.

## Fold-in: #212 and #46

| Finding of #212 (2026-10-18 review) | Overlap with this sweep |
|---|---|
| 12, Specialization is a dead end, explained in a 90-word paragraph | SP4, SP5 and SP6 here: shortened, moved, and the raw row stays a closed disclosure |
| 18, "the console" appears in visible text | one visible mention is left (CG6); it moves into a disclosure |
| 21, the safety net is hard to find | the backup and undo information is kept visible (CG4, D6, H18) and is not moved into a disclosure |
| 36, the Automatic refill card is on two pages | not done here (a layout change); A1-A6 are decided for the card wherever it is drawn |
| 43, Give item mixes Give and top-ups | already split by #415 (the Fill card); I1 removes the pointer left behind |
| 45, wording nits | not touched (wording of results and errors stays) |

[#46](https://github.com/Project-Arrakis/tabr-tau/issues/46) (a second window opens when a command runs) is not about text, has no overlap, and stays open as a manual check on a real desktop. A comment says so.

## Plan: one page at a time

Each step is its own branch and pull request, referencing #426, with tests written first, a CHANGELOG line and the check below it. A step merges only when every required check is green.

| # | Pull request | Pages | IDs |
|---|---|---|---|
| 1 (#428) | The mechanism: `.tip`, `details.about`, the remembered open state, the helper that computes the default visible text, the accessibility checks; first user: Character > Overview | Character > Overview | H18, O1-O18 |
| 2 (#430) | Character > Inventory, Reputation, Care package | Character | I1-I8, R1, C1-C8 |
| 3 (#431) | Crafting, Research, Building sets, Customizations, Skills, Specialization, Journey | Player pages | CF, RS, BS, CU, SK, SP, J |
| 4 (#432) | Bases, with the Land claim editor and the Automatic refill and repair card | Player > Bases | B1-B22, A1-A6 |
| 5 (#433) | Vehicles | Player > Vehicles | V1-V9 |
| 6 (#434) | Admin | Player > Admin | AD1-AD13 |
| 7 (#435) | Saved map and Landsraad | Saved map, Landsraad | M1-M6, L1-L6 |
| 8 (#436) | Config and Database | Config, Database | CG1-CG8, D1-D7 |

The README gets a short paragraph where the pages are described (what a tooltip and "About this" are, and that the queued state and the backup information stay in view). The repository has no UAT table; the by-eye checks are listed per
step in the pull request and again in the end report.

## Choices made while carrying it out

The decisions above stand; where carrying one out showed a better way, the way is written here (and in the pull request of the step), so the table is not rewritten.

* **One place for a sentence.** Where the table said "tooltip and About this" for the same words (Fill blood sacks, Refill power, Refill base water), the words are in the tooltip only, and the About this holds only the part that is about the card; the caveat that a blood-sack fill is not checked in the game yet is the one sentence that is in both, on purpose.
* **The blood-sack caveat is also in the result** (CR-4 of the 2026-10-27 review): the fill does not ask first, so the sentence that it has not been checked in the game yet is added to the result line of a fill that changed a sack, as well as being in the tooltip and About this. **The reason a control is off is text** (CR-5): a disabled button cannot take the keyboard focus, so a tooltip alone never reaches the keyboard; where a reason was only a `title` (New kit, Import and Edit while a kit is edited, the specialization rows with no track number, a stored vehicle, a Group marker in Research, the config Reset buttons) it is now a short line next to the control, and the control is described by it. The `title` stays for the pointer.
* **Quick rewards** has tooltips on Give XP and Give Intel and no About this (O18).
* **Many boxes in a table** (Reputation) carry their range as the native `title` of the box, which is a tooltip too, instead of a tooltip box for each row.
* **A reset button** keeps the id of its explanation: when the reset can run, that id is the tooltip; when it cannot, the same id is the state line that says why. The button is described by it either way.
* **The Automatic refill card** shows a new conditional line, "Waiting: the save cannot be written yet (see the banner above). The automatic pass runs once the game has closed.", while the master switch is on and the save cannot be written: the fact that the pass waits for the game was one sentence of the paragraph that is now About this, and it is a pending state, so it is drawn where it applies.
* **The staking-check warning** of the Land claim editor is drawn next to Apply while cells are chosen to be added (and is always in About this, with the Resize by square card's), not only in About this.
* **Several "tooltip on the line" decisions became About this**, because a line of text cannot take the keyboard focus that opens a tooltip: the reason a stored vehicle is not on the Saved map, the idle hint of the map, and the Vehicles line about the world's other vehicles.
* **The move between maps** is a separate note under the Teleport row (`#tmapwarn`), drawn while a map other than this one is chosen; the same words are in the Movement card's About this.
* **About this is the last thing in its card**, which a test checks on every page; on the Landsraad the explanation therefore sits under the table, not above it.
* **Complete chosen and Reopen chosen** (#451) ask nothing: the result line (how many houses were done, how many were left alone, and for Reopen that faction progress and contributions were not changed) and the automatic backup are the way back. The faction list now starts on the character's own faction and is labelled "Faction for Fill faction goal, Complete and Complete chosen".
* **Fill faction goal** keeps the sentence it already had as its tooltip (it said the same as the line that was removed); Complete gained "Marks the house taken, for the chosen faction."
* **The Database backups line** keeps the backup facts in one sentence; the two-minute detail is in About this. The config warning points at the Backups list below it.
* **`tip` and `about`** are plain functions, so the map's script (a separate file) can use them.
* **A bug found by the sweep** (the Journey lists could not be switched before the filter was used) is #429, fixed in the step that touched that code, with its own test.

## What shipped, in numbers

The code has 41 tooltip sites (39 in `app.js`, 2 in `livemap.js`; the job descriptions, the shrink buttons and the layer buttons are drawn from a list) and 24 closed **About this** disclosures, each remembered per card. Beyond the 168 rows: 33 web tests were added (`uitext*.test.mjs`: the mechanism on every page, then one file per step), and existing tests in 7 files (fillcard, liquids, landsraad-hydration, reset, autojobs, refuel, ui.hostile) were changed on purpose to read a tooltip, an About this or a shorter line instead of a permanent note, and one bug was found and fixed (#429).

## By-eye check (what the operator looks at)

Open each page once with About this closed: it should show state, counts, results and what is queued, and each button should show its sentence when the pointer is over it or it has the focus (Tab), and lose it on Escape.

* Player > Character: Overview (Fill card and its About this, Reset level and XP), Inventory, Reputation, Care package (open the kit editor)
* Player: Crafting, Research, Building sets, Customizations, Skills, Specialization (try a save with no known track), Journey (switch the lists first)
* Player > Bases: Power, Water, Inventory, Land claim editor (choose a cell), Structures; the Automatic refill card with the game running
* Player > Vehicles, Player > Admin (choose The Deep Desert in Map)
* Saved map (hover a marker; Spice and Flour layer buttons), Landsraad (Task board About this)
* Config (a file with defaults), Database (Tables, SQL, Save backups)

## Not in this sweep

* Reordering or merging cards (#212 findings 16, 35, 36), renaming, the theme, internal ids in columns (42): separate changes.
* Wording of results, errors and the questions that ask first.
* The write path, the single write gate and the queue: untouched.

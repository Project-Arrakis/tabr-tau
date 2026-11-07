# Bug validation checklist, 2026-11-07 (editor)

What is left to check by hand for the editor's open bugs: every fix below is merged and has tests, and none of the tests shows what the screen or the game does. Short steps, grouped by what can be done in one sitting, highest priority first.

How to use: do the sittings in order (1 to 3). Tick PASS or FAIL for each item. A FAIL goes to the issue named in the item: one comment with the words the page showed and what you did. Do not attach saves, `Game.ini`, snapshots, logs or screenshots that show account names or ids.

Built from main at 32543e7 or later. Rules for every item: work on a **copy of a save** unless the item says the game's own save (the header then says **NOT LIVE: a copy**), keep the editor's backups, and do not repeat a write more than twice.

## Summary

| Order | Sitting | Items (priority) | About |
|---|---|---|---|
| 1 | Editor teleport (game closed, at its menu, then in a world) | #506 P1, #501 P1, #516 P1 | 15 min |
| 2 | Editor base and vehicles (editor, then the game to look) | #538 P1, #480 P1, #540 P2, #483 P2, #469 P2 | 35 min |
| 3 | Editor jobs (saving rules, automatic refill, fills, Landsraad) | #508 P2, #499 P2, #498 P2, #470 P2, #482 P2, #465 P2 | 35 min |
| | Total | 14 items | about 1 hour 25 min |

P1 first: sitting 1 and #538 and #480 of sitting 2 hold all five P1 items.

### Already checked by tests (still needs one in-game or on-screen look)

"Verified by tests, still needs one in-game look". Each item below also names its tests.

| Issue | Checked before you | PR |
|---|---|---|
| #506 | Page tests in the browser engine: the choice and the line "Selected: ..." survive the page's redraws | #507, #520, #522 |
| #501 | One test per state of the game (closed, menu, world running, queued, a copy) | #510 |
| #516 | Tests: the three Deep Desert keys are listed under Taxis and never under Trading posts; the generator maps the object to the taxi kind | #517, #523 |
| #538 | A test that reads every string of every entry finds no `#` id; a strict check of the console's rule runs on every file before it is handed out | #539 |
| #480 | Placement tests with a wall at 8 m, a floor above, a closed ring of pieces, a claim boundary, no clear point | #486 |
| #540 | Placement tests: an aircraft is at least 30 m ahead and 7 m above, for every class and several facings | #541 |
| #483 | Tests with real vehicle rows: no Add item into a hold with no slots, refused by the server, real "N of M slots used" | #487 |
| #469 | Tests for every class and grade against the game's own table | #478 |
| #508 | Tests: a copy gets no automatic pass at launch, the game's own save still does; manual fills still work on a copy | #509 |
| #499 | Tests with fake game states: runs at the menu, waits during a session and in the 30 s after leaving a world, waits when the check cannot be made | #505 |
| #498 | Tests: a copy takes an edit at once while a fake game runs a session, the game's own save still queues; the header test | #504 |
| #470 | Tests with fake logs and file times: 30 s after leaving the world when the game's last write was seen, longer cases unchanged | #471 |
| #482 | Tests for each of the 17 water containers and the 31 worn suits | #484 |
| #465 | Tests on real-shaped rows: the written time is inside the term's window; Reopen unchanged | #479 |

## 1. Editor teleport (about 15 min)

Setup for the sitting: the editor open on your save, Player > Admin > Movement. Items 1.1 and 1.2 are one chain: start with the world running.

### 1.1 #501 Teleport says what happened (P1; PR #510)
- Setup: a world running in the game, the editor open beside it. Choose any place on your own map in the Movement card.
1. Press **Teleport** while the world is running. Read the message.
2. Leave the world to the game's menu, wait until the banner about the queued edit clears (about 30 s), choose a place and press **Teleport** again. Read the message.
3. Press **Continue** in the game.
- Expected: step 1: "Position queued, but the running game keeps its own copy of the save: it will not move you now, and the game may overwrite it when it saves. Leave the world, wait about 30 seconds, then teleport and press Continue." (it stays on screen until dismissed). Step 2: "Position saved. Press Continue to load it." Step 3: you are at the place. With the game closed the message is "Position saved. You will be there when you load this save."; on a copy it is "Position saved to this file. This is a copy, so the game will not see it unless this file is installed as its game.db."
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #501 with the message you saw and the state of the game.

### 1.2 #506 The chosen place stays in the Teleport boxes (P1; PRs #507, #520, #522)
- Setup: the Movement card, the game at its menu or closed. The list is one drop-down, **Place to go to**, filled at once ("N places on <map>, nearest first").
1. Choose the nearest entry. Read X, Y and Z and the line next to the Teleport button.
2. Choose the farthest entry on the same map. Wait 20 seconds without touching the page (it redraws by itself when the file changes).
3. Press **Teleport**, then press **Continue** in the game.
- Expected: each choice puts that place's own numbers in the boxes and shows "Selected: <name>, <distance> away"; the boxes and the line stay after the 20 seconds and after any redraw; you arrive at the farthest place, not where you were. A trading post or a hub arrives at the game's arrival point, not at its marker. Stay on your own map: a move into another map is not proven (the page warns you).
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #506 with the entry you chose and the numbers that stayed or went back.

### 1.3 #516 The Deep Desert keys are taxis, not trading posts (P1; PRs #517, #523)
- Setup: a copy of a save whose character is on the Deep Desert (or pick Deep Desert in the Map box). Open the **Place to go to** drop-down.
1. Look at the headings and the entries under **Taxis** and **Trading posts**.
2. Open the same drop-down for Hagga Basin.
- Expected: Deep Desert: **Taxis** lists "Deep Desert Taxi Service (A2)", "(A5)" and "(A8)"; the name "Deep Desert Tradepost" appears nowhere and there is no Trading posts heading for the Deep Desert. Hagga Basin: Trading posts are Pinnacle Station Tradepost, Crossroads Tradepost, Griffin's Reach Tradepost and The Anvil Tradepost; no Deep Desert taxi is listed there (the Arrakeen and Harko Village taxi hubs are).
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #516 with the list you saw.

## 2. Editor base and vehicles (about 35 min)

Setup for the sitting: the editor on the save (a copy is fine for the editor part; the game must load the same file to look at the result, so for the in-game look use your own save with a backup). The game is closed or at its menu when you write.

### 2.1 #538 The console accepts the exported base file (P1; PR #539)
- Setup: a standing base in the save; a self-hosted Dune console with its Base Backups import.
1. Player > Bases: choose the base and press **Export for console**; save the file.
2. In the console: Base Backups > Import, choose the file. At the game-version warning press **Import Anyway** (the warning is expected: the file carries no game build).
- Expected: the import completes. It does not stop with "Non-transfer id !!itm#0 ..." or any "Non-transfer id" message. If it fails with another message, copy that message exactly.
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #538 with the exact console message and the base's piece count (no file).

### 2.2 #480 A created vehicle is not inside a wall or on another floor (P1; PR #486)
- Setup: your own save, the character standing inside a multi-storey base when you left the world; the game at its menu or closed.
1. Player > Vehicles > Create a vehicle: a Buggy. Read the Preview line, then **Create**.
2. Repeat with a larger vehicle (a Medium or Transport Ornithopter, or a Sand Crawler).
3. Load the game and look for both.
- Expected: the Preview says where the vehicle goes ("outside your base, N m <direction> of you", naming what it moved away from). In the game neither vehicle is inside a wall or on another storey; the larger one has more room round it. If no clear spot is within 80 m the page refuses with a message and writes nothing: that is correct, not a FAIL.
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #480 with the Preview line and where each vehicle was.

### 2.3 #540 An ornithopter is created in the air, never in rock (P2; PR #541)
- Setup: the character standing in the open, once on flat ground and once next to a rock formation; the game closed or at its menu.
1. Create a Light Ornithopter. Read the Preview line.
2. Load the game and find it. Repeat next to the rock formation.
- Expected: the line says where it goes and that it is "7 m above you; it starts in the air and the game settles it", at least 30 m ahead of you. In the game it is above the ground or on it, never inside rock. A ground vehicle is never placed below your height.
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #540 saying which of the two spots failed and what the vehicle did.

### 2.4 #483 No cargo hold means no Add item (P2; PR #487)
- Setup: Player > Vehicles with a Sandbike that has no storage part (and, if you have one, a vehicle with a hold).
1. Open the Sandbike's card.
2. Open the card of a vehicle with a hold; add more of an item than fits.
3. If the Sandbike holds surplus items from before, press **Clear this container** on them.
- Expected: the Sandbike says "This vehicle has no cargo hold (no storage part)." with no Add item row (any surplus is listed with Delete and Clear this container). The vehicle with a hold shows "N of M slots used" (never "10 of 0"), takes items up to its slots and says how many did not fit.
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #483.

### 2.5 #469 Refuel uses the tank of the vehicle's chassis (P2; PR #478)
- Setup: a Mk6 buggy (or any vehicle of a grade the old table got wrong, such as a Mk5 Sandbike) with a low tank; the game closed or at its menu.
1. Player > Vehicles: read the Fuel column of that vehicle.
2. Press **Refuel** (the vehicle's button or the Refuel vehicles job).
3. Load the game and read the gauge.
- Expected: the Fuel column shows the real tank (300 for a Mk6 buggy, 140 for a Mk5 Sandbike, 100 for a Mk1 Sandbike) and never more than 100 %; after the refuel the gauge reads 100 % and the value is the tank (300 for the buggy).
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #469 with the vehicle, its grade and the number you read.

## 3. Editor jobs (about 35 min)

Setup for the sitting: the Automatic refill and repair card (a base's Power or Water tab) has its master switch on and at least one job ticked. Items 3.1 and 3.2 are about a copy; 3.3 and 3.4 about the game's own save and need the game; 3.5 and 3.6 are on-screen.

### 3.1 #498 A copy is shown as a copy and written at once (P2; PR #504)
- Setup: the game running with a world in play. Start the editor on a copy of the save made outside the game's save folder (`--save <path to the copy>`).
1. Read the page header, the tab title and the banner.
2. Make one edit on the copy (for example a Fill).
3. Then open the game's own save in the editor instead and read the header (make no edit to it).
- Expected: the header says **NOT LIVE: a copy** between TABR TAU and the file name, the tab title starts with the same words, and the banner says "This is a copy, not the game's save: edits are written at once and do not reach the game unless this file is installed as its game.db" with the path. The edit on the copy is written at once, with no queue, while the game runs. The game's own save shows nothing that reads as "copy" (an edit to it would queue while the world runs).
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #498.

### 3.2 #508 The automatic refill does not run on a copy (P2; PR #509)
- Setup: as 3.1, with the master switch of the Automatic refill card on.
1. Open the editor on the copy. Watch the top of the page for 20 seconds.
2. Press **Run the automatic pass now**.
3. Press a manual **Fill** or **Refill** button. Then open Database > Save backups.
- Expected: no 15-second countdown; the page says "Automatic refill not run: this is a copy, not the game's save." and the card says "Not run: this is a copy ...". Run now answers "This is a copy, not the game's save: the automatic pass only runs on the game's own save." and writes nothing. The manual Fill still works. Backups of a copy are marked "made from a copy". No text says "to the game" for the copy.
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #508.

### 3.3 #499 The automatic refill at launch uses the same gate as an edit (P2; PR #505)
- Setup: the game's own save, master switch and a job on, the game open at its main menu a minute after leaving a world.
1. Open the editor and let the 15-second countdown end.
2. Leave the world again; within 30 seconds of the exit open the editor (or press **Run the automatic pass now**).
- Expected: step 1: the refill runs and saves with the game still open. Step 2: the page says it is queued and why (for example "a single-player session is active", or the game's last write was seconds ago), not "the game is open; applied when it closes", and it runs by itself when that clears.
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #499 with the wording you saw.

### 3.4 #470 An edit after leaving the world waits about 30 seconds (P2; PR #471)
- Setup: the game's own save, a world running, the editor open.
1. Leave the world to the menu and at once press **Create** (a vehicle) or **Apply** (any edit).
2. Read the waiting text; wait for the write.
3. Load the character. Then make a second edit more than a minute later.
- Expected: the waiting text counts down ("about N seconds more", not 3 minutes); the edit is written about 30 seconds after the exit with no action from you; the character loads with no repair prompt and the edit is there; the second edit is written at once.
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #470 with the seconds you waited.

### 3.5 #482 Every liquid container has a capacity or a reason (P2; PR #484)
- Setup: Player > Character > Inventory with a worn stillsuit and some water containers (any Literjon, a Dewpack).
1. Read the stillsuit's line. Press **Fill water containers**.
2. Load the game and look at a filled container.
- Expected: the suit reads "Water N / 500 (x%), the suit fills itself" (or 1500 for the high-capacity suit), never "capacity unknown", and no fill button touches it. Each water container is filled to the game's capacity; a full one says already full. In the game the container is full.
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #482 with the container name and the number you saw.

### 3.6 #465 Landsraad Complete writes the game's own clock (P2; PR #479)
- Setup: the Landsraad tab with a current term and at least one open house.
1. Press **Complete** on one house.
2. Open Database, browse the table `landsraad_tasks`, find that house's row: read `completion_time`.
3. Load the game and open the Landsraad board. Back in the editor press **Reopen** on the house.
- Expected: `completion_time` is a small number in the same range as the rows the game wrote itself (about six digits), not about 1.79 billion. The board shows the house completed this term with nothing odd. **Reopen** clears the column.
- Result: [ ] PASS  [ ] FAIL
- Report: comment on #465 with the number and what the board showed.

## After the sittings

Tell the maintainers the PASS and FAIL ticks. A PASS lets the issue be closed; a FAIL stays open with your comment.

Last verified / changed: 2026-11-07, written from the issues' "Done when" lines and the merged pull requests; no step here has been walked yet.

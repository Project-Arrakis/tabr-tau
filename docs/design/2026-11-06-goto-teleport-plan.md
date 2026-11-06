# Teleport and Go to a place: plan for the category (2026-11-06)

Scope: the issues about Admin > Movement (Teleport and Go to a place): #506, #501, #516, #521, #67, #500, #519, #515. Written against `main` at the commit that merged #543. Docs only; nothing here changes behaviour.

## Where things stand

Every issue of the category already has its code merged. None is closed, because each waits for the operator's check in the game (the README row "Teleport and Go to a place" carries the check list). Go and browser-engine tests pass on `main` (`go test ./internal/ops ./internal/web ./tools/...`; `web-tests/gotoplace.test.mjs` and `teleportmsg.test.mjs`, 28 tests).

| Issue | What was asked | Merged in | Left |
|---|---|---|---|
| #506 | the chosen place must stay in the X, Y, Z boxes and be named next to Teleport | #507 | in-game check |
| #501 | the teleport message says what happened and what to do, by the state of the game | #510 | in-game check |
| #516 | the three Deep Desert keys are taxi service points, not trading posts | #517 | check in a Deep Desert save (a copy) |
| #521 | Deep Desert taxis from level data; Arrakeen and Harko Village from every map | #523 | check on a copy; the map change is unproven |
| #67 | teleport parity with the console: destinations, map, facing | #140, #395 (earlier), and the items above | three in-game items (below) |
| #500 | trading posts, bases, vehicles, taxis by their in-game names | #512 | in-game check |
| #519 | every kind as a drop-down filled at once, no filter, no Find | #520, #522 | in-game check |
| #515 | the game's own arrival point for each trading post and hub | #518 | in-game check; two points are computed only |

Labels: the issues do not carry `cat:goto-teleport` (the label does not exist in the repository), and #506 is `p2-next`, #501 and #516 have no priority label, #519 and #515 are `inbox`. The priorities below are the operator's order as given in the request; the labels have not been changed.

## Batches, in the operator's order

### Batch 1 (P1): #506, #501, #516

Approach when they were built, and why they hold:

- #506. Cause: the Movement card was drawn from the saved position on every redraw (after an action, by the status tick, by the automatic reload of the file), so a chosen place was dropped. Fix: the page keeps what the person chose or typed (`GO` in `internal/web/static/app.js`) and draws the card from it until a Teleport succeeds; the list of places is redrawn with the chosen place still selected (kept by group and name); typing over a coordinate drops the place name; "Selected: name, distance" is shown next to Teleport. Tests: `web-tests/gotoplace.test.mjs` chooses the nearest and the farthest place in turn and redraws by an action, by a refused change and by the automatic reload.
- #501. Cause: one message for every state. Fix: `teleportMessage` in `internal/web/server.go` chooses the sentence from `save.GameStateFor` and the outcome (written or queued): game closed, game at its menu, world running (queued or written), queued for another reason, a copy of a save. A message that asks the person to wait or act stays on screen until dismissed. Tests: `internal/web/teleport_message_test.go` (one per state) and `web-tests/teleportmsg.test.mjs`.
- #516. Cause: the generator named the rows of the game's static location table that carry the trading-post tag and belong to the map object `DeepDesert_1_Travel` "Deep Desert Tradepost" and put them in the trading-post group. Fix: `tools/placenamesgen` maps that object to the taxi kind, `internal/ops/place-names.json` was regenerated, `internal/catalog/PROVENANCE.md` says these are taxi service points. Tests: `internal/ops/places_test.go`, `teleport_map_test.go`, `tools/placenamesgen/generate_test.go`.

Nothing is left to build in this batch. The batch ends with the operator's check:

1. #506, game closed, on a copy: open Admin > Movement > Go to a place, choose the nearest place, read the three boxes and the "Selected:" line; wait 20 seconds (a status tick and an automatic reload happen); choose the farthest place; the boxes change and stay. Press Teleport. Report the place the character is at when the save loads.
2. #501, with a world running: press Teleport and read the message (it must say the running game will not move you, and what to do, and it stays until dismissed). Leave the world to the menu, press Teleport, read the message, press Continue and say where the character is. With the game closed, press Teleport and read the message.
3. #516, in a Deep Desert save (a copy): Go to a place lists the three taxis by name under Taxis and no Trading posts heading; in a Hagga Basin save the four trading posts are unchanged.

### Batch 2 (P2): #521, #67

- #521 is built (#523): three Deep Desert taxi service points from `internal/ops/deep-desert-taxis.json` (made by `tools/deepdeserttaxisgen` from the game's level data; named by the grid cell, for example "(A2)"; `observed: false`), listed when the chosen map is the Deep Desert whether or not the save holds a marker for them; Arrakeen and Harko Village under Taxis from every map, at their arrival points, with the cross-map warning and the About sentence that a map change by save edit is not proven. The three standing points lie inside row A of the Deep Desert grid recorded in the same file, as the operator's comment required.
- #67 has three items the editor cannot know from a save, recorded in the issue: (1) a move from Hagga Basin to a Deep Desert base and back on a copy, then the position and the map after load; (2) the position still holds after the game's own autosave; (3) whether the game accepts a character whose body is on another map than the one it saved on. One cross-map move to the Deep Desert has been seen working (README "Which write features were checked in the game"). Everything else in the issue's gap list is built: destination picker, facing, offline-only note, map. The console's "to a player" mode has no single-player counterpart and is not planned.

Steps for the operator (game closed, on a copy):

1. Deep Desert save: Go to a place, choose a Deep Desert taxi, Teleport, load the save. Say whether the character stands beside the taxi on the ground.
2. Hagga Basin save: choose Arrakeen, Teleport, load the save; do the same for Harko Village. Say which map the character is on (a hub, Hagga Basin or a failure to load). This is the one place a save edit might not be accepted; if it is refused by the game, the plan is to keep the entries but change the About text to say so, or to offer the hubs only from the taxi (to be decided from the result, not before).
3. For #67: on a copy, Hagga Basin to a Deep Desert base and back; load; read position and map; then play until the game's autosave and read the position again.

### Batch 3 (P3): #500, #519, #515

All three are built (#512; #520 and #522; #518). #515 asks that the operator decides whether a hub is offered only on its own map or with the warning; #521 settled that (hubs from every map with the warning). Open items:

- #500: the display name of The Anvil (the game's table names its object after Mirzabah's Shadow) is "The Anvil Tradepost" with "Mirzabah's Shadow" matched by the filter alias; the page has no filter now, so the alias only serves the route's `q` parameter. Nothing to build; the in-game list is the check.
- #519: the drop-down refills on a map change and each time the card is drawn from the save, which covers "refreshes on its own"; no refresh control was added. If the operator wants one, it is a small follow-up.
- #515: Pinnacle Station, The Anvil, Arrakeen and Harko Village points match trips seen in the logs; Griffin's Reach and Crossroads are computed only.

Steps for the operator (game closed, on a copy): open Go to a place and see one drop-down at once (no kind list, no filter box, no Find) with the headings Trading posts, Personal bases, Vehicles, Respawn points, Taxis; Personal bases lists only deployed bases; Vehicles only own vehicles standing in the world; choose Pinnacle Station Tradepost, Teleport, load, and say whether the character stands on the ground at the post (not inside a building or on a roof). Repeat for Griffin's Reach Tradepost and Crossroads Tradepost and say ground or roof (and for The Anvil Tradepost).

## Shared causes

- One model feeds both the list and the teleport: `namedPlaces` in `internal/ops/places.go` (and `place_taxis.go`) builds the rows, `Destinations` in `internal/ops/teleport.go` sorts and groups them, and `Teleport` re-checks hub moves against the arrival points. A change to a group touches all three of #500, #519, #521.
- Three generated data files carry the game-derived facts: `place-names.json` (names, kinds), `place-arrivals.json` (arrival points) and `deep-desert-taxis.json` (taxi stand points). Each has its generator under `tools/` and a section in `internal/catalog/PROVENANCE.md`. A wrong name or point is fixed in the generator and the file regenerated, never by hand.
- The page keeps its own state for the card (`GO`, `TD`, `TDR`) because the card is redrawn from the save on timers; #506 was this cause, and any new field in the card must be added to `GO_IDS`.
- The unproven premise behind #67, #515 and #521 is that the game accepts a character written to a different map by a save edit. Only the Deep Desert move has been seen.

## What tests can show and what needs the game

Tests (done): the groups and their order, names, arrival points, distances and sort, cross-map flags, the hub refusal for any non-arrival position, the data files, the message per state, the fields through a redraw, the drop-down markup and accessibility labels.

Needs the game (the editor cannot know it from a save): that the character lands on the ground at the named points; that a hub or a Deep Desert taxi spot is accepted by the game; that the position survives the game's autosave; the sentence of #501 as the person reads it.

## Duplicates and obsolete items

- #500 asked for a filter box matching names; #519 removed the filter box and Find. The filter part of #500 is superseded by #519 (the route's `q` stays for old callers).
- #515's wanted item 3 (hub offered only on its own map, or with a warning) is superseded by #521.
- #516 and #521 both concern the Deep Desert taxi keys; #516 fixed the group and the name, #521 added the positions. Not duplicates.
- #67 is the umbrella; its build items are done and it stays open only for the three in-game items.
- No issue is closed by this note. The operator closes each after the check.

## Order of work from here

1. Merge this note.
2. Batch 1 needs no build; the operator's checks above. If a check fails, the cause is investigated from the failing step and fixed in one small PR with a test.
3. Batches 2 and 3 likewise: build only on a failed check or a new ask.

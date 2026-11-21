# Command Center M3: the live board, the House Inspector and edit parity

> Note (2026-11-20, #1028): the Classic Landsraad list described below was retired. The Command Center board is now the only Landsraad view and the default (see DECISIONS.md, section M). Mentions of the classic list, its result line or its switch are the record of that milestone, not the current page.


Milestone M3 of the Landsraad and Specialization Command Center. The Landsraad tab gets a second view: the Command Center board, a fixed five-by-five grid of the 25 Houses with a list layout and a House Inspector, drawn from the same read as the classic list and writing through the same code. The classic list stays, is the default, and is the fallback when the Command Center files are not loaded.

Read with `03-parity-baseline.md` (the L01 to L28 rows this note answers), `05-m2-visual-foundation.md` (the parts and art this milestone uses) and `DECISIONS.md`.

## 1. What was built

| Part | Where |
|---|---|
| The reader: one value per House from `GET /api/landsraad`, validated (25 Houses, `board_index` 0 to 24 and unique), with filters, sort keys, groups and the totals of the same pass | `internal/web/static/lcc/lcc-model.js` (pure: no DOM, no storage, no network) |
| The board, the list layout and the Inspector | `internal/web/static/lcc/lcc-board.js` and `lcc-board.css` |
| The view switch, the host the board asks for sentences and choices, and the single write path | `internal/web/static/app.js` (`lbViewSwitch`, `lbBoardInner`, `lbMountBoard`, `lbCtl`, `lbBoardAsk`) |
| One dial and two tokens for light and outline: `--lcc-glow` (plaque, 0.5), `--lcc-glow-win` (a winning House, 1) and `--lcc-outline-win` | `internal/web/static/lcc/lcc.css`; samples on the gallery page |
| House number 01 to 25 on the tile and on the rows of the classic list (#768) | `lcc-board.js`, `app.js` (`lbRow`) |

### 1.1 How parity is kept

The board has no write code of its own. Every control that changes the save (a tier, Fill, Fill faction goal, Complete, Reopen, the Choose boxes, Choose all, Clear, the three actions on several Houses, the faction drop-down) carries the same `data-lb` attribute and focus key as the classic list's control, so the one click handler, the one request builder (`lbItem`), the one question (`lbAskOf`), the one write function (`lbSend`, `lbRun`) and the one result line serve both views. The board adds only what it shows: the tile, the Inspector, the filters and the sort of its own list.

Writes still go through `POST /api/landsraad/apply` with the stale guard (`term_id` and the `was_*` fields), the write queue and the backup. No route, no schema and no query was added.

## 2. Parity checklist

Status is what a web test proves in a page with a stand-in answer. "Not checked in the game" applies to every row: nothing was run against a real save or a running game.

| Row | Feature | On the board | Proved by (web-tests) |
|---|---|---|---|
| L01 | Tier raise, cumulative | Same request and same result line as the classic list | `lcc-board-write`: parity, tier raise |
| L02 | Remove only the highest tier | `lower:true`, same refusal for a lower tier, same note | parity, removal of the highest reached tier |
| L03 | Fill, with its off states | Same on/off and the same reason text, from `lbQa` | parity, Fill; `another faction chosen` |
| L04 | Fill faction goal | Same request; refused for another faction | parity, Fill faction goal; `another faction chosen` |
| L05 | Complete for the chosen faction | Same request; on a House another faction took the board asks first and names whose win is replaced (the classic list writes at once, unchanged) | parity, Complete; `Complete on a House another faction took asks first`; `the classic list is not changed` |
| L06 | Reopen | Same request and wording | parity, Reopen |
| L07 | Bulk: Choose, Choose all, Clear, Fill, Complete and Reopen chosen | One atomic request; the question names the Houses and says how many are left alone; Choose all chooses only what the board shows | `several Houses` (two tests); `Choose all houses chooses what the board shows` |
| L08 | Stale guard and re-read | A conflict is shown as the save said it and the board is read again | `the stale guard` |
| L09 | One write at a time | A second click sends nothing, says to wait, the board is marked busy | `the write lock` |
| L10 | Written versus queued | Never called saved when queued, in the result line and on the board | `written and queued are said apart` |
| L11 | Grant preview, confirm, result sentence | The Inspector shows what a raise would write; a raise over the limit is shown as refused; the question keeps the classic text | `the grant before the click` |
| L12 | Faction drop-down | Same control, default the character's faction; no standing note beside it (#1045), the hover text on the box says what choosing it does | `another faction chosen` |
| L13 | Reward states per tier | The Inspector ledger tells waiting, collected and delivery unknown apart | `the reward ledger` (`lcc-board`) |
| L14 | Two scales | Faction progress and the player's tiers are two sections | `choosing a House opens the Inspector` |
| L15 L16 L17 | Show, House filter, Jump to House | Dim the Houses they leave out and keep the board fixed; Jump selects and focuses the tile; a House the filter hides is dropped from the choice and a question about it closes | `Show, House and Path dim`; `Jump to House`; `a filter drops the Houses` |
| L18 L19 | Sort and group by region | List layout, sortable by every column both ways, grouped by region with unmapped last; ties by name | `the list: board order by default` |
| L20 | Location and Saved map link | In the list and the Inspector, only where the map has a picture, with the explanation when it has none | `the layout is remembered` |
| L21 | Glance cards, Expand all | Figures above the board replace the glance cards; Expand all does not apply to a tile | no test (not applicable) |
| L22 L23 L24 L25 | Term and decree controls, decree names, term history, `SetTerm` fields | The panels of the classic list are drawn below the board unchanged, the same code; nothing new is sent | the classic list's own tests for these panels, which run unchanged |
| L26 | Result line, announcement | The same `#lsb-result` line with the same wording | parity tests compare it |
| L27 | `board_index`, number 01 to 25 | Read and validated; the number is drawn only from a valid board; a board of 24 Houses or a duplicate position is shown read-only with the reasons | `a board that is not a board`; `a duplicate board position` |
| L28 | Header facts from the real read | Plaque and figures come from the same read and add up | `the plaque and the figures` |

Not parity-complete: the Landsraad tab's classic Expand all and Collapse all (L21, not needed on a tile), and the Command Center board does not draw the specialization banners of M5 or the twelve lines of M4.

## 3. What the board says, and does not say

- There is no vote-start snapshot in the save. A lead in the count is a lead, never a win: the plaque says "VOTE NOT CONFIRMED" and no board text calls a count a win.
- The column `sysselraad` is not read.
- Top contributors are the biggest contributors the save records (#913), worked out here; missions are shown as "Not in the save".
- A House whose owner cannot be read is counted for neither faction and the plaque says how many.
- When the board fails its checks (not 25 Houses, a position outside 0 to 24, a duplicate) it is a read-only list of what was read, with the reasons, no number and no write control. The classic list is the way to change anything then.
- Keystone writes are global only (M5).

## 4. Win tokens

`--lcc-glow-win` (default 1, the full strength of the approved look, not scaled by `--lcc-glow`) and `--lcc-outline-win` (3px solid gold) are defined in `lcc.css`. On the board, when a faction holds more than half of the Houses read (13 or more of 25), the outline is drawn on those Houses; the wording was the House count ("a count of Houses, not a vote result"), never "wins"; since 2026-11-18 (DECISIONS section H) a faction with 13 or more Houses is shown as "SYSSELRAAD WON · <FACTION>, by 13 Houses". M4 draws the five-in-a-line lines and decides what a vote-start majority is.

An optional animated border (class `lcc-win-anim`, a gold arc moved by `transform` only, no script per frame) is added only when Motion is Full and the system has not asked for reduced motion; the style checks the same two conditions again. Calm, Off and reduced motion show the still outline. A web test checks that the class is not added in those cases. The gallery page has a still and an animated sample of each outline.

## 5. Write-path changes

No new endpoint, no new field in the request, no SQL. The changes to code that sits on the write path, all in `app.js`:

1. `lbQa` is extracted from `lbRow` unchanged, so the board's Inspector reads the same on/off and tooltips as the classic row (display only).
2. `lbAskOf` appends `lbBoardAsk(...)` to the question only when the board view is open. The classic question text is byte-for-byte as before.
3. `lbGo` asks before a single click when the board view is open and the click would take a win from another faction. In the classic view a single click still writes at once.
4. `pickall` chooses the Houses the board shows in the board view; `pickno` also clears the board's boxes.
5. `lbApply` returns early (after updating the count of chosen Houses) when there is no classic list on the page.
6. After a filter of the board changes, `lbBoardFiltered` drops Houses no longer shown from the choice and closes a question about them: what is not shown is never written.

## 6. Readiness for M4

The reader returns per House the owner, the board position (row, column, index), the path and the result of the checks, and the board keeps cells addressable by index, so a line evaluator can take the `grid` of `readBoard` without touching the page. `victory(elm, on)` in `lcc.js` is the hook for the one win outline (two outlines until 2026-11-18). Open for M4: the vote-start majority rule (D03), what `sysselraad` means (D02), and the line geometry on the live board.

## 7. Not verified

- Nothing was run against a real save or a running game. Every test and capture uses a stand-in answer.
- The animated border cannot be shown in a headless capture; only the still outline is seen in the captures, and the 13-House outline is covered by a test, not a capture.
- The write results are as the classic list's, which are themselves "not checked in the game yet" for the pay-out of missions.
- Windows high contrast and forced colours are covered by style rules and a style test, not by a visual check.
- In the list layout with the Inspector open the table scrolls sideways at 1280 px.

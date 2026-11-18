# Command Center M4: the Sysselraad evaluator and its display

Milestone M4 of the Landsraad and Specialization Command Center. It adds a pure, read-only reading of the 25 Houses (which lines of five one faction holds, which lines are blocked or one House away, how many lines are still possible, the House counts and the 13-House threshold), and draws what it finds on the board of the Command Center. It changes nothing that is written: no route, no request, no field, no `data-lb` and no `data-fk`, and no Go code.

Read with `DECISIONS.md` (D02, D03, D04, D05, D10a, D16 and D13a, the rules this milestone encodes), `04-fixture-catalog.md` (the board cases the evaluator is checked against) and `06-m3-live-board.md` (the board and the win tokens this milestone draws with).

## 1. What was built

| Part | Where |
|---|---|
| The evaluator: `LINES` (the twelve lines), `lineStatus`, `evaluateOwners(owners, evidence, meta)` and `evaluate(board, evidence)`. Pure: no DOM, no storage, no network, no write | `internal/web/static/lcc/lcc-model.js` |
| The POTENTIAL LINES figure, the TERM STATUS tile for a held line, the outline of a line of five and of a faction with 13 or more Houses, the legend, the Imperial Victory Analysis states, and the folded list of the twelve lines | `internal/web/static/lcc/lcc-board.js`, `lcc-board.css` |

The evaluator is in the existing reader file, not a new file, so no script, route or asset list changed.

## 2. What the evaluator reads, and the rules as implemented

Input: the 25 Houses of a board that passed the reader's checks, in `board_index` order, each with the owner the reader gives it (`a`, `h`, `open`, or unreadable). It never takes the order of a sorted or filtered list. A board that did not pass (not 25 Houses, a position outside 0 to 24, a duplicate, an unknown faction) has no counts, no lines, no number of potential lines and the verdict `unknown`, with the reader's reasons.

The twelve lines, fixed: five rows (`0..4`, `5..9`, ...), five columns (`0,5,10,15,20`, ...), the main diagonal `0,6,12,18,24` and the opposite diagonal `4,8,12,16,20`.

Status of a line, in this order of precedence:

| Status | When |
|---|---|
| `unknown` | a House of the line cannot be read. Nothing is inferred about the line; an unreadable House is never an open one |
| `blocked` | both factions hold a House of it |
| `complete-a`, `complete-h` | five Houses of one faction (the line is "held") |
| `one-away-a`, `one-away-h` | four Houses of one faction and one open (the missing House is named from the board) |
| `open-a`, `open-h` | Houses of one faction only, more than one still open |
| `open-any` | nothing claimed on it |

Facts returned, kept apart (never one yes or no): the counts (`a`, `h`, `open`, `unknown`) and whether they are complete; the twelve lines with the Houses each faction holds on them and the Houses still missing; the held lines; the lines one House away; the potential lines (`total`, for Atreides, for Harkonnen, and whether the figure is exact); the leader and the margin; `thresholdReached` and `majority` (13 of 25 or more: the other faction cannot reach as many); the cutoff reading; the verdict; a conflict flag with its reason; and a key made of the term id and the 25 owners, so a result is tied to the board it was read from.

POTENTIAL LINES is the approved design's own count: the lines that are not blocked and not held. A line with nothing claimed counts for both factions and once in the total. With any unreadable House the figure is "Count incomplete" and no number is shown (the same fail-closed rule as the House count).

The verdict names which fact decides, in this order:

1. Both factions hold a line: `conflict`. Nobody is chosen; the order of the lines never decides (D04).
2. One faction holds a line: `line_held`. A held line is read from the Houses as they stand, so an unreadable House elsewhere does not cancel it. If a verified cutoff gives the other faction more Houses, it is a `conflict` instead (D13a).
3. A verified cutoff with more Houses for one faction: `majority_at_cutoff`; equal Houses: `none`.
4. An unreadable House and nothing above: `unknown`.
5. A lead in the House count without a verified cutoff: `pending_vote`, even at 13 or more (D05).
6. A line one House away: `possible`. Otherwise `none`.

The cutoff evidence is `null` or `{ verified: true, owners: [25 values of a, h or open] }`. Anything else (a snapshot of 24, one with an unreadable owner, `verified: "yes"`) is no evidence. **The page always passes `null`**: the save holds no record of the start of the Voting Session (D03), and the `sysselraad` column of a task is carried and never read (D02). Rule 3 and the cutoff half of rule 2 are specified and tested so that a source, if one is ever found, does not change the evaluator; they cannot fire in the product today.

The evaluator is checked against all 40 board cases of M1 (`fixtures/board-victory-cases.json`): the status of every line, the held and one-away lines, the leader, the margin, the 13-House flag and the verdict, for the valid boards, and the "no counts, no lines, verdict unknown" result for the four that are not boards.

## 3. What the board shows

* **POTENTIAL LINES** shows the number, with "Unblocked five-House alignments", the number for each faction and, when a line is held, "N held, not counted". With an unreadable House it says "Count incomplete" and no number; with a board that is not a board it says "Not counted".
* **A line of five** (five Houses of one faction in a row, a column or a diagonal): those five Houses, and only those, get the circuit outline: the gold outline of `--lcc-outline-win`, the glow at `--lcc-glow-win` (1, not scaled by the 0.5 dial of the plaque and the rest of the page), the words LINE OF FIVE, and a name and a tip that say which line ("part of a line of five held by Atreides: row 3 ... the Houses as they stand now, not a vote result"). Both factions with a line: both lines are outlined and the panel says CONFLICT; neither is chosen.
* **13 or more Houses** held by a faction: its Houses outside a line get the majority outline (a double border at the same glow), the words MAJORITY HOUSE, and a tip that says it is a count and "not verified: the save does not record the start of the Voting Session". The legend gains "MAJORITY HOUSE · COUNT NOT VERIFIED". A House in a line has the line outline only.
* **The moving border**: the optional animated border chosen for M3 (class `lcc-win-anim`, a gold arc moved by `transform` only) is added to the Houses of a line and to the Houses of the 13 only with Motion Full and no reduced-motion request; the style checks both again. Calm, Off, reduced motion and forced colours show the still outline (forced colours: a 4 px solid and a 5 px double border and a system-colour outline).
* **Imperial Victory Analysis** has five states, read from the verdict. None held: "AWAITING VOTING SESSION / SYSSELRAAD CONTESTED" as in M3. A line: "FIVE IN A LINE", "Atreides · FIVE IN A LINE" and the lines named. 13 or more Houses: "IMPERIAL DOMINION · NOT VERIFIED" and "N HOUSES HELD", with the sentence that a House-count lead cannot be confirmed here. Conflict: "BOTH FACTIONS HOLD A LINE". Board not usable: "COUNT UNAVAILABLE". Under the figures a line says the House count as a comparison ("Atreides ahead 15 to 3 · not verified: the save does not record the start of the Voting Session"), or that it is incomplete. A folded list gives the twelve lines in the board's own order with one sentence each ("Atreides 4 of 5, one House away: House Wydras is still open").
* **TERM STATUS** says FIVE IN A LINE or CONFLICT when that is what the board shows, and CONTESTED otherwise. The plaque headline "VOTING SESSION · SEAT 13 OF 25" is unchanged.

## 4. What it cannot know

* **The start of the Voting Session.** No table or column of the save holds it. So a House-count lead is never a result, 13 Houses is a count that cannot be overtaken on the board and not a vote result, and "Imperial Dominion" is only ever shown as a count comparison marked not verified. A confirmed Dominion is never shown.
* **What `sysselraad` means.** The column is not read.
* **Whether a held line is what the game counts as a result.** A held line is a fact about the Houses now. The game's rule (five connected Houses or a House-count lead at the start of the Voting Session) is the wiki's; the save does not say a vote was held, so no sentence says one was won. Which of two lines came first is not recorded either, hence the conflict.
* **Houses that cannot be read.** Counted for neither side and never as open; the count is incomplete and no number of potential lines is made up.

## 5. Differences from the approved prototype and the package prompt

* The prototype joins the five Houses of a line with a drawn route through their centres. This milestone draws the animated border round each House instead (the choice of the project owner for the winning treatment); there is no route between the Houses, and so no measured geometry, no resize observer and nothing that can cover a button. The five Houses are told apart from a faction's majority by the outline, the badge text and the words, not by a route. Adding a route would be a separate decision.
* The prototype names the badges VICTORY LINE and DOMINION; the board says LINE OF FIVE and MAJORITY HOUSE (the legend keeps the prototype's VICTORY LINE with a tip that says what it is), because the save cannot say a vote was won.
* The package's Strategy Mode (a list of the lines to click, a jump to the missing House, a violet preview) is not part of this milestone; the folded list of twelve lines is plain text and does not select or highlight anything.
* The prototype's "SYSSELRAAD SECURED" and "IMPERIAL DOMINION" headline for a majority is not used: no state says secured or won.

## 6. Tests

`web-tests/lcc-lines.test.mjs` (the evaluator: the twelve lines and their order, each line held by each faction, all 40 cases of M1, mixed ownership, blocked and open lines, one House away, the 12 and 13 boundary, a verified cutoff and malformed evidence, conflicts, unreadable Houses, board order and the key, and the words), `web-tests/lcc-lines-board.test.mjs` (the figure, the outlines, the panel states, the conflict, the incomplete count, the list layout, Motion Full, Calm and Off, and that a click only selects) and `web-tests/lcc-lines-css.test.mjs` (the win token and the 0.5 dial, the moving border rules and their gates, the reduced-motion and forced-colours rules, the text floor). The existing board tests that expected a dash in the POTENTIAL LINES tile were updated.

## 7. Not verified

* Nothing was run against a real save or a running game. Every test and capture uses the synthetic answer of M1 with the owners set by the test.
* The moving border cannot be seen in a still. Its rule exists and was read from the style engine of a browser (computed styles: the keyframes and the duration with Motion Full, no sweep element with a reduced-motion request, Calm, or forced colours); the captures show the still outline.
* Windows high contrast is covered by the forced-colours rules and an emulated forced-colours capture, not by Windows itself.
* The list layout of the board does not outline Houses (it is a table); the panel and the figure are the same in both layouts.

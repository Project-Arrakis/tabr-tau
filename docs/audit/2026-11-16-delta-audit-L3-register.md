# Findings register, 2026-11-16: delta audit L3 of the editor (tabr-tau)

Tracking issue: [#817](https://github.com/Project-Arrakis/tabr-tau/issues/817). Layer: **L3** (code review of the whole diff), under `docs/decisions/2026-11-14-audit-cadence-and-no-peer-review.md`. The L2 register of the same range is `docs/audit/2026-11-16-delta-audit-L2-register.md` (tracking #809); its findings R2-01 to R2-06 (#810 to #815) are not repeated here.

Base: `002ba13`. Range: `002ba13..acbde7d` (main on 2026-11-16), 39 first-parent commits (#652 and #706 to #816; the last is the L2 register itself). 142 files, about 14,300 lines added and 1,000 removed. The pull requests and their risk classes are listed in the L2 register.

## Method

The range was read as one diff, not one pull request at a time, for correctness, regressions and the interactions between the merged changes:

- one review of the whole diff from several angles (correctness, removed behaviour, callers of changed code, reuse, simplification, cost);
- two more readers on slices: the page code with its server routes (`internal/web/static/app.js`, `theme-menu.js`, `app.css`, `internal/web/server.go`), and the vehicle ledger, the repair caps, Teleport and the character's place, the public-text check with its workflows, and `internal/save` outside the Landsraad;
- every candidate was then read again in the code at `acbde7d` with its callers before it was filed, and compared with the L2 register and the open issues of the range (#772, #775, #778, #793, #794, #797, #799, #800, #805 and the public-text follow-ups), so nothing already filed is filed again;
- the issue texts were run through `cmd/publictextcheck -items` before filing.

Everything was traced by reading. No test suite was run and nothing was tried in a browser or in the game (see Not covered). Single-player only. **Nothing was fixed by this review.**

## Result

| Severity | Count |
|---|---|
| Critical | 0 |
| High | 0 |
| Medium | 1 |
| Low | 9 |
| Total | 10 |

No finding loses the game's data or opens a path from outside the machine. The Medium finding is on the Landsraad board: a bulk question that is still open after a filter unticked its houses writes the hidden houses when it is answered with Yes.

## Findings

| Id | Severity | Kind | File and line | What is wrong | Issue |
|---|---|---|---|---|---|
| R3-01 | MEDIUM | interaction (page, write path) | `internal/web/static/app.js:2371`, `:2323-2334`, `:2453-2471` | A bulk question keeps its list of houses when a filter unticks the hidden rows; Yes writes all of them, also the ones shown as not chosen. | [#818](https://github.com/Project-Arrakis/tabr-tau/issues/818) |
| R3-02 | LOW | interaction (gate, status) | `internal/save/durable.go:261-296`; `internal/ops/ops.go:152`, `:180` | A queue of only the automatic pass's edit is written under the unattended rule, the status and Check again answer with the hand rule: the page says writable while every write is refused. The rule is read from a prefix of the edit's description. | [#819](https://github.com/Project-Arrakis/tabr-tau/issues/819) |
| R3-03 | LOW | regression (read) | `internal/ops/landsraad.go:144-147`; `internal/ops/landsraad_locations.go:50-55` | The Landsraad read fails as a whole when the optional marker query for the house places fails; the fixed places are not used as the fallback. | [#820](https://github.com/Project-Arrakis/tabr-tau/issues/820) |
| R3-04 | LOW | correctness (gate) | `internal/save/save.go:746-754`; `internal/save/filetime.go:103-124` | The gate added after the encode does not keep the file watch unbroken when the encode alone takes longer than the watch gap: the watch restarts and the write is refused each time. Times not measured. | [#821](https://github.com/Project-Arrakis/tabr-tau/issues/821) |
| R3-05 | LOW | regression (read) | `internal/ops/livemap.go:70-82`; `internal/ops/character_place.go:49-52` | The Saved map draws no character when the place read succeeds without an answer; the fallback to the body's row runs only on a read error. | [#822](https://github.com/Project-Arrakis/tabr-tau/issues/822) |
| R3-06 | LOW | interaction (page) | `internal/web/static/app.js:2271-2273`, `:2313-2314`; `internal/web/static/app.css:8`, `:53` | The board's result line and question are scrolled to the top edge, under the sticky header and banner. | [#823](https://github.com/Project-Arrakis/tabr-tau/issues/823) |
| R3-07 | LOW | regression (page) | `internal/web/static/app.js:2726`, `:3172-3180` | A rows redraw drops the scroll offset of every box, also of the list that was clicked in; the refocused button is hidden inside the box. | [#824](https://github.com/Project-Arrakis/tabr-tau/issues/824) |
| R3-08 | LOW | correctness (page and server) | `internal/web/static/app.js:1211-1216`; `internal/ops/teleport.go:120` | Go to a place counts the hub the character stands in as a place of the chosen map: "other map" is worked out against the character's map, the count is about the chosen one. | [#825](https://github.com/Project-Arrakis/tabr-tau/issues/825) |
| R3-09 | LOW | concurrency (answer text) | `internal/web/server.go:524-530`; `internal/save/flush.go:85-87` | A repeat of a Create that overlaps the first request's write can be answered as queued although the vehicle is in the file. | [#826](https://github.com/Project-Arrakis/tabr-tau/issues/826) |
| R3-10 | LOW | upkeep | `internal/ops/landsraad_grant.go:291`; `internal/ops/landsraad_paid.go:111`, `:180`, `:325`, `:348`; `internal/ops/landsraad.go:184` | The wallet write and the standing-tier rule each exist twice, test-only helpers sit in the shipped path, and the board read merges the record once per house. No wrong behaviour today. | [#827](https://github.com/Project-Arrakis/tabr-tau/issues/827) |

Labels: every finding has `audit`, a `severity:*`, one `cat:*` and a priority (`p2-next` for the Medium, `p3-later` for the Low ones).

## Candidates not filed

Raised by a reader, checked, and left out because they are already tracked or documented:

- `shortErr` cuts ordinary text with slashes and leaves some path forms: #800.
- A discard right after a write that could not be read back loses the notes of edits that are in the file: #799 (item 3).
- A version 1 journal whose write already landed is reported as not put back: #778 (L4).
- The paid-ground record is a file beside the save, by the save's path: opened under another path, or with the record gone, a tier removal and the same raise pay again. This is the documented limit of #798 (`docs/save-database.md`, rule 9) with R2-02 (#811) and #799; nothing new was found beyond them.

## Checked and found fine

**Landsraad page and server.** The fields the page sends with a click (`was_mine`, `was_done`, `was_win`, `was_events`, `mine_to`, `lower`, `complete_for`, `reopen`, `term_id`) are parsed the way the page means them; the mission count the page shows before a click matches the write, including the order of the "already covered" and problem checks. Term and decree writes take the id forms the page sends.

**Create a vehicle.** The request id is kept with the body and only a success is remembered; the stored answer is copied, never changed by the handler; the lock order is always the ledger, then the save; a held queue refuses the write, so a repeat that finds no vehicle cannot make a second one; every reload path is covered by the identity check at each use. The in-flight flags of the page are cleared on the error paths.

**Repair.** The best-health query is valid against the schema; holograms are left out of both the target and the write; the statement is journaled as one update without arguments.

**Teleport and the character's place.** The refusal of a body that is not standing comes before the map comparison and never marks the map as parked; the paths with no controller, no body and no pawn hold in the reader, the player page and the summary (the Saved map is R3-05).

**Public-text check.** Commit ids reach the shell through the environment only; concurrency groups hold numbers only; an event or file that cannot be read fails the run; a stale base over-includes commits, which fails closed.

**Page.** New routes go through the common wrappers (content type, session, same origin, body size); save text goes through the escaping template or `textContent`; the base storage's open container is cleared on every move away; the periodic poll cannot come between an edit's answer and its read.

## Not covered

- No test suite, race run or web test was run (the machine was in use); CI ran them on each merge. Every finding is from reading the code and is marked so in its issue.
- Nothing was tried in a browser or in the game: the page findings (R3-01, R3-06, R3-07, R3-08) are traced from the code and the style sheet only.
- R3-04 rests on an encode longer than the watch gap; no encode was timed.
- The end-of-session lines the gate reads from the game's log were compared with the repository's fixtures only, not with a current log.
- The tests added in the range were read only where a finding touched them; the regular expressions of the public-text rules were not exercised beyond the forms already filed.
- The saved map's own script beyond its small diff, the Landsraad row markup and its accessibility strings, the building-sets count line and the decree-name helpers were skimmed. Overlapping redraws (the automatic refresh during a redraw after an action) have no sequence guard; this is older than the range and was not traced.
- The kit and design modules were not changed in the range and were not read.

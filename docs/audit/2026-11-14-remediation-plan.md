# Remediation plan, 2026-11-14: full audit of the editor (tabr-tau)

Companion of `docs/audit/2026-11-14-full-audit-register.md` (finding ids G-001 to G-068; issues #653 to #662 and the comments listed there). Audit tracker: [#606](https://github.com/Project-Arrakis/tabr-tau/issues/606). This plan only orders the work; nothing in it has been done. Single player only.

Order of the plan: first the decisions that block code, then waves by severity, effort and dependency. Effort: S is under a day, M one to three days, L more. "Reviewed" means a save write path, which is reviewed before it merges (the standing rule for this repository).

## 0. Decisions needed first

These block work or change what a fix should do. One line each, with the recommendation.

| # | Decision | Findings | Issue | Why it blocks | Recommendation |
|---|---|---|---|---|---|
| 1 | Landsraad progress events above the game's cursor: may PR #652 merge before the in-game check, or must it wait? | G-004, G-016 (D-U5, G-015) | #655 | #652 turns every click into its own event, so the shape the game is not known to read is written many times more often. | Run the in-game test on a copy first (raise, save, load, play past the editor's event, compare contribution, reward rows and the event sum, confirm the next event id). Until it is done, label the delivery "not yet verified in the game" in the tab, the README and the changelog. If #652 merges first, ask before a click that crosses the top tier. |
| 2 | "Delivered once per term": decided per item today. Move it to per tier? | G-005, G-017 | #656 | The rule is the heart of `deliverContribution`; #652 and every later Landsraad change builds on it. Real houses are already affected (a collected Solari tier is not given again; 11 of 150 tasks repeat an item at two thresholds). | Per tier: deliver only thresholds above the larger of the old contribution and the character's own event sum for the task; keep the merge into a waiting amount. Also ask the game's behaviour once (does it treat a collected row as delivered this term for a second tier of the same item?). |
| 3 | The character's place when the body is parked on another map: refuse "next to the character", or ask for an explicit place? | G-001, G-008, G-022, G-068 | #654 | The fix changes what Bring, Create, Duplicate and Teleport answer. | Refuse with a plain sentence by default, and let the person choose a place by hand. Treat a teleport as a move between maps (all three actors). |
| 4 | Unattended automatic pass: restore the process-presence rule, or wait for the file-time signal? | G-007 | #612 | The pass writes at every launch with nobody watching, now on one signal. | Restore the process rule for the unattended path only; later relax it when the second signal exists. |
| 5 | Edit public issues and PRs, and the personal path in #470? Accept or rewrite history? | G-002, G-011, G-028 | #653 | It changes public content, so it is the owner's call. | Edit the listed items now (bodies and one comment); accept the history residue and record it on #621 (a rewrite breaks clones and PR links). |
| 6 | Which servers is the base export for? | G-012 | #658 | The README's scope paragraph and the export document disagree. | If it is the person's own self-hosted server only, say that in the README scope paragraph and the document, and retitle #534. |
| 7 | How did commit 38e0806 reach `main` without a PR, and is there a bypass? | G-031 | #661 | If a bypass exists it weakens every other control. | Look at the repository's rules and remove any bypass. |
| 8 | Vendor or fork the WebView2 wrapper, or offer the fixes upstream first? | G-024, G-025 | #660 | Decides the shape of F8. | Set `WEBVIEW2_USER_DATA_FOLDER` and re-read the two settings now (small); offer the HRESULT and keep-alive fixes upstream; vendor only if they are not taken. |
| 9 | Will the editor ship binaries? | G-055 (G-056) | #661 | Decides whether releases need checksums and provenance, or a README sentence. | Say on the README that there are none until #626 and #577 are decided. |

Decisions 1 and 2 are the two Landsraad write decisions that affect PR #652. Decision 3 is the one for the High finding G-001.

## 1. Wave 1: High (p1-now)

Both can start at once; the first has parts that need no decision.

| Work | Findings | Issue | Effort | Depends on | Notes |
|---|---|---|---|---|---|
| Public text rule | G-003, G-027, G-029, G-030 | #653 | S to M | none | Tree and CI changes: neutral labels and comments, the digest check over issue and PR text, the commit-message check over the PR title and body, template lines. A PR with tests; the hygiene test is the check. |
| Public text rule, edits to public content | G-002, G-011, G-028 | #653 | S | decision 5 | Editing the listed issues, PR bodies and the comment by hand, in the owner's name. |
| The character's place in one helper | G-001, G-008, G-068; then G-022 | #654 (#617) | M to L | decision 3 | One helper inside the edit that reads controller, body and player state; used by Bring, Create, Duplicate, Teleport and the Landsraad distance. Fixture with the parked-body shape. Reviewed. Do G-022 (the hub rule) in the same change. |

## 2. Wave 2: Medium (p2-next)

Landsraad group, in this order because the items share `landsraad_apply.go` and `landsraad_rewards.go`:

| Step | Work | Findings | Issue | Effort | Depends on |
|---|---|---|---|---|---|
| 2a | In-game test of the progress events | G-004 (G-015) | #655 | S, by the operator | decision 1 |
| 2b | Decide delivery per tier; one `tierState` for read and write; the "not written" warning | G-005, G-017 | #656 | M, reviewed | decision 2, ahead of or together with #652 |
| 2c | Retire the seven older routes (or route them through the apply core), one faction rule, one helper each | G-006, G-018, G-019, G-047, G-067 | #657 | M, reviewed | 2b (same files); #652 for G-019 |
| 2d | Contributions that differ from the event sum shown on the read; no second event for a re-raise; the removal's confirm text | G-015, G-016 | #655 | M | 2a, 2b |
| 2e | The four Landsraad tables in the layout check | G-020 | #615 | S | the D5-1 work |

The rest of the Medium work is independent and can run in parallel:

| Work | Findings | Issue | Effort | Depends on |
|---|---|---|---|---|
| The unattended pass: the process rule | G-007 | #612 | S | decision 4 |
| Base import: best health only from standing bases, or a table per building type | G-009 (G-021, G-045) | #613 | M, reviewed | none |
| Seven redraws that jump to the top | G-010 | #650 | M | none; do it with #650's own fix, before #651 |
| Branch and tag protection of the public repository: tag ruleset, one required review, CODEOWNERS for the modules, the four module jobs required | G-013, G-046 (G-056) | #622 | S (settings) | none |
| The flaky test with a path in the reason | G-014 | #567 | S | none; quick win |
| The base export and the stated scope | G-012 | #658 | S | decision 6 |

## 3. Wave 3: Low (p3-later)

Group by issue; all independent unless noted.

| Work | Findings | Issue | Effort | Notes |
|---|---|---|---|---|
| Board accessibility, touch sizes, one write model per tab | G-034 to G-040 | #659 | M | Re-check G-036 and G-040 after #652 merges. G-034, G-038 are one-line fixes. |
| Small defects and drift: export notes "+", README rows and route list, QA line, stale labels | G-041 to G-044 | #662 | S | G-043 again after #652. |
| The embedded window | G-024, G-025 | #660 | M | decision 8; the INFO items G-048 to G-051 are done by the same patch. |
| CI scanners read their ignore files from the PR; the commit without a PR | G-031, G-033 | #661 | S | decision 7. |
| Notices: provenance for the house art and a test over the art folders | G-032 | #621 | S | |
| Window title shows only the base name | G-026 | #619 | S | |
| Read the live save with the delete share mode | G-023 | #623 | S, a module change | Tag the module promptly (G-046). |
| Hub rule on a same-map teleport | G-022 | #617 | S | Done with Wave 1's helper. |

## 4. Wave 4: Info

Done when a nearby change touches the code, or in one tidy-up change; none is filed on its own.

- Window: G-048 to G-051 with F8. Scanners and releases: G-053 to G-059 with F9 (a pinned workflow linter, a monthly refresh of the scanner pin, `cache: false` in the release jobs, `allow: dependency-type: all`, signed tags with the ruleset).
- Board and tests: G-060, G-061 with F7; G-062 (run the real-file tests before a release and say so in the notes), G-063 (an injected clock), G-064 to G-066 (cache the own-save answer, test size first, read the clock once per edit).
- G-045 (a shared policy package for the ceilings) with the G-009 fix; G-052 (document the cookie scope).

## 5. Order in one line

Decisions 1 to 5, then in parallel: public text (G-002 and the rest), the character's place (G-001), the in-game test of the events (G-004); then the Landsraad group 2b to 2d, with the Medium items beside it; then the Low items by issue. The remaining wait is on the operator, not on code: the in-game test and decisions 1 to 3.

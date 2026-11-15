# Decision: audit cadence, and why changes are not peer reviewed

Date: 2026-11-14. Owner and approver: the project owner (sole maintainer). Tracking issue: #673. Review date: 2026-12-14, or sooner on any trigger below.

## Why this is written down

The organisation's operating rules describe a layered audit: L1 on a design before code, L2 on a feature's implementation before it is complete, L3 on the whole diff before the pull request. This project does not run that on every change, and no second person reviews a pull request. Both are deviations from what the rules describe. A deviation that is not written down is an unmanaged risk, so this record states the decision, the reasons, the controls that make up for it, and the risk the owner accepts.

## Decision 1: audit cadence

- A layered audit does not run per change. The owner asks for an audit, normally overnight. An overnight audit means **L2 and L3** together, over everything merged since the previous audit.
- **L1** (design audit) is done only for a real design, before code: a new subsystem, a new write path, a new trust boundary. Small features and fixes do not get one.
- Every audit produces a findings register in `docs/audit/`, every finding is filed as an issue with a severity and category, Critical and High findings are fixed before the audit is called closed, Medium and Low findings are filed with a priority, and an audit issue is closed only when its fix and its regression check are merged.
- Before any release handed to other people, all three layers must be complete for the content of that release. That rule is not relaxed.

**Why:** between 2026-11-11 and 2026-11-14 this project had 34 pull requests, 27 of them touching code and 5 of those over 1,000 changed lines. A layered audit of each would mean roughly nine independent review runs per code change. That cost is not sustainable and would push the owner to skip audits altogether, which is worse than a lower, honest cadence.

## Decision 2: no peer review of pull requests

Pull requests are not reviewed by a second person before merge. The repository has no required approving review.

**Why:** the project has one maintainer. There is no second person with the context and the time to review. Requiring an approval would either block every change or be satisfied by a rubber stamp, which is worse than saying plainly that there is none.

**Risk the owner accepts:** the author's mistakes, and a mistaken or compromised author, are not caught by a second human. A change can reach the main branch with only the owner's own review.

## Compensating controls (what stands in for peer review)

1. **The owner approves code pull requests, with an overnight exception.** By day nothing code-bearing is merged without the owner's explicit word. Overnight (22:00 to 07:00 local, or a run the owner has declared overnight) code pull requests may merge on green CI, with one extra guard: a pull request that writes to a save or the game, changes security or CI controls, or carries a High label also needs a clean high-effort code review before it merges. Docs-only changes merge on green CI at any time, under the owner's standing agreement of 2026-11-14 (the overnight rule for code was agreed the same day). Automated workers open pull requests and never merge them.
2. **CI on every pull request,** which must be green before merge: build, tests, the commit-message policy check, static analysis and secret scanning, with every action pinned to a commit SHA.
3. **Every change is tracked:** a pull request references its tracking issue and states its risk (Critical, High, Medium, Low) and blast radius in the body.
4. **Periodic audits** (Decision 1) catch what a per-change review would have: independent reviews from several viewpoints and a high-effort code review, with written registers.
5. **A regression ledger** records which check pins which fixed finding, so a fix cannot silently come back.
6. **An independent automated review:** the code review runs separately from the session that wrote the change. It is an extra look, not a human peer, and is counted as a reduction of risk, not as peer review.
7. **Write safety:** the editor writes only to a save file the user points it at, shows what it will change first, and keeps the queue visible; tests never use a real save.

## Known gaps this does not close

- The owner approves pull requests that the owner's own account merges: there is no separation of duties.
- Organisation two-factor authentication, tag protection for release tags, code-owner review, and signed commits are not all in place yet. They are tracked as audit findings and are the owner's to switch on one setting at a time.
- There is no written incident, access-review or evidence-retention policy beyond the registers and issues.

## Revisit this decision when

- a second maintainer or outside contributor joins;
- a release is handed to other people;
- a security finding is traced to a change that a second reviewer would have caught;
- the review date above passes.

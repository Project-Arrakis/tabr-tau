# Removing or weakening a control that an audit register relies on

A control is a check, a gate, a confirmation or a record that an audit register (`docs/audit/`) names as the answer to a finding: the
game-running check, the changed-on-disk check, the integrity check, the backup before a write, the typed word before a far-reaching action,
the review of what will be written, and so on. Removing one is a change in the safety of the product, however small the diff is.

The review pane (a list of every edit that had to be looked at before anything was saved) was removed in #240 as part of "write edits as they
are made". The change was classed High by its author, was merged without an issue analysis or a review, and left the register saying that the
review was the control. The audit of 2026-10-22 recorded this (GRC-12). This note is the rule that follows from it. History is not rewritten:
the register keeps both the old rows and the note of what replaced them.

## What a change that removes or weakens a control must have before it is merged

1. **An issue** that names the control, the finding or requirement it answers (for example F-06 and F-10 for the review pane), the risk the
   control was holding back, and what will hold it back afterwards. If nothing will, the issue says so and the owner decides.
2. **A review of the issue analysis before the code**: a second reader (a person or a review pass that is not the author) agrees that the
   replacement is enough, or lists what is missing. A change the author classes High or Critical is not self-approved.
3. **The register updated in the same pull request**: the finding rows the control answered get a "replaced by" statement that says what now
   plays the role, where in the code it lives and which test pins it. If the replacement is weaker, the gap is filed as a finding.
4. **The documents in the same pull request**: `SECURITY.md`, the README, the changelog entries that describe the control, and the page text
   (`internal/web/static`) stop describing it. A test that fails when the old wording returns is added where one is practical (the page text
   has one: `internal/web/static_text_test.go`).
5. **The pull request template filled in honestly**: the risk class, the documentation impact and the audit trail say that a control is
   removed or weakened, and name the issue.

## When it was missed

If a control was removed without these steps, open an issue for the analysis after the fact, add the "replaced by" statement to the register,
correct the documents, and say in the issue what the review found. Do not delete or rewrite the original pull request, commit or register row.

The replacement statement for the review pane is in the E5 group of the 2026-10-22 register.

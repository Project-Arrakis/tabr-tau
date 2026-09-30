## Summary
<!-- What changes and why. Link the issue(s): Fixes #N / Refs #N. -->

## Risk classification
<!-- Critical / High / Medium / Low, plus one line on the blast radius: what could this break, for whom. -->

## Documentation impact (Requirement 14)
<!-- Which docs did you review? Did they need updating? If something is deferred: what, why, who owns it, which issue. -->

## Verification (real output, not "tests pass")
<!-- Paste the command and its actual output: go test / go vet / staticcheck, and for tools/*.ps1 the PSScriptAnalyzer result.
     If a live test in docs/live-test-protocol.md applies, link the evidence. -->

## Audit trail (Requirement 20)
<!-- Which audit finding(s) F-nn does this close? Which layer (1 design, 2 implementation, 3 integration) was run? -->

## Checklist
- [ ] No save, `Game.ini`, snapshot, log, or screenshot containing account/platform IDs is included
- [ ] No AI co-author trailers or attribution lines in any commit message
- [ ] Failing test written first for a bug fix or security finding
- [ ] `CHANGELOG.md` updated (or "no user-visible change" stated above)
- [ ] Nothing here touches a live save without the game-running and on-disk-change checks

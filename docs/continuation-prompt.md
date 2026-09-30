# Continuation prompt: tabr-tau

Paste this into a new Claude Code session (or read it yourself) to resume. Written 2026-09-30.
Contains no credentials and no account IDs. Verify anything time-sensitive before relying on it.

## Who and what
`tabr-tau` (`Project-Arrakis/tabr-tau`, Go, MIT) is a local-web-UI editor for a **single-player Dune: Awakening
save** on a Windows PC. Operator: the repo owner. Operating rules: `~/projects/meta/Project-Arrakis/README.md`
(re-read it first; Requirement 17). Key rules here: work on branches and PRs (21), findings as issues and a STRIDE
table (20), no secrets or IDs in the repo (5, 24), no AI co-author trailers in commits (operator rule), verify
before asserting (12).

## State (2026-09-30)
- Design and Layer 1 audit are done and on PR #1 (`docs/dune-docker-parity-design`). **Read `docs/README.md` first.**
  Then `docs/design/2026-09-30-architecture-and-implementation-plan.md` (v0.2) and
  `docs/audit/2026-09-30-L1-findings-register.md` (F-01..F-22).
- Application code is unchanged from the original editor. **Nothing from the plan is implemented.**
- Not done: GitHub issues for the findings, branch protection, CI, secret scanning, the `meta` README entry.

## Verified facts (evidence in `docs/`)
- Single-player is an in-process listen server (`Survival_1?listen`) with SQLite persistence. **No RMQ.**
- Save = `game.db` = `uint32 1`, `uint32 size`, zlib(SQLite). 96 tables (94 application). 92 are STRICT.
- State lives in relational columns, JSONB (`fgl_entities.components`, `actors.gas_attributes`, `actors.properties`) and text
  JSON (`items.stats`). Character XP and skill points: `FLevelComponent[1]` of the `DuneCharacter` entity. Hydration:
  `actors.gas_attributes.DuneHydrationAttributeSet.CurrentHydration`.
- dune-docker uses RMQ only for live commands (give-item, XP, skill points, water, kick, broadcast, teleport, vehicle spawn).
  Single-player equivalents are direct save edits. Multiplayer-only tables (guilds, exchange, world_partition) do not exist.
- The existing code sets `foreign_keys(0)`, has an unbounded zlib read, serves the API token at `GET /`, and inserts
  unescaped save values into the UI. These are audit findings F-03/F-04/F-08, **not yet fixed**.

## Do next, in this order (the plan's phase order)
1. **Operator decisions (ask, do not assume):** F-01 (credential exposure in the transfer path: details are withheld from this public repo and tracked privately by the operator; do not ask for or handle the credential); F-12 is done (history rewritten) if the register says so; approval to open the `meta` PR.
2. **P0 governance**, in order: secret scanning + push protection, then `.gitignore`/gitleaks/PR template/CHANGELOG,
   then CI, then branch protection with `<job> / <inner>` check names, then the `meta` README PR (repo missing from
   its layout, repo notes, Requirement 15 list and Requirement 28 list), then file F-01..F-22 as issues with the STRIDE table.
3. **S0 security hardening of existing code**, failing-test-first: bounded decode, output escaping + CSP, refuse non-loopback `--addr`,
   separate read-only connection with an authorizer, schema fingerprint. (Plan section 4.4.)
4. **S1 write pipeline** (single `save.Mutate`, retained original, Go diff engine, durable Commit/Restore, `foreign_key_check`),
   **S2 UX safety**, in parallel **P1 live tests** (`docs/live-test-protocol.md` v2). **S3 features only after P1 passes.**

## Live testing
Transfer a snapshot with `tools/send-snapshot.ps1` (scp, key auth, hash-verified, unprivileged account). Compare copies with the redacted
`tools/snapsummary.py` output (committable under `docs/evidence/summaries/`). Never commit raw snapshots anywhere, including `meta`.
Run on the Windows PC with the game closed. `tools/snapshot.ps1 -Label <label>`; analyse with
`python tools/snapdiff.py <before> <after> --noise` (Python >= 3.11, redacts IDs by default). Each snapshot contains account IDs:
keep it local and private, delete when done. Never commit saves, snapshots, `Game.ini`, logs or screenshots with IDs.

## Do not
- Do not run tests or installers that write outside a scratch directory on a shared or production host, and never use `/tmp`.
- Do not edit a live save while the game runs, and do not touch `autosave/`, `game_prepatch.db` or `game.crash.db`.
- Do not trust `[I]` or `[T]` claims. Do not treat the audit's "not confirmed" items (register footer) as facts.
- Do not merge PRs or file issues on the public repo without operator approval.

## Where the analysis inputs went
The real sample save is **not** in git (contains IDs). The schema-only extract is `docs/evidence/`. The operator has the
original files on the PC (`%LOCALAPPDATA%\DuneSandbox`). Regenerate the DDL and diffs with the commands in
`docs/design/2026-09-30-dune-docker-parity.md` section 7.

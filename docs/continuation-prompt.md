# Continuation prompt: tabr-tau

Paste this into a new Claude Code session (or read it yourself) to resume. Written 2026-09-30, rewritten 2026-10-15 to the current state.
Contains no credentials and no account IDs. Verify anything time-sensitive before relying on it.

## Who and what
`tabr-tau` (`Project-Arrakis/tabr-tau`, Go, MIT) is a local-web-UI editor for a **single-player Dune: Awakening
save** on a Windows PC (Windows 10/11 only; one exe that serves `127.0.0.1` and shows the UI in a WebView2 window or the browser).
Operator: the repo owner. Operating rules: the Project Arrakis `meta` README (re-read it first; Requirement 17). Key rules here:
work on branches and PRs (21), every finding filed as an issue with a STRIDE table (20), no secrets or IDs in the repo (5, 24),
no AI co-author trailers in commits (operator rule), verify before asserting (12), documentation drift is a defect (14).

## State (2026-10-22)
- Edits are written to the game's file as they are made, or queued while the file cannot be written (#238); there is no review pane
  and no Save step. What stands in its place is recorded in `docs/audit/2026-10-22-L2-findings-register.md` (E5) and `SECURITY.md`.
- Done and merged: governance and CI (P0), security hardening of the original code (S0), the single write path `save.Mutate` with
  durable writes, restore and a foreign-key gate (S1, S2), the Player tabs mirrored from the console (S3) including
  specialization, skills, research, crafting, Journey, augments, building sets and customizations, bases (power, water, inventory,
  land-claim editor), vehicles, teleport, faction, the Live Map drawn from the save, config and database tabs, and the automatic refill.
  The full list is in `CHANGELOG.md` and `README.md`.
- Audits on record: Layer 1 (design) 2026-09-30, Layer 2 2026-10-07 (write pipeline), Layer 2 2026-10-15 (everything since;
  tracking issue #157, register `docs/audit/2026-10-15-L2-findings-register.md`, issues #158-#182). Open findings are in those issues.
- Open work: see the open issues (`gh issue list`). Notable: #67 teleport map change and Live Map click, #111 Blueprints (needs a save
  that has blueprints), #110 and #109 (Deep Desert and Landsraad, waiting on the owner reaching them), #77 (ServerCustomSettings.ini), #39/#40/#41.
- Which write features have been checked in the game and which only by tests: the verification matrix in `README.md`.

## Verified facts (evidence in `docs/`)
- Single-player is an in-process listen server (`Survival_1?listen`) with SQLite persistence. **No RMQ.**
- Save = `game.db` = `uint32 1`, `uint32 size`, zlib(SQLite). 96 tables (94 application). 92 are STRICT; 61 are WITHOUT ROWID.
- State lives in relational columns, JSONB (`fgl_entities.components`, `actors.gas_attributes`, `actors.properties`) and text
  JSON (`items.stats`). Character XP and skill points: `FLevelComponent[1]` of the `DuneCharacter` entity. Hydration:
  `actors.gas_attributes.DuneHydrationAttributeSet.CurrentHydration`.
- dune-docker uses RMQ only for live commands (give-item, XP, skill points, water, kick, broadcast, teleport, vehicle spawn).
  Single-player equivalents are direct save edits. Multiplayer-only tables (guilds, exchange, world_partition) do not exist.
- The game rewrites game.db every 60 seconds while a single-player session runs (its log says so) and once more within a few seconds of leaving the world; an edit is never written
  onto a file the game changed since it was read (`ErrChangedOnDisk`): the queued edits are put back onto the newer file first, and held
  with a reason when they no longer apply. With the game closed, or open at its menu and quiet (see README "How saving works"), edits are written; until then they are queued.
- Vehicle module durability is absolute and set by the vehicle family and the part (`internal/ops/upkeep.go`); weapons, armor and tools
  use a 0 to 100 scale.

## How to work
1. `git pull`, read `README.md`, `CHANGELOG.md`, `docs/README.md`, then the open issues.
2. Issue first, then a branch from `main` (`fix/<n>-slug`, `feat/<n>-slug`, `docs/<n>-slug`), then a PR that says which docs were reviewed.
3. Run the real checks before saying done: `go vet ./...`, `go run honnef.co/go/tools/cmd/staticcheck@v0.8.1 ./...`,
   `go test -count=1 ./...`, and `cd web-tests && node --test --test-timeout=120000` (always with a timeout: a failing UI test can hang).
4. After pushing, read the CI run (Requirement 11). Squash-merge; do not delete a branch that another open PR is based on (GitHub closes that PR).

## Live testing
Run on the Windows PC with the game closed. `tools/snapshot.ps1 -Label <label>`; analyse with
`python tools/snapdiff.py <before> <after> --noise` (Python >= 3.11, redacts IDs by default). Each snapshot contains account IDs:
keep it local and private, delete when done. Never commit saves, snapshots, `Game.ini`, logs or screenshots with IDs.

## Do not
- Do not run tests or installers that write outside a scratch directory on a shared or production host, and never use `/tmp`.
- Do not edit a live save while the game runs, and do not touch `autosave/`, `game_prepatch.db` or `game.crash.db`.
- Do not trust `[I]` or `[T]` claims. Do not treat the audit's "not confirmed" items as facts.
- Do not merge PRs or file issues on the public repo without operator approval (the operator has approved the routine PR flow in this
  repository; anything outward-facing beyond it still needs a yes).

## Where the analysis inputs went
The real sample save is **not** in git (contains IDs). The schema-only extract is `docs/evidence/`. The operator has the
original files on the PC (`%LOCALAPPDATA%\DuneSandbox`). Regenerate the DDL and diffs with the commands in
`docs/design/2026-09-30-dune-docker-parity.md` section 7.

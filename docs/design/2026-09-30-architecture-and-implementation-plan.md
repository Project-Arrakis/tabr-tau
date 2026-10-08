# tabr-tau: architecture and implementation plan (v0.2, post Layer 1 audit)

Status: v0.2. Revised after the Layer 1 (design) eight-hat audit (Requirement 20). Findings register:
`../audit/2026-09-30-L1-findings-register.md` (F-01..F-22). Where this plan cites `F-nn` it is closing that finding.
Evidence base: `2026-09-30-dune-docker-parity.md`, `../single-player-files.md`, `../live-test-protocol.md`.
Every claim is tagged **[V]** verified against the real sample save/logs/code, **[I]** inferred, or
**[T]** needs a live test before relying on it.

## 1. Goals and non-goals

Goals
1. Let one person safely inspect and edit their own single-player Dune: Awakening save on their own PC.
2. Reach parity with the subset of dune-docker's admin features that make sense without a server.
3. Never corrupt a save: every write is backed up, verified, reversible, and refused when unsafe.
4. Survive game patches: detect an unknown schema and degrade to read-only, not to silent damage.

Non-goals
- Hosting, containers, RMQ, multiplayer, guilds, exchange, Discord, IAM, autoscaling, server backups.
- Editing online-server characters or caches. Anti-cheat circumvention. Any network service.
- Injecting into or reading the game process. (The saved-file approach only.)

## 2. Ground truth: how single-player works

- Runs `Survival_1?listen`, an in-process listen server. Persistence is SQLite. **No RMQ.** [V: client logs]
- The save is `game.db` = `uint32 1`, `uint32 size`, zlib(SQLite 3). 96 tables (94 application). [V]
- State is split three ways: relational columns, **JSONB** columns (`fgl_entities.components`,
  `actors.gas_attributes`, `actors.properties`), and **text JSON** (`items.stats`). [V]
- 92 of 96 tables are `STRICT`; `items` has CHECK constraints; 102 foreign keys, many `ON DELETE CASCADE`. [V, audit]
- The schema has `applied_patches` (25 named rows), one trigger
  (`actor_fgl_entities_cleanup_orphaned_entities`), 89 indexes, and `ON DELETE CASCADE` foreign keys. [V]
- 63/80 tables shared with dune-docker's Postgres schema are identical; 17 differ mostly by
  transform/position splitting. [V]

## 3. Feature disposition (dune-docker feature -> single-player)

Class: **WORKS** identical schema, **ADAPT** SQL changes, **NEW** needs a JSONB/derived mechanism,
**BLOCKED** required data absent, **N/A** concept absent. Status: ✅ in tabr-tau today, ◻ planned phase.

### Player
| ID | Feature | dune-docker mechanism | SP mechanism | Class | Evidence | Status |
|---|---|---|---|---|---|---|
| P1 | Give item (by template) | RMQ `AddItemToInventory` or SQL insert | `items`+`inventories` insert, `items_id_sequencer` | ADAPT | [V] insert; [T] game accepts and fill-level stats | ✅ partial (no stats/catalog) |
| P2 | Item catalog/search | `admin-items.json` | reuse MIT catalog (attribution) + derive from save | NEW | [V] license MIT | ◻ P3 |
| P3 | Edit/delete/repair item | SQL | same | ADAPT | [V] | ✅ |
| P4 | Augment / pre-augmented gear | SQL + `augment-compatibility.json` | `items.stats` text JSON | ADAPT | [I] | ◻ P4 |
| P5 | Solari / currency | SQL | `items` (Solari item) | WORKS | [V] | ✅ |
| P6 | XP, skill points, per-skill points | RMQ `AwardXP` etc. | JSONB `FLevelComponent` via `jsonb_set` | NEW | [V] read; [V] scratch write; [T] game accepts | ◻ P3 (gated by T1) |
| P7 | Reset progression / skill modules | RMQ | JSONB edit + table edits | NEW | [I] | ◻ P3 |
| P8 | Hydration, spice, health; **Refill Container** | RMQ `UpdateAllWaterFillables` (sends 1,000,000; the game fills each fillable to capacity) | Container fill: item `stats` `FFillableItemStats.CurrentAmount` (absent when empty); capacity per template; health/vitals in `actors.gas_attributes` JSONB, `FHealthComponent` | NEW | Container fill [V] live PASS 2026-10-08 (#55, PR #62); vitals [V] read, [T] write | ✅ containers (Literjon family); ◻ P3 vitals |
| P9 | Teleport | RMQ / SQL | `actors.location_*` | ADAPT | [V] | ✅ |
| P10 | Faction, reputation | SQL | `player_faction*` | WORKS | [V] | ✅ |
| P11 | Specializations, keystones | SQL | `specialization_tracks`, keystones | WORKS | [V] schema; tracks empty in sample | ✅ partial |
| P12 | Journey, tutorials, tags, recipes | SQL | tables + `properties` JSONB recipes | WORKS/NEW | [V] | ✅ (recipes partial) |
| P13 | Tech knowledge points / research | SQL | `properties.TechKnowledgePlayerComponent` | NEW | [V] observed changing | ◻ P3 |
| P14 | Clean inventory | RMQ | delete `items` rows | ADAPT | [I] | ◻ P3 |
| P15 | Kick, broadcast, whisper, MOTD | RMQ | none | N/A | [V] | never |

### Bases, vehicles, world
| ID | Feature | SP mechanism | Class | Evidence | Status |
|---|---|---|---|---|---|
| B1 | List bases, land claim, totems | `totems`, `buildings` | WORKS | [V] | ✅ |
| B2 | Storage contents give/fill/remove | `inventories`/`items` | ADAPT | [V] | ✅ |
| B3 | Repair buildings, clear sand | `building_instances.health`, sand fields | ADAPT | [V] | ✅ |
| B4 | Machine water/fuel refill (cisterns, generators; #63) | JSONB `FWaterStorageComponent.m_WaterStored`; generator fuel items | NEW | [V] structure in the sample (16 cisterns, 5 generators); [V] live PASS 2026-10-08 (#63, PR #64) | ✅ verified; wind turbines ◻ |
| B5 | Base/vehicle permissions | `permission_actor*` | N/A | multiplayer concept, **cut 2026-10-07** | never |
| B6 | Blueprints import/export | `building_blueprints` and related (`building_blueprint_*`; all empty in sample) | ADAPT | [I] | deferred: needs a save that has blueprints; low SP value (**2026-10-07**) |
| B7 | Delete base completely | cascades + trigger | ADAPT, **high risk** | [I] | **cut 2026-10-07** (demolish in game; pieces stay editable in the Database tab) |
| B8 | Fully repair a base | `building_instances.health`, `placeables.health` | ADAPT | [V] column; "full" = highest health seen for the type (in the sample every type shares one max, so this holds); [T] a type with every piece damaged has no reference | ✅ (#56) |
| B9 | Give items to base chests | `inventories`/`items` (same as B2) | ADAPT | [V] | ✅ (full stats follow S3) |
| B10 | Increase land-claim size | `landclaim_segments` (one row per extra 10x10-foundation cell, the totem's own cell (0,0) is implicit), `totems.landclaim_vertical_level` | ADAPT | [V] same tables and constants as the Dune Docker Land Claim Editor; the sample save has 1 totem, 0 extra cells; [T] write pending the in-game check (#57) | ✅ built, unverified |
| V1 | Vehicles list/bring/durability/refuel | `vehicles`, `actors` | ADAPT | [I] 0 vehicles in sample | ✅ partial (untested) |
| V2 | Repair a vehicle (modules) | `vehicle_modules.stats` JSONB `FVehicleModuleDurabilityStats` (`CurrentDurability` to `DecayedMaxDurability`) | NEW | [V] structure (9 vehicles, 75 modules); the original per-template maximum is not in the save, so decay is not undone; [V] live PASS 2026-10-08 (#56, PR #64) | ✅ verified; chassis health/wrecks ◻ |
| W1 | Resource/spice fields | `resource_nodes`, `resourcefield_state` | ADAPT | [V] rows exist | ◻ P5 |
| W2 | Sandstorm / Coriolis schedule | SQLite-only tables | NEW | [V] tables exist | parked behind P1 (**2026-10-07**) |
| W3 | Map fog reveal | `FogOfWar/*.data` bitmap | NEW | [V] format; [I] layout | parked behind P1 (**2026-10-07**) |
| W4 | Live map / partitions / autoscaler / Deep Desert instances | needs `world_partition` | N/A | [V] | never |

### Landsraad, vendors, config
| ID | Feature | SP mechanism | Class | Evidence | Status |
|---|---|---|---|---|---|
| L1 | Landsraad term/decree/tasks/rewards | `landsraad_*` | WORKS/ADAPT | [V] | ✅ |
| L2 | Simulated-guild contributions | `landsraad_task_*_contributions`, `landsraad_simulated_guilds` (60) | NEW | [V] | **cut 2026-10-07** (edits numbers the game invents) |
| E1 | Vendor stock/limits/restock | `vendor_stock_*` (empty in sample) | WORKS | [V] schema | ✅ |
| E2 | Dune Exchange market | none | N/A | [V] | never |
| C1 | Difficulty/rate multipliers | `Config\Windows\ServerCustomSettings.ini` | NEW | [V] present with custom values; [T] game applies changes | ✅ raw editor; typed editor only if the P6 live test shows SP applies the file (**2026-10-07**) |
| G1 | Guilds, parties, permissions to guild | none | N/A | [V] | never |
| A1 | Audit log, playtime, cheater tracking, Discord links | console-owned | N/A | [V] | never |

## 4. Architecture

```
Browser (localhost) --HTTP+token--> web --> ops (feature modules, composes leaf packages)
                                             |          \--> diff, catalog(data), component(takes *sql.Tx)   [leaf packages]
                                             v
                                   save (transport: working copy, Mutate, Commit, backups, probe)   config (ini)
                                             |
                                          codec (wrapper) --> game.db on disk
CLI: find | decode | encode | snapshot | diff | verify   (share codec + diff)
```

Layering rule (F-16): `codec` <- `save`. `diff`, `catalog` and `component` are **leaf** packages that never
import `save`. `component` functions take a `*sql.Tx` (never call `save.Query` inside a transaction: the
connection is single and holds `s.mu`, so that deadlocks). `ops` composes them. Name discovery from the user's
save lives in `ops`, not `catalog`. There is no `savex` package.

### 4.1 Components: what exists today, what changes
| Package | Verified today | Change |
|---|---|---|
| `internal/save/codec.go` | wrapper; flag check exists (`codec.go:33`); **`io.ReadAll` unbounded** (`:40`) so a 1 MB file can expand about 1000:1 | S0: `io.LimitReader(zr, min(header size, 128 MiB)+1)`, reject header over cap before decompress, cap compressed input; fuzz target; tests for bomb, truncation, size mismatch, wrong flag |
| `internal/save` | temp file on disk (not memory), single connection, `foreign_keys(0)` explicit, `Commit` with integrity check, hash check, backup, tmp+rename, `force` skips two checks, `Restore` truncates in place (as audited 2026-09-30; **since fixed**, see the 4.2 implementation status) | S1: see 4.2 and F-05..F-08 |
| `internal/ops` | 32 write sites, 22 outside any Tx, errors ignored in multi-step ops, `Exec`/`ExecScript` reachable from the SQL console with a regex denylist (as audited; **since fixed**: `save.Mutate` only, tokenizer-based vetting) | S1: `save.Mutate(desc, fn(tx))` is the only write path; migrate all sites; lint test forbids `S.Exec` from `ops` |
| `internal/web` | Host check, per-run token (128-bit), 4 MiB POST cap, no CORS; **`GET /` is token-exempt and serves the token; no CSP; no Content-Type check; `--addr` unrestricted** | S0: see 4.4 |
| `internal/web/static/index.html` | many unescaped interpolations of save values (stored XSS); review pane built (S2 slice 1, #50); bare `confirm()` remains; no `beforeunload` | S0 escape + CSP; S2 review pane, confirmations, banners |
| **`internal/diff`** (new, leaf) | Python prototype `tools/snapdiff.py` (fixed in v0.2) | S1: Go port with redaction on by default; used by CLI, review pane, tests |
| **`internal/component`** (new, leaf) | n/a | S3: typed JSONB editor; path is `[]string`, never a concatenated string (F-18) |
| **`internal/catalog`** (new, leaf) | n/a | S3: data only (embedded MIT JSON with attribution, conditional on provenance check) |
| `internal/config` | ini edit with backup, regex name allowlist | P6: typed schema for `ServerCustomSettings.ini` |
| `cmd/tabr-tau` | `serve`, `find`, `decode`, `encode` | add `snapshot`, `diff`, `verify`; `--addr` policy (S0) |

### 4.2 Data-safety pipeline (every write): target design
Steps marked **(exists)** are in the code today; **(new)** are not.
1. **(done, #52/#54/#65)** Refuse while a single-player session is active: the game process alone is not enough (it stays up in multiplayer and at the menu), so the editor reads the game's log for the session start and end markers (see `docs/design` issue #65), plus a 15 s wait after the end for the game's last write; if it cannot tell, it blocks. The UI polls and disables Save with the reason.
2. **(exists)** Refuse if `game.db` changed on disk since load (hash). **(new)** re-check immediately before the rename.
   `force` skips **only** the process check and requires a typed reason that is logged; it never skips the hash check (F-07).
3. **(new)** Edits happen only inside `save.Mutate(desc, fn(tx))` on the working copy, recorded as structured pending ops.
4. **(new)** Post-edit invariants: schema fingerprint unchanged; `PRAGMA integrity_check` **(exists at commit)**;
   `PRAGMA foreign_key_check` before/after with a violation-count diff (fail on any new violation) (F-08);
   touched JSON paths still parse and `typeof='blob' AND json_valid(col,8)` for JSONB columns; `items_id_sequencer.next_id > max(items.id)`.
5. **(new)** Show the diff between the retained pristine `orig` copy and the working copy in a review pane. Save is reachable only from that pane (F-06).
6. **(exists, hardened)** Backup the original: **(new)** unique nanosecond name, `fsync`, decode-and-compare verify, retention (default keep 20). Sidecar JSON and pinned backups were **cut 2026-10-07**.
7. **(exists, hardened)** Encode; round-trip decode-compare; write `.tmp` mode 0600 with exclusive create; **(new)** `fsync`; rename with retry/backoff (Windows holds by AV/OneDrive/game); clean stale `.tmp` at open.
   Honest claim: rename is crash-safe against **process** death; power-loss safety requires the `fsync`.
8. **(new)** Reopen the written file and re-run invariants. On failure restore the verified backup automatically and report. A reload error after a successful rename is non-fatal and reported as a warning.
9. **Restore** uses the same pipeline (tmp+sync+rename, integrity check, patch-set subset check, hash check); it never truncates the live file in place.

**Implementation status (2026-10-07).** Done: step 2 (hash re-checked immediately before the rename; `force` never skips it);
step 3 (`save.Mutate` is the only write path, `Exec`/`ExecScript`/`Tx` removed so the compiler enforces it); step 4 partly
(fingerprint checked at load and refuses all writes; `foreign_key_check` gate with a per-row baseline diff on every edit);
step 6 partly (unique fsynced read-back-verified 0600 backups; sidecar JSON and retention not built); step 7 (exclusive 0600
temp, fsync, rename retry, read-back compare, stale-temp cleanup); step 8 partly (a read-back mismatch restores the previous
bytes; reopening and re-running invariants is not built; a reload failure after a successful write is a warning); step 9
(Restore through the same pipeline, no in-place truncation, integrity and patch-subset checks).
**Decision recorded 2026-10-07 (F-08):** item deletes use `MutateCascade`, which switches `foreign_keys=ON` for that one
edit so the delete cascades exactly as the schema declares (every FK on `items`/`inventories` is `ON DELETE CASCADE`),
instead of hand-written child deletes. This departs from the earlier wording ("explicit child deletes, decide `foreign_keys`
after T4") because explicit deletes would re-implement the schema's cascade by hand and miss references; the default
connection setting stays `foreign_keys=0` and the global decision still waits on T4.
Step 5 data side done (F-06): pristine baseline kept at load, structured op record, `internal/diff`, review endpoint and
`diff` CLI. Step 5 UI side done (S2 slice 1, PR #50): the review pane, Save reachable only from it (the commit route requires the
review token, a hash of the pending edits), a polled game-running banner, sticky errors, and Reload from disk after a
changed-on-disk failure. S2 slice 2 adds tiered in-app confirmations (typed word for bulk, raw SQL write and restore). **Not built yet:** the typed reason for `force` (the UI never sends `force`), JSON/JSONB invariants, post-write verification against the baseline.

### 4.3 Compatibility strategy
- **Capability probe:** each feature declares required tables, columns and JSON paths; unavailable features are shown disabled with the reason (F-21), and their writes are refused.
- **Fingerprint** (F-19): sorted `applied_patches` names **plus** a hash of `sqlite_master` (currently 186 objects: 96 tables, 89 indexes, 1 trigger). `applied_patches` dates are a synthetic counter and are not used. Any unknown trigger, view, virtual table or extra table refuses **all** writes (F-03) and shows why.
- Unknown newer fingerprint: banner "save is newer than tested", read-only for JSONB writes, snapshot/diff still allowed.
- Restore refuses a backup whose patch set is not a subset of the current one.
- `game_prepatch.db` and the autosave slots are fixtures for migration tests.

### 4.4 Security and privacy: current state versus target
Current state (verified by the audit):
- The server binds where `--addr` says; default `127.0.0.1:8090`. **`--addr 0.0.0.0:8090` would expose the editor** because the Host check accepts `Host: localhost` from any peer and `GET /` returns the token.
- No CSP/frame protection; token compare is `!=`; no Content-Type/Origin check; only `ReadHeaderTimeout`.
- `trusted_schema`, `query_only` (for the SQL console) and `load_extension` restrictions are **not** set on the main connection; the SQL "read-only" mode toggles `PRAGMA query_only` on the shared connection and can be turned off by the user's own SQL; `ATTACH` is possible on the read path; `Tables()` interpolates save-controlled identifiers.
- Save values are inserted into the page without escaping in many places (stored XSS from a hostile save).
- Backups/temps are 0644; no symlink checks; the decoded save sits in the OS temp dir (0644 by default on some systems).

Target (Phase S0, before any new feature):
1. Refuse a non-loopback `--addr` (the `--allow-remote` override is **cut 2026-10-07**: remove the flag, see the scope-cut section);  additionally reject requests whose `RemoteAddr` is not loopback; never serve the token to a non-loopback peer. Default to a random free port (fallback), IPv4 loopback only (documented: `[::1]` is not bound).
2. Headers on every response: `Content-Security-Policy: default-src 'self'; script-src 'self'; frame-ancestors 'none'`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Cross-Origin-Resource-Policy: same-origin`. The token is delivered once via an HttpOnly, SameSite=Strict cookie set at bootstrap, not embedded in inline script; inline JS moves to a file.
3. POST requires `Content-Type: application/json`; constant-time token compare; check `Origin`/`Sec-Fetch-Site`; handle `rand.Read` errors; Read/Write/Idle timeouts.
4. Output encoding: every interpolated value goes through `esc()` or DOM `textContent`; a test loads a hostile-string fixture.
5. Two SQLite connections: the write path (working copy, `trusted_schema=OFF`) and a separate `mode=ro` connection **with an authorizer** for browse and read SQL (allow `SELECT`/`WITH` only, single statement, no `ATTACH`/`PRAGMA`/`VACUUM`); the write console goes through `Mutate` with statement-kind allowlist, blocks PRAGMA/BEGIN/COMMIT/ATTACH/VACUUM, and requires typed confirmation plus the review pane.
6. Identifier helper doubles quotes and applies `identRe` before any use, including names read from `sqlite_master`.
7. Untrusted-save handling: bounded decode (4.1), schema fingerprint (4.3), JSON depth 32 / size 1 MiB limits for components, refuse to open an online-server cache folder (tested; today only asserted).
8. Files: 0600 for backups/temps, `Lstat` refusal of symlinks in the backup dir and target, exclusive create for temps; a private temp directory cleaned at start (crash leaves the decoded save behind otherwise).
9. Privacy: IDs masked in the UI and diff output by default; `.gitignore` and gitleaks cover every ID-bearing artifact type (F-11); Requirement 24: the token and file paths never go to a persisted log; CSV export prefixes cells starting with `= + - @`.
10. No outbound network: enforced by a CI check that forbids `net/http` client use outside the server; no telemetry, no auto-update.
Local processes and other Windows users can still `GET /` and read the token: the token stops browser-origin attacks only. This is stated in the README.

### 4.5 Testing strategy (F-02)
- **Contract fixture from the real DDL:** a script emits `ddl.sql` from `select sql from sqlite_master` of a real save (schema only, no rows, committed); a test builds fixtures from it and **fails if any table the ops code touches is missing or a column differs**. Row data is synthetic and generated, with all ID columns replaced (`platform_id`, `funcom_id`, `platform_name`, names) and the output scanned by gitleaks.
- **Every mutator test** asserts on the resulting full row **and** `integrity_check`, `foreign_key_check`, the diff engine output, and the STRICT/CHECK behavior.
- Required tests (from the QA audit): `TestContractFixtureMatchesRealDDL`, `TestGiveItemOnRealSchema`, `TestGiveFullInventoryRealSchema`, `TestDeleteParity_FKOnVsOff`, codec (`Bomb`, `Truncated`, `SizeMismatch`, `WrongFlag`, `FuzzDecode`, round-trip at real size), `TestCommitCrashWindows` (fault injection through an FS interface at backup / tmp write / rename), `TestCommitTwiceSameSecond`, `TestCommitRefusesOnForeignKeyCheckFailure`, `TestJSONBSetQuotedTag`, `TestConfigRoundTripNoOp`, `TestHostHeader`/`TestRemoteAddrRefused`/`TestNoTokenOnGetRootForRemote`, `TestHostileSaveXSS`, a restore-drill test.
- **Web layer:** table-driven `httptest` cases (currently none exist).
- **Live:** `docs/live-test-protocol.md` v2 (pre-registered); results in `docs/evidence/` (redacted).
- **CI (P0):** `gofmt` check, `go vet`, `staticcheck`, `go test -race -cover`, short `-fuzztime` for codec, `govulncheck`, `go mod verify`, PSScriptAnalyzer for `snapshot.ps1`, a Windows runner for `GameRunning()`, shared `reusable-security-scan.yml` with explicit secrets only (never `secrets: inherit`), gitleaks with a 17-digit-ID rule.

## 5. Implementation plan

Phases are gated; no phase starts feature work until its gate evidence exists. Order: **P0 -> S0 -> S1 -> S2/P1 -> S3 -> P3 ...**

### P0 Governance (blocks everything; F-09, F-12, F-11)
Sequence matters (an ordering mistake makes required checks unsatisfiable):
1. Enable native secret scanning, push protection, Dependabot alerts/updates (free, public repo).
2. Commit `.gitignore` additions (`*.data`, `*.meta`, `*.log`, `*.dmp`, `*.png`, `*.vdf`, `manifest.json`, `snapshots/`, `docs/evidence/**` except redacted `RESULTS.md`, `*.sql` dumps), `.gitleaks.toml` (17-digit ID, `funcom_id`), commit-msg check (no AI co-author trailers), PR template with documentation-impact and risk classification, `CODEOWNERS`, labels (`severity:*`, `stride:*`, `security`), issue templates, `CHANGELOG.md`.
3. Add CI (workflows with `permissions: contents: read`, SHA-pinned actions, `sha_pinning_required`), then branch protection with `<job> / <inner>` check names.
4. `meta` PR: repo added to Directory Layout, Repo-by-Repo Notes, Requirement 15 board list, Requirement 28 watched repos and the CI-adoption list; add the repo to the Project Arrakis board.
5. File the findings register as issues plus a tracking issue with the STRIDE table (Requirement 20); replace this interim file's status column with issue links.
6. Operator decisions F-01 (credential exposure; details withheld) and F-12 (history rewrite).
Acceptance: green CI on `main`; protection on; scanners on; CHANGELOG; findings issues filed; README PR merged.

### S0 Security hardening of the existing code (F-03, F-04; before any new feature)
Tasks: 4.4 items 1-10. Acceptance: each item has a failing-first test; `TestHostileSaveXSS`, `TestDecodeBomb`, `TestRemoteAddrRefused`, `TestReadOnlyConsoleCannotWrite`, `TestUnknownTriggerRefusesWrites` pass; manual check that `--addr 0.0.0.0` is refused.

### S1 Write pipeline (F-05..F-08)
`save.Mutate`, pending-op record, retained `orig`, Go `internal/diff`, invariants, durable Commit/Restore, `foreign_key_check` gate, cascading deletes for container items (see the 2026-10-07 decision in 4.2), unique backups, retention, `snapshot`/`diff`/`verify` CLI. Acceptance: fault-injection tests pass; a lint test forbids `S.Exec` from `ops`; the existing ops tests migrated onto the real-DDL fixture (F-02).

### S2 UX safety (F-10, F-21)
Review pane (Save only from it), tiered confirmations (typed for destructive/bulk), persistent game-running banner with polling, `beforeunload`, sticky commit errors with "reload and re-apply", in-browser save picker, error taxonomy and status codes, toast wording "Queued (not in game yet)", Backups and history page, "not available in single-player" catalog, accessibility pass, ID masking. Acceptance: a written flow for each state (first launch, multiple saves, game running, stale save, capability degraded) with a screenshot or scripted UI test.

### P1 Verify by experiment (runs in parallel with S0-S2; gates S3)
Run `docs/live-test-protocol.md` v2: T1, T1b, T2, T3, T4, T5, T6. Each ends with a PASS/FAIL/PARTIAL line and redacted evidence.

### S3 Component editor and player parity (gated by P1)
`internal/component` (segments, `jsonb_set` only, invariants, limits), `internal/catalog` (provenance-checked), P1 give with canonical stats and per-template stack cap and volume check, P6/P7/P8/P13/P14. Decision on `foreign_keys` from T4. Acceptance: every feature has a real-DDL test, a diff assertion and a live PASS.

### P4 Bases and machines / P5 World and Landsraad / P6 Config
As in v0.1 section 3. P4 entry gate: the operator provides a test save with vehicles, blueprints and permissions (audit ARCH-8). B7 (delete base) requires the dry-run diff and typed confirmation.

### Release gate (Requirement 20)
Layer 2 audit at the end of S1, S3, P4 and P5; Layer 3 before each tagged release. Semver `v0.x`; CI builds with `-trimpath`, publishes `SHA256SUMS`; the README states the binary is unsigned (attestation and Authenticode signing **cut 2026-10-07**).

## 6. Risks
| ID | Risk | L | I | Mitigation |
|---|---|---|---|---|
| R1 | Game rejects/normalizes our JSONB or rows | med | high | P1 gate (T1-T3), automatic restore, small edits |
| R2 | Patch changes schema; silent corruption | med | high | fingerprint + read-only degrade + restore patch-subset check |
| R3 | Game/Steam Cloud/autosave overwrites edits | med | med | game-running check, hash check, T5 experiment, Steam Cloud noted in protocol |
| R4 | Cascade/trigger side effects on deletes | med | high | `foreign_key_check` gate + per-edit cascading deletes (`MutateCascade`) now; global `foreign_keys` decision from T4 |
| R5 | **Hostile shared save (memory bomb, XSS, hostile triggers)** | **med** (raised from low: audit chained SEC-1..4) | high | S0 |
| R6 | Privacy leak of IDs (git, cloud, screenshots, diffs) | med | med | F-11 controls; masking by default |
| R7 | Catalog names stale/unlicensed | high | low | show raw IDs; provenance check; fallback to names from own save |
| R8 | Unverifiable features without matching save data | high | low | operator test save; experimental badge |
| R9 | ToS/anti-cheat exposure | low | med | expanded disclaimer; single-player only; **built and tested** refusal of online-server folders |
| R10 | Tests pass but prove nothing about the real game | high | high | real-DDL contract fixture (F-02) |
| R11 | Credential exposure through the test-data transfer path (details withheld) | realized | high | F-01: revoke, token-free transfer, data-lifecycle rule |
| R12 | Governance gaps on a public repo (no protection, no scanning) | realized | med | P0 first |

## 7. Open questions (unchanged questions are answered only by P1 evidence)
1. Does the game run with `PRAGMA foreign_keys=ON`? (T4)
2. Does the game rebuild JSON components on load, discarding our edits? (T1-T3)
3. Where does fill level live for liquid containers? (Phase B)
4. Does `Config\Windows\ServerCustomSettings.ini` apply to single-player at runtime, and when is it re-read? (P6 test)
5. Which file is authoritative (`game.db` vs `autosave/N.bak`), and does Steam Cloud interfere? (T5)
6. Is `items.volume_override` used in single-player (NULL in all 65 sample items)?
7. Catalog provenance and per-file licence/attribution; is Funcom-derived content present? (GRC-5)
8. ~~Does `modernc.org/sqlite` v1.60.1 bundle SQLite >= 3.45?~~ **Answered [V, inferred from the cached older v1.56.0 whose CHANGELOG states SQLite 3.53.3; v1.60.1 is newer].** Still add a startup/test assertion on `select sqlite_version()` >= 3.45, because the scratch JSONB write test used Python's SQLite 3.46.1, not the Go driver.
9. Does `player()` resolve the pawn the way the game links it (`player_state.id` is the controller id, not the pawn id)? (QA-2d)

## 8. Scope cuts for single-player (decided 2026-10-07)

The operator reviewed the roadmap for items that make no sense without a server. Decisions:

**Cut:** B5 permissions; B7 delete base; L2 simulated-guild contributions; the `--allow-remote` flag; the Exchange tab
(Solari stays on Player, vendor restock moves into Player or World); the typed `force` reason (the API no longer takes
`force`); sidecar backup JSON, pinned backups and retention tiers (keep the last N, restore one); Authenticode signing and
build-provenance attestation; the codec fuzz target and `-race` matrix.

**Shrunk:** #48 keeps the current Bases and Vehicles tabs and fixes only real usability problems instead of copying the
Docker console layout; B6 blueprints wait for a save that has some; P6 typed config waits for the live test; ID masking
applies to diff and CLI output, not the UI; the in-browser save picker serves `--web` only; the error taxonomy covers only
what the UI shows; W2 and W3 are parked behind P1.

**Kept, with new tasks:** Refill Container stays (P8, #55). Done 2026-10-08: the fill is the item's own `FFillableItemStats.CurrentAmount`, written at the template's capacity (live PASS). Bases and vehicles get full repair (B8, V2, #56), base chests
get items (B9, done as B2), and the land claim can grow (B10, #57). Code and doc removal of the cut items is tracked in #58.

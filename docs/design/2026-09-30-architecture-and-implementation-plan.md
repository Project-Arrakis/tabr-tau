# tabr-tau: architecture and implementation plan (v0.1 DRAFT, pre-audit)

Status: DRAFT for the Layer 1 (design) eight-hat audit required by Project-Arrakis Requirement 20.
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
| P8 | Hydration, spice, health | RMQ `UpdateAllWaterFillables` | `actors.gas_attributes` JSONB, `FHealthComponent` | NEW | [V] read; [T] write | ◻ P3 |
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
| B4 | Machine water/fuel/power refill | JSONB machine components | NEW | [V] structure seen; [T] write | ◻ P4 |
| B5 | Base/vehicle permissions | `permission_actor*` (dune-docker joins missing `map_names`) | ADAPT | [V] tables, 16 rows | ◻ P4 |
| B6 | Blueprints import/export | `building_blueprint*` (empty in sample) | ADAPT | [I] | ◻ P4 (needs test save) |
| B7 | Delete base completely | cascades + trigger | ADAPT, **high risk** | [I] | ◻ P5 |
| V1 | Vehicles list/bring/durability/refuel | `vehicles`, `actors` | ADAPT | [I] 0 vehicles in sample | ✅ partial (untested) |
| W1 | Resource/spice fields | `resource_nodes`, `resourcefield_state` | ADAPT | [V] rows exist | ◻ P5 |
| W2 | Sandstorm / Coriolis schedule | SQLite-only tables | NEW | [V] tables exist | ◻ P5 |
| W3 | Map fog reveal | `FogOfWar/*.data` bitmap | NEW | [V] format; [I] layout | ◻ P5 |
| W4 | Live map / partitions / autoscaler / Deep Desert instances | needs `world_partition` | N/A | [V] | never |

### Landsraad, vendors, config
| ID | Feature | SP mechanism | Class | Evidence | Status |
|---|---|---|---|---|---|
| L1 | Landsraad term/decree/tasks/rewards | `landsraad_*` | WORKS/ADAPT | [V] | ✅ |
| L2 | Simulated-guild contributions | `landsraad_task_*_contributions`, `landsraad_simulated_guilds` (60) | NEW | [V] | ◻ P5 |
| E1 | Vendor stock/limits/restock | `vendor_stock_*` (empty in sample) | WORKS | [V] schema | ✅ |
| E2 | Dune Exchange market | none | N/A | [V] | never |
| C1 | Difficulty/rate multipliers | `Config\Windows\ServerCustomSettings.ini` | NEW | [V] present with custom values; [T] game applies changes | ✅ raw editor; ◻ P6 typed |
| G1 | Guilds, parties, permissions to guild | none | N/A | [V] | never |
| A1 | Audit log, playtime, cheater tracking, Discord links | console-owned | N/A | [V] | never |

## 4. Architecture

```
Browser (localhost) --HTTP+token--> web  --> ops (feature modules) --> savex (component editor, catalog, schema probe)
                                                         |                       |
                                                         v                       v
                                            save (working copy, tx, commit)  config (ini)
                                                         |
                                                       codec (wrapper) --> game.db on disk
CLI: find | decode | encode | snapshot | diff  (share codec + savex + diff engine)
```

### 4.1 Components (existing -> planned)
| Package | Today | Change |
|---|---|---|
| `internal/save/codec.go` | Decode/Encode wrapper; **[V] `Decode` uses unbounded `io.ReadAll` on the zlib stream** | add decompression size cap (128 MiB) and header tag check as a hard error; fuzz test |
| `internal/save` | working copy, tx, Commit, backups | add: schema/patch capability probe, foreign-key parity, pre/post-write verification, backup retention, dry-run diff preview |
| `internal/ops` | feature functions using raw SQL | split by domain; every mutator goes through one `mutate()` that records a pending-op with a diff |
| **`internal/component`** (new) | n/a | typed read/edit of JSONB components: `Entity(actor, slot)`, `Get/SetPath`, schema-aware setters with range validation |
| **`internal/catalog`** (new) | n/a | item/skill/vehicle catalogs: embedded MIT JSON from dune-docker (attribution) + names discovered from the user's own save; unknown IDs display as raw IDs |
| **`internal/diff`** (new) | `tools/snapdiff.py` prototype | Go port: row + JSON-path diff, used by CLI `diff`, the UI's "review changes" pane, and tests |
| `internal/config` | ini edit with backup | add typed schema for `ServerCustomSettings.ini` with bounds and presets |
| `internal/web` | routes, token, host check | add CSRF-safe method rules, request size limits, no CORS |
| `cmd/tabr-tau` | serve, find, decode, encode | add `snapshot`, `diff`, `verify` |

### 4.2 Data-safety pipeline (every write)
1. Refuse if the game process is running (existing). Also refuse if `game.db` mtime/hash changed since load (existing).
2. Apply edits in one SQLite transaction on the private working copy. **[V] Today the connection sets
   `foreign_keys(0)` explicitly (`internal/save/save.go:74`), so `ON DELETE CASCADE` does not fire for tabr-tau
   deletes, while the schema's trigger still does.** Decision needed: `foreign_keys=ON` to match the game's
   cascade behavior (**[T]** confirm what the game uses).
3. Post-edit invariants (new): schema probe still matches; `PRAGMA integrity_check` and `foreign_key_check`
   clean; touched JSON paths still parse; `items_id_sequencer` > max item id; counts of unrelated tables unchanged.
4. Show the diff (`internal/diff`) between original and working copy before commit (new).
5. Backup original to `tabr-tau-backups/` with retention (default keep last 20) (existing + retention).
6. Encode, round-trip decode-compare, write `.tmp`, atomic rename (existing).
7. Reopen the written file and re-run invariants; on failure restore the backup automatically (new).

### 4.3 Compatibility strategy
- **Capability probe** (like dune-docker's `supports*`): each feature declares required tables, columns and JSON paths;
  at load the probe marks features available/degraded/unavailable. Writes are disabled for a feature whose probe fails.
- **Patch fingerprint:** read `applied_patches` (25 rows today). Unknown newest patch -> banner "save is newer
  than tested", read-only for JSONB writes, still allow snapshot/diff. [V table exists; T behavior across a real patch]
- `game_prepatch.db` is available as a fixture source for schema-migration tests.

### 4.4 Security and privacy
- Server binds `127.0.0.1` only, rejects other `Host` headers, requires a per-run token on every API call (existing).
- State-changing routes are POST only; JSON content-type required; request bodies size-limited.
- Untrusted input is the **save file** (users share saves): decompression cap, SQLite opened from memory copy with
  `trusted_schema=OFF`, `query_only` for browse, no `load_extension`, JSON depth/size limits.
- Config filenames come from a fixed allowlist; no path parameters reach the filesystem unchecked.
- Write SQL console: kept, but requires explicit confirmation and always runs inside the same pipeline.
- Privacy: `Game.ini`/`game.db`/logs contain platform IDs and display names. Never logged, never included in
  diff output by default (redaction list), never committed (`.gitignore`, gitleaks rule for 17-digit IDs).
- No telemetry, no outbound network calls, no auto-update.

### 4.5 Testing strategy
- Unit: codec round-trip (+ fuzz), JSONB set/get, catalog lookup, probe logic, diff engine.
- Contract: a **sanitized minimal real-schema fixture** (schema DDL + a few synthetic rows, generated by script from
  the real save, no account data) so tests catch drift from the real schema, not just from our own synthetic tables.
- Regression: the two known failure classes from dune-docker's audits: tautological tests and mocks that diverge
  from the real schema. Every mutator test asserts on the resulting rows *and* the diff engine output.
- Live: `docs/live-test-protocol.md` results recorded as evidence in `docs/evidence/`.
- CI: `go vet`, `go test -race`, shared `reusable-security-scan.yml`, `govulncheck`, gitleaks (incl. custom ID rule).

## 5. Implementation plan

Phases are gated. No phase starts its feature work until its gate evidence exists.

### P0 Repo hygiene (blocks everything; Requirements 1, 10, 13, 14, 21)
- Branch protection; CI (`go vet`, `go test -race`, shared security scan, govulncheck); `CHANGELOG.md`; issue templates and
  labels (`severity:*`, `stride:*`); `.gitleaks.toml` with a 17-digit-SteamID rule; add repo to the Arrakis project board and `meta` README repo lists;
  commit-message policy: no AI co-author trailers (operator standing rule).
- Acceptance: green CI on `main`; CHANGELOG present; issues enabled; README listing PR merged in `meta`.

### P1 Verify by experiment (gate for P2-P4)
- T1 literjon protocol (give / game-normalizes / fill / drink) -> evidence: where fill level lives, whether a tabr-tau row survives a game load.
- T2 XP/skill-point edit accepted by the game; T3 hydration edit accepted; T4 `foreign_keys` behavior; T5 noise floor.
- Acceptance: each T has a snapshot set, diff output, screenshot, and a PASS/FAIL line in `docs/evidence/`.

### P2 Core hardening
- Go `internal/diff`; `snapshot`/`diff`/`verify` CLI; codec caps + fuzz; capability probe + patch fingerprint;
  pipeline steps 3, 4, 7 of section 4.2; backup retention; `PRAGMA foreign_keys` decision from T4.
- Acceptance: kill-during-write test leaves a loadable file or a restorable backup; probe blocks a save with a renamed column.

### P3 Player parity
- `internal/component` + P6, P7, P8, P13; catalog + P2 item search; P1 give with correct `stats`; P14.
- Acceptance: each feature has a unit test, a diff-engine assertion, and a live PASS from P1.

### P4 Base and machine parity
- B4, B5, B6, P4 augments. Requires a test save containing vehicles, blueprints and permissions (operator to create).

### P5 World and Landsraad extras
- W1, W2, W3, L2, B7 (with extra confirmations and a mandatory dry-run diff).

### P6 Config
- Typed `ServerCustomSettings.ini` editor with bounds/presets; verify in game (T-config).

### Release gate (Requirement 20)
Layer 2 audit per phase; Layer 3 before each tagged release. Semver, tag `v0.x`; Windows binary with published SHA-256.

## 6. Risks
| ID | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| R1 | Game rejects/normalizes our JSONB or rows -> lost edits or crash on load | med | high | T1-T3 gate; auto-restore; small edits |
| R2 | Patch changes schema -> silent corruption | med | high | probe + patch fingerprint + read-only degrade |
| R3 | Game holds save in memory / autosave overwrites edits | med | med | game-running check; on-disk-change check; instruct to fully exit |
| R4 | Cascade/trigger side effects on deletes | med | high | `foreign_keys` parity (T4); dry-run diff; tests on real-schema fixture |
| R5 | Malicious shared save | low | med | caps, in-memory open, `trusted_schema=OFF` |
| R6 | Privacy leak of platform IDs via logs/screenshots/diffs | med | med | redaction, gitleaks rule, docs |
| R7 | Catalog names go stale after patches | high | low | show raw IDs when unknown; catalog is data, updatable |
| R8 | Feature gaps unverifiable without matching save data (vehicles, blueprints) | high | low | operator-made test save; features stay flagged experimental |
| R9 | ToS/anti-cheat concerns for editing saves | low | med | single-player only; refuse online-server folders; README disclaimer (exists) |

## 7. Open questions
1. Does the game run with `PRAGMA foreign_keys=ON`? (T4)
2. Does the game rebuild JSON components from other columns on load, discarding our edits? (T1-T3)
3. Where does fill level live for liquid containers? (T1)
4. Does `Config\Windows\ServerCustomSettings.ini` apply to single-player at runtime, and when is it re-read? (T-config)
5. Autosave ring: do slots 0/1 or 9 hold the newest state, and does the game ever prefer `.bak` over `game.db`?
6. Is `items.volume_override` used in single-player (NULL in all 65 sample items; dune-docker sets it per unit)?
7. Item catalog licensing/attribution for redistribution (MIT upstream; confirm each JSON's provenance).

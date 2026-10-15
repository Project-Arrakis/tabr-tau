# Layer 2 (implementation) audit, 2026-10-15: consolidated findings register

Date: 2026-10-15. Subject: everything built since the last Layer 2 audit (`docs/audit/2026-10-07-L2-findings-register.md`), base commit `d7caef2`, head `1ad814c`: 65 commits, 164 files (internal/ops, internal/web, internal/save, internal/journey, internal/augments, web-tests, CI). Tracking issue: #157.

Method (Project Arrakis meta Requirement 20): eight independent read-only hat workers, each dispatched with its own mandate, told to verify against real code or command output, to cite file:line and to map every finding to STRIDE (the mapping instruction was in every dispatch prompt): Architect, Security Architect, GRC, Network, Cloud Security, UI/UX, DBA, QA/Test. The workers ran experiments only in scratch copies (Go tests against `internal/testsave` real-schema fixtures, 105 + 14 mutation runs, a built copy of the app on a fixture save). 93 raw findings were merged into 25 issue groups before any fix. The consolidator spot-checked four high-impact claims against the code before accepting them (all confirmed): `writeJSON` ignores the encode error (server.go), `pickedItemId` returns raw text as a template id (app.js:77-81), `RefillGenerators` reads all placeables with no base filter (upkeep.go:118), and `Commit` rejects on a changed file hash (save.go:552-558).

No hat contradicted another on a fact. Overlaps resolved in the merge: the unreviewed start-up auto-commit (Architect, DBA, GRC, Security, UI/UX), reading outside the transaction (Architect, DBA), the whole-document stats rewrite in Repair Gear (Architect, DBA, QA), non-finite and out-of-range numbers (Security, DBA), the port-agnostic session cookie (Network, Security), the unscoped health route (DBA, Architect, GRC), and the reusable workflow at `@main` (Cloud; the earlier C-1).

## Disposition summary
- CRITICAL: none.
- HIGH: 6 groups, 6 raw findings (QA-1, QA-2, UX-01, UX-02, GRC-1, GRC-2). To be resolved before the layer is considered passed; G07 (the Funcom art) needs a decision from the owner.
- MEDIUM: 41 raw findings in 18 groups. Fixed where cheap and safe; the deferred ones carry a justification in their issue (G23 and the repository-settings part of G21).
- LOW: 46 raw findings, all filed inside their groups. None was dropped.

## Issue groups
| Group | Issue | Severity | STRIDE | Plan | Status | Title |
|---|---|---|---|---|---|---|
| G01 | #158 | HIGH | Tampering, Denial of Service | fix | Open | Save-write safety gates are not pinned by any test |
| G02 | #159 | HIGH | Tampering | fix | Open | Single-vehicle repair scope and the whole-save one-click ops are untested |
| G03 | #160 | HIGH | Repudiation | fix | Open | The editor invites editing while the game runs, then rejects the save |
| G04 | #161 | HIGH | Tampering | fix | Open | Free text in the item box is sent as a template id |
| G05 | #162 | HIGH | Repudiation | fix | Open | Master docs describe a project that no longer exists, and contradict the code |
| G07 | #164 | HIGH | N/A | operator | Open | Funcom game art ships in the MIT repository and the executable |
| G06 | #163 | MEDIUM | Repudiation | fix | Open | Which write features are verified in game is not recorded or shown |
| G08 | #165 | MEDIUM | Tampering, Repudiation | fix | Open | The start-up auto-refill commits unreviewed, saves the whole world, and grew without a recorded decision |
| G09 | #166 | MEDIUM | Tampering | fix | Open | Refills and repairs plan outside the transaction; a double-click over-fills |
| G10 | #167 | MEDIUM | Tampering | fix | Open | Repair Gear rewrites the whole stats document through float64 |
| G11 | #168 | MEDIUM | Denial of Service, Tampering | fix | Open | Non-finite and out-of-range numbers are accepted, and a JSON encode error is swallowed |
| G12 | #169 | MEDIUM | Tampering, Elevation of Privilege | fix | Open | Unscoped or dead write routes remain after the UI boxes were removed |
| G13 | #170 | MEDIUM | Denial of Service | fix | Open | RepairBuildings is quadratic and holds the save lock |
| G14 | #171 | MEDIUM | Tampering | fix | Open | Vehicle repair can lower a recorded maximum; the durability table is thinly tested |
| G15 | #172 | MEDIUM | Tampering | fix | Open | Land-claim shrink can strand cells; two expand paths drifted |
| G16 | #173 | MEDIUM | N/A | fix | Open | The Tables view and cell editor fail on 61 of the 94 tables |
| G17 | #174 | MEDIUM | Denial of Service, Information Disclosure, Tampering | fix | Open | Hostile-save robustness and identifier exposure: panics, size cap after read, CSV header, path leaks, unmasked ids |
| G18 | #175 | MEDIUM | Denial of Service, Information Disclosure, Spoofing | fix | Open | A silent port fallback and a port-agnostic session cookie |
| G19 | #176 | MEDIUM | Tampering | fix | Open | The game-running guard assumes the save is on this machine |
| G20 | #177 | MEDIUM | Tampering, Information Disclosure, Elevation of Privilege | fix | Open | Smaller hardening, supply chain and disclosure channel |
| G21 | #178 | MEDIUM | Tampering, Elevation of Privilege | operator | Open | CI trust: the reusable security workflow is referenced at @main and required checks are incomplete |
| G22 | #179 | MEDIUM | Repudiation | fix | Open | Messages that misreport state, and dead ends |
| G23 | #180 | MEDIUM | N/A | defer | Open | Accessibility and polish backlog |
| G24 | #181 | MEDIUM | N/A | fix | Open | Test hardening |
| G25 | #182 | LOW | Tampering | fix | Open | Smaller correctness items |

## Findings by hat

### Architect (11)
| ID | Sev | Group | Finding |
|---|---|---|---|
| ARCH-1 | MEDIUM | G08 (#165) | Per-base buttons (Refill generators/water, Repair all to max, Clear sand) act on every placeable and building in the save; RunStartupTasks commits that unreviewed. |
| ARCH-2 | MEDIUM | G09 (#166) | RefillGenerators reads item rows and builds the plan before Mutate; 40 trials of two concurrent calls ended with 998 Oil in 2 rows against a cap of 499. Same pattern in AddSpecXP, RepairGear, GrantCustomizations, RefillBaseWater. |
| ARCH-3 | MEDIUM | G10 (#167) | RepairGear unmarshals stats into map[string][]any and marshals it back: 64-bit integers lose precision, keys reorder, 100.0 becomes 100. |
| ARCH-10 | LOW | G25 (#182) | settings.json: unlocked load-mutate-save from two routes, Load returns empty on a parse error so the next Save wipes the other field, trackNumbers is global not per save. |
| ARCH-11 | LOW | G25 (#182) | livemap storageTypes depends on slice order; lmMount adds a window mouseup listener on every render; MaxSkills sits in catalogsets.go. |
| ARCH-4 | LOW | G11 (#168) | Teleport returns ok when it changed nothing (pawn id 0); ResetSpec toasts "reset" with removed:0. |
| ARCH-5 | LOW | G25 (#182) | JourneySet on a node with child rows but no own row returns ok and writes nothing for the node; LIKE treats _ as a wildcard. |
| ARCH-6 | LOW | G08 (#165) | Two definitions of a power device's fuel inventory (baseview.go inventory_type=3 vs upkeep.go first inventory). |
| ARCH-7 | LOW | G12 (#169) | db.go says items_id_sequencer is locked for the SQL write too; sqlvet.go does not block it and a test asserts it is allowed. |
| ARCH-8 | LOW | G15 (#172) | ExpandLandClaim (rings 1-5) and ApplyLandClaim (+-128 cells) duplicate the have-set and level logic; landclaim.go and app.js text still say cells are never removed. |
| ARCH-9 | LOW | G12 (#169) | /api/items/update, /api/items/delete and /api/bases/give take any id; narrowing in #92/#75 was not applied to them. |

### Security (10)
| ID | Sev | Group | Finding |
|---|---|---|---|
| SEC-1 | MEDIUM | G11 (#168) | writeJSON ignores the Encode error after sending 200: one row holding Inf (e.g. actors.location_x = 9e999) makes Player, Live Map and Database views return 200 with an empty body. |
| SEC-2 | MEDIUM | G11 (#168) | Args.Float accepts "NaN"/"Inf"; SetPieceHealth, SetRecoveredDurability, SetTaskProgress and SetSpec store them (NaN becomes NULL, Inf is committed). |
| SEC-10 | LOW | G17 (#174) | respond returns raw err.Error(), leaking absolute paths with the Windows user name. |
| SEC-3 | LOW | G11 (#168) | Args.Int is int64(float64) with no check: 1e19, "NaN" and -1e300 all give MinInt64; ids above 2^53 address the wrong row. |
| SEC-4 | LOW | G17 (#174) | CSV export writes header cells unescaped: a column named =HYPERLINK(...) exports as an executable formula. |
| SEC-5 | LOW | G17 (#174) | Unchecked type assertions panic on NULL columns (e.g. /api/bases/water with a NULL building_type); net/http recovers but the client gets a reset connection. |
| SEC-6 | LOW | G17 (#174) | os.ReadFile runs before the maxDecoded check; a multi-GB file is fully read first; a 256 MB save exists as about four copies. |
| SEC-7 | LOW | G18 (#175) | Same cookie finding; another local server could also plant a shadow cookie. |
| SEC-8 | LOW | G20 (#177) | tasklist and rundll32 are found through PATH; a shadowed tasklist.exe could report "no game". |
| SEC-9 | LOW | G08 (#165) | autoRefillOnOpen turns on with one click and no confirmation; the setting is not logged. |

### GRC (14)
| ID | Sev | Group | Finding |
|---|---|---|---|
| GRC-1 | HIGH | G07 (#164) | internal/web/static/maps (4.4 MB: two map pictures and 62 icons) is embedded in the binary; LICENSE/README say MIT with no carve-out; #21 still open; no recorded owner decision. |
| GRC-2 | HIGH | G05 (#162) | docs/README.md:3 says implementation not started and continuation-prompt.md says nothing from the plan is implemented, CI/branch protection not done, token served at GET / (all false: S0-S2 merged, 8+ CI jobs, cookie). |
| GRC-10 | MEDIUM | G05 (#162) | The plan status table, feature-inventory.md and the comparison doc are stale (augments, XP, skills, faction, wind turbines, Live Map, vehicle maxima rated as gaps; catalog count 2551 vs 2558). |
| GRC-3 | MEDIUM | G06 (#163) | "Live PASS" claims have only issue-comment evidence; the plan's own S3 gate was passed with T1-T3 open and no recorded waiver. |
| GRC-4 | MEDIUM | G06 (#163) | The README lists the Player tabs with no verification status; "not checked in game" lives only in long CHANGELOG bullets. |
| GRC-5 | MEDIUM | G17 (#174) | The Player Summary header prints platform, Funcom and FLS ids unmasked on every Player sub-tab (summary.go:31, app.js:289), while the review pane masks them and README says saves contain them; screenshots for bug reports leak them (F-11, #12 is the same theme). |
| GRC-6 | MEDIUM | G05 (#162) | CHANGELOG omits the Go 1.26.9 raise (GO-2026-6617) and several merged PRs; line 108 says 1.26.6. |
| GRC-7 | MEDIUM | G05 (#162) | CHANGELOG contradicts itself: duplicated Live Map and Phase E entries, repeated headings inside [Unreleased], Extras/Vendors tab claims, F-06/F-10 "still open" vs delivered, superseded --allow-remote text. |
| GRC-8 | MEDIUM | G05 (#162) | README says Vehicles can "set chassis durability" (removed in #155), "still open: checks after the write and a recovery tool" (both exist), omits the Live Map tab and trackNumbers setting. |
| GRC-9 | MEDIUM | G08 (#165) | The 2026-10-08 operator decision covered base water and generators; the auto-save now also fills turbines and windtraps, global setting, no recorded scope change. |
| GRC-11 | LOW | G07 (#164) | THIRD-PARTY-NOTICES does not cover code and tables ported from the console (XP table, water types, fuel specs, building-set groups, research logic); header says "Windows and Linux builds". |
| GRC-12 | LOW | G07 (#164) | Provenance of numbers taken from third-party sites (dune.gaming.tools, method.gg) is not recorded; fillables.go itself notes the site restricts bulk scraping. |
| GRC-13 | LOW | G05 (#162) | Issue state out of step with code: #58 and #87 closed with work remaining, #56 and #57 open though the work shipped. |
| GRC-14 | LOW | G06 (#163) | No audit record covered about 40 write paths landed since 2026-10-07 (this audit is that record). |

### Network (7)
| ID | Sev | Group | Finding |
|---|---|---|---|
| NET-1 | MEDIUM | G18 (#175) | With the default --addr a busy port silently falls back to a random one; two instances (even on the same save) run, and the second overwrites the first's host-only cookie, so the first tab gets 403. |
| NET-2 | MEDIUM | G18 (#175) | tabr_session is Path=/ and not port-bound: the browser sends the token to any other listener on 127.0.0.1, and a replay with curl is accepted (no Origin/Sec-Fetch needed). |
| NET-3 | MEDIUM | G19 (#176) | The guard uses local tasklist and a log found next to the save; on a NAS, a synced folder or another PC no Dune process is seen, Mode is "none" and a write proceeds while the game may hold the file. |
| NET-4 | LOW | G20 (#177) | "localhost" is accepted by name and resolved by the OS; a modified hosts file could bind non-loopback (backstopped by the loopback-peer check). |
| NET-5 | LOW | G20 (#177) | Browser mode hands the boot token in a URL to the default browser (single-use, 10 minutes). |
| NET-6 | LOW | G20 (#177) | WebView2 and Edge telemetry/update traffic is outside the app's control; not documented. |
| NET-7 | LOW | G20 (#177) | The windows CI job still pins checkout at v4.3.0 while other jobs use v7.0.1. |

### Cloud (10)
| ID | Sev | Group | Finding |
|---|---|---|---|
| CLD-1 | MEDIUM | G21 (#178) | ci.yml:157 uses Project-Arrakis/.github/...reusable-security-scan.yml@main; that repo's main is unprotected and the job receives SEMGREP_APP_TOKEN. Prior C-1 not fixed. |
| CLD-2 | MEDIUM | G21 (#178) | Required checks omit windows and ui; required_approving_review_count is 0 so CODEOWNERS is advisory; strict is false. |
| CLD-3 | MEDIUM | G21 (#178) | The security gates read config from the PR under test (.gitleaks.toml allowlists any testdata/ path); semgrep rule sets are fetched unpinned. |
| CLD-6 | MEDIUM | G20 (#177) | go-webview2 embeds WebView2Loader.dll files loaded from memory by an experimental PE loader, bypassing Authenticode/WDAC; no CI check of their hashes. |
| CLD-10 | LOW | G20 (#177) | No SECURITY.md and private vulnerability reporting disabled on a public repo. |
| CLD-4 | LOW | G20 (#177) | The semgrep job installs unhashed transitive pip dependencies while holding SEMGREP_APP_TOKEN. |
| CLD-5 | LOW | G20 (#177) | Tool versions pinned in workflow text that Dependabot cannot see; floating node and runner images; pending Dependabot PRs #149/#150. |
| CLD-7 | LOW | G20 (#177) | No build, sign or release process exists; future binaries would have no provenance or checksum. |
| CLD-8 | LOW | G21 (#178) | Actions settings: allowed_actions all, sha_pinning_required false, first-time-contributor approval only, 9 of 10 jobs without timeout-minutes, persist-credentials left on. |
| CLD-9 | LOW | G20 (#177) | 106 of 166 commits carry a personal email address as author. |

### UI/UX (14)
| ID | Sev | Group | Finding |
|---|---|---|---|
| UX-01 | HIGH | G03 (#160) | The banner says "You can keep editing", but Commit compares the loaded file hash (save.go:552-558) and refuses with ErrChangedOnDisk once the game has rewritten game.db (every ~5 minutes and at session end); the only recovery discards all edits. Nothing watches the file. |
| UX-02 | HIGH | G04 (#161) | pickedItemId falls back to the raw text and the server checks only a regex, so the placeholder example "Spice" (not an item; the item is Spice Melange / MelangeSpice) inserts a nonexistent template into the backpack. Same path in the give queue and base-container Add item. |
| UX-03 | MEDIUM | G22 (#179) | Admin "Repair Vehicles" is pre-filled with Repair Below 50 %, the Vehicles tab button means 100 %, and "Repaired 0" does not say why. |
| UX-04 | MEDIUM | G22 (#179) | The review pane lists edits as ids and codes ("edit item 4521", "currency 0 +100"); values only in collapsed raw JSON paths. |
| UX-05 | MEDIUM | G22 (#179) | act() appends "(not saved yet)" to config and track-number edits that are written immediately; "Save raw text" overwrites a whole file without asking. |
| UX-06 | MEDIUM | G08 (#165) | The one unreviewed disk write reports itself in a 3.5 s toast; "Last automatic save" is in memory only. |
| UX-07 | MEDIUM | G23 (#180) | render() destroys the focused control after every action; the review dialog has no focus trap or accessible name; the toast has no live region. |
| UX-08 | MEDIUM | G23 (#180) | Land-claim cells, live-map info and sortable headers are pointer-only; inline inputs are unlabeled; layer toggles have no aria-pressed. |
| UX-09 | MEDIUM | G23 (#180) | White text on the accent colour is 2.63:1 (dark) and 4.07:1 (light); other computed contrast ratios below 4.5. |
| UX-10 | LOW | G22 (#179) | The friendly 403 message is dead code (plain-text 403); "open the link printed in the terminal" is wrong in window mode; failed views have no Retry. |
| UX-11 | LOW | G23 (#180) | Live Map always opens on Hagga Basin and shows raw marker type ids; no reset view. |
| UX-12 | LOW | G22 (#179) | Land-claim confirms say "cannot be undone from here" though Discard undoes them; Repair all/Clear sand sit under one base but run on every base. |
| UX-13 | LOW | G23 (#180) | Jargon ("the console" 21 times, "(#96)", "template id"), raw column headers, Go-to-place needs a separate Teleport press. |
| UX-14 | LOW | G22 (#179) | Rejected inline edits leave the typed value in the box. |

### DBA (12)
| ID | Sev | Group | Finding |
|---|---|---|---|
| DBA-1 | MEDIUM | G13 (#170) | The correlated max(health) per row with no index took 4.0 s at 5,000 pieces and 66.5 s at 20,000, freezing every other call; one click since #70. |
| DBA-2 | MEDIUM | G09 (#166) | Six concurrent refills over-filled a turbine in 3 of 20 trials; the foreign-key gate cannot catch it. |
| DBA-3 | MEDIUM | G10 (#167) | Same: seed 9007199254740993 became ...992; every x.0 float becomes an integer across the whole document. |
| DBA-4 | MEDIUM | G12 (#169) | SetPieceHealth updates every building sharing an instance_id (the key is building_id + instance_id); the route is live though the UI box is gone. |
| DBA-5 | MEDIUM | G15 (#172) | ShrinkLandClaim keeps a cell whose only link was removed (reproduced); the game needs the claim connected edge to edge. |
| DBA-8 | MEDIUM | G16 (#173) | db.go selects rowid and edits "where rowid=?": 61 WITHOUT ROWID tables (fgl_entities, totems, specialization_tracks, landclaim_segments...) fail with "no such column: rowid". |
| DBA-9 | MEDIUM | G08 (#165) | The auto-commit includes the wind-turbine/windtrap shapes (#141, retemplate path, five stacks of 100) with no recorded in-game check. |
| DBA-10 | LOW | G12 (#169) | Land-claim writes accept any totem_id that exists. |
| DBA-11 | LOW | G16 (#173) | The foreign-key gate counts orphans per table group, so an edit that fixes one and creates another passes (61 WITHOUT ROWID tables). |
| DBA-12 | LOW | G05 (#162) | README says only a single-player session blocks saving while the UI says Restore needs the game closed; no documented restore procedure or backup location. |
| DBA-6 | LOW | G14 (#171) | SandbikeEngine_6 with Decayed 1200 became 1000/1000, contradicting "only raises". |
| DBA-7 | LOW | G11 (#168) | Unvalidated numbers reach REAL columns (spec level "Inf"/-500, xp 1e9 against a maximum of 44,182). |

### QA (15)
| ID | Sev | Group | Finding |
|---|---|---|---|
| QA-1 | HIGH | G01 (#158) | The "do not write while single-player is running" gate (Commit, Restore, RunStartupTasks) can be deleted with all 14 packages still green (mutants c1, c2, w1, w2 survived). |
| QA-2 | HIGH | G02 (#159) | Repair-one-vehicle UPDATE scope (vehicle_id) can be dropped with the suite green: the only test has no second owned vehicle, so the ownership clause hides it. No Go test posts /api/vehicles/repair. |
| QA-3 | MEDIUM | G02 (#159) | RepairBuildings, ClearSand, ResetVendors, SetPieceHealth, GiveToInventory, SetReputation, SetSpec, TutorialSet and TagSet have 0% coverage; mutants of their scoping survived (and #70 made two of them one click). |
| QA-4 | MEDIUM | G14 (#171) | Seven of the 98 family-by-part rules (treadwheel, light and medium ornithopter bases; spice container, inventory, scanner, seat factors) survive mutation. |
| QA-5 | MEDIUM | G24 (#181) | TestUpdateRowRefusesAccountsAndIdentityColumns is partly tautological: with the items_id_sequencer lock deleted it still passes (STRICT rejects the string "x"). |
| QA-6 | MEDIUM | G24 (#181) | spsession tracker: a start marker split across two reads (s9) and four more mutants survive; the test never appends complete lines plus a partial tail. |
| QA-7 | MEDIUM | G24 (#181) | New() captures the real %APPDATA%\tabr-tau\settings.json, so fixtures and results depend on the developer's machine. |
| QA-8 | MEDIUM | G24 (#181) | Shipped assets and main() wiring are untested: removing the /livemap.js route or RunStartupTasks() in main stays green. |
| QA-9 | MEDIUM | G24 (#181) | Hostile fixtures are empty or errored for skills, research, crafting, power, water and inventory; ok >= 10 threshold; NO_DATA set disables the hostile-text assertion on six tabs. |
| QA-10 | LOW | G10 (#167) | Same round-trip; surviving mutants g2 (cur >= target) and g5 (MaxDurability 0) show weak tests. |
| QA-11 | LOW | G24 (#181) | Self-referential tests and unpinned boundaries (xpByLevel, decodeFieldPosition golden ids, claim and teleport limits, spice thresholds). |
| QA-12 | LOW | G24 (#181) | TestSymlinkedSaveIsRefused skips on Windows; gui_windows.go 0%; no -race on Windows. |
| QA-13 | LOW | G24 (#181) | The ui CI job has no timeout; cancel-in-progress also cancels runs on main. |
| QA-14 | LOW | G24 (#181) | Weak assertions in livemap_icons_test, the teleport label check and a duplicated CheckListenAddr loop. |
| QA-15 | LOW | G01 (#158) | internal/save/testhook.go exports NoGameRunningForTests, which disables the game check, from a shipped package. |

## STRIDE report
Mapped by issue group (each hat also mapped its own findings; group categories are the union).

| Category | Groups | Highest | Status |
|---|---|---|---|
| Spoofing | G18 (#175) | MEDIUM | 0 of 1 groups fixed |
| Tampering | G01 (#158), G02 (#159), G04 (#161), G08 (#165), G09 (#166), G10 (#167), G11 (#168), G12 (#169), G14 (#171), G15 (#172), G17 (#174), G19 (#176), G20 (#177), G21 (#178), G25 (#182) | HIGH | 0 of 15 groups fixed |
| Repudiation | G03 (#160), G05 (#162), G06 (#163), G08 (#165), G22 (#179) | HIGH | 0 of 5 groups fixed |
| Information Disclosure | G17 (#174), G18 (#175), G20 (#177) | MEDIUM | 0 of 3 groups fixed |
| Denial of Service | G01 (#158), G11 (#168), G13 (#170), G17 (#174), G18 (#175) | HIGH | 0 of 5 groups fixed |
| Elevation of Privilege | G12 (#169), G20 (#177), G21 (#178) | MEDIUM | 0 of 3 groups fixed |
| N/A | G07, G16, G23, G24 | | stale docs, test gaps and a licence question do not map to STRIDE |

## Checked and found fine (summary from the hats)
- Every new write goes through `save.Mutate`; none uses the read connection. Single-player session detection fails closed on every error path. Commit re-checks the game state itself.
- Network: one TCP listener on 127.0.0.1 per instance, no outbound calls, no external URLs in the UI, CSP `default-src 'none'` with same-origin allowances, exact Host allow-list, loopback-peer check, 48 path-normalisation variants never bypassed the cookie check.
- Security: every SQL string built from Go constants or tables; all request text bound; no shell use; path inputs validated; the `html` tagged template escapes by construction; zlib bounded by a declared-size check; no committed secrets in 166 commits (regex search; gitleaks passes in CI).
- Cloud: all five action SHAs resolve to their tagged releases, no `pull_request_target`, no untrusted interpolation into `run:`, workflow permissions are `contents: read`, `npm ci --ignore-scripts`, 0 open Dependabot or secret-scanning alerts.
- DBA: the Mutate path (one transaction per edit, deferred rollback, foreign-key gate, read-back before rename, hash checked twice); jsonb_set keeps columns valid JSONB; skill-module keys are byte-identical to the game's; give-item sequencing and caps.
- UI/UX: first-run and save-picker flows, blocked-save wording, confirmations on destructive actions, the game's own building and item names, honest counts in repair and refill toasts.
- QA: `go vet` clean; `go test -count=1 ./...` 14/14 packages ok (301 pass, 3 skip); `cd web-tests && node --test` 110/110; shuffle runs ok; coverage ops 83%, save 78%, web 73%. Mutation: 49 of 105 Go mutants killed, 12 of 14 UI mutants killed (the survivors are the findings in G01, G02, G14, G24).

## Progress log
- 2026-10-15: audit run, 93 raw findings consolidated into 25 issues (#158-#182), register committed. Remediation and the Layer 3 `/code-review high` pass follow; their dispositions are recorded in the Status column and appended below.

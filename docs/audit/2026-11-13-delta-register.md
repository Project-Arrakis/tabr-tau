# Findings register, 2026-11-13: delta audit of the editor (tabr-tau)

Part of the delta audit, [#606](https://github.com/Project-Arrakis/tabr-tau/issues/606). Range: `c54e7d8` (the last commit of the 2026-10-27 code review, `docs/audit/2026-10-27-code-review-delta.md`) to `b9bca07` (main on 2026-11-13): 79 commits, the 73 merged pull requests since that review, 357 files, about 51,700 lines added (the Landsraad rework, vehicle placement, base import and export, the live-save rules, the newer save layout, the two nested modules `kit` and `design`, the shared look and the Theme menu).

Method: four independent read-only reviews, in the order the issue asks: (1) write paths A (Landsraad, fills, refuel, repair, the automatic pass, queued-edit replay, durable writes), (2) write paths B (Create, Bring and Duplicate vehicle, kit apply, base import and export, teleport, the live-save guards, both save layouts), (3) readers and data (catalog and place data, provenance and notices, the kit modules as readers, workflows, dependencies, docs, public-repository hygiene), (4) web UI and server (request gates, validation, error shapes, XSS, accessibility, internal ids shown to players, the Theme menu, settings). Each reviewer read the real code at `b9bca07`, cited file and line, and ran small scratch tests in a copy where a claim could be tested cheaply (none committed). Findings were merged into 13 groups before anything was filed. `go vet ./...` and `go test ./...` pass at `b9bca07`; the `kit` and `design` modules' tests and `staticcheck` pass. Not run: the jsdom UI tests (`web-tests` has no installed dependencies in the audit copy; nothing was installed) and a vulnerability scan (`govulncheck` is not installed locally; CI runs it). Single-player only: no multiplayer findings. **Nothing was fixed by the audit.**

Severity scale: Critical (data loss or a safety problem), High, Medium, Low, Info. STRIDE: S spoofing, T tampering, R repudiation, I information disclosure, D denial of service, E elevation of privilege; `-` where none applies.

## Result

| Severity | Count |
|---|---|
| Critical | 0 |
| High | 0 |
| Medium | 8 |
| Low | 28 |
| Info | 17 |
| Total | 53 |

No finding loses data on its own or breaks the loopback and same-origin rules. The Medium findings are: a Fill faction goal that writes a contribution row under a faction the character is not in (D1-1) and Landsraad reads that hide their errors (D1-2); a live-save gate that depends only on text in the game's log (D2-1); a base file whose values are not checked and whose health can be copied onto the player's own bases by the repair (D3-1); internal ids still shown to players across the tabs (D8-1); a public document that names a private program's file (D11-1); embedded game data with no notice (D11-3); and the new module checks not being required for merge (D12-1).

## Groups

| Group | Severity | Issue | Findings | Open |
|---|---|---|---|---|
| D1: Landsraad writes and reads | MEDIUM | #611 | 9 | 9 |
| D2: The live-save gate rests on text in the game's log | MEDIUM | #612 | 5 | 5 |
| D3: Base file import: values are not checked | MEDIUM | #613 | 2 | 2 |
| D4: Create, Bring and Duplicate vehicle: arguments, copies, preview route | LOW | #614 | 4 | 4 |
| D5: Edits that create rows do not check the table layout | LOW | #615 | 2 | 2 |
| D6: Gear repair in the backpack and two automatic passes | LOW | #616 | 2 | 2 |
| D7: Teleport bounds and unconfirmed arrival points | LOW | #617 | 2 | 2 |
| D8: The game's internal ids shown to players | MEDIUM | #618 | 2 | 2 |
| D9: UI text and accessibility of the new slices | LOW | #619 | 4 | 4 |
| D10: Server request handling and error shapes | LOW | #620 | 6 | 6 |
| D11: Public-repository hygiene, notices and provenance | MEDIUM | #621 | 6 | 6 |
| D12: Repository settings and CI for the nested modules | MEDIUM | #622 | 3 | 3 |
| D13: Readers: command line, discovery, data currency | LOW | #623 | 6 | 6 |

## What was checked and found sound

Write paths:

- Every Landsraad write runs in one `Mutate`, with the term compared inside the edit and the houses counted before writing; a wrong expected count or term answers 409 and rolls back. Nothing lowers a value, no reward row is written, and the types fit the STRICT columns. A bad faction is stopped by the foreign-key scan. `task_ids` is bounded to 100 and rejects duplicates. Numbers come through `Args.Int/Float`, which refuse NaN, Inf, fractions and values above 2^53.
- The Landsraad clock (#465): the play clock is `(lastactive - universe_time - down_time)/1e6`, with a fall back to the term start and NULL when no term is known, never Unix time; the value is clamped inside the term window and an open house is never touched.
- Fills and refuel read through the edit's own query (so the reads are in the replay read set), never lower a value, skip and report unknown templates, take capacities from the kit table, and touch only the character's own pawn inventories and own vehicles. `json_set` changes one path, so other keys keep the game's text.
- The automatic pass and the launch pass refuse a copy of a save in all three places, use the same game check as a hand edit (#499), retry while waiting, never queue Run now, and roll back the whole pass when a job fails. The countdown is started by a GET guarded by the session cookie and the same-origin checks.
- `ownsave.go` answers "the game's own" whenever it is in doubt (blank path, unset `LOCALAPPDATA`, device path, short name, junction, hard link, the cloud layout anywhere), which is the cautious direction; a table of odd paths (trailing dot or space, `\\localhost\C$`, `\\?\`, `::$DATA`, `..`, a sibling folder) classified as expected. A copy still gets the location rule.
- Durable writes: `kit/durable` keeps temp file, fsync, close, rename with retries and a folder sync; `installBlob` and `Restore` call the gate right before the replace; backup pruning keeps the first, the oldest and the just-taken backup; a read-back is checked before a write counts as done; `settings.json` is synced and an empty file is treated as damaged.
- Base import (`importBaseTx`): one transaction; every actor, inventory, item, entity and backup gets a new id from the allocators; ids inside JSON are rewritten and read back; a depth limit and a `maxNewID` of 2^53; the preview is the same code rolled back; import is gated by the file hash (409 on a different file); size and row caps (a 15.8 MB, 245k-row file loaded in 0.8 s); every refusal leaves the save as it was. The console file is decoded strictly, unknown keys refused, and an export is read back by the editor's own reader before it is handed out.
- Create vehicle: one edit; the foreign keys of ten tables are checked before and after; derived numbers are recomputed from the written modules and a mismatch undoes the edit; entity ids are unique per prefix; fuel NaN is refused; the dry run is a rollback. Placement is bounded arithmetic (rings to 80 m) and stored vehicles are excluded.
- Both save layouts: the markers layout is decided from the columns at load; nothing in the editor writes `markers` or `player_markers`; the trigger text is the same in both layouts, so the newer one is not blocked by the fingerprint; an unknown layout gives a note and never blocks; both layouts also pass the suites of `internal/ops`, `internal/save` and `internal/web` when run with the layout switch.
- Kit apply: the hash handshake answers 409 for a different kit, a refused apply does not queue, the dry run is a rollback. Teleport: NaN, Inf and size are refused, a cross-map move writes all three character actors in one transaction and refuses when one is missing, and a hub move goes only to the arrival point.

Readers, data and modules:

- The save container codec checks the size before and after reading, the declared size before decompression, reads through a limit of size plus one, requires an exact size and the magic, and verifies the checksum; a fuzz target runs in CI. `kit/webguard` accepts `localhost` or a loopback IP only, needs a loopback peer, an `http` Origin equal to Host and a parsed `application/json` type; navigation refuses backslashes, control characters, user-info and any other origin.
- `kit/gamefolder` reads no environment and no process; `Saves` globs a fixed depth, so no link loops. `kit/sessionlog` parses in linear time and the log follower caps a line at 1 MiB and reads in 4 MiB chunks.
- The module boundary is enforced by `kit/boundary_test.go`, `design/boundary_test.go`, `scripts/check-kit.sh` and `scripts/check-design.sh` (standard library only, the replace directive checked, a clean export tested with a consumer program).
- Workflows: every action is pinned by full commit SHA, `permissions: contents: read` with `contents: write` only in the release jobs, the tag is passed through `env:` (no expression injection), tag regex and commit-on-main checks, `persist-credentials: false`, timeouts, pinned staticcheck and govulncheck, the shared scan pinned by commit. `go.sum` is present, `go mod verify` runs in CI, Dependabot covers the root, `kit`, `design`, `web-tests` and Actions, jsdom is a dev dependency installed with `--ignore-scripts`, Go is pinned at 1.26.11.
- The item catalog agrees with its PROVENANCE counts (2,570 ids, no duplicates, no empty names, the 11 Halloween ids present). The place list is limited to the character's own bases, vehicles and respawn points; Deep Desert taxis and hubs are de-duplicated against saved markers. No real SteamID, GUID, user path or save is in the docs or the tests.

Web UI and server:

- The request gate: loopback Host with exact names (IPv4-mapped and `::1` handled; `localhost@evil`, a trailing dot and hex hosts refused), loopback peer, a per-port session cookie (every cookie of that name scanned, constant-time compare, HttpOnly, SameSite=Strict), a boot token consumed even when expired, Origin and Fetch Metadata allow-list, JSON content type on every non-GET under `/api/`. `CheckListenAddr` refuses non-loopback and binds the address it checked. The Content-Security-Policy is byte-identical to `c54e7d8`, so the CHANGELOG claim "CSP unchanged" is true; nosniff, no-referrer, same-origin resource policy and X-Frame-Options DENY are sent on every answer including errors and panics.
- `/design/` is served from an embedded filesystem, GET and HEAD only, no folder listing, an explicit content type with nosniff; there is no traversal.
- XSS: `app.js` and `livemap.js` have no `innerHTML`, `insertAdjacentHTML`, `eval` or `raw()` call; every string goes through html`` and `setHTML`, which throws for non-Safe values; backtick is escaped, every attribute read is quoted; toasts, the Theme menu and `look-boot.js` use `textContent` or `createElement` only; file, base, kit, place and decree names are escaped; download names are sanitised.
- Browser storage: `look-boot.js`, `motion-boot.js`, `browserStore` and `viewLoad` compare a stored value only against a fixed list or a shape, inside try/catch, and nothing builds a file name, selector or markup from a stored value.
- New POST routes: body cap 40 MiB and file cap 16 MiB (re-checked by `basefile.Check`), JSON strictness on kits (`DisallowUnknownFields`), bounded coordinates, caps on item lists (`maxGiveBatch`, `maxSpawnItems`, `maxFillHouses`), a places route that refuses an unknown map, base export ids that must be positive integers.
- Accessibility that held: the Theme menu has roving tabindex with arrows, Home, End, Enter, Space and Escape, focus returns to the button, `aria-checked` on radios, forced-colours and coarse-pointer rules, 44 px items on touch; the modal question has the dialog role, `aria-modal`, an inert page behind it, a focus trap and focus return; Calm motion is the default under "reduce motion", toasts have a shape as well as a colour, errors persist until dismissed; Auto-refresh does not redraw while a field has focus; the NOT LIVE chip, tab title and banner are all there.

## Earlier fixes: kept

| Earlier finding | Result at `b9bca07` |
|---|---|
| 2026-10-22 E1 (replay hold: queued edit replayed over a newer save) | Kept. `journal.go`, `readset.go` and `replaydeps.go` are unchanged in the range, and the new Landsraad, fill and refuel edits read through `m.One` and `m.Query`, so their reads are in the hashed read set. |
| 2026-10-22 E3 (backup prune and first backup) | Kept in `flush.go`, with `savefile.ReadCapped` in place of the old reader. |
| 2026-10-22 E7 (lifecycle gates), 2026-10-22 DBA-15 / #297 (gate again before the rename) | Kept: every writer goes through the gate, `installBlob` re-checks it right before the rename, the unverified-write path is intact. D2 is a new concern about the gate's input, not a regression. |
| 2026-10-22 E8 (stack and slot caps on Give) | Kept for Give; not applied to base import (D3-1). |
| 2026-10-22 E9 and 2026-10-15 NET-1, NET-2, SEC-7; 2026-10-23 #259 | Kept. `kit/webguard` holds the same rules, pinned input by input by `guard_pin_test.go`; the navigation rule moved with its tests. |
| 2026-10-22 E10 (provenance notices for embedded game tables) | Kept for the old tables; the tables added since have none (D11-3). |
| 2026-10-22 DBA-16 (schema note: warn, do not block) | Kept; `schemanote.go` compares against both layouts. D5-1 asks for a narrower check on row-creating edits. |
| 2026-10-15 SEC-1, SEC-3, SEC-5, SEC-6, SEC-10, UX-10 | Kept, except SEC-10 (a path as page text in the NOT LIVE banner, D9-1) and SEC-6 (the cap is on the open path but not in the command line, D13-2). |
| 2026-10-27 CR-1 (#441, waiting pass reloads the save every 3 s) | Kept: `retryWaitingPass` checks the switches before the reload and clears the wait at both early returns. |
| 2026-10-27 CR-2 (#442, `settings.json` without a sync) | Kept, now through `kit/durable` with a file and folder sync; an empty file counts as damaged. |
| 2026-10-27 CR-3, CR-4, CR-5, CR-6, CR-7, CR-8, CR-10 | Kept (kit hash handshake, blood-sack sentence, reason text with `aria-describedby`, `SetTaskProgress` ceiling, friendly kit errors, the `!l.term` guards, only the character's own rank copied). CR-5 has a few plain disabled states with no reason (Apply changes, Previous and Next, the tier box when a whole house is chosen); not a regression. CR-9 and CR-11 were not re-read. |
| 2026-10-22 CLD-12 (author mailbox), #263 (`.claude/` ignored) | Kept. |

## D1: Landsraad writes and reads

Severity MEDIUM. Issue #611.

Fill faction goal can write a personal contribution row under a faction the character is not in, and the Landsraad page reads discard their errors, so a failed read looks like an empty board. The smaller findings are in the clock repair, the single Complete, the queue text and unverified reward tables of the newer layout.

**Recommended fix.** Write the personal part only for the character's own faction; return the error of every read; check term and faction in the single Complete; validate the term window; describe each Landsraad edit by house and faction.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D1-1 | MEDIUM | T | `internal/ops/landsraad_goal.go:77-95, 104-106` | Fill faction goal for a faction the character is not in writes the character's own contribution row under the chosen faction and reads the "before" value only under that faction. Reproduced in a scratch test: character in faction 2, house with 1300 already contributed; a fill for faction 1 leaves rows (1, faction 1, 6000) and (1, faction 2, 1300), and the answer says `contribution_before:0`. A second run for faction 2 then leaves both at 6000. Fill chosen always uses the character's own faction. How the game treats two such rows is unverified. | Write the personal part only when the chosen faction is the character's own, or refuse and say so. | open |
| D1-2 | MEDIUM | I, T | `internal/ops/landsraad.go:28,52,61,77,83,136,137,155` | Eight reads discard their error (`x, _ := o.S.Query/One`). A failed or schema-mismatched read shows as "no contributions", "no tiers" or "no decrees"; if the term read (line 28) fails, lines 29-35 fall back to the tasks of every term. Fill and Complete are then offered against that picture. Same class as the 2026-10-23 fix "a give returns the error of a read that failed". The failing case was not reproduced. | Return the error of each read, as `tasks` does. | open |
| D1-3 | LOW | T, R | `internal/ops/landsraad_chosen.go:96-99`; `landsraad_goal.go:83` | `completeHouse` overwrites `winning_faction_id` and `completion_time` of a house another faction already won; the answer only says `completed_changed`. | Report the previous winner in the answer and in the write log. | open |
| D1-4 | LOW | T | `internal/ops/landsraad_clock.go:83-93`; `internal/ops/landsraad.go:245-290` (`SetTerm`) | `repairCompletionTimes` rewrites every completed task of the term whose `completion_time` is outside the term window, not only the chosen houses; `SetTerm` writes `end_time` with no range check. Shortening the term end and then pressing Complete or Reopen on one house rewrites other houses' times to the new window and the answer does not say so; with end below start a completion gets a NULL time. | Validate `end_time >= start_time`; report the repair; repair only rows holding a Unix-like value or only the chosen houses. | open |
| D1-5 | LOW | T | `internal/ops/landsraad.go:183-221` (`CompleteTask`) | The single Complete has no term check and no check that the faction is a real one (Fill chosen and Fill faction goal check both). Scratch test: task 149 of the earlier term 6 was completed without error; faction `None` is accepted; a bad id is stopped only by the foreign-key scan. | Use the same `currentTerm` and `None` checks, or retire the route. | open |
| D1-6 | LOW | R | `internal/ops/landsraad_goal.go:53`; `landsraad_chosen.go:152,207`; `internal/web/static/app.js:518-531` | The edit description of Fill faction goal, Complete chosen and Reopen chosen is just "landsraad", so the queue list, the chip tooltip and the held-conflict banner do not say which house or faction is waiting. | Give each edit a describing text and a `queueLine` rule. | open |
| D1-7 | INFO | - | `internal/ops/landsraad_goal.go:34-45`; `docs/save-database.md` | The newer layout's one-time house reward tables are read by no Landsraad write; "top tier" means the highest `landsraad_task_rewards` threshold. How the game ties a completion to the one-time reward is unverified (the docs mark the table meaning as inferred). | In-game check before relying on Fill faction goal in the newer layout (see #609). | open |
| D1-8 | INFO | - | `internal/ops/landsraad_clock.go:21-35` | The clock read is part of the edit's read set, so a Landsraad edit queued while the game runs is held when the game saves meanwhile (safe, by design); the person has to redo it. | Say so in the held message. | open |
| D1-9 | INFO | - | `internal/ops/landsraad_goal.go:48-49` | Any error of `o.player()` is read as "no character"; a read error silently drops the personal part (`has_character:false`). | Tell "no player" from other errors. | open |

## D2: The live-save gate rests on text in the game's log

Severity MEDIUM. Issue #612.

The decision "a world is loaded, so an edit must wait" depends on marker text in the game's log. If a game update rewords the lines the gate reads, the editor can write while a world is loaded and the game overwrites the edit.

**Recommended fix.** A second, independent signal (the file's own modification time), "no marker ever seen for a running game" treated as unknown, longer quiet margins, and the strict line policy.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D2-1 | MEDIUM | T, D | `internal/save/spsession.go:240-291` (especially 256, 262-283, 290); `kit/sessionlog/sessionlog.go:96-158` | The gate decides "world loaded or not" only from marker text in the game's log (`?listen`, `?closed`, the database-saved line). If a game process exists, the log is readable and no marker matches (a game update rewording a line, a different launch URL), the status is empty and `processState` falls through to the file-handle test and answers mode `menu`, not blocked. The editor then writes while a world is loaded; the game keeps its world loaded and overwrites the file about every 20 s, so the edit is lost or items are stranded (as in #419). Reproduced in a scratch test in `internal/save` (`listProcs` returns a process, `statusFor` returns an empty status, `fileHeld` false: `blocked=false mode=menu`). The likelihood of a marker change is unknown; the game changed its save layout in early November, so a change is plausible. | Add a second independent signal (the file's modification time younger than the quiet period and not set by the editor), or treat "process running for N minutes and no marker ever seen" as unknown and blocked. | open |
| D2-2 | LOW | T | `internal/save/spsession.go:47-51, 262-270` | The menu write is allowed 30 s after the session end and 23 s after a later database write; 23 s is only 0.3 s above the largest write gap seen (22.7 s). Any local `?closed` browse counts as the end, and a later map load without `?listen` does not re-set the state. Code read only, not reproduced; the file-held check is the backstop and the code says it is not known whether the game holds the file at its menu. | Raise `menuQuietAfterWrite` (for example 60 s, waiting is cheap) and treat `?closed` as an end only when no write line follows in the grace. | open |
| D2-3 | LOW | T | `internal/save/spsession.go:56` (`spPolicy`); `kit/sessionlog/sessionlog.go:112-131` | The editor reads the log with the wide policy: "Browse (net mode", "Log file closed" and "FPlatformMisc::RequestExit(" count anywhere in a line, so a line that merely contains them ends the session in the state machine, which is the unsafe direction. `Strict` exists and the kit's own consumer check shows it rejects such a line. Mitigated by the quiet checks and the open-file check. | Evaluate `Strict`, or document why wide is needed. | open |
| D2-4 | INFO | - | `internal/save/ownsave.go:39-83` | A copy is recognised by path; a save the game was pointed at elsewhere counts as a copy (edits written at once, automatic pass off). Documented in the file header and the banner says "copy". The function answers "the game's own" whenever in doubt, the cautious direction. | None needed. | open |
| D2-5 | INFO | - | `internal/save/testhook.go` | Exported test hooks that weaken the guard remain in a production package (accepted since 2026-10-15, guarded by `TestTestHooksAreNotCalledByProductionCode`; a new one, `RealOwnSaveCheckForTests`, was added). | Unchanged. | open |

## D3: Base file import: values are not checked

Severity MEDIUM. Issue #613.

Import checks ids, references and names but not the value columns, so a hand-edited or foreign base file can put impossible items and extreme health values into the save, and the repair functions then spread the health to the player's own bases.

**Recommended fix.** In the file check: stack against the catalog maximum, slot uniqueness and bounds per inventory, health within a sane range. Read the export tables under one lock.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D3-1 | MEDIUM | T | `internal/basefile/load.go:193-437` (items: only `stack_size > 0` and `position_index >= 0` at 431); `internal/ops/baseimport.go:452-467, 497-512`; `internal/ops/world.go:41-65`; `internal/ops/basestructures.go:96-125` | A base file passes preview and import with `stack_size` up to 2^63, two items in one slot, a slot at or above the inventory's `max_item_count`, and any finite `health`. The editor's own give paths cap stacks and slots (2026-10-22 E8/DBA-6). A piece with `health` 1e300 becomes the "highest seen for its type", so Repair buildings, Repair base and the automatic repair job copy it onto every piece of that type, including the player's own bases. Reproduced in a scratch test on the test world: an edited file with stack 9e18, a duplicate slot and foundation health 1e300 passed preview and import; after `RepairBuildings` all foundations, including the original bases, read 1e+300. | In `Verify`, check the stack against `catalog.MaxStack`, slot uniqueness and bounds per inventory, and health within a sane range (or clamp to the best known value for the type). | open |
| D3-2 | INFO | T | `internal/ops/baseexport.go:95,162`; `internal/ops/baseconsole.go:46,187,362` | Export reads the tables with separate queries and detects a concurrent change only by the count of queued ops. A reload or rebase between the queries that leaves the count unchanged would give a file that mixes two states (it may still pass `Verify`). Rare. | Read all tables under one lock. | open |

## D4: Create, Bring and Duplicate vehicle: arguments, copies, preview route

Severity LOW. Issue #614.

**Recommended fix.** Refuse arguments of the wrong type instead of dropping them, check the map, refuse or null row references in a duplicate, register the preview as a settings route, report vehicles whose chassis is unknown.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D4-1 | LOW | T | `internal/ops/vehicle_spawn.go:150-164, 217, 198-200, 473-480` | (a) a non-string module id in `uniques` becomes `""` and empties an optional slot; (b) `items` that is not a list is dropped without a word; (c) any `[A-Za-z0-9_]{1,64}` map name is accepted for `place`, even one that does not exist. The vehicle is created differently from what was asked, with no error (local API only). Scratch test: `{"UtilitySlot2": 5}` gave changed `from:BuggyMining_5 to:""`; `items:"MelangeSpice"` gave requested 0 and no error; map `NoSuchMap` gave no error. | Refuse a non-string module and a non-list `items`; check `place.map` against the maps in the save. | open |
| D4-2 | LOW | T | `internal/ops/vehicle_duplicate.go:123-126, 195-196` | Duplicate copies actor properties, entity components and module stats as they are, with none of the `!!act#/!!itm#/!!inv#` rewriting that import has (`internal/basefile/refs.go`). A copy of a vehicle whose JSON names other rows would point at the original's rows. Unproven (created vehicles write `!!act#0` and the game loaded two copies). | Scan the copied JSON for `!!kind#` and refuse or null the references. | open |
| D4-3 | LOW | R | `internal/web/server.go:793` (route), `:511-536` (`postMaxAfter`) | `/api/vehicles/spawn-preview` is registered with `s.post`, which flushes any other pending edit after the handler and adds `outcome` written or queued to the answer. The kit preview and the base import preview use `postSettings` and promise to write nothing. A "preview" can write pending edits and report "written". Impact is small (the 3 s ticker flushes them anyway). | Register it with `postSettings`. | open |
| D4-4 | LOW | T | `internal/ops/vehicletank.go:11-24` | When no chassis module is known to the catalog, the class tank measured for one grade is used (buggy 240, light ornithopter 300). A lower-grade chassis the catalog does not know could be filled above its real tank. Only a catalog gap after a game update triggers it. Low confidence. | Report such vehicles as unknown instead of using the class value. | open |

## D5: Edits that create rows do not check the table layout

Severity LOW. Issue #615.

**Recommended fix.** Before Create, Duplicate and base import, compare the definitions of the tables they write with the two known layouts and refuse on a difference (narrower than the global block decided against in 2026-10-24).

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D5-1 | LOW | T | `internal/ops/vehicle_spawn.go:476`; `internal/save/save.go:181` (`fingerprint`); `internal/save/schemanote.go` | Row-creating edits (create vehicle, duplicate, base import) do not check that the tables they write match a known layout; `schemaNote` is only a header warning and `fingerprint` compares triggers and views against the older layout only. If a game update changes only those tables, the editor still writes rows with fixed column lists: a NOT NULL column without a default fails and rolls back (safe), a new column with a default gives rows that lack the new data and the game may load them wrongly. | Compare the written tables with `KnownSchema` and the newer layout before these edits; refuse on a difference. | open |
| D5-2 | INFO | I | `internal/save/markers.go:32-46` | `MarkersTable` joins `map_names` with an inner join, so a marker whose `map_name_id` has no row there disappears from every read in the newer layout. Not checked against a real save. | Use a left join if the foreign key is not enforced. | open |

## D6: Gear repair in the backpack and two automatic passes

Severity LOW. Issue #616.

**Recommended fix.** Skip backpack items with no maximum; refuse or wait for Run now while a pass is working.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D6-1 | LOW | T | `internal/ops/ops.go:887-905` (`repairGearTx`); `internal/ops/autojobs.go` ("repair-gear") | Gear repair now includes the backpack (type 0, #524). An item with a durability record and no `MaxDurability` is raised to 100, and the automatic pass at launch runs this on every start with the job switch on by default. An item whose scale is not 0..100 would get a wrong value. Only raising is done, never lowering. | Skip backpack items with no max, or say in the job text that the backpack is included. | open |
| D6-2 | LOW | R | `internal/web/launchpass.go:147-156` (`RunPassNow`); `internal/web/server.go:1092` (`runPass`) | "Run the automatic pass now" pressed while the countdown's pass is still working: `RunPassNow` ends only scheduled or counting passes, so two passes can run. The save lock serialises them, but the later one overwrites `startupNote` and `lastAuto` with "nothing needed", hiding what the first saved. | Refuse or wait while `busy`. | open |

## D7: Teleport bounds and unconfirmed arrival points

Severity LOW. Issue #617.

**Recommended fix.** Apply the destination map's bounds to a move on the current map too; send `observed` to the page and say "not yet confirmed in the game".

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D7-1 | LOW | T | `internal/ops/teleport.go:174` (same-map move), `:253` (bounds only for cross-map moves) | A same-map teleport is bounded only to +/-1e7 units; the map bounds (Hagga Basin about -4.6e5 to 3.5e5) apply only when the map changes. A typo (an extra digit) puts the character outside the map with no warning. | Apply the map's own bounds, or warn when outside them. | open |
| D7-2 | LOW | I | `internal/ops/places.go:69-73, 97-103`; `internal/ops/place-arrivals.json:5-9`; `internal/ops/deep-desert-taxis.json`; `internal/ops/teleport.go:26-35` | `observed` is parsed but never read or sent to the page. Griffin's Reach, Crossroads and all three Deep Desert taxi stand points are `"observed": false` (computed from level data, never seen in use) and are offered exactly like the observed ones as "the game's arrival point"; a move there can land in rock or void. | Send `observed` and show "not yet confirmed in the game", or leave unobserved points out. | open |

## D8: The game's internal ids shown to players

Severity MEDIUM. Issue #618. Extends #608 (the sweep it asks for).

**Recommended fix.** Player-facing names for decrees, vendors, claims and the rest, or a split-words form flagged unconfirmed; the id as a small tooltip; "Unnamed trading post" and similar for unknown places.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D8-1 | MEDIUM | - | `internal/web/static/app.js:1705, 1708, 1709, 1625, 526-527, 1012-1013, 1395, 1413, 1418, 1465, 2218, 2257, 1550, 1571, 1604, 2261` | Any save with a Landsraad term, vendors, bases or created vehicles. **Landsraad:** the Decree pool "ID" column; raw decree names such as `ExperienceRateIncrease`; "Pool flag (disabled)"; "Current term N"; help text naming `SpecialVendorActive_Vehicles`; house names that are template names with "DA_House" cut off. **Elsewhere:** "Vendor ID" columns and "counters of vendor <id>"; "ID" columns on Bases, Base backups and Stored vehicles; "(#<base_id>)"; "<claim name> <totem_id>"; "Inventory <id>"; "Created vehicle <id>" and "Would create vehicle <id>"; queue lines "pool flag of decree N" and "Journey step <id>". Template ids appear only in tooltips, which is acceptable. | Player-facing names, or a split-words form flagged unconfirmed, with the id as a small tooltip. | open |
| D8-2 | LOW | - | `internal/ops/places.go` (`tradePostName` about line 190, `baseLabel`, `vehicleLabel`, `respawnPlaces`) | A trading post whose key is not in `place-names.json` is listed as "TradingPost (key 00271320...)" or "(marker N)"; bases, vehicles and respawn points carry actor ids in the label ("Sandbike #1234"). | "Unnamed trading post"; keep the id as a tooltip. | open |

## D9: UI text and accessibility of the new slices

Severity LOW. Issue #619.

**Recommended fix.** Basename in the NOT LIVE banner, the count in the chip's label, the visible text first in the Theme button's label, player wording for the two Movement notes.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D9-1 | LOW | I | `internal/web/static/app.js:585` (`notLiveMsg`) | The "This is a copy, not the game's save" banner prints the full file path as page text, so text copied into a bug report carries the Windows user name when the copy is under the home folder (the 2026-10-15 SEC-10 rule; the header shows only the basename). Regression of SEC-10. | Show the basename in the banner and keep the full path in a title. | open |
| D9-2 | LOW | - | `app.js:620-621`; `internal/web/static/index.html:21` | The header chip has a fixed `aria-label="Show the queued changes"` that replaces the text "N queued" for a screen reader; the count reaches only the `title`. | Put the count in the label. | open |
| D9-3 | LOW | - | `internal/web/static/theme-menu.js` about `:213-215` | The Theme button's visible text is "Theme: Fremen" plus the look's name, but its accessible name is "Theme. Look: Fremen. ... Open the menu", which does not contain the visible label (WCAG 2.5.3, voice control). The menu also has no typeahead (info). | Start the aria-label with the visible text. | open |
| D9-4 | INFO | - | `app.js` constants `TRAVEL_NOTE` and `PLACE_NOTE` about `:1236-1239` | Player copy carries test-session detail ("2026-10-27, one character, a throwaway save", "nobody has stood there yet"). Accurate, no leak. | Reword in the wording pass #584. | open |

## D10: Server request handling and error shapes

Severity LOW. Issue #620.

**Recommended fix.** 413 with a size sentence for an oversized body, reject trailing data, one error code for a bad argument, map SQL errors to a plain sentence.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D10-1 | LOW | - | `internal/web/server.go:492-500` (`postArgsMax`), `:387-398`, `:634`, `:640`, `:907`, `:917-971` | (a) A body over the cap is answered "invalid JSON body" (400); for a console base file of 16-32 MiB the page says "this is not a base file", although the editor's own limit (`basefile.MaxFileBytes`, 16 MiB) is half the console's 32 MiB cap. (b) An empty body, `null` or trailing data after the first JSON value is accepted as empty arguments (harmless: no handler writes into `Args`). (c) Bad-argument errors built with `errors.New` answer 400 with no code where the same fault elsewhere answers 422 `bad_argument`. | 413 with a size sentence; reject trailing data; use `apierr.ErrBadArgument` consistently; word the 16 MiB refusal as the editor's own limit. | open |
| D10-2 | LOW | I | `internal/save/mutate.go:177-198`; `internal/web/server.go:387-398` | An edit that fails on a changed schema or the newer layout reaches the toast uncoded: `publicError` strips home paths but not SQL text, so a player can see "no such column ..." or "FOREIGN KEY constraint failed". Not reproduced. | Map unknown SQL errors to a plain sentence with the detail under Details. | open |
| D10-3 | INFO | - | `internal/web/server.go:659-669` | `GET /api/autopass/launch` starts the countdown and `GET /api/startup` consumes the note: state changes on GET and HEAD (still gated by the session cookie and the same-origin check). | Document, or make them POST. | open |
| D10-4 | INFO | S | `internal/web/server.go:206, 256`; `kit/webguard` (`SameOrigin`) | `Sec-Fetch-Site: none` is accepted on every method. No exploit (JSON content type and the SameSite=Strict cookie still block a cross-site POST); a documented choice (#259). | Allow "none" for GET only. | open |
| D10-5 | INFO | T | `app.js` generally | Ids of 2^53 or more are parsed with `JSON.parse` and sent back as numbers; a larger id rounded to exactly 2^53 would pass the server's limit and address another row. Matters only if such ids exist in a save. | None now. | open |
| D10-6 | INFO | - | `internal/web/server.go:247-252` | `Cache-Control: no-store` on every static file: the 1.5 MB map images, the 290 KB `app.js` and the font are re-sent on each load (loopback, no harm). | Short caching for static assets. | open |

## D11: Public-repository hygiene, notices and provenance

Severity MEDIUM. Issue #621.

A public document names a file of a second, private program, and the notices that ship in the executable do not cover the embedded tables added since 2026-10-27. Smaller items are wording that reveals more of the private setup than needed, stale notice versions and stale provenance text.

**Recommended fix.** Delete the sentence in `docs/save-database.md`; reword the other hits; add a notice section for each embedded game table and extend `notices_test.go` to every one; match module versions in the notice test.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D11-1 | MEDIUM | I | `docs/save-database.md:6` | The public document names a file kept in the repository of a second, private program that reads these saves. This breaks the public-repository rule (no private tool named). Found by a search of the tracked files for the private program's name. | Delete the sentence; say nothing about another program. | open |
| D11-2 | LOW | I | `kit/PROVENANCE.md` (rows `gamefolder`, `sessionlog`, `ddgrid`); `kit/sessionlog/sessionlog.go` about line 90 (`Policy` comment); `design/files_test.go:362` (a regex that lists private-setup words and a private folder name); `tools/deepdeserttaxisgen/generate.go` header; `.github/workflows/ci.yml:290`; `CHANGELOG.md`; `docs/continuation-prompt.md` | Wording that tells a reader a second private program exists ("written for the editor and a second program", "where two programs differ"), and a test that spells out private terms. No live-memory wording, personal path, real id or save was found in the tree. | Say "other programs"; split or drop the word list from the test; say "the person" instead of the role name. | open |
| D11-3 | MEDIUM | R | `internal/notices/THIRD-PARTY-NOTICES.md` (about lines 464-535); `internal/notices/notices_test.go:12-17` | The notices cover `stacks.json`, `volumes.json`, `customization_unlocks.json` and `vehicles.json`, but nothing names `internal/ops/place-names.json` (the game's own place-name text), `place-arrivals.json`, `deep-desert-taxis.json`, `kit/gamedata/fillables/item_table_fillables.json` or the `kit/schema/*.sql` DDL (grep count 0 for each). Same class as 2026-10-22 E10: embedded game-derived data with no entry in the notices that ship in the executable. The provenance is in `internal/catalog/PROVENANCE.md` and `kit/PROVENANCE.md`, but the notice test pins only the old files. | Add a notice section for each; extend the test to every embedded game table. | open |
| D11-4 | LOW | R | `THIRD-PARTY-NOTICES.md` headings vs `go.mod` | Five module versions are stale in the notices: go-humanize v1.0.1 (go.mod v1.1.0), ncruces/go-strftime v1.0.0 (v1.1.0), x/sys v0.48.0 (v0.49.0), modernc.org/libc v1.77.1 (v1.80.1), modernc.org/sqlite v1.60.1 (v1.62.0). `TestEveryModuleHasItsLicenceText` matches the name only, so dependency bumps pass CI. | Match the version in the test; refresh the five licence texts. | open |
| D11-5 | INFO | - | `design/PROVENANCE.md` (font row); `CHANGELOG.md` design 0.2.0 entry | The text says the font file is not in the tree yet; `design/assets/fonts/DuneRise.ttf` (22,964 bytes, with its checksum file) is. | Reword. | open |
| D11-6 | INFO | - | `design/PROVENANCE.md` (font row) | The font's licence claim rests on a file taken from a font site on 2026-11-09; the author's own page was not checked. | Confirm the licence on the author's page and record the URL. | open |

## D12: Repository settings and CI for the nested modules

Severity MEDIUM. Issue #622.

**Recommended fix.** Add `kit`, `kit-windows`, `design` and `design-windows` to the required checks; a tag ruleset for `kit/v*` and `design/v*`; list the two folders in CODEOWNERS; run one job at the modules' stated Go floor.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D12-1 | MEDIUM | T | branch protection of `main` (required status checks); `.github/workflows/ci.yml` jobs `kit`, `kit-windows`, `design`, `design-windows` | The required checks are `go`, `govulncheck`, `powershell`, `python-tools`, `commit-messages`, `shellcheck`, the three security-scan jobs, `windows` and `ui`. None of the four module jobs is required. `go test ./...` at the root skips `kit/` and `design/`, so the save container codec, the durable writer, the session log, the web guard, the fuzz of `Decode` and the boundary tests can fail and still merge. Read from the repository's settings with the GitHub API. | Add the four jobs to the required checks. | open |
| D12-2 | LOW | T, E | repository rulesets (none); `.github/CODEOWNERS` | No tag protection for `kit/v*` and `design/v*`; `kit/README.md` says only the owner tags, but the release workflow only checks that the tagged commit is on `main` and the changelog has an entry. Required approvals are 0, and `/kit/` and `/design/` are not in CODEOWNERS. A published module version cannot be recalled, only retracted. Depends on who has write access. | Tag ruleset for `kit/v*` and `design/v*`; list the two folders in CODEOWNERS. | open |
| D12-3 | LOW | - | `.github/workflows/ci.yml` jobs `kit`, `design`; `kit/go.mod`, `design/go.mod` (`go 1.26.0`) | `go-version-file: go.mod` resolves from the repository root, not from `working-directory`, so the modules are tested only with the root go.mod's Go 1.26.11; the stated floor 1.26.0 is never tested. | Test the floor in one job. | open |

## D13: Readers: command line, discovery, data currency

Severity LOW. Issue #623.

**Recommended fix.** Read the command line's input with `ReadCapped`; return no save when `LOCALAPPDATA` is empty; re-read the data tables from the current game build or say which are unverified for it.

| ID | Sev | STRIDE | Location | Finding | Suggested fix | Status |
|---|---|---|---|---|---|---|
| D13-1 | LOW | T | `internal/catalog/PROVENANCE.md` ("Game build of the other two files" vs the volumes paragraph) | `stacks.json` and `customization_unlocks.json` say build 20502675 and "no later update"; the volumes paragraph reads build 26083551 (2026-10-22). The game updated again in early November (save layout change). `vehicles.json` (read 2026-10-27) and `item_table_fillables.json` (2026-10-25) predate that too. Stack caps for Give, cosmetic unlocks, vehicle modules and fill capacities may not match the current game; unknown vehicle module ids are refused, stack caps are not. | Re-read the tables from the current build, or state which files are unverified for it. | open |
| D13-2 | LOW | D | `cmd/tabr-tau/main.go:273, 286` | The `decode` and `encode` subcommands read the input with `os.ReadFile`, not `savefile.ReadCapped`; the decoded output is written with mode 0644. A huge file is read whole before `Decode` refuses it (2026-10-15 SEC-6 was fixed only for the editor's open path). | Use `ReadCapped` here too. | open |
| D13-3 | LOW | T | `internal/save/save.go:894-896`; `kit/gamefolder/gamefolder.go:57-58`; `cmd/tabr-tau/main.go:92-95` | `Discover()` passes `os.Getenv("LOCALAPPDATA")` straight through. With the variable unset, `SavedDir("")` is the relative path `DuneSandbox\Saved`, so the glob runs in the working directory; a `DuneSandbox\Saved\Cloud\PlayerClientStorage\x\y\game.db` there is the one result and is opened automatically. The kit comment says the caller must check for empty first. | Return nothing when the variable is empty. | open |
| D13-4 | INFO | T | `kit/savefile/codec.go` about `:65-88` | `Decode` ignores bytes after the zlib stream; trailing junk is dropped silently. | Optional: refuse trailing data. | open |
| D13-5 | INFO | - | `kit/durable/durable.go` (`Rename`) | The pause runs after the last failed try too (about 3 s lost before the error). | Skip the final pause. | open |
| D13-6 | INFO | - | `internal/save/ownsave.go` (`sameFileAsGames`) | Only the first 200 matches per glob are compared for the hard-link check; a hard link past 200 in the ring is not recognised (the other rules still apply). | None needed. | open |

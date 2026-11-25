# Findings register, 2026-11-25: delta audit of the editor (tabr-tau)

Tracking issue: [#809](https://github.com/Project-Arrakis/tabr-tau/issues/809). Delta audit (security and correctness) of everything merged to `main` since the 2026-11-16 registers (`2026-11-16-delta-audit-L2-register.md`, `2026-11-16-delta-audit-L3-register.md`). Their findings (R2-01 to R2-06, R3-01 to R3-10) and the candidates they left out are not repeated here.

Base: `981af62` (the head of the 2026-11-16 L2 range). Range: `981af62..0eb4812` (main on 2026-11-25), 145 first-parent commits, from #808 to #1221. 651 files, about 71,900 lines added and 4,700 removed. Most of the page lines are the Landsraad Command Center (display code and its web tests).

## Method

The diff was read by risk:

1. **Write paths to the save and to files outside the repository.** `internal/ops`: Recycle base (`recyclebase.go`, `solido.go`, `recycle_room.go`), Back up and remove (`baseremove.go`), Move the Reserve (`movereserve.go`, the box scan of #1215), the Deep Desert vehicles (`deepdesert_vehicles.go`, `deepdesert_holds.go`, `vehicle_place.go`), the shared give and volume rules (`giveupto.go`), the repair caps moved to `internal/policy`, and the Landsraad writes (`landsraad_takeback.go`, `landsraad_paid.go`, `landsraad_apply.go`, `landsraad_grant.go`). `internal/layoutroll` in full (`ini.go`, `roll.go`, `deepdesert.go`, `recycle.go`, `recyclemove.go`, `reconcile.go`, `vetted.go`), with `cmd/tabr-tau/layoutroll.go` and `scripts/layout-roll-task.ps1`. The open PRs #1217 (markers) and #1223 (`vet`) were not part of the range.
2. **The write gate and the safety checks.** `internal/save`: Back up now and the layout-roll checkpoint (`backupnow.go`), `KeepObserving` and `keepWatching` (`filetime.go`, #1159, #821), the re-compare before each rename retry (`durable.go`, `kit/durable`), the end-of-session rule for two writes (`spsession.go`, #1122), the gate the status asks (`GateState`, #819), the journal of another version (`journal_version.go`), the in-edit checks (`mutcheck.go`), the notes (`notes.go`) and the Discard of an unverified write (`save.go`, #799).
3. **The web surface.** `internal/web/server.go` (new routes and their argument parsing, the automatic pass queued by hand and its Cancel), `about.go`, `help.go`, and the page code (`app.js`, `help.js`, `lcc/*.js`, `livemap.js`) for HTML sinks fed by save or server text.
4. **Network.** Every changed Go file was searched for outgoing requests.
5. **Public text.** The added lines of `docs/`, `CHANGELOG.md`, `README.md`, the web-test and Go test fixtures, and the scripts.
6. **Tests.** The tests of the write paths were read where a finding or a claim touched them.

Two readers took slices in parallel: the Landsraad write code, and the page code with the public text. Each candidate was then read again in the code at `0eb4812`, with its callers, before it went into this register. Everything was traced by reading. No test suite was run and nothing was tried in the game. Single-player only. **Nothing was fixed by this audit.**

Severity scale and categories are those of the 2026-11-16 registers. Medium means a person can lose or wrongly gain save data in a plausible flow, or a guard can be bypassed. Low means an edge case, an answer that misleads, or upkeep with a concrete failure.

## Result

| Severity | Count |
|---|---|
| Critical | 0 |
| High | 0 |
| Medium | 0 |
| Low | 6 |
| Total | 6 |

No finding loses the game's data in an ordinary flow or opens a path from outside the machine. Every new save write plans inside its own transaction, refuses before it writes, reads back inside the edit and fails as a whole. Every new route goes through the common wrappers (session cookie, loopback Host, same origin, body size). No network code was added. The lows are edge cases of the Landsraad paid-ground record, one unchecked assumption in the Solido, unbounded growth of the backups the pruning never removes, and one refusal text of the layout roll that `--force` contradicts.

## Findings

| Id | Severity | Category | File and line | What is wrong | Issue |
|---|---|---|---|---|---|
| D-01 | LOW | write path, correctness | `internal/ops/landsraad_paid.go:794-797` | After an Unfill, the take-back ground (`back`) is trusted whatever the save shows (`limit = max(standing, back)`). If the person then restores an autosave from before the raise (reward rows, contribution and wallets back at 0) and raises again to the same level, the raise rewrites the reward rows but pays no Solari, House Scrip or specialization XP, and does not raise the faction progress. Before #944, `standing` would have capped the record at 0 and the raise would have paid in full. | register only |
| D-02 | LOW | write path, error handling | `internal/save/notes.go:101-113`, `:131-142`; `internal/ops/landsraad_paid.go:428-429`, `:804` | Once a sink that keeps failing has dropped notes past `maxUndelivered`, raises are refused only until the editor restarts: the flag is kept in memory. The dropped notes are gone for good, so after a restart an Unfill and a raise on such a house pay the same missions again. The refusal text tells the person to restart, which is exactly what clears the guard. It needs more than 1,000 failed deliveries. | register only |
| D-03 | LOW | interaction (page, write path) | `internal/ops/landsraad.go:217-232`; `internal/ops/landsraad_takeback.go:326` | When notes were dropped (D-02), the write treats every reward row as unsure and takes back none. The board's `boardTakeBack` and `boardUnfill` still use the record as read, so they preview "take back N". The page sends `was_taken` = N, the write computes 0, and every Unfill and tier removal on that house is refused with a conflict until a restart. | register only |
| D-04 | LOW | write path, correctness | `internal/ops/solido.go:401-410`, `:547` | The blank Solido device is chosen with `stack_size>=1`, and the blueprint record is written into that item row's stats. If the game ever keeps blank devices as one stack of more than one, the whole stack becomes filled devices with the same blueprint: the blanks are lost, or the base is copied N times. No case of a stacked device is in the tests or the catalog (no stack size is recorded for the item). The game's behaviour was not checked. Fix: require `stack_size=1`, or split one device off the stack. | register only |
| D-05 | LOW | write path (disk) | `internal/save/backupnow.go:38-42`; `internal/save/flush.go:255` | Backups tagged `-manual` (Back up now) and `-layoutroll` (the checkpoint of every real layout roll) are never counted or removed by the pruning, and nothing else removes them. A weekly scheduled roll adds one full copy of the save every week to the save's backup folder, without end. On a large save this fills the disk, and once the free-space check (`requireFreeSpace`) fails, edits stay queued with no clear cause. Fix: keep the newest N of each tag, or say how many are kept and how much space they take. | register only |
| D-06 | LOW | write path (state), docs | `internal/layoutroll/roll.go:104-111`; `internal/layoutroll/reconcile.go:39`, `:109` | The refusal on an unconfirmed note says that `--force` does not bypass it. But when the state file cannot be loaded (damaged, or larger than `maxState`), `--force` replaces the whole state with an empty one. The next write of the state file then drops the unconfirmed recycle or vehicle note, the recycle history (the checkpoints that are the way back) and the vehicle history, with only a one-line note on the console. Fix: with `--force`, keep a copy of the unreadable state file beside it before it is replaced, and refuse when a copy cannot be kept. | register only |

No finding reaches medium, so no issue was filed. The lows are recorded here only.

## Candidates not filed

These were raised by a reader, checked, and left out:

- **A re-raise after a partial take-back does not restore a taken row below a row that still stands** (`landsraad_grant.go:196`, `:275`). `loRows` is the highest tier whose item still has a row. The rule "counts only up to the highest tier whose row stands" is documented in the code. This is a design limit, not a defect. The Unfill sentence could say it.
- **A raise and a reopen in one request judge the reopen on the row read before the raise** (`landsraad_apply.go:238-243`, `:412`). The board never sends both at once. Only a hand-built request reaches it, and the answer then says that nothing was reopened.
- **`addWallet` has no ceiling** (`rewards.go`). The amount of one request is bounded, so only a hand-made balance near the int64 limit could overflow.
- **A box move counts items of unknown volume as none.** This is the documented rule of Give item, applied on purpose (#1191), and the answer gives the count.
- **A recycle refunds 100% in the Deep Desert, where placing costs half.** This is the decision of #1211, documented in the code and the help.

## Checked and found fine

**Recycle base, Solido, Back up and remove.** Ownership is the owner rank alone (standing base) or the backup's player (picked-up base). The plan is made inside the edit. Every declared foreign key to an actor, inventory, entity or permission row that the recycle does not handle refuses the base by name (`recycleOtherRefs`, table and column names checked against a pattern before they are quoted). The statements interpolate only integer lists and program constants. A respawn pending at the base refuses. A crafting queue that is running refuses. The read back counts every table against the plan, and the fingerprint covers the option. The Reserve-empty rule runs in the plan, so the question and the write both refuse. A base picked up by Back up and remove whose unsupported placeables were left standing is refused by the state check, not half recycled.

**Move the Reserve.** The box names are matched exactly with parameters. A box must be unique, owned, standing, not a hologram, not in the Deep Desert, not in a base being recycled, and must have one inventory with a slot count. Only `inventory_id` and `position_index` change. The read back checks position, every other column (digest), shared positions, the slot count and the volume. The query and body parsers refuse an empty list and the old `box1` and `box2`.

**Deep Desert vehicles.** Only owned vehicles that are not stored, on the Deep Desert map. A store is guarded by the state that was read and by the one-row-per-character backup table. A move keeps the state. The read back covers every row and the character's backup row count.

**Layout roll.** Game.ini: only the digits change (or one line is added), links and non-8-bit files are refused, the backup is written new and read back, the file is compared again before a durable write and read back after it, the game check is made twice, and the pick is limited to 0 to 11 and to the vetted list (`ParseVetted` refuses signs, typos and empty lists). The save: a real run holds the save's lock for the whole run. The checkpoint is verified by its hash. The intent is written before the save. The save is written through the editor's own gated path, then reopened and verified. An unconfirmed note stops the next run unless it can be settled from the file. The scheduled-task script quotes the save path and refuses a path with a double quote.

**Gate and durable writes.** `KeepObserving` and `keepWatching` only record observations and take none of the save's locks for the decision. The gates before the backup and the rename still decide. The re-compare before each rename retry ends the tries when the file changed or cannot be read. The rule of two writes near the end marker is bounded by the log's own times. `GateState` agrees with the rule a write uses. A journal of an older version is dropped only when its write is the file on disk.

**Web.** New routes use `s.get` and `s.post` under the common handler (session cookie, loopback Host and peer, same origin for all `/api/`). `/api/about` reads only `notes.md` from the editor's folder, with a length cap and control characters dropped. `/api/help` serves embedded pages. Cancel of the waiting pass is atomic with the retry (`cancelGen`). Page code: the only `innerHTML` sink is `setHTML`, which accepts only escaped `html` templates. The Command Center and Help views build the DOM with `textContent`. Help links can only name embedded pages. The "also on GitHub" link is checked against the repository's own address. Picture paths come from allow-listed keys.

**Network.** No changed Go file makes an outgoing request. The release and update design is documents only.

**Public text.** No user name, local user path, email (other than `test@example.com`), real ids, save hashes or tokens were found in the added lines. Fixtures are synthetic.

## Could not verify

- No test suite, race run or web test was run, and nothing was tried in the game or a browser. CI ran the tests on each merge.
- Whether the game ever stacks blank Solido devices (D-04).
- Whether the game accepts vehicles moved in a line, and stored vehicles in a state other than 0 (the store writes state 2 over whatever state was read).
- The 800-line take-back and unfill tests were read only by name and in part. They were not checked assertion by assertion for vacuity.
- The binary art, the generated `internal/catalog/buildcosts.json` and unchanged context lines were not scanned for public text.
- The import value limits of #1189 (`internal/basefile/values.go`, `internal/policy`) were read only for their write effect. The caps were not compared with game data.
- The open pull requests #1217 and #1223 were not part of the range.

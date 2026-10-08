# Feature comparison: tabr-tau vs the self-host console

Date: 2026-10-08. Issue: #90. Status: first version, to be kept current.

Compared: **tabr-tau** (this repo, main at the time of writing) against **Dune Docker Console v1.4.47 (the fork's main, in sync with upstream Red-Blink release v1.4.47)**
([dune-awakening-selfhost-docker](https://github.com/Project-Arrakis/dune-awakening-selfhost-docker), MIT, RedBlink), read from its
`docs/console/API-REFERENCE.md` and `docs/console/*.md`. Facts about routes and features only; no text is copied.
Complements `2026-09-30-dune-docker-parity.md` (save format and schema).

Why most of the console does not apply: it manages a **dedicated server** (Docker, Postgres, RabbitMQ, several map servers, many
players). tabr-tau edits a **single-player save** on the player's own Windows PC (see `2026-10-08-windows-only-and-go.md`).
There is no server to run, no other players, no message bus, and the game keeps the live database in memory
(`2026-09-30-dune-docker-parity.md` §1).

Status key: **Done** = in tabr-tau and verified in game or by test; **Partial** = some of it; **Gap** = applies to single-player and is
missing; **Cut** = deliberately dropped (operator decision); **N/A** = server-only or multiplayer-only.
"Plan id" is the feature id from `2026-09-30-architecture-and-implementation-plan.md`.

## 1. Players (the console's biggest area)

| Console feature | tabr-tau | Status | Plan id / note |
|---|---|---|---|
| Profile, position, currency totals, Solaris | Player tab, Solari | Done | P5 |
| Give item by name or template id | Give item with the 2,551-item catalog picker | Done | P1; catalog gaps tracked in #72 |
| Give item with quality, grade, durability | Item rows are editable (stack, durability) | Partial | quality and grade fields exist in rows; not verified in game |
| Augments on an item (`augment-item`) | Not supported | Gap | live-test T7-T9 (worn set, augments) are documented only |
| Give several items at once | One at a time | Gap | convenience; low |
| Edit or delete an inventory item | Edit stack/durability, delete | Done | P3 |
| Repair all gear | Repair gear | Done | P3 |
| Refill hydration / water | Refill water containers | Done | the console refills the player's hydration stat; tabr-tau fills the held containers (#55) |
| Add XP, set level | Not supported | Gap | P6; JSONB `FLevelComponent` |
| Set skill points, set skill module | Not supported | Gap | P6 |
| Reset progression | Not supported | Gap | high blast radius; needs the review pane |
| Vitals (health, hydration, addiction) | Not supported | Gap | P8; JSONB `FHealthComponent` and gas attributes |
| Faction reputation (add) | Set reputation | Done | P10 |
| Assign faction (Atreides/Harkonnen/Neutral) | Not supported | Gap | small; check which tables carry it |
| Intel (add) | Not supported | Gap | single number; low |
| Other currencies (`add-currency`) | Solari only | Gap | confirm the save has other currency items |
| Specializations: add XP, grant max, reset, keystones | Specialization tracks (set) | Partial | P11; keystone grant-all / reset-all not done |
| Unlock crafting recipes | Learned building recipes | Partial | P12; crafting recipes vs building recipes need checking |
| Unlock research items | Not supported | Gap | P12 |
| Journey nodes complete / reset | Done | Done | P12 |
| Tutorials complete / reset | Done | Done | P12 |
| Tags | Player tags | Done | P12 |
| Clean invalid items from inventory | Not supported | Gap | maintenance; low |
| Teleport to coordinates | Teleport (base, respawn point, coordinates) | Done | P9; destinations picker, map and facing in #67 |
| Spawn a vehicle | Bring a vehicle to you | Partial | V1; spawning a new vehicle is not offered |
| Repair vehicle decay, refuel vehicle | Repair module durability, chassis durability | Partial | V2; fuel not done |
| List, search, paginate, online status, kick, ban, login-queue repair, kick-all, deleted characters | n/a | N/A | one character, no server |

## 2. Bases, storage, blueprints

| Console feature | tabr-tau | Status | Note |
|---|---|---|---|
| List bases, base inventory rolled up by item | Bases tab, storage list | Done | list-only filter issues: #75 |
| Open a container, add or remove items | Storage > Open, add / remove | Done | B2/B9; add-item box matching the player picker: #73 |
| Refill generators and fuel | Refill generators | Done | B4; also optional on editor start (#63) |
| Refill base water | Refill base water | Done | B4 |
| Auto-refill per base, thresholds, queued refills | One on/off switch at editor start | Partial | queued refills are a server concept; per-base opt-in not needed |
| Repair buildings, clear sand | Repair all, clear sand | Done | B3 |
| Land claim expand / shrink, vertical level | Done | Done | goes beyond the console (shrink) |
| Base permission roster, child access, system custodian | Read-only card removed | Cut | #58; single-player has only the owner |
| Delete a base | Not offered | Cut | #58 |
| Base backups (picked-up bases): list, export, import, reassign, delete | Not supported | Gap | the save has base backup tables; candidate feature |
| Export a live base as a blueprint; import / export / delete blueprints | Not supported | Gap | candidate; file format unverified |
| Storage containers (server-wide list, export JSON) | Per-save storage list; DB export | Partial | |

## 3. Vehicles

| Console feature | tabr-tau | Status | Note |
|---|---|---|---|
| List vehicles with condition and fuel | Vehicles tab (world and recovered) | Partial | no fuel column |
| Permissions, system custodian | n/a | Cut | single owner |
| Cargo hold: read, delete stacks | Not supported | Gap | candidate |
| Delete a vehicle | Not offered | Cut | #58 |

## 4. Market, Landsraad, world

| Console feature | tabr-tau | Status | Note |
|---|---|---|---|
| CHOAM exchange: listings, stats, market bot seeding / buyback | Exchange tab removed; vendor limits moved to Player > Vendors | Cut | #58; the exchange is a multiplayer economy |
| Vendor limits and restock | Vendors | Done | E1 |
| Landsraad: overview, task goals, reward tiers, player contribution | Term, decree pool, active decree, task board, complete / reopen, rewards | Partial | L1; goal amounts and reward tiers are not editable |
| Spicefields (spawn limits and weights) | Not supported | Gap? | world setting; confirm it lives in the save or in a game config |
| CHOAM terminals, sietches, Deep Desert, memory, autoscaler | n/a | N/A | server layout |
| Live map (players, bases, spice, POI) | Not supported | Gap? | a read-only map of your own save is possible; low priority |

## 5. Configuration and database

| Console feature | tabr-tau | Status | Note |
|---|---|---|---|
| Game settings / `ServerCustomSettings.ini`, UserEngine, UserGame | Config tab: edit `.ini` files with backups | Done | does a setting survive in single-player: #77 |
| Database browse, edit a row, search, SQL (read and write), export | Database tab, CSV / JSON export, read-only SQL, write SQL (limited to INSERT/UPDATE/DELETE/REPLACE) | Done | goes beyond the console on safety (review, atomic, backup); key/identity columns shown editable: #71 |
| Database password | n/a | N/A | SQLite file |
| Care package (automatic kits) | Not supported | Gap? | could be a "starter kit" item set; low |

## 6. Operations around the editor

| Console feature | tabr-tau | Status | Note |
|---|---|---|---|
| Backups with preview before apply | Backup before every save, restore, review pane before commit | Done | stronger: crash-safe write, integrity check, read-back |
| Server start / stop / update / logs / readiness / restart queue | n/a | N/A | no server |
| Accounts, roles (IAM), API keys, two-factor, Discord adapter, addons, public directory, metrics | n/a | N/A | one user on one PC; loopback-only UI |
| Broadcasts, MOTD, map chat, shutdown notices | n/a | N/A | |
| Confirmation phrases for dangerous actions | Tiered confirmations | Done | |

## 7. Ranked gaps worth doing (single-player only)

1. **XP, level and skill points** (P6) and **health / vitals** (P8): the most used player edits in the console; JSONB entities, so they
   need the review pane and a live check.
2. **Quality, grade and augments on give-item**, and the missing catalog items (#72): the picker is the first thing people use.
3. **Specializations and keystones** completion (P11) and **research / crafting recipes** (P12).
4. **Base backups** and **blueprints** import / export: valuable and testable, but the file formats need to be mapped first.
5. **Vehicle cargo hold** and **fuel**.
6. Small: faction assignment, intel, other currencies, give several items, clean invalid items.

Each of these needs a verification in game and an entry in CHANGELOG, like the existing features. Items marked "Gap?" need a check of
whether the value lives in the save at all before they are planned.

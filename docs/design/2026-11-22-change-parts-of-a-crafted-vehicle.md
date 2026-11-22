# Change the parts of a vehicle that is already crafted (#787)

Status: design note, proposal. No code is changed by this note and nothing is built until the note is agreed and the operator says go. Single player only. Evidence: this repository's fixtures,
tests, notes and recorded evidence (`docs/save-database.md`, `docs/evidence/single-player-schema.ddl.sql`, `kit/gamedata/vehicles`, `internal/catalog`, `docs/design/2026-10-27-vehicle-by-template.md` and the
other notes named below). No save of a running or installed game was read for this note: what only a real save can answer is listed under "Needs a test copy from the operator".
Game build the vehicle tables were read from: 1.5.15.0 (read 2026-10-27).

Related: #651 (swap a part to a higher tier, parked), #672 (part picker in the Create vehicle preview, merged), #650 (redraw keeps the page in place), #480 and #483 (where a vehicle goes, what a cargo hold takes).

## Summary

The save keeps one row per part and no link to anything else about that part, so changing a part in place is a small write on a few well-known rows: the part's row, the vehicle's health, fuel and
cargo hold numbers that follow from its parts, and nothing more. The editor already has every rule needed to work those numbers out (`vehicles.Derive`) and to check them after the write (`verifyVehicle`). What the
repository cannot say is how the game reacts to a vehicle whose part row was changed by a program: no vehicle composed or changed by the editor from the game's module tables has been loaded in the game yet (the
"Not proven" section of the Create a vehicle note). The recommendation is therefore to build in slices and to put one in-game experiment on a test copy between the read-only slice and the first write slice
(see "Recommendation").

## 1. Which parts can the save replace in place

### What the save holds

A vehicle's parts are the rows of `vehicle_modules` with that `vehicle_id`: `id` (AUTOINCREMENT), `vehicle_id` (foreign key to `vehicles`, cascade), `template_id` (text, the module id such as `BuggyEngine_6`) and
`stats` (JSONB). There is **no slot column and no link to an item**. Two facts make an in-place change possible:

- **The slot follows from the module id.** In the vehicle catalog (`kit/gamedata/vehicles/vehicles.json`, 398 module rows over 7 classes, grades 0 to 6, 101 uniques) every module id belongs to exactly one class and one
  slot. Each wheel, tread or wing position has its own ids (`BuggyLocomotionFrontLeft_6`, `OrnithopterMediumLocomotionCenterRight_5`), and a part that fits several positions is still one id per position, so a
  row is never ambiguous: its slot is the slot of its id. The Create a vehicle code relies on the same fact (`vehicles.Derive`, `ModulesFor`, `ResolveModule`).
- **The row can keep its id.** The only things that point at a module row are `inventories.vehicle_module_id` (cascade) and `vehicle_module_inventories`; the editor does not use the second, and the studies of #483 found every vehicle's cargo hold on the vehicle's actor (`inventories.actor_id`), where Create writes it too. Whether any row of a real save
  points at a module is query 1 of "Needs a test copy". A change that updates the row (same `id`, new `template_id` and `stats`) has nothing to cascade into if none does. A delete and an insert could, so the write plan never does that for a part that stays.

### What the catalog allows, slot by slot

Any module of the vehicle's own class whose slot is the slot being changed. The Create a vehicle note records that the game's own player vehicles mix grades (an engine of one grade on a chassis of another), so a
mixed set is not an invention of the editor. A part of another class never fits (the catalog's `ModulesFor(class, slot)` is by class), which answers the question of #651 whether a part of another vehicle type fits: it does not.

| Group | Slots | Derived numbers that follow | Notes |
|---|---|---|---|
| Engine, generator, treads / wheels / wings | `Engine`, `Generator`, `Locomotion` .. `Locomotion8` | none (the catalog gives these modules no health, fuel, critical health or cargo) | simplest case: only the row changes |
| Hulls | `CockpitHull`, `BodyHull`, `BodyHull2` .. `BodyHull4` | maximum health (hulls count as Body and Cockpit) | a cockpit of one kind may fix which hull goes with it (see below) |
| Chassis | `Chassis` | maximum health, critical health, fuel capacity, and the durability the part has | the vehicle's identity inside its class; the class never changes |
| Utility slots | `UtilitySlot1`, `UtilitySlot2` | cargo slots and volume for a storage part (29 catalog rows give some); a weapon module changes the shape of `stats` | optional slots may be empty: a part can be added or removed |
| Seat (Sandbike) | `UtilitySlot1` (`SandbikeSeat_0`, `_1`) | none | a seat is a part of the slot, like a boost |

Facts about slots that the write must respect, all from the catalog:

- Slots have parents (a utility slot hangs on a hull, a hull and an engine on the chassis). The game's slot table does not tie a utility slot to a specific hull, but every template uses the same hulls for the same
  utility slots; `BodyUtilitySeen` already reports a combination no template has. The same warning applies to a change: it is allowed and said in the preview, not refused.
- A mandatory slot cannot be left empty and an optional one can (the Create rule, `spawnSpecFrom`).
- Grades 7 to 9 are rows of the game's table that appear in no save and in no template, and the catalog does not hold them; the item catalog has placeholder part items for grade 7. A vehicle with a module id the catalog does not know
  (another game build, a grade 7 part, a class the editor does not know) is **not changed at all**: `Derive` refuses an unknown id, so the numbers could not be worked out or checked.
- Cargo hold room is a sum over parts (`cargolimits.go`, #483): a Sandbike without a storage part has 0 slots; the same chassis with `SandbikeInventory_2` has 15 slots and 250 volume.

## 2. How the vehicle's numbers and the part items follow

### What the save stores per part

`vehicle_modules.stats` is a JSONB document of groups, each `[x, {fields}]` with the fields in element 1 (`docs/save-database.md` section 3, item 11):

| Group | Field | Meaning | Written by the editor today |
|---|---|---|---|
| `FVehicleModuleDurabilityStats` | `CurrentDurability` | the wear; absent when 0 | Create: the table's `MaxDurability` of the module; Repair: `jsonb_set` |
| same | `DecayedMaxDurability`, `MaxDurability` | the lower maximum after a workbench repair (only some rows) | Repair raises it; Create writes neither |
| `FCustomizationStats` | `SwatchId` | the paint of the part | Create: the player's default swatch |
| `FVehicleWeaponModuleStats` | `CharacterStateTag` | present on weapon-type modules (the catalog flags 29 module rows: buggy launcher and cutteray, ornithopter launchers and scanners, Sandbike scanner Mk0 to Mk2) | Create adds it for a `weapon` module |

The full durability of a part depends only on the vehicle family and the kind of part, not on the grade (`kit/gamedata/vehicles`: no slot of any class has two different durabilities; the Repair note of #151 says the
same). So a tier swap never changes the maximum durability of a part in a slot; it changes the id, the derived numbers and, for a new part, the starting wear.

### What the save stores per vehicle, and the rule that gives it

Each of these follows from the parts by one rule (`vehicles.Derive`, the same rule Create writes with and checks against; the Create note records that it matched real vehicles of several classes):

| Where | Field | Rule |
|---|---|---|
| `actors.properties` | `DamageableActorComponent.m_TotalMaxHealth`, `m_CurrentMaxHealth` | sum of `Health` over the Chassis, Body and Cockpit modules |
| Actor entity (`fgl_entities.components`, linked as slot `Actor`) | `FHealthComponent.m_CurrentHealth` | the wear of the vehicle (Create writes the maximum) |
| same | `FHealthComponent.m_MaxDownButNotOutStateHealth`, `m_CurrentDownButNotOutStateHealth` | the chassis's `Critical` |
| same | `FVehicleComponent.CurrentFuel` | the fuel in the tank; the tank is the chassis's `Fuel` (`vehicletank.go`, #469) |
| `inventories` (type 0 on the vehicle's actor) | `max_item_count`, `max_item_volume` | the sum of cargo slots and volume over the modules |

`recovered_vehicles.chassis_durability` is a stored vehicle's chassis wear. Stored vehicles are never changed (below), so it is not touched.

### What the game recomputes at load

**Not known from the repository.** In every vehicle of the played-save copies studied for #483 and #469 the stored hold and the stored fuel equal what `Derive` gives from the modules; that shows the values agree,
not who makes them agree (the game may write them from the parts at every save, or recompute them at load). Create writes all of them, so a vehicle it composes does not depend on the answer. A part change must do
the same: write every number that follows, and treat "the game would have fixed it at load" as an assumption it does not make. Which of them the game really recomputes is the first experiment of the plan (below).

### Policy for the numbers when a part changes (proposed)

| Number | New value | Why |
|---|---|---|
| New part's `CurrentDurability` | the table's full durability for that module (as Create) | #651 asks for it; it is the same value for every grade of a slot, so the preview can say it plainly |
| Maximum health | `Derive` of the new set | the rule |
| Current health | the same fraction of the new maximum as before (a full vehicle stays full) | a hull swap should not heal or hurt; the editor has no better evidence |
| Critical health | `Derive` | the rule |
| Fuel | the same fraction of the new tank (the gauge does not move); never above the new tank | a chassis swap changes the tank (Mk6 buggy 300, Mk3 buggy 240); #469 already shows the tank follows the chassis |
| Cargo slots and volume | `Derive` | the rule |
| Items already in the hold | untouched; a change that would leave an item in a slot at or beyond the new slot count, or more volume than the new hold, is **refused** with the numbers, and the person empties the hold first (Delete or Clear this container) | #483 settled that the editor never gives past a hold and never removes items silently; the same applies to shrinking one |

The current-health and fuel choices are proposals, not findings; they are listed again in the experiments.

### The part items

A part is installed from an item (`BuggyEngine_6` "Buggy Engine Mk6" in the item catalog, category `vehicles`; one item `BuggyLocomotion_6` stands for every tread position, and a unique has an item whose id spells the grade
with a leading zero, which `ModuleName` and `ResolveModule` already handle). The module row records **no item**: nothing in the save says which item a part came from, so a change in the save cannot "use up" a specific item,
and the only honest models are:

1. the new part appears out of nothing, as in Create a vehicle and the console's spawn (proposed), or
2. the editor also takes the matching item out of the backpack and puts the removed part in, which needs the shape of a part item's `stats` (does it carry the part's wear) and the position rules of the backpack: see
   "Needs a test copy" and slice 4.

## 3. What happens to the removed part

Two options, a proposal for each slice:

- **Slices 2 and 3: discarded.** The old row is replaced; no item is made. The preview says it in one line ("The Mk3 engine is removed and is not returned"), so nothing is lost silently, and a backup is taken before the edit
  as for every edit (Restore on the Database tab).
- **Slice 4 (optional): returned to the backpack** as the part's item, by the Give Items path (slots, stack size, volume, "as much as fits", with what did not fit said). This needs two answers from a real save: whether a
  part item records its wear (`items.stats`), and what the game itself does when a part is removed in play (does a part item appear, at what wear). Until those are known, the editor does not guess the item's `stats`
  (the lesson of the Give Items audit finding F-17 and of #188: `stats` cloned from a random row, or rewritten through float64, corrupts per-instance data).

Not offered: keeping the old part "in the vehicle" in any form. A row exists once per slot.

## 4. The guard

All of these are checked by the server, never only by the page (the page may be stale, and a queued edit is replayed later).

| Guard | Rule | Existing code it reuses |
|---|---|---|
| Own vehicles only | the vehicle has a rank-1 `permission_actor_rank` row for the character's controller and a row in `vehicles` (what "owned" means in single player); a world vehicle or one of another character is refused with the same message as Repair ("that vehicle is not one of yours") | `ownedVehicleSQL` (`world.go`), `RepairVehiclesWhere` |
| Not stored | a vehicle in the vehicle backup (`state` 2) or stored for recovery (`state` 4), or with a row in `backup_vehicles` or `recovered_vehicles`, is refused: it keeps only its place, and loses its permission roster | `refuseStored`, `storedKindBy` (`lifecycle.go`) |
| Known parts only | every current module id is in the catalog and belongs to the vehicle's class; the vehicle has exactly one chassis | `vehicles.Derive`, `ClassFor` |
| The live-save gate | the edit goes through the one write pipeline (`Mutate`): a backup first, written only when the gate says no world is loaded, and otherwise queued with the reason. A vehicle in a loaded world is rewritten by the game about every 20 seconds, so an edit written then would be lost | `docs/design/2026-11-15-live-save-gate.md`, `docs/design/2026-10-27-menu-write-grace.md` |
| Queued edits replay safely | a queue entry is only INSERT, UPDATE or DELETE statements plus a hash of every row its queries read; when the file changed since, the reads are asked again and the edit is **not applied over a vehicle the game has changed** (it is reported). The write must therefore read the vehicle's module rows and cargo rows inside the edit, not only before it. The request also carries the module ids the person saw (`expect`), so a change made against a stale page is refused too | `internal/save/journal.go`, the `was_*` fields of the Landsraad write |
| One vehicle per request | no "change all my vehicles" in the first versions; the blast radius of a mistake is one vehicle | |
| A repeat writes once | a `requestId` as in Create (#740), so a double click is one edit | `spawnLedger` |
| Checks before the edit stays | foreign keys of the touched tables before and after (no new violation), the modules read back and `Derive` again against the numbers written, the inventories counted; any difference undoes the whole edit; the preview runs the same checks in a rolled-back edit | `vehicleFKViolations`, `verifyVehicle`, `errSpawnDryRun` |

Single player only (the whole editor is); there is no multiplayer wording in the page.

## 5. The proposed page (Player > Vehicles)

The Components card of the opened vehicle (a table of component, condition, durability) gets one button, **Change parts...**. Nothing else of the form is drawn until it is pressed, and the catalog is not read before
(as Create a vehicle does). The form opens inside the same card, so the person stays at the vehicle they are looking at.

- **One row per slot of the vehicle's class**, in the Create order: Slot, Part now, a **Part** drop-down (the same option builder as the Create form and its preview table, `vcSlotOptions`, so the parts that fit, the
  "(unique)" and "(N slots)" labels and "(empty)" for an optional slot are one function), and the durability the part will have. The four treads, wheels or wings of a class are shown as one row, **Treads (4)**, whose
  drop-down sets every position to the same grade (the position suffix is kept); one position can be changed under that row, for a unique that exists for one position only. A seat of a Sandbike is just a utility part.
- **Preview** (asks the server, keeps nothing) shows, before anything is queued: the list of changes (slot, from, to), the numbers before and after (maximum health, critical health, fuel in the tank and the percent, cargo
  slots and volume, with the hold's items counted against the new room), "the new part has full durability", "the old part is removed and not returned", any warning (a hull and utility combination no template has), and
  the refusal reason in the page's own words when there is one. It also says what Apply will do now: "written to the save" or "queued until the game is closed" (the page already has this sentence for Create).
- **Apply** posts exactly the previewed request (the draft is the single source, as in #672: a pick refreshes the preview or is put back, and Apply waits while a preview is being asked for). After the write a line stays
  until dismissed: "Changed N parts of vehicle ID. It is written to the save; the game has not loaded it yet." **Cancel** discards the draft.
- Every redraw in the form uses `render('scroll')`, so the scroll position and the focus stay (the rule of #650; a new feature there inherits the G-010 audit finding unless it follows it). Each drop-down has a label
  ("Part for Rear hull"). The preview status line is the live region; the table is outside it.
- The tier shortcut of #651 is a button in the same form, **Set all parts to Mk N**, which only fills the draft from the class's template of that tier (the slots that differ), and goes through the same Preview and
  Apply. It adds no write path of its own.
- Text budget: the explanation lives in the card's About this and in tooltips, not on the page (#426).

A stored vehicle, a vehicle with an unknown part and a vehicle that is not the person's get no button, with the reason in the button's tooltip (as Bring vehicle does for a stored vehicle).

## 6. The write plan

Request: `POST /api/vehicles/parts-preview` and `POST /api/vehicles/parts` with `{vehicle_id, changes: {slot: module id, or "" to empty an optional slot}, expect: {slot: module id the page saw}, requestId?}`.
Every statement runs inside one `Mutate` (the dry run rolls back), in this order. Anything that is not an INSERT, UPDATE or DELETE statement would turn the journal off (a script, DDL or a pragma), so none is used.

1. **Read inside the edit** (so the journal hashes it): the vehicle's `actors` row (state, class), `permission_actor_rank`, the `vehicle_modules` rows, the Actor entity's components, the cargo `inventories` row and
   its `items`. Check the guards of section 4 and `expect`.
2. **Plan** (pure function, no SQL): from the current module ids and `changes`, the new set; `Derive` of both; the per-slot rows to update, insert or delete; the new numbers by the policy of section 2; refusals
   (wrong slot, mandatory slot empty, hold too small for its items, unknown id, a change that changes nothing).
3. `vehicle_modules`: for each changed part, `update vehicle_modules set template_id=?, stats=<new> where id=? and vehicle_id=?`. `<new>` is built with SQL JSON functions on the existing document, not by decoding
   and re-encoding it through Go numbers: `jsonb_set` of `FVehicleModuleDurabilityStats[1].CurrentDurability` to the new module's full value, the groups `DecayedMaxDurability` and `MaxDurability` removed (the new part
   has no workbench history), `FCustomizationStats` kept as it is (the paint stays), `FVehicleWeaponModuleStats` added or removed to match the new module's `weapon` flag. For an optional slot that gets a part:
   `insert` with the id from `nextID` and `moduleStats` of Create. For one that is emptied: `delete` of the row, only when no `inventories.vehicle_module_id` points at it (otherwise refused).
4. `actors.properties`: `jsonb_set` of `DamageableActorComponent.m_TotalMaxHealth` and `m_CurrentMaxHealth`, only when the Chassis, Body or Cockpit set changed.
5. The Actor entity (`fgl_entities.components`): `jsonb_set` of `FHealthComponent.m_CurrentHealth`, `m_MaxDownButNotOutStateHealth`, `m_CurrentDownButNotOutStateHealth` and `FVehicleComponent.CurrentFuel`, only the
   ones that change (health when a health module changed; critical and fuel when the chassis changed).
6. `inventories` (type 0 of the vehicle): `max_item_count` and `max_item_volume`, only when a storage part changed. `items` and the component holds are not touched.
7. **Verify** (as `verifyVehicle`): re-read the modules, `Derive`, compare with what step 3 to 6 wrote; compare the foreign-key violations before and after; every other vehicle's rows are unchanged (a hash of the
   rows of the vehicle tables, excluding this vehicle); a repeated request with the same final set changes no row.
8. Answer with the before and after rows of the touched tables (as Create answers with `rows`), the derived numbers, the notes and, once, `requestId` (the repeat ledger).

Not written, on purpose: `actors` place, class, state, serial; the `permission_*` rows; the entity ids and links; `actor_inventories` and the component holds (they follow the class, not the parts: Create writes
the same hashes for every template of a class); `m_CustomizationId`; `recovered_vehicles` and `backup_vehicles`. The three things in the plan whose meaning is not known (the weapon component, the spice fill, the
module order) are in "Needs a test copy".

Code touched, for the build (not by this note): a new `internal/ops/vehicle_parts.go`, the routes in `internal/web/server.go`, the page in `internal/web/static/app.js`, tests, `CHANGELOG.md`; `verifyVehicle` and
`vehicleFKViolations` are shared with Create (their signature takes a `spawnSpec` today and would take the module ids and the wanted numbers, with the Create tests unchanged).

## 7. Tests

All with fixtures built in the repository (the vehicles of `vehicle_spawn_test.go` and `vehicletank_test.go`, whose rows were copied in shape from real vehicles, such as the medium ornithopter of #578), never a live save.

1. **Plan function, table-driven.** For each class and each slot, every module the catalog offers is accepted and gives the `Derive` numbers; a part of another class, of another slot, an unknown id, an empty
   mandatory slot, grade 7 and a vehicle with two chassis are refused with the plain reason (the same style as `TestEveryPartTheCatalogOffersForASlotIsAcceptedByThePreviewAndShowsInItsRows`).
2. **Golden before and after on a test copy.** Build the fixture, dump every row of the vehicle tables for the vehicle (and a hash of all other vehicles' rows), apply a change, dump again, and compare the diff with
   an expected list of changed fields written in the test, one case per group of section 1: an engine (only the row), a hull (row and health), a chassis (row, health, critical, fuel), a storage part (row and the hold),
   a weapon part (row and the stats groups), an optional slot added and removed. The test fails when any field outside the list changes.
3. **No-op and repeat.** The same final set changes no row; a second request with the same `requestId` writes once; a second different one writes again.
4. **Undo.** A hook that changes a row after the write (as `afterVehicleRows` does for Create) proves the checks notice and the whole edit is undone; a refused change leaves the save byte-identical.
5. **Guards.** A world vehicle, a vehicle of another rank holder, a stored vehicle (both states) and a vehicle with an unknown module are refused and change nothing; the hold that would no longer fit its items is
   refused with the numbers.
6. **Queue.** A change made with the game open is queued (the gate), written when it opens; replayed onto a file where the vehicle's modules changed it is not applied and is reported; the journal holds only
   INSERT, UPDATE and DELETE statements.
7. **Integrity.** `pragma foreign_key_check` and `integrity_check` on the fixture after each case, on both save layouts (the CI job for the newer layout runs `./internal/ops`).
8. **Page tests** (`web-tests`, a browser engine): the button is absent for a stored or unknown vehicle with its reason; the drop-downs offer exactly the catalog's parts for the slot; Preview shows the numbers; Apply
   posts what Preview showed and cannot run while a preview is pending; scroll and focus stay after each pick; the text budget holds.
9. **Real-copy test, skipped without a copy**: a test like `TestComposeVehiclesOnACopyOfARealSave` (it reads `TAU_REAL_SAVE_SQLITE`, a decoded copy, and writes only into a temp copy): change a part on each real vehicle of the
   copy, check the foreign keys, the integrity of the file and the derived numbers, and that no other vehicle changed.

Not testable without the game: that the game loads the changed vehicle and keeps it. That is the experiments below, run by the operator in the game on a copy.

## 8. Risks

| Risk | Why it matters | What reduces it |
|---|---|---|
| **The game rebuilds a part from its own rules at load.** | The repository already has two precedents of the game replacing what a program wrote: store and DLC cosmetics are removed on load unless the account owns them (`docs/design/2026-10-24-player-tabs-status.md`, row 5; the customization unlock skips them), and whether the game keeps a raised Landsraad contribution or works it out again from its own events is not known (`docs/save-database.md`, section 3 item 9). A part row it does not accept could be dropped, reset to the chassis's own, or make the whole vehicle vanish or recover. | Experiment A and B below before slice 2; a backup before every edit; the preview says the change is not checked in the game until it has been; the CHANGELOG entry says so. |
| The editor's numbers and the game's disagree for a combination no template has | Create already warns (`BodyUtilitySeen`); a swap makes such combinations easy | the warning in the preview; slices 2 and 3 begin with swaps inside the game's own templates (same hull family) |
| Stale derived numbers | a part change that does not update health, fuel or the hold leaves the vehicle at the old numbers | step 4 to 6 of the plan and the golden test; `verifyVehicle` |
| The part row changes id or an owner row cascades | would silently drop an inventory of the part | update in place; a delete only when nothing points at the row |
| `stats` damaged | rewriting the whole document through Go numbers corrupted gear in the past (audit G10, #188) | SQL JSON functions only, golden test of the whole document |
| Weapon parts | the weapon stats shape of a launcher or a dart module was seen only on a mining laser and a scanner (Create note); the light ornithopter carries a `WeaponActorComponent` with `m_FavoriteWeaponItemDatabaseId` that may refer to the old weapon | slice 3 keeps weapon parts last; a copy query below; if unclear, weapon swaps stay out |
| Cargo hold shrinks under items | items in a hold with no room (the #483 failure) | refused with numbers, never a silent delete |
| A vehicle in a loaded world | the game overwrites the edit | the live-save gate and the queue; the preview says which it will be |
| A queued edit on a changed vehicle | wrong rows overwritten | journal read hashes and `expect` |
| Double click | two writes | `requestId` |
| A new game build changes the module tables | the catalog says the build it was read from | a vehicle with an id the catalog does not know is refused; the snapshot refresh is the existing process |
| Page redraw loses the place | G-010 of the full audit | `render('scroll')` and the page tests |
| Scope creep into items, other vehicles, bulk edits | the issue asks for parts of one vehicle | the scope clause; slice 4 is separate and optional |

## 9. Needs a test copy from the operator

These can only be answered from a real save or from the game. Each is a read of a copy (never the live save), unless it says it is an in-game experiment. The queries are for a copy decoded to plain SQLite.

1. **Module-owned inventories.** `select count(*) from inventories where vehicle_module_id is not null;` and `select * from vehicle_module_inventories;`. If both are empty in every copy (the schema note only says the editor does not use that table),
   the in-place update has nothing to cascade into. Use a copy where a vehicle with a storage part exists.
2. **The shapes of `vehicle_modules.stats`.** `select template_id, json(stats) from vehicle_modules where vehicle_id in (<a few vehicles>) order by vehicle_id, id;` for a vehicle with each of: a launcher, a mining
   laser, a scanner, a boost, a storage part, a unique, a part repaired at a workbench. What is needed: which groups appear for which module kinds, whether any group carries more than durability, swatch and the
   weapon tag (for example a boost or heat state), and the exact fields of `FVehicleWeaponModuleStats` for a launcher.
3. **Health bookkeeping.** `select a.id, json_extract(a.properties,'$.DamageableActorComponent.m_TotalMaxHealth') t, json_extract(a.properties,'$.DamageableActorComponent.m_CurrentMaxHealth') c from actors a where a.id in (select id from vehicles) and t is not c;`
   and the same vehicles' `FHealthComponent` from their Actor entity. Whether `m_CurrentMaxHealth` ever differs from the total (damage to a part lowering it), and whether current health is above the maximum in
   any vehicle.
4. **Row order.** `select vehicle_id, id, template_id from vehicle_modules order by vehicle_id, id;` on a played vehicle that was crafted part by part: is the id order the slot order, and where does a part
   installed later (an optional slot filled after crafting) sit. This decides whether adding a part to an optional slot (a new, higher id) is something the game may not like.
5. **The part item.** `select i.template_id, i.stack_size, i.quality_level, i.position_index, i.stats from items i where i.template_id in ('BuggyEngine_6','BuggyChassis_6','BuggyLocomotion_6', <the other part items>);`
   from a character that carries spare parts. Does a part item carry its wear in `stats`, and in which field. In the game (on a copy): uninstall a part from a vehicle, save, and diff the save for what appeared
   (an item, its stats, its slot). Needed only for slice 4.
6. **The weapon component of the light ornithopter.** `select json_extract(a.properties,'$.WeaponActorComponent') from actors a where a.class like '%LightOrnithopter%';` for a vehicle with a launcher and one
   without. What `m_FavoriteWeaponItemDatabaseId` holds and whether it refers to an item or a module.
7. **The Sandcrawler spice fill.** `select json_extract(a.properties,'$.BP_Sandcrawler_Choam_C.m_SpiceInventoryFillState')` (the class's key is in the catalog as `propsKey`) for crawlers with `SandcrawlerSpiceContainer_0`,
   `_6` and the unique: what the number means (a fraction or an amount) and whether a swap of the container module has to change it.
8. **Mixed-grade vehicles.** `select vehicle_id, group_concat(template_id) from vehicle_modules group by vehicle_id;` over all vehicles: which mixes the game itself produced (a count by pair of slots and grades), so
   the preview's warning for "no template has this" is calibrated on what the game does, not only on templates.
9. **In-game experiments on a copy with a backup**, with the editor writing while the game is closed. Each is one vehicle, one change, then Continue, look, drive two minutes, save and quit, then read the rows again
   and diff:
   - **A. A part with no derived effect** (an engine, Mk3 to Mk6, same class): does the vehicle load, show the new part in its module panel, keep the row, drive.
   - **B. A hull or chassis swap with every derived number written** by the plan: health, critical, fuel and cargo as the plan says; after the save, are the numbers still the plan's?
   - **C. The same swap with the derived numbers deliberately left stale** (only the part row changed): after load and save, did the game recompute health, fuel and the hold, or keep the stale values? This answers the
     first risk of section 8 and says whether the game recomputes at load.
   - **D. A storage part swapped** on a vehicle with a few items in the hold: slots, volume and items after load.
   - **E. The fraction policy**: a half-damaged, half-fuelled vehicle, chassis swapped to another grade: the editor's health and fuel (same fraction) against what looks right in the game.
   - **F. The same set of experiments for a weapon part**, only if slice 3 is to include weapons.
   These are the same kind of check the Create a vehicle note lists ("Checks in the game"); the operator may fold them into the check of Create (its items 1 to 9), because both rest on the same assumption.

## 10. Recommendation

**Build, in slices; do not build the write until one experiment has passed.** The reason: the feature is small in code (one function, one route, one page card on top of rules the editor already has and tests) and the
rules for the numbers are proven against real vehicles, so the cost and the risk of the code are low. The risk that remains is not in the code but in the game: no vehicle with parts chosen or changed by the editor
has been loaded in the game yet, and a part change is a write that the game can silently undo or answer by dropping the vehicle. A short experiment on a test copy settles it before the part of the work that writes.

| Slice | What | Writes the save | Gate |
|---|---|---|---|
| 0 | The queries and experiment A of section 9 (and B and C when slice 3 is wanted), by the operator on a copy | no | the operator's copy and a check in the game |
| 1 | Read-only **Change parts** form and `parts-preview`: the plan function, the derived before and after, every refusal, the page, all tests of section 7 that need no write. Useful on its own as "what would this change", and it settles the page design | no | none; can start after this note is agreed |
| 2 | The write for slots with no derived effect (engine, generator, treads, wheels, wings), part discarded, full durability, one vehicle, the guard of section 4, golden tests | yes | experiment A passed |
| 3 | Hulls, chassis (health, critical, fuel) and storage parts (hold, refusal when items do not fit); weapon parts last and only when query 2 and 6 give their shape | yes | experiments B and C (and D for storage) passed |
| 4 | Optional: the **Set all parts to Mk N** shortcut (closes #651) if it was not done in slice 1; returning the removed part to the backpack and consuming a part item | yes | answers to query 5 |

Not recommended: building slices 2 and 3 first and checking in the game afterwards (the answer to the first risk can change the design, for example if the game rebuilds health at load, the editor should not
write it at all), and a bulk "upgrade every vehicle" in any slice (one vehicle per request keeps a mistake to one vehicle and one backup).

Decisions this note asks for before slice 1:

1. The removed part is discarded in slices 2 and 3 (or the feature waits for slice 4).
2. The new part appears out of nothing, like Create (or the backpack item is consumed, slice 4).
3. Health and fuel keep their fraction on a swap; a hold that would no longer fit its items refuses the change.
4. Treads, wheels and wings are one row in the form, with the positions under it.

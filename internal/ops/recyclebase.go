package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Recycle a standing base and return everything to the character's Reserve (#1176, phase 1). The game's own recycle works on a base backup: it deletes the backup with every
// actor and row of the base, moves what was stored in the base to the character's Reserve (the character's inventory of type 33, which has no size limit) and adds a refund for
// the pieces and placeables. One before and after pair of saves from the game (2026-11-23: a backed-up base of 23 pieces, a totem, a door and a silo with four stacks) showed
// which rows go. This edit does the same to a base that still stands, in one step, with no base backup written at all:
//
//   - deleted: the base's actors (the totem, every placeable the totem owns, the building actors that hold its pieces) with their entity links and entities, their inventories
//     and inventory links, every building piece, the buildings rows, the placeables rows, the totem row, the totem's fuel times, the permission records of the base's actors with
//     their ranks, and the base's own map marker of the character. A standing base also has land claim segments (the measured base, already picked up, had none left): they hang
//     from the totem's actor in the save's own layout and go with it. So do the respawn points that are at the base (player_respawn_locations rows whose locator is one of the
//     base's actors, the "BaseTotem" row of a base the character respawns at): the save's own layout deletes them with the actor;
//   - moved: every item row of every inventory of those actors (chests, the totem's own slots, windtrap filters, generator fuel, refinery and fabricator slots) goes to the
//     Reserve unchanged. Only inventory_id and position_index of a row change, as in the game's own recycle;
//   - added: the refund, as new stacks in the Reserve. Every piece and placeable gives back 100% of what the game's building tables say it costs to place, on every map (the
//     Deep Desert too: 100%, not the half its map table gives a piece there, #1211), with three fixed rules: a Developer storage container gives 10 IronBar, a hologram gives nothing, and a door is
//     counted once (as its own placeable row; the door its frame carries in the tables is the same object and is not added again). A type the tables give no cost is never free:
//     the base is refused and the type named;
//   - dropped: water stored in cisterns and windtraps (it is an amount in the entity, not an item) and a unit of fuel that is already burning.
//
// The game's own recycle returned about half of the table cost in the measured case; returning everything is this editor's rule, by the operator's decision.
//
// A base the game's tool already picked up (a base backup, #1207: the Deep Desert's cannot be placed or recycled in the game) goes through the same plan and the same write. It is found
// by its shape (the totem and every actor of the base in state 5, listed by exactly one base_backups row, no permission record on the totem) and owned by the character when that row
// records the character as its player; the backup must list exactly the actors the totem's entity owns. The write also deletes the backup's rows (base_backup_linked_actors, then
// base_backups), as the game's own recycle of the backup did (probe8), and looks for no map marker (the pick-up took it; the game's recycle changed none).
//
// Refused: a base that is not the character's own (the owner rank, as Back up and remove checks it), a base the game already picked up or that is not in the normal state, a base
// with a crafting queue that is running, a base an actor of which is a vehicle or part of a base backup, a base a character is waiting to respawn at, a base that a row of any
// other table of the save points at (every reference the save's layout declares to an actor, an inventory or an entity is looked up, see recycleHandledRefs), and a save in
// which the character has no Reserve, and a Reserve that holds any stack (#1202; see RecycleOptions). Vehicles are never touched. The same code is the preview (rolled back) and the write; the write must be sent the preview's fingerprint, and it fails whole, changing nothing, when what it reads
// back after writing is not what was planned.

// sqlNameRe is a table or column name that can be written into a statement between double quotes.
var sqlNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// errRecycleDry ends a preview's edit so that it is rolled back.
var errRecycleDry = errors.New("base recycle dry run")

// recycleBeforeReadBack is nil outside tests. A test sets it to change the save between the writes and the read back, to show that a difference fails the whole edit.
var recycleBeforeReadBack func(m *save.Mut)

const (
	reserveInventoryType = 33 // inventories.inventory_type of a character's Reserve
	devStorageType       = "Developer_StorageContainer_Placeable"
	devStorageItem       = "IronBar"
	devStorageAmount     = 10
	recycleNotOwner      = "Only your own bases can be recycled here. Nothing was changed."
	// recycleMaxRefundStacks bounds the rows one recycle adds. The largest base seen needs about 170.
	recycleMaxRefundStacks = 5000
)

// recycleHandledRefs are the references to a row of the base that the recycle itself takes care of: the child rows are deleted with the base, or (items) moved to the Reserve.
// The key is the child table and column, a dot, and the table referred to. Every other reference the save's layout declares to an actor, an inventory, an entity or a permission
// record is looked up in the plan, and a base that such a row points at is refused with the table named: what the game does with that row on a recycle is not known, and an
// edit that left it would leave a dangling reference.
var recycleHandledRefs = map[string]bool{
	"actor_fgl_entities.actor_id>actors": true, "buildings.id>actors": true, "building_instances.building_id>actors": true, "placeables.id>actors": true, "totems.id>actors": true,
	"landclaim_segments.totem_id>actors": true, "totem_fuel_update_times.totem_id>actors": true, "inventories.actor_id>actors": true, "permission_actor.actor_id>actors": true,
	"player_respawn_locations.locator_actor_id>actors":           true,
	"permission_actor_rank.permission_actor_id>permission_actor": true,
	"actor_inventories.inventory_id>inventories":                 true, "items.inventory_id>inventories": true,
	"actor_fgl_entities.entity_id>fgl_entities": true, "building_instances.owner_entity_id>fgl_entities": true, "placeables.owner_entity_id>fgl_entities": true,
}

// recyclePickedUpRefs are the references the recycle of a picked-up base handles besides recycleHandledRefs (#1207): the rows that link the base backup to its actors are deleted
// with the base (base_backup_linked_actors, then the base_backups row they hang from).
var recyclePickedUpRefs = map[string]bool{"base_backup_linked_actors.actor_id>actors": true}

// recycleOtherRefs looks up every reference the save's own layout declares to the rows a recycle deletes, leaves out those the recycle handles, and returns the others that
// have rows, as "table.column (n rows)". pickedUp adds the references only a picked-up base's recycle handles.
func recycleOtherRefs(q baseQ, actors, inventories string, pickedUp bool) ([]string, error) {
	fks, err := q.Query(`select m.name tbl, f."from" col, f."table" ref from sqlite_master m join pragma_foreign_key_list(m.name) f
		where m.type='table' and f."table" in ('actors','totems','placeables','buildings','permission_actor','inventories','fgl_entities') order by 1,2,3`)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range fks {
		tbl, _ := f["tbl"].(string)
		col, _ := f["col"].(string)
		ref, _ := f["ref"].(string)
		if k := tbl + "." + col + ">" + ref; recycleHandledRefs[k] || (pickedUp && recyclePickedUpRefs[k]) {
			continue
		}
		if !sqlNameRe.MatchString(tbl) || !sqlNameRe.MatchString(col) {
			return nil, apierr.Newf(apierr.ErrConflict, "The save has a table or column name this version cannot read (%s.%s), so what points at the base cannot be checked. Nothing was changed.", tbl, col)
		}
		in := actors
		switch ref {
		case "inventories":
			in = inventories
		case "fgl_entities":
			in = `select entity_id from actor_fgl_entities where actor_id in (` + actors + `)`
		}
		r, err := q.One(`select count(*) n from "` + tbl + `" where "` + col + `" in (` + in + `)`)
		if err != nil {
			return nil, err
		}
		if n, _ := toInt(r["n"]); n > 0 {
			out = append(out, fmt.Sprintf("%s.%s (%d rows)", tbl, col, n))
		}
	}
	return out, nil
}

// RefundLine is one item of the refund.
type RefundLine struct {
	Item   string `json:"item"`
	Name   string `json:"name"`
	Count  int64  `json:"count"`
	Stacks int64  `json:"stacks"`
	max    int64
}

type recyclePlan struct {
	Totem          int64
	Ctl, Pawn, Ent int64
	Name, RawName  string
	Map            string
	Actors         []int64 // the totem and every placeable it owns, ascending
	Buildings      []int64 // the building actors that hold the pieces, ascending
	Pieces         int64
	Placeables     int64 // without the totem
	Holograms      int64
	DevStorage     int64
	Unowned        int64
	NoSupport      int64
	Inventories    []int64
	ItemRows       int64
	ItemUnits      int64
	Reserve        int64
	Refund         []RefundLine
	RefundStacks   int64
	Water          int64
	WaterDevices   int64
	RespawnPoints  int64            // respawn points at the base: deleted with it
	Types          string           // every type and its count, for the fingerprint
	Rows           map[string]int64 // rows to delete, by table
	Marker         *baseMarker
	MarkerNote     string
	Solido         *solidoPlan // nil when no Solido is made first (#1187)
	PickedUp       bool        // the base is a base backup the game's tool made (#1207): its actors are in state 5 and a base_backups row lists them
	Backup         int64       // the base_backups id of a picked-up base
}

func (p *recyclePlan) all() []int64 { return append(append([]int64{}, p.Actors...), p.Buildings...) }

// solidoFingerprintPrefix starts the fingerprint of a recycle that makes a Solido first, so that a write sent with the other choice can be told from a base that changed.
const solidoFingerprintPrefix = "solido-"

func (p *recyclePlan) fingerprint() string {
	h := sha256.New()
	fmt.Fprintf(h, "recycle|%d|%d|%d|%d|%s|%s|%v|%v|%s|%v|%d|%d|%d|%d|", p.Totem, p.Ctl, p.Pawn, p.Ent, p.RawName, p.Map, p.Actors, p.Buildings, p.Types, p.Inventories,
		p.ItemRows, p.ItemUnits, p.Reserve, p.Water)
	for _, r := range p.Refund {
		fmt.Fprintf(h, "%s=%d/%d;", r.Item, r.Count, r.Stacks)
	}
	tables := make([]string, 0, len(p.Rows))
	for t := range p.Rows {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		fmt.Fprintf(h, "%s:%d;", t, p.Rows[t])
	}
	if p.Marker != nil {
		fmt.Fprintf(h, "m%d/%d/%s/%d", p.Marker.Hash, p.Marker.Dim, p.Marker.Map, p.Marker.MapID)
	}
	if p.PickedUp { // only a picked-up base, so that a standing base adds nothing here
		fmt.Fprintf(h, "|pickedup|%d", p.Backup)
	}
	if s := p.Solido; s != nil { // only when a Solido is made, so that a recycle without one adds nothing here
		fmt.Fprintf(h, "|solido|%d|%d|%d|%d|%d|%d|%s", s.Item, s.ItemInv, len(s.Rows.Instances), len(s.Rows.Placeables), len(s.Rows.Pentas), s.Rows.SkippedPieces+s.Rows.SkippedDoors, s.Digest)
	}
	if p.Solido != nil {
		return solidoFingerprintPrefix + hex.EncodeToString(h.Sum(nil))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// refundPercent is the share of the table cost that a recycle gives back: all of it, on every map, the Deep Desert included (operator decision, #1211). The game's map table gives the
// Deep Desert a placement cost multiplier of 0.5 (catalog.PlacementCostPercent); the refund does not use it.
const refundPercent int64 = 100

// addCost adds what n units of one type give back to the refund: each item of the type's cost, in full.
func addCost(total, cost map[string]int64, n int64) {
	for item, c := range cost {
		total[item] += c * n
	}
}

// refundTotals works out what the base gives back: for every piece type (rows of t, n) and every placeable type (rows of t, h, n) the cost of one unit times the count, by
// the fixed rules. It returns the sum per item, the types the cost table does not know, and every type with its count (for the fingerprint), and it counts the pieces, the
// holograms and the Developer storage containers into the plan.
func refundTotals(p *recyclePlan, pieces, plcs []save.Row) (map[string]int64, []string, []string) {
	total := map[string]int64{}
	var missing, types []string
	num := func(r save.Row, k string) int64 { n, _ := toInt(r[k]); return n }
	for _, r := range pieces {
		t, _ := r["t"].(string)
		n := num(r, "n")
		p.Pieces += n
		types = append(types, fmt.Sprintf("b:%s=%d", t, n))
		cost, ok := catalog.PieceCost(t)
		if !ok {
			missing = append(missing, t)
			continue
		}
		addCost(total, cost, n)
	}
	for _, r := range plcs {
		t, _ := r["t"].(string)
		n := num(r, "n")
		holo := num(r, "h") != 0
		types = append(types, fmt.Sprintf("p:%s/%v=%d", t, holo, n))
		switch {
		case holo: // a hologram was not paid for
			p.Holograms += n
		case t == devStorageType:
			p.DevStorage += n
			total[devStorageItem] += devStorageAmount * n
		default:
			cost, ok := catalog.PlaceableCost(t)
			if !ok {
				missing = append(missing, t)
				continue
			}
			addCost(total, cost, n)
		}
	}
	return total, missing, types
}

// recycleStandingOwnership checks that the character holds the owner rank (1) over the totem of a standing base, and nobody else does. The same check as Back up and remove (#1173).
func recycleStandingOwnership(q baseQ, totem int64, pl player) error {
	ranks, err := q.Query(`select player_id, rank from permission_actor_rank where permission_actor_id=?`, totem)
	if err != nil {
		return err
	}
	mine, others, owners := false, false, 0
	for _, r := range ranks {
		pid, _ := toInt(r["player_id"])
		rk, _ := toInt(r["rank"])
		if rk == 1 {
			owners++
			if pid == pl.Controller {
				mine = true
			} else {
				others = true
			}
		}
	}
	switch {
	case others:
		return apierr.New(apierr.ErrConflict, "This base belongs to another player (or is shared), so it was not changed. "+recycleNotOwner)
	case !mine && len(ranks) > 0:
		return apierr.New(apierr.ErrConflict, "You are only a member of this base, not its owner. "+recycleNotOwner)
	case !mine || owners != 1:
		return apierr.New(apierr.ErrConflict, "Nobody holds the owner rank over this base, so it is not known to be yours. "+recycleNotOwner)
	}

	return nil
}

// planBaseRecycle reads the save (or the edit's view of it) and works out what the recycle deletes, moves and gives back, or says why this base cannot be recycled. It writes nothing.
func (o *Ops) planBaseRecycle(q baseQ, msql markerSQL, totem int64, pl player, opt RecycleOptions) (*recyclePlan, error) {
	head, err := q.One(`select a.state state, coalesce(a.map,'') map, a.dimension_index dim, a.location_x lx, a.location_y ly, a.location_z lz, coalesce(pa.actor_name,'') name, coalesce(tp.building_type,'') totem_type,
		(select entity_id from actor_fgl_entities where actor_id=t.id and slot_name='Actor' order by entity_id limit 1) ent,
		(select count(*) from base_backup_linked_actors where actor_id=t.id) linked,
		(select min(id) from base_backup_linked_actors where actor_id=t.id) backup,
		exists (select 1 from permission_actor where actor_id=t.id) has_permission
		from totems t join actors a on a.id=t.id left join permission_actor pa on pa.actor_id=t.id left join placeables tp on tp.id=t.id where t.id=?`, totem)
	if err != nil {
		return nil, err
	}
	if head == nil {
		return nil, apierr.Newf(apierr.ErrNotFound, "There is no base with the totem %d in this save.", totem)
	}
	state, _ := toInt(head["state"])
	linked, _ := toInt(head["linked"])
	hasPerm, _ := toInt(head["has_permission"])
	backup, _ := toInt(head["backup"])
	// A picked-up base (#1207) is the shape the game's base tool leaves: the totem's actor and the actors of the base are in state 5, a base_backups row lists them, and the totem has no
	// permission record (the claim is gone). Anything else that is in state 5 or listed in a base backup is not known, and is refused as before.
	pickedUp := state == pickedUpState && linked == 1 && hasPerm == 0
	switch {
	case pickedUp:
	case state == pickedUpState:
		return nil, apierr.New(apierr.ErrConflict, "This base is in the picked-up state, but it is not listed in exactly one base backup or it still has a claim record, so this version does not know what it is. Nothing was changed.")
	case linked > 0:
		return nil, apierr.New(apierr.ErrConflict, "This base is standing and is also listed in a base backup, so it is not known which of the two it is. Nothing was changed.")
	case state != 0:
		return nil, apierr.Newf(apierr.ErrConflict, "This base is not in the normal state (state %d), so it was not changed.", state)
	case hasPerm == 0:
		return nil, apierr.New(apierr.ErrConflict, "This base has no permission record, so it is not known to be yours. "+recycleNotOwner)
	}

	if pickedUp {
		// Ownership of a base backup: it records the character that picked it up. The totem has no permission record to say more.
		bk, err := q.One(`select player_id from base_backups where id=?`, backup)
		if err != nil {
			return nil, err
		}
		if bk == nil {
			return nil, apierr.New(apierr.ErrConflict, "The base backup this base is listed in is not in the save, so it is damaged and was not changed.")
		}
		if pid, _ := toInt(bk["player_id"]); pid != pl.Controller {
			return nil, apierr.New(apierr.ErrConflict, "This base backup belongs to another player, so it was not changed. "+recycleNotOwner)
		}
	} else {
		if err := recycleStandingOwnership(q, totem, pl); err != nil {
			return nil, err
		}
	}

	ent, ok := toInt(head["ent"])
	if !ok {
		return nil, apierr.New(apierr.ErrBadArgument, "The totem of this base has no entity, so its pieces cannot be told from those of another base. Nothing was changed.")
	}
	p := &recyclePlan{Totem: totem, Ctl: pl.Controller, Pawn: pl.Pawn, Ent: ent, Rows: map[string]int64{}, Refund: []RefundLine{}} // an empty refund is a list, not null, for the page
	p.PickedUp, p.Backup = pickedUp, backup
	p.RawName, _ = head["name"].(string)
	totemType, _ := head["totem_type"].(string)
	p.Map, _ = head["map"].(string)
	dim, _ := toInt(head["dim"])
	p.Name = backupDisplayName(p.RawName, totemType)

	// The actors: the totem, every placeable the totem owns, and the building actors that hold its pieces.
	eid := strconv.FormatInt(ent, 10)
	acts, err := q.Query(`select ` + strconv.FormatInt(totem, 10) + ` id union select id from placeables where owner_entity_id=` + eid)
	if err != nil {
		return nil, err
	}
	p.Actors = rowsInt64(acts, "id")
	blds, err := q.Query(`select distinct building_id id from building_instances where owner_entity_id=` + eid)
	if err != nil {
		return nil, err
	}
	p.Buildings = rowsInt64(blds, "id")
	all := idList(p.all())
	bl := "-1" // no building actor: a base of a totem and placeables only
	if len(p.Buildings) > 0 {
		bl = idList(p.Buildings)
	}

	// A standing base has no actor in a base backup and every actor in state 0; a picked-up base's actors are listed in its own base backup only, and are all in state 5, and the
	// backup lists exactly the actors the totem's entity owns (nothing more, nothing less).
	wantState, otherBackup := "0", `select count(*) from base_backup_linked_actors where actor_id in (`+all+`)`
	if pickedUp {
		wantState, otherBackup = strconv.Itoa(pickedUpState), `select count(*) from base_backup_linked_actors where actor_id in (`+all+`) and id<>`+strconv.FormatInt(backup, 10)
		listed, err := q.Query(`select actor_id id from base_backup_linked_actors where id=? order by actor_id`, backup)
		if err != nil {
			return nil, err
		}
		want := p.all()
		sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
		if idList(rowsInt64(listed, "id")) != idList(want) {
			return nil, apierr.Newf(apierr.ErrConflict, "The base backup lists %d actors and the totem's entity owns %d, and they are not the same actors, so the base was not changed.", len(listed), len(p.all()))
		}
	}

	chk, err := q.One(`select
		(select count(*) from actors where id in (` + all + `)) present,
		(select count(*) from actors where id in (` + all + `) and state<>` + wantState + `) odd,
		(select count(*) from vehicles where id in (` + all + `)) vehicles,
		(` + otherBackup + `) linked,
		(select count(*) from permission_actor_rank where permission_actor_id in (` + all + `) and player_id<>` + strconv.FormatInt(pl.Controller, 10) + `) other_ranks,
		(select count(*) from building_instances where building_id in (` + bl + `) and owner_entity_id is not null and owner_entity_id<>` + eid + `
			and owner_entity_id in (select entity_id from actor_fgl_entities where slot_name='Actor' and actor_id in (select id from totems))) shared,
		(select count(*) from actor_fgl_entities where actor_id not in (` + all + `) and entity_id in (select entity_id from actor_fgl_entities where actor_id in (` + all + `))) shared_entities`)
	if err != nil {
		return nil, err
	}
	num := func(r save.Row, k string) int64 { n, _ := toInt(r[k]); return n }
	switch {
	case num(chk, "present") != int64(len(p.Actors)+len(p.Buildings)):
		return nil, apierr.New(apierr.ErrBadArgument, "The base names an actor that is not in the save, so it is damaged and was not changed.")
	case num(chk, "odd") > 0:
		return nil, apierr.Newf(apierr.ErrConflict, "%d actors of this base are not in the %s state, so it was not changed.", num(chk, "odd"), map[bool]string{false: "normal", true: "picked-up"}[pickedUp])
	case num(chk, "vehicles") > 0:
		return nil, apierr.New(apierr.ErrConflict, "An actor of this base is a vehicle. Vehicles are never recycled, so the base was not changed.")
	case num(chk, "linked") > 0:
		return nil, apierr.New(apierr.ErrConflict, "An actor of this base is part of a base backup, so the base was not changed.")
	case pickedUp && num(chk, "other_ranks") > 0:
		return nil, apierr.New(apierr.ErrConflict, "An actor of this base backup is also held by another player (a rank of another player is recorded for it), so the base was not changed. "+recycleNotOwner)
	case num(chk, "shared") > 0:
		return nil, apierr.Newf(apierr.ErrConflict, "A building actor of this base also holds %d pieces of another base, so the base cannot be recycled on its own. Nothing was changed.", num(chk, "shared"))
	case num(chk, "shared_entities") > 0:
		return nil, apierr.New(apierr.ErrConflict, "An entity of this base is also linked to something outside it, so the base was not changed.")
	}

	// A crafting queue that is running holds ingredients that are in no inventory: what the game does with them on a recycle has not been measured, so the base is refused.
	queues, err := q.One(`select count(*) n from actor_fgl_entities ae join fgl_entities fe on fe.entity_id=ae.entity_id
		where ae.actor_id in (` + all + `) and json_type(fe.components,'$.FItemCraftingComponent') is not null
		and coalesce(json_array_length(fe.components,'$.FItemCraftingComponent[1].RequestsQueue'),-1)<>0`)
	if err != nil {
		return nil, err
	}
	if n := num(queues, "n"); n > 0 {
		return nil, apierr.Newf(apierr.ErrConflict, "A crafting queue is running in this base (%d stations), or cannot be read. Let it finish or cancel it in the game, then ask again. Nothing was changed.", n)
	}

	// The refund: every piece and placeable, from the game's building tables.
	pieces, err := q.Query(`select building_type t, count(*) n from building_instances where building_id in (` + bl + `) group by 1 order by 1`)
	if err != nil {
		return nil, err
	}
	plcs, err := q.Query(`select building_type t, is_hologram h, count(*) n from placeables where id in (` + idList(p.Actors) + `) group by 1,2 order by 1,2`)
	if err != nil {
		return nil, err
	}
	total, missing, types := refundTotals(p, pieces, plcs)
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, apierr.Newf(apierr.ErrConflict, "The building cost of %s is not known to this version (the cost table has no entry), so what the base gives back cannot be worked out. Nothing was changed. Report the type so the table can be completed.", strings.Join(missing, ", "))
	}
	p.Types = strings.Join(types, ",")
	items := make([]string, 0, len(total))
	for item := range total {
		items = append(items, item)
	}
	sort.Strings(items)
	for _, item := range items {
		max, ok := catalog.MaxStack(item)
		if !ok || max <= 0 {
			return nil, apierr.Newf(apierr.ErrConflict, "The stack size of %s is not known to this version, so the refund cannot be written as stacks. Nothing was changed. Report the item so the table can be completed.", item)
		}
		n := total[item]
		line := RefundLine{Item: item, Name: item, Count: n, Stacks: (n + max - 1) / max, max: max}
		if name, ok := catalog.Name(item); ok {
			line.Name = name
		}
		p.Refund = append(p.Refund, line)
		p.RefundStacks += line.Stacks
	}
	if p.RefundStacks > recycleMaxRefundStacks {
		return nil, apierr.Newf(apierr.ErrConflict, "The refund of this base is %s stacks, more than one edit writes (%s). Nothing was changed.", commas(p.RefundStacks), commas(recycleMaxRefundStacks))
	}

	// The Reserve: the character's own inventory of type 33. It is found by its type, never by a number.
	res, err := q.Query(`select id from inventories where actor_id=? and inventory_type=? and item_id is null`, pl.Pawn, reserveInventoryType)
	if err != nil {
		return nil, err
	}
	if len(res) != 1 {
		return nil, apierr.Newf(apierr.ErrConflict, "The character has %d Reserve inventories in this save, not one, so there is nowhere certain to put what the base returns. Nothing was changed.", len(res))
	}
	p.Reserve, _ = toInt(res[0]["id"])
	// The game allows one Reserve, so no base is recycled while it holds any stack (#1202). This runs in the plan, so the question (rolled back edit) and the write (which plans again
	// inside its own transaction, on the save as it is then) both refuse, and a Reserve change the game saved between the two is caught by the write.
	held, err := q.One(`select count(*) n from items where inventory_id=?`, p.Reserve)
	if err != nil {
		return nil, err
	}
	if n, _ := toInt(held["n"]); n > 0 && !opt.AllowNonEmptyReserve {
		return nil, ReserveNotEmptyRefusal(n)
	}

	// What is stored: every item row of every inventory of the base's actors.
	invs, err := q.Query(`select id from inventories where actor_id in (` + all + `)`)
	if err != nil {
		return nil, err
	}
	p.Inventories = rowsInt64(invs, "id")
	il := "-1"
	if len(p.Inventories) > 0 {
		il = idList(p.Inventories)
	}
	for _, id := range p.Inventories {
		if id == p.Reserve {
			return nil, apierr.New(apierr.ErrConflict, "The character's Reserve is recorded as an inventory of this base, so the base was not changed.")
		}
	}
	c, err := q.One(`select
		(select count(*) from items where inventory_id in (` + il + `)) item_rows,
		(select coalesce(sum(stack_size),0) from items where inventory_id in (` + il + `)) item_units,
		(select count(*) from placeables where id in (` + idList(p.Actors) + `) and id<>` + strconv.FormatInt(totem, 10) + `) placeables,
		(select count(*) from placeables where id in (` + idList(p.Actors) + `) and has_buildable_support=0) no_support,
		(select count(*) from building_instances where building_id in (` + bl + `) and (owner_entity_id is null or owner_entity_id<>` + eid + `)) unowned,
		(select count(*) from actor_fgl_entities where actor_id in (` + all + `)) actor_fgl_entities,
		(select count(distinct entity_id) from actor_fgl_entities where actor_id in (` + all + `)) fgl_entities,
		(select count(*) from actor_inventories where inventory_id in (` + il + `)) actor_inventories,
		(select count(*) from buildings where id in (` + bl + `)) buildings,
		(select count(*) from placeables where id in (` + idList(p.Actors) + `)) placeables_rows,
		(select count(*) from totems where id=` + strconv.FormatInt(totem, 10) + `) totems,
		(select count(*) from landclaim_segments where totem_id=` + strconv.FormatInt(totem, 10) + `) landclaim_segments,
		(select count(*) from totem_fuel_update_times where totem_id=` + strconv.FormatInt(totem, 10) + `) totem_fuel_update_times,
		(select count(*) from permission_actor_rank where permission_actor_id in (` + all + `)) permission_actor_rank,
		(select count(*) from permission_actor where actor_id in (` + all + `)) permission_actor,
		(select count(*) from player_respawn_locations where locator_actor_id in (` + all + `)) player_respawn_locations,
		(select count(*) from player_state where pending_respawn_location_id is not null and pending_respawn_location_id in (select id from player_respawn_locations where locator_actor_id in (` + all + `))) pending_respawn`)
	if err != nil {
		return nil, err
	}
	p.ItemRows, p.ItemUnits = num(c, "item_rows"), num(c, "item_units")
	p.Placeables, p.NoSupport, p.Unowned = num(c, "placeables"), num(c, "no_support"), num(c, "unowned")
	// A character that is waiting to respawn at this base (player_state.pending_respawn_location_id names its respawn point) would be left pointing at a row that is gone. The
	// save's layout declares no reference there, so nothing would catch it: the base is refused.
	if n := num(c, "pending_respawn"); n > 0 {
		return nil, apierr.New(apierr.ErrConflict, "A character is waiting to respawn at this base (player_state.pending_respawn_location_id names its respawn point). Respawn in the game first, then ask again. Nothing was changed.")
	}
	// Rows of other tables that point at the base: refused by name (see recycleHandledRefs).
	otherRefs, err := recycleOtherRefs(q, all, il, pickedUp)
	if err != nil {
		return nil, err
	}
	if len(otherRefs) > 0 {
		return nil, apierr.Newf(apierr.ErrConflict, "Rows of other tables of the save point at this base, and what the game does with them when a base is recycled is not known to this version: %s. Nothing was changed. Report this so the case can be covered.", strings.Join(otherRefs, ", "))
	}
	p.RespawnPoints = num(c, "player_respawn_locations")
	for _, t := range []string{"actor_fgl_entities", "fgl_entities", "actor_inventories", "buildings", "totems", "landclaim_segments", "totem_fuel_update_times", "permission_actor_rank", "permission_actor", "player_respawn_locations"} {
		p.Rows[t] = num(c, t)
	}
	p.Rows["placeables"] = num(c, "placeables_rows")
	p.Rows["building_instances"] = p.Pieces
	p.Rows["inventories"] = int64(len(p.Inventories))
	p.Rows["actors"] = int64(len(p.Actors) + len(p.Buildings))
	if pickedUp { // the base backup that listed the base goes with it (the rows the game's own recycle of a backup deleted: base_backup_linked_actors and base_backups)
		p.Rows["base_backup_linked_actors"], p.Rows["base_backups"] = int64(len(p.all())), 1
	}

	// The Solido, when it is asked for: made from the base's rows as they are now, in this same edit (#1187).
	if opt.Solido {
		sp, err := planSolido(q, p)
		if err != nil {
			return nil, err
		}
		p.Solido = sp
	}

	// Water: an amount in the device's entity. It is not an item and is not returned.
	w, err := q.One(`select count(*) n, coalesce(sum(cast(json_extract(fe.components,'$.FWaterStorageComponent[1].m_WaterStored') as integer)),0) w
		from actor_fgl_entities ae join fgl_entities fe on fe.entity_id=ae.entity_id
		where ae.actor_id in (` + all + `) and json_type(fe.components,'$.FWaterStorageComponent') is not null`)
	if err != nil {
		return nil, err
	}
	p.WaterDevices, p.Water = num(w, "n"), num(w, "w")

	// A picked-up base has no marker of its own: the game's pick-up took it off the map, and the game's own recycle of a backup changed no marker (probe8). One at the totem's place now
	// would be another base's, so none is looked for.
	if pickedUp {
		return p, nil
	}

	// The base's marker, found as Back up and remove finds it: the character's own discovered HomeBase marker of this map at the totem's position. Only one clearly tied to this
	// base is removed; none or several are left alone and said.
	lx, ok1 := head["lx"].(float64)
	ly, ok2 := head["ly"].(float64)
	lz, ok3 := head["lz"].(float64)
	if !ok1 || !ok2 || !ok3 {
		p.MarkerNote = "The totem has no position, so its map marker was not looked for and none was removed."
		return p, nil
	}
	cols := "pm.marker_hash_id h, pm.dimension_index d, pm.map_name mn"
	if msql.newer {
		cols += ", pm.map_name_id mid"
	}
	ms, err := q.Query(`select `+cols+` from `+msql.player+` pm join `+msql.markers+` mk
		on mk.marker_hash_id=pm.marker_hash_id and mk.dimension_index=pm.dimension_index and mk.map_name=pm.map_name
		where pm.player_id=? and pm.discovery_method=10 and mk.marker_type='HomeBase' and pm.map_name=? and pm.dimension_index=?
		and abs(mk.x-?)<0.5 and abs(mk.y-?)<0.5 and abs(mk.z-?)<0.5`, pl.Controller, p.Map, dim, math.Trunc(lx), math.Trunc(ly), math.Trunc(lz))
	if err != nil {
		return nil, err
	}
	switch len(ms) {
	case 0:
		p.MarkerNote = "No map marker of this base was found, so none was removed."
	case 1:
		h, _ := toInt(ms[0]["h"])
		d, _ := toInt(ms[0]["d"])
		mn, _ := ms[0]["mn"].(string)
		mid, _ := toInt(ms[0]["mid"])
		p.Marker = &baseMarker{Hash: h, Dim: d, Map: mn, MapID: mid}
	default:
		p.MarkerNote = fmt.Sprintf("%d map markers could belong to this base, so none was removed.", len(ms))
	}
	return p, nil
}

// PreviewBaseRecycle says what recycling this base would do: what is deleted, how many item stacks move to the Reserve, the refund per item, what is dropped, and the
// fingerprint the write must be sent. It does the whole edit, reads it back, runs the save layer's two checks on it (CheckReferences and CheckJSON: the checks a write gets
// when its function has returned, which a rolled back edit would otherwise never reach; the refund stacks carry items.stats, a JSON cell) and rolls it back. So a base the plan
// refuses, a base whose read back differs and a base whose edit would leave a dangling reference or a JSON cell the game may not read are all refused here, and nothing in the
// save changes. What a preview cannot see is anything that changes in the save between the question and the answer; that is what the fingerprint is for.
func (o *Ops) PreviewBaseRecycle(totem int64) (any, error) {
	return o.recycleBase(totem, "", true, RecycleOptions{})
}

// RecycleOptions are the choices of a recycle. The zero value refuses a Reserve that holds stacks. Solido makes the game's own Solido of the base first (#1187), in the same edit.
type RecycleOptions struct {
	Solido bool
	// AllowNonEmptyReserve lets the question and the write go on while the Reserve holds stacks. Without it both refuse (#1202: the game allows one Reserve), so a caller that sets nothing
	// has the rule. Only the layout roll sets it: it has its own rule (it moves the Reserve into the boxes before and between its bases, #1191) and asks about its bases before that move.
	AllowNonEmptyReserve bool
}

// PreviewBaseRecycleWith is PreviewBaseRecycle with options.
func (o *Ops) PreviewBaseRecycleWith(totem int64, opt RecycleOptions) (any, error) {
	return o.recycleBase(totem, "", true, opt)
}

// RecycleBase recycles the standing base with this totem: the base is deleted, what it stored and what it cost go to the character's Reserve. fingerprint is the one the
// preview gave.
func (o *Ops) RecycleBase(totem int64, fingerprint string) (any, error) {
	return o.RecycleBaseWith(totem, fingerprint, RecycleOptions{})
}

// RecycleBaseWith is RecycleBase with options; a fingerprint that was made with an option is only good with the same option.
func (o *Ops) RecycleBaseWith(totem int64, fingerprint string, opt RecycleOptions) (any, error) {
	if fingerprint == "" {
		return nil, apierr.New(apierr.ErrBadArgument, "fingerprint is missing: a base is recycled as the question showed it")
	}
	return o.recycleBase(totem, fingerprint, false, opt)
}

func (o *Ops) recycleBase(totem int64, want string, dry bool, opt RecycleOptions) (any, error) {
	if totem <= 0 {
		return nil, apierr.New(apierr.ErrBadArgument, "base must be the number of a base in this save")
	}
	pl, err := o.player()
	if err != nil || pl.Controller <= 0 || pl.Pawn <= 0 {
		return nil, apierr.New(apierr.ErrNotFound, "This save has no character to receive what the base returns, so nothing was changed.")
	}
	var res map[string]any
	msql := markerSQL{newer: o.S.MarkersMapNameID(), player: o.S.PlayerMarkersTable(), markers: o.S.MarkersTable()}
	_, err = o.S.Mutate("Recycle base", func(m *save.Mut) error {
		plan, err := o.planBaseRecycle(m, msql, totem, pl, opt)
		if err != nil {
			return err
		}
		if want != "" && want != plan.fingerprint() {
			if strings.HasPrefix(want, solidoFingerprintPrefix) != (plan.Solido != nil) {
				return apierr.New(apierr.ErrConflict, "The Solido option changed since the question was shown. Nothing was changed. Ask again.")
			}
			return apierr.New(apierr.ErrConflict, "This base has changed since the question was shown. Nothing was changed. Ask again.")
		}
		r, err := applyBaseRecycle(m, msql, plan)
		if err != nil {
			return err
		}
		res = r
		m.Desc = fmt.Sprintf("Recycle base \"%s\": %d pieces, %d placeables, %d item stacks and the refund to the Reserve", importLabel(plan.Name), plan.Pieces, plan.Placeables, plan.ItemRows)
		if plan.PickedUp {
			m.Desc = fmt.Sprintf("Recycle base \"%s\" (picked up, base backup %d): %d pieces, %d placeables, %d item stacks and the refund to the Reserve", importLabel(plan.Name), plan.Backup, plan.Pieces, plan.Placeables, plan.ItemRows)
		}
		if plan.Solido != nil {
			m.Desc = fmt.Sprintf("Make a Solido of the base, then recycle it: \"%s\", %d pieces, %d placeables, %d item stacks and the refund to the Reserve", importLabel(plan.Name), plan.Pieces, plan.Placeables, plan.ItemRows)
		}
		res["dryRun"] = dry
		if dry {
			// The save layer checks the references of an edit when the edit's function has returned without an error. A preview ends with an error (to be rolled back), so
			// it would never get there: the same check is run here, and a base the write would be refused for is refused when the question is asked.
			if err := m.CheckReferences(); err != nil {
				return apierr.Newf(apierr.ErrConflict, "This base cannot be recycled by this version: %v. Report this so the case can be covered.", err)
			}
			// The same for the JSON cells the edit wrote: the refund stacks carry items.stats, which the save layer checks at the end of a write.
			if err := m.CheckJSON(); err != nil {
				return apierr.Newf(apierr.ErrConflict, "This base cannot be recycled by this version: %v. Report this so the case can be covered.", err)
			}
			return errRecycleDry
		}
		return nil
	})
	if dry && errors.Is(err, errRecycleDry) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

// recycleMismatch is the refusal of a write whose read back is not the plan. It is an error of the edit, so the whole edit is rolled back.
func recycleMismatch(what string, got, want int64) error {
	return apierr.Newf(apierr.ErrConflict, "The recycle was not written: %s read back as %d, and the plan was %d. Nothing was changed.", what, got, want)
}

func applyBaseRecycle(m *save.Mut, msql markerSQL, p *recyclePlan) (map[string]any, error) {
	all := idList(p.all())
	il := "-1"
	if len(p.Inventories) > 0 {
		il = idList(p.Inventories)
	}
	one := func(q string, args ...any) (int64, error) {
		r, err := m.One(q, args...)
		if err != nil {
			return 0, err
		}
		n, ok := toInt(r["n"])
		if !ok {
			return 0, errors.New("a count could not be read back")
		}
		return n, nil
	}

	// 0. The Solido, when it is asked for, is made first, from the base's rows as they still are. It is read back after everything else is written (below).
	var solido map[string]any
	var solidoBP int64
	if p.Solido != nil {
		var err error
		if solido, solidoBP, err = applySolido(m, p); err != nil {
			return nil, err
		}
	}

	// 1. What is stored moves to the Reserve, each row to a free position; nothing else of a row changes.
	capRow, err := m.One(`select coalesce(max_item_count,0) cap, (select count(*) from items where inventory_id=i.id) used from inventories i where id=?`, p.Reserve)
	if err != nil {
		return nil, err
	}
	capacity, _ := toInt(capRow["cap"])
	usedCount, _ := toInt(capRow["used"])
	if capacity > 0 && usedCount+p.ItemRows+p.RefundStacks > capacity {
		return nil, apierr.Newf(apierr.ErrConflict, "The Reserve of this save has room for %s stacks and the base returns %s, so nothing was changed.", commas(capacity-usedCount), commas(p.ItemRows+p.RefundStacks))
	}
	used := map[int64]bool{}
	pos, err := m.Query(`select position_index p from items where inventory_id=?`, p.Reserve)
	if err != nil {
		return nil, err
	}
	for _, r := range pos {
		if n, ok := toInt(r["p"]); ok {
			used[n] = true
		}
	}
	stored, err := m.Query(`select id from items where inventory_id in (` + il + `) order by inventory_id, position_index, id`)
	if err != nil {
		return nil, err
	}
	if int64(len(stored)) != p.ItemRows {
		return nil, recycleMismatch("the stored item stacks", int64(len(stored)), p.ItemRows)
	}
	var slot int64
	for _, r := range stored {
		for used[slot] {
			slot++
		}
		used[slot] = true
		res, err := m.Exec(`update items set inventory_id=?, position_index=? where id=?`, p.Reserve, slot, r["id"])
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return nil, recycleMismatch("an item stack that was moved", n, 1)
		}
	}

	// 2. The refund: new stacks of the item's own stack size, after the moved rows.
	var newIDs []int64
	var refundIDs [][]int64
	for _, line := range p.Refund {
		sizes := make([]int64, 0, line.Stacks)
		for left := line.Count; left > 0; left -= line.max {
			if left > line.max {
				sizes = append(sizes, line.max)
			} else {
				sizes = append(sizes, left)
			}
		}
		ids, err := giveRowsInTx(m, p.Reserve, line.Item, sizes, 0)
		if err != nil {
			return nil, err
		}
		if int64(len(ids)) != line.Stacks {
			return nil, recycleMismatch("the refund stacks of "+line.Item, int64(len(ids)), line.Stacks)
		}
		refundIDs = append(refundIDs, ids)
		newIDs = append(newIDs, ids...)
	}

	// 3. The base goes: the rows the game's own recycle deleted, plus the land claim segments a standing base still has.
	deleted := map[string]int64{}
	del := func(table, q string, args ...any) error {
		res, err := m.Exec(q, args...)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		deleted[table] += n
		return nil
	}
	tid := strconv.FormatInt(p.Totem, 10)
	bl := "-1"
	if len(p.Buildings) > 0 {
		bl = idList(p.Buildings)
	}
	steps := []struct{ table, q string }{
		{"actor_inventories", `delete from actor_inventories where inventory_id in (` + il + `)`},
		{"inventories", `delete from inventories where id in (` + il + `)`},
		{"fgl_entities", `delete from fgl_entities where entity_id in (select entity_id from actor_fgl_entities where actor_id in (` + all + `))`},
		{"actor_fgl_entities", `delete from actor_fgl_entities where actor_id in (` + all + `)`},
		{"building_instances", `delete from building_instances where building_id in (` + bl + `)`},
		{"buildings", `delete from buildings where id in (` + bl + `)`},
		{"placeables", `delete from placeables where id in (` + idList(p.Actors) + `)`},
		{"landclaim_segments", `delete from landclaim_segments where totem_id=` + tid},
		{"totem_fuel_update_times", `delete from totem_fuel_update_times where totem_id=` + tid},
		{"totems", `delete from totems where id=` + tid},
		{"player_respawn_locations", `delete from player_respawn_locations where locator_actor_id in (` + all + `)`},
		{"permission_actor_rank", `delete from permission_actor_rank where permission_actor_id in (` + all + `)`},
		{"permission_actor", `delete from permission_actor where actor_id in (` + all + `)`},
		{"actors", `delete from actors where id in (` + all + `)`},
	}
	if p.PickedUp { // the base backup that listed the base: its rows go first (they hang from the actors and from the backup), then the backup itself
		bk := strconv.FormatInt(p.Backup, 10)
		steps = append([]struct{ table, q string }{
			{"base_backup_linked_actors", `delete from base_backup_linked_actors where id=` + bk},
			{"base_backups", `delete from base_backups where id=` + bk},
		}, steps...)
	}
	for _, s := range steps {
		if err := del(s.table, s.q); err != nil {
			return nil, err
		}
	}
	markerRemoved := false
	if mk := p.Marker; mk != nil {
		var err error
		if msql.newer {
			err = del("player_markers", `delete from player_markers where player_id=? and marker_hash_id=? and dimension_index=? and map_name_id=?`, p.Ctl, mk.Hash, mk.Dim, mk.MapID)
		} else {
			err = del("player_markers", `delete from player_markers where player_id=? and marker_hash_id=? and dimension_index=? and map_name=?`, p.Ctl, mk.Hash, mk.Dim, mk.Map)
		}
		if err != nil {
			return nil, err
		}
		if deleted["player_markers"] != 1 {
			return nil, recycleMismatch("the base's map marker", deleted["player_markers"], 1)
		}
		markerRemoved = true
	}

	if recycleBeforeReadBack != nil {
		recycleBeforeReadBack(m)
	}

	// 4. Read back. Every count must be the plan's, or the whole edit fails and nothing is changed.
	if p.Solido != nil {
		item, _ := toInt(solido["item"])
		if err := readBackSolido(m, p, solidoBP, item); err != nil {
			return nil, err
		}
	}
	for _, s := range steps {
		if deleted[s.table] != p.Rows[s.table] {
			return nil, recycleMismatch("the deleted rows of "+s.table, deleted[s.table], p.Rows[s.table])
		}
	}
	backupLeft := "0"
	if p.PickedUp {
		backupLeft = `(select count(*) from base_backups where id=` + strconv.FormatInt(p.Backup, 10) + `) + (select count(*) from base_backup_linked_actors where id=` + strconv.FormatInt(p.Backup, 10) + ` or actor_id in (` + all + `))`
	}
	left, err := one(`select ` + backupLeft + ` +
		(select count(*) from actors where id in (` + all + `)) + (select count(*) from actor_fgl_entities where actor_id in (` + all + `)) +
		(select count(*) from inventories where actor_id in (` + all + `) or id in (` + il + `)) + (select count(*) from actor_inventories where inventory_id in (` + il + `)) +
		(select count(*) from items where inventory_id in (` + il + `)) + (select count(*) from building_instances where building_id in (` + bl + `)) +
		(select count(*) from buildings where id in (` + bl + `)) + (select count(*) from placeables where id in (` + all + `)) + (select count(*) from totems where id=` + tid + `) +
		(select count(*) from landclaim_segments where totem_id=` + tid + `) + (select count(*) from totem_fuel_update_times where totem_id=` + tid + `) +
		(select count(*) from permission_actor where actor_id in (` + all + `)) + (select count(*) from permission_actor_rank where permission_actor_id in (` + all + `)) +
		(select count(*) from player_respawn_locations where locator_actor_id in (` + all + `)) +
		(select count(*) from building_instances where owner_entity_id=` + strconv.FormatInt(p.Ent, 10) + `) + (select count(*) from placeables where owner_entity_id=` + strconv.FormatInt(p.Ent, 10) + `) n`)
	if err != nil {
		return nil, err
	}
	if left != 0 {
		return nil, recycleMismatch("the rows of the base that are left", left, 0)
	}
	if len(stored) > 0 {
		ids := make([]int64, 0, len(stored))
		for _, r := range stored {
			id, _ := toInt(r["id"])
			ids = append(ids, id)
		}
		back, err := m.One(`select count(*) n, coalesce(sum(stack_size),0) u, count(distinct position_index) d from items where inventory_id=? and id in (`+idList(ids)+`)`, p.Reserve)
		if err != nil {
			return nil, err
		}
		n, _ := toInt(back["n"])
		u, _ := toInt(back["u"])
		d, _ := toInt(back["d"])
		if n != p.ItemRows {
			return nil, recycleMismatch("the item stacks moved to the Reserve", n, p.ItemRows)
		}
		if d != p.ItemRows {
			return nil, recycleMismatch("the positions of the item stacks moved to the Reserve", d, p.ItemRows)
		}
		if u != p.ItemUnits {
			return nil, recycleMismatch("the items moved to the Reserve", u, p.ItemUnits)
		}
	}
	for i, line := range p.Refund {
		got, err := one(`select coalesce(sum(stack_size),0) n from items where inventory_id=? and template_id=? and id in (`+idList(refundIDs[i])+`)`, p.Reserve, line.Item)
		if err != nil {
			return nil, err
		}
		if got != line.Count {
			return nil, recycleMismatch("the refund of "+line.Item, got, line.Count)
		}
	}
	if len(newIDs) > 0 {
		n, err := one(`select count(*) n from items where inventory_id=? and id in (`+idList(newIDs)+`)`, p.Reserve)
		if err != nil {
			return nil, err
		}
		if n != p.RefundStacks {
			return nil, recycleMismatch("the refund stacks in the Reserve", n, p.RefundStacks)
		}
	}
	clash, err := one(`select count(*) - count(distinct position_index) n from items where inventory_id=?`, p.Reserve)
	if err != nil {
		return nil, err
	}
	if clash != 0 {
		return nil, recycleMismatch("the Reserve stacks that share a position", clash, 0)
	}

	// 5. The answer, in words.
	warns := []string{}
	if p.NoSupport > 0 {
		warns = append(warns, fmt.Sprintf("%d placeables of this base have no buildable support. They belong to the base's totem, so they are recycled with it.", p.NoSupport))
	}
	if p.Unowned > 0 {
		warns = append(warns, fmt.Sprintf("%d pieces of the building actor belong to no base. They go with the building actor and are refunded like the others.", p.Unowned))
	}
	if p.MarkerNote != "" {
		warns = append(warns, p.MarkerNote)
	}
	refundWords := make([]string, 0, len(p.Refund))
	for _, r := range p.Refund {
		refundWords = append(refundWords, fmt.Sprintf("%s %s", commas(r.Count), r.Name))
	}
	rate := "100% of the building cost"
	summary := []string{}
	if p.PickedUp {
		summary = append(summary, fmt.Sprintf("Deleted: the base backup %d that listed this picked-up base (its record and its %d linked actors).", p.Backup, p.Rows["base_backup_linked_actors"]))
	}
	summary = append(summary,
		fmt.Sprintf("Deleted: the base \"%s\" with %s building pieces, its totem and %s placeables (%s actors with their entities, inventories and permission records), its land claim and its fuel times.",
			p.Name, commas(p.Pieces), commas(p.Placeables), commas(p.Rows["actors"])),
		fmt.Sprintf("Moved: %s item stacks (%s items) that were stored in the base, unchanged.", commas(p.ItemRows), commas(p.ItemUnits)))
	if len(refundWords) > 0 {
		summary = append(summary, fmt.Sprintf("Refund (%s, as %s new stacks): %s.", rate, commas(p.RefundStacks), strings.Join(refundWords, ", ")))
	} else {
		summary = append(summary, "Refund: nothing (the base holds nothing that has a building cost).")
	}
	if p.RespawnPoints > 0 {
		summary = append(summary, fmt.Sprintf("Deleted with the base: %d respawn points at it. You can no longer respawn there.", p.RespawnPoints))
	}
	if p.Holograms > 0 {
		summary = append(summary, fmt.Sprintf("%d holograms give nothing back: they were not paid for.", p.Holograms))
	}
	if p.DevStorage > 0 {
		summary = append(summary, fmt.Sprintf("%d Developer storage containers give %d Iron Ingot each, not their table cost.", p.DevStorage, devStorageAmount))
	}
	if p.Water > 0 {
		summary = append(summary, fmt.Sprintf("Dropped: %s water stored in %d devices. Water is not an item and cannot be returned.", commas(p.Water), p.WaterDevices))
	} else {
		summary = append(summary, "Dropped: nothing (the base stores no water).")
	}
	summary = append(summary, "Everything returned is in the character's Reserve. No base backup was written, and vehicles were not touched.")
	if solido != nil {
		summary = append([]string{solidoWords(solido)}, summary...)
	}

	out := map[string]any{
		"ok": true, "changed": true, "base": p.Totem, "name": p.Name, "map": p.Map, "costPercent": refundPercent,
		"pieces": p.Pieces, "placeables": p.Placeables, "holograms": p.Holograms, "devStorage": p.DevStorage, "actors": p.Rows["actors"],
		"deleted": deleted, "itemStacksMoved": p.ItemRows, "itemsMoved": p.ItemUnits,
		"refund": p.Refund, "refundStacks": p.RefundStacks, "waterDropped": p.Water, "waterDevices": p.WaterDevices,
		"respawnPoints": p.RespawnPoints, "reserveInventory": p.Reserve, "destination": "Reserve", "markerRemoved": markerRemoved,
		"summary": summary, "warnings": warns, "fingerprint": p.fingerprint(),
	}
	if solido != nil {
		out["solido"] = solido
	}
	if p.PickedUp {
		out["pickedUp"], out["backup"] = true, p.Backup
	}
	return out, nil
}

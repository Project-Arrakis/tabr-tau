package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Back up and remove a base the way the game's own base backup does (#1127, bases only). The game's tool picks a base up: the base leaves the world and becomes a base backup that
// the player can place again with the same tool. This edit writes exactly the rows a real pick-up wrote in a before and after pair of saves from the game (the probe of 2026-11-23,
// a small base of 73 pieces, one totem and one placeable):
//
//   - the totem and the placeables that stand on the base go to actors.state 5 in place (same id, serial one higher);
//   - the building actor that holds the pieces is not changed in place: it is deleted (with its entity) and made again under a NEW id, with the same class, map, dimension and
//     transform, state 5, serial 0, no properties and no entity. The buildings row and every building_instances row of the old actor move to the new id;
//   - one base_backups row (the character's controller as player and as last editor, named as the totem's permission record was) and one base_backup_linked_actors row for each of
//     the totem, the placeables and the new building actors;
//   - the totem's permission_actor and permission_actor_rank rows are deleted (the claim is gone);
//   - the base's own marker (HomeBase, discovered by the character, at the totem's position) is deleted from player_markers.
//
// Not touched, on purpose: what is stored in the base's containers (items, inventories, actor_inventories: a second probe, with a chest of four stacks, showed the game leaves them
// exactly where they are, the chest actor going to state 5 in place), the totem row, the claim's land segments, the totem's fuel times, the placeables that have no buildable support
// (the game's pick-up does not take them either), the markers table itself.
//
// Only the character's own bases: the totem must hold the owner rank (1) for the character and nobody else, must stand (state 0) and must not be a base the game already picked up.
// One base per edit. The same code is the preview (rolled back) and the write, and the write must be sent the fingerprint the preview gave: what was previewed is what is written.

// errBaseRemoveDry ends a preview's edit so that it is rolled back.
var errBaseRemoveDry = errors.New("base removal dry run")

// baseQ is what a plan needs to read: the save itself (the list and the preview's first look) or an edit's transaction.
type baseQ interface {
	Query(q string, args ...any) ([]save.Row, error)
	One(q string, args ...any) (save.Row, error)
}

// baseMarker is the row of player_markers that belongs to the base, by its key in the save's layout.
type baseMarker struct {
	Hash, Dim int64
	Map       string
	MapID     int64
}

type removalPlan struct {
	Totem         int64
	Name, RawName string
	TotemType     string
	Map           string
	Dim           int64
	Ctl           int64
	Ent           int64
	Actors        []int64 // the totem and the placeables the pick-up takes, ascending
	Buildings     []int64 // the building actors that hold the pieces, ascending
	Pieces        int64
	Placeables    int64 // taken, without the totem
	LeftStanding  int64 // placeables of the base that have no buildable support: not taken
	StoredItems   int64 // items in the base's containers: not touched
	Unowned       int64 // pieces of the building actors that no base owns: they move with the actor
	Marker        *baseMarker
	MarkerNote    string
	buildingExtra map[int64]save.Row // the old building actors' rows, for the new ones
}

func (p *removalPlan) fingerprint() string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%d|%d|%s|%v|%v|%d|%d|%d|%d|", p.Totem, p.Ctl, p.Ent, p.RawName, p.Actors, p.Buildings, p.Pieces, p.Placeables, p.LeftStanding, p.StoredItems)
	if p.Marker != nil {
		fmt.Fprintf(h, "m%d/%d/%s/%d", p.Marker.Hash, p.Marker.Dim, p.Marker.Map, p.Marker.MapID)
	}
	return hex.EncodeToString(h.Sum(nil))
}

const (
	removeAlready  = "This base has already been picked up with the game's base tool, so there is nothing to back up."
	removeNotOwner = "Only your own bases can be backed up and removed here. Nothing was changed."
	// the game does not take a base backup of a base in the Deep Desert (#1177), so a backup row for one would be a state the game never produces
	removeDeepDesert = "A base in the Deep Desert cannot be backed up and removed here: the game does not back up bases there. A recycle of Deep Desert bases is planned (#1176). Nothing was changed."
)

func idList(ids []int64) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(s, ",")
}

func rowsInt64(rows []save.Row, col string) []int64 {
	var out []int64
	for _, r := range rows {
		if v, ok := toInt(r[col]); ok {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// planBaseRemoval reads the save (or the edit's view of it) and works out what the pick-up takes, or says why this base cannot be backed up. It writes nothing.
func (o *Ops) planBaseRemoval(q baseQ, msql markerSQL, totem int64, ctl int64) (*removalPlan, error) {
	head, err := q.One(`select a.state state, coalesce(a.map,'') map, a.dimension_index dim, a.location_x lx, a.location_y ly, a.location_z lz, coalesce(pa.actor_name,'') name, coalesce(tp.building_type,'') totem_type,
		(select entity_id from actor_fgl_entities where actor_id=t.id and slot_name='Actor' order by entity_id limit 1) ent,
		exists (select 1 from base_backup_linked_actors where actor_id=t.id) linked,
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
	if state == pickedUpState || (linked == 1 && hasPerm == 0) {
		return nil, apierr.New(apierr.ErrConflict, removeAlready)
	}
	if state != 0 {
		return nil, apierr.Newf(apierr.ErrConflict, "This base is not in the normal state (state %d), so it was not changed.", state)
	}

	// The Deep Desert: the game takes no base backup there (#1177). Refused in the preview and in the write alike, before anything is read or changed.
	if isDeepDesertMap(fmt.Sprint(head["map"])) {
		return nil, apierr.New(apierr.ErrConflict, removeDeepDesert)
	}

	// Ownership: the character holds the owner rank (1) over the totem, and nobody else does.
	ranks, err := q.Query(`select player_id, rank from permission_actor_rank where permission_actor_id=?`, totem)
	if err != nil {
		return nil, err
	}
	mine, others, owners := false, false, 0
	for _, r := range ranks {
		pid, _ := toInt(r["player_id"])
		rk, _ := toInt(r["rank"])
		if rk == 1 {
			owners++
			if pid == ctl {
				mine = true
			} else {
				others = true
			}
		}
	}
	switch {
	case others:
		return nil, apierr.New(apierr.ErrConflict, "This base belongs to another player (or is shared), so it was not changed. "+removeNotOwner)
	case !mine && len(ranks) > 0:
		return nil, apierr.New(apierr.ErrConflict, "You are only a member of this base, not its owner. "+removeNotOwner)
	case !mine || owners != 1:
		return nil, apierr.New(apierr.ErrConflict, "Nobody holds the owner rank over this base, so it is not known to be yours. "+removeNotOwner)
	}

	ent, ok := toInt(head["ent"])
	if !ok {
		return nil, apierr.New(apierr.ErrBadArgument, "The totem of this base has no entity, so its pieces cannot be told from those of another base. Nothing was changed.")
	}
	p := &removalPlan{Totem: totem, Ctl: ctl, Ent: ent, buildingExtra: map[int64]save.Row{}}
	p.RawName, _ = head["name"].(string)
	p.TotemType, _ = head["totem_type"].(string)
	p.Map, _ = head["map"].(string)
	p.Dim, _ = toInt(head["dim"])
	p.Name = backupDisplayName(p.RawName, p.TotemType)

	// The actors the game's pick-up takes: the totem, the placeables the totem owns that stand on the base (buildable support) and the building actors that hold its pieces.
	eid := strconv.FormatInt(ent, 10)
	acts, err := q.Query(`select ` + strconv.FormatInt(totem, 10) + ` id union select id from placeables where owner_entity_id=` + eid + ` and has_buildable_support=1`)
	if err != nil {
		return nil, err
	}
	p.Actors = rowsInt64(acts, "id")
	blds, err := q.Query(`select distinct building_id id from building_instances where owner_entity_id=` + eid)
	if err != nil {
		return nil, err
	}
	p.Buildings = rowsInt64(blds, "id")
	if len(p.Buildings) == 0 {
		return nil, apierr.New(apierr.ErrBadArgument, "This base has no building pieces, so it has no building actor to back up. Nothing was changed.")
	}
	// an actor that is not in the normal state is part of something else (a backup, a travel); the pick-up is for standing bases
	odd, err := q.One(`select count(*) n from actors where id in (` + idList(append(append([]int64{}, p.Actors...), p.Buildings...)) + `) and state<>0`)
	if err != nil {
		return nil, err
	}
	if n, _ := toInt(odd["n"]); n > 0 {
		return nil, apierr.Newf(apierr.ErrConflict, "%d actors of this base are not in the normal state, so it was not changed.", n)
	}
	for _, b := range p.Buildings {
		row, err := q.One(`select class, map, location_x, location_y, location_z, rotation_x, rotation_y, rotation_z, rotation_w, dimension_index, owner_account_id, (select count(*) from buildings where id=a.id) has_row from actors a where id=?`, b)
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, apierr.Newf(apierr.ErrBadArgument, "The base names the building actor %d, which is not in the save, so it is damaged and was not changed.", b)
		}
		p.buildingExtra[b] = row
	}
	// a building actor that holds the pieces of another base cannot be moved with this one
	shared, err := q.One(`select count(*) n from building_instances where building_id in (` + idList(p.Buildings) + `) and owner_entity_id is not null and owner_entity_id<>` + eid + `
		and owner_entity_id in (select entity_id from actor_fgl_entities where slot_name='Actor' and actor_id in (select id from totems))`)
	if err != nil {
		return nil, err
	}
	if n, _ := toInt(shared["n"]); n > 0 {
		return nil, apierr.Newf(apierr.ErrConflict, "A building actor of this base also holds %d pieces of another base, so the base cannot be backed up on its own. Nothing was changed.", n)
	}

	// what is counted for the question
	r, err := q.One(`select
		(select count(*) from building_instances where building_id in (`+idList(p.Buildings)+`)) pieces,
		(select count(*) from building_instances where building_id in (`+idList(p.Buildings)+`) and (owner_entity_id is null or owner_entity_id<>`+eid+`)) unowned,
		(select count(*) from placeables where owner_entity_id=`+eid+` and has_buildable_support=1 and id<>?) placeables,
		(select count(*) from placeables where owner_entity_id=`+eid+` and has_buildable_support=0 and id<>?) left_standing,
		(select coalesce(sum(stack_size),0) from items where inventory_id in (select id from inventories where actor_id in (select id from placeables where owner_entity_id=`+eid+`) or actor_id=?)) stored`,
		totem, totem, totem)
	if err != nil {
		return nil, err
	}
	p.Pieces, _ = toInt(r["pieces"])
	p.Unowned, _ = toInt(r["unowned"])
	p.Placeables, _ = toInt(r["placeables"])
	p.LeftStanding, _ = toInt(r["left_standing"])
	p.StoredItems, _ = toInt(r["stored"])

	// The base's marker: the character's own discovered HomeBase marker (discovery method 10) of this map, at the totem's position (the game writes the totem's position cut to whole
	// numbers). Only one clearly tied to this base is removed; none or several are left alone and said.
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
		and abs(mk.x-?)<0.5 and abs(mk.y-?)<0.5 and abs(mk.z-?)<0.5`, ctl, p.Map, p.Dim, math.Trunc(lx), math.Trunc(ly), math.Trunc(lz))
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

// PreviewBaseRemoval says what backing up and removing this base would do: the name, the pieces, the placeables, what is left, and the fingerprint the write must be sent. It does
// the whole edit and rolls it back, so a base the edit cannot be written for is refused here, and nothing in the save changes.
func (o *Ops) PreviewBaseRemoval(totem int64) (any, error) { return o.removeBase(totem, "", true) }

// BackUpAndRemoveBase backs up the base with this totem as a base backup and removes it from the world, as the game's own tool does. fingerprint is the one the preview gave.
func (o *Ops) BackUpAndRemoveBase(totem int64, fingerprint string) (any, error) {
	if fingerprint == "" {
		return nil, apierr.New(apierr.ErrBadArgument, "fingerprint is missing: a base is backed up as the question showed it")
	}
	return o.removeBase(totem, fingerprint, false)
}

func (o *Ops) removeBase(totem int64, want string, dry bool) (any, error) {
	if totem <= 0 {
		return nil, apierr.New(apierr.ErrBadArgument, "base must be the number of a base in this save")
	}
	pl, err := o.player()
	if err != nil || pl.Controller <= 0 {
		return nil, apierr.New(apierr.ErrNotFound, "This save has no character to own the base backup, so nothing was changed.")
	}
	var res map[string]any
	msql := markerSQL{newer: o.S.MarkersMapNameID(), player: o.S.PlayerMarkersTable(), markers: o.S.MarkersTable()}
	_, err = o.S.Mutate("Back up and remove base", func(m *save.Mut) error {
		plan, err := o.planBaseRemoval(m, msql, totem, pl.Controller)
		if err != nil {
			return err
		}
		if want != "" && want != plan.fingerprint() {
			return apierr.New(apierr.ErrConflict, "This base has changed since the question was shown. Nothing was changed. Ask again.")
		}
		r, err := o.applyBaseRemoval(m, msql, plan, pl)
		if err != nil {
			return err
		}
		res = r
		m.Desc = fmt.Sprintf("Back up and remove base \"%s\": %d pieces, %d placeables", importLabel(plan.Name), plan.Pieces, plan.Placeables)
		res["dryRun"] = dry
		if dry {
			return errBaseRemoveDry
		}
		return nil
	})
	if dry && errors.Is(err, errBaseRemoveDry) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (o *Ops) applyBaseRemoval(m *save.Mut, msql markerSQL, p *removalPlan, pl player) (map[string]any, error) {
	exec := func(q string, args ...any) error {
		_, err := m.Exec(q, args...)
		return err
	}
	backup, err := nextID(m, "base_backups")
	if err != nil {
		return nil, err
	}
	first, err := nextID(m, "actors")
	if err != nil {
		return nil, err
	}
	if first+int64(len(p.Buildings)) >= maxNewID || backup >= maxNewID {
		return nil, apierr.New(apierr.ErrBadArgument, "This save has no room left for new ids, so nothing was changed.")
	}

	// the totem and the placeables: picked up in place
	for _, id := range p.Actors {
		if err := exec(`update actors set state=?, serial=serial+1 where id=?`, pickedUpState, id); err != nil {
			return nil, err
		}
	}
	// the building actors: made again under a new id, with the pieces and the buildings row moved to it, then the old actor and its entity are deleted
	newIDs := make([]int64, 0, len(p.Buildings))
	for i, old := range p.Buildings {
		nid := first + int64(i)
		newIDs = append(newIDs, nid)
		b := p.buildingExtra[old]
		// gas_attributes and properties take the table's default (empty), as the game leaves them; the serial starts again
		if err := exec(`insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,owner_account_id,serial,state)
			values(?,?,?,?,?,?,?,?,?,?,?,?,0,?)`, nid, b["class"], b["map"], b["location_x"], b["location_y"], b["location_z"], b["rotation_x"], b["rotation_y"], b["rotation_z"], b["rotation_w"],
			b["dimension_index"], b["owner_account_id"], pickedUpState); err != nil {
			return nil, err
		}
		if n, _ := toInt(b["has_row"]); n > 0 {
			if err := exec(`insert into buildings(id, owner_id) values(?, NULL)`, nid); err != nil {
				return nil, err
			}
		}
		if err := exec(`update building_instances set building_id=? where building_id=?`, nid, old); err != nil {
			return nil, err
		}
		if err := exec(`delete from buildings where id=?`, old); err != nil {
			return nil, err
		}
		ents, err := m.Query(`select entity_id e from actor_fgl_entities where actor_id=?`, old)
		if err != nil {
			return nil, err
		}
		if err := exec(`delete from actor_fgl_entities where actor_id=?`, old); err != nil {
			return nil, err
		}
		for _, e := range ents {
			if err := exec(`delete from fgl_entities where entity_id=?`, e["e"]); err != nil {
				return nil, err
			}
		}
		if err := exec(`delete from actors where id=?`, old); err != nil {
			return nil, err
		}
	}
	// the backup and its links
	name := p.RawName
	if name == "" || name == "None" {
		name = "##" + p.TotemType
	}
	if err := exec(`insert into base_backups(id, player_id, base_backup_name, last_edited_by_player_id) values(?,?,?,?)`, backup, pl.Controller, name, pl.Controller); err != nil {
		return nil, err
	}
	for _, id := range append(append([]int64{}, p.Actors...), newIDs...) {
		if err := exec(`insert into base_backup_linked_actors(id, actor_id) values(?,?)`, backup, id); err != nil {
			return nil, err
		}
	}
	// the claim is gone: the totem's permission record and its ranks
	if err := exec(`delete from permission_actor_rank where permission_actor_id=?`, p.Totem); err != nil {
		return nil, err
	}
	if err := exec(`delete from permission_actor where actor_id=?`, p.Totem); err != nil {
		return nil, err
	}
	markerRemoved := false
	if mk := p.Marker; mk != nil {
		var err error
		if msql.newer {
			err = exec(`delete from player_markers where player_id=? and marker_hash_id=? and dimension_index=? and map_name_id=?`, pl.Controller, mk.Hash, mk.Dim, mk.MapID)
		} else {
			err = exec(`delete from player_markers where player_id=? and marker_hash_id=? and dimension_index=? and map_name=?`, pl.Controller, mk.Hash, mk.Dim, mk.Map)
		}
		if err != nil {
			return nil, err
		}
		markerRemoved = true
	}

	warns := []string{}
	if p.LeftStanding > 0 {
		warns = append(warns, fmt.Sprintf("%d placeables of this base have no buildable support. The game's pick-up does not take them, so they stay as they are.", p.LeftStanding))
	}
	if p.Unowned > 0 {
		warns = append(warns, fmt.Sprintf("%d pieces of the building actor belong to no base. They move with the building actor, as the game moves it whole.", p.Unowned))
	}
	if p.MarkerNote != "" {
		warns = append(warns, p.MarkerNote)
	}
	if p.StoredItems > 0 {
		warns = append(warns, fmt.Sprintf("%d items are stored in this base. They stay where they are, with the base backup, as the game's own pick-up leaves them (seen in the game with a chest of four stacks).", p.StoredItems))
	}
	return map[string]any{
		"ok": true, "changed": true, "base": p.Totem, "name": p.Name, "map": p.Map, "backupId": backup, "backupName": name,
		"pieces": p.Pieces, "placeables": p.Placeables, "leftStanding": p.LeftStanding, "storedItems": p.StoredItems,
		"actors": len(p.Actors) + len(newIDs), "oldBuildingActors": p.Buildings, "newBuildingActors": newIDs,
		"markerRemoved": markerRemoved, "warnings": warns, "fingerprint": p.fingerprint(),
	}, nil
}

// markerSQL is how the save names the markers tables (they differ between the game's two layouts). It is worked out before the edit starts: the save's own lock is held during an edit.
type markerSQL struct {
	newer           bool
	player, markers string
}

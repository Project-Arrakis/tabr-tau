package ops

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Duplicate this vehicle is the console's Spawn Vehicle for a single-player save (#112): a copy of one of the character's own vehicles, set down next to the
// character. The editor cannot ask the game to spawn one, so the copy is made of the rows a vehicle is made of in a save the game wrote (read from a real one):
//
//	actors                  the vehicle, with the reference's class, turn, properties and gas attributes
//	vehicles                its id
//	permission_actor        its record (type 2) and the character's permission_actor_rank (rank 1), which makes it the character's; the ranks of other players are not copied
//	actor_fgl_entities      two entities, one linked as Actor and one as VehicleInventory, and fgl_entities with their components
//	vehicle_modules         the modules (chassis, engine, hull, treads, ...), with the reference's stats
//	inventories             the cargo hold and the component holds, empty, and actor_inventories with their component hashes
//
// The cargo of the reference is not copied. The serial number and the blobs of the entities and the modules are copied as the reference has them. Whether the game loads such a copy has not been checked in the game.

var classNameRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,120}$`)

// abandonedStamp is the time format of a vehicle's m_AbandonedTimestamp property ("2026.10.21-23.11.10", UTC).
const abandonedStamp = "2006.01.02-15.04.05"

// nextID is the id the next row of an AUTOINCREMENT table gets: above the largest id in use and above the table's sequence (a row the game deleted left its number
// behind). It is read inside the edit, and the insert names the id, so the statements an edit keeps for a put-back are the same ones whatever the file does.
func nextID(m *save.Mut, table string) (int64, error) {
	r, err := m.One(`select max(coalesce((select max(id) from `+table+`),0), coalesce((select seq from sqlite_sequence where name=?),0)) n`, table)
	if err != nil {
		return 0, err
	}
	n, ok := toInt(r["n"])
	if !ok {
		return 0, fmt.Errorf("could not read the next id of %s", table)
	}
	return n + 1, nil
}

// newEntityIDs gives a copy its own entity ids. An entity id is a 32-bit prefix and a 32-bit counter (the entities of one vehicle share the prefix and sit a few
// counts apart). Each new id keeps the prefix and the distance to the others, and starts above every counter the prefix already has, so it is not one the
// save holds. taken is every entity id in the save.
func newEntityIDs(ids []int64, taken map[int64]bool) ([]int64, error) {
	maxLow := map[uint64]uint64{}
	for id := range taken {
		u := uint64(id)
		if low := u & 0xFFFFFFFF; low >= maxLow[u>>32] {
			maxLow[u>>32] = low
		}
	}
	minLow := map[uint64]uint64{}
	for _, id := range ids {
		u := uint64(id)
		if cur, ok := minLow[u>>32]; !ok || u&0xFFFFFFFF < cur {
			minLow[u>>32] = u & 0xFFFFFFFF
		}
	}
	out := make([]int64, len(ids))
	for i, id := range ids {
		u := uint64(id)
		prefix := u >> 32
		low := maxLow[prefix] + 10 + (u&0xFFFFFFFF - minLow[prefix])
		if low > 0xFFFFFFFF {
			return nil, errors.New("this save has no room left for new entity ids next to the vehicle's own")
		}
		n := int64(prefix<<32 | low)
		if taken[n] {
			return nil, errors.New("an entity id for the copy is already in use")
		}
		out[i] = n
	}
	return out, nil
}

// DuplicateVehicle copies one of the character's own vehicles (args id) and puts the copy next to the character, as Bring vehicle does. The copy has no cargo.
func (o *Ops) DuplicateVehicle(a Args) (any, error) {
	id, err := a.Int("id")
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	if p.Pawn == 0 {
		return nil, errors.New("this save has no character body to put the copy next to")
	}
	var newID int64
	var nMods, nInvs int
	if _, err := o.S.Mutate(fmt.Sprintf("duplicate vehicle %d", id), func(m *save.Mut) error {
		src, err := m.One(`select class from actors where id=? and id in (`+ownedVehicleSQL+`)`, id, p.Controller)
		if err != nil {
			return err
		}
		if src == nil {
			return apierr.New(apierr.ErrNotFound, "that is not one of your vehicles")
		}
		if err := refuseStored(m.One, id, "duplicated"); err != nil {
			return err
		}
		cls := blueprintClass(src["class"])
		me, err := m.One(`select map, location_x x, location_y y, location_z z, dimension_index d from actors where id=?`, p.Pawn)
		if err != nil {
			return err
		}
		if me == nil {
			return errors.New("player position unknown")
		}
		if newID, err = nextID(m, "actors"); err != nil {
			return err
		}
		// next to the character, 800 along x and 150 up, as Bring vehicle puts a vehicle; the turn and everything else are the reference's
		if _, err := m.Exec(`insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,gas_attributes,properties,owner_account_id,serial,state)
			select ?,class,?,?+800,?,?+150,rotation_x,rotation_y,rotation_z,rotation_w,?,gas_attributes,properties,owner_account_id,serial,state from actors where id=?`,
			newID, me["map"], me["x"], me["y"], me["z"], me["d"], id); err != nil {
			return err
		}
		// a copy that is just set down has not been abandoned since the reference was: its abandoned time is now
		if classNameRe.MatchString(cls) {
			path := `$."` + cls + `".m_AbandonedTimestamp`
			if _, err := m.Exec(`update actors set properties=jsonb_set(properties, ?, ?) where id=? and json_type(properties, ?) is not null`, path, time.Now().UTC().Format(abandonedStamp), newID, path); err != nil {
				return err
			}
		}
		if _, err := m.Exec(`insert into vehicles(id) values (?)`, newID); err != nil {
			return err
		}
		if _, err := m.Exec(`insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id)
			select ?,actor_name,actor_type,access_level,is_child,edited_by_player_id from permission_actor where actor_id=?`, newID, id); err != nil {
			return err
		}
		// only the character's own rank: the copy is the character's, and the rank another player may hold on the original is not carried over to it (CR-10)
		if _, err := m.Exec(`insert into permission_actor_rank(permission_actor_id,player_id,rank) select ?,player_id,rank from permission_actor_rank where permission_actor_id=? and player_id=?`, newID, id, p.Controller); err != nil {
			return err
		}
		if err := duplicateEntities(m, id, newID); err != nil {
			return err
		}
		if nMods, err = duplicateModules(m, id, newID); err != nil {
			return err
		}
		if nInvs, err = duplicateInventories(m, id, newID); err != nil {
			return err
		}
		m.Desc = fmt.Sprintf("duplicated vehicle %d as vehicle %d (%d modules, %d inventories, no cargo)", id, newID, nMods, nInvs)
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "id": newID, "modules": nMods, "inventories": nInvs}, nil
}

// duplicateEntities copies the entities a vehicle is linked to, under new ids.
func duplicateEntities(m *save.Mut, from, to int64) error {
	rows, err := m.Query(`select entity_id e, slot_name s from actor_fgl_entities where actor_id=? order by slot_name`, from)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	var old []int64
	for _, r := range rows {
		e, ok := toInt(r["e"])
		if !ok {
			return errors.New("an entity of the vehicle has no usable id")
		}
		old = append(old, e)
	}
	all, err := m.Query(`select entity_id e from fgl_entities`)
	if err != nil {
		return err
	}
	taken := make(map[int64]bool, len(all))
	for _, r := range all {
		if e, ok := toInt(r["e"]); ok {
			taken[e] = true
		}
	}
	fresh, err := newEntityIDs(old, taken)
	if err != nil {
		return err
	}
	for i, r := range rows {
		if _, err := m.Exec(`insert into fgl_entities(entity_id,components) select ?,components from fgl_entities where entity_id=?`, fresh[i], old[i]); err != nil {
			return err
		}
		if _, err := m.Exec(`insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (?,?,?)`, to, fresh[i], r["s"]); err != nil {
			return err
		}
	}
	return nil
}

// duplicateModules copies the vehicle's modules with their stats. It returns how many.
func duplicateModules(m *save.Mut, from, to int64) (int, error) {
	rows, err := m.Query(`select id from vehicle_modules where vehicle_id=? order by id`, from)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	next, err := nextID(m, "vehicle_modules")
	if err != nil {
		return 0, err
	}
	for i, r := range rows {
		if _, err := m.Exec(`insert into vehicle_modules(id,vehicle_id,template_id,stats) select ?,?,template_id,stats from vehicle_modules where id=?`, next+int64(i), to, r["id"]); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

// duplicateInventories gives the copy empty inventories of the same kinds and sizes, with the component hashes the game looks them up by. An inventory that hangs
// from an item or a module is not copied: where the copy's would hang from is not something the editor can know, so the whole copy is refused.
func duplicateInventories(m *save.Mut, from, to int64) (int, error) {
	rows, err := m.Query(`select id, item_id, vehicle_module_id from inventories where actor_id=? order by id`, from)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	for _, r := range rows {
		if r["item_id"] != nil || r["vehicle_module_id"] != nil {
			return 0, errors.New("this vehicle has an inventory that hangs from an item or a module, and the editor does not know how to copy that, so nothing was copied")
		}
	}
	next, err := nextID(m, "inventories")
	if err != nil {
		return 0, err
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		id, _ := toInt(r["id"])
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, old := range ids {
		n := next + int64(i)
		if _, err := m.Exec(`insert into inventories(id,actor_id,item_id,vehicle_module_id,inventory_type,max_item_count,max_item_volume)
			select ?,?,NULL,NULL,inventory_type,max_item_count,max_item_volume from inventories where id=?`, n, to, old); err != nil {
			return 0, err
		}
		if _, err := m.Exec(`insert into actor_inventories(inventory_id,component_name_hash) select ?,component_name_hash from actor_inventories where inventory_id=?`, n, old); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

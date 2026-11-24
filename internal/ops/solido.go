package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// A Solido of a standing base (#1187): the game's own copy of a base, made before the base is recycled. It is one BuildingBlueprint_CopyDevice item with a building_blueprints
// row and the rows of three child tables. The conversion from the base's rows to the blueprint's rows was measured on a pair of saves from the game (before and after the
// player made a Solido of a Hagga base: 2,141 building pieces and 254 placeables went in, 2,133 pieces and 251 placeables came out), and what is not measured is refused, not
// guessed. What was measured:
//
//   - Origin. Every transform is relative to the base's building actor (its location, and its yaw), not to the totem and not to the world. A piece's blueprint position is its
//     world position minus the actor's, turned by minus the actor's yaw; its blueprint yaw is its yaw minus the actor's. Height is a plain difference.
//   - Pieces. All of the base's pieces except those whose building_flags has bit 4 set (8 of 2,141 in the measured base: exactly the 8 that the game left out; the bit's meaning is
//     not known). provides_stability is bit 1 of building_flags (flags 0 and 2 gave 0, flag 1 gave 1: 282 of 282). health is 0 and hologram is 1 for every row.
//   - Placeables. All the placeables the totem owns except the totem itself and the doors and gates (a type with "Door" in its name: both doors of the base were left out, and no
//     other type was; the gate Gate_M_SquishUp_Placeable is the door Door_S_SquishUp_Placeable's sibling and is left out the same way, by the same rule, though no gate was in the
//     measured base: see solidoIsDoor). Holograms and placeables with no hit on the ground are kept. Position as for a piece. The three angle columns hold, in the game's own labelling, transform_pitch
//     = the yaw, transform_yaw = the pitch and transform_roll = the roll (the console's own import swaps the same two names). hologram is 1.
//   - Pentashields. One row for each Pentashield placeable that is kept: the three scale numbers are m_PersistentScale.X, .Y and .Z of the placeable's properties, times 10
//     (27 of 27 values; a scale of 8.000014 is 80).
//   - Numbering. The game numbers both from 0 with no gaps, in an order that is not that of the base's own ids (it repeats the type in runs, and the placeables look hash
//     ordered). The numbers are only labels: nothing refers to one except the pentashield rows, which name the placeable's new number. This edit numbers pieces by the piece's
//     own instance id and placeables by their actor id, ascending, which is also the order the base was built in.
//
// Not measured, so refused when it comes up: a base with more than one building actor, a base with no building actor, a building actor that is tilted, a piece or placeable that is
// tilted (pitch or roll), a Pentashield with no readable scale, a type with no name. Nothing was ever seen with a tilt in the measured saves.
const (
	solidoDevice          = "BuildingBlueprint_CopyDevice"
	solidoStatsKey        = "FBuildingBlueprintItemStats"
	solidoBackpackType    = 0  // the character's backpack: where a new device goes first, as the console and the editor's give put one
	solidoLoadoutType     = 15 // the character's loadout (the 8-slot hotbar): where a new device goes only when the backpack has no room
	solidoBlankStats      = `{"FItemStackAndDurabilityStats":[[],{"DecayedMaxDurability":0.0}],"FCustomizationStats":[[],{}]}`
	solidoSkipFlag        = 4 // building_flags bit of a piece the game leaves out
	solidoStabilityFlag   = 1
	solidoTiltLimit       = 1e-3 // a quaternion component that is more than this is a tilt
	solidoScaleStep       = 10.0
	solidoMaxScale        = 1000
	solidoNoDeviceMessage = "The Solido could not be made: "
)

type solidoInstance struct {
	ID                int64
	Type              string
	X, Y, Z, Yaw      float64
	ProvidesStability int64
	Source            int64 // the piece's own instance id
}

type solidoPlaceable struct {
	ID                        int64
	Type                      string
	X, Y, Z, Yaw, Pitch, Roll float64
	Source                    int64 // the placeable's actor id
}

type solidoPenta struct {
	Placeable int64 // the blueprint's placeable number
	X, Y, Z   int64
}

// solidoRows is the content of a Solido: what goes into the three child tables.
type solidoRows struct {
	Instances  []solidoInstance
	Placeables []solidoPlaceable
	Pentas     []solidoPenta
	// what was left out, for the answer
	SkippedPieces int64
	SkippedDoors  int64
	SkippedTotems int64
}

// fmtF writes a number so that it reads the same from the save (a minus zero is written as a zero: SQLite does not keep its sign).
func fmtF(f float64) string {
	if f == 0 {
		f = 0
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// digest is a hash of every row, in the table's own order, written the way the read back writes it.
func (r *solidoRows) digest() string {
	h := sha256.New()
	for _, i := range r.Instances {
		fmt.Fprintf(h, "i|%d|%s|%s|%s|%s|%s|%d|0|1\n", i.ID, i.Type, fmtF(i.X), fmtF(i.Y), fmtF(i.Z), fmtF(i.Yaw), i.ProvidesStability)
	}
	for _, p := range r.Placeables {
		fmt.Fprintf(h, "p|%d|%s|%s|%s|%s|%s|%s|%s|1\n", p.ID, p.Type, fmtF(p.X), fmtF(p.Y), fmtF(p.Z), fmtF(p.Pitch), fmtF(p.Yaw), fmtF(p.Roll))
	}
	for _, s := range r.Pentas {
		fmt.Fprintf(h, "s|%d|%d|%d|%d\n", s.Placeable, s.X, s.Y, s.Z)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// solidoSource is what a Solido is made from: the building actor and the base's own rows.
type solidoSource struct {
	OriginX, OriginY, OriginZ    float64
	OriginQX, OriginQY, OriginQZ float64
	OriginQW                     float64
	Pieces                       []solidoSrcPiece
	Placeables                   []solidoSrcPlaceable
	TotemID                      int64
}

type solidoSrcPiece struct {
	ID             int64
	Type           string
	X, Y, Z        float64
	QX, QY, QZ, QW float64
	Flags          int64
}

type solidoSrcPlaceable struct {
	ID             int64
	Type           string
	X, Y, Z        float64
	QX, QY, QZ, QW float64
	Scale          [3]float64 // m_PersistentScale of the properties, when it has one
	HasScale       bool
}

// solidoQuat turns a (maybe not exactly unit) quaternion into a unit one.
func solidoQuat(x, y, z, w float64) (float64, float64, float64, float64, bool) {
	n := math.Sqrt(x*x + y*y + z*z + w*w)
	if n < 1e-9 || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, 0, 0, 1, false
	}
	return x / n, y / n, z / n, w / n, true
}

// solidoAngles are the yaw, pitch and roll in degrees of a unit quaternion (the usual Unreal order: yaw about Z, pitch about Y, roll about X).
func solidoAngles(x, y, z, w float64) (yaw, pitch, roll float64) {
	yaw = math.Atan2(2*(w*z+x*y), 1-2*(y*y+z*z))
	s := 2 * (w*y - z*x)
	if s > 1 {
		s = 1
	} else if s < -1 {
		s = -1
	}
	pitch = math.Asin(s)
	roll = math.Atan2(2*(w*x+y*z), 1-2*(x*x+y*y))
	const d = 180 / math.Pi
	return yaw * d, pitch * d, roll * d
}

// solidoScale turns the three numbers of m_PersistentScale (of a Pentashield's properties: 8.000014, 6.000018, 1.0) into the three whole numbers of the blueprint's scale (80, 60,
// 10): ten times each, to the nearest whole number. A number that is not within a twentieth of a whole step is an error, never rounded away.
func solidoScale(v [3]float64, ok bool) ([3]int64, error) {
	var out [3]int64
	if !ok {
		return out, errors.New("no m_PersistentScale")
	}
	for i, x := range v {
		s := x * solidoScaleStep
		if math.IsNaN(s) || math.IsInf(s, 0) || math.Abs(s-math.Round(s)) > 0.05 || s < 0 || s > solidoMaxScale {
			return out, fmt.Errorf("the scale %s is not a whole number of steps", fmtF(x))
		}
		out[i] = int64(math.Round(s))
	}
	return out, nil
}

// solidoIsDoor says whether a placeable type is one the Solido leaves out as a door: a type with "Door" in its name, and the gate that is a door's sibling
// (Gate_M_SquishUp_Placeable beside Door_S_SquishUp_Placeable, and the same with an owner's prefix, as Choam_Door_S_SquishUp_Placeable). The cost table holds no other Gate type
// among the placeables (the other Gate types in it are building pieces, which the game's flag decides on, not the name).
func solidoIsDoor(t string) bool {
	return strings.Contains(t, "Door") || strings.Contains(t, "Gate_M_SquishUp")
}

// convertSolido is the whole conversion, from a base's rows to a Solido's rows. It reads nothing and writes nothing.
func convertSolido(src solidoSource) (*solidoRows, error) {
	bx, by, bz, bw, ok := solidoQuat(src.OriginQX, src.OriginQY, src.OriginQZ, src.OriginQW)
	if !ok {
		return nil, errors.New("the building actor has no rotation")
	}
	if math.Abs(bx) > solidoTiltLimit || math.Abs(by) > solidoTiltLimit {
		return nil, errors.New("the building actor is tilted, and a tilt was not measured")
	}
	byaw, _, _ := solidoAngles(bx, by, bz, bw)
	cs, sn := math.Cos(byaw*math.Pi/180), math.Sin(byaw*math.Pi/180)
	// the relative rotation: the conjugate of the actor's, then the piece's
	rel := func(qx, qy, qz, qw float64) (float64, float64, float64, float64, bool) {
		x, y, z, w, ok := solidoQuat(qx, qy, qz, qw)
		if !ok {
			return 0, 0, 0, 1, false
		}
		ax, ay, az, aw := -bx, -by, -bz, bw
		return aw*x + ax*w + ay*z - az*y, aw*y - ax*z + ay*w + az*x, aw*z + ax*y - ay*x + az*w, aw*w - ax*x - ay*y - az*z, true
	}
	local := func(x, y, z float64) (float64, float64, float64) {
		dx, dy := x-src.OriginX, y-src.OriginY
		return dx*cs + dy*sn, -dx*sn + dy*cs, z - src.OriginZ
	}
	norm := func(a float64) float64 { // (-180, 180]
		a = math.Mod(a, 360)
		if a > 180 {
			a -= 360
		} else if a <= -180 {
			a += 360
		}
		return a
	}
	out := &solidoRows{}
	pieces := append([]solidoSrcPiece(nil), src.Pieces...)
	sort.Slice(pieces, func(i, j int) bool { return pieces[i].ID < pieces[j].ID })
	for _, p := range pieces {
		if p.Type == "" {
			return nil, fmt.Errorf("piece %d has no type", p.ID)
		}
		if p.Flags&solidoSkipFlag != 0 {
			out.SkippedPieces++
			continue
		}
		x, y, z, w, ok := rel(p.QX, p.QY, p.QZ, p.QW)
		if !ok {
			return nil, fmt.Errorf("piece %d has no rotation", p.ID)
		}
		if math.Abs(x) > solidoTiltLimit || math.Abs(y) > solidoTiltLimit {
			return nil, fmt.Errorf("piece %d (%s) is tilted, and a tilt was not measured", p.ID, p.Type)
		}
		yaw, _, _ := solidoAngles(x, y, z, w)
		lx, ly, lz := local(p.X, p.Y, p.Z)
		out.Instances = append(out.Instances, solidoInstance{ID: int64(len(out.Instances)), Type: p.Type, X: lx, Y: ly, Z: lz, Yaw: norm(yaw), ProvidesStability: p.Flags & solidoStabilityFlag, Source: p.ID})
	}
	plcs := append([]solidoSrcPlaceable(nil), src.Placeables...)
	sort.Slice(plcs, func(i, j int) bool { return plcs[i].ID < plcs[j].ID })
	for _, p := range plcs {
		if p.ID == src.TotemID || strings.HasPrefix(p.Type, "Totem") {
			out.SkippedTotems++
			continue
		}
		if p.Type == "" {
			return nil, fmt.Errorf("placeable %d has no type", p.ID)
		}
		if solidoIsDoor(p.Type) {
			out.SkippedDoors++
			continue
		}
		x, y, z, w, ok := rel(p.QX, p.QY, p.QZ, p.QW)
		if !ok {
			return nil, fmt.Errorf("placeable %d has no rotation", p.ID)
		}
		if math.Abs(x) > solidoTiltLimit || math.Abs(y) > solidoTiltLimit {
			return nil, fmt.Errorf("placeable %d (%s) is tilted, and a tilt was not measured", p.ID, p.Type)
		}
		yaw, pitch, roll := solidoAngles(x, y, z, w)
		lx, ly, lz := local(p.X, p.Y, p.Z)
		n := int64(len(out.Placeables))
		out.Placeables = append(out.Placeables, solidoPlaceable{ID: n, Type: p.Type, X: lx, Y: ly, Z: lz, Yaw: norm(yaw), Pitch: pitch, Roll: roll, Source: p.ID})
		if strings.Contains(p.Type, "Pentashield") {
			sc, err := solidoScale(p.Scale, p.HasScale)
			if err != nil {
				return nil, fmt.Errorf("the scale of the Pentashield %d could not be read (%v)", p.ID, err)
			}
			out.Pentas = append(out.Pentas, solidoPenta{Placeable: n, X: sc[0], Y: sc[1], Z: sc[2]})
		}
	}
	if len(out.Instances)+len(out.Placeables) == 0 {
		return nil, errors.New("the base has nothing a Solido can hold")
	}
	return out, nil
}

// loadSolidoSource reads what a Solido is made from: the base's one building actor, the pieces the totem owns, and the placeables it owns.
func loadSolidoSource(q baseQ, totem, ent int64, actors, buildings []int64) (solidoSource, error) {
	var src solidoSource
	src.TotemID = totem
	if len(buildings) != 1 {
		return src, fmt.Errorf("the base has %d building actors, and a Solido was measured only for a base with one", len(buildings))
	}
	o, err := q.One(`select location_x lx, location_y ly, location_z lz, rotation_x qx, rotation_y qy, rotation_z qz, rotation_w qw from actors where id=?`, buildings[0])
	if err != nil {
		return src, err
	}
	if o == nil {
		return src, errors.New("the building actor is not in the save")
	}
	f := func(r save.Row, k string) (float64, error) {
		switch v := r[k].(type) {
		case float64:
			return v, nil
		case int64:
			return float64(v), nil
		}
		return 0, fmt.Errorf("%s is not a number", k)
	}
	read := func(r save.Row, keys ...string) ([]float64, error) {
		out := make([]float64, len(keys))
		for i, k := range keys {
			v, err := f(r, k)
			if err != nil {
				return nil, err
			}
			out[i] = v
		}
		return out, nil
	}
	v, err := read(o, "lx", "ly", "lz", "qx", "qy", "qz", "qw")
	if err != nil {
		return src, fmt.Errorf("the building actor: %v", err)
	}
	src.OriginX, src.OriginY, src.OriginZ, src.OriginQX, src.OriginQY, src.OriginQZ, src.OriginQW = v[0], v[1], v[2], v[3], v[4], v[5], v[6]
	rows, err := q.Query(`select instance_id id, building_type t, location_x x, location_y y, location_z z, rotation_x qx, rotation_y qy, rotation_z qz, rotation_w qw, coalesce(building_flags,0) fl
		from building_instances where building_id=? and owner_entity_id=? order by instance_id`, buildings[0], ent)
	if err != nil {
		return src, err
	}
	for _, r := range rows {
		id, _ := toInt(r["id"])
		fl, _ := toInt(r["fl"])
		t, _ := r["t"].(string)
		v, err := read(r, "x", "y", "z", "qx", "qy", "qz", "qw")
		if err != nil {
			return src, fmt.Errorf("piece %d: %v", id, err)
		}
		src.Pieces = append(src.Pieces, solidoSrcPiece{ID: id, Type: t, X: v[0], Y: v[1], Z: v[2], QX: v[3], QY: v[4], QZ: v[5], QW: v[6], Flags: fl})
	}
	prows, err := q.Query(`select p.id id, p.building_type t, a.location_x x, a.location_y y, a.location_z z, a.rotation_x qx, a.rotation_y qy, a.rotation_z qz, a.rotation_w qw
		from placeables p join actors a on a.id=p.id where p.id in (` + idList(actors) + `) order by p.id`)
	if err != nil {
		return src, err
	}
	for _, r := range prows {
		id, _ := toInt(r["id"])
		t, _ := r["t"].(string)
		v, err := read(r, "x", "y", "z", "qx", "qy", "qz", "qw")
		if err != nil {
			return src, fmt.Errorf("placeable %d: %v", id, err)
		}
		src.Placeables = append(src.Placeables, solidoSrcPlaceable{ID: id, Type: t, X: v[0], Y: v[1], Z: v[2], QX: v[3], QY: v[4], QZ: v[5], QW: v[6]})
	}
	// A Pentashield's scale is in its properties, under the actor's own class name: {"BP_...C":{"m_PersistentScale":{"X":8.0,"Y":6.0,"Z":1.0}}}.
	scales, err := q.Query(`select p.id id,
		(select json_extract(j.value,'$.m_PersistentScale.X') from json_each(a.properties) j where json_type(j.value,'$.m_PersistentScale') is not null limit 1) sx,
		(select json_extract(j.value,'$.m_PersistentScale.Y') from json_each(a.properties) j where json_type(j.value,'$.m_PersistentScale') is not null limit 1) sy,
		(select json_extract(j.value,'$.m_PersistentScale.Z') from json_each(a.properties) j where json_type(j.value,'$.m_PersistentScale') is not null limit 1) sz
		from placeables p join actors a on a.id=p.id where p.id in (` + idList(actors) + `) and p.building_type like '%Pentashield%'`)
	if err != nil {
		return src, err
	}
	at := map[int64]int{}
	for i, pl := range src.Placeables {
		at[pl.ID] = i
	}
	for _, r := range scales {
		id, _ := toInt(r["id"])
		x, ok1 := r["sx"].(float64)
		y, ok2 := r["sy"].(float64)
		z, ok3 := r["sz"].(float64)
		if i, ok := at[id]; ok && ok1 && ok2 && ok3 {
			src.Placeables[i].Scale, src.Placeables[i].HasScale = [3]float64{x, y, z}, true
		}
	}
	return src, nil
}

// solidoPlan is what the recycle will do about the Solido: the rows, and the item that carries it.
type solidoPlan struct {
	Rows    *solidoRows
	Digest  string
	Item    int64  // the blank device that is used; 0 when a new one is made
	ItemInv int64  // where the new device goes, 0 when Item is used
	Where   string // "backpack" or "loadout": the kind of inventory ItemInv is
	Pieces  int64  // the base's pieces and placeables, for the answer
	Plcs    int64
}

// solidoRefusal is the refusal of the option: the base is not changed.
func solidoRefusal(why string) error {
	return apierr.Newf(apierr.ErrConflict, "%s%s Nothing was changed. Recycle the base without the Solido, or fix this first.", solidoNoDeviceMessage, why)
}

// planSolido plans the Solido of the base. It reads, in the edit's own transaction, everything the write will use.
func planSolido(q baseQ, p *recyclePlan) (*solidoPlan, error) {
	src, err := loadSolidoSource(q, p.Totem, p.Ent, p.Actors, p.Buildings)
	if err != nil {
		return nil, solidoRefusal(err.Error() + ".")
	}
	rows, err := convertSolido(src)
	if err != nil {
		return nil, solidoRefusal(err.Error() + ".")
	}
	sp := &solidoPlan{Rows: rows, Digest: rows.digest(), Pieces: int64(len(src.Pieces)), Plcs: int64(len(src.Placeables))}

	// The device: the lowest id of the character's own blank ones. A blank one has no blueprint record in its stats and no building_blueprints row. A filled one is never reused.
	devs, err := q.Query(`select i.id id from items i join inventories v on v.id=i.inventory_id
		where v.actor_id=? and v.inventory_type in (0,1,15) and i.template_id=? and i.stack_size>=1
		and (i.stats is null or (json_valid(i.stats) and json_type(i.stats)='object' and json_type(i.stats,'$.`+solidoStatsKey+`') is null))
		and not exists (select 1 from building_blueprints b where b.item_id=i.id) order by i.id limit 1`, p.Pawn, solidoDevice)
	if err != nil {
		return nil, err
	}
	if len(devs) > 0 {
		sp.Item, _ = toInt(devs[0]["id"])
		return sp, nil
	}
	// None: a new device, in the character's backpack (as the console and the editor's give do), or in the loadout when the backpack has no room. Only when both are full is it refused.
	var why []string
	for _, c := range []struct {
		where string
		typ   int
	}{{"backpack", solidoBackpackType}, {"loadout", solidoLoadoutType}} {
		invs, err := q.Query(`select v.id id from inventories v where v.actor_id=? and v.inventory_type=? and v.item_id is null order by v.id limit 1`, p.Pawn, c.typ)
		if err != nil {
			return nil, err
		}
		if len(invs) == 0 {
			why = append(why, "there is no "+c.where)
			continue
		}
		inv, _ := toInt(invs[0]["id"])
		room, err := solidoRoom(q, inv)
		if err != nil {
			return nil, err
		}
		if room != "" {
			why = append(why, "the "+c.where+" "+room)
			continue
		}
		sp.ItemInv, sp.Where = inv, c.where
		return sp, nil
	}
	return nil, solidoRefusal("The character has no blank Solido device, and a new one has no room: " + strings.Join(why, ", and ") + ". Free a slot in the game.")
}

// solidoRoom says why a new device does not fit into the inventory ("" when it does): the slot check and the volume check of the editor's own give.
func solidoRoom(q baseQ, inv int64) (string, error) {
	free, err := freeSlotsInTx(q, inv)
	if err != nil {
		return "", err
	}
	if free == 0 {
		return "has no free slot", nil
	}
	fit, checked, _, err := volumeLeftInTx(q, inv, solidoDevice)
	if err != nil {
		return "", err
	}
	if checked && fit == 0 {
		return "has no room by volume", nil
	}
	return "", nil
}

// withBlueprintStats adds the blueprint record to an item's stats, after the keys that are there, as the game writes it. The stats must be an object that has no record yet.
func withBlueprintStats(stats string, blueprint int64) (string, error) {
	rec := fmt.Sprintf(`"%s":[[],{"PlayerBlueprintId":"!!bbp#%d","PlayerBaseBackupId":{}}]`, solidoStatsKey, blueprint)
	s := strings.TrimSpace(stats)
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil || m == nil {
		return "", errors.New("the device's stats are not a JSON object")
	}
	if _, has := m[solidoStatsKey]; has {
		return "", errors.New("the device already holds a blueprint")
	}
	if !strings.HasSuffix(s, "}") {
		return "", errors.New("the device's stats are not a JSON object")
	}
	body := strings.TrimSpace(strings.TrimSuffix(s, "}"))
	if body == "{" {
		return "{" + rec + "}", nil
	}
	return body + "," + rec + "}", nil
}

// applySolido writes the Solido: the device (a new one when the plan says so), the blueprint row, the three child tables and the blueprint record in the device's stats. It must
// run before the base's rows are deleted, and it reads back what it wrote. It returns the part of the answer that describes it.
func applySolido(m *save.Mut, p *recyclePlan) (map[string]any, int64, error) {
	sp := p.Solido
	item, created := sp.Item, false
	if item == 0 {
		ids, err := giveRowsInTx(m, sp.ItemInv, solidoDevice, []int64{1}, 0)
		switch {
		case errors.Is(err, errInventoryFull), errors.Is(err, errNotEnoughRoom):
			return nil, 0, solidoRefusal("The character has no blank Solido device, and the " + sp.Where + ", where a new one would go, has no free slot. Free a slot in the game.")
		case err != nil:
			return nil, 0, err
		}
		item, created = ids[0], true
		// what the game writes for this item: its own two records (a giveRowsInTx copy of a filled device's stats would carry that device's blueprint)
		if _, err := m.Exec(`update items set stats=? where id=?`, solidoBlankStats, item); err != nil {
			return nil, 0, err
		}
	}
	// The blueprint's number is read through the guarded reader (hashed, so a queued copy of this edit is put back only onto a file where the next number is the same) and
	// written as given: the table's own counter and its highest row, whichever is higher, plus one.
	nx, err := m.One(`select max(coalesce((select max(id) from building_blueprints),0), coalesce((select seq from sqlite_sequence where name='building_blueprints'),0)) n`)
	if err != nil {
		return nil, 0, err
	}
	last, ok := toInt(nx["n"])
	if !ok || last < 0 {
		return nil, 0, errors.New("the next blueprint number could not be read")
	}
	bp := last + 1
	if _, err := m.Exec(`insert into building_blueprints(id, item_id, player_id, building_blueprint_map) values (?, ?, null, '')`, bp, item); err != nil {
		return nil, 0, err
	}
	for _, i := range sp.Rows.Instances {
		if _, err := m.Exec(`insert into building_blueprint_instances(building_blueprint_id, instance_id, building_type, transform_x, transform_y, transform_z, transform_yaw, provides_stability, health, hologram)
			values (?,?,?,?,?,?,?,?,0,1)`, bp, i.ID, i.Type, i.X, i.Y, i.Z, i.Yaw, i.ProvidesStability); err != nil {
			return nil, 0, err
		}
	}
	for _, pl := range sp.Rows.Placeables {
		// the game's labels: the column named transform_yaw holds the pitch, and transform_pitch holds the yaw
		if _, err := m.Exec(`insert into building_blueprint_placeables(building_blueprint_id, placeable_id, building_type, transform_x, transform_y, transform_z, transform_yaw, transform_pitch, transform_roll, hologram)
			values (?,?,?,?,?,?,?,?,?,1)`, bp, pl.ID, pl.Type, pl.X, pl.Y, pl.Z, pl.Pitch, pl.Yaw, pl.Roll); err != nil {
			return nil, 0, err
		}
	}
	for _, s := range sp.Rows.Pentas {
		if _, err := m.Exec(`insert into building_blueprint_pentashields(building_blueprint_id, placeable_id, scale_x, scale_y, scale_z) values (?,?,?,?,?)`, bp, s.Placeable, s.X, s.Y, s.Z); err != nil {
			return nil, 0, err
		}
	}
	st, err := m.One(`select stats from items where id=?`, item)
	if err != nil {
		return nil, 0, err
	}
	if st == nil {
		return nil, 0, errors.New("the Solido device is not in the save")
	}
	cur, _ := st["stats"].(string)
	if cur == "" {
		cur = "{}"
	}
	next, err := withBlueprintStats(cur, bp)
	if err != nil {
		return nil, 0, solidoRefusal(err.Error() + ".")
	}
	if r, err := m.Exec(`update items set stats=? where id=?`, next, item); err != nil {
		return nil, 0, err
	} else if n, _ := r.RowsAffected(); n != 1 {
		return nil, 0, recycleMismatch("the Solido device", n, 1)
	}
	return map[string]any{
		"made": true, "blueprint": bp, "item": item, "itemCreated": created, "itemWhere": sp.Where,
		"pieces": int64(len(sp.Rows.Instances)), "placeables": int64(len(sp.Rows.Placeables)), "pentashields": int64(len(sp.Rows.Pentas)),
		"piecesLeftOut": sp.Rows.SkippedPieces, "doorsLeftOut": sp.Rows.SkippedDoors,
	}, bp, nil
}

// readBackSolido reads the Solido that applySolido wrote and says whether it is the plan: the counts, every row (as a digest), the device and its record, the blueprint's owner.
func readBackSolido(m *save.Mut, p *recyclePlan, bp, item int64) error {
	sp := p.Solido
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
	for _, c := range []struct {
		what, tbl string
		want      int
	}{{"the Solido's pieces", "building_blueprint_instances", len(sp.Rows.Instances)}, {"the Solido's placeables", "building_blueprint_placeables", len(sp.Rows.Placeables)},
		{"the Solido's pentashields", "building_blueprint_pentashields", len(sp.Rows.Pentas)}} {
		n, err := one(`select count(*) n from `+c.tbl+` where building_blueprint_id=?`, bp)
		if err != nil {
			return err
		}
		if n != int64(c.want) {
			return recycleMismatch(c.what, n, int64(c.want))
		}
	}
	n, err := one(`select count(*) n from building_blueprints where id=? and item_id=? and player_id is null and building_blueprint_map=''`, bp, item)
	if err != nil {
		return err
	}
	if n != 1 {
		return recycleMismatch("the Solido's blueprint row", n, 1)
	}
	n, err = one(`select count(*) n from building_blueprints where item_id=?`, item)
	if err != nil {
		return err
	}
	if n != 1 {
		return recycleMismatch("the blueprints of the Solido device", n, 1)
	}
	n, err = one(`select count(*) n from items where id=? and template_id=? and json_extract(stats,'$.`+solidoStatsKey+`[1].PlayerBlueprintId')=?`, item, solidoDevice, fmt.Sprintf("!!bbp#%d", bp))
	if err != nil {
		return err
	}
	if n != 1 {
		return recycleMismatch("the Solido device's record of its blueprint", n, 1)
	}
	got := &solidoRows{}
	is, err := m.Query(`select instance_id id, building_type t, transform_x x, transform_y y, transform_z z, transform_yaw yaw, provides_stability s, health h, hologram g
		from building_blueprint_instances where building_blueprint_id=? order by instance_id`, bp)
	if err != nil {
		return err
	}
	for _, r := range is {
		id, _ := toInt(r["id"])
		s, _ := toInt(r["s"])
		if h, g := r["h"], r["g"]; fmt.Sprint(h) != "0" || fmt.Sprint(g) != "1" {
			return recycleMismatch("the health or hologram flag of a Solido piece", 0, 1)
		}
		t, _ := r["t"].(string)
		x, _ := r["x"].(float64)
		y, _ := r["y"].(float64)
		z, _ := r["z"].(float64)
		yaw, _ := r["yaw"].(float64)
		got.Instances = append(got.Instances, solidoInstance{ID: id, Type: t, X: x, Y: y, Z: z, Yaw: yaw, ProvidesStability: s})
	}
	ps, err := m.Query(`select placeable_id id, building_type t, transform_x x, transform_y y, transform_z z, transform_yaw pitch, transform_pitch yaw, transform_roll roll, hologram g
		from building_blueprint_placeables where building_blueprint_id=? order by placeable_id`, bp)
	if err != nil {
		return err
	}
	for _, r := range ps {
		id, _ := toInt(r["id"])
		if fmt.Sprint(r["g"]) != "1" {
			return recycleMismatch("the hologram flag of a Solido placeable", 0, 1)
		}
		t, _ := r["t"].(string)
		x, _ := r["x"].(float64)
		y, _ := r["y"].(float64)
		z, _ := r["z"].(float64)
		yaw, _ := r["yaw"].(float64)
		pitch, _ := r["pitch"].(float64)
		roll, _ := r["roll"].(float64)
		got.Placeables = append(got.Placeables, solidoPlaceable{ID: id, Type: t, X: x, Y: y, Z: z, Yaw: yaw, Pitch: pitch, Roll: roll})
	}
	ss, err := m.Query(`select placeable_id id, scale_x x, scale_y y, scale_z z from building_blueprint_pentashields where building_blueprint_id=? order by placeable_id`, bp)
	if err != nil {
		return err
	}
	for _, r := range ss {
		id, _ := toInt(r["id"])
		x, _ := toInt(r["x"])
		y, _ := toInt(r["y"])
		z, _ := toInt(r["z"])
		got.Pentas = append(got.Pentas, solidoPenta{Placeable: id, X: x, Y: y, Z: z})
	}
	if got.digest() != sp.Digest {
		return apierr.New(apierr.ErrConflict, "The recycle was not written: the Solido's rows read back differently from the plan. Nothing was changed.")
	}
	return nil
}

// solidoWords is the line of the answer about the Solido.
func solidoWords(s map[string]any) string {
	where := "a blank Solido device that was already in your inventory"
	if s["itemCreated"] == true {
		where = "a new Solido device in your " + fmt.Sprint(s["itemWhere"])
	}
	w := fmt.Sprintf("Solido: made first from the base, in %s (item %v): %s building pieces, %s placeables (%s pentashields).",
		where, s["item"], commas(s["pieces"].(int64)), commas(s["placeables"].(int64)), commas(s["pentashields"].(int64)))
	var out []string
	if n := s["piecesLeftOut"].(int64); n > 0 {
		out = append(out, fmt.Sprintf("%d pieces the game leaves out", n))
	}
	if n := s["doorsLeftOut"].(int64); n > 0 {
		out = append(out, fmt.Sprintf("%d doors and gates", n))
	}
	out = append(out, "the claim totem")
	return w + " Left out, as the game leaves them: " + strings.Join(out, ", ") + "."
}

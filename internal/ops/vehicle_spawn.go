package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/vehicles"
)

// Create a vehicle from a template (#401): the console's Spawn Vehicle for a single-player save. The editor cannot ask the game to spawn one, so the vehicle is written as the rows
// a vehicle is made of in a save the game wrote, the same set Duplicate vehicle writes (and the game loaded a Duplicate's rows in 2026-10-27):
//
//	actors, vehicles                     the vehicle, with its health in its properties
//	permission_actor, ..._rank           the record (type 2) and the character's rank 1, which makes it the character's
//	fgl_entities, actor_fgl_entities     two entities: Actor (health, fuel and the other components) and VehicleInventory
//	vehicle_modules                      one row per module of the template, with the stats of a new module
//	inventories, actor_inventories       the cargo hold and the component holds, and the items put in the cargo hold
//
// What the template is (class, tier, the module of each slot) comes from the game's own tables (kit/gamedata/vehicles/vehicles.json). Every number that is not an id or a place follows
// from the modules by the rules of vehicles.Derive: health, critical health, a full tank, the cargo hold, and a new module's durability. The edit is one transaction. After
// the rows are written it checks that the foreign keys of the tables it touched hold as they did before, and works the numbers out again from the modules it wrote and compares
// them with what it wrote; a difference undoes the whole edit.
//
// Not proven in the game: a vehicle composed from the tables (rather than a copy of one the game wrote) has not been loaded yet.

// vehicleTables are the tables a vehicle's rows are in; their foreign keys are checked before and after.
var vehicleTables = []string{"actors", "vehicles", "permission_actor", "permission_actor_rank", "fgl_entities", "actor_fgl_entities", "vehicle_modules", "inventories", "actor_inventories", "items"}

// afterVehicleRows is called by a test after the rows are written and before they are checked.
var afterVehicleRows func(m *save.Mut, id int64) error

// errSpawnDryRun ends a preview's edit so that it is rolled back: the rows are the real ones, written by the same code, and nothing is kept.
var errSpawnDryRun = errors.New("dry run")

const (
	maxSpawnItems = 50
	maxCoord      = 5_000_000.0 // centimetres; the maps are a few hundred thousand across
	defaultSwatch = "Sardaukar Global"
)

var mapNameRe = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

// jf is a float that is always written with a decimal point ("750.0"), as the game writes them: a JSON number without one is an integer to the save's JSON format.
type jf float64

func (f jf) MarshalJSON() ([]byte, error) {
	v := float64(f)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, errors.New("not a finite number")
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.ContainsAny(s, ".") {
		s += ".0"
	}
	return []byte(s), nil
}

// floatsToJF returns v with every float64 in it (in maps and lists) as a jf.
func floatsToJF(v any) any {
	switch x := v.(type) {
	case float64:
		return jf(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = floatsToJF(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = floatsToJF(e)
		}
		return out
	}
	return v
}

func jsonText(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

// spawnSpec is a request that has passed its checks.
type spawnSpec struct {
	classKey string
	class    *vehicles.Class
	template vehicles.Template
	modules  map[string]string // slot -> module id, after the changes the request made to the template
	changed  []map[string]any  // what the request changed in the template: slot, from, to
	fuel     float64           // a fraction of a full tank
	place    *spawnPlace       // nil: next to the character
	lines    []giveLine
	notes    []string // things worth saying that are not refusals
}

type spawnPlace struct {
	mapName string
	x, y, z float64
}

// spawnSpecFrom reads and checks a request: args "class", "template", "uniques" ({slot: module id}; "" empties an optional slot), "fuel" (0 to 1, default 1), "place"
// ({map, x, y, z}; default next to the character) and "items" ([{template_id, quantity, quality}]).
func (o *Ops) spawnSpecFrom(a Args) (*spawnSpec, error) {
	key := a.Str("class")
	cls, ok := vehicles.ClassFor(key)
	if !ok {
		return nil, apierr.Newf(apierr.ErrBadArgument, "unknown vehicle class %q: pick one from the list", clip(key))
	}
	sp := &spawnSpec{classKey: key, class: cls, fuel: 1, modules: map[string]string{}}
	tname := a.Str("template")
	t, ok := cls.Template(tname)
	if !ok {
		return nil, apierr.Newf(apierr.ErrBadArgument, "%s has no template %q: pick one from the list", cls.Label, clip(tname))
	}
	sp.template = t
	for slot, id := range t.Modules {
		sp.modules[slot] = id
	}
	if raw, has := a["uniques"]; has && raw != nil {
		u, ok := raw.(map[string]any)
		if !ok {
			return nil, apierr.New(apierr.ErrBadArgument, "uniques must be an object of slot and module")
		}
		slots := make([]string, 0, len(u))
		for slot := range u {
			slots = append(slots, slot)
		}
		sort.Strings(slots)
		for _, slot := range slots {
			if !cls.HasSlot(slot) {
				return nil, apierr.Newf(apierr.ErrBadArgument, "%s has no slot %q", cls.Label, clip(slot))
			}
			want, _ := u[slot].(string)
			want = strings.TrimSpace(want)
			from := sp.modules[slot]
			if want == "" || want == "none" {
				for _, s := range cls.Slots {
					if s.Slot == slot && s.Mandatory {
						return nil, apierr.Newf(apierr.ErrBadArgument, "the %s slot of a %s cannot be left empty", slot, strings.ToLower(cls.Label))
					}
				}
				delete(sp.modules, slot)
				if from != "" {
					sp.changed = append(sp.changed, map[string]any{"slot": slot, "from": from, "to": ""})
				}
				continue
			}
			id, ok := vehicles.ResolveModule(sp.classKey, want)
			if !ok {
				return nil, apierr.Newf(apierr.ErrBadArgument, "%q is not a module of the %s in the game's table", clip(want), strings.ToLower(cls.Label))
			}
			m, _ := vehicles.ModuleFor(id)
			if m.Slot != slot {
				return nil, apierr.Newf(apierr.ErrBadArgument, "%s goes in the %s slot, not %s", id, m.Slot, slot)
			}
			if id != want {
				sp.notes = append(sp.notes, fmt.Sprintf("%s is the module row %s", want, id))
			}
			sp.modules[slot] = id
			if from != id {
				sp.changed = append(sp.changed, map[string]any{"slot": slot, "from": from, "to": id})
			}
		}
	}
	if _, has := a["fuel"]; has && a["fuel"] != nil {
		f, err := a.Float("fuel")
		if err != nil {
			return nil, err
		}
		if f < 0 || f > 1 {
			return nil, apierr.New(apierr.ErrBadArgument, "fuel is a fraction of a full tank, from 0 to 1")
		}
		sp.fuel = f
	}
	if raw, has := a["place"]; has && raw != nil {
		pm, ok := raw.(map[string]any)
		if !ok {
			return nil, apierr.New(apierr.ErrBadArgument, "place must be an object with map, x, y and z")
		}
		pa := Args(pm)
		pl := &spawnPlace{mapName: pa.Str("map")}
		if !mapNameRe.MatchString(pl.mapName) {
			return nil, apierr.New(apierr.ErrBadArgument, "place needs a map name (letters, digits and underscores)")
		}
		for _, c := range []struct {
			k string
			p *float64
		}{{"x", &pl.x}, {"y", &pl.y}, {"z", &pl.z}} {
			v, err := pa.Float(c.k)
			if err != nil {
				return nil, apierr.Newf(apierr.ErrBadArgument, "place: %v", err)
			}
			if math.Abs(v) > maxCoord {
				return nil, apierr.Newf(apierr.ErrBadArgument, "place: %s is out of range", c.k)
			}
			*c.p = v
		}
		sp.place = pl
	}
	if list, _ := a["items"].([]any); len(list) > 0 {
		if len(list) > maxSpawnItems {
			return nil, apierr.Newf(apierr.ErrBadArgument, "at most %d items at a time", maxSpawnItems)
		}
		for i, e := range list {
			m, ok := e.(map[string]any)
			if !ok {
				return nil, apierr.Newf(apierr.ErrBadArgument, "item %d is not an object", i+1)
			}
			_, tmpl, qty, q, err := o.giveArgs(Args(m))
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", i+1, err)
			}
			sp.lines = append(sp.lines, giveLine{tmpl: tmpl, qty: qty, q: q, grade: 1})
		}
	}
	// the slots of the class that must hold a module
	for _, s := range cls.Slots {
		if s.Mandatory && sp.modules[s.Slot] == "" {
			return nil, apierr.Newf(apierr.ErrBadArgument, "the %s slot needs a module", s.Slot)
		}
	}
	ids := sp.moduleIDs()
	d, err := vehicles.Derive(ids)
	if err != nil {
		return nil, apierr.New(apierr.ErrBadArgument, err.Error())
	}
	if len(sp.lines) > 0 && d.CargoSlots == 0 {
		return nil, apierr.Newf(apierr.ErrBadArgument, "this %s has no cargo hold, so items cannot be put in it. Give it a storage part, or remove the items", strings.ToLower(cls.Label))
	}
	// the game's templates always use the same hull for the same utility slots: say so when a request builds a combination none of them has
	if body := sp.modules["BodyHull"]; body != "" {
		for _, slot := range []string{"UtilitySlot1", "UtilitySlot2", "UtilitySlot3"} {
			if sp.modules[slot] != "" && cls.HasSlot(slot) && !vehicles.BodyUtilitySeen(cls, vehicles.ModuleBase(body), slot) {
				sp.notes = append(sp.notes, fmt.Sprintf("none of the game's templates puts a module in %s with the rear hull %s; the game's slot table allows it, but it has not been seen built", slot, body))
			}
		}
	}
	return sp, nil
}

// moduleIDs lists the modules of the spec in the order of the class's slots.
func (sp *spawnSpec) moduleIDs() []string {
	var out []string
	for _, s := range sp.class.Slots {
		if id := sp.modules[s.Slot]; id != "" {
			out = append(out, id)
		}
	}
	return out
}

func clip(s string) string {
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}

// moduleStats is the stats a new module has: its durability at the table's maximum, the swatch of a player's vehicle, and the weapon stats of a weapon-type module.
func moduleStats(m vehicles.Module) (string, error) {
	dur, err := jsonText(jf(m.Durability))
	if err != nil {
		return "", err
	}
	s := `{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":` + dur + `}],"FCustomizationStats":[[],{"SwatchId":"` + defaultSwatch + `"}]`
	if m.Weapon {
		s += `,"FVehicleWeaponModuleStats":[[],{"CharacterStateTag":{}}]`
	}
	return s + `}`, nil
}

func actorComponents(health, crit, fuel float64) (string, error) {
	h, err := jsonText(jf(health))
	if err != nil {
		return "", err
	}
	c, _ := jsonText(jf(crit))
	f, _ := jsonText(jf(fuel))
	return `{"FAggroControllerComponent":[0,{"m_TotalDamageDone":0.0}],` +
		`"FBiomeWeatherModifierComponent":[0,{"CurrentTemperatureModifier":1.0,"CurrentSandBuildupModifier":0.0,"m_CurrentSandColor":{"r":0.428689,"G":0.145263,"B":0.06859,"A":1.0}}],` +
		`"FHealthComponent":[0,{"m_CurrentHealth":` + h + `,"m_MaxDownButNotOutStateHealth":` + c + `,"m_CurrentDownButNotOutStateHealth":` + c + `}],` +
		`"FShelterComponent":[0,{"m_IsShelteredTraceResults":[true,true,true,true,true,true,true,true,true],"m_ShelteredPercentage":1.0}],` +
		`"FVehicleComponent":[0,{"CurrentFuel":` + f + `,"RemainingTimeUntilDespawn":0.0}]}`, nil
}

// kv and omap write a JSON object with its keys in the order given. The game writes a vehicle's properties in one order and reads them by key; the order is kept so that a
// created vehicle's rows read like the game's.
type kv struct {
	k string
	v any
}
type omap []kv

func (m omap) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, e := range m {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(e.k)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(floatsToJF(e.v))
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func sortedKV(m map[string]any) omap {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(omap, 0, len(keys))
	for _, k := range keys {
		out = append(out, kv{k, m[k]})
	}
	return out
}

func vehicleProperties(sp *spawnSpec, d vehicles.Derived, abandoned string) (string, error) {
	// the order the game writes them in: the class's own extras (the sandcrawler's spice fill) first, then the common ones
	own := append(sortedKV(sp.class.ExtraOwn), []kv{
		{"m_HarnessedVehicleId", "!!act#0"}, {"m_InitialFuel", 0.0}, {"m_InitialHealth", 0.0}, {"m_SpawnerName", ""}, {"m_CustomizationId", "None"},
		{"m_AbandonedTimestamp", abandoned}, {"m_bIsInSafezone", true},
	}...)
	props := omap{
		{sp.class.PropsKey, own},
		{"DamageableActorComponent", omap{{"m_CurrentMaxHealth", d.MaxHealth}, {"m_TotalMaxHealth", d.MaxHealth}}},
		{"EncounterActorComponent", omap{{"m_OwningEncounterUid", omap{{"EncounterUid", "None"}}}}},
	}
	props = append(props, sortedKV(sp.class.Extra)...)
	return jsonText(props)
}

// yawOnly turns the character's rotation into one that only turns about the vertical axis: the way the character faces, without a lean.
func yawOnly(x, y, z, w float64) (float64, float64, float64, float64) {
	if x == 0 && y == 0 && z == 0 && w == 0 {
		return 0, 0, 0, 1
	}
	yaw := math.Atan2(2*(w*z+x*y), 1-2*(y*y+z*z))
	if math.IsNaN(yaw) {
		return 0, 0, 0, 1
	}
	return 0, 0, math.Sin(yaw / 2), math.Cos(yaw / 2)
}

func num(v any) float64 {
	f, _ := toFloat(v)
	return f
}

// vehicleFKViolations counts the foreign-key violations of the tables a vehicle's rows are in, by table and parent.
func vehicleFKViolations(m *save.Mut) (map[string]int, error) {
	out := map[string]int{}
	for _, t := range vehicleTables {
		rows, err := m.Query(`select "table" t, parent p, fkid k from pragma_foreign_key_check(?)`, t)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[fmt.Sprintf("%v>%v/%v", r["t"], r["p"], r["k"])]++
		}
	}
	return out, nil
}

// SpawnVehicle creates a vehicle from a template (see the comment above) and answers with what it wrote. Args as spawnSpecFrom reads them.
func (o *Ops) SpawnVehicle(a Args) (any, error) { return o.spawnVehicle(a, false) }

// SpawnVehiclePreview does everything SpawnVehicle does, in the same edit and by the same code, and then undoes it: the answer holds the rows that would be written and nothing
// is kept. The answer's id is the id the vehicle would get now.
func (o *Ops) SpawnVehiclePreview(a Args) (any, error) { return o.spawnVehicle(a, true) }

// spawnAnswers remembers the answers of the last Creates that came with a requestId (#740). The page gives each form one requestId, so a second request for the same form
// (a double click, a retry after a lost answer) gets the first one's answer and writes nothing: one form makes one vehicle. A request with the same id and another body is a
// different request and is written. Only a Create that succeeded is remembered, so one that failed can be sent again. The ledger is kept in memory only.
type spawnLedger struct {
	sync.Mutex
	seen  map[spawnAsked]map[string]any
	order []spawnAsked
}

var spawnAnswers = spawnLedger{seen: map[spawnAsked]map[string]any{}}

type spawnAsked struct {
	s    *save.Save
	id   string
	body string
}

const maxSpawnAnswers = 16

// forget drops one remembered answer. The caller holds the lock.
func (l *spawnLedger) forget(key spawnAsked) {
	delete(l.seen, key)
	for i, k := range l.order {
		if k == key {
			l.order = append(l.order[:i], l.order[i+1:]...)
			break
		}
	}
}

// ForgetSpawnAnswers empties the ledger of a save. It is called when edits that were made are dropped (Discard, Restore), so that an id whose vehicle is gone is not answered as a repeat.
func ForgetSpawnAnswers(s *save.Save) {
	spawnAnswers.Lock()
	defer spawnAnswers.Unlock()
	for k := range spawnAnswers.seen {
		if k.s == s {
			spawnAnswers.forget(k)
		}
	}
}

// PruneSpawnAnswers drops the answers of a save whose own vehicle is not in the working copy, and keeps the others. It is for a reload that dropped only some of the edits (a
// Discard of a queue, a Reload now with a conflict): a Create that was put back, or written earlier, keeps its answer, so a retry after a lost answer still writes nothing (#760).
// "Its own vehicle" is the one the answer made (vehicleThere), not any vehicle with the same id: the game may have taken the id. An answer is dropped only when the working copy
// was read and the vehicle is not in it; a read that failed keeps it. While a queue is held the working copy is the game's file, which has none of the held Creates, so nothing
// is dropped then.
func PruneSpawnAnswers(s *save.Save) {
	if s.Held() != nil {
		return
	}
	spawnAnswers.Lock()
	defer spawnAnswers.Unlock()
	for k, prev := range spawnAnswers.seen {
		if k.s != s {
			continue
		}
		if there, read := vehicleThere(s, prev); read && !there {
			spawnAnswers.forget(k)
		}
	}
}

// SpawnAnswerCount is how many answers the repeat memory holds for a save (for tests and diagnostics).
func SpawnAnswerCount(s *save.Save) int {
	spawnAnswers.Lock()
	defer spawnAnswers.Unlock()
	n := 0
	for k := range spawnAnswers.seen {
		if k.s == s {
			n++
		}
	}
	return n
}

// dropSpawnAnswersNamingHeld drops every answer of a save that names a vehicle id. Ids are reused (the next one is the highest plus one), so once a vehicle has taken an id, an answer
// of another form that names it is stale: a retry of that form must write, not be answered with this vehicle. The caller holds the lock.
func dropSpawnAnswersNamingHeld(s *save.Save, id int64) {
	for k, v := range spawnAnswers.seen {
		if pid, ok := v["id"].(int64); ok && k.s == s && pid == id {
			spawnAnswers.forget(k)
		}
	}
}

// dropSpawnAnswersNaming is dropSpawnAnswersNamingHeld for a vehicle written outside the memory (a Create without an id, Duplicate vehicle).
func dropSpawnAnswersNaming(s *save.Save, id int64) {
	spawnAnswers.Lock()
	defer spawnAnswers.Unlock()
	dropSpawnAnswersNamingHeld(s, id)
}

// spawnIdentity is what makes a vehicle the one a Create made, apart from its id: the class of its actor and the entities linked to it, which the Create gave ids no entity of
// the save had. A vehicle the game made under the same id has other entities.
func spawnIdentity(prev map[string]any) (class string, entities string, ok bool) {
	rows, _ := prev["rows"].(map[string]any)
	acts, _ := rows["actors"].([]save.Row)
	ents, _ := rows["entities"].([]save.Row)
	if len(acts) != 1 || len(ents) == 0 {
		return "", "", false
	}
	class, _ = acts[0]["class"].(string)
	var parts []string
	for _, e := range ents {
		parts = append(parts, fmt.Sprintf("%v=%v", e["slot_name"], e["entity_id"]))
	}
	return class, strings.Join(parts, ","), class != ""
}

// vehicleThere says whether the vehicle an earlier answer made is a vehicle of the working copy (the same id, class and entities, not just the same id), and whether the working
// copy could be read to find out.
func vehicleThere(s *save.Save, prev map[string]any) (there, read bool) {
	id, ok := prev["id"].(int64)
	if !ok {
		return false, true
	}
	r, err := s.One(`select class from actors where id=? and id in (select id from vehicles)`, id)
	if err != nil {
		return false, false
	}
	if r == nil {
		return false, true
	}
	wantClass, wantEnts, ok := spawnIdentity(prev)
	if !ok {
		return true, true // an answer without its rows: the id is all there is to go by
	}
	if c, _ := r["class"].(string); c != wantClass {
		return false, true
	}
	ents, err := s.Query(`select slot_name, entity_id from actor_fgl_entities where actor_id=? order by slot_name`, id)
	if err != nil {
		return false, false
	}
	var parts []string
	for _, e := range ents {
		parts = append(parts, fmt.Sprintf("%v=%v", e["slot_name"], e["entity_id"]))
	}
	return strings.Join(parts, ",") == wantEnts, true
}

// spawnedStillThere says whether the vehicle an earlier answer made is still a vehicle of the working copy.
func (o *Ops) spawnedStillThere(prev map[string]any) bool {
	there, _ := vehicleThere(o.S, prev)
	return there
}

var requestIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func (o *Ops) spawnVehicle(a Args, dry bool) (any, error) {
	sp, err := o.spawnSpecFrom(a)
	if err != nil {
		return nil, err
	}
	if !dry {
		if rid, has := a["requestId"]; has && rid != nil {
			id, _ := rid.(string)
			if !requestIDRe.MatchString(id) {
				return nil, apierr.New(apierr.ErrBadArgument, "requestId must be 1 to 64 letters, digits, - or _")
			}
			rest := Args{}
			for k, v := range a {
				if k != "requestId" {
					rest[k] = v
				}
			}
			b, err := json.Marshal(rest)
			if err != nil {
				return nil, apierr.New(apierr.ErrBadArgument, "the request cannot be read")
			}
			key := spawnAsked{o.S, id, string(b)}
			// held through the edit in memory (not through the write of the file that follows it), so that a second request of the same form waits for the first edit and then gets its answer
			spawnAnswers.Lock()
			defer spawnAnswers.Unlock()
			prev, ok := spawnAnswers.seen[key]
			if ok && !o.spawnedStillThere(prev) {
				// the vehicle is not in the working copy (a discard, a restore or a rebase dropped the edit that made it): the id is a new request, not a repeat. The entry is not
				// dropped here but replaced when the write below succeeds: with the queue held the working copy is the game's file, the vehicle is still in the held edits, and a write
				// that is refused must not forget it (#760).
				ok = false
			}
			if ok {
				again := make(map[string]any, len(prev)+1)
				for k, v := range prev {
					again[k] = v
				}
				again["repeat"] = true
				return again, nil
			}
			res, err := o.spawnWrite(sp, dry)
			if err != nil {
				return nil, err
			}
			if m, ok := res.(map[string]any); ok {
				spawnAnswers.forget(key)
				// an answer of another form that names the id this vehicle took is stale (#760)
				if id, ok := m["id"].(int64); ok {
					dropSpawnAnswersNamingHeld(o.S, id)
				}
				spawnAnswers.seen[key] = m
				spawnAnswers.order = append(spawnAnswers.order, key)
				if len(spawnAnswers.order) > maxSpawnAnswers {
					delete(spawnAnswers.seen, spawnAnswers.order[0])
					spawnAnswers.order = spawnAnswers.order[1:]
				}
			}
			return res, nil
		}
	}
	res, err := o.spawnWrite(sp, dry)
	if m, ok := res.(map[string]any); err == nil && ok && !dry {
		if id, ok := m["id"].(int64); ok { // the lock is taken after the write, not during it
			dropSpawnAnswersNaming(o.S, id)
		}
	}
	return res, err
}

func (o *Ops) spawnWrite(sp *spawnSpec, dry bool) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	if p.Pawn == 0 {
		return nil, errors.New("this save has no character body to put the vehicle next to")
	}
	d, err := vehicles.Derive(sp.moduleIDs())
	if err != nil {
		return nil, err
	}
	var res map[string]any
	desc := fmt.Sprintf("create a %s (%s)", strings.ToLower(sp.class.Label), sp.template.Name)
	_, err = o.S.Mutate(desc, func(m *save.Mut) error {
		var e error
		if res, e = writeVehicle(m, p, sp, d); e != nil {
			return e
		}
		res["dryRun"] = dry
		m.Desc = fmt.Sprintf("created vehicle %v: a %s (%s), %d modules", res["id"], strings.ToLower(sp.class.Label), sp.template.Name, len(sp.moduleIDs()))
		if dry {
			return errSpawnDryRun
		}
		return nil
	})
	if dry && errors.Is(err, errSpawnDryRun) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

// writeVehicle writes the rows inside the caller's edit, checks them and answers with them.
func writeVehicle(m *save.Mut, p player, sp *spawnSpec, d vehicles.Derived) (map[string]any, error) {
	fkBefore, err := vehicleFKViolations(m)
	if err != nil {
		return nil, err
	}
	// the character's place: the controller's map and dimension, the body's position; "next to the character" is refused when the body is not where the player is (#654).
	// A place the person gave is theirs, so it goes on; it takes the character's dimension and facing.
	here, err := readCharacterPlace(m.One, p, true)
	if err != nil {
		return nil, err
	}
	if here.Body == nil {
		return nil, errors.New("player position unknown")
	}
	if here.Parked != nil && sp.place == nil {
		return nil, here.Parked
	}
	me, dim := here.Body, here.Dim
	// next to the character: the point 10 m ahead along the facing (800 along x when the facing is unknown), 150 up, when it is clear, else the nearest clear spot (#480); or where the request said
	mapName := here.Map
	size := sizeOfClassKey(sp.classKey)
	var placed placement
	var x, y, z float64
	var placeInfo map[string]any
	if sp.place == nil {
		if placed, err = choosePlace(m.Query, mapName, dim, poseOf(me), size, 0, sp.class.Label); err != nil {
			return nil, err
		}
		mapName, x, y, z = placed.Map, placed.X, placed.Y, placed.Z
		placeInfo = placed.Info()
	} else {
		mapName, x, y, z = sp.place.mapName, sp.place.x, sp.place.y, sp.place.z
		if w, err := loadPlaceWorld(m.Query, mapName, dim, x, y, 0); err != nil {
			return nil, err
		} else if note := w.placeNote(x, y, z, size); note != "" {
			sp.notes = append(sp.notes, note)
		}
		placeInfo = map[string]any{"map": mapName, "x": x, "y": y, "z": z, "words": "where you asked", "moved": false, "inBase": false, "blocked": []string{}, "claim": "", "distance": int64(0), "up": int64(0), "inAir": false}
	}
	rx, ry, rz, rw := yawOnly(num(me["rx"]), num(me["ry"]), num(me["rz"]), num(me["rw"]))

	abandoned := time.Now().UTC().Format(abandonedStamp)
	props, err := vehicleProperties(sp, d, abandoned)
	if err != nil {
		return nil, err
	}
	id, err := nextID(m, "actors")
	if err != nil {
		return nil, err
	}
	if _, err := m.Exec(`insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,dimension_index,gas_attributes,properties,owner_account_id,serial,state)
		values (?,?,?,?,?,?,?,?,?,?,?,jsonb('{}'),jsonb(?),NULL,0,0)`, id, sp.class.ActorClass, mapName, x, y, z, rx, ry, rz, rw, dim, props); err != nil {
		return nil, err
	}
	if _, err := m.Exec(`insert into vehicles(id) values (?)`, id); err != nil {
		return nil, err
	}
	if _, err := m.Exec(`insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (?,?,2,3,0,0)`, id, sp.class.PermissionName); err != nil {
		return nil, err
	}
	if _, err := m.Exec(`insert into permission_actor_rank(permission_actor_id,player_id,rank) values (?,?,1)`, id, p.Controller); err != nil {
		return nil, err
	}

	// entities
	actorEnt, invEnt, err := newVehicleEntityIDs(m, p)
	if err != nil {
		return nil, err
	}
	comps, err := actorComponents(d.MaxHealth, d.Critical, d.Fuel*sp.fuel)
	if err != nil {
		return nil, err
	}
	for _, e := range []struct {
		id   int64
		slot string
		json string
	}{{actorEnt, "Actor", comps}, {invEnt, "VehicleInventory", "{}"}} {
		if _, err := m.Exec(`insert into fgl_entities(entity_id,components) values (?,jsonb(?))`, e.id, e.json); err != nil {
			return nil, err
		}
		if _, err := m.Exec(`insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (?,?,?)`, id, e.id, e.slot); err != nil {
			return nil, err
		}
	}

	// modules
	next, err := nextID(m, "vehicle_modules")
	if err != nil {
		return nil, err
	}
	var modRows []map[string]any
	for i, mid := range sp.moduleIDs() {
		mod, _ := vehicles.ModuleFor(mid)
		stats, err := moduleStats(mod)
		if err != nil {
			return nil, err
		}
		if _, err := m.Exec(`insert into vehicle_modules(id,vehicle_id,template_id,stats) values (?,?,?,jsonb(?))`, next+int64(i), id, mid, stats); err != nil {
			return nil, err
		}
		modRows = append(modRows, map[string]any{"id": next + int64(i), "template_id": mid, "slot": mod.Slot, "name": catalog.ModuleName(mid), "durability": mod.Durability, "unique": mod.Unique, "stats": stats})
	}

	// inventories: the cargo hold first, then the component holds
	nextInv, err := nextID(m, "inventories")
	if err != nil {
		return nil, err
	}
	cargoID := nextInv
	if _, err := m.Exec(`insert into inventories(id,actor_id,item_id,vehicle_module_id,inventory_type,max_item_count,max_item_volume) values (?,?,NULL,NULL,0,?,?)`, cargoID, id, d.CargoSlots, d.CargoVolume); err != nil {
		return nil, err
	}
	if _, err := m.Exec(`insert into actor_inventories(inventory_id,component_name_hash) values (?,?)`, cargoID, sp.class.CargoHash); err != nil {
		return nil, err
	}
	for i, h := range sp.class.Holds {
		hid := cargoID + 1 + int64(i)
		if _, err := m.Exec(`insert into inventories(id,actor_id,item_id,vehicle_module_id,inventory_type,max_item_count,max_item_volume) values (?,?,NULL,NULL,NULL,NULL,NULL)`, hid, id); err != nil {
			return nil, err
		}
		if _, err := m.Exec(`insert into actor_inventories(inventory_id,component_name_hash) values (?,?)`, hid, h); err != nil {
			return nil, err
		}
	}

	// the items in the cargo hold, by the Give Items path (slots, stack sizes, volume)
	res := map[string]any{"ok": true, "id": id, "class": sp.classKey, "label": sp.class.Label, "template": sp.template.Name, "tier": sp.template.Tier,
		"changed": sp.changed, "notes": sp.notes, "build": vehicles.Build(), "complete": true,
		"place": placeInfo}
	placeInfo["nextToCharacter"] = sp.place == nil
	res["items"], res["requested"], res["given"], res["notGiven"], res["volumeNote"] = []map[string]any{}, int64(0), int64(0), []map[string]any{}, ""
	if len(sp.lines) > 0 {
		_, req, got, results, e := giveLinesInTx(m, cargoID, sp.lines)
		if e != nil {
			return nil, e
		}
		notGiven := []map[string]any{}
		for i, r := range results {
			r["name"] = itemLabel(sp.lines[i].tmpl)
			if short := sp.lines[i].qty - r["given"].(int64); short > 0 {
				notGiven = append(notGiven, map[string]any{"name": r["name"], "requested": r["requested"], "short": short, "reason": r["reason"], "limit": r["limit"]})
			}
			if n, _ := r["volumeNote"].(string); n != "" && res["volumeNote"] == "" {
				res["volumeNote"] = n
			}
		}
		res["items"], res["requested"], res["given"], res["notGiven"] = results, req, got, notGiven
		res["complete"] = got == req
	}

	if afterVehicleRows != nil { // a test changes a row here to prove the checks below notice
		if err := afterVehicleRows(m, id); err != nil {
			return nil, err
		}
	}
	// checks: the foreign keys of the tables touched hold as they did, and the numbers follow from the modules that were written
	fkAfter, err := vehicleFKViolations(m)
	if err != nil {
		return nil, err
	}
	for k, n := range fkAfter {
		if n > fkBefore[k] {
			return nil, fmt.Errorf("the vehicle's rows would break a foreign key (%s); nothing was changed", k)
		}
	}
	if err := verifyVehicle(m, id, sp, d); err != nil {
		return nil, fmt.Errorf("the vehicle's rows do not match what its modules give (%v); nothing was changed", err)
	}

	// the rows, as written
	rows, err := readVehicleRows(m, id)
	if err != nil {
		return nil, err
	}
	rows["vehicle_modules"] = modRows
	res["rows"] = rows
	res["derived"] = map[string]any{"maxHealth": d.MaxHealth, "critical": d.Critical, "fuel": d.Fuel * sp.fuel, "fuelCapacity": d.Fuel, "cargoSlots": d.CargoSlots, "cargoVolume": d.CargoVolume}
	res["counts"] = map[string]any{"actors": 1, "vehicles": 1, "permission_actor": 1, "permission_actor_rank": 1, "fgl_entities": 2, "actor_fgl_entities": 2,
		"vehicle_modules": len(modRows), "inventories": 1 + len(sp.class.Holds), "actor_inventories": 1 + len(sp.class.Holds)}
	return res, nil
}

// newVehicleEntityIDs gives the vehicle's two entities ids no entity of the save has. An entity id is a 32-bit prefix and a 32-bit counter; the vehicle takes the prefix of the
// newest vehicle that has entities (else the character's own) and counts from above everything that prefix already has, leaving room for the game, as Duplicate vehicle does.
func newVehicleEntityIDs(m *save.Mut, p player) (actorEnt, invEnt int64, err error) {
	all, err := m.Query(`select entity_id e from fgl_entities`)
	if err != nil {
		return 0, 0, err
	}
	taken := make(map[int64]bool, len(all))
	for _, r := range all {
		if e, ok := toInt(r["e"]); ok {
			taken[e] = true
		}
	}
	r, err := m.One(`select l.entity_id e from actor_fgl_entities l where l.slot_name='Actor' and l.actor_id=(
		select max(x.actor_id) from actor_fgl_entities x where x.slot_name='VehicleInventory' and x.actor_id in (select id from vehicles))`)
	if err != nil {
		return 0, 0, err
	}
	if r == nil {
		if r, err = m.One(`select entity_id e from actor_fgl_entities where actor_id=? order by entity_id limit 1`, p.Pawn); err != nil {
			return 0, 0, err
		}
	}
	base, ok := toInt(nil)
	if r != nil {
		base, ok = toInt(r["e"])
	}
	if !ok {
		return 0, 0, errors.New("this save has no entity to number the new vehicle's entities from (neither a vehicle's nor the character's)")
	}
	prefix := uint64(base) &^ 0xFFFFFFFF
	ids, err := newEntityIDs([]int64{int64(prefix), int64(prefix + 5)}, taken)
	if err != nil {
		return 0, 0, err
	}
	return ids[0], ids[1], nil
}

// verifyVehicle reads the rows back and works the numbers out again from the modules it finds there.
func verifyVehicle(m *save.Mut, id int64, sp *spawnSpec, want vehicles.Derived) error {
	mods, err := m.Query(`select template_id t from vehicle_modules where vehicle_id=? order by id`, id)
	if err != nil {
		return err
	}
	var ids []string
	for _, r := range mods {
		ids = append(ids, fmt.Sprint(r["t"]))
	}
	got, err := vehicles.Derive(ids)
	if err != nil {
		return err
	}
	if got != want || len(ids) != len(sp.moduleIDs()) {
		return fmt.Errorf("derived %+v from %d modules, expected %+v", got, len(ids), want)
	}
	chk, err := m.One(`select json_extract(a.properties,'$.DamageableActorComponent.m_TotalMaxHealth') th, json_extract(a.properties,'$.DamageableActorComponent.m_CurrentMaxHealth') ch,
		(select json_extract(f.components,'$.FHealthComponent[1].m_CurrentHealth') from actor_fgl_entities l join fgl_entities f on f.entity_id=l.entity_id where l.actor_id=a.id and l.slot_name='Actor') hp,
		(select json_extract(f.components,'$.FHealthComponent[1].m_MaxDownButNotOutStateHealth') from actor_fgl_entities l join fgl_entities f on f.entity_id=l.entity_id where l.actor_id=a.id and l.slot_name='Actor') crit,
		(select json_extract(f.components,'$.FVehicleComponent[1].CurrentFuel') from actor_fgl_entities l join fgl_entities f on f.entity_id=l.entity_id where l.actor_id=a.id and l.slot_name='Actor') fuel,
		(select max_item_count from inventories where actor_id=a.id and inventory_type=0) slots, (select max_item_volume from inventories where actor_id=a.id and inventory_type=0) vol,
		(select count(*) from inventories where actor_id=a.id) invs
		from actors a where a.id=?`, id)
	if err != nil || chk == nil {
		return fmt.Errorf("the vehicle cannot be read back: %v", err)
	}
	const eps = 1e-6
	same := func(a any, b float64) bool { f, ok := toFloat(a); return ok && math.Abs(f-b) < eps }
	switch {
	case !same(chk["th"], want.MaxHealth) || !same(chk["ch"], want.MaxHealth) || !same(chk["hp"], want.MaxHealth):
		return fmt.Errorf("health %v/%v/%v, expected %v", chk["th"], chk["ch"], chk["hp"], want.MaxHealth)
	case !same(chk["crit"], want.Critical):
		return fmt.Errorf("critical health %v, expected %v", chk["crit"], want.Critical)
	case !same(chk["fuel"], want.Fuel*sp.fuel):
		return fmt.Errorf("fuel %v, expected %v", chk["fuel"], want.Fuel*sp.fuel)
	case !same(chk["slots"], float64(want.CargoSlots)) || !same(chk["vol"], want.CargoVolume):
		return fmt.Errorf("cargo hold %v/%v, expected %d/%v", chk["slots"], chk["vol"], want.CargoSlots, want.CargoVolume)
	case chk["invs"] != int64(1+len(sp.class.Holds)):
		return fmt.Errorf("%v inventories, expected %d", chk["invs"], 1+len(sp.class.Holds))
	}
	return nil
}

// readVehicleRows reads the rows written for the vehicle (the modules are added by the caller, who has them in the order written).
func readVehicleRows(m *save.Mut, id int64) (map[string]any, error) {
	out := map[string]any{}
	for name, q := range map[string]string{
		"actors":                `select id, class, map, location_x x, location_y y, location_z z, rotation_x, rotation_y, rotation_z, rotation_w, dimension_index, json(properties) properties, serial, state from actors where id=?`,
		"permission_actor":      `select actor_id, actor_name, actor_type, access_level, is_child from permission_actor where actor_id=?`,
		"permission_actor_rank": `select permission_actor_id, player_id, rank from permission_actor_rank where permission_actor_id=?`,
		"entities":              `select l.entity_id, l.slot_name, json(f.components) components from actor_fgl_entities l join fgl_entities f on f.entity_id=l.entity_id where l.actor_id=? order by l.slot_name`,
		"inventories":           `select v.id, v.inventory_type, v.max_item_count, v.max_item_volume, h.component_name_hash from inventories v left join actor_inventories h on h.inventory_id=v.id where v.actor_id=? order by v.id`,
		"items":                 `select i.id, i.inventory_id, i.template_id, i.stack_size, i.position_index, i.quality_level from items i join inventories v on v.id=i.inventory_id where v.actor_id=? order by i.id`,
	} {
		rows, err := m.Query(q, id)
		if err != nil {
			return nil, err
		}
		out[name] = rows
	}
	return out, nil
}

// VehicleCatalog describes what can be built, for the page: the game build, and for each class its slots, its templates and, for each slot, the modules that fit it.
func (o *Ops) VehicleCatalog() (any, error) {
	var classes []map[string]any
	for _, key := range vehicles.ClassKeys() {
		c, _ := vehicles.ClassFor(key)
		slots := make([]map[string]any, 0, len(c.Slots))
		for _, s := range c.Slots {
			var opts []map[string]any
			for _, id := range vehicles.ModulesFor(key, s.Slot) {
				mod, _ := vehicles.ModuleFor(id)
				opts = append(opts, map[string]any{"id": id, "name": catalog.ModuleName(id), "grade": mod.Grade, "unique": mod.Unique, "cargoSlots": mod.CargoSlots, "cargoVolume": mod.CargoVolume, "weapon": mod.Weapon})
			}
			slots = append(slots, map[string]any{"slot": s.Slot, "mandatory": s.Mandatory, "parent": s.Parent, "modules": opts})
		}
		tpls := make([]map[string]any, 0, len(c.Templates))
		for _, t := range c.Templates {
			mods := make([]string, 0, len(t.Modules))
			for _, s := range c.Slots {
				if id := t.Modules[s.Slot]; id != "" {
					mods = append(mods, id)
				}
			}
			dv, _ := vehicles.Derive(mods)
			tpls = append(tpls, map[string]any{"name": t.Name, "tier": t.Tier, "modules": t.Modules, "maxHealth": dv.MaxHealth, "fuel": dv.Fuel, "cargoSlots": dv.CargoSlots, "cargoVolume": dv.CargoVolume})
		}
		classes = append(classes, map[string]any{"key": key, "label": c.Label, "slots": slots, "templates": tpls})
	}
	return map[string]any{"build": vehicles.Build(), "classes": classes}, nil
}

package ops

import (
	"strings"
)

// The Bases view of the console's player page: the list of bases and, for one base, its Power and Water tabs. A device belongs to a
// base when its owner entity is one of the base totem's entities, the same link the building pieces and containers use.
const ownedByBase = `owner_entity_id in (select entity_id from actor_fgl_entities where actor_id=?)`

// powerKinds are the base devices on the Power tab. Fuel is what a generator burns (Oil and spiced fuel cells; a full tank is one
// stack of 500, see generatorFuel); a windtrap takes filters, which the save does not name, so any item in it counts; a wind
// turbine takes lubricant, shown but not refilled because its refill is not verified in game.
var powerKinds = []struct {
	Kind, Name, Fuel string
	Full             int64
}{
	{"generator_placeable", "Generator", "Oil", 500},
	{"spicegenerator_placeable", "Spice Generator", "SpicedFuelCell", 500},
	{"windtrap_placeable", "Windtrap", "", 0},
	{"largewindtrap_placeable", "Large Windtrap", "", 0},
	{"windturbineomnidirectional_placeable", "Wind Turbine", "WindTurbineLubricant1", 0},
}

// waterNames are the names of the devices that hold water (the keys are waterDeviceCapacity's).
var waterNames = map[string]string{
	"watercistern_placeable":                 "Water Cistern",
	"mediumwatercistern_placeable":           "Medium Water Cistern",
	"largewatercistern_placeable":            "Large Water Cistern",
	"windtrap_placeable":                     "Windtrap",
	"largewindtrap_placeable":                "Large Windtrap",
	"bloodwaterextractor_placeable":          "Blood Water Extractor",
	"bloodwaterextractionadvanced_placeable": "Advanced Blood Water Extractor",
}

// totemTypes names the base by its claim structure.
var totemTypes = map[string]string{"totem_placeable": "Advanced Sub-Fief", "totem_small_placeable": "Sub-Fief"}

// BaseList is the table of the player's bases: id, name, type, owner, map, how many generators and windtraps, building pieces,
// placeables and where it is. In single-player there is one owner, the player.
func (o *Ops) BaseList() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	return o.baseListRows(p.Name)
}

func (o *Ops) baseListRows(owner string) ([]map[string]any, error) {
	totems, err := o.S.Query(`select t.id base_id, a.map, a.location_x x, a.location_y y, a.location_z z, coalesce(pa.actor_name,'') name, coalesce(tp.building_type,'') totem_type
		from totems t join actors a on a.id=t.id left join placeables tp on tp.id=t.id left join permission_actor pa on pa.actor_id=t.id order by t.id`)
	if err != nil {
		return nil, err
	}
	for _, b := range totems {
		id := b["base_id"]
		b["pieces"] = o.countWhere(`select count(*) n from building_instances where `+ownedByBase, id)
		b["placeables"] = o.countWhere(`select count(*) n from placeables where is_hologram=0 and `+ownedByBase, id)
		var marks []string
		args := []any{id}
		for _, k := range powerKinds[:4] {
			marks = append(marks, "?")
			args = append(args, k.Kind)
		}
		b["generators"] = o.countWhere(`select count(*) n from placeables where is_hologram=0 and `+ownedByBase+` and lower(building_type) in (`+strings.Join(marks, ",")+`)`, args...)
		tt, _ := b["totem_type"].(string)
		b["base_type"] = totemTypes[strings.ToLower(tt)]
		if b["base_type"] == "" {
			b["base_type"] = "Unknown"
		}
		name, _ := b["name"].(string)
		if name == "" || name == "None" || strings.HasPrefix(name, "##") {
			name = b["base_type"].(string)
		}
		b["name"] = name
		b["owner"] = owner
		delete(b, "totem_type")
	}
	return totems, nil
}

func (o *Ops) countWhere(q string, args ...any) any {
	r, _ := o.S.One(q, args...)
	return r["n"]
}

// BasePower is the Power tab of one base: for each kind of generator, windtrap and wind turbine, how many there are, what they
// hold (fuel, filters or lubricant) and how many hold nothing.
func (o *Ops) BasePower(baseID int64) (any, error) {
	byKind := map[string]map[string]any{}
	for _, k := range powerKinds {
		byKind[k.Kind] = map[string]any{"kind": k.Kind, "name": k.Name, "fuel": k.Fuel, "devices": int64(0), "queued": int64(0), "empty": int64(0), "capacity": int64(0)}
	}
	var marks []string
	args := []any{baseID}
	for _, k := range powerKinds {
		marks = append(marks, "?")
		args = append(args, k.Kind)
	}
	rows, err := o.S.Query(`select lower(p.building_type) kind, p.id, coalesce((select sum(i.stack_size) from items i join inventories v on v.id=i.inventory_id
			where v.actor_id=p.id and v.inventory_type=3),0) held
		from placeables p where p.is_hologram=0 and p.`+ownedByBase+` and lower(p.building_type) in (`+strings.Join(marks, ",")+`) order by p.id`, args...)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		t := byKind[r["kind"].(string)]
		held, _ := toInt(r["held"])
		t["devices"] = t["devices"].(int64) + 1
		t["queued"] = t["queued"].(int64) + held
		if held == 0 {
			t["empty"] = t["empty"].(int64) + 1
		}
	}
	types := []map[string]any{}
	for _, k := range powerKinds {
		t := byKind[k.Kind]
		if t["devices"].(int64) == 0 {
			continue
		}
		if k.Full > 0 {
			t["capacity"] = t["devices"].(int64) * k.Full
		}
		t["refillable"] = k.Full > 0
		types = append(types, t)
	}
	return map[string]any{"types": types}, nil
}

// BaseWater is the Water tab of one base: for each kind of water device, how many there are, the water stored and the capacity.
func (o *Ops) BaseWater(baseID int64) (any, error) {
	rows, err := o.S.Query(`select lower(p.building_type) kind, cast(json_extract(fe.components,'$.FWaterStorageComponent[1].m_WaterStored') as integer) stored
		from placeables p
		join actor_fgl_entities afe on afe.actor_id=p.id
		join fgl_entities fe on fe.entity_id=afe.entity_id
		where p.is_hologram=0 and p.`+ownedByBase+` and json_valid(fe.components,8) and json_type(fe.components,'$.FWaterStorageComponent') is not null
		order by p.id`, baseID)
	if err != nil {
		return nil, err
	}
	order := []string{}
	agg := map[string]map[string]any{}
	for _, r := range rows {
		kind := r["kind"].(string)
		t, ok := agg[kind]
		if !ok {
			name := waterNames[kind]
			if name == "" {
				name = kind
			}
			t = map[string]any{"kind": kind, "name": name, "devices": int64(0), "stored": int64(0), "capacity": int64(0), "known": waterDeviceCapacity[kind] > 0}
			agg[kind] = t
			order = append(order, kind)
		}
		t["devices"] = t["devices"].(int64) + 1
		stored, _ := toInt(r["stored"])
		t["stored"] = t["stored"].(int64) + stored
		t["capacity"] = t["capacity"].(int64) + waterDeviceCapacity[kind]
	}
	types := make([]map[string]any, 0, len(order))
	for _, k := range order {
		t := agg[k]
		if c := t["capacity"].(int64); c > 0 {
			t["fillPercent"] = float64(t["stored"].(int64)) * 100 / float64(c)
		}
		types = append(types, t)
	}
	return map[string]any{"types": types}, nil
}

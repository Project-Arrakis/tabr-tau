package ops

import "strings"

// vehicleType is the kind of vehicle from its blueprint name: "BP_Sandbike_CHOAM" is a "Sandbike".
func vehicleType(name string) string {
	n := strings.TrimPrefix(name, "BP_")
	if i := strings.Index(n, "_"); i > 0 {
		n = n[:i]
	}
	if n == "" {
		return name
	}
	return n
}

// vehicleDetails adds what the console's vehicle row and its expanded view show, all read from the save and never guessed:
//
//	condition  the lowest module condition in percent (current / its decayed maximum), nil when no module has a known maximum
//	fuel       the current fuel from the vehicle component, nil when there is none (the save stores no tank capacity)
//	components every module with its durability; percent only where the maximum is known
//	cargo      the cargo hold, the inventory with inventory_type 0 (a vehicle's other inventories are per-component holds with no
//	           capacity), with its items; nil when the vehicle has none
func (o *Ops) vehicleDetails(v map[string]any) {
	mods, _ := o.S.Query(`select id, template_id,
			json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') current,
			coalesce(json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability'), json_extract(stats,'$.FVehicleModuleDurabilityStats[1].MaxDurability')) max
		from vehicle_modules where vehicle_id=? order by id`, v["id"])
	var lowest *float64
	comps := make([]map[string]any, 0, len(mods))
	for _, m := range mods {
		c := map[string]any{"template_id": m["template_id"], "current": m["current"], "max": m["max"], "percent": nil}
		cur, ok1 := toFloat(m["current"])
		mx, ok2 := toFloat(m["max"])
		if ok1 && ok2 && mx > 0 {
			pct := cur / mx * 100
			if pct > 100 {
				pct = 100
			}
			c["percent"] = pct
			if lowest == nil || pct < *lowest {
				x := pct
				lowest = &x
			}
		}
		comps = append(comps, c)
	}
	v["components"] = comps
	v["condition"] = nil
	if lowest != nil {
		v["condition"] = *lowest
	}

	v["fuel"] = nil
	if f, _ := o.S.One(`select json_extract(f.components,'$.FVehicleComponent[1].CurrentFuel') fuel
		from actor_fgl_entities a join fgl_entities f on f.entity_id=a.entity_id
		where a.actor_id=? and json_valid(f.components,8) and json_type(f.components,'$.FVehicleComponent') is not null`, v["id"]); len(f) > 0 && f["fuel"] != nil {
		v["fuel"] = f["fuel"]
	}

	v["cargo"] = nil
	if hold, _ := o.S.One(`select id, max_item_count slots from inventories where actor_id=? and inventory_type=0 order by id limit 1`, v["id"]); len(hold) > 0 {
		items, _ := o.S.Query(`select id, template_id, stack_size, quality_level, position_index from items where inventory_id=? order by position_index`, hold["id"])
		v["cargo"] = map[string]any{"inventory_id": hold["id"], "slots": hold["slots"], "items": items}
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	}
	return 0, false
}

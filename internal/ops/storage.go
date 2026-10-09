package ops

import "strings"

// containerGroups are the building types that hold items at a base, in the Dune Docker console's four groups of the Bases >
// Inventory tab (console BASE_INVENTORY_TYPES, MIT), with the name the build menu shows. The lists are explicit on purpose, so
// generators, wind turbines, vehicles, the world's loot containers and terminals never show up as base inventory. A building type
// missing here is not listed (it fails closed) rather than guessed from its inventory type.
// SpiceSilo is the name every placed "Small Storage Container" still carries; both are listed so a rename cannot empty the list.
var containerGroups = []struct {
	ID, Label string
	Types     map[string]string
}{
	{"storage", "Storage", map[string]string{
		"storagecontainer_placeable":           "Storage Container",
		"mediumstoragecontainer_placeable":     "Medium Storage Container",
		"developer_storagecontainer_placeable": "Developer Storage Container",
		"genericcontainer_placeable":           "Chest",
		"spicesilo_placeable":                  "Small Storage Container",
		"smallstoragecontainer_placeable":      "Small Storage Container",
	}},
	{"refining", "Refining", map[string]string{
		"smallorerefinery_placeable":       "Small Ore Refinery",
		"mediumorerefinery_placeable":      "Medium Ore Refinery",
		"largeorerefinery_placeable":       "Large Ore Refinery",
		"smallchemicalrefinery_placeable":  "Small Chemical Refinery",
		"mediumchemicalrefinery_placeable": "Medium Chemical Refinery",
		"spicerefinery_placeable":          "Spice Refinery",
		"mediumspicerefinery_placeable":    "Medium Spice Refinery",
		"largespicerefinery_placeable":     "Large Spice Refinery",
	}},
	{"crafting", "Crafting", map[string]string{
		"fabricator_placeable":                  "Fabricator",
		"survivalfabricator_placeable":          "Survival Fabricator",
		"vehiclesfabricator_placeable":          "Vehicles Fabricator",
		"weaponsfabricator_placeable":           "Weapons Fabricator",
		"wearablesfabricator_placeable":         "Garment Fabricator",
		"advanced_survivalfabricator_placeable": "Advanced Survival Fabricator",
		"advanced_vehiclesfabricator_placeable": "Advanced Vehicles Fabricator",
		"advancedweaponsfabricator_placeable":   "Advanced Weapons Fabricator",
		"advancedwearablesfabricator_placeable": "Advanced Garment Fabricator",
	}},
	{"other", "Other", map[string]string{
		"recycler_placeable":      "Recycler",
		"repairstation_placeable": "Repair Station",
		"totem_small_placeable":   "Sub-Fief Console",
		"totem_placeable":         "Advanced Sub-Fief",
	}},
}

// storageTypes is the "Storage" group: plain storage containers (the Bases > Storage list and the map's storage markers).
var storageTypes = containerGroups[0].Types

// baseContainers lists the containers of the given groups that belong to a base: placeables of one of those building types whose
// owner entity is a totem's entity (the same link the building pieces use), not holograms, with a real capacity (the second,
// uncapped inventory every refinery and fabricator carries is skipped). baseID 0 means every base. One row per container
// inventory: the container's type label and group, the player's own name for it when there is one, the items it holds and its slot
// capacity.
func (o *Ops) baseContainers(baseID int64, groups ...string) ([]map[string]any, error) {
	var types []any
	var marks []string
	label := map[string]string{}
	group := map[string]string{}
	for _, g := range containerGroups {
		want := false
		for _, id := range groups {
			want = want || id == g.ID
		}
		if !want {
			continue
		}
		for t, l := range g.Types {
			types = append(types, t)
			marks = append(marks, "?")
			label[t], group[t] = l, g.ID
		}
	}
	args := append([]any{}, types...)
	scope := ""
	if baseID > 0 {
		scope = " and t.id=?"
		args = append(args, baseID)
	}
	rows, err := o.S.Query(`select inv.id inventory_id, p.id actor_id, t.id base_id, p.building_type, coalesce(pa.actor_name,'') custom_name,
			count(i.id) items, inv.max_item_count
		from placeables p
		join actor_fgl_entities afe on afe.entity_id=p.owner_entity_id
		join totems t on t.id=afe.actor_id
		join inventories inv on inv.actor_id=p.id and inv.max_item_count>=0
		left join items i on i.inventory_id=inv.id
		left join permission_actor pa on pa.actor_id=p.id
		where p.is_hologram=0 and lower(p.building_type) in (`+strings.Join(marks, ",")+`)`+scope+`
		group by inv.id order by t.id, p.id, inv.id`, args...)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		bt := strings.ToLower(r["building_type"].(string))
		r["type"] = label[bt]
		r["group"] = group[bt]
		name, _ := r["custom_name"].(string)
		if name == "" || name == "None" || strings.HasPrefix(name, "##") { // "##<building_type>" and "None" are the game's placeholders for a container never renamed
			name = label[bt]
		}
		r["name"] = name
		delete(r, "custom_name")
	}
	return rows, nil
}

// Storage lists the storage containers that belong to the player's base (the "Storage" group).
func (o *Ops) Storage() (any, error) { return o.baseContainers(0, "storage") }

// BaseInventory is the console's Bases > Inventory tab for one base: every container of the four groups (Storage, Refining,
// Crafting, Other) with its items and capacity, and the groups with their counts.
func (o *Ops) BaseInventory(baseID int64) (any, error) {
	rows, err := o.baseContainers(baseID, "storage", "refining", "crafting", "other")
	if err != nil {
		return nil, err
	}
	count := map[string]int{}
	for _, r := range rows {
		count[r["group"].(string)]++
	}
	groups := make([]map[string]any, 0, len(containerGroups))
	for _, g := range containerGroups {
		groups = append(groups, map[string]any{"id": g.ID, "label": g.Label, "count": count[g.ID]})
	}
	return map[string]any{"groups": groups, "containers": rows}, nil
}

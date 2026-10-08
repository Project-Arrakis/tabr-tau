package ops

import "strings"

// storageTypes are the building types that are plain storage containers, with the name the game's build menu shows. Mirrors the
// "Storage" group of the Dune Docker console (MIT): the list is explicit on purpose, so generators, refineries, fabricators, the
// totem, vehicles and the world's loot containers and terminals never appear as "storage". A building type missing here is not
// listed (it fails closed) rather than guessed from its inventory type.
// SpiceSilo is the name every placed "Small Storage Container" still carries; both are listed so a rename cannot empty the list.
var storageTypes = map[string]string{
	"storagecontainer_placeable":           "Storage Container",
	"mediumstoragecontainer_placeable":     "Medium Storage Container",
	"developer_storagecontainer_placeable": "Developer Storage Container",
	"genericcontainer_placeable":           "Chest",
	"spicesilo_placeable":                  "Small Storage Container",
	"smallstoragecontainer_placeable":      "Small Storage Container",
}

// Storage lists the storage containers that belong to the player's base: placeables of a storage type whose owner entity is a
// totem's entity (the same link the building pieces use). One row per container inventory, with the container's friendly type, the
// player's own name for it when there is one, the items it holds and its slot capacity. The inventory with a negative capacity
// (not a real container) is skipped, and so are holograms (build previews), as in the console.
func (o *Ops) Storage() (any, error) {
	types := make([]any, 0, len(storageTypes))
	marks := make([]string, 0, len(storageTypes))
	for t := range storageTypes {
		types = append(types, t)
		marks = append(marks, "?")
	}
	rows, err := o.S.Query(`select inv.id inventory_id, p.id actor_id, t.id base_id, p.building_type, coalesce(pa.actor_name,'') custom_name,
			count(i.id) items, inv.max_item_count
		from placeables p
		join actor_fgl_entities afe on afe.entity_id=p.owner_entity_id
		join totems t on t.id=afe.actor_id
		join inventories inv on inv.actor_id=p.id and inv.max_item_count>=0
		left join items i on i.inventory_id=inv.id
		left join permission_actor pa on pa.actor_id=p.id
		where p.is_hologram=0 and lower(p.building_type) in (`+strings.Join(marks, ",")+`)
		group by inv.id order by t.id, p.id, inv.id`, types...)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		bt, _ := r["building_type"].(string)
		r["type"] = storageTypes[strings.ToLower(bt)]
		name, _ := r["custom_name"].(string)
		if name == "" || name == "None" || strings.HasPrefix(name, "##") { // "##<building_type>" and "None" are the game's placeholders for a container never renamed
			name = r["type"].(string)
		}
		r["name"] = name
		delete(r, "custom_name")
	}
	return rows, nil
}

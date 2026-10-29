// Package consolefixture builds a small made-up file in the shape of the Dune console's base backup export (format "dune-base-backup", version 1), for the tests of internal/basefile and
// internal/ops. It is not a copy of anything: it has the same kinds, keys and kinds of value as a real export of console v1.4.52, with a handful of rows.
package consolefixture

import "encoding/json"

// Entry is one element of entries[].
type Entry = map[string]any

// Data is the data of an entry.
type Data = map[string]any

// The transfer ids of the fixture. Entry 1 is the owner placeholder.
const (
	Placeholder = 1
	Totem       = 2
	Building    = 3
	Chest       = 4
	Refinery    = 5
	EntTotem    = 6
	EntChest    = 7
	EntStation  = 8
	Backup      = 26
)

func act(id int, cls string, x, y, z, qz, qw float64, serial int, props Data) Entry {
	return Entry{"id": id, "kind": "act", "data": Data{
		"map": "HaggaBasin", "class": cls, "state": "BaseBackup", "serial": serial, "properties": props, "gas_attributes": Data{}, "dimension_index": 2, "owner_account_id": nil,
		"transform": Data{"location": Data{"x": x, "y": y, "z": z}, "rotation": Data{"x": 0, "y": 0, "z": qz, "w": qw}},
	}}
}

// Entries are the entries of the fixture: a totem, a building actor holding three pieces, a chest and a refinery with their entities, an item inventory each, three items, a
// permission record, a claim segment and the backup with its links.
func Entries() []Entry {
	e := []Entry{
		{"id": Placeholder, "kind": "act", "data": Data{}},
		act(Totem, "/Game/BP_Totem.BP_Totem_C", 5000, 6000, 7, 0, 1, 24, Data{"BP_Totem_C": Data{"m_SelfUniqueID": "!!act@2", "m_Other": "!!act@0"}}),
		act(Building, "/Game/BP_Building.BP_Building_C", 5000, 6000, 7, 0, 1, 25, Data{}),
		act(Chest, "/Game/BP_Chest.BP_Chest_C", 5010, 6000, 7, 0.7071, 0.7071, 26, Data{}),
		act(Refinery, "/Game/BP_Refinery.BP_Refinery_C", 5020, 6000, 7, 0, 1, 27, Data{}),
		{"id": EntTotem, "kind": "fgl", "data": Data{"actor_id": Totem, "slot_name": "Actor", "components": Data{"FHealthComponent": []any{0, Data{"m_CurrentHealth": 500.0}}}}},
		{"id": EntChest, "kind": "fgl", "data": Data{"actor_id": Chest, "slot_name": "ContainerInventory", "components": Data{"FPlaceableComponent": []any{0, Data{}}}}},
		{"id": EntStation, "kind": "fgl", "data": Data{"actor_id": Refinery, "slot_name": "ItemCraftingComponent", "components": Data{"ItemCraftingComponent": []any{[]any{}, Data{
			"Queue": []any{Data{"ItemUniqueId": "!!itm@14", "InstigatorActorId": "!!act#2", "Spare": "!!act#0"}}}}}}},
		{"id": 9, "kind": "PermissionActor", "data": Data{"actor_id": Chest, "is_child": true, "actor_name": "##GenericContainer_Placeable", "actor_type": 1, "access_level": 3, "edited_by_player_id": nil}},
		{"id": 10, "kind": "inv", "data": Data{"actor_id": Chest, "inventory_type": 4, "max_item_count": 20, "max_item_volume": 1000, "vehicle_module_id": nil}},
		{"id": 11, "kind": "inv", "data": Data{"actor_id": Refinery, "inventory_type": 12, "max_item_count": 5, "max_item_volume": 1000, "vehicle_module_id": nil}},
		{"id": 12, "kind": "itm", "data": Data{"stats": Data{}, "is_new": true, "stack_size": 3, "template_id": "CopperOre", "inventory_id": 10, "quality_level": 0, "position_index": 0, "volume_override": nil, "acquisition_time": 1790000000}},
		{"id": 13, "kind": "itm", "data": Data{"stats": Data{"FItemStackAndDurabilityStats": []any{[]any{}, Data{"DecayedMaxDurability": 0.0}}}, "is_new": false, "stack_size": 5, "template_id": "IronOre",
			"inventory_id": 10, "quality_level": 2, "position_index": 1, "volume_override": 0.25, "acquisition_time": 1790000001}},
		{"id": 14, "kind": "itm", "data": Data{"stats": Data{}, "is_new": true, "stack_size": 1, "template_id": "ScrapMetal", "inventory_id": 11, "quality_level": 0, "position_index": 0, "volume_override": nil, "acquisition_time": 1790000002}},
		{"id": 15, "kind": "ActorInventory", "data": Data{"inventory_id": 10, "component_name_hash": 1264785389}},
		{"id": 16, "kind": "ActorInventory", "data": Data{"inventory_id": 11, "component_name_hash": 222}},
		{"id": 17, "kind": "Building", "data": Data{"id": Building, "owner_id": nil}},
	}
	for i, p := range []struct {
		inst int
		typ  string
		t    []float64
	}{{1, "Foundation", []float64{5000, 6000, 7, 0, 0, 0, 1}}, {2, "Wall", []float64{5010, 6000.5, 7, 0, 0, 0.7071, 0.7071}}, {3, "Roof", []float64{5020, 6000, 10, 0, 0, 0, 1}}} {
		e = append(e, Entry{"id": 18 + i, "kind": "BuildingInstance", "data": Data{"health": 100.5, "shelter": 1, "transform": p.t, "building_id": Building, "instance_id": p.inst,
			"sand_buildup": -1, "building_type": p.typ, "building_flags": 0, "owner_entity_id": EntTotem, "last_placed_by_player_id": 0}})
	}
	for i, p := range []struct {
		id  int
		typ string
	}{{Totem, "Totem_Placeable"}, {Chest, "GenericContainer_Placeable"}, {Refinery, "SmallOreRefinery_Placeable"}} {
		e = append(e, Entry{"id": 21 + i, "kind": "Placeable", "data": Data{"id": p.id, "health": 500, "is_hologram": false, "building_type": p.typ, "has_hit_ground": false,
			"owner_entity_id": EntTotem, "has_buildable_support": true, "last_placed_by_player_id": 0}})
	}
	e = append(e,
		Entry{"id": 24, "kind": "Totem", "data": Data{"id": Totem, "last_backup_timestamp": 6636633, "landclaim_vertical_level": 0, "landclaim_original_global_location": []float64{5000, 6000, 7}, "landclaim_original_global_yaw_rotation": 125}},
		Entry{"id": 25, "kind": "LandclaimSegment", "data": Data{"totem_id": Totem, "grid_location_x": 1, "grid_location_y": -1}},
		Entry{"id": Backup, "kind": "BaseBackup", "data": Data{"player_id": Placeholder, "base_backup_name": "Test Reserve", "last_edited_by_player_id": 0}},
	)
	for i, a := range []int{Totem, Building, Chest, Refinery} {
		e = append(e, Entry{"id": 27 + i, "kind": "BaseBackupLinkedActor", "data": Data{"id": Backup, "actor_id": a}})
	}
	return e
}

// Header is the part of the file that is not entries.
func Header() Data {
	return Data{
		"game":    Data{"build": "2162777", "steamBuildId": "26081962", "latestPatches": []string{"DA-1_x"}, "patchesChecksum": "14557f952d24b402f2122b399b9cc102", "appliedPatchesCount": 932},
		"format":  "dune-base-backup",
		"source":  Data{"map": "HaggaBasin", "name": "Test Reserve", "counts": Data{"items": 3, "pieces": 3, "placeables": 3}, "rawName": "Test Reserve", "backupId": 2, "ownerName": "", "totemType": "Totem_Placeable"},
		"console": Data{"buildId": "0d27f4befd09ca28", "version": "v1.4.52"},
		"version": 1, "exportedAt": "2026-10-25T18:02:57.884Z", "ownerPlaceholderTransferId": Placeholder,
	}
}

// File is the fixture as text. edit may change the header and the entries (the entries are under the key "entries" as []Entry) before it is written.
func File(edit func(h Data, entries *[]Entry)) []byte {
	h, e := Header(), Entries()
	if edit != nil {
		edit(h, &e)
	}
	h["entries"] = e
	b, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}

// Find is the entry with the id, or nil.
func Find(entries []Entry, id int) Entry {
	for _, e := range entries {
		if e["id"] == id {
			return e
		}
	}
	return nil
}

// DataOf is the data of the entry with the id.
func DataOf(entries []Entry, id int) Data { return Find(entries, id)["data"].(Data) }

// Command vehiclegen makes internal/catalog/vehicles.json (#401) from dumps of the game's own vehicle tables. The file is the editor's catalog of vehicle templates and
// modules; this tool is how it is made again after a game update. It reads only JSON files, never the game: producing the dumps takes a reader of the game's content files,
// which this repository does not contain (see internal/catalog/PROVENANCE.md, "How vehicles.json is made").
//
// Inputs (all JSON, the shape a table dump has: a one-element array whose Rows object is keyed by row name):
//
//	-modules     CDT_BaseVehicleModules: one row per module id (m_StaticData, m_DurabilityStats)
//	-schematics  DT_VehicleSchematics: per vehicle, its slots (m_CraftingSchematic.m_VehicleSlots)
//	-templates   DT_VehicleTemplates: per vehicle, its actor class and the names of its templates
//	-decoded     the module lists of the templates: a reader of the table returns m_Modules as null, so the lists are read from the raw package and written as
//	             {"<Row>__slots": {"<TemplateName>": [["<slot type>", "<module id>"], ...]}}
//	-weapons     DT_VehicleWeaponModules: the module ids that are weapon-type modules
//
// Usage: go run ./tools/vehiclegen -modules ... -build 1.5.15.0 -read 2026-10-27 -out internal/catalog/vehicles.json
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
)

// shape is what a class's rows look like in a save, which the tables do not say; it was read from saves the game wrote (the same for every vehicle of the class).
type shape struct {
	key, model, templateRow, schematic, label, permission string
	holds                                                 []int64
	extra, extraOwn                                       map[string]any
}

const (
	cargoHash = -72424899 // the component hash of the cargo hold, the same on every class
	holdHash  = 462019337 // the component hash of the component hold, the same on every class
)

var shapes = []shape{
	{key: "Sandbike", model: "Sandbike", templateRow: "Sandbike", schematic: "OneManGroundcarChoam", label: "Sandbike", permission: "##OneManGroundcarChoam", holds: []int64{holdHash, -1011699082}},
	{key: "Buggy", model: "Buggy", templateRow: "Buggy", schematic: "BuggyCHOAM", label: "Buggy", permission: "##BuggyChoam", holds: []int64{holdHash}},
	{key: "OrnithopterLight", model: "LightOrnithopter", templateRow: "OrnithopterLight", schematic: "LightOrnithopterChoam", label: "Light ornithopter", permission: "##LightOrnithopterChoam", holds: []int64{holdHash},
		extra: map[string]any{"WeaponActorComponent": map[string]any{"m_FavoriteWeaponItemDatabaseId": "!!itm#0"}}},
	{key: "OrnithopterMedium", model: "MediumOrnithopter", templateRow: "OrnithopterMedium", schematic: "MediumOrnithopter", label: "Medium ornithopter", permission: "##MediumOrnithopter", holds: []int64{holdHash}},
	{key: "OrnithopterTransport", model: "TransportOrnithopter", templateRow: "OrnithopterTransport", schematic: "TranspOrnithopter", label: "Carrier ornithopter", permission: "##TranspOrnithopter", holds: []int64{holdHash}},
	{key: "Sandcrawler", model: "Sandcrawler", templateRow: "Sandcrawler", schematic: "Sandcrawler", label: "Sandcrawler", permission: "##Sandcrawler", holds: []int64{holdHash},
		extraOwn: map[string]any{"m_SpiceInventoryFillState": 0.0}},
	{key: "ContainerVehicle", model: "Container", templateRow: "ContainerVehicle", schematic: "ContainerVehicle", label: "Container vehicle", permission: "##ContainerVehicle", holds: []int64{holdHash}},
}

// maxGrade is the highest module grade offered. Grades 7 to 9 are rows of the module table but belong to no template and appear in no save.
const maxGrade = 6

type tableDump struct {
	Rows map[string]json.RawMessage `json:"Rows"`
}

func readRows(path string) (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d []tableDump
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(d) == 0 || d[0].Rows == nil {
		return nil, fmt.Errorf("%s: no Rows", path)
	}
	return d[0].Rows, nil
}

type moduleRow struct {
	Static struct {
		Type   string  `json:"VehicleModuleType"`
		Class  string  `json:"VehicleModuleClass"`
		Model  string  `json:"VehicleModel"`
		Slots  float64 `json:"NumInventorySlots"`
		Volume float64 `json:"m_MaxInventoryVolume"`
		Health float64 `json:"Health"`
		Crit   float64 `json:"CriticalStateHealth"`
		Fuel   float64 `json:"m_FuelCapacity"`
	} `json:"m_StaticData"`
	Durability struct {
		Max float64 `json:"MaxDurability"`
	} `json:"m_DurabilityStats"`
}

type schematicRow struct {
	Crafting struct {
		Slots []struct {
			Key   string `json:"Key"`
			Value struct {
				Mandatory bool   `json:"m_bIsMandatory"`
				Parent    string `json:"m_ParentSlot"`
			} `json:"Value"`
		} `json:"m_VehicleSlots"`
	} `json:"m_CraftingSchematic"`
}

type templateRow struct {
	ActorClass struct {
		Path string `json:"AssetPathName"`
	} `json:"m_VehicleActorClass"`
	Templates []struct {
		Key string `json:"Key"`
	} `json:"m_Templates"`
}

// Inputs are the paths of the dumps.
type Inputs struct {
	Modules, Schematics, Templates, Decoded, Weapons string
	Build, Read                                      string
}

func suffix(s, prefix string) string { return strings.TrimPrefix(s, prefix) }

var gradeRe = regexp.MustCompile(`_(\d+)$`)
var tierRe = regexp.MustCompile(`^T(\d+)`)

// Generate makes the catalog from the dumps.
func Generate(in Inputs) (*catalog.VehicleData, error) {
	modRows, err := readRows(in.Modules)
	if err != nil {
		return nil, err
	}
	schRows, err := readRows(in.Schematics)
	if err != nil {
		return nil, err
	}
	tplRows, err := readRows(in.Templates)
	if err != nil {
		return nil, err
	}
	wpnRows, err := readRows(in.Weapons)
	if err != nil {
		return nil, err
	}
	var decoded map[string]map[string]json.RawMessage
	b, err := os.ReadFile(in.Decoded)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &decoded); err != nil {
		return nil, fmt.Errorf("%s: %w", in.Decoded, err)
	}
	rows := map[string]moduleRow{}
	for id, raw := range modRows {
		var r moduleRow
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("module %s: %w", id, err)
		}
		rows[id] = r
	}
	out := &catalog.VehicleData{
		Build: in.Build, Read: in.Read,
		Tables:  []string{"CDT_BaseVehicleModules", "DT_VehicleSchematics", "DT_VehicleTemplates", "DT_VehicleWeaponModules"},
		Classes: map[string]*catalog.VehicleClass{}, Modules: map[string]catalog.VehicleModule{},
	}
	for _, sh := range shapes {
		c := &catalog.VehicleClass{Label: sh.label, PermissionName: sh.permission, CargoHash: cargoHash, Holds: sh.holds, Extra: sh.extra, ExtraOwn: sh.extraOwn}
		var tr templateRow
		raw, ok := tplRows[sh.templateRow]
		if !ok {
			return nil, fmt.Errorf("DT_VehicleTemplates has no row %s", sh.templateRow)
		}
		if err := json.Unmarshal(raw, &tr); err != nil {
			return nil, fmt.Errorf("template row %s: %w", sh.templateRow, err)
		}
		c.ActorClass = tr.ActorClass.Path
		i := strings.LastIndex(c.ActorClass, ".")
		if i < 0 {
			return nil, fmt.Errorf("%s: no actor class", sh.key)
		}
		c.PropsKey = c.ActorClass[i+1:]
		var sr schematicRow
		raw, ok = schRows[sh.schematic]
		if !ok {
			return nil, fmt.Errorf("DT_VehicleSchematics has no row %s", sh.schematic)
		}
		if err := json.Unmarshal(raw, &sr); err != nil {
			return nil, fmt.Errorf("schematic %s: %w", sh.schematic, err)
		}
		for _, s := range sr.Crafting.Slots {
			p := suffix(s.Value.Parent, "EVehicleModuleType::")
			if p == "Uninitialized" {
				p = ""
			}
			c.Slots = append(c.Slots, catalog.VehicleSlot{Slot: suffix(s.Key, "EVehicleModuleType::"), Mandatory: s.Value.Mandatory, Parent: p})
		}
		// the module rows of the model
		inTemplate := map[string]bool{}
		var lists map[string][][2]string
		if rawLists, ok := decoded[sh.templateRow+"__slots"]; ok {
			lists = map[string][][2]string{}
			for name, r := range rawLists {
				var l [][2]string
				if err := json.Unmarshal(r, &l); err != nil {
					return nil, fmt.Errorf("decoded %s/%s: %w", sh.templateRow, name, err)
				}
				lists[name] = l
			}
		}
		modelRows := []string{}
		for id, r := range rows {
			if suffix(r.Static.Model, "EVehicleModel::") == sh.model {
				modelRows = append(modelRows, id)
			}
		}
		sort.Strings(modelRows)
		for _, tp := range tr.Templates {
			l := lists[tp.Key]
			if len(l) == 0 && len(modelRows) == 1 {
				// the one template of a vehicle that is a single module (the container vehicle): the model has exactly one row
				r := rows[modelRows[0]]
				l = [][2]string{{suffix(r.Static.Type, "EVehicleModuleType::"), modelRows[0]}}
			}
			if len(l) == 0 {
				return nil, fmt.Errorf("%s/%s: the template has no decoded module list", sh.key, tp.Key)
			}
			t := catalog.VehicleTemplate{Name: tp.Key, Modules: map[string]string{}}
			if m := tierRe.FindStringSubmatch(tp.Key); m != nil {
				t.Tier, _ = strconv.Atoi(m[1])
			}
			for _, sm := range l {
				// The slot a module sits in is the module row's own slot type (a save keeps only the module id, so the game can only read the slot from the row). The key
				// the raw decode paired with the module is not used: for a few utility modules it names another utility slot than the row does.
				row, ok := rows[sm[1]]
				if !ok {
					return nil, fmt.Errorf("%s/%s: module %s is not in the module table", sh.key, tp.Key, sm[1])
				}
				slot := suffix(row.Static.Type, "EVehicleModuleType::")
				if _, dup := t.Modules[slot]; dup {
					return nil, fmt.Errorf("%s/%s: two modules for slot %s", sh.key, tp.Key, slot)
				}
				t.Modules[slot] = sm[1]
				inTemplate[sm[1]] = true
			}
			c.Templates = append(c.Templates, t)
		}
		for _, id := range modelRows {
			r := rows[id]
			g := 0
			if m := gradeRe.FindStringSubmatch(id); m != nil {
				g, _ = strconv.Atoi(m[1])
			} else if sh.key != "ContainerVehicle" && !inTemplate[id] {
				continue // a row with no grade is a faction's own module (an NPC vehicle's), not one of the player's grades
			}
			if g > maxGrade && !inTemplate[id] {
				continue
			}
			if strings.Contains(strings.ToLower(id), "polarcap") && !inTemplate[id] {
				continue // placeholder rows
			}
			kind := suffix(r.Static.Class, "EVehicleModuleClass::")
			m := catalog.VehicleModule{
				Class: sh.key, Slot: suffix(r.Static.Type, "EVehicleModuleType::"), Kind: kind, Grade: g,
				Unique: strings.Contains(id, "_Unique_"), Durability: r.Durability.Max,
			}
			switch kind {
			case "Chassis":
				m.Health, m.Critical, m.Fuel = r.Static.Health, r.Static.Crit, r.Static.Fuel
			case "Body", "Cockpit":
				m.Health = r.Static.Health
			}
			if r.Static.Slots > 0 {
				m.CargoSlots, m.CargoVolume = int64(r.Static.Slots), r.Static.Volume
			}
			if _, w := wpnRows[id]; w {
				m.Weapon = true
			}
			out.Modules[id] = m
		}
		out.Classes[sh.key] = c
	}
	if err := catalog.CheckVehicles(out); err != nil {
		return nil, errors.New("the generated catalog is not consistent: " + err.Error())
	}
	return out, nil
}

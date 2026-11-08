// Package vehicles is the game's vehicle tables for the vehicles that appear in saves: for each of the seven classes (Sandbike, Buggy, Light, Medium and Carrier ornithopter,
// Sandcrawler, Container vehicle) the class's actor path and the shape of its rows in a save, its slots, the game's named templates (tier, and the module id of each slot),
// and one row per module the game's module table lists for the class (slot, kind, grade, whether it is a unique, the durability a new module has, health, critical health,
// fuel capacity, cargo slots and volume, and whether it is a weapon-type module).
//
// The rows are read from the installed game's content files (CDT_BaseVehicleModules, DT_VehicleSchematics, DT_VehicleTemplates, DT_VehicleWeaponModules) and kept as a snapshot
// in vehicles.json: the game's identifiers and numbers only, no names, text or images. The snapshot names the game build it was read from. A new game version is a new
// snapshot; a row it adds or changes shows up in the diff and in the tests. The Tank and the Treadwheel are in the game's tables but in no save, and are not in the file.
//
// The package says what the game's tables say and derives from module ids what a vehicle's rows hold (Derive). Which tank a program trusts for a vehicle whose modules it can
// not read, or that lists two chassis, is the program's decision; ChassisFuel and ChassisTanks only give the numbers of the Chassis rows.
package vehicles

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed vehicles.json
var snapshot []byte

// Data is the content of vehicles.json.
type Data struct {
	Build   string            `json:"build"`   // the game build the tables were read from
	Read    string            `json:"read"`    // the date they were read
	Tables  []string          `json:"tables"`  // the game tables the file was made from
	Classes map[string]*Class `json:"classes"` // by class key: Sandbike, Buggy, OrnithopterLight, ...
	Modules map[string]Module `json:"modules"` // by module id (what vehicle_modules.template_id holds)
}

// Class is one kind of vehicle.
type Class struct {
	Label          string         `json:"label"`              // a plain name for the class
	ActorClass     string         `json:"actorClass"`         // actors.class
	PropsKey       string         `json:"propsKey"`           // the key of the class's own object in actors.properties; the blueprint name, "BP_Buggy_CHOAM_C"
	PermissionName string         `json:"permissionName"`     // permission_actor.actor_name
	Extra          map[string]any `json:"extra,omitempty"`    // properties the game writes for the class besides the common ones (top-level keys)
	ExtraOwn       map[string]any `json:"extraOwn,omitempty"` // properties inside the class's own object
	CargoHash      int64          `json:"cargoHash"`          // actor_inventories.component_name_hash of the cargo hold
	Holds          []int64        `json:"holds"`              // the hashes of the component holds
	Slots          []Slot         `json:"slots"`
	Templates      []Template     `json:"templates"`
}

// Slot is a place for a module.
type Slot struct {
	Slot      string `json:"slot"` // the game's slot type, EVehicleModuleType without its prefix: Chassis, Engine, UtilitySlot1, ...
	Mandatory bool   `json:"mandatory"`
	Parent    string `json:"parent,omitempty"`
}

// Template is a named template of the game: a tier of a class with a module for each of its slots.
type Template struct {
	Name    string            `json:"name"` // T6_Combat
	Tier    int               `json:"tier"` // the grade of its chassis (the Mk number)
	Modules map[string]string `json:"modules"`
}

// Module is one module row of the game's module table.
type Module struct {
	Class       string  `json:"class"` // the class key it belongs to
	Slot        string  `json:"slot"`
	Kind        string  `json:"kind"` // EVehicleModuleClass: Chassis, Body, Cockpit, Engine, Generator, Locomotion, Utility
	Grade       int     `json:"grade"`
	Unique      bool    `json:"unique,omitempty"`
	Durability  float64 `json:"durability"` // m_DurabilityStats.MaxDurability: what a new module has
	Health      float64 `json:"health,omitempty"`
	Critical    float64 `json:"critical,omitempty"`
	Fuel        float64 `json:"fuel,omitempty"`
	CargoSlots  int64   `json:"cargoSlots,omitempty"`
	CargoVolume float64 `json:"cargoVolume,omitempty"`
	Weapon      bool    `json:"weapon,omitempty"` // the game lists it in its weapon-module table: its saved stats carry FVehicleWeaponModuleStats
}

var data *Data

func init() {
	d, err := Parse(snapshot)
	if err != nil {
		panic("vehicles: vehicles.json: " + err.Error())
	}
	data = d
}

// Parse reads the text of a snapshot. It refuses unknown fields and a catalog that Check refuses.
func Parse(b []byte) (*Data, error) {
	var d Data
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, err
	}
	if err := Check(&d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Marshal makes the text of a snapshot: one space of indent and a final newline, maps in key order, so that the same catalog always gives the same bytes and a refreshed
// snapshot shows only what changed in a diff. The catalog is not checked; Parse checks it.
func Marshal(d *Data) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", " ")
	if err := enc.Encode(d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Build is the game build the vehicle tables were read from.
func Build() string { return data.Build }

// Read is the day the tables were read (YYYY-MM-DD).
func Read() string { return data.Read }

// ClassOrder is the order the classes are listed in.
var ClassOrder = []string{"Sandbike", "Buggy", "OrnithopterLight", "OrnithopterMedium", "OrnithopterTransport", "Sandcrawler", "ContainerVehicle"}

// ClassKeys lists the classes in the file, in the order of ClassOrder.
func ClassKeys() []string {
	var out []string
	for _, k := range ClassOrder {
		if _, ok := data.Classes[k]; ok {
			out = append(out, k)
		}
	}
	return out
}

// ClassFor returns a class by key. The value must not be changed.
func ClassFor(key string) (*Class, bool) {
	c, ok := data.Classes[key]
	return c, ok
}

// ModuleFor returns the module row with that id.
func ModuleFor(id string) (Module, bool) {
	m, ok := data.Modules[id]
	return m, ok
}

// ChassisFuel is the tank of a chassis module: the fuel capacity of the Chassis row with that id, when the row belongs to the vehicle with that blueprint name ("BP_Buggy_CHOAM_C",
// the class's PropsKey). It is false for an id that is not a row, a row that is not a chassis, a chassis with no tank (the Container vehicle's) and a chassis of another vehicle.
func ChassisFuel(blueprint, moduleID string) (float64, bool) {
	m, ok := data.Modules[moduleID]
	if !ok || m.Kind != "Chassis" || !(m.Fuel > 0) {
		return 0, false
	}
	if c, ok := data.Classes[m.Class]; !ok || c.PropsKey != blueprint {
		return 0, false
	}
	return m.Fuel, true
}

// ChassisTanks is every Chassis row that has a tank, by the blueprint name of its vehicle and then by module id. The caller gets a copy. A vehicle with no such row (the
// Container vehicle) is not a key.
func ChassisTanks() map[string]map[string]float64 {
	out := map[string]map[string]float64{}
	for id, m := range data.Modules {
		if m.Kind != "Chassis" || !(m.Fuel > 0) {
			continue
		}
		c, ok := data.Classes[m.Class]
		if !ok {
			continue
		}
		if out[c.PropsKey] == nil {
			out[c.PropsKey] = map[string]float64{}
		}
		out[c.PropsKey][id] = m.Fuel
	}
	return out
}

// Template returns the class's template with that name.
func (c *Class) Template(name string) (Template, bool) {
	for _, t := range c.Templates {
		if t.Name == name {
			return t, true
		}
	}
	return Template{}, false
}

// HasSlot says whether the class has a slot of that type.
func (c *Class) HasSlot(slot string) bool {
	for _, s := range c.Slots {
		if s.Slot == slot {
			return true
		}
	}
	return false
}

// ModulesFor lists the ids of the modules of a class that fit a slot (the same slot type of the same model: the stock module of any grade and the uniques), by grade then id.
func ModulesFor(class, slot string) []string {
	var out []string
	for id, m := range data.Modules {
		if m.Class == class && m.Slot == slot {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := data.Modules[out[i]], data.Modules[out[j]]
		if a.Grade != b.Grade {
			return a.Grade < b.Grade
		}
		return out[i] < out[j]
	})
	return out
}

var leadingZeroGrade = regexp.MustCompile(`_0(\d)$`)
var plainGrade = regexp.MustCompile(`_(\d)$`)

// ResolveModule finds the module row a name stands for in a class. The game spells the grade of some rows with a leading zero (BuggyEngine_Unique_Accelerate_06) and some
// without (BuggyInventory_Unique_Capacity_6), and the item that installs a module is spelled otherwise than its row (item BuggyInventory_Unique_Capacity_06, row
// BuggyInventory_Unique_Capacity_6), so an id that is not a row is tried with the other spelling. It returns the row's id.
func ResolveModule(class, id string) (string, bool) {
	try := func(s string) bool {
		m, ok := data.Modules[s]
		return ok && m.Class == class
	}
	if try(id) {
		return id, true
	}
	if alt := leadingZeroGrade.ReplaceAllString(id, "_$1"); alt != id && try(alt) {
		return alt, true
	}
	if alt := plainGrade.ReplaceAllString(id, "_0$1"); alt != id && try(alt) {
		return alt, true
	}
	return "", false
}

// BodyUtilitySeen says whether any template of the class puts a module in the utility slot while the body hull is the given kind of module (the module id without its grade,
// BuggyHullBackExtra for BuggyHullBackExtra_6). The game's slot table does not tie a utility slot to a hull, but its templates always use the same hulls for the same
// utility slots; a combination no template has is one the game has not been seen to build.
func BodyUtilitySeen(class *Class, bodyBase, utilitySlot string) bool {
	for _, t := range class.Templates {
		if base := ModuleBase(t.Modules["BodyHull"]); base == bodyBase && t.Modules[utilitySlot] != "" {
			return true
		}
	}
	return false
}

var gradeSuffix = regexp.MustCompile(`_\d+$`)

// ModuleBase is a module id without its grade (BuggyHullBackExtra_6 -> BuggyHullBackExtra).
func ModuleBase(id string) string { return gradeSuffix.ReplaceAllString(id, "") }

// Derived is what the rows of a vehicle hold that follows from its modules.
type Derived struct {
	MaxHealth   float64 // the health of an undamaged vehicle: the sum of Health over the Chassis, Body and Cockpit modules
	Critical    float64 // the health at which the vehicle goes down but not out: the chassis's CriticalStateHealth
	Fuel        float64 // a full tank: the chassis's fuel capacity
	CargoSlots  int64   // the cargo hold's slots: the sum over the modules that provide some
	CargoVolume float64 // and its volume
}

// Derive works out the numbers a vehicle's rows hold from the ids of its modules. It is the one rule to use both to write them and to check what was written.
func Derive(moduleIDs []string) (Derived, error) {
	var d Derived
	chassis := 0
	for _, id := range moduleIDs {
		m, ok := data.Modules[id]
		if !ok {
			return d, fmt.Errorf("module %q is not in the vehicle catalog", id)
		}
		switch m.Kind {
		case "Chassis":
			chassis++
			d.Critical, d.Fuel = m.Critical, m.Fuel
			d.MaxHealth += m.Health
		case "Body", "Cockpit":
			d.MaxHealth += m.Health
		}
		if m.CargoSlots > 0 {
			d.CargoSlots += m.CargoSlots
			d.CargoVolume += m.CargoVolume
		}
	}
	if chassis != 1 {
		return d, fmt.Errorf("a vehicle needs exactly one chassis (this has %d)", chassis)
	}
	return d, nil
}

// Check verifies a vehicle catalog: every template of every class names one existing module of the class for each slot it fills, in the slot of the module, with
// every mandatory slot filled; the tier of a template is its chassis's grade; every module belongs to a class of the file and a slot the class has. Parse runs it, and
// so should a tool that makes a snapshot before it writes the file.
func Check(d *Data) error {
	if d.Build == "" || len(d.Tables) == 0 {
		return errors.New("the file names no game build or tables")
	}
	var problems []string
	add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
	for key, c := range d.Classes {
		if c.ActorClass == "" || c.PropsKey == "" || c.PermissionName == "" || c.CargoHash == 0 || len(c.Holds) == 0 {
			add("class %s: its row shape is incomplete", key)
		}
		slots := map[string]Slot{}
		for _, s := range c.Slots {
			if _, dup := slots[s.Slot]; dup {
				add("class %s: slot %s twice", key, s.Slot)
			}
			slots[s.Slot] = s
		}
		if len(c.Templates) == 0 {
			add("class %s: no template", key)
		}
		for _, t := range c.Templates {
			chassis := 0
			for slot, id := range t.Modules {
				m, ok := d.Modules[id]
				switch {
				case !ok:
					add("%s/%s: module %s is not in the module table", key, t.Name, id)
				case m.Class != key:
					add("%s/%s: module %s belongs to %s", key, t.Name, id, m.Class)
				case m.Slot != slot:
					add("%s/%s: module %s is for slot %s, not %s", key, t.Name, id, m.Slot, slot)
				case m.Kind == "Chassis":
					chassis++
					if m.Grade != t.Tier && key != "ContainerVehicle" {
						add("%s/%s: tier %d but the chassis is grade %d", key, t.Name, t.Tier, m.Grade)
					}
				}
				if _, ok := slots[slot]; !ok {
					add("%s/%s: slot %s is not a slot of the class", key, t.Name, slot)
				}
			}
			if chassis != 1 {
				add("%s/%s: %d chassis", key, t.Name, chassis)
			}
			for _, s := range c.Slots {
				if s.Mandatory && t.Modules[s.Slot] == "" {
					add("%s/%s: mandatory slot %s is empty", key, t.Name, s.Slot)
				}
			}
		}
	}
	for id, m := range d.Modules {
		c, ok := d.Classes[m.Class]
		if !ok {
			add("module %s: class %s is not in the file", id, m.Class)
			continue
		}
		found := false
		for _, s := range c.Slots {
			if s.Slot == m.Slot {
				found = true
			}
		}
		if !found {
			add("module %s: slot %s is not a slot of %s", id, m.Slot, m.Class)
		}
		if m.Durability <= 0 {
			add("module %s: no durability", id)
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		if len(problems) > 12 {
			problems = append(problems[:12], fmt.Sprintf("and %d more", len(problems)-12))
		}
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

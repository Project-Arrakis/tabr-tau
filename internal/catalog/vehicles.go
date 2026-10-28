package catalog

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// vehicles.json is what the editor needs to build a vehicle from a template (#401): for the seven vehicle classes that appear in saves, the class's actor path and the
// shape of its rows, its slots, the game's named templates (tier -> slot -> module id) and one row per module the game's module table lists for the class (slot, kind,
// grade, whether it is a unique, the durability a new module has, health, critical health, fuel capacity, cargo slots and volume, and whether it is a weapon-type module).
// It holds the game's identifiers and numbers only (no names, text or images). The game build the tables were read from is in the file; tools/vehiclegen makes the file
// from dumps of the game's tables, and internal/catalog/PROVENANCE.md says where each input comes from. Tank and Treadwheel are not in the file: they have never appeared
// in a save.
//
//go:embed vehicles.json
var rawVehicles []byte

// VehicleData is the content of vehicles.json.
type VehicleData struct {
	Build   string                   `json:"build"`   // the game build the tables were read from
	Read    string                   `json:"read"`    // the date they were read
	Tables  []string                 `json:"tables"`  // the game tables the file was made from
	Classes map[string]*VehicleClass `json:"classes"` // by class key: Sandbike, Buggy, OrnithopterLight, ...
	Modules map[string]VehicleModule `json:"modules"` // by module id (what vehicle_modules.template_id holds)
}

// VehicleClass is one kind of vehicle.
type VehicleClass struct {
	Label          string            `json:"label"`              // the editor's own name for the class
	ActorClass     string            `json:"actorClass"`         // actors.class
	PropsKey       string            `json:"propsKey"`           // the key of the class's own object in actors.properties
	PermissionName string            `json:"permissionName"`     // permission_actor.actor_name
	Extra          map[string]any    `json:"extra,omitempty"`    // properties the game writes for the class besides the common ones (top-level keys)
	ExtraOwn       map[string]any    `json:"extraOwn,omitempty"` // properties inside the class's own object
	CargoHash      int64             `json:"cargoHash"`          // actor_inventories.component_name_hash of the cargo hold
	Holds          []int64           `json:"holds"`              // the hashes of the component holds
	Slots          []VehicleSlot     `json:"slots"`
	Templates      []VehicleTemplate `json:"templates"`
}

// VehicleSlot is a place for a module.
type VehicleSlot struct {
	Slot      string `json:"slot"` // the game's slot type, EVehicleModuleType without its prefix: Chassis, Engine, UtilitySlot1, ...
	Mandatory bool   `json:"mandatory"`
	Parent    string `json:"parent,omitempty"`
}

// VehicleTemplate is a named template of the game: a tier of a class with a module for each of its slots.
type VehicleTemplate struct {
	Name    string            `json:"name"` // T6_Combat
	Tier    int               `json:"tier"` // the grade of its chassis (the Mk number)
	Modules map[string]string `json:"modules"`
}

// VehicleModule is one module row of the game's module table.
type VehicleModule struct {
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

var vehicleData VehicleData

func init() {
	if err := json.Unmarshal(rawVehicles, &vehicleData); err != nil {
		panic("catalog: vehicles.json is not valid: " + err.Error())
	}
}

// VehicleBuild is the game build the vehicle tables were read from.
func VehicleBuild() string { return vehicleData.Build }

// VehicleClassOrder is the order the classes are listed in.
var VehicleClassOrder = []string{"Sandbike", "Buggy", "OrnithopterLight", "OrnithopterMedium", "OrnithopterTransport", "Sandcrawler", "ContainerVehicle"}

// VehicleClassKeys lists the classes in the file, in the order of VehicleClassOrder.
func VehicleClassKeys() []string {
	var out []string
	for _, k := range VehicleClassOrder {
		if _, ok := vehicleData.Classes[k]; ok {
			out = append(out, k)
		}
	}
	return out
}

// VehicleClassFor returns a class by key. The value must not be changed.
func VehicleClassFor(key string) (*VehicleClass, bool) {
	c, ok := vehicleData.Classes[key]
	return c, ok
}

// VehicleModuleFor returns the module row with that id.
func VehicleModuleFor(id string) (VehicleModule, bool) {
	m, ok := vehicleData.Modules[id]
	return m, ok
}

// Template returns the class's template with that name.
func (c *VehicleClass) Template(name string) (VehicleTemplate, bool) {
	for _, t := range c.Templates {
		if t.Name == name {
			return t, true
		}
	}
	return VehicleTemplate{}, false
}

// HasSlot says whether the class has a slot of that type.
func (c *VehicleClass) HasSlot(slot string) bool {
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
	for id, m := range vehicleData.Modules {
		if m.Class == class && m.Slot == slot {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := vehicleData.Modules[out[i]], vehicleData.Modules[out[j]]
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
		m, ok := vehicleData.Modules[s]
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
func BodyUtilitySeen(class *VehicleClass, bodyBase, utilitySlot string) bool {
	for _, t := range class.Templates {
		if base := moduleBase(t.Modules["BodyHull"]); base == bodyBase && t.Modules[utilitySlot] != "" {
			return true
		}
	}
	return false
}

var gradeSuffix = regexp.MustCompile(`_\d+$`)

// ModuleBase is a module id without its grade (BuggyHullBackExtra_6 -> BuggyHullBackExtra).
func ModuleBase(id string) string { return moduleBase(id) }

func moduleBase(id string) string { return gradeSuffix.ReplaceAllString(id, "") }

// Derived is what the rows of a vehicle hold that follows from its modules.
type Derived struct {
	MaxHealth   float64 // the health of an undamaged vehicle: the sum of Health over the Chassis, Body and Cockpit modules
	Critical    float64 // the health at which the vehicle goes down but not out: the chassis's CriticalStateHealth
	Fuel        float64 // a full tank: the chassis's fuel capacity
	CargoSlots  int64   // the cargo hold's slots: the sum over the modules that provide some
	CargoVolume float64 // and its volume
}

// DeriveVehicle works out the numbers a vehicle's rows hold from the ids of its modules. It is the one rule the editor uses to write them and to check what it wrote.
func DeriveVehicle(moduleIDs []string) (Derived, error) {
	var d Derived
	chassis := 0
	for _, id := range moduleIDs {
		m, ok := vehicleData.Modules[id]
		if !ok {
			return d, fmt.Errorf("module %q is not in the editor's vehicle catalog", id)
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

// CheckVehicles verifies a vehicle catalog: every template of every class names one existing module of the class for each slot it fills, in the slot of the module, with
// every mandatory slot filled; the tier of a template is its chassis's grade; every module belongs to a class of the file and a slot the class has. It is used by the tests
// and by the generator before it writes the file.
func CheckVehicles(d *VehicleData) error {
	if d.Build == "" || len(d.Tables) == 0 {
		return errors.New("the file names no game build or tables")
	}
	var problems []string
	add := func(f string, a ...any) { problems = append(problems, fmt.Sprintf(f, a...)) }
	for key, c := range d.Classes {
		if c.ActorClass == "" || c.PropsKey == "" || c.PermissionName == "" || c.CargoHash == 0 || len(c.Holds) == 0 {
			add("class %s: its row shape is incomplete", key)
		}
		slots := map[string]VehicleSlot{}
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

// ModuleName is the in-game name of a module, from the item catalog (the item that installs it), or "" when the catalog has none. The item's id spells the grade of some modules
// otherwise than the module's row does, so both spellings are tried.
func ModuleName(id string) string {
	for _, base := range []string{id, locomotionPosition.ReplaceAllString(id, "Locomotion")} {
		for _, c := range []string{base, leadingZeroGrade.ReplaceAllString(base, "_$1"), plainGrade.ReplaceAllString(base, "_0$1")} {
			if n, ok := Name(c); ok {
				return n
			}
		}
	}
	return ""
}

// locomotionPosition matches the wheel or leg position in a locomotion module's id: the item that installs them is one item for all positions (BuggyLocomotion_6).
var locomotionPosition = regexp.MustCompile(`Locomotion(FrontLeft|FrontRight|BackLeft|BackRight|BackCenter|CenterLeft\d?|CenterRight\d?|Left|Right)`)

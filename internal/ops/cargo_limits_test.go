package ops

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The room in a vehicle's cargo hold (#483). The numbers below are those of real vehicles in saves the game wrote (docs/design/2026-10-28-cargo-limits.md):
//
//	Sandbike with no storage part  hold row 0 slots, volume 0.0
//	Sandbike with SandbikeInventory_2  15 slots, volume 250.0
//	Mk6 buggy with the Bigger Boot (BuggyInventory_Unique_Capacity_6)  35 slots, volume 3500.0
//
// Carbon Ore (DolomiteRock) stacks to 500 and takes 0.2 volume a unit.
const carbon = "DolomiteRock"

var (
	// the module lists are those of real vehicles
	sandbikePlain = []string{"SandbikeChassis_1", "SandbikeHull_1", "SandbikeEngine_1", "SandbikeLocomotionFrontRight_1", "SandbikeLocomotionBackCenter_1", "SandbikeLocomotionFrontLeft_1", "SandbikeGenerator_1", "SandbikeBoost_2"}
	sandbikeHold  = []string{"SandbikeChassis_2", "SandbikeHull_2", "SandbikeEngine_2", "SandbikeLocomotionFrontRight_2", "SandbikeLocomotionBackCenter_2", "SandbikeLocomotionFrontLeft_2", "SandbikeGenerator_2", "SandbikeInventory_2"}
	buggyMk6      = []string{"BuggyChassis_6", "BuggyGenerator_6", "BuggyEngine_Unique_Accelerate_06", "BuggyHullFront_6", "BuggyHullBackExtra_6", "BuggyInventory_Unique_Capacity_6", "BuggyMining_Unique_YieldIncrease_06",
		"BuggyLocomotionFrontLeft_6", "BuggyLocomotionFrontRight_6", "BuggyLocomotionBackLeft_6", "BuggyLocomotionBackRight_6"}
)

// cargoVehicle is one owned vehicle of the fixture: its actor class, modules and hold row (slots/volume as SQL, so NULL can be written).
type cargoVehicle struct {
	id, hold      int64
	class         string
	modules       []string
	slots, volume string
	stacks        int // stacks of Carbon Ore (500 each) already in the hold
}

func (v cargoVehicle) sql() string {
	var b strings.Builder
	fmt.Fprintf(&b, "insert into actors(id,class,map,location_x,location_y,location_z) values (%d,'/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/%s.%s_C','HaggaBasin',0,0,0);\n", v.id, v.class, v.class)
	fmt.Fprintf(&b, "insert into vehicles(id) values (%d);\n", v.id)
	fmt.Fprintf(&b, "insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (%d,'mine',2,3,0);\n", v.id)
	fmt.Fprintf(&b, "insert into permission_actor_rank(permission_actor_id,player_id,rank) values (%d,1,1);\n", v.id)
	for i, m := range v.modules {
		fmt.Fprintf(&b, "insert into vehicle_modules(id,vehicle_id,template_id,stats) values (%d,%d,'%s',jsonb('{}'));\n", v.id*100+int64(i), v.id, m)
	}
	fmt.Fprintf(&b, "insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (%d,%d,0,%s,%s);\n", v.hold, v.id, v.slots, v.volume)
	for i := 0; i < v.stacks; i++ {
		fmt.Fprintf(&b, "insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (%d,%d,500,%d,'%s',0,1790000000,'{}',0);\n", v.hold*100+int64(i), v.hold, i, carbon)
	}
	return b.String()
}

const (
	sandbikeNone    = 8211 // Sandbike with no storage part
	buggyBigBoot    = 8212 // the Mk6 buggy with the Bigger Boot
	sandbikeSurplus = 8213 // Sandbike with no storage part and the 10 stacks of Carbon Ore the editor used to accept (5000)
	sandbikeSmall   = 8214 // Sandbike with SandbikeInventory_2
	sandbikeNoCount = 8215 // Sandbike with SandbikeInventory_2 whose hold row has no count (the catalog fills in)
	sandbikeNoPart  = 8216 // Sandbike with no storage part whose hold row has no count
)

func cargoLimitsWorld() string {
	var b strings.Builder
	b.WriteString(storageWorld + vehicleWorld)
	for _, v := range []cargoVehicle{
		{id: 211, hold: sandbikeNone, class: "BP_Sandbike_CHOAM", modules: sandbikePlain, slots: "0", volume: "0.0"},
		{id: 212, hold: buggyBigBoot, class: "BP_Buggy_CHOAM", modules: buggyMk6, slots: "35", volume: "3500.0"},
		{id: 213, hold: sandbikeSurplus, class: "BP_Sandbike_CHOAM", modules: sandbikePlain, slots: "0", volume: "0.0", stacks: 10},
		{id: 214, hold: sandbikeSmall, class: "BP_Sandbike_CHOAM", modules: sandbikeHold, slots: "15", volume: "250.0"},
		{id: 215, hold: sandbikeNoCount, class: "BP_Sandbike_CHOAM", modules: sandbikeHold, slots: "NULL", volume: "NULL"},
		{id: 216, hold: sandbikeNoPart, class: "BP_Sandbike_CHOAM", modules: sandbikePlain, slots: "NULL", volume: "NULL"},
	} {
		b.WriteString(v.sql())
	}
	return b.String()
}

func cargoLimitsOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, cargoLimitsWorld())} }

func cargoOf(t *testing.T, o *Ops, vehicle int64) map[string]any {
	t.Helper()
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
		if v["id"].(int64) == vehicle {
			c, _ := v["cargo"].(map[string]any)
			if c == nil {
				t.Fatalf("vehicle %d has no cargo entry: %v", vehicle, v)
			}
			return c
		}
	}
	t.Fatalf("vehicle %d is not listed", vehicle)
	return nil
}

func giveIntoHold(o *Ops, inv int64, tmpl string, qty float64) (map[string]any, error) {
	r, err := o.GiveToInventory(Args{"inventory_id": float64(inv), "template_id": tmpl, "quantity": qty})
	if err != nil {
		return nil, err
	}
	return r.(map[string]any), nil
}

// The card shows what the vehicle's parts give, never "10 of 0": no storage part is noHold, a hold shows its real slots and slots in use.
func TestVehicleCardShowsTheRoomOfTheHold(t *testing.T) {
	o := cargoLimitsOps(t)
	for _, c := range []struct {
		name    string
		hold    int64
		slots   any // int64 or nil
		used    int64
		noHold  bool
		volume  any
		items   int
		vehicle int64
	}{
		{"a Sandbike with no storage part", sandbikeNone, int64(0), 0, true, 0.0, 0, 211},
		{"the Mk6 buggy with the Bigger Boot", buggyBigBoot, int64(35), 0, false, 3500.0, 0, 212},
		{"a Sandbike with the 10 surplus stacks", sandbikeSurplus, int64(0), 10, true, 0.0, 10, 213},
		{"a Sandbike with a storage part", sandbikeSmall, int64(15), 0, false, 250.0, 0, 214},
		{"a hold row with no count, the catalog fills in (SandbikeInventory_2)", sandbikeNoCount, int64(15), 0, false, nil, 0, 215},
		{"a hold row with no count and no storage part", sandbikeNoPart, int64(0), 0, true, nil, 0, 216},
	} {
		got := cargoOf(t, o, c.vehicle)
		if got["inventory_id"] != c.hold || got["slots"] != c.slots || got["used"] != c.used || got["noHold"] != c.noHold || got["volume"] != c.volume || len(got["items"].([]map[string]any)) != c.items {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// What the editor reads from the hold row is what the parts give: the numbers of real vehicles (studied in docs/design/2026-10-28-cargo-limits.md) equal the catalog's sum over their modules.
func TestTheStoredRoomOfRealVehiclesIsWhatTheCatalogDerives(t *testing.T) {
	for _, c := range []struct {
		name    string
		modules []string
		slots   int64
		volume  float64
	}{
		{"Sandbike with no storage part", sandbikePlain, 0, 0},
		{"Sandbike with SandbikeInventory_2", sandbikeHold, 15, 250},
		{"Mk6 buggy with the Bigger Boot", buggyMk6, 35, 3500},
		{"Buggy with BuggyInventory_5", []string{"BuggyChassis_5", "BuggyInventory_5"}, 20, 2500},
		{"Medium ornithopter with OrnithopterMediumInventory_5", []string{"OrnithopterMediumChassis_5", "OrnithopterMediumInventory_5"}, 20, 2000},
		{"Sandcrawler with the big spice container", []string{"SandcrawlerChassis_6", "SandcrawlerSpiceContainer_Unique_Capacity_6"}, 30, 11250},
		{"Container vehicle", []string{"ContainerVehicle"}, 150, 15000},
	} {
		d, err := catalog.DeriveVehicle(c.modules)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if d.CargoSlots != c.slots || d.CargoVolume != c.volume {
			t.Errorf("%s: the catalog derives %d slots and volume %v, the saves hold %d and %v", c.name, d.CargoSlots, d.CargoVolume, c.slots, c.volume)
		}
	}
}

// A give into the hold of a vehicle with no storage part is refused by the server, with a clear message, and writes nothing. That holds whether the hold row says 0 or has no count.
func TestAGiveIntoAHoldWithNoSlotsIsRefused(t *testing.T) {
	o := cargoLimitsOps(t)
	for name, inv := range map[string]int64{"no storage part": sandbikeNone, "the surplus Sandbike": sandbikeSurplus, "no count, no storage part": sandbikeNoPart} {
		_, err := giveIntoHold(o, inv, carbon, 5000)
		if !errors.Is(err, apierr.ErrNoRoom) || err.Error() != "This vehicle has no cargo hold (no storage part)." {
			t.Errorf("%s: %v", name, err)
		}
		if o.S.Dirty() {
			t.Fatalf("%s: a refused give changed the save", name)
		}
	}
	if n := mustOne(t, o, `select count(*) c from items where inventory_id=?`, sandbikeNone)["c"].(int64); n != 0 {
		t.Fatalf("the hold of a Sandbike with no storage part is still empty: %d", n)
	}
}

// A hold with room takes what fits and says how much did not.
func TestAGiveThatFitsTheMk6BuggyIsWhole(t *testing.T) {
	o := cargoLimitsOps(t)
	r, err := giveIntoHold(o, buggyBigBoot, carbon, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(5000) || r["requested"] != int64(5000) || r["reason"] != "" || r["stacks"] != 10 {
		t.Fatalf("10 of the 35 slots: %v", r)
	}
	if c := cargoOf(t, o, 212); c["used"] != int64(10) || c["slots"] != int64(35) {
		t.Fatalf("the card reads 10 of 35: %v", c)
	}
}

func TestAGiveThatDoesNotFitTheMk6BuggyGivesWhatFitsAndSaysSo(t *testing.T) {
	o := cargoLimitsOps(t)
	// 35 slots of 500 are 17,500 units, which at 0.2 a unit is the hold's volume of 3,500: both limits meet there, and the volume is the one reported
	r, err := giveIntoHold(o, buggyBigBoot, carbon, 20000)
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(17500) || r["requested"] != int64(20000) || r["reason"] != "the container is full (volume)" || r["stacks"] != 35 {
		t.Fatalf("%v", r)
	}
	if c := cargoOf(t, o, 212); c["used"] != int64(35) {
		t.Fatalf("35 of 35: %v", c)
	}
	if _, err := giveIntoHold(o, buggyBigBoot, carbon, 1); !errors.Is(err, apierr.ErrNoRoom) || !strings.Contains(err.Error(), "full") {
		t.Fatalf("a full hold takes nothing more: %v", err)
	}
}

// The volume of the hold counts too: a Sandbike hold of 15 slots and volume 250 takes 1,250 Carbon Ore (3 slots), not the 7,500 its slots could.
func TestTheVolumeOfASmallHoldCutsAGive(t *testing.T) {
	o := cargoLimitsOps(t)
	r, err := giveIntoHold(o, sandbikeSmall, carbon, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(1250) || r["requested"] != int64(5000) || r["reason"] != "the container is full (volume)" || r["limit"] != limitVolume {
		t.Fatalf("%v", r)
	}
	if c := cargoOf(t, o, 214); c["used"] != int64(3) || c["slots"] != int64(15) {
		t.Fatalf("3 of 15: %v", c)
	}
}

// A hold row with no count is limited by what the parts give.
func TestAHoldRowWithNoCountStillGivesAnswerForItsParts(t *testing.T) {
	o := cargoLimitsOps(t)
	if c := cargoOf(t, o, 215); c["slots"] != int64(15) || c["noHold"] != false {
		t.Fatalf("%v", c)
	}
	if _, err := giveIntoHold(o, sandbikeNoCount, carbon, 500); err != nil {
		t.Fatalf("a hold the parts give is not refused: %v", err)
	}
}

// A base container does the same at its limit. The fixture's 7001 is a GenericContainer (20 slots, 1000 volume) with two stacks in it; the silo 7002 is empty with 10 slots and
// 1000 volume: five thousand Melange at 0.2 a unit fill both limits.
func TestABaseContainerTakesWhatFitsAtItsLimit(t *testing.T) {
	o := cargoLimitsOps(t)
	r, err := giveIntoHold(o, 7002, "MelangeSpice", 6000)
	if err != nil {
		t.Fatal(err)
	}
	if r["given"] != int64(5000) || r["requested"] != int64(6000) || r["reason"] != "the container is full (volume)" {
		t.Fatalf("%v", r)
	}
	if n := mustOne(t, o, `select count(distinct position_index) c from items where inventory_id=7002`)["c"].(int64); n != 10 {
		t.Fatalf("10 of 10 slots: %d", n)
	}
	if _, err := giveIntoHold(o, 7002, "MelangeSpice", 1); !errors.Is(err, apierr.ErrNoRoom) {
		t.Fatalf("a full container takes nothing more: %v", err)
	}
	// the other limit of a container: 7001 has room for 18 more stacks, but 1000 volume holds 5000 Carbon Ore (the Copper Ore and Knife in it count as far as they have a volume)
	r, err = giveIntoHold(o, 7001, carbon, 20000)
	if err != nil {
		t.Fatal(err)
	}
	given := r["given"].(int64)
	if given >= 20000 || given > 5000 || r["reason"] == "" {
		t.Fatalf("a container's volume cuts a give: %v", r)
	}
	if n := mustOne(t, o, `select count(distinct position_index) c from items where inventory_id=7001`)["c"].(int64); n > 20 {
		t.Fatalf("never past the 20 slots: %d", n)
	}
}

// Clear this container works on a hold with no slots, so the surplus the editor once accepted can be removed; so does Delete on a single row.
func TestClearAndDeleteRemoveTheSurplusInAHoldWithNoSlots(t *testing.T) {
	o := cargoLimitsOps(t)
	if _, err := o.DeleteItem(Args{"id": float64(sandbikeSurplus*100 + 9)}); err != nil {
		t.Fatalf("delete one surplus stack: %v", err)
	}
	p, err := o.ClearPreview(sandbikeSurplus, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := p.(map[string]any)["items"].(int64); n != 9 {
		t.Fatalf("9 stacks left to clear: %v", p)
	}
	res, err := o.ClearContainer(clearArgs(o, sandbikeSurplus, 9))
	if err != nil {
		t.Fatalf("clear a hold with no slots: %v", err)
	}
	if res.(map[string]any)["removed"] != int64(9) {
		t.Fatalf("%v", res)
	}
	c := cargoOf(t, o, 213)
	if len(c["items"].([]map[string]any)) != 0 || c["used"] != int64(0) || c["noHold"] != true {
		t.Fatalf("the hold is empty and still has no slots: %v", c)
	}
}

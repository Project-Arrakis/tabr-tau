package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The cargo hold of the character's own vehicle is a place the character may change (#48, the console's "View Contents" of a vehicle):
// Add item, the stack of an item, Delete and Clear this container. The cargo hold of a vehicle that is not the character's, and the
// per-component holds of a vehicle (inventory_type NULL), stay out of reach, as before (ARCH-9).
//
// vehicleDetailsWorld has the owned Sandbike 201 with its cargo hold 8001 (one stack, item 8101) and a component hold 8002. This adds the
// cargo hold of the vehicle 203 that belongs to someone else (8003, item 8103), an item in the component hold (8104), and the cargo
// hold of a vehicle that has no permission record at all, like the world's (8005 of 202, item 8105).
const vehicleCargoWorld = vehicleDetailsWorld + `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (8003,203,0,15,500.0),(8005,202,0,15,500.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (8103,8003,5,0,'CopperOre',0,1790000000,'{}',0),
  (8104,8002,6,0,'CopperOre',0,1790000000,'{}',0),
  (8105,8005,7,0,'CopperOre',0,1790000000,'{}',0);`

func cargoOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, vehicleCargoWorld)} }

func TestItemsInTheCargoHoldOfMyVehicleCanBeEditedAndDeleted(t *testing.T) {
	o := cargoOps(t)
	if _, err := o.SetItem(Args{"id": 8101.0, "stack_size": 55.0}); err != nil {
		t.Fatalf("the stack of an item in my vehicle's cargo hold: %v", err)
	}
	if got := stackOf(t, o, 8101); got != 55 {
		t.Fatalf("stack %d", got)
	}
	if _, err := o.DeleteItem(Args{"id": 8101.0}); err != nil {
		t.Fatalf("delete from my vehicle's cargo hold: %v", err)
	}
	if n := mustOne(t, o, `select count(*) c from items where id=8101`)["c"].(int64); n != 0 {
		t.Fatalf("the item is gone: %d", n)
	}
}

func TestItemsInOtherVehiclesAndComponentHoldsStayOutOfReach(t *testing.T) {
	o := cargoOps(t)
	for name, id := range map[string]float64{"the cargo hold of a vehicle that is someone else's": 8103, "a component hold of my own vehicle": 8104, "the cargo hold of a world vehicle": 8105} {
		if _, err := o.SetItem(Args{"id": id, "stack_size": 7.0}); err == nil || !strings.Contains(err.Error(), "not in your") {
			t.Errorf("an edit in %s must be refused: %v", name, err)
		}
		if _, err := o.DeleteItem(Args{"id": id}); err == nil || !strings.Contains(err.Error(), "not in your") {
			t.Errorf("a delete in %s must be refused: %v", name, err)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused edit must not change the save")
	}
}

func TestGiveIntoMyVehiclesCargoHoldAndNowhereElseInVehicles(t *testing.T) {
	o := cargoOps(t)
	if _, err := o.GiveToInventory(Args{"inventory_id": 8001.0, "template_id": "MelangeSpice", "quantity": 3.0}); err != nil {
		t.Fatalf("give into my vehicle's cargo hold: %v", err)
	}
	if n := mustOne(t, o, `select coalesce(sum(stack_size),0) c from items where inventory_id=8001 and template_id='MelangeSpice'`)["c"].(int64); n != 3 {
		t.Fatalf("3 melange in the cargo hold: %d", n)
	}
	o.S.Discard()
	for name, inv := range map[string]float64{"someone else's vehicle": 8003, "a component hold": 8002, "a world vehicle": 8005} {
		if _, err := o.GiveToInventory(Args{"inventory_id": inv, "template_id": "MelangeSpice", "quantity": 1.0}); err == nil || !strings.Contains(err.Error(), "not a container at one of your bases") {
			t.Errorf("a give into %s must be refused: %v", name, err)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused give must not change the save")
	}
}

func TestClearMyVehiclesCargoHold(t *testing.T) {
	o := cargoOps(t)
	if _, err := o.ClearPreview(8003, false); err == nil {
		t.Fatal("the preview of another vehicle's cargo hold must be refused")
	}
	for _, inv := range []int64{8003, 8002, 8005} {
		if _, err := o.ClearContainer(clearArgs(o, inv, 1)); err == nil {
			t.Errorf("container %d must not be cleared", inv)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused clear must not change the save")
	}
	p, err := o.ClearPreview(8001, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := p.(map[string]any)["items"].(int64); n != 1 {
		t.Fatalf("the cargo hold holds one stack: %v", p)
	}
	if _, err := o.ClearContainer(clearArgs(o, 8001, 1)); err != nil {
		t.Fatalf("clear my vehicle's cargo hold: %v", err)
	}
	if n := mustOne(t, o, `select count(*) c from items where inventory_id=8001`)["c"].(int64); n != 0 {
		t.Fatalf("the cargo hold is empty: %d", n)
	}
	if n := mustOne(t, o, `select count(*) c from items where inventory_id in (8002,8003,8005)`)["c"].(int64); n != 3 {
		t.Fatalf("the other holds keep their items: %d", n)
	}
}

func TestCargoHoldScopeNeedsAnOwnedVehicleNotJustTheType(t *testing.T) {
	// the character's own backpack is inventory_type 0 too, on the body: it is carried, not a vehicle, and still reachable through its own rule;
	// a vehicle that loses its permission record is no longer mine, and neither is its hold
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleCargoWorld+`delete from permission_actor_rank where permission_actor_id=201;`)}
	if _, err := o.SetItem(Args{"id": 8101.0, "stack_size": 9.0}); err == nil {
		t.Fatal("a vehicle the character no longer owns is out of reach")
	}
}

// The fuel column used to show a bare number. Where the tank of the vehicle's kind was measured in the game (the table Refuel uses) the
// list also says how big the tank is and how full it is; a kind that was not measured shows the number alone, never a guess.
func TestVehicleFuelSaysHowFullTheTankIsOnlyWhereTheTankIsKnown(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleCargoWorld+`
insert into actors(id,class,map,location_x,location_y,location_z) values (205,'/Game/BP_Unmeasured.BP_Unmeasured_C','HaggaBasin',0,0,0);
insert into vehicles(id) values (205);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (205,'mine',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (205,1,1);
insert into fgl_entities(entity_id,components) values (-205, jsonb('{"FVehicleComponent":[34,{"CurrentFuel":500.0}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (205,-205,'Actor');`)}
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	by := map[int64]map[string]any{}
	for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
		by[v["id"].(int64)] = v
	}
	if v := by[201]; v["fuelMax"] != 100.0 || v["fuelPercent"] != 90.5 {
		t.Errorf("a Sandbike holds 100 and has 90.5: %v / %v", v["fuelMax"], v["fuelPercent"])
	}
	if v := by[205]; v["fuel"] == nil || v["fuelMax"] != nil || v["fuelPercent"] != nil {
		t.Errorf("a kind with no measured tank shows the number alone: fuel %v max %v percent %v", v["fuel"], v["fuelMax"], v["fuelPercent"])
	}
}

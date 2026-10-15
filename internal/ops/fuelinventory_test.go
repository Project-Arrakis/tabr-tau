package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Audit L2 ARCH-6 (#165): the refill and the Power tab used to find a power device's fuel inventory differently (the first inventory of any
// type against the inventory of type 3). They share one definition now: a generator whose lowest-numbered inventory is something else is
// filled in its type-3 inventory, and the Power tab counts what the refill wrote.
func TestRefillAndPowerTabAgreeOnTheFuelInventory(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into actors(id,class,map) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin'),(831,'c','HaggaBasin');
insert into totems(id) values (601);
insert into fgl_entities(entity_id,components) values (-601, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'Actor');
insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id,is_hologram) values (601,-601,'Totem_Placeable',1,0),(831,-601,'Generator_Placeable',1,0);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (841,831,9,5,100.0),(842,831,3,5,100.0);`)}
	if _, err := o.RefillGenerators(); err != nil {
		t.Fatal(err)
	}
	if n := mustOne(t, o, `select count(*) n from items where inventory_id=841`)["n"].(int64); n != 0 {
		t.Fatalf("the refill wrote fuel into an inventory that is not the fuel inventory (%d items)", n)
	}
	if _, n, total := stacks(t, o, 842); n != 1 || total != 499 {
		t.Fatalf("the fuel inventory holds one full stack, got %d rows / %d", n, total)
	}
	r, err := o.BasePower(601)
	if err != nil {
		t.Fatal(err)
	}
	g := r.(map[string]any)["types"].([]map[string]any)[0]
	if g["kind"] != "generator_placeable" || g["queued"].(int64) != 499 || g["empty"].(int64) != 0 {
		t.Fatalf("the Power tab counts what the refill wrote: %v", g)
	}
}

package ops

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Repair buildings and Repair base (#613, audit G-009): the repair target of a type is the best health seen on STANDING pieces of the player's own bases, never above the cap of the
// type. The pieces of an imported backup (picked-up state, no owner) and any piece whose actor is picked up are not a source of the target, and a standing piece with a health far above
// what the game gives is clamped, not spread.
//
// Base 601 stands. Foundation: 7000 and 3500. Wall: 5600 and 1000. Cistern placeable: 2500 and 300.
const capWorld = `
insert into actors(id,class,map,state) values (601,'t','HaggaBasin',0),(801,'b','HaggaBasin',0),(851,'p','HaggaBasin',0),(852,'p','HaggaBasin',0);
insert into totems(id) values (601);
insert into fgl_entities(entity_id,components) values (-601, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'Actor');
insert into building_instances(building_id,instance_id,building_type,health,owner_entity_id) values
  (801,1,'Foundation',7000.0,-601),(801,2,'Foundation',3500.0,-601),(801,3,'Wall',5600.0,-601),(801,4,'Wall',1000.0,-601);
insert into placeables(id,health,building_type,owner_entity_id) values (851,2500.0,'Cistern',-601),(852,300.0,'Cistern',-601);`

func capOps(t *testing.T, extra ...string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, capWorld+"\n"+strings.Join(extra, "\n"))}
}

func repairAs(t *testing.T, o *Ops, how string) {
	t.Helper()
	var err error
	if how == "base" {
		_, err = o.RepairBase(601)
	} else {
		_, err = o.RepairBuildings()
	}
	if err != nil {
		t.Fatalf("repair (%s): %v", how, err)
	}
}

// standingRepaired checks the rows of capWorld after a repair that took the normal target: Foundation 7000, Wall 5600, Cistern 2500.
func standingRepaired(t *testing.T, o *Ops) {
	t.Helper()
	if got := healths(t, o, `select health h from building_instances where building_id=801 order by instance_id`); got[0] != 7000 || got[1] != 7000 || got[2] != 5600 || got[3] != 5600 {
		t.Fatalf("standing piece healths %v, want [7000 7000 5600 5600]", got)
	}
	if got := healths(t, o, `select health h from placeables where id in (851,852) order by id`); got[0] != 2500 || got[1] != 2500 {
		t.Fatalf("standing placeable healths %v, want [2500 2500]", got)
	}
}

// Normal repair is what it was: each piece goes to the best of its type, and a repeat finds nothing.
func TestRepairTargetNormalRepairUnchanged(t *testing.T) {
	for _, how := range []string{"base", "all"} {
		t.Run(how, func(t *testing.T) {
			o := capOps(t)
			repairAs(t, o, how)
			standingRepaired(t, o)
			if r, _ := o.RepairBuildings(); r.(map[string]any)["pieces"].(int64) != 0 || r.(map[string]any)["placeables"].(int64) != 0 {
				t.Fatalf("a repeat must find nothing: %v", r)
			}
		})
	}
}

// Pieces of a backup (state 5) with a health far above the standing ones do not raise the target: the ones no base owns, and the ones an imported totem's entity owns.
func TestRepairTargetIgnoresPiecesOfABackup(t *testing.T) {
	backup := `
insert into actors(id,class,map,state) values (602,'t','HaggaBasin',5),(802,'b','HaggaBasin',5),(853,'p','HaggaBasin',5);
insert into totems(id) values (602);
insert into fgl_entities(entity_id,components) values (-602, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (602,-602,'Actor');
insert into building_instances(building_id,instance_id,building_type,health,owner_entity_id) values
  (802,1,'Foundation',900000.0,NULL),(802,2,'Wall',900000.0,-602);
insert into placeables(id,health,building_type,owner_entity_id) values (853,900000.0,'Cistern',-602);`
	for _, how := range []string{"base", "all"} {
		t.Run(how, func(t *testing.T) {
			o := capOps(t, backup)
			repairAs(t, o, how)
			standingRepaired(t, o)
		})
	}
}

// A piece whose owner is a totem but whose actor is picked up (state 5) is not standing and is not a source; neither is a piece no base owns, a piece of a foreign owner or a hologram.
func TestRepairTargetIgnoresPiecesThatAreNotStanding(t *testing.T) {
	o := capOps(t, `
insert into actors(id,class,map,state) values (803,'b','HaggaBasin',5),(804,'b','HaggaBasin',0),(854,'p','HaggaBasin',0);
insert into building_instances(building_id,instance_id,building_type,health,owner_entity_id) values
  (803,1,'Foundation',8000.0,-601),(804,1,'Foundation',8100.0,NULL),(804,2,'Wall',8100.0,5);
insert into placeables(id,health,building_type,owner_entity_id,is_hologram) values (854,4000.0,'Cistern',-601,1);`)
	repairAs(t, o, "all")
	standingRepaired(t, o)
}

// A standing piece above the cap of its type is not spread: the target is the cap. A piece that is already above the cap is not lowered.
func TestRepairTargetIsClampedToTheCap(t *testing.T) {
	// Sardaukar_Foundation has a cap of its own (7000 seen, times 1.25 = 8750); Foundation and Cistern are not listed and get the overall cap of their table (8750; 25000);
	// LargeWindtrap_Placeable has a cap of its own (3500 seen, times 1.25 = 4375).
	o := capOps(t, `
insert into actors(id,class,map,state) values (805,'b','HaggaBasin',0),(855,'p','HaggaBasin',0),(856,'p','HaggaBasin',0),(857,'p','HaggaBasin',0);
insert into building_instances(building_id,instance_id,building_type,health,owner_entity_id) values
  (805,1,'Sardaukar_Foundation',900000.0,-601),(805,2,'Sardaukar_Foundation',100.0,-601),(805,3,'Foundation',999999.0,-601);
insert into placeables(id,health,building_type,owner_entity_id) values
  (855,999999.0,'Cistern',-601),(856,900000.0,'LargeWindtrap_Placeable',-601),(857,10.0,'LargeWindtrap_Placeable',-601);`)
	repairAs(t, o, "all")
	if got := healths(t, o, `select health h from building_instances where building_type='Sardaukar_Foundation' order by instance_id`); got[0] != 900000 || got[1] != 8750 {
		t.Fatalf("Sardaukar_Foundation healths %v, want [900000 8750]", got)
	}
	if got := healths(t, o, `select health h from building_instances where building_type='Foundation' order by building_id, instance_id`); got[0] != 8750 || got[1] != 8750 || got[2] != 999999 {
		t.Fatalf("Foundation healths %v, want [8750 8750 999999]", got)
	}
	if got := healths(t, o, `select health h from placeables where building_type='Cistern' order by id`); got[0] != 25000 || got[1] != 25000 || got[2] != 999999 {
		t.Fatalf("Cistern healths %v, want [25000 25000 999999]", got)
	}
	if got := healths(t, o, `select health h from placeables where building_type='LargeWindtrap_Placeable' order by id`); got[0] != 900000 || got[1] != 4375 {
		t.Fatalf("windtrap healths %v, want [900000 4375]", got)
	}
}

// The overall caps and the per-type caps agree with the figures they were measured from, and a type name is quoted safely in the statement.
func TestHealthCapsAreAboveWhatWasMeasured(t *testing.T) {
	if MaxRepairPieceHealth != 8750 || MaxRepairPlaceableHealth != 25000 {
		t.Fatalf("the overall caps are %v and %v", MaxRepairPieceHealth, MaxRepairPlaceableHealth)
	}
	for ty, seen := range pieceTypeHealthSeen {
		if seen > pieceHealthSeen || healthCapOf(seen) > MaxRepairPieceHealth || healthCapOf(seen) < seen {
			t.Errorf("%s: seen %v does not fit the caps of the pieces", ty, seen)
		}
	}
	for ty, seen := range placeableTypeHealthSeen {
		if seen > placeableHealthSeen || healthCapOf(seen) > MaxRepairPlaceableHealth || healthCapOf(seen) < seen {
			t.Errorf("%s: seen %v does not fit the caps of the placeables", ty, seen)
		}
	}
	if got := capValues(map[string]float64{"A'B": 100}); got != "('A''B',125)" {
		t.Fatalf("a quote in a type name must be doubled: %s", got)
	}
}

// A real import of a backup whose pieces carry a high (but in-range) health does not raise what Repair buildings writes to the pieces that stand.
func TestImportedHighHealthBackupDoesNotRaiseTheRepairTarget(t *testing.T) {
	high := editFile(t, exportedFile(t), func(tb map[string][]map[string]any) {
		for _, r := range tb["building_instances"] {
			r["health"] = json.Number("900000")
		}
		for _, r := range tb["placeables"] {
			r["health"] = json.Number("900000")
		}
	})
	o := capOps(t)
	mustImport(t, o, high)
	if cnt(t, o, `select count(*) from building_instances where health=900000`) == 0 {
		t.Fatal("the imported pieces are not in the save")
	}
	if _, err := o.RepairBuildings(); err != nil {
		t.Fatal(err)
	}
	standingRepaired(t, o)
}

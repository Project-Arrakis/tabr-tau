package ops

import (
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Audit L2 G13 (#170): RepairBuildings looked the best health of a type up again for every piece (a correlated subquery over the whole
// table), so it grew with the square of the piece count and held the save locked for all of it. A big base must repair in well under a
// second; the old statement took about 16 s for the 10,000 pieces here and about 4 s more for the placeables.
func TestRepairBuildingsIsLinearInThePieceCount(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into actors(id,class,map) values (900000,'c','HaggaBasin');
with recursive n(i) as (select 1 union all select i+1 from n where i<10000)
  insert into building_instances(building_id,instance_id,building_type,health)
  select 900000, i, 'T'||(i%4), case when i%50=0 then 4000.0 else 100.0 end from n;
with recursive n(i) as (select 1 union all select i+1 from n where i<5000)
  insert into actors(id,class,map) select 1000000+i, 'c', 'HaggaBasin' from n;
with recursive n(i) as (select 1 union all select i+1 from n where i<5000)
  insert into placeables(id,health,building_type) select 1000000+i, case when i%50=0 then 900.0 else 50.0 end, 'P'||(i%4) from n;`)}
	start := time.Now()
	r, err := o.RepairBuildings()
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("repairing 10,000 pieces and 5,000 placeables took %v: the work must not grow with the square of the piece count", took)
	}
	res := r.(map[string]any)
	// types T0 and T2 hold the 4000s (every 50th piece is 4000, and 50 and 100 fall in T2 and T0): 2,500 of each, less the 100 that already are
	if res["pieces"].(int64) != 4800 || res["placeables"].(int64) != 2400 {
		t.Fatalf("counts: %v", res)
	}
	row := mustOne(t, o, `select min(health) lo, max(health) hi from building_instances where building_type in ('T0','T2')`)
	if row["lo"].(float64) != 4000 || row["hi"].(float64) != 4000 {
		t.Fatalf("every piece of a type holding a 4000 goes to 4000: %v", row)
	}
	row = mustOne(t, o, `select min(health) lo, max(health) hi from building_instances where building_type in ('T1','T3')`)
	if row["lo"].(float64) != 100 || row["hi"].(float64) != 100 {
		t.Fatalf("types whose best is 100 do not change: %v", row)
	}
}

// Pieces with no type belong to no group: they are never raised to another type's best, and a type's best is never lowered onto them.
func TestRepairBuildingsLeavesUntypedPiecesAlone(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `
insert into actors(id,class,map) values (801,'c','HaggaBasin'),(851,'c','HaggaBasin'),(852,'c','HaggaBasin'),(853,'c','HaggaBasin');
insert into building_instances(building_id,instance_id,building_type,health) values
  (801,1,'Wall',900.0),(801,2,'Wall',100.0),(801,3,null,10.0),(801,4,null,5000.0);
insert into placeables(id,health,building_type) values (851,500.0,'Cistern'),(852,null,'Cistern'),(853,5.0,null);`)}
	r, err := o.RepairBuildings()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["pieces"].(int64) != 1 || res["placeables"].(int64) != 0 {
		t.Fatalf("only the 100 Wall is raised: %v", res)
	}
	got := healths(t, o, `select health h from building_instances order by instance_id`)
	if got[0] != 900 || got[1] != 900 || got[2] != 10 || got[3] != 5000 {
		t.Fatalf("building healths %v", got)
	}
	if h := mustOne(t, o, `select health h from placeables where id=853`)["h"].(float64); h != 5 {
		t.Fatalf("an untyped placeable is not touched: %v", h)
	}
	if h := mustOne(t, o, `select health h from placeables where id=852`)["h"]; h != nil {
		t.Fatalf("a placeable with no recorded health is not given one: %v", h)
	}
}

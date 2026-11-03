package ops

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Two bases (totems 601 and 602, entities -601 and -602), each with its own building pieces and placeables of DIFFERENT health, and things
// no base owns: pieces and a placeable with no owner, a piece whose owner entity belongs to a plain actor (900), and a hologram of base A.
// Actor 900 is not a totem.
//
//	A pieces: Foundation 7000 (sand 10), Foundation 3500 (sand 40), Wall 1000          placeables: totem 5000, cistern 300, hologram generator 1
//	B pieces: Foundation 6000 (sand 5), Wall 5600 (sand 12), Wall 2000                  placeables: totem 4000, cistern 1250, generator 2500
//	none:     Wall 100 (sand 3, no owner), Foundation 200 (owner -609 of a plain actor)  placeable: cistern 50 (no owner)
const twoBaseWorld = `
insert into actors(id,class,map) values (601,'t','HaggaBasin'),(602,'t','HaggaBasin'),(900,'x','HaggaBasin'),
  (801,'b','HaggaBasin'),(802,'b','HaggaBasin'),(803,'b','HaggaBasin'),
  (851,'p','HaggaBasin'),(852,'p','HaggaBasin'),(853,'p','HaggaBasin'),(854,'p','HaggaBasin'),(855,'p','HaggaBasin'),(856,'p','HaggaBasin'),(857,'p','HaggaBasin');
insert into totems(id) values (601),(602);
insert into fgl_entities(entity_id,components) values (-601, jsonb('{}')),(-602, jsonb('{}')),(-609, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'Actor'),(602,-602,'Actor'),(900,-609,'Actor');
insert into building_instances(building_id,instance_id,building_type,health,sand_buildup,owner_entity_id) values
  (801,1,'Foundation',7000.0,10,-601),(801,2,'Foundation',3500.0,40,-601),(801,3,'Wall',1000.0,0,-601),
  (802,1,'Foundation',6000.0,5,-602),(802,2,'Wall',5600.0,12,-602),(802,3,'Wall',2000.0,0,-602),
  (803,1,'Wall',100.0,3,NULL),(803,2,'Foundation',200.0,0,-609);
insert into placeables(id,owner_entity_id,health,building_type,last_placed_by_player_id,is_hologram) values
  (601,-601,5000.0,'Totem_Placeable',1,0),(851,-601,300.0,'WaterCistern_Placeable',1,0),(855,-601,1.0,'Generator_Placeable',1,1),
  (602,-602,4000.0,'Totem_Placeable',1,0),(852,-602,1250.0,'WaterCistern_Placeable',1,0),(853,-602,2500.0,'Generator_Placeable',1,0),
  (854,NULL,50.0,'WaterCistern_Placeable',1,0);`

func twoBaseOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, twoBaseWorld)} }

func structures(t *testing.T, o *Ops, base int64) map[string]any {
	t.Helper()
	r, err := o.BaseStructures(base)
	if err != nil {
		t.Fatalf("base %d: %v", base, err)
	}
	return r.(map[string]any)
}

func placeableIDs(rows []map[string]any) []int64 {
	var out []int64
	for _, r := range rows {
		out = append(out, r["id"].(int64))
	}
	return out
}

func sameIDs(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// Base A open shows A only: the health card, the piece types and the placeables, with B's different health values present in the save.
func TestBaseStructuresShowsTheOpenBaseOnly(t *testing.T) {
	o := twoBaseOps(t)
	a := structures(t, o, 601)
	p := a["pieces"].(map[string]any)
	if p["n"].(int64) != 3 || p["minh"].(float64) != 1000 || p["sand"].(int64) != 50 {
		t.Fatalf("A health card: %v", p)
	}
	if got := p["avgh"].(float64); got < 3833.3 || got > 3833.4 { // (7000 + 3500 + 1000) / 3
		t.Fatalf("A average health %v", got)
	}
	types := a["types"].([]map[string]any)
	if len(types) != 2 {
		t.Fatalf("A piece types: %v", types)
	}
	f, w := types[0], types[1] // Foundation (2) before Wall (1)
	if f["building_type"] != "Foundation" || f["n"].(int64) != 2 || f["minh"].(float64) != 3500 || f["maxh"].(float64) != 7000 || f["avgh"].(float64) != 5250 {
		t.Fatalf("A Foundation row: %v", f)
	}
	if w["building_type"] != "Wall" || w["n"].(int64) != 1 || w["minh"].(float64) != 1000 || w["maxh"].(float64) != 1000 || w["avgh"].(float64) != 1000 {
		t.Fatalf("A Wall row: %v", w) // B's walls (5600, 2000) are not in it
	}
	// the totem and the cistern; not B's, not the unowned one, not the hologram
	if ids := placeableIDs(a["placeables"].([]map[string]any)); !sameIDs(ids, []int64{601, 851}) { // ordered by type: Totem, WaterCistern
		t.Fatalf("A placeables: %v", ids)
	}

	b := structures(t, o, 602)
	bp := b["pieces"].(map[string]any)
	if bp["n"].(int64) != 3 || bp["minh"].(float64) != 2000 || bp["sand"].(int64) != 17 {
		t.Fatalf("B health card: %v", bp)
	}
	bt := b["types"].([]map[string]any)
	if len(bt) != 2 || bt[0]["building_type"] != "Wall" || bt[0]["n"].(int64) != 2 || bt[0]["minh"].(float64) != 2000 || bt[0]["maxh"].(float64) != 5600 || bt[0]["avgh"].(float64) != 3800 ||
		bt[1]["building_type"] != "Foundation" || bt[1]["n"].(int64) != 1 || bt[1]["maxh"].(float64) != 6000 {
		t.Fatalf("B piece types: %v", bt)
	}
	if ids := placeableIDs(b["placeables"].([]map[string]any)); !sameIDs(ids, []int64{853, 602, 852}) {
		t.Fatalf("B placeables: %v", ids)
	}
}

// Pieces no base owns are listed apart and never counted into a base; all the bases and the unowned add up to the whole save.
func TestBaseStructuresListsUnownedPiecesApartAndTheCountsAddUp(t *testing.T) {
	o := twoBaseOps(t)
	a, b := structures(t, o, 601), structures(t, o, 602)
	un := a["unowned"].(map[string]any)
	up := un["pieces"].(map[string]any)
	if up["n"].(int64) != 2 || up["minh"].(float64) != 100 || up["sand"].(int64) != 3 {
		t.Fatalf("unowned pieces: %v", up)
	}
	if ids := placeableIDs(un["placeables"].([]map[string]any)); !sameIDs(ids, []int64{854}) {
		t.Fatalf("unowned placeables: %v", ids)
	}
	if len(un["types"].([]map[string]any)) != 2 {
		t.Fatalf("unowned types: %v", un["types"])
	}
	// the unowned figures are the same whichever base is open
	if fmt.Sprint(b["unowned"]) != fmt.Sprint(a["unowned"]) {
		t.Fatalf("unowned differs between bases:\n%v\n%v", a["unowned"], b["unowned"])
	}
	sv := a["save"].(map[string]any)
	pieces := a["pieces"].(map[string]any)["n"].(int64) + b["pieces"].(map[string]any)["n"].(int64) + up["n"].(int64)
	placeables := int64(len(a["placeables"].([]map[string]any)) + len(b["placeables"].([]map[string]any)) + len(un["placeables"].([]map[string]any)))
	if pieces != sv["pieces"].(int64) || sv["pieces"].(int64) != 8 {
		t.Fatalf("pieces: A + B + unowned = %d, the save has %v", pieces, sv["pieces"])
	}
	if placeables != sv["placeables"].(int64) || sv["placeables"].(int64) != 6 { // the hologram is not built and is not counted
		t.Fatalf("placeables: A + B + unowned = %d, the save has %v", placeables, sv["placeables"])
	}
}

// A base with no pieces says so (an empty health card), and an owner entity of a plain actor is not a base.
func TestBaseStructuresOfABaseWithNothingIsEmpty(t *testing.T) {
	o := twoBaseOps(t)
	testsave.Exec(t, o.S, `insert into actors(id,class,map) values (603,'t','HaggaBasin'); insert into totems(id) values (603);`)
	r := structures(t, o, 603)
	if r["pieces"].(map[string]any)["n"].(int64) != 0 || len(r["types"].([]map[string]any)) != 0 || len(r["placeables"].([]map[string]any)) != 0 {
		t.Fatalf("an empty base: %v", r)
	}
}

func TestStructureRoutesRefuseAnIdThatIsNotATotem(t *testing.T) {
	o := twoBaseOps(t)
	for _, id := range []int64{0, -601, 900, 801, 851, 99999} {
		if _, err := o.BaseStructures(id); !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("BaseStructures(%d): %v", id, err)
		}
		if _, err := o.RepairBase(id); !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("RepairBase(%d): %v", id, err)
		}
		if _, err := o.ClearSandBase(id); !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("ClearSandBase(%d): %v", id, err)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused id must write nothing")
	}
}

func structureRowsDump(t *testing.T, o *Ops) string {
	t.Helper()
	pieces, err := o.S.Query(`select building_id, instance_id, health, sand_buildup from building_instances order by building_id, instance_id`)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := o.S.Query(`select id, health from placeables order by id`)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(pieces, pl)
}

func rowsWhere(t *testing.T, o *Ops, q string) string {
	t.Helper()
	rows, err := o.S.Query(q)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(rows)
}

// Repair this base changes only A's rows, to the best health of the type in the save; B, the unowned rows and the hologram stay as they were.
func TestRepairBaseChangesOnlyThatBase(t *testing.T) {
	o := twoBaseOps(t)
	bPieces := `select health, sand_buildup from building_instances where building_id=802 order by instance_id`
	bPlace := `select id, health from placeables where id in (602,852,853)`
	other := `select building_id, health from building_instances where building_id=803 union all select id, health from placeables where id in (854,855)`
	wantB, wantBP, wantOther := rowsWhere(t, o, bPieces), rowsWhere(t, o, bPlace), rowsWhere(t, o, other)

	r, err := o.RepairBase(601)
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["pieces"].(int64) != 2 || res["placeables"].(int64) != 1 { // the 3500 Foundation and the 1000 Wall; the 300 cistern
		t.Fatalf("counts: %v", res)
	}
	got := healths(t, o, `select health h from building_instances where building_id=801 order by instance_id`)
	if got[0] != 7000 || got[1] != 7000 || got[2] != 5600 { // the target is the best of the type in the whole save: Wall 5600 is B's
		t.Fatalf("A pieces: %v", got)
	}
	if pl := healths(t, o, `select health h from placeables where id in (601,851) order by id`); pl[0] != 5000 || pl[1] != 1250 {
		t.Fatalf("A placeables: %v", pl)
	}
	if rowsWhere(t, o, bPieces) != wantB || rowsWhere(t, o, bPlace) != wantBP || rowsWhere(t, o, other) != wantOther {
		t.Fatalf("another base, an unowned row or a hologram changed:\nB %s\nB placeables %s\nother %s", rowsWhere(t, o, bPieces), rowsWhere(t, o, bPlace), rowsWhere(t, o, other))
	}
	// a repeat finds nothing
	r2, _ := o.RepairBase(601)
	if r2.(map[string]any)["pieces"].(int64) != 0 || r2.(map[string]any)["placeables"].(int64) != 0 {
		t.Fatalf("a repeat must find nothing: %v", r2)
	}
}

// The open base after its repair, and the other base's own numbers, as the tab shows them.
func TestStructuresAfterRepairShowOnlyThatBaseChanged(t *testing.T) {
	o := twoBaseOps(t)
	beforeB := fmt.Sprint(structures(t, o, 602)["pieces"], structures(t, o, 602)["types"])
	if _, err := o.RepairBase(601); err != nil {
		t.Fatal(err)
	}
	if afterB := fmt.Sprint(structures(t, o, 602)["pieces"], structures(t, o, 602)["types"]); afterB != beforeB {
		t.Fatalf("base B's card changed:\n%s\n%s", beforeB, afterB)
	}
	if p := structures(t, o, 601)["pieces"].(map[string]any); p["minh"].(float64) != 5600 {
		t.Fatalf("A's lowest health after the repair: %v", p)
	}
}

// Clearing sand on a base clears that base's pieces only, and the health is left alone.
func TestClearSandBaseChangesOnlyThatBase(t *testing.T) {
	o := twoBaseOps(t)
	healthBefore := rowsWhere(t, o, `select sum(health) h from building_instances`)
	r, err := o.ClearSandBase(601)
	if err != nil || r.(map[string]any)["pieces"].(int64) != 2 { // the two foundations with sand
		t.Fatalf("%v %v", r, err)
	}
	if s := mustOne(t, o, `select coalesce(sum(sand_buildup),0) s from building_instances where building_id=801`)["s"].(int64); s != 0 {
		t.Fatalf("A still has sand: %d", s)
	}
	if s := mustOne(t, o, `select sum(sand_buildup) s from building_instances where building_id<>801`)["s"].(int64); s != 5+12+3 {
		t.Fatalf("B's and the unowned sand must stay: %d", s)
	}
	if rowsWhere(t, o, `select sum(health) h from building_instances`) != healthBefore {
		t.Fatal("clearing sand changed a health")
	}
}

// The whole-save actions stay what they were, and reach every row including those of no base.
func TestWholeSaveRepairAndSandStillReachEveryRow(t *testing.T) {
	o := twoBaseOps(t)
	if r, err := o.ClearSand(); err != nil || r.(map[string]any)["pieces"].(int64) != 5 {
		t.Fatalf("%v %v", r, err)
	}
	if s := mustOne(t, o, `select sum(sand_buildup) s from building_instances`)["s"].(int64); s != 0 {
		t.Fatalf("sand left: %d", s)
	}
	before := structureRowsDump(t, o)
	if _, err := o.RepairBuildings(); err != nil {
		t.Fatal(err)
	}
	if structureRowsDump(t, o) == before {
		t.Fatal("the save-wide repair changed nothing")
	}
	if h := healths(t, o, `select health h from building_instances where building_id=803 order by instance_id`); h[0] != 5600 || h[1] != 7000 {
		t.Fatalf("unowned pieces: %v", h)
	}
}

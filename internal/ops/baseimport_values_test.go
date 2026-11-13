package ops

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Audit D3-1 (#613): a base file whose numbers are out of range is refused whole, with the reason, and nothing is written. `refuses` (baseimport_test.go) runs both the preview and the
// import and checks that the save is exactly as it was, nothing queued and nothing dirty, after each. The fixture is the made-up pick-up of baseexport_test.go: inventory 7001 has 20
// slots and holds items 9001 (slot 0) and 9002 (slot 1); inventory 7002 has 5.
func TestImportRefusesValuesOutOfRange(t *testing.T) {
	good := exportedFile(t)
	if n, known := catalog.MaxStack("SteelBar"); !known || n != 500 {
		t.Fatalf("the test needs SteelBar to be known with a stack of 500: %d %v", n, known)
	}
	num := func(s string) json.Number { return json.Number(s) }
	item := func(f func(r map[string]any)) func() []byte {
		return func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { f(rowWhere(tb, "items", "id", 9001)) })
		}
	}
	cases := []struct {
		name string
		data func() []byte
		want string
	}{
		{"a stack of 9e18", item(func(r map[string]any) { r["stack_size"] = num("9000000000000000000") }), "is more than one slot holds"},
		{"a stack above the item's stack size", item(func(r map[string]any) { r["template_id"] = "SteelBar"; r["stack_size"] = jnum(501) }), "a stack of 501 SteelBar is more than one slot holds (500)"},
		{"a stack of an item that does not stack", item(func(r map[string]any) { r["template_id"] = "HighCapacityLiterjon_06"; r["stack_size"] = jnum(2) }), "is more than one slot holds (1)"},
		{"a stack of an item the catalog does not know", item(func(r map[string]any) { r["template_id"] = "NoSuchItemInTheCatalog"; r["stack_size"] = jnum(basefile.MaxUnknownStack + 1) }), "is more than one slot holds (100000)"},
		{"a negative stack", item(func(r map[string]any) { r["stack_size"] = jnum(-3) }), "stack size"},
		{"two items in one slot", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "items", "id", 9002)["position_index"] = jnum(0) })
		}, "slot 0 of inventory 7001 already holds another item"},
		{"a slot at the end of the inventory", item(func(r map[string]any) { r["position_index"] = jnum(20) }), "slot 20 is outside inventory 7001, which has 20 slots"},
		{"a slot far outside", item(func(r map[string]any) { r["position_index"] = num("9000000000000000000") }), "outside inventory 7001"},
		{"a slot far outside an inventory with no slot count", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "inventories", "id", 7001)["max_item_count"] = nil
				rowWhere(tb, "items", "id", 9001)["position_index"] = jnum(basefile.MaxSlot + 1)
			})
		}, "above the highest slot allowed"},
		{"a negative slot count", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "inventories", "id", 7001)["max_item_count"] = jnum(-1) })
		}, "slot count of -1"},
		{"a piece with a health of 1e300", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { tb["building_instances"][0]["health"] = num("1e300") })
		}, "the health of this building piece is 1e+300"},
		{"a piece with a health just above the ceiling", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { tb["building_instances"][0]["health"] = num("1000000.5") })
		}, "health of this building piece"},
		{"a piece with a negative health", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { tb["building_instances"][1]["health"] = num("-1") })
		}, "health of this building piece is -1"},
		{"a piece with a health that is not finite", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { tb["building_instances"][0]["health"] = num("1e999") })
		}, "health"},
		{"a placeable with a health of 1e300", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "placeables", "id", 701)["health"] = num("1e300") })
		}, "the health of this placeable is 1e+300"},
		{"a placeable with a negative health", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "placeables", "id", 701)["health"] = num("-0.5") })
		}, "health of this placeable"},
		{"a durability of 1e300", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "items", "id", 9002)["stats"] = map[string]any{"FItemStackAndDurabilityStats": []any{[]any{}, map[string]any{"CurrentDurability": num("1e300")}}}
			})
		}, "CurrentDurability is 1e+300"},
		{"a negative maximum durability", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "items", "id", 9002)["stats"] = map[string]any{"FItemStackAndDurabilityStats": []any{[]any{}, map[string]any{"MaxDurability": num("-5")}}}
			})
		}, "MaxDurability is -5"},
		{"a durability that does not fit a number", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "items", "id", 9002)["stats"] = map[string]any{"FItemStackAndDurabilityStats": []any{[]any{}, map[string]any{"DecayedMaxDurability": num("1e999")}}}
			})
		}, "DecayedMaxDurability is a number the editor cannot hold"},
		{"text longer than the limit", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "permission_actor", "actor_id", 701)["actor_name"] = strings.Repeat("a", basefile.MaxText+1)
			})
		}, "longer than 1000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			refuses(t, target(t), c.data(), c.want)
		})
	}
}

// The limits themselves are allowed, and a file as the editor exports it still imports: the checks refuse what is out of range, not what is at the edge.
func TestImportAcceptsValuesAtTheLimits(t *testing.T) {
	good := exportedFile(t)
	edge := editFile(t, good, func(tb map[string][]map[string]any) {
		r := rowWhere(tb, "items", "id", 9001)
		r["template_id"], r["stack_size"], r["position_index"] = "SteelBar", jnum(500), jnum(19) // the last slot of 20, a full stack
		tb["building_instances"][0]["health"] = json.Number("1000000")
		tb["building_instances"][1]["health"] = json.Number("0")
		rowWhere(tb, "placeables", "id", 701)["health"] = json.Number("1000000")
		rowWhere(tb, "items", "id", 9002)["stats"] = map[string]any{"FItemStackAndDurabilityStats": []any{[]any{}, map[string]any{"CurrentDurability": json.Number("0"), "MaxDurability": json.Number("1000000")}}}
	})
	o := target(t)
	if _, err := o.PreviewBaseImport(edge); err != nil {
		t.Fatalf("a file at the limits was refused: %v", err)
	}
	mustImport(t, o, edge)
	if n := cnt(t, o, `select count(*) from base_backups`); n != 1 {
		t.Fatalf("backups: %d", n)
	}
	// the export of the game's own tool, untouched, imports as it did
	mustImport(t, target(t), good)
}

// Repair buildings and Repair base copy the best health seen for a type onto the others. A health outside 0 to MaxPieceHealth is never "the best seen" and never copied, so a piece
// that got one (a file imported before the check, a hand edit) cannot spread it to the player's own bases; the pieces with a sane health are raised to the best sane one.
const rangeWorld = `
insert into actors(id,class,map) values (601,'t','HaggaBasin'),(801,'b','HaggaBasin'),(802,'b','HaggaBasin'),(851,'p','HaggaBasin'),(852,'p','HaggaBasin'),(853,'p','HaggaBasin'),(854,'p','HaggaBasin');
insert into totems(id) values (601);
insert into fgl_entities(entity_id,components) values (-601, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'Actor');
insert into building_instances(building_id,instance_id,building_type,health,owner_entity_id) values
  (801,1,'Foundation',7000.0,-601),(801,2,'Foundation',3500.0,-601),(801,3,'Foundation',1e300,NULL),(801,4,'Foundation',2000000.0,NULL),
  (802,1,'Wall',1000.0,-601),(802,2,'Wall',1e300,NULL),(802,3,'Wall',1e300,-601),
  (802,4,'Roof',-5.0,-601),(802,5,'Roof',100.0,-601);
insert into placeables(id,health,building_type,owner_entity_id) values
  (851,1250.0,'Cistern',-601),(852,300.0,'Cistern',-601),(853,1e300,'Cistern',NULL),(854,40.0,'Lamp',-601);`

func rangeOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, rangeWorld)} }

func TestRepairBuildingsNeverCopiesAHealthOutOfRange(t *testing.T) {
	o := rangeOps(t)
	if _, err := o.RepairBuildings(); err != nil {
		t.Fatal(err)
	}
	got := healths(t, o, `select health h from building_instances order by building_id, instance_id`)
	// Foundation: the best sane one is 7000 (not the 1e300 or the 2,000,000), which goes to the 3500; the two out of range stay as they were. Wall: 1000 is the only sane one, so the
	// two 1e300 are not touched and nothing is raised to them. Roof: the only sane health is 100, so the -5 is raised to it.
	want := []float64{7000, 7000, 1e300, 2000000, 1000, 1e300, 1e300, 100, 100}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("building healths %v, want %v", got, want)
		}
	}
	pl := healths(t, o, `select health h from placeables order by id`)
	if pl[0] != 1250 || pl[1] != 1250 || pl[2] != 1e300 || pl[3] != 40 { // the 300 Cistern is raised to 1250 and never to 1e300
		t.Fatalf("placeable healths %v", pl)
	}
}

func TestRepairBaseNeverCopiesAHealthOutOfRange(t *testing.T) {
	o := rangeOps(t)
	if _, err := o.RepairBase(601); err != nil {
		t.Fatal(err)
	}
	got := healths(t, o, `select health h from building_instances order by building_id, instance_id`)
	// only the pieces owned by base 601 are written: the 3500 Foundation (to 7000), and the -5 Roof (to 100); the 1e300 Wall the base owns is not "the best" and is left as it is
	want := []float64{7000, 7000, 1e300, 2000000, 1000, 1e300, 1e300, 100, 100}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("building healths %v, want %v", got, want)
		}
	}
	pl := healths(t, o, `select health h from placeables order by id`)
	if pl[0] != 1250 || pl[1] != 1250 || pl[2] != 1e300 || pl[3] != 40 {
		t.Fatalf("placeable healths %v", pl)
	}
}

package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// A test against a real save. TAU_REAL_SAVE_SQLITE names a COPY of a save the game wrote, already decoded to a plain SQLite file (never a live save: the test writes into it, after
// copying it to a temp folder). Without it the test is skipped, so the suite does not depend on any save being present. It composes vehicles on the copy and checks what the fixtures
// of the other tests can only imitate: the foreign keys of the whole database, the integrity of the file, the numbers derived from the modules written, that no existing vehicle
// changed, and that the vehicle built for the class and tier of a real vehicle has that vehicle's module set.
func realSaveCopy(t *testing.T) *Ops {
	src := os.Getenv("TAU_REAL_SAVE_SQLITE")
	if src == "" {
		t.Skip("TAU_REAL_SAVE_SQLITE is not set: no real save copy to compose vehicles on")
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := save.Encode(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "game.db")
	if err := os.WriteFile(p, blob, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := save.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return &Ops{S: s}
}

func allVehicleIDs(t *testing.T, o *Ops) []int64 {
	rows, err := o.S.Query(`select id from vehicles order by id`)
	if err != nil {
		t.Fatal(err)
	}
	var out []int64
	for _, r := range rows {
		out = append(out, r["id"].(int64))
	}
	return out
}

func fkTotal(t *testing.T, o *Ops) int {
	rows, err := o.S.Query(`pragma foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	return len(rows)
}

func TestComposeVehiclesOnACopyOfARealSave(t *testing.T) {
	o := realSaveCopy(t)
	existing := allVehicleIDs(t, o)
	before := vehicleRowsDump(t, o, existing...)
	fkBefore := fkTotal(t, o)
	var made []int64
	for name, a := range map[string]Args{
		"light T4_Inventory":  {"class": "OrnithopterLight", "template": "T4_Inventory"},
		"buggy T5_Mining":     {"class": "Buggy", "template": "T5_Mining"},
		"buggy T6_Combat":     {"class": "Buggy", "template": "T6_Combat"},
		"medium T5_Inventory": {"class": "OrnithopterMedium", "template": "T5_Inventory"},
		"carrier T6_Boost":    {"class": "OrnithopterTransport", "template": "T6_Boost"},
		"crawler T6":          {"class": "Sandcrawler", "template": "T6_Harvesting"},
		"sandbike T5":         {"class": "Sandbike", "template": "T5"},
		"container":           {"class": "ContainerVehicle", "template": "Container"},
		"operator's Mk6 buggy": {"class": "Buggy", "template": "T6_Combat", "uniques": map[string]any{
			"Engine": "BuggyEngine_Unique_Accelerate_06", "UtilitySlot1": "BuggyInventory_Unique_Capacity_06", "UtilitySlot2": "BuggyMining_Unique_YieldIncrease_06"},
			"items": []any{map[string]any{"template_id": "MelangeSpice", "quantity": 25.0}}},
	} {
		nBefore := mustOne(t, o, `select (select count(*) from actors) a, (select count(*) from vehicle_modules) m, (select count(*) from fgl_entities) e, (select count(*) from inventories) i`)
		pr, err := o.SpawnVehiclePreview(a)
		if err != nil {
			t.Fatalf("%s: preview: %v", name, err)
		}
		if nAfter := mustOne(t, o, `select (select count(*) from actors) a, (select count(*) from vehicle_modules) m, (select count(*) from fgl_entities) e, (select count(*) from inventories) i`); fmt.Sprint(nAfter) != fmt.Sprint(nBefore) {
			t.Fatalf("%s: a preview changed the save: %v -> %v", name, nBefore, nAfter)
		}
		id, res := spawn(t, o, a)
		if id != pr.(map[string]any)["id"] {
			t.Errorf("%s: the apply took id %d, the preview named %v", name, id, pr.(map[string]any)["id"])
		}
		made = append(made, id)
		t.Logf("%s -> vehicle %d, %v modules, derived %v", name, id, len(viewOf(t, o, id).modules), res["derived"])
	}
	if r := mustOne(t, o, `pragma integrity_check`); r["integrity_check"] != "ok" {
		t.Errorf("integrity: %v", r)
	}
	if n := fkTotal(t, o); n != fkBefore {
		t.Errorf("foreign key violations: %d before, %d after", fkBefore, n)
	}
	rest := vehicleRowsDump(t, o, existing...)
	if rest != before {
		t.Error("an existing vehicle changed")
	}
	if got := len(allVehicleIDs(t, o)); got != len(existing)+len(made) {
		t.Errorf("%d vehicles, want %d", got, len(existing)+len(made))
	}
	// the entity ids are all distinct in the whole file
	if n := mustOne(t, o, `select count(*) c from fgl_entities`)["c"].(int64); n < int64(2*len(made)) {
		t.Errorf("%d entities", n)
	}
}

// Each real vehicle of the copy whose module set is exactly a template's module set is reported; the world's grade 0 vehicles and the character's plain-template vehicles are.
func TestTemplateModuleSetsMatchTheRealVehiclesOfACopy(t *testing.T) {
	o := realSaveCopy(t)
	rows, err := o.S.Query(`select a.id, a.class from vehicles v join actors a on a.id=v.id order by a.id`)
	if err != nil {
		t.Fatal(err)
	}
	checked, matched := 0, 0
	byTemplate := map[string]int{}
	for _, r := range rows {
		cls, ok := vehicleClassKeyOf(r["class"])
		if !ok {
			continue
		}
		real, _ := o.S.Query(`select template_id t from vehicle_modules where vehicle_id=?`, r["id"])
		var ids []string
		for _, m := range real {
			ids = append(ids, m["t"].(string))
		}
		if len(ids) == 0 {
			continue
		}
		checked++
		c, _ := catalog.VehicleClassFor(cls)
		for _, tpl := range c.Templates {
			var tids []string
			for _, id := range tpl.Modules {
				tids = append(tids, id)
			}
			if sameSet(tids, ids) {
				matched++
				byTemplate[cls+"/"+tpl.Name]++
				break
			}
		}
	}
	t.Logf("%d vehicles with modules, %d are exactly a template of the catalog: %v", checked, matched, byTemplate)
	if checked > 0 && matched == 0 {
		t.Error("no real vehicle is exactly a template")
	}
}

func vehicleClassKeyOf(class any) (string, bool) {
	for _, key := range catalog.VehicleClassKeys() {
		c, _ := catalog.VehicleClassFor(key)
		if blueprintClass(c.ActorClass) == blueprintClass(class) {
			return key, true
		}
	}
	return "", false
}

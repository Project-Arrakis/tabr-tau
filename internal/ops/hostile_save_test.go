package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A save is data from outside. A building with no type recorded (a NULL building_type) must not panic the views that name buildings:
// net/http recovers the panic, but the page then gets a reset connection (SEC-5 of the 2026-10-15 audit).
func TestBaseViewsSurviveABuildingWithNoType(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, baseViewWorld+`
insert into actors(id,class,map,location_x,location_y,location_z) values (712,'p','HaggaBasin',0,0,0);
insert into fgl_entities(entity_id,components) values (-712, jsonb('{"FWaterStorageComponent":[[],{"m_WaterStored":123}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (712,-712,'Water');
insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id,is_hologram) values (712,-601,NULL,1,0);`)}
	for name, call := range map[string]func() (any, error){
		"water":     func() (any, error) { return o.BaseWater(601) },
		"power":     func() (any, error) { return o.BasePower(601) },
		"inventory": func() (any, error) { return o.BaseInventory(601) },
		"bases":     func() (any, error) { return o.BaseList() },
	} {
		if _, err := call(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	r, _ := o.BaseWater(601)
	var stored int64
	for _, ty := range r.(map[string]any)["types"].([]map[string]any) {
		stored += ty["stored"].(int64)
	}
	if stored != 5000+10000 {
		t.Errorf("only the typed water buildings count (the base's cistern and medium cistern), got %d stored", stored)
	}
}

// A column name comes from the save as well, and a spreadsheet runs a cell that starts with = as a formula (SEC-4).
func TestCSVExportNeutralizesFormulaColumnNames(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `create table "csvhead" ("=HYPERLINK(""http://example.invalid/"",""x"")" text, "+cmd" integer, plain text);
insert into csvhead values ('a', 1, 'b');`)}
	b, _, err := o.Export("csvhead", "csv")
	if err != nil {
		t.Fatal(err)
	}
	head := strings.SplitN(string(b), "\n", 2)[0]
	if strings.HasPrefix(head, "=") || strings.Contains(head, ",=") || strings.Contains(head, ",+") || strings.HasPrefix(head, "+") {
		t.Fatalf("a header cell that starts a formula must be neutralized: %s", head)
	}
	if !strings.Contains(head, "'=HYPERLINK") || !strings.Contains(head, "'+cmd") || !strings.HasSuffix(head, ",plain") {
		t.Fatalf("header: %s", head)
	}
}

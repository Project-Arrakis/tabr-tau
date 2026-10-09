package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

const catalogWorld = `
insert into building_progression_learned_building_sets(character_id,learned_building_set) values (1,'BasicLighting_Patent'),(1,'MTX_Store_Pack_Patent');
insert into building_progression_new_buildable_pieces(character_id,new_buildable_piece) values (1,'Some_Placeable');
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (41,2,1,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (401,41,1,0,'AtreidesSet',0,1790000000,'{}',0),(402,41,1,1,'HarkSandbike_MeshCustomization',0,1790000000,'{}',0);`

func TestBuildingSetsShowLearnedAndInventoryAndStorePacks(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, catalogWorld)}
	r, err := o.BuildingSets()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	by := map[string]CatalogRow{}
	for _, x := range m["rows"].([]CatalogRow) {
		by[x.ID] = x
	}
	if b := by["BasicLighting_Patent"]; !b.Learned || b.InInventory || !b.InCatalog || b.Name != "Basic Lighting" {
		t.Fatalf("a learned patent: %+v", b)
	}
	if a := by["AtreidesSet"]; a.Learned || !a.InInventory || !a.InCatalog {
		t.Fatalf("a set in the inventory: %+v", a)
	}
	if s := by["MTX_Store_Pack_Patent"]; !s.Learned || s.InCatalog || s.Name == "" {
		t.Fatalf("a learned set the catalog does not list is still shown: %+v", s)
	}
	if by["HarkonnenSet"].Learned || by["HarkonnenSet"].InInventory {
		t.Fatalf("an unowned set: %+v", by["HarkonnenSet"])
	}
	if _, ok := by["HarkSandbike_MeshCustomization"]; ok {
		t.Fatal("customizations are not building sets")
	}
	if len(m["newPieces"].([]map[string]any)) != 1 {
		t.Fatalf("new pieces: %v", m["newPieces"])
	}
}

func TestCustomizationsShowInventoryPresence(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, catalogWorld)}
	r, err := o.Customizations()
	if err != nil {
		t.Fatal(err)
	}
	rows := r.(map[string]any)["rows"].([]CatalogRow)
	if len(rows) < 300 {
		t.Fatalf("the catalog's customizations are listed: %d", len(rows))
	}
	var have, other int
	for _, x := range rows {
		if x.InInventory {
			have++
			if x.ID != "HarkSandbike_MeshCustomization" {
				t.Fatalf("unexpected inventory flag on %s", x.ID)
			}
		} else {
			other++
		}
	}
	if have != 1 {
		t.Fatalf("exactly the one customization item in the inventory is flagged: %d", have)
	}
}

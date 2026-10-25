package ops

import (
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #406: places are told by region name. A thing is in the region of the nearest map marker that has a named area; the names are the
// Hagga Basin sub-regions of the console's hagga-regions.json, keyed by the same area id the save's markers carry.
const regionWorld = `
insert into actors(id,class,map,location_x,location_y,location_z) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',9000,9500,0),
  (201,'/Game/BP_Sandbike.BP_Sandbike_C','HaggaBasin',150,120,0),
  (202,'/Game/BP_Buggy.BP_Buggy_C','DeepDesert',100,100,0);
insert into totems(id) values (601);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (201,'mine',2,3,0),(202,'mine too',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,1,1),(202,1,1);
insert into vehicles(id) values (201),(202);
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,area_id) values
  (1,0,'HaggaBasin','Cave',0,0,0,3),
  (2,0,'HaggaBasin','Cave',10000,10000,0,12),
  (3,0,'HaggaBasin','Cave',100,150,0,0),
  (4,0,'HaggaBasin','Cave',9000,9400,0,99),
  (5,0,'DeepDesert','Cave',100,100,0,3);
insert into map_areas(character_id,area_id,time_discovered,time_first_entered,map) values
  (1,3,100,100,'HaggaBasin'),(1,12,200,200,'HaggaBasin'),(1,99,300,300,'HaggaBasin'),(1,3,400,400,'DeepDesert');`

func TestTheRegionNamesAreTheTwelveOfTheHaggaBasin(t *testing.T) {
	names := regionNamesOf("HaggaBasin")
	if len(names) != 12 {
		t.Fatalf("%d names: %v", len(names), names)
	}
	for id, want := range map[int64]string{3: "Hagga Rift", 12: "Hagga Basin South", 16: "Jabal Eifrit Al-gharb", 6: "The O'odham"} {
		if names[id] != want {
			t.Errorf("area %d: %q, want %q", id, names[id], want)
		}
	}
	if regionNamesOf("DeepDesert") != nil || regionNamesOf("nonsense") != nil {
		t.Error("only the Hagga Basin has region names")
	}
}

func TestThePlacesOnTheSavedMapHaveARegion(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, regionWorld)}
	r, err := o.LiveMap("HaggaBasin")
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	// the character (100,200) and the vehicle (150,120) are nearest to the marker of area 3 (0,0); the marker of area 0 at (100,150) has no area, and is not used
	if c := m["character"].(map[string]any); c["region"] != "Hagga Rift" {
		t.Errorf("character: %v", c)
	}
	if v := m["vehicles"].([]map[string]any); len(v) != 1 || v[0]["region"] != "Hagga Rift" {
		t.Errorf("vehicles: %v", v)
	}
	// the base (9000,9500) is nearest to the marker of area 99 (9000,9400), which has no name, so it is skipped for the marker of area 12
	if b := m["bases"].([]map[string]any); len(b) != 1 || b[0]["region"] != "Hagga Basin South" {
		t.Errorf("bases: %v", b)
	}
}

func TestTheDeepDesertHasNoRegionNames(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, regionWorld)}
	r, _ := o.LiveMap("DeepDesert")
	m := r.(map[string]any)
	for _, v := range m["vehicles"].([]map[string]any) {
		if _, has := v["region"]; has {
			t.Errorf("a vehicle on the Deep Desert has no region name: %v", v)
		}
	}
	if a := m["areas"].(map[string]any); a["named"] != false {
		t.Errorf("areas: %v", a)
	}
}

func TestTheDiscoveredAreasAreListedByName(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, regionWorld)}
	r, _ := o.LiveMap("HaggaBasin")
	a := r.(map[string]any)["areas"].(map[string]any)
	if a["named"] != true || a["total"].(int) != 12 {
		t.Fatalf("areas: %v", a)
	}
	got := strings.Join(a["discovered"].([]string), ", ")
	if got != "Hagga Basin South, Hagga Rift" {
		t.Errorf("discovered: %q (sorted by name, the other map's row and the unnamed area 99 left out)", got)
	}
	if a["other"].(int) != 1 {
		t.Errorf("one discovered area has no name: %v", a)
	}
}

func TestTheVehicleAndBaseListsHaveARegion(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, regionWorld)}
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	by := map[int64]map[string]any{}
	for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
		by[v["id"].(int64)] = v
	}
	if by[201]["region"] != "Hagga Rift" {
		t.Errorf("vehicle 201: %v", by[201])
	}
	if _, has := by[202]["region"]; has {
		t.Errorf("a Deep Desert vehicle has no region: %v", by[202])
	}
	bl, err := o.BaseList()
	if err != nil {
		t.Fatal(err)
	}
	if b := bl.([]map[string]any); len(b) != 1 || b[0]["region"] != "Hagga Basin South" {
		t.Errorf("bases: %v", b)
	}
}

// A save with no markers, or with markers that have no area, gives no region and no error.
func TestNoMarkersMeansNoRegion(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into actors(id,class,map,location_x,location_y,location_z) values (201,'/Game/BP_Sandbike.BP_Sandbike_C','HaggaBasin',1,1,0);
		insert into vehicles(id) values (201);
		insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (201,'mine',2,3,0);
		insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,1,1);`)}
	r, err := o.Vehicles()
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range r.(map[string]any)["vehicles"].([]map[string]any) {
		if _, has := v["region"]; has {
			t.Errorf("no region without markers: %v", v)
		}
	}
}

// The data file is the console's, unchanged, and has its notice.
func TestTheRegionFileHasItsNotice(t *testing.T) {
	b, err := os.ReadFile("../notices/THIRD-PARTY-NOTICES.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"internal/ops/hagga-regions.json", "hagga-regions.json", "82548d72"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the notices do not say %q", want)
		}
	}
}

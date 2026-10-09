package ops

import (
	"strconv"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The player (pawn 2, on HaggaBasin at 100,200,300), a base, an owned vehicle and a world vehicle, one storage chest, a hologram
// chest, and markers with and without discovery. One marker is on another map.
const mapWorld = `
insert into actors(id,class,map,location_x,location_y,location_z) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,0),
  (701,'p','HaggaBasin',5100,6100,0),(702,'p','HaggaBasin',5200,6200,0),
  (201,'/Game/BP_Sandbike.BP_Sandbike_C','HaggaBasin',7000,8000,0),
  (202,'/Game/BP_Sandbike.BP_Sandbike_C','HaggaBasin',9000,9000,0);
insert into totems(id) values (601);
insert into fgl_entities(entity_id,components) values (-601, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,-601,'Actor');
insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id,is_hologram) values
  (701,-601,'GenericContainer_Placeable',1,0),(702,-601,'GenericContainer_Placeable',1,1);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (701,'Ore Out',1,3,1),(201,'mine',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (201,1,1);
insert into vehicles(id) values (201),(202);
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values
  (1,0,'HaggaBasin','AzuritePickup',-1000,2000,0),
  (2,0,'HaggaBasin','EnemyCamp',3000,4000,0),
  (3,0,'DeepDesert','Cave',5,5,0);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,1,0,'HaggaBasin',3,1);`

func TestLiveMapShowsTheSavesOwnThingsOnTheirMap(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, mapWorld)}
	r, err := o.LiveMap("HaggaBasin")
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	cfg := m["config"].(mapConfig)
	if cfg.Key != "HaggaBasin" || cfg.Image != "/maps/hagga-basin.png" {
		t.Fatalf("config: %+v", cfg)
	}
	ch := m["character"].(map[string]any)
	if ch["x"].(float64) != 100 || ch["y"].(float64) != 200 || ch["name"] != "Tester" {
		t.Fatalf("character: %v", ch)
	}
	if v := m["vehicles"].([]map[string]any); len(v) != 1 || v[0]["id"].(int64) != 201 || v[0]["name"] != "BP_Sandbike" {
		t.Fatalf("only the player's own vehicle is on the map: %v", v)
	}
	if b := m["bases"].([]map[string]any); len(b) != 1 || b[0]["x"].(float64) != 5000 {
		t.Fatalf("bases: %v", b)
	}
	st := m["storage"].([]map[string]any)
	if len(st) != 1 || st[0]["id"].(int64) != 701 || st[0]["name"] != "Ore Out" {
		t.Fatalf("one named chest, no hologram: %v", st)
	}
	mk := m["markers"].([]map[string]any)
	if len(mk) != 2 {
		t.Fatalf("two Hagga Basin markers (the other map's is not shown): %v", mk)
	}
	for _, x := range mk {
		switch x["t"] {
		case "AzuritePickup":
			if x["d"].(int64) != 3 {
				t.Errorf("discovered marker: %v", x)
			}
		case "EnemyCamp":
			if x["d"].(int64) != 0 {
				t.Errorf("undiscovered marker: %v", x)
			}
		default:
			t.Errorf("unexpected marker %v", x)
		}
	}
}

func TestLiveMapOtherMapAndUnknownKey(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, mapWorld)}
	r, err := o.LiveMap("DeepDesert")
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if mk := m["markers"].([]map[string]any); len(mk) != 1 || mk[0]["t"] != "Cave" {
		t.Fatalf("deep desert markers: %v", mk)
	}
	if m["character"] != nil {
		t.Fatalf("the character is not on the Deep Desert: %v", m["character"])
	}
	r, _ = o.LiveMap("nonsense")
	if r.(map[string]any)["config"].(mapConfig).Key != "HaggaBasin" {
		t.Fatal("an unknown map falls back to Hagga Basin")
	}
}

func TestLiveMapWorksWithAnEmptySave(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	if _, err := o.LiveMap("HaggaBasin"); err != nil {
		t.Fatal(err)
	}
}

// packField builds a resource field id the way the game packs one: x, y and z as 21-bit two's-complement fields, low to high.
func packField(x, y, z int64) int64 {
	m := int64(1<<21 - 1)
	return (x & m) | (y&m)<<21 | (z&m)<<42
}

func TestDecodeFieldPositionRoundTripsPositiveAndNegative(t *testing.T) {
	for _, c := range [][3]int64{{0, 0, 0}, {-413944, 195656, 90}, {283763, 118663, 1732}, {-1, -1, -1}, {1048575, -1048576, 5}} {
		x, y, z := decodeFieldPosition(packField(c[0], c[1], c[2]))
		if x != c[0] || y != c[1] || z != c[2] {
			t.Errorf("%v decoded as %d %d %d", c, x, y, z)
		}
	}
}

func TestLiveMapShowsDecodedSpiceAndFlourSandFieldsInsideTheMap(t *testing.T) {
	sql := mapWorld + `
insert into resourcefield_state(field_id,map,dimension_index,spawn_time,value_remaining) values
  (` + strconv.FormatInt(packField(-413944, 195656, 90), 10) + `,'HaggaBasin',0,1.0,5000),
  (` + strconv.FormatInt(packField(283763, 118663, 1732), 10) + `,'HaggaBasin',0,1.0,60000),
  (` + strconv.FormatInt(packField(-240112, -36912, 294), 10) + `,'HaggaBasin',0,1.0,200000),
  (` + strconv.FormatInt(packField(1000000, 1000000, 5), 10) + `,'HaggaBasin',0,1.0,60000),
  (` + strconv.FormatInt(packField(10, 10, 5), 10) + `,'DeepDesert',0,1.0,60000);`
	o := &Ops{S: testsave.PlayerWithSQL(t, sql)}
	r, err := o.LiveMap("HaggaBasin")
	if err != nil {
		t.Fatal(err)
	}
	fields := r.(map[string]any)["resourceFields"].([]map[string]any)
	if len(fields) != 3 {
		t.Fatalf("a field outside the map's bounds and one on another map are not shown: %v", fields)
	}
	by := map[string]map[string]any{}
	for _, f := range fields {
		by[f["kind"].(string)+"/"+f["size"].(string)] = f
	}
	if f := by["spice/Small"]; f == nil || f["x"].(int64) != -413944 || f["y"].(int64) != 195656 {
		t.Fatalf("a 5,000 field is Small spice at its decoded place: %v", by)
	}
	if by["spice/Large"] == nil || by["flour/"] == nil {
		t.Fatalf("200,000 is Large spice and 60,000 is flour sand: %v", by)
	}
}

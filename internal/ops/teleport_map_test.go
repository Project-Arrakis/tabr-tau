package ops

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// ---- the map (#67): the Deep Desert as a destination, and the move between maps

// The character stands on HaggaBasin (teleportWorld). The Deep Desert holds a base, a vehicle of the character, a respawn point and two
// markers (one discovered); a dungeon holds a base too. Positions are in each map's own space.
const teleportDeepDesert = `
insert into actors(id,class,map,location_x,location_y,location_z) values
  (701,'/Game/BP_Totem.BP_Totem_C','DeepDesert',-300000,-590000,1000),(702,'/Game/BP_Totem.BP_Totem_C','PitDungeon',5,5,5),
  (811,'/Game/Vehicles/BP_Buggy.BP_Buggy_C','DeepDesert',-310000,-585000,2000),(812,'/Game/Beacon.Beacon_C','DeepDesert',-305000,-587000,10);
insert into totems(id,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z) values
  (701,-300000.0,-590000.0,1000.0),(702,5.0,5.0,5.0);
insert into vehicles(id) values (811);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (811,'mine',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (811,1,1);
insert into player_respawn_locations(id,character_id,"group",locator_actor_id,map,dimension) values (x'11',1,'BaseTotem',812,'DeepDesert',0);
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values
  (11,0,'DeepDesert','SpiceField',-306000.0,-588000.0,40.0),(12,0,'DeepDesert','Cave',-1.0,-1.0,0.0);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,11,0,'DeepDesert',3,1);`

func destinationsOn(t *testing.T, o *Ops, kind, mp string) map[string]any {
	t.Helper()
	res, err := o.Destinations(kind, "", mp)
	if err != nil {
		t.Fatal(err)
	}
	return res.(map[string]any)
}

func TestDestinationsOfAnotherMapAreThatMapsPlacesAndHaveNoDistance(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, teleportWorld+teleportDeepDesert)}
	for kind, want := range map[string]string{"base": "Base #701", "respawn": "BaseTotem", "vehicle": "BP_Buggy #811", "marker": "SpiceField"} {
		r := destinationsOn(t, o, kind, "DeepDesert")
		d := r["destinations"].([]Destination)
		if r["map"] != "DeepDesert" || r["sameMap"] != false || len(d) != 1 || d[0].Label != want {
			t.Errorf("%s on the Deep Desert: %v", kind, r)
		}
		if d[0].Distance != 0 {
			t.Errorf("%s: a distance across maps means nothing and is not given: %v", kind, d[0].Distance)
		}
	}
	// no map given, or the one the character is on: its own map and the distances from the character
	for _, mp := range []string{"", "HaggaBasin"} {
		r := destinationsOn(t, o, "base", mp)
		if r["map"] != "HaggaBasin" || r["sameMap"] != true || r["destinations"].([]Destination)[0].Distance == 0 {
			t.Errorf("map %q: %v", mp, r)
		}
	}
	// the maps the page may offer: the console's two, the one the character is on marked
	maps := destinationsOn(t, o, "base", "")["maps"].([]TravelMap)
	if len(maps) != 2 || maps[0].Key != "HaggaBasin" || !maps[0].Current || maps[1].Key != "DeepDesert" || maps[1].Current {
		t.Errorf("maps: %+v", maps)
	}
}

func TestDestinationsRefuseAMapTheEditorCannotMoveTo(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, teleportWorld+teleportDeepDesert)}
	for _, mp := range []string{"PitDungeon", "Arrakeen", "nowhere", "../x", "deepdesert"} {
		if _, err := o.Destinations("base", "", mp); err == nil {
			t.Errorf("map %q must be refused", mp)
		}
	}
}

func playerActorMaps(t *testing.T, o *Ops) string {
	t.Helper()
	rows, err := o.S.Query(`select id, map, dimension_index d, location_x x, location_y y, location_z z from actors where id in (1,2,3) order by id`)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rows {
		out = append(out, fmt.Sprintf("%v:%v/%v@%v,%v,%v", r["id"], r["map"], r["d"], r["x"], r["y"], r["z"]))
	}
	return strings.Join(out, " ")
}

func TestTeleportToAnotherMapMovesAllThreePlayerActorsAsTheGameKeepsThem(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, teleportWorld+teleportDeepDesert)}
	if _, err := o.Teleport(Args{"x": -300000.0, "y": -590000.0, "z": 1500.0, "map": "DeepDesert", "yaw": 90.0}); err != nil {
		t.Fatal(err)
	}
	want := "1:DeepDesert/0@-300000,-590000,1500 2:DeepDesert/0@-300000,-590000,1500 3:DeepDesert/0@-300000,-590000,1500"
	if got := playerActorMaps(t, o); got != want {
		t.Fatalf("the controller, the body and the player state of a save the game wrote share a map and a position:\n got %s\nwant %s", got, want)
	}
	r := mustOne(t, o, `select rotation_z rz, rotation_w rw from actors where id=2`)
	if math.Abs(r["rz"].(float64)-math.Sqrt2/2) > 1e-9 {
		t.Errorf("facing: %v", r)
	}
	// and back again
	if _, err := o.Teleport(Args{"x": 10.0, "y": 20.0, "z": 30.0, "map": "HaggaBasin"}); err != nil {
		t.Fatal(err)
	}
	if got := playerActorMaps(t, o); got != "1:HaggaBasin/0@10,20,30 2:HaggaBasin/0@10,20,30 3:HaggaBasin/0@10,20,30" {
		t.Fatalf("back on Hagga Basin: %s", got)
	}
}

func TestTeleportOnTheSameMapStillMovesOnlyTheBody(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, teleportWorld)}
	for _, a := range []Args{{"x": 5.0, "y": 6.0, "z": 7.0}, {"x": 5.0, "y": 6.0, "z": 7.0, "map": "HaggaBasin"}, {"x": 5.0, "y": 6.0, "z": 7.0, "map": ""}} {
		if _, err := o.Teleport(a); err != nil {
			t.Fatal(err)
		}
		if got := playerActorMaps(t, o); got != "1:HaggaBasin/0@100,200,300 2:HaggaBasin/0@5,6,7 3:HaggaBasin/0@100,200,300" {
			t.Fatalf("%v: only the body moves on its own map, as it did when this was checked in the game: %s", a, got)
		}
	}
}

func TestTeleportToAnotherMapRefusesWhatCannotBeRight(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, teleportWorld+teleportDeepDesert)}
	before := playerActorMaps(t, o)
	for name, a := range map[string]Args{
		"a dungeon":                            {"x": 5.0, "y": 5.0, "z": 5.0, "map": "PitDungeon"},
		"a social hub":                         {"x": 5.0, "y": 5.0, "z": 5.0, "map": "Arrakeen"},
		"a map that does not exist":            {"x": 5.0, "y": 5.0, "z": 5.0, "map": "nowhere"},
		"a map that is not text":               {"x": 5.0, "y": 5.0, "z": 5.0, "map": 7.0},
		"a position off the Deep Desert map":   {"x": 5e6, "y": 0.0, "z": 300.0, "map": "DeepDesert"},
	} {
		if _, err := o.Teleport(a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if o.S.Dirty() || playerActorMaps(t, o) != before {
		t.Fatal("a refused move must change nothing")
	}
	// the other way: a character in the Deep Desert is not sent to Hagga Basin with a Deep Desert position
	o = &Ops{S: testsave.PlayerWithSQL(t, teleportWorld+teleportDeepDesert+`update actors set map='DeepDesert' where id in (1,2,3);`)}
	if _, err := o.Teleport(Args{"x": -300000.0, "y": -590000.0, "z": 1.0, "map": "HaggaBasin"}); err == nil {
		t.Error("Deep Desert coordinates are off the Hagga Basin map")
	}
}

func TestTeleportBetweenMapsNeedsAllThreePlayerActors(t *testing.T) {
	// the player state's row is gone: a half move (the body on another map, the rest on the old one) is worse than none
	o := &Ops{S: testsave.PlayerWithSQL(t, teleportWorld+teleportDeepDesert+`update player_state set player_state_id=NULL where id=1;`)}
	before := playerActorMaps(t, o)
	if _, err := o.Teleport(Args{"x": -300000.0, "y": -590000.0, "z": 1.0, "map": "DeepDesert"}); err == nil {
		t.Fatal("a move between maps without the player state row must be refused")
	}
	if o.S.Dirty() || playerActorMaps(t, o) != before {
		t.Fatal("nothing may change")
	}
}

package ops

import (
	"fmt"
	"math"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The character stands at 100, 200, 300 on HaggaBasin (the Player fixture). Places: base 601 (the character's own, east, 5000), base 602 on another map,
// a respawn point with its own coordinates, one that takes its place from an actor, one with no place at all, an owned vehicle, and
// markers: a trading post (discovered, named by its payload's LocationKey), a camp (discovered; no group of the list offers it), a trading post that is not discovered
// and a taxi hub on another map.
var teleportWorld = teleportWorldBase + `update markers set payload=` + payloadSQL(locPinnacle) + ` where marker_hash_id=1 and map_name='HaggaBasin';`

const teleportWorldBase = `
insert into actors(id,class,map,location_x,location_y,location_z) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7),(602,'/Game/BP_Totem.BP_Totem_C','Arrakeen',0,0,0),
  (801,'/Game/Vehicles/BP_Sandbike.BP_Sandbike_C','HaggaBasin',300,400,50),(802,'/Game/Vehicles/BP_Buggy.BP_Buggy_C','HaggaBasin',9,9,9),
  (803,'/Game/Beacon.Beacon_C','HaggaBasin',1000,1000,10);
insert into totems(id,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z) values
  (601,5000.0,6000.0,7.0),(602,1.0,1.0,1.0);
insert into vehicles(id) values (801),(802);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (601,'##Totem_Placeable',1,3,1),(801,'mine',2,3,0),(802,'theirs',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(801,1,1);
insert into player_respawn_locations(id,character_id,"group",locator_location_x,locator_location_y,locator_location_z,map,dimension) values
  (x'01',1,'Vehicle',150.0,250.0,310.0,'HaggaBasin',0),(x'02',1,'PlayerStart',NULL,NULL,NULL,'HaggaBasin',0);
insert into player_respawn_locations(id,character_id,"group",locator_actor_id,map,dimension) values (x'03',1,'BaseTotem',803,'HaggaBasin',0);
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values
  (1,0,'HaggaBasin','TradingPost',400.0,500.0,20.0),(2,0,'HaggaBasin','EnemyCamp',9000.0,9000.0,30.0),(3,0,'HaggaBasin','TradingPost',100000.0,0.0,0.0),
  (4,0,'Arrakeen','TaxiService',1.0,1.0,1.0);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values
  (1,1,0,'HaggaBasin',3,1),(1,2,0,'HaggaBasin',1,1),(1,4,0,'Arrakeen',3,1);`

// listDestinations is the places of one group (tradepost, base, vehicle, respawn, taxi), or of every group when group is empty, on the character's map.
func listDestinations(t *testing.T, o *Ops, group, q string) []Destination {
	t.Helper()
	res, err := o.Destinations(q, "")
	if err != nil {
		t.Fatal(err)
	}
	all := res.(map[string]any)["destinations"].([]Destination)
	if group == "" {
		return all
	}
	var out []Destination
	for _, d := range all {
		if d.Group == group {
			out = append(out, d)
		}
	}
	return out
}

func TestDestinationsListPlacesOnTheCurrentMapNearestFirst(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, teleportWorld)}

	bases := listDestinations(t, o, groupBase, "")
	if len(bases) != 1 || bases[0].Label != "Base #601" || bases[0].X != 5000 || bases[0].Z != 7 {
		t.Fatalf("bases (the one on another map is left out): %+v", bases)
	}
	respawns := listDestinations(t, o, groupRespawn, "")
	if len(respawns) != 2 {
		t.Fatalf("respawns (the one with no place is left out): %+v", respawns)
	}
	if respawns[0].Label != "Respawn point (Vehicle)" || respawns[0].Z != 310 || respawns[1].Label != "Respawn point #803" || respawns[1].X != 1000 {
		t.Fatalf("respawns, nearest first, one placed by its actor: %+v", respawns)
	}
	vehicles := listDestinations(t, o, groupVehicle, "")
	if len(vehicles) != 1 || vehicles[0].Label != "mine #801" {
		t.Fatalf("only the character's own vehicle: %+v", vehicles)
	}
	posts := listDestinations(t, o, groupTradePost, "")
	if len(posts) != 1 || posts[0].Label != "Pinnacle Station Tradepost" {
		t.Fatalf("the trading post by its name (not the camp or the post never discovered): %+v", posts)
	}
	all := listDestinations(t, o, "", "")
	var order []string
	for _, d := range all {
		order = append(order, d.Group)
	}
	if fmt.Sprint(order) != "[tradepost base vehicle respawn respawn taxi taxi]" { // the two taxi rows are the hubs, offered from every map (#521)
		t.Fatalf("one list, the groups in the page's order: %v", labelsOf(all))
	}
	if got := listDestinations(t, o, "", "camp"); len(got) != 0 {
		t.Fatalf("a camp is not a place: %+v", got)
	}
	if got := listDestinations(t, o, "", "pinn"); len(got) != 1 || got[0].Label != "Pinnacle Station Tradepost" {
		t.Fatalf("the filter narrows by name: %+v", got)
	}
	// #515: the trading post has an arrival point, so the place is that point (not the marker at 400, 500) and the distance shown beside it is the distance to that point
	if posts[0].X != -32905.3 || posts[0].Y != -313888.6 || posts[0].Z != 12053 || !posts[0].Arrival {
		t.Fatalf("the trading post is at the game's arrival point: %+v", posts[0])
	}
	if want := math.Hypot(-32905.3-100, -313888.6-200); math.Abs(posts[0].Distance-want) > 1e-6 {
		t.Fatalf("distance is flat, from the character to what will be used (the arrival point): %v want %v", posts[0].Distance, want)
	}
	if vehicles[0].Arrival || vehicles[0].X != 300 || vehicles[0].Y != 400 {
		t.Fatalf("a vehicle keeps its own position: %+v", vehicles[0])
	}
}

func TestTeleportMovesAndOnlyTurnsWhenAskedTo(t *testing.T) {
	o := newPlayerOps(t)
	before, _ := o.S.One(`select rotation_x rx, rotation_y ry, rotation_z rz, rotation_w rw from actors where id=2`)
	if _, err := o.Teleport(Args{"x": 1000.0, "y": -2000.0, "z": 3000.0}); err != nil {
		t.Fatal(err)
	}
	after, _ := o.S.One(`select location_x x, location_y y, location_z z, rotation_x rx, rotation_y ry, rotation_z rz, rotation_w rw from actors where id=2`)
	if after["x"] != 1000.0 || after["y"] != -2000.0 || after["z"] != 3000.0 {
		t.Fatalf("position: %v", after)
	}
	if after["rx"] != before["rx"] || after["rz"] != before["rz"] || after["rw"] != before["rw"] {
		t.Fatalf("without a facing the rotation stays as it was: %v vs %v", after, before)
	}

	if _, err := o.Teleport(Args{"x": 1.0, "y": 2.0, "z": 3.0, "yaw": 90.0}); err != nil {
		t.Fatal(err)
	}
	r, _ := o.S.One(`select rotation_x rx, rotation_y ry, rotation_z rz, rotation_w rw from actors where id=2`)
	s := math.Sqrt2 / 2
	for k, want := range map[string]float64{"rx": 0, "ry": 0, "rz": s, "rw": s} {
		if got, _ := toFloat(r[k]); math.Abs(got-want) > 1e-9 {
			t.Errorf("%s = %v, want %v (a quarter turn about the vertical axis)", k, got, want)
		}
	}
}

func TestTeleportRefusesNonsense(t *testing.T) {
	o := newPlayerOps(t)
	for _, a := range []Args{
		{"x": 1.0, "y": 2.0},                               // z missing
		{"x": math.NaN(), "y": 2.0, "z": 3.0},              // not a number
		{"x": 1e9, "y": 2.0, "z": 3.0},                     // off the map by a long way
		{"x": 1.0, "y": 2.0, "z": 3.0, "yaw": "sideways"},  // facing is not a number
		{"x": 1.0, "y": 2.0, "z": 3.0, "yaw": math.Inf(1)}, // nor is this
	} {
		if _, err := o.Teleport(a); err == nil {
			t.Errorf("%v must be refused", a)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused teleport must leave no pending change")
	}
}

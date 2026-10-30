package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #519: Go to a place is one list. These tests fix what each group holds. The character stands at 100, 200, 300 on HaggaBasin (the Player fixture) and is player 1.

func groupLabels(t *testing.T, o *Ops, group string) []string {
	t.Helper()
	var out []string
	for _, d := range listDestinations(t, o, group, "") {
		out = append(out, d.Label)
	}
	return out
}

// basesWorld: a deployed base of the character (601), a base the game's tool picked up that still has the character's owner record (602: state 5, linked in a base backup),
// a base that was picked up and claimed again (603: its backup rows remain, its actors stand in the world again), another player's base (604), a base the character is only a member
// of (605), a deployed base on another map (606) and a deployed base with no owner record at all (607).
const basesWorld = `
insert into actors(id,class,map,location_x,location_y,location_z,state) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7,0),(602,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',20,30,40,5),(603,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',8000,0,0,0),
  (604,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',90000,0,0,0),(605,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',1,1,1,0),(606,'/Game/BP_Totem.BP_Totem_C','DeepDesert',9,9,9,0),
  (607,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',60,60,6,0);
insert into totems(id,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z) values
  (601,5000.0,6000.0,7.0),(602,21.0,31.0,41.0),(603,8000.0,0.0,0.0),(604,90000.0,0.0,0.0),(605,1.0,1.0,1.0),(606,9.0,9.0,9.0),(607,61.0,61.0,6.0);
insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id,is_hologram) values (601,-601,'Totem_Placeable',1,0),(603,-603,'Totem_Small_Placeable',1,0);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values
  (601,'##Totem_Placeable',1,3,1),(602,'Old Camp',1,3,1),(603,'##Totem_Small_Placeable',1,3,1),(604,'Not Mine',1,3,1),(605,'Shared With Me',1,3,1),(606,'Far Base',1,3,1);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(602,1,1),(603,1,1),(604,9,1),(605,1,2),(606,1,1);
insert into base_backups(id,player_id,base_backup_name) values (1,1,'Old Camp'),(2,1,'Again');
insert into base_backup_linked_actors(id,actor_id) values (1,602),(2,603);`

func TestBasesAreTheCharactersOwnDeployedBasesOnly(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, basesWorld)}
	got := groupLabels(t, o, groupBase)
	// nearest first: 601 and 603 (claimed again after it was picked up, so it stands in the world)
	if fmt.Sprint(got) != "[Advanced Sub-Fief #601 Sub-Fief #603]" {
		t.Fatalf("bases: %v (not the picked-up 602, not another player's 604, not the one the character is only a member of 605, not the one on another map 606, not the one with no owner record 607)", got)
	}
	// the Deep Desert has its own: the character's base there
	if _, d := namedPlacesOf(t, o, "", "DeepDesert"); fmt.Sprint(labelsOf(d)) != "[base: Far Base #606]" {
		t.Errorf("the Deep Desert: %v", labelsOf(d))
	}
}

func TestAPickedUpBaseIsNotAPlaceEvenWithTheCharactersOwnerRecord(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, basesWorld)}
	for _, d := range listDestinations(t, o, "", "") {
		if strings.Contains(d.Label, "#602") {
			t.Errorf("base 602 was picked up with the game's tool (state 5): %+v", d)
		}
	}
	// the same base, put back in the world (its actors have another state), is a place again
	o2 := &Ops{S: testsave.PlayerWithSQL(t, basesWorld+`update actors set state=0 where id=602;`)}
	if got := groupLabels(t, o2, groupBase); len(got) != 3 || !strings.Contains(fmt.Sprint(got), "Old Camp #602") {
		t.Errorf("a deployed base is listed: %v", got)
	}
}

// vehiclesWorld: vehicles of the character that stand in the world (801 a sandbike with no name given, 802 named by the character, 806 and 807 two of one kind, 808 a medium
// ornithopter of the game's own blueprint), and the ones that are not places: stored in the vehicle backup (803: state 2 only, 811: its row only), stored for recovery (809: state 4 only,
// 810: a recovery row only), another player's (804) and one on another map (805).
const vehiclesWorld = `
insert into actors(id,class,map,location_x,location_y,location_z,state) values
  (801,'/Game/Vehicles/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',300,400,50,0),(802,'/Game/Vehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',9000,9000,10,0),
  (803,'/Game/Vehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',150,250,5,2),(804,'/Game/Vehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',160,260,5,0),
  (805,'/Game/Vehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','DeepDesert',1,1,1,0),
  (806,'/Game/Vehicles/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',500,600,50,0),(807,'/Game/Vehicles/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin',700,800,50,0),
  (808,'/Game/Dune/Systems/Vehicles/Blueprints/FlyingVehicles/BP_MediumOrnithopter_CHOAM.BP_MediumOrnithopter_CHOAM_C','HaggaBasin',2000,2000,500,0),
  (809,'/Game/Vehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',171,271,5,4),(810,'/Game/Vehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',172,272,5,0),
  (811,'/Game/Vehicles/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin',173,273,5,0);
insert into vehicles(id) values (801),(802),(803),(804),(805),(806),(807),(808),(809),(810),(811);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values
  (801,'##OneManGroundcarChoam',2,3,0),(802,'Red Runner',2,3,0),(803,'Stored',2,3,0),(804,'Theirs',2,3,0),(805,'Far Buggy',2,3,0),(806,'##OneManGroundcarChoam',2,3,0),(807,'None',2,3,0),
  (808,'##MediumOrnithopter',2,3,0),(809,'Recovering',2,3,0),(810,'Recovered',2,3,0),(811,'Backed up',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (801,1,1),(802,1,1),(803,1,1),(804,9,1),(805,1,1),(806,1,1),(807,1,1),(808,1,1),(809,1,1),(810,1,1),(811,1,1);
insert into backup_vehicles(character_id,vehicle_id,customization_id) values (1,811,'None');
insert into recovered_vehicles(character_id,vehicle_id,vehicle_name,time_stored,chassis_durability,customization_id,reason) values (1,810,'Recovered',0,1.0,'None',0);`

func TestVehiclesAreTheCharactersOwnVehiclesInTheWorldOnly(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, vehiclesWorld)}
	got := groupLabels(t, o, groupVehicle)
	if fmt.Sprint(got) != "[Sandbike #801 Sandbike #806 Sandbike #807 Medium ornithopter #808 Red Runner #802]" {
		t.Fatalf("vehicles: %v (not the stored 803, 809, 810 and 811, not another player's 804, not the one on another map 805)", got)
	}
}

func TestVehiclesAreNamedByTheirNameOrTheNameOfTheirKindNeverTheBlueprint(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, vehiclesWorld)}
	for _, l := range groupLabels(t, o, groupVehicle) {
		if strings.Contains(l, "BP_") || strings.Contains(l, "_C") || strings.Contains(l, "##") || strings.Contains(l, "None") {
			t.Errorf("a vehicle must show a name a player knows: %q", l)
		}
	}
	// two of one kind are told apart by their number; a name the character gave is kept
	got := fmt.Sprint(groupLabels(t, o, groupVehicle))
	for _, want := range []string{"Sandbike #806", "Sandbike #807", "Red Runner #802"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from %s", want, got)
		}
	}
	// a blueprint the catalog does not know is still shown in words
	for in, want := range map[string]string{
		"/Game/X/BP_SandCrawler_CHOAM.BP_SandCrawler_CHOAM_C":                   "Sandcrawler",
		"/Game/X/BP_TransportOrnithopter_CHOAM.BP_TransportOrnithopter_CHOAM_C": "Carrier ornithopter",
		"/Game/X/BP_Tank_Heavy.BP_Tank_Heavy_C":                                 "Tank Heavy",
		"/Game/X/BP_ThopterNew_CHOAM.BP_ThopterNew_CHOAM_C":                     "Thopter New",
		"": "Vehicle",
	} {
		if got := vehicleKindName(in); got != want {
			t.Errorf("vehicleKindName(%q) = %q, want %q", in, got, want)
		}
	}
}

// respawnWorld: respawn points of the character: a beacon that stands in the world (803), one with the name the character gave it (804), a point kept as bare coordinates, one whose
// locator actor was picked up with a base (805, state 5), one on another map (806), and a point of another character (807).
const respawnWorld = `
insert into actors(id,class,map,location_x,location_y,location_z,state) values
  (803,'/Game/Beacon.Beacon_C','HaggaBasin',1000,1000,10,0),(804,'/Game/Beacon.Beacon_C','HaggaBasin',3000,3000,10,0),(805,'/Game/Beacon.Beacon_C','HaggaBasin',150,250,10,5),
  (806,'/Game/Beacon.Beacon_C','DeepDesert',-300000,-590000,10,0),(807,'/Game/Beacon.Beacon_C','HaggaBasin',110,210,10,0);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (804,'Camp bed',1,3,0);
insert into player_respawn_locations(id,character_id,"group",locator_actor_id,map,dimension) values
  (x'03',1,'BaseTotem',803,'HaggaBasin',0),(x'04',1,'Beacon',804,'HaggaBasin',0),(x'05',1,'BaseTotem',805,'HaggaBasin',0),(x'06',1,'BaseTotem',806,'DeepDesert',0),(x'07',9,'BaseTotem',807,'HaggaBasin',0);
insert into player_respawn_locations(id,character_id,"group",locator_location_x,locator_location_y,locator_location_z,map,dimension) values
  (x'01',1,'Vehicle',150.0,250.0,310.0,'HaggaBasin',0),(x'02',1,'PlayerStart',NULL,NULL,NULL,'HaggaBasin',0);`

func TestRespawnPointsAreTheCharactersOwnThatExistInTheWorld(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, respawnWorld)}
	got := groupLabels(t, o, groupRespawn)
	// nearest first: the bare coordinates (150, 250), the beacon at 1000, 1000 and the named one at 3000, 3000
	if fmt.Sprint(got) != "[Respawn point (Vehicle) Respawn point #803 Camp bed #804]" {
		t.Fatalf("respawn points: %v (not the one whose actor was picked up with a base, not the one on another map, not another character's, not the one with no position)", got)
	}
	d := listDestinations(t, o, groupRespawn, "")
	if d[1].X != 1000 || d[1].Y != 1000 || d[1].Z != 10 || d[0].Z != 310 {
		t.Errorf("a point with an actor is at the actor's position, one without at its own coordinates: %+v", d)
	}
	if got := groupLabels(t, &Ops{S: testsave.PlayerWithSQL(t, respawnWorld)}, groupRespawn); len(got) != 3 {
		t.Errorf("%v", got)
	}
	_, dd := namedPlacesOf(t, o, "", "DeepDesert")
	if fmt.Sprint(labelsOf(dd)) != "[respawn: Respawn point #806]" {
		t.Errorf("the Deep Desert: %v", labelsOf(dd))
	}
}

func TestARespawnPointWhoseActorIsGoneIsNotAPlace(t *testing.T) {
	// a row that names an actor the save no longer has (the editor reads a save with its foreign keys off)
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into player_respawn_locations(id,character_id,"group",locator_actor_id,locator_location_x,locator_location_y,locator_location_z,map,dimension) values (x'31',1,'BaseTotem',9999,500.0,500.0,5.0,'HaggaBasin',0);`)}
	if got := groupLabels(t, o, groupRespawn); len(got) != 0 {
		t.Errorf("nothing stands at a point whose actor is gone, even though the row keeps coordinates: %v", got)
	}
}

func TestEveryGroupIsInOneListInThePagesOrder(t *testing.T) {
	o := placesOps(t, respawnWorldForPlaces)
	m, d := namedPlacesOf(t, o, "", "")
	var order []string
	for _, x := range d {
		if len(order) == 0 || order[len(order)-1] != x.Group {
			order = append(order, x.Group)
		}
	}
	if fmt.Sprint(order) != "[tradepost base vehicle respawn]" {
		t.Errorf("groups in one list: %v", labelsOf(d))
	}
	var keys []string
	for _, g := range m["groups"].([]DestinationGroup) {
		keys = append(keys, g.Label)
	}
	if fmt.Sprint(keys) != "[Trading posts Personal bases Vehicles Respawn points]" {
		t.Errorf("headings: %v", keys)
	}
	if _, ok := m["kind"]; ok {
		t.Errorf("the answer has no kind any more: %v", m["kind"])
	}
}

const respawnWorldForPlaces = `
insert into actors(id,class,map,location_x,location_y,location_z,state) values (901,'/Game/Beacon.Beacon_C','HaggaBasin',400,500,10,0);
insert into player_respawn_locations(id,character_id,"group",locator_actor_id,map,dimension) values (x'41',1,'BaseTotem',901,'HaggaBasin',0);`

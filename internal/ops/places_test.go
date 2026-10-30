package ops

import (
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #500: Go to a place offers four groups of places by the names a player sees. The LocationKeys below are rows of the game's own table DT_StaticLocationData and are in
// place-names.json; the character stands at 100, 200, 300 on HaggaBasin (the Player fixture).
const (
	locPinnacle    = "09D3DED34D48F0AB2994DE853D7B2FCF"
	locPinnacle2   = "84AAE0014040BCD05E16849A140E4CF5"
	locGriffin     = "1546D0AC43DF1AADFA14E5A2C5C56D19"
	locAnvil       = "2445CEFC45F16149BFD5D78C80841113"
	locCrossroads  = "00271320416D180DAAF847BB767C31A6"
	locDeepDesert  = "6F72F943466BF7C8FC1D3E88853764D5" // one of the three Deep Desert taxi service points (#516)
	locDeepDesert2 = "5B1E5FA645B571DF9714B2B2F8EDD06B"
	locDeepDesert3 = "747ED59B48C052C7577385A5C8DEC0C1"
	locUnknownPost = "ABCDEF0123456789ABCDEF0123456789" // a made-up key that no table has
)

// payloadSQL is a blob literal holding a marker payload with the given LocationKey.
func payloadSQL(key string) string {
	p := testsave.MarkerPayload(key, `(GameplayTags=((TagName=\"Exploration.POI.TradePost\")))`, `LOCTABLE(\"/Game/Dune/Localization/ST_Localization_NPCs_World.ST_Localization_NPCs_World\", \"NPCS_AND_WORLD/X\")`)
	return "x'" + hex.EncodeToString(p) + "'"
}

// placesWorld: trading posts (three named ones, one with a key nobody has, one never discovered, one with no payload), things the list must not offer (an outpost, a camp, a
// cave, a resource, a vendor, a hazard, a HomeBase marker), a taxi hub on another map, and on the Deep Desert a marker the save types TradingPost whose key is a taxi service point (#516); bases (two of the character's, another player's, one the character is only a member of, one picked up, one on another map)
// and vehicles (two of the character's, one stored, another player's, one on another map).
var placesWorld = `
insert into actors(id,class,map,location_x,location_y,location_z,state) values
  (9,'/Game/Other.Other_C','HaggaBasin',0,0,0,0),
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7,0),(602,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',20,30,40,0),(603,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',90000,0,0,0),
  (604,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',1,1,1,0),(605,'/Game/BP_Totem.BP_Totem_C','DeepDesert',9,9,9,0),(606,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',60,60,6,0),
  (801,'/Game/Vehicles/BP_Sandbike.BP_Sandbike_C','HaggaBasin',300,400,50,0),(802,'/Game/Vehicles/BP_Buggy.BP_Buggy_C','HaggaBasin',9000,9000,10,0),
  (803,'/Game/Vehicles/BP_Buggy.BP_Buggy_C','HaggaBasin',150,250,5,2),(804,'/Game/Vehicles/BP_Buggy.BP_Buggy_C','HaggaBasin',160,260,5,0),
  (805,'/Game/Vehicles/BP_Buggy.BP_Buggy_C','DeepDesert',1,1,1,0);
insert into totems(id,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z) values
  (601,5000.0,6000.0,7.0),(602,21.0,31.0,41.0),(603,90000.0,0.0,0.0),(604,1.0,1.0,1.0),(605,9.0,9.0,9.0),(606,61.0,61.0,6.0);
insert into placeables(id,owner_entity_id,building_type,last_placed_by_player_id,is_hologram) values (601,-601,'Totem_Placeable',1,0),(602,-602,'Totem_Small_Placeable',1,0);
insert into vehicles(id) values (801),(802),(803),(804),(805);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values
  (601,'##Totem_Placeable',1,3,1),(602,'Spice Camp',1,3,1),(603,'Not Mine',1,3,1),(605,'Far Base',1,3,1),(606,'Shared With Me',1,3,1),
  (801,'##SandbikeChoam',2,3,0),(802,'Red Runner',2,3,0),(803,'Stored',2,3,0),(804,'Theirs',2,3,0),(805,'Far Buggy',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(602,1,1),(603,9,1),(605,1,1),(606,1,2),(801,1,1),(802,1,1),(803,1,1),(804,9,1),(805,1,1);
insert into backup_vehicles(character_id,vehicle_id,customization_id) values (1,803,'None');
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,payload) values
  (1,0,'HaggaBasin','TradingPost',3000.0,4000.0,50.0,` + payloadSQL(locPinnacle) + `),
  (2,0,'HaggaBasin','TradingPost',50000.0,0.0,60.0,` + payloadSQL(locGriffin) + `),
  (3,0,'HaggaBasin','TradingPost',800.0,900.0,0.0,` + payloadSQL(locUnknownPost) + `),
  (4,0,'HaggaBasin','TradingPost',70000.0,70000.0,0.0,` + payloadSQL(locCrossroads) + `),
  (5,0,'HaggaBasin','TradingPost',1500.0,1500.0,0.0,NULL),
  (6,0,'HaggaBasin','EnemyOutpost',110.0,210.0,0.0,NULL),(7,0,'HaggaBasin','EnemyCamp',120.0,220.0,0.0,NULL),(8,0,'HaggaBasin','Cave',130.0,230.0,0.0,NULL),
  (9,0,'HaggaBasin','SpiceField',140.0,240.0,0.0,NULL),(10,0,'HaggaBasin','WeaponsVendor',150.0,250.0,0.0,NULL),(11,0,'HaggaBasin','Hazard_Radiation',160.0,260.0,0.0,NULL),
  (12,0,'HaggaBasin','HomeBase',170.0,270.0,0.0,x'cc5187'),(13,0,'HaggaBasin','Ecolab',180.0,280.0,0.0,NULL),
  (21,0,'Arrakeen','TaxiService',11695.0,-258.0,51585.0,NULL),(22,0,'HarkoVillage','TaxiService',-1390.0,26838.0,14739.0,NULL),
  (31,0,'DeepDesert','TradingPost',-306000.0,-588000.0,40.0,` + payloadSQL(locDeepDesert) + `);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method)
  select 1,marker_hash_id,0,map_name,3,1 from markers where marker_hash_id not in (4,13);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,4,0,'HaggaBasin',0,1),(1,13,0,'HaggaBasin',0,1);`

func placesOps(t *testing.T, extra ...string) *Ops {
	return &Ops{S: testsave.PlayerWithSQL(t, placesWorld+strings.Join(extra, "\n"))}
}

func namedPlacesOf(t *testing.T, o *Ops, q, mp string) (map[string]any, []Destination) {
	t.Helper()
	res, err := o.Destinations(q, mp)
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	return m, m["destinations"].([]Destination)
}

func labelsOf(d []Destination) []string {
	out := make([]string, 0, len(d))
	for _, x := range d {
		out = append(out, x.Group+": "+x.Label)
	}
	return out
}

func TestNamedPlacesListTradingPostsByTheirInGameNameAndNothingElseOnTheMap(t *testing.T) {
	_, d := namedPlacesOf(t, placesOps(t), "", "")
	var posts []string
	for _, x := range d {
		if x.Group == groupTradePost {
			posts = append(posts, x.Label)
		}
	}
	// nearest first: the post with a key nobody has (marker 3), the post with no payload (5), then Griffin's Reach and Pinnacle Station, ordered by their arrival points
	// (#515: Griffin's Reach is nearer to the character than Pinnacle Station, though its marker, at 50000, 0, is farther than Pinnacle Station's, at 3000, 4000);
	// Crossroads was never discovered
	want := []string{"TradingPost (key " + locUnknownPost + ")", "TradingPost (marker 5)", "Griffin's Reach Tradepost", "Pinnacle Station Tradepost"}
	if fmt.Sprint(posts) != fmt.Sprint(want) {
		t.Fatalf("trading posts: %v, want %v", posts, want)
	}
	for _, x := range d {
		for _, bad := range []string{"Outpost", "EnemyCamp", "Cave", "SpiceField", "Vendor", "Hazard", "HomeBase", "Ecolab", "Crossroads"} {
			if strings.Contains(x.Label, bad) {
				t.Errorf("%q must not be offered: %+v", bad, x)
			}
		}
	}
	if len(d) != 4+2+2 {
		t.Errorf("4 trading posts, 2 bases, 2 vehicles and no taxi hub on this map: %v", labelsOf(d))
	}
}

func TestNamedPlaceFilterMatchesTheName(t *testing.T) {
	o := placesOps(t)
	for q, want := range map[string]string{
		"pinn":       "Pinnacle Station Tradepost",
		"PINNACLE":   "Pinnacle Station Tradepost",
		"griffin":    "Griffin's Reach Tradepost",
		"tradepost":  "", // two named posts: checked below
		"red runner": "Red Runner #802",
		"spice":      "Spice Camp #602",
	} {
		_, d := namedPlacesOf(t, o, q, "")
		if want == "" {
			if len(d) != 2 {
				t.Errorf("filter %q: %v", q, labelsOf(d))
			}
			continue
		}
		if len(d) != 1 || d[0].Label != want {
			t.Errorf("filter %q: %v, want %q", q, labelsOf(d), want)
		}
	}
	if _, d := namedPlacesOf(t, o, "outpost", ""); len(d) != 0 {
		t.Errorf("an outpost is not in any group, so no filter finds it: %v", labelsOf(d))
	}
	// the other name of the place that the game calls The Anvil is matched too, and not shown
	o2 := placesOps(t, `update markers set payload=`+payloadSQL(locAnvil)+` where marker_hash_id=2;`)
	_, d := namedPlacesOf(t, o2, "mirzabah", "")
	if len(d) != 1 || d[0].Label != "The Anvil Tradepost" {
		t.Errorf("Mirzabah's Shadow is where the game's The Anvil is: %v", labelsOf(d))
	}
}

func TestNamedPlacesGroupsAndTheirOrder(t *testing.T) {
	m, d := namedPlacesOf(t, placesOps(t), "", "")
	groups := m["groups"].([]DestinationGroup)
	if fmt.Sprint(groups) != fmt.Sprint([]DestinationGroup{{"tradepost", "Trading posts"}, {"base", "Personal bases"}, {"vehicle", "Vehicles"}}) {
		t.Errorf("only the groups that have places, in the page's order: %+v", groups)
	}
	var seen []string
	for _, x := range d {
		if len(seen) == 0 || seen[len(seen)-1] != x.Group {
			seen = append(seen, x.Group)
		}
	}
	if fmt.Sprint(seen) != "[tradepost base vehicle]" {
		t.Errorf("a group's places sit together: %v", labelsOf(d))
	}

	var bases, vehicles []Destination
	for _, x := range d {
		switch x.Group {
		case groupBase:
			bases = append(bases, x)
		case groupVehicle:
			vehicles = append(vehicles, x)
		}
	}
	// the character's own bases, nearest first, by the name they were given (or the game's name for the claim structure), at the claim position
	if len(bases) != 2 || bases[0].Label != "Spice Camp #602" || bases[1].Label != "Advanced Sub-Fief #601" || bases[1].X != 5000 || bases[1].Y != 6000 || bases[1].Z != 7 {
		t.Errorf("bases (not another player's, not the picked-up one, not the one on another map): %+v", bases)
	}
	// the character's own vehicles that stand in the world
	if len(vehicles) != 2 || vehicles[0].Label != "Sandbike #801" || vehicles[1].Label != "Red Runner #802" || vehicles[0].X != 300 || vehicles[0].Z != 50 {
		t.Errorf("vehicles (not the stored one, not another player's, not the one on another map): %+v", vehicles)
	}
	if vehicles[0].Distance <= 0 || vehicles[1].Distance <= vehicles[0].Distance {
		t.Errorf("each place shows its distance: %+v", vehicles)
	}
}

func TestNamedPlacesTaxiHubsAreOfferedOnTheirOwnMap(t *testing.T) {
	// the hubs are maps of their own; the character stands in Arrakeen
	o := placesOps(t, `update actors set map='Arrakeen' where id in (1,2,3);`)
	m, d := namedPlacesOf(t, o, "", "")
	if len(d) != 1 || d[0].Label != "Arrakeen Taxi Service" || d[0].Group != groupTaxi || d[0].X != 11495 || d[0].Y != -316 || d[0].Z != 51457 || !d[0].Arrival {
		t.Fatalf("taxi hub: %v", labelsOf(d))
	}
	if g := m["groups"].([]DestinationGroup); len(g) != 1 || g[0].Label != "Taxis" {
		t.Errorf("groups: %+v", g)
	}
	o = placesOps(t, `update actors set map='HarkoVillage' where id in (1,2,3);`)
	if _, d := namedPlacesOf(t, o, "harko", ""); len(d) != 1 || d[0].Label != "Harko Village Taxi Service" {
		t.Errorf("the hub's name is its map's name in words: %v", labelsOf(d))
	} else if d[0].X != -1415 || d[0].Y != 26560 || d[0].Z != 14673.9 || !d[0].Arrival {
		t.Errorf("a hub is at the game's arrival point for it (#515), not at its marker (-1390, 26838, 14739): %+v", d[0])
	}
	// on Hagga Basin there is no hub marker, so there is no Taxis group
	if m, _ := namedPlacesOf(t, placesOps(t), "", ""); len(m["groups"].([]DestinationGroup)) != 3 {
		t.Errorf("groups: %+v", m["groups"])
	}
}

func TestNamedPlacesOfAnotherMapHaveNoDistance(t *testing.T) {
	m, d := namedPlacesOf(t, placesOps(t), "", "DeepDesert")
	if m["sameMap"] != false || len(d) != 3 {
		t.Fatalf("the Deep Desert's base, vehicle and taxi service point: %v", labelsOf(d))
	}
	if d[0].Label != "Far Base #605" || d[1].Label != "Far Buggy #805" || d[2].Label != "Deep Desert Taxi Service" || d[2].Group != groupTaxi {
		t.Errorf("%v", labelsOf(d))
	}
	for _, x := range d {
		if x.Distance != 0 {
			t.Errorf("a distance across maps means nothing: %+v", x)
		}
	}
}

func TestNamedPlacesWithTheSameNameAreToldApart(t *testing.T) {
	// the game's table has three keys for Pinnacle Station; a save may hold more than one of them
	o := placesOps(t, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,payload) values (41,0,'HaggaBasin','TradingPost',-4000.0,-4000.0,0.0,`+payloadSQL(locPinnacle2)+`);
		insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,41,0,'HaggaBasin',3,1);`)
	_, d := namedPlacesOf(t, o, "pinnacle", "")
	if len(d) != 2 || d[0].Label != "Pinnacle Station Tradepost (09D3DE)" || d[1].Label != "Pinnacle Station Tradepost (84AAE0)" {
		t.Errorf("each key's start tells the two apart (the distance is shown beside the name): %v", labelsOf(d))
	}
	// a name that is alone is left as it is
	if _, d := namedPlacesOf(t, o, "griffin", ""); len(d) != 1 || d[0].Label != "Griffin's Reach Tradepost" {
		t.Errorf("%v", labelsOf(d))
	}
}

func TestMarkerLocationKey(t *testing.T) {
	good := testsave.MarkerPayload(locPinnacle, "(GameplayTags=)", "x")
	for name, c := range map[string]struct {
		in   []byte
		want string
	}{
		"as the game writes it":        {good, locPinnacle},
		"lower case is read as upper":  {testsave.MarkerPayload(strings.ToLower(locPinnacle), "t", "x"), locPinnacle},
		"no payload":                   {nil, ""},
		"no key in it":                 {[]byte("\xb7OwnerUID\x131"), ""},
		"the name and nothing after":   {[]byte("\xb7LocationKey"), ""},
		"a key that is too short":      {[]byte("\xb7LocationKey\xc7\x1f09D3DED34D48F0AB2994DE853D7B2FC"), ""},
		"a key that is too long":       {[]byte("\xb7LocationKey\xc7\x21" + locPinnacle + "0"), ""},
		"not hexadecimal":              {[]byte("\xb7LocationKey\xc7\x20" + strings.Repeat("Z", 32)), ""},
		"the digits far from the name": {[]byte("\xb7LocationKey" + strings.Repeat("-", 20) + locPinnacle), ""},
	} {
		if got := markerLocationKey(c.in); got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
}

// place-names.json is what the page's names come from, so what it holds is checked: every key is a LocationKey, every name is there, and the four names the ornithopter
// pilot offers are the ones the editor shows.
func TestPlaceNamesFileIsWellFormed(t *testing.T) {
	hex32 := regexp.MustCompile(`^[0-9A-F]{32}$`)
	if len(placeNames) < 10 {
		t.Fatalf("the file names %d places", len(placeNames))
	}
	byName := map[string]int{}
	for k, p := range placeNames {
		if !hex32.MatchString(k) {
			t.Errorf("%q is not a LocationKey (32 upper-case hexadecimal digits)", k)
		}
		if strings.TrimSpace(p.Name) == "" || p.Name != strings.TrimSpace(p.Name) || strings.HasSuffix(p.Name, ".") {
			t.Errorf("%s: name %q", k, p.Name)
		}
		if p.Kind != "tradepost" && p.Kind != kindTaxi {
			t.Errorf("%s: kind %q", k, p.Kind)
		}
		if (p.Kind == kindTaxi) != strings.Contains(p.Name, "Taxi Service") || (p.Kind == "tradepost") != strings.HasSuffix(p.Name, "Tradepost") {
			t.Errorf("%s: the name %q does not fit the kind %q", k, p.Name, p.Kind)
		}
		byName[p.Name]++
	}
	// the Deep Desert has taxi service points and no trading post (#516)
	for _, k := range []string{locDeepDesert, locDeepDesert2, locDeepDesert3} {
		if p := placeNames[k]; p.Kind != kindTaxi || p.Name != "Deep Desert Taxi Service" || !p.Derived {
			t.Errorf("%s: %+v", k, p)
		}
	}
	for _, name := range []string{"Pinnacle Station Tradepost", "Crossroads Tradepost", "Griffin's Reach Tradepost", "The Anvil Tradepost", "Deep Desert Taxi Service"} {
		if byName[name] == 0 {
			t.Errorf("no key is named %q: %v", name, byName)
		}
	}
	if placeNames[locPinnacle].Name != "Pinnacle Station Tradepost" || placeNames[locAnvil].Alias != "Mirzabah's Shadow" {
		t.Errorf("%+v %+v", placeNames[locPinnacle], placeNames[locAnvil])
	}
	if byName["Deep Desert Tradepost"] != 0 {
		t.Errorf("the Deep Desert has no trading post: %v", byName)
	}
}

// #516: the three Deep Desert keys carry the trading-post tag in the game's table but are the Deep Desert's taxi service points. A marker the save types TradingPost with such
// a key is listed under Taxis, by the taxi name, and never under Trading posts.
func TestDeepDesertTaxiKeysAreTaxisAndNeverTradingPosts(t *testing.T) {
	o := placesOps(t, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,payload) values
		(32,0,'DeepDesert','TradingPost',-305000.0,-587000.0,40.0,`+payloadSQL(locDeepDesert2)+`),
		(33,0,'DeepDesert','TradingPost',-304000.0,-586000.0,40.0,`+payloadSQL(locDeepDesert3)+`);
		insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,32,0,'DeepDesert',3,1),(1,33,0,'DeepDesert',3,1);`)
	m, d := namedPlacesOf(t, o, "", "DeepDesert")
	var taxis []string
	for _, x := range d {
		if x.Group == groupTradePost || strings.Contains(x.Label, "Tradepost") {
			t.Errorf("the Deep Desert has no trading post: %+v", x)
		}
		if x.Group == groupTaxi {
			taxis = append(taxis, x.Label)
		}
	}
	// three keys of one name: told apart by the start of the key, as for any group
	if len(taxis) != 3 {
		t.Fatalf("taxis: %v", taxis)
	}
	for _, want := range []string{"Deep Desert Taxi Service (6F72F9)", "Deep Desert Taxi Service (5B1E5F)", "Deep Desert Taxi Service (747ED5)"} {
		if !strings.Contains(fmt.Sprint(taxis), want) {
			t.Errorf("taxis: %v, want %q", taxis, want)
		}
	}
	if g := fmt.Sprint(m["groups"]); g != fmt.Sprint([]DestinationGroup{{"base", "Personal bases"}, {"vehicle", "Vehicles"}, {"taxi", "Taxis"}}) {
		t.Errorf("no Trading posts heading on the Deep Desert: %v", g)
	}
	// the filter finds them by the taxi name and not as trading posts
	if _, d := namedPlacesOf(t, o, "taxi", "DeepDesert"); len(d) != 3 {
		t.Errorf("taxi: %v", labelsOf(d))
	}
	if _, d := namedPlacesOf(t, o, "tradepost", "DeepDesert"); len(d) != 0 {
		t.Errorf("tradepost: %v", labelsOf(d))
	}
	// Hagga Basin: no Taxis heading, and the trading posts are the same as without these markers
	m, d = namedPlacesOf(t, o, "", "")
	var posts []string
	for _, x := range d {
		if x.Group == groupTradePost {
			posts = append(posts, x.Label)
		}
		if x.Group == groupTaxi {
			t.Errorf("a save on Hagga Basin lists no taxi: %+v", x)
		}
	}
	want := []string{"TradingPost (key " + locUnknownPost + ")", "TradingPost (marker 5)", "Griffin's Reach Tradepost", "Pinnacle Station Tradepost"}
	if fmt.Sprint(posts) != fmt.Sprint(want) {
		t.Errorf("trading posts of Hagga Basin: %v, want %v", posts, want)
	}
	if len(m["groups"].([]DestinationGroup)) != 3 {
		t.Errorf("groups: %+v", m["groups"])
	}
	// every trading-post key of the file is one of the four Hagga Basin posts
	fourPosts := map[string]bool{"Pinnacle Station Tradepost": true, "Crossroads Tradepost": true, "Griffin's Reach Tradepost": true, "The Anvil Tradepost": true}
	for k, p := range placeNames {
		if p.Kind == "tradepost" && !fourPosts[p.Name] {
			t.Errorf("%s: %+v", k, p)
		}
	}
}

// #515: Go to a place fills X, Y and Z with the game's own arrival point for a trading post and a hub, and with the marker's position for every other place.
// The coordinates below are the values of place-arrivals.json, written out so that a change to the file is seen.
var arrivalWant = map[string][3]float64{
	locPinnacle:   {-32905.3, -313888.6, 12053.0},
	locAnvil:      {193966.6, 2552.5, 13515.0},
	locGriffin:    {23860.4, 227191.3, 8550.0},
	locCrossroads: {-218763.0, -162786.8, 7387.2},
}

func TestEachTradingPostWithAnArrivalPointTeleportsToIt(t *testing.T) {
	for loc, want := range arrivalWant {
		o := placesOps(t, `update markers set payload=`+payloadSQL(loc)+` where marker_hash_id=1;
			update player_markers set discovery_level=3 where marker_hash_id=1;`)
		_, d := namedPlacesOf(t, o, "tradepost", "")
		var got *Destination
		for i := range d {
			if d[i].Group == groupTradePost && d[i].X == want[0] && d[i].Y == want[1] && d[i].Z == want[2] {
				got = &d[i]
			}
			if d[i].Group == groupTradePost && d[i].X == 3000 && d[i].Y == 4000 {
				t.Errorf("%s: the marker's own position is not offered for a post that has an arrival point: %+v", loc, d[i])
			}
		}
		if got == nil || !got.Arrival {
			t.Errorf("%s: want the arrival point %v in %v", loc, want, d)
			continue
		}
		if _, err := o.Teleport(Args{"x": got.X, "y": got.Y, "z": got.Z}); err != nil {
			t.Fatal(err)
		}
		r, _ := o.S.One(`select location_x x, location_y y, location_z z from actors where id=2`)
		if r["x"] != want[0] || r["y"] != want[1] || r["z"] != want[2] {
			t.Errorf("%s: teleported to %v, want %v", loc, r, want)
		}
	}
}

func TestPlacesWithoutAnArrivalPointKeepTheirMarkerPosition(t *testing.T) {
	_, d := namedPlacesOf(t, placesOps(t), "", "")
	at := map[string]Destination{}
	for _, x := range d {
		at[x.Label] = x
		if x.Group != groupTradePost && x.Arrival {
			t.Errorf("only trading posts and hubs have an arrival point: %+v", x)
		}
	}
	// a post whose key the file does not have, and one with no payload
	if p := at["TradingPost (key "+locUnknownPost+")"]; p.Arrival || p.X != 800 || p.Y != 900 {
		t.Errorf("%+v", p)
	}
	if p := at["TradingPost (marker 5)"]; p.Arrival || p.X != 1500 || p.Y != 1500 {
		t.Errorf("%+v", p)
	}
	// a base
	if p := at["Spice Camp #602"]; p.Arrival || p.X != 21 {
		t.Errorf("%+v", p)
	}
	// the Deep Desert taxi service points have no arrival point
	_, dd := namedPlacesOf(t, placesOps(t), "taxi", "DeepDesert")
	if len(dd) != 1 || dd[0].Arrival || dd[0].X != -306000 || dd[0].Y != -588000 || dd[0].Z != 40 {
		t.Errorf("a Deep Desert taxi service point keeps the marker position: %+v", dd)
	}
	// a hub of a map the file has no arrival point for keeps its marker
	o := placesOps(t, `update markers set map_name='SomeNewHub' where marker_hash_id=21; update player_markers set map_name='SomeNewHub' where marker_hash_id=21;
		update actors set map='SomeNewHub' where id in (1,2,3);`)
	if _, h := namedPlacesOf(t, o, "", ""); len(h) != 1 || h[0].Arrival || h[0].X != 11695 {
		t.Errorf("%+v", h)
	}
	// a post whose marker has no position in the save is still left out, arrival point or not
	o = placesOps(t, `update markers set payload=`+payloadSQL(locAnvil)+`, x=NULL where marker_hash_id=1;`)
	if _, n := namedPlacesOf(t, o, "anvil", ""); len(n) != 0 {
		t.Errorf("no position in the save, no place: %v", labelsOf(n))
	}
}

func TestSeveralKeysOfOnePostAllGetThatPostsArrivalPoint(t *testing.T) {
	o := placesOps(t, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,payload) values (41,0,'HaggaBasin','TradingPost',-4000.0,-4000.0,0.0,`+payloadSQL(locPinnacle2)+`);
		insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,41,0,'HaggaBasin',3,1);`)
	_, d := namedPlacesOf(t, o, "pinnacle", "")
	w := arrivalWant[locPinnacle]
	if len(d) != 2 {
		t.Fatalf("%v", labelsOf(d))
	}
	for _, x := range d {
		if x.X != w[0] || x.Y != w[1] || x.Z != w[2] || !x.Arrival {
			t.Errorf("each key of Pinnacle Station is at its arrival point: %+v", x)
		}
	}
	if d[0].Label == d[1].Label {
		t.Errorf("the two are still told apart: %v", labelsOf(d))
	}
}

// The distance shown beside a place is the distance to what Go to a place will use (the arrival point), so the nearest-first order follows it too.
func TestDistanceOfANamedPostIsToItsArrivalPoint(t *testing.T) {
	_, d := namedPlacesOf(t, placesOps(t), "pinnacle", "")
	if len(d) != 1 {
		t.Fatalf("%v", labelsOf(d))
	}
	w := arrivalWant[locPinnacle]
	if want := math.Hypot(w[0]-100, w[1]-200); math.Abs(d[0].Distance-want) > 1e-6 {
		t.Errorf("distance %v, want %v (to the arrival point; the marker is %v away)", d[0].Distance, want, math.Hypot(3000-100, 4000-200))
	}
}

// place-arrivals.json is what the arrival points come from, so what it holds is checked: six places (four trading posts and two hubs), finite numbers, an observed flag on each.
func TestPlaceArrivalsFileIsWellFormed(t *testing.T) {
	a := placeArrivals
	if len(a.Tradeposts) != 4 || len(a.Hubs) != 2 {
		t.Fatalf("%d trading posts and %d hubs", len(a.Tradeposts), len(a.Hubs))
	}
	check := func(name string, p arrivalPoint) {
		for _, v := range []float64{p.X, p.Y, p.Z} {
			if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e6 {
				t.Errorf("%s: %+v", name, p)
			}
		}
		if p.X == 0 && p.Y == 0 && p.Z == 0 {
			t.Errorf("%s: no point", name)
		}
		if p.Observed == nil {
			t.Errorf("%s: no observed flag", name)
		}
	}
	for name, p := range a.Tradeposts {
		check(name, p)
		found := false
		for _, n := range placeNames {
			if n.Name == name && n.Kind == "tradepost" {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is not the name of a trading post in place-names.json", name)
		}
	}
	for _, n := range placeNames { // every trading post of the names file has an arrival point, and nothing else has
		if _, ok := a.Tradeposts[n.Name]; ok != (n.Kind == "tradepost") {
			t.Errorf("%q: kind %q, has an arrival point: %v", n.Name, n.Kind, ok)
		}
	}
	for _, h := range []string{"Arrakeen", "HarkoVillage"} {
		p, ok := a.Hubs[h]
		if !ok {
			t.Fatalf("no arrival point for %s", h)
		}
		check(h, p)
		if p.Observed == nil || !*p.Observed {
			t.Errorf("%s: the hub points equal logged trips", h)
		}
	}
	// the two computed points have not been seen in use; the other two posts have
	notSeen := map[string]bool{"Griffin's Reach Tradepost": true, "Crossroads Tradepost": true}
	for name, p := range a.Tradeposts {
		if p.Observed != nil && *p.Observed == notSeen[name] {
			t.Errorf("%s: observed %v", name, *p.Observed)
		}
	}
	for loc, w := range arrivalWant {
		if p, ok := tradePostArrival(loc); !ok || p.X != w[0] || p.Y != w[1] || p.Z != w[2] {
			t.Errorf("%s: %+v", loc, p)
		}
	}
	if _, ok := tradePostArrival(locDeepDesert); ok {
		t.Error("a Deep Desert taxi service point has no arrival point")
	}
	if _, ok := tradePostArrival(locUnknownPost); ok {
		t.Error("a key the names file does not have has no arrival point")
	}
}

func TestHubName(t *testing.T) {
	for in, want := range map[string]string{"Arrakeen": "Arrakeen", "HarkoVillage": "Harko Village", "": ""} {
		if got := hubName(in); got != want {
			t.Errorf("hubName(%q) = %q, want %q", in, got, want)
		}
	}
}

// A taxi service point is never given an arrival point, even if the arrival file were to name one: the kind of the key and the group of the row both say no.
func TestAnArrivalPointIsNeverUsedForATaxiKey(t *testing.T) {
	placeArrivals.Tradeposts["Deep Desert Taxi Service"] = arrivalPoint{X: 1, Y: 2, Z: 3}
	defer delete(placeArrivals.Tradeposts, "Deep Desert Taxi Service")
	if _, ok := tradePostArrival(locDeepDesert); ok {
		t.Error("a taxi key is not a trading post")
	}
	_, dd := namedPlacesOf(t, placesOps(t), "taxi", "DeepDesert")
	if len(dd) != 1 || dd[0].Arrival || dd[0].X != -306000 {
		t.Errorf("%+v", dd)
	}
}

package ops

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #521: Go to a place lists the Deep Desert's three taxi service points from the game's level data (deep-desert-taxis.json) whenever the chosen map is the Deep Desert, and offers the
// two hubs, Arrakeen and Harko Village, from every map. The numbers below are the values of the two data files, written out so that a change to a file is seen.
var deepDesertWant = []struct {
	label string
	key   string
	x, y  float64
	z     float64
}{
	{"Deep Desert Taxi Service (A2)", locDeepDesert2, -990770, 971700, 17035.5},
	{"Deep Desert Taxi Service (A5)", locDeepDesert, -121120, 930720, 18149.8},
	{"Deep Desert Taxi Service (A8)", locDeepDesert3, 767350, 906130, 17642.6},
}

// noDeepDesertMarker is a save with no marker for any Deep Desert taxi: the one the shared world has (marker 31) is taken out.
const noDeepDesertMarker = `delete from player_markers where marker_hash_id=31; delete from markers where marker_hash_id=31;`

// inDeepDesert puts the character (controller, body, state) in the Deep Desert at x, y, z.
func inDeepDesert(x, y, z float64) string {
	return fmt.Sprintf(`update actors set map='DeepDesert', location_x=%v, location_y=%v, location_z=%v where id in (1,2,3);`, x, y, z)
}

func taxiRows(d []Destination) []Destination {
	var out []Destination
	for _, x := range d {
		if x.Group == groupTaxi {
			out = append(out, x)
		}
	}
	return out
}

func deepDesertRows(d []Destination) []Destination {
	var out []Destination
	for _, x := range taxiRows(d) {
		if strings.HasPrefix(x.Label, "Deep Desert Taxi Service") {
			out = append(out, x)
		}
	}
	return out
}

func TestDeepDesertListsItsThreeTaxisFromTheLevelDataWhenTheSaveHasNoMarker(t *testing.T) {
	// the character is in the Deep Desert, near the eastern point (A8): nearest first A8, A5, A2
	o := placesOps(t, noDeepDesertMarker, inDeepDesert(766000, 905000, 17500))
	m, d := namedPlacesOf(t, o, "", "")
	if m["map"] != "DeepDesert" || m["sameMap"] != true {
		t.Fatalf("the list is for the Deep Desert: %v", m["map"])
	}
	dd := deepDesertRows(d)
	if len(dd) != 3 {
		t.Fatalf("three taxi service points with no marker in the save: %v", labelsOf(d))
	}
	for i, w := range []int{2, 1, 0} { // nearest first
		got, want := dd[i], deepDesertWant[w]
		if got.Label != want.label || got.X != want.x || got.Y != want.y || got.Z != want.z || got.Arrival || got.OtherMap || got.Map != "" {
			t.Errorf("point %d: %+v, want %+v", i, got, want)
		}
		if exp := math.Hypot(want.x-766000, want.y-905000); math.Abs(got.Distance-exp) > 1e-6 {
			t.Errorf("%s: distance %v, want %v", want.label, got.Distance, exp)
		}
		if i > 0 && got.Distance < dd[i-1].Distance {
			t.Errorf("nearest first: %v", labelsOf(dd))
		}
	}
	if g := fmt.Sprint(m["groups"]); g != fmt.Sprint([]DestinationGroup{{"base", "Personal bases"}, {"vehicle", "Vehicles"}, {"taxi", "Taxis"}}) {
		t.Errorf("the character's base and vehicle on this map, and Taxis: %v", g)
	}
	// the three points come before the hubs, which are another map
	if r := taxiRows(d); len(r) != 5 || !r[3].OtherMap || !r[4].OtherMap {
		t.Errorf("the hubs come after the Deep Desert points: %v", labelsOf(d))
	}
}

func TestDeepDesertTaxisOfAnotherMapAreInGridOrderWithNoDistance(t *testing.T) {
	// the character is on Hagga Basin and asks for the Deep Desert's list
	o := placesOps(t, noDeepDesertMarker)
	m, d := namedPlacesOf(t, o, "", "DeepDesert")
	if m["map"] != "DeepDesert" || m["sameMap"] != false {
		t.Fatalf("%v", m)
	}
	dd := deepDesertRows(d)
	if len(dd) != 3 {
		t.Fatalf("%v", labelsOf(d))
	}
	for i, want := range deepDesertWant {
		if dd[i].Label != want.label || dd[i].X != want.x || dd[i].Distance != 0 {
			t.Errorf("%d: %+v", i, dd[i])
		}
	}
}

func TestOnlyTheDeepDesertListsTheDeepDesertTaxis(t *testing.T) {
	for name, c := range map[string]struct{ sql, mp string }{
		"Hagga Basin, the character's own map": {"", ""},
		"Hagga Basin, asked for":               {inDeepDesert(1, 2, 3), "HaggaBasin"},
		"the character in Arrakeen":            {`update actors set map='Arrakeen' where id in (1,2,3);`, ""},
		"the character in Harko Village":       {`update actors set map='HarkoVillage' where id in (1,2,3);`, ""},
	} {
		_, d := namedPlacesOf(t, placesOps(t, noDeepDesertMarker, c.sql), "", c.mp)
		if got := deepDesertRows(d); len(got) != 0 {
			t.Errorf("%s: %v", name, labelsOf(got))
		}
		if q := taxiRows(d); len(q) != 2 {
			t.Errorf("%s: only the two hubs under Taxis: %v", name, labelsOf(q))
		}
	}
	// a Deep Desert marker in the save does not put the point on another map's list
	_, d := namedPlacesOf(t, placesOps(t), "", "")
	if got := deepDesertRows(d); len(got) != 0 {
		t.Errorf("%v", labelsOf(got))
	}
}

func TestADeepDesertTaxiMarkerInTheSaveIsNotListedASecondTime(t *testing.T) {
	// the shared world holds a marker typed TradingPost with the key of one of the points, at a made-up position, discovered
	_, d := namedPlacesOf(t, placesOps(t), "", "DeepDesert")
	dd := deepDesertRows(d)
	if len(dd) != 3 {
		t.Fatalf("one row for each point and no row for the marker: %v", labelsOf(d))
	}
	for _, x := range dd {
		if x.X == -306000 {
			t.Errorf("the marker's own position is not offered: %+v", x)
		}
	}
	// a TaxiService marker of the Deep Desert at one of the points (the marker of the level data) is the same place; another one far from the points is a place of its own
	o := placesOps(t, noDeepDesertMarker, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values
		(51,0,'DeepDesert','TaxiService',768375.4,906402.0,17730.3),(52,0,'DeepDesert','TaxiService',100.0,200.0,300.0);
		insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,51,0,'DeepDesert',3,1),(1,52,0,'DeepDesert',3,1);`)
	_, d = namedPlacesOf(t, o, "", "DeepDesert")
	if fmt.Sprint(labelsOf(deepDesertRows(d))) != "[taxi: Deep Desert Taxi Service (A2) taxi: Deep Desert Taxi Service (A5) taxi: Deep Desert Taxi Service (A8) taxi: Deep Desert Taxi Service]" {
		t.Errorf("the marker at a point is not a fourth row, the far one is: %v", labelsOf(deepDesertRows(d)))
	}
	// a marker with no position cannot be matched and is left out as always
	o = placesOps(t, noDeepDesertMarker, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values (53,0,'DeepDesert','TaxiService',NULL,NULL,NULL);
		insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,53,0,'DeepDesert',3,1);`)
	if _, d := namedPlacesOf(t, o, "", "DeepDesert"); len(deepDesertRows(d)) != 3 {
		t.Errorf("%v", labelsOf(d))
	}
}

func TestHubsAreOfferedFromEveryMapWithTheirArrivalPoints(t *testing.T) {
	check := func(name string, d []Destination, cur string) {
		t.Helper()
		var hubs []Destination
		for _, x := range taxiRows(d) {
			if x.Map != "" {
				hubs = append(hubs, x)
			}
		}
		if len(hubs) != 2 || hubs[0].Label != "Arrakeen Taxi Service" || hubs[1].Label != "Harko Village Taxi Service" {
			t.Fatalf("%s: both hubs: %v", name, labelsOf(d))
		}
		if hubs[0].Map != "Arrakeen" || hubs[0].X != 11495 || hubs[0].Y != -316 || hubs[0].Z != 51457 || !hubs[0].Arrival {
			t.Errorf("%s: Arrakeen is at the game's arrival point: %+v", name, hubs[0])
		}
		if hubs[1].Map != "HarkoVillage" || hubs[1].X != -1415 || hubs[1].Y != 26560 || hubs[1].Z != 14673.9 || !hubs[1].Arrival {
			t.Errorf("%s: Harko Village is at the game's arrival point: %+v", name, hubs[1])
		}
		for _, h := range hubs {
			if other := h.Map != cur; h.OtherMap != other || (other && h.Distance != 0) || (!other && h.Distance == 0) {
				t.Errorf("%s: %s: another map is flagged and has no distance, the hub the character is in has one: %+v", name, h.Label, h)
			}
		}
	}
	// from Hagga Basin and from the Deep Desert, with no hub marker in the save at all
	_, d := namedPlacesOf(t, placesOps(t, `delete from player_markers where marker_hash_id in (21,22); delete from markers where marker_hash_id in (21,22);`), "", "")
	check("Hagga Basin", d, "HaggaBasin")
	_, d = namedPlacesOf(t, placesOps(t, noDeepDesertMarker, `delete from player_markers where marker_hash_id in (21,22);`), "", "DeepDesert")
	check("the Deep Desert, asked for from Hagga Basin", d, "HaggaBasin")
	_, d = namedPlacesOf(t, placesOps(t, noDeepDesertMarker, inDeepDesert(1, 2, 3)), "", "")
	check("the Deep Desert", d, "DeepDesert")
	// the hub the character is in: its own row has a distance, the other hub is another map; a found marker (placesWorld has both, discovered) is not a second row
	_, d = namedPlacesOf(t, placesOps(t, `update actors set map='Arrakeen' where id in (1,2,3);`), "", "")
	check("Arrakeen", d, "Arrakeen")
	if r := taxiRows(d); len(r) != 2 || r[0].Label != "Arrakeen Taxi Service" || r[0].OtherMap {
		t.Errorf("the hub the character is in first, then the other: %v", labelsOf(r))
	}
	_, d = namedPlacesOf(t, placesOps(t, `update actors set map='HarkoVillage' where id in (1,2,3);`), "harko", "")
	if len(d) != 1 || d[0].Label != "Harko Village Taxi Service" || d[0].OtherMap {
		t.Errorf("the filter finds a hub by its name: %v", labelsOf(d))
	}
	// the hub the character is in, asked for from another map's list, still has its distance
	m, d := namedPlacesOf(t, placesOps(t, `update actors set map='Arrakeen' where id in (1,2,3);`), "", "HaggaBasin")
	if m["sameMap"] != false {
		t.Fatalf("%v", m)
	}
	check("Arrakeen, Hagga Basin asked for", d, "Arrakeen")
}

func TestHubsComeAfterTheTaxisOfTheMapAndAreNotCountedAsNearest(t *testing.T) {
	// on Hagga Basin the first hub is not nearer than anything: a hub is another map, so the places of the character's own map in Taxis (none here) come first and the hubs follow in
	// the order of their names; the Trading posts heading is ahead of Taxis as always
	_, d := namedPlacesOf(t, placesOps(t), "", "")
	last := len(d) - 1
	if d[last].Label != "Harko Village Taxi Service" || d[last-1].Label != "Arrakeen Taxi Service" || d[last-2].Group == groupTaxi {
		t.Errorf("%v", labelsOf(d))
	}
}

// A hub is another map: Teleport moves the character into it, to the arrival point only (#521).
func TestTeleportIntoAHubWritesTheThreeActorsAtTheArrivalPointAndNothingElse(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, teleportWorld)}
	if _, err := o.Teleport(Args{"x": 11495.0, "y": -316.0, "z": 51457.0, "map": "Arrakeen"}); err != nil {
		t.Fatal(err)
	}
	if got := playerActorMaps(t, o); got != "1:Arrakeen/0@11495,-316,51457 2:Arrakeen/0@11495,-316,51457 3:Arrakeen/0@11495,-316,51457" {
		t.Fatalf("the controller, the body and the player state: %s", got)
	}
	// the page puts the rounded numbers in the boxes: -1415, 26560, 14674 is still the arrival point of Harko Village
	if _, err := o.Teleport(Args{"x": -1415.0, "y": 26560.0, "z": 14674.0, "map": "HarkoVillage"}); err != nil {
		t.Fatal(err)
	}
	if got := playerActorMaps(t, o); !strings.HasPrefix(got, "1:HarkoVillage/0@-1415,26560,14674 ") {
		t.Fatalf("%s", got)
	}
	// the editor knows no bounds of a hub, so it moves into one only to the arrival point
	o = &Ops{S: testsave.PlayerWithSQL(t, teleportWorld)} // a character on Hagga Basin again, so that every case below is a move into another map
	before := playerActorMaps(t, o)
	for name, a := range map[string]Args{
		"a position that is not the arrival point": {"x": 5.0, "y": 5.0, "z": 5.0, "map": "Arrakeen"},
		"Harko Village at Arrakeen's point":        {"x": 11495.0, "y": -316.0, "z": 51457.0, "map": "HarkoVillage"},
		"Arrakeen, 5 units too high":               {"x": 11495.0, "y": -316.0, "z": 51462.0, "map": "Arrakeen"},
		"Arrakeen, 5 units off in X":               {"x": 11500.0, "y": -316.0, "z": 51457.0, "map": "Arrakeen"},
		"Arrakeen, 5 units off in Y":               {"x": 11495.0, "y": -311.0, "z": 51457.0, "map": "Arrakeen"},
		"a dungeon":                                {"x": 11495.0, "y": -316.0, "z": 51457.0, "map": "PitDungeon"},
	} {
		if _, err := o.Teleport(a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if playerActorMaps(t, o) != before {
		t.Errorf("a refused move changed the save: %s", playerActorMaps(t, o))
	}
	// and the list for a hub is still refused: the page lists places of a map the Map box offers
	if _, err := o.Destinations("", "Arrakeen"); err == nil {
		t.Error("the list of a hub map is not offered")
	}
}

// deep-desert-taxis.json is what the points come from, so what it holds is checked: the three taxi keys of place-names.json, each in row A of the grid the editor draws the Deep Desert on,
// the standing position beside the marker and in the same cell, finite numbers, an observed flag on each, and nothing for the other maps.
func TestDeepDesertTaxisFileIsWellFormed(t *testing.T) {
	hex32 := regexp.MustCompile(`^[0-9A-F]{32}$`)
	var taxiKeys []string
	for k, p := range placeNames {
		if p.Kind == kindTaxi {
			taxiKeys = append(taxiKeys, k)
		}
	}
	if len(taxiKeys) != 3 || len(deepDesertTaxis) != 3 {
		t.Fatalf("%d taxi keys in place-names.json, %d points", len(taxiKeys), len(deepDesertTaxis))
	}
	dd := findMap("DeepDesert")
	cw, ch := (dd.MaxX-dd.MinX)/9, (dd.MaxY-dd.MinY)/9
	cells := map[string]bool{}
	for _, k := range taxiKeys {
		tx, ok := deepDesertTaxis[k]
		if !ok || !hex32.MatchString(k) {
			t.Fatalf("no point for the taxi key %q", k)
		}
		for _, v := range []float64{tx.Marker.X, tx.Marker.Y, tx.Marker.Z, tx.Stand.X, tx.Stand.Y, tx.Stand.Z} {
			if math.IsNaN(v) || math.IsInf(v, 0) || v == 0 {
				t.Errorf("%s: %+v", k, tx)
			}
		}
		if tx.Observed == nil {
			t.Errorf("%s: no observed flag", k)
		} else if *tx.Observed {
			t.Errorf("%s: nobody has stood there yet, so observed is false", k)
		}
		// the Deep Desert grid of the editor: nine columns along X, nine rows along Y with A at the greatest Y
		cell := func(x, y float64) string {
			c, r := int(math.Floor((x-dd.MinX)/cw))+1, int(math.Floor((dd.MaxY-y)/ch))
			if c < 1 || c > 9 || r < 0 || r > 8 {
				return "outside"
			}
			return fmt.Sprintf("%c%d", 'A'+r, c)
		}
		if got := cell(tx.Marker.X, tx.Marker.Y); got != tx.Cell || !strings.HasPrefix(tx.Cell, "A") {
			t.Errorf("%s: the marker is in %s, the file says %q, and a Deep Desert taxi is in row A", k, got, tx.Cell)
		}
		if got := cell(tx.Stand.X, tx.Stand.Y); got != tx.Cell {
			t.Errorf("%s: the standing position is in %s, not in the marker's cell %s", k, got, tx.Cell)
		}
		if d := math.Hypot(tx.Stand.X-tx.Marker.X, tx.Stand.Y-tx.Marker.Y); d > 10000 {
			t.Errorf("%s: the standing position is %.0f units from the marker: not beside it", k, d)
		}
		if cells[tx.Cell] {
			t.Errorf("two points in %s", tx.Cell)
		}
		cells[tx.Cell] = true
	}
	// the grid in the file is the rectangle of the Deep Desert picture (livemap.go)
	var f struct {
		Grid struct {
			MinX, MaxX, MinY, MaxY float64
			Sectors                int
		}
	}
	if err := json.Unmarshal(rawDeepDesertTaxis, &f); err != nil {
		t.Fatal(err)
	}
	if f.Grid.MinX != dd.MinX || f.Grid.MaxX != dd.MaxX || f.Grid.MinY != dd.MinY || f.Grid.MaxY != dd.MaxY || f.Grid.Sectors != 9 {
		t.Errorf("the file's grid %+v is not the Deep Desert rectangle %+v", f.Grid, dd)
	}
	// the values, so that a change is seen
	for _, w := range deepDesertWant {
		tx := deepDesertTaxis[w.key]
		if tx.Stand.X != w.x || tx.Stand.Y != w.y || tx.Stand.Z != w.z || w.label != "Deep Desert Taxi Service ("+tx.Cell+")" {
			t.Errorf("%s: %+v", w.label, tx)
		}
	}
}

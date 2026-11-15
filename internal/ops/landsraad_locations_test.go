package ops

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Where a house is (#639): the board takes the place of a house from the save's own map markers "HouseRepresentative<Name>". The markers are written for the older layout
// (markers.map_name); testsave converts them for the newer one, so every test here runs on both.

const locationWorld = `
insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount, completed) values
  (176, 7, 2, 'DA_HouseMikkarol', 4000, 0), (177, 7, 3, 'DA_HouseVarota', 4000, 0), (178, 7, 4, 'DA_HouseWayku', 4000, 0), (179, 7, 5, 'DA_HouseImota', 4000, 0),
  (180, 7, 6, 'DA_HouseMikarrol', 4000, 0), (181, 7, 7, 'DA_HouseLindaren', 4000, 0), (182, 7, 8, 'DA_HouseNovebruns', 4000, 0), (183, 7, 9, 'DA_HouseTseida', 4000, 0), (184, 7, 10, 'DA_HouseSor', 4000, 0), (185, 7, 11, 'DA_HouseNobodyKnown', 4000, 0);
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,area_id) values
  (1,0,'HaggaBasin','HouseRepresentativeMaros',30100,40200,0,3),
  (2,0,'HaggaBasin','HouseRepresentativeMikarrol',-1000,-2000,0,7),
  (3,0,'Arrakeen','HouseRepresentativeVarota',3622,-3222,53844,0),
  (4,0,'DeepDesert','HouseRepresentativeWayku',1000,2000,0,0),
  (5,0,'HaggaBasin','ControlPointHouseImota',5,5,0,3),
  (6,0,'HaggaBasin','Cave',7,7,0,3),
  (10,0,'HaggaBasin','HouseRepresentativeLindaren',100,100,0,99),
  (11,0,'HaggaBasin','HouseRepresentativeNovebruns',100,null,0,3),
  (12,0,'HaggaBasin','HouseRepresentativeTseida',90000000,100,0,3),
  (13,0,'HaggaBasin','HouseRepresentativeSor',100,100,0,11),
  (14,0,'Arrakeen','HouseRepresentativeSor',100,100,0,0);
`

func locationOps(t *testing.T, l testsave.Layout, extra string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQLIn(t, l, mineWorld+locationWorld+extra)}
}

func eachLayout(t *testing.T, f func(t *testing.T, l testsave.Layout)) {
	for _, l := range testsave.Layouts {
		t.Run(l.String(), func(t *testing.T) { f(t, l) })
	}
}

func TestBoardLocationComesFromTheHouseRepresentativeMarker(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := locationOps(t, l, "")
		m := findMap("HaggaBasin")
		// the marker's area number 3 is a named area of the Hagga Basin; its place is a fraction of the map the Saved map draws at
		loc := taskByID(t, o, 174)["location"].(map[string]any)
		if loc["known"] != true || loc["map"] != "Hagga Basin" || loc["area"] != "Hagga Rift" || loc["map_key"] != "HaggaBasin" || loc["place"] != "" {
			t.Fatalf("Maros = %v", loc)
		}
		if x, y := loc["x"].(float64), loc["y"].(float64); math.Abs(x-(30100-m.MinX)/(m.MaxX-m.MinX)) > 1e-9 || math.Abs(y-(40200-m.MinY)/(m.MaxY-m.MinY)) > 1e-9 {
			t.Fatalf("the pin is not where the marker is: %v", loc)
		}
		// the save writes the house "Mikkarol", the marker "Mikarrol"
		if loc := taskByID(t, o, 176)["location"].(map[string]any); loc["known"] != true || loc["area"] != "Jabal Eifrit Al-janub" {
			t.Fatalf("Mikkarol = %v", loc)
		}
		// a hub has no picture: its name and map, no pin and no distance
		hub := taskByID(t, o, 177)["location"].(map[string]any)
		if hub["known"] != true || hub["map"] != "Arrakeen" || hub["area"] != "Arrakeen" || hub["map_key"] != "" || hub["x"] != nil || hub["y"] != nil || hub["from_you_cm"] != nil {
			t.Fatalf("Varota = %v", hub)
		}
		// the Deep Desert has a picture but no area names: the row says the grid tile the marker is in (a marker of the save wins over the fixed place of the house)
		if dd := taskByID(t, o, 178)["location"].(map[string]any); dd["known"] != true || dd["map"] != "Deep Desert" || dd["area"] != "Deep Desert" || dd["map_key"] != "DeepDesert" || dd["x"] == nil || dd["place"] != "tile E5" {
			t.Fatalf("Wayku = %v", dd)
		}
	})
}

func TestBoardLocationAliasWorksWithEitherSpelling(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := locationOps(t, l, "")
		// the save's "Mikkarol" (task 176) and the game's own "Mikarrol" (task 180) are the same house
		for _, id := range []int64{176, 180} {
			if loc := taskByID(t, o, id)["location"].(map[string]any); loc["known"] != true || loc["area"] != "Jabal Eifrit Al-janub" {
				t.Fatalf("task %d: %v", id, loc)
			}
		}
	})
}

// The area names below are the kit's table of the Hagga Basin areas (kit/gamedata/regions); a rename there changes these expectations.
func TestBoardLocationAreaFallsBackToTheMapName(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := locationOps(t, l, "")
		// area number 99 has no name in the table: the row says the map, not a guess
		if loc := taskByID(t, o, 181)["location"].(map[string]any); loc["known"] != true || loc["area"] != "Hagga Basin" || loc["map"] != "Hagga Basin" {
			t.Fatalf("unnamed area: %v", loc)
		}
	})
}

func TestBoardLocationWithOnlyOneCoordinateIsNoMarkerSoTheFixedPlaceIsUsed(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		// Novebruns' only marker has no y: it is no position, so the house gets its fixed place (Western Vermillius Gap)
		loc := taskByID(t, locationOps(t, l, ""), 182)["location"].(map[string]any)
		wantFixedPin(t, loc, "Novebruns", "Western Vermillius Gap")
	})
}

func TestBoardLocationOutsideThePictureGivesTheAreaButNoPinDistanceOrMapLink(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		loc := taskByID(t, locationOps(t, l, ""), 183)["location"].(map[string]any)
		if loc["known"] != true || loc["area"] != "Hagga Rift" || loc["map_key"] != "" || loc["x"] != nil || loc["y"] != nil || loc["from_you_cm"] != nil {
			t.Fatalf("a marker beyond the edge of the map: %v", loc)
		}
	})
}

func TestBoardLocationOfAHouseOnTwoMapsIsTheFirstByMapName(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := locationOps(t, l, "")
		for i := 0; i < 3; i++ {
			if loc := taskByID(t, o, 184)["location"].(map[string]any); loc["map"] != "Arrakeen" {
				t.Fatalf("read %d: %v", i, loc)
			}
		}
	})
}

// wantFixedPin checks a Hagga Basin location against the fixed table: the area, and the pin at the fixed position.
func wantFixedPin(t *testing.T, loc map[string]any, house, area string) {
	t.Helper()
	f := fixedHousePlaces[house]
	m := findMap("HaggaBasin")
	if loc["known"] != true || loc["map"] != "Hagga Basin" || loc["area"] != area || loc["map_key"] != "HaggaBasin" || loc["place"] != "" {
		t.Fatalf("%s = %v", house, loc)
	}
	if x, y := loc["x"].(float64), loc["y"].(float64); math.Abs(x-(f.X-m.MinX)/(m.MaxX-m.MinX)) > 1e-9 || math.Abs(y-(f.Y-m.MinY)/(m.MaxY-m.MinY)) > 1e-9 {
		t.Fatalf("%s: the pin is not at the fixed place: %v", house, loc)
	}
}

func TestBoardLocationOfAHouseWithoutAMarkerIsItsFixedPlace(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := locationOps(t, l, "")
		// Kenola has no marker; Imota has only a control point of its name, which is not its representative
		wantFixedPin(t, taskByID(t, o, 175)["location"].(map[string]any), "Kenola", "Mysa Tarill")
		imota := taskByID(t, o, 179)["location"].(map[string]any)
		wantFixedPin(t, imota, "Imota", "Jabal Eifrit Al-sharq")
		// the test character stands at (100, 200) on the Hagga Basin: the distance is worked out from the fixed place
		if d, ok := imota["from_you_cm"].(float64); !ok || math.Abs(d-math.Hypot(216722-100, -283171-200)) > 1 {
			t.Fatalf("distance = %v", imota)
		}
	})
}

func TestBoardLocationOfAHouseNeitherTheSaveNorTheTableHoldsStaysUnknown(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		loc := taskByID(t, locationOps(t, l, ""), 185)["location"].(map[string]any)
		if loc["known"] != false || len(loc) != 1 {
			t.Fatalf("location = %v: no position may be invented", loc)
		}
	})
}

func TestBoardLocationOfAHubOrDeepDesertHouseWithoutAMarker(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		// a world with a house of each kind of map and no marker at all
		o := &Ops{S: testsave.PlayerWithSQLIn(t, l, mineWorld+`insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount, completed) values
			(300, 7, 20, 'DA_HouseWallach', 4000, 0), (301, 7, 21, 'DA_HouseSpinette', 4000, 0), (303, 7, 23, 'DA_HouseWayku', 4000, 0),
			(304, 7, 24, 'DA_HouseTseida', 4000, 0), (305, 7, 25, 'DA_HouseMikkarol', 4000, 0);`)}
		for id, want := range map[int64][3]string{300: {"Arrakeen", "Arrakeen", ""}, 301: {"Harko Village", "Harko Village", ""}, 174: {"Deep Desert", "Deep Desert", "tile B8"}, 303: {"Deep Desert", "Deep Desert", "tile A2"},
			304: {"Hagga Basin", "Eastern Vermillius Gap", "The Anvil Tradepost"}, 305: {"Hagga Basin", "Jabal Eifrit Al-janub", ""}} {
			loc := taskByID(t, o, id)["location"].(map[string]any)
			if loc["known"] != true || loc["map"] != want[0] || loc["area"] != want[1] || loc["place"] != want[2] {
				t.Fatalf("task %d = %v, want %v", id, loc, want)
			}
			hub := want[0] == "Arrakeen" || want[0] == "Harko Village"
			if hub && (loc["map_key"] != "" || loc["x"] != nil || loc["from_you_cm"] != nil) {
				t.Fatalf("task %d: a hub has no picture, so no pin, link or distance: %v", id, loc)
			}
			if !hub && (loc["map_key"] == "" || loc["x"] == nil) {
				t.Fatalf("task %d: a place on a map with a picture gets a pin and the map link: %v", id, loc)
			}
		}
	})
}

func TestBoardLocationOfAMarkerBeatsTheFixedPlace(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		// Kenola's fixed place is in the Mysa Tarill; a marker in the save at another place (Sheol, area 11) is the answer
		o := locationOps(t, l, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,area_id) values (20,0,'HaggaBasin','HouseRepresentativeKenola',1000,-1000,0,11);`)
		m := findMap("HaggaBasin")
		loc := taskByID(t, o, 175)["location"].(map[string]any)
		if loc["area"] != "Sheol" || math.Abs(loc["x"].(float64)-(1000-m.MinX)/(m.MaxX-m.MinX)) > 1e-9 {
			t.Fatalf("the marker must win: %v", loc)
		}
	})
}

// The operator's rule: no house may lack a place or a way to its map. A save with no marker at all must give all 25 houses a place from the table alone, and each place must be one the
// editor can open: a map, and on a map with a picture the pin (a fraction inside the picture) and the map link, and the grid tile on the Deep Desert. The only places that cannot be opened
// on a picture are the two hubs, which the Saved map has no picture of yet (#758); the test names them so that adding a hub map makes it fail until the list is shortened.
func TestEveryHouseHasAPlaceAndAWayToItsMap(t *testing.T) {
	hubsWithoutPicture := map[string]bool{"Arrakeen": true, "Harko Village": true}
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		var ids = map[string]int64{"Maros": 174, "Kenola": 175} // the two houses mineWorld already has
		var insert []string
		next := int64(400)
		for name := range fixedHousePlaces {
			saveName := name
			for save, marker := range houseMarkerName {
				if marker == name {
					saveName = save
				}
			}
			if _, ok := ids[name]; ok {
				continue
			}
			ids[name] = next
			insert = append(insert, fmt.Sprintf("(%d, 7, %d, 'DA_House%s', 4000, 0)", next, next, saveName))
			next++
		}
		o := &Ops{S: testsave.PlayerWithSQLIn(t, l, mineWorld+"insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount, completed) values "+strings.Join(insert, ", ")+";")}
		if len(ids) != 25 {
			t.Fatalf("%d houses, want 25", len(ids))
		}
		for name, id := range ids {
			loc := taskByID(t, o, id)["location"].(map[string]any)
			if loc["known"] != true {
				t.Errorf("House %s has no place with no marker in the save: %v", name, loc)
				continue
			}
			label, _ := loc["map"].(string)
			if label == "" || loc["area"] == "" {
				t.Errorf("House %s: no map or area: %v", name, loc)
			}
			if hubsWithoutPicture[label] {
				if loc["map_key"] != "" || loc["x"] != nil {
					t.Errorf("House %s: a hub has no picture to pin on: %v", name, loc)
				}
				continue
			}
			x, okx := loc["x"].(float64)
			y, oky := loc["y"].(float64)
			if loc["map_key"] == "" || !okx || !oky || x < 0 || x > 1 || y < 0 || y > 1 {
				t.Errorf("House %s: its place cannot be opened on its map: %v", name, loc)
			}
			if label == "Deep Desert" && !strings.HasPrefix(fmt.Sprint(loc["place"]), "tile ") {
				t.Errorf("House %s: a Deep Desert house names its tile: %v", name, loc)
			}
		}
	})
}

func TestFixedHousePlacesHoldEveryHouse(t *testing.T) {
	entries, err := os.ReadDir("../web/static/houses") // one symbol per house of the Landsraad
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 25 || len(fixedHousePlaces) != 25 {
		t.Fatalf("%d symbols, %d fixed places: want 25 of each", len(entries), len(fixedHousePlaces))
	}
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".webp")
		name = strings.ToUpper(name[:1]) + name[1:]
		if canon := canonicalHouse(name); fixedHousePlaces[canon].Map == "" {
			t.Errorf("no fixed place for House %s", name)
		}
	}
	for name, f := range fixedHousePlaces {
		if _, ok := houseMapLabel[f.Map]; !ok {
			t.Errorf("%s: %q is not a map a representative stands on", name, f.Map)
		}
		l := placeAt(f.Map, f.X, f.Y, f.Area, f.Place)
		switch f.Map {
		case "HaggaBasin":
			if !l.Pinned || l.Area == "Hagga Basin" {
				t.Errorf("%s: a Hagga Basin place has a pin inside the picture and a named area: %+v", name, l)
			}
		case "DeepDesert":
			if !l.Pinned || !strings.HasPrefix(l.Place, "tile ") {
				t.Errorf("%s: a Deep Desert place has a pin and a grid tile: %+v", name, l)
			}
		default:
			if l.Pinned || l.Area != l.Map {
				t.Errorf("%s: a hub has no picture: %+v", name, l)
			}
		}
	}
}

func TestBoardLocationIgnoresMarkersOnOtherMapsAndWithoutAPosition(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := locationOps(t, l, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,area_id) values
			(7,0,'LostHarvest','HouseRepresentativeKenola',1,1,0,0), (8,0,'HaggaBasin','HouseRepresentativeKenola',null,null,0,0), (9,0,'HaggaBasin','houserepresentativeKenola',1,1,0,0);`)
		// none of the three markers is a place of the house, so the house gets its fixed place and not (1, 1)
		wantFixedPin(t, taskByID(t, o, 175)["location"].(map[string]any), "Kenola", "Mysa Tarill")
	})
}

func TestBoardLocationWithTwoMarkersForOneHouseIsTheSameEveryRead(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := locationOps(t, l, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,area_id) values (0,0,'HaggaBasin','HouseRepresentativeMaros',-9000,-9000,0,11);`)
		for i := 0; i < 3; i++ {
			if loc := taskByID(t, o, 174)["location"].(map[string]any); loc["area"] != "Sheol" {
				t.Fatalf("read %d: %v", i, loc)
			}
		}
	})
}

func TestBoardLocationGivesTheDistanceFromYouOnlyOnTheSameMapWithAMapping(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		// the test character stands at (100, 200) on the Hagga Basin; the marker of House Maros is at (30100, 40200): 30,000 and 40,000 centimetres away along the two axes
		o := locationOps(t, l, "")
		d, ok := taskByID(t, o, 174)["location"].(map[string]any)["from_you_cm"].(float64)
		if !ok || d < 49999 || d > 50001 {
			t.Fatalf("distance = %v", taskByID(t, o, 174)["location"])
		}
		for _, id := range []int64{177, 178} { // a hub has no mapping; the Deep Desert is another map than the character's
			if loc := taskByID(t, o, id)["location"].(map[string]any); loc["known"] != true || loc["from_you_cm"] != nil {
				t.Fatalf("task %d: another map or a hub has no distance: %v", id, loc)
			}
		}
	})
}

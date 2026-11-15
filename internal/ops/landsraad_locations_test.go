package ops

import (
	"math"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Where a house is (#639): the board takes the place of a house from the save's own map markers "HouseRepresentative<Name>". The markers are written for the older layout
// (markers.map_name); testsave converts them for the newer one, so every test here runs on both.

const locationWorld = `
insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount, completed) values
  (176, 7, 2, 'DA_HouseMikkarol', 4000, 0), (177, 7, 3, 'DA_HouseVarota', 4000, 0), (178, 7, 4, 'DA_HouseWayku', 4000, 0), (179, 7, 5, 'DA_HouseImota', 4000, 0),
  (180, 7, 6, 'DA_HouseMikarrol', 4000, 0), (181, 7, 7, 'DA_HouseLindaren', 4000, 0), (182, 7, 8, 'DA_HouseNovebruns', 4000, 0), (183, 7, 9, 'DA_HouseTseida', 4000, 0), (184, 7, 10, 'DA_HouseSor', 4000, 0);
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
		// the Deep Desert has a picture but no area names
		if dd := taskByID(t, o, 178)["location"].(map[string]any); dd["known"] != true || dd["map"] != "Deep Desert" || dd["area"] != "Deep Desert" || dd["map_key"] != "DeepDesert" || dd["x"] == nil {
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

func TestBoardLocationWithOnlyOneCoordinateIsUnknown(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		if loc := taskByID(t, locationOps(t, l, ""), 182)["location"].(map[string]any); loc["known"] != false {
			t.Fatalf("a marker with no y is no position: %v", loc)
		}
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

func TestBoardLocationOfAHouseWithoutAMarkerIsUnknownAndNothingElse(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := locationOps(t, l, "")
		// Kenola has no marker, Imota has only a control point of its name, which is not its representative
		for _, id := range []int64{175, 179} {
			loc := taskByID(t, o, id)["location"].(map[string]any)
			if loc["known"] != false || len(loc) != 1 {
				t.Fatalf("task %d: location = %v: no position may be invented", id, loc)
			}
		}
	})
}

func TestBoardLocationIgnoresMarkersOnOtherMapsAndWithoutAPosition(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := locationOps(t, l, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z,area_id) values
			(7,0,'LostHarvest','HouseRepresentativeKenola',1,1,0,0), (8,0,'HaggaBasin','HouseRepresentativeKenola',null,null,0,0), (9,0,'HaggaBasin','houserepresentativeKenola',1,1,0,0);`)
		if loc := taskByID(t, o, 175)["location"].(map[string]any); loc["known"] != false {
			t.Fatalf("a marker on a map no representative stands on, or with no position, is no place: %v", loc)
		}
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

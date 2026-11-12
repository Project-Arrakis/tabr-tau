package ops

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #588: the game's save names the map of a marker in two ways. Until its update of early November 2026, markers and player_markers have the text column map_name; since, they
// have map_name_id and the names are in map_names. Every read of the markers gives the same answer from a save of either layout. The worlds are the ones the other tests of this
// package use (written for the older layout; testsave converts them for the newer one), with synthetic rows only.

// A save of each layout, and a check that it is the layout it is meant to be.
func inEachLayout(t *testing.T, world string, check func(t *testing.T, o *Ops)) {
	t.Helper()
	for _, l := range testsave.Layouts {
		t.Run(l.String(), func(t *testing.T) {
			o := &Ops{S: testsave.PlayerWithSQLIn(t, l, world)}
			if got := o.S.MarkersMapNameID(); got != (l == testsave.LayoutMapNameID) {
				t.Fatalf("layout %v: the editor sees the newer layout: %v", l, got)
			}
			rows, err := o.S.Query(`select name from pragma_table_info('markers') where name in ('map_name','map_name_id')`)
			if err != nil || len(rows) != 1 || rows[0]["name"] != l.String() {
				t.Fatalf("layout %v: the markers table has %v (%v)", l, rows, err)
			}
			if n := o.S.SchemaNote(); n != "" {
				t.Fatalf("a save of a known layout has no schema note: %q", n)
			}
			check(t, o)
		})
	}
}

func TestTheSavedMapShowsMarkersAndDiscoveryInEitherLayout(t *testing.T) {
	inEachLayout(t, mapWorld, func(t *testing.T, o *Ops) {
		r, err := o.LiveMap("HaggaBasin")
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int64{}
		for _, x := range r.(map[string]any)["markers"].([]map[string]any) {
			got[x["t"].(string)] = x["d"].(int64)
		}
		if want := (map[string]int64{"AzuritePickup": 3, "EnemyCamp": 0}); !reflect.DeepEqual(got, want) {
			t.Fatalf("markers of the Hagga Basin with their discovery: %v, want %v", got, want)
		}
		r, err = o.LiveMap("DeepDesert")
		if err != nil {
			t.Fatal(err)
		}
		if mk := r.(map[string]any)["markers"].([]map[string]any); len(mk) != 1 || mk[0]["t"] != "Cave" || mk[0]["d"].(int64) != 0 {
			t.Fatalf("Deep Desert markers: %v", mk)
		}
	})
}

// A marker's hash is not unique across maps: the same hash on two maps is two markers, and what a character discovered on one is not discovered on the other.
func TestDiscoveryBelongsToTheMapOfTheMarkerInEitherLayout(t *testing.T) {
	world := mapWorld + `
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values (1,0,'DeepDesert','AzuritePickup',7,8,0);`
	inEachLayout(t, world, func(t *testing.T, o *Ops) {
		r, err := o.LiveMap("DeepDesert")
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range r.(map[string]any)["markers"].([]map[string]any) {
			if x["t"] == "AzuritePickup" && x["d"].(int64) != 0 {
				t.Fatalf("the Hagga Basin's discovery of marker 1 shows on the Deep Desert's marker 1: %v", x)
			}
		}
		r, _ = o.LiveMap("HaggaBasin")
		found := false
		for _, x := range r.(map[string]any)["markers"].([]map[string]any) {
			found = found || x["t"] == "AzuritePickup" && x["d"].(int64) == 3
		}
		if !found {
			t.Fatal("the discovered Hagga Basin marker lost its discovery")
		}
	})
}

func TestRegionsOfPlacesComeFromMarkersInEitherLayout(t *testing.T) {
	inEachLayout(t, regionWorld, func(t *testing.T, o *Ops) {
		r, err := o.LiveMap("HaggaBasin")
		if err != nil {
			t.Fatal(err)
		}
		m := r.(map[string]any)
		if c := m["character"].(map[string]any); c["region"] != "Hagga Rift" {
			t.Errorf("character: %v", c)
		}
		if v := m["vehicles"].([]map[string]any); len(v) != 1 || v[0]["region"] != "Hagga Rift" {
			t.Errorf("vehicles: %v", v)
		}
	})
}

func TestDiscoveredTradingPostsAndTaxisAreListedInEitherLayout(t *testing.T) {
	var answers [][]string
	for _, l := range testsave.Layouts {
		o := &Ops{S: testsave.PlayerWithSQLIn(t, l, placesWorld)}
		_, d := namedPlacesOf(t, o, "", "")
		_, dd := namedPlacesOf(t, o, "", "DeepDesert")
		labels := append(labelsOf(d), labelsOf(dd)...)
		if len(d) == 0 {
			t.Fatalf("%v: no destination at all", l)
		}
		posts := 0
		for _, s := range labels {
			if strings.HasPrefix(s, groupTradePost+": ") {
				posts++
			}
		}
		if posts == 0 {
			t.Fatalf("%v: no trading post among %v", l, labels)
		}
		answers = append(answers, labels)
	}
	if !reflect.DeepEqual(answers[0], answers[1]) {
		t.Fatalf("the destinations differ between the layouts:\n%v\n%v", answers[0], answers[1])
	}
}

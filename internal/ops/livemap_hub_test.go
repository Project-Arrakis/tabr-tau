package ops

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The Saved map opens the two social hubs (#758): Arrakeen and Harko Village are maps of the Saved map (the selector, the frame that turns a world position into a place on the map, the house
// representatives that stand there), and they are not maps the editor moves the character to (travel keeps its two maps).

func TestSavedMapsAreTheTwoMapsAndTheTwoHubsAndTravelKeepsItsTwo(t *testing.T) {
	var keys []string
	for _, m := range savedMaps() {
		keys = append(keys, m.Key)
	}
	if strings.Join(keys, ",") != "HaggaBasin,DeepDesert,Arrakeen,HarkoVillage" {
		t.Fatalf("the Saved map offers %v", keys)
	}
	if len(liveMaps) != 2 {
		t.Fatalf("liveMaps is the travel list, two maps: %v", liveMaps)
	}
	for _, hub := range []string{"Arrakeen", "HarkoVillage"} {
		if _, ok := travelMapOf(hub); ok {
			t.Errorf("%s is a map to look at, not a map the character is moved to", hub)
		}
	}
	if findMap("HarkoVillage").Key != "HarkoVillage" || findMap("Arrakeen").Key != "Arrakeen" || findMap("nonsense").Key != "HaggaBasin" {
		t.Error("findMap finds a hub by its key and falls back to the Hagga Basin")
	}
}

// The frame of a hub: the scale is the same in both directions (a picture of the frame is not stretched), and the five house representatives of the table lie inside it. The residual of the
// transform is printed: a position turned into a fraction of the frame and back is the same position, and the margin of each house to the nearest edge is the distance a frame that was
// wrong by that much would lose the house at.
func TestHubFramesAreNotStretchedAndHoldTheirHouses(t *testing.T) {
	for _, m := range hubMaps {
		sx, sy := (m.MaxX-m.MinX)/float64(m.Width), (m.MaxY-m.MinY)/float64(m.Height)
		if math.Abs(sx/sy-1) > 0.001 {
			t.Errorf("%s: %.4f centimetres to the pixel across and %.4f down: the frame is stretched", m.Key, sx, sy)
		}
		t.Logf("%s: frame x %.2f to %.2f, y %.2f to %.2f; %.4f centimetres to the pixel (across) and %.4f (down) of %d x %d", m.Key, m.MinX, m.MaxX, m.MinY, m.MaxY, sx, sy, m.Width, m.Height)
	}
	houses := 0
	for name, f := range fixedHousePlaces {
		if !isHubMap(f.Map) {
			continue
		}
		houses++
		m := findMap(f.Map)
		l := placeAt(f.Map, f.X, f.Y, f.Area, f.Place)
		if !l.Pinned {
			t.Errorf("%s: not inside the frame of %s", name, f.Map)
			continue
		}
		// the same two steps as the board (a fraction) and the Saved map (a pixel of the map space), and the way back
		px, py := l.X*float64(m.Width), l.Y*float64(m.Height)
		bx, by := m.MinX+px/float64(m.Width)*(m.MaxX-m.MinX), m.MinY+py/float64(m.Height)*(m.MaxY-m.MinY)
		res := math.Hypot(bx-f.X, by-f.Y)
		if res > 1e-6 {
			t.Errorf("%s: the transform does not come back to the position: %.9f centimetres off", name, res)
		}
		margin := math.Min(math.Min(f.X-m.MinX, m.MaxX-f.X), math.Min(f.Y-m.MinY, m.MaxY-f.Y))
		t.Logf("%-9s %-12s world (%7.0f, %7.0f) -> pixel (%7.1f, %7.1f) of %d x %d, fraction (%.5f, %.5f), round trip residual %.1e cm, %6.0f cm to the nearest edge", name, f.Map, f.X, f.Y, px, py, m.Width, m.Height, l.X, l.Y, res, margin)
	}
	if houses != 5 {
		t.Errorf("%d house representatives stand in the hubs, want 5 (Mutelli, Varota, Wallach in Arrakeen; Alexin and Spinette in Harko Village)", houses)
	}
}

func TestLiveMapOfAHubOffersTheFourMapsAndGivesTheHubsConfig(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		o := &Ops{S: testsave.PlayerWithSQLIn(t, l, mineWorld)}
		for key, want := range map[string]mapConfig{"Arrakeen": hubMaps[0], "HarkoVillage": hubMaps[1]} {
			r, err := o.LiveMap(key)
			if err != nil {
				t.Fatal(err)
			}
			m := r.(map[string]any)
			if cfg := m["config"].(mapConfig); cfg != want || cfg.Key != key {
				t.Fatalf("%s: config %+v", key, cfg)
			}
			var got []string
			for _, mp := range m["maps"].([]map[string]any) {
				got = append(got, mp["key"].(string)+"="+mp["label"].(string))
			}
			if strings.Join(got, ",") != "HaggaBasin=Hagga Basin,DeepDesert=The Deep Desert,Arrakeen=Arrakeen,HarkoVillage=Harko Village" {
				t.Fatalf("%s: the selector is %v", key, got)
			}
			// the character stands on the Hagga Basin: it is not drawn on a hub
			if m["character"] != nil {
				t.Errorf("%s: the character is not here: %v", key, m["character"])
			}
		}
	})
}

// The house representatives of a hub: the table puts every representative that stands there on the hub's map, as an undiscovered marker, unless the save holds a marker for the house on that map
// (the save's marker wins); the other maps show only the save's markers.
func TestLiveMapOfAHubShowsTheRepresentativesOfItsTableAndTheSavesMarkerWins(t *testing.T) {
	eachLayout(t, func(t *testing.T, l testsave.Layout) {
		reps := func(r any) map[string][3]float64 {
			out := map[string][3]float64{}
			for _, mk := range r.(map[string]any)["markers"].([]map[string]any) {
				if ty, _ := mk["t"].(string); strings.HasPrefix(ty, houseMarkerPrefix) {
					x, _ := mk["x"].(float64)
					y, _ := mk["y"].(float64)
					d, _ := toFloat(mk["d"])
					out[strings.TrimPrefix(ty, houseMarkerPrefix)] = [3]float64{x, y, d}
				}
			}
			return out
		}
		o := &Ops{S: testsave.PlayerWithSQLIn(t, l, mineWorld)}
		r, _ := o.LiveMap("Arrakeen")
		got := reps(r)
		want := map[string][3]float64{"Mutelli": {3622, -3222, 0}, "Varota": {-8708, -7988, 0}, "Wallach": {-27134, 2864, 0}}
		if len(got) != 3 {
			t.Fatalf("Arrakeen: %v, want the three Houses of its table", got)
		}
		for name, w := range want {
			if got[name] != w {
				t.Errorf("Arrakeen %s: %v, want %v", name, got[name], w)
			}
		}
		r, _ = o.LiveMap("HarkoVillage")
		if got := reps(r); len(got) != 2 || got["Alexin"] != [3]float64{9978, 11503, 0} || got["Spinette"] != [3]float64{3216, 9403, 0} {
			t.Errorf("Harko Village: %v, want Alexin and Spinette at their table places", got)
		}
		// the other maps keep the save's own markers only: no House of the table is added to the Hagga Basin or the Deep Desert
		r, _ = o.LiveMap("HaggaBasin")
		if got := reps(r); len(got) != 0 {
			t.Errorf("Hagga Basin: %v: only a hub gets the table's representatives", got)
		}

		// a marker of the save for Varota, at another place and found (discovery 3), is the one that is drawn; the table adds only the two Houses the save has no marker for
		o = &Ops{S: testsave.PlayerWithSQLIn(t, l, mineWorld+`insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values (31,0,'Arrakeen','HouseRepresentativeVarota',-8000,-7000,0);
			insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values (1,31,0,'Arrakeen',3,1);`)}
		r, _ = o.LiveMap("Arrakeen")
		got = reps(r)
		if len(got) != 3 || got["Varota"] != [3]float64{-8000, -7000, 3} || got["Mutelli"] != want["Mutelli"] || got["Wallach"] != want["Wallach"] {
			t.Errorf("Arrakeen with a marker for Varota: %v", got)
		}
		n := 0
		for _, mk := range r.(map[string]any)["markers"].([]map[string]any) {
			if ty, _ := mk["t"].(string); ty == "HouseRepresentativeVarota" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("Varota is drawn %d times, want once", n)
		}
	})
}

// A map's picture, its manifest entry and the notice stay in step (the web package holds the other half: every picture in the folder is in the manifest and served): a map whose Image is set has
// its file in internal/web/static and in the manifest's mapArt files; a map whose Image is empty is in the manifest's withoutPicture list, and only such maps are.
func TestEveryMapPictureIsInTheManifestAndEveryMapWithoutOneIsSaidSo(t *testing.T) {
	b, err := os.ReadFile("../../docs/design/landsraad-command-center/art-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var man struct {
		MapArt struct {
			Files []struct {
				Map  string `json:"map"`
				File string `json:"file"`
			} `json:"files"`
			WithoutPicture []string `json:"withoutPicture"`
		} `json:"mapArt"`
	}
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	listed := map[string]string{} // the map's key -> the picture's path as the page asks for it
	for _, f := range man.MapArt.Files {
		listed[f.Map] = "/maps/" + f.File
	}
	without := map[string]bool{}
	for _, k := range man.MapArt.WithoutPicture {
		without[k] = true
	}
	var missing []string
	for _, m := range savedMaps() {
		switch {
		case m.Image == "" && !without[m.Key]:
			t.Errorf("%s has no picture but the manifest does not say so (mapArt.withoutPicture)", m.Key)
		case m.Image != "" && without[m.Key]:
			t.Errorf("%s has a picture and the manifest lists it as without one", m.Key)
		case m.Image != "":
			if listed[m.Key] != m.Image {
				t.Errorf("%s: the picture %s is not the one the manifest lists (%q)", m.Key, m.Image, listed[m.Key])
			}
			if _, err := os.Stat("../web/static" + m.Image); err != nil {
				t.Errorf("%s: %v", m.Key, err)
			}
		}
		if m.Image == "" {
			missing = append(missing, m.Key)
		}
	}
	sort.Strings(missing)
	t.Logf("maps without a picture: %v", missing)
	for k := range without {
		if findMap(k).Key != k {
			t.Errorf("withoutPicture names %s, which is not a map of the Saved map", k)
		}
	}
}

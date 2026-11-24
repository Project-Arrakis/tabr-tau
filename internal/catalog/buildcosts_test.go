package catalog

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// The building costs (#1176) are ids and whole numbers above 0, and every type has at least one item.
func TestBuildCostsAreSane(t *testing.T) {
	if len(buildCosts.Pieces) < 700 || len(buildCosts.Placeables) < 500 {
		t.Fatalf("only %d piece types and %d placeable types", len(buildCosts.Pieces), len(buildCosts.Placeables))
	}
	for kind, types := range map[string]map[string]map[string]int64{"piece": buildCosts.Pieces, "placeable": buildCosts.Placeables} {
		for typ, cost := range types {
			if !idRe.MatchString(typ) || len(cost) == 0 {
				t.Errorf("%s %q: not an id, or no cost", kind, typ)
			}
			for item, n := range cost {
				if !idRe.MatchString(item) || n <= 0 {
					t.Errorf("%s %s costs %d of %q", kind, typ, n, item)
				}
			}
		}
	}
	if BuildCostsBuild() == "" || buildCosts.Read == "" {
		t.Error("the file does not say which game build it was read from, or when")
	}
}

func TestKnownBuildCosts(t *testing.T) {
	for typ, want := range map[string]string{
		"Sardaukar_Wall_01":                 "map[Stone:10]",
		"Sardaukar_Foundation":              "map[Stone:15]",
		"Sardaukar_Door_Frame_Tall_L":       "map[Stone:7]",
		"Harkonnen_Outpost_Door_Frame_Wide": "map[Plastone:25]",
	} {
		if got, ok := PieceCost(typ); !ok || fmt.Sprint(got) != want {
			t.Errorf("%s: %v %v, want %s", typ, got, ok, want)
		}
	}
	for typ, want := range map[string]string{
		"Totem_Placeable":                      "map[ScrapMetal:40]",
		"SpiceSilo_Placeable":                  "map[ScrapMetal:35]",
		"MTX_Sardaukar_TallDoor_Placeable_L":   "map[Stone:10]",
		"Developer_StorageContainer_Placeable": "map[IronBar:20]",
		"LargeWaterCistern_Placeable":          "map[DuraluminumRod:115 Silicone:120 T4HarkSpiceSiloComponent1:20]",
		"B1C5_MTX_Sardaukar_Banner_Placeable":  "map[Plastone:50 SaguaroResourceRaw:15]",
	} {
		if got, ok := PlaceableCost(typ); !ok || fmt.Sprint(got) != want {
			t.Errorf("%s: %v %v, want %s", typ, got, ok, want)
		}
	}
	// a type the tables give no cost has no answer; a piece type is not a placeable type
	if _, ok := PlaceableCost("Ladder_Placeable"); ok {
		t.Error("Ladder_Placeable has no cost in the game's tables")
	}
	if _, ok := PieceCost("Totem_Placeable"); ok {
		t.Error("a placeable type was found among the pieces")
	}
	if _, ok := PieceCost("No_Such_Piece"); ok {
		t.Error("an unknown type has a cost")
	}
	if PlacementCostPercent("DeepDesert") != 50 || PlacementCostPercent("HaggaBasin") != 100 || PlacementCostPercent("") != 100 {
		t.Error("a piece costs half in the Deep Desert and the full cost elsewhere")
	}
}

// The base the game's own recycle was measured on (2026-11-23): 23 pieces, a totem, a door and a silo. Its table cost is Stone 242 and ScrapMetal 75; the game returned Stone 125
// and ScrapMetal 37, about half.
func TestTheMeasuredBaseCostsWhatTheResearchFound(t *testing.T) {
	total := map[string]int64{}
	for typ, n := range map[string]int64{"Sardaukar_Wall_Round_Corner": 8, "Sardaukar_Floor_Round_Corner_02": 4, "Sardaukar_Roof_Round_Corner": 4, "Sardaukar_Roof_03_L": 2, "Sardaukar_Wall_01": 2,
		"Sardaukar_Door_Frame_Tall_L": 1, "Sardaukar_Floor_02": 1, "Sardaukar_Foundation": 1} {
		cost, ok := PieceCost(typ)
		if !ok {
			t.Fatalf("%s has no cost", typ)
		}
		for item, c := range cost {
			total[item] += c * n
		}
	}
	for _, typ := range []string{"MTX_Sardaukar_TallDoor_Placeable_L", "SpiceSilo_Placeable", "Totem_Placeable"} {
		cost, ok := PlaceableCost(typ)
		if !ok {
			t.Fatalf("%s has no cost", typ)
		}
		for item, c := range cost {
			total[item] += c
		}
	}
	if fmt.Sprint(total) != "map[ScrapMetal:75 Stone:242]" {
		t.Fatalf("the measured base costs %v, want Stone 242 and ScrapMetal 75", total)
	}
}

func TestBuildCostsProvenanceMatchesTheData(t *testing.T) {
	b, err := os.ReadFile("PROVENANCE.md")
	if err != nil {
		t.Fatal(err)
	}
	p := string(b)
	for _, want := range []string{
		"`buildcosts.json`",
		fmt.Sprintf("%d building piece types and %d placeable types", len(buildCosts.Pieces), len(buildCosts.Placeables)),
		"build `" + buildCosts.Build + "`",
		buildCosts.Read,
	} {
		if !strings.Contains(p, want) {
			t.Errorf("PROVENANCE.md does not contain %q; update it with the data", want)
		}
	}
}

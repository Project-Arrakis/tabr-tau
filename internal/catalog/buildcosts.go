package catalog

import (
	_ "embed"
	"encoding/json"
)

// buildcosts.json says what one building piece and one placeable costs to place, as the game's own building data tables list it (#1176): for every type, item template ids
// and whole numbers. It also holds, for the maps that have their own, the share of the cost a piece costs there (the Deep Desert: 50 percent). It holds ids and whole numbers
// only. tools/buildcostsgen makes it from dumps of the tables; PROVENANCE.md says which tables and how.
//
//go:embed buildcosts.json
var rawBuildCosts []byte

var buildCosts struct {
	Build                string                      `json:"build"`
	Read                 string                      `json:"read"`
	PlacementCostPercent map[string]int64            `json:"placementCostPercent"`
	Pieces               map[string]map[string]int64 `json:"pieces"`
	Placeables           map[string]map[string]int64 `json:"placeables"`
}

func init() {
	if err := json.Unmarshal(rawBuildCosts, &buildCosts); err != nil {
		panic("catalog: buildcosts.json is not valid: " + err.Error())
	}
}

// PieceCost returns what one building piece of this type (building_instances.building_type) costs to place: item template id to amount. The second value is false when the
// game's tables give the type no cost; that is a gap in the data, never a piece that costs nothing. The map must not be changed.
func PieceCost(buildingType string) (map[string]int64, bool) {
	c, ok := buildCosts.Pieces[buildingType]
	return c, ok && len(c) > 0
}

// PlaceableCost is PieceCost for a placeable type (placeables.building_type).
func PlaceableCost(buildingType string) (map[string]int64, bool) {
	c, ok := buildCosts.Placeables[buildingType]
	return c, ok && len(c) > 0
}

// PlacementCostPercent returns the share of the table cost that a piece or a placeable costs on this map, in percent: 100 for every map but those the game's map table gives
// their own value (the Deep Desert: 50).
func PlacementCostPercent(mapName string) int64 {
	if p, ok := buildCosts.PlacementCostPercent[mapName]; ok && p > 0 {
		return p
	}
	return 100
}

// BuildCostsBuild returns the game build the building costs were read from.
func BuildCostsBuild() string { return buildCosts.Build }

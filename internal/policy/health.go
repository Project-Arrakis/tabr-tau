// Package policy holds the numeric limits of the editor's two users of the game's figures: the check of a base file on import (internal/basefile) and the repair functions
// (internal/ops). They used to be kept in the file-format package, which the repair then imported; one small package with no imports of its own keeps the two in step (#613, audit
// G-045).
package policy

import "math"

// Health (#613, audit G-009).
//
// The caps were measured on one real save (3,457 building pieces of 70 types, 230 placeables of 45 types, all standing, none in a backup). The figures in the two tables below are the
// most health seen on any piece of the type, for each type that has at least five pieces in that save; the cap of a type is that figure times HealthCapMargin. A type the table does
// not list (fewer than five samples, or a type the save does not hold) is held to the overall cap of its table: the most seen on any piece of the table, times the same margin.
// Nothing here models the game: the game's tables hold no maximum health per piece, so the caps are the highest figure seen with room above it, and a file or a repair that
// would go past one is refused (import) or held to it (repair), never lowered.
const (
	// HealthCapMargin is the room above the highest figure seen.
	HealthCapMargin = 1.25
	// PieceHealthSeen and PlaceableHealthSeen are the highest health seen on any building piece (7,000) and on any placeable (20,000, one large refinery; the next is 4,000).
	PieceHealthSeen     = 7000.0
	PlaceableHealthSeen = 20000.0
	// MaxRepairPieceHealth and MaxRepairPlaceableHealth are the overall caps of the two tables.
	MaxRepairPieceHealth     = PieceHealthSeen * HealthCapMargin
	MaxRepairPlaceableHealth = PlaceableHealthSeen * HealthCapMargin
	// MaxPieceHealth is the outer ceiling of a health figure of any piece or placeable (a hologram has no measured figure and is held to this). It is far above every cap; a health above it is
	// never "the best seen" by the repair, and a file is refused for it.
	MaxPieceHealth = 1_000_000.0
)

// PieceTypeHealthSeen is the highest health seen per building type, for the types with at least five pieces in the measured save.
var PieceTypeHealthSeen = map[string]float64{
	"MTX_B1C5_Choam_Level2Extention_Floor_01":   5600,
	"MTX_Smug_Floor_Wedge":                      5600,
	"Sardaukar_Column_corner":                   5600,
	"Sardaukar_Door_Frame_Wide":                 3500,
	"Sardaukar_Floor":                           5600,
	"Sardaukar_Floor_02":                        5600,
	"Sardaukar_Floor_Round_Corner":              5600,
	"Sardaukar_Floor_Round_Corner_02":           5600,
	"Sardaukar_Floor_Round_Inverted":            5600,
	"Sardaukar_Floor_Wedge":                     5600,
	"Sardaukar_Foundation":                      7000,
	"Sardaukar_Foundation_Round_Corner":         7000,
	"Sardaukar_Foundation_Wedge":                7000,
	"Sardaukar_Pillar_Bottom":                   7000,
	"Sardaukar_Pillar_Middle":                   7000,
	"Sardaukar_Pillar_Top":                      7000,
	"Sardaukar_Railing":                         5600,
	"Sardaukar_Railing_Gate":                    5600,
	"Sardaukar_Railing_Inclined":                5600,
	"Sardaukar_Railing_Inclined_Half":           5600,
	"Sardaukar_Railing_Round_Corner":            5600,
	"Sardaukar_Ramp":                            5600,
	"Sardaukar_Ramp_Half":                       5586.25,
	"Sardaukar_Roof_01":                         5600,
	"Sardaukar_Roof_02":                         5600,
	"Sardaukar_Roof_02_L":                       5600,
	"Sardaukar_Roof_03_L":                       5600,
	"Sardaukar_Roof_Round_Corner":               5600,
	"Sardaukar_Roof_Wedge_Bottom":               5600,
	"Sardaukar_Roof_Wedge_Top":                  5600,
	"Sardaukar_Rooftop_02":                      5600,
	"Sardaukar_Rooftop_Round_Corner":            5600,
	"Sardaukar_Rooftop_Wedge":                   5600,
	"Sardaukar_Stairs":                          5600,
	"Sardaukar_Wall_01":                         5600,
	"Sardaukar_Wall_02":                         5600,
	"Sardaukar_Wall_02_L":                       5600,
	"Sardaukar_Wall_Half":                       5586.25,
	"Sardaukar_Wall_Inclined_Corner_Tall":       5585,
	"Sardaukar_Wall_Inclined_Tall_01":           5600,
	"Sardaukar_Wall_Inclined_Tall_01_L":         5600,
	"Sardaukar_Wall_Inclined_Tall_02":           5600,
	"Sardaukar_Wall_Inclined_Tall_02_L":         5600,
	"Sardaukar_Wall_Round_Corner":               5600,
	"Sardaukar_Wall_Triangle_Bottom_Half_Left":  5600,
	"Sardaukar_Wall_Triangle_Bottom_Half_Right": 5600,
	"Sardaukar_Wall_Triangle_Bottom_Left":       5600,
	"Sardaukar_Wall_Triangle_Bottom_Right":      5600,
	"Sardaukar_Wall_Triangle_Bottom_Tall_Left":  5600,
	"Sardaukar_Wall_Triangle_Bottom_Tall_Right": 5600,
	"Sardaukar_Wall_Triangle_Top_Left":          5600,
	"Sardaukar_Wall_Triangle_Top_Right":         5600,
	"Sardaukar_Wall_Triangle_Top_Tall_Left":     5600,
	"Sardaukar_Wall_Triangle_Top_Tall_Right":    5600,
	"Sardaukar_Window":                          5600,
	"Sardaukar_Window_Inclined_Tall":            5586.25,
	"Sardaukar_Window_Inclined_Tall_L":          5585,
	"Sardaukar_Window_L":                        5600,
}

// PlaceableTypeHealthSeen is the same for placeables (holograms left out), for the types with at least five placeables in the measured save.
var PlaceableTypeHealthSeen = map[string]float64{
	"B1C5_MTX_Sardaukar_WallLamp_Placeable":      500,
	"Choam_PentashieldSurfaceVertical_Placeable": 3500,
	"Developer_StorageContainer_Placeable":       1250,
	"LargeWaterCistern_Placeable":                2500,
	"LargeWindtrap_Placeable":                    3500,
	"MTX_Neut_LightPole_Placeable":               500,
	"MediumWaterCistern_Placeable":               2500,
	"SpiceGenerator_Placeable":                   4000,
	"WindTurbineDirectional_Placeable":           3500,
}

// HealthCapOf is the cap of one type: the figure seen times the margin, rounded up to a whole number.
func HealthCapOf(seen float64) float64 { return math.Ceil(seen * HealthCapMargin) }

// PieceHealthCap is the most health a building piece of the type may have: the cap of its type, or the overall cap of the pieces for a type the table does not list.
func PieceHealthCap(buildingType string) float64 {
	if seen, ok := PieceTypeHealthSeen[buildingType]; ok {
		return HealthCapOf(seen)
	}
	return MaxRepairPieceHealth
}

// PlaceableHealthCap is the same for a placeable.
func PlaceableHealthCap(buildingType string) float64 {
	if seen, ok := PlaceableTypeHealthSeen[buildingType]; ok {
		return HealthCapOf(seen)
	}
	return MaxRepairPlaceableHealth
}

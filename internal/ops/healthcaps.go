package ops

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// The health the repair functions may copy onto a piece (#613, audit G-009).
//
// The repair target of a type is the best health seen on a STANDING piece of the player's own bases (see bestHealthOf in world.go), and never above the cap of the type. The caps were
// measured on one real save (3,457 building pieces of 70 types, 230 placeables of 45 types, all standing, none in a backup). The figures below are the most health seen on any piece
// of the type, for each type that has at least five pieces in that save; a cap is that figure times healthCapMargin. A type the table does not list (fewer than five samples, or a type
// the save does not hold) is held to the overall cap of its table: the most seen on any piece of the table, times the same margin. The caps only limit what a repair writes; no
// piece is ever lowered, and the file check on an import (basefile.MaxPieceHealth) is a separate and much wider ceiling.
const (
	// healthCapMargin is the room above the highest figure seen.
	healthCapMargin = 1.25
	// pieceHealthSeen and placeableHealthSeen are the highest health seen on any building piece (7,000) and on any placeable (20,000, one large refinery; the next is 4,000).
	pieceHealthSeen     = 7000.0
	placeableHealthSeen = 20000.0
	// MaxRepairPieceHealth and MaxRepairPlaceableHealth are the overall caps of the two tables.
	MaxRepairPieceHealth     = pieceHealthSeen * healthCapMargin
	MaxRepairPlaceableHealth = placeableHealthSeen * healthCapMargin
)

// pieceTypeHealthSeen is the highest health seen per building type, for the types with at least five pieces in the measured save.
var pieceTypeHealthSeen = map[string]float64{
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

// placeableTypeHealthSeen is the same for placeables (holograms left out), for the types with at least five placeables in the measured save.
var placeableTypeHealthSeen = map[string]float64{
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

// healthCapOf is the cap of one type: the figure seen times the margin, rounded up to a whole number.
func healthCapOf(seen float64) float64 { return math.Ceil(seen * healthCapMargin) }

// capValues is the text of a VALUES list of (type, cap) pairs for the types of a table, to be joined to the best health of each type. The type names are the program's own constants;
// a quote in one is doubled all the same. The list is sorted so that the statement text does not change from run to run.
func capValues(seen map[string]float64) string {
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("('" + strings.ReplaceAll(n, "'", "''") + "'," + strconv.FormatFloat(healthCapOf(seen[n]), 'f', -1, 64) + ")")
	}
	return b.String()
}

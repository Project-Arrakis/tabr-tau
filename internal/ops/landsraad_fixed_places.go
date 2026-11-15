package ops

// ---------------------------------------------------------------- the fixed places of the house representatives (refs #639)
//
// A house representative stands at one fixed place of the game's maps, in every game. The save holds a map marker for a representative only once the character has found it (18 of the
// 25 houses on the save checked), so the board takes a house's place from the save's own marker when there is one, and from this table when there is not. A marker in the save always wins:
// houseLocations reads the table only for a house that has no usable marker.
//
// Where the table comes from: the positions are the game's fixed map places for the house representatives, as the reference map shows them, for each of the four maps they stand on (Hagga Basin,
// Harko Village, Arrakeen, the Deep Desert; the same world positions in centimetres as the save's markers, which the 18 houses with a marker in the save checked agree with to within 2 centimetres).
// The Deep Desert has the same two representatives in every layout of the map. The area number of a Hagga Basin house is the number the save's own markers carry in that spot; the name of
// the area comes from the kit's table (kit/gamedata/regions), as for a marker. Three houses are not in the reference map's list of representatives: Maros and Wayku (Deep Desert; their position is in its Deep Desert data files, the same in every layout, and falls in the
// tiles the public guides give, B8 and A2) and House Tseida, which has a control point there and no representative, and two public guides to the representatives (credited in internal/notices/THIRD-PARTY-NOTICES.md; location facts only) put it inside The Anvil Tradepost, so its position is that tradepost's own position
// on the reference map (the same place the save marks as TradingPost; Place says so). The tile of a Deep Desert house is worked out from its position with kit/ddgrid.
//
// Keys are the house's name as the marker spells it (the save's "Mikkarol" is "Mikarrol" here, see houseMarkerName); houseLocations turns them into the save's "DA_House<Name>".

// fixedHousePlace is one representative's fixed place: the save's map name, the world position in centimetres, the area number of a Hagga Basin place (0 elsewhere) and, when the
// position is not the representative's own, the words that say what it is.
type fixedHousePlace struct {
	Map   string
	X, Y  float64
	Area  int64
	Place string
}

var fixedHousePlaces = map[string]fixedHousePlace{
	// Hagga Basin
	"Argosaz":   {Map: "HaggaBasin", X: 20712, Y: 225883, Area: 12},
	"Dyvetz":    {Map: "HaggaBasin", X: 260395, Y: -2542, Area: 3},
	"Ecaz":      {Map: "HaggaBasin", X: 306154, Y: -30677, Area: 3},
	"Hagal":     {Map: "HaggaBasin", X: -107198, Y: -228884, Area: 5},
	"Hurata":    {Map: "HaggaBasin", X: 320154, Y: 77064, Area: 14},
	"Imota":     {Map: "HaggaBasin", X: 216722, Y: -283171, Area: 13},
	"Kenola":    {Map: "HaggaBasin", X: -316767, Y: -229449, Area: 4},
	"Lindaren":  {Map: "HaggaBasin", X: -271053, Y: -425184, Area: 8},
	"Mikarrol":  {Map: "HaggaBasin", X: 95585, Y: -175300, Area: 7},
	"Moritani":  {Map: "HaggaBasin", X: 278987, Y: 11351, Area: 3},
	"Novebruns": {Map: "HaggaBasin", X: -17641, Y: -20655, Area: 2},
	"Richese":   {Map: "HaggaBasin", X: -41960, Y: -229566, Area: 5},
	"Sor":       {Map: "HaggaBasin", X: -32588, Y: -312768, Area: 16},
	"Taligari":  {Map: "HaggaBasin", X: -219649, Y: -163672, Area: 4},
	"Thorvald":  {Map: "HaggaBasin", X: 285447, Y: -125154, Area: 3},
	"Tseida":    {Map: "HaggaBasin", X: 193124, Y: 3142, Area: 14, Place: "The Anvil Tradepost"},
	"Vernius":   {Map: "HaggaBasin", X: -51671, Y: -234471, Area: 5},
	"Wydras":    {Map: "HaggaBasin", X: -315868, Y: 39346, Area: 11},
	// Arrakeen
	"Mutelli": {Map: "Arrakeen", X: 3622, Y: -3222},
	"Varota":  {Map: "Arrakeen", X: -8708, Y: -7988},
	"Wallach": {Map: "Arrakeen", X: -27134, Y: 2864},
	// Harko Village
	"Alexin":   {Map: "HarkoVillage", X: 9978, Y: 11503},
	"Spinette": {Map: "HarkoVillage", X: 3216, Y: 9403},
	// the Deep Desert (the same two places in every layout)
	"Maros": {Map: "DeepDesert", X: 693699, Y: 866212},
	"Wayku": {Map: "DeepDesert", X: -755998, Y: 906988},
}

package ops

// ---------------------------------------------------------------- where a house is (refs #599)
//
// The save has no column for the place of a house. Every house representative is drawn on its map in the game, so the place is known, but this editor does not yet hold a table of
// those positions (map, area name, place, coordinates, one row per house).
//
// THIS IS A PLACEHOLDER SOURCE. houseLocations is empty on purpose: no position is invented. The board reads locationOf for every house, shows "not mapped yet" for a house the
// table does not hold, and draws the pop-up and the map dialog only when a position is there. When the table is built (from the game's map data, in the same way as
// place-names.json and regions.go) it goes into this map, keyed by the house name as the save writes it (for example "DA_HouseAlexin"), and nothing else has to change.

// houseLocation is where the representative of one house is: the map ("Hagga Basin", "Deep Desert", "Harko Village", "Arrakeen"), the named area of the map (the main text of the
// row), the finer place (the pop-up only), and the position as a fraction of the schematic map (0 to 1, from the left and from the top).
type houseLocation struct {
	Map, Area, Place string
	X, Y             float64
}

// houseLocations is the table of house representatives' positions. Empty until it is built from the game's map data.
var houseLocations = map[string]houseLocation{}

// locationOf is the board's view of a house's place: known is false for a house the table does not hold.
func locationOf(house string) map[string]any {
	l, ok := houseLocations[house]
	if !ok {
		return map[string]any{"known": false}
	}
	return map[string]any{"known": true, "map": l.Map, "area": l.Area, "place": l.Place, "x": l.X, "y": l.Y}
}

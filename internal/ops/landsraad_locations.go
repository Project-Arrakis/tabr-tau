package ops

import (
	"math"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/kit/ddgrid"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/regions"
)

// ---------------------------------------------------------------- where a house is (refs #639)
//
// The save has no column for the place of a house, but it holds the house representatives as map markers: marker type "HouseRepresentative<Name>", one row per representative, with the
// map, the position and the number of the area the marker stands in. The Saved map draws the same rows. The board reads them each time it is read. A representative the character has not found
// yet has no marker, so for such a house the fixed place of landsraad_fixed_places.go is used; a marker in the save always wins. A house in neither is shown as "location unknown".
//
// The save names a house "DA_House<Name>" and its marker "HouseRepresentative<Name>". One name is spelled differently in the two (houseMarkerName).

// houseLocation is where the representative of one house is: the map ("Hagga Basin", "Deep Desert", "Harko Village", "Arrakeen"), the named area of the map (the main text of the row: the
// area on the Hagga Basin, the map's name elsewhere), the finer place (the tradepost a representative stands in, from public guides; empty otherwise; the Deep Desert's grid cell is Cell), and the position as a fraction of the schematic map (0 to 1,
// from the left and from the top; only on a map the Saved map has a mapping for, see houseMapKey).
type houseLocation struct {
	Map, Area, Place string
	Cell             string // the Deep Desert's lettered grid cell ("B8"), the editor's own words for it; empty elsewhere
	Unconfirmed      bool   // Place is from public guides and not confirmed in the game
	X, Y             float64
	Pinned           bool // X and Y are inside the frame of a map the Saved map has a mapping for: only then are the pin, the distance and the map link given
}

// houseMarkerPrefix is the start of the type of the marker of a house representative.
const houseMarkerPrefix = "HouseRepresentative"

// houseMarkerName is the house names whose marker spells them differently: the save's house "Mikkarol" is the marker's "Mikarrol".
var houseMarkerName = map[string]string{"Mikkarol": "Mikarrol"}

// houseMapLabel is the words for the maps a house representative stands on, by the save's map name. A marker on another map gives no place: it is not where a house representative is.
var houseMapLabel = map[string]string{"HaggaBasin": "Hagga Basin", "DeepDesert": "Deep Desert", "HarkoVillage": "Harko Village", "Arrakeen": "Arrakeen"}

// houseMapKey names the save's map key for the maps the Saved map has a world-to-picture mapping for (savedMaps: the two maps of liveMaps and the hubs, #758). A position on one of them gets a pin, a distance and the link to the Saved map.
var houseMapKey = map[string]string{"Hagga Basin": "HaggaBasin", "Deep Desert": "DeepDesert", "Arrakeen": "Arrakeen", "Harko Village": "HarkoVillage"}

// charPlace is where the character stands: the save's map key (for example "HaggaBasin") and the world position in centimetres.
type charPlace struct {
	Map  string
	X, Y float64
}

// houseLocations reads the places of the house representatives from the save's map markers, by the house name as the save writes it ("DA_HouseAlexin"). A house with no marker is not in
// the table. When two markers carry the same house, the first by map name (alphabetical), marker id and dimension is used, so the answer does not change between reads. Like the Saved map, this does not filter on the dimension.
func (o *Ops) houseLocations() (map[string]houseLocation, error) {
	rows, err := o.S.Query(`select m.marker_type t, m.map_name map, m.x x, m.y y, m.area_id area from `+o.S.MarkersTable()+` m
		where glob(?, m.marker_type) and m.x is not null and m.y is not null order by m.map_name, m.marker_hash_id, m.dimension_index`, houseMarkerPrefix+"*")
	if err != nil {
		return nil, readFailed("the house representatives' markers", err)
	}
	byName := map[string]houseLocation{}
	for _, r := range rows {
		typ, _ := r["t"].(string)
		name := canonicalHouse(strings.TrimPrefix(typ, houseMarkerPrefix))
		mapName, _ := r["map"].(string)
		_, ok := houseMapLabel[mapName]
		x, okx := toFloat(r["x"])
		y, oky := toFloat(r["y"])
		if name == "" || !ok || !okx || !oky || math.IsNaN(x) || math.IsNaN(y) {
			continue
		}
		if _, dup := byName[name]; dup {
			continue
		}
		area, _ := r["area"].(int64)
		l := placeAt(mapName, x, y, area, "")
		byName[name] = l
	}
	return houseTable(byName), nil
}

// fixedHouseLocations is the table of places when the markers cannot be read (a save without a usable markers table, or without a column the query names): only the fixed place of each house (#820).
func fixedHouseLocations() map[string]houseLocation {
	return houseTable(map[string]houseLocation{})
}

// houseTable gives a house the save has no marker for its fixed place (a marker, even one on another map than the fixed place, is never replaced) and keys the result by the save's
// spelling of the house and by the marker's own (a save may use either).
func houseTable(byName map[string]houseLocation) map[string]houseLocation {
	for name, f := range fixedHousePlaces {
		if _, has := byName[name]; !has {
			byName[name] = placeAt(f.Map, f.X, f.Y, f.Area, f.Place)
		}
	}
	out := make(map[string]houseLocation, len(byName))
	for name, l := range byName {
		out["DA_House"+name] = l
		for saveName, markerName := range houseMarkerName {
			if markerName == name {
				out["DA_House"+saveName] = l
			}
		}
	}
	return out
}

// canonicalHouse is the marker's spelling of a house name: a save may write the marker with the save's own spelling ("Mikkarol") or the game's ("Mikarrol").
func canonicalHouse(name string) string {
	if m, ok := houseMarkerName[name]; ok {
		return m
	}
	return name
}

// placeAt is the place of a representative at a position of a map (the save's map name): its map and area, and on a map the editor has a picture for the pin as a fraction of the
// picture. The Hagga Basin area comes from the area number (the map's name when the number has no name), a Deep Desert place says its grid cell. place, when it is not empty, says
// what the position is (a tradepost the representative stands in).
func placeAt(mapName string, x, y float64, areaID int64, place string) houseLocation {
	label := houseMapLabel[mapName]
	l := houseLocation{Map: label, Area: label, Place: place, Unconfirmed: place != ""}
	if n := regions.Name(mapName, areaID); n != "" {
		l.Area = n
	}
	if mapName == "DeepDesert" {
		l.Cell = ddgrid.Cell(x, y)
	}
	if key, ok := houseMapKey[label]; ok {
		if m := findMap(key); m.Key == key {
			fx, fy := (x-m.MinX)/(m.MaxX-m.MinX), (y-m.MinY)/(m.MaxY-m.MinY)
			// a position beyond the edge of the frame gets no pin, no distance and no map link: the row still gives its area
			if fx >= 0 && fx <= 1 && fy >= 0 && fy <= 1 {
				l.X, l.Y, l.Pinned = fx, fy, true
			}
		}
	}
	return l
}

// distanceFromYou is the straight-line distance in centimetres from the character to the house representative, when both are on the same map and the map has a mapping:
// the table's fraction of the map becomes a world position the way the Saved map draws it (x = MinX + fx * (MaxX - MinX), y likewise).
func distanceFromYou(l houseLocation, here *charPlace) (float64, bool) {
	key, ok := houseMapKey[l.Map]
	if !ok || !l.Pinned || here == nil || here.Map != key {
		return 0, false
	}
	m := findMap(key)
	if m.Key != key {
		return 0, false
	}
	hx, hy := m.MinX+l.X*(m.MaxX-m.MinX), m.MinY+l.Y*(m.MaxY-m.MinY)
	return math.Hypot(hx-here.X, hy-here.Y), true
}

// locationOf is the board's view of a house's place: known is false for a house the table does not hold. x and y (the pin) and map_key are there only on a map with a picture;
// from_you_cm only when the distance can be worked out.
func locationOf(house string, table map[string]houseLocation, here *charPlace) map[string]any {
	l, ok := table[house]
	if !ok {
		return map[string]any{"known": false}
	}
	out := map[string]any{"known": true, "map": l.Map, "area": l.Area, "place": l.Place, "map_key": "", "cell": l.Cell, "unconfirmed": l.Unconfirmed}
	if l.Pinned {
		out["map_key"], out["x"], out["y"] = houseMapKey[l.Map], l.X, l.Y
	}
	if d, ok := distanceFromYou(l, here); ok {
		out["from_you_cm"] = math.Round(d)
	}
	return out
}

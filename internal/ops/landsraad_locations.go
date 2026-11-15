package ops

import (
	"math"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/regions"
)

// ---------------------------------------------------------------- where a house is (refs #639)
//
// The save has no column for the place of a house, but it holds the house representatives as map markers: marker type "HouseRepresentative<Name>", one row per representative, with the
// map, the position and the number of the area the marker stands in. The Saved map draws the same rows. The board reads them each time it is read, so nothing is stored in the editor
// and no position is invented: a house whose representative has no marker in the save is shown as "location unknown".
//
// The save names a house "DA_House<Name>" and its marker "HouseRepresentative<Name>". One name is spelled differently in the two (houseMarkerName).

// houseLocation is where the representative of one house is: the map ("Hagga Basin", "Deep Desert", "Harko Village", "Arrakeen"), the named area of the map (the main text of the row: the
// marker's own area on the Hagga Basin, the map's name elsewhere), the finer place (the save has none, so it is empty), and the position as a fraction of the schematic map (0 to 1,
// from the left and from the top; only on a map the editor has a picture for, see houseMapKey).
type houseLocation struct {
	Map, Area, Place string
	X, Y             float64
}

// houseMarkerPrefix is the start of the type of the marker of a house representative.
const houseMarkerPrefix = "HouseRepresentative"

// houseMarkerName is the house names whose marker spells them differently: the save's house "Mikkarol" is the marker's "Mikarrol".
var houseMarkerName = map[string]string{"Mikkarol": "Mikarrol"}

// houseMapLabel is the words for the maps a house representative stands on, by the save's map name. A marker on another map gives no place: it is not where a house representative is.
var houseMapLabel = map[string]string{"HaggaBasin": "Hagga Basin", "DeepDesert": "Deep Desert", "HarkoVillage": "Harko Village", "Arrakeen": "Arrakeen"}

// houseMapKey names the save's map key for the maps the editor has a world-to-picture mapping for (liveMaps). A hub (Harko Village, Arrakeen) has none, so no pin and no distance are given there.
var houseMapKey = map[string]string{"Hagga Basin": "HaggaBasin", "Deep Desert": "DeepDesert"}

// charPlace is where the character stands: the save's map key (for example "HaggaBasin") and the world position in centimetres.
type charPlace struct {
	Map  string
	X, Y float64
}

// houseLocations reads the places of the house representatives from the save's map markers, by the house name as the save writes it ("DA_HouseAlexin"). A house with no marker is not in
// the table. When two markers carry the same house, the first by map and marker id is used, so the answer does not change between reads.
func (o *Ops) houseLocations() (map[string]houseLocation, error) {
	rows, err := o.S.Query(`select m.marker_type t, m.map_name map, m.x x, m.y y, m.area_id area from `+o.S.MarkersTable()+` m
		where m.marker_type like ? and m.x is not null and m.y is not null order by m.map_name, m.marker_hash_id, m.dimension_index`, houseMarkerPrefix+"%")
	if err != nil {
		return nil, readFailed("the house representatives' markers", err)
	}
	byName := map[string]houseLocation{}
	for _, r := range rows {
		typ, _ := r["t"].(string)
		name := strings.TrimPrefix(typ, houseMarkerPrefix)
		mapName, _ := r["map"].(string)
		label, ok := houseMapLabel[mapName]
		x, okx := toFloat(r["x"])
		y, oky := toFloat(r["y"])
		if name == "" || !ok || !okx || !oky || math.IsNaN(x) || math.IsNaN(y) {
			continue
		}
		if _, dup := byName[name]; dup {
			continue
		}
		l := houseLocation{Map: label, Area: label}
		if a, ok := r["area"].(int64); ok {
			if n := regions.Name(mapName, a); n != "" {
				l.Area = n
			}
		}
		if key, ok := houseMapKey[label]; ok {
			if m := findMap(key); m.Key == key {
				l.X, l.Y = (x-m.MinX)/(m.MaxX-m.MinX), (y-m.MinY)/(m.MaxY-m.MinY)
			}
		}
		byName[name] = l
	}
	// by the save's spelling of the house
	out := make(map[string]houseLocation, len(byName))
	for name, l := range byName {
		house := name
		for saveName, markerName := range houseMarkerName {
			if markerName == name {
				house = saveName
			}
		}
		out["DA_House"+house] = l
	}
	return out, nil
}

// distanceFromYou is the straight-line distance in centimetres from the character to the house representative, when both are on the same map and the map has a mapping:
// the table's fraction of the map becomes a world position the way the Saved map draws it (x = MinX + fx * (MaxX - MinX), y likewise).
func distanceFromYou(l houseLocation, here *charPlace) (float64, bool) {
	key, ok := houseMapKey[l.Map]
	if !ok || here == nil || here.Map != key {
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
	out := map[string]any{"known": true, "map": l.Map, "area": l.Area, "place": l.Place, "map_key": houseMapKey[l.Map]}
	if _, mapped := houseMapKey[l.Map]; mapped {
		out["x"], out["y"] = l.X, l.Y
	}
	if d, ok := distanceFromYou(l, here); ok {
		out["from_you_cm"] = math.Round(d)
	}
	return out
}

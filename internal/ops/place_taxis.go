package ops

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// The taxis of Go to a place that do not come from the save's markers (#521): the Deep Desert's three taxi service points, from the game's level data, whenever the chosen map is the
// Deep Desert; and the two hubs (Arrakeen, Harko Village) from every map. The Deep Desert's taxis stand in row A of its grid, which is static (the same in every layout), so no layout
// is consulted. deep-desert-taxis.json says where (internal/catalog/PROVENANCE.md, "deep-desert-taxis.json"; tools/deepdeserttaxisgen makes it from a dump of the game's level
// DeepDesert_1_Travel). The hubs' points are in place-arrivals.json (#515).

//go:embed deep-desert-taxis.json
var rawDeepDesertTaxis []byte

// mapDeepDesert is the map name the save and the editor use for the Deep Desert.
const mapDeepDesert = "DeepDesert"

// taxiMarkerTolerance is how near (game units, flat) a saved TaxiService marker of the Deep Desert has to be to the marker of a point of the file to be the same place.
const taxiMarkerTolerance = 5000.0

type levelPoint struct{ X, Y, Z float64 }

// deepDesertTaxi is one of the Deep Desert's taxi service points: the grid cell its marker lies in, the marker, the standing position beside it (the game's spawn location), and
// whether anyone has stood there (Observed).
type deepDesertTaxi struct {
	Cell     string
	Marker   levelPoint
	Stand    levelPoint
	Observed *bool
}

// deepDesertTaxis are the points by LocationKey (upper case), the same keys place-names.json gives the kind "taxi".
var deepDesertTaxis = func() map[string]deepDesertTaxi {
	var f struct {
		Taxis map[string]deepDesertTaxi `json:"taxis"`
	}
	if err := json.Unmarshal(rawDeepDesertTaxis, &f); err != nil {
		panic("ops: deep-desert-taxis.json is not valid: " + err.Error())
	}
	return f.Taxis
}()

// deepDesertTaxiPlaces: the Deep Desert's taxi service points, when the list is for the Deep Desert; none for any other map. They come first by their grid cell (A2, A5, A8), which is
// the order of a list for a map the character is not on; on the character's own map the caller puts them nearest first. Each is at its standing position and is named by the taxi
// name and its grid cell ("Deep Desert Taxi Service (A5)").
func (o *Ops) deepDesertTaxiPlaces(_ player, mp string) ([]placeRow, error) {
	if mp != mapDeepDesert {
		return nil, nil
	}
	keys := make([]string, 0, len(deepDesertTaxis))
	for k := range deepDesertTaxis {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := deepDesertTaxis[keys[i]], deepDesertTaxis[keys[j]]
		if a.Cell != b.Cell {
			return a.Cell < b.Cell
		}
		return keys[i] < keys[j]
	})
	var out []placeRow
	for _, k := range keys {
		t := deepDesertTaxis[k]
		name := placeNames[k].Name
		if name == "" {
			name = "Deep Desert Taxi Service"
		}
		out = append(out, placeRow{group: groupTaxi, label: fmt.Sprintf("%s (%s)", name, t.Cell), alias: placeNames[k].Alias, key: k, x: t.Stand.X, y: t.Stand.Y, z: t.Stand.Z})
	}
	return out, nil
}

// isDeepDesertTaxiKey says whether the LocationKey is one of the three the file has a point for.
func isDeepDesertTaxiKey(key string) bool {
	_, ok := deepDesertTaxis[key]
	return ok
}

// isListedDeepDesertTaxi says whether a TaxiService marker of the Deep Desert at x, y is the place of one of the file's points (so the file's row stands for it and it is not listed
// twice). A marker with no position in the save cannot be matched.
func isListedDeepDesertTaxi(x, y any) bool {
	fx, ok1 := toFloat(x)
	fy, ok2 := toFloat(y)
	if !ok1 || !ok2 {
		return false
	}
	for _, t := range deepDesertTaxis {
		if math.Hypot(fx-t.Marker.X, fy-t.Marker.Y) <= taxiMarkerTolerance {
			return true
		}
	}
	return false
}

// hubPlaces: the two hubs, Arrakeen and Harko Village, offered from every map, each at the game's arrival point for it (#515) and named as the hub's taxi service is ("Arrakeen Taxi
// Service"), under Taxis. A hub is another map: the row says which one (onMap), so that choosing it moves the character into that map. The character does not need to have found the
// hub's taxi marker: a marker of such a hub in the save is not listed a second time (tradeAndTaxiPlaces leaves it out).
func (o *Ops) hubPlaces(_ player, _ string) ([]placeRow, error) {
	names := make([]string, 0, len(placeArrivals.Hubs))
	for n := range placeArrivals.Hubs {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []placeRow
	for _, n := range names {
		a := placeArrivals.Hubs[n]
		out = append(out, placeRow{group: groupTaxi, label: taxiName(n), x: a.X, y: a.Y, z: a.Z, arrival: true, onMap: n})
	}
	return out, nil
}

// hubArrival is the arrival point of the hub whose map is named mp, or false when mp is not a hub the file has a point for.
func hubArrival(mp string) (arrivalPoint, bool) {
	a, ok := placeArrivals.Hubs[mp]
	return a, ok
}

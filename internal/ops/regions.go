package ops

import (
	_ "embed"
	"encoding/json"
	"math"
	"sort"
	"strconv"
)

// Region names (#406). The Hagga Basin is cut into 12 named sub-regions; the save's markers carry the number of the region they stand in
// (markers.area_id) and the character's discovered regions are in map_areas, but neither says a name. hagga-regions.json is the Dune Docker
// console's table of those names (MIT; see the notices): the console took them from the game's own area definitions. A place is given the region
// of the nearest map marker that has a named area, which is how the console labels its vehicles; it is a good guess, not the game's own answer, so the
// page calls it "near". Only the Hagga Basin has names; the Deep Desert and the other maps have none, and a thing there gets no region.
//
//go:embed hagga-regions.json
var rawRegions []byte

// regionTable is map name -> area id -> name.
var regionTable = func() map[string]map[int64]string {
	var file map[string]json.RawMessage
	if err := json.Unmarshal(rawRegions, &file); err != nil {
		panic("ops: hagga-regions.json is not valid: " + err.Error())
	}
	out := map[string]map[int64]string{}
	for mapName, raw := range file {
		var areas map[string]string
		if json.Unmarshal(raw, &areas) != nil { // "_comment" is a string, not a table
			continue
		}
		names := map[int64]string{}
		for k, v := range areas {
			if id, err := strconv.ParseInt(k, 10, 64); err == nil && v != "" {
				names[id] = v
			}
		}
		if len(names) > 0 {
			out[mapName] = names
		}
	}
	return out
}()

// regionNamesOf is the area id -> name table of one map, or nil when the map has none.
func regionNamesOf(mapName string) map[int64]string { return regionTable[mapName] }

// regionPoint is a marker that stands in a named area.
type regionPoint struct {
	x, y float64
	area int64
}

// regionFinder labels positions on any map. It reads each map's markers once.
type regionFinder struct {
	o      *Ops
	points map[string][]regionPoint
}

func (o *Ops) regionFinder() *regionFinder {
	return &regionFinder{o: o, points: map[string][]regionPoint{}}
}

func (f *regionFinder) load(mapName string) []regionPoint {
	if pts, ok := f.points[mapName]; ok {
		return pts
	}
	names := regionNamesOf(mapName)
	var pts []regionPoint
	if names != nil {
		rows, _ := f.o.S.Query(`select x, y, area_id from markers where map_name=? and area_id is not null and area_id<>0 and x is not null and y is not null`, mapName)
		for _, r := range rows {
			x, ok1 := toFloat(r["x"])
			y, ok2 := toFloat(r["y"])
			a, ok3 := r["area_id"].(int64)
			if _, named := names[a]; ok1 && ok2 && ok3 && named {
				pts = append(pts, regionPoint{x, y, a})
			}
		}
	}
	f.points[mapName] = pts
	return pts
}

// name is the region of a position on a map: the name of the area of the nearest marker with a named area, or "" when the map has no names, the
// position is unknown, or the save has no such marker.
func (f *regionFinder) name(mapName string, x, y any) string {
	px, ok1 := toFloat(x)
	py, ok2 := toFloat(y)
	if !ok1 || !ok2 || math.IsNaN(px) || math.IsNaN(py) {
		return ""
	}
	best, bestD := int64(-1), math.Inf(1)
	for _, p := range f.load(mapName) {
		if d := (p.x-px)*(p.x-px) + (p.y-py)*(p.y-py); d < bestD {
			best, bestD = p.area, d
		}
	}
	if best < 0 {
		return ""
	}
	return regionNamesOf(mapName)[best]
}

// label puts the region on a row that has map, x and y (the keys are given because the lists name them differently).
func (f *regionFinder) label(row map[string]any, mapName string, xk, yk string) {
	if n := f.name(mapName, row[xk], row[yk]); n != "" {
		row["region"] = n
	}
}

// discoveredAreas is the character's discovered areas of one map, by name. named says whether the map has area names at all; total is how many it
// has; discovered are the names found (sorted); other counts discovered areas that have no name.
func (o *Ops) discoveredAreas(characterID int64, mapName string) map[string]any {
	names := regionNamesOf(mapName)
	out := map[string]any{"named": names != nil, "total": len(names), "discovered": []string{}, "other": 0}
	if names == nil {
		return out
	}
	rows, _ := o.S.Query(`select distinct area_id from map_areas where character_id=? and map=? and area_id is not null`, characterID, mapName)
	found := []string{}
	other := 0
	for _, r := range rows {
		if a, ok := r["area_id"].(int64); ok {
			if n, named := names[a]; named {
				found = append(found, n)
			} else {
				other++
			}
		}
	}
	sort.Strings(found)
	out["discovered"], out["other"] = found, other
	return out
}

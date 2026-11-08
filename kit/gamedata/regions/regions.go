// Package regions names the areas of a map by the area id that the markers of a save carry (markers.area_id).
//
// The game divides the Hagga Basin into twelve named areas (fog-of-war area splines of the survival level, each with an id and a name) and every map marker of a save carries
// the id of the area it lies in. hagga-regions.json is the table of those twelve names. Area 0 means no area, and an id that has no name here has none: the package never
// guesses one. The Deep Desert and the other maps have no names in the table.
//
// Which place gets which region (for example the area of the nearest marker that has a named area), and how a program words it, is the program's decision. The package only
// says what the table says.
package regions

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

//go:embed hagga-regions.json
var snapshot []byte

var table map[string]map[int64]string

func init() {
	var err error
	if table, err = Parse(snapshot); err != nil {
		panic("regions: hagga-regions.json: " + err.Error())
	}
}

// Maps lists, in name order, the maps that have area names.
func Maps() []string {
	out := make([]string, 0, len(table))
	for m := range table {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// Names is the table of one map, area id to name, as a copy the caller may change. It is nil for a map with no names.
func Names(mapName string) map[int64]string {
	t := table[mapName]
	if len(t) == 0 {
		return nil
	}
	out := make(map[int64]string, len(t))
	for id, n := range t {
		out[id] = n
	}
	return out
}

// Name is the name of an area of a map, or "" when it has none (area 0 means no area).
func Name(mapName string, id int64) string { return table[mapName][id] }

// Parse reads the text of the table: one object whose keys are map names, each a table of area ids (whole numbers written as text) to names. A key "_comment" holding a string
// is allowed anywhere at the top and is ignored. It refuses anything else: an id that is not a whole number written plainly (no sign, no leading zero), an empty name, a map with no area, a file with no map.
func Parse(b []byte) (map[string]map[int64]string, error) {
	var file map[string]json.RawMessage
	if err := json.Unmarshal(b, &file); err != nil {
		return nil, err
	}
	out := map[string]map[int64]string{}
	for mapName, raw := range file {
		if mapName == "_comment" {
			var s string
			if err := json.Unmarshal(raw, &s); err != nil {
				return nil, errors.New("_comment is not a string")
			}
			continue
		}
		var areas map[string]string
		if err := json.Unmarshal(raw, &areas); err != nil {
			return nil, fmt.Errorf("map %q: %w", mapName, err)
		}
		if len(areas) == 0 {
			return nil, fmt.Errorf("map %q has no area", mapName)
		}
		names := make(map[int64]string, len(areas))
		for k, v := range areas {
			id, err := strconv.ParseInt(k, 10, 64)
			if err != nil || strconv.FormatInt(id, 10) != k {
				return nil, fmt.Errorf("map %q: area id %q is not a whole number", mapName, k)
			}
			if v == "" {
				return nil, fmt.Errorf("map %q: area %d has no name", mapName, id)
			}
			names[id] = v
		}
		out[mapName] = names
	}
	if len(out) == 0 {
		return nil, errors.New("no map")
	}
	return out, nil
}

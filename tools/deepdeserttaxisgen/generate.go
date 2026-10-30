// Command deepdeserttaxisgen makes internal/ops/deep-desert-taxis.json (#521) from a dump of one of the game's own level files: the Deep Desert's travel level, DeepDesert_1_Travel.
// The file says where the Deep Desert's three Taxi Service points stand, so that Admin > Movement > Go to a place can offer them whenever the chosen map is the Deep Desert, whether
// or not the save holds a marker for them. It reads only a JSON file, never the game: producing the dump takes a reader of the game's content files, which this repository does not
// contain (see internal/catalog/PROVENANCE.md, "deep-desert-taxis.json").
//
// Inputs:
//
//	-level   the dump of the package .../DeepDesert_1/SubLevels/DeepDesert_1_Travel (a JSON array of the package's objects, each with Type, Name, Outer and Properties)
//	-names   internal/ops/place-names.json: its keys of the kind "taxi" are the three keys the level has to hold (the rows of DT_StaticLocationData that belong to DeepDesert_1_Travel)
//	-read    the date the dump was read (YYYY-MM-DD)
//
// The rule (repeatable, and the same on every run):
//
//  1. A taxi service marker is an actor of the class BP_AutoDiscoveredStaticMarkerActor_C. Its m_Guid (four groups of eight digits) is the LocationKey of the table row, with the
//     hyphens taken out; its position is the RelativeLocation of its root component (these actors have no parent, so it is a world position in game units).
//     Exactly one marker for every taxi key of -names must be in the level; a missing or an extra one is an error.
//  2. A standing place is a spawn location: an actor of the class BP_StaticSpawnLocation_C. The marker's spawn location is the nearest one, within 10,000 units (100 m), and two markers
//     may not have the same one. The standing position is the spawn location's first slot (m_Slots[0] of its StaticSpawnLocationComponent: the slot at the actor's own place, turned
//     by the actor's yaw) added to the actor's root RelativeLocation.
//  3. The grid cell. The Deep Desert grid is the one the editor draws its Deep Desert picture on: nine sectors across (columns 1 to 9 along X) and nine down (rows along Y, A at the
//     greatest Y, I at the least), the rectangle of internal/ops/livemap.go (the constants below; a test in internal/ops pins them to that file). A point outside the bottom row (row A)
//     is an error: the Deep Desert's taxis are in row A, and a point that is not there is not offered.
//
// Numbers are rounded to one decimal. Every point is written with "observed": false: nobody has stood there yet. When the operator has, set that to true by hand in the committed
// file and keep it when the file is made again.
//
// Usage: go run ./tools/deepdeserttaxisgen -level <dump of DeepDesert_1_Travel> -names internal/ops/place-names.json -read 2026-10-30 -out internal/ops/deep-desert-taxis.json
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
)

// The Deep Desert rectangle of internal/ops/livemap.go (liveMaps, "DeepDesert") and the number of sectors on a side.
const (
	gridMinX, gridMaxX = -1268450.0, 1158400.0
	gridMinY, gridMaxY = -1261434.0, 1165416.0
	gridSectors        = 9
	maxStandDistance   = 10000.0 // the farthest a spawn location may be from its marker, in game units
	markerClass        = "BP_AutoDiscoveredStaticMarkerActor_C"
	spawnClass         = "BP_StaticSpawnLocation_C"
	spawnComponent     = "StaticSpawnLocationComponent"
)

// Point is a position in game units, the axes of the save's markers table.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Taxi is one Deep Desert taxi service point.
type Taxi struct {
	Cell     string `json:"cell"`     // the grid cell the marker lies in, such as "A5"
	Marker   Point  `json:"marker"`   // the taxi service marker
	Stand    Point  `json:"stand"`    // the spawn location beside it: where Go to a place puts the character
	Observed bool   `json:"observed"` // true once a person has stood at Stand in the game
}

// Grid is the rectangle the cells are counted on.
type Grid struct {
	MinX    float64 `json:"minX"`
	MaxX    float64 `json:"maxX"`
	MinY    float64 `json:"minY"`
	MaxY    float64 `json:"maxY"`
	Sectors int     `json:"sectors"` // on a side
}

// File is the content of deep-desert-taxis.json.
type File struct {
	Source string          `json:"source"`
	Level  string          `json:"level"`
	Read   string          `json:"read"`
	Grid   Grid            `json:"grid"`
	Taxis  map[string]Taxi `json:"taxis"` // by LocationKey (32 upper-case hexadecimal digits)
}

// Inputs are the paths of the dump and the names file and what to record in the file.
type Inputs struct {
	Level, Names, Read string
}

type objectRef struct {
	ObjectName string `json:"ObjectName"`
}

type vec struct{ X, Y, Z float64 }

type export struct {
	Type       string    `json:"Type"`
	Name       string    `json:"Name"`
	Outer      objectRef `json:"Outer"`
	Properties struct {
		Guid        string                 `json:"m_Guid"`
		RootComp    objectRef              `json:"RootComponent"`
		RelativeLoc *vec                   `json:"RelativeLocation"`
		RelativeRot *struct{ Yaw float64 } `json:"RelativeRotation"`
		Slots       []struct {
			Location vec     `json:"Location"`
			Yaw      float64 `json:"Yaw"`
		} `json:"m_Slots"`
	} `json:"Properties"`
}

// inner is the part of an object name between its quotes: "SceneComponent'L:PersistentLevel.A.RootComponent'" is "L:PersistentLevel.A.RootComponent".
func inner(name string) string {
	i, j := strings.Index(name, "'"), strings.LastIndex(name, "'")
	if i < 0 || j <= i {
		return name
	}
	return name[i+1 : j]
}

var guidKey = regexp.MustCompile(`^[0-9A-Fa-f]{8}(-[0-9A-Fa-f]{8}){3}$`)

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// Cell is the grid cell of a point, or an error when it is outside the grid or not in row A.
func Cell(x, y float64) (string, error) {
	w := (gridMaxX - gridMinX) / gridSectors
	h := (gridMaxY - gridMinY) / gridSectors
	col := int(math.Floor((x-gridMinX)/w)) + 1
	row := int(math.Floor((gridMaxY - y) / h)) // 0 for the row at the greatest Y
	if col < 1 || col > gridSectors || row < 0 || row >= gridSectors {
		return "", fmt.Errorf("the point %.1f, %.1f is outside the Deep Desert grid", x, y)
	}
	cell := fmt.Sprintf("%c%d", 'A'+row, col)
	if row != 0 {
		return cell, fmt.Errorf("the point %.1f, %.1f is in %s, not in row A: it is not offered", x, y, cell)
	}
	return cell, nil
}

// worldPos is the position of the actor's root component and the root's yaw in degrees.
func worldPos(comps map[string]export, root objectRef) (vec, float64, error) {
	c, ok := comps[inner(root.ObjectName)]
	if !ok || c.Properties.RelativeLoc == nil {
		return vec{}, 0, fmt.Errorf("the root component %q has no RelativeLocation in the dump", root.ObjectName)
	}
	yaw := 0.0
	if c.Properties.RelativeRot != nil {
		yaw = c.Properties.RelativeRot.Yaw
	}
	return *c.Properties.RelativeLoc, yaw, nil
}

// Generate applies the rule above.
func Generate(in Inputs) (*File, error) {
	var level []export
	if err := readJSON(in.Level, &level); err != nil {
		return nil, err
	}
	if len(level) == 0 {
		return nil, errors.New("the level dump is empty")
	}
	var names struct {
		Places map[string]struct {
			Kind string `json:"kind"`
		} `json:"places"`
	}
	if err := readJSON(in.Names, &names); err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for k, p := range names.Places {
		if p.Kind == "taxi" {
			want[strings.ToUpper(k)] = true
		}
	}
	if len(want) == 0 {
		return nil, errors.New("the names file has no key of the kind taxi")
	}

	comps := map[string]export{}      // components by their full name
	spawnComps := map[string]export{} // the StaticSpawnLocationComponent of an actor, by the actor's name
	var markers, spawns []export
	for _, e := range level {
		switch e.Type {
		case markerClass:
			markers = append(markers, e)
		case spawnClass:
			spawns = append(spawns, e)
		case spawnComponent:
			spawnComps[inner(e.Outer.ObjectName)] = e
		}
		if e.Name != "" && e.Outer.ObjectName != "" {
			comps[inner(e.Outer.ObjectName)+"."+e.Name] = e
		}
	}

	type found struct {
		pos vec
	}
	taxis := map[string]found{}
	for _, m := range markers {
		if !guidKey.MatchString(m.Properties.Guid) {
			continue
		}
		key := strings.ToUpper(strings.ReplaceAll(m.Properties.Guid, "-", ""))
		if !want[key] {
			continue // a marker that is not one of the taxi keys
		}
		pos, _, err := worldPos(comps, m.Properties.RootComp)
		if err != nil {
			return nil, fmt.Errorf("marker %s: %w", key, err)
		}
		if _, dup := taxis[key]; dup {
			return nil, fmt.Errorf("two markers carry the key %s", key)
		}
		taxis[key] = found{pos}
	}
	for k := range want {
		if _, ok := taxis[k]; !ok {
			return nil, fmt.Errorf("no marker in the level carries the taxi key %s", k)
		}
	}

	type stand struct {
		name string
		pos  vec
	}
	var stands []stand
	for _, s := range spawns {
		root, yaw, err := worldPos(comps, s.Properties.RootComp)
		if err != nil {
			return nil, fmt.Errorf("spawn location %s: %w", s.Name, err)
		}
		sc, ok := spawnComps[inner(s.Outer.ObjectName)+"."+s.Name]
		if !ok || len(sc.Properties.Slots) == 0 {
			return nil, fmt.Errorf("spawn location %s has no slot in the dump", s.Name)
		}
		slot := sc.Properties.Slots[0].Location
		a := yaw * math.Pi / 180
		stands = append(stands, stand{s.Name, vec{
			root.X + slot.X*math.Cos(a) - slot.Y*math.Sin(a),
			root.Y + slot.X*math.Sin(a) + slot.Y*math.Cos(a),
			root.Z + slot.Z,
		}})
	}

	out := &File{
		Source: "the game's own level data: the level DeepDesert_1_Travel, the taxi service markers and the spawn locations beside them (game units, the axes of the save's markers table)",
		Level:  "DeepDesert_1_Travel",
		Read:   in.Read,
		Grid:   Grid{gridMinX, gridMaxX, gridMinY, gridMaxY, gridSectors},
		Taxis:  map[string]Taxi{},
	}
	keys := make([]string, 0, len(taxis))
	for k := range taxis {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	used := map[string]string{}
	for _, k := range keys {
		m := taxis[k].pos
		best, bestD := -1, math.Inf(1)
		for i, s := range stands {
			if d := math.Hypot(s.pos.X-m.X, s.pos.Y-m.Y); d < bestD {
				best, bestD = i, d
			}
		}
		if best < 0 || bestD > maxStandDistance {
			return nil, fmt.Errorf("no spawn location within %.0f units of the marker %s", maxStandDistance, k)
		}
		if other, dup := used[stands[best].name]; dup {
			return nil, fmt.Errorf("the markers %s and %s have the same nearest spawn location %s", other, k, stands[best].name)
		}
		used[stands[best].name] = k
		s := stands[best].pos
		cell, err := Cell(m.X, m.Y)
		if err != nil {
			return nil, fmt.Errorf("marker %s: %w", k, err)
		}
		if c2, err := Cell(s.X, s.Y); err != nil || c2 != cell {
			return nil, fmt.Errorf("the spawn location of %s is not in the marker's cell %s", k, cell)
		}
		out.Taxis[k] = Taxi{Cell: cell, Marker: Point{round1(m.X), round1(m.Y), round1(m.Z)}, Stand: Point{round1(s.X), round1(s.Y), round1(s.Z)}}
	}
	return out, nil
}

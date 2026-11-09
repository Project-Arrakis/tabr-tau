package main

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type obj = map[string]any

const (
	locA = "6F72F943466BF7C8FC1D3E88853764D5"
	locB = "5B1E5FA645B571DF9714B2B2F8EDD06B"
	guid = "6F72F943-466BF7C8-FC1D3E88-853764D5"
	lvl  = "DeepDesert_1_Travel:PersistentLevel."
)

func write(t *testing.T, dir, name string, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func loc(x, y, z float64) obj { return obj{"X": x, "Y": y, "Z": z} }

// A marker actor and its root component, in the shape of the game's level dump.
func marker(name, g string, x, y, z float64) []obj {
	return []obj{
		{"Type": markerClass, "Name": name, "Outer": obj{"ObjectName": "Level'" + lvl[:len(lvl)-1] + "'"}, "Properties": obj{"m_Guid": g, "RootComponent": obj{"ObjectName": "SceneComponent'" + lvl + name + ".RootComponent'"}}},
		{"Type": "SceneComponent", "Name": "RootComponent", "Outer": obj{"ObjectName": "BP_X'" + lvl + name + "'"}, "Properties": obj{"RelativeLocation": loc(x, y, z)}},
	}
}

// A spawn location actor, its root and its component with one slot.
func spawn(name string, x, y, z, yaw float64, slot obj) []obj {
	root := obj{"RelativeLocation": loc(x, y, z)}
	if yaw != 0 {
		root["RelativeRotation"] = obj{"Pitch": 0, "Yaw": yaw, "Roll": 0}
	}
	return []obj{
		{"Type": spawnClass, "Name": name, "Outer": obj{"ObjectName": "Level'" + lvl[:len(lvl)-1] + "'"}, "Properties": obj{"RootComponent": obj{"ObjectName": "SceneComponent'" + lvl + name + ".DefaultSceneRoot'"}}},
		{"Type": "SceneComponent", "Name": "DefaultSceneRoot", "Outer": obj{"ObjectName": "BP_S'" + lvl + name + "'"}, "Properties": root},
		{"Type": spawnComponent, "Name": "StaticSpawnLocation", "Outer": obj{"ObjectName": "BP_S'" + lvl + name + "'"}, "Properties": obj{"m_Slots": []obj{{"Location": slot, "Yaw": 0}}}},
	}
}

func join(parts ...[]obj) []obj {
	var out []obj
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func namesFile(t *testing.T, dir string, keys ...string) string {
	places := obj{"AAAA0000AAAA0000AAAA0000AAAA0000": obj{"name": "Some Tradepost", "kind": "tradepost"}}
	for _, k := range keys {
		places[k] = obj{"name": "Deep Desert Taxi Service", "kind": "taxi"}
	}
	return write(t, dir, "names.json", obj{"places": places})
}

// Grid cells on the Deep Desert grid (kit/ddgrid): row A is the one at the greatest Y; column 5 is the middle one.
const (
	rowAY = 1000000.0 // inside row A
	colX5 = -120000.0 // inside column 5
	colX2 = -990000.0 // inside column 2
)

func TestGenerateReadsMarkersAndTheirSpawnLocationsAndPutsThemInRowA(t *testing.T) {
	dir := t.TempDir()
	level := join(
		marker("M1", guid, colX5, rowAY+1000, 19000),
		marker("M2", strings.ToLower(locB[:8]+"-"+locB[8:16]+"-"+locB[16:24]+"-"+locB[24:]), colX2, rowAY-2000, 17000),
		spawn("S1", colX5-500, rowAY+300, 18000, 90, loc(0, 0, -80)),
		spawn("S2", colX2+1500, rowAY-2500, 17050, 0, loc(0, 0, -14.46)),
		marker("OtherMarker", "11111111-22222222-33333333-44444444", 0, 0, 0), // a marker that is not a taxi key is ignored
	)
	f, err := Generate(Inputs{Level: write(t, dir, "l.json", level), Names: namesFile(t, dir, locA, locB), Read: "2026-10-30"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Taxis) != 2 {
		t.Fatalf("%+v", f.Taxis)
	}
	a, b := f.Taxis[locA], f.Taxis[locB]
	if a.Cell != "A5" || b.Cell != "A2" {
		t.Errorf("cells %q %q", a.Cell, b.Cell)
	}
	if a.Marker != (Point{colX5, rowAY + 1000, 19000}) || a.Stand != (Point{colX5 - 500, rowAY + 300, 17920}) {
		t.Errorf("%+v", a)
	}
	if b.Stand != (Point{colX2 + 1500, rowAY - 2500, 17035.5}) { // 17050 - 14.46 rounded to one decimal
		t.Errorf("%+v", b)
	}
	if a.Observed || b.Observed {
		t.Error("a fresh file says nobody has stood there")
	}
	if f.Level != "DeepDesert_1_Travel" || f.Read != "2026-10-30" || f.Grid.Sectors != 9 {
		t.Errorf("%+v", f)
	}
}

func TestTheSlotIsTurnedByTheSpawnLocationsYaw(t *testing.T) {
	dir := t.TempDir()
	level := join(marker("M1", guid, colX5, rowAY, 100), spawn("S1", colX5, rowAY, 100, 90, loc(100, 0, 0)))
	f, err := Generate(Inputs{Level: write(t, dir, "l.json", level), Names: namesFile(t, dir, locA), Read: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if s := f.Taxis[locA].Stand; s.X != colX5 || s.Y != rowAY+100 { // an offset along the actor's X, turned a quarter turn, is along Y
		t.Errorf("%+v", s)
	}
}

func TestGenerateRefusesWhatIsNotInRowA(t *testing.T) {
	dir := t.TempDir()
	// the same marker, one row down (row B)
	level := join(marker("M1", guid, colX5, rowAY-300000, 100), spawn("S1", colX5, rowAY-300000, 100, 0, loc(0, 0, 0)))
	_, err := Generate(Inputs{Level: write(t, dir, "l.json", level), Names: namesFile(t, dir, locA), Read: "d"})
	if err == nil || !strings.Contains(err.Error(), "not in row A") {
		t.Errorf("a point outside row A is not offered: %v", err)
	}
	// outside the grid altogether
	level = join(marker("M1", guid, 5e6, rowAY, 100), spawn("S1", 5e6, rowAY, 100, 0, loc(0, 0, 0)))
	if _, err := Generate(Inputs{Level: write(t, dir, "l2.json", level), Names: namesFile(t, dir, locA), Read: "d"}); err == nil || !strings.Contains(err.Error(), "outside the Deep Desert grid") {
		t.Errorf("%v", err)
	}
}

func TestGenerateRefusesAnIncompleteLevel(t *testing.T) {
	dir := t.TempDir()
	ok := join(marker("M1", guid, colX5, rowAY, 100), spawn("S1", colX5, rowAY, 100, 0, loc(0, 0, 0)))
	for name, c := range map[string]struct {
		level []obj
		keys  []string
		want  string
	}{
		"a taxi key with no marker":           {ok, []string{locA, locB}, "no marker in the level carries the taxi key"},
		"no taxi key in the names file":       {ok, nil, "no key of the kind taxi"},
		"no spawn location beside the marker": {join(marker("M1", guid, colX5, rowAY, 100), spawn("S1", colX5+50000, rowAY, 100, 0, loc(0, 0, 0))), []string{locA}, "no spawn location within"},
		"two markers with one key":            {join(ok, marker("M9", guid, colX5, rowAY, 100)), []string{locA}, "two markers carry the key"},
		"two markers for one spawn location":  {join(ok, marker("M2", "5B1E5FA6-45B571DF-9714B2B2-F8EDD06B", colX5+100, rowAY, 100)), []string{locA, locB}, "same nearest spawn location"},
		"a spawn location with no slot":       {join(marker("M1", guid, colX5, rowAY, 100), spawn("S1", colX5, rowAY, 100, 0, loc(0, 0, 0))[:2]), []string{locA}, "no slot"},
		"a marker whose root has no location": {[]obj{marker("M1", guid, 0, 0, 0)[0], spawn("S1", colX5, rowAY, 100, 0, loc(0, 0, 0))[0]}, []string{locA}, "no RelativeLocation"},
	} {
		_, err := Generate(Inputs{Level: write(t, dir, "l.json", c.level), Names: namesFile(t, dir, c.keys...), Read: "d"})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
	if _, err := Generate(Inputs{Level: write(t, dir, "e.json", []obj{}), Names: namesFile(t, dir, locA), Read: "d"}); err == nil {
		t.Error("an empty dump is refused")
	}
}

func TestCellRowsAndColumns(t *testing.T) {
	for _, c := range []struct {
		x, y float64
		want string
		ok   bool
	}{
		{-1268450 + 1, 1163467 - 1, "A1", true},
		{1158400 - 1, 1163467 - 1, "A9", true},
		{-120000, 1000000, "A5", true},
		{-120000, 1163467 - 269217.0 - 1, "B5", false}, // row B is a valid cell but not row A
		{0, -1259486 + 1, "I5", false},
	} {
		got, err := Cell(c.x, c.y)
		if got != c.want || (err == nil) != c.ok {
			t.Errorf("Cell(%v, %v) = %q, %v; want %q, ok %v", c.x, c.y, got, err, c.want, c.ok)
		}
	}
	if _, err := Cell(-2e6, 0); err == nil {
		t.Error("outside the grid")
	}
}

// squaredUpCell is Cell as it was before the cell letters moved to the shared module: the grid counted on the picture's squared-up rectangle (west edge -1,268,450, north edge
// 1,165,416, nine columns 269,650 wide and nine rows 269,650 high). It stays here so that the difference between the two can be tested; it is the code that pinned the answers
// before the move.
func squaredUpCell(x, y float64) (string, error) {
	w := (gridMaxX - gridMinX) / gridSectors
	h := (gridMaxY - gridMinY) / gridSectors
	col := int(math.Floor((x-gridMinX)/w)) + 1
	row := int(math.Floor((gridMaxY - y) / h))
	if col < 1 || col > gridSectors || row < 0 || row >= gridSectors {
		return "", errOutside
	}
	cell := string(rune('A'+row)) + string(rune('0'+col))
	if row != 0 {
		return cell, errNotRowA
	}
	return cell, nil
}

var errOutside, errNotRowA = errors.New("outside"), errors.New("not row A")

// The row borders of the game's grid (north edge 1,163,467, rows 269,217 high) and of the picture's rectangle (north edge 1,165,416, rows 269,650 high). Row A of the game's grid
// ends at y 894,250, row A of the rectangle at y 895,766.
func TestCellIsTheGamesGridAndRowAIsTheOnlyRowOffered(t *testing.T) {
	for _, c := range []struct {
		x, y float64
		want string
		ok   bool
	}{
		{-1268450, 1163467, "A1", true},    // the north-west corner: a cell owns its north and west edges
		{-1268450, 1163467 + 1, "", false}, // one unit north of the grid
		{1158400 - 1, 1163467, "A9", true}, // just inside the east edge
		{1158400, 1163467, "", false},      // the east edge is outside
		{-120000, 894251, "A5", true},      // one unit inside row A
		{-120000, 894250, "B5", false},     // on the border: row B owns its north edge, and only row A is offered
		{-120000, 895765, "A5", true},      // row B of the picture's rectangle, row A of the game's grid
		{0, 1163467 - 9*269217 + 1, "I5", false},
		{0, 1163467 - 9*269217, "", false}, // the south edge is outside
	} {
		got, err := Cell(c.x, c.y)
		if got != c.want || (err == nil) != c.ok {
			t.Errorf("Cell(%v, %v) = %q, %v; want %q, ok %v", c.x, c.y, got, err, c.want, c.ok)
		}
	}
}

// The move changed the letter only where the game's grid and the picture's rectangle disagree: near a row border, and in the 1,949 units past the picture's north edge and the 1,948
// before its south edge. Everywhere else (every x, every y at least 2,000 units from a border) the two give the same cell and the same error. The positions are on a fixed grid of
// points and a seeded scatter.
func TestCellAgreesWithThePictureRectangleExceptNearRowBorders(t *testing.T) {
	const gameMaxY, gameH, picMaxY, picH = 1163467.0, 269217.0, 1165416.0, 269650.0
	nearBorder := func(y float64) bool {
		for k := 0; k <= 9; k++ {
			lo, hi := gameMaxY-float64(k)*gameH, picMaxY-float64(k)*picH
			if lo > hi {
				lo, hi = hi, lo
			}
			if y >= lo-1 && y <= hi+1 {
				return true
			}
		}
		return false
	}
	differ, agree := 0, 0
	check := func(x, y float64) {
		a, ea := Cell(x, y)
		b, eb := squaredUpCell(x, y)
		if a == b && (ea == nil) == (eb == nil) {
			agree++
			return
		}
		differ++
		if !nearBorder(y) {
			t.Errorf("(%v, %v): now %q %v, before %q %v, and y is not between two row borders", x, y, a, ea, b, eb)
		}
	}
	for x := -1.4e6; x < 1.3e6; x += 54321.5 {
		for y := -1.4e6; y < 1.3e6; y += 3001.25 {
			check(x, y)
		}
	}
	state := uint64(12345)
	for i := 0; i < 20000; i++ {
		state = state*6364136223846793005 + 1442695040888963407
		x := -1.4e6 + float64(state>>40)/float64(1<<24)*2.7e6
		state = state*6364136223846793005 + 1442695040888963407
		y := -1.4e6 + float64(state>>40)/float64(1<<24)*2.7e6
		check(x, y)
	}
	if differ == 0 || agree == 0 {
		t.Errorf("%d differ, %d agree: the test does not cover both", differ, agree)
	}
	// the border bands themselves are where the answers move: halfway between the borders of the same index, the two name different rows
	for k := 1; k < 9; k++ {
		y := ((gameMaxY - float64(k)*gameH) + (picMaxY - float64(k)*picH)) / 2
		a, _ := Cell(0, y)
		b, _ := squaredUpCell(0, y)
		if a == b {
			t.Errorf("row border %d, y %.0f: both say %q", k, y, a)
		}
	}
}

func TestCellNumbersThatAreNotOrdinaryAreOutside(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	for _, c := range []struct{ x, y float64 }{{nan, 0}, {0, nan}, {inf, 0}, {-inf, 0}, {0, inf}, {0, -inf}, {1e300, 0}, {0, -1e300}} {
		if got, err := Cell(c.x, c.y); got != "" || err == nil {
			t.Errorf("Cell(%v, %v) = %q, %v: want outside the grid", c.x, c.y, got, err)
		}
	}
}

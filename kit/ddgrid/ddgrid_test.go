package ddgrid

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"os"
	"strings"
	"testing"
)

// The frame is five numbers that other programs rely on; a change of one of them is a change of the grid.
func TestTheFrameIsTheMeasuredOne(t *testing.T) {
	if N != 9 || MinX != -1268450 || MaxY != 1163467 || CellW != 269650 || CellH != 269217 {
		t.Fatalf("frame %v %v %v %v %v", N, MinX, MaxY, CellW, CellH)
	}
	if MaxX != 1158400 || MinY != -1259486 {
		t.Errorf("the grid ends at x %v and y %v", float64(MaxX), float64(MinY))
	}
}

func TestEveryBorderOneUnitEitherSide(t *testing.T) {
	const letters = "ABCDEFGHI"
	for row := 0; row <= N; row++ {
		border := MaxY - float64(row)*CellH
		var wantAbove, wantBelow, wantOn string
		if row > 0 {
			wantAbove = letters[row-1:row] + "5"
		}
		if row < N {
			wantBelow = letters[row:row+1] + "5"
			wantOn = wantBelow // a cell owns its north edge
		}
		if got := Cell(0, border+1); got != wantAbove {
			t.Errorf("row border %d, one unit north: %q, want %q", row, got, wantAbove)
		}
		if got := Cell(0, border-1); got != wantBelow {
			t.Errorf("row border %d, one unit south: %q, want %q", row, got, wantBelow)
		}
		if got := Cell(0, border); got != wantOn {
			t.Errorf("row border %d exactly: %q, want %q", row, got, wantOn)
		}
	}
	for col := 0; col <= N; col++ {
		border := MinX + float64(col)*CellW
		var wantLeft, wantOn string
		if col > 0 {
			wantLeft = "E" + string(rune('0'+col))
		}
		if col < N {
			wantOn = "E" + string(rune('1'+col)) // a cell owns its west edge
		}
		if got := Cell(border-1, 0); got != wantLeft {
			t.Errorf("column border %d, one unit west: %q, want %q", col, got, wantLeft)
		}
		if got := Cell(border, 0); got != wantOn {
			t.Errorf("column border %d exactly: %q, want %q", col, got, wantOn)
		}
		if got := Cell(border+1, 0); got != wantOn {
			t.Errorf("column border %d, one unit east: %q, want %q", col, got, wantOn)
		}
	}
}

func TestCornersTheOutsideAndNumbersThatAreNotOrdinary(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	negZero := math.Copysign(0, -1)
	for _, c := range []struct {
		name string
		x, y float64
		want string
	}{
		{"north-west corner", MinX, MaxY, "A1"},
		{"north-east, just inside", MaxX - 1, MaxY, "A9"},
		{"south-west, just inside", MinX, MinY + 1, "I1"},
		{"south-east, just inside", MaxX - 1, MinY + 1, "I9"},
		{"south-east corner is outside", MaxX, MinY, ""},
		{"east of the grid", MaxX, 0, ""},
		{"west of the grid", MinX - 1, 0, ""},
		{"north of the grid", 0, MaxY + 1, ""},
		{"south of the grid", 0, MinY, ""},
		{"far east", 1e9, 0, ""},
		{"far south", 0, -1e9, ""},
		{"origin", 0, 0, "E5"},
		{"negative zero", negZero, negZero, "E5"},
		{"a negative position inside", -307000, -585000, "G4"},
		{"NaN x", nan, 0, ""},
		{"NaN y", 0, nan, ""},
		{"NaN both", nan, nan, ""},
		{"+Inf x", inf, 0, ""},
		{"-Inf x", -inf, 0, ""},
		{"+Inf y", 0, inf, ""},
		{"-Inf y", 0, -inf, ""},
		{"largest finite", math.MaxFloat64, -math.MaxFloat64, ""},
		{"smallest subnormal", math.SmallestNonzeroFloat64, math.SmallestNonzeroFloat64, "E5"},
	} {
		if got := Cell(c.x, c.y); got != c.want {
			t.Errorf("%s: Cell(%v, %v) = %q, want %q", c.name, c.x, c.y, got, c.want)
		}
	}
}

// The grid and the squared-up rectangle of a map picture (north edge 1,165,416, rows 269,650 high) have the same columns and differ in the rows; the test says where.
func TestTheGridIsNotTheSquaredUpRectangle(t *testing.T) {
	const picMaxY, picH = 1165416.0, 269650.0
	pic := func(y float64) int { return int(math.Floor((picMaxY - y) / picH)) }
	grid := func(y float64) int { return int(math.Floor((MaxY - y) / CellH)) }
	// halfway between the two borders of the same index the two name different rows
	for row := 1; row < N; row++ {
		gridBorder, picBorder := MaxY-float64(row)*CellH, picMaxY-float64(row)*picH
		mid := (gridBorder + picBorder) / 2
		if pic(mid) == grid(mid) {
			t.Errorf("row border %d: the two agree at y %.0f", row, mid)
		}
		if got := Cell(0, mid); got[:1] != string(rune('A'+grid(mid))) {
			t.Errorf("y %.0f: %q", mid, got)
		}
	}
	// where the picture rectangle's row A ends (y 895,766) the grid's row A goes on to y 894,250
	if got := Cell(0, 895000); got != "A5" {
		t.Errorf("y 895000: %q, want A5", got)
	}
	// the columns are the same for every x
	for x := MinX - 5; x < MaxX+5; x += 12345.6 {
		col := int(math.Floor((x - MinX) / (2426850.0 / 9)))
		got := Cell(x, 0)
		inside := x >= MinX && x < MaxX
		if inside != (got != "") || (inside && got[1:] != string(rune('1'+col))) {
			t.Fatalf("x %.1f: %q, column %d", x, got, col+1)
		}
	}
}

// The answers over a seeded set of positions, pinned by hash: positions over a box wider than the grid, and positions on and one unit and a hundredth either side of every
// border. This is the check that a change of the code keeps every answer.
func TestSeededCorpus(t *testing.T) {
	var sb strings.Builder
	state := uint64(0x9E3779B97F4A7C15)
	next := func() float64 { // splitmix64: the same stream on every Go version
		state += 0x9E3779B97F4A7C15
		z := state
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		z ^= z >> 31
		return float64(z>>11) / (1 << 53)
	}
	add := func(x, y float64) { sb.WriteString(Cell(x, y)); sb.WriteByte('|') }
	for i := 0; i < 200000; i++ {
		add(-1.6e6+next()*3.2e6, -1.5e6+next()*3.0e6)
	}
	for row := 0; row <= 9; row++ {
		b := MaxY - float64(row)*CellH
		for _, d := range []float64{-1, -0.01, 0, 0.01, 1} {
			for col := 0; col <= 9; col++ {
				add(MinX+float64(col)*CellW+0.5*CellW*float64(col%2), b+d)
			}
		}
	}
	for col := 0; col <= 9; col++ {
		b := MinX + float64(col)*CellW
		for _, d := range []float64{-1, -0.01, 0, 0.01, 1} {
			for row := 0; row <= 9; row++ {
				add(b+d, MaxY-float64(row)*CellH-0.5*CellH)
			}
		}
	}
	sum := sha256.Sum256([]byte(sb.String()))
	const want = "a60772ed0ff403b2c2b04fbacbcd42f154c3f86dd351a7d6347f5a9317db43fc"
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("corpus hash %s, want %s", got, want)
	}
}

// NOTICE carries the licence of the project whose measure this is, and says that the frame is covered by it.
func TestNoticeCarriesTheLicenceOfTheFrame(t *testing.T) {
	b, err := os.ReadFile("../NOTICE")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"ddgrid", "liveMapSector.js", "RedBlink", "MIT License"} {
		if !strings.Contains(string(b), s) {
			t.Errorf("NOTICE does not mention %q", s)
		}
	}
}

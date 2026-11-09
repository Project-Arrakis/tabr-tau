// Package ddgrid names the squares of the lettered grid the game draws over the Deep Desert (A1 to I9) for a world position, in one place.
//
// The grid is nine by nine, but its cells are not square and it does not follow the rectangle of the map picture: the cells are 269,650 wide and 269,217 high, the west edge is
// at x -1,268,450 and the north edge at y 1,163,467, so the grid ends at x 1,158,400 and y -1,259,486. A picture drawn on the squared-up rectangle (north edge 1,165,416,
// south edge -1,261,434, rows 269,650 high) has the same columns, but its row borders differ from the grid's by 1,949 units at the north edge, by 433 fewer for each row
// further south, and by 1,948 units the other way at the south edge. Near a row border the two therefore give different letters; this package is the grid, not the picture.
//
// The frame is that of the sector grid of the Dune Docker console (liveMapSector.js in https://github.com/Red-Blink/dune-awakening-selfhost-docker, MIT, copyright RedBlink;
// see NOTICE), which measured it in the game. Columns count from the west, rows are letters from the largest y (A) to the smallest (I). A cell owns its north and west edges:
// a position exactly on the border between rows A and B is in B, and one exactly on the border between columns 1 and 2 is in column 2; the east and south edges of the whole
// grid are outside it.
//
// The package has no state and reads nothing. World positions are in the game's units (centimetres), the axes of a save's position columns.
package ddgrid

import (
	"fmt"
	"math"
)

// The grid's frame, in world units.
const (
	N     = 9          // cells on a side
	MinX  = -1268450.0 // west edge
	MaxY  = 1163467.0  // north edge
	CellW = 269650.0   // width of a cell (along x)
	CellH = 269217.0   // height of a cell (along y)

	MaxX = MinX + N*CellW // east edge, 1,158,400
	MinY = MaxY - N*CellH // south edge, -1,259,486
)

// Cell is the square a world position is in ("G4"), or "" when it is outside the grid or when either number is not finite.
func Cell(x, y float64) string {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return ""
	}
	col := math.Floor((x - MinX) / CellW)
	row := math.Floor((MaxY - y) / CellH)
	if col < 0 || col >= N || row < 0 || row >= N {
		return ""
	}
	return fmt.Sprintf("%c%d", rune('A'+int(row)), int(col)+1)
}

// Package fillables is the game's table of fillable items: for every item template that can hold a liquid (a Literjon, a Decajon, a Dewpack, a blood sack, a worn
// stillsuit) the most it holds, the amount a new item starts with, the liquid it takes and whether it is a container.
//
// The rows are the game's own item table, DT_ItemTableFillables (row structure FillableItemTableRow, field FillableStats), read from the installed game's content files and
// kept as a snapshot in item_table_fillables.json: item names and whole numbers, nothing else. The unit is that of a save's FFillableItemStats.CurrentAmount (a half-full
// Literjon Mk6 holds 11000 of 20000). A new game version is a new snapshot; a row it adds or changes shows up in the diff and in the tests.
//
// Which rows a program fills, and how it treats the worn stillsuits (their Container is false: the game changes the amount itself), is the program's decision. The package
// only says what the game's table says.
package fillables

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// The liquids the table names in Row.Restriction.
const (
	Water = "Water"
	Blood = "Blood"
)

// Row is one row of the game's table of fillable items (its FillableStats).
type Row struct {
	Name          string  `json:"name"`          // the item template
	MaxAmount     int32   `json:"maxAmount"`     // the capacity
	CurrentAmount float64 `json:"currentAmount"` // the amount a new item starts with (0 except the consumable WaterPack)
	Restriction   string  `json:"restriction"`   // FillableTypeRestriction: the liquid it takes (Water or Blood)
	Container     bool    `json:"container"`     // bIsContainer: false for the stillsuits (the water they store) and the consumable
}

type file struct {
	Source string `json:"source"`
	Rows   []Row  `json:"rows"`
}

//go:embed item_table_fillables.json
var snapshot []byte

var (
	rows   []Row
	byName map[string]Row
	source string
)

func init() {
	var err error
	if source, rows, err = Parse(snapshot); err != nil {
		panic("fillables: item_table_fillables.json: " + err.Error())
	}
	byName = make(map[string]Row, len(rows))
	for _, r := range rows {
		byName[r.Name] = r
	}
}

// Table is the snapshot of the game's table, sorted by name. The caller gets a copy.
func Table() []Row { return append([]Row(nil), rows...) }

// Lookup returns the row of an item template.
func Lookup(template string) (Row, bool) {
	r, ok := byName[template]
	return r, ok
}

// Source says where the snapshot came from.
func Source() string { return source }

// Parse reads the text of a snapshot: the source line and the rows. It refuses unknown fields, a table without rows, a row without a name, a row with no capacity or a
// negative one, and rows that are not in strictly ascending name order (which also refuses a duplicate).
func Parse(b []byte) (string, []Row, error) {
	var f file
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return "", nil, err
	}
	if len(f.Rows) == 0 {
		return "", nil, errors.New("no rows")
	}
	for i, r := range f.Rows {
		if r.Name == "" {
			return "", nil, fmt.Errorf("row %d has no name", i)
		}
		if r.MaxAmount < 0 {
			return "", nil, fmt.Errorf("row %q has a negative capacity", r.Name)
		}
		if i > 0 && f.Rows[i-1].Name >= r.Name {
			return "", nil, fmt.Errorf("row %q is a duplicate or out of order", r.Name)
		}
	}
	return f.Source, f.Rows, nil
}

// Marshal makes the text of a snapshot from a source line and rows: the rows sorted by name, one space of indent and a final newline, so that the same table always gives the
// same bytes and a refreshed snapshot shows only what changed in a diff. The rows are not checked; Parse checks them.
func Marshal(source string, in []Row) ([]byte, error) {
	sorted := append([]Row(nil), in...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a].Name < sorted[b].Name })
	out, err := json.MarshalIndent(file{Source: source, Rows: sorted}, "", " ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

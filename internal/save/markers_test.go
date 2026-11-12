package save

import (
	"database/sql"
	"testing"
)

// #588: which of the game's two layouts of markers a database has, decided from its columns. The tables here are written by hand: only the columns matter.
func TestTheLayoutOfMarkersIsDecidedFromItsColumns(t *testing.T) {
	for name, c := range map[string]struct {
		ddl  string
		want bool
	}{
		"the older layout, map_name":                 {`create table markers(marker_hash_id integer, map_name text)`, false},
		"the newer layout, map_name_id":              {`create table markers(marker_hash_id integer, map_name_id integer); create table map_names(map_name_id integer, map_name text)`, true},
		"both columns is not the newer layout":       {`create table markers(marker_hash_id integer, map_name text, map_name_id integer)`, false},
		"neither column":                             {`create table markers(marker_hash_id integer)`, false},
		"no markers table":                           {`create table other(a integer)`, false},
		"a map_name_id column in another table only": {`create table markers(map_name text); create table player_markers(map_name_id integer)`, false},
	} {
		t.Run(name, func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if _, err := db.Exec(c.ddl); err != nil {
				t.Fatal(err)
			}
			if got := markersHaveMapNameID(db); got != c.want {
				t.Fatalf("markersHaveMapNameID = %v, want %v", got, c.want)
			}
		})
	}
}

func TestTheTableExpressionsAreTheTablesOrAJoinOnMapNames(t *testing.T) {
	old, now := &Save{}, &Save{mapNameIDs: true}
	if old.MarkersTable() != "markers" || old.PlayerMarkersTable() != "player_markers" || old.MarkersMapNameID() {
		t.Errorf("the older layout reads the tables themselves: %q %q", old.MarkersTable(), old.PlayerMarkersTable())
	}
	for _, e := range []string{now.MarkersTable(), now.PlayerMarkersTable()} {
		if !(len(e) > 2 && e[0] == '(' && e[len(e)-1] == ')') {
			t.Errorf("the newer layout reads a parenthesised select: %q", e)
		}
	}
	if !now.MarkersMapNameID() {
		t.Error("MarkersMapNameID")
	}
}

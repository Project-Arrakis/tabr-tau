package testsave

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Layout is one of the two table layouts the game's save has had (#588).
type Layout int

const (
	// LayoutMapName is the layout up to the game's update of early November 2026: markers and player_markers carry the map as text, in map_name (save.KnownSchema).
	LayoutMapName Layout = iota
	// LayoutMapNameID is the layout since: map_name_id and a map_names table in their place, two Landsraad reward tables, a few other indexes (save.KnownSchemaMapNames).
	LayoutMapNameID
)

// Layouts is both layouts, in the order of the game's history.
var Layouts = []Layout{LayoutMapName, LayoutMapNameID}

func (l Layout) String() string {
	if l == LayoutMapNameID {
		return "map_name_id"
	}
	return "map_name"
}

// EnvLayout names the environment variable that picks the layout of every save a test builds without naming one: "map_names" (or "map_name_id") for the newer layout,
// anything else for the older. CI runs the editor's packages once with it set, so that every test that builds a save also runs against the newer layout.
const EnvLayout = "TABR_TEST_LAYOUT"

// DefaultLayout is the layout of a save built without naming one.
func DefaultLayout() Layout {
	switch strings.ToLower(os.Getenv(EnvLayout)) {
	case "map_names", "map_name_id":
		return LayoutMapNameID
	}
	return LayoutMapName
}

// SchemaFor returns the real DDL of a layout.
func SchemaFor(l Layout) string {
	if l == LayoutMapNameID {
		return save.KnownSchemaMapNames
	}
	return save.KnownSchema
}

// EmptyIn is Empty in a layout.
func EmptyIn(t testing.TB, l Layout) *save.Save { return buildIn(t, l, nil) }

// WithSQLIn is WithSQL in a layout. The statements are written for the older layout (they name markers.map_name) and are run on it; for the newer one the save is converted
// afterwards, as the game would have: rows of markers and player_markers get the id of their map in map_names.
func WithSQLIn(t testing.TB, l Layout, extra string) *save.Save {
	return buildIn(t, l, func(db *sql.DB) {
		if _, err := db.Exec(extra); err != nil {
			t.Fatalf("extra SQL failed: %v\n%s", err, extra)
		}
	})
}

// PlayerWithSQLIn is PlayerWithSQL in a layout (the statements are written for the older layout, see WithSQLIn).
func PlayerWithSQLIn(t testing.TB, l Layout, extra string) *save.Save {
	return buildPlayerIn(t, l, extra)
}

// PlayerWithNewerSQL is Player() in the newer layout plus extra statements written for that layout, run after the conversion: for a test that changes what only the newer layout has
// (map_names, the Landsraad reward tables, markers.map_name_id).
func PlayerWithNewerSQL(t testing.TB, extra string) *save.Save {
	return buildThen(t, LayoutMapNameID, playerRows(t, ""), func(db *sql.DB) {
		if _, err := db.Exec(extra); err != nil {
			t.Fatalf("extra SQL failed: %v\n%s", err, extra)
		}
	})
}

// OpenSchemaOnlyIn is OpenSchemaOnly in a layout.
func OpenSchemaOnlyIn(t testing.TB, l Layout) *sql.DB { return memIn(t, l) }

// object is one stored definition of a database: a table, an index or a trigger.
type object struct{ typ, name, tbl, sql string }

func objectsOf(db *sql.DB) ([]object, error) {
	rs, err := db.Query(`select type, name, tbl_name, coalesce(sql,'') from sqlite_master where name not like 'sqlite\_%' escape '\' order by rowid`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []object
	for rs.Next() {
		var o object
		if err := rs.Scan(&o.typ, &o.name, &o.tbl, &o.sql); err != nil {
			return nil, err
		}
		if o.sql != "" {
			out = append(out, o)
		}
	}
	return out, rs.Err()
}

func objectsOfDDL(t testing.TB, ddl string) []object {
	t.Helper()
	db := memDDL(t, ddl)
	defer db.Close()
	objs, err := objectsOf(db)
	if err != nil {
		t.Fatal(err)
	}
	return objs
}

// toMapNameID converts a database that has the older layout, and whatever a test added to it (extra tables, triggers, a changed game table), to the newer one in place, the way
// an update of the game does: the two Landsraad tables and map_names are created; markers and player_markers are rebuilt from the game's own new definitions with the id of each
// row's map in place of its name; the indexes the new layout drops are dropped and the ones it adds are created. Nothing else of the database is touched. Foreign keys are off, as in
// every builder here.
func toMapNameID(t testing.TB, db *sql.DB) {
	t.Helper()
	oldObjs, newObjs := objectsOfDDL(t, save.KnownSchema), objectsOfDDL(t, save.KnownSchemaMapNames)
	inNew, inOld := map[string]bool{}, map[string]bool{}
	for _, o := range newObjs {
		inNew[o.name] = true
	}
	for _, o := range oldObjs {
		inOld[o.name] = true
	}
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("converting the test save to the newer layout: %v\n%s", err, q)
		}
	}
	mustExec(`create temp table zz_markers as select * from markers`)
	mustExec(`create temp table zz_player_markers as select * from player_markers`)
	mustExec(`drop table player_markers`)
	mustExec(`drop table markers`)
	for _, o := range oldObjs {
		if o.typ == "index" && !inNew[o.name] {
			mustExec(`drop index if exists "` + o.name + `"`)
		}
	}
	have := map[string]bool{}
	cur, err := objectsOf(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range cur {
		have[o.name] = true
	}
	for _, typ := range []string{"table", "index", "trigger"} {
		for _, o := range newObjs {
			// an object the older layout has and the save lacks (a test removed it on purpose) stays missing; the rebuilt markers tables and their indexes are made again
			if o.typ == typ && !have[o.name] && (!inOld[o.name] || o.tbl == "markers" || o.tbl == "player_markers") {
				mustExec(o.sql)
			}
		}
	}
	mustExec(`insert into map_names(map_name_id, map_name) select row_number() over (order by map_name), map_name from
		(select map_name from zz_markers union select map_name from zz_player_markers)`)
	mustExec(insertConverted("markers", "zz_markers", db, t))
	mustExec(insertConverted("player_markers", "zz_player_markers", db, t))
	mustExec(`drop table zz_markers`)
	mustExec(`drop table zz_player_markers`)
}

// insertConverted is the statement that copies the rows of the temporary copy of an older-layout table (tmp) into the newer-layout table (table), column by column, with the id of the
// map in place of its name.
func insertConverted(table, tmp string, db *sql.DB, t testing.TB) string {
	t.Helper()
	rs, err := db.Query(`select name from pragma_table_info(?)`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var cols, vals []string
	for rs.Next() {
		var c string
		if err := rs.Scan(&c); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, `"`+c+`"`)
		if c == "map_name_id" {
			vals = append(vals, `(select mn.map_name_id from map_names mn where mn.map_name=o.map_name)`)
		} else {
			vals = append(vals, `o."`+c+`"`)
		}
	}
	return fmt.Sprintf(`insert into "%s"(%s) select %s from %s o`, table, strings.Join(cols, ","), strings.Join(vals, ","), tmp) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
}

// AddMarker adds one map marker to a save, in whichever layout the save has: map_name in the older, the id of the map in map_names (made when the map is new) in the newer.
func AddMarker(t testing.TB, s *save.Save, hash int64, mapName, markerType string, x, y, z float64) {
	t.Helper()
	if s.MarkersMapNameID() {
		Exec(t, s, `insert or ignore into map_names(map_name_id, map_name) values ((select coalesce(max(map_name_id),0)+1 from map_names), ?)`, mapName)
		Exec(t, s, `insert into markers(marker_hash_id,dimension_index,map_name_id,marker_type,x,y,z) values (?,0,(select map_name_id from map_names where map_name=?),?,?,?,?)`, hash, mapName, markerType, x, y, z)
		return
	}
	Exec(t, s, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values (?,0,?,?,?,?,?)`, hash, mapName, markerType, x, y, z)
}

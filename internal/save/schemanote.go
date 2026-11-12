package save

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DBA-16 of the 2026-10-22 audit: the schema fingerprint compares triggers, views and virtual tables (code that could run behind an edit), but the definitions of the tables
// themselves were never compared. They differ legitimately when the game has been updated (a migration that adds a column rewrites the stored text of its table), so a
// difference does not make the save read-only: the person is told at open, and editing continues. The editor's queries and writes name the columns they use, so an added
// column does not hurt them; a column that was renamed or removed shows as an error on the screen that uses it.

// tableDefinitions returns the stored definition of every ordinary table, keyed by name. Runs of white space and the kind of line ending are not part of a definition.
func tableDefinitions(db interface {
	Query(q string, args ...any) (*sql.Rows, error)
}) (map[string]string, error) {
	rs, err := db.Query(`select name, coalesce(sql,'') from sqlite_master where type='table' and rootpage<>0 and name not like 'sqlite\_%' escape '\'`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[string]string{}
	for rs.Next() {
		var name, def string
		if err := rs.Scan(&name, &def); err != nil {
			return nil, err
		}
		out[name] = strings.Join(strings.Fields(def), " ")
	}
	return out, rs.Err()
}

// The game changed the layout of its save in an update of early November 2026 (markers.map_name became markers.map_name_id with a map_names table, and two Landsraad
// tables appeared, #588). The editor knows both layouts, KnownSchema (before) and KnownSchemaMapNames (since), and compares a save with each: a save is known when it matches
// either one exactly, and otherwise the note names the differences from the layout it is nearest to.
var knownTables = sync.OnceValues(func() ([]map[string]string, error) {
	var out []map[string]string
	for _, ddl := range []string{KnownSchema, KnownSchemaMapNames} {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(1)
		if _, err := db.Exec(ddl); err != nil {
			db.Close()
			return nil, err
		}
		defs, err := tableDefinitions(db)
		db.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, defs)
	}
	return out, nil
})

// schemaNote says, in a sentence for the person, which tables of db are not defined as in the game's schema the editor was built against, or "" when all are (or when the
// comparison could not be made: the fingerprint reports a save that cannot be read, and a note that cannot be trusted is not shown). The editor knows two layouts of the
// game's save; a save that matches either gives "", and any other save is described by its differences from the layout with fewer of them.
func schemaNote(db *sql.DB) string {
	layouts, err := knownTables()
	if err != nil {
		return ""
	}
	got, err := tableDefinitions(db)
	if err != nil {
		return ""
	}
	var best string
	bestN := -1
	for _, want := range layouts {
		note, n := schemaDifferences(want, got)
		if n == 0 {
			return ""
		}
		if bestN < 0 || n < bestN {
			best, bestN = note, n
		}
	}
	return best
}

// schemaDifferences compares the definitions of a save's tables with one known layout: how many tables differ, and the sentence that says which.
func schemaDifferences(want, got map[string]string) (string, int) {
	var differ, extra, missing []string
	for k, v := range got {
		if w, ok := want[k]; !ok {
			extra = append(extra, k)
		} else if w != v {
			differ = append(differ, k)
		}
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			missing = append(missing, k)
		}
	}
	n := len(differ) + len(extra) + len(missing)
	if n == 0 {
		return "", 0
	}
	var parts []string
	list := func(label string, names []string) {
		if len(names) == 0 {
			return
		}
		sort.Strings(names)
		more := ""
		if len(names) > 6 {
			more, names = fmt.Sprintf(" and %d more", len(names)-6), names[:6]
		}
		parts = append(parts, label+" "+strings.Join(names, ", ")+more)
	}
	list("defined differently:", differ)
	list("not known to this editor:", extra)
	list("missing:", missing)
	return fmt.Sprintf("This save's tables are not all defined as in the game this editor was built against (%s). The game may have been updated since. You can keep editing; if a screen reports an error about a column, restore the newest backup from the Backups list and tell the maintainers which game version you have.", strings.Join(parts, "; ")), n
}

// SchemaNote is the sentence schemaNote made when the save was loaded, or "". It is a warning only: it never makes the save read-only.
func (s *Save) SchemaNote() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.schemaDiff
}

// Package testsave builds test saves from the REAL single-player schema (internal/save/schema.sql, exposed as save.KnownSchema, is the DDL extracted
// from a real save: 94 tables, STRICT typing, CHECK constraints, foreign keys, the game's trigger).
//
// Why: the original tests used hand-written tables that silently diverged from the game's schema, so a
// wrong write could pass (audit finding F-02 / QA-1). Tests built on this package fail if code assumes a
// table, column or constraint the real game does not have. No real player data is used: rows are synthetic.
package testsave

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

var schemaSQL = save.KnownSchema

// Every test binary that builds saves here is independent of whether the developer has the game open (issue #102): the process
// check reports no game unless a test says otherwise.
func init() { save.NoGameRunningForTests() }

// Hostile is a string for every TEXT column of Hostile(): HTML and attribute breakout, quotes, markup, and
// characters that matter in CSV, SQL and JSON contexts.
const HostileText = `"><img src=x onerror=window.__pwned=1>'&<b>x</b>=cmd|' /c calc'!A1   ${7*7}`

// Schema returns the real DDL.
func Schema() string { return schemaSQL }

// TableNames returns every application table in the real schema.
func TableNames(t testing.TB) []string {
	t.Helper()
	db := mem(t)
	defer db.Close()
	rows, err := db.Query(`select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		out = append(out, n)
	}
	return out
}

func mem(t testing.TB) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("real schema failed to load: %v", err)
	}
	return db
}

// build creates a real-schema database, lets fn populate it, and opens it as a save. The working files live in
// t.TempDir() and are removed with the test.
func build(t testing.TB, fn func(db *sql.DB)) *save.Save {
	t.Helper()
	dir := t.TempDir()
	plain := filepath.Join(dir, "p.sqlite")
	db, err := sql.Open("sqlite", plain)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	// The working file is thrown away after it is read back, so it does not need to survive a crash: without these two
	// pragmas every one of the schema's statements waits for the disk, which cost most of a second per save (#89).
	if _, err := db.Exec(`pragma synchronous=off; pragma journal_mode=off`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("real schema failed to load: %v", err)
	}
	if fn != nil {
		fn(db)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(plain)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := save.Encode(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "game.db")
	if err := os.WriteFile(p, blob, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := save.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// Empty is the real schema with no rows.
func Empty(t testing.TB) *save.Save { return build(t, nil) }

// WithSQL is the real schema plus extra statements run before the save is sealed. Use it to build hostile saves:
// extra triggers, views, virtual tables, oddly named tables, altered game objects.
func WithSQL(t testing.TB, extra string) *save.Save {
	return build(t, func(db *sql.DB) {
		if _, err := db.Exec(extra); err != nil {
			t.Fatalf("extra SQL failed: %v\n%s", err, extra)
		}
	})
}

// PlayerWithSQL is Player() plus extra statements.
func PlayerWithSQL(t testing.TB, extra string) *save.Save {
	return buildPlayer(t, extra)
}

// fill inserts exactly one row into every table, choosing values by declared column type. text is used for
// every TEXT column. Constraint failures abort the test: the real schema is the authority.
func fill(t testing.TB, db *sql.DB, text string) {
	t.Helper()
	rows, err := db.Query(`select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var n string
		rows.Scan(&n)
		tables = append(tables, n)
	}
	rows.Close()
	for _, tb := range tables {
		ci, err := db.Query(`select name, upper(type) from pragma_table_info(?)`, tb)
		if err != nil {
			t.Fatal(err)
		}
		var cols []string
		var marks []string
		var args []any
		for ci.Next() {
			var name, typ string
			ci.Scan(&name, &typ)
			cols = append(cols, `"`+strings.ReplaceAll(name, `"`, `""`)+`"`)
			switch typ {
			case "INTEGER", "INT":
				marks, args = append(marks, "?"), append(args, 1)
			case "REAL":
				marks, args = append(marks, "?"), append(args, 1.5)
			case "TEXT":
				marks, args = append(marks, "?"), append(args, text)
			default: // BLOB / ANY
				if name == "components" || name == "gas_attributes" || name == "properties" {
					marks, args = append(marks, "jsonb(?)"), append(args, `{"k":"<b>x</b>"}`)
				} else {
					marks, args = append(marks, "?"), append(args, []byte{0x0c})
				}
			}
		}
		ci.Close()
		q := fmt.Sprintf(`insert into "%s"(%s) values(%s)`, strings.ReplaceAll(tb, `"`, `""`), strings.Join(cols, ","), strings.Join(marks, ",")) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli (test-only; table and column names come from the embedded DDL and identifiers cannot be bound parameters; names are quoted) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli (test-only; table and column names come from the embedded DDL and identifiers cannot be bound parameters; names are quoted)
		if _, err := db.Exec(q, args...); err != nil {                                                                                            // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli
			t.Fatalf("real schema rejected a generic row for table %s: %v", tb, err)
		}
	}
}

// Hostile is the real schema with one row in every table, every TEXT value set to HostileText. Used to prove
// that nothing in the API or UI trusts save contents.
func Hostile(t testing.TB) *save.Save {
	return build(t, func(db *sql.DB) { fill(t, db, HostileText) })
}

// Filled is like Hostile but with harmless text, for tests that need populated tables.
func Filled(t testing.TB) *save.Save {
	return build(t, func(db *sql.DB) { fill(t, db, "x") })
}

// Player is the real schema with a small, consistent single-player world that mirrors how the game links a
// character (verified against a real save): three actors owned by the account (controller, pawn, player
// state), a player_state row pointing at them, a backpack with Solari and a knife, and the item-id sequencer.
func Player(t testing.TB) *save.Save { return buildPlayer(t, "") }

func buildPlayer(t testing.TB, extra string) *save.Save {
	return build(t, func(db *sql.DB) {
		fill1(t, db, "accounts", map[string]any{"id": 1, "user": "u", "funcom_id": "FUNCOM-TEST", "takeoverable": 0, "platform_id": "PLATFORM-TEST", "platform_name": "Tester"})
		const pc, pawn, pstate = 1, 2, 3
		for id, class := range map[int]string{
			pc:     "/Game/Dune/Characters/Player/BP_DunePlayerController.BP_DunePlayerController_C",
			pawn:   "/Game/Dune/Characters/Player/BP_DunePlayerCharacter.BP_DunePlayerCharacter_C",
			pstate: "/Script/DuneSandbox.DunePlayerState",
		} {
			fill1(t, db, "actors", map[string]any{"id": id, "class": class, "map": "HaggaBasin", "location_x": 100.0, "location_y": 200.0, "location_z": 300.0,
				"owner_account_id": 1, "dimension_index": 0})
		}
		fill1(t, db, "player_state", map[string]any{"id": pc, "account_id": 1, "character_name": "Tester",
			"player_controller_id": pc, "player_pawn_id": pawn, "player_state_id": pstate})
		fill1(t, db, "inventories", map[string]any{"id": 1, "actor_id": pawn, "inventory_type": 0, "max_item_count": 5, "max_item_volume": 1000.0})
		fill1(t, db, "items", map[string]any{"id": 10, "inventory_id": 1, "stack_size": 100, "position_index": 0, "template_id": "SolarisCoin", "is_new": 0,
			"acquisition_time": 1790000000, "stats": `{"FItemStackAndDurabilityStats":[[],{}],"FCustomizationStats":[[],{}]}`, "quality_level": 0})
		fill1(t, db, "items", map[string]any{"id": 11, "inventory_id": 1, "stack_size": 1, "position_index": 1, "template_id": "Knife", "is_new": 0,
			"acquisition_time": 1790000000, "stats": `{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":10.0}],"FCustomizationStats":[[],{}]}`, "quality_level": 0})
		fill1(t, db, "items_id_sequencer", map[string]any{"next_id": 500})
		if extra != "" {
			if _, err := db.Exec(extra); err != nil {
				t.Fatalf("extra SQL failed: %v\n%s", err, extra)
			}
		}
	})
}

// fill1 inserts one row. Columns not named in set: nullable columns get NULL (so foreign keys and CHECKs that
// allow NULL are satisfied); NOT NULL columns get a type default (0 / 0.0 / "" / valid JSONB for the game's
// JSONB columns / an empty blob).
func fill1(t testing.TB, db *sql.DB, table string, set map[string]any) {
	t.Helper()
	ci, err := db.Query(`select name, upper(type), "notnull" from pragma_table_info(?)`, table)
	if err != nil {
		t.Fatal(err)
	}
	var cols, marks []string
	var args []any
	for ci.Next() {
		var name, typ string
		var notnull int
		ci.Scan(&name, &typ, &notnull)
		cols = append(cols, `"`+name+`"`)
		if v, ok := set[name]; ok {
			marks, args = append(marks, "?"), append(args, v)
			continue
		}
		if notnull == 0 {
			marks, args = append(marks, "?"), append(args, nil)
			continue
		}
		switch typ {
		case "INTEGER", "INT":
			marks, args = append(marks, "?"), append(args, 0)
		case "REAL":
			marks, args = append(marks, "?"), append(args, 0.0)
		case "TEXT":
			marks, args = append(marks, "?"), append(args, "")
		default:
			if name == "components" || name == "gas_attributes" || name == "properties" {
				marks, args = append(marks, "jsonb(?)"), append(args, "{}")
			} else {
				marks, args = append(marks, "?"), append(args, []byte{0x0c})
			}
		}
	}
	ci.Close()
	q := fmt.Sprintf(`insert into "%s"(%s) values(%s)`, table, strings.Join(cols, ","), strings.Join(marks, ",")) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli (test-only; table and column names come from the embedded DDL and identifiers cannot be bound parameters; names are quoted)
	if _, err := db.Exec(q, args...); err != nil {                                                                // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli
		t.Fatalf("fixture insert into %s: %v", table, err)
	}
}

// Exec runs one edit statement on s through the editor's only write path (Mutate) and fails the test on error.
func Exec(t testing.TB, s *save.Save, q string, args ...any) {
	t.Helper()
	if _, err := s.Mutate("test edit", func(m *save.Mut) error { _, err := m.Exec(q, args...); return err }); err != nil {
		t.Fatalf("test edit failed: %v\n%s", err, q)
	}
}

// OpenSchemaOnly returns an in-memory database holding the real schema and no rows, for tests that inspect structure.
func OpenSchemaOnly(t testing.TB) *sql.DB { return mem(t) }

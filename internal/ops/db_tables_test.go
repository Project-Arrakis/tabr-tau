package ops

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Issue #173: the Tables view and the cell editor addressed rows by rowid, which the game's 61 WITHOUT ROWID tables
// do not have. These tests run against the real schema (all 94 tables), not a hand-written one.

func withoutRowidTables(t *testing.T, o *Ops) map[string]bool {
	t.Helper()
	rows, err := o.S.Query(`select name from sqlite_master where type='table' and sql like '%WITHOUT ROWID%'`)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, r := range rows {
		out[r["name"].(string)] = true
	}
	return out
}

func TestTablesViewWorksOnAllRealSchemaTables(t *testing.T) {
	o := &Ops{S: testsave.Filled(t)} // one row in every table
	names := testsave.TableNames(t)
	if len(names) != 94 {
		t.Fatalf("the real schema has %d tables, the test expects 94", len(names))
	}
	wr := withoutRowidTables(t, o)
	if len(wr) != 61 {
		t.Fatalf("%d WITHOUT ROWID tables, the test expects 61", len(wr))
	}
	edited := 0
	for _, name := range names {
		res, err := o.TableRows(name, "", 100, 0)
		if err != nil {
			t.Errorf("%s: TableRows: %v", name, err)
			continue
		}
		m := res.(map[string]any)
		cols := m["columns"].([]string)
		rows := m["rows"].([][]any)
		if want := map[bool]string{true: "_key_", false: "_rowid_"}[wr[name]]; cols[0] != want {
			t.Errorf("%s: first column is %q, want %q", name, cols[0], want)
		}
		if len(rows) != 1 {
			t.Errorf("%s: %d rows, want the 1 the fixture inserted", name, len(rows))
			continue
		}
		if _, err := o.TableRows(name, "x", 100, 0); err != nil {
			t.Errorf("%s: search: %v", name, err)
		}
		if _, _, err := o.Export(name, "csv"); err != nil {
			t.Errorf("%s: export: %v", name, err)
		}
		if m["tableLocked"].(string) != "" {
			continue
		}
		// edit the first plain column to the value it already has: the row must be found by its key
		locked := m["locked"].(map[string]string)
		_, types, _ := o.tableInfo(name)
		for i, c := range cols[1:] {
			if _, l := locked[c]; l || types[i] == "BLOB" {
				continue
			}
			args := Args{"table": name, "column": c, "value": rows[0][i+1]}
			if wr[name] {
				args["key"] = rows[0][0]
			} else {
				args["rowid"] = float64(rows[0][0].(int64))
			}
			if _, err := o.UpdateRow(args); err != nil {
				t.Errorf("%s.%s: UpdateRow: %v", name, c, err)
			}
			// and a key that names no row is refused, not silently ignored
			if wr[name] {
				args["key"] = noSuchKey(t, o, name)
			} else {
				args["rowid"] = float64(987654321)
			}
			if _, err := o.UpdateRow(args); err == nil || !strings.Contains(err.Error(), "no such row") {
				t.Errorf("%s.%s: an unknown row must be refused, got %v", name, c, err)
			}
			edited++
			break
		}
	}
	if edited < 50 {
		t.Errorf("only %d tables had an editable column; the check has gone hollow", edited)
	}
}

// noSuchKey is a key of the right length that matches no row.
func noSuchKey(t *testing.T, o *Ops, table string) string {
	t.Helper()
	cols, types, _ := o.tableInfo(table)
	k, err := o.rowKeyOf(table, cols, types)
	if err != nil {
		t.Fatal(err)
	}
	vals := make([]string, len(k.pk))
	for i := range vals {
		vals[i] = "no such"
	}
	b, _ := json.Marshal(vals)
	return string(b)
}

func TestUpdateRowOnWithoutRowidTableChangesOnlyTheNamedRow(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `
		insert into player_travel_state values ('A', 1), ('B', 2);
		insert into player_faction_reputation values (1, 1, 10), (1, 2, 20), (2, 2, 30);
		insert into player_respawn_locations(id, character_id, "group", map, dimension) values (x'aa', 1, 'g1', 'm', 0), (x'bb', 1, 'g2', 'm', 0);`)}
	keyOf := func(table, where string) string {
		t.Helper()
		res, err := o.TableRows(table, "", 100, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range res.(map[string]any)["rows"].([][]any) {
			if strings.Contains(fmt.Sprint(r[0]), where) {
				return r[0].(string)
			}
		}
		t.Fatalf("no %s row with key containing %q", table, where)
		return ""
	}
	cases := []struct {
		table, col, key string
		val             any
		check           string // counts rows holding the new value: it must be exactly one
	}{
		{"player_travel_state", "login_target_dimension_index", keyOf("player_travel_state", "B"), 9.0, `select count(*) n from player_travel_state where login_target_dimension_index=9 and fls_id='B'`},
		{"player_faction_reputation", "reputation_amount", keyOf("player_faction_reputation", `"1","2"`), 99.0, `select count(*) n from player_faction_reputation where reputation_amount=99 and actor_id=1 and faction_id=2`},
		{"player_respawn_locations", "group", keyOf("player_respawn_locations", "BB"), "edited", `select count(*) n from player_respawn_locations where "group"='edited' and id=x'bb'`},
	}
	for _, c := range cases {
		if _, err := o.UpdateRow(Args{"table": c.table, "column": c.col, "key": c.key, "value": c.val}); err != nil {
			t.Fatalf("%s: %v", c.table, err)
		}
		if n := mustOne(t, o, c.check)["n"].(int64); n != 1 {
			t.Errorf("%s: %d rows hold the new value, want only the named one", c.table, n)
		}
		if n := mustOne(t, o, fmt.Sprintf(`select count(*) n from "%s"`, c.table))["n"].(int64); n < 2 {
			t.Errorf("%s: rows went missing", c.table)
		}
	}
	// the others are untouched
	if n := mustOne(t, o, `select login_target_dimension_index n from player_travel_state where fls_id='A'`)["n"].(int64); n != 1 {
		t.Errorf("row A changed to %d", n)
	}
	if n := mustOne(t, o, `select reputation_amount n from player_faction_reputation where actor_id=1 and faction_id=1`)["n"].(int64); n != 10 {
		t.Errorf("faction row (1,1) changed to %d", n)
	}
	if n := mustOne(t, o, `select reputation_amount n from player_faction_reputation where actor_id=2`)["n"].(int64); n != 30 {
		t.Errorf("faction row (2,2) changed to %d", n)
	}
	if n := mustOne(t, o, `select count(*) n from player_respawn_locations where "group"='g1'`)["n"].(int64); n != 1 {
		t.Errorf("respawn row aa changed")
	}
	assertDBSound(t, o)
}

func TestUpdateRowOnWithoutRowidTableNeedsTheWholeKey(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into player_faction_reputation values (1, 1, 10), (1, 2, 20);`)}
	for _, a := range []Args{
		{"table": "player_faction_reputation", "column": "reputation_amount", "value": 5.0},                                  // no key
		{"table": "player_faction_reputation", "column": "reputation_amount", "key": `["1"]`, "value": 5.0},                  // half the key
		{"table": "player_faction_reputation", "column": "reputation_amount", "key": `["1","1","1"]`, "value": 5.0},          // too much
		{"table": "player_faction_reputation", "column": "reputation_amount", "key": `not json`, "value": 5.0},               // garbage
		{"table": "player_faction_reputation", "column": "reputation_amount", "rowid": 1.0, "value": 5.0},                    // a rowid names nothing here
		{"table": "player_faction_reputation", "column": "actor_id", "key": `["1","1"]`, "value": 7.0},                       // part of the key
		{"table": "player_faction_reputation", "column": "reputation_amount", "key": `["1\" or \"1\"=\"1","1"]`, "value": 5}, // a key is a value, never SQL
	} {
		if _, err := o.UpdateRow(a); err == nil {
			t.Errorf("%v must be refused", a)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused edit must leave no pending change")
	}
	if n := mustOne(t, o, `select count(*) n from player_faction_reputation where reputation_amount=5`)["n"].(int64); n != 0 {
		t.Fatalf("%d rows changed by refused edits", n)
	}
}

// DBA-11: the foreign-key gate keyed WITHOUT ROWID rows per table group, so an edit that fixed one orphan and made a
// different one passed. It now keys them by primary-key values.
func TestFKGateTellsOrphansOfWithoutRowidTablesApart(t *testing.T) {
	// player_tags(character_id -> player_state.id, tag) is WITHOUT ROWID; the save already holds one orphan (900)
	newOps := func() *Ops {
		return &Ops{S: testsave.PlayerWithSQL(t, `insert into player_tags values (1, 'ok'), (900, 'orphan');`)}
	}
	o := newOps()
	// moving the orphan elsewhere leaves the same number of orphans but a different one: refused
	if _, err := o.ExecSQL(`update player_tags set character_id=901 where character_id=900`); err == nil || !strings.Contains(err.Error(), "dangling") || !strings.Contains(err.Error(), "player_tags") {
		t.Fatalf("an edit that creates a different orphan must be refused, got %v", err)
	}
	// repairing the orphan while creating another is refused too
	if _, err := o.ExecSQL(`update player_tags set character_id=1 where character_id=900; insert into player_tags values (902, 'new')`); err == nil {
		t.Fatal("fixing one orphan and creating another must be refused")
	}
	if o.S.Dirty() {
		t.Fatal("a refused edit must leave nothing pending")
	}
	// repairing the orphan alone, and edits that leave it alone, still pass
	if _, err := o.ExecSQL(`update player_tags set character_id=1 where character_id=900`); err != nil {
		t.Fatalf("repairing an orphan must pass: %v", err)
	}
	o = newOps()
	if _, err := o.ExecSQL(`delete from player_tags where tag='ok'`); err != nil {
		t.Fatalf("an edit that leaves the existing orphan alone must pass: %v", err)
	}
	// renaming a primary-key part of the existing orphan names a different row, so it is refused (conservative)
	if _, err := o.ExecSQL(`update player_tags set tag='renamed' where character_id=900`); err == nil {
		t.Fatal("changing the key of an orphan row is a different orphan and is refused")
	}
	if _, err := o.ExecSQL(`insert into player_tags values (1, 'fine')`); err != nil {
		t.Fatalf("a valid row must pass: %v", err)
	}
}

func TestTableRowsKeyRoundTripsEveryKeyShape(t *testing.T) {
	// integer, text, composite and BLOB primary keys all produce a key the editor can send back
	o := &Ops{S: testsave.PlayerWithSQL(t, `
		insert into player_travel_state values ('with "quote" and ''apostrophe''', 3);
		insert into player_respawn_locations(id, character_id, "group", map, dimension) values (x'00ff', 1, 'g', 'm', 0);`)}
	for table, c := range map[string]struct {
		col string
		val any
	}{"player_travel_state": {"login_target_dimension_index", 5.0}, "player_respawn_locations": {"group", "g2"}} {
		res, err := o.TableRows(table, "", 100, 0)
		if err != nil {
			t.Fatal(err)
		}
		row := res.(map[string]any)["rows"].([][]any)[0]
		var vals []string
		if err := json.Unmarshal([]byte(row[0].(string)), &vals); err != nil {
			t.Fatalf("%s: key %v is not a JSON array of strings: %v", table, row[0], err)
		}
		if _, err := o.UpdateRow(Args{"table": table, "column": c.col, "key": row[0], "value": c.val}); err != nil {
			t.Errorf("%s: the key the view gave does not find its row: %v", table, err)
		}
	}
}

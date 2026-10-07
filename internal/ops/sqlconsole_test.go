package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

func newPlayerOps(t *testing.T) *Ops { return &Ops{S: testsave.Player(t)} }

func stackOf(t *testing.T, o *Ops, id int) int64 {
	t.Helper()
	return mustOne(t, o, `select stack_size n from items where id=?`, id)["n"].(int64)
}

// ---- read-only console (SEC-3, ARCH-7)

func TestReadConsoleAllowsOnlySingleSelectOrWith(t *testing.T) {
	o := newPlayerOps(t)
	ok := []string{
		"select 1", "SELECT * FROM items", "  select 1;", "-- comment\nselect 1", "/* c */ select 1",
		"with x as (select 1 a) select a from x", "select 'a;b' as s", `select "x;y" from (select 1 as "x;y")`,
		"select * from pragma_table_info('items')", "select name from sqlite_master",
	}
	for _, q := range ok {
		if _, err := o.SQL(q); err != nil {
			t.Errorf("should be allowed: %q -> %v", q, err)
		}
	}
	bad := []string{
		"pragma query_only=off", "PRAGMA foreign_keys=ON", "pragma table_info(items)",
		"attach database '/etc/hostname' as x", "  ATTACH DATABASE ':memory:' AS y", "detach database x",
		"vacuum", "vacuum into '/root/projects/tabr-tmp/out.db'",
		"insert into items(id) values (1)", "update items set stack_size=1", "delete from items", "replace into items(id) values(1)",
		"create table x(a)", "drop table items", "alter table items add column z", "reindex", "analyze",
		"begin", "commit", "rollback", "savepoint a", "release a",
		"select 1; select 2", "select 1; attach database ':memory:' as z", "select 1;\n-- x\nupdate items set stack_size=1",
		"/* select */ attach database ':memory:' as z", "-- select\nvacuum",
		"", "   ", ";", "select 'unterminated", "select 1 /* unterminated",
		"select load_extension('x')", "with x as (select 1) insert into items(id) select 1 from x",
		"with x as (select 1) delete from items",
	}
	for _, q := range bad {
		if _, err := o.SQL(q); err == nil {
			t.Errorf("must be refused: %q", q)
		}
	}
	if got := stackOf(t, o, 10); got != 100 {
		t.Fatalf("a refused or allowed read must never change data; stack is %d", got)
	}
}

// Even if the statement check were bypassed or wrong, the read connection itself cannot write.
func TestReadConnectionIsReadOnlyAtTheDatabaseLevel(t *testing.T) {
	o := newPlayerOps(t)
	for _, q := range []string{
		"update items set stack_size=999", "delete from items", "insert into items_id_sequencer(next_id) values(1)",
		"pragma query_only=off", "drop table items",
	} {
		_, _, err := o.S.ReadOnlyTable(q, 10)
		if q == "pragma query_only=off" {
			// executing the pragma itself is harmless; what matters is that writes still fail afterwards
			if _, _, err := o.S.ReadOnlyTable("update items set stack_size=999", 10); err == nil {
				t.Fatal("query_only=OFF must not make the read connection writable")
			}
			continue
		}
		if err == nil {
			t.Errorf("read connection executed %q", q)
		}
	}
	if got := stackOf(t, o, 10); got != 100 {
		t.Fatalf("data changed through the read connection: %d", got)
	}
}

func TestReadConsoleSeesCommittedWrites(t *testing.T) {
	o := newPlayerOps(t)
	if _, err := o.ExecSQL("update items set stack_size=77 where id=10"); err != nil {
		t.Fatal(err)
	}
	r, err := o.SQL("select stack_size from items where id=10")
	if err != nil {
		t.Fatal(err)
	}
	rows := r.(map[string]any)["rows"].([][]any)
	if len(rows) != 1 || rows[0][0].(int64) != 77 {
		t.Fatalf("read connection must see the working copy's committed edits, got %v", rows)
	}
}

func TestReadConsoleBoundsRowsAndTime(t *testing.T) {
	o := newPlayerOps(t)
	old := save.ReadTimeout
	save.ReadTimeout = 700 * time.Millisecond
	t.Cleanup(func() { save.ReadTimeout = old })

	start := time.Now()
	r, err := o.SQL("with recursive c(x) as (select 1 union all select x+1 from c) select x from c")
	if err != nil {
		t.Fatalf("an unbounded result must be truncated, not fail: %v", err)
	}
	if n := len(r.(map[string]any)["rows"].([][]any)); n != 1000 {
		t.Fatalf("rows = %d, want the 1000-row cap", n)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("row-cap path took %v", time.Since(start))
	}

	start = time.Now()
	_, err = o.SQL("with recursive c(x) as (select 1 union all select x+1 from c where x<1000000000000) select count(*) from c")
	if err == nil {
		t.Fatal("a runaway query must be stopped")
	}
	if time.Since(start) > 4*time.Second {
		t.Fatalf("timeout not enforced: %v", time.Since(start))
	}
	if _, err := o.SQL("select 1"); err != nil {
		t.Fatalf("the console must keep working after a timeout: %v", err)
	}
}

// ---- write console (ARCH-7)

func TestWriteConsoleAllowlist(t *testing.T) {
	ok := []string{
		"update items set stack_size=101 where id=10",
		"update items set stack_size=102 where id=10; update items set stack_size=103 where id=10;",
		"delete from items where id=11",
		"insert into items(id, inventory_id, stack_size, position_index, template_id, is_new, acquisition_time, stats, quality_level) values (900,1,1,2,'X',0,0,'{}',0)",
		"replace into items_id_sequencer(next_id) values(901)",
		"with c as (select 7 v) update items set stack_size=(select v from c) where id=10",
		"update items set template_id='a;b' where id=10", // a semicolon inside a string literal is not a statement break
		"-- change a stack\nupdate items set stack_size=5 where id=10",
		"with c as (select 11 v) delete from items where id=(select v from c)",
	}
	for _, q := range ok {
		o := newPlayerOps(t)
		if _, err := o.ExecSQL(q); err != nil {
			t.Errorf("should be allowed: %q -> %v", q, err)
		}
	}
	bad := []string{
		"pragma foreign_keys=ON", "pragma foreign_keys=ON; delete from items", "PRAGMA writable_schema=1",
		"attach database ':memory:' as x", "detach x", "vacuum", "vacuum into '/root/projects/tabr-tmp/o.db'",
		"begin", "commit", "rollback", "savepoint s", "release s", "begin; delete from items; commit",
		"create table x(a)", "create trigger tr after delete on items begin delete from items; end", "create view v as select 1",
		"create virtual table v using rtree(id,a,b)", "drop table items", "alter table items add column zz", "reindex", "analyze",
		"select 1", "select * from items", // reads belong in the read-only console
		"/* update */ pragma foreign_keys=ON", "update items set stack_size=1; pragma foreign_keys=ON",
		"update items set stack_size=1; attach database ':memory:' as x", "", " ; ",
		"update items set stack_size=1 where id=10 /* unterminated", "update items set template_id='unterminated",
		"select load_extension('x')", "update sqlite_master set sql=''",
	}
	for _, q := range bad {
		o := newPlayerOps(t)
		before := stackOf(t, o, 10)
		if _, err := o.ExecSQL(q); err == nil {
			t.Errorf("must be refused: %q", q)
		}
		if after := stackOf(t, o, 10); after != before {
			t.Errorf("a refused statement changed data: %q", q)
		}
	}
}

func TestWriteConsoleIsAtomicAcrossStatements(t *testing.T) {
	o := newPlayerOps(t)
	_, err := o.ExecSQL("update items set stack_size=555 where id=10; update no_such_table set a=1")
	if err == nil {
		t.Fatal("second statement is invalid; the script must fail")
	}
	if got := stackOf(t, o, 10); got != 100 {
		t.Fatalf("first statement must be rolled back, stack is %d", got)
	}
	assertDBSound(t, o)
}

func TestWriteConsoleKeepsForeignKeySettingFixed(t *testing.T) {
	o := newPlayerOps(t)
	if _, err := o.ExecSQL("pragma foreign_keys=ON"); err == nil {
		t.Fatal("PRAGMA must be refused so the session's foreign-key mode cannot be flipped")
	}
	r := mustOne(t, o, `pragma foreign_keys`)
	if r["foreign_keys"].(int64) != 0 {
		t.Fatalf("foreign_keys changed: %v", r)
	}
}

// ---- identifiers (SEC-10) and CSV (SEC-9)

func TestTablesNeverBuildsSQLFromSaveControlledNames(t *testing.T) {
	evil := "x\" where 1=0 union select 1 from \"items"
	s := testsave.PlayerWithSQL(t, `create table "`+strings.ReplaceAll(evil, `"`, `""`)+`" (id integer primary key);
		create table "plain_extra" (id integer primary key); insert into plain_extra values (1),(2);`)
	o := &Ops{S: s}
	var errs []string
	save.SetSQLErrorHook(func(q string, err error) { errs = append(errs, err.Error()+" <- "+oneLine(q)) })
	t.Cleanup(func() { save.SetSQLErrorHook(nil) })
	res, err := o.Tables()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	rows := map[string]int64{}
	for _, r := range res.([]map[string]any) {
		n := r["name"].(string)
		names = append(names, n)
		rows[n] = r["rows"].(int64)
	}
	for _, n := range names {
		if n == evil {
			t.Errorf("a table whose name is not a plain identifier must not be listed or queried: %q", n)
		}
	}
	if rows["plain_extra"] != 2 {
		t.Errorf("normal tables must still be listed with correct counts: %v", rows)
	}
	for _, e := range errs {
		if strings.Contains(e, "where 1=0 union") {
			t.Errorf("save-controlled name reached SQL text: %s", e)
		}
	}
}

func TestCSVExportNeutralizesFormulaCells(t *testing.T) {
	o := newPlayerOps(t)
	for _, tc := range []struct{ in, want string }{
		{"=HYPERLINK(\"http://x\")", "'=HYPERLINK"}, {"+1+1", "'+1+1"}, {"-2+3", "'-2+3"}, {"@SUM(A1)", "'@SUM"},
		{"\tTAB", "'\tTAB"}, {"\rCR", "'\rCR"}, {"plain", "plain"}, {"a=b", "a=b"},
	} {
		testsave.Exec(t, o.S, `update items set template_id=? where id=11`, tc.in)
		b, ct, err := o.Export("items", "csv")
		if err != nil || ct != "text/csv" {
			t.Fatalf("export: %v %s", err, ct)
		}
		if !strings.Contains(string(b), tc.want) {
			t.Errorf("cell %q: want %q in CSV, got:\n%s", tc.in, tc.want, b)
		}
		if tc.in != tc.want && strings.Contains(strings.ReplaceAll(string(b), "'"+tc.in, ""), tc.in) && strings.HasPrefix(tc.want, "'") {
			// the raw formula text must only appear after the neutralizing quote
			if strings.Contains(string(b), ","+tc.in+",") || strings.Contains(string(b), "\n"+tc.in+",") {
				t.Errorf("cell %q appears unneutralized", tc.in)
			}
		}
	}
	// numbers are not text and must not be touched: a negative integer stays -5
	testsave.Exec(t, o.S, `update items set acquisition_time=-5 where id=11`)
	b, _, _ := o.Export("items", "csv")
	if !strings.Contains(string(b), ",-5,") {
		t.Errorf("a numeric cell must stay numeric in CSV: %s", b)
	}
}

// ---- single write path (F-05)

func TestSetItemReportsMissingItemAndValidatesBeforeWriting(t *testing.T) {
	o := newPlayerOps(t)
	if _, err := o.SetItem(Args{"id": float64(99999), "stack_size": float64(5)}); err == nil {
		t.Fatal("editing a nonexistent item must be an error, not ok")
	}
	// an out-of-range second field is rejected before anything is written, so the first must not be applied either
	// (transaction rollback itself is covered in internal/save/mutate_test.go)
	before := stackOf(t, o, 10)
	_, err := o.SetItem(Args{"id": float64(10), "stack_size": float64(5), "quality": float64(99)})
	if err == nil {
		t.Fatal("quality 99 is out of range")
	}
	if got := stackOf(t, o, 10); got != before {
		t.Fatalf("a rejected edit changed data: %d -> %d", before, got)
	}
}

func TestDeleteItemMissingIsAnErrorAndLeavesNoPendingEntry(t *testing.T) {
	o := newPlayerOps(t)
	if _, err := o.DeleteItem(Args{"id": float64(99999)}); err == nil {
		t.Fatal("expected not found")
	}
	if o.S.Dirty() || len(o.S.Pending()) != 0 {
		t.Fatal("nothing changed, nothing should be pending")
	}
	if _, err := o.DeleteItem(Args{"id": float64(11)}); err != nil {
		t.Fatal(err)
	}
	if p := o.S.Pending(); len(p) != 1 || !strings.Contains(p[0], "delete item 11") {
		t.Fatalf("pending = %v", p)
	}
}

// ---- review fixes

func TestVetRefusesInternalsAndPragmaFunctions(t *testing.T) {
	o := newPlayerOps(t)
	for _, q := range []string{
		`select "load_extension"('x')`, `select * from 'sqlite_stat1'`, `select * from pragma_database_list`,
		`select * from pragma_writable_schema(1)`, `select * from "pragma_table_info"('items')`,
	} {
		if _, err := o.SQL(q); err == nil {
			t.Errorf("read console must refuse %q", q)
		}
	}
	for _, q := range []string{
		`update items set stack_size=(select 1 from pragma_table_info('items') limit 1) where id=11`,
		`update sqlite_master set sql=''`, `delete from 'sqlite_sequence'`, `update applied_patches set name='x'`,
		`delete from "applied_patches"`,
	} {
		if _, err := o.ExecSQL(q); err == nil {
			t.Errorf("write console must refuse %q", q)
		}
	}
	// LIKE patterns and the catalogue views still work
	for _, q := range []string{`select name from sqlite_master where name not like 'sqlite_%'`, `select * from pragma_table_info('items')`} {
		if _, err := o.SQL(q); err != nil {
			t.Errorf("should be allowed: %q -> %v", q, err)
		}
	}
}

func TestColumnNamesFromTheSaveCannotInjectSQL(t *testing.T) {
	s := testsave.PlayerWithSQL(t, `create table t_inj ("a""=1,""b" text, c text); insert into t_inj values ('x','y'),('p','q');`)
	o := &Ops{S: s}
	if _, err := o.TableRows("t_inj", "x", 50, 0); err != nil {
		t.Fatalf("browsing and searching a table with a hostile column name must work safely: %v", err)
	}
	res, err := o.UpdateRow(Args{"table": "t_inj", "column": `a"=1,"b`, "rowid": float64(1), "value": "z"})
	if err != nil {
		t.Fatal(err)
	}
	_ = res
	r := mustOne(t, o, `select count(*) n from t_inj where c is not null`)
	if r["n"].(int64) != 2 {
		t.Fatalf("a hostile column name changed other columns or rows: %v", r)
	}
	rows, _ := s.Query(`select * from t_inj order by rowid`)
	if rows[1]["a\"=1,\"b"] != "p" {
		t.Fatalf("only the targeted cell may change: %v", rows)
	}
}

func TestUpdateRowRefusesInternalTablesAndMissingRows(t *testing.T) {
	o := newPlayerOps(t)
	for _, tab := range []string{"applied_patches", "sqlite_sequence"} {
		if _, err := o.UpdateRow(Args{"table": tab, "column": "name", "rowid": float64(1), "value": "x"}); err == nil {
			t.Errorf("%s must not be editable here", tab)
		}
	}
	if _, err := o.UpdateRow(Args{"table": "items", "column": "template_id", "rowid": float64(99999), "value": "x"}); err == nil {
		t.Error("editing a missing row must be an error")
	}
}

func TestMissingTargetsAreErrors(t *testing.T) {
	o := newPlayerOps(t)
	if _, err := o.SetPieceHealth(Args{"id": float64(99999), "health": float64(5)}); err == nil {
		t.Error("SetPieceHealth on a missing piece must fail")
	}
	if _, err := o.CompleteTask(Args{"task_id": float64(99999), "faction_id": float64(1)}); err == nil {
		t.Error("CompleteTask on a missing task must fail")
	}
	if _, err := o.SetDecree(Args{"id": float64(99999)}); err == nil {
		t.Error("SetDecree on a missing decree must fail")
	}
	if o.S.Dirty() {
		t.Error("failed edits must leave the save clean")
	}
}

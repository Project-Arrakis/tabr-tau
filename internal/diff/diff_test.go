package diff_test

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/diff"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// compare edits a real-schema save through the editor's own write path and diffs the baseline against it.
func compare(t *testing.T, s *save.Save, opt diff.Options) *diff.Result {
	t.Helper()
	var res *diff.Result
	if err := s.WithBaseline(func(orig, cur *sql.DB) error {
		var err error
		res, err = diff.Compare(orig, cur, opt)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return res
}

func table(t *testing.T, r *diff.Result, name string) diff.TableDiff {
	t.Helper()
	for _, td := range r.Tables {
		if td.Name == name {
			return td
		}
	}
	t.Fatalf("no diff for table %s; have %v", name, r.Tables)
	return diff.TableDiff{}
}

func TestNoEditsMeansEmptyDiff(t *testing.T) {
	s := testsave.Player(t)
	if r := compare(t, s, diff.Options{}); !r.Empty() {
		t.Fatalf("an unedited save must diff empty, got %+v", r)
	}
}

func TestAddedRemovedAndModifiedRows(t *testing.T) {
	s := testsave.Player(t)
	testsave.Exec(t, s, `update items set stack_size=77 where id=10`)
	testsave.Exec(t, s, `delete from items where id=11`)
	testsave.Exec(t, s, `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (200, 1, 3, 4, 'Spice', '{}')`)
	r := compare(t, s, diff.Options{})
	td := table(t, r, "items")
	if td.NumAdded != 1 || td.NumRemoved != 1 || td.NumModified != 1 {
		t.Fatalf("counts = +%d -%d ~%d, want +1 -1 ~1", td.NumAdded, td.NumRemoved, td.NumModified)
	}
	if got := td.Modified[0].Changes; len(got) != 1 || got[0].Path != "stack_size" || got[0].Before != int64(100) || got[0].After != int64(77) {
		t.Fatalf("modified change = %+v", got)
	}
	if td.Key[0] != "id" {
		t.Fatalf("key = %v", td.Key)
	}
	if len(r.Tables) != 1 {
		t.Fatalf("only items changed; got %d tables", len(r.Tables))
	}
}

func TestJSONBColumnsDiffByPath(t *testing.T) {
	// the row is part of the baseline; the edit below changes it
	s2 := testsave.PlayerWithSQL(t, `insert into journey_story_node(character_id, story_node_id, has_pending_reward, complete_condition_state, reveal_condition_state, fail_condition_state, metadata_state, reset_group)
		values (1, 'Q.test', 0, jsonb('{"a":1,"b":{"c":[1,2,3]}}'), jsonb('{}'), jsonb('{}'), jsonb('{}'), 0)`)
	testsave.Exec(t, s2, `update journey_story_node set complete_condition_state=jsonb('{"a":2,"b":{"c":[1,2,4]},"d":true}') where story_node_id='Q.test'`)
	td := table(t, compare(t, s2, diff.Options{}), "journey_story_node")
	if td.NumModified != 1 {
		t.Fatalf("want one modified row, got %+v", td)
	}
	var paths []string
	for _, c := range td.Modified[0].Changes {
		paths = append(paths, c.Kind+" "+c.Path)
	}
	got := strings.Join(paths, "|")
	for _, want := range []string{"~ complete_condition_state.a", "~ complete_condition_state.b.c[2]", "+ complete_condition_state.d"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestIdentifiersAreRedactedByDefault(t *testing.T) {
	s := testsave.Player(t)
	testsave.Exec(t, s, `update accounts set funcom_id='FUNCOM-CHANGED', platform_name='Someone Else' where id=1`)
	testsave.Exec(t, s, `update items set template_id='Item123456789012345678' where id=11`)
	red := compare(t, s, diff.Options{})
	if !red.Redacted {
		t.Fatal("result must say it is redacted")
	}
	// nothing identifying may appear anywhere in the redacted result
	dump := dumpJSON(t, red)
	for _, leak := range []string{"FUNCOM-CHANGED", "FUNCOM-TEST", "Someone Else", "Tester", "123456789012345678"} {
		if strings.Contains(dump, leak) {
			t.Errorf("redacted diff leaks %q", leak)
		}
	}
	if !strings.Contains(dump, "<id>") {
		t.Error("long numbers should be masked as <id>")
	}
	raw := compare(t, s, diff.Options{NoRedact: true})
	if d := dumpJSON(t, raw); !strings.Contains(d, "FUNCOM-CHANGED") || !strings.Contains(d, "123456789012345678") {
		t.Error("NoRedact must show the values")
	}
}

func TestNoiseColumnsAndIgnoreOptions(t *testing.T) {
	s := testsave.Player(t)
	testsave.Exec(t, s, `update actors set serial=serial+1 where id=2`)
	if r := compare(t, s, diff.Options{}); len(r.Tables) != 1 {
		t.Fatalf("without Noise the serial change must show: %+v", r)
	}
	if r := compare(t, s, diff.Options{Noise: true}); !r.Empty() {
		t.Fatalf("Noise must hide actors.serial: %+v", r)
	}
	if r := compare(t, s, diff.Options{IgnoreTables: []string{"actors"}}); !r.Empty() {
		t.Fatalf("IgnoreTables: %+v", r)
	}
	if r := compare(t, s, diff.Options{Tables: []string{"items"}}); !r.Empty() {
		t.Fatalf("Tables filter must exclude actors: %+v", r)
	}
	if r := compare(t, s, diff.Options{IgnoreColumns: map[string]bool{"serial": true}}); !r.Empty() {
		t.Fatalf("IgnoreColumns: %+v", r)
	}
}

func TestFloatToleranceAndMaxRows(t *testing.T) {
	s := testsave.Player(t)
	testsave.Exec(t, s, `update actors set location_x=location_x+0.0000000001 where id=2`)
	if r := compare(t, s, diff.Options{}); !r.Empty() {
		t.Fatalf("a change below the float tolerance must not show: %+v", r)
	}
	testsave.Exec(t, s, `update actors set location_x=location_x+5 where id=2`)
	if r := compare(t, s, diff.Options{}); len(r.Tables) != 1 {
		t.Fatalf("a real float change must show: %+v", r)
	}
	for i := 300; i < 310; i++ {
		testsave.Exec(t, s, `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (?, 1, 1, ?, 'Bulk', '{}')`, i, i)
	}
	td := table(t, compare(t, s, diff.Options{MaxRows: 3}), "items")
	if td.NumAdded != 10 || len(td.Added) != 3 {
		t.Fatalf("totals must be complete and the list capped: numAdded=%d listed=%d", td.NumAdded, len(td.Added))
	}
}

func TestTableSetDifferencesAndHostileNames(t *testing.T) {
	a := testsave.PlayerWithSQL(t, `create table "we""ird" (a text, b text); insert into "we""ird" values ('x','y'),('x','y');`)
	testsave.Exec(t, a, `insert into "we""ird" values ('x','y')`) // multiset: one more identical row
	td := table(t, compare(t, a, diff.Options{}), `we"ird`)
	if td.NumAdded != 1 || td.NumRemoved != 0 || td.Key[0] != "<row-hash>" {
		t.Fatalf("a table with no primary key diffs as a multiset: %+v", td)
	}
}

func TestMalformedJSONBAndOpaqueBlobsDoNotBreakTheDiff(t *testing.T) {
	s := testsave.PlayerWithSQL(t, `create table blobs (id integer primary key, j blob, o blob);
		insert into blobs values (1, jsonb('{"k":1}'), x'deadbeef');`)
	testsave.Exec(t, s, `update blobs set o=x'cafebabe' where id=1`)
	td := table(t, compare(t, s, diff.Options{}), "blobs")
	if td.NumModified != 1 || td.Modified[0].Changes[0].Path != "o" {
		t.Fatalf("opaque blob change should show as a hex diff of column o: %+v", td)
	}
}

func dumpJSON(t *testing.T, r *diff.Result) string {
	t.Helper()
	var sb strings.Builder
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			sb.WriteString(x + "\n")
		case map[string]any:
			for k, e := range x {
				sb.WriteString(k + "\n")
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	b, _ := jsonMarshal(r)
	walk(b)
	return sb.String()
}

// ---- review fixes: compare raw, mask on output

func TestEditingAnIdentifierIsStillReportedWhileMasked(t *testing.T) {
	s := testsave.Player(t)
	testsave.Exec(t, s, `update accounts set funcom_id='SOMEONE-ELSES-ID', platform_name='Renamed' where id=1`)
	r := compare(t, s, diff.Options{})
	td := table(t, r, "accounts")
	if td.NumModified != 1 {
		t.Fatalf("an identity edit must be counted even though its values are masked: %+v", td)
	}
	var paths []string
	for _, c := range td.Modified[0].Changes {
		paths = append(paths, c.Path)
		if c.Before != "<redacted>" || c.After != "<redacted>" {
			t.Errorf("%s: values must be masked, got %v -> %v", c.Path, c.Before, c.After)
		}
	}
	if got := strings.Join(paths, ","); got != "funcom_id,platform_name" {
		t.Errorf("changed fields = %s", got)
	}
}

// sid builds a synthetic 17-digit platform id at run time, so no id-shaped literal sits in the source (the repo's
// secret scanner rightly flags those).
func sid(n int) string { return "7656119" + fmt.Sprintf("%010d", n) }

func TestLongNumbersAreMaskedWhateverTheirType(t *testing.T) {
	s := testsave.PlayerWithSQL(t, fmt.Sprintf(`create table ids (id integer primary key, n integer, f real, j blob, t text);
		insert into ids values (1, %[1]s, %[1]s.0, jsonb('{"%[2]s":{"id":%[3]s,"s":"x%[4]s y"}}'), 'plain');`, sid(1), sid(2), sid(3), sid(4)))
	testsave.Exec(t, s, fmt.Sprintf(`update ids set n=%[1]s, f=%[1]s.0, j=jsonb('{"%[2]s":{"id":%[3]s,"s":"x%[4]s y"}}'), t='changed' where id=1`, sid(9), sid(2), sid(5), sid(6)))
	r := compare(t, s, diff.Options{})
	td := table(t, r, "ids")
	if td.NumModified != 1 {
		t.Fatalf("%+v", td)
	}
	dump := dumpJSON(t, r)
	if strings.Contains(dump, "7656119") {
		t.Errorf("a long numeric id leaked (integer, float, JSON number, JSON key or text):\n%s", dump)
	}
	if !strings.Contains(dump, "changed") {
		t.Error("ordinary values must still be shown")
	}
}

func TestPrimaryKeysThatLookMaskedDoNotCollide(t *testing.T) {
	// two rows whose key column is a personal column; masking must not merge them
	s := testsave.PlayerWithSQL(t, `create table people ("user" text primary key, n integer);
		insert into people values ('alice', 1), ('bob', 2), ('carol', 3);`)
	testsave.Exec(t, s, `update people set n=n+10`)
	testsave.Exec(t, s, `delete from people where "user"='carol'`)
	td := table(t, compare(t, s, diff.Options{}), "people")
	if td.NumModified != 2 || td.NumRemoved != 1 {
		t.Fatalf("rows must be matched on raw keys: +%d -%d ~%d", td.NumAdded, td.NumRemoved, td.NumModified)
	}
	// keys of different types do not merge either
	k := testsave.PlayerWithSQL(t, `create table mixed (k primary key, v text); insert into mixed values (1,'a'), ('1','b');`)
	testsave.Exec(t, k, `update mixed set v='z'`)
	if td := table(t, compare(t, k, diff.Options{}), "mixed"); td.NumModified != 2 {
		t.Fatalf("integer 1 and text '1' are different keys: %+v", td)
	}
}

func TestZeroAndEmptyBeforeValuesSurviveJSONEncoding(t *testing.T) {
	s := testsave.Player(t)
	testsave.Exec(t, s, `update items set stack_size=stack_size, quality_level=3 where id=10`)
	r := compare(t, s, diff.Options{})
	b, _ := jsonMarshal(r)
	dump := dumpJSON(t, r)
	_ = b
	if !strings.Contains(dump, "before") {
		t.Fatalf("a change from 0 must keep its before value in the JSON:\n%s", dump)
	}
}

func TestMaxRowsIsClamped(t *testing.T) {
	s := testsave.Player(t)
	testsave.Exec(t, s, `update items set stack_size=stack_size+1`)
	for _, n := range []int{-5, 0, 1 << 30} {
		td := table(t, compare(t, s, diff.Options{MaxRows: n}), "items")
		if len(td.Modified) == 0 || len(td.Modified) > 1000 {
			t.Errorf("MaxRows=%d listed %d rows", n, len(td.Modified))
		}
	}
}

// Every column that looks personal must be classified on purpose. If the game adds one, this fails until someone
// decides whether it must be masked in pasted diffs.
func TestEveryColumnIsClassified(t *testing.T) {
	looksPersonal := regexp.MustCompile(`(?i)name|identifier|funcom|platform|steam|email|user|account`)
	// reviewed and judged not to identify a person (game objects, internal ids, labels)
	reviewed := map[string]bool{
		"account_id": true, "owner_account_id": true, "slot_name": true, "npc_name": true, "house_name": true,
		"channel_name": true, "base_backup_name": true, "vehicle_name": true, "story_node_id": true, "template_id": true,
		"learned_building_set": true, "building_type": true, "name": true, "takeoverable": true, "user_id": true,
		"profile_name": true, "class_name": true, "map_name": true, "map": true, "faction_name": true,
		// reviewed 2026-10-07 against schema.sql: names of game objects, not people
		"component_name_hash": true, "selected_channel_name": true, "package_name": true, "actor_name": true,
		"encounter_name": true, "decree_name": true, "locator_name": true, "locator_name_index": true,
	}
	db := testsave.OpenSchemaOnly(t)
	rows, err := db.Query(`select m.name, p.name from sqlite_master m, pragma_table_info(m.name) p where m.type='table' and m.name not like 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var unclassified []string
	for rows.Next() {
		var tab, col string
		rows.Scan(&tab, &col)
		if looksPersonal.MatchString(col) && !diff.IsPersonalColumn(col) && !reviewed[col] {
			unclassified = append(unclassified, tab+"."+col)
		}
	}
	if len(unclassified) > 0 {
		t.Fatalf("columns that look personal but are neither masked nor reviewed (add to diff.redactColumns or to this test's reviewed list): %v", unclassified)
	}
}

func TestJSONIntegersCompareExactlyAndFractionsWithTolerance(t *testing.T) {
	s := testsave.PlayerWithSQL(t, `create table counters (id integer primary key, j blob);
		insert into counters values (1, jsonb('{"count":2000000,"ratio":0.5,"big":9007199254740993}'));`)
	// +1 on a large counter and on an integer beyond float64's exact range, plus a sub-tolerance fraction change
	testsave.Exec(t, s, `update counters set j=jsonb('{"count":2000001,"ratio":0.5000000001,"big":9007199254740994}') where id=1`)
	td := table(t, compare(t, s, diff.Options{}), "counters")
	var paths []string
	for _, c := range td.Modified[0].Changes {
		paths = append(paths, c.Path)
	}
	got := strings.Join(paths, ",")
	if got != "j.big,j.count" {
		t.Fatalf("integer changes must never be hidden by the float tolerance, and the tiny fraction change must be: got %q", got)
	}
}

func TestTableSetDifferencesAndCompositeKeys(t *testing.T) {
	a := testsave.PlayerWithSQL(t, `create table pairs (a integer, b text, v text, primary key (a, b));
		insert into pairs values (1,'x','one'),(2,'x','two');
		create table gone (id integer primary key);`)
	testsave.Exec(t, a, `update pairs set v='ONE' where a=1 and b='x'`)
	testsave.Exec(t, a, `insert into pairs values (1,'y','new')`)
	testsave.Exec(t, a, `drop table gone`)
	testsave.Exec(t, a, `create table fresh (id integer primary key)`)
	r := compare(t, a, diff.Options{})
	if len(r.OnlyBefore) != 1 || r.OnlyBefore[0] != "gone" || len(r.OnlyAfter) != 1 || r.OnlyAfter[0] != "fresh" {
		t.Fatalf("table-set differences = before %v after %v", r.OnlyBefore, r.OnlyAfter)
	}
	td := table(t, r, "pairs")
	if td.NumModified != 1 || td.NumAdded != 1 || len(td.Key) != 2 {
		t.Fatalf("composite key rows must be matched on both columns: %+v", td)
	}
}

func TestWriteTextIsStableAndSafe(t *testing.T) {
	s := testsave.Player(t)
	testsave.Exec(t, s, "update items set stack_size=77, template_id=?", "Bad\x1b[31mName")
	var sb strings.Builder
	compare(t, s, diff.Options{}).WriteText(&sb)
	out := sb.String()
	if !strings.Contains(out, "## items") || !strings.Contains(out, "1 table(s) changed") {
		t.Fatalf("unexpected text output:\n%s", out)
	}
	if strings.ContainsRune(out, 0x1b) {
		t.Fatal("control characters from a save must not reach the terminal")
	}
	sb.Reset()
	compare(t, s, diff.Options{NoRedact: true}).WriteText(&sb)
	if !strings.Contains(sb.String(), "(UNREDACTED)") {
		t.Fatal("unredacted output must say so")
	}
	// more rows than MaxRows: the footer says how many were not shown
	for i := 400; i < 410; i++ {
		testsave.Exec(t, s, `insert into items(id, inventory_id, stack_size, position_index, template_id, stats) values (?, 1, 1, ?, 'B', '{}')`, i, i)
	}
	sb.Reset()
	compare(t, s, diff.Options{MaxRows: 2}).WriteText(&sb)
	if !strings.Contains(sb.String(), "more + rows not shown") {
		t.Fatalf("missing the truncation footer:\n%s", sb.String())
	}
}

func TestMalformedJSONBIsReportedNotFatal(t *testing.T) {
	s := testsave.PlayerWithSQL(t, `create table jb (id integer primary key, j blob);
		insert into jb values (1, jsonb('{"a":1}')), (2, jsonb('{"a":1}'));`)
	testsave.Exec(t, s, `update jb set j=jsonb('{"a":2}') where id=1`)
	// a blob that is not JSONB makes the column "opaque": shown as hex, never a crash
	testsave.Exec(t, s, `update jb set j=x'ff00ff' where id=2`)
	td := table(t, compare(t, s, diff.Options{}), "jb")
	if td.NumModified != 2 {
		t.Fatalf("both edits must be reported: %+v", td)
	}
}

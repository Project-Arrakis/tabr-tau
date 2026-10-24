package save

import (
	"strconv"
	"strings"
	"testing"
)

// A save with one actor, one entity (with and without components) and two items, so every JSON column the edit check covers holds a value.
const jsonDDL = `
insert into actors(id, class, properties, gas_attributes) values (1, 'BP_Pawn', jsonb('{"a":{"b":1}}'), jsonb('{"hp":10}'));
insert into fgl_entities(entity_id, components) values (7, jsonb('{"FVehicleComponent":[1,{"CurrentFuel":5}]}')), (8, NULL);
insert into items(id, stack_size, position_index, template_id, stats) values (1, 1, 0, 'Sword', '{"d":{"x":1}}'), (2, 1, 1, 'Shield', '{"d":{"x":2}}');
`

func jsonSave(t *testing.T) *Save {
	t.Helper()
	s, err := Open(makeSave(t, jsonDDL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// What an edit may do to a JSON document: the ordinary edits go through and are counted as before.
func TestAnOrdinaryJSONEditIsAccepted(t *testing.T) {
	s := jsonSave(t)
	n, err := s.Exec(`update items set stats=json_set(stats,'$.d.x',99) where id=1`)
	if err != nil || n != 1 {
		t.Fatalf("a text json_set on items.stats: n=%d err=%v", n, err)
	}
	if n, err = s.Exec(`update actors set properties=jsonb_set(properties,'$.a.b',2) where id=1`); err != nil || n != 1 {
		t.Fatalf("a jsonb_set on actors.properties: n=%d err=%v", n, err)
	}
	if n, err = s.Exec(`update fgl_entities set components=jsonb_set(components,'$.FVehicleComponent[1].CurrentFuel',9) where entity_id=7`); err != nil || n != 1 {
		t.Fatalf("a jsonb_set on fgl_entities.components: n=%d err=%v", n, err)
	}
	if n, err = s.Exec(`insert into items(id, stack_size, position_index, template_id, stats) values (3, 1, 2, 'Bow', '{"d":{"x":3}}')`); err != nil || n != 1 {
		t.Fatalf("a new item with valid stats: n=%d err=%v", n, err)
	}
	if n, err = s.Exec(`update fgl_entities set components=NULL where entity_id=7`); err != nil || n != 1 {
		t.Fatalf("an empty components cell is not a JSON problem: n=%d err=%v", n, err)
	}
	r, _ := s.One(`select json_extract(stats,'$.d.x') x from items where id=1`)
	if r["x"].(int64) != 99 || !s.Dirty() {
		t.Fatalf("the edit stands: %v", r)
	}
}

// Every way an edit can leave a stored document unusable refuses the whole edit and changes nothing.
func TestAnEditThatLeavesBadJSONIsRolledBack(t *testing.T) {
	big := `replace(hex(zeroblob(600000)),'0','a')` // 1,200,000 characters
	cases := []struct {
		name, sql, want string
	}{
		{"text that is not JSON", `update items set stats='{broken' where id=1`, "items.stats #1: not valid JSON"},
		{"an insert with bad stats", `insert into items(id, stack_size, position_index, template_id, stats) values (9, 1, 5, 'Bad', '{"d":')`, "items.stats #9: not valid JSON"},
		{"JSON text stored as bytes in a jsonb column", `update actors set properties=cast('{"a":1}' as blob) where id=1`, "actors.properties #1: stored as text, not as binary JSON"},
		{"JSON text stored as bytes in the components column", `update fgl_entities set components=cast('{"a":1}' as blob) where entity_id=7`, "fgl_entities.components #7: stored as text, not as binary JSON"},
		{"garbage bytes in a jsonb column", `update actors set gas_attributes=x'00010203' where id=1`, "actors.gas_attributes #1: not valid JSON"},
		{"a document over 1 MiB", `update items set stats=json_set(stats,'$.big',` + big + `) where id=1`, "items.stats #1: larger than 1 MiB"},
		{"a jsonb document over 1 MiB", `update actors set properties=jsonb_set(properties,'$.big',` + big + `) where id=1`, "actors.properties #1: larger than 1 MiB"},
	}
	for _, c := range cases {
		s := jsonSave(t)
		before, _ := s.One(`select (select stats from items where id=1) s1, (select hex(properties) from actors where id=1) p1`)
		n, err := s.Exec(c.sql)
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "nothing was changed") {
			t.Errorf("%s: want an error naming %q, got n=%d err=%v", c.name, c.want, n, err)
			continue
		}
		after, _ := s.One(`select (select stats from items where id=1) s1, (select hex(properties) from actors where id=1) p1`)
		if before["s1"] != after["s1"] || before["p1"] != after["p1"] || s.Dirty() || len(s.Pending()) != 0 {
			t.Errorf("%s: the refused edit changed something (dirty=%v pending=%v)", c.name, s.Dirty(), s.Pending())
		}
	}
}

// json_set on a jsonb column returns text, which the STRICT table already refuses (the statement fails before the check runs); the check does not weaken that.
func TestTextWhereBinaryJSONIsExpectedIsStillRefused(t *testing.T) {
	for _, q := range []string{
		`update actors set properties=json_set(properties,'$.a.b',2) where id=1`,
		`update fgl_entities set components='{"a":1}' where entity_id=7`,
		`update items set stats=jsonb('{"a":1}') where id=2`,
	} {
		s := jsonSave(t)
		if _, err := s.Exec(q); err == nil || s.Dirty() {
			t.Errorf("%s: want a refusal and no change, got err=%v dirty=%v", q, err, s.Dirty())
		}
	}
}

// The edit is one transaction: a good change followed by a bad one takes the good one back too.
func TestABadJSONStepTakesTheWholeEditBack(t *testing.T) {
	s := jsonSave(t)
	_, err := s.Mutate("two steps", func(m *Mut) error {
		if _, err := m.Exec(`update items set stats=json_set(stats,'$.d.x',50) where id=1`); err != nil {
			return err
		}
		_, err := m.Exec(`update items set stats='nope' where id=2`)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "items.stats #2: not valid JSON") {
		t.Fatalf("the bad step must refuse the edit: %v", err)
	}
	r, _ := s.One(`select json_extract(stats,'$.d.x') x from items where id=1`)
	if r["x"].(int64) != 1 || s.Dirty() {
		t.Fatalf("the good step must be taken back: %v dirty=%v", r, s.Dirty())
	}
}

// The cap is per cell and exact: 1 MiB is accepted, one character more is not.
func TestTheCellSizeCapIsExact(t *testing.T) {
	s := jsonSave(t)
	if MaxJSONCell != 1<<20 {
		t.Fatalf("the cap is %d, want 1 MiB", MaxJSONCell)
	}
	pad := func(n int) string {
		return `'"' || substr(replace(hex(zeroblob(` + "700000" + `)),'0','a'),1,` + strconv.Itoa(n-2) + `) || '"'`
	}
	if _, err := s.Exec(`update items set stats=` + pad(MaxJSONCell) + ` where id=1`); err != nil {
		t.Fatalf("a document of exactly 1 MiB is accepted: %v", err)
	}
	if _, err := s.Exec(`update items set stats=` + pad(MaxJSONCell+1) + ` where id=2`); err == nil || !strings.Contains(err.Error(), "larger than 1 MiB") {
		t.Fatalf("one character over is refused: %v", err)
	}
}

// A document the game wrote that is already bad (or already over the cap) is not this edit's doing: an edit of another cell goes through. Only a cell the edit itself
// writes is judged.
func TestAnExistingBadCellDoesNotBlockAnEditOfAnotherCell(t *testing.T) {
	s, err := Open(makeSave(t, jsonDDL+`insert into items(id, stack_size, position_index, template_id, stats) values (5, 1, 9, 'Old', 'not json at all');`))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.Exec(`update items set stats=json_set(stats,'$.d.x',7) where id=1`); err != nil || n != 1 {
		t.Fatalf("an edit of a good cell next to a bad one: n=%d err=%v", n, err)
	}
	if _, err := s.Exec(`update items set stack_size=3 where id=5`); err != nil {
		t.Fatalf("an edit of another column of the bad row does not write the document: %v", err)
	}
	if _, err := s.Exec(`update items set stats='still not json' where id=5`); err == nil {
		t.Fatal("writing a bad document is refused even over one that was already bad")
	}
}

// The rows changed are counted as before: the check must not add to the number of rows an edit reports.
func TestTheJSONCheckDoesNotChangeTheRowCount(t *testing.T) {
	s := jsonSave(t)
	n, err := s.Exec(`update items set stats=json_set(stats,'$.d.y',1)`) // two rows
	if err != nil || n != 2 {
		t.Fatalf("two rows were changed: n=%d err=%v", n, err)
	}
	if ops := s.Pending(); len(ops) != 1 {
		t.Fatalf("one edit, one pending entry: %v", ops)
	}
}

// Nothing of the check is left on the connection after an edit, whether it was accepted or refused.
func TestTheJSONCheckLeavesNothingBehind(t *testing.T) {
	s := jsonSave(t)
	s.Exec(`update items set stats='{"ok":1}' where id=1`)
	s.Exec(`update items set stats='bad' where id=1`)
	r, err := s.One(`select (select count(*) from sqlite_temp_master) c`)
	if err != nil || r["c"].(int64) != 0 {
		t.Fatalf("temporary objects left on the connection: %v %v", r, err)
	}
}

// nest is a JSON text of n arrays inside each other.
func nest(n int) string { return strings.Repeat("[", n) + strings.Repeat("]", n) }

// The depth cap is exact: 32 levels are accepted, 33 are refused, in a text column and in a jsonb column.
func TestTheDepthCapIsExact(t *testing.T) {
	s := jsonSave(t)
	if MaxJSONDepth != 32 {
		t.Fatalf("the cap is %d, want 32", MaxJSONDepth)
	}
	if _, err := s.Exec(`update items set stats='{"d":` + nest(MaxJSONDepth-1) + `}' where id=1`); err != nil {
		t.Fatalf("32 levels are accepted: %v", err)
	}
	if _, err := s.Exec(`update actors set properties=jsonb('{"d":` + nest(MaxJSONDepth-1) + `}') where id=1`); err != nil {
		t.Fatalf("32 levels are accepted in a jsonb column: %v", err)
	}
	for _, q := range []string{
		`update items set stats='{"d":` + nest(MaxJSONDepth) + `}' where id=2`,
		`update actors set gas_attributes=jsonb('{"d":` + nest(MaxJSONDepth) + `}') where id=1`,
		`update fgl_entities set components=jsonb('{"d":` + nest(MaxJSONDepth) + `}') where entity_id=7`,
	} {
		_, err := s.Exec(q)
		if err == nil || !strings.Contains(err.Error(), "nested more than 32 levels deep") || !strings.Contains(err.Error(), "nothing was changed") {
			t.Errorf("33 levels must be refused: %v", err)
		}
	}
	r, _ := s.One(`select stats from items where id=2`)
	if r["stats"].(string) != `{"d":{"x":2}}` {
		t.Fatalf("the refused edit left a change: %v", r)
	}
}

// jsonDepth counts brackets outside strings only.
func TestJSONDepthIgnoresBracketsInStrings(t *testing.T) {
	for doc, want := range map[string]int{
		`1`:                           0,
		`{}`:                          1,
		`{"a":"[[[[","b":[{"c":[]}]}`: 4,
		`{"a":"\"[[[","b":["]]]"]}`:   2,
		`{"a":"x\\","b":[[1]]}`:       3,
	} {
		if got := jsonDepth(doc); got != want {
			t.Errorf("%s: depth %d, want %d", doc, got, want)
		}
	}
}

// The notes the check writes are not changes of the edit: the count an edit reports to its own function is the same with and without a JSON cell in it.
func TestAnEditsOwnChangeCountLeavesOutTheCheckNotes(t *testing.T) {
	s := jsonSave(t)
	_, err := s.Mutate("count", func(m *Mut) error {
		if _, err := m.Exec(`update items set stats=json_set(stats,'$.d.x',5)`); err != nil {
			return err
		}
		if n, err := m.changes(); err != nil || n != 2 {
			t.Errorf("two rows were changed: %d err=%v", n, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

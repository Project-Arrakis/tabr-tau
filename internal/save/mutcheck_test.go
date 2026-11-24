package save

import (
	"errors"
	"strings"
	"testing"
)

// The checks a preview runs inside its edit before it rolls back (#1176): they say what the checks at the end of a write would say, they change nothing, and an edit that goes
// on after them is judged at its end as it would have been without them.
func TestCheckReferencesAndCheckJSONSayWhatTheEndOfAnEditWouldSay(t *testing.T) {
	s, err := Open(makeSave(t, jsonDDL+`insert into inventories(id, actor_id, inventory_type) values (5, 1, 0); update items set inventory_id=5;`))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	dry := errors.New("dry run")

	// an edit that is fine: both checks pass, before and after it wrote something
	_, err = s.Mutate("fine", func(m *Mut) error {
		if err := m.CheckReferences(); err != nil {
			t.Errorf("references before any write: %v", err)
		}
		if err := m.CheckJSON(); err != nil {
			t.Errorf("JSON before any write: %v", err)
		}
		if _, err := m.Exec(`update items set stats=json_set(stats,'$.d.x',5) where id=1`); err != nil {
			return err
		}
		if err := m.CheckReferences(); err != nil {
			t.Errorf("references after a good write: %v", err)
		}
		if err := m.CheckJSON(); err != nil {
			t.Errorf("JSON after a good write: %v", err)
		}
		return dry
	})
	if !errors.Is(err, dry) {
		t.Fatalf("the dry run: %v", err)
	}

	// an edit that leaves a dangling reference: said inside the edit, in the words of the check at the end
	_, err = s.Mutate("dangling", func(m *Mut) error {
		if _, err := m.Exec(`delete from inventories where id=5`); err != nil {
			return err
		}
		err := m.CheckReferences()
		if err == nil || !strings.Contains(err.Error(), "would leave dangling references") || !strings.Contains(err.Error(), "items") {
			t.Errorf("a dangling reference inside the edit: %v", err)
		}
		if err := m.CheckJSON(); err != nil {
			t.Errorf("no JSON cell was written: %v", err)
		}
		return dry
	})
	if !errors.Is(err, dry) {
		t.Fatalf("the dry run: %v", err)
	}

	// an edit that writes a JSON cell the game may not read: said inside the edit; and the check at the end still refuses the same edit when it goes on
	for _, goOn := range []bool{false, true} {
		_, err = s.Mutate("bad json", func(m *Mut) error {
			if _, err := m.Exec(`update items set stats='{not json' where id=2`); err != nil {
				return err
			}
			err := m.CheckJSON()
			if err == nil || !strings.Contains(err.Error(), "stored JSON the game may not be able to read") {
				t.Errorf("a bad JSON cell inside the edit: %v", err)
			}
			if err := m.CheckReferences(); err != nil {
				t.Errorf("no reference was broken: %v", err)
			}
			if goOn {
				return nil
			}
			return dry
		})
		if goOn && (err == nil || !strings.Contains(err.Error(), "stored JSON the game may not be able to read")) {
			t.Fatalf("the check at the end, after CheckJSON was called: %v", err)
		}
	}

	// nothing of any of it is in the save, and an ordinary edit still goes through
	r, err := s.One(`select (select count(*) from inventories) inv, (select stats from items where id=1) s1, (select stats from items where id=2) s2`)
	if err != nil || r["inv"] != int64(1) || r["s1"] != `{"d":{"x":1}}` || r["s2"] != `{"d":{"x":2}}` || len(s.Ops()) != 0 {
		t.Fatalf("the save changed: %v %v %v", r, err, s.Ops())
	}
	if n, err := s.Exec(`update items set stats=json_set(stats,'$.d.x',7) where id=1`); err != nil || n != 1 {
		t.Fatalf("an ordinary edit afterwards: n=%d err=%v", n, err)
	}
}

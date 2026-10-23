package ops

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Deleting a bag takes the inventory it owns and everything in it, because the edit runs with the schema's foreign keys on. The queued edit is kept in the journal and put back
// when the editor starts again or the game saved since, and it must run the same way then: with the cascade, so it changes the same rows. Without it the replay would delete
// one row, find five were deleted before, and drop the edit as a conflict (or, worse, leave the bag's contents behind).

func containerGone(t *testing.T, o *Ops) {
	t.Helper()
	if n := countWhere(t, o, `select count(*) n from items where id in (20,21,22)`); n != 0 {
		t.Errorf("the bag and what is nested in it must be gone, %d items remain", n)
	}
	if n := countWhere(t, o, `select count(*) n from inventories where id in (30,31)`); n != 0 {
		t.Errorf("the bag's inventories must be gone, %d remain", n)
	}
	if n := countWhere(t, o, `select count(*) n from items where id in (10,11,23)`); n != 3 {
		t.Errorf("unrelated items must survive, %d of 3 remain", n)
	}
	if v := fkViolations(t, o); v != 0 {
		t.Errorf("%d dangling references", v)
	}
	if r := mustOne(t, o, `pragma foreign_keys`); r["foreign_keys"].(int64) != 0 {
		t.Error("foreign_keys must be switched back off after the cascading edit")
	}
}

func TestACascadingDeleteIsPutBackWhenTheEditorStartsAgain(t *testing.T) {
	s1 := testsave.PlayerWithSQL(t, containerSQL)
	jp := filepath.Join(t.TempDir(), "pending.json")
	s1.EnableJournal(jp)
	if _, err := (&Ops{S: s1}).DeleteItem(Args{"id": float64(20)}); err != nil {
		t.Fatal(err)
	}
	pending := s1.Pending()
	if len(pending) != 1 || !strings.Contains(pending[0], "Bag") {
		t.Fatalf("the delete is pending: %v", pending)
	}
	s1.Close()

	s2, err := save.Open(s1.Path) // the editor is opened again: the file still holds the bag
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s2.Close)
	s2.EnableJournal(jp)
	o := &Ops{S: s2}
	if n := countWhere(t, o, `select count(*) n from items where id in (20,21,22)`); n != 3 {
		t.Fatalf("the file still holds the bag and what is in it: %d", n)
	}
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("the delete is put back whole: %+v %v", r, err)
	}
	containerGone(t, o)
	if got := s2.Pending(); len(got) != 1 || got[0] != pending[0] {
		t.Fatalf("the same edit is pending again: %v", got)
	}
	assertDBSound(t, o)
}

func TestACascadingDeleteIsPutBackOntoAFileTheGameSavedSince(t *testing.T) {
	s := testsave.PlayerWithSQL(t, containerSQL)
	s.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	o := &Ops{S: s}
	if _, err := o.DeleteItem(Args{"id": float64(20)}); err != nil {
		t.Fatal(err)
	}
	r, err := s.Rebase() // reads the file again and puts the pending edit back onto it
	if err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 || !r.Rebased {
		t.Fatalf("rebase: %+v %v", r, err)
	}
	containerGone(t, o)
	if !s.Dirty() {
		t.Fatal("the delete is pending again")
	}
}

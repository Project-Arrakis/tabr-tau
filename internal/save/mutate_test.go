package save_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

func count(t *testing.T, s *save.Save) int64 {
	t.Helper()
	r, err := s.One(`select count(*) c from items`)
	if err != nil {
		t.Fatal(err)
	}
	return r["c"].(int64)
}

func TestMutateRollsBackEverythingOnError(t *testing.T) {
	s := testsave.Player(t)
	before := count(t, s)
	_, err := s.Mutate("two steps", func(m *save.Mut) error {
		if _, err := m.Exec(`delete from items where id=11`); err != nil {
			return err
		}
		return errors.New("second step failed")
	})
	if err == nil {
		t.Fatal("error must propagate")
	}
	if count(t, s) != before || s.Dirty() || len(s.Pending()) != 0 {
		t.Fatalf("failed edit must leave no trace: items=%d dirty=%v pending=%v", count(t, s), s.Dirty(), s.Pending())
	}
}

func TestMutateRecordsDescriptionOnlyWhenRowsChanged(t *testing.T) {
	s := testsave.Player(t)
	if n, err := s.Mutate("noop", func(m *save.Mut) error { _, err := m.Exec(`update items set stack_size=1 where id=99999`); return err }); err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if s.Dirty() || len(s.Pending()) != 0 {
		t.Fatal("an edit that changed nothing must not dirty the save or add a pending entry")
	}
	n, err := s.Mutate("initial", func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=7 where id=11`)
		m.Desc = "updated knife stack"
		return err
	})
	if err != nil || n != 1 || !s.Dirty() {
		t.Fatalf("n=%d err=%v dirty=%v", n, err, s.Dirty())
	}
	if p := s.Pending(); len(p) != 1 || p[0] != "updated knife stack" {
		t.Fatalf("pending = %v", p)
	}
}

func TestMutateReadsSeeTheEditsOwnWrites(t *testing.T) {
	s := testsave.Player(t)
	_, err := s.Mutate("x", func(m *save.Mut) error {
		if _, err := m.Exec(`update items set stack_size=42 where id=11`); err != nil {
			return err
		}
		r, err := m.One(`select stack_size n from items where id=11`)
		if err != nil || r["n"].(int64) != 42 {
			return errors.New("the edit cannot see its own write")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMutateRefusedOnReadOnlySave(t *testing.T) {
	s := testsave.PlayerWithSQL(t, `create view v as select 1;`)
	ran := false
	_, err := s.Mutate("x", func(m *save.Mut) error { ran = true; return nil })
	if err == nil || ran || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("err=%v ran=%v", err, ran)
	}
}

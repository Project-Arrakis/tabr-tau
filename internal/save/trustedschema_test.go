package save_test

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A save is a file from outside: triggers, views and generated columns in it must not be allowed to call SQLite functions
// that have side effects or reveal anything, whatever the file says. SQLite's trusted_schema switch decides this and it
// defaults to ON, so the editor turns it OFF on the connections that open the save (register 2026-09-30, F-03).
func TestTheSavesSchemaIsNotTrusted(t *testing.T) {
	s := testsave.Player(t)
	rows, err := s.Query(`pragma trusted_schema`)
	if err != nil || len(rows) != 1 {
		t.Fatalf("reading the switch on the read connection: %v %v", rows, err)
	}
	for _, v := range rows[0] {
		if v != int64(0) {
			t.Errorf("the read connection trusts the schema: %v", v)
		}
	}
	if _, err := s.Mutate("probe", func(m *save.Mut) error {
		r, err := m.One(`pragma trusted_schema`)
		if err != nil {
			return err
		}
		for _, v := range r {
			if v != int64(0) {
				t.Errorf("the write connection trusts the schema: %v", v)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

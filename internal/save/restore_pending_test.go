package save

import "testing"

// Restoring a backup reloads the file, which would drop pending edits; it refuses instead.
func TestRestoreRefusesWithPendingEdits(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	s.Exec(`update t set a=2`)
	if _, err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	name := s.Backups()[0]["name"].(string)
	s.Exec(`update t set a=3`)
	if err := s.Restore(name); err == nil {
		t.Fatal("restore must refuse while there are unsaved edits")
	}
	if !s.Dirty() {
		t.Fatal("the pending edit must survive the refusal")
	}
}

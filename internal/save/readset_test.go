package save

import (
	"os"
	"strings"
	"testing"
)

// addN is an edit that works its value out from the row it reads, as "Solari +250" does.
func addN(t *testing.T, s *Save, a int, delta int64) {
	t.Helper()
	if _, err := s.Mutate("add to n", func(m *Mut) error {
		r, err := m.One(`select n from jt where a=?`, a)
		if err != nil || r == nil {
			return err
		}
		_, err = m.Exec(`update jt set n=? where a=?`, r["n"].(int64)+delta, a)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func nOf(t *testing.T, s *Save, a int) int64 {
	t.Helper()
	r, err := s.One(`select n from jt where a=?`, a)
	if err != nil || r == nil {
		t.Fatalf("row %d: %v %v", a, r, err)
	}
	return r["n"].(int64)
}

func putNewerFile(t *testing.T, p, ddl string) {
	t.Helper()
	b, err := os.ReadFile(makeSave(t, ddl))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The game earned something while the edit waited: the edit is not applied over the new value, and the reason says so.
func TestAnEditWorkedOutFromARowTheGameChangedIsNotPutBackOverIt(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	addN(t, s1, 1, 250) // 10 + 250 = 260 stored as an absolute value
	s1.Close()
	putNewerFile(t, p, journalDDL+`update jt set n=5100 where a=1;`) // the game saved 5,100 meanwhile
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 0 || len(r.Conflicts) != 1 || r.Conflicts[0].Desc != "add to n" {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if !strings.Contains(r.Conflicts[0].Why, "game changed") {
		t.Fatalf("the reason should say the game changed what it was worked out from: %q", r.Conflicts[0].Why)
	}
	if got := nOf(t, s2, 1); got != 5100 {
		t.Fatalf("the game's value must stay: %d", got)
	}
	if len(s2.Pending()) != 0 {
		t.Fatalf("nothing applied, nothing pending: %v", s2.Pending())
	}
}

// A change to a row the edit never looked at does not hold it back, and the edit's own result is the same as before.
func TestAnEditWhoseRowsTheGameLeftAloneIsStillPutBack(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	addN(t, s1, 1, 250)
	s1.Close()
	putNewerFile(t, p, journalDDL+`update jt set n=77 where a=2;`) // another row
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if nOf(t, s2, 1) != 260 || nOf(t, s2, 2) != 77 {
		t.Fatalf("the edit and the game's own change should both be there: %d %d", nOf(t, s2, 1), nOf(t, s2, 2))
	}
	// what the put-back edit read is kept again, so the next put-back is guarded the same way
	s2.Close()
	putNewerFile(t, p, journalDDL+`update jt set n=999 where a=1;`)
	s3 := openJournaled(t, p, dir)
	if r, _ := s3.RestoreJournal(); r == nil || len(r.Conflicts) != 1 {
		t.Fatalf("a second put-back must be guarded too: %+v", r)
	}
}

// The automatic write: the queue is held and nothing is written when an edit was worked out from rows the game has since changed.
func TestFlushHoldsAQueueWhoseEditWasWorkedOutFromRowsTheGameChanged(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	restore := ActiveSessionForTests() // the game is running: the edit waits
	addN(t, s, 1, 250)
	if f := s.Flush(); f.Saved {
		t.Fatalf("a running game blocks the write: %+v", f)
	}
	restore() // the game is closed; its last save is on disk
	other, _ := Open(p)
	if _, err := other.Exec(`update jt set n=5100 where a=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Commit(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	f := s.Flush()
	if f.Saved || s.Held() == nil {
		t.Fatalf("nothing is written and the queue is held: %+v", f)
	}
	chk, _ := Open(p)
	defer chk.Close()
	if r, _ := chk.One(`select n from jt where a=1`); r["n"] != int64(5100) {
		t.Fatalf("the game's value on disk must be untouched: %v", r)
	}
}

// A journal written before reads were kept has none to check, and is put back as it always was.
func TestAJournalWithoutReadsIsPutBackAsBefore(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	if _, err := s1.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	s1.Close()
	putNewerFile(t, p, journalDDL+`update jt set n=5 where a=1;`)
	s2 := openJournaled(t, p, dir)
	if r, err := s2.RestoreJournal(); err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("restore: %+v %v", r, err)
	}
}

func TestOnlyASelectIsRunAgainAsARead(t *testing.T) {
	for q, want := range map[string]bool{
		"select n from jt where a=?":           true,
		"  SELECT 1;":                          true,
		"with x as (select 1) select * from x": true,
		"update jt set n=1":                    false,
		"select 1; delete from jt":             false,
		"with x as (select 1) delete from jt":  false,
		"":                                     false,
		"pragma writable_schema=on":            false,
	} {
		if got := readable(q); got != want {
			t.Errorf("readable(%q) = %v, want %v", q, got, want)
		}
	}
}

func TestHashRowsSeparatesTypesAndOrder(t *testing.T) {
	a := hashRows([]Row{{"n": int64(5)}})
	for name, other := range map[string][]Row{
		"float":  {{"n": float64(5)}},
		"text":   {{"n": "5"}},
		"nil":    {{"n": nil}},
		"two":    {{"n": int64(5)}, {"n": int64(5)}},
		"rename": {{"m": int64(5)}},
	} {
		if hashRows(other) == a {
			t.Errorf("%s hashes like the integer", name)
		}
	}
	if hashRows([]Row{{"n": int64(5)}}) != a {
		t.Error("the same rows must hash the same")
	}
	if hashRows([]Row{{"b": []byte{1}}}) == hashRows([]Row{{"b": []byte{2}}}) {
		t.Error("blobs differ")
	}
}

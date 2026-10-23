package save

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// At start-up a queue with an edit that no longer applies is held whole, as the automatic write holds it: the edits that did apply are not written alone, and the journal is kept
// (it used to be deleted when none applied, and the conflicts were only a notice that the next start did not have).
func TestAStartUpQueueWithAConflictIsHeldWholeAndItsJournalIsKept(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	s1 := openJournaled(t, p, dir)
	for _, d := range []struct{ desc, q string }{
		{"rename one", `update jt set name='uno' where a=1`},
		{"rename two", `update jt set name='dos' where a=2`},
	} {
		if _, err := s1.Mutate(d.desc, func(m *Mut) error { _, err := m.Exec(d.q); return err }); err != nil {
			t.Fatal(err)
		}
	}
	s1.Close()
	putNewerFile(t, p, journalDDL+`delete from jt where a=1;`) // the game deleted row 1: "rename one" no longer applies
	game, _ := os.ReadFile(p)
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 0 || len(r.Conflicts) != 1 || s2.Held() == nil {
		t.Fatalf("held, with the conflict reported: %+v %v held=%v", r, err, s2.Held())
	}
	if f := s2.Flush(); f.Saved {
		t.Fatalf("a held queue is not written: %+v", f)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, game) {
		t.Fatal("the game's file is untouched: the edit that applied was not written alone")
	}
	jb, err := os.ReadFile(jp)
	if err != nil || bytes.Count(jb, []byte(`"desc"`)) != 2 {
		t.Fatalf("the journal still holds both edits: %v %s", err, jb)
	}
	s2.Close()
	// the next start finds the same queue and holds it again
	s3 := openJournaled(t, p, dir)
	if r, err := s3.RestoreJournal(); err != nil || r == nil || len(r.Conflicts) != 1 || s3.Held() == nil {
		t.Fatalf("a second start: %+v %v", r, err)
	}
}

// When nothing applies the journal is no longer deleted.
func TestAStartUpQueueWhereNothingAppliesKeepsItsJournal(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	s1 := openJournaled(t, p, dir)
	if _, err := s1.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	s1.Close()
	putNewerFile(t, p, journalDDL+`delete from jt where a=1;`)
	s2 := openJournaled(t, p, dir)
	if r, err := s2.RestoreJournal(); err != nil || r == nil || len(r.Conflicts) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := os.Stat(jp); err != nil {
		t.Fatalf("the journal of a queue that did not apply is kept: %v", err)
	}
}

package save

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// A queue is put back onto the game's newer file as a whole. When one edit conflicts the file is read again to undo the ones that applied; if that read fails the queue must not be
// left as the part that applied (in memory and in the journal), or the next write would put a partial queue into the game's file.
func TestAFailedSecondLoadLeavesTheWholeQueueHeldNotAPartOfIt(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate("add row 3", func(m *Mut) error { _, err := m.Exec(`insert into jt values (3,'ours',0.5,x'00',1)`); return err }); err != nil {
		t.Fatal(err)
	}
	other, _ := Open(p) // the game saved, and used id 3 for a row of its own
	if _, err := other.Exec(`insert into jt values (3,'the game''s',9.5,x'09',9)`); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	game, _ := os.ReadFile(p)
	// the file disappears after the first edit is put back, so the read that undoes it fails
	replayStep = func() { os.Remove(p) }
	f := s.Flush()
	replayStep = nil
	if f.Saved || f.Why == "" {
		t.Fatalf("nothing is written: %+v", f)
	}
	if err := os.WriteFile(p, game, 0o600); err != nil { // the file is back, as the game had it
		t.Fatal(err)
	}
	if h := s.Held(); h == nil || len(h.ops) != 2 {
		t.Fatalf("the whole queue is held, not the part that applied: %+v", h)
	}
	jb, err := os.ReadFile(filepath.Join(dir, "pending.json"))
	if err != nil || bytes.Count(jb, []byte(`"desc"`)) != 2 {
		t.Fatalf("the journal still holds both edits: %v %s", err, jb)
	}
	again := s.Flush()
	if again.Saved {
		t.Fatalf("a held queue is not written: %+v", again)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, game) {
		t.Fatal("the game's file must be untouched")
	}
}

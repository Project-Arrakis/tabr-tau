package save

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The notes of an edit (#777) are handed to the sink once the edit is in the game's file, and never before.

type sunk struct{ got []Note }

func (k *sunk) sink(n []Note) error { k.got = append(k.got, n...); return nil }

func noteEdit(t *testing.T, s *Save, a int, kind string, v int) {
	t.Helper()
	if _, err := s.Mutate("edit", func(m *Mut) error {
		if _, err := m.Exec(`update jt set n=? where a=?`, a*100, a); err != nil {
			return err
		}
		return m.Note(kind, map[string]int{"v": v})
	}); err != nil {
		t.Fatal(err)
	}
}

func values(t *testing.T, ns []Note) []int {
	t.Helper()
	var out []int
	for _, n := range ns {
		var d map[string]int
		if err := json.Unmarshal(n.Data, &d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d["v"])
	}
	return out
}

func TestNotesAreHandedOverOnlyAfterTheWriteAndInOrder(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s, _ := Open(p)
	defer s.Close()
	var k sunk
	s.SetNoteSink(k.sink)
	noteEdit(t, s, 1, "x", 1)
	noteEdit(t, s, 2, "x", 2)
	if len(k.got) != 0 {
		t.Fatalf("a note was handed over before the write: %v", k.got)
	}
	if f := s.Flush(); !f.Saved {
		t.Fatalf("flush: %+v", f)
	}
	if got := values(t, k.got); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("notes handed over: %v", got)
	}
	// written once: the next write hands over only what is new
	noteEdit(t, s, 1, "x", 3)
	s.Flush()
	if got := values(t, k.got); len(got) != 3 || got[2] != 3 {
		t.Fatalf("notes after the second write: %v", got)
	}
}

func TestNotesOfAFailedWriteAreNotHandedOver(t *testing.T) {
	p := makeSave(t, journalDDL)
	s, _ := Open(p)
	defer s.Close()
	var k sunk
	s.SetNoteSink(k.sink)
	noteEdit(t, s, 1, "x", 1)
	old, oldWait := fsRename, renameBackoff
	fsRename = func(a, b string) error { return errors.New("locked") }
	renameBackoff = 0
	defer func() { fsRename, renameBackoff = old, oldWait }()
	if _, err := s.CommitForTests(); err == nil {
		t.Fatal("expected the write to fail")
	}
	if len(k.got) != 0 {
		t.Fatalf("a note of an edit that is not in the file was handed over: %v", k.got)
	}
}

func TestNotesOfAFailedEditAndOfADiscardedQueueAreDropped(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s, _ := Open(p)
	defer s.Close()
	var k sunk
	s.SetNoteSink(k.sink)
	if _, err := s.Mutate("fails", func(m *Mut) error {
		if _, err := m.Exec(`update jt set n=1 where a=1`); err != nil {
			return err
		}
		if err := m.Note("x", map[string]int{"v": 1}); err != nil {
			return err
		}
		return errors.New("refused")
	}); err == nil {
		t.Fatal("expected a refusal")
	}
	if got := s.PendingNotes("x"); len(got) != 0 {
		t.Fatalf("the note of a refused edit is pending: %v", got)
	}
	noteEdit(t, s, 1, "x", 2)
	if got := s.PendingNotes("x"); len(got) != 1 {
		t.Fatalf("a queued edit's note is pending: %v", got)
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if got := s.PendingNotes("x"); len(got) != 0 {
		t.Fatalf("a discarded edit's note is still pending: %v", got)
	}
	noteEdit(t, s, 2, "x", 3)
	s.Flush()
	if got := values(t, k.got); len(got) != 1 || got[0] != 3 {
		t.Fatalf("only the edit that was written hands its note over: %v", got)
	}
}

func TestPendingNotesAreSeenInsideALaterEditAndGoOnceWritten(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s, _ := Open(p)
	defer s.Close()
	noteEdit(t, s, 1, "x", 7)
	noteEdit(t, s, 2, "y", 8)
	var seen []int
	if _, err := s.Mutate("looks", func(m *Mut) error {
		seen = values(t, notesOfRaw(m.Pending("x")))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != 7 {
		t.Fatalf("pending notes of the kind inside an edit: %v", seen)
	}
	s.Flush()
	if got := s.PendingNotes("x"); len(got) != 0 {
		t.Fatalf("a written edit's note is still pending: %v", got)
	}
}

func notesOfRaw(r []json.RawMessage) []Note {
	var out []Note
	for _, d := range r {
		out = append(out, Note{Data: d})
	}
	return out
}

// An edit kept in the journal keeps its note, and the note is handed over when the put-back edit is written by the next run.
func TestNotesSurviveTheJournalAndAreHandedOverWhenWritten(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	noteEdit(t, s1, 1, "x", 41)
	s1.Close()

	s2 := openJournaled(t, p, dir)
	var k sunk
	s2.SetNoteSink(k.sink)
	if r, err := s2.RestoreJournal(); err != nil || r == nil || r.Applied != 1 {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if got := s2.PendingNotes("x"); len(got) != 1 {
		t.Fatalf("the put-back edit's note is pending: %v", got)
	}
	if len(k.got) != 0 {
		t.Fatal("handed over before it was written")
	}
	if f := s2.Flush(); !f.Saved {
		t.Fatalf("flush: %+v", f)
	}
	if got := values(t, k.got); len(got) != 1 || got[0] != 41 {
		t.Fatalf("notes handed over after the restart: %v", got)
	}
	if _, err := filepath.Glob(filepath.Join(dir, "pending.json")); err != nil {
		t.Fatal(err)
	}
}

// A queue that is put back onto a newer file (the game saved meanwhile) takes its notes with it.
func TestNotesGoBackWithTheQueueOntoANewerFile(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	var k sunk
	s.SetNoteSink(k.sink)
	noteEdit(t, s, 1, "x", 5)
	if r, err := s.Rebase(); err != nil || r == nil || r.Applied != 1 {
		t.Fatalf("rebase: %+v %v", r, err)
	}
	if got := s.PendingNotes("x"); len(got) != 1 {
		t.Fatalf("the note did not come back with its edit: %v", got)
	}
	s.Flush()
	if got := values(t, k.got); len(got) != 1 || got[0] != 5 {
		t.Fatalf("notes handed over: %v", got)
	}
}

// A sink that fails keeps the notes for the next delivery, the answer of the write carries a warning that names no folder, and the next write hands over everything in order (#777).
func TestNotesASinkThatFailsKeepsThemForTheNextWriteAndSaysSo(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s, _ := Open(p)
	defer s.Close()
	var k sunk
	fail := true
	s.SetNoteSink(func(n []Note) error {
		if fail {
			return &os.PathError{Op: "open", Path: `C:\Users\someone\secret-folder\paid.json`, Err: errors.New("sharing violation")}
		}
		return k.sink(n)
	})
	noteEdit(t, s, 1, "x", 1)
	f := s.Flush()
	if !f.Saved || !strings.Contains(f.Warning, "could not be recorded") || strings.Contains(f.Warning, "secret-folder") || strings.Contains(f.Warning, `C:\`) {
		t.Fatalf("the write is saved, the warning says what failed and names no folder: %+v", f)
	}
	fail = false
	noteEdit(t, s, 2, "x", 2)
	if f := s.Flush(); !f.Saved || f.Warning != "" {
		t.Fatalf("the next write: %+v", f)
	}
	if got := values(t, k.got); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("the kept note comes first, then the new one: %v", got)
	}
	// a write with no notes also tries again
	fail = true
	noteEdit(t, s, 1, "x", 3)
	s.Flush()
	fail = false
	if _, err := s.Exec(`update jt set n=1 where a=2`); err != nil {
		t.Fatal(err)
	}
	if f := s.Flush(); !f.Saved {
		t.Fatalf("flush: %+v", f)
	}
	if got := values(t, k.got); len(got) != 3 || got[2] != 3 {
		t.Fatalf("a write with no notes of its own hands over the ones that were kept: %v", got)
	}
}

// A restored backup replaces the game's file wholesale: the sink is told, after the file is replaced.
func TestNotesARestoredBackupTellsTheSinkTheFileWasReplaced(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s, _ := Open(p)
	defer s.Close()
	noteEdit(t, s, 1, "x", 1)
	if f := s.Flush(); !f.Saved {
		t.Fatalf("flush: %+v", f)
	}
	var k sunk
	s.SetNoteSink(k.sink)
	bs := s.Backups()
	if len(bs) == 0 {
		t.Fatal("no backup")
	}
	if err := s.Restore(bs[0]["name"].(string)); err != nil {
		t.Fatal(err)
	}
	if len(k.got) != 1 || k.got[0].Kind != NoteSaveReplaced {
		t.Fatalf("the sink was not told the file was replaced: %v", k.got)
	}
}

// A restart that finds a journal marked incomplete and the file to be the one the edits were written as hands over the notes the journal holds before it goes.
func TestNotesAnIncompleteJournalOfAWrittenQueueHandsOverItsNotes(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	noteEdit(t, s1, 1, "x", 77)
	jp := filepath.Join(dir, "pending.json")
	kept, err := os.ReadFile(jp)
	if err != nil {
		t.Fatal(err)
	}
	if f := s1.Flush(); !f.Saved {
		t.Fatalf("flush: %+v", f)
	}
	s1.Close()
	// the journal could not be removed after the write, and the mark says which file the edits were written as
	disk, _ := os.ReadFile(p)
	h := hashOf(disk)
	if err := os.WriteFile(jp, kept, 0o600); err != nil {
		t.Fatal(err)
	}
	mark := `{"version":2,"reason":"the kept file could not be removed","writing":"` + hex.EncodeToString(h[:]) + `"}`
	if err := os.WriteFile(jp+".incomplete", []byte(mark), 0o600); err != nil {
		t.Fatal(err)
	}
	s2 := openJournaled(t, p, dir)
	var k sunk
	s2.SetNoteSink(k.sink)
	if r, err := s2.RestoreJournal(); err != nil || r != nil {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if got := values(t, k.got); len(got) != 1 || got[0] != 77 {
		t.Fatalf("the notes of the written edits are handed over: %v", got)
	}
}

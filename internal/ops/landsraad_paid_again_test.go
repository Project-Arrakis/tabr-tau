package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// A kept note that comes again (a journal that outlived the write and holds only the first of two edits, and a record that cannot be written at the restart) asks to replace what the
// record holds for the house. The sink only merges such a note, so the board and a raise must not read it as a replace either: the 6,000 already in the file stands, and a removal
// followed by the same raise pays nothing (#810).
func TestPaidAKeptNoteThatComesAgainDoesNotLowerTheRecordForARaise(t *testing.T) {
	o, path := paidOps(t, "")
	jp := filepath.Join(t.TempDir(), "pending.json")
	o.S.EnableJournal(jp)
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	putBackBeforeTheRaise(t, o, 0)
	fillKenolaDoubled(t, o, 0, 1200) // pays tier 1 under a record that the cap cut: its note replaces
	first, err := os.ReadFile(jp)    // the journal as it was after the first of the two edits
	if err != nil {
		t.Fatal(err)
	}
	fillKenolaDoubled(t, o, 1200, 6000)
	flush(t, o)
	if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].Paid != 6000 {
		t.Fatalf("the record after the whole batch: %+v", f)
	}
	// the editor stopped between the write and the removal of the journal; the journal held only the first edit and is marked incomplete with the hash of the file that was written
	b, err := os.ReadFile(o.S.Path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	if err := os.WriteFile(jp, first, 0o600); err != nil {
		t.Fatal(err)
	}
	mark, _ := json.Marshal(map[string]any{"version": 2, "reason": "test", "writing": hex.EncodeToString(h[:])})
	if err := os.WriteFile(jp+".incomplete", mark, 0o600); err != nil {
		t.Fatal(err)
	}
	p := o.S.Path
	o.S.Close()
	s2, err := save.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	o2 := &Ops{S: s2, Paid: NewPaidLedger(path, s2.Path)}
	fail := true
	failingRecord(o2, &fail) // the record cannot be written at start
	s2.EnableJournal(jp)
	if _, err := s2.RestoreJournal(); err != nil {
		t.Fatal(err)
	}
	kept := s2.PendingNotes(paidNoteKind)
	if s2.NotesProblem() == "" || len(kept) != 1 || !kept[0].Again {
		t.Fatalf("the note must be kept, and marked as one that comes again: %+v, %q", kept, s2.NotesProblem())
	}
	if got := taskByID(t, o2, 175)["my_paid"]; got != 6000.0 {
		t.Fatalf("the board must show the 6000 of the file, not the replace of the note that comes again: %v", got)
	}
	solari := wallet(t, o2, 0)
	removeTo(t, o2, 6000, 0)
	m := fillKenolaDoubled(t, o2, 0, 6000)
	if m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 0 || wallet(t, o2, 0) != solari {
		t.Fatalf("the same raise paid again: %v, wallet %d -> %d", m, solari, wallet(t, o2, 0))
	}
}

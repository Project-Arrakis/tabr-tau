package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Third review of #798: a write in another term leaves this term's record alone, a tier removal does not need the record, and the bound evicts the oldest terms only.

func paidNoteOf(term int64, house string, paid float64) save.Note {
	b, _ := json.Marshal(paidNote{Player: 1, Term: term, House: house, Faction: 2, Paid: paid})
	return save.Note{Kind: paidNoteKind, Data: b}
}

// A backup of last week put back, one raise there, and the file put back again: the first write is in term 6, the record of term 7 must still be there.
func TestPaidAWriteInAnotherTermLeavesThisTermsRecordAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, `C:\X\game.db`)
	if err := l.Record([]save.Note{paidNoteOf(7, "A", 6000), paidNoteOf(7, "B", 2400)}); err != nil {
		t.Fatal(err)
	}
	if err := l.Record([]save.Note{paidNoteOf(6, "A", 1200)}); err != nil {
		t.Fatal(err)
	}
	if l.recorded(1, 7, "A") != 6000 || l.recorded(1, 7, "B") != 2400 || l.recorded(1, 6, "A") != 1200 {
		t.Fatalf("the term 7 record went with the term 6 write: %+v", readPaid(t, path))
	}
}

func TestPaidEvictionTakesTheOldestTermsFirstAndNeverTheTermBeingWritten(t *testing.T) {
	var es []paidNote
	for term := int64(1); term <= 5; term++ {
		for h := 0; h < 4; h++ {
			es = append(es, paidNote{Player: 1, Term: term, House: fmt.Sprint("H", h), Faction: 2, Paid: 1200})
		}
	}
	count := func(got []paidNote, term int64) int {
		n := 0
		for _, e := range got {
			if e.Term == term {
				n++
			}
		}
		return n
	}
	got := evictPaid(es, 3, 12)
	if len(got) != 12 || count(got, 1) != 0 || count(got, 2) != 0 || count(got, 3) != 4 || count(got, 5) != 4 {
		t.Fatalf("oldest terms go first: %d entries, terms 1/2/3/5: %d %d %d %d", len(got), count(got, 1), count(got, 2), count(got, 3), count(got, 5))
	}
	got = evictPaid(es, 1, 12) // the term being written is the oldest: it stays, the next oldest go
	if len(got) != 12 || count(got, 1) != 4 || count(got, 2) != 0 || count(got, 3) != 0 || count(got, 4) != 4 {
		t.Fatalf("the term being written stays: %d entries, terms 1/2/3/4: %d %d %d %d", len(got), count(got, 1), count(got, 2), count(got, 3), count(got, 4))
	}
	if got := evictPaid(es[:4], 1, 2); len(got) != 2 {
		t.Fatalf("only the term being written is above the bound: %d", len(got))
	}
	if got := evictPaid(es, 3, 20); len(got) != 20 {
		t.Fatalf("within the bound nothing goes: %d", len(got))
	}
}

// Through Record, at the real bound: a full file of one term, then a note of another.
func TestPaidTheBoundEvictsOldTermsThroughRecord(t *testing.T) {
	full := func(term int64) []paidNote {
		var es []paidNote
		for i := 0; i < paidMaxEntries; i++ {
			es = append(es, paidNote{Player: 1, Term: term, House: fmt.Sprint("h", i), Faction: 2, Paid: 1200})
		}
		return es
	}
	for _, c := range []struct {
		name         string
		inFile, note int64
	}{{"a newer term is written", 1, 2}, {"an older term is written", 2, 1}} {
		path := filepath.Join(t.TempDir(), "p.json")
		l := NewPaidLedger(path, `C:\X\game.db`)
		b, _ := json.Marshal(paidFile{Version: paidFileVersion, Save: l.save, Entries: full(c.inFile)})
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := l.Record([]save.Note{paidNoteOf(c.note, "new", 600)}); err != nil {
			t.Fatal(err)
		}
		f := readPaid(t, path)
		if len(f.Entries) != paidMaxEntries || l.recorded(1, c.note, "new") != 600 {
			t.Fatalf("%s: %d entries, the new one is %v", c.name, len(f.Entries), l.recorded(1, c.note, "new"))
		}
	}
}

// A tier removal pays nothing and needs no record: an unreadable record does not stop it, and it does not read the file (it reads it only when the top tier's house swatch is among
// the removed tiers, to tell whether the editor wrote the swatch: landsraad_takeback_test.go). A raise is refused with a message that says it and names no folder.
func TestPaidARemovalGoesThroughWithAnUnreadableRecordAndARaiseIsRefused(t *testing.T) {
	o, _ := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 4800)
	flush(t, o)
	oldOpen, oldPause := paidOpen, paidReadPause
	paidReadPause = 0
	defer func() { paidOpen, paidReadPause = oldOpen, oldPause }()
	paidOpen = unreadable(nil)
	before := paidReads.Load()
	removeTo(t, o, 4800, 1200) // applyWith fails the test on an error
	if n := paidReads.Load() - before; n != 0 {
		t.Fatalf("a removal read the record %d times", n)
	}
	if got := rows(t, o, `select amount from landsraad_task_player_contributions where player_id=1 and task_id=175`); !strings.Contains(got, "1200") {
		t.Fatalf("the removal was not made: %s", got)
	}
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1200.0, "mine_to": 6000.0})))
	if !errors.Is(err, apierr.ErrWriteFailed) {
		t.Fatalf("the raise is refused: %v", err)
	}
	if !strings.Contains(err.Error(), "nothing was written") || !strings.Contains(err.Error(), "could not be read") || strings.Contains(err.Error(), os.TempDir()) || strings.Contains(err.Error(), "sharing violation") {
		t.Fatalf("the refusal says what happened and that nothing was written, and names no path: %v", err)
	}
}

package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// A record that could not be written must not let a raise pay the same ground again (#810): the save keeps the notes of the written edits for the next write, and the raise counts them.

// failingRecord makes the sink of the record fail while *fail is true and record as usual after it.
func failingRecord(o *Ops, fail *bool) {
	o.S.SetNoteSink(func(n []save.Note) error {
		if *fail {
			return errors.New("the record file is held by another program")
		}
		return o.Paid.Record(n)
	})
}

func flushWithWarning(t *testing.T, o *Ops) {
	t.Helper()
	f := o.S.Flush()
	if !f.Saved || !strings.Contains(o.S.NotesProblem(), "could not be recorded") {
		t.Fatalf("expected a write whose record failed: %+v, %q", f, o.S.NotesProblem())
	}
}

func TestPaidARaiseAfterAFailedWriteOfTheRecordDoesNotPayTheSameGroundAgain(t *testing.T) {
	o, path := paidOps(t, "")
	fail := true
	failingRecord(o, &fail)
	fillKenolaDoubled(t, o, 1300, 6000)
	flushWithWarning(t, o) // the game's file is written, the record is not
	removeTo(t, o, 6000, 0)
	flushWithWarning(t, o) // still failing
	solari, scrip := wallet(t, o, 0), wallet(t, o, 1)
	facBefore, _ := progress(t, o, 2, 175)
	// the board counts the ground the record has not taken yet
	if got := taskByID(t, o, 175)["my_paid"]; got != 6000.0 {
		t.Fatalf("my_paid of a note the record could not take: %v", got)
	}
	m := fillKenolaDoubled(t, o, 0, 6000)
	// no mission is paid again (the record the file could not take is still counted); the five rewards the removal took back are written again
	if m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 5 || m["solari"].(int64) != 0 || m["ground_paid_before"].(float64) != 6000 {
		t.Fatalf("the same raise paid again: %v", m)
	}
	if wallet(t, o, 0) != solari || wallet(t, o, 1) != scrip {
		t.Fatal("a wallet changed")
	}
	if v, _ := progress(t, o, 2, 175); v != facBefore {
		t.Fatalf("faction progress: %v, was %v", v, facBefore)
	}
	// the record comes back: the next write hands over the kept note, and the ground is still paid once
	fail = false
	flush(t, o)
	if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].Paid != 6000 || o.S.NotesProblem() != "" {
		t.Fatalf("the kept note was not recorded: %+v, %q", f, o.S.NotesProblem())
	}
	removeTo(t, o, 6000, 0)
	if m := fillKenolaDoubled(t, o, 0, 6000); m["missions"].(int64) != 0 || m["solari"].(int64) != 0 {
		t.Fatalf("a raise after the record was written paid again: %v", m)
	}
}

// A raise above what a failed record holds pays only the ground above it.
func TestPaidAHigherRaiseAfterAFailedWriteOfTheRecordPaysOnlyTheNewTiers(t *testing.T) {
	o, _ := paidOps(t, "")
	fail := true
	failingRecord(o, &fail)
	fillKenolaDoubled(t, o, 1300, 2400)
	flushWithWarning(t, o)
	removeTo(t, o, 2400, 0)
	flushWithWarning(t, o)
	solari := wallet(t, o, 0)
	m := fillKenolaDoubled(t, o, 0, 4800)
	if m["missions"].(int64) != 1 || m["reward_rows"].(int64) != 4 || m["missions_paid_before"].(int64) != 2 || m["ground_paid_before"].(float64) != 2400 {
		t.Fatalf("answer: %v", m)
	}
	if wallet(t, o, 0) != solari+6000 {
		t.Fatalf("Solari: %d, was %d", wallet(t, o, 0), solari)
	}
}

// When a failing record has dropped notes of written edits (past the save's bound), a raise is refused with nothing written, and a removal is not.
func TestPaidARaiseIsRefusedWhenAFailingRecordDroppedNotes(t *testing.T) {
	t.Parallel()
	o, _ := paidOps(t, `insert into player_virtual_currency_balances(player_controller_id, currency_id, balance) values (1, 0, 1000);`)
	fail := true
	failingRecord(o, &fail)
	for i := 0; i < 1001; i++ { // one more than the save keeps for a sink that fails
		if _, err := o.S.Mutate("note", func(m *save.Mut) error {
			if _, err := m.Exec(`update player_virtual_currency_balances set balance=balance+1 where player_controller_id=1 and currency_id=0`); err != nil {
				return err
			}
			return m.Note(paidNoteKind, paidNote{Player: 1, Term: 7, House: "DA_HouseOther", Faction: 2, Paid: 100})
		}); err != nil {
			t.Fatal(err)
		}
	}
	flushWithWarning(t, o)
	if !o.S.NotesDropped() {
		t.Fatal("the save does not say that notes were dropped")
	}
	solari := wallet(t, o, 0)
	refused := func(was float64) {
		t.Helper()
		_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": was, "mine_to": 6000.0})))
		if !errors.Is(err, errPaidNotesDropped) || !errors.Is(err, apierr.ErrWriteFailed) {
			t.Fatalf("a raise was not refused with the dropped-notes error of kind write_failed: %v", err)
		}
		if wallet(t, o, 0) != solari {
			t.Fatal("a refused raise wrote")
		}
	}
	refused(1300)
	removeTo(t, o, 1300, 0) // a removal pays nothing and needs no record
	// the record works again and the next write hands over what was kept: the notes that were dropped are still unknown, so the refusal stays for the session
	fail = false
	flush(t, o)
	if o.S.NotesProblem() != "" || !o.S.NotesDropped() {
		t.Fatalf("the kept notes were handed over (problem %q) and the flag must stay (%v)", o.S.NotesProblem(), o.S.NotesDropped())
	}
	refused(0)
}

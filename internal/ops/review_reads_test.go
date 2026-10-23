package ops

import (
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// gameChanges is the game saving a change to the file while an edit of the editor waits.
func gameChanges(t *testing.T, path, q string, args ...any) {
	t.Helper()
	other, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.Mutate("the game's change", func(m *save.Mut) error { _, err := m.Exec(q, args...); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
}

func mustBeHeld(t *testing.T, s *save.Save) {
	t.Helper()
	f := s.Flush()
	if f.Saved || s.Held() == nil {
		t.Fatalf("the edit was worked out from rows the game has changed: it is held, not written: %+v", f)
	}
}

// An edit that read the player's position outside the edit replayed the old position as literals: bringing a vehicle must be held when the player has moved.
func TestAQueuedBringVehicleIsHeldWhenThePlayerMoved(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := vehicleOps(t)
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if _, err := o.BringVehicle(Args{"id": 201.0}); err != nil {
		t.Fatal(err)
	}
	p, err := o.player()
	if err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `update actors set location_x=location_x+5000 where id=?`, p.Pawn)
	mustBeHeld(t, o.S)
}

// The same for the Landsraad term: an edit made on one term is not applied to the term id it replays as a literal when the game has started another.
func TestAQueuedLandsraadTermEditIsHeldWhenTheGameStartedANewTerm(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into landsraad_decree_term(term_id,start_time,end_time) values (1,0,100);`)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if _, err := o.SetTerm(Args{"end_time": 500.0}); err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `insert into landsraad_decree_term(term_id,start_time,end_time) values (2,100,200)`)
	mustBeHeld(t, o.S)
}

// And for the list of containers a refill works from.
func TestAQueuedRefillIsHeldWhenTheGameChangedTheContainers(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := newOps(t)
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	// the container is written first, so that only the refill is queued
	if _, err := o.S.Mutate("fixture", func(m *save.Mut) error {
		_, err := m.Exec(`insert into items values (202, 1, 1, 222, 'Literjon', 0, 0, '{"FItemStackAndDurabilityStats":[[],{}]}', 0)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if f := o.S.Flush(); !f.Saved {
		t.Fatalf("fixture written: %+v", f)
	}
	if _, err := o.RefillContainers(); err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `insert into items values (203, 1, 1, 223, 'Literjon', 0, 0, '{"FItemStackAndDurabilityStats":[[],{}]}', 0)`)
	mustBeHeld(t, o.S)
}

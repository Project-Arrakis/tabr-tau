package save_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// An edit that changed rows is always listed (#592): a save that is waiting to be written with an empty list could not be told from an empty queue, and the page offered to
// discard "0 changes". An edit with no description gets a line of its own, with the rows it changed, and is kept in the journal like any other.
func TestAnEditWithNoDescriptionIsStillListed(t *testing.T) {
	s := testsave.Player(t)
	n, err := s.Mutate("", func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=7 where id=11`)
		return err
	})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	dirty, pending := s.QueueState()
	if !dirty || len(pending) != 1 {
		t.Fatalf("dirty=%v pending=%v: a change that is waiting must be listed", dirty, pending)
	}
	if pending[0] != "unnamed change (1 rows)" {
		t.Fatalf("pending = %q", pending[0])
	}
	if got := s.Pending(); len(got) != 1 || got[0] != pending[0] {
		t.Fatalf("Pending = %v", got)
	}
}

func TestAnEditThatChangedNothingLeavesNothingListedWhateverItsDescription(t *testing.T) {
	s := testsave.Player(t)
	for _, desc := range []string{"", "named"} {
		if n, err := s.Mutate(desc, func(m *save.Mut) error {
			_, err := m.Exec(`update items set stack_size=1 where id=99999`)
			return err
		}); err != nil || n != 0 {
			t.Fatalf("desc %q: n=%d err=%v", desc, n, err)
		}
	}
	if dirty, pending := s.QueueState(); dirty || len(pending) != 0 {
		t.Fatalf("dirty=%v pending=%v: nothing changed, nothing is queued", dirty, pending)
	}
}

// What the page is told is read under one lock: whenever the save is dirty, something is listed. Read as two calls, a write that finished between them gave dirty with nothing listed.
func TestQueueStateNeverSaysDirtyWithNothingListed(t *testing.T) {
	s := testsave.Player(t)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // edits and discards, as a person editing and a write completing would
		defer wg.Done()
		for i := 0; i < 300; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := s.Mutate(fmt.Sprintf("edit %d", i), func(m *save.Mut) error {
				_, err := m.Exec(`update items set stack_size=? where id=11`, i+2)
				return err
			}); err != nil {
				t.Error(err)
				return
			}
			if err := s.Discard(); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for i := 0; i < 3000; i++ {
		if dirty, pending := s.QueueState(); dirty && len(pending) == 0 {
			close(stop)
			wg.Wait()
			t.Fatalf("poll %d: dirty with nothing listed", i)
		}
	}
	close(stop)
	wg.Wait()
	if dirty, pending := s.QueueState(); dirty != s.Dirty() || strings.Join(pending, "|") != strings.Join(s.Pending(), "|") {
		t.Fatalf("QueueState %v %v disagrees with Dirty/Pending when nothing moves", dirty, pending)
	}
}

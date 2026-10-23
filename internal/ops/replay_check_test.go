package ops

import (
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

func gameSetsSolari(t *testing.T, path string, n int) {
	t.Helper()
	other, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.Mutate("the game's Solari", func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=? where id=10`, n)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.Commit(false); err != nil {
		t.Fatal(err)
	}
}

func solariOnDisk(t *testing.T, path string) int64 {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select stack_size from items where id=10`)
	if err != nil {
		t.Fatal(err)
	}
	return r["stack_size"].(int64)
}

// A queued "Solari +250" worked out from 100 must not be written over the 5,100 the game saved meanwhile (it used to leave 350): checked on the editor's own Solari edit, through
// the automatic write and through Reload now.
func TestAQueuedSolariEditIsNotWrittenOverWhatTheGameSavedMeanwhile(t *testing.T) {
	for _, via := range []string{"flush", "reload"} {
		t.Run(via, func(t *testing.T) {
			o := newOps(t)
			o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
			if _, err := o.AddSolari(Args{"amount": 250.0}); err != nil {
				t.Fatal(err)
			}
			if o.S.Dirty() == false {
				t.Fatal("the edit is queued")
			}
			gameSetsSolari(t, o.S.Path, 5100)
			switch via {
			case "flush":
				f := o.S.Flush()
				if f.Saved || o.S.Held() == nil {
					t.Fatalf("the queue is held, not written: %+v", f)
				}
			case "reload":
				r, err := o.S.Rebase()
				if err != nil || r == nil || r.Applied != 0 || len(r.Conflicts) != 1 {
					t.Fatalf("the edit is reported, not forced: %+v %v", r, err)
				}
				if f := o.S.Flush(); f.Saved && o.S.Dirty() {
					t.Fatalf("%+v", f)
				}
			}
			if got := solariOnDisk(t, o.S.Path); got != 5100 {
				t.Fatalf("the game's 5,100 Solari are kept in the file, found %d", got)
			}
			if r, _ := o.S.One(`select stack_size from items where id=10`); r["stack_size"].(int64) != 5100 {
				t.Fatalf("and in the editor: %v", r)
			}
		})
	}
}

package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The state the page polls says "dirty" only with something listed (#592): the page offers Discard queue from it, and a queue of "0 changes" is not one.
func TestStateListsWhatIsWaitingWhenItSaysDirty(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	check := func(when string, want int) {
		t.Helper()
		st, err := o.State()
		if err != nil {
			t.Fatal(err)
		}
		m := st.(map[string]any)
		pending, _ := m["pending"].([]string)
		if dirty, _ := m["dirty"].(bool); dirty != (want > 0) || len(pending) != want {
			t.Fatalf("%s: dirty=%v pending=%v, want %d listed", when, m["dirty"], pending, want)
		}
	}
	check("at open", 0)
	if _, err := o.ExecSQL("update items set stack_size=101 where id=10"); err != nil {
		t.Fatal(err)
	}
	check("after a console edit", 1)
	if _, err := o.S.Mutate("", func(m *save.Mut) error { _, err := m.Exec(`update items set stack_size=102 where id=11`); return err }); err != nil {
		t.Fatal(err)
	}
	check("after an edit with no description", 2)
}

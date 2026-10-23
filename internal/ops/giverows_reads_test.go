package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A read of the give that fails is an error of the give, not "nothing is in the way": the used slots and the stats source are read inside the edit, and their errors are returned
// (ARCH-6 of register 2026-10-07, issue #39). The read is made to fail by taking the column it names away inside the edit.
func TestAGiveReturnsTheErrorOfAReadThatFailed(t *testing.T) {
	for _, col := range []string{"position_index", "inventory_id"} {
		s := testsave.Player(t)
		_, err := s.Mutate("give with a broken read", func(m *save.Mut) error {
			if _, err := m.Exec(`alter table items rename column ` + col + ` to gone_` + col); err != nil {
				return err
			}
			_, err := giveRowsInTx(m, 1, "Knife", []int64{1}, 0)
			return err
		})
		if err == nil || strings.Contains(err.Error(), "not found") {
			t.Errorf("%s: the failed read is the error of the give, got %v", col, err)
		}
		if s.Dirty() {
			t.Errorf("%s: a failed give changes nothing", col)
		}
	}
}

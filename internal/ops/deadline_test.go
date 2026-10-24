package ops

import (
	"errors"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// S7c of #40: the list of tables (a count of every table) stops at the read deadline and says so, instead of holding the save's lock or showing a list that looks complete.
func TestTheTableListStopsAtItsDeadline(t *testing.T) {
	o := newPlayerOps(t)
	if _, err := o.Tables(); err != nil {
		t.Fatalf("the list works: %v", err)
	}
	defer save.DeadlinesForTests(-time.Second)()
	if _, err := o.Tables(); !errors.Is(err, save.ErrStopped) {
		t.Fatalf("a list past its deadline is an error: %v", err)
	}
}

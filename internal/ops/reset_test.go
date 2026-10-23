package ops

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// digest renders every row of every table (and of one entity's columns when skip says so) as sorted text, so a test can say "nothing
// else changed" by comparing two digests. skipTable names tables left out; the fgl_entities row of entity skipEntity is left out, because the
// test inspects it on its own.
func digest(t *testing.T, o *Ops, skipTable map[string]bool, skipEntity int64) string {
	t.Helper()
	var lines []string
	for _, tb := range testsave.TableNames(t) {
		if skipTable[tb] {
			continue
		}
		q := `select * from "` + tb + `"`
		if tb == "fgl_entities" {
			q += fmt.Sprintf(` where entity_id<>%d`, skipEntity)
		}
		rows, err := o.S.Query(q)
		if err != nil {
			t.Fatalf("digest %s: %v", tb, err)
		}
		for _, r := range rows {
			lines = append(lines, tb+": "+fmt.Sprint(r))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// wantRefusal checks that err is a typed refusal of the given kind.
func wantRefusal(t *testing.T, err, kind error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: want a refusal, got none", what)
	}
	if !errors.Is(err, kind) {
		t.Fatalf("%s: want %v, got %v", what, kind, err)
	}
}

func TestExpectedCountIsRequiredAndBounded(t *testing.T) {
	for _, a := range []Args{{}, {"expected": -1.0}, {"expected": 1.5}, {"expected": "x"}, {"expected": 1e12}} {
		if _, err := expectedCount(a); err == nil || !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("%v: want a bad argument, got %v", a, err)
		}
	}
	if _, err := expectedCount(Args{}); err == nil || !strings.Contains(err.Error(), "expected is required") {
		t.Errorf("a missing count says what is missing: %v", err)
	}
	if n, err := expectedCount(Args{"expected": 0.0}); err != nil || n != 0 {
		t.Errorf("0 is a valid count: %d %v", n, err)
	}
	if n, err := expectedCount(Args{"expected": 7.0}); err != nil || n != 7 {
		t.Errorf("7: %d %v", n, err)
	}
}

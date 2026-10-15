package save

import "testing"

// A REAL cell holding infinity (9e999 in SQL) cannot be written as JSON. It leaves a query as NULL, so one hostile row cannot blank a whole
// view (SEC-1 of the 2026-10-15 audit).
func TestQueryReturnsNonFiniteNumbersAsNull(t *testing.T) {
	p := makeSave(t, `create table t(a real, b real, c real); insert into t values (9e999, -9e999, 1.5);`)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.One(`select a, b, c from t`)
	if err != nil || r == nil {
		t.Fatalf("%v %v", r, err)
	}
	if r["a"] != nil || r["b"] != nil {
		t.Fatalf("infinity must come back as NULL: %v", r)
	}
	if r["c"] != 1.5 {
		t.Fatalf("an ordinary number must be left alone: %v", r)
	}
	_, rows, err := s.ReadOnlyTable(`select a from t`, 10)
	if err != nil || len(rows) != 1 || rows[0][0] != nil {
		t.Fatalf("the read-only console path must do the same: %v %v", rows, err)
	}
}

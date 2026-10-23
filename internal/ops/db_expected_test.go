package ops

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// SEC-7 of the 2026-10-22 audit: a cell edit from the tables view carries the text the cell showed, and is refused when the cell holds something else now.

// shown is the text the page keeps of a cell (data-orig): nothing for an empty cell, the number as JavaScript writes it, the text as it is.
func shown(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func TestACellEditIsRefusedWhenTheCellChangedSinceTheTableWasShown(t *testing.T) {
	o := newPlayerOps(t)
	row := mustOne(t, o, `select rowid r, stack_size n, template_id t from items where id=10`)
	rid := float64(row["r"].(int64))
	// the table was shown with this value; the game (or a queued edit put back) changed it since
	if _, err := o.S.Mutate("the game's change", func(m *save.Mut) error { _, err := m.Exec(`update items set stack_size=777 where id=10`); return err }); err != nil {
		t.Fatal(err)
	}
	_, err := o.UpdateRow(Args{"table": "items", "column": "stack_size", "rowid": rid, "value": "5", "expected": shown(row["n"])})
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("a stale view is refused with the conflict kind: %v", err)
	}
	if stackOf(t, o, 10) != 777 {
		t.Fatal("the refused edit changed nothing")
	}
	// with what the table shows now it goes through
	if _, err := o.UpdateRow(Args{"table": "items", "column": "stack_size", "rowid": rid, "value": "5", "expected": "777"}); err != nil {
		t.Fatalf("the current value is accepted: %v", err)
	}
	if stackOf(t, o, 10) != 5 {
		t.Fatal("the edit was made")
	}
	// a row that is not there is not a conflict
	_, err = o.UpdateRow(Args{"table": "items", "column": "stack_size", "rowid": float64(987654321), "value": "5", "expected": "1"})
	if !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("an unknown row is still not found: %v", err)
	}
}

func TestTheExpectedTextIsComparedTheWayTheColumnReadsIt(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `
		create table tt(a integer primary key, i integer, r real, s text, n text);
		insert into tt values (1, 5, 5.0, 'x', null), (2, 9007199254740993, 1.5, '', ''), (3, 2, 2.5, 'y', 'z');`)}
	edit := func(rowid int, col string, expected any) error {
		_, err := o.UpdateRow(Args{"table": "tt", "column": col, "rowid": float64(rowid), "value": "1", "expected": expected})
		return err
	}
	cases := []struct {
		name     string
		row      int
		col      string
		expected any
		ok       bool
	}{
		{"an integer", 1, "i", "5", true},
		{"a real that the page shows as a whole number", 1, "r", "5", true},
		{"a real", 2, "r", "1.5", true},
		{"text", 1, "s", "x", true},
		{"NULL shown as nothing", 1, "n", "", true},
		{"NULL sent as null", 1, "n", nil, true},
		{"empty text shown as nothing", 2, "s", "", true},
		{"a whole number past 2^53, shown rounded by the page", 2, "i", "9007199254740992", true},
		{"a different integer", 1, "i", "6", false},
		{"different text", 1, "s", "y", false},
		{"nothing shown for a cell that holds text", 3, "s", "", false},
		{"text shown for a NULL", 1, "n", "x", false},
		{"a number for a cell that holds text", 3, "s", 7.0, false},
	}
	for _, c := range cases {
		// every case starts from the same rows
		if _, err := o.S.Mutate("reset", func(m *save.Mut) error {
			_, err := m.Exec(`delete from tt; insert into tt values (1, 5, 5.0, 'x', null), (2, 9007199254740993, 1.5, '', ''), (3, 2, 2.5, 'y', 'z')`)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		err := edit(c.row, c.col, c.expected)
		switch {
		case c.ok && err != nil:
			t.Errorf("%s: refused: %v", c.name, err)
		case !c.ok && !errors.Is(err, apierr.ErrConflict):
			t.Errorf("%s: should be a conflict, got %v", c.name, err)
		}
	}
	if _, err := o.UpdateRow(Args{"table": "tt", "column": "i", "rowid": 1.0, "value": "1", "expected": true}); !errors.Is(err, apierr.ErrBadArgument) {
		t.Errorf("an expected value of another type is a bad argument: %v", err)
	}
}

// Without an expected value (a caller older than this check) the edit is made as before.
func TestACellEditWithoutAnExpectedValueIsMadeAsBefore(t *testing.T) {
	o := newPlayerOps(t)
	if _, err := o.UpdateRow(Args{"table": "items", "column": "stack_size", "rowid": float64(mustOne(t, o, `select rowid r from items where id=10`)["r"].(int64)), "value": "3"}); err != nil {
		t.Fatal(err)
	}
	if stackOf(t, o, 10) != 3 {
		t.Fatal("not edited")
	}
}

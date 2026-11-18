package ops

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// #960: a decree is only in force because a faction won the term, so the term write refuses to leave an active decree set with no reigning faction. The refusal is judged on the row as
// the edit would leave it: the value the request names, else the stored one. Every test runs on a temp copy of the goal world (term 7 of 100 to 200, Atreides 1, Harkonnen 2).

func decreeTermOps(t *testing.T, faction, decree string) *Ops {
	t.Helper()
	o := factionOps(t)
	setup(t, o, `insert into landsraad_decrees(id, decree_name, disabled, weight) values (1, 'DecreeA', 0, 1), (2, 'DecreeB', 0, 1)`)
	setup(t, o, `update landsraad_decree_term set reigning_faction_id=`+faction+`, active_decree_id=`+decree+` where term_id=7`)
	return o
}

func termRow(t *testing.T, o *Ops) (faction, decree, end any) {
	t.Helper()
	r := mustOne(t, o, `select reigning_faction_id f, active_decree_id d, end_time e from landsraad_decree_term where term_id=7`)
	return r["f"], r["d"], r["e"]
}

func TestADecreeWriteWithAnEmptyFactionIsRefusedAndWritesNothing(t *testing.T) {
	for name, args := range map[string]Args{
		"decree and an empty string": {"active_decree_id": 1.0, "reigning_faction_id": ""},
		"decree and null":            {"active_decree_id": 2.0, "reigning_faction_id": nil},
		"decree as text and empty":   {"active_decree_id": "2", "reigning_faction_id": ""},
		"decree with the end time":   {"active_decree_id": 1.0, "reigning_faction_id": "", "end_time": 150.0},
	} {
		o := decreeTermOps(t, "null", "null") // a term with no winner and no decree
		f0, d0, e0 := termRow(t, o)
		queued := len(o.S.Pending())
		_, err := o.SetTerm(args)
		if !errors.Is(err, apierr.ErrBadArgument) || err == nil || !strings.Contains(err.Error(), "needs a reigning faction") {
			t.Errorf("%s: %v", name, err)
		}
		if f, d, e := termRow(t, o); f != f0 || d != d0 || e != e0 || len(o.S.Pending()) != queued {
			t.Errorf("%s: a refused write changed the term: %v %v %v (queued %d, was %d)", name, f, d, e, len(o.S.Pending()), queued)
		}
	}
}

func TestADecreeWriteThatLeavesTheStoredFactionEmptyIsRefusedToo(t *testing.T) {
	// the request names a decree and no faction, and the term has none: the row would hold a decree with no faction
	o := decreeTermOps(t, "null", "null")
	if _, err := o.SetTerm(Args{"active_decree_id": 1.0}); !errors.Is(err, apierr.ErrBadArgument) {
		t.Fatalf("no faction named, none stored: %v", err)
	}
	if f, d, _ := termRow(t, o); f != nil || d != nil {
		t.Fatalf("nothing was written: %v %v", f, d)
	}
	// clearing the faction while the decree stays set is refused, however it is asked
	o = decreeTermOps(t, "2", "1")
	for _, v := range []any{"", nil} {
		if _, err := o.SetTerm(Args{"reigning_faction_id": v}); !errors.Is(err, apierr.ErrBadArgument) {
			t.Fatalf("clearing the faction under a set decree (%#v): %v", v, err)
		}
	}
	if f, d, _ := termRow(t, o); f.(int64) != 2 || d.(int64) != 1 {
		t.Fatalf("nothing was written: %v %v", f, d)
	}
}

func TestADecreeWithAFactionStillWritesAndSoDoesLeavingTheStoredFactionAlone(t *testing.T) {
	o := decreeTermOps(t, "null", "null")
	if _, err := o.SetTerm(Args{"active_decree_id": 2.0, "reigning_faction_id": 1.0}); err != nil {
		t.Fatal(err)
	}
	if f, d, _ := termRow(t, o); f.(int64) != 1 || d.(int64) != 2 {
		t.Fatalf("decree and faction: %v %v", f, d)
	}
	// the request names the decree only and the term already has a faction: that faction stays (the page does this when the save holds a faction its list does not offer)
	if _, err := o.SetTerm(Args{"active_decree_id": 1.0}); err != nil {
		t.Fatalf("a decree only, with a stored faction: %v", err)
	}
	if f, d, _ := termRow(t, o); f.(int64) != 1 || d.(int64) != 1 {
		t.Fatalf("the faction stayed: %v %v", f, d)
	}
	// the other faction, the same decree
	if _, err := o.SetTerm(Args{"active_decree_id": 1.0, "reigning_faction_id": 2.0}); err != nil {
		t.Fatal(err)
	}
	if f, _, _ := termRow(t, o); f.(int64) != 2 {
		t.Fatalf("faction: %v", f)
	}
}

func TestClearingTheDecreeAndTheFactionTogetherStillWorksAndSoDoesClearingAFactionWithNoDecree(t *testing.T) {
	o := decreeTermOps(t, "1", "2")
	if _, err := o.SetTerm(Args{"active_decree_id": "", "reigning_faction_id": ""}); err != nil {
		t.Fatalf("a term with no winner and no decree: %v", err)
	}
	if f, d, _ := termRow(t, o); f != nil || d != nil {
		t.Fatalf("both cleared: %v %v", f, d)
	}
	o = decreeTermOps(t, "1", "null")
	if _, err := o.SetTerm(Args{"reigning_faction_id": nil}); err != nil {
		t.Fatalf("clearing the faction of a term with no decree: %v", err)
	}
	// the decree cleared alone keeps the faction
	o = decreeTermOps(t, "1", "2")
	if _, err := o.SetTerm(Args{"active_decree_id": ""}); err != nil {
		t.Fatal(err)
	}
	if f, d, _ := termRow(t, o); f.(int64) != 1 || d != nil {
		t.Fatalf("the decree cleared, the faction kept: %v %v", f, d)
	}
}

func TestARequestThatTouchesNeitherTheDecreeNorTheFactionIsNotJudged(t *testing.T) {
	// a save the game wrote with a decree and no faction keeps working for the other fields: the end time and the elected decree
	o := decreeTermOps(t, "null", "1")
	if _, err := o.SetTerm(Args{"end_time": 150.0}); err != nil {
		t.Fatalf("end time of a save that holds a decree and no faction: %v", err)
	}
	if _, err := o.SetTerm(Args{"elected_decree_id": 2.0}); err != nil {
		t.Fatalf("elected decree: %v", err)
	}
	if f, d, e := termRow(t, o); f != nil || d.(int64) != 1 || fmt.Sprint(e) != "150" {
		t.Fatalf("%v %v %v", f, d, e)
	}
}

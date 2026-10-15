package ops

import (
	"math"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A request can spell NaN or infinity as text, and a JSON number can be huge; none of that may reach a column (SEC-2, SEC-3 of the 2026-10-15 audit).
func TestArgsFloatRefusesNonFinite(t *testing.T) {
	for _, v := range []any{"NaN", "nan", "Inf", "+Inf", "-Inf", "inf", "Infinity", "1e999", math.NaN(), math.Inf(1), math.Inf(-1)} {
		if f, err := (Args{"k": v}).Float("k"); err == nil {
			t.Errorf("%v must be refused, got %v", v, f)
		}
	}
	for _, v := range []any{"abc", "", nil, true, []any{1.0}} {
		if _, err := (Args{"k": v}).Float("k"); err == nil {
			t.Errorf("%v is not a number", v)
		}
	}
	if f, err := (Args{"k": " 12.5 "}).Float("k"); err != nil || f != 12.5 {
		t.Errorf("a number typed with spaces is fine: %v %v", f, err)
	}
	if f, err := (Args{"k": -3.0}).Float("k"); err != nil || f != -3 {
		t.Errorf("%v %v", f, err)
	}
}

func TestArgsIntNeedsAnExactInteger(t *testing.T) {
	for _, v := range []any{1e19, -1e300, 9007199254740994.0, 2.5, "2.5", "NaN", math.Inf(1), "9223372036854775807"} {
		if n, err := (Args{"k": v}).Int("k"); err == nil {
			t.Errorf("%v must be refused, got %d", v, n)
		}
	}
	for _, c := range []struct {
		in   any
		want int64
	}{{7.0, 7}, {"42", 42}, {-5.0, -5}, {0.0, 0}, {9007199254740992.0, 9007199254740992}} {
		if n, err := (Args{"k": c.in}).Int("k"); err != nil || n != c.want {
			t.Errorf("%v: %d %v", c.in, n, err)
		}
	}
	// the ranged form keeps its message for an out-of-range whole number
	if _, err := (Args{"k": 7.0}).IntRange("k", 1, 5); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("%v", err)
	}
}

// A specialization row holds at most the XP of the last level and a level of at most 100 (DBA-7).
func TestSetSpecRefusesValuesTheTrackCannotHold(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	for _, a := range []Args{
		{"track_type": 0.0, "xp": 1e9, "level": 1.0},
		{"track_type": 0.0, "xp": 100.0, "level": "Inf"},
		{"track_type": 0.0, "xp": 100.0, "level": -500.0},
		{"track_type": 0.0, "xp": 100.0, "level": 101.0},
		{"track_type": 0.0, "xp": 100.5, "level": 1.0},
	} {
		if _, err := o.SetSpec(a); err == nil {
			t.Errorf("%v must be refused", a)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused edit must not change the save")
	}
	if _, err := o.SetSpec(Args{"track_type": 0.0, "xp": float64(maxSpecXP), "level": 100.0}); err != nil {
		t.Fatalf("the largest valid row is fine: %v", err)
	}
}

// A save with no character body: a move would change nothing, so it must not say it worked (ARCH-4).
func TestTeleportSaysSoWhenThereIsNoBodyToMove(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `delete from actors where class like '%PlayerCharacter%'`)}
	if _, err := o.Teleport(Args{"x": 1.0, "y": 2.0, "z": 3.0}); err == nil || !strings.Contains(err.Error(), "no character body") {
		t.Fatalf("a move of nothing must be refused: %v", err)
	}
	if o.S.Dirty() {
		t.Fatal("nothing was changed")
	}
}

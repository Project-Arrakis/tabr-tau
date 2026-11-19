package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// "A faction the Landsraad has" (#657, G-018): one rule for the list the page offers, the faction of the character and every write that takes a faction id. The world is goalWorld
// (the character in Harkonnen, 2; Atreides is 1; None is 3) with the Smuggler faction (4) the game's save also holds and its Landsraad never uses.

func factionOps(t *testing.T) *Ops {
	t.Helper()
	o := goalOps(t)
	setup(t, o, `insert or replace into factions(id, name) values (1, 'Atreides'), (2, 'Harkonnen'), (3, 'None'), (4, 'Smuggler')`)
	return o
}

func TestTheFactionHelperAcceptsTheLandsraadsFactionsAndRefusesTheRest(t *testing.T) {
	o := factionOps(t)
	for id, want := range map[int64]string{1: "Atreides", 2: "Harkonnen"} {
		got, err := landsraadFactionName(o.S.One, id)
		if err != nil || got != want {
			t.Errorf("faction %d: %q, %v, want %q", id, got, err, want)
		}
	}
	for _, id := range []int64{3, 4, 0, -1, 99} { // None, Smuggler, and ids that are not factions of the Landsraad
		if got, err := landsraadFactionName(o.S.One, id); !errors.Is(err, apierr.ErrBadArgument) || got != "" {
			t.Errorf("faction %d: %q, %v, want a bad argument", id, got, err)
		}
	}
	// the same answer inside an edit
	if _, err := o.S.Mutate("test", func(m *save.Mut) error {
		if _, err := landsraadFactionName(m.One, 4); !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("inside an edit, Smuggler: %v", err)
		}
		if n, err := landsraadFactionName(m.One, 1); err != nil || n != "Atreides" {
			t.Errorf("inside an edit, Atreides: %q %v", n, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestTheBoardOffersOnlyTheFactionsTheLandsraadHas(t *testing.T) {
	o := factionOps(t)
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	fs := res.(map[string]any)["factions"].([]save.Row)
	if len(fs) != 2 || fs[0]["id"].(int64) != 1 || fs[1]["id"].(int64) != 2 {
		t.Fatalf("the list the page offers holds Atreides and Harkonnen only: %v", fs)
	}
}

func TestTheBoardWriteRefusesAFactionTheLandsraadDoesNotHaveAndWritesNothing(t *testing.T) {
	o := factionOps(t)
	queued := len(o.S.Pending()) // the setup of the world
	for _, f := range []float64{3, 4, 99} {
		wantApplyError(t, o, applyArgs(house(176, Args{"complete_for": f})), apierr.ErrBadArgument)
		// one house of several: the whole request is refused
		wantApplyError(t, o, applyArgs(house(175, Args{"complete_for": 2.0}), house(176, Args{"complete_for": f})), apierr.ErrBadArgument)
	}
	if len(o.S.Pending()) != queued {
		t.Fatalf("nothing more is queued: %v", o.S.Pending())
	}
	// the real factions are accepted, for a house that is open
	for _, f := range []float64{1, 2} {
		o := factionOps(t)
		if m := mustApply(t, o, applyArgs(house(176, Args{"complete_for": f}))); m["completed"].(int64) != 1 {
			t.Fatalf("faction %v: %v", f, m)
		}
		if h := houseRow(t, o, 176); h["c"].(int64) != 1 || h["w"].(int64) != int64(f) {
			t.Fatalf("faction %v: %v", f, h)
		}
	}
}

func TestTheReigningFactionOfTheTermIsOneTheLandsraadHas(t *testing.T) {
	o := factionOps(t)
	before, queued := boardRows(t, o), len(o.S.Pending())
	for _, f := range []float64{3, 4, 99} {
		if _, err := o.SetTerm(Args{"reigning_faction_id": f}); !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("faction %v: %v", f, err)
		}
	}
	if len(o.S.Pending()) != queued || boardRows(t, o) != before {
		t.Fatal("a refused term wrote something")
	}
	for _, f := range []float64{1, 2} {
		if _, err := o.SetTerm(Args{"reigning_faction_id": f}); err != nil {
			t.Fatalf("faction %v: %v", f, err)
		}
		if r := mustOne(t, o, `select reigning_faction_id f from landsraad_decree_term where term_id=7`); r["f"].(int64) != int64(f) {
			t.Fatalf("faction %v: %v", f, r)
		}
	}
	if _, err := o.SetTerm(Args{"reigning_faction_id": nil}); err != nil { // clearing it is not a faction
		t.Fatal(err)
	}
	if r := mustOne(t, o, `select reigning_faction_id f from landsraad_decree_term where term_id=7`); r["f"] != nil {
		t.Fatalf("cleared: %v", r)
	}
}

func TestTheReigningFactionAlreadyHeldIsNotRefusedWhenTheRequestLeavesItAlone(t *testing.T) {
	o := factionOps(t)
	setup(t, o, `insert into landsraad_decrees(id, decree_name, disabled, weight) values (1, 'DecreeA', 0, 1), (2, 'DecreeB', 0, 1)`)
	setup(t, o, `update landsraad_decree_term set reigning_faction_id=4, active_decree_id=1 where term_id=7`) // a save that holds Smuggler as the reigning faction
	if _, err := o.SetTerm(Args{"active_decree_id": 2.0, "reigning_faction_id": 4.0}); err != nil {
		t.Fatalf("the same value as the row holds: %v", err)
	}
	if r := mustOne(t, o, `select reigning_faction_id f, active_decree_id d from landsraad_decree_term where term_id=7`); r["f"].(int64) != 4 || r["d"].(int64) != 2 {
		t.Fatalf("the decree changed and the faction stayed: %v", r)
	}
	// a change to another id the Landsraad does not have is still refused, and nothing is written
	queued := len(o.S.Pending())
	for _, f := range []float64{3, 99} {
		if _, err := o.SetTerm(Args{"reigning_faction_id": f}); !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("faction %v: %v", f, err)
		}
	}
	if len(o.S.Pending()) != queued {
		t.Fatal("a refused term wrote something")
	}
	// the real factions still work, and clearing the faction works together with the decree (the term here holds a decree, and a decree needs a faction: #960)
	for _, f := range []any{1.0, 2.0} {
		if _, err := o.SetTerm(Args{"reigning_faction_id": f}); err != nil {
			t.Fatalf("%v: %v", f, err)
		}
	}
	if _, err := o.SetTerm(Args{"reigning_faction_id": nil}); !errors.Is(err, apierr.ErrBadArgument) {
		t.Fatalf("clearing the faction under a set decree is refused: %v", err)
	}
	if _, err := o.SetTerm(Args{"active_decree_id": nil, "reigning_faction_id": nil}); err != nil {
		t.Fatalf("clearing both: %v", err)
	}
}

func TestTheFactionConditionIsBuiltFromTheIDs(t *testing.T) {
	if landsraadFactionCond != "f.id in (1, 2)" {
		t.Fatalf("condition: %q", landsraadFactionCond)
	}
	o := factionOps(t)
	rows, err := o.S.Query(`select f.id from factions f where ` + landsraadFactionCond + ` order by f.id`)
	if err != nil || len(rows) != len(landsraadFactionIDs) {
		t.Fatalf("%v %v", rows, err)
	}
	for i, r := range rows {
		if r["id"].(int64) != landsraadFactionIDs[i] || !isLandsraadFaction(r["id"].(int64)) {
			t.Fatalf("row %d: %v", i, r)
		}
	}
}

func TestACharacterInAFactionTheLandsraadDoesNotHaveHasNoLandsraadContribution(t *testing.T) {
	o := factionOps(t)
	setup(t, o, `update player_faction set faction_id=4 where actor_id=1`) // Smuggler
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	if ch := res.(map[string]any)["character"].(map[string]any); ch["faction_id"] != nil {
		t.Fatalf("a Smuggler has no Landsraad faction: %v", ch)
	}
	wantApplyError(t, o, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0})), apierr.ErrBadArgument)
	// the same for the character with no faction (None, 3): nothing is queued by the refusal
	o = factionOps(t)
	setup(t, o, `update player_faction set faction_id=3 where actor_id=1`)
	queued := len(o.S.Pending())
	wantApplyError(t, o, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0})), apierr.ErrBadArgument)
	if len(o.S.Pending()) != queued {
		t.Fatalf("nothing more is queued: %v", o.S.Pending())
	}
}

// #842: a character that is in a faction the Landsraad does not have (Smuggler, on a hand-edited save) is not told it is in no faction: the answer names the faction, and
// Neutral keeps its own sentence.
func TestAChangeOfACharacterInAFactionTheLandsraadDoesNotHaveNamesTheFaction(t *testing.T) {
	o := rewardOps(t, `insert or replace into factions(id, name) values (1, 'Atreides'), (2, 'Harkonnen'), (3, 'None'), (4, 'Smuggler');
		update player_faction set faction_id=4 where actor_id=1;`)
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0})))
	wantRefusal2(t, err, apierr.ErrBadArgument, `"Smuggler" faction, which the Landsraad does not have`, "assign Atreides or Harkonnen under Admin first")
	if strings.Contains(err.Error(), "no faction (Neutral)") {
		t.Fatalf("a faction the Landsraad does not have is not Neutral: %v", err)
	}
	n := rewardOps(t, `insert or replace into factions(id, name) values (1, 'Atreides'), (2, 'Harkonnen'), (3, 'None'), (4, 'Smuggler');
		update player_faction set faction_id=3 where actor_id=1;`)
	_, err = n.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0})))
	wantRefusal2(t, err, apierr.ErrBadArgument, "no faction (Neutral)")
}

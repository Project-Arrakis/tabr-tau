package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Two keystones per track in the save's map (named by track), the character owns one Combat keystone, and the save has one
// specialization row, number 2, with 3,000 XP.
const specWorld = `
insert into specialization_keystones_map(id,name) values
  (1,'Combat_CombatKeystone_A'),(2,'Combat_CombatKeystone_B'),(3,'Crafting_CraftingKeystone_A'),(4,'Crafting_CraftingKeystone_B'),
  (5,'Exploration_ExplorationKeystone_A'),(6,'Exploration_ExplorationKeystone_B'),(7,'Gathering_GatheringKeystone_A'),(8,'Gathering_GatheringKeystone_B'),
  (9,'Sabotage_SabotageKeystone_A'),(10,'Sabotage_SabotageKeystone_B');
insert into purchased_specialization_keystones(player_id,keystone_id) values (1,1);
insert into specialization_tracks(player_id,track_type,xp_amount,level) values (1,2,3000,19.0);`

func specOps(t *testing.T) *Ops { t.Helper(); return &Ops{S: testsave.PlayerWithSQL(t, specWorld)} }

func rowsOf(r any) map[string]SpecRow {
	out := map[string]SpecRow{}
	for _, x := range r.(map[string]any)["rows"].([]SpecRow) {
		out[x.Track] = x
	}
	return out
}

func TestSpecsListsEveryTrackLikeTheConsoleEvenWithNoXp(t *testing.T) {
	o := specOps(t)
	r, err := o.Specs(nil) // no track numbers learned yet
	if err != nil {
		t.Fatal(err)
	}
	by := rowsOf(r)
	if len(by) != 5 {
		t.Fatalf("all five tracks, as in the console: %v", by)
	}
	if c := by["Combat"]; c.KeystoneOwned != 1 || c.KeystoneTotal != 2 || c.Granted || c.XP != 0 || c.Number != nil {
		t.Fatalf("combat: %+v", c)
	}
	if by["Crafting"].KeystoneTotal != 2 || by["Sabotage"].KeystoneTotal != 2 {
		t.Fatalf("keystones are counted by the track in their name: %+v %+v", by["Crafting"], by["Sabotage"])
	}
	un := r.(map[string]any)["unassigned"].([]SpecRaw)
	if len(un) != 1 || un[0].Number != 2 || un[0].XP != 3000 {
		t.Fatalf("the row whose track is not known is offered for assignment: %+v", un)
	}
}

func TestSpecsShowsXpOnceTheTrackNumberIsKnown(t *testing.T) {
	o := specOps(t)
	r, _ := o.Specs(map[string]int64{"Gathering": 2})
	g := rowsOf(r)["Gathering"]
	if g.XP != 3000 || g.Level != 19 || g.Number == nil || *g.Number != 2 {
		t.Fatalf("%+v", g)
	}
	if un := r.(map[string]any)["unassigned"].([]SpecRaw); len(un) != 0 {
		t.Fatalf("nothing is left unassigned: %+v", un)
	}
}

func TestAddSpecXPGrantMaxAndResetNeedTheNumberAndStayInRange(t *testing.T) {
	o := specOps(t)
	nums := map[string]int64{"Gathering": 2, "Combat": 0}
	if _, err := o.AddSpecXP(nil, Args{"track": "Gathering", "amount": 100.0}); err == nil || !strings.Contains(err.Error(), "not known yet") {
		t.Fatalf("without the number the message says how to learn it: %v", err)
	}
	r, err := o.AddSpecXP(nums, Args{"track": "Gathering", "amount": 1000.0})
	if err != nil || r.(map[string]any)["after"].(int64) != 4000 {
		t.Fatalf("%v %v", r, err)
	}
	row := mustOne(t, o, `select xp_amount x, level l from specialization_tracks where player_id=1 and track_type=2`)
	if row["x"].(int64) != 4000 || row["l"].(float64) < 22 || row["l"].(float64) > 23 {
		t.Fatalf("the level follows the XP: %v", row)
	}
	// a track with no row yet gets one
	if _, err := o.AddSpecXP(nums, Args{"track": "Combat", "amount": 205.0}); err != nil {
		t.Fatal(err)
	}
	if mustOne(t, o, `select level l from specialization_tracks where player_id=1 and track_type=0`)["l"].(float64) != 2 {
		t.Fatal("205 XP is level 2 on the console's table")
	}
	// clamped at both ends
	r, _ = o.AddSpecXP(nums, Args{"track": "Gathering", "amount": -44182.0})
	if r.(map[string]any)["after"].(int64) != 0 {
		t.Fatalf("not below 0: %v", r)
	}
	r, _ = o.AddSpecXP(nums, Args{"track": "Gathering", "amount": 44182.0})
	if r.(map[string]any)["after"].(int64) != 44182 {
		t.Fatalf("not above the last level: %v", r)
	}
	if _, err := o.GrantMaxSpec(nums, Args{"track": "Combat"}); err != nil {
		t.Fatal(err)
	}
	m := mustOne(t, o, `select xp_amount x, level l from specialization_tracks where player_id=1 and track_type=0`)
	if m["x"].(int64) != 44182 || m["l"].(float64) != 100 {
		t.Fatalf("grant max is the console's 44182 XP at level 100: %v", m)
	}
	if _, err := o.ResetSpec(nums, Args{"track": "Combat"}); err != nil {
		t.Fatal(err)
	}
	if mustOne(t, o, `select count(*) c from specialization_tracks where player_id=1 and track_type=0`)["c"].(int64) != 0 {
		t.Fatal("reset removes the row")
	}
	if mustOne(t, o, `select count(*) c from purchased_specialization_keystones`)["c"].(int64) != 1 {
		t.Fatal("reset leaves the keystones, as in the console")
	}
	for name, a := range map[string]Args{"zero": {"track": "Gathering", "amount": 0.0}, "huge": {"track": "Gathering", "amount": 99999.0}, "no track": {"amount": 5.0}, "unknown track": {"track": "Nope", "amount": 5.0}} {
		if _, err := o.AddSpecXP(nums, a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

func TestKeystoneGrantAndReset(t *testing.T) {
	o := specOps(t)
	r, err := o.GrantAllKeystones()
	if err != nil || r.(map[string]any)["granted"].(int64) != 9 {
		t.Fatalf("nine more of the ten: %v %v", r, err)
	}
	if r, _ := o.GrantAllKeystones(); r.(map[string]any)["granted"].(int64) != 0 {
		t.Fatal("a second grant adds nothing")
	}
	by := rowsOf(mustSpecs(t, o))
	if !by["Combat"].Granted || by["Sabotage"].KeystoneOwned != 2 {
		t.Fatalf("%+v", by)
	}
	r, _ = o.ResetAllKeystones()
	if r.(map[string]any)["removed"].(int64) != 10 {
		t.Fatalf("%v", r)
	}
}

func mustSpecs(t *testing.T, o *Ops) any {
	t.Helper()
	r, err := o.Specs(nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCheckAssign(t *testing.T) {
	next, err := CheckAssign(map[string]int64{"Combat": 0}, "Crafting", 3)
	if err != nil || next["Crafting"] != 3 || next["Combat"] != 0 {
		t.Fatalf("%v %v", next, err)
	}
	if _, err := CheckAssign(map[string]int64{"Combat": 0}, "Crafting", 0); err == nil {
		t.Fatal("a number two tracks share is refused")
	}
	if n, err := CheckAssign(map[string]int64{"Combat": 0}, "Combat", 0); err != nil || n["Combat"] != 0 {
		t.Fatal("assigning the same track the same number again is fine")
	}
	for _, bad := range []struct {
		track string
		n     int64
	}{{"Nope", 1}, {"Combat", -1}, {"Combat", 256}} {
		if _, err := CheckAssign(nil, bad.track, bad.n); err == nil {
			t.Errorf("%v must be refused", bad)
		}
	}
}

func TestSpecLevelFollowsTheConsolesTable(t *testing.T) {
	for xp, want := range map[int64]float64{0: 0, 100: 1, 205: 2, 44182: 100} {
		if got := specLevel(xp); got != want {
			t.Errorf("%d XP: level %v, want %v", xp, got, want)
		}
	}
	if l := specLevel(150); l <= 1 || l >= 2 {
		t.Errorf("the level is fractional between levels: %v", l)
	}
}

package ops

import (
	"errors"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The character's own contribution (#380). The fixture is shaped like the save it was settled on: the character is player_state 1 in Harkonnen; the other ids of the
// contribution tables are simulated guild actors; term 7 is the current term and term 6 is an earlier one whose board the save still holds. House Maros (task 174) has the
// character at 7,850 (above the goal and above the last tier) with the five tiers delivered; House Kenola (task 175) has the character at 1,300 and a simulated guild at 3,000.
const mineWorld = `
insert or ignore into factions(id, name) values (1, 'Atreides'), (2, 'Harkonnen'), (3, 'None');
insert into player_faction(actor_id, faction_id, utc_time_faction_change) values (1, 2, 1);
insert into actors(id, class) values (50, 'LandsraadSimulatedGuild'), (51, 'LandsraadSimulatedGuild');
insert into landsraad_decree_term(term_id, start_time, end_time) values (6, 0, 100), (7, 100, 200);
insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount, completed) values
  (149, 6, 0, 'DA_HouseMaros', 4000, 1), (174, 7, 0, 'DA_HouseMaros', 4000, 1), (175, 7, 1, 'DA_HouseKenola', 4000, 0);
insert into landsraad_task_rewards(task_id, threshold, template_id, amount) values
  (149, 1200, 'T6IndustrialPump', 25),
  (174, 1200, 'T6IndustrialPump', 25), (174, 2400, 'Schematic.A', 1), (174, 3600, 'T6LightCalliberCompressor', 50), (174, 4800, 'Schematic.B', 1), (174, 6000, 'Swatch.Maros', 1),
  (175, 1200, 'SolarisCoin', 10000), (175, 2400, 'Schematic.K', 1), (175, 3600, 'T6RangeFinder', 50), (175, 4800, 'Schematic.L', 1), (175, 6000, 'Swatch.Kenola', 1);
insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values
  (1, 2, 174, 7850), (50, 2, 174, 1000), (1, 2, 175, 1300), (50, 1, 175, 3000), (1, 2, 149, 500);
insert into landsraad_task_faction_contributions(faction_id, task_id, amount) values (2, 174, 4250), (2, 175, 1000);
insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values
  (1, 'DA_HouseMaros', 25, 'T6IndustrialPump', 150), (1, 'DA_HouseMaros', 1, 'Schematic.A', 150), (1, 'DA_HouseMaros', 50, 'T6LightCalliberCompressor', 150),
  (1, 'DA_HouseMaros', 1, 'Schematic.B', 150), (1, 'DA_HouseMaros', 1, 'Swatch.Maros', 150),
  (50, 'DA_HouseKenola', 10000, 'SolarisCoin', 150);
`

func mineOps(t *testing.T, extra string) *Ops {
	return &Ops{S: testsave.PlayerWithSQL(t, mineWorld+extra)}
}

func taskByID(t *testing.T, o *Ops, id int64) map[string]any {
	t.Helper()
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range res.(map[string]any)["tasks"].([]map[string]any) {
		if x["id"].(int64) == id {
			return x
		}
	}
	return nil
}

func TestLandsraadKnowsWhichRecordedContributionIsTheCharacters(t *testing.T) {
	o := mineOps(t, "")
	res, _ := o.Landsraad()
	m := res.(map[string]any)
	ch := m["character"].(map[string]any)
	if ch["player_id"].(int64) != 1 || ch["faction_id"].(int64) != 2 {
		t.Fatalf("character: %v", ch)
	}
	if n := len(m["tasks"].([]map[string]any)); n != 2 {
		t.Fatalf("only the current term's two houses are returned, got %d", n)
	}
	if taskByID(t, o, 149) != nil {
		t.Fatal("a house of an earlier term is not on the board")
	}
	maros := taskByID(t, o, 174)
	if got := maros["my_contribution"].(float64); got != 7850 {
		t.Fatalf("Maros contribution = %v, want 7850 (above the goal, whole)", got)
	}
	var mine, others int
	for _, c := range maros["player_contributions"].([]map[string]any) {
		if c["mine"] == true {
			mine++
			if c["amount"].(float64) != 7850 {
				t.Fatalf("the row marked mine: %v", c)
			}
		} else {
			others++
		}
	}
	if mine != 1 || others != 1 {
		t.Fatalf("one row is the character's and one is a guild's: %d/%d", mine, others)
	}
	kenola := taskByID(t, o, 175)
	if got := kenola["my_contribution"].(float64); got != 1300 {
		t.Fatalf("Kenola contribution = %v, want the character's 1300, not the guild's 3000", got)
	}
}

func TestLandsraadSaysWhichTiersAreReachedAndDelivered(t *testing.T) {
	o := mineOps(t, "")
	flags := func(task map[string]any) (reached, delivered []bool) {
		for _, x := range task["rewards"].([]any) {
			r := x.(save.Row)
			reached, delivered = append(reached, r["reached"].(bool)), append(delivered, r["delivered"].(bool))
		}
		return
	}
	r, d := flags(taskByID(t, o, 174))
	for i := range r {
		if !r[i] || !d[i] {
			t.Fatalf("Maros: all five tiers are reached and delivered: reached %v delivered %v", r, d)
		}
	}
	r, d = flags(taskByID(t, o, 175))
	wantR := []bool{true, false, false, false, false}
	for i := range r {
		if r[i] != wantR[i] {
			t.Fatalf("Kenola reached = %v, want %v", r, wantR)
		}
		if d[i] {
			t.Fatalf("Kenola: the 10,000 Solari delivery in the save belongs to a guild, not to the character: delivered %v", d)
		}
	}
}

func TestADeliveryFromBeforeThisTermIsNotCountedAsThisTermsTier(t *testing.T) {
	// the same item delivered in the previous term (last_updated 50, the term began at 100)
	o := mineOps(t, `update landsraad_house_rewards set last_updated=50 where player_id=1;`)
	for _, x := range taskByID(t, o, 174)["rewards"].([]any) {
		if x.(save.Row)["delivered"].(bool) {
			t.Fatalf("delivered before the term began is not this term's: %v", x)
		}
	}
}

func contribution(t *testing.T, o *Ops, player, task int64) float64 {
	t.Helper()
	return mustOne(t, o, `select amount a from landsraad_task_player_contributions where player_id=? and task_id=?`, player, task)["a"].(float64)
}

func TestFillMyContributionRaisesToTheTierAndTouchesNothingElse(t *testing.T) {
	o := mineOps(t, "")
	rewardsBefore := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`)
	progressBefore := mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_task_faction_contributions`)
	got, err := o.FillMyContribution(Args{"task_id": 175.0, "tier": 6000.0})
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if m["before"].(float64) != 1300 || m["after"].(float64) != 6000 || m["changed"] != true {
		t.Fatalf("answer: %v", m)
	}
	if c := contribution(t, o, 1, 175); c != 6000 {
		t.Fatalf("the character's row = %v, want 6000", c)
	}
	if c := contribution(t, o, 50, 175); c != 3000 {
		t.Fatalf("a guild's row was changed: %v", c)
	}
	if c := contribution(t, o, 1, 174); c != 7850 {
		t.Fatalf("another house was changed: %v", c)
	}
	if mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_house_rewards`)["s"] != rewardsBefore["s"] ||
		mustOne(t, o, `select count(*) n, sum(amount) s from landsraad_task_faction_contributions`)["s"] != progressBefore["s"] {
		t.Fatal("the rewards and the faction's progress must not be written")
	}
	if n := len(o.S.Pending()); n != 1 {
		t.Fatalf("one edit, got %d", n)
	}
	assertDBSound(t, o)
}

func TestFillMyContributionNeverLowersAndAnAmountAboveTheGoalIsKept(t *testing.T) {
	o := mineOps(t, "")
	got, err := o.FillMyContribution(Args{"task_id": 174.0, "tier": 1200.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"] != false || m["after"].(float64) != 7850 {
		t.Fatalf("a contribution above the tier stays: %v", m)
	}
	if c := contribution(t, o, 1, 174); c != 7850 {
		t.Fatalf("lowered to %v", c)
	}
	if len(o.S.Pending()) != 0 {
		t.Fatalf("nothing changed, so no edit is recorded: %v", o.S.Pending())
	}
}

func TestFillMyContributionCreatesTheRowWhenThereIsNone(t *testing.T) {
	o := mineOps(t, `delete from landsraad_task_player_contributions where player_id=1 and task_id=175;`)
	if _, err := o.FillMyContribution(Args{"task_id": 175.0, "tier": 3600.0}); err != nil {
		t.Fatal(err)
	}
	r := mustOne(t, o, `select faction_id f, amount a from landsraad_task_player_contributions where player_id=1 and task_id=175`)
	if r["f"].(int64) != 2 || r["a"].(float64) != 3600 {
		t.Fatalf("row: %v", r)
	}
	assertDBSound(t, o)
}

func TestFillMyContributionAlsoFillsTheFactionGoalOnlyWhenAsked(t *testing.T) {
	o := mineOps(t, "")
	if _, err := o.FillMyContribution(Args{"task_id": 175.0, "tier": 2400.0}); err != nil {
		t.Fatal(err)
	}
	if v := mustOne(t, o, `select amount a from landsraad_task_faction_contributions where task_id=175`)["a"].(float64); v != 1000 {
		t.Fatalf("the faction's progress moved without being asked: %v", v)
	}
	got, err := o.FillMyContribution(Args{"task_id": 175.0, "tier": 2400.0, "with_faction": true})
	if err != nil {
		t.Fatal(err)
	}
	if got.(map[string]any)["faction_changed"] != true {
		t.Fatalf("answer: %v", got)
	}
	if v := mustOne(t, o, `select amount a from landsraad_task_faction_contributions where task_id=175 and faction_id=2`)["a"].(float64); v != 4000 {
		t.Fatalf("the faction's progress = %v, want the goal 4000", v)
	}
	// 4250 on Maros is above its goal and stays
	if _, err := o.FillMyContribution(Args{"task_id": 174.0, "tier": 1200.0, "with_faction": true}); err != nil {
		t.Fatal(err)
	}
	if v := mustOne(t, o, `select amount a from landsraad_task_faction_contributions where task_id=174`)["a"].(float64); v != 4250 {
		t.Fatalf("an amount above the goal was lowered: %v", v)
	}
}

func TestFillMyContributionRefusals(t *testing.T) {
	o := mineOps(t, "")
	for name, a := range map[string]Args{
		"a tier the house does not have": {"task_id": 175.0, "tier": 1500.0},
		"a fraction of a tier":           {"task_id": 175.0, "tier": 1200.5},
		"no tier":                        {"task_id": 175.0},
		"zero":                           {"task_id": 175.0, "tier": 0.0},
		"not a flag":                     {"task_id": 175.0, "tier": 1200.0, "with_faction": "yes"},
		"no task":                        {"tier": 1200.0},
	} {
		if _, err := o.FillMyContribution(a); !errors.Is(err, apierr.ErrBadArgument) {
			t.Errorf("%s: err = %v, want a bad argument", name, err)
		}
	}
	if _, err := o.FillMyContribution(Args{"task_id": 999.0, "tier": 1200.0}); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("unknown task: %v", err)
	}
	if _, err := o.FillMyContribution(Args{"task_id": 149.0, "tier": 1200.0}); !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("a house of an earlier term: %v", err)
	}
	if c := contribution(t, o, 1, 149); c != 500 {
		t.Errorf("the earlier term's row was written: %v", c)
	}
	if len(o.S.Pending()) != 0 {
		t.Errorf("a refused fill leaves no edit: %v", o.S.Pending())
	}
}

func TestFillMyContributionNeedsAFaction(t *testing.T) {
	o := mineOps(t, `update player_faction set faction_id=3 where actor_id=1;`)
	if _, err := o.FillMyContribution(Args{"task_id": 175.0, "tier": 1200.0}); !errors.Is(err, apierr.ErrBadArgument) {
		t.Fatalf("a Neutral character has no faction to contribute for: %v", err)
	}
	if c := contribution(t, o, 1, 175); c != 1300 {
		t.Fatalf("written anyway: %v", c)
	}
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	if res.(map[string]any)["character"].(map[string]any)["faction_id"] != nil {
		t.Fatalf("no faction is shown as none: %v", res.(map[string]any)["character"])
	}
}

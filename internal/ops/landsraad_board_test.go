package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// What the Landsraad board reads beyond the first page (#599, #609): the state of every tier, the swatch rule for the highest tier, the place of a house and the earlier terms.
// The fixture is mineWorld (landsraad_mine_test.go): the character is player 1 in Harkonnen, term 7 is the current term (clock 100 to 200), term 6 an earlier one.

const swatchWorld = `
insert into landsraad_one_time_house_rewards(house_name, threshold, template_id, amount) values
  ('DA_HouseMaros', 6000, 'Maros_Placeables_Swatch', 1), ('DA_HouseKenola', 6000, 'Kenola_Placeables_Swatch', 1);
`

func newerOps(t *testing.T, extra string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithNewerSQL(t, mineWorld+swatchWorld+extra)}
}

func tierOf(t *testing.T, task map[string]any, threshold int64) save.Row {
	t.Helper()
	for _, x := range task["rewards"].([]any) {
		r := x.(save.Row)
		if r["threshold"].(int64) == threshold {
			return r
		}
	}
	t.Fatalf("no tier %d on %v", threshold, task["house_name"])
	return nil
}

func TestBoardTierStates(t *testing.T) {
	// Maros: the character is at 7,850, so all five tiers are reached. The delivery rows hold amounts above 0 (waiting), and two are collected (amount 0, written in this term).
	o := newerOps(t, `update landsraad_house_rewards set amount=0 where player_id=1 and template_id in ('T6IndustrialPump','Schematic.A');
		insert into landsraad_task_rewards(task_id, threshold, template_id, amount) values (174, 7000, 'ExtraItem', 1);`)
	maros := taskByID(t, o, 174)
	want := map[int64]string{1200: "delivered", 2400: "delivered", 3600: "waiting", 4800: "waiting", 6000: "waiting", 7000: "no_row"}
	for th, st := range want {
		if got := tierOf(t, maros, th)["reward_state"]; got != st {
			t.Errorf("Maros tier %d: state %v, want %s", th, got, st)
		}
	}
	// Kenola: 1,300 is the character's own row, so tier 1 is reached; there is no delivery row for the character (the 10,000 row belongs to a simulated guild).
	kenola := taskByID(t, o, 175)
	if got := tierOf(t, kenola, 1200)["reward_state"]; got != "no_row" {
		t.Errorf("Kenola tier 1200: %v, want no_row", got)
	}
	if got := tierOf(t, kenola, 2400)["reward_state"]; got != "not_reached" {
		t.Errorf("Kenola tier 2400: %v, want not_reached", got)
	}
}

func TestBoardDeliveryOfAnEarlierTermIsNotThisTerms(t *testing.T) {
	// a collected row (amount 0) written before this term started does not say the tier of this term was delivered
	o := newerOps(t, `update landsraad_house_rewards set amount=0, last_updated=10 where player_id=1 and template_id='T6IndustrialPump'`)
	if got := tierOf(t, taskByID(t, o, 174), 1200)["reward_state"]; got != "no_row" {
		t.Fatalf("state = %v, want no_row", got)
	}
}

func TestBoardSwatchRuleForTheTopTier(t *testing.T) {
	// the task's own top row is 75 Diamodine blade parts; the swatch is the house's one-time reward
	own := `update landsraad_task_rewards set template_id='T6DiamodineBladeParts', amount=75 where task_id in (174,175) and threshold=6000;`
	t.Run("not held: the swatch", func(t *testing.T) {
		o := newerOps(t, own)
		k := taskByID(t, o, 175)
		top := tierOf(t, k, 6000)
		if top["shown_template_id"] != "Kenola_Placeables_Swatch" || top["shown_amount"].(int64) != 1 || top["swatch"] != true {
			t.Fatalf("top tier = %v", top)
		}
		if k["swatch_held"] != false || k["swatch_template_id"] != "Kenola_Placeables_Swatch" {
			t.Fatalf("swatch facts: held %v template %v", k["swatch_held"], k["swatch_template_id"])
		}
		if low := tierOf(t, k, 3600); low["shown_template_id"] != "T6RangeFinder" || low["swatch"] != false {
			t.Fatalf("a lower tier shows its own item: %v", low)
		}
	})
	t.Run("held since an earlier term: the task's own item", func(t *testing.T) {
		o := newerOps(t, own+`insert into landsraad_player_one_time_house_rewards(player_id, house_name, template_id, term_id) values (1, 'DA_HouseMaros', 'Maros_Placeables_Swatch', 6);`)
		m := taskByID(t, o, 174)
		top := tierOf(t, m, 6000)
		if top["shown_template_id"] != "T6DiamodineBladeParts" || top["shown_amount"].(int64) != 75 || top["swatch"] != false || m["swatch_held"] != true {
			t.Fatalf("top tier = %v held %v", top, m["swatch_held"])
		}
		if top["reward_state"] != "no_row" { // the 75 items were not delivered: there is no row for them
			t.Fatalf("state = %v", top["reward_state"])
		}
	})
	t.Run("won in this term: still the swatch", func(t *testing.T) {
		o := newerOps(t, own+`insert into landsraad_player_one_time_house_rewards(player_id, house_name, template_id, term_id) values (1, 'DA_HouseMaros', 'Maros_Placeables_Swatch', 7);
			insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseMaros', 1, 'Maros_Placeables_Swatch', 150);`)
		top := tierOf(t, taskByID(t, o, 174), 6000)
		if top["shown_template_id"] != "Maros_Placeables_Swatch" || top["swatch"] != true || top["reward_state"] != "waiting" {
			t.Fatalf("top tier = %v", top)
		}
	})
}

func TestBoardOlderLayoutHasNoSwatchTables(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQLIn(t, testsave.LayoutMapName, mineWorld)}
	m := taskByID(t, o, 174)
	if m["swatch_held"] != false {
		t.Fatalf("held = %v", m["swatch_held"])
	}
	if _, ok := m["swatch_template_id"]; ok {
		t.Fatal("a save without the swatch tables names no swatch")
	}
	if top := tierOf(t, m, 6000); top["swatch"] != false || top["shown_template_id"] != "Swatch.Maros" {
		t.Fatalf("top tier = %v", top)
	}
}

func TestBoardLocationIsAPlaceholderUntilTheTableIsBuilt(t *testing.T) {
	o := newerOps(t, "")
	loc := taskByID(t, o, 174)["location"].(map[string]any)
	if loc["known"] != false || len(loc) != 1 {
		t.Fatalf("location = %v: no position may be invented", loc)
	}
	houseLocations["DA_HouseMaros"] = houseLocation{Map: "Hagga Basin", Area: "Sample Area", Place: "somewhere", X: .25, Y: .5}
	defer delete(houseLocations, "DA_HouseMaros")
	loc = taskByID(t, o, 174)["location"].(map[string]any)
	if loc["known"] != true || loc["area"] != "Sample Area" || loc["x"].(float64) != .25 {
		t.Fatalf("location = %v", loc)
	}
}

func TestBoardEarlierTerms(t *testing.T) {
	o := newerOps(t, "")
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if n := len(m["terms"].([]save.Row)); n != 2 {
		t.Fatalf("terms = %d, want 2", n)
	}
	earlier := m["earlier_terms"].([]map[string]any)
	if len(earlier) != 1 || earlier[0]["term_id"].(int64) != 6 {
		t.Fatalf("earlier terms = %v", earlier)
	}
	h := earlier[0]["houses"].([]map[string]any)
	if len(h) != 1 || h[0]["house_name"] != "DA_HouseMaros" || h[0]["my_contribution"].(float64) != 500 || h[0]["tiers_reached"].(int) != 0 || h[0]["tier_count"].(int) != 1 {
		t.Fatalf("houses of term 6 = %v", h)
	}
}

func TestBoardOnlyTheCurrentTermIsAMissingHistory(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert or ignore into factions(id, name) values (1, 'Atreides'), (2, 'Harkonnen'), (3, 'None');
		insert into player_faction(actor_id, faction_id, utc_time_faction_change) values (1, 2, 1);
		insert into landsraad_decree_term(term_id, start_time, end_time) values (7, 100, 200);`)}
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	if e := res.(map[string]any)["earlier_terms"].([]map[string]any); len(e) != 0 {
		t.Fatalf("earlier terms = %v", e)
	}
}

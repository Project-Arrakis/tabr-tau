package ops

import (
	"testing"

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
	o := mineOps(t, marosEvents+kenolaEvents) // the events cover Maros's tiers, as the game leaves them
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
	o := mineOps(t, marosEvents+`update landsraad_house_rewards set last_updated=50 where player_id=1;`)
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

// The board names a contributor that is a simulated guild (#913): the row of a guild carries its identifier from landsraad_simulated_guilds, the character's own row and a row of an actor with no
// guild entry carry none.
func TestLandsraadNamesTheGuildsThatContributed(t *testing.T) {
	o := mineOps(t, `insert into landsraad_simulated_guilds(guild_actor_id, guild_identifier) values (50, 'red_hawk_corps');`)
	for _, c := range taskByID(t, o, 174)["player_contributions"].([]map[string]any) {
		switch c["player_id"].(int64) {
		case 50:
			if c["guild"] != "red_hawk_corps" {
				t.Fatalf("the guild row has no name: %v", c)
			}
		case 1:
			if _, has := c["guild"]; has {
				t.Fatalf("the character's own row carries a guild name: %v", c)
			}
		}
	}
	// no entry for the actor: no key
	o = mineOps(t, "")
	for _, c := range taskByID(t, o, 174)["player_contributions"].([]map[string]any) {
		if _, has := c["guild"]; has {
			t.Fatalf("a row with no guild entry carries a name: %v", c)
		}
	}
}

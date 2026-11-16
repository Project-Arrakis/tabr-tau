package ops

import (
	"fmt"
	"testing"
)

// Rewards are delivered per tier, and the board reads them the same way (#656). World: rewardOps (landsraad_rewards_test.go): House Kenola (task 175) has the tiers 1,200 Solari
// 10,000 / 2,400 Schematic.K / 3,600 T6RangeFinder 50 / 4,800 Schematic.L / 6,000 its swatch; the character is player 1, the term runs from 100 to 200, the play clock is 150, and an
// earlier term 6 runs from 0 to 100. These cases make Solari the reward of two more tiers (3,600: 500, 4,800: 5,000) and start the character at 0 with no event.

const solariTiers = `update landsraad_task_rewards set template_id='SolarisCoin', amount=500 where task_id=175 and threshold=3600;
	update landsraad_task_rewards set template_id='SolarisCoin', amount=5000 where task_id=175 and threshold=4800;
	update landsraad_task_player_contributions set amount=0 where player_id=1 and task_id=175;`

func solari(t *testing.T, o *Ops) string {
	t.Helper()
	return rows(t, o, `select amount, last_updated from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola' and template_id='SolarisCoin'`)
}

func raise(t *testing.T, o *Ops, from, to float64) map[string]any {
	t.Helper()
	return applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": from, "mine_to": to}))
}

// An item that repeats across tiers pays at every tier: the amounts add to a waiting row, and a collected row (amount 0) is set to the new tier's amount only.
func TestTiersARepeatedItemPaysAtEveryTier(t *testing.T) {
	o := rewardOps(t, solariTiers)
	if m := raise(t, o, 0, 1200); m["reward_rows"].(int64) != 1 || m["rewards_already_given"].(int64) != 0 {
		t.Fatalf("first tier: %v", m)
	}
	if got := solari(t, o); got != "map[amount:10000 last_updated:150]\n" {
		t.Fatalf("tier 1,200: %s", got)
	}
	// collected at the representative: the amount is 0 and the time moves on
	setup(t, o, `update landsraad_house_rewards set amount=0, last_updated=160 where player_id=1 and template_id='SolarisCoin'`)
	if m := raise(t, o, 1200, 3600); m["reward_rows"].(int64) != 2 || m["rewards_already_given"].(int64) != 0 { // 2,400 Schematic.K and 3,600 Solari
		t.Fatalf("second raise: %v", m)
	}
	if got := solari(t, o); got != "map[amount:500 last_updated:150]\n" {
		t.Fatalf("the collected Solari row is set to the new tier's 500, not to 10,500: %s", got)
	}
	// not collected: the next tier adds to what waits
	if m := raise(t, o, 3600, 4800); m["reward_rows"].(int64) != 1 {
		t.Fatalf("third raise: %v", m)
	}
	if got := solari(t, o); got != "map[amount:5500 last_updated:150]\n" {
		t.Fatalf("a waiting amount is added to: %s", got)
	}
}

// A reward of an earlier term, waiting or collected later, is not this term's tier: the raise pays it and the row keeps what waits.
func TestTiersARewardOfAnEarlierTermDoesNotBlockThisTerm(t *testing.T) {
	// collected in term 6 (amount 0, written at 60) and a waiting 20,000 of terms 2 and 5
	for name, c := range map[string]struct{ row, want string }{
		"collected": {`insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 0, 'SolarisCoin', 60);`, "map[amount:10000 last_updated:150]\n"},
		"waiting":   {`insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 20000, 'SolarisCoin', 60);`, "map[amount:30000 last_updated:150]\n"},
	} {
		o := rewardOps(t, solariTiers+c.row)
		if m := raise(t, o, 0, 1200); m["reward_rows"].(int64) != 1 || m["rewards_already_given"].(int64) != 0 {
			t.Fatalf("%s: answer %v", name, m)
		}
		if got := solari(t, o); got != c.want {
			t.Errorf("%s: %s", name, got)
		}
	}
}

// A house raised twice: the same tier again pays nothing, a higher tier adds only the tiers above. The editor writes no event (#655), so a removal followed by the same raise pays
// the tiers above the lower contribution again; only the character's own events, written by the game, cover ground for good (TestTiersTheGamesEventsCoverATierForGood).
func TestTiersARaisedHouseIsPaidOncePerTierWhileTheContributionStands(t *testing.T) {
	o := rewardOps(t, solariTiers)
	raise(t, o, 0, 3600)
	// the same contribution again: nothing to do
	if m := raise(t, o, 3600, 3600); m["changed"].(int64) != 0 {
		t.Fatalf("the same tier twice changed something: %v", m)
	}
	// removed and given again: nothing remembers the tiers, the two tiers above 1,200 are paid again
	applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": 3600.0, "mine_to": 1200.0, "lower": true}))
	if m := raise(t, o, 1200, 3600); m["reward_rows"].(int64) != 2 || m["rewards_already_given"].(int64) != 0 {
		t.Fatalf("the tiers above 1,200 were not paid again: %v", m)
	}
	if solari(t, o) != "map[amount:11000 last_updated:150]\n" {
		t.Fatalf("Solari after the repeat: %s", solari(t, o))
	}
	// a higher tier adds only the new tier
	if m := raise(t, o, 3600, 4800); m["reward_rows"].(int64) != 1 || m["rewards_already_given"].(int64) != 0 {
		t.Fatalf("higher tier: %v", m)
	}
	if got := solari(t, o); got != "map[amount:16000 last_updated:150]\n" {
		t.Fatalf("only the 4,800 tier of 5,000 is added: %s", got)
	}
}

// The character's own events, written by the game, cover a tier for good: lowered and raised again, the tiers they cover are not paid again, and the editor adds no event.
func TestTiersTheGamesEventsCoverATierForGood(t *testing.T) {
	o := rewardOps(t, solariTiers+`insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (5, 2, 175, 0, 0, 3600, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (5, 1);
		update landsraad_task_player_contributions set amount=1200 where player_id=1 and task_id=175;`)
	queue := progressTables(t, o)
	if m := raise(t, o, 1200, 3600); m["reward_rows"].(int64) != 0 || m["rewards_already_given"].(int64) != 2 || m["missions"].(int64) != 0 {
		t.Fatalf("ground the game's events cover was paid again: %v", m)
	}
	if m := raise(t, o, 3600, 4800); m["reward_rows"].(int64) != 1 {
		t.Fatalf("the tier above the events: %v", m)
	}
	if got := progressTables(t, o); got != queue {
		t.Fatalf("the progress tables or the cursor changed: %s", got)
	}
}

// What the board says about a tier: the row of its item written in this term decides (#655); a row of an earlier term is never a delivery of this term's tier. The row is of the item, so
// the same item at two tiers cannot be told apart (the edge recorded on #772).
func TestTiersTheBoardReadsWhatTheWriteDoes(t *testing.T) {
	state := func(o *Ops, th int64) (string, bool) {
		r := tierOf(t, taskByID(t, o, 175), th)
		return r["reward_state"].(string), r["delivered"].(bool)
	}
	// the character is at 3,600 and the events cover only the first tier (1,200); Solari was collected in this term (written at 120): the row of the item decides for every reached tier
	// that gives Solari, and the events are not asked (the edge: it shows the 3,600 tier as delivered although only the 1,200 one was)
	o := rewardOps(t, solariTiers+`update landsraad_task_player_contributions set amount=3600 where player_id=1 and task_id=175;
		insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (5, 2, 175, 0, 0, 1200, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (5, 1);
		insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 0, 'SolarisCoin', 120);`)
	if s, d := state(o, 1200); s != "delivered" || !d {
		t.Errorf("tier 1,200 is covered by the events: %s %v", s, d)
	}
	if s, d := state(o, 3600); s != "delivered" || !d {
		t.Errorf("tier 3,600: the collected row of this term decides: %s %v", s, d)
	}
	setup(t, o, `update landsraad_house_rewards set last_updated=60 where player_id=1 and template_id='SolarisCoin'`)
	for _, th := range []int64{1200, 3600} {
		if s, d := state(o, th); s != "no_row" || d {
			t.Errorf("tier %d with a row of an earlier term: %s %v", th, s, d)
		}
	}
	// after the editor raised the house (it writes no event, #655) the rows say: waiting, then collected, the two fields agree at every step
	o = rewardOps(t, solariTiers)
	raise(t, o, 0, 3600)
	for _, th := range []int64{1200, 2400, 3600} {
		if s, d := state(o, th); s != "waiting" || !d {
			t.Errorf("tier %d after the raise: %s %v", th, s, d)
		}
	}
	if s, d := state(o, 4800); s != "not_reached" || d {
		t.Errorf("tier 4,800: %s %v", s, d)
	}
	setup(t, o, `update landsraad_house_rewards set amount=0, last_updated=160 where player_id=1 and template_id='SolarisCoin'`)
	for _, th := range []int64{1200, 3600} {
		if s, d := state(o, th); s != "delivered" || !d {
			t.Errorf("tier %d after the collection: %s %v", th, s, d)
		}
	}
	// a row written before the term began is never this term's delivery
	setup(t, o, `update landsraad_house_rewards set last_updated=60 where player_id=1 and template_id='SolarisCoin'`)
	if s, d := state(o, 1200); s != "no_row" || d {
		t.Errorf("a row written before the term began: %s %v", s, d)
	}
}

// Where the character has no event at all for the house (a house filled by a build before #652), nothing can cover a tier, so a row written in this term decides: waiting while the
// amount is above 0, delivered when it is 0, and a row of an earlier term reads no_row. The same holds for a save without the progress tables.
func TestTiersWithNoEventsTheRowOfThisTermDecides(t *testing.T) {
	state := func(o *Ops, th int64) string { return tierOf(t, taskByID(t, o, 175), th)["reward_state"].(string) }
	row := func(amount, at int) string {
		return solariTiers + `update landsraad_task_player_contributions set amount=1300 where player_id=1 and task_id=175;
			insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', ` + fmt.Sprint(amount) + `, 'SolarisCoin', ` + fmt.Sprint(at) + `);`
	}
	if got := state(rewardOps(t, row(10000, 120)), 1200); got != "waiting" {
		t.Errorf("amount above 0, this term: %s", got)
	}
	if got := state(rewardOps(t, row(0, 120)), 1200); got != "delivered" {
		t.Errorf("amount 0, this term: %s", got)
	}
	if got := state(rewardOps(t, row(10000, 60)), 1200); got != "no_row" {
		t.Errorf("a row of an earlier term: %s", got)
	}
	// a save without the progress tables
	o := rewardOps(t, row(10000, 120)+`drop table landsraad_task_progress_player; drop table landsraad_task_progress;`)
	if got := state(o, 1200); got != "waiting" {
		t.Errorf("no progress tables, amount above 0: %s", got)
	}
	if r := tierOf(t, taskByID(t, o, 175), 1200); r["delivered"] != true {
		t.Errorf("no progress tables, delivered: %v", r["delivered"])
	}
}

// The editor writes no event for a raise (#655), so a tier it raised above the character's events is told by its row: waiting, also where the same item is given at several tiers
// (Solari here, at three tiers) and the house has events of the game.
func TestTiersATierTheEditorRaisedAboveTheEventsReadsByItsRow(t *testing.T) {
	o := rewardOps(t, solariTiers+`update landsraad_task_player_contributions set amount=1200 where player_id=1 and task_id=175;
		insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (5, 2, 175, 0, 0, 1200, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (5, 1);
		insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 10000, 'SolarisCoin', 120);`)
	queue := progressTables(t, o)
	if m := raise(t, o, 1200, 4800); m["reward_rows"].(int64) != 2 { // Schematic.K and Solari (the 1,200 tier is the game's)
		t.Fatalf("answer: %v", m)
	}
	if got := progressTables(t, o); got != queue {
		t.Fatalf("an event, a link or the cursor was written: %s", got)
	}
	state := func(th int64) (string, bool) {
		r := tierOf(t, taskByID(t, o, 175), th)
		return r["reward_state"].(string), r["delivered"].(bool)
	}
	for th, want := range map[int64]string{1200: "waiting", 2400: "waiting", 3600: "waiting", 4800: "waiting", 6000: "not_reached"} {
		if s, d := state(th); s != want || d != (want == "waiting") {
			t.Errorf("tier %d: %s %v, want %s", th, s, d, want)
		}
	}
	// the Schematic.K tier collected at the representative reads delivered
	setup(t, o, `update landsraad_house_rewards set amount=0, last_updated=160 where player_id=1 and template_id='Schematic.K'`)
	if s, d := state(2400); s != "delivered" || !d {
		t.Errorf("tier 2,400 after the collection: %s %v", s, d)
	}
	// a row of an earlier term is never this term's
	setup(t, o, `update landsraad_house_rewards set last_updated=60 where player_id=1 and template_id='Schematic.K'`)
	if s, d := state(2400); s != "no_row" || d {
		t.Errorf("tier 2,400 with a row of an earlier term: %s %v", s, d)
	}
}

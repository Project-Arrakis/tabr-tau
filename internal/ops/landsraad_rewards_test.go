package ops

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Reward delivery for a raised contribution (part 3 of the tab rework, refs #609). Every case runs the board's write on a copy of a save (the test database, the newer layout
// that has the swatch tables) and asserts the exact rows. World: mineWorld + goalWorld (the character is player 1 in Harkonnen, 2; term 7 runs from 100 to 200 and the play clock
// is 150) + the swatches of Kenola (175) and Vernius (176). Kenola's tiers are 1,200 Solari 10,000 / 2,400 Schematic.K / 3,600 T6RangeFinder 50 / 4,800 Schematic.L / 6,000 its
// own item Swatch.Kenola; the character is at 1,300 there and has no reward row for it.

const rewardWorld = swatchWorld + `
insert into landsraad_one_time_house_rewards(house_name, threshold, template_id, amount) values ('DA_HouseVernius', 6000, 'Vernius_Placeables_Swatch', 1);
`

func rewardOps(t *testing.T, extra string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithNewerSQL(t, mineWorld+goalWorld+rewardWorld+extra)}
}

func rows(t *testing.T, o *Ops, q string, args ...any) string {
	t.Helper()
	rs, err := o.S.Query(q, args...)
	if err != nil {
		t.Fatal(err)
	}
	s := ""
	for _, r := range rs {
		s += fmt.Sprint(r) + "\n"
	}
	return s
}

func houseRewards(t *testing.T, o *Ops, house string) string {
	return rows(t, o, `select player_id, amount, template_id, last_updated from landsraad_house_rewards where house_name=? order by player_id, template_id`, house)
}

func events(t *testing.T, o *Ops) string {
	return rows(t, o, `select p.id, p.faction_id, p.task_id, p.faction_progress, p.guild_progress, p.player_progress, p.timestamp, l.player_id from landsraad_task_progress p
		left join landsraad_task_progress_player l on l.progress_id=p.id order by p.id`)
}

// progressTables is the game's progress queue as one string: the events, their player links and the cursor. The editor writes none of them (#655).
func progressTables(t *testing.T, o *Ops) string {
	return events(t, o) + "--\n" + rows(t, o, `select * from landsraad_task_progress_player order by progress_id`) + "--\n" + rows(t, o, `select * from landsraad_task_progress_processed`)
}

func oneTimeRows(t *testing.T, o *Ops) string {
	return rows(t, o, `select player_id, house_name, template_id, term_id from landsraad_player_one_time_house_rewards order by player_id, house_name`)
}

func fillKenola(t *testing.T, o *Ops, extra Args) map[string]any {
	h := Args{"was_mine": 1300.0, "mine_to": 6000.0}
	for k, v := range extra {
		h[k] = v
	}
	return mustApply(t, o, applyArgs(house(175, h)))
}

func TestRewardsFillDeliversEveryTierCrossedAndTheSwatchInsteadOfTheTopItem(t *testing.T) {
	o := rewardOps(t, "")
	queue := progressTables(t, o)
	maros, other := houseRewards(t, o, "DA_HouseMaros"), houseRewards(t, o, "DA_HouseKenola")
	contribs := rows(t, o, `select * from landsraad_task_player_contributions where task_id<>175 order by player_id, task_id`)
	m := fillKenola(t, o, nil)
	if _, has := m["events"]; has || m["reward_rows"].(int64) != 4 || m["swatches"].(int64) != 1 || m["rewards_already_given"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	want := "map[amount:1 last_updated:150 player_id:1 template_id:Kenola_Placeables_Swatch]\n" +
		"map[amount:1 last_updated:150 player_id:1 template_id:Schematic.K]\n" +
		"map[amount:1 last_updated:150 player_id:1 template_id:Schematic.L]\n" +
		"map[amount:50 last_updated:150 player_id:1 template_id:T6RangeFinder]\n" +
		"map[amount:10000 last_updated:150 player_id:50 template_id:SolarisCoin]\n"
	if got := houseRewards(t, o, "DA_HouseKenola"); got != want {
		t.Fatalf("Kenola's rows:\n%s\nwant\n%s", got, want)
	}
	if got := oneTimeRows(t, o); got != "map[house_name:DA_HouseKenola player_id:1 template_id:Kenola_Placeables_Swatch term_id:7]\n" {
		t.Fatalf("the one-time row: %s", got)
	}
	if got := progressTables(t, o); got != queue {
		t.Fatalf("a raise wrote an event, a link or moved the cursor (#655):\n%s\n---\n%s", queue, got)
	}
	if houseRewards(t, o, "DA_HouseMaros") != maros {
		t.Fatal("another house's rows were touched")
	}
	if got := rows(t, o, `select * from landsraad_task_player_contributions where task_id<>175 order by player_id, task_id`); got != contribs {
		t.Fatalf("another house's contribution changed:\n%s\n%s", contribs, got)
	}
	if other == "" || rows(t, o, `select amount from landsraad_house_rewards where player_id=50`) != "map[amount:10000]\n" {
		t.Fatal("the simulated guild's row was touched")
	}
}

func TestRewardsTheTopTierDeliversTheTaskItemOnceTheSwatchIsHeld(t *testing.T) {
	o := rewardOps(t, `insert into landsraad_player_one_time_house_rewards(player_id, house_name, template_id, term_id) values (1, 'DA_HouseKenola', 'Kenola_Placeables_Swatch', 6);`)
	m := fillKenola(t, o, nil)
	if m["reward_rows"].(int64) != 4 || m["swatches"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	want := "map[amount:1 last_updated:150 player_id:1 template_id:Schematic.K]\n" +
		"map[amount:1 last_updated:150 player_id:1 template_id:Schematic.L]\n" +
		"map[amount:1 last_updated:150 player_id:1 template_id:Swatch.Kenola]\n" +
		"map[amount:50 last_updated:150 player_id:1 template_id:T6RangeFinder]\n" +
		"map[amount:10000 last_updated:150 player_id:50 template_id:SolarisCoin]\n"
	if got := houseRewards(t, o, "DA_HouseKenola"); got != want {
		t.Fatalf("Kenola's rows:\n%s\nwant\n%s", got, want)
	}
	if got := oneTimeRows(t, o); got != "map[house_name:DA_HouseKenola player_id:1 template_id:Kenola_Placeables_Swatch term_id:6]\n" {
		t.Fatalf("no one-time row is added and the old one stays: %s", got)
	}
}

func TestRewardsAHouseWithoutASwatchRowDeliversTheTaskItem(t *testing.T) {
	// the older layout has no swatch tables at all; the newer one may list no swatch for the house
	for name, o := range map[string]*Ops{
		"no tables": mineOps(t, goalWorld),
		"no row":    &Ops{S: testsave.PlayerWithNewerSQL(t, mineWorld+goalWorld)},
	} {
		m := fillKenola(t, o, nil)
		if m["reward_rows"].(int64) != 4 || m["swatches"].(int64) != 0 {
			t.Fatalf("%s: answer: %v", name, m)
		}
		if got := rows(t, o, `select amount from landsraad_house_rewards where player_id=1 and template_id='Swatch.Kenola'`); got != "map[amount:1]\n" {
			t.Fatalf("%s: the task's own item: %q", name, got)
		}
	}
}

func TestRewardsFillFactionGoalTakesTheHouseAndTheFactionGainsWhatTheCharacterGained(t *testing.T) {
	o := rewardOps(t, "")
	queue := progressTables(t, o)
	fillKenola(t, o, Args{"complete_for": 2.0})
	if v, _ := progress(t, o, 2, 175); v != 5700 {
		t.Fatalf("the faction's progress: %v", v)
	}
	if got := progressTables(t, o); got != queue {
		t.Fatalf("the queue changed: %s", got)
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 1 || h["w"].(int64) != 2 || h["ct"].(int64) != 150 {
		t.Fatalf("taken: %v", h)
	}
	// the same request on an open house the faction is already at the goal on: the faction still gains what the character gained, as a plain Fill gives it (#751)
	o = rewardOps(t, `update landsraad_task_faction_contributions set amount=4000 where task_id=175`)
	fillKenola(t, o, Args{"complete_for": 2.0})
	if v, _ := progress(t, o, 2, 175); v != 8700 {
		t.Fatalf("the faction's progress (4,000 and the 4,700 the character gained): %v", v)
	}
}

func TestRewardsATierGiveOnAHouseWithoutAContributionRowDeliversFromTheFirstTier(t *testing.T) {
	o := rewardOps(t, "")
	queue := progressTables(t, o)
	m := mustApply(t, o, applyArgs(house(176, Args{"mine_to": 3600.0})))
	if m["reward_rows"].(int64) != 3 || m["swatches"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	want := "map[amount:1 last_updated:150 player_id:1 template_id:Schematic.V]\n" +
		"map[amount:10000 last_updated:150 player_id:1 template_id:SolarisCoin]\n" +
		"map[amount:50 last_updated:150 player_id:1 template_id:T6Machinery]\n"
	if got := houseRewards(t, o, "DA_HouseVernius"); got != want {
		t.Fatalf("rows:\n%s\nwant\n%s", got, want)
	}
	if got := progressTables(t, o); got != queue {
		t.Fatalf("the queue changed: %s", got)
	}
	if oneTimeRows(t, o) != "" {
		t.Fatal("no swatch was delivered, so there is no one-time row")
	}
}

func TestRewardsNothingIsWrittenForARemovalACompleteOrAnotherFaction(t *testing.T) {
	for name, hs := range map[string][]map[string]any{
		"removal":          {house(174, Args{"was_mine": 7850.0, "mine_to": 4800.0, "lower": true})},
		"complete":         {house(175, Args{"complete_for": 2.0})},
		"complete other":   {house(175, Args{"complete_for": 1.0})},
		"reopen":           {house(174, Args{"was_done": true, "reopen": true})},
		"fill above (set)": {house(174, Args{"was_mine": 7850.0, "mine_to": 6000.0, "lower": true})},
	} {
		o := rewardOps(t, "")
		rBefore, eBefore, oBefore := rows(t, o, `select * from landsraad_house_rewards order by 1,2,4`), progressTables(t, o), oneTimeRows(t, o)
		mustApply(t, o, applyArgs(hs...))
		if got := rows(t, o, `select * from landsraad_house_rewards order by 1,2,4`); got != rBefore || progressTables(t, o) != eBefore || oneTimeRows(t, o) != oBefore {
			t.Errorf("%s wrote reward rows or an event", name)
		}
	}
}

// The editor writes no event (#655) and remembers the ground it paid itself instead (#777). A removal takes back the waiting reward of the removed tier (#944), and the same raise writes it
// again, once: the rows are what they were before the removal, and the waiting amount of an item is not added to.
func TestRewardsATierIsDeliveredOnceAgainAfterARemovalTookItBack(t *testing.T) {
	o := rewardOps(t, "")
	mustApply(t, o, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 3600.0})))
	before := houseRewards(t, o, "DA_HouseKenola")
	m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 3600.0, "mine_to": 2400.0, "lower": true})))
	if m["rewards_taken_back"].(int64) != 1 || houseRewards(t, o, "DA_HouseKenola") == before {
		t.Fatalf("the removal did not take back the waiting reward of the removed tier: %v", m)
	}
	m = mustApply(t, o, applyArgs(house(175, Args{"was_mine": 2400.0, "mine_to": 3600.0})))
	if m["reward_rows"].(int64) != 1 || m["rewards_already_given"].(int64) != 0 || m["missions"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if got := houseRewards(t, o, "DA_HouseKenola"); got != before {
		t.Fatalf("the 3,600 tier is not what it was: %s (rows before: %s)", got, before)
	}
	if got := events(t, o); got != "" {
		t.Fatalf("an event was written: %s", got)
	}
}

// The same with the reward collected: a removal never touches it, and the raise does not deliver the tier again.
func TestRewardsACollectedTierIsNotDeliveredAgainAfterARemoval(t *testing.T) {
	o := rewardOps(t, "")
	mustApply(t, o, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 3600.0})))
	collectRewards(t, o)
	before := houseRewards(t, o, "DA_HouseKenola")
	mustApply(t, o, applyArgs(house(175, Args{"was_mine": 3600.0, "mine_to": 2400.0, "lower": true})))
	if houseRewards(t, o, "DA_HouseKenola") != before {
		t.Fatal("a removal changed a collected reward")
	}
	m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 2400.0, "mine_to": 3600.0})))
	if m["reward_rows"].(int64) != 0 || m["rewards_already_given"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if got := houseRewards(t, o, "DA_HouseKenola"); got != before {
		t.Fatalf("the 3,600 tier was delivered again: %s (rows before: %s)", got, before)
	}
}

func TestRewardsAnEarlierTermsRowIsMergedWhenWaitingAndSetWhenCollected(t *testing.T) {
	o := rewardOps(t, `insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values
		(1, 'DA_HouseKenola', 1, 'Schematic.K', 50), (1, 'DA_HouseKenola', 0, 'T6RangeFinder', 50);`)
	fillKenola(t, o, nil)
	want := "map[amount:1 last_updated:150 player_id:1 template_id:Kenola_Placeables_Swatch]\n" +
		"map[amount:2 last_updated:150 player_id:1 template_id:Schematic.K]\n" +
		"map[amount:1 last_updated:150 player_id:1 template_id:Schematic.L]\n" +
		"map[amount:50 last_updated:150 player_id:1 template_id:T6RangeFinder]\n" +
		"map[amount:10000 last_updated:150 player_id:50 template_id:SolarisCoin]\n"
	if got := houseRewards(t, o, "DA_HouseKenola"); got != want {
		t.Fatalf("rows:\n%s\nwant\n%s", got, want)
	}
}

// The game's own events and its cursor are read, never written (#655): an event of the game stays as it is, the cursor is not moved, and no event or link is added.
func TestRewardsTheGamesEventsAndCursorAreLeftAsTheyAre(t *testing.T) {
	o := rewardOps(t, `insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (3, 2, 174, 0, 0, 100, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (3, 1);
		insert into landsraad_task_progress_processed(id, last_processed_id) values (1, 7);`)
	queue := progressTables(t, o)
	fillKenola(t, o, nil)
	fillKenola(t, o, Args{"was_mine": 6000.0, "mine_to": 6000.0})
	if got := progressTables(t, o); got != queue {
		t.Fatalf("the events, their links or the cursor changed:\n%s\n---\n%s", queue, got)
	}
}

func TestRewardsNothingIsLoweredAndAnotherPlayersRowsAreKept(t *testing.T) {
	o := rewardOps(t, "")
	before := rows(t, o, `select player_id, house_name, template_id, amount from landsraad_house_rewards order by 1,2,3`)
	fillKenola(t, o, nil)
	// every row that was there is still there with the same amount
	for _, l := range strings.Split(strings.TrimSpace(before), "\n") {
		if !strings.Contains(rows(t, o, `select player_id, house_name, template_id, amount from landsraad_house_rewards order by 1,2,3`), l) {
			t.Fatalf("a row is gone or changed: %s", l)
		}
	}
	if c := contribution(t, o, 1, 175); c != 6000 {
		t.Fatal(c)
	}
	if c := contribution(t, o, 1, 174); c != 7850 {
		t.Fatalf("a contribution above the top tier is not lowered: %v", c)
	}
}

// The board shows the swatch as this term's top reward while the swatch was won in this term (landsraad_board.go), so giving the top tier again after a removal must not write the
// task's own item instead: the swatch was already delivered in this term, and nothing is written.
func TestRewardsACollectedSwatchWonThisTermIsNotReplacedByTheTaskItemAfterARemoval(t *testing.T) { // the one-time row of this term is what remembers it, the editor writes no event (#655)
	o := rewardOps(t, "")
	fillKenola(t, o, nil)
	collectRewards(t, o) // a waiting swatch is taken back with its one-time row (#944, the next test); a collected one stays
	before := houseRewards(t, o, "DA_HouseKenola")
	mustApply(t, o, applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true})))
	m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 4800.0, "mine_to": 6000.0})))
	if m["reward_rows"].(int64) != 0 || m["rewards_already_given"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if got := houseRewards(t, o, "DA_HouseKenola"); got != before {
		t.Fatalf("the top tier was delivered again as another item:\n%s\n%s", before, got)
	}
	if got := oneTimeRows(t, o); got != "map[house_name:DA_HouseKenola player_id:1 template_id:Kenola_Placeables_Swatch term_id:7]\n" {
		t.Fatalf("the one-time row changed: %s", got)
	}
}

// A swatch that still waits is taken back with its one-time row (#944), and the same raise gives the swatch again, not the task's own item: the one-time row is what decides, and it is back.
func TestRewardsAWaitingSwatchIsGivenAgainAsTheSwatchAfterARemoval(t *testing.T) {
	o := rewardOps(t, "")
	fillKenola(t, o, nil)
	before := houseRewards(t, o, "DA_HouseKenola")
	m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true})))
	if m["rewards_taken_back"].(int64) != 1 || m["swatches_taken_back"].(int64) != 1 || oneTimeRows(t, o) != "" {
		t.Fatalf("the waiting swatch and its one-time row were not taken back: %v, %q", m, oneTimeRows(t, o))
	}
	m = mustApply(t, o, applyArgs(house(175, Args{"was_mine": 4800.0, "mine_to": 6000.0})))
	if m["reward_rows"].(int64) != 1 || m["swatches"].(int64) != 1 || m["missions"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if got := houseRewards(t, o, "DA_HouseKenola"); got != before {
		t.Fatalf("the top tier is not the swatch it was:\n%s\n%s", before, got)
	}
	if got := oneTimeRows(t, o); got != "map[house_name:DA_HouseKenola player_id:1 template_id:Kenola_Placeables_Swatch term_id:7]\n" {
		t.Fatalf("the one-time row is not back: %s", got)
	}
}

// Fill under another faction: the contribution is the character's, under its own faction, so a request that raises it while completing the house for another faction (what
// Fill faction goal sent before the page stopped doing that) is refused as a whole, and nothing is written (#628, #609).
func TestRewardsFillWithAnotherFactionChosenIsRefusedAndWritesNothing(t *testing.T) {
	o := rewardOps(t, "")
	rBefore, eBefore, oBefore := rows(t, o, `select * from landsraad_house_rewards order by 1,2,4`), progressTables(t, o), oneTimeRows(t, o)
	cBefore := rows(t, o, `select * from landsraad_task_player_contributions order by 1,2,3`)
	if _, err := o.LandsraadApply(applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 1.0}))); err == nil {
		t.Fatal("raising the contribution while completing for another faction was accepted")
	}
	if rows(t, o, `select * from landsraad_house_rewards order by 1,2,4`) != rBefore || progressTables(t, o) != eBefore || oneTimeRows(t, o) != oBefore ||
		rows(t, o, `select * from landsraad_task_player_contributions order by 1,2,3`) != cBefore {
		t.Fatal("a refused request changed the save")
	}
	// the same house, the character's own faction (2): both parts are written, with the rewards
	if m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}))); m["reward_rows"].(int64) != 4 {
		t.Fatalf("answer: %v", m)
	}
}

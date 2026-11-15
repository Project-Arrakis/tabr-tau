package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What a board write that raises the character's contribution grants (refs #609, #652, #656): the faction's share and the completion, the event and the cursor, the reward rows by
// tier, the two wallets and the specialization track of the house's line. Every case runs on a copy of a save built from the real schema (the test database); no real save is read.
// World: rewardOps (landsraad_rewards_test.go): the character is player 1 in Harkonnen (2); House Kenola (task 175, Gathering) is open with the faction at 1,000 and the character at
// 1,300 with no event; the play clock is 150 s.

// tables is every row a grant can touch, in a fixed order, as one string.
func grantRows(t *testing.T, o *Ops) string {
	t.Helper()
	return boardRows(t, o) + rows(t, o, `select * from landsraad_task_progress order by id`) + "--\n" +
		rows(t, o, `select * from landsraad_task_progress_player order by progress_id`) + "--\n" +
		rows(t, o, `select * from landsraad_task_progress_processed`) + "--\n" +
		rows(t, o, `select * from landsraad_player_one_time_house_rewards order by 1,2`) + "--\n" +
		rows(t, o, `select * from player_virtual_currency_balances order by 1,2`) + "--\n" +
		rows(t, o, `select * from specialization_tracks order by 1,2`)
}

func wallet(t *testing.T, o *Ops, currency int64) int64 {
	t.Helper()
	r, err := o.S.One(`select balance b from player_virtual_currency_balances where player_controller_id=1 and currency_id=?`, currency)
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		return 0
	}
	return r["b"].(int64)
}

func track(t *testing.T, o *Ops, number int64) (xp int64, level float64, has bool) {
	t.Helper()
	r, err := o.S.One(`select xp_amount x, level l from specialization_tracks where player_id=1 and track_type=?`, number)
	if err != nil || r == nil {
		return 0, 0, false
	}
	return r["x"].(int64), r["l"].(float64), true
}

func applyWith(t *testing.T, o *Ops, g GrantSettings, hs ...map[string]any) map[string]any {
	t.Helper()
	got, err := o.LandsraadApplyWith(g, applyArgs(hs...))
	if err != nil {
		t.Fatal(err)
	}
	return got.(map[string]any)
}

// the settings of the measured save: both multipliers 2
func doubled() GrantSettings {
	return GrantSettings{ContributionMultiplier: 2, SpecializationXPMultiplier: 2}
}

func TestGrantAFillGrantsWhatTheMissionsItStandsForGrant(t *testing.T) {
	o := rewardOps(t, `insert into player_virtual_currency_balances(player_controller_id, currency_id, balance) values (1, 0, 1000);
		insert into specialization_tracks(player_id, track_type, xp_amount, level) values (1, 2, 18000, 57.37);`)
	// Kenola, 1,300 to 6,000 with a mission step of 2,000 (1,000 x 2): the missions that take the character from 1,300 to 6,000 are the 2nd and the 3rd
	m := applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	if m["missions"].(int64) != 2 || m["solari"].(int64) != 12000 || m["house_scrip"].(int64) != 2000 || m["specialization_xp"].(map[string]int64)["Gathering"] != 6000 {
		t.Fatalf("answer: %v", m)
	}
	if got := wallet(t, o, 0); got != 13000 {
		t.Fatalf("Solari: %d", got)
	}
	if got := wallet(t, o, 1); got != 2000 {
		t.Fatalf("House Scrip (a wallet with no row is created): %d", got)
	}
	// Kenola is a Gathering house, track 2; 2 x 1,500 per mission, two missions
	xp, level, _ := track(t, o, 2)
	if xp != 24000 || level != specLevel(24000) {
		t.Fatalf("the track: %d xp, level %v", xp, level)
	}
	if xp, _, has := track(t, o, 3); has || xp != 0 {
		t.Fatal("another line's track was created")
	}
	// the house side: the faction gained what the character gained (1,000 + 4,700) and the house was taken by the event that took it past the goal
	if v, _ := progress(t, o, 2, 175); v != 5700 {
		t.Fatalf("faction progress: %v", v)
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 1 || h["w"].(int64) != 2 || h["ct"].(int64) != 150 {
		t.Fatalf("the house: %v", h)
	}
	if got := rows(t, o, `select * from landsraad_task_progress_processed`); got != "map[id:1 last_processed_id:1]\n" {
		t.Fatalf("the cursor moves to the event: %s", got)
	}
}

func TestGrantAtTheStockSettingsAMissionIsOneThousandAndFifteenHundred(t *testing.T) {
	o := rewardOps(t, "")
	// a house with no row: 0 to 1,200 is one mission at the stock step of 1,000? No: ceil(1200 / 1000) = 2 missions cross it
	m := applyWith(t, o, DefaultGrantSettings(), house(176, Args{"mine_to": 1200.0}))
	if m["missions"].(int64) != 2 || m["solari"].(int64) != 12000 || m["house_scrip"].(int64) != 2000 || m["specialization_xp"].(map[string]int64)["Sabotage"] != 3000 {
		t.Fatalf("answer: %v", m)
	}
	// Vernius is a Sabotage house: track 5, a row created by the first mission
	if xp, level, has := track(t, o, 5); !has || xp != 3000 || level != specLevel(3000) {
		t.Fatalf("the track: %d %v %v", xp, level, has)
	}
	// a learned number takes the place of the measured one
	o = rewardOps(t, "")
	g := DefaultGrantSettings()
	g.TrackNumbers = map[string]int64{"Sabotage": 9}
	applyWith(t, o, g, house(176, Args{"mine_to": 1200.0}))
	if _, _, has := track(t, o, 9); !has {
		t.Fatal("the learned number is not used")
	}
	if _, _, has := track(t, o, 5); has {
		t.Fatal("the measured number was used although a number was learned")
	}
}

func TestGrantASecondFillInTheSameTermGrantsNothingTwice(t *testing.T) {
	o := rewardOps(t, "")
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	after := grantRows(t, o)
	// the same Fill again: the house is already at the tier, so nothing changes at all
	m := applyWith(t, o, doubled(), house(175, Args{"was_mine": 6000.0, "mine_to": 6000.0}))
	if m["changed"].(int64) != 0 || m["missions"].(int64) != 0 || grantRows(t, o) != after {
		t.Fatalf("a second Fill changed something: %v", m)
	}
	// a tier removed and the same Fill again, with the wallets, the track, the events and the reward rows unchanged
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true}))
	removed := grantRows(t, o)
	m = applyWith(t, o, doubled(), house(175, Args{"was_mine": 4800.0, "mine_to": 6000.0}))
	if m["events"].(int64) != 0 || m["reward_rows"].(int64) != 0 || m["missions"].(int64) != 0 || m["solari"].(int64) != 0 || m["rewards_already_given"].(int64) != 1 {
		t.Fatalf("a Fill after a removal granted again: %v", m)
	}
	// only the contribution went back up
	if got := grantRows(t, o); got == removed || strings.Replace(got, "amount:6000", "amount:4800", 1) != removed {
		t.Fatalf("something besides the contribution changed:\n%s\n---\n%s", removed, got)
	}
	// removed down to nothing and filled again: still nothing is granted, the events cover the ground
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true}))
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 4800.0, "mine_to": 3600.0, "lower": true}))
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 3600.0, "mine_to": 2400.0, "lower": true}))
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 2400.0, "mine_to": 1200.0, "lower": true}))
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1200.0, "mine_to": 0.0, "lower": true}))
	m = applyWith(t, o, doubled(), house(175, Args{"mine_to": 6000.0}))
	if m["events"].(int64) != 0 || m["missions"].(int64) != 0 || m["rewards_already_given"].(int64) != 5 {
		t.Fatalf("a Fill after the row was deleted granted again: %v", m)
	}
	if wallet(t, o, 0) != 12000 || wallet(t, o, 1) != 2000 {
		t.Fatalf("the wallets: %d, %d", wallet(t, o, 0), wallet(t, o, 1))
	}
	if n := rows(t, o, `select count(*) n from landsraad_task_progress`); n != "map[n:1]\n" {
		t.Fatalf("events: %s", n)
	}
}

func TestGrantTiersOneByOneAddUpToTheSameAsOneFill(t *testing.T) {
	a, b := rewardOps(t, ""), rewardOps(t, "")
	applyWith(t, a, doubled(), house(176, Args{"mine_to": 6000.0}))
	was := 0.0
	for _, th := range []float64{1200, 2400, 3600, 4800, 6000} {
		applyWith(t, b, doubled(), house(176, Args{"was_mine": was, "mine_to": th}))
		was = th
	}
	for name, o := range map[string]*Ops{"one fill": a, "five clicks": b} {
		if wallet(t, o, 0) != 18000 || wallet(t, o, 1) != 3000 {
			t.Errorf("%s: wallets %d, %d", name, wallet(t, o, 0), wallet(t, o, 1))
		}
		if xp, _, _ := track(t, o, 5); xp != 9000 {
			t.Errorf("%s: the Sabotage track: %d", name, xp)
		}
		if n := rows(t, o, `select count(*) n from landsraad_house_rewards where player_id=1 and house_name='DA_HouseVernius'`); n != "map[n:5]\n" {
			t.Errorf("%s: reward rows: %s", name, n)
		}
	}
}

// A reward collected in a later term, and one item at two tiers (#656): the tiers the character's own events have not crossed are delivered whatever rows of the same item are there.
func TestGrantACollectedThenRaisedHouseIsDecidedPerTier(t *testing.T) {
	// (a) the Solari of Kenola's first tier was collected in this term (amount 0, written at 120, in the term's window of 100 to 200) and the character has no event for the task:
	// the first tier is delivered again, the row is set, not skipped
	o := rewardOps(t, `insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 0, 'SolarisCoin', 120);
		update landsraad_task_player_contributions set amount=0 where player_id=1 and task_id=175;`)
	m := applyWith(t, o, DefaultGrantSettings(), house(175, Args{"mine_to": 1200.0}))
	if m["reward_rows"].(int64) != 1 || m["rewards_already_given"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if got := rows(t, o, `select amount, last_updated from landsraad_house_rewards where player_id=1 and template_id='SolarisCoin'`); got != "map[amount:10000 last_updated:150]\n" {
		t.Fatalf("the collected row: %s", got)
	}
	// (b) one item at two tiers (3,600 and 4,800 both give T6RangeFinder): the first by play (an event of the character, the row waiting from this term), the second by the editor
	o = rewardOps(t, `update landsraad_task_rewards set template_id='T6RangeFinder', amount=2 where task_id=175 and threshold=4800;
		insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (5, 2, 175, 0, 0, 3600, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (5, 1);
		update landsraad_task_player_contributions set amount=3600 where player_id=1 and task_id=175;
		insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 50, 'T6RangeFinder', 120);`)
	m = applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": 3600.0, "mine_to": 4800.0}))
	if m["reward_rows"].(int64) != 1 || m["rewards_already_given"].(int64) != 0 {
		t.Fatalf("the second tier of the same item was skipped: %v", m)
	}
	if got := rows(t, o, `select amount from landsraad_house_rewards where player_id=1 and template_id='T6RangeFinder'`); got != "map[amount:52]\n" {
		t.Fatalf("the waiting amount is added to: %s", got)
	}
	// (c) the tiers the events crossed are delivered, whatever the contribution says: the character's events cover 3,600, the contribution was lowered to 1,200, a raise to 4,800 delivers
	// the 4,800 tier only
	o = rewardOps(t, `insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (5, 2, 175, 0, 0, 3600, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (5, 1);
		update landsraad_task_player_contributions set amount=1200 where player_id=1 and task_id=175;`)
	m = applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": 1200.0, "mine_to": 4800.0}))
	if m["reward_rows"].(int64) != 1 || m["rewards_already_given"].(int64) != 2 {
		t.Fatalf("answer: %v", m)
	}
	if got := rows(t, o, `select template_id from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola'`); got != "map[template_id:Schematic.L]\n" {
		t.Fatalf("only the 4,800 tier: %s", got)
	}
	// the event records the ground that was added beyond the events: the sum of the character's events is the contribution again
	if got := rows(t, o, `select sum(p.player_progress) s from landsraad_task_progress p join landsraad_task_progress_player l on l.progress_id=p.id where l.player_id=1`); got != "map[s:4800]\n" {
		t.Fatalf("sum of events: %s", got)
	}
}

// The edit is one transaction: an error at any step leaves no step behind.
func TestGrantAnInterruptedWriteLeavesNothingHalfWritten(t *testing.T) {
	for name, trigger := range map[string]string{
		"the specialization track": `create trigger boom before insert on specialization_tracks begin select raise(abort, 'disk full'); end;`,
		"the Solari wallet":        `create trigger boom before insert on player_virtual_currency_balances begin select raise(abort, 'disk full'); end;`,
		"the reward rows":          `create trigger boom before insert on landsraad_house_rewards begin select raise(abort, 'disk full'); end;`,
		"the cursor":               `create trigger boom before insert on landsraad_task_progress_processed begin select raise(abort, 'disk full'); end;`,
		"the swatch row":           `create trigger boom before insert on landsraad_player_one_time_house_rewards begin select raise(abort, 'disk full'); end;`,
	} {
		o := rewardOps(t, "")
		before := grantRows(t, o)
		setup(t, o, trigger)
		if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}), house(176, Args{"mine_to": 3600.0}))); err == nil {
			t.Fatalf("%s: the failing write was accepted", name)
		}
		setup(t, o, `drop trigger boom`)
		if after := grantRows(t, o); after != before {
			t.Errorf("%s: a failed write left rows behind:\n%s\n---\n%s", name, before, after)
		}
	}
}

func TestGrantRefusalsLeaveNothingAndSayWhy(t *testing.T) {
	// a house the table has no line for
	o := rewardOps(t, `insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (177, 7, 3, 'DA_HouseNobody', 4000);
		insert into landsraad_task_rewards(task_id, threshold, template_id, amount) values (177, 1200, 'SolarisCoin', 1);`)
	before := grantRows(t, o)
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}), house(177, Args{"mine_to": 1200.0})))
	if err == nil || !strings.Contains(err.Error(), "Nobody is not a house the editor knows a specialization line for") || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("an unknown house: %v", err)
	}
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
	// a settings value that is not a number from 0 to 10
	g := doubled()
	g.Problem = "The Landsraad amounts cannot be worked out: LandsraadContributionMultiplier in ServerCustomSettings.ini is \"x\", not a number from 0 to 10. Nothing was changed."
	if _, err := o.LandsraadApplyWith(g, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))); err == nil || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("a bad setting: %v", err)
	}
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
	// the same bad setting does not stop what needs no amounts: a removal
	if _, err := o.LandsraadApplyWith(g, applyArgs(house(174, Args{"was_mine": 7850.0, "mine_to": 4800.0, "lower": true}))); err != nil {
		t.Fatalf("a removal needs no settings: %v", err)
	}
}

func TestGrantWhatTheFactionGainsAndWhoTakesTheHouse(t *testing.T) {
	// a house that is open: the event carries the faction's share, and the event after the completion carries none
	o := rewardOps(t, "")
	applyWith(t, o, DefaultGrantSettings(), house(176, Args{"mine_to": 3600.0}))
	applyWith(t, o, DefaultGrantSettings(), house(176, Args{"was_mine": 3600.0, "mine_to": 6000.0}))
	if got := events(t, o); got != "map[faction_id:2 faction_progress:3600 guild_progress:0 id:1 player_id:1 player_progress:3600 task_id:176 timestamp:150]\n"+
		"map[faction_id:2 faction_progress:2400 guild_progress:0 id:2 player_id:1 player_progress:2400 task_id:176 timestamp:150]\n" {
		t.Fatalf("events (the second takes the faction from 3,600 past the goal and records the 2,400 the character added): %s", got)
	}
	// a house the other faction took: the faction gains nothing and the winner is left
	o = rewardOps(t, `update landsraad_tasks set completed=1, winning_faction_id=1, completion_time=140 where id=176;`)
	applyWith(t, o, DefaultGrantSettings(), house(176, Args{"mine_to": 3600.0}))
	if v, has := progress(t, o, 2, 176); has {
		t.Fatalf("the faction gained on a house that was taken: %v", v)
	}
	if h := houseRow(t, o, 176); h["w"].(int64) != 1 || h["ct"].(int64) != 140 {
		t.Fatalf("the winner changed: %v", h)
	}
	if got := events(t, o); !strings.Contains(got, "faction_progress:0") || !strings.Contains(got, "player_progress:3600") {
		t.Fatalf("the event after the completion carries no faction progress: %s", got)
	}
	// the wallets and the track are still granted
	if wallet(t, o, 0) == 0 || wallet(t, o, 1) == 0 {
		t.Fatal("no wallet was granted")
	}
}

func TestGrantTheTrackStopsAtItsLastLevel(t *testing.T) {
	o := rewardOps(t, `insert into specialization_tracks(player_id, track_type, xp_amount, level) values (1, 5, 44000, 99.7);`)
	m := applyWith(t, o, doubled(), house(176, Args{"mine_to": 6000.0}))
	if xp, level, _ := track(t, o, 5); xp != maxSpecXP || level != 100 {
		t.Fatalf("the track: %d %v", xp, level)
	}
	if m["specialization_xp"].(map[string]int64)["Sabotage"] != maxSpecXP-44000 {
		t.Fatalf("what was added is what fitted: %v", m)
	}
	// a track at its last level changes nothing, and the wallets are still paid
	o = rewardOps(t, `insert into specialization_tracks(player_id, track_type, xp_amount, level) values (1, 5, 44182, 100);`)
	m = applyWith(t, o, doubled(), house(176, Args{"mine_to": 6000.0}))
	if len(m["specialization_xp"].(map[string]int64)) != 0 || m["solari"].(int64) == 0 {
		t.Fatalf("answer: %v", m)
	}
}

// Five houses to a line, 25 in all, each a house whose symbol the page shows (the list of houses of the board).
func TestGrantTheHouseLineTableCoversEveryHouseFiveToALine(t *testing.T) {
	if len(houseLine) != 25 {
		t.Fatalf("%d houses", len(houseLine))
	}
	per := map[string]int{}
	for _, l := range houseLine {
		per[l]++
	}
	for _, line := range SpecTracks {
		if per[line] != 5 {
			t.Errorf("%s has %d houses", line, per[line])
		}
		if _, ok := measuredTrackNumbers[line]; !ok {
			t.Errorf("%s has no measured number", line)
		}
	}
	files, err := filepath.Glob(filepath.Join("..", "web", "static", "houses", "*.webp"))
	if err != nil || len(files) != 25 {
		t.Fatalf("symbols: %d %v", len(files), err)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".webp")
		if lineOfHouse("DA_House"+strings.ToUpper(name[:1])+name[1:]) == "" {
			t.Errorf("%s: the table has no line for this house", name)
		}
	}
	if lineOfHouse("DA_HouseArgosaz") != "Exploration" || lineOfHouse("DA_HouseMikkarol") != "Combat" {
		t.Error("the two measured houses")
	}
	if _, err := os.Stat(filepath.Join("..", "web", "static", "houses")); err != nil {
		t.Fatal(err)
	}
}

// #750: the number of a line is the learned one or the measured one, line by line; two lines on one number would write the XP of both to one row
func TestGrantRefusesTwoLinesOnTheSameTrackNumber(t *testing.T) {
	o := rewardOps(t, "")
	before := grantRows(t, o)
	g := doubled()
	g.TrackNumbers = map[string]int64{"Crafting": 2} // Gathering falls back to the measured 2
	_, err := o.LandsraadApplyWith(g, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	if err == nil || !strings.Contains(err.Error(), "Crafting and Gathering both use number 2") || !strings.Contains(err.Error(), "Nothing was changed") || !strings.Contains(err.Error(), "Specialization tab") {
		t.Fatalf("two lines on one number: %v", err)
	}
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
	// a removal needs no track number
	if _, err := o.LandsraadApplyWith(g, applyArgs(house(174, Args{"was_mine": 7850.0, "mine_to": 4800.0, "lower": true}))); err != nil {
		t.Fatalf("a removal needs no track numbers: %v", err)
	}
	// with no XP multiplier nothing is written to a track, so the numbers do not matter
	g.SpecializationXPMultiplier = 0
	if _, err := o.LandsraadApplyWith(g, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))); err != nil {
		t.Fatalf("no XP, no track: %v", err)
	}
}

func TestGrantAcceptsLearnedNumbersThatAreAFullDifferentPermutation(t *testing.T) {
	o := rewardOps(t, "")
	g := doubled()
	// Gathering moves to 5 and Sabotage to 2, Crafting to 4 and Combat to 1: five different numbers, none of them the measured ones for its own line
	g.TrackNumbers = map[string]int64{"Crafting": 4, "Gathering": 5, "Exploration": 3, "Combat": 1, "Sabotage": 2}
	m := applyWith(t, o, g, house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	if m["specialization_xp"].(map[string]int64)["Gathering"] != 6000 {
		t.Fatalf("answer: %v", m)
	}
	if xp, _, has := track(t, o, 5); !has || xp != 6000 {
		t.Fatalf("Gathering's learned number 5: %d %v", xp, has)
	}
	if _, _, has := track(t, o, 2); has {
		t.Fatal("the measured number of Gathering was written")
	}
}

// #751.1: one house taken by the raise is one completion
func TestGrantFillFactionGoalCountsTheHouseOnce(t *testing.T) {
	o := rewardOps(t, "")
	m := applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}))
	if m["completed"].(int64) != 1 {
		t.Fatalf("completed: %v", m["completed"])
	}
}

// #751.2: Fill faction goal leaves the faction where a plain Fill would (what the character gained is added), and never below the goal
func TestGrantFillFactionGoalGivesTheFactionWhatAPlainFillGives(t *testing.T) {
	plain, goal := rewardOps(t, ""), rewardOps(t, "")
	applyWith(t, plain, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	applyWith(t, goal, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}))
	fac := func(o *Ops) float64 {
		r, err := o.S.One(`select amount a from landsraad_task_faction_contributions where faction_id=2 and task_id=175`)
		if err != nil || r == nil {
			t.Fatal(err, r)
		}
		return r["a"].(float64)
	}
	if fac(plain) != 5700 || fac(goal) != 5700 {
		t.Fatalf("the faction: plain %v, goal %v", fac(plain), fac(goal))
	}
	// a faction short of the goal after the gain is brought up to it
	small := rewardOps(t, "")
	applyWith(t, small, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0, "complete_for": 2.0}))
	if got := fac(small); got != 4000 {
		t.Fatalf("a faction short of the goal: %v", got)
	}
}

func TestGrantTheBoardReadSaysWhatTheCharactersEventsCover(t *testing.T) {
	o := rewardOps(t, "")
	applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	v, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range v.(map[string]any)["tasks"].([]map[string]any) {
		if x["id"] == int64(175) && (x["my_events"] != 6000.0 || x["line"] != "Gathering") {
			t.Fatalf("Kenola: my_events %v, line %v", x["my_events"], x["line"])
		}
	}
}

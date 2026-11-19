package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What a board write that raises the character's contribution grants (refs #609, #652, #655, #656): the faction's share and the completion, the reward rows by
// tier (no event and no cursor move, #655), the two wallets and the specialization track of the house's line. Every case runs on a copy of a save built from the real schema (the test database); no real save is read.
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
	queue := progressTables(t, o)
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
	// the house side: the faction gained what the character gained (1,000 + 4,700) and the house was taken when the faction went past the goal
	if v, _ := progress(t, o, 2, 175); v != 5700 {
		t.Fatalf("faction progress: %v", v)
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 1 || h["w"].(int64) != 2 || h["ct"].(int64) != 150 {
		t.Fatalf("the house: %v", h)
	}
	if got := progressTables(t, o); got != queue {
		t.Fatalf("no event, link or cursor move is written (#655): %s", got)
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

// #1006: the settings the page really passes are EffectiveTrackNumbers of what was learned, and a track that map leaves out falls back to its measured number in the grant. With only Combat
// learned as 1 the map has no Crafting, so Crafting reads 1 as well: the raise is refused, with nothing changed, and not written to one row for two lines.
func TestGrantRefusesWhenTheEffectiveMapLeavesATrackOutAndItsMeasuredNumberIsTaken(t *testing.T) {
	o := rewardOps(t, "")
	before := grantRows(t, o)
	g := doubled()
	g.TrackNumbers = EffectiveTrackNumbers(map[string]int64{"Combat": 1})
	if _, has := g.TrackNumbers["Crafting"]; has {
		t.Fatalf("the premise: Crafting is left out of the map: %v", g.TrackNumbers)
	}
	_, err := o.LandsraadApplyWith(g, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	if err == nil || !strings.Contains(err.Error(), "Crafting and Combat both use number 1") || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("a track left out by the map and read as its measured number: %v", err)
	}
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
}

// A second Fill grants nothing while the contribution stands, and a removal followed by the same raise grants nothing either: the editor remembers the ground it paid (#777), so
// the missions, the reward rows and the wallets are not paid again.
func TestGrantASecondFillAndARaiseAfterARemovalGrantNothingTwice(t *testing.T) {
	o := rewardOps(t, "")
	queue := progressTables(t, o)
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	after := grantRows(t, o)
	// the same Fill again: the house is already at the tier, so nothing changes at all
	m := applyWith(t, o, doubled(), house(175, Args{"was_mine": 6000.0, "mine_to": 6000.0}))
	if m["changed"].(int64) != 0 || m["missions"].(int64) != 0 || grantRows(t, o) != after {
		t.Fatalf("a second Fill changed something: %v", m)
	}
	// a tier removed and the same Fill again: the removal takes back the waiting swatch of the top tier (#944) and the Fill writes it again, but the 4,800 tier is not repeated as a mission
	// (the step is 2,000: ceil(6000/2000) = ceil(4800/2000)) and no wallet changes
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true}))
	removed := grantRows(t, o)
	if removed == after || strings.Contains(removed, "Kenola_Placeables_Swatch") {
		t.Fatalf("the removal did not take back the waiting swatch:\n%s", removed)
	}
	m = applyWith(t, o, doubled(), house(175, Args{"was_mine": 4800.0, "mine_to": 6000.0}))
	if _, has := m["events"]; has || m["reward_rows"].(int64) != 1 || m["swatches"].(int64) != 1 || m["missions"].(int64) != 0 || m["solari"].(int64) != 0 || m["rewards_already_given"].(int64) != 0 {
		t.Fatalf("a Fill after a removal of the top tier granted again, or did not give the swatch back: %v", m)
	}
	// everything is where it was after the first Fill: the swatch row, its one-time row, the wallets, the faction's progress
	if got := grantRows(t, o); got != after {
		t.Fatalf("something differs from the state after the first Fill:\n%s\n---\n%s", after, got)
	}
	// removed down to nothing and filled again: the ground was paid by the editor, so no mission and no wallet change, the five rewards come back, and the answer says so (#777, #944)
	paidWallets := [2]int64{wallet(t, o, 0), wallet(t, o, 1)}
	for _, st := range [][2]float64{{6000, 4800}, {4800, 3600}, {3600, 2400}, {2400, 1200}, {1200, 0}} {
		applyWith(t, o, doubled(), house(175, Args{"was_mine": st[0], "mine_to": st[1], "lower": true}))
	}
	m = applyWith(t, o, doubled(), house(175, Args{"mine_to": 6000.0}))
	if m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 5 || m["rewards_already_given"].(int64) != 0 || m["missions_paid_before"].(int64) != 3 || m["ground_paid_before"].(float64) != 6000 {
		t.Fatalf("a Fill after the row was deleted paid again or did not say why: %v", m)
	}
	if got := [2]int64{wallet(t, o, 0), wallet(t, o, 1)}; got != paidWallets {
		t.Fatalf("the wallets changed: %v, was %v", got, paidWallets)
	}
	if got := progressTables(t, o); got != queue {
		t.Fatalf("the progress tables or the cursor changed (#655): %s", got)
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
	// the editor adds no event: the character's events are still the one the game wrote (3,600), and the cursor is not written
	if got := rows(t, o, `select sum(p.player_progress) s, count(*) n from landsraad_task_progress p join landsraad_task_progress_player l on l.progress_id=p.id where l.player_id=1`); got != "map[n:1 s:3600]\n" {
		t.Fatalf("events: %s", got)
	}
	if got := rows(t, o, `select * from landsraad_task_progress_processed`); got != "" {
		t.Fatalf("the cursor was written: %s", got)
	}
}

// The edit is one transaction: an error at any step leaves no step behind.
func TestGrantAnInterruptedWriteLeavesNothingHalfWritten(t *testing.T) {
	for name, trigger := range map[string]string{
		"the specialization track": `create trigger boom before insert on specialization_tracks begin select raise(abort, 'disk full'); end;`,
		"the Solari wallet":        `create trigger boom before insert on player_virtual_currency_balances begin select raise(abort, 'disk full'); end;`,
		"the reward rows":          `create trigger boom before insert on landsraad_house_rewards begin select raise(abort, 'disk full'); end;`,
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
	// a house that is open: the faction gains what the character gains, and takes the house when it passes the goal
	o := rewardOps(t, "")
	applyWith(t, o, DefaultGrantSettings(), house(176, Args{"mine_to": 3600.0}))
	applyWith(t, o, DefaultGrantSettings(), house(176, Args{"was_mine": 3600.0, "mine_to": 6000.0}))
	if v, _ := progress(t, o, 2, 176); v != 6000 {
		t.Fatalf("the faction's progress (3,600 and the 2,400 the character added): %v", v)
	}
	if h := houseRow(t, o, 176); h["c"].(int64) != 1 || h["w"].(int64) != 2 {
		t.Fatalf("the house was not taken by the faction that passed the goal: %v", h)
	}
	if got := events(t, o); got != "" {
		t.Fatalf("an event was written: %s", got)
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
	if got := events(t, o); got != "" {
		t.Fatalf("an event was written: %s", got)
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

// #751: Fill faction goal on a house another faction took. The house is not open, so the faction gains nothing from the raise (no event after a completion carries faction progress)
// and is taken to the goal by the completion alone, not to goal plus what the character gained; the house is counted once.
func TestGrantFillFactionGoalOnAHouseTheOtherFactionTookCountsItOnceAndBringsTheFactionToTheGoal(t *testing.T) {
	o := rewardOps(t, `update landsraad_tasks set completed=1, winning_faction_id=1, completion_time=140 where id=175;`)
	m := applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_done": true, "was_win": 1.0, "was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}))
	if m["completed"].(int64) != 1 || m["houses"].(int64) != 1 || m["changed"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if v, _ := progress(t, o, 2, 175); v != 4000 {
		t.Fatalf("the faction's progress (its 1,000 taken to the goal; the house was not open for the 4,700 gained): %v", v)
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 1 || h["w"].(int64) != 2 {
		t.Fatalf("the house was not taken for the faction: %v", h)
	}
	// a faction already past the goal is not lowered
	o = rewardOps(t, `update landsraad_tasks set completed=1, winning_faction_id=1, completion_time=140 where id=175;
		update landsraad_task_faction_contributions set amount=4500 where task_id=175 and faction_id=2;`)
	applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_done": true, "was_win": 1.0, "was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}))
	if v, _ := progress(t, o, 2, 175); v != 4500 {
		t.Fatalf("a faction above the goal was changed: %v", v)
	}
}

// #751: Fill faction goal when the character's own events already cover the ground (the raise pays nothing): the house is still taken, once, and the faction is at the goal at
// least, which a plain Fill (that gains nothing here) would not do. Never lower than a plain Fill.
func TestGrantFillFactionGoalOnGroundTheEventsCoverCountsTheHouseOnceAndPaysNothingTwice(t *testing.T) {
	const covered = `insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (5, 2, 175, 0, 0, 6000, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (5, 1);`
	plain, goal := rewardOps(t, covered), rewardOps(t, covered)
	applyWith(t, plain, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	m := applyWith(t, goal, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}))
	if m["completed"].(int64) != 1 || m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 0 || m["solari"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if wallet(t, goal, 0) != 0 || wallet(t, goal, 1) != 0 {
		t.Fatal("ground the events cover was paid again")
	}
	pf, _ := progress(t, plain, 2, 175)
	gf, _ := progress(t, goal, 2, 175)
	if pf != 1000 || gf != 4000 || gf < pf {
		t.Fatalf("the faction: plain %v, goal %v", pf, gf)
	}
	if h := houseRow(t, goal, 175); h["c"].(int64) != 1 || h["w"].(int64) != 2 {
		t.Fatalf("the house was not taken for the faction: %v", h)
	}
}

// #751: Fill faction goal on a house that is already the character's faction's, with the faction's progress still short of the goal. The raise pays something, the faction's progress
// is taken to the goal, and the answer and the write log say the house was completed (as they do for Complete alone), counted once.
func TestGrantFillFactionGoalOnAHouseOfTheCharactersFactionBelowTheGoalSaysItWasCompleted(t *testing.T) {
	o := rewardOps(t, `update landsraad_tasks set completed=1, winning_faction_id=2, completion_time=140 where id=175;`)
	m := applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_done": true, "was_win": 2.0, "was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}))
	if m["completed"].(int64) != 1 || m["contribution_raised"].(int64) != 1 || m["missions"].(int64) == 0 {
		t.Fatalf("answer: %v", m)
	}
	if v, _ := progress(t, o, 2, 175); v != 4000 {
		t.Fatalf("the faction's progress: %v", v)
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 1 || h["w"].(int64) != 2 || h["ct"].(int64) != 140 {
		t.Fatalf("the house: %v", h)
	}
	if d := lastEdit(t, o); d != "landsraad Kenola: your contribution 1300 -> 6000, completed for Harkonnen; 4 missions: +24,000 Solari, +4,000 House Scrip, +6,000 Gathering XP; 4 reward rows" {
		t.Fatalf("write log %q", d)
	}
}

// The character's events are the game's own: a raise leaves them as they are (#655), so the board reads the same ground after a raise as before it.
func TestGrantTheBoardReadSaysWhatTheCharactersEventsCover(t *testing.T) {
	o := rewardOps(t, `insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (5, 2, 175, 0, 0, 1300, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (5, 1);`)
	applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	v, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range v.(map[string]any)["tasks"].([]map[string]any) {
		if x["id"] == int64(175) && (x["my_events"] != 1300.0 || x["line"] != "Gathering") {
			t.Fatalf("Kenola: my_events %v, line %v", x["my_events"], x["line"])
		}
	}
}

// The cap of 1,000 missions is on the whole request, not on each house: houses that are each under it must not add up to more than one change may grant.
func TestGrantTheCapCountsEveryHouseOfTheRequest(t *testing.T) {
	o := rewardOps(t, "")
	before := grantRows(t, o)
	g := GrantSettings{ContributionMultiplier: 0.01, SpecializationXPMultiplier: 1} // a step of 10: Kenola 1,300 to 6,000 is 470 missions, Vernius 0 to 6,000 is 600
	each := []map[string]any{house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}), house(176, Args{"mine_to": 6000.0})}
	for _, h := range each { // each house alone is under the cap (a fresh copy for each)
		if _, err := rewardOps(t, "").LandsraadApplyWith(g, applyArgs(h)); err != nil {
			t.Fatalf("one house is under the cap: %v", err)
		}
	}
	_, err := o.LandsraadApplyWith(g, applyArgs(each...))
	if err == nil || !strings.Contains(err.Error(), "counting every house in it") || !strings.Contains(err.Error(), "Nothing was changed") || !strings.Contains(err.Error(), "fill fewer houses at once") {
		t.Fatalf("two houses over the cap together: %v", err)
	}
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
}

// A step so small that the number of missions does not fit a whole number is refused, not wrapped round into a negative count that still writes rows.
func TestGrantRefusesAStepTooSmallToCount(t *testing.T) {
	for _, m := range []float64{1e-300, 1e-320, 1e-6} {
		o := rewardOps(t, "")
		before := grantRows(t, o)
		g := GrantSettings{ContributionMultiplier: m, SpecializationXPMultiplier: 1}
		for name, h := range map[string]map[string]any{"from a contribution": house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}), "from none": house(176, Args{"mine_to": 6000.0})} {
			_, err := o.LandsraadApplyWith(g, applyArgs(h))
			if err == nil || !strings.Contains(err.Error(), "more than the 1000 missions one change may grant") || !strings.Contains(err.Error(), "Nothing was changed") {
				t.Fatalf("multiplier %g %s: %v", m, name, err)
			}
			if grantRows(t, o) != before {
				t.Fatalf("multiplier %g %s: a refused request changed the save", m, name)
			}
		}
	}
}

// A faction change writes only player_faction; the contribution row is per faction. What the character reached under the old faction is ground already paid, so the same raise under the
// new faction pays no mission and no reward again (it used to be covered by the editor's event, which is per player and task).
func TestGrantAFactionChangeDoesNotMakeTheSameRaisePayAgain(t *testing.T) {
	o := rewardOps(t, "")
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	solari, scrip, rewardsBefore := wallet(t, o, 0), wallet(t, o, 1), rows(t, o, `select * from landsraad_house_rewards order by 1,2,4`)
	setup(t, o, `update player_faction set faction_id=1 where actor_id=1`)
	m := applyWith(t, o, doubled(), house(175, Args{"was_mine": 0.0, "mine_to": 6000.0}))
	if m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 0 || m["solari"].(int64) != 0 {
		t.Fatalf("a raise under the new faction paid again: %v", m)
	}
	if wallet(t, o, 0) != solari || wallet(t, o, 1) != scrip || rows(t, o, `select * from landsraad_house_rewards order by 1,2,4`) != rewardsBefore {
		t.Fatal("wallets or reward rows changed")
	}
	// the contribution row of the new faction is written, the old one stays
	if got := rows(t, o, `select faction_id, amount from landsraad_task_player_contributions where player_id=1 and task_id=175 order by faction_id`); got != "map[amount:6000 faction_id:1]\nmap[amount:6000 faction_id:2]\n" {
		t.Fatalf("contribution rows: %s", got)
	}
	if got := events(t, o); got != "" {
		t.Fatalf("an event was written: %s", got)
	}
	// ground reached under the old faction covers only what it reached: a tier above it is paid
	o = rewardOps(t, "")
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 2400.0}))
	setup(t, o, `update player_faction set faction_id=1 where actor_id=1`)
	if m := applyWith(t, o, doubled(), house(175, Args{"was_mine": 0.0, "mine_to": 4800.0})); m["reward_rows"].(int64) != 2 {
		t.Fatalf("the tiers above what the old faction reached are paid: %v", m)
	}
}

// Lowering and raising one tier of an open house does not add the same ground to the faction's progress again, and a repeated click on the highest tier does not pay twice (#777).
func TestGrantARemovalAndTheSameRaiseDoesNotAddTheFactionsProgressAgain(t *testing.T) {
	o := rewardOps(t, "")
	applyWith(t, o, DefaultGrantSettings(), house(176, Args{"mine_to": 1200.0}))
	if v, _ := progress(t, o, 2, 176); v != 1200 {
		t.Fatalf("faction progress after the first raise: %v", v)
	}
	applyWith(t, o, DefaultGrantSettings(), house(176, Args{"was_mine": 1200.0, "mine_to": 0.0, "lower": true}))
	applyWith(t, o, DefaultGrantSettings(), house(176, Args{"mine_to": 1200.0}))
	if v, _ := progress(t, o, 2, 176); v != 1200 {
		t.Fatalf("faction progress after the same raise again: %v", v)
	}
	if wallet(t, o, 0) != 2*6000 { // two missions at the stock step for 1,200, paid once
		t.Fatalf("Solari: %d", wallet(t, o, 0))
	}
	// a higher tier adds only the ground above what was paid
	applyWith(t, o, DefaultGrantSettings(), house(176, Args{"was_mine": 1200.0, "mine_to": 2400.0}))
	if v, _ := progress(t, o, 2, 176); v != 2400 {
		t.Fatalf("faction progress after a higher tier: %v", v)
	}
}

// Without the swatch tables (the older layout) the top tier delivers the task's own item; the editor remembers the ground it paid, so after a removal the same raise does not deliver it again (#777).
func TestGrantTheTopTierAfterARemovalWithoutSwatchTablesIsNotDeliveredAgain(t *testing.T) {
	o := mineOps(t, goalWorld)
	mustApply(t, o, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	own := func() string {
		return rows(t, o, `select amount from landsraad_house_rewards where player_id=1 and template_id='Swatch.Kenola'`)
	}
	if got := own(); got != "map[amount:1]\n" {
		t.Fatalf("the task's own item after the first raise: %s", got)
	}
	mustApply(t, o, applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true})))
	if got := own(); got != "" { // the waiting reward of the removed top tier is taken back (#944)
		t.Fatalf("the removal left the top tier's own item: %s", got)
	}
	// the raise writes it again, once, and pays no mission
	if m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 4800.0, "mine_to": 6000.0}))); m["reward_rows"].(int64) != 1 || m["rewards_already_given"].(int64) != 0 || m["missions"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if got := own(); got != "map[amount:1]\n" {
		t.Fatalf("the top tier's own item is not delivered once: %s", got)
	}
}

// The same with the rewards collected (#944): a removal takes back nothing, so the Fill after it writes no reward row and says the tier was given already.
func TestGrantAFillAfterARemovalOverCollectedRewardsWritesNoRowAndPaysNothing(t *testing.T) {
	o := rewardOps(t, "")
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	collectRewards(t, o)
	after := grantRows(t, o)
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true}))
	m := applyWith(t, o, doubled(), house(175, Args{"was_mine": 4800.0, "mine_to": 6000.0}))
	if m["reward_rows"].(int64) != 0 || m["missions"].(int64) != 0 || m["solari"].(int64) != 0 || m["rewards_already_given"].(int64) != 1 {
		t.Fatalf("a Fill after a removal of a collected top tier: %v", m)
	}
	if got := grantRows(t, o); got != after {
		t.Fatalf("something besides the contribution moved:\n%s\n---\n%s", after, got)
	}
}

// Without the swatch tables, with the top tier's own item collected: the same.
func TestGrantACollectedTopTierAfterARemovalWithoutSwatchTablesIsNotDeliveredAgain(t *testing.T) {
	o := mineOps(t, goalWorld)
	mustApply(t, o, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	setup(t, o, `update landsraad_house_rewards set amount=0 where player_id=1`)
	mustApply(t, o, applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true})))
	if m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 4800.0, "mine_to": 6000.0}))); m["reward_rows"].(int64) != 0 || m["rewards_already_given"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if got := rows(t, o, `select amount from landsraad_house_rewards where player_id=1 and template_id='Swatch.Kenola'`); got != "map[amount:0]\n" {
		t.Fatalf("the top tier's own item is delivered again: %s", got)
	}
}

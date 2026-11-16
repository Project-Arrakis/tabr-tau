package ops

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// Leftovers of the review of the mission grant (#752, #754, #762). World: rewardOps (landsraad_rewards_test.go); the character is player 1 in Harkonnen (2); Kenola (175, Gathering)
// is open with the faction at 1,000 and the character at 1,300; Vernius (176, Sabotage) has no row of the character.

// a second house of the Sabotage line, one with no line and a contribution that is not on a mission step
const leftoverHouses = `insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (177, 7, 3, 'DA_HouseWydras', 4000), (178, 7, 4, 'DA_HouseNobody', 4000);
	insert into landsraad_task_rewards(task_id, threshold, template_id, amount) values
	  (177, 1200, 'SolarisCoin', 10000), (177, 6000, 'Swatch.Wydras', 1),
	  (178, 1200, 'SolarisCoin', 1), (178, 1800, 'Schematic.N', 1);
	insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 2, 178, 1234.5);`

// events of the character for Kenola, made under a faction, that cover this much of the contribution
func coverKenola(faction int, amount int) string {
	return fmt.Sprintf(`insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (5, %d, 175, 0, 0, %d, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (5, 1);`, faction, amount)
}

func wantRefusal2(t *testing.T, err error, kind error, contains ...string) {
	t.Helper()
	if err == nil || !errors.Is(err, kind) {
		t.Fatalf("want a refusal of kind %v, got %v", kind, err)
	}
	for _, c := range contains {
		if !strings.Contains(err.Error(), c) {
			t.Fatalf("the refusal %q does not say %q", err.Error(), c)
		}
	}
}

// ---------------------------------------------------------------- #752: the write log names the amounts, and a track at its last level is not silent

func TestGrantTheWriteLogNamesTheMissionsAndTheirAmounts(t *testing.T) {
	// one house
	o := rewardOps(t, "")
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	if d := lastEdit(t, o); d != "landsraad Kenola: your contribution 1300 -> 6000, completed, its faction reached the goal; 2 missions: +12,000 Solari, +2,000 House Scrip, +6,000 Gathering XP; 4 reward rows" {
		t.Fatalf("one house: %q", d)
	}
	// several houses: the totals, the lines in the order of the Specialization tab, and the list of houses last
	o = rewardOps(t, "")
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}), house(176, Args{"mine_to": 3600.0}))
	if d := lastEdit(t, o); d != "landsraad: 2 houses changed from the board; 4 missions: +24,000 Solari, +4,000 House Scrip, +6,000 Gathering XP, +6,000 Sabotage XP; 7 reward rows; houses: Kenola, Vernius" {
		t.Fatalf("two houses: %q", d)
	}
	// ground the character's own events cover pays nothing, and the line says nothing about missions
	o = rewardOps(t, coverKenola(2, 6000))
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	if d := lastEdit(t, o); strings.Contains(d, "mission") || strings.Contains(d, "Solari") || strings.Contains(d, "reward row") {
		t.Fatalf("a raise that paid nothing names amounts: %q", d)
	}
}

func TestGrantAnswerAndWriteLogNameTheExperienceThatDidNotFit(t *testing.T) {
	// a track at its last level: nothing is added, all of it is left out
	o := rewardOps(t, `insert into specialization_tracks(player_id, track_type, xp_amount, level) values (1, 5, 44182, 100);`)
	m := applyWith(t, o, doubled(), house(176, Args{"mine_to": 6000.0}))
	if len(m["specialization_xp"].(map[string]int64)) != 0 || m["specialization_xp_not_added"].(map[string]int64)["Sabotage"] != 9000 {
		t.Fatalf("answer: %v", m)
	}
	if d := lastEdit(t, o); !strings.Contains(d, "9,000 Sabotage XP not added (last level)") || strings.Contains(d, "+9,000 Sabotage XP") {
		t.Fatalf("write log %q", d)
	}
	// a track that has room for part of it
	o = rewardOps(t, `insert into specialization_tracks(player_id, track_type, xp_amount, level) values (1, 5, 44000, 99.7);`)
	m = applyWith(t, o, doubled(), house(176, Args{"mine_to": 6000.0}))
	if m["specialization_xp"].(map[string]int64)["Sabotage"] != maxSpecXP-44000 || m["specialization_xp_not_added"].(map[string]int64)["Sabotage"] != 9000-(maxSpecXP-44000) {
		t.Fatalf("answer: %v", m)
	}
	// a track with room for all of it: the field is there and empty, and the log says nothing
	o = rewardOps(t, "")
	m = applyWith(t, o, doubled(), house(176, Args{"mine_to": 6000.0}))
	if left, ok := m["specialization_xp_not_added"].(map[string]int64); !ok || len(left) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if d := lastEdit(t, o); strings.Contains(d, "not added") {
		t.Fatalf("write log %q", d)
	}
}

// two houses of one line in one request, the first takes the track to its last level: the second adds nothing and the totals in the answer add up (#754)
func TestGrantTwoHousesOfOneLineShareTheRoomOfTheTrack(t *testing.T) {
	o := rewardOps(t, leftoverHouses+`insert into specialization_tracks(player_id, track_type, xp_amount, level) values (1, 5, 44000, 99.7);`)
	m := applyWith(t, o, doubled(), house(176, Args{"mine_to": 6000.0}), house(177, Args{"mine_to": 6000.0}))
	room := int64(maxSpecXP - 44000)
	if got := m["specialization_xp"].(map[string]int64)["Sabotage"]; got != room {
		t.Fatalf("added %d, the track had room for %d: %v", got, room, m)
	}
	if got := m["specialization_xp_not_added"].(map[string]int64)["Sabotage"]; got != 2*9000-room {
		t.Fatalf("left out %d: %v", got, m)
	}
	if xp, level, _ := track(t, o, 5); xp != maxSpecXP || level != 100 {
		t.Fatalf("the track: %d %v", xp, level)
	}
	if m["missions"].(int64) != 6 || m["solari"].(int64) != 36000 {
		t.Fatalf("the wallets are paid for both houses: %v", m)
	}
}

// ---------------------------------------------------------------- #754: failures at the write steps the first test did not reach

func TestGrantAnInterruptedWriteAtTheOtherStepsLeavesNothingHalfWritten(t *testing.T) {
	const out = `select raise(abort, 'disk full'); end;`
	clock := `update landsraad_tasks set completion_time=999 where id=174;` // a time outside the term's window, so that the clock repair at the end of the edit writes
	request := func() Args {
		return applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}), house(176, Args{"mine_to": 3600.0}))
	}
	// without a trigger the request writes every step, the repair included
	ok := rewardOps(t, clock)
	if m, err := ok.LandsraadApplyWith(doubled(), request()); err != nil || m.(map[string]any)["repaired"].(int64) != 1 {
		t.Fatalf("the request without a failure: %v %v", m, err)
	}
	for name, trigger := range map[string]string{
		"the faction's contribution (insert)": `create trigger boom before insert on landsraad_task_faction_contributions begin ` + out,
		"the faction's contribution (update)": `create trigger boom before update on landsraad_task_faction_contributions begin ` + out,
		"the completion of the house":         `create trigger boom before update on landsraad_tasks when new.id=175 begin ` + out,
		"the House Scrip wallet":              `create trigger boom before insert on player_virtual_currency_balances when new.currency_id=1 begin ` + out,
		"the clock repair after every step":   `create trigger boom before update on landsraad_tasks when new.id=174 begin ` + out,
	} {
		o := rewardOps(t, clock)
		before := grantRows(t, o)
		setup(t, o, trigger)
		queued := len(o.S.Pending()) // the setup is itself an edit
		if _, err := o.LandsraadApplyWith(doubled(), request()); err == nil {
			t.Fatalf("%s: the failing write was accepted", name)
		}
		if got := len(o.S.Pending()); got != queued {
			t.Errorf("%s: the failed write left an edit in the list: %d, want %d", name, got, queued)
		}
		setup(t, o, `drop trigger boom`)
		if after := grantRows(t, o); after != before {
			t.Errorf("%s: a failed write left rows behind:\n%s\n---\n%s", name, before, after)
		}
	}
}

// the editor writes no event, no player link and no cursor move (#655), so a failing trigger on those tables is never reached
func TestGrantNeverTouchesTheProgressTables(t *testing.T) {
	o := rewardOps(t, "")
	for _, tr := range []string{
		`create trigger a before insert on landsraad_task_progress begin select raise(abort, 'no'); end;`,
		`create trigger b before update on landsraad_task_progress begin select raise(abort, 'no'); end;`,
		`create trigger c before insert on landsraad_task_progress_player begin select raise(abort, 'no'); end;`,
		`create trigger d before update on landsraad_task_progress_processed begin select raise(abort, 'no'); end;`,
		`create trigger e before insert on landsraad_task_progress_processed begin select raise(abort, 'no'); end;`,
	} {
		setup(t, o, tr)
	}
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}))); err != nil {
		t.Fatalf("a raise reached a progress table: %v", err)
	}
}

// ---------------------------------------------------------------- #762.1: a house taken or reopened since the page was drawn is refused for a raise and a removal too

func TestGrantARaiseOnAHouseTakenSinceThePageWasDrawnIsRefused(t *testing.T) {
	taken := `update landsraad_tasks set completed=1, winning_faction_id=1, completion_time=140 where id=175;`
	o := rewardOps(t, taken)
	before := grantRows(t, o)
	// the page showed the house open
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_done": false, "was_mine": 1300.0, "mine_to": 6000.0})))
	wantRefusal2(t, err, apierr.ErrConflict, "taken or reopened since you looked", "nothing was changed")
	// the page showed another winner
	_, err = o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_done": true, "was_win": 2.0, "was_mine": 1300.0, "mine_to": 6000.0})))
	wantRefusal2(t, err, apierr.ErrConflict, "taken or reopened since you looked")
	// the same for a tier removal (Maros is taken by Harkonnen, not shown as open)
	_, err = o.LandsraadApplyWith(doubled(), applyArgs(house(174, Args{"was_done": false, "was_mine": 7850.0, "mine_to": 4800.0, "lower": true})))
	wantRefusal2(t, err, apierr.ErrConflict, "taken or reopened since you looked")
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
	// what the page showed is what is there: written
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_done": true, "was_win": 1.0, "was_mine": 1300.0, "mine_to": 6000.0}))); err != nil {
		t.Fatal(err)
	}
	// a request that does not say what the house was (an older caller) is not refused for it
	o = rewardOps(t, taken)
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))); err != nil {
		t.Fatal(err)
	}
}

func TestGrantARaiseWhoseEventsAreNotWhatThePageShowedIsRefused(t *testing.T) {
	o := rewardOps(t, coverKenola(2, 1200))
	before := grantRows(t, o)
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_events": 0.0, "was_mine": 1300.0, "mine_to": 6000.0})))
	wantRefusal2(t, err, apierr.ErrConflict, "earlier progress on Kenola is not what you were shown", "nothing was changed")
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_events": 1200.0, "was_mine": 1300.0, "mine_to": 6000.0}))); err != nil {
		t.Fatal(err)
	}
	// a removal does not depend on the events
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_events": 99.0, "was_mine": 6000.0, "mine_to": 2400.0, "lower": true}))); err != nil {
		t.Fatalf("a removal: %v", err)
	}
	// and a request without the number is not refused for it
	o = rewardOps(t, coverKenola(2, 1200))
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))); err != nil {
		t.Fatalf("no number: %v", err)
	}
}

// ---------------------------------------------------------------- #762.3: Fill faction goal on a house another faction took retakes it and names the win it takes away

func TestGrantFillFactionGoalOnAHouseTheOtherFactionTookNamesTheWinItTakesAway(t *testing.T) {
	o := rewardOps(t, `update landsraad_tasks set completed=1, winning_faction_id=1, completion_time=140 where id=175;`)
	m := applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_done": true, "was_win": 1.0, "was_mine": 1300.0, "mine_to": 6000.0, "complete_for": 2.0}))
	r := m["replaced"].([]map[string]any)
	if len(r) != 1 || r[0]["house"] != "Kenola" || r[0]["previous_winner"] != "Atreides" {
		t.Fatalf("replaced: %v", m["replaced"])
	}
	if d := lastEdit(t, o); !strings.Contains(d, "Atreides") {
		t.Fatalf("write log %q", d)
	}
	if got := events(t, o); got != "" {
		t.Fatalf("an event was written: %s", got)
	}
}

// ---------------------------------------------------------------- #762.4: a raise that writes no experience is not refused for the experience

func TestGrantRefusesALineOrATrackOnlyWhenExperienceWouldBeWritten(t *testing.T) {
	// a house with no line: a raise that stands for no mission is written
	o := rewardOps(t, leftoverHouses)
	m := applyWith(t, o, doubled(), house(178, Args{"was_mine": 1234.5, "mine_to": 1800.0}))
	if m["missions"].(int64) != 0 || m["contribution_raised"].(int64) != 1 || m["reward_rows"].(int64) != 1 {
		t.Fatalf("inside one mission step: %v", m)
	}
	if wallet(t, o, 0) != 0 {
		t.Fatal("a wallet was paid for no mission")
	}
	// and so is one whose experience rounds to nothing, the Solari still paid
	o = rewardOps(t, leftoverHouses)
	m = applyWith(t, o, GrantSettings{ContributionMultiplier: 0.5, SpecializationXPMultiplier: 0.0001}, house(178, Args{"was_mine": 1234.5, "mine_to": 1800.0}))
	if m["missions"].(int64) != 1 || m["solari"].(int64) != 6000 || len(m["specialization_xp"].(map[string]int64)) != 0 {
		t.Fatalf("experience that rounds to nothing: %v", m)
	}
	// two lines on one track number: refused when a mission is written, not when none is
	g := doubled()
	g.TrackNumbers = map[string]int64{"Sabotage": 1}
	o = rewardOps(t, leftoverHouses+`insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (179, 7, 5, 'DA_HouseRichese', 4000);
		insert into landsraad_task_rewards(task_id, threshold, template_id, amount) values (179, 1200, 'SolarisCoin', 1), (179, 1800, 'Schematic.R', 1);
		insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 2, 179, 1234.5);`)
	if _, err := o.LandsraadApplyWith(g, applyArgs(house(179, Args{"was_mine": 1234.5, "mine_to": 1800.0}))); err != nil {
		t.Fatalf("no mission, no experience: %v", err)
	}
	if _, err := o.LandsraadApplyWith(g, applyArgs(house(176, Args{"mine_to": 3600.0}))); err == nil || !strings.Contains(err.Error(), "both use number 1") {
		t.Fatalf("a mission with experience on a doubled track number: %v", err)
	}
	// still refused when a mission is written for a house with no line
	if _, err := o.LandsraadApplyWith(GrantSettings{ContributionMultiplier: 0.5, SpecializationXPMultiplier: 1}, applyArgs(house(178, Args{"was_mine": 1234.5, "mine_to": 1800.0}))); err == nil || !strings.Contains(err.Error(), "Nobody is not a house the editor knows a specialization line for") {
		t.Fatalf("an unknown house: %v", err)
	}
}

// ---------------------------------------------------------------- #754: cases that had no test

func TestGrantAFactionReachingTheGoalInTheMiddleOfARequestOfSeveralHouses(t *testing.T) {
	o := rewardOps(t, "")
	m := applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}), house(176, Args{"mine_to": 3600.0}))
	if m["completed"].(int64) != 1 || m["changed"].(int64) != 2 || m["missions"].(int64) != 8 {
		t.Fatalf("answer: %v", m)
	}
	if h := houseRow(t, o, 175); h["c"].(int64) != 1 || h["w"].(int64) != 2 {
		t.Fatalf("Kenola, past its goal: %v", h)
	}
	if h := houseRow(t, o, 176); h["c"].(int64) != 0 {
		t.Fatalf("Vernius, short of its goal, was taken: %v", h)
	}
	if v, _ := progress(t, o, 2, 176); v != 3600 {
		t.Fatalf("Vernius's faction progress: %v", v)
	}
}

// Decision (#754): events of the character count as ground whatever faction they were made under, so a change of faction in the same term does not let a Fill pay the same missions again.
func TestGrantEventsUnderAnotherFactionStillCoverTheGround(t *testing.T) {
	o := rewardOps(t, coverKenola(2, 6000)+`update player_faction set faction_id=1 where actor_id=1;`)
	m := applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": 0.0, "mine_to": 6000.0}))
	if m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 0 || m["solari"].(int64) != 0 || wallet(t, o, 0) != 0 || wallet(t, o, 1) != 0 {
		t.Fatalf("a faction change let the same ground pay again: %v", m)
	}
}

// the events of another character of the save are not this character's cover, and none of their rows changes
func TestGrantAnotherCharactersEventsAreNotCoverAndStayAsTheyAre(t *testing.T) {
	other := `insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (5, 2, 175, 0, 0, 6000, 120);
		insert into landsraad_task_progress_player(progress_id, player_id) values (5, 50);`
	o := rewardOps(t, other)
	queue, theirs := progressTables(t, o), rows(t, o, `select * from landsraad_task_player_contributions where player_id=50 order by task_id`)
	m := applyWith(t, o, DefaultGrantSettings(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
	if m["missions"].(int64) != 4 || m["solari"].(int64) != 24000 {
		t.Fatalf("the other character's events counted as cover: %v", m)
	}
	if progressTables(t, o) != queue || rows(t, o, `select * from landsraad_task_player_contributions where player_id=50 order by task_id`) != theirs {
		t.Fatal("a row of another character changed")
	}
}

// a contribution that is not on a step counts the steps it crosses: ceil(after / step) - ceil(before / step)
func TestGrantAContributionNotOnAStepCountsTheStepsItCrosses(t *testing.T) {
	for _, c := range []struct {
		g    GrantSettings
		want int64
	}{{DefaultGrantSettings(), 6 - 2}, {doubled(), 3 - 1}} {
		o := rewardOps(t, `update landsraad_task_player_contributions set amount=1234.5 where player_id=1 and task_id=175;`)
		if m := applyWith(t, o, c.g, house(175, Args{"was_mine": 1234.5, "mine_to": 6000.0})); m["missions"].(int64) != c.want {
			t.Fatalf("step %v: %v, want %d", c.g.ContributionMultiplier, m, c.want)
		}
	}
}

// a multiplier of 0 stands for no number of missions, and one so small that a raise counts over 1,000 is refused (through the route in internal/web as well)
func TestGrantAMultiplierOfZeroOrNearZeroRefusesARaiseThatPays(t *testing.T) {
	o := rewardOps(t, "")
	before := grantRows(t, o)
	_, err := o.LandsraadApplyWith(GrantSettings{ContributionMultiplier: 0, SpecializationXPMultiplier: 1}, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	wantRefusal2(t, err, apierr.ErrBadArgument, "LandsraadContributionMultiplier is 0", "Nothing was changed")
	_, err = o.LandsraadApplyWith(GrantSettings{ContributionMultiplier: 0.001, SpecializationXPMultiplier: 1}, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	wantRefusal2(t, err, apierr.ErrBadArgument, "more than the 1000 missions one change may grant for one house", "Nothing was changed")
	if grantRows(t, o) != before {
		t.Fatal("a refused request changed the save")
	}
}

// a raise whose ground is covered pays nothing and needs no amounts, so a setting that cannot be used does not refuse it (the design note says so)
func TestGrantACoveredRaiseIsNotRefusedForASettingItDoesNotNeed(t *testing.T) {
	bad := doubled()
	bad.Problem = "The Landsraad amounts cannot be worked out: LandsraadContributionMultiplier in ServerCustomSettings.ini is \"x\", not a number from 0 to 10. Nothing was changed."
	for name, g := range map[string]GrantSettings{"a value that cannot be read": bad, "a multiplier of 0": {ContributionMultiplier: 0, SpecializationXPMultiplier: 1}} {
		o := rewardOps(t, coverKenola(2, 6000))
		m := applyWith(t, o, g, house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}))
		if m["missions"].(int64) != 0 || m["contribution_raised"].(int64) != 1 || wallet(t, o, 0) != 0 {
			t.Fatalf("%s: %v", name, m)
		}
	}
}

// the board read carries the room each track has left, so the line before a click does not overstate near the last level (#762.2)
func TestGrantSpecRoomIsWhatEachTrackStillHasRoomFor(t *testing.T) {
	o := rewardOps(t, `insert into specialization_tracks(player_id, track_type, xp_amount, level) values (1, 2, 40000, 90), (1, 5, 99999999, 100);`)
	room := o.SpecRoom(DefaultGrantSettings())
	if room["Gathering"] != int64(maxSpecXP-40000) || room["Sabotage"] != 0 || room["Crafting"] != int64(maxSpecXP) {
		t.Fatalf("room: %v", room)
	}
	// a learned number moves the line to its track
	g := DefaultGrantSettings()
	g.TrackNumbers = map[string]int64{"Gathering": 9}
	if room = o.SpecRoom(g); room["Gathering"] != int64(maxSpecXP) {
		t.Fatalf("room with a learned number: %v", room)
	}
}

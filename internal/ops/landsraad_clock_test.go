package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The clock of landsraad_tasks.completion_time (#465). The fixture has the shapes of the rows read in a real save (the old character's, term 10 of 305,395 to 341,395 on the
// game's play clock): House Sor (231) is complete and was written by the game, completion_time 312,412 (inside the window); Hurata (228) and Lindaren (240) were completed by
// the editor before the fix and hold Unix time (1,793,118,379 and 1,793,118,361); Kenola (233) and Vernius (234) are open (completed 0, no winner, NULL time). Term 9 has one
// editor-written house (200) that is not part of the board. The farm_variables row is the one of that save: the play clock at the time of the save is
// (1793123333243000 - 1792016530566000 - 788709987000) / 1,000,000 = 318,092 seconds.
const clockWorld = `
insert or ignore into factions(id, name) values (1, 'Atreides'), (2, 'Harkonnen'), (3, 'None');
insert into farm_variables(one_row, farm_id, universe_time_timestamp, universe_lastactive_timestamp, down_time_accumulation) values (1, null, 1792016530566000, 1793123333243000, 788709987000);
insert into landsraad_decree_term(term_id, start_time, end_time) values (9, 275678, 305395), (10, 305395, 341395);
insert into landsraad_tasks(id, term_id, board_index, house_name, completed, winning_faction_id, goal_amount, completion_time) values
  (200, 9, 3, 'DA_HouseDyvik', 1, 1, 4000, 1793000000),
  (231, 10, 5, 'DA_HouseSor', 1, 2, 4000, 312412),
  (228, 10, 2, 'DA_HouseHurata', 1, 2, 4000, 1793118379),
  (240, 10, 14, 'DA_HouseLindaren', 1, 2, 4000, 1793118361),
  (233, 10, 7, 'DA_HouseKenola', 0, null, 4000, null),
  (234, 10, 8, 'DA_HouseVernius', 0, null, 4000, null);
insert into landsraad_task_faction_contributions(faction_id, task_id, amount) values (2, 231, 4000), (2, 228, 4000), (2, 240, 4000);
`

const playClock = 318092 // what the farm_variables row above gives

func clockOps(t *testing.T, extra ...string) *Ops {
	s := testsave.PlayerWithSQL(t, clockWorld)
	o := &Ops{S: s}
	for _, q := range extra {
		setup(t, o, q)
	}
	return o
}

// clockApply sends the board's write for houses of term 10 (the clock world) and answers as LandsraadApply does.
func clockApply(o *Ops, houses ...map[string]any) (any, error) {
	a := applyArgs(houses...)
	a["term_id"] = 10.0
	return o.LandsraadApply(a)
}

// completeBoth completes the two open houses of the clock world (233 and 234) for Harkonnen; again is true when they are complete already (a second request).
func completeBoth(o *Ops, again bool) (any, error) {
	st := Args{}
	if again {
		st = Args{"was_done": true, "was_win": 2.0}
	}
	h := func(id float64) map[string]any {
		a := Args{"complete_for": 2.0}
		for k, v := range st {
			a[k] = v
		}
		return house(id, a)
	}
	return clockApply(o, h(233), h(234))
}

func ctOf(t *testing.T, o *Ops, id int64) any {
	t.Helper()
	return mustOne(t, o, `select completion_time ct from landsraad_tasks where id=?`, id)["ct"]
}

func TestCompletingAHouseWritesTheGamesPlayClockNotUnixTime(t *testing.T) {
	o := clockOps(t)
	if _, err := clockApply(o, house(233, Args{"complete_for": 2.0})); err != nil {
		t.Fatal(err)
	}
	r := mustOne(t, o, `select completed c, winning_faction_id w, completion_time ct from landsraad_tasks where id=233`)
	if r["c"].(int64) != 1 || r["w"].(int64) != 2 || r["ct"] != int64(playClock) {
		t.Fatalf("a house completed now is complete, won by the faction, at the play clock %d: %v", playClock, r)
	}
	if ct := ctOf(t, o, 233).(int64); ct < 305395 || ct > 341395 {
		t.Fatalf("the time must lie inside the term's window like every value the game wrote: %d", ct)
	}
}

func TestCompletingTwoHousesWritesTheGamesPlayClock(t *testing.T) {
	o := clockOps(t)
	got, err := completeBoth(o, false)
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"].(int64) != 2 {
		t.Fatalf("answer: %v", m)
	}
	for _, id := range []int64{233, 234} {
		if ct := ctOf(t, o, id); ct != int64(playClock) {
			t.Errorf("house %d: completion_time = %v, want the play clock %d", id, ct, playClock)
		}
	}
}

func TestTheClockIsHeldInsideTheTermWindow(t *testing.T) {
	// the term is over (the clock is past its end): the end of the term
	o := clockOps(t, `update landsraad_decree_term set end_time=310000 where term_id=10`)
	if _, err := clockApply(o, house(233, Args{"complete_for": 2.0})); err != nil {
		t.Fatal(err)
	}
	if ct := ctOf(t, o, 233); ct != int64(310000) {
		t.Errorf("past the end of the term: %v", ct)
	}
	// the clock is before the term's start (not seen in a save): the start of the term
	o = clockOps(t, `update landsraad_decree_term set start_time=320000, end_time=356000 where term_id=10`)
	if _, err := completeBoth(o, false); err != nil {
		t.Fatal(err)
	}
	if ct := ctOf(t, o, 233); ct != int64(320000) {
		t.Errorf("before the start of the term: %v", ct)
	}
}

func TestWithoutAPlayClockTheTermStartIsWritten(t *testing.T) {
	o := clockOps(t, `delete from farm_variables`)
	if _, err := clockApply(o, house(233, Args{"complete_for": 2.0})); err != nil {
		t.Fatal(err)
	}
	if ct := ctOf(t, o, 233); ct != int64(305395) {
		t.Errorf("no farm_variables row: %v", ct)
	}
}

func TestReopenGivesTheShapeOfAnOpenRowTheGameWrote(t *testing.T) {
	o := clockOps(t)
	// a row the game wrote complete is reopened to completed 0, NULL winner and NULL time (the game's open rows hold NULL, never 0)
	if _, err := clockApply(o, house(231, Args{"was_done": true, "was_win": 2.0, "reopen": true})); err != nil {
		t.Fatal(err)
	}
	r := mustOne(t, o, `select completed c, winning_faction_id w, completion_time ct from landsraad_tasks where id=231`)
	if r["c"].(int64) != 0 || r["w"] != nil || r["ct"] != nil {
		t.Fatalf("an open row: %v", r)
	}
	got, err := clockApply(o, house(228, Args{"was_done": true, "was_win": 2.0, "reopen": true}))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["changed"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if r := mustOne(t, o, `select completed c, winning_faction_id w, completion_time ct from landsraad_tasks where id=228`); r["c"].(int64) != 0 || r["w"] != nil || r["ct"] != nil {
		t.Fatalf("a row the editor wrote with Unix time is reopened to an open row: %v", r)
	}
}

// assertRepaired checks the board after a repair: the editor's Unix times of the term are on the play clock, the game's own time is untouched, an open house is still open
// and the earlier term's row is left alone.
func assertRepaired(t *testing.T, o *Ops, wantOpen ...int64) {
	t.Helper()
	for _, id := range []int64{228, 240} {
		if got := ctOf(t, o, id); got == nil || got.(int64) != playClock {
			if !contains(wantOpen, id) {
				t.Errorf("house %d: completion_time = %v, want the play clock %d", id, got, playClock)
			}
		}
	}
	if got := ctOf(t, o, 231); got != int64(312412) && !contains(wantOpen, 231) {
		t.Errorf("a time the game wrote is not touched: %v", got)
	}
	if got := ctOf(t, o, 200); got != int64(1793000000) {
		t.Errorf("a row of an earlier term is not touched: %v", got)
	}
}

func contains(xs []int64, x int64) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

func TestCompleteOneHouseBringsTheEditorsUnixTimesOfTheTermOntoThePlayClock(t *testing.T) {
	o := clockOps(t)
	if _, err := clockApply(o, house(233, Args{"complete_for": 2.0})); err != nil {
		t.Fatal(err)
	}
	assertRepaired(t, o)
}

func TestReopenOneHouseBringsTheEditorsUnixTimesOfTheTermOntoThePlayClock(t *testing.T) {
	o := clockOps(t)
	if _, err := clockApply(o, house(231, Args{"was_done": true, "was_win": 2.0, "reopen": true})); err != nil {
		t.Fatal(err)
	}
	assertRepaired(t, o, 231)
	if ctOf(t, o, 231) != nil {
		t.Errorf("the reopened house holds no time: %v", ctOf(t, o, 231))
	}
}

func TestCompletingAndReopeningRepairTheTermsUnixTimes(t *testing.T) {
	o := clockOps(t)
	got, err := completeBoth(o, false)
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["repaired"].(int64) != 2 {
		t.Fatalf("two houses held Unix time: %v", m)
	}
	assertRepaired(t, o)

	o = clockOps(t)
	got, err = clockApply(o, house(231, Args{"was_done": true, "was_win": 2.0, "reopen": true}))
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["repaired"].(int64) != 2 || m["changed"].(int64) != 1 {
		t.Fatalf("one house reopened and two repaired: %v", m)
	}
	assertRepaired(t, o, 231)
}

func TestARepairNeedsNoConfirmationAndDoesNotChangeTheCountThePageShowed(t *testing.T) {
	o := clockOps(t)
	// 228 is complete for faction 2 with its progress at the goal, so it is skipped (expected 0): its Unix time is repaired all the same and nothing is refused
	got, err := clockApply(o, house(228, Args{"was_done": true, "was_win": 2.0, "complete_for": 2.0}))
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]any)
	if m["changed"].(int64) != 0 || m["skipped"].(int64) != 1 || m["repaired"].(int64) != 2 {
		t.Fatalf("answer: %v", m)
	}
	assertRepaired(t, o)
}

func TestARepairIsDoneOnce(t *testing.T) {
	o := clockOps(t)
	if _, err := completeBoth(o, false); err != nil {
		t.Fatal(err)
	}
	pending := len(o.S.Pending())
	got, err := completeBoth(o, true)
	if err != nil {
		t.Fatal(err)
	}
	if m := got.(map[string]any); m["repaired"].(int64) != 0 || m["noop"] != true {
		t.Fatalf("nothing is left to do: %v", m)
	}
	if len(o.S.Pending()) != pending {
		t.Fatalf("no edit when nothing changes: %v", o.S.Pending())
	}
}

func TestARepairLeavesAnOpenHouseAndATimeInsideTheWindowAlone(t *testing.T) {
	o := clockOps(t, `update landsraad_tasks set completion_time=330000 where id=240`)
	if _, err := o.Landsraad(); err != nil {
		t.Fatal(err)
	}
	if _, err := clockApply(o, house(233, Args{"complete_for": 2.0})); err != nil {
		t.Fatal(err)
	}
	if got := ctOf(t, o, 240); got != int64(330000) {
		t.Errorf("a time inside the term's window is the game's shape and stays: %v", got)
	}
	if got := ctOf(t, o, 234); got != nil {
		t.Errorf("an open house stays open: %v", got)
	}
}

func TestTheClockReadsTheRowsOfARealSaveShape(t *testing.T) {
	// the same sums on the other saves read: the value is the play clock at the save, inside the latest term
	for _, c := range []struct {
		ut, la, dt int64
		want       int64
	}{
		{1790722581277000, 1791525466252000, 741394300000, 61490},  // term 2 of 31,563 to 67,563
		{1792016530566000, 1792383620324000, 262611098000, 104478}, // term 4 of 95,678 to 131,678
		{1792016530566000, 1793034299865000, 770895314000, 246873}, // term 8 of 239,678 to 275,678
		{1793129227450000, 1793145154900000, 1863421000, 14064},    // a new character, term 1 of 0 to 86,400
	} {
		s := testsave.PlayerWithSQL(t, "")
		o := &Ops{S: s}
		setup(t, o, `insert into farm_variables(one_row, universe_time_timestamp, universe_lastactive_timestamp, down_time_accumulation) values (1, ?, ?, ?)`, c.ut, c.la, c.dt)
		var got int64
		var ok bool
		if _, err := s.Mutate("read", func(m *save.Mut) error { got, ok = gameClockNow(m); return nil }); err != nil {
			t.Fatal(err)
		}
		if !ok || got != c.want {
			t.Errorf("clock = %d (%v), want %d", got, ok, c.want)
		}
	}
}

// The read gives the position of the save on the play clock and the real time of the save (#922), and nothing is invented when the save has no usable clock.
func TestLandsraadReadGivesTheSaveClock(t *testing.T) {
	o := clockOps(t)
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	c, _ := res.(map[string]any)["clock"].(map[string]any)
	if c == nil || c["play_seconds"] != int64(playClock) || c["saved_at"] != int64(1793123333) {
		t.Errorf("clock = %v, want play_seconds %d and saved_at 1793123333", c, playClock)
	}
	for _, q := range []string{`update farm_variables set universe_lastactive_timestamp = universe_time_timestamp`, `delete from farm_variables`} {
		o2 := clockOps(t, q)
		res, err := o2.Landsraad()
		if err != nil {
			t.Fatal(err)
		}
		if c, ok := res.(map[string]any)["clock"].(map[string]any); ok && c != nil {
			t.Errorf("%s: clock = %v, want none", q, c)
		}
	}
}

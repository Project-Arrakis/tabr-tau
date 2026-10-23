package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// ---- Level and XP of the character

func TestResetLevelPutsXpAtZeroAndTakesBackTheSkillPointsTheLevelsGave(t *testing.T) {
	o := xpOps(t, 41221, 70, 9) // level 71
	before := digest(t, o, nil, -10)
	r, err := o.ResetLevel(Args{"expected": 41221.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["changed"].(int64) != 41221 || m["noop"] != false || m["levelBefore"].(int) != 71 {
		t.Fatalf("answer: %v", m)
	}
	// 71 levels gave 71 points (the rule Give XP follows), but only 9 are unspent: the other 61 are spent in skills and stay, and the total still covers them
	if xp, total, unspent := levelState(t, o); xp != 0 || total != 61 || unspent != 0 {
		t.Fatalf("state after: xp=%d total=%d unspent=%d", xp, total, unspent)
	}
	if m := r.(map[string]any); m["pointsTaken"].(int64) != 9 || m["pointsKept"].(int64) != 62 {
		t.Fatalf("the answer says how many points were taken and how many stayed: %v", m)
	}
	row := mustOne(t, o, `select json_extract(components,'$.FLevelComponent[1].LastViewedSkillTree') v, json_extract(components,'$.FLevelComponent[1].ModuleData."Skills.Key.Mentat1".SkillPointsSpent') x,
		json_extract(components,'$.FHealthComponent[1].m_CurrentHealth') h, typeof(components) ty from fgl_entities where entity_id=-10`)
	if row["v"] != "Mentat" || row["x"].(int64) != 1 || row["h"].(float64) != 100 || row["ty"] != "blob" {
		t.Fatalf("the skills and the rest of the record must survive: %v", row)
	}
	if digest(t, o, nil, -10) != before {
		t.Fatal("something outside the level record changed")
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "reset level and xp (41221 XP, level 71 to 0)" {
		t.Fatalf("one write-log entry: %q", p)
	}
	assertDBSound(t, o)
}

func TestResetLevelKeepsPointsThatDidNotComeFromLevels(t *testing.T) {
	o := xpOps(t, 1000, 10, 7) // level 4: four points came from levels, the other 6 total and 3 of the unspent did not
	if _, err := o.ResetLevel(Args{"expected": 1000.0}); err != nil {
		t.Fatal(err)
	}
	if xp, total, unspent := levelState(t, o); xp != 0 || total != 6 || unspent != 3 {
		t.Fatalf("state after: xp=%d total=%d unspent=%d", xp, total, unspent)
	}
}

// Points that are spent in skills are not taken back: the total never falls below the points spent (total minus unspent is what the skills hold).
func TestResetLevelNeverLeavesFewerTotalPointsThanAreSpent(t *testing.T) {
	for _, c := range []struct{ xp, total, unspent int }{
		{1000, 20, 0},   // level 4, every point spent: nothing can be taken back
		{1000, 20, 2},   // two unspent: two taken, not four
		{41221, 70, 9},  // level 71, nine unspent
		{1000, 3, 3},    // fewer points than the levels gave: only the three that exist
		{1000, 100, 50}, // plenty: the four levels' points come off both
	} {
		o := xpOps(t, c.xp, c.total, c.unspent)
		if _, err := o.ResetLevel(Args{"expected": float64(c.xp)}); err != nil {
			t.Fatal(err)
		}
		_, total, unspent := levelState(t, o)
		if spent := int64(c.total - c.unspent); total < spent || total-unspent != spent || unspent < 0 {
			t.Fatalf("%+v: total=%d unspent=%d, but %d points are spent", c, total, unspent, spent)
		}
	}
}

// A character with every point spent keeps them all; the answer says so.
func TestResetLevelWithAllPointsSpentTakesNothingBackAndSaysSo(t *testing.T) {
	o := xpOps(t, 1000, 20, 0)
	r, err := o.ResetLevel(Args{"expected": 1000.0})
	if err != nil {
		t.Fatal(err)
	}
	if xp, total, unspent := levelState(t, o); xp != 0 || total != 20 || unspent != 0 {
		t.Fatalf("state after: xp=%d total=%d unspent=%d", xp, total, unspent)
	}
	if m := r.(map[string]any); m["pointsTaken"].(int64) != 0 || m["pointsKept"].(int64) != 4 {
		t.Fatalf("answer: %v", m)
	}
}

// The reset is the inverse of the Give XP edit: giving XP to a fresh level record and resetting returns the record byte for byte.
func TestResetLevelUndoesGiveXPExactly(t *testing.T) {
	o := xpOps(t, 0, 0, 0)
	q := `select json(components) c from fgl_entities where entity_id=-10`
	start := mustOne(t, o, q)["c"]
	if _, err := o.AddXP(Args{"amount": 1000.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.ResetLevel(Args{"expected": 1000.0}); err != nil {
		t.Fatal(err)
	}
	if mustOne(t, o, q)["c"] != start {
		t.Fatalf("not back to the start:\n%v\n%v", start, mustOne(t, o, q)["c"])
	}
}

func TestResetLevelTwiceIsANoOp(t *testing.T) {
	o := xpOps(t, 500, 3, 3)
	if _, err := o.ResetLevel(Args{"expected": 500.0}); err != nil {
		t.Fatal(err)
	}
	was, pending := digest(t, o, nil, 0), len(o.S.Pending())
	r, err := o.ResetLevel(Args{"expected": 0.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["changed"].(int64) != 0 || m["noop"] != true {
		t.Fatalf("answer: %v", m)
	}
	if digest(t, o, nil, 0) != was || len(o.S.Pending()) != pending {
		t.Fatal("nothing to do means nothing written and no log entry")
	}
}

func TestResetLevelRefusesAStaleCountABadCountAndASaveWithoutTheRecord(t *testing.T) {
	o := xpOps(t, 500, 3, 3)
	was := digest(t, o, nil, 0)
	for _, n := range []float64{0, 499, 501} {
		_, err := o.ResetLevel(Args{"expected": n})
		wantRefusal(t, err, apierr.ErrConflict, "stale count")
	}
	for _, a := range []Args{{}, {"expected": -5.0}, {"expected": "x"}} {
		_, err := o.ResetLevel(a)
		wantRefusal(t, err, apierr.ErrBadArgument, "bad count")
	}
	if digest(t, o, nil, 0) != was || o.S.Dirty() {
		t.Fatal("a refused reset must change nothing")
	}
	if _, err := (&Ops{S: testsave.Player(t)}).ResetLevel(Args{"expected": 0.0}); err == nil {
		t.Fatal("a save with no level record is refused")
	}
}

// ---- XP of the specialization tracks

const specResetWorld = specWorld + `
insert into specialization_tracks(player_id,track_type,xp_amount,level) values (1,3,500,4.0),(1,7,900,8.0);`

func specResetOps(t *testing.T) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, specResetWorld)}
}

var knownTracks = map[string]int64{"Combat": 2, "Crafting": 3, "Gathering": 4} // 4 has no row

func TestSpecsShowsWhatAResetOfTheKnownTracksWouldChange(t *testing.T) {
	o := specResetOps(t)
	r, err := o.Specs(knownTracks)
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["resetCount"].(int64) != 2 {
		t.Fatalf("two known tracks have a row: %v", m["resetCount"])
	}
	if un := m["unassigned"].([]SpecRaw); len(un) != 1 || un[0].Number != 7 {
		t.Fatalf("the row of the unknown track is listed as left alone: %+v", un)
	}
}

func TestResetSpecTracksClearsTheKnownTracksOnly(t *testing.T) {
	o := specResetOps(t)
	before := digest(t, o, map[string]bool{"specialization_tracks": true}, 0)
	r, err := o.ResetSpecTracks(knownTracks, Args{"expected": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["changed"].(int64) != 2 || m["noop"] != false {
		t.Fatalf("answer: %v", m)
	}
	rows, err := o.S.Query(`select track_type n, xp_amount xp from specialization_tracks order by track_type`)
	if err != nil || len(rows) != 1 || rows[0]["n"].(int64) != 7 || rows[0]["xp"].(int64) != 900 {
		t.Fatalf("only the row of the track whose number is not known is left: %v %v", rows, err)
	}
	// keystones, and every other table, are untouched
	if mustOne(t, o, `select count(*) c from purchased_specialization_keystones`)["c"].(int64) != 1 {
		t.Fatal("keystones must not be touched")
	}
	if digest(t, o, map[string]bool{"specialization_tracks": true}, 0) != before {
		t.Fatal("something outside the track rows changed")
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "reset specialization XP (2 changed)" {
		t.Fatalf("log: %q", p)
	}
	assertDBSound(t, o)
}

func TestResetSpecTracksEqualsResettingEachTrack(t *testing.T) {
	a, b := specResetOps(t), specResetOps(t)
	if _, err := a.ResetSpecTracks(knownTracks, Args{"expected": 2.0}); err != nil {
		t.Fatal(err)
	}
	for _, tr := range []string{"Combat", "Crafting", "Gathering"} {
		if _, err := b.ResetSpec(knownTracks, Args{"track": tr}); err != nil {
			t.Fatal(err)
		}
	}
	if digest(t, a, nil, 0) != digest(t, b, nil, 0) {
		t.Fatal("the reset of all tracks and the reset of each track ended differently")
	}
}

func TestResetSpecTracksNoOpStaleCountAndNoKnownTracks(t *testing.T) {
	o := specResetOps(t)
	for _, n := range []float64{0, 1, 3} {
		_, err := o.ResetSpecTracks(knownTracks, Args{"expected": n})
		wantRefusal(t, err, apierr.ErrConflict, "stale count")
	}
	_, err := o.ResetSpecTracks(knownTracks, Args{})
	wantRefusal(t, err, apierr.ErrBadArgument, "no count")
	if o.S.Dirty() {
		t.Fatal("a refused reset must change nothing")
	}
	// with no track number learned there is nothing the editor can reset, and it says so rather than touching a row
	r, err := o.ResetSpecTracks(nil, Args{"expected": 0.0})
	if err != nil || r.(map[string]any)["noop"] != true || o.S.Dirty() {
		t.Fatalf("nothing known, nothing done: %v %v", r, err)
	}
	if _, err := o.ResetSpecTracks(knownTracks, Args{"expected": 2.0}); err != nil {
		t.Fatal(err)
	}
	was, pending := digest(t, o, nil, 0), len(o.S.Pending())
	r, err = o.ResetSpecTracks(knownTracks, Args{"expected": 0.0})
	if err != nil || r.(map[string]any)["noop"] != true {
		t.Fatalf("second time: %v %v", r, err)
	}
	if digest(t, o, nil, 0) != was || len(o.S.Pending()) != pending {
		t.Fatal("a repeat writes nothing")
	}
}

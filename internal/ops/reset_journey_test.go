package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Journey rows of the character (1), as a played save has them: finished story steps (one with a reward still waiting), a step
// that is only revealed, a codex entry, a contract (derived from tags, read-only on the page), a step with no finished state, and a
// finished step of another character. Tutorials: one started, one complete, one with state 0.
const journeyResetWorld = `
insert into accounts(id,"user") values (9,'other-account');
insert into player_state(id, account_id, character_name) values (7, 9, 'Other');
insert into journey_story_node(character_id,story_node_id,complete_condition_state,reveal_condition_state,fail_condition_state,metadata_state,has_pending_reward) values
  (1,'Quest.A',x'01',x'01',x'0c',x'0c',0),
  (1,'Quest.A.child1',x'01',x'01',x'0c',x'0c',1),
  (1,'Quest.B',x'01',x'01',x'0c',x'0c',1),
  (1,'Quest.C',x'0c',x'01',x'0c',x'0c',0),
  (1,'DA_Dunipedia_Thing',x'01',x'01',x'0c',x'0c',0),
  (1,'DA_CT_SomeContract',x'01',x'01',x'0c',x'0c',0),
  (1,'Quest.D',x'0c',x'0c',x'0c',x'0c',0),
  (7,'Quest.Z',x'01',x'01',x'0c',x'0c',0);
insert into tutorials(id,name) values (11,'TUT_One'),(12,'TUT_Two'),(13,'TUT_Three');
insert into tutorial_per_player(player_id,tutorial_id,tutorial_state) values (1,11,1),(1,12,2),(1,13,0);
insert into player_tags(character_id,tag) values (1,'Some.Tag');`

func journeyResetOps(t *testing.T) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, journeyResetWorld)}
}

// the Story and Codex nodes complete now are Quest.A, Quest.A.child1, Quest.B and DA_Dunipedia_Thing (4); the contract is not one of them;
// the tutorials with a state are 11 and 12 (2).
const journeyResetCount = 6

func TestJourneyShowsHowManyAResetWouldChange(t *testing.T) {
	r, err := journeyResetOps(t).JourneyBrowse()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["resetCount"].(int64) != journeyResetCount || m["resetNodes"].(int64) != 4 || m["resetTutorials"].(int64) != 2 {
		t.Fatalf("counts: %v %v %v", m["resetCount"], m["resetNodes"], m["resetTutorials"])
	}
}

func TestResetJourneyUnfinishesStoryAndCodexNodesAndTutorialsAndNothingElse(t *testing.T) {
	o := journeyResetOps(t)
	tables := map[string]bool{"journey_story_node": true, "tutorial_per_player": true}
	before := digest(t, o, tables, 0)
	r, err := o.ResetJourney(Args{"expected": float64(journeyResetCount)})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["changed"].(int64) != journeyResetCount || m["noop"] != false {
		t.Fatalf("answer: %v", m)
	}
	// finished nodes are no longer finished; the rows stay, revealed, as with the Reset button of a row
	for _, id := range []string{"Quest.A", "Quest.A.child1", "Quest.B", "DA_Dunipedia_Thing"} {
		if _, done := journeyState(t, o, id); done {
			t.Errorf("%s must no longer be complete", id)
		}
		if exists, _ := journeyState(t, o, id); !exists {
			t.Errorf("%s: the row must stay", id)
		}
	}
	// the contract is derived from tags and not part of this; another character's rows are not touched
	if _, done := journeyState(t, o, "DA_CT_SomeContract"); !done {
		t.Error("a contract row is left alone")
	}
	if n := mustOne(t, o, `select count(*) c from journey_story_node where character_id=7 and complete_condition_state=x'01'`)["c"].(int64); n != 1 {
		t.Error("another character's journey is not touched")
	}
	// rewards are not granted or lost by the reset: the pending flags are as they were
	if n := mustOne(t, o, `select count(*) c from journey_story_node where character_id=1 and has_pending_reward=1`)["c"].(int64); n != 2 {
		t.Errorf("reward-pending flags changed: %d", n)
	}
	// the node that was never revealed stays unrevealed (it was not complete, so the reset has nothing to do with it)
	if r := mustOne(t, o, `select reveal_condition_state=x'01' rv from journey_story_node where character_id=1 and story_node_id='Quest.D'`); r["rv"].(int64) != 0 {
		t.Error("a node that was not finished is not touched")
	}
	// tutorials: the ones with a state are back to not started (no row), the one at 0 is as it was
	rows, _ := o.S.Query(`select tutorial_id id, tutorial_state s from tutorial_per_player order by tutorial_id`)
	if len(rows) != 1 || rows[0]["id"].(int64) != 13 || rows[0]["s"].(int64) != 0 {
		t.Errorf("tutorials: %v", rows)
	}
	// tags, and every other table, are untouched
	if mustOne(t, o, `select count(*) c from player_tags`)["c"].(int64) != 1 {
		t.Error("tags must not be touched")
	}
	if digest(t, o, tables, 0) != before {
		t.Fatal("something outside the journey and tutorial rows changed")
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "reset journey (6 changed: 4 nodes, 2 tutorials)" {
		t.Fatalf("log: %q", p)
	}
	assertDBSound(t, o)
}

// The reset is the Reset button of each row applied to every finished node, in one edit.
func TestResetJourneyEqualsResettingEachNodeAndTutorial(t *testing.T) {
	a, b := journeyResetOps(t), journeyResetOps(t)
	if _, err := a.ResetJourney(Args{"expected": float64(journeyResetCount)}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"Quest.A", "Quest.A.child1", "Quest.B", "DA_Dunipedia_Thing"} {
		if _, err := b.JourneySet(Args{"node_id": id, "complete": false}); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []float64{11, 12} {
		if _, err := b.TutorialSet(Args{"id": id, "complete": false}); err != nil {
			t.Fatal(err)
		}
	}
	if digest(t, a, nil, 0) != digest(t, b, nil, 0) {
		t.Fatal("the reset and the per-row resets ended differently")
	}
}

func TestResetJourneyTwiceIsANoOp(t *testing.T) {
	o := journeyResetOps(t)
	if _, err := o.ResetJourney(Args{"expected": float64(journeyResetCount)}); err != nil {
		t.Fatal(err)
	}
	was, pending := digest(t, o, nil, 0), len(o.S.Pending())
	r, err := o.ResetJourney(Args{"expected": 0.0})
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

func TestResetJourneyRefusesAStaleOrMissingCount(t *testing.T) {
	o := journeyResetOps(t)
	was := digest(t, o, nil, 0)
	for _, n := range []float64{0, 4, 5, 7} {
		_, err := o.ResetJourney(Args{"expected": n})
		wantRefusal(t, err, apierr.ErrConflict, "stale count")
	}
	for _, a := range []Args{{}, {"expected": -1.0}, {"expected": "six"}} {
		_, err := o.ResetJourney(a)
		wantRefusal(t, err, apierr.ErrBadArgument, "bad count")
	}
	if digest(t, o, nil, 0) != was || o.S.Dirty() {
		t.Fatal("a refused reset must change nothing")
	}
}

func TestResetJourneyKeepsHiddenChildrenOfFinishedNodesAsTheRowResetDoes(t *testing.T) {
	// the Reset button of a row reveals the node and its children; the reset of all does the same and says so (see the page text)
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into journey_story_node(character_id,story_node_id,complete_condition_state,reveal_condition_state,fail_condition_state,metadata_state) values
		(1,'Quest.P',x'01',x'01',x'0c',x'0c'),(1,'Quest.P.hidden',x'0c',x'0c',x'0c',x'0c');`)}
	if _, err := o.ResetJourney(Args{"expected": 1.0}); err != nil {
		t.Fatal(err)
	}
	r := mustOne(t, o, `select reveal_condition_state=x'01' rv from journey_story_node where story_node_id='Quest.P.hidden'`)
	if r["rv"].(int64) != 1 {
		t.Fatal("a child is revealed, as the Reset button of the row does")
	}
}

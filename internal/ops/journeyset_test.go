package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

const journeyNodes = `insert into journey_story_node(character_id,story_node_id,complete_condition_state,reveal_condition_state,fail_condition_state,metadata_state) values
  (1,'Quest.A.child1',x'0c',x'0c',x'0c',x'0c'),(1,'Quest.A.child2',x'0c',x'0c',x'0c',x'0c'),
  (1,'Quest_A.other',x'0c',x'0c',x'0c',x'0c'),(1,'QuestXA.other',x'0c',x'0c',x'0c',x'0c'),(1,'quest.a.lower',x'0c',x'0c',x'0c',x'0c'),
  (1,'Quest.AB',x'0c',x'0c',x'0c',x'0c');`

func journeyState(t *testing.T, o *Ops, id string) (exists bool, complete bool) {
	t.Helper()
	r := mustOne(t, o, `select count(*) c, coalesce(sum(complete_condition_state=x'01'),0) done from journey_story_node where character_id=1 and story_node_id=?`, id)
	return r["c"].(int64) == 1, r["done"].(int64) == 1
}

// ARCH-5: a node that has child rows but no row of its own was left incomplete while the call reported success, and "_" in the
// id matched any character (and case was ignored) when the children were found.
func TestJourneySetCompletesTheNodeItselfWhenOnlyChildrenHaveRows(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, journeyNodes)}
	if exists, _ := journeyState(t, o, "Quest.A"); exists {
		t.Fatal("test setup: the node must have no row of its own")
	}
	r, err := o.JourneySet(Args{"node_id": "Quest.A", "complete": true})
	if err != nil {
		t.Fatal(err)
	}
	if exists, done := journeyState(t, o, "Quest.A"); !exists || !done {
		t.Fatalf("the node's own row must exist and be complete: exists=%v complete=%v", exists, done)
	}
	for _, c := range []string{"Quest.A.child1", "Quest.A.child2"} {
		if _, done := journeyState(t, o, c); !done {
			t.Errorf("%s must be complete", c)
		}
	}
	if rows := r.(map[string]any)["rows"].(int64); rows != 3 {
		t.Errorf("rows = %d, want the 2 children and the new node row", rows)
	}
	assertDBSound(t, o)
}

func TestJourneySetOnlyTouchesTheNodeAndItsRealChildren(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, journeyNodes)}
	if _, err := o.JourneySet(Args{"node_id": "Quest.A", "complete": true}); err != nil {
		t.Fatal(err)
	}
	// none of these is a child of "Quest.A": "_" is not a wildcard, "." is not any character, case counts, and a longer name is not a child
	for _, other := range []string{"Quest_A.other", "QuestXA.other", "quest.a.lower", "Quest.AB"} {
		if _, done := journeyState(t, o, other); done {
			t.Errorf("%s is not a child of Quest.A and must not be completed", other)
		}
	}
	// the underscore node completes its own child and still leaves Quest.A's alone
	if _, err := o.JourneySet(Args{"node_id": "Quest_A", "complete": true}); err != nil {
		t.Fatal(err)
	}
	if _, done := journeyState(t, o, "Quest_A.other"); !done {
		t.Error("Quest_A.other is a child of Quest_A")
	}
	if _, done := journeyState(t, o, "QuestXA.other"); done {
		t.Error("QuestXA.other must not match the underscore")
	}
}

func TestJourneyResetDoesNotCreateRows(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, journeyNodes)}
	if _, err := o.JourneySet(Args{"node_id": "Quest.A", "complete": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.JourneySet(Args{"node_id": "Quest.A", "complete": false}); err != nil {
		t.Fatal(err)
	}
	if _, done := journeyState(t, o, "Quest.A.child1"); done {
		t.Error("a reset un-completes the children")
	}
	if exists, done := journeyState(t, o, "Quest.A"); !exists || done {
		t.Errorf("the node row stays, no longer complete: exists=%v complete=%v", exists, done)
	}
	if _, err := o.JourneySet(Args{"node_id": "Never.Seen", "complete": false}); err != nil {
		t.Fatal(err)
	}
	if exists, _ := journeyState(t, o, "Never.Seen"); exists {
		t.Error("resetting a node with no rows must not create one")
	}
	// completing a node that has no rows at all still creates it
	if _, err := o.JourneySet(Args{"node_id": "Never.Seen", "complete": true}); err != nil {
		t.Fatal(err)
	}
	if exists, done := journeyState(t, o, "Never.Seen"); !exists || !done {
		t.Errorf("a node with no rows is created complete: exists=%v complete=%v", exists, done)
	}
}

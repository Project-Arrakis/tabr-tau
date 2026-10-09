package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/journey"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A character with a finished main-quest node and its child, a revealed sibling, a not yet revealed one, a codex entry, a contract
// node of the journey table, three tutorials in the three states, and the tags of one catalog contract.
func journeyWorld(t *testing.T) string {
	t.Helper()
	contracts := journey.Contracts()
	if len(contracts) == 0 || len(journey.ContractTags(contracts[0])) == 0 {
		t.Fatal("the catalog has no contract with tags")
	}
	tagSQL := ""
	for _, tag := range journey.ContractTags(contracts[0]) {
		tagSQL += "insert into player_tags(character_id,tag) values (1,'" + tag + "');\n"
	}
	return `
insert into journey_story_node(character_id,story_node_id,has_pending_reward,complete_condition_state,reveal_condition_state,fail_condition_state,metadata_state,reset_group) values
  (1,'DA_MQ_ANewBeginning',0,jsonb('true'),jsonb('true'),jsonb('{}'),jsonb('{}'),0),
  (1,'DA_MQ_ANewBeginning.Wake',1,jsonb('true'),jsonb('true'),jsonb('{}'),jsonb('{}'),0),
  (1,'DA_MQ_ANewBeginning.Walk',0,jsonb('{}'),jsonb('true'),jsonb('{}'),jsonb('{}'),0),
  (1,'DA_MQ_ANewBeginning.Run',0,jsonb('{}'),jsonb('{}'),jsonb('{}'),jsonb('{}'),0),
  (1,'DA_Dunipedia_KnownUniverse',0,jsonb('true'),jsonb('true'),jsonb('{}'),jsonb('{}'),0),
  (1,'DA_Dunipedia_KnownUniverse.CHOAM',0,jsonb('{}'),jsonb('true'),jsonb('{}'),jsonb('{}'),0),
  (1,'DA_CT_Test_Contract',0,jsonb('{}'),jsonb('true'),jsonb('{}'),jsonb('{}'),0);
insert into tutorials(id,name) values (1,'DA_Tut_First'),(2,'DA_Tut_Second'),(3,'DA_Tut_Third');
insert into tutorial_per_player(player_id,tutorial_id,tutorial_state) values (1,1,2),(1,2,1);
` + tagSQL
}

func TestJourneyBrowseGroupsNamesAndStatus(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, journeyWorld(t))}
	r, err := o.JourneyBrowse()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	by := func(rows []JourneyRow) map[string]JourneyRow {
		out := map[string]JourneyRow{}
		for _, x := range rows {
			out[x.ID] = x
		}
		return out
	}
	story := by(m["story"].([]JourneyRow))
	root, wake, walk, run := story["DA_MQ_ANewBeginning"], story["DA_MQ_ANewBeginning.Wake"], story["DA_MQ_ANewBeginning.Walk"], story["DA_MQ_ANewBeginning.Run"]
	if root.Status != "Complete" || wake.Status != "Complete" || walk.Status != "Revealed" || run.Status != "Incomplete" {
		t.Fatalf("status: %v %v %v %v", root.Status, wake.Status, walk.Status, run.Status)
	}
	if !wake.PendingReward || walk.PendingReward {
		t.Fatalf("pending reward flags: %v %v", wake.PendingReward, walk.PendingReward)
	}
	if root.Depth != 0 || wake.Depth != 1 || wake.Parent != "DA_MQ_ANewBeginning" || !wake.Actionable {
		t.Fatalf("tree: %+v", wake)
	}
	if root.Name != journey.DisplayName("DA_MQ_ANewBeginning") || root.Name == "" {
		t.Fatalf("name: %q", root.Name)
	}
	// children follow their parent in the list, not sorted away from it
	rows := m["story"].([]JourneyRow)
	ri := -1
	for i, x := range rows {
		if x.ID == "DA_MQ_ANewBeginning" {
			ri = i
		}
	}
	if ri < 0 || ri+1 >= len(rows) || rows[ri+1].Parent != "DA_MQ_ANewBeginning" {
		t.Fatalf("the first child must follow its parent")
	}
	if len(by(m["codex"].([]JourneyRow))) != 2 || by(m["codex"].([]JourneyRow))["DA_Dunipedia_KnownUniverse.CHOAM"].Depth != 1 {
		t.Fatalf("codex: %v", m["codex"])
	}
	contract := by(m["contract"].([]JourneyRow))
	if _, ok := contract["DA_CT_Test_Contract"]; !ok || contract["DA_CT_Test_Contract"].Actionable {
		t.Fatalf("a journey contract node is listed and read-only: %v", contract["DA_CT_Test_Contract"])
	}
	first := journey.Contracts()[0]
	if c := contract[first]; !c.Complete || c.Status != "Complete" || c.Actionable {
		t.Fatalf("a contract whose tags are all on the character is complete: %+v", c)
	}
	var other JourneyRow
	for _, id := range journey.Contracts()[1:] {
		if len(journey.ContractTags(id)) > 0 {
			other = contract[id]
			break
		}
	}
	if other.Complete {
		t.Fatalf("a contract with tags the character lacks is not complete: %+v", other)
	}
	tut := m["tutorial"].([]JourneyRow)
	states := map[string]string{}
	for _, x := range tut {
		states[x.ID] = x.Status
	}
	if states["1"] != "Complete" || states["2"] != "Started" || states["3"] != "Not Started" {
		t.Fatalf("tutorials: %v", states)
	}
}

func TestJourneyBrowseWithNoJourneyDataStillListsTheCatalog(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	r, err := o.JourneyBrowse()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if len(m["story"].([]JourneyRow)) == 0 {
		t.Fatal("the catalog's story nodes are listed even when the save has none")
	}
	for _, x := range m["story"].([]JourneyRow) {
		if x.Complete {
			t.Fatalf("nothing is complete in an empty save: %+v", x)
		}
	}
}

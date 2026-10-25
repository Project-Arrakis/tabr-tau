package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A new character has 95 journey steps and four tutorials already finished (read from a save made right after the character was created), so a reset of the
// journey does not undo them: it would take the character below a new one.
const journeyStarterWorld = journeyResetWorld + `
insert into journey_story_node(character_id,story_node_id,complete_condition_state,reveal_condition_state,fail_condition_state,metadata_state,has_pending_reward) values
  (1,'DA_MQ_NPEAutocompleted',x'01',x'01',x'0c',x'0c',0),
  (1,'DA_MQ_NPEAutocompleted.Escape.TalkToZantara',x'01',x'01',x'0c',x'0c',0),
  (1,'DA_ACH_SteamAchievements.sb-ach-combat1',x'01',x'01',x'0c',x'0c',0);
insert into tutorials(id,name) values (14,'Movement'),(15,'TechTree');
insert into tutorial_per_player(player_id,tutorial_id,tutorial_state) values (1,14,2),(1,15,2);`

func TestResetJourneyLeavesWhatANewCharacterHasFinished(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, journeyStarterWorld)}
	// the count the page shows is the same as before the starter steps were added: they are not part of it
	r, err := o.JourneyBrowse()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["resetCount"].(int64) != journeyResetCount || m["resetNodes"].(int64) != 4 || m["resetTutorials"].(int64) != 2 {
		t.Fatalf("the steps and tutorials a new character has are not counted: %v %v %v", m["resetCount"], m["resetNodes"], m["resetTutorials"])
	}
	if _, err := o.ResetJourney(Args{"expected": float64(journeyResetCount)}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"DA_MQ_NPEAutocompleted", "DA_MQ_NPEAutocompleted.Escape.TalkToZantara", "DA_ACH_SteamAchievements.sb-ach-combat1"} {
		if _, done := journeyState(t, o, id); !done {
			t.Errorf("%s is finished in a new character and stays finished", id)
		}
	}
	if n := mustOne(t, o, `select count(*) c from tutorial_per_player where tutorial_id in (14,15) and tutorial_state=2`)["c"].(int64); n != 2 {
		t.Errorf("the tutorials a new character has complete stay complete: %d", n)
	}
	if _, done := journeyState(t, o, "Quest.A"); done {
		t.Error("a quest is still reset")
	}
}

func TestTheStarterListsAreWhatWasReadFromANewCharacter(t *testing.T) {
	if len(starterJourneyNodes) != 95 || len(starterTutorials) != 4 {
		t.Fatalf("%d steps and %d tutorials", len(starterJourneyNodes), len(starterTutorials))
	}
	for _, id := range []string{"DA_MQ_NPEAutocompleted", "DA_Dunipedia_KnownUniverse.Imperium.ImperiumFragment1Title"} {
		if !starterJourneyNodes[id] {
			t.Errorf("%s is missing", id)
		}
	}
}

// A step resets its children too: resetting a Dunipedia entry that a new character does not have finished must not take the fragment below it that a new
// character has finished (found on a copy of a played save: 17 of the 95 were lost that way).
func TestResetJourneyKeepsAStarterStepThatIsAChildOfAResetStep(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, journeyResetWorld+`
insert into journey_story_node(character_id,story_node_id,complete_condition_state,reveal_condition_state,fail_condition_state,metadata_state,has_pending_reward) values
  (1,'DA_Dunipedia_KnownUniverse.Imperium',x'01',x'01',x'0c',x'0c',0),
  (1,'DA_Dunipedia_KnownUniverse.Imperium.ImperiumFragment1Title',x'01',x'01',x'0c',x'0c',0),
  (1,'DA_Dunipedia_KnownUniverse.Imperium.ImperiumFragment1Title.ImperiumFragment1Content',x'01',x'01',x'0c',x'0c',0);`)}
	r, err := o.JourneyBrowse()
	if err != nil {
		t.Fatal(err)
	}
	n := r.(map[string]any)["resetCount"].(int64)
	if _, err := o.ResetJourney(Args{"expected": float64(n)}); err != nil {
		t.Fatal(err)
	}
	if _, done := journeyState(t, o, "DA_Dunipedia_KnownUniverse.Imperium"); done {
		t.Error("the entry a new character does not have finished is reset")
	}
	for _, id := range []string{"DA_Dunipedia_KnownUniverse.Imperium.ImperiumFragment1Title", "DA_Dunipedia_KnownUniverse.Imperium.ImperiumFragment1Title.ImperiumFragment1Content"} {
		if _, done := journeyState(t, o, id); !done {
			t.Errorf("%s is finished in a new character and stays finished", id)
		}
	}
}

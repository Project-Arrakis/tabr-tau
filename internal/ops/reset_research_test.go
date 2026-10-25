package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A character who has researched a lot. HealthPackRecipe is one of the four entries a new character starts with (read from a save of a
// brand-new character); T4_Rifle_Recipe, Sandbike_Mk2 (purchased, its recipe missing), BasicLighting and ChoamShelterSet (two building sets) and
// the group marker are bought on top of that. RCP_Unbought is not bought but its recipe is known (a schematic found in the world), and
// Stillsuit_Unique_04_recipe is a known recipe the research tree does not list at all: both stay.
const resetResearchProps = `{"TechKnowledgePlayerComponent":{"m_TechKnowledgePoints":10,"m_TechKnowledge":{"m_TechKnowledgeData":[` +
	`{"ItemKey":"RCP_HealthPackRecipe","UnlockedState":"Purchased","bIsNewEntry":false},` +
	`{"ItemKey":"RCP_T4_Rifle_Recipe","UnlockedState":"Purchased","bIsNewEntry":false},` +
	`{"ItemKey":"RCP_Sandbike_Mk2_Recipe","UnlockedState":"Purchased","bIsNewEntry":true},` +
	`{"ItemKey":"BLD_BasicLighting_Patent","UnlockedState":"Purchased","bIsNewEntry":false},` +
	`{"ItemKey":"BLD_ChoamShelterSet","UnlockedState":"Purchased","bIsNewEntry":false},` +
	`{"ItemKey":"DA_GRP_Water","UnlockedState":"Purchased","bIsNewEntry":false},` +
	`{"ItemKey":"RCP_Unbought","UnlockedState":"NotPurchased","bIsNewEntry":true}]}},` +
	`"CraftingRecipesLibraryActorComponent":{"m_KnownItemRecipes":[` +
	`{"BaseRecipeId":{"Name":"HealthPackRecipe"},"m_QualityLevel":0,"m_bIsNew":false,"m_NumberOfRecipeUses":-1,"m_bIsLimitedUseRecipe":false,"m_Source":"SchematicPickup"},` +
	`{"BaseRecipeId":{"Name":"T4_Rifle_Recipe"},"m_QualityLevel":0,"m_bIsNew":false,"m_NumberOfRecipeUses":0,"m_bIsLimitedUseRecipe":false,"m_Source":"SchematicPickup"},` +
	`{"BaseRecipeId":{"Name":"Stillsuit_Unique_04_recipe"},"m_QualityLevel":0,"m_bIsNew":false,"m_NumberOfRecipeUses":1,"m_bIsLimitedUseRecipe":true,"m_Source":"SchematicPickup"},` +
	`{"BaseRecipeId":{"Name":"Unbought"},"m_QualityLevel":0,"m_bIsNew":false,"m_NumberOfRecipeUses":1,"m_bIsLimitedUseRecipe":true,"m_Source":"SchematicPickup"}]}}`

const resetResearchWorld = `update actors set properties=jsonb('` + resetResearchProps + `') where id=2;
insert into building_progression_learned_building_sets(character_id,learned_building_set) values
  (1,'BasicLighting_Patent'),(1,'ChoamShelterSet'),(1,'MTX_Store_Pack_Patent');
insert into building_progression_new_buildable_pieces(character_id,new_buildable_piece) values
  (1,'BasicLighting_Placeable'),(1,'Old_Piece_Placeable');`

// The entries a reset takes back: the five bought ones that a new character does not have (rifle, sandbike, two sets, the group marker).
const resetResearchCount = 5

func resetResearchOps(t *testing.T) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, resetResearchWorld)}
}

func researchState(t *testing.T, o *Ops) map[string]string {
	t.Helper()
	rows, err := o.S.Query(`select json_extract(i.value,'$.ItemKey') k, json_extract(i.value,'$.UnlockedState') s from actors a,
		json_each(a.properties,'` + researchArr + `') i where a.id=2`)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r["k"].(string)] = r["s"].(string)
	}
	return out
}

func knownRecipeNames(t *testing.T, o *Ops) []string {
	t.Helper()
	rows, err := o.S.Query(`select json_extract(i.value,'$.BaseRecipeId.Name') n from actors a, json_each(a.properties,'` + recipesArr + `') i where a.id=2 order by i.key`)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rows {
		out = append(out, r["n"].(string))
	}
	return out
}

func TestResearchShowsHowManyAResetWouldChange(t *testing.T) {
	r, err := resetResearchOps(t).Research()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["resetCount"].(int64) != resetResearchCount || m["resetRecipes"].(int64) != 1 || m["resetSets"].(int64) != 2 {
		t.Fatalf("counts: entries %v recipes %v sets %v", m["resetCount"], m["resetRecipes"], m["resetSets"])
	}
}

func TestResetResearchGivesBackWhatANewCharacterHasAndNothingElse(t *testing.T) {
	o := resetResearchOps(t)
	// the actor row is read on its own below; every other table must be unchanged except the two building tables
	before := digest(t, o, map[string]bool{"building_progression_learned_building_sets": true, "building_progression_new_buildable_pieces": true, "actors": true}, 0)
	r, err := o.ResetResearch(Args{"expected": float64(resetResearchCount)})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["changed"].(int64) != resetResearchCount || m["noop"] != false || m["recipes"].(int64) != 1 || m["sets"].(int64) != 2 {
		t.Fatalf("answer: %v", m)
	}
	st := researchState(t, o)
	if st["RCP_HealthPackRecipe"] != "Purchased" {
		t.Error("the four entries a new character starts with stay bought")
	}
	for _, k := range []string{"RCP_T4_Rifle_Recipe", "RCP_Sandbike_Mk2_Recipe", "BLD_BasicLighting_Patent", "BLD_ChoamShelterSet", "DA_GRP_Water", "RCP_Unbought"} {
		if st[k] != "NotPurchased" {
			t.Errorf("%s = %q, want NotPurchased", k, st[k])
		}
	}
	// the recipes the research gave are gone; the starter recipe, the unlisted one and the one found as a schematic stay, in their order
	if got := strings.Join(knownRecipeNames(t, o), ","); got != "HealthPackRecipe,Stillsuit_Unique_04_recipe,Unbought" {
		t.Errorf("recipes: %s", got)
	}
	// the building sets the research gave are gone, with the "new piece" rows the unlock added; sets the tree does not list stay
	sets := mustOne(t, o, `select group_concat(learned_building_set) s from (select learned_building_set from building_progression_learned_building_sets order by 1)`)["s"]
	if sets != "MTX_Store_Pack_Patent" {
		t.Errorf("sets: %v", sets)
	}
	pieces := mustOne(t, o, `select group_concat(new_buildable_piece) s from building_progression_new_buildable_pieces`)["s"]
	if pieces != "Old_Piece_Placeable" {
		t.Errorf("pieces: %v", pieces)
	}
	// Intel and everything outside the research record are not touched
	if n := mustOne(t, o, `select json_extract(properties,'$.TechKnowledgePlayerComponent.m_TechKnowledgePoints') n from actors where id=2`)["n"].(int64); n != 10 {
		t.Errorf("intel changed: %d", n)
	}
	if after := digest(t, o, map[string]bool{"building_progression_learned_building_sets": true, "building_progression_new_buildable_pieces": true, "actors": true}, 0); after != before {
		t.Error("a table other than the research record and the two building tables changed")
	}
	// the entries a reset puts back are marked new again, as in a new character's list
	if mustOne(t, o, `select json_extract(i.value,'$.bIsNewEntry') n from actors a, json_each(a.properties,'`+researchArr+`') i
		where a.id=2 and json_extract(i.value,'$.ItemKey')='RCP_T4_Rifle_Recipe'`)["n"].(int64) != 1 {
		t.Error("a put-back entry is new again")
	}
}

func TestResetResearchRefusesAStaleCountAndChangesNothing(t *testing.T) {
	o := resetResearchOps(t)
	before := digest(t, o, nil, 0)
	_, err := o.ResetResearch(Args{"expected": float64(resetResearchCount + 1)})
	wantRefusal(t, err, apierr.ErrConflict, "a count that is not the one in the save")
	if o.S.Dirty() || digest(t, o, nil, 0) != before {
		t.Fatal("a refused reset must change nothing")
	}
	_, err = o.ResetResearch(Args{})
	wantRefusal(t, err, apierr.ErrBadArgument, "no count")
}

func TestResetResearchWithNothingToResetIsANoop(t *testing.T) {
	o := resetResearchOps(t)
	if _, err := o.ResetResearch(Args{"expected": float64(resetResearchCount)}); err != nil {
		t.Fatal(err)
	}
	r, err := o.ResetResearch(Args{"expected": 0.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["changed"].(int64) != 0 || m["noop"] != true {
		t.Fatalf("answer: %v", m)
	}
}

func TestResetResearchNeedsTheResearchRecord(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set properties=jsonb('{}') where id=2;`)}
	if _, err := o.ResetResearch(Args{"expected": 0.0}); err == nil {
		t.Fatal("a character without a research record is refused, not reported as nothing to do")
	}
}

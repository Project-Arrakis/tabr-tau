package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The character actor (2) as a real save has it: a research list with purchased and not purchased recipes, buildings and a group,
// and a known-recipes list. The player state (1) has two learned building sets.
const researchProps = `{"TechKnowledgePlayerComponent":{"m_TechKnowledgePoints":10,"m_TechKnowledge":{"m_TechKnowledgeData":[` +
	`{"ItemKey":"RCP_HealthPackRecipe","UnlockedState":"Purchased","bIsNewEntry":false},` +
	`{"ItemKey":"RCP_T4_Rifle_Recipe","UnlockedState":"NotPurchased","bIsNewEntry":true},` +
	`{"ItemKey":"RCP_Sandbike_Mk2_Recipe","UnlockedState":"Purchased","bIsNewEntry":false},` +
	`{"ItemKey":"BLD_BasicLighting_Patent","UnlockedState":"Purchased","bIsNewEntry":false},` +
	`{"ItemKey":"BLD_AugmentStation_Patent","UnlockedState":"NotPurchased","bIsNewEntry":false},` +
	`{"ItemKey":"BLD_ChoamShelterSet","UnlockedState":"NotPurchased","bIsNewEntry":false},` +
	`{"ItemKey":"DA_GRP_Water","UnlockedState":"NotPurchased","bIsNewEntry":false}]}},` +
	`"CraftingRecipesLibraryActorComponent":{"m_KnownItemRecipes":[` +
	`{"BaseRecipeId":{"Name":"HealthPackRecipe"},"m_QualityLevel":0,"m_bIsNew":false,"m_NumberOfRecipeUses":1,"m_bIsLimitedUseRecipe":true,"m_Source":"SchematicPickup"},` +
	`{"BaseRecipeId":{"Name":"Stillsuit_Unique_04_recipe"},"m_QualityLevel":0,"m_bIsNew":false,"m_NumberOfRecipeUses":1,"m_bIsLimitedUseRecipe":true,"m_Source":"SchematicPickup"}]}}`

const researchWorld = `update actors set properties=jsonb('` + researchProps + `') where id=2;
insert into building_progression_learned_building_sets(character_id,learned_building_set) values (1,'BasicLighting_Patent');
insert into building_progression_new_buildable_pieces(character_id,new_buildable_piece) values (1,'Old_Piece_Placeable');`

func researchOps(t *testing.T) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, researchWorld)}
}

func TestResearchListsNamesStatesAndWhatIsReallyThere(t *testing.T) {
	o := researchOps(t)
	r, err := o.Research()
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]ResearchRow{}
	for _, x := range r.(map[string]any)["rows"].([]ResearchRow) {
		by[x.ItemKey] = x
	}
	if len(by) != 7 {
		t.Fatalf("rows: %d", len(by))
	}
	if h := by["RCP_HealthPackRecipe"]; !h.Unlocked || !h.Materialized || h.Type != "Recipe" || h.Name != "Health Pack" || h.NeedsRepair {
		t.Fatalf("a purchased recipe the character knows: %+v", h)
	}
	if s := by["RCP_Sandbike_Mk2_Recipe"]; !s.Purchased || s.Materialized || !s.NeedsRepair || s.Unlocked || s.Category != "Vehicles" {
		t.Fatalf("purchased but its recipe is missing needs repair: %+v", s)
	}
	if b := by["BLD_BasicLighting_Patent"]; !b.Unlocked || b.Type != "Building" || b.UnlockID != "BasicLighting_Patent" {
		t.Fatalf("learned building: %+v", b)
	}
	if g := by["DA_GRP_Water"]; g.Actionable || g.Type != "Group" {
		t.Fatalf("a group marker cannot be unlocked: %+v", g)
	}
	if by["RCP_T4_Rifle_Recipe"].ProductGroup != "Aluminum Products" || by["RCP_T4_Rifle_Recipe"].Category != "Combat" {
		t.Fatalf("category and product group: %+v", by["RCP_T4_Rifle_Recipe"])
	}
}

func TestUnlockResearchRecipeWritesOnlyThatEntryAndAddsTheRecipe(t *testing.T) {
	o := researchOps(t)
	r, err := o.UnlockResearch(Args{"item_key": "RCP_T4_Rifle_Recipe"})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["alreadyPurchased"] != false || m["unlockAdded"] != true {
		t.Fatalf("%v", m)
	}
	row := mustOne(t, o, `select json_extract(properties,'`+researchArr+`[1].UnlockedState') st, json_extract(properties,'`+researchArr+`[1].bIsNewEntry') nw,
		json_extract(properties,'`+researchArr+`[0].UnlockedState') other, json_extract(properties,'`+researchArr+`[4].UnlockedState') b,
		json_array_length(properties,'`+recipesArr+`') n, json_extract(properties,'`+recipesArr+`[2].BaseRecipeId.Name') id,
		json_extract(properties,'$.TechKnowledgePlayerComponent.m_TechKnowledgePoints') pts, typeof(properties) ty from actors where id=2`)
	if row["st"] != "Purchased" || row["nw"].(int64) != 0 || row["other"] != "Purchased" || row["b"] != "NotPurchased" || row["n"].(int64) != 3 || row["id"] != "T4_Rifle_Recipe" || row["pts"].(int64) != 10 || row["ty"] != "blob" {
		t.Fatalf("row: %v", row)
	}
	// the new entry has the shape of the game's own
	e := mustOne(t, o, `select json(json_extract(properties,'`+recipesArr+`[2]')) e from actors where id=2`)["e"].(string)
	if e != `{"BaseRecipeId":{"Name":"T4_Rifle_Recipe"},"m_QualityLevel":0,"m_bIsNew":true,"m_NumberOfRecipeUses":0,"m_bIsLimitedUseRecipe":false,"m_Source":"SchematicPickup"}` {
		t.Fatalf("recipe entry: %s", e)
	}
	// unlocking again changes nothing more
	r, err = o.UnlockResearch(Args{"item_key": "RCP_T4_Rifle_Recipe"})
	if err != nil || r.(map[string]any)["alreadyPurchased"] != true || r.(map[string]any)["unlockAdded"] != false {
		t.Fatalf("%v %v", r, err)
	}
	if mustOne(t, o, `select json_array_length(properties,'`+recipesArr+`') n from actors where id=2`)["n"].(int64) != 3 {
		t.Fatal("no duplicate recipe")
	}
	assertDBSound(t, o)
}

func TestUnlockResearchRepairsAPurchasedEntryWhoseRecipeIsMissing(t *testing.T) {
	o := researchOps(t)
	r, err := o.UnlockResearch(Args{"item_key": "RCP_Sandbike_Mk2_Recipe"})
	if err != nil || r.(map[string]any)["repaired"] != true {
		t.Fatalf("%v %v", r, err)
	}
	if mustOne(t, o, `select json_array_length(properties,'`+recipesArr+`') n from actors where id=2`)["n"].(int64) != 3 {
		t.Fatal("the missing recipe is added")
	}
}

func TestUnlockResearchBuildingAddsTheSetAndItsPiece(t *testing.T) {
	o := researchOps(t)
	if _, err := o.UnlockResearch(Args{"item_key": "BLD_AugmentStation_Patent"}); err != nil {
		t.Fatal(err)
	}
	sets := mustOne(t, o, `select count(*) c from building_progression_learned_building_sets where character_id=1 and learned_building_set='AugmentStation_Patent'`)["c"].(int64)
	pieces := mustOne(t, o, `select count(*) c from building_progression_new_buildable_pieces where character_id=1 and new_buildable_piece='AugmentStation_Placeable'`)["c"].(int64)
	if sets != 1 || pieces != 1 {
		t.Fatalf("set %d piece %d", sets, pieces)
	}
	if mustOne(t, o, `select count(*) c from building_progression_learned_building_sets`)["c"].(int64) != 2 {
		t.Fatal("other sets are untouched")
	}
}

func TestUnlockResearchRefusals(t *testing.T) {
	o := researchOps(t)
	for name, a := range map[string]Args{
		"a group marker":        {"item_key": "DA_GRP_Water"},
		"a key not in the list": {"item_key": "RCP_Invented_Recipe"},
		"a key with a quote":    {"item_key": `RCP_X"]`},
		"an empty key":          {"item_key": ""},
		"a path in the key":     {"item_key": "RCP_X].Y"},
	} {
		if _, err := o.UnlockResearch(a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if o.S.Dirty() {
		t.Fatal("refusals must not change the save")
	}
}

func TestCraftingListsKnownAndResearchableRecipes(t *testing.T) {
	o := researchOps(t)
	r, err := o.Crafting()
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]CraftingRow{}
	for _, x := range r.(map[string]any)["rows"].([]CraftingRow) {
		by[x.RecipeID] = x
	}
	if h := by["HealthPackRecipe"]; !h.Known || !h.Limited || h.Uses != 1 || h.Source != "SchematicPickup" {
		t.Fatalf("known: %+v", h)
	}
	if _, ok := by["Stillsuit_Unique_04_recipe"]; !ok || by["Stillsuit_Unique_04_recipe"].Category != "Water Discipline" {
		t.Fatalf("a known recipe that is not in the research list is still listed: %+v", by["Stillsuit_Unique_04_recipe"])
	}
	if x := by["T4_Rifle_Recipe"]; x.Known || x.Source != "Research" || x.ResearchOf != "RCP_T4_Rifle_Recipe" {
		t.Fatalf("a recipe the research tree can still give: %+v", x)
	}
	if len(by) != 4 {
		t.Fatalf("recipes: %d (%v)", len(by), by)
	}
}

func TestUnlockRecipeRules(t *testing.T) {
	o := researchOps(t)
	if r, err := o.UnlockRecipe(Args{"recipe_id": "T4_Rifle_Recipe"}); err != nil || r.(map[string]any)["alreadyKnown"] != false {
		t.Fatalf("%v %v", r, err)
	}
	if r, _ := o.UnlockRecipe(Args{"recipe_id": "T4_Rifle_Recipe"}); r.(map[string]any)["alreadyKnown"] != true {
		t.Fatal("second time it is already known")
	}
	for _, id := range []string{"Not_In_Anything", "bad id", `x"y`, ""} {
		if _, err := o.UnlockRecipe(Args{"recipe_id": id}); err == nil {
			t.Errorf("%q must be refused", id)
		}
	}
	// a save without a known-recipes list is refused, not given an invented one
	o2 := &Ops{S: testsave.PlayerWithSQL(t, strings.Replace(researchWorld, `,"CraftingRecipesLibraryActorComponent":{"m_KnownItemRecipes":[`, `,"Other":{"x":[`, 1))}
	if _, err := o2.UnlockRecipe(Args{"recipe_id": "T4_Rifle_Recipe"}); err == nil {
		t.Fatal("no recipes list, no write")
	}
}

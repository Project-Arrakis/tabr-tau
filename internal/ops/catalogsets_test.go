package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

func TestBuildingGroupsFollowTheConsole(t *testing.T) {
	for _, c := range []struct{ id, name, want string }{
		{"Developer_Storage_Container_Patent", "Developer Storage Container", "Experimental"},
		{"IceRefinery_Patent", "Ice Refinery", "Experimental"},
		{"SomeThing_Patent", "SomeThing_Patent", "Experimental"}, // id equals name: incomplete metadata
		{"MTX_Sardaukar_BuildingSet_Patent", "Sardaukar Building Set", "Special & Promotional"},
		{"AtreidesSet", "Atreides Building Set", "Faction & House Sets"},
		{"Atre_BasicLighting_Patent", "Atreides Lighting", "Faction & House Sets"},
		{"SmallOreRefinery_Patent", "Small Ore Refinery", "Crafting & Utilities"},
		{"Table_Patent", "Table", "Furniture & Decorations"},
		{"SomeFloor_Patent", "Some Floor", "Structures & Building Sets"},
	} {
		if g, _ := buildingGroup(c.id, c.name); g != c.want {
			t.Errorf("%s is %q, want %q", c.id, g, c.want)
		}
	}
}

func TestBuildingSetsHaveGroupsAnExperimentalGroupAndTheNewFilmicSets(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, catalogWorld)}
	r, err := o.BuildingSets()
	if err != nil {
		t.Fatal(err)
	}
	rows := r.(map[string]any)["rows"].([]CatalogRow)
	groups := map[string]int{}
	by := map[string]CatalogRow{}
	for _, x := range rows {
		groups[x.Group]++
		by[x.ID] = x
	}
	for _, g := range []string{"Experimental", "Faction & House Sets", "Crafting & Utilities", "Structures & Building Sets", "Special & Promotional"} {
		if groups[g] == 0 {
			t.Errorf("group %q is empty: %v", g, groups)
		}
	}
	for _, id := range []string{"MTX_Sardaukar_BuildingSet_Patent", "MTX_SardaukarDecorationSet_Patent", "MTX_SardaukarFightingDummy_Patent"} {
		if x, ok := by[id]; !ok || x.RequiredDLC != "Filmic Archive" {
			t.Errorf("%s must be listed and need Filmic Archive: %+v", id, x)
		}
	}
	if x := by["BuildingBlueprint_CopyDevice"]; x.ID != "" {
		t.Errorf("a blueprint tool is not a building set: %+v", x)
	}
	if !by["BasicLighting_Patent"].Learned {
		t.Error("a learned set is shown learned")
	}
}

func TestCustomizationsAreInTheConsolesSets(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, catalogWorld)}
	r, err := o.Customizations()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	counts := map[string]int{}
	req := map[string]string{}
	for _, row := range m["rows"].([]CatalogRow) {
		counts[row.GroupID]++
	}
	groups, ok := m["groups"].([]CustomizationSet)
	if !ok {
		t.Fatalf("groups: %T", m["groups"])
	}
	for _, gr := range groups {
		req[gr.ID] = gr.Requirement
		if gr.Count != counts[gr.ID] {
			t.Errorf("set %s says %d but has %d", gr.ID, gr.Count, counts[gr.ID])
		}
	}
	// the console's numbers: Atreides 14, Harkonnen 14, Smuggler 25, Dune Man 10, Filmic Archive 3
	for id, want := range map[string]int{"atreides": 14, "harkonnen": 14, "smuggler": 25, "dune-man": 10, "filmic-archive": 3} {
		if counts[id] != want {
			t.Errorf("set %s has %d cosmetics, the console has %d", id, counts[id], want)
		}
	}
	if req["dune-man"] != "Requires Lost Harvest" || req["filmic-archive"] != "Requires Filmic Archive" || req["smuggler"] != "Entitlement controlled" || req["atreides"] != "" {
		t.Errorf("requirements: %v", req)
	}
	if counts[otherCustomizations] == 0 {
		t.Error("the cosmetics outside the console's sets stay available under Other")
	}
}

func bigBackpack(t *testing.T, extra string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=500 where id=1;`+extra)}
}

func TestGrantCustomizationsGivesOneOfEachAndSkipsWhatIsThere(t *testing.T) {
	o := bigBackpack(t, ``)
	r, err := o.GrantCustomizations(Args{"group": "filmic-archive"})
	if err != nil || r.(map[string]any)["granted"].(int) != 3 {
		t.Fatalf("%v %v", r, err)
	}
	n := mustOne(t, o, `select count(*) c from items where template_id in ('MTX_Fremen_FedaykinArmor_SetVariant','MTX_Atre_CaladanTrenchcoat_SetVariant','MTX_Sard_Scout_SetVariant')`)["c"].(int64)
	if n != 3 {
		t.Fatalf("three items: %d", n)
	}
	r, err = o.GrantCustomizations(Args{"group": "filmic-archive"})
	if err != nil || r.(map[string]any)["granted"].(int) != 0 {
		t.Fatalf("a second grant adds nothing: %v %v", r, err)
	}
	r, err = o.GrantCustomizations(Args{"group": "all"})
	if err != nil {
		t.Fatal(err)
	}
	if g := r.(map[string]any)["granted"].(int); g != 66-3 {
		t.Fatalf("all five sets, minus the 3 already there: %d", g)
	}
	if _, err := o.GrantCustomizations(Args{"group": "nope"}); err == nil {
		t.Fatal("an unknown set is refused")
	}
}

func TestGrantCustomizationsIsAllOrNothingWhenTheBackpackIsFull(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=4 where id=1;`)} // the fixture holds 2 items already
	_, err := o.GrantCustomizations(Args{"group": "filmic-archive"})
	if err == nil || !strings.Contains(err.Error(), "room for 2 of the 3") {
		t.Fatalf("the message says how many fit: %v", err)
	}
	if o.S.Dirty() {
		t.Fatal("nothing may be added when they do not all fit")
	}
}

func TestUnlockAllBuildingSetsSkipsExperimentalAndWhatIsLearned(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, catalogWorld)}
	before := mustOne(t, o, `select count(*) c from building_progression_learned_building_sets`)["c"].(int64)
	r, err := o.UnlockAllBuildingSets()
	if err != nil {
		t.Fatal(err)
	}
	added := r.(map[string]any)["added"].(int)
	after := mustOne(t, o, `select count(*) c from building_progression_learned_building_sets`)["c"].(int64)
	if int64(added) != after-before || added < 150 {
		t.Fatalf("added %d, rows %d to %d", added, before, after)
	}
	for _, id := range []string{"Developer_Storage_Container_Patent", "IceRefinery_Patent"} {
		if mustOne(t, o, `select count(*) c from building_progression_learned_building_sets where learned_building_set=?`, id)["c"].(int64) != 0 {
			t.Errorf("experimental set %s must not be learned", id)
		}
	}
	r, _ = o.UnlockAllBuildingSets()
	if r.(map[string]any)["added"].(int) != 0 {
		t.Fatal("a second run adds nothing")
	}
	var catalogSets int
	for _, e := range catalog.All() {
		if e.Category == "buildings" && e.Source == "BuildingSets" {
			catalogSets++
		}
	}
	if catalogSets < 200 {
		t.Fatalf("catalog building sets: %d", catalogSets)
	}
}

func TestUnlockAllResearchAndRecipes(t *testing.T) {
	o := researchOps(t)
	r, err := o.UnlockAllResearch()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	// bought: T4_Rifle, AugmentStation and ChoamShelterSet; repaired (purchased but something missing): Sandbike's recipe and BasicLighting's piece
	if m["bought"].(int) != 3 || m["repaired"].(int) != 2 {
		t.Fatalf("%v", m)
	}
	rr, _ := o.Research()
	for _, x := range rr.(map[string]any)["rows"].([]ResearchRow) {
		if x.Actionable && !x.Unlocked {
			t.Errorf("%s is still not unlocked", x.ItemKey)
		}
		if x.Type == "Group" && x.Purchased {
			t.Errorf("a group marker is left alone: %s", x.ItemKey)
		}
	}
	if r2, _ := o.UnlockAllResearch(); r2.(map[string]any)["bought"].(int) != 0 {
		t.Fatal("a second run buys nothing")
	}
	o2 := researchOps(t)
	rc, err := o2.UnlockAllRecipes()
	if err != nil || rc.(map[string]any)["added"].(int) != 2 { // T4_Rifle and Sandbike_Mk2 are not known
		t.Fatalf("%v %v", rc, err)
	}
	if rc2, _ := o2.UnlockAllRecipes(); rc2.(map[string]any)["added"].(int) != 0 {
		t.Fatal("a second run adds nothing")
	}
}

func TestMaxSkillsForASchoolAndForAll(t *testing.T) {
	o := skillOps(t, 45)
	r, err := o.MaxSkills(Args{"school": "Trooper"})
	if err != nil || r.(map[string]any)["changed"].(int) < 20 {
		t.Fatalf("%v %v", r, err)
	}
	sk, _ := o.Skills()
	for _, s := range sk.(map[string]any)["modules"].([]SkillRow) {
		if s.School == "Trooper" && s.Rank != s.Max {
			t.Errorf("%s is rank %d of %d", s.ID, s.Rank, s.Max)
		}
		if s.ID == "Skills.Ability.Hypersprint" && s.Rank != 2 {
			t.Errorf("another school's skill must not change: rank %d", s.Rank)
		}
	}
	if unspentNow(t, o) != 45 {
		t.Fatal("the unspent points are not touched")
	}
	if _, err := o.MaxSkills(Args{"school": "all"}); err != nil {
		t.Fatal(err)
	}
	sk, _ = o.Skills()
	for _, s := range sk.(map[string]any)["modules"].([]SkillRow) {
		if s.Known && s.School != "Hidden" && s.Rank != s.Max {
			t.Errorf("%s is rank %d of %d after maxing all schools", s.ID, s.Rank, s.Max)
		}
	}
	if _, err := o.MaxSkills(Args{"school": "Nope"}); err == nil {
		t.Fatal("unknown school")
	}
}

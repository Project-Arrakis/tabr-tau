package ops

import (
	"fmt"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/skills"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A character with skill modules stored as a real save stores them (keys are `(TagName="Skills.X")`): Hypersprint at rank 2
// (ladder 1, 3, 6 cumulative, so 3 points), a perk at 2 points, an untouched one, and one the catalog does not know.
func skillOps(t *testing.T, unspent int) *Ops {
	t.Helper()
	comp := `{"FLevelComponent":[[],{"ModuleData":{` +
		`"(TagName=\"Skills.Ability.Hypersprint\")":{"SkillPointsSpent":3},` +
		`"(TagName=\"Skills.Perk.BodyShots\")":{"SkillPointsSpent":2},` +
		`"(TagName=\"Skills.Ability.Blindspot\")":{"SkillPointsSpent":0},` +
		`"(TagName=\"Skills.Attribute.Explorer6\")":{"SkillPointsSpent":9}},` +
		`"TotalXPEarned":63009,"TotalSkillPoints":106,"UnspentSkillPoints":` + itoa64(unspent) + `,"LastViewedSkillTree":"Mentat"}],"FHealthComponent":[[],{"m_CurrentHealth":100.0}]}`
	return &Ops{S: testsave.PlayerWithSQL(t, `insert into fgl_entities(entity_id,components) values (-10, jsonb('`+comp+`'));
		insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,-10,'DuneCharacter');`)}
}

func itoa64(n int) string { return fmt.Sprint(n) }

func moduleSpent(t *testing.T, o *Ops, id string) any {
	t.Helper()
	return mustOne(t, o, `select json_extract(components,'`+modulePath(id)+`.SkillPointsSpent') s from fgl_entities where entity_id=-10`)["s"]
}

func unspentNow(t *testing.T, o *Ops) int64 {
	t.Helper()
	return mustOne(t, o, `select json_extract(components,'$.FLevelComponent[1].UnspentSkillPoints') u from fgl_entities where entity_id=-10`)["u"].(int64)
}

func TestSkillsListsRanksFromTheLadderAndTheTotals(t *testing.T) {
	o := skillOps(t, 45)
	r, err := o.Skills()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["total"].(int64) != 106 || m["unspent"].(int64) != 45 || m["spent"].(int64) != 14 {
		t.Fatalf("totals: %v %v %v", m["total"], m["unspent"], m["spent"])
	}
	by := map[string]SkillRow{}
	for _, s := range m["modules"].([]SkillRow) {
		by[s.ID] = s
	}
	if h := by["Skills.Ability.Hypersprint"]; h.Rank != 2 || h.Max != 3 || h.Spent != 3 || !h.Known || h.School != "BeneGesserit" {
		t.Fatalf("hypersprint: %+v", h)
	}
	if by["Skills.Ability.Blindspot"].Rank != 0 {
		t.Fatalf("an untouched module is rank 0")
	}
	if e := by["Skills.Attribute.Explorer6"]; e.Known || e.Rank != 1 || e.School != "Other" {
		t.Fatalf("a module the catalog omits is listed, not hidden, and never claims more than one rank: %+v", e)
	}
	if len(by) < 140 {
		t.Fatalf("the whole catalog is listed even for modules the save lacks: %d", len(by))
	}
}

func TestSetSkillModuleWritesTheLadderPointsAndNothingElse(t *testing.T) {
	o := skillOps(t, 45)
	if _, err := o.SetSkillModule(Args{"module": "Skills.Ability.Hypersprint", "level": 3.0}); err != nil {
		t.Fatal(err)
	}
	if moduleSpent(t, o, "Skills.Ability.Hypersprint").(int64) != 6 {
		t.Fatalf("rank 3 stores the cumulative 6")
	}
	if unspentNow(t, o) != 45 {
		t.Fatal("without charge the unspent points are the console's: untouched")
	}
	if moduleSpent(t, o, "Skills.Perk.BodyShots").(int64) != 2 {
		t.Fatal("other modules are untouched")
	}
	row := mustOne(t, o, `select json_extract(components,'$.FLevelComponent[1].LastViewedSkillTree') v, json_extract(components,'$.FHealthComponent[1].m_CurrentHealth') h, typeof(components) ty from fgl_entities where entity_id=-10`)
	if row["v"] != "Mentat" || row["h"].(float64) != 100 || row["ty"] != "blob" {
		t.Fatalf("the rest of the record must survive: %v", row)
	}
	// reset to rank 0
	if _, err := o.SetSkillModule(Args{"module": "Skills.Ability.Hypersprint", "level": 0.0}); err != nil || moduleSpent(t, o, "Skills.Ability.Hypersprint").(int64) != 0 {
		t.Fatalf("rank 0: %v", err)
	}
	assertDBSound(t, o)
}

func TestSetSkillModuleCreatesAMissingModule(t *testing.T) {
	o := skillOps(t, 45)
	if _, err := o.SetSkillModule(Args{"module": "Skills.Ability.CablePull", "level": 1.0}); err != nil {
		t.Fatal(err)
	}
	if moduleSpent(t, o, "Skills.Ability.CablePull") == nil {
		t.Fatal("the module was not created")
	}
}

func TestSetSkillModuleChargeRefusesWhenThePointsAreShort(t *testing.T) {
	o := skillOps(t, 2)
	if _, err := o.SetSkillModule(Args{"module": "Skills.Ability.Hypersprint", "level": 3.0, "charge": true}); err == nil {
		t.Fatal("rank 3 costs 3 more points than the 2 unspent")
	}
	if o.S.Dirty() || unspentNow(t, o) != 2 {
		t.Fatal("a refused change must leave the save as it was")
	}
}

func TestSetSkillModuleChargeAndRefund(t *testing.T) {
	o := skillOps(t, 5)
	if _, err := o.SetSkillModule(Args{"module": "Skills.Ability.Hypersprint", "level": 3.0, "charge": true}); err != nil {
		t.Fatal(err)
	}
	if unspentNow(t, o) != 2 {
		t.Fatalf("unspent after paying 3: %d", unspentNow(t, o))
	}
	if _, err := o.SetSkillModule(Args{"module": "Skills.Ability.Hypersprint", "level": 1.0, "charge": true}); err != nil {
		t.Fatal(err)
	}
	if unspentNow(t, o) != 7 || moduleSpent(t, o, "Skills.Ability.Hypersprint").(int64) != 1 {
		t.Fatalf("lowering gives the difference back: unspent %d", unspentNow(t, o))
	}
}

func TestSetSkillModuleRefusals(t *testing.T) {
	o := skillOps(t, 45)
	for name, a := range map[string]Args{
		"a rank the module lacks": {"module": "Skills.Ability.Hypersprint", "level": 4.0},
		"a negative rank":         {"module": "Skills.Ability.Hypersprint", "level": -1.0},
		"a module not in catalog": {"module": "Skills.Attribute.Explorer6", "level": 1.0},
		"a quote in the id":       {"module": `Skills.Ability.X")".Y`, "level": 1.0},
		"a path in the id":        {"module": "Skills.Ability.X].Y", "level": 1.0},
		"not a skill":             {"module": "Other.Thing", "level": 1.0},
		"no module":               {"level": 1.0},
	} {
		if _, err := o.SetSkillModule(a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if o.S.Dirty() {
		t.Fatal("refusals must not change the save")
	}
}

func TestSetSkillPoints(t *testing.T) {
	o := skillOps(t, 45)
	r, err := o.SetSkillPoints(Args{"points": 120.0})
	if err != nil || r.(map[string]any)["before"].(int64) != 45 || unspentNow(t, o) != 120 {
		t.Fatalf("%v %v", r, err)
	}
	for _, bad := range []float64{-1, 100001} {
		if _, err := o.SetSkillPoints(Args{"points": bad}); err == nil {
			t.Errorf("%v must be refused", bad)
		}
	}
	if moduleSpent(t, o, "Skills.Perk.BodyShots").(int64) != 2 {
		t.Fatal("modules are untouched")
	}
}

func TestRestoreStarterSkillsOnlyRaises(t *testing.T) {
	o := skillOps(t, 45)
	r, err := o.RestoreStarterSkills(Args{"school": "Trooper"})
	if err != nil || r.(map[string]any)["changed"].(int) != 2 {
		t.Fatalf("%v %v", r, err)
	}
	for _, s := range skills.StarterSkills("Trooper") {
		mod, _ := skills.Lookup(s.ID)
		want, _ := skills.Points(mod, s.Rank)
		if moduleSpent(t, o, s.ID).(int64) != int64(want) {
			t.Errorf("%s not at rank %d", s.ID, s.Rank)
		}
	}
	r, _ = o.RestoreStarterSkills(Args{"school": "Trooper"})
	if r.(map[string]any)["changed"].(int) != 0 {
		t.Fatal("a second restore changes nothing")
	}
	if _, err := o.RestoreStarterSkills(Args{"school": "Nope"}); err == nil {
		t.Fatal("unknown school")
	}
	if unspentNow(t, o) != 45 {
		t.Fatal("the unspent points are not touched")
	}
}

func TestSkillsRefusesASaveWithoutTheRecord(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	if _, err := o.Skills(); err == nil {
		t.Fatal("no skill record, no guess")
	}
	if _, err := o.SetSkillPoints(Args{"points": 5.0}); err == nil {
		t.Fatal("no skill record, no write")
	}
}

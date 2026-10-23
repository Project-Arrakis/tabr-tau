package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A character with skills in the five schools (Hypersprint 3 points and BodyShots 2 are learned, Blindspot is at 0), one Hidden
// module the catalog lists (ControlSpace, 2 points), and one module the catalog omits (Explorer6, 9 points).
func resetSkillOps(t *testing.T, unspent int) *Ops {
	t.Helper()
	comp := `{"FLevelComponent":[[],{"ModuleData":{` +
		`"(TagName=\"Skills.Ability.Hypersprint\")":{"SkillPointsSpent":3},` +
		`"(TagName=\"Skills.Perk.BodyShots\")":{"SkillPointsSpent":2},` +
		`"(TagName=\"Skills.Ability.Blindspot\")":{"SkillPointsSpent":0},` +
		`"(TagName=\"Skills.Ability.ControlSpace\")":{"SkillPointsSpent":2},` +
		`"(TagName=\"Skills.Attribute.Explorer6\")":{"SkillPointsSpent":9}},` +
		`"TotalXPEarned":63009,"TotalSkillPoints":106,"UnspentSkillPoints":` + itoa64(unspent) + `,"KeystoneBonusSkillPoints":0,"LastViewedSkillTree":"Mentat"}],"FHealthComponent":[[],{"m_CurrentHealth":100.0}]}`
	return &Ops{S: testsave.PlayerWithSQL(t, `insert into fgl_entities(entity_id,components) values (-10, jsonb('`+comp+`'));
		insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,-10,'DuneCharacter');`)}
}

func resetCountShown(t *testing.T, o *Ops) int64 {
	t.Helper()
	r, err := o.Skills()
	if err != nil {
		t.Fatal(err)
	}
	return r.(map[string]any)["resetCount"].(int64)
}

func TestSkillsShowsHowManyAResetWouldChange(t *testing.T) {
	o := resetSkillOps(t, 45)
	// Hypersprint and BodyShots: the Hidden module and the one the catalog omits are not part of the reset
	if n := resetCountShown(t, o); n != 2 {
		t.Fatalf("resetCount = %d, want 2", n)
	}
}

func TestResetSkillsZeroesTheFiveSchoolsAndNothingElse(t *testing.T) {
	o := resetSkillOps(t, 45)
	before := digest(t, o, nil, -10)
	r, err := o.ResetSkills(Args{"expected": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["changed"].(int64) != 2 || m["noop"] != false {
		t.Fatalf("answer: %v", m)
	}
	for _, id := range []string{"Skills.Ability.Hypersprint", "Skills.Perk.BodyShots", "Skills.Ability.Blindspot"} {
		if got := moduleSpent(t, o, id); got.(int64) != 0 {
			t.Errorf("%s = %v, want 0 (rank 0)", id, got)
		}
	}
	// the Hidden module and the one the catalog does not know keep what they had
	if got := moduleSpent(t, o, "Skills.Ability.ControlSpace"); got.(int64) != 2 {
		t.Errorf("a Hidden module must be left alone: %v", got)
	}
	if got := moduleSpent(t, o, "Skills.Attribute.Explorer6"); got.(int64) != 9 {
		t.Errorf("a module the catalog omits must be left alone: %v", got)
	}
	// the rest of the level record, the other components and every other table are as they were
	row := mustOne(t, o, `select json_extract(components,'$.FLevelComponent[1].LastViewedSkillTree') v, json_extract(components,'$.FHealthComponent[1].m_CurrentHealth') h,
		json_extract(components,'$.FLevelComponent[1].TotalXPEarned') xp, json_extract(components,'$.FLevelComponent[1].TotalSkillPoints') tot, typeof(components) ty from fgl_entities where entity_id=-10`)
	if row["v"] != "Mentat" || row["h"].(float64) != 100 || row["xp"].(int64) != 63009 || row["tot"].(int64) != 106 || row["ty"] != "blob" {
		t.Fatalf("the rest of the record must survive: %v", row)
	}
	if unspentNow(t, o) != 45 {
		t.Fatal("without the box ticked the unspent points are not touched, as with Set rank")
	}
	if after := digest(t, o, nil, -10); after != before {
		t.Fatal("something outside the skill record changed")
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "reset skills (2 changed)" {
		t.Fatalf("one write-log entry worded like the others: %q", p)
	}
	assertDBSound(t, o)
}

func TestResetSkillsGivesThePointsBackWhenAsked(t *testing.T) {
	o := resetSkillOps(t, 45)
	if _, err := o.ResetSkills(Args{"expected": 2.0, "charge": true}); err != nil {
		t.Fatal(err)
	}
	// Hypersprint 3 + BodyShots 2 points go back to the unspent points: the same as lowering each with Set rank and the box ticked
	if got := unspentNow(t, o); got != 50 {
		t.Fatalf("unspent = %d, want 50", got)
	}
	if p := o.S.Pending(); len(p) != 1 || !strings.HasPrefix(p[0], "reset skills (2 changed") {
		t.Fatalf("log: %q", p)
	}
	assertDBSound(t, o)
}

// The reset must be the same edit as the per-skill one: ranking each skill to 0 by hand ends in the same save.
func TestResetSkillsEqualsSettingEachRankToZero(t *testing.T) {
	a, b := resetSkillOps(t, 45), resetSkillOps(t, 45)
	if _, err := a.ResetSkills(Args{"expected": 2.0, "charge": true}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"Skills.Ability.Hypersprint", "Skills.Perk.BodyShots"} {
		if _, err := b.SetSkillModule(Args{"module": id, "level": 0.0, "charge": true}); err != nil {
			t.Fatal(err)
		}
	}
	q := `select json(components) c from fgl_entities where entity_id=-10`
	if mustOne(t, a, q)["c"] != mustOne(t, b, q)["c"] {
		t.Fatalf("the reset and the single-skill edits ended differently:\n%v\n%v", mustOne(t, a, q)["c"], mustOne(t, b, q)["c"])
	}
}

func TestResetSkillsTwiceIsANoOp(t *testing.T) {
	o := resetSkillOps(t, 45)
	if _, err := o.ResetSkills(Args{"expected": 2.0}); err != nil {
		t.Fatal(err)
	}
	if n := resetCountShown(t, o); n != 0 {
		t.Fatalf("after a reset the page shows %d", n)
	}
	was := digest(t, o, nil, 0)
	pending := len(o.S.Pending())
	r, err := o.ResetSkills(Args{"expected": 0.0})
	if err != nil {
		t.Fatal(err)
	}
	if m := r.(map[string]any); m["changed"].(int64) != 0 || m["noop"] != true {
		t.Fatalf("answer: %v", m)
	}
	if digest(t, o, nil, 0) != was || len(o.S.Pending()) != pending {
		t.Fatal("a reset with nothing to do must not change the save or add a log entry")
	}
}

func TestResetSkillsRefusesWhenTheCountIsNotTheOneConfirmed(t *testing.T) {
	o := resetSkillOps(t, 45)
	was := digest(t, o, nil, 0)
	for _, n := range []float64{0, 1, 3, 100} {
		_, err := o.ResetSkills(Args{"expected": n})
		wantRefusal(t, err, apierr.ErrConflict, "expected "+itoa64(int(n)))
	}
	if digest(t, o, nil, 0) != was || o.S.Dirty() {
		t.Fatal("a refused reset must change nothing")
	}
	// a count of 0 on a save with nothing to reset passes, but a count above 0 on such a save is the same refusal
	z := resetSkillOps(t, 45)
	if _, err := z.ResetSkills(Args{"expected": 2.0}); err != nil {
		t.Fatal(err)
	}
	_, err := z.ResetSkills(Args{"expected": 2.0})
	wantRefusal(t, err, apierr.ErrConflict, "a stale page after the reset")
}

func TestResetSkillsRefusesAMissingOrBadCountAndAnUnreadableRecord(t *testing.T) {
	o := resetSkillOps(t, 45)
	for _, a := range []Args{{}, {"expected": -1.0}, {"expected": "two"}} {
		_, err := o.ResetSkills(a)
		wantRefusal(t, err, apierr.ErrBadArgument, "bad count")
	}
	if o.S.Dirty() {
		t.Fatal("a refused reset must change nothing")
	}
	if _, err := (&Ops{S: testsave.Player(t)}).ResetSkills(Args{"expected": 0.0}); err == nil || errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("a save with no skill record is refused as such: %v", err)
	}
}

// charge says whether the points come back: a value that is not true or false is refused, never read as false.
func TestResetSkillsRefusesAChargeThatIsNotABoolean(t *testing.T) {
	o := resetSkillOps(t, 45)
	for _, v := range []any{"true", "yes", 1.0, 0.0, nil, []any{true}} {
		_, err := o.ResetSkills(Args{"expected": 2.0, "charge": v})
		wantRefusal(t, err, apierr.ErrBadArgument, "charge that is not a boolean")
		if o.S.Dirty() {
			t.Fatalf("%v: a refused reset changes nothing", v)
		}
	}
	if _, err := o.ResetSkills(Args{"expected": 2.0, "charge": false}); err != nil {
		t.Fatalf("false is a value: %v", err)
	}
}

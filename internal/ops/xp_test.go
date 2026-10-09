package ops

import (
	"fmt"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A character whose FLevelComponent holds the given XP and skill points, with other data beside them that must survive.
func xpOps(t *testing.T, xp, total, unspent int) *Ops {
	t.Helper()
	comp := fmt.Sprintf(`{"FLevelComponent":[[],{"ModuleData":{"Skills.Key.Mentat1":{"SkillPointsSpent":1}},"TotalXPEarned":%d,"KeystoneBonusSkillPoints":0,"TotalSkillPoints":%d,"UnspentSkillPoints":%d,"LastViewedSkillTree":"Mentat"}],"FHealthComponent":[[],{"m_CurrentHealth":100.0}]}`, xp, total, unspent)
	return &Ops{S: testsave.PlayerWithSQL(t, `insert into fgl_entities(entity_id,components) values (-10, jsonb('`+comp+`'));
		insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,-10,'DuneCharacter');`)}
}

func levelState(t *testing.T, o *Ops) (xp, total, unspent int64) {
	t.Helper()
	r := mustOne(t, o, `select json_extract(components,'$.FLevelComponent[1].TotalXPEarned') xp, json_extract(components,'$.FLevelComponent[1].TotalSkillPoints') total,
		json_extract(components,'$.FLevelComponent[1].UnspentSkillPoints') unspent from fgl_entities where entity_id=-10`)
	return r["xp"].(int64), r["total"].(int64), r["unspent"].(int64)
}

func TestAddXPOneLevelGivesOneSkillPoint(t *testing.T) {
	o := xpOps(t, 0, 0, 0)
	r, err := o.AddXP(Args{"amount": 215.0})
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["levelBefore"].(int) != 0 || res["levelAfter"].(int) != 2 || res["skillPointsGained"].(int) != 2 || res["capped"] != false {
		t.Fatalf("result: %v", res)
	}
	// 215 XP is level 2 (levels 1 and 2 are reached at 40 and 215); a level-n character has n-1 skill points in total.
	if xp, total, unspent := levelState(t, o); xp != 215 || total != 2 || unspent != 2 {
		t.Fatalf("state after: xp=%d total=%d unspent=%d", xp, total, unspent)
	}
	row := mustOne(t, o, `select json_extract(components,'$.FLevelComponent[1].LastViewedSkillTree') v, json_extract(components,'$.FLevelComponent[1].ModuleData."Skills.Key.Mentat1".SkillPointsSpent') x,
		json_extract(components,'$.FHealthComponent[1].m_CurrentHealth') h, typeof(components) ty from fgl_entities where entity_id=-10`)
	if row["v"] != "Mentat" || row["x"].(int64) != 1 || row["h"].(float64) != 100 || row["ty"] != "blob" {
		t.Fatalf("other data or the storage type changed: %v", row)
	}
	assertDBSound(t, o)
}

func TestAddXPKeepsSpentPointsAndOnlyAddsTheNewOnes(t *testing.T) {
	o := xpOps(t, 215, 1, 0)                                  // level 2, the one point already spent
	if _, err := o.AddXP(Args{"amount": 300.0}); err != nil { // 515 XP is level 3
		t.Fatal(err)
	}
	if xp, total, unspent := levelState(t, o); xp != 515 || total != 2 || unspent != 1 {
		t.Fatalf("state after: xp=%d total=%d unspent=%d", xp, total, unspent)
	}
}

func TestAddXPWithinALevelChangesOnlyTheXP(t *testing.T) {
	o := xpOps(t, 50, 1, 1)
	r, err := o.AddXP(Args{"amount": 10.0})
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["skillPointsGained"].(int) != 0 {
		t.Fatalf("no new level, no new points: %v", res)
	}
	if xp, total, unspent := levelState(t, o); xp != 60 || total != 1 || unspent != 1 {
		t.Fatalf("state after: xp=%d total=%d unspent=%d", xp, total, unspent)
	}
}

func TestAddXPStopsAtTheLastLevel(t *testing.T) {
	last := xpByLevel[len(xpByLevel)-1]
	o := xpOps(t, 1000, 3, 3) // 1,000 XP is level 4, so 3 skill points in total
	r, err := o.AddXP(Args{"amount": 100000000.0})
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["capped"] != true || res["after"].(int64) != last || res["levelAfter"].(int) != len(xpByLevel)-1 {
		t.Fatalf("result: %v", res)
	}
	// the last level is 200: 199 skill points in total, all of the 196 new ones unspent on top of the 3 that were
	if xp, total, unspent := levelState(t, o); xp != last || total != 199 || unspent != 199 {
		t.Fatalf("state after: xp=%d total=%d unspent=%d", xp, total, unspent)
	}
	r, err = o.AddXP(Args{"amount": 5.0})
	if err != nil {
		t.Fatal(err)
	}
	if r.(map[string]any)["applied"].(int64) != 0 {
		t.Fatalf("at the last level nothing more is added: %v", r)
	}
}

func TestAddXPRefusesBadInputAndSavesWithoutTheRecord(t *testing.T) {
	o := xpOps(t, 100, 1, 1)
	for _, amount := range []float64{0, -5, 1e10} {
		if _, err := o.AddXP(Args{"amount": amount}); err == nil {
			t.Errorf("amount %v must be refused", amount)
		}
	}
	if _, err := o.AddXP(Args{}); err == nil {
		t.Error("a missing amount must be refused")
	}
	if xp, _, _ := levelState(t, o); xp != 100 {
		t.Fatal("a refused request must not change the save")
	}
	if _, err := (&Ops{S: testsave.Player(t)}).AddXP(Args{"amount": 5.0}); err == nil {
		t.Error("a save with no level record must be refused")
	}
}

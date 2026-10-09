package ops

import (
	"fmt"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

func intelOps(t *testing.T, points int) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, `update actors set properties=jsonb('{"Other":{"keep":"me"},"TechKnowledgePlayerComponent":{"m_TechKnowledge":{},"m_TechKnowledgePoints":`+fmt.Sprint(points)+`,"m_NextTechTreeUpgradeIndex":4}}') where id=2;`)}
}

func intelNow(t *testing.T, o *Ops) int64 {
	t.Helper()
	return mustOne(t, o, `select json_extract(properties,'$.TechKnowledgePlayerComponent.m_TechKnowledgePoints') n from actors where id=2`)["n"].(int64)
}

func TestAddIntelAddsAndKeepsTheRestOfTheRecord(t *testing.T) {
	o := intelOps(t, 333)
	r, err := o.AddIntel(Args{"amount": 100.0})
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["before"].(int64) != 333 || res["after"].(int64) != 433 || res["applied"].(int64) != 100 || res["capped"] != false {
		t.Fatalf("result: %v", res)
	}
	if intelNow(t, o) != 433 {
		t.Fatal("the save was not changed")
	}
	row := mustOne(t, o, `select json_extract(properties,'$.Other.keep') k, json_extract(properties,'$.TechKnowledgePlayerComponent.m_NextTechTreeUpgradeIndex') i, typeof(properties) ty from actors where id=2`)
	if row["k"] != "me" || row["i"].(int64) != 4 || row["ty"] != "blob" {
		t.Fatalf("other data or the storage type changed: %v", row)
	}
	assertDBSound(t, o)
}

func TestAddIntelNeverPassesTheCap(t *testing.T) {
	o := intelOps(t, 2750)
	r, err := o.AddIntel(Args{"amount": 100.0})
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["after"].(int64) != maxIntel || res["applied"].(int64) != 29 || res["capped"] != true {
		t.Fatalf("result: %v", res)
	}
	r, err = o.AddIntel(Args{"amount": 5.0})
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["applied"].(int64) != 0 || intelNow(t, o) != maxIntel {
		t.Fatalf("at the cap nothing more is added: %v", res)
	}
}

func TestAddIntelRefusesBadInput(t *testing.T) {
	o := intelOps(t, 10)
	for _, amount := range []float64{0, -5, 1e10} {
		if _, err := o.AddIntel(Args{"amount": amount}); err == nil {
			t.Errorf("amount %v must be refused", amount)
		}
	}
	if _, err := o.AddIntel(Args{}); err == nil {
		t.Error("a missing amount must be refused")
	}
	if intelNow(t, o) != 10 {
		t.Fatal("a refused request must not change the save")
	}
	// a save without the component is refused, not given a made-up one
	bare := &Ops{S: testsave.Player(t)}
	if _, err := bare.AddIntel(Args{"amount": 5.0}); err == nil {
		t.Error("a save with no Intel component must be refused")
	}
}

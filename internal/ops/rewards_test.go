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

func balance(t *testing.T, o *Ops, id int) any {
	t.Helper()
	rows, err := o.S.Query(`select balance from player_virtual_currency_balances where player_controller_id=1 and currency_id=?`, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		return nil
	}
	return rows[0]["balance"]
}

func TestAddCurrencyAddsToTheBalanceAndCreatesAMissingWallet(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into player_virtual_currency_balances(player_controller_id,currency_id,balance) values (1,0,591227);`)}
	r, err := o.AddCurrency(Args{"currency": 0.0, "amount": 1000.0})
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["before"].(int64) != 591227 || res["after"].(int64) != 592227 {
		t.Fatalf("result: %v", res)
	}
	if balance(t, o, 0).(int64) != 592227 {
		t.Fatal("the balance was not changed")
	}
	if balance(t, o, 1) != nil {
		t.Fatal("the other wallet must not exist yet")
	}
	if _, err := o.AddCurrency(Args{"currency": 1.0, "amount": 50.0}); err != nil {
		t.Fatal(err)
	}
	if balance(t, o, 1).(int64) != 50 || balance(t, o, 0).(int64) != 592227 {
		t.Fatalf("wallets: %v %v", balance(t, o, 0), balance(t, o, 1))
	}
	assertDBSound(t, o)
}

func TestAddCurrencyRefusesBadInput(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	for name, a := range map[string]Args{
		"zero amount": {"currency": 0.0, "amount": 0.0}, "negative": {"currency": 0.0, "amount": -5.0}, "too large": {"currency": 0.0, "amount": 1e13},
		"unknown wallet": {"currency": 2.0, "amount": 5.0}, "negative wallet": {"currency": -1.0, "amount": 5.0}, "missing amount": {"currency": 0.0},
	} {
		if _, err := o.AddCurrency(a); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if balance(t, o, 0) != nil || balance(t, o, 1) != nil {
		t.Fatal("a refused request must not create a balance")
	}
}

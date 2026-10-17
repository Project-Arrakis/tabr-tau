package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// the shape the real save has: base and current values stored as text, beside the other attribute sets
func hydrationOps(t *testing.T) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":"70.114852","CurrentValue":"70.114852"},"ClothingCapturedWater":{"BaseValue":"0","CurrentValue":"0"}},"DuneSpiceAddictionAttributeSet":{"SpiceAddictionLevel":{"BaseValue":"1.5","CurrentValue":"1.5"}}}') where id=2;`)}
}

func TestFillHydrationSetsBothValuesAndLeavesTheRestOfTheRecord(t *testing.T) {
	o := hydrationOps(t)
	r, err := o.FillHydration()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["before"].(float64) != 70.114852 || res["after"].(float64) != 100 {
		t.Fatalf("result: %v", res)
	}
	row := mustOne(t, o, `select json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.BaseValue') b,
		json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') c,
		json_extract(gas_attributes,'$.DuneHydrationAttributeSet.ClothingCapturedWater.CurrentValue') w,
		json_extract(gas_attributes,'$.DuneSpiceAddictionAttributeSet.SpiceAddictionLevel.CurrentValue') s,
		typeof(gas_attributes) ty from actors where id=2`)
	if row["b"] != "100.000000" || row["c"] != "100.000000" {
		t.Fatalf("hydration not filled, and kept as text like the game's: %v", row)
	}
	if row["w"] != "0" || row["s"] != "1.5" || row["ty"] != "blob" {
		t.Fatalf("other data or the storage type changed: %v", row)
	}
	assertDBSound(t, o)
}

func TestFillHydrationIsHarmlessWhenAlreadyFull(t *testing.T) {
	o := hydrationOps(t)
	if _, err := o.FillHydration(); err != nil {
		t.Fatal(err)
	}
	r, err := o.FillHydration()
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["before"].(float64) != 100 || res["after"].(float64) != 100 {
		t.Fatalf("result: %v", res)
	}
}

func TestFillHydrationRefusesASaveWithNoHydrationRecord(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set gas_attributes=jsonb('{"DuneSpiceAddictionAttributeSet":{"SpiceAddictionLevel":{"CurrentValue":"0"}}}') where id=2;`)}
	if _, err := o.FillHydration(); err == nil {
		t.Fatal("a save without a hydration record was given one")
	}
	row := mustOne(t, o, `select json_extract(gas_attributes,'$.DuneHydrationAttributeSet') h from actors where id=2`)
	if row["h"] != nil {
		t.Fatalf("a made-up record was written: %v", row)
	}
}

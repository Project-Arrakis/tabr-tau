package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// the shape an older save (or one written by an older editor) has: base and current values stored as text, beside the other attribute sets
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
	if res["before"].(float64) != 70.114852 || res["after"].(float64) != 100 || res["type"] != "text" || res["baseType"] != "text" {
		t.Fatalf("result: %v", res)
	}
	row := mustOne(t, o, `select json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.BaseValue') b,
		json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') c,
		json_extract(gas_attributes,'$.DuneHydrationAttributeSet.ClothingCapturedWater.CurrentValue') w,
		json_extract(gas_attributes,'$.DuneSpiceAddictionAttributeSet.SpiceAddictionLevel.CurrentValue') s,
		typeof(gas_attributes) ty from actors where id=2`)
	if row["b"] != "100.000000" || row["c"] != "100.000000" {
		t.Fatalf("hydration not filled, and kept as text like it was: %v", row)
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

// the shape the game writes in current saves: base and current values are JSON numbers (59.289321, 74.04, 99.43)
const hydrationNumberSQL = `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":59.289321,"CurrentValue":59.289321},"ClothingCapturedWater":{"BaseValue":0.0,"CurrentValue":0.0}},"DuneSpiceAddictionAttributeSet":{"SpiceAddictionLevel":{"BaseValue":1.5,"CurrentValue":1.5}}}') where id=2;`

const hydrationTypesSQL = `select json_type(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.BaseValue') bt,
	json_type(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') ct,
	json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.BaseValue') b,
	json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') c,
	json_extract(gas_attributes,'$.DuneHydrationAttributeSet.ClothingCapturedWater.CurrentValue') w,
	json_type(gas_attributes,'$.DuneHydrationAttributeSet.ClothingCapturedWater.CurrentValue') wt,
	json_extract(gas_attributes,'$.DuneSpiceAddictionAttributeSet.SpiceAddictionLevel.CurrentValue') s,
	json_type(gas_attributes,'$.DuneSpiceAddictionAttributeSet.SpiceAddictionLevel.CurrentValue') st,
	typeof(gas_attributes) ty from actors where id=2`

func TestFillHydrationKeepsNumbersAsNumbers(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, hydrationNumberSQL)}
	r, err := o.FillHydration()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["before"].(float64) != 59.289321 || res["after"].(float64) != 100 || res["type"] != "number" || res["baseType"] != "number" {
		t.Fatalf("result: %v", res)
	}
	row := mustOne(t, o, hydrationTypesSQL)
	if row["bt"] != "real" || row["ct"] != "real" || row["b"] != 100.0 || row["c"] != 100.0 {
		t.Fatalf("hydration must be the real number 100, not text: %v", row)
	}
	if row["w"] != 0.0 || row["wt"] != "real" || row["s"] != 1.5 || row["st"] != "real" || row["ty"] != "blob" {
		t.Fatalf("other data or the storage type changed: %v", row)
	}
	assertDBSound(t, o)
}

// a whole number (the game writes 100 as an integer in some records) is a number too and stays one
func TestFillHydrationWritesANumberOverAnIntegerToo(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":40,"CurrentValue":40}}}') where id=2;`)}
	r, err := o.FillHydration()
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["type"] != "number" || res["before"].(float64) != 40 {
		t.Fatalf("result: %v", res)
	}
	row := mustOne(t, o, hydrationTypesSQL)
	if row["bt"] == "text" || row["ct"] == "text" || row["b"] != 100.0 || row["c"] != 100.0 {
		t.Fatalf("not a number: %v", row)
	}
}

// each value keeps its own type: a text base beside a number current (and the reverse) is not unified
func TestFillHydrationKeepsEachValuesOwnType(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":"70.5","CurrentValue":59.5}}}') where id=2;`)}
	r, err := o.FillHydration()
	if err != nil {
		t.Fatal(err)
	}
	if res := r.(map[string]any); res["type"] != "number" || res["baseType"] != "text" {
		t.Fatalf("result: %v", res)
	}
	row := mustOne(t, o, hydrationTypesSQL)
	if row["bt"] != "text" || row["b"] != "100.000000" || row["ct"] != "real" || row["c"] != 100.0 {
		t.Fatalf("types were not kept: %v", row)
	}
}

// a record with a current value but no base value gets the base in the current value's type
func TestFillHydrationGivesAMissingBaseTheCurrentTypes(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"CurrentValue":59.5}}}') where id=2;`)}
	if _, err := o.FillHydration(); err != nil {
		t.Fatal(err)
	}
	if row := mustOne(t, o, hydrationTypesSQL); row["bt"] != "real" || row["b"] != 100.0 || row["ct"] != "real" {
		t.Fatalf("base: %v", row)
	}
}

func TestFillHydrationRefusesAValueThatIsNotANumber(t *testing.T) {
	for name, v := range map[string]string{"text": `"x"`, "boolean": `true`, "object": `{"a":1}`} {
		o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"BaseValue":`+v+`,"CurrentValue":`+v+`}}}') where id=2;`)}
		if _, err := o.FillHydration(); err == nil {
			t.Errorf("%s: a hydration value that is not a number was overwritten", name)
		}
		if row := mustOne(t, o, `select json_extract(gas_attributes,'$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue') c from actors where id=2`); row["c"] == 100.0 || row["c"] == "100.000000" {
			t.Errorf("%s: written: %v", name, row)
		}
	}
}

// the number is a real 100.0 in the save's JSON, as the game writes its floats
func TestFillHydrationNumberIsWrittenAsAFloat(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, hydrationNumberSQL)}
	if _, err := o.FillHydration(); err != nil {
		t.Fatal(err)
	}
	row := mustOne(t, o, `select json(gas_attributes) j from actors where id=2`)
	if j, _ := row["j"].(string); !strings.Contains(j, `"CurrentHydration":{"BaseValue":100.0,"CurrentValue":100.0}`) {
		t.Fatalf("not written as 100.0: %v", row["j"])
	}
}

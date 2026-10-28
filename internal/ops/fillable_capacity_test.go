package ops

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The capacities of the other fillable items (issue #482): the maximum amount the game's item data gives for each template (the fillable items table,
// field MaxAmount; docs/design/2026-10-28-fillable-capacities.md has the whole table). As for the blood sacks (bloodsack_capacity_test.go) the expected numbers
// are written here a second time on purpose, so that a change to waterCapacity or stillsuitCapacity has to change a test too.

// Rows with FillableTypeRestriction Water and bIsContainer true: the editor fills these.
var gameWaterCapacity = map[string]float64{
	"Literjon":                1000,
	"Literjon_03":             1100,
	"Literjon_04":             1200,
	"Literjon_05":             1300,
	"Literjon_06":             1400,
	"Literjon_07":             1500,
	"Literjon_08":             1600,
	"Literjon_09":             1700,
	"Literjon_T6":             20000,
	"Decajon":                 10000,
	"Dewpack":                 250,
	"HighCapacityLiterjon":    1500,
	"HighCapacityLiterjon_02": 1750,
	"HighCapacityLiterjon_03": 2000,
	"HighCapacityLiterjon_04": 2250,
	"HighCapacityLiterjon_05": 2500,
	"HighCapacityLiterjon_06": 3000,
}

// Rows with FillableTypeRestriction Water and bIsContainer false (a worn stillsuit): the game fills these, the editor only shows them.
var gameSuitCapacity = map[string]float64{
	"AdvancedStillsuit":                    500,
	"Combat_Nati_FremenExile04_Top":        500,
	"SimpleStillsuit":                      500,
	"Stillsuit_Choam_01_Top":               500,
	"Stillsuit_Choam_02_Top":               500,
	"Stillsuit_Choam_04_Top":               500,
	"Stillsuit_Choam_05_Top":               500,
	"Stillsuit_Choam_06_Top":               500,
	"Stillsuit_Choam_Unique_Dashed02_Top":  500,
	"Stillsuit_Choam_Unique_Dashed03_Top":  500,
	"Stillsuit_Choam_Unique_Dashed04_Top":  500,
	"Stillsuit_Choam_Unique_Dashed05_Top":  500,
	"Stillsuit_Choam_Unique_Dashed06_Top":  500,
	"Stillsuit_Nati_05_Body":               500,
	"Stillsuit_Nati_06_Body":               500,
	"Stillsuit_Nati_07_Body":               500,
	"Stillsuit_Nati_08_Body":               500,
	"Stillsuit_Nati_Arrakeen05_Body":       500,
	"Stillsuit_Neut_Leaking01_Top":         500,
	"Stillsuit_Neut_Patchy02_Top":          500,
	"Stillsuit_Unique_Armored_01_Top":      500,
	"Stillsuit_Unique_Armored_02_Top":      500,
	"Stillsuit_Unique_Armored_03_Top":      500,
	"Stillsuit_Unique_Armored_04_Top":      500,
	"Stillsuit_Unique_Armored_05_Top":      500,
	"Stillsuit_Unique_Armored_06_Top":      500,
	"Stillsuit_Unique_Efficient_04_Top":    500,
	"Stillsuit_Unique_Efficient_05_Top":    500,
	"Stillsuit_Unique_Efficient_06_Top":    500,
	"Stillsuit_Unique_HighCapacity_06_Top": 1500,
	"Stillsuit_Unique_ThermalSuit_06_Top":  500,
}

func sortedKeys(m map[string]float64) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func TestWaterCapacityIsExactlyWhatTheGameDataGives(t *testing.T) {
	if len(waterCapacity) != len(gameWaterCapacity) {
		t.Fatalf("waterCapacity has %d templates, the game's data gives %d: %v", len(waterCapacity), len(gameWaterCapacity), waterCapacity)
	}
	for tpl, want := range gameWaterCapacity {
		if got, ok := waterCapacity[tpl]; !ok || got != want {
			t.Errorf("%s: got %v (listed: %v), the game's data gives %v", tpl, got, ok, want)
		}
	}
	// the nine that were known before the game's data was read are unchanged
	for tpl, want := range map[string]float64{"Literjon": 1000, "Literjon_T6": 20000, "Decajon": 10000, "HighCapacityLiterjon": 1500, "HighCapacityLiterjon_02": 1750,
		"HighCapacityLiterjon_03": 2000, "HighCapacityLiterjon_04": 2250, "HighCapacityLiterjon_05": 2500, "HighCapacityLiterjon_06": 3000} {
		if waterCapacity[tpl] != want {
			t.Errorf("%s changed: %v, was %v", tpl, waterCapacity[tpl], want)
		}
	}
}

func TestStillsuitCapacityIsExactlyWhatTheGameDataGives(t *testing.T) {
	if len(stillsuitCapacity) != len(gameSuitCapacity) {
		t.Fatalf("stillsuitCapacity has %d templates, the game's data gives %d", len(stillsuitCapacity), len(gameSuitCapacity))
	}
	for tpl, want := range gameSuitCapacity {
		if got, ok := stillsuitCapacity[tpl]; !ok || got != want {
			t.Errorf("%s: got %v (listed: %v), the game's data gives %v", tpl, got, ok, want)
		}
		if _, both := waterCapacity[tpl]; both {
			t.Errorf("%s is both a container and a suit", tpl)
		}
	}
	for tpl := range bloodCapacity {
		if _, both := stillsuitCapacity[tpl]; both {
			t.Errorf("%s is both a blood sack and a suit", tpl)
		}
	}
}

// For every listed water container: an empty record, no record, a partly full container, a full one, one holding something else and one above its capacity.
func TestEveryListedWaterContainerFillsToItsCapacityAndNoFurther(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(clearWorld + "\ninsert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values\n")
	type ids struct{ empty, none, partial, full, blood, over int }
	byTpl := map[string]ids{}
	n := 0
	row := func(id int, tpl, stats string) {
		if n > 0 {
			sb.WriteString(",\n")
		}
		n++
		fmt.Fprintf(&sb, "  (%d,1,1,%d,'%s',0,1790000000,'%s',1)", id, 1000+id, tpl, stats)
	}
	for i, tpl := range sortedKeys(gameWaterCapacity) {
		base := 2000 + i*10
		c := gameWaterCapacity[tpl]
		byTpl[tpl] = ids{base, base + 1, base + 2, base + 3, base + 4, base + 5}
		row(base, tpl, `{"FFillableItemStats":[[],{}],`+bsDur+`}`)
		row(base+1, tpl, `{`+bsDur+`}`)
		row(base+2, tpl, `{`+fillRec("40.0", "Water")+`,`+bsDur+`}`)
		row(base+3, tpl, `{`+fillRec(strconv.FormatFloat(c, 'f', 1, 64), "Water")+`,`+bsDur+`}`)
		row(base+4, tpl, `{`+fillRec("40.0", "Blood")+`,`+bsDur+`}`)
		row(base+5, tpl, `{`+fillRec(strconv.FormatFloat(c+500, 'f', 1, 64), "Water")+`,`+bsDur+`}`)
	}
	sb.WriteString(";")
	o := &Ops{S: testsave.PlayerWithSQL(t, sb.String())}

	untouched := map[int]string{}
	for _, tpl := range sortedKeys(gameWaterCapacity) {
		x := byTpl[tpl]
		for _, id := range []int{x.full, x.blood, x.over} {
			untouched[id] = sackStats(t, o, id)
		}
	}
	res, err := o.RefillContainers()
	if err != nil {
		t.Fatal(err)
	}
	k := len(gameWaterCapacity)
	m := res.(map[string]any)
	if m["ok"] != true || m["filled"].(int) != 3*k || m["alreadyFull"].(int) != 2*k || len(m["skippedUnknown"].([]string)) != 0 || m["stillsuits"].(int) != 0 {
		t.Fatalf("answer: %v", m)
	}
	for _, tpl := range sortedKeys(gameWaterCapacity) {
		x := byTpl[tpl]
		want := strconv.FormatFloat(gameWaterCapacity[tpl], 'f', 1, 64)
		if got := sackStats(t, o, x.empty); got != `{"FFillableItemStats":[[],{"CurrentAmount":`+want+`,"FillableType":"Water"}],`+bsDur+`}` {
			t.Errorf("%s empty record: %s", tpl, got)
		}
		if got := sackStats(t, o, x.none); got != `{`+bsDur+`,`+fillRec(want, "Water")+`}` {
			t.Errorf("%s no record: %s", tpl, got)
		}
		if got := sackStats(t, o, x.partial); got != `{`+fillRec(want, "Water")+`,`+bsDur+`}` {
			t.Errorf("%s partly full: %s", tpl, got)
		}
		for what, id := range map[string]int{"a full container": x.full, "a container holding blood": x.blood, "a container above its capacity": x.over} {
			if got := sackStats(t, o, id); got != untouched[id] {
				t.Errorf("%s: %s was rewritten: %s", tpl, what, got)
			}
		}
	}
	if v := fkViolations(t, o); v != 0 {
		t.Errorf("%d dangling references", v)
	}
	if r := mustOne(t, o, `pragma integrity_check`); r["integrity_check"] != "ok" {
		t.Errorf("integrity check: %v", r)
	}
	again, err := o.RefillContainers()
	if err != nil {
		t.Fatal(err)
	}
	if a := again.(map[string]any); a["filled"].(int) != 0 || a["alreadyFull"].(int) != 5*k {
		t.Fatalf("second run: %v", a)
	}
}

// A water container that the game's data does not list stays unknown: left alone and reported (when it has a record), however close its name is to a listed one.
func TestAnUnlistedWaterContainerIsStillLeftAloneAndReported(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, clearWorld+`
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (601,1,1,101,'Literjon_03',0,1790000000,'{"FFillableItemStats":[[],{}]}',2),
  (602,1,1,102,'Literjon_02',0,1790000000,'{"FFillableItemStats":[[],{}]}',2),
  (603,1,1,103,'Literjon_10',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":10.0,"FillableType":"Water"}]}',2),
  (604,1,1,104,'Dewpack_02',0,1790000000,'{"FFillableItemStats":[[],{}]}',2),
  (605,1,1,105,'Literjon_11',0,1790000000,'{}',2);`)}
	before := map[int]string{}
	for _, id := range []int{602, 603, 604, 605} {
		before[id] = sackStats(t, o, id)
	}
	res, err := o.RefillContainers()
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["filled"].(int) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if sk := strings.Join(m["skippedUnknown"].([]string), ","); sk != "Dewpack_02 x1,Literjon_02 x1,Literjon_10 x1" {
		t.Fatalf("the unknown kinds are reported: %q", sk)
	}
	for id, s := range before {
		if got := sackStats(t, o, id); got != s {
			t.Errorf("item %d must not change: %s -> %s", id, s, got)
		}
	}
	if got := sackStats(t, o, 601); got != `{"FFillableItemStats":[[],{"CurrentAmount":1100.0,"FillableType":"Water"}]}` {
		t.Errorf("Literjon_03: %s", got)
	}
}

// A stillsuit is not a container the person fills: neither water button writes it, and it is not reported as a container of unknown capacity.
func TestAWornStillsuitIsLeftAloneByEveryFillButton(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(clearWorld + "\ninsert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values\n")
	n := 0
	var all []int
	row := func(id int, tpl, stats string) {
		if n > 0 {
			sb.WriteString(",\n")
		}
		n++
		all = append(all, id)
		fmt.Fprintf(&sb, "  (%d,2,1,%d,'%s',0,1790000000,'%s',1)", id, 1000+id, tpl, stats)
	}
	for i, tpl := range sortedKeys(gameSuitCapacity) {
		base := 3000 + i*10
		row(base, tpl, `{"FFillableItemStats":[[],{"CurrentAmount":140.765075}],`+bsDur+`}`) // as the game saves it: no liquid type, an amount that moves
		row(base+1, tpl, `{"FFillableItemStats":[[],{"CurrentAmount":`+strconv.FormatFloat(gameSuitCapacity[tpl], 'f', 1, 64)+`}],`+bsDur+`}`)
		row(base+2, tpl, `{"FFillableItemStats":[[],{}],`+bsDur+`}`)
		row(base+3, tpl, `{`+bsDur+`}`)
	}
	sb.WriteString(";")
	o := &Ops{S: testsave.PlayerWithSQL(t, sb.String())}
	// the world's own items in inventory 2 (worn gear) are not part of the check: only the rows above
	before := map[int]string{}
	for _, id := range all {
		before[id] = sackStats(t, o, id)
	}
	k := len(gameSuitCapacity)

	st, err := o.LiquidsStatus()
	if err != nil {
		t.Fatal(err)
	}
	if w := st.(map[string]any)["water"].(map[string]any); w["total"].(int) != 0 || w["notFull"].(int) != 0 || w["unknown"].(int) != 0 {
		t.Fatalf("the count beside Fill all liquids must not count a suit: %v", w)
	}
	res, err := o.RefillContainers()
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["filled"].(int) != 0 || m["alreadyFull"].(int) != 0 || len(m["skippedUnknown"].([]string)) != 0 || m["stillsuits"].(int) != 4*k {
		t.Fatalf("Fill water containers: %v", m)
	}
	all2, err := o.FillAllLiquids(Args{"vehicles": false})
	if err != nil {
		t.Fatal(err)
	}
	if w := all2.(map[string]any)["water"].(map[string]any); w["filled"].(int) != 0 || len(w["skippedUnknown"].([]string)) != 0 {
		t.Fatalf("Fill all liquids: %v", w)
	}
	for _, id := range all {
		if got := sackStats(t, o, id); got != before[id] {
			t.Errorf("item %d (a stillsuit) was rewritten: %s -> %s", id, before[id], got)
		}
	}
}

// What the Inventory line shows for a stillsuit (the case of issue #482: Menol's Stillsuit Garment, Stillsuit_Unique_Armored_02_Top).
func TestAStillsuitShowsItsWaterAgainstTheGamesCapacityAndSaysItFillsItself(t *testing.T) {
	for _, tpl := range sortedKeys(gameSuitCapacity) {
		c := gameSuitCapacity[tpl]
		got := fillOf(tpl, `{"FFillableItemStats":[[],{"CurrentAmount":140.765075}]}`)
		if got == nil || got["kind"] != "Water" || got["state"] != "partial" || got["selfFilled"] != true || got["capacity"].(float64) != c ||
			got["percent"].(float64) != math.Floor(140.765075/c*100+0.5) || got["amount"].(float64) != 140.765075 {
			t.Errorf("%s with water: %v", tpl, got)
		}
		if got := fillOf(tpl, `{"FFillableItemStats":[[],{"CurrentAmount":`+ftoa(c)+`}]}`); got["state"] != "full" || got["selfFilled"] != true {
			t.Errorf("%s full: %v", tpl, got)
		}
		if got := fillOf(tpl, `{"FFillableItemStats":[[],{}]}`); got["state"] != "empty" || got["kind"] != "Water" || got["selfFilled"] != true || got["capacity"].(float64) != c {
			t.Errorf("%s empty: %v", tpl, got)
		}
		if got := fillOf(tpl, `{}`); got == nil || got["state"] != "empty" || got["selfFilled"] != true {
			t.Errorf("%s with no record: %v", tpl, got)
		}
		// the person's other containers are not marked
		if got := fillOf(tpl, `{"FFillableItemStats":[[],{"CurrentAmount":9.0,"FillableType":"Blood"}]}`); got["state"] != "unknown" || got["selfFilled"] != nil {
			t.Errorf("%s holding blood: %v", tpl, got)
		}
	}
	for _, tpl := range sortedKeys(gameWaterCapacity) {
		if got := fillOf(tpl, `{"FFillableItemStats":[[],{"CurrentAmount":9.0,"FillableType":"Water"}]}`); got["selfFilled"] != nil || got["capacity"].(float64) != gameWaterCapacity[tpl] {
			t.Errorf("%s: %v", tpl, got)
		}
	}
	// a suit the data does not list is not given a capacity
	if got := fillOf("Stillsuit_Unique_Armored_07_Top", `{"FFillableItemStats":[[],{"CurrentAmount":140.0}]}`); got["state"] != "unknown" || got["capacity"] != nil || got["selfFilled"] != nil {
		t.Errorf("an unlisted suit: %v", got)
	}
}

// The Inventory read for the operator's case: a worn Menol's Stillsuit Garment holding 140.765075, a Literjon_03 and an unlisted container.
func TestInventoryShowsMenolsGarmentAndTheNewLiterjon(t *testing.T) {
	s := testsave.PlayerWithSQL(t, `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (21,2,1,10,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (201,21,1,0,'Stillsuit_Unique_Armored_02_Top',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":140.765075}]}',1),
  (202,21,1,1,'Literjon_03',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":550.0,"FillableType":"Water"}]}',1),
  (203,21,1,2,'Mystery_Flask',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":5.0,"FillableType":"Water"}]}',1);`)
	before, _ := s.One(`select stats from items where id=201`)
	o := &Ops{S: s}
	r, err := o.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]map[string]any{}
	for _, it := range r.(map[string]any)["items"].([]map[string]any) {
		f, _ := it["fill"].(map[string]any)
		seen[it["id"].(int64)] = f
	}
	if f := seen[201]; f == nil || f["selfFilled"] != true || f["capacity"].(float64) != 500 || f["percent"].(float64) != 28 || f["state"] != "partial" {
		t.Errorf("Menol's Stillsuit Garment: %v", f)
	}
	if f := seen[202]; f == nil || f["state"] != "partial" || f["capacity"].(float64) != 1100 || f["percent"].(float64) != 50 {
		t.Errorf("Literjon_03: %v", f)
	}
	if f := seen[203]; f == nil || f["state"] != "unknown" || f["capacity"] != nil {
		t.Errorf("an unlisted container is still unknown: %v", f)
	}
	after, _ := s.One(`select stats from items where id=201`)
	if before["stats"] != after["stats"] {
		t.Error("reading the fill level changed the item")
	}
}

package ops

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The capacities of the blood sacks (issue #472): the maximum amount the game's item data gives for each template (the fillable items table,
// field MaxAmount; docs/design/2026-10-28-blood-sack-capacity.md). The expected numbers are written here a second time on purpose, so that a
// change to bloodCapacity has to change a test too.
var gameBloodCapacity = map[string]float64{
	"Bloodsack_01":                3000,
	"Bloodsack_02":                9000,
	"Bloodsack_03":                15000,
	"Bloodsack_T6":                24000,
	"Bloodsack_Unique_Durable_02": 13000,
	"Bloodsack_Unique_Durable_03": 16000,
	"Bloodsack_Unique_Durable_04": 19000,
	"Bloodsack_Unique_Durable_05": 22000,
	"Bloodsack_Unique_Durable_06": 30000,
}

func sortedSacks() []string {
	names := make([]string, 0, len(gameBloodCapacity))
	for n := range gameBloodCapacity {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func TestBloodCapacityIsExactlyWhatTheGameDataGives(t *testing.T) {
	if len(bloodCapacity) != len(gameBloodCapacity) {
		t.Fatalf("bloodCapacity has %d templates, the game's data gives %d: %v", len(bloodCapacity), len(gameBloodCapacity), bloodCapacity)
	}
	for tpl, want := range gameBloodCapacity {
		if got, ok := bloodCapacity[tpl]; !ok || got != want {
			t.Errorf("%s: got %v (listed: %v), the game's data gives %v", tpl, got, ok, want)
		}
	}
	// the two that were known before the game's data was read are unchanged
	if bloodCapacity["Bloodsack_01"] != 3000 || bloodCapacity["Bloodsack_Unique_Durable_06"] != 30000 {
		t.Errorf("the earlier two changed: %v %v", bloodCapacity["Bloodsack_01"], bloodCapacity["Bloodsack_Unique_Durable_06"])
	}
}

// For every listed template: an empty record, no record, a partly full sack, a full one and a sack holding something else.
func TestEveryListedBloodSackFillsToItsCapacityAndNoFurther(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(clearWorld + "\ninsert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values\n")
	type ids struct{ empty, none, partial, full, water int }
	byTpl := map[string]ids{}
	n := 0
	row := func(id int, tpl, stats string) {
		if n > 0 {
			sb.WriteString(",\n")
		}
		n++
		fmt.Fprintf(&sb, "  (%d,1,1,%d,'%s',0,1790000000,'%s',1)", id, 1000+id, tpl, stats)
	}
	for i, tpl := range sortedSacks() {
		base := 2000 + i*10
		c := gameBloodCapacity[tpl]
		full := fillRec(strconv.FormatFloat(c, 'f', 1, 64), "Blood")
		byTpl[tpl] = ids{base, base + 1, base + 2, base + 3, base + 4}
		row(base, tpl, `{"FFillableItemStats":[[],{}],`+bsDur+`}`)
		row(base+1, tpl, `{`+bsDur+`}`)
		row(base+2, tpl, `{`+fillRec("1466.0", "Blood")+`,`+bsDur+`}`)
		row(base+3, tpl, `{`+full+`,`+bsDur+`}`)
		row(base+4, tpl, `{`+fillRec("40.0", "Water")+`,`+bsDur+`}`)
	}
	sb.WriteString(";")
	o := &Ops{S: testsave.PlayerWithSQL(t, sb.String())}

	untouched := map[int]string{}
	for _, tpl := range sortedSacks() {
		x := byTpl[tpl]
		untouched[x.full] = sackStats(t, o, x.full)
		untouched[x.water] = sackStats(t, o, x.water)
	}
	res, err := o.FillBloodSacks()
	if err != nil {
		t.Fatal(err)
	}
	k := len(gameBloodCapacity)
	m := res.(map[string]any)
	if m["ok"] != true || m["filled"].(int) != 3*k || m["alreadyFull"].(int) != k || m["holdingOther"].(int) != k || m["unknown"].(int) != 0 {
		t.Fatalf("answer: %v", m)
	}
	for _, tpl := range sortedSacks() {
		x := byTpl[tpl]
		want := strconv.FormatFloat(gameBloodCapacity[tpl], 'f', 1, 64)
		if by := m["byTemplate"].(map[string]int); by[tpl] != 3 {
			t.Errorf("%s: filled %d, want 3", tpl, by[tpl])
		}
		if got := sackStats(t, o, x.empty); got != `{"FFillableItemStats":[[],{"CurrentAmount":`+want+`,"FillableType":"Blood"}],`+bsDur+`}` {
			t.Errorf("%s empty record: %s", tpl, got)
		}
		if got := sackStats(t, o, x.none); got != `{`+bsDur+`,`+fillRec(want, "Blood")+`}` {
			t.Errorf("%s no record: %s", tpl, got)
		}
		if got := sackStats(t, o, x.partial); got != `{`+fillRec(want, "Blood")+`,`+bsDur+`}` {
			t.Errorf("%s partly full: %s", tpl, got)
		}
		if got := sackStats(t, o, x.full); got != untouched[x.full] {
			t.Errorf("%s: a full sack was rewritten: %s", tpl, got)
		}
		if got := sackStats(t, o, x.water); got != untouched[x.water] {
			t.Errorf("%s: a sack holding water was rewritten: %s", tpl, got)
		}
	}
	if v := fkViolations(t, o); v != 0 {
		t.Errorf("%d dangling references", v)
	}
	if r := mustOne(t, o, `pragma integrity_check`); r["integrity_check"] != "ok" {
		t.Errorf("integrity check: %v", r)
	}
	again, err := o.FillBloodSacks()
	if err != nil {
		t.Fatal(err)
	}
	if a := again.(map[string]any); a["filled"].(int) != 0 || a["alreadyFull"].(int) != 4*k {
		t.Fatalf("second run: %v", a)
	}
}

// The case that opened issue #472: four Bloodsack_02, one holding 1466, two with durability only, one with an empty record.
func TestTheFourBloodsack02OfIssue472AreFilledTo9000(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, clearWorld+`
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (401,1,1,101,'Bloodsack_02',0,1790000000,'{`+fillRec("1466.0", "Blood")+`,`+bsDur+`}',2),
  (402,1,1,102,'Bloodsack_02',0,1790000000,'{`+bsDur+`}',2),
  (403,1,1,103,'Bloodsack_02',0,1790000000,'{`+bsDur+`}',2),
  (404,1,1,104,'Bloodsack_02',0,1790000000,'{}',2);`)}
	before, err := o.BloodSacks()
	if err != nil {
		t.Fatal(err)
	}
	if m := before.(map[string]any); m["total"].(int) != 4 || m["notFull"].(int) != 4 || m["unknown"].(int) != 0 {
		t.Fatalf("the count beside the button: %v", m)
	}
	res, err := o.FillBloodSacks()
	if err != nil {
		t.Fatal(err)
	}
	if m := res.(map[string]any); m["filled"].(int) != 4 || m["unknown"].(int) != 0 || m["alreadyFull"].(int) != 0 {
		t.Fatalf("answer: %v", m)
	}
	for _, id := range []int{401, 402, 403, 404} {
		if got := sackStats(t, o, id); !strings.Contains(got, `"CurrentAmount":9000.0`) || !strings.Contains(got, `"FillableType":"Blood"`) {
			t.Errorf("sack %d: %s", id, got)
		}
	}
	after, _ := o.BloodSacks()
	if m := after.(map[string]any); m["total"].(int) != 4 || m["notFull"].(int) != 0 {
		t.Fatalf("after the fill: %v", m)
	}
}

// A blood sack template that the game's data does not list stays unknown: left alone and reported, however close its name is to a listed one.
func TestAnUnlistedBloodSackTemplateIsStillLeftAloneAndReported(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, clearWorld+`
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (501,1,1,101,'Bloodsack_02',0,1790000000,'{"FFillableItemStats":[[],{}]}',2),
  (502,1,1,102,'Bloodsack_04',0,1790000000,'{"FFillableItemStats":[[],{}]}',2),
  (503,1,1,103,'Bloodsack_Unique_Durable_01',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":10.0,"FillableType":"Blood"}]}',6),
  (504,1,1,104,'Bloodsack_Unique_Durable_07',0,1790000000,'{}',6);`)}
	before := map[int]string{}
	for _, id := range []int{502, 503, 504} {
		before[id] = sackStats(t, o, id)
	}
	res, err := o.FillBloodSacks()
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["filled"].(int) != 1 || m["unknown"].(int) != 3 {
		t.Fatalf("answer: %v", m)
	}
	if sk := strings.Join(m["skippedUnknown"].([]string), ","); sk != "Bloodsack_04 x1,Bloodsack_Unique_Durable_01 x1,Bloodsack_Unique_Durable_07 x1" {
		t.Fatalf("the unknown kinds are reported: %q", sk)
	}
	for id, s := range before {
		if got := sackStats(t, o, id); got != s {
			t.Errorf("item %d must not change: %s -> %s", id, s, got)
		}
	}
}

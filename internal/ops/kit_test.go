package ops

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/kits"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Apply kit (#417): a care package goes to the character in ONE edit, by the Give Items path (slots, stacks, volume, augments), then the XP and the skill points. The fixture's backpack
// (inventory 1) has 5 slots and 2 are used (the Solari in slot 0, a Knife in slot 1), so 3 are free; Spice-infused Fuel Cells stack to 500; the level record is that of xp_test.go.

func kitOps(t *testing.T, xp, total, unspent int) *Ops {
	t.Helper()
	comp := fmt.Sprintf(`{"FLevelComponent":[[],{"ModuleData":{"Skills.Key.Mentat1":{"SkillPointsSpent":1}},"TotalXPEarned":%d,"KeystoneBonusSkillPoints":0,"TotalSkillPoints":%d,"UnspentSkillPoints":%d,"LastViewedSkillTree":"x"}]}`, xp, total, unspent)
	return &Ops{S: testsave.PlayerWithSQL(t, `insert into fgl_entities(entity_id,components) values (-10, jsonb('`+comp+`'));
		insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,-10,'DuneCharacter');`)}
}

func starterKit() kits.Kit {
	return kits.Kit{Name: "Starter", Items: []kits.Line{
		{Item: fuel, Quantity: 1000},
		{Item: helmet, Quantity: 1, Grade: 3, Augments: []string{"T6_Augment_Armor10"}, AugmentGrade: 4},
	}, XP: 215, SkillPoints: 3}
}

func asMap(t *testing.T, v any, err error) map[string]any {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%T", v)
	}
	return m
}

func TestApplyKitGivesTheItemsTheXPAndTheSkillPointsInOneEdit(t *testing.T) {
	o := kitOps(t, 0, 0, 0)
	r := apply(t, o, starterKit(), false)
	if r["ok"] != true || r["kit"] != "Starter" || r["dryRun"] != false || r["complete"] != true || r["requested"] != int64(1001) || r["given"] != int64(1001) {
		t.Fatalf("%v", r)
	}
	if got := fuelStacks(t, o); got != "[500 500]" {
		t.Fatalf("fuel stacks %s", got)
	}
	// the helmet: grade 3, the augment at grade 4, in the one slot left
	h := mustOne(t, o, `select quality_level q, stats from items where template_id=?`, helmet)
	if h["q"] != int64(3) || !strings.Contains(h["stats"].(string), `"AppliedAugmentQualities":[4]`) || !strings.Contains(h["stats"].(string), "T6_Augment_Armor10") {
		t.Fatalf("%v", h)
	}
	// 215 XP is level 2: two skill points from the levels, three from the kit
	xp, total, unspent := levelState(t, o)
	if xp != 215 || total != 2+3 || unspent != 2+3 {
		t.Fatalf("xp=%d total=%d unspent=%d", xp, total, unspent)
	}
	xr := r["xp"].(map[string]any)
	if xr["applied"] != int64(215) || xr["levelAfter"] != 2 || xr["requested"] != int64(215) {
		t.Fatalf("%v", xr)
	}
	if sr := r["skillPoints"].(map[string]any); sr["applied"] != int64(3) || sr["requested"] != int64(3) || sr["capped"] != false {
		t.Fatalf("%v", sr)
	}
	if ng := r["notGiven"].([]map[string]any); len(ng) != 0 {
		t.Fatalf("%v", ng)
	}
	// ONE edit, with a line for the write log that says what the kit did
	p := o.S.Pending()
	if len(p) != 1 || p[0] != `Apply kit "Starter": 2 items, 1,001 units, +215 XP, +3 skill points` {
		t.Fatalf("%v", p)
	}
}

func TestApplyKitGivesWhatFitsAndSaysWhatDidNotAndWhy(t *testing.T) {
	// 3 free slots: 2,000 fuel cells need 4 stacks, so 1,500 go in and the helmet has no slot at all
	o := kitOps(t, 0, 0, 0)
	k := starterKit()
	k.Items[0].Quantity = 2000
	r := apply(t, o, k, false)
	if r["complete"] != false || r["requested"] != int64(2001) || r["given"] != int64(1500) {
		t.Fatalf("%v", r)
	}
	ng := r["notGiven"].([]map[string]any)
	if len(ng) != 2 || ng[0]["name"] != fuelLabel || ng[0]["short"] != int64(500) || ng[0]["reason"] != "the backpack is full" || ng[1]["short"] != int64(1) {
		t.Fatalf("%v", ng)
	}
	lines := r["lines"].([]map[string]any)
	if lines[0]["given"] != int64(1500) || lines[1]["given"] != int64(0) || lines[1]["limit"] != "slots" {
		t.Fatalf("%v", lines)
	}
	// the XP and the skill points still went in, in the same edit
	if xp, total, unspent := levelState(t, o); xp != 215 || total != 5 || unspent != 5 {
		t.Fatalf("%d %d %d", xp, total, unspent)
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != `Apply kit "Starter": 1,500 of 2,001 units fitted, +215 XP, +3 skill points` {
		t.Fatalf("%v", p)
	}
}

func TestAKitThatFitsNothingAndAddsNothingElseIsRefusedAndWritesNothing(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=2 where id=1;`)}
	_, err := o.ApplyKit(kits.Kit{Name: "Fuel", Items: []kits.Line{{Item: fuel, Quantity: 10}}}, false)
	if err == nil || !isNoRoom(err) || !strings.Contains(err.Error(), "Nothing was given") {
		t.Fatalf("%v", err)
	}
	if o.S.Dirty() || len(o.S.Pending()) != 0 {
		t.Fatal("nothing may be pending")
	}
	// the same kit with XP is not a refusal: the XP is a reason to write; the items that did not fit are reported
	o = &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=2 where id=1;
		insert into fgl_entities(entity_id,components) values (-10, jsonb('{"FLevelComponent":[[],{"ModuleData":{},"TotalXPEarned":0,"TotalSkillPoints":0,"UnspentSkillPoints":0}]}'));
		insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,-10,'DuneCharacter');`)}
	r := apply(t, o, kits.Kit{Name: "Fuel", Items: []kits.Line{{Item: fuel, Quantity: 10}}, XP: 40}, false)
	if r["given"] != int64(0) || r["complete"] != false || r["xp"].(map[string]any)["applied"] != int64(40) {
		t.Fatalf("%v", r)
	}
}

func TestApplyKitWithoutALevelRecordRefusesTheWholeKit(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	_, err := o.ApplyKit(starterKit(), false)
	if err == nil || !strings.Contains(err.Error(), "no level record") {
		t.Fatalf("%v", err)
	}
	if o.S.Dirty() || fuelIn(t, o) != 0 {
		t.Fatal("the items must not stay when the XP cannot be given: one edit, all or nothing for a fault")
	}
}

func fuelIn(t *testing.T, o *Ops) int64 {
	t.Helper()
	r := mustOne(t, o, `select coalesce(sum(stack_size),0) n from items where template_id=?`, fuel)
	return r["n"].(int64)
}

func TestSkillPointsStopAtTheLimitOfUnspentPoints(t *testing.T) {
	o := kitOps(t, 0, 99990, 99990)
	r := apply(t, o, kits.Kit{Name: "SP", SkillPoints: 50}, false)
	sr := r["skillPoints"].(map[string]any)
	if sr["applied"] != int64(10) || sr["capped"] != true || sr["requested"] != int64(50) {
		t.Fatalf("%v", sr)
	}
	if _, total, unspent := levelState(t, o); total != 100000 || unspent != 100000 {
		t.Fatalf("%d %d", total, unspent)
	}
	// at the limit already: nothing changes and nothing is left pending
	o = kitOps(t, 0, 100000, 100000)
	r = apply(t, o, kits.Kit{Name: "SP", SkillPoints: 5}, false)
	if r["changed"] != false || r["skillPoints"].(map[string]any)["applied"] != int64(0) || o.S.Dirty() {
		t.Fatalf("%v", r)
	}
}

func TestAKitOfXPAloneNeedsNoBackpack(t *testing.T) {
	o := kitOps(t, 0, 0, 0)
	r := apply(t, o, kits.Kit{Name: "Levels", XP: 440}, false)
	if r["requested"] != int64(0) || r["given"] != int64(0) || r["complete"] != true || r["changed"] != true {
		t.Fatalf("%v", r)
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != `Apply kit "Levels": +440 XP` {
		t.Fatalf("%v", p)
	}
}

func TestADryRunReportsTheSameAndChangesNothing(t *testing.T) {
	for _, k := range []kits.Kit{starterKit(), {Name: "Big", Items: []kits.Line{{Item: fuel, Quantity: 2000}, {Item: helmet, Quantity: 1}}, XP: 215, SkillPoints: 3}} {
		dry := kitOps(t, 0, 0, 0)
		before := rowsExcept(t, dry, "")
		d := apply(t, dry, k, true)
		if d["dryRun"] != true {
			t.Fatalf("%v", d)
		}
		if dry.S.Dirty() || len(dry.S.Pending()) != 0 || rowsExcept(t, dry, "") != before {
			t.Fatal("a dry run changed the save")
		}
		real := kitOps(t, 0, 0, 0)
		r := apply(t, real, k, false)
		delete(d, "dryRun")
		delete(r, "dryRun")
		if fmt.Sprint(d) != fmt.Sprint(r) {
			t.Fatalf("the dry run said\n%v\nand the apply did\n%v", d, r)
		}
	}
}

func TestADryRunOfAKitThatFitsNothingReportsInsteadOfRefusing(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_count=2 where id=1;`)}
	r := apply(t, o, kits.Kit{Name: "Fuel", Items: []kits.Line{{Item: fuel, Quantity: 10}}}, true)
	if r["given"] != int64(0) || r["complete"] != false || len(r["notGiven"].([]map[string]any)) != 1 {
		t.Fatalf("%v", r)
	}
}

func TestApplyKitStopsAtTheVolumeLimitLikeGive(t *testing.T) {
	// a backpack with room for 142 aluminum bars (0.7 each) by volume
	o := &Ops{S: testsave.PlayerWithSQL(t, `update inventories set max_item_volume=100.0, max_item_count=0 where id=1;`)}
	r := apply(t, o, kits.Kit{Name: "Bars", Items: []kits.Line{{Item: "AluminiumBar", Quantity: 1000}}}, false)
	ng := r["notGiven"].([]map[string]any)
	if r["given"] != int64(142) || len(ng) != 1 || ng[0]["reason"] != "the backpack is full (volume)" || ng[0]["short"] != int64(858) {
		t.Fatalf("%v", r)
	}
}

func TestApplyKitChecksTheKitAgain(t *testing.T) {
	o := kitOps(t, 0, 0, 0)
	for _, k := range []kits.Kit{
		{Name: "Bad", Items: []kits.Line{{Item: "NoSuchThing", Quantity: 1}}},
		{Name: "Bad", Items: []kits.Line{{Item: fuel, Quantity: 0}}},
		{Name: "../x", XP: 1},
		{Name: "Empty"},
		{Name: "Aug", Items: []kits.Line{{Item: fuel, Quantity: 1, Augments: []string{"T6_Augment_Armor10"}}}},
	} {
		for _, dry := range []bool{true, false} {
			if _, err := o.ApplyKit(k, dry); err == nil || !strings.Contains(fmt.Sprint(apierrCode(err)), "bad") {
				t.Errorf("%+v dry=%v: %v", k, dry, err)
			}
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused kit wrote something")
	}
}

func apierrCode(err error) string {
	if errors.Is(err, apierr.ErrBadArgument) {
		return "bad_argument"
	}
	return "other"
}

// A kit given twice gives twice (the kit remembers nothing about the save): the second apply tops up the stacks of the first.
func TestApplyingAKitTwiceGivesItTwice(t *testing.T) {
	o := kitOps(t, 0, 0, 0)
	k := kits.Kit{Name: "Fuel", Items: []kits.Line{{Item: fuel, Quantity: 300}}, XP: 40}
	apply(t, o, k, false)
	r := apply(t, o, k, false)
	if r["given"] != int64(300) || fuelIn(t, o) != 600 {
		t.Fatalf("%v", r)
	}
	if got := fuelStacks(t, o); got != "[500 100]" {
		t.Fatalf("the second apply fills the stack of the first first: %s", got)
	}
	if xp, _, _ := levelState(t, o); xp != 80 {
		t.Fatalf("%d", xp)
	}
	if len(o.S.Pending()) != 2 {
		t.Fatalf("one edit per apply: %v", o.S.Pending())
	}
}

func apply(t *testing.T, o *Ops, k kits.Kit, dry bool) map[string]any {
	t.Helper()
	v, err := o.ApplyKit(k, dry)
	return asMap(t, v, err)
}

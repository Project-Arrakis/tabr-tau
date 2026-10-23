package ops

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Filling blood sacks (issue #60), on the real schema. The character (pawn 2) carries, in the backpack (inventory 1), the worn gear (2) and the loadout (3):
//
//	301 Bloodsack_Unique_Durable_06, key present, empty object          -> 30000
//	302 Bloodsack_Unique_Durable_06, 24000 blood                       -> 30000
//	303 Bloodsack_Unique_Durable_06, 30000 blood (full)                 -> unchanged
//	304 Bloodsack_01, no fill key at all                                -> 3000
//	305 Bloodsack_01, key present, empty object                        -> 3000
//	306 Bloodsack_02, an unknown template                               -> left alone, reported
//	307 Bloodsack_Unique_Durable_06 in the loadout, empty              -> 30000
//	308 Bloodsack_01 holding water                                      -> left alone
//	309 Bloodsack_01 with an odd record ([])                            -> 3000
//	310 Bloodsack_01, integer amount 3000 (full)                        -> unchanged
//	311 Bloodsack_Unique_Durable_06 above its capacity (31000)          -> never lowered
//	312 Bloodsack_01 with a record whose second part is not an object -> 3000
//	320 Literjon with 500 water (backpack)                              -> not touched by the blood fill
//	330 Bloodsack_01 in a base chest (actor 701, not the character's)   -> not touched
const (
	bsCust = `"FCustomizationStats":[[],{"Color":1.5}]`
	bsDur  = `"FItemStackAndDurabilityStats":[[],{"CurrentDurability":77.5}]`
)

const bloodWorld = clearWorld + `
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (301,1,1,101,'Bloodsack_Unique_Durable_06',0,1790000000,'{` + bsCust + `,"FFillableItemStats":[[],{}],` + bsDur + `}',6),
  (302,1,1,102,'Bloodsack_Unique_Durable_06',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":24000.0,"FillableType":"Blood","OtherKey":7}],` + bsCust + `,` + bsDur + `}',6),
  (303,1,1,103,'Bloodsack_Unique_Durable_06',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":30000.0,"FillableType":"Blood"}],` + bsDur + `}',6),
  (304,1,1,104,'Bloodsack_01',0,1790000000,'{` + bsCust + `,` + bsDur + `}',1),
  (305,1,1,105,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[[],{}],` + bsDur + `}',1),
  (306,1,1,106,'Bloodsack_02',0,1790000000,'{"FFillableItemStats":[[],{}],` + bsDur + `}',2),
  (307,3,1,107,'Bloodsack_Unique_Durable_06',0,1790000000,'{"FFillableItemStats":[[],{}],` + bsDur + `}',6),
  (308,1,1,108,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":40.0,"FillableType":"Water"}],` + bsDur + `}',1),
  (309,1,1,109,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[],` + bsDur + `}',1),
  (310,1,1,110,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":3000,"FillableType":"Blood"}],` + bsDur + `}',1),
  (311,1,1,111,'Bloodsack_Unique_Durable_06',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":31000.0,"FillableType":"Blood"}],` + bsDur + `}',6),
  (312,1,1,112,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[[],0],` + bsDur + `}',1),
  (320,1,1,120,'Literjon',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":500.0,"FillableType":"Water"}],` + bsDur + `}',0),
  (330,7001,1,130,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[[],{}],` + bsDur + `}',1);`

func bloodOps(t *testing.T) *Ops { return &Ops{S: testsave.PlayerWithSQL(t, bloodWorld)} }

func sackStats(t *testing.T, o *Ops, id int) string {
	t.Helper()
	return mustOne(t, o, `select stats from items where id=?`, id)["stats"].(string)
}

func fillRec(amount, kind string) string {
	return `"FFillableItemStats":[[],{"CurrentAmount":` + amount + `,"FillableType":"` + kind + `"}]`
}

func TestFillBloodSacksFillsEachKnownSackAndOnlyThose(t *testing.T) {
	o := bloodOps(t)
	before := map[int]string{}
	for _, id := range []int{303, 306, 308, 310, 311, 320, 330} {
		before[id] = sackStats(t, o, id)
	}
	otherTables := digest(t, o, map[string]bool{"items": true}, 0)
	itemRowsBefore := countWhere(t, o, `select count(*) n from items`)

	res, err := o.FillBloodSacks()
	if err != nil {
		t.Fatal(err)
	}
	m := res.(map[string]any)
	if m["ok"] != true || m["filled"].(int) != 7 || m["alreadyFull"].(int) != 3 || m["holdingOther"].(int) != 1 || m["unknown"].(int) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if by := m["byTemplate"].(map[string]int); len(by) != 2 || by["Bloodsack_Unique_Durable_06"] != 3 || by["Bloodsack_01"] != 4 {
		t.Fatalf("filled per template: %v", by)
	}
	if sk := m["skippedUnknown"].([]string); len(sk) != 1 || sk[0] != "Bloodsack_02 x1" {
		t.Fatalf("the unknown kind is reported: %v", sk)
	}

	// empty object, partly full, key absent, odd record, loadout: all filled; every other key of the stats is byte for byte what it was
	want := map[int]string{
		301: `{` + bsCust + `,` + fillRec("30000.0", "Blood") + `,` + bsDur + `}`,
		302: `{"FFillableItemStats":[[],{"CurrentAmount":30000.0,"FillableType":"Blood","OtherKey":7}],` + bsCust + `,` + bsDur + `}`,
		304: `{` + bsCust + `,` + bsDur + `,` + fillRec("3000.0", "Blood") + `}`,
		305: `{` + fillRec("3000.0", "Blood") + `,` + bsDur + `}`,
		307: `{` + fillRec("30000.0", "Blood") + `,` + bsDur + `}`,
		309: `{` + fillRec("3000.0", "Blood") + `,` + bsDur + `}`,
		312: `{` + fillRec("3000.0", "Blood") + `,` + bsDur + `}`,
	}
	for id, w := range want {
		if got := sackStats(t, o, id); got != w {
			t.Errorf("sack %d\n got %s\nwant %s", id, got, w)
		}
	}
	// full, above capacity, unknown template, holding water, a Literjon and a base chest's sack: not rewritten
	for id, s := range before {
		if got := sackStats(t, o, id); got != s {
			t.Errorf("item %d must not change: %s -> %s", id, s, got)
		}
	}
	if got := sackStats(t, o, 311); !strings.Contains(got, `"CurrentAmount":31000.0`) {
		t.Errorf("a value above capacity was lowered: %s", got)
	}
	// nothing else in the save changed
	if d := digest(t, o, map[string]bool{"items": true}, 0); d != otherTables {
		t.Error("a table other than items changed")
	}
	if n := countWhere(t, o, `select count(*) n from items`); n != itemRowsBefore {
		t.Errorf("item rows %d -> %d", itemRowsBefore, n)
	}
	// one edit, in plain words
	if p := o.S.Pending(); len(p) != 1 || p[0] != "Filled 7 blood sacks" {
		t.Fatalf("one edit with a plain description: %v", p)
	}
	if r := mustOne(t, o, `pragma integrity_check`); r["integrity_check"] != "ok" {
		t.Errorf("integrity check: %v", r)
	}
	if v := fkViolations(t, o); v != 0 {
		t.Errorf("%d dangling references", v)
	}
	// a second press finds nothing to do and is not an edit
	again, err := o.FillBloodSacks()
	if err != nil {
		t.Fatal(err)
	}
	if a := again.(map[string]any); a["filled"].(int) != 0 || a["alreadyFull"].(int) != 10 {
		t.Fatalf("second run: %v", a)
	}
	if p := o.S.Pending(); len(p) != 1 {
		t.Fatalf("a run that fills nothing must not add an edit: %v", p)
	}
}

func TestOneFilledSackIsDescribedInTheSingular(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, clearWorld+`
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (301,1,1,101,'Bloodsack_01',0,1790000000,'{}',1);`)}
	if _, err := o.FillBloodSacks(); err != nil {
		t.Fatal(err)
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "Filled 1 blood sack" {
		t.Fatalf("%v", p)
	}
	if got := sackStats(t, o, 301); got != `{`+fillRec("3000.0", "Blood")+`}` {
		t.Fatalf("a sack whose stats are an empty object: %s", got)
	}
}

func TestNothingToFillIsNotAnEdit(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, clearWorld)}
	res, err := o.FillBloodSacks()
	if err != nil {
		t.Fatal(err)
	}
	if m := res.(map[string]any); m["filled"].(int) != 0 || m["unknown"].(int) != 0 {
		t.Fatalf("%v", m)
	}
	if o.S.Dirty() || len(o.S.Pending()) != 0 {
		t.Fatal("no edit when there is nothing to fill")
	}
}

func TestTheBloodCountSaysHowManyAreNotFull(t *testing.T) {
	o := bloodOps(t)
	res, err := o.BloodSacks()
	if err != nil {
		t.Fatal(err)
	}
	// known kinds: 301 302 303 304 305 307 308 309 310 311 312 = 11; not full: 301 302 304 305 307 309 312 = 7
	if m := res.(map[string]any); m["total"].(int) != 11 || m["notFull"].(int) != 7 || m["unknown"].(int) != 1 || m["holdingOther"].(int) != 1 {
		t.Fatalf("%v", m)
	}
	if o.S.Dirty() {
		t.Fatal("counting is not an edit")
	}
	if _, err := o.FillBloodSacks(); err != nil {
		t.Fatal(err)
	}
	res, _ = o.BloodSacks()
	if m := res.(map[string]any); m["total"].(int) != 11 || m["notFull"].(int) != 0 {
		t.Fatalf("after the fill: %v", m)
	}
}

// The water refill and the blood fill each leave the other's containers alone.
func TestTheWaterRefillLeavesBloodSacksAloneAndTheBloodFillLeavesWaterAlone(t *testing.T) {
	o := bloodOps(t)
	sacks := `select id, stats from items where template_id like 'Bloodsack%' order by id`
	before, err := o.S.Query(sacks)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.RefillContainers(); err != nil {
		t.Fatal(err)
	}
	after, _ := o.S.Query(sacks)
	if len(before) != len(after) {
		t.Fatal("sack count changed")
	}
	for i := range before {
		if before[i]["stats"] != after[i]["stats"] {
			t.Errorf("the water refill changed sack %v: %v -> %v", before[i]["id"], before[i]["stats"], after[i]["stats"])
		}
	}
	if got := sackStats(t, o, 320); !strings.Contains(got, `"CurrentAmount":1000.0`) {
		t.Fatalf("the water refill fills the Literjon: %s", got)
	}
	// and the blood fill does not touch the Literjon that the refill just filled
	lit := sackStats(t, o, 320)
	if _, err := o.FillBloodSacks(); err != nil {
		t.Fatal(err)
	}
	if sackStats(t, o, 320) != lit {
		t.Fatal("the blood fill touched a Literjon")
	}
}

// Only the character's own inventories: a sack in somebody else's storage is not filled (the water refill's scope).
func TestOnlyTheCharactersOwnSacksAreFilled(t *testing.T) {
	o := bloodOps(t)
	if _, err := o.FillBloodSacks(); err != nil {
		t.Fatal(err)
	}
	if got := sackStats(t, o, 330); !strings.Contains(got, `"FFillableItemStats":[[],{}]`) {
		t.Fatalf("a sack in a base chest was filled: %s", got)
	}
}

// Queued while the game runs: the sacks are read inside the edit, so a queued fill is held when the game has changed the sacks since.
func TestAQueuedBloodFillIsHeldWhenTheGameChangedTheSacks(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, clearWorld+`
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (301,1,1,101,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[[],{}]}',1);`)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if _, err := o.FillBloodSacks(); err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values (302,1,1,102,'Bloodsack_01',0,1790000000,'{}',1)`)
	mustBeHeld(t, o.S)
}

// And it is written when the game has changed something that has nothing to do with the sacks.
func TestAQueuedBloodFillIsWrittenWhenTheGameChangedSomethingElse(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, clearWorld+`
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (301,1,1,101,'Bloodsack_01',0,1790000000,'{"FFillableItemStats":[[],{}]}',1);`)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if _, err := o.FillBloodSacks(); err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `update items_id_sequencer set next_id=next_id+1`)
	f := o.S.Flush()
	if !f.Saved || o.S.Held() != nil {
		t.Fatalf("the queued fill is written: %+v", f)
	}
	chk, err := save.Open(o.S.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select stats from items where id=301`)
	if err != nil || r == nil || r["stats"] != `{`+fillRec("3000.0", "Blood")+`}` {
		t.Fatalf("%v %v", r, err)
	}
}

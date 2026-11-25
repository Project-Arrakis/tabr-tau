package ops

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Recycle base for a base the game's tool picked up (#1207). The world is the recycle's own, with the base of totem 601 picked up the way the game's tool leaves it (the shape of the
// saves measured: probe8, a Hagga backup, and a Deep Desert backup of 942 pieces): every actor of the base in state 5, the totem's permission record and the ranks of the base's actors
// gone (the placeables keep their child records, without ranks), and one base_backups row (9, the character's) listing the eight actors. The map marker is not looked for.
// It is named as the Bases card names it: by its totem type (the totem has no permission record to carry a name).
const pickedUpIDs = `601,602,701,702,703,704,705,706`

const pickedUpSQL = `
update actors set state=5 where id in (` + pickedUpIDs + `);
delete from permission_actor_rank where permission_actor_id in (601,701);
delete from permission_actor where actor_id=601;
insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (9,1,'##Totem_Placeable',1);
insert into base_backup_linked_actors(id,actor_id) values (9,601),(9,602),(9,701),(9,702),(9,703),(9,704),(9,705),(9,706);`

func pickedUpOps(t *testing.T, extra string) *Ops { return recycleOps(t, pickedUpSQL+extra) }

// tableRows is every row of every table of the save as text, by table: the whole save, to say what a recycle changed and what it did not.
func dumpTableRows(t *testing.T, o *Ops) map[string]map[string]bool {
	t.Helper()
	tables, err := o.S.Query(`select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name`)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]bool{}
	for _, tb := range tables {
		name := tb["name"].(string)
		rows, err := o.S.Query(`select * from "` + name + `"`)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		set := map[string]bool{}
		for _, r := range rows {
			set[fmt.Sprint(r)] = true
		}
		out[name] = set
	}
	return out
}

// diffTableRows says, by table, how many rows are gone and how many are new between two dumps (a changed row is one of each).
func diffTableRows(before, after map[string]map[string]bool) map[string][2]int {
	out := map[string][2]int{}
	for name, b := range before {
		var gone, added int
		for r := range b {
			if !after[name][r] {
				gone++
			}
		}
		for r := range after[name] {
			if !b[r] {
				added++
			}
		}
		if gone+added > 0 {
			out[name] = [2]int{gone, added}
		}
	}
	return out
}

func TestAPickedUpBaseIsPlannedAndRecycledLikeAStandingOne(t *testing.T) {
	inEachLayout(t, recycleWorld+pickedUpSQL, func(t *testing.T, o *Ops) {
		before := everything(t, o)
		pv, err := o.PreviewBaseRecycle(601)
		if err != nil {
			t.Fatal(err)
		}
		p := pv.(map[string]any)
		if p["dryRun"] != true || p["pickedUp"] != true || p["backup"] != int64(9) || p["pieces"] != int64(4) || p["placeables"] != int64(6) || p["itemStacksMoved"] != int64(4) || p["itemsMoved"] != int64(352) || p["costPercent"] != int64(100) {
			t.Fatalf("preview: %v", p)
		}
		// the refund is the standing base's (the same plan): 100% of the table cost
		if got, want := refundOf(p), "DuraluminumRod 115/1, IronBar 10/1, ScrapMetal 150/1, Silicone 120/1, Stone 52/1, T4HarkSpiceSiloComponent1 20/1"; got != want {
			t.Fatalf("refund: %s, want %s", got, want)
		}
		if everything(t, o) != before || len(o.S.Ops()) != 0 {
			t.Fatal("the preview changed the save")
		}

		dumpBefore := dumpTableRows(t, o)
		r := recycle(t, o, 601)
		deleted := r["deleted"].(map[string]int64)
		for table, want := range map[string]int64{"actors": 8, "actor_fgl_entities": 7, "fgl_entities": 7, "inventories": 5, "actor_inventories": 5, "building_instances": 4, "buildings": 1, "placeables": 7,
			"totems": 1, "totem_fuel_update_times": 1, "landclaim_segments": 2, "permission_actor": 2, "permission_actor_rank": 0, "player_respawn_locations": 1,
			"base_backups": 1, "base_backup_linked_actors": 8} {
			if deleted[table] != want {
				t.Errorf("deleted from %s: %d, want %d (%v)", table, deleted[table], want, deleted)
			}
		}
		if _, has := deleted["player_markers"]; has || r["markerRemoved"] != false {
			t.Fatalf("a picked-up base has no marker of its own: %v", deleted)
		}
		for _, q := range []string{`actors where id in (` + pickedUpIDs + `)`, `base_backups where id=9`, `base_backup_linked_actors where id=9`, `building_instances where building_id=602`, `placeables where id between 601 and 706`,
			`totems where id=601`, `landclaim_segments where totem_id=601`, `permission_actor where actor_id in (601,701,702)`} {
			if n := rmCount(t, o, q); n != 0 {
				t.Fatalf("%d rows left in %s", n, q)
			}
		}
		// the stored items arrive in the Reserve unchanged, and the refund is there
		if rmCount(t, o, `items where inventory_id=12`) != 10 || rmCount(t, o, `items where id in (9001,9002,9003,9005) and inventory_id=12`) != 4 {
			t.Fatal("the stored stacks and the refund are in the Reserve")
		}
		// the answer and the undo note say what it was
		words := strings.Join(r["summary"].([]string), "\n")
		for _, w := range []string{"Deleted: the base backup 9 that listed this picked-up base", `Deleted: the base "Advanced Sub-Fief" with 4 building pieces, its totem and 6 placeables`, "Moved: 4 item stacks (352 items)", "No base backup was written"} {
			if !strings.Contains(words, w) {
				t.Fatalf("the answer does not say %q:\n%s", w, words)
			}
		}
		if n := o.S.Ops(); len(n) != 1 || !strings.HasPrefix(n[0].Desc, `Recycle base "Advanced Sub-Fief" (picked up, base backup 9): 4 pieces, 6 placeables, 4 item stacks`) {
			t.Fatalf("one edit with a description: %v", n)
		}
		// nothing else changed: the only rows gone are the base's and the backup's (and the moved stacks); the only new ones are the refund and the id counter
		changed := diffTableRows(dumpBefore, dumpTableRows(t, o))
		wantGone := map[string]int{"actors": 8, "actor_fgl_entities": 7, "fgl_entities": 7, "inventories": 5, "actor_inventories": 5, "building_instances": 4, "buildings": 1, "placeables": 7, "totems": 1,
			"totem_fuel_update_times": 1, "landclaim_segments": 2, "permission_actor": 2, "player_respawn_locations": 1, "base_backups": 1, "base_backup_linked_actors": 8, "items": 4}
		for table, c := range changed {
			switch table {
			case "items": // the four moved stacks are one row gone and one row new each; the six refund stacks are new
				if c != [2]int{4, 10} {
					t.Errorf("items: %v", c)
				}
			case "items_id_sequencer":
			default:
				if c[1] != 0 || c[0] != wantGone[table] {
					t.Errorf("%s: %v rows (gone, new), want %d gone", table, c, wantGone[table])
				}
			}
		}
		var tables []string
		for table := range wantGone {
			if _, ok := changed[table]; !ok {
				tables = append(tables, table)
			}
		}
		sort.Strings(tables)
		if len(tables) > 0 {
			t.Fatalf("tables that did not change: %v", tables)
		}
		// the base is gone for the next question
		_, err = o.PreviewBaseRecycle(601)
		refusedWith(t, err, apierr.ErrNotFound, "no base with the totem")
	})
}

// The Deep Desert gives back half, as for a standing base; a picked-up Deep Desert base is the case this was made for.
func TestAPickedUpDeepDesertBaseGivesBackHalf(t *testing.T) {
	o := pickedUpOps(t, `update actors set map='DeepDesert' where id in (`+pickedUpIDs+`);`)
	r := recycle(t, o, 601)
	if got, want := refundOf(r), "DuraluminumRod 58/1, IronBar 10/1, ScrapMetal 76/1, Silicone 60/1, Stone 27/1, T4HarkSpiceSiloComponent1 10/1"; got != want {
		t.Fatalf("Deep Desert refund: %s, want %s", got, want)
	}
	if r["costPercent"] != int64(50) || !strings.Contains(strings.Join(r["summary"].([]string), "\n"), "which is 50% of the table cost") {
		t.Fatalf("%v", r)
	}
	if rmCount(t, o, `base_backups where id=9`) != 0 {
		t.Fatal("the backup was not deleted")
	}
}

// The Reserve must be empty first, in the question and in the write, for a picked-up base as for a standing one.
func TestAPickedUpBaseIsRefusedWhileTheReserveHoldsAStack(t *testing.T) {
	o := pickedUpOps(t, reserveStacks)
	before := everything(t, o)
	_, err := o.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "The Reserve is not empty (it holds 3 stack(s)) and no recycle can happen until it is empty")
	_, err = o.RecycleBase(601, "any")
	refusedWith(t, err, apierr.ErrConflict, "The Reserve is not empty")
	if everything(t, o) != before || len(o.S.Ops()) != 0 || len(o.S.Pending()) != 0 {
		t.Fatal("a refusal must change nothing")
	}
	// and a Reserve with no room is a refusal, as before
	o2 := pickedUpOps(t, `update inventories set max_item_count=9 where id=12;`)
	_, err = o2.PreviewBaseRecycle(601)
	refusedWith(t, err, apierr.ErrConflict, "The Reserve of this save has room for 9 stacks and the base returns 10")
}

// What is refused, and nothing changed. Each case is one guard of the picked-up path.
func TestAPickedUpBaseIsRefusedWhenItIsNotPlainlyTheCharactersOwnBackup(t *testing.T) {
	for name, c := range map[string]struct{ sql, want string }{
		"the totem still has a permission record (a claimed base)": {`insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (601,'My base',3,3,0,0);`, "picked-up state, but"},
		"a base backup of another player":                          {`update base_backups set player_id=99 where id=9;`, "base backup belongs to another player"},
		"a rank of another player on an actor of the base":         {`insert into permission_actor_rank(permission_actor_id,player_id,rank) values (701,99,3);`, "also held by another player"},
		"the backup lists an actor the totem does not own":         {`insert into base_backup_linked_actors(id,actor_id) values (9,99);`, "not the same actors"},
		"the backup misses an actor the totem owns":                {`delete from base_backup_linked_actors where actor_id=706;`, "not the same actors"},
		"an actor is also in another base backup":                  {`insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (10,1,'x',1); insert into base_backup_linked_actors(id,actor_id) values (10,706);`, "part of a base backup"},
		"the totem is in two base backups":                         {`insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (10,1,'x',1); insert into base_backup_linked_actors(id,actor_id) values (10,601);`, "not listed in exactly one base backup"},
		"an actor is a vehicle":                                    {`insert into vehicles(id) values (705);`, "Vehicles are never recycled"},
		"an actor is in the normal state":                          {`update actors set state=0 where id=706;`, "1 actors of this base are not in the picked-up state"},
		"a building actor also holds pieces of another base":       {`insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values (602,9,'Sardaukar_Wall_01',5764607523034235001,1.0);`, "also holds 1 pieces of another base"},
		"a row of another table points at the base":                {`insert into game_events(actor_id,universe_time,map,partition_id,event_type,x,y,z) values (701,1,'HaggaBasin',0,3,0,0,0);`, "game_events.actor_id (1 rows)"},
		"a character is waiting to respawn at the base":            {`update player_state set pending_respawn_location_id=x'01' where id=1;`, "waiting to respawn at this base"},
		"a crafting queue is running":                              {`update fgl_entities set components=jsonb('{"FItemCraftingComponent":[0,{"RequestsQueue":[{"RequestId":4}],"State":"Crafting"}]}') where entity_id=5764607523034234885;`, "A crafting queue is running"},
		"a piece with no cost":                                     {`update building_instances set building_type='Unheard_Of_Wall' where building_id=602 and instance_id=2;`, "Unheard_Of_Wall is not known"},
		"the save has no Reserve":                                  {`delete from items where inventory_id=12; delete from inventories where id=12;`, "0 Reserve inventories"},
	} {
		t.Run(name, func(t *testing.T) {
			o := pickedUpOps(t, c.sql)
			before := everything(t, o)
			dump := dumpTableRows(t, o)
			_, err := o.PreviewBaseRecycle(601)
			refusedWith(t, err, apierr.ErrConflict, c.want)
			if !strings.Contains(err.Error(), "Nothing was changed") && !strings.Contains(err.Error(), "was not changed") {
				t.Fatalf("the refusal says nothing was changed: %v", err)
			}
			_, err = o.RecycleBase(601, "anything")
			refusedWith(t, err, apierr.ErrConflict, c.want)
			if everything(t, o) != before || len(diffTableRows(dump, dumpTableRows(t, o))) != 0 || len(o.S.Ops()) != 0 || len(o.S.Pending()) != 0 {
				t.Fatal("a refusal must change nothing")
			}
		})
	}
}

// A standing base that is also listed in a base backup (a base that was picked up and claimed again keeps the rows), and a base in state 5 that is not listed, are not known shapes.
func TestABaseThatIsNotAPickedUpBaseByShapeStaysRefused(t *testing.T) {
	for name, c := range map[string]struct{ sql, want string }{
		"standing and listed in a backup":          {`insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (10,1,'x',1); insert into base_backup_linked_actors(id,actor_id) values (10,601);`, "standing and is also listed in a base backup"},
		"picked up but not listed in any backup":   {`update actors set state=5 where id in (` + pickedUpIDs + `); delete from permission_actor_rank where permission_actor_id in (601,701); delete from permission_actor where actor_id=601;`, "picked-up state, but"},
		"picked up, listed, with the claim record": {`update actors set state=5 where id in (` + pickedUpIDs + `); insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (10,1,'x',1); insert into base_backup_linked_actors(id,actor_id) values (10,601);`, "picked-up state, but"},
	} {
		t.Run(name, func(t *testing.T) {
			o := recycleOps(t, c.sql)
			before := everything(t, o)
			_, err := o.PreviewBaseRecycle(601)
			refusedWith(t, err, apierr.ErrConflict, c.want)
			_, err = o.RecycleBase(601, "anything")
			refusedWith(t, err, apierr.ErrConflict, c.want)
			if everything(t, o) != before {
				t.Fatal("a refusal must change nothing")
			}
		})
	}
	// the totem of the fixture's own picked-up base (901, listed in backup 3 with its building actor) is a picked-up base now: the recycle takes it
	o := recycleOps(t, "")
	r := recycle(t, o, 901)
	if r["pickedUp"] != true || r["backup"] != int64(3) || rmCount(t, o, `base_backups`) != 0 || rmCount(t, o, `base_backup_linked_actors`) != 0 {
		t.Fatalf("%v", r)
	}
}

// A failed read back rolls the whole edit back, backup rows included.
func TestAPickedUpBaseWhoseReadBackDiffersFailsTheWholeEdit(t *testing.T) {
	for name, tamper := range map[string]string{
		"the backup row is back":    `insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (9,1,'x',1)`,
		"a linked row is back":      `insert into base_backup_linked_actors(id,actor_id) values (9,601)`,
		"a moved stack lost items":  `update items set stack_size=1 where id=9001`,
		"a refund stack is short":   `update items set stack_size=stack_size-1 where inventory_id=12 and template_id='Stone'`,
		"a row of the base is back": `insert into totems(id) values (601)`,
	} {
		t.Run(name, func(t *testing.T) {
			o := pickedUpOps(t, "")
			pv, err := o.PreviewBaseRecycle(601)
			if err != nil {
				t.Fatal(err)
			}
			before := everything(t, o)
			dump := dumpTableRows(t, o)
			recycleBeforeReadBack = func(m *save.Mut) {
				if _, err := m.Exec(tamper); err != nil {
					t.Errorf("tamper: %v", err)
				}
			}
			defer func() { recycleBeforeReadBack = nil }()
			_, err = o.RecycleBase(601, pv.(map[string]any)["fingerprint"].(string))
			refusedWith(t, err, apierr.ErrConflict, "The recycle was not written")
			if everything(t, o) != before || len(diffTableRows(dump, dumpTableRows(t, o))) != 0 || len(o.S.Ops()) != 0 || len(o.S.Pending()) != 0 {
				t.Fatal("a failed read back must roll the whole edit back")
			}
		})
	}
}

// The question's fingerprint carries the backup: a write after the base changed, or after the backup changed, is refused.
func TestAPickedUpRecycleIsRefusedWhenTheBaseChangedSinceTheQuestion(t *testing.T) {
	for name, change := range map[string]string{
		"a stack was taken out": `delete from items where id=9002`,
		"a piece was added":     `insert into building_instances(building_id,instance_id,building_type,owner_entity_id,health) values (602,9,'Sardaukar_Wall_01',5764607523034234881,1.0)`,
	} {
		t.Run(name, func(t *testing.T) {
			o := pickedUpOps(t, "")
			pv, err := o.PreviewBaseRecycle(601)
			if err != nil {
				t.Fatal(err)
			}
			from := pv.(map[string]any)["fingerprint"].(string)
			if standing := recycleOps(t, ""); true {
				sv, _ := standing.PreviewBaseRecycle(601)
				if sv.(map[string]any)["fingerprint"] == from {
					t.Fatal("a picked-up base and a standing one must not share a fingerprint")
				}
			}
			testsave.Exec(t, o.S, change)
			before := everything(t, o)
			_, err = o.RecycleBase(601, from)
			refusedWith(t, err, apierr.ErrConflict, "changed since the question")
			if everything(t, o) != before {
				t.Fatal("a refused write changed the save")
			}
		})
	}
}

// A Solido of a picked-up base. The conversion reads the building actor, the pieces the totem's entity owns and the placeables; nothing of it is the totem's place or the standing
// state, so a picked-up base can have one made by the same code, in the same edit. It was measured on a base that stood; it is not measured on the game's own Solido of a backup.
func TestASolidoCanBeMadeOfAPickedUpBaseInTheSameEdit(t *testing.T) {
	o := recycleOps(t, solidoWorld(blankDevice)+`
update actors set state=5 where id=707;`+pickedUpSQL+`insert into base_backup_linked_actors(id,actor_id) values (9,707);`)
	opt := RecycleOptions{Solido: true, RequireEmptyReserve: true}
	pv, err := o.PreviewBaseRecycleWith(601, opt)
	if err != nil {
		t.Fatal(err)
	}
	r, err := o.RecycleBaseWith(601, pv.(map[string]any)["fingerprint"].(string), opt)
	if err != nil {
		t.Fatal(err)
	}
	out := r.(map[string]any)
	sol, _ := out["solido"].(map[string]any)
	if sol == nil || out["pickedUp"] != true || rmCount(t, o, `building_blueprints`) != 1 || rmCount(t, o, `base_backups where id=9`) != 0 {
		t.Fatalf("%v", out)
	}
	if !strings.HasPrefix(o.S.Ops()[0].Desc, "Make a Solido of the base, then recycle it") {
		t.Fatalf("%q", o.S.Ops()[0].Desc)
	}
	// the question with and without the Solido are different questions
	o2 := recycleOps(t, solidoWorld(blankDevice)+`update actors set state=5 where id=707;`+pickedUpSQL+`insert into base_backup_linked_actors(id,actor_id) values (9,707);`)
	without, err := o2.PreviewBaseRecycle(601)
	if err != nil {
		t.Fatal(err)
	}
	_, err = o2.RecycleBaseWith(601, without.(map[string]any)["fingerprint"].(string), opt)
	refusedWith(t, err, apierr.ErrConflict, "The Solido option changed")
}

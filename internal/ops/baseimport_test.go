package ops

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Import of a base file (#475, phase 2). The source is the made-up pick-up of baseexport_test.go (exportWorld); the targets are real-schema saves with one character.

// players77 puts the base's player ids on a player the target does not have: the importer must give every one of them to the target's own character.
const players77 = `update placeables set last_placed_by_player_id=77; update building_instances set last_placed_by_player_id=77; update permission_actor set edited_by_player_id=77 where actor_id=701;
update permission_actor_rank set player_id=77 where permission_actor_id=701; update base_backups set player_id=77, last_edited_by_player_id=77 where id=1;`

// dumpSave is every row of every table, hashed: two saves with the same dump hold the same data, counters included.
func dumpSave(t *testing.T, s *save.Save) string {
	t.Helper()
	tabs, err := s.Query(`select name from sqlite_master where type='table' order by name`)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	for _, tb := range tabs {
		name := tb["name"].(string)
		rows, err := s.Query(`select * from "` + name + `"`)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%s %d\n", name, len(rows))
		for _, r := range rows {
			keys := make([]string, 0, len(r))
			for k := range r {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(h, "%s=%v;", k, r[k])
			}
			h.Write([]byte{'\n'})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func exportedFile(t *testing.T, extra ...string) []byte {
	t.Helper()
	return mustExport(t, exportOps(t, extra...), 1).Data
}

func target(t *testing.T, extra ...string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, strings.Join(extra, "\n"))}
}

func mustImport(t *testing.T, o *Ops, data []byte) map[string]any {
	t.Helper()
	r, err := o.ImportBaseBackup(data)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	return r.(map[string]any)
}

func one(t *testing.T, o *Ops, q string, args ...any) save.Row {
	t.Helper()
	r, err := o.S.One(q, args...)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return r
}

func cnt(t *testing.T, o *Ops, q string, args ...any) int64 {
	t.Helper()
	r := one(t, o, q, args...)
	if r == nil {
		t.Fatalf("%s: no row", q)
	}
	for _, v := range r {
		return v.(int64)
	}
	return -1
}

// editFile changes the tables of a base file and makes the file whole again (counts, rows, checksum), so that what is refused is refused for its content and not for a stale checksum.
func editFile(t *testing.T, data []byte, fn func(tb map[string][]map[string]any)) []byte {
	t.Helper()
	f, err := basefile.Check(data)
	if err != nil {
		t.Fatal(err)
	}
	var tb map[string][]map[string]any
	dec := json.NewDecoder(bytes.NewReader(f.Tables))
	dec.UseNumber()
	if err := dec.Decode(&tb); err != nil {
		t.Fatal(err)
	}
	fn(tb)
	raw, err := json.Marshal(tb)
	if err != nil {
		t.Fatal(err)
	}
	m := f.Manifest
	m.Counts = map[string]int{}
	m.Summary.Rows = 0
	for _, tt := range basefile.Tables {
		m.Counts[tt.Name] = len(tb[tt.Name])
		m.Summary.Rows += len(tb[tt.Name])
	}
	at, _ := time.Parse(time.RFC3339, f.ExportedAt)
	out, err := basefile.Assemble(f.Kind, at, f.Editor, m, raw, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func jnum(n int64) json.Number { return json.Number(strconv.FormatInt(n, 10)) }

func rowWhere(tb map[string][]map[string]any, table, col string, v int64) map[string]any {
	for _, r := range tb[table] {
		if r[col].(json.Number).String() == strconv.FormatInt(v, 10) {
			return r
		}
	}
	panic(fmt.Sprintf("no row of %s with %s=%d", table, col, v))
}

// refuses runs the preview and the import of a file that must be refused, and checks that the save is exactly as it was after each.
func refuses(t *testing.T, o *Ops, data []byte, want string) {
	t.Helper()
	before := dumpSave(t, o.S)
	for name, fn := range map[string]func([]byte) (any, error){"preview": o.PreviewBaseImport, "import": o.ImportBaseBackup} {
		r, err := fn(data)
		if err == nil {
			t.Fatalf("%s accepted the file: %v", name, r)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: error %q does not say %q", name, err, want)
		}
		if !errors.Is(err, apierr.ErrBadArgument) && !errors.Is(err, apierr.ErrNotFound) {
			t.Fatalf("%s: kind of %v", name, err)
		}
		if dumpSave(t, o.S) != before || o.S.Dirty() || len(o.S.Ops()) != 0 {
			t.Fatalf("%s: a refused file changed the save", name)
		}
	}
}

func TestImportIntoTheSameSaveAddsANewBackupAndLeavesTheOriginalAlone(t *testing.T) {
	o := exportOps(t, players77)
	data := mustExport(t, o, 1).Data
	origin := dumpOf(t, o, 601, 602, 701, 702, 801, 802)
	fk0 := len(mustQuery(t, o, `pragma foreign_key_check`)) // the made-up source names player 77, who has no actor: those rows were dangling before the import
	r := mustImport(t, o, data)
	if r["ok"] != true || r["dryRun"] != false || r["name"] != "Sub-Fief" || r["state"] != basefile.StateBackup {
		t.Fatalf("result: %v", r)
	}
	if !o.S.Dirty() || len(o.S.Ops()) != 1 || !strings.HasPrefix(o.S.Ops()[0].Desc, `Import base backup "Sub-Fief": 3 pieces, 3 placeables, 3 stored items`) {
		t.Fatalf("one edit expected: %+v", o.S.Ops())
	}
	// the original rows are byte for byte what they were
	if dumpOf(t, o, 601, 602, 701, 702, 801, 802) != origin {
		t.Fatal("the original base changed")
	}
	// a second backup, with new ids above everything that was there
	if n := cnt(t, o, `select count(*) from base_backups`); n != 2 {
		t.Fatalf("backups: %d", n)
	}
	nb := one(t, o, `select id, player_id, base_backup_name, last_edited_by_player_id from base_backups where id<>1`)
	if nb["id"].(int64) != 2 || nb["player_id"].(int64) != 1 || nb["last_edited_by_player_id"].(int64) != 1 || nb["base_backup_name"] != "##Totem_Small_Placeable" {
		t.Fatalf("new backup: %v", nb)
	}
	rows, _ := o.S.Query(`select a.id, a.class, a.state, a.serial, a.owner_account_id, a.dimension_index from base_backup_linked_actors l join actors a on a.id=l.actor_id where l.id=2 order by a.id`)
	if len(rows) != 4 {
		t.Fatalf("new actors: %v", rows)
	}
	for _, a := range rows {
		if a["id"].(int64) <= 802 || a["state"].(int64) != 5 || a["owner_account_id"] != nil {
			t.Fatalf("actor: %v", a)
		}
	}
	if rows[0]["class"] != "/Game/BP_Totem.BP_Totem_C" || rows[0]["serial"].(int64) != 3 {
		t.Fatalf("first new actor: %v", rows[0])
	}
	if n := cnt(t, o, `select count(*) from actors where state=5`); n != 8 {
		t.Fatalf("actors in state 5: %d", n)
	}
	// every player id became the target's character (controller 1)
	if n := cnt(t, o, `select count(*) from placeables where id in (select actor_id from base_backup_linked_actors where id=2) and last_placed_by_player_id=1`); n != 3 {
		t.Fatalf("placeables owned: %d", n)
	}
	if n := cnt(t, o, `select count(*) from building_instances where building_id in (select actor_id from base_backup_linked_actors where id=2) and last_placed_by_player_id=1`); n != 3 {
		t.Fatalf("pieces placed by: %d", n)
	}
	if n := cnt(t, o, `select count(*) from permission_actor where actor_id in (select actor_id from base_backup_linked_actors where id=2) and edited_by_player_id=1`); n != 1 {
		t.Fatalf("permission record: %d", n)
	}
	if n := cnt(t, o, `select count(*) from permission_actor_rank where permission_actor_id in (select actor_id from base_backup_linked_actors where id=2) and player_id=1 and rank=2`); n != 1 {
		t.Fatalf("rank rows: %d", n)
	}
	if n := cnt(t, o, `select count(*) from permission_actor_rank where player_id=77`); n != 1 { // the original's own row, untouched
		t.Fatalf("player 77 rows: %d", n)
	}
	if n := cnt(t, o, `select count(*) from buildings where id in (select actor_id from base_backup_linked_actors where id=2) and owner_id is null`); n != 1 {
		t.Fatalf("buildings: %d", n)
	}
	// inventories and items: new ids, same content
	if n := cnt(t, o, `select count(*) from inventories where actor_id in (select actor_id from base_backup_linked_actors where id=2)`); n != 2 {
		t.Fatalf("inventories: %d", n)
	}
	it := one(t, o, `select i.id, i.stack_size, i.quality_level, i.volume_override, i.stats, i.acquisition_time from items i where i.template_id='IronOre' and i.id<>9002`)
	if it["id"].(int64) <= 9801 || it["stack_size"].(int64) != 5 || it["quality_level"].(int64) != 2 || it["volume_override"].(float64) != 0.25 || it["acquisition_time"].(int64) != 1790000001 ||
		!strings.Contains(it["stats"].(string), "10.5") {
		t.Fatalf("copied item: %v", it)
	}
	// the item sequencer moved past the new ids; nothing in the save is over it
	seq := cnt(t, o, `select next_id from items_id_sequencer`)
	if mx := cnt(t, o, `select max(id) from items`); seq <= mx {
		t.Fatalf("item counter %d is not above the largest item %d", seq, mx)
	}
	// no row of any kind hangs off a missing row: the edit leaves nothing dangling that was not there before
	if fk := len(mustQuery(t, o, `pragma foreign_key_check`)); fk != fk0 {
		t.Errorf("dangling rows: %d before the import, %d after", fk0, fk)
	}
}

func dumpOf(t *testing.T, o *Ops, actors ...int64) string {
	t.Helper()
	var ids []string
	for _, a := range actors {
		ids = append(ids, strconv.FormatInt(a, 10))
	}
	in := strings.Join(ids, ",")
	h := sha256.New()
	for _, q := range []string{
		`select * from actors where id in (` + in + `) order by id`,
		`select * from actor_fgl_entities where actor_id in (` + in + `) order by actor_id, entity_id`,
		`select * from fgl_entities where entity_id in (select entity_id from actor_fgl_entities where actor_id in (` + in + `)) order by entity_id`,
		`select * from inventories where actor_id in (` + in + `) order by id`,
		`select * from items where inventory_id in (select id from inventories where actor_id in (` + in + `)) order by id`,
		`select * from building_instances where building_id in (` + in + `) order by building_id, instance_id`,
		`select * from placeables where id in (` + in + `) order by id`,
		`select * from permission_actor where actor_id in (` + in + `) order by actor_id`,
		`select * from permission_actor_rank where permission_actor_id in (` + in + `) order by permission_actor_id`,
		`select * from totems where id in (` + in + `) order by id`, `select * from base_backups where id=1`, `select * from base_backup_linked_actors where id=1 order by actor_id`,
	} {
		rows, err := o.S.Query(q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		fmt.Fprintf(h, "%v\n", rows)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestImportRewritesTheIDsInsideJSONAndTheEntitiesKeepTheirPrefix(t *testing.T) {
	o := exportOps(t, players77)
	mustImport(t, o, mustExport(t, o, 1).Data)
	totem := one(t, o, `select a.id, json(a.properties) p from actors a join base_backup_linked_actors l on l.actor_id=a.id where l.id=2 and a.class like '%Totem%'`)
	id := totem["id"].(int64)
	if want := fmt.Sprintf(`{"m_SelfUniqueID":"!!act#%d"}`, id); totem["p"] != want {
		t.Fatalf("the totem's own id: %v, want %s", totem["p"], want)
	}
	if id == 601 {
		t.Fatal("the totem kept its id")
	}
	// the station: its queued item is the new item, the character reference is the null reference, the null reference stays
	station := one(t, o, `select json(e.components) c, e.entity_id e from fgl_entities e join actor_fgl_entities ae on ae.entity_id=e.entity_id join base_backup_linked_actors l on l.actor_id=ae.actor_id where l.id=2 and ae.slot_name='ItemCraftingComponent'`)
	itm := one(t, o, `select i.id from items i join inventories v on v.id=i.inventory_id join base_backup_linked_actors l on l.actor_id=v.actor_id where l.id=2 and i.template_id='Scrap'`)["id"].(int64)
	want := fmt.Sprintf(`{"ItemCraftingComponent":[[],{"Queue":[{"ItemUniqueId":"!!itm#%d","InstigatorActorId":"!!act#0","Spare":"!!act#0"}]}]}`, itm)
	if station["c"] != want {
		t.Fatalf("the station: %v\nwant %s", station["c"], want)
	}
	if itm == 9003 {
		t.Fatal("the item kept its id")
	}
	// entity ids: the 32-bit prefix of the file is kept, the counter is above every counter the save had, and no entity of the original base is touched
	ents, _ := o.S.Query(`select ae.entity_id e from actor_fgl_entities ae join base_backup_linked_actors l on l.actor_id=ae.actor_id where l.id=2 order by ae.entity_id`)
	if len(ents) != 3 {
		t.Fatalf("entities: %v", ents)
	}
	top := uint64(eOther) & 0xFFFFFFFF
	for _, e := range ents {
		u := uint64(e["e"].(int64))
		if u>>32 != uint64(eTotem)>>32 || u&0xFFFFFFFF <= top {
			t.Fatalf("entity %d: prefix %x counter %d (the save's highest was %d)", e["e"], u>>32, u&0xFFFFFFFF, top)
		}
	}
	// every piece and placeable of the new base names an entity of the new base
	if n := cnt(t, o, `select count(*) from placeables where id in (select actor_id from base_backup_linked_actors where id=2) and owner_entity_id in (select ae.entity_id from actor_fgl_entities ae join base_backup_linked_actors l on l.actor_id=ae.actor_id where l.id=2)`); n != 3 {
		t.Fatalf("placeables with an owner entity of the new base: %d", n)
	}
	if n := cnt(t, o, `select count(*) from building_instances where building_id in (select actor_id from base_backup_linked_actors where id=2) and owner_entity_id in (select ae.entity_id from actor_fgl_entities ae join base_backup_linked_actors l on l.actor_id=ae.actor_id where l.id=2)`); n != 3 {
		t.Fatalf("pieces with an owner entity of the new base: %d", n)
	}
}

func TestPreviewIsTheImportRolledBack(t *testing.T) {
	o := exportOps(t, players77)
	data := mustExport(t, o, 1).Data
	before := dumpSave(t, o.S)
	pv, err := o.PreviewBaseImport(data)
	if err != nil {
		t.Fatal(err)
	}
	if dumpSave(t, o.S) != before || o.S.Dirty() || len(o.S.Ops()) != 0 || len(o.S.Backups()) != 0 {
		t.Fatal("a preview changed the save")
	}
	p := pv.(map[string]any)
	if p["dryRun"] != true {
		t.Fatalf("preview: %v", p)
	}
	// previewing again, and importing, give the same numbers: the preview ran the code of the import on the same rows
	again, _ := o.PreviewBaseImport(data)
	im := mustImport(t, o, data)
	for _, k := range []string{"counts", "ids", "summary", "owner", "references", "warnings", "fileHash", "name"} {
		a, _ := json.Marshal(p[k])
		b, _ := json.Marshal(again.(map[string]any)[k])
		c, _ := json.Marshal(im[k])
		if string(a) != string(b) || string(a) != string(c) {
			t.Errorf("%s differs: %s | %s | %s", k, a, b, c)
		}
	}
	sum := p["summary"].(basefile.Summary)
	if sum != (basefile.Summary{Actors: 4, Pieces: 3, Placeables: 3, StoredItems: 3, Entities: 3, Rows: 34}) {
		t.Errorf("summary: %+v", sum)
	}
	ids := p["ids"].(map[string]any)
	if a := ids["actors"].(map[string]any); a["first"] != int64(803) || a["last"] != int64(806) || a["count"] != 4 {
		t.Errorf("actor ids: %v", a)
	}
	if e := ids["entities"].(map[string]any); e["count"] != 3 || e["first"].(string) == strconv.FormatInt(eTotem, 10) {
		t.Errorf("entity ids: %v", e)
	}
	if ow := p["owner"].(map[string]any); ow["controller"] != int64(1) || ow["character"] != "Tester" {
		t.Errorf("owner: %v", ow)
	}
	if !strings.Contains(strings.Join(p["warnings"].([]string), "|"), "1 references to an actor that is not in the file") {
		t.Errorf("warnings: %v", p["warnings"])
	}
	if p["fileHash"] != basefile.Hash(data) {
		t.Error("the hash is not the file's")
	}
}

func TestImportIntoASaveWithNoBaseAtAll(t *testing.T) {
	src := exportOps(t, players77)
	data := mustExport(t, src, 1).Data
	o := target(t)
	if n := cnt(t, o, `select count(*) from base_backups`); n != 0 {
		t.Fatal("the target should be empty")
	}
	mustImport(t, o, data)
	list, err := o.BaseBackups()
	if err != nil {
		t.Fatal(err)
	}
	rows := list.([]save.Row)
	if len(rows) != 1 || rows[0]["name"] != "Sub-Fief" || rows[0]["actors"].(int64) != 4 || rows[0]["picked_up"].(int64) != 4 || rows[0]["pieces"].(int64) != 3 ||
		rows[0]["placeables"].(int64) != 3 || rows[0]["stored_items"].(int64) != 3 || rows[0]["map"] != "HaggaBasin" {
		t.Fatalf("the Bases tab lists: %v", rows)
	}
	// the new ids start after the target's own (its three actors, its backpack and two items), and the base is not placed: the Bases list of standing bases does not show it
	if r := one(t, o, `select min(a.id) lo from actors a join base_backup_linked_actors l on l.actor_id=a.id`); r["lo"].(int64) != 4 {
		t.Fatalf("first new actor: %v", r)
	}
	if r := one(t, o, `select min(i.id) lo from items i join inventories v on v.id=i.inventory_id where v.actor_id>3`); r["lo"].(int64) != 500 {
		t.Fatalf("first new item should come from the sequencer (500): %v", r)
	}
	if seq := cnt(t, o, `select next_id from items_id_sequencer`); seq != 503 {
		t.Fatalf("sequencer: %d", seq)
	}
	// and the export of what was imported works (the round trip is tested below)
	if _, err := o.ExportBaseBackup(1, exportAt, "x"); err != nil {
		t.Fatalf("export of the import: %v", err)
	}
}

func TestImportIntoASaveWhoseCountersAreHigh(t *testing.T) {
	high := `insert into actors(id,class,map,state) values (4000000,'/Game/BP_Building.BP_Building_C','HaggaBasin',0), (4000001,'/Game/BP_Chest.BP_Chest_C','HaggaBasin',0);
insert into inventories(id,actor_id,inventory_type) values (3000000,4000001,4);
insert into items(id,inventory_id,stack_size,position_index,template_id,stats) values (39900668,3000000,1,0,'Scrap','{}');
update items_id_sequencer set next_id=39987823;
insert into fgl_entities(entity_id,components) values (5764607523034234880+4294967040,jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (4000001,5764607523034234880+4294967040,'Actor');
insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (50,1,'x',1);`
	o := target(t, high)
	data := exportedFile(t)
	mustImport(t, o, data)
	if lo := cnt(t, o, `select min(a.id) from actors a join base_backup_linked_actors l on l.actor_id=a.id and l.id=51`); lo != 4000002 {
		t.Fatalf("first new actor: %d", lo)
	}
	if b := cnt(t, o, `select max(id) from base_backups`); b != 51 {
		t.Fatalf("backup id: %d", b)
	}
	if lo := cnt(t, o, `select min(i.id) from items i where i.template_id in ('CopperOre','IronOre')`); lo != 39987823 {
		t.Fatalf("first new item should be the sequencer's: %d", lo)
	}
	if seq := cnt(t, o, `select next_id from items_id_sequencer`); seq != 39987826 {
		t.Fatalf("sequencer: %d", seq)
	}
	for _, e := range mustQuery(t, o, `select ae.entity_id e from actor_fgl_entities ae where ae.actor_id>4000001`) {
		if low := uint64(e["e"].(int64)) & 0xFFFFFFFF; low < 4294967040+10 || low > 0xFFFFFFFF {
			t.Fatalf("entity counter %d is not above the save's", low)
		}
	}
	// the counters the game keeps for AUTOINCREMENT tables moved with the inserts, so the game's next row does not take an id of the import
	if s := cnt(t, o, `select seq from sqlite_sequence where name='actors'`); s < 4000005 {
		t.Fatalf("actors sequence %d", s)
	}
}

func mustQuery(t *testing.T, o *Ops, q string, args ...any) []save.Row {
	t.Helper()
	r, err := o.S.Query(q, args...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestImportRefusesWhenThereIsNoRoomForEntityIDs(t *testing.T) {
	full := `insert into actors(id,class,map,state) values (4000001,'/Game/BP_Chest.BP_Chest_C','HaggaBasin',0);
insert into fgl_entities(entity_id,components) values (5764607523034234880+4294967290,jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (4000001,5764607523034234880+4294967290,'Actor');`
	o := target(t, full)
	refuses(t, o, exportedFile(t), "entity ids")
}

func TestTheSameFileImportedTwiceIsTwoBackups(t *testing.T) {
	o := target(t)
	data := exportedFile(t)
	mustImport(t, o, data)
	mustImport(t, o, data)
	if n := cnt(t, o, `select count(*) from base_backups`); n != 2 {
		t.Fatalf("backups: %d", n)
	}
	if n := cnt(t, o, `select count(distinct entity_id) from actor_fgl_entities`); n != 6 {
		t.Fatalf("entities: %d", n)
	}
	// the two bases share no id
	for _, q := range []string{`select count(*) from (select actor_id from base_backup_linked_actors group by actor_id having count(*)>1)`,
		`select count(*) from (select id from items group by id having count(*)>1)`} {
		if n := cnt(t, o, q); n != 0 {
			t.Fatalf("%s: %d", q, n)
		}
	}
	if len(o.S.Ops()) != 2 {
		t.Fatalf("two edits: %v", o.S.Ops())
	}
}

// canon turns the tables of a file into text that does not depend on the ids: ids are replaced by their place among the ids of their kind, references inside JSON likewise, the
// players by one player, and what the import changes on purpose (an actor of another save in a queue) by the null reference.
func canon(t *testing.T, data []byte) string {
	t.Helper()
	tb := tablesOf(t, data)
	rank := func(table, col string) map[string]int {
		var ids []int64
		for _, r := range tb[table] {
			n, _ := strconv.ParseInt(string(r[col].(json.Number)), 10, 64)
			ids = append(ids, n)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		m := map[string]int{}
		for i, n := range ids {
			m[strconv.FormatInt(n, 10)] = i + 1
		}
		return m
	}
	kinds := map[string]map[string]int{"act": rank("actors", "id"), "itm": rank("items", "id"), "inv": rank("inventories", "id"), "fgl": rank("fgl_entities", "entity_id")}
	idCols := map[string]string{ // table.column -> kind
		"base_backup_linked_actors.actor_id": "act", "actors.id": "act", "actor_fgl_entities.actor_id": "act", "actor_fgl_entities.entity_id": "fgl", "fgl_entities.entity_id": "fgl",
		"permission_actor.actor_id": "act", "permission_actor_rank.permission_actor_id": "act", "inventories.id": "inv", "inventories.actor_id": "act", "items.id": "itm", "items.inventory_id": "inv",
		"actor_inventories.inventory_id": "inv", "buildings.id": "act", "building_instances.building_id": "act", "building_instances.owner_entity_id": "fgl", "placeables.id": "act",
		"placeables.owner_entity_id": "fgl", "totems.id": "act", "landclaim_segments.totem_id": "act",
	}
	players := map[string]bool{"base_backups.player_id": true, "base_backups.last_edited_by_player_id": true, "permission_actor.edited_by_player_id": true, "permission_actor_rank.player_id": true,
		"placeables.last_placed_by_player_id": true, "building_instances.last_placed_by_player_id": true}
	var out strings.Builder
	for _, tt := range basefile.Tables {
		out.WriteString("== " + tt.Name + "\n")
		var lines []string
		for _, r := range tb[tt.Name] {
			var parts []string
			for _, c := range tt.Cols {
				v := r[c.Name]
				key := tt.Name + "." + c.Name
				s := fmt.Sprint(v)
				switch {
				case tt.Name == "base_backups" && c.Name == "id", tt.Name == "base_backup_linked_actors" && c.Name == "id":
					s = "B"
				case idCols[key] != "" && v != nil:
					s = fmt.Sprint(kinds[idCols[key]][string(v.(json.Number))])
				case players[key]:
					if n, ok := v.(json.Number); ok && string(n) != "0" {
						s = "P"
					}
				case tt.Name == "buildings" && c.Name == "owner_id":
					s = "none"
				case c.Kind == basefile.JSON && v != nil:
					b, _ := json.Marshal(v)
					rw, err := basefile.RewriteRefs(string(b), func(ref basefile.Ref) (string, error) {
						if ref.ID == 0 {
							return "!!" + ref.Kind + "#0", nil
						}
						if n, ok := kinds[ref.Kind][strconv.FormatInt(ref.ID, 10)]; ok {
							return fmt.Sprintf("!!%s#r%d", ref.Kind, n), nil
						}
						return "!!" + ref.Kind + "#0", nil // not in the file: the import sets it to none
					})
					if err != nil {
						t.Fatal(err)
					}
					s = rw
				}
				parts = append(parts, c.Name+"="+s)
			}
			lines = append(lines, strings.Join(parts, " "))
		}
		sort.Strings(lines)
		out.WriteString(strings.Join(lines, "\n") + "\n")
	}
	return out.String()
}

func TestRoundTripExportImportExportGivesTheSameTablesModuloIDs(t *testing.T) {
	src := exportOps(t, players77)
	first := mustExport(t, src, 1).Data
	for name, o := range map[string]*Ops{"an empty save": target(t), "the same save": src} {
		t.Run(name, func(t *testing.T) {
			mustImport(t, o, first)
			id := cnt(t, o, `select max(id) from base_backups`)
			second, err := o.ExportBaseBackup(id, exportAt, "test-1.2.3")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := basefile.Check(second.Data); err != nil {
				t.Fatal(err)
			}
			a, b := canon(t, first), canon(t, second.Data)
			if a != b {
				t.Fatalf("the tables differ after a round trip:\n--- first\n%s\n--- second\n%s", a, b)
			}
			// the manifest of the second file is whole: no reference to anything outside the file any more
			f, _ := basefile.Check(second.Data)
			for _, r := range f.Manifest.References {
				if r.Outside != 0 {
					t.Errorf("reference left outside: %+v", r)
				}
			}
		})
	}
}

func TestEveryRefusalLeavesTheSaveAsItWas(t *testing.T) {
	good := exportedFile(t)
	cases := []struct {
		name string
		data func() []byte
		want string
	}{
		{"garbage", func() []byte { return []byte("hello") }, "not a base file"},
		{"empty", func() []byte { return nil }, "not a base file"},
		{"another format", func() []byte { return bytes.Replace(good, []byte(`"tabr-tau-base"`), []byte(`"tabr-tau-kits"`), 1) }, "not a base file"},
		{"another version", func() []byte { return bytes.Replace(good, []byte(`"version": 1,`), []byte(`"version": 2,`), 1) }, "version 2"},
		{"another kind", func() []byte {
			return bytes.Replace(good, []byte(`"kind": "base-backup"`), []byte(`"kind": "solido"`), 1)
		}, "kind"},
		{"a wrong checksum", func() []byte { return bytes.Replace(good, []byte(`"value": "`), []byte(`"value": "0`), 1) }, "checksum"},
		{"a cut off file", func() []byte { return good[:len(good)/2] }, "not a base file"},
		{"a table that is not on the list", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { tb["accounts"] = []map[string]any{} })
		}, "table (accounts)"},
		{"a column that is not on the list", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				for _, r := range tb["actors"] {
					r["owner_account_id"] = jnum(1)
				}
			})
		}, "columns of the table"},
		{"an item in a bag of nothing", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "items", "id", 9001)["inventory_id"] = jnum(99999) })
		}, "inventory 99999 that is not in the file"},
		{"an owner entity outside", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "placeables", "id", 701)["owner_entity_id"] = jnum(5764607523034239999)
			})
		}, "entity 5764607523034239999 that is not in the file"},
		{"a linked actor that is not there", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				tb["base_backup_linked_actors"] = append(tb["base_backup_linked_actors"], map[string]any{"id": jnum(1), "actor_id": jnum(9999)})
			})
		}, "actor 9999 that is not in the file"},
		{"an actor that is not linked", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				tb["base_backup_linked_actors"] = tb["base_backup_linked_actors"][1:]
			})
		}, "not linked to its base backup"},
		{"an entity linked to no actor", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				tb["fgl_entities"] = append(tb["fgl_entities"], map[string]any{"entity_id": jnum(5764607523034239998), "components": map[string]any{}})
			})
		}, "linked to no actor"},
		{"an entity linked to an entity that is not in the file", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "actor_fgl_entities", "entity_id", eChest)["entity_id"] = jnum(5764607523034239997)
			})
		}, "entity 5764607523034239997 that is not in the file"},
		{"a rank on a record that is not there", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "permission_actor_rank", "permission_actor_id", 701)["permission_actor_id"] = jnum(5555)
			})
		}, "permission record 5555"},
		{"a claim segment of no totem", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { tb["landclaim_segments"][0]["totem_id"] = jnum(8888) })
		}, "totem 8888"},
		{"a piece of no building", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { tb["building_instances"][0]["building_id"] = jnum(701) })
		}, "building 701"},
		{"the same actor twice", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { tb["actors"][1]["id"] = tb["actors"][0]["id"] })
		}, "is used by two rows"},
		{"the same entity twice", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				tb["fgl_entities"][1]["entity_id"] = tb["fgl_entities"][0]["entity_id"]
			})
		}, "is used by two rows"},
		{"the same item twice", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { tb["items"][1]["id"] = tb["items"][0]["id"] })
		}, "is used by two rows"},
		{"the same piece twice", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				tb["building_instances"][1]["instance_id"] = tb["building_instances"][0]["instance_id"]
			})
		}, "repeats a row"},
		{"the same entity linked to two actors", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				tb["actor_fgl_entities"][1]["entity_id"] = tb["actor_fgl_entities"][0]["entity_id"]
			})
		}, "linked to two actors"},
		{"an actor id that is not positive", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "actors", "id", 602)["id"] = jnum(-5) })
		}, "must be a positive number"},
		{"two backups", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				tb["base_backups"] = append(tb["base_backups"], map[string]any{"id": jnum(2), "player_id": jnum(1), "base_backup_name": "x", "last_edited_by_player_id": jnum(1)})
			})
		}, "one base backup"},
		{"a player's actor", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "actors", "id", 702)["class"] = "/Game/Dune/Characters/Player/BP_DunePlayerCharacter.BP_DunePlayerCharacter_C"
			})
		}, "a player's actor"},
		{"a class that is not a name", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "actors", "id", 702)["class"] = "x; drop table actors"
			})
		}, "not one of the game's names"},
		{"an empty class", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "actors", "id", 702)["class"] = "" })
		}, "not one of the game's names"},
		{"a vehicle's state", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "actors", "id", 702)["state"] = jnum(4) })
		}, "state 4"},
		{"an unknown state", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "actors", "id", 702)["state"] = jnum(77) })
		}, "state 77"},
		{"a map that is not a name", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "actors", "id", 702)["map"] = "../../x" })
		}, "map is not one of the game's names"},
		{"a missing required value", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "actors", "id", 702)["location_x"] = nil })
		}, "has no location_x"},
		{"a Solido in a storage", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "items", "id", 9001)["template_id"] = "BuildingBlueprint_CopyDevice"
			})
		}, "Solido"},
		{"an item template that is not a name", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "items", "id", 9001)["template_id"] = "a b" })
		}, "template_id is not one of the game's names"},
		{"a stack of nothing", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) { rowWhere(tb, "items", "id", 9001)["stack_size"] = jnum(0) })
		}, "stack size"},
		{"a control character in a name", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "permission_actor", "actor_id", 701)["actor_name"] = "a\nb"
			})
		}, "control character"},
		{"a NUL in JSON", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "actors", "id", 702)["properties"] = map[string]any{"a": "x\x00y"}
			})
		}, "NUL"},
		{"a name that is too long", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "permission_actor", "actor_id", 701)["actor_name"] = strings.Repeat("a", 1001)
			})
		}, "longer than 1000"},
		{"an item reference outside the file", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "fgl_entities", "entity_id", eStation)["components"] = map[string]any{"Q": "!!itm#123456"}
			})
		}, "names !!itm#123456, which is not in the file"},
		{"a kind of reference that is not moved", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "fgl_entities", "entity_id", eStation)["components"] = map[string]any{"Q": "!!bbp#5"}
			})
		}, "does not move"},
		{"an id inside other text", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "actors", "id", 602)["properties"] = map[string]any{"Q": "see !!act#601 here"}
			})
		}, "inside other text"},
		{"a negative reference", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "actors", "id", 602)["properties"] = map[string]any{"Q": "!!act#-4"}
			})
		}, "not an id"},
		{"a reference that does not fit", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				rowWhere(tb, "actors", "id", 602)["properties"] = map[string]any{"Q": "!!act#99999999999999999999"}
			})
		}, "does not fit"},
		{"JSON nested too deeply", func() []byte {
			return editFile(t, good, func(tb map[string][]map[string]any) {
				var v any = "x"
				for i := 0; i < 120; i++ {
					v = []any{v}
				}
				rowWhere(tb, "actors", "id", 602)["properties"] = v
			})
		}, "nested more than"},
		{"invalid UTF-8", func() []byte { return bytes.Replace(good, []byte(`"Ore Out"`), []byte("\"Ore \xff\""), 1) }, "UTF-8"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			refuses(t, target(t), c.data(), c.want)
		})
	}
}

func TestCapsRefuseALargeFileAndTooManyRows(t *testing.T) {
	good := exportedFile(t)
	oldB, oldR := importByteCap, importRowCap
	t.Cleanup(func() { importByteCap, importRowCap = oldB, oldR })
	importByteCap = 3000
	refuses(t, target(t), good, "larger than 3000 bytes")
	importByteCap = oldB
	importRowCap = 20
	refuses(t, target(t), good, "more than 20 rows")
	importRowCap = oldR
	mustImport(t, target(t), good) // the same file is fine under the real caps
	if importRowCap != 250000 || importByteCap != 16<<20 {
		t.Fatalf("limits: %d rows, %d bytes", importRowCap, importByteCap)
	}
}

func TestImportRefusesASaveWithNoCharacter(t *testing.T) {
	o := &Ops{S: testsave.Empty(t)}
	refuses(t, o, exportedFile(t), "no character")
	// a character whose controller is not an actor of the save
	o2 := target(t, `delete from player_state; insert into player_state(id,account_id,character_name,player_controller_id,player_pawn_id,player_state_id) values (900,1,'Ghost',900,2,3);`)
	refuses(t, o2, exportedFile(t), "not among its actors")
}

func TestPreviewWarnsAboutWhatTheEditorChanges(t *testing.T) {
	// a file whose actors were standing, from a save with other patches, with an item the catalog does not know and a base on another map
	src := exportOps(t, `update actors set state=0 where id=702;`, `update actors set map='DeepDesert_1';`,
		`insert into applied_patches(name,date) values ('P_ONE',1790000001);`, `update items set template_id='NoSuchThing' where id=9001;`)
	data := mustExport(t, src, 1).Data
	o := target(t, `insert into applied_patches(name,date) values ('P_ONE',1790000001),('P_TWO',1790000002);`)
	r, err := o.PreviewBaseImport(data)
	if err != nil {
		t.Fatal(err)
	}
	if r.(map[string]any)["patchesDiffer"] != true {
		t.Error("the preview does not say that the patches differ")
	}
	w := strings.Join(r.(map[string]any)["warnings"].([]string), "\n")
	for _, want := range []string{"1 of 4 actors in the file were not in the picked-up state", "1 references to an actor that is not in the file", "NoSuchThing",
		"map DeepDesert_1", "had 1 game patches applied; this save has 2"} {
		if !strings.Contains(w, want) {
			t.Errorf("no warning with %q in:\n%s", want, w)
		}
	}
	// the same patches: no warning about them
	o2 := target(t, `insert into applied_patches(name,date) values ('P_ONE',1790000001);`)
	r2, _ := o2.PreviewBaseImport(data)
	if r2.(map[string]any)["patchesDiffer"] != false || strings.Contains(strings.Join(r2.(map[string]any)["warnings"].([]string), "\n"), "game patches") {
		t.Error("a warning about patches for the same patches")
	}
	// after the import the state is 5 whatever the file said
	mustImport(t, o, data)
	if n := cnt(t, o, `select count(*) from base_backup_linked_actors l join actors a on a.id=l.actor_id where a.state<>5`); n != 0 {
		t.Fatalf("actors not in state 5: %d", n)
	}
}

func TestImportMergesRankRowsOfSeveralPlayers(t *testing.T) {
	src := exportOps(t, `insert into permission_actor_rank(permission_actor_id,player_id,rank) values (701,4,3),(701,5,1);`)
	r, err := target(t).PreviewBaseImport(mustExport(t, src, 1).Data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(r.(map[string]any)["warnings"].([]string), "\n"), "2 rank rows named other players") {
		t.Fatalf("warnings: %v", r.(map[string]any)["warnings"])
	}
	o := target(t)
	mustImport(t, o, mustExport(t, src, 1).Data)
	rows := mustQuery(t, o, `select player_id, rank from permission_actor_rank`)
	if len(rows) != 1 || rows[0]["player_id"].(int64) != 1 || rows[0]["rank"].(int64) != 1 {
		t.Fatalf("ranks: %v", rows)
	}
}

func TestImportQueuedForTheJournalAndFailsWholeIfTheSaveCannotTakeIt(t *testing.T) {
	// a target whose items table already holds a row that the file's new ids would meet cannot happen (ids come from the counters); a table the import writes to that is missing makes
	// the whole edit fail and leaves the save as it was
	o := target(t)
	if _, err := o.S.Mutate("break", func(m *save.Mut) error { _, err := m.Exec(`drop table landclaim_segments`); return err }); err != nil {
		t.Skipf("the save does not allow dropping a table here: %v", err)
	}
	before := dumpSave(t, o.S)
	ops := len(o.S.Ops())
	if _, err := o.ImportBaseBackup(exportedFile(t)); err == nil {
		t.Fatal("imported into a save without a table")
	}
	if dumpSave(t, o.S) != before || len(o.S.Ops()) != ops {
		t.Fatal("a failed import changed the save")
	}
}

// Against a real base file and a COPY of a save. TAU_REAL_BASE_FILE names a base file made by the export, TAU_REAL_BASE_SAVE_SQLITE a plain SQLite copy of a save the game wrote.
// The copy is copied again into a temporary folder and only that second copy is written; without the two variables the test is skipped. Nothing from either file is committed.
func TestImportOfARealBaseFileIntoACopyOfARealSave(t *testing.T) {
	file, src := os.Getenv("TAU_REAL_BASE_FILE"), os.Getenv("TAU_REAL_BASE_SAVE_SQLITE")
	if file == "" || src == "" {
		t.Skip("TAU_REAL_BASE_FILE and TAU_REAL_BASE_SAVE_SQLITE are not both set: no real base file and save copy to import")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := save.Encode(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "game.db")
	if err := os.WriteFile(p, blob, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := save.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	o := &Ops{S: s}
	f, err := basefile.Check(data)
	if err != nil {
		t.Fatalf("the real file does not pass its own check: %v", err)
	}
	before := dumpSave(t, s)
	pv, err := o.PreviewBaseImport(data)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if dumpSave(t, s) != before {
		t.Fatal("the preview changed the save")
	}
	pm := pv.(map[string]any)
	t.Logf("preview: %v | ids %v | owner %v | warnings %v", pm["summary"], pm["ids"], pm["owner"], pm["warnings"])
	backups0 := cnt(t, o, `select count(*) from base_backups`)
	im := mustImport(t, o, data)
	if im["summary"].(basefile.Summary) != f.Manifest.Summary {
		t.Errorf("imported %+v, the file says %+v", im["summary"], f.Manifest.Summary)
	}
	if n := cnt(t, o, `select count(*) from base_backups`); n != backups0+1 {
		t.Fatalf("backups %d -> %d", backups0, n)
	}
	id := cnt(t, o, `select max(id) from base_backups`)
	// round trip: the export of what was imported equals the file, modulo ids
	second, err := o.ExportBaseBackup(id, exportAt, "x")
	if err != nil {
		t.Fatalf("export of the import: %v", err)
	}
	if a, b := canon(t, data), canon(t, second.Data); a != b {
		t.Fatalf("the real file and the export of its import differ (%d and %d bytes of canonical text)", len(a), len(b))
	}
	if n := cnt(t, o, `select count(*) from actors where state=5`); n < int64(f.Manifest.Summary.Actors) {
		t.Fatalf("state 5 actors: %d", n)
	}
	// a second import is a second backup, and the one before is as it was
	mustImport(t, o, data)
	if n := cnt(t, o, `select count(*) from base_backups`); n != backups0+2 {
		t.Fatalf("backups after the second import: %d", n)
	}
	// the save the editor would write passes the game's own checks
	if r := one(t, o, `pragma integrity_check`); r["integrity_check"] != "ok" {
		t.Fatalf("integrity: %v", r)
	}
	if fl := s.Flush(); !fl.Saved || fl.Warning != "" || fl.Stuck {
		t.Fatalf("writing the imported copy: %+v", fl)
	}
}

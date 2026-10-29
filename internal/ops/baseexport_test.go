package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Export of a picked-up base (#475, phase 1). The world is the shape of a real pick-up (see docs/research/2026-10-28-base-export-import-from-console.md section 3): a totem, a
// building actor that holds the pieces, two placeables with entities (a storage and a crafting station), the entities with 64-bit ids, and a second base that is still placed and
// must not be in the file.
const (
	eTotem   = int64(5764607523034234881) // above 2^53: the page must never read these as JavaScript numbers
	eChest   = int64(5764607523034234882)
	eStation = int64(5764607523034234891)
	eOther   = int64(5764607523034235001)
)

var exportWorld = `
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,state,serial,owner_account_id,properties) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7,0,0,0,1,5,3,1,jsonb('{"m_SelfUniqueID":"!!act#601"}')),
  (602,'/Game/BP_Building.BP_Building_C','HaggaBasin',5000,6000,7,0,0,0,1,5,4,1,jsonb('{}')),
  (701,'/Game/BP_Chest.BP_Chest_C','HaggaBasin',5010,6000,7,0,0,0.7071,0.7071,5,5,1,jsonb('{}')),
  (702,'/Game/BP_Refinery.BP_Refinery_C','HaggaBasin',5020,6000,7,0,0,0,1,5,6,1,jsonb('{}')),
  (801,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',90000,90000,0,0,0,0,1,0,7,1,jsonb('{}')),
  (802,'/Game/BP_Building.BP_Building_C','HaggaBasin',90000,90000,0,0,0,0,1,0,8,1,jsonb('{}'));
insert into fgl_entities(entity_id,components) values
  (5764607523034234881, jsonb('{"Actor":[[],{"x":1.5}]}')),
  (5764607523034234882, jsonb('{"ContainerInventory":[[],{}]}')),
  (5764607523034234891, jsonb('{"ItemCraftingComponent":[[],{"Queue":[{"ItemUniqueId":"!!itm#9003","InstigatorActorId":"!!act#2","Spare":"!!act#0"}]}]}')),
  (5764607523034235001, jsonb('{"Actor":[[],{}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values
  (601,5764607523034234881,'Actor'),(701,5764607523034234882,'ContainerInventory'),(702,5764607523034234891,'ItemCraftingComponent'),(801,5764607523034235001,'Actor');
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (701,'Ore Out',1,3,1,1),(801,'Other base',1,3,1,1);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (701,1,2),(801,1,1);
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (7001,701,4,20,1000.0),(7002,702,12,5,1000.0),(7801,801,4,20,1000.0);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level,volume_override) values
  (9001,7001,3,0,'CopperOre',0,1790000000,'{}',0,null),
  (9002,7001,5,1,'IronOre',1,1790000001,'{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":10.5}]}',2,0.25),
  (9003,7002,1,0,'Scrap',0,1790000002,'{}',0,null),
  (9801,7801,9,0,'CopperOre',0,1790000003,'{}',0,null);
insert into actor_inventories(inventory_id,component_name_hash) values (7001,111),(7002,222),(7801,333);
insert into buildings(id,owner_id) values (602,null),(802,null);
insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,owner_entity_id,building_flags,health,shelter,sand_buildup,last_placed_by_player_id) values
  (602,1,'Foundation',5000,6000,7,0,0,0,1,5764607523034234881,0,100.0,0,0,1),
  (602,2,'Wall',5010,6000,7,0,0,0.7071,0.7071,5764607523034234881,0,90.5,1,2,1),
  (602,3,'Roof',5020,6000,10,0,0,0,1,5764607523034234881,0,80.0,1,0,1),
  (802,1,'Foundation',90000,90000,0,0,0,0,1,5764607523034235001,0,100.0,0,0,1),
  (802,2,'Wall',90010,90000,0,0,0,0,1,5764607523034235001,0,100.0,0,0,1);
insert into placeables(id,owner_entity_id,health,building_type,has_hit_ground,has_buildable_support,is_hologram,last_placed_by_player_id) values
  (601,5764607523034234881,1000.0,'Totem_Small_Placeable',1,1,0,1),(701,5764607523034234881,50.0,'GenericContainer_Placeable',1,1,0,1),
  (702,5764607523034234881,60.0,'SmallOreRefinery_Placeable',1,1,0,1),(801,5764607523034235001,1000.0,'Totem_Placeable',1,1,0,1);
insert into totems(id,landclaim_vertical_level,last_backup_timestamp,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values
  (601,0,0,5000,6000,7,90),(801,0,0,90000,90000,0,0);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,0,0),(601,1,0),(801,0,0);
insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (1,1,'##Totem_Small_Placeable',1);
insert into base_backup_linked_actors(id,actor_id) values (1,601),(1,602),(1,701),(1,702);`

func exportOps(t *testing.T, extra ...string) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, exportWorld+strings.Join(extra, "\n"))}
}

var exportAt = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

func mustExport(t *testing.T, o *Ops, id int64) *BaseExport {
	t.Helper()
	out, err := o.ExportBaseBackup(id, exportAt, "test-1.2.3")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return out
}

// tablesOf reads the tables of a file with the numbers kept as text, so an id above 2^53 can be compared exactly.
func tablesOf(t *testing.T, data []byte) map[string][]map[string]any {
	t.Helper()
	var f struct {
		Tables map[string][]map[string]any `json:"tables"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&f); err != nil {
		t.Fatal(err)
	}
	return f.Tables
}

func TestBaseBackupsAreListedWithTheCountsOfTheFile(t *testing.T) {
	o := exportOps(t)
	r, err := o.BaseBackups()
	if err != nil {
		t.Fatal(err)
	}
	rows := r.([]save.Row)
	if len(rows) != 1 {
		t.Fatalf("one backup: %v", rows)
	}
	b := rows[0]
	if b["backup_id"].(int64) != 1 || b["name"] != "Sub-Fief" || b["backup_name"] != "##Totem_Small_Placeable" || b["map"] != "HaggaBasin" {
		t.Fatalf("backup: %v", b)
	}
	// the placed base (801, 802) is not a backup: only the 4 linked actors count
	if b["actors"].(int64) != 4 || b["picked_up"].(int64) != 4 || b["pieces"].(int64) != 3 || b["placeables"].(int64) != 3 || b["stored_items"].(int64) != 3 {
		t.Fatalf("counts: %v", b)
	}
	out := mustExport(t, o, 1)
	s := out.Summary
	if int64(s.Actors) != b["actors"] || int64(s.Pieces) != b["pieces"] || int64(s.Placeables) != b["placeables"] || int64(s.StoredItems) != b["stored_items"] {
		t.Fatalf("the list and the file disagree: %v against %+v", b, s)
	}
}

func TestExportOfABackupHoldsOnlyItsRowsWithTheOriginalIDs(t *testing.T) {
	o := exportOps(t)
	out := mustExport(t, o, 1)
	f, err := basefile.Check(out.Data)
	if err != nil {
		t.Fatalf("the file does not pass its own check: %v", err)
	}
	m := f.Manifest
	want := map[string]int{"base_backups": 1, "base_backup_linked_actors": 4, "actors": 4, "fgl_entities": 3, "actor_fgl_entities": 3, "permission_actor": 1, "permission_actor_rank": 1,
		"inventories": 2, "items": 3, "actor_inventories": 2, "buildings": 1, "building_instances": 3, "placeables": 3, "totems": 1, "landclaim_segments": 2}
	for tb, n := range want {
		if m.Counts[tb] != n {
			t.Errorf("%s: %d rows, want %d", tb, m.Counts[tb], n)
		}
	}
	if m.Summary != (basefile.Summary{Actors: 4, Pieces: 3, Placeables: 3, StoredItems: 3, Entities: 3, Rows: 34}) {
		t.Errorf("summary: %+v", m.Summary)
	}
	if m.Backup.ID != 1 || m.Backup.Name != "##Totem_Small_Placeable" || f.Format != "tabr-tau-base" || f.Version != 1 || f.Kind != "base-backup" || f.Editor != "test-1.2.3" || f.ExportedAt != "2026-10-28T12:00:00Z" {
		t.Errorf("header: %+v %+v", f, m.Backup)
	}
	tb := tablesOf(t, out.Data)
	// the original ids, exact: a 64-bit entity id is written as an integer literal
	ids := map[string]bool{}
	for _, r := range tb["fgl_entities"] {
		ids[string(r["entity_id"].(json.Number))] = true
	}
	for _, e := range []int64{eTotem, eChest, eStation} {
		if !ids[strconv.FormatInt(e, 10)] {
			t.Errorf("entity %d missing from %v", e, ids)
		}
	}
	if ids[strconv.FormatInt(eOther, 10)] {
		t.Error("the entity of the placed base is in the file")
	}
	for _, r := range tb["actors"] {
		if id := string(r["id"].(json.Number)); id == "801" || id == "802" {
			t.Errorf("an actor of the placed base is in the file: %v", r)
		}
	}
	for _, r := range tb["items"] {
		if string(r["id"].(json.Number)) == "9801" {
			t.Error("an item of the placed base's chest is in the file")
		}
	}
	// a stored item is whole: every column, the stats as JSON, a NULL as null
	var it map[string]any
	for _, r := range tb["items"] {
		if string(r["id"].(json.Number)) == "9002" {
			it = r
		}
	}
	if it["template_id"] != "IronOre" || string(it["stack_size"].(json.Number)) != "5" || string(it["volume_override"].(json.Number)) != "0.25" || string(it["quality_level"].(json.Number)) != "2" {
		t.Errorf("item 9002: %v", it)
	}
	if st, ok := it["stats"].(map[string]any); !ok || st["FItemStackAndDurabilityStats"] == nil {
		t.Errorf("stats are not JSON: %v", it["stats"])
	}
	for _, r := range tb["items"] {
		if string(r["id"].(json.Number)) == "9001" && r["volume_override"] != nil {
			t.Errorf("a NULL must stay null: %v", r)
		}
	}
	// the totem's own properties come across as JSON
	for _, r := range tb["actors"] {
		if string(r["id"].(json.Number)) == "601" {
			if p := r["properties"].(map[string]any); p["m_SelfUniqueID"] != "!!act#601" {
				t.Errorf("properties: %v", p)
			}
		}
	}
	if out.FileName != "tabr-tau-base-backup-1-Totem_Small_Placeable.json" {
		t.Errorf("file name: %s", out.FileName)
	}
}

func TestExportManifestSaysWhereTheBaseNamesThingsOutsideItself(t *testing.T) {
	o := exportOps(t, `insert into applied_patches(name,date) values ('P_ONE',1790000001),('P_TWO',1790000002),('P_THREE',1790000003),('P_FOUR',1790000004),('P_FIVE',1790000005),('P_SIX',1790000006);`)
	f, err := basefile.Check(mustExport(t, o, 1).Data)
	if err != nil {
		t.Fatal(err)
	}
	refs := map[string]basefile.Reference{}
	for _, r := range f.Manifest.References {
		refs[r.Kind] = r
	}
	// act: the totem's own id is inside, the character's pawn (a queued recipe) is outside, #0 is the null reference; itm: the recipe's item is in the station's own inventory
	if refs["act"] != (basefile.Reference{Kind: "act", Inside: 1, Outside: 1, Null: 1}) || refs["itm"] != (basefile.Reference{Kind: "itm", Inside: 1}) {
		t.Errorf("references: %+v", f.Manifest.References)
	}
	if refs["ownerEntity"] != (basefile.Reference{Kind: "ownerEntity", Inside: 6}) {
		t.Errorf("owner entities: %+v", refs["ownerEntity"])
	}
	if len(f.Manifest.Warnings) != 1 || !strings.Contains(f.Manifest.Warnings[0], "1 references name an actor that is not in the file") {
		t.Errorf("warnings: %v", f.Manifest.Warnings)
	}
	src := f.Manifest.Source
	if strings.Join(src.Maps, ",") != "HaggaBasin" || src.ActorStates["5"] != 4 || len(src.ActorStates) != 1 {
		t.Errorf("source: %+v", src)
	}
	if src.Patches.Count != 6 || len(src.Patches.Latest) != 5 || src.Patches.Latest[4] != "P_SIX" || len(src.Patches.Checksum) != 64 {
		t.Errorf("patches: %+v", src.Patches)
	}
	if f.Manifest.NotCarried.InventoriesOfStoredItems != 0 {
		t.Errorf("not carried: %+v", f.Manifest.NotCarried)
	}
}

func TestExportLeavesOutWhatIsPrivate(t *testing.T) {
	o := exportOps(t)
	data := string(mustExport(t, o, 1).Data)
	// the account of the save (the editor's test character is "Tester" on "PLATFORM-TEST" / "FUNCOM-TEST") and the account column never reach the file
	for _, s := range []string{"owner_account_id", "Tester", "PLATFORM-TEST", "FUNCOM-TEST", "account"} {
		if strings.Contains(data, s) {
			t.Errorf("the file mentions %q", s)
		}
	}
}

func TestExportIsDeterministicAndReadOnly(t *testing.T) {
	o := exportOps(t)
	before, _ := o.S.Query(`select count(*) n from actors`)
	a := mustExport(t, o, 1)
	b := mustExport(t, o, 1)
	if !bytes.Equal(a.Data, b.Data) {
		t.Fatal("the same save, time and editor gave different bytes")
	}
	later, err := o.ExportBaseBackup(1, exportAt.Add(time.Hour), "test-1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	fa, _ := basefile.Check(a.Data)
	fb, _ := basefile.Check(later.Data)
	if fa.Manifest.Checksum != fb.Manifest.Checksum || bytes.Equal(a.Data, later.Data) {
		t.Fatal("only the export time may differ; the checksum of the tables must be the same")
	}
	if strings.Contains(string(a.Data), "\r") || !strings.HasSuffix(string(a.Data), "}\n") {
		t.Error("the file is LF text that ends in a newline")
	}
	if o.S.Dirty() || len(o.S.Ops()) != 0 || len(o.S.Backups()) != 0 {
		t.Fatal("an export changed the save")
	}
	after, _ := o.S.Query(`select count(*) n from actors`)
	if before[0]["n"] != after[0]["n"] {
		t.Fatal("rows changed")
	}
}

func TestExportWritesOnlyAllowlistedTablesAndColumns(t *testing.T) {
	o := exportOps(t)
	data := mustExport(t, o, 1).Data
	var raw struct {
		Tables map[string][]map[string]json.RawMessage `json:"tables"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Tables) != len(basefile.Tables) {
		t.Fatalf("%d tables, want %d", len(raw.Tables), len(basefile.Tables))
	}
	for name, rows := range raw.Tables {
		tb, ok := basefile.Find(name)
		if !ok {
			t.Fatalf("table %s is not allowlisted", name)
		}
		for _, r := range rows {
			if len(r) != len(tb.Cols) {
				t.Fatalf("%s: %d columns in a row, want %d", name, len(r), len(tb.Cols))
			}
			for _, c := range tb.Cols {
				if _, ok := r[c.Name]; !ok {
					t.Fatalf("%s: column %s missing", name, c.Name)
				}
			}
		}
	}
	// the columns that must not travel
	for _, c := range []string{"owner_account_id", "vehicle_module_id"} {
		if strings.Contains(string(data), `"`+c+`"`) {
			t.Errorf("column %s is in the file", c)
		}
	}
	// every allowlisted column exists in the real schema (a typo in the list would fail the export of a real save)
	for _, tb := range basefile.Tables {
		have, err := o.S.Query(`select name from pragma_table_info(?)`, tb.Name)
		if err != nil || len(have) == 0 {
			t.Fatalf("table %s: %v", tb.Name, err)
		}
		names := map[string]bool{}
		for _, h := range have {
			names[h["name"].(string)] = true
		}
		for _, c := range tb.Cols {
			if !names[c.Name] {
				t.Errorf("%s.%s is not in the real schema", tb.Name, c.Name)
			}
		}
	}
}

func TestExportNameIsCarriedAsTextNotMarkup(t *testing.T) {
	o := exportOps(t)
	hostile := strings.ReplaceAll(testsave.HostileText, "'", "''")
	if _, err := o.S.Mutate("name the backup", func(m *save.Mut) error {
		_, err := m.Exec(`update base_backups set base_backup_name='` + hostile + `' where id=1`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	out, err := o.ExportBaseBackup(1, exportAt, "x")
	if err != nil {
		t.Fatal(err)
	}
	f, err := basefile.Check(out.Data)
	if err != nil || f.Manifest.Backup.Name != testsave.HostileText {
		t.Fatalf("name %q err %v", f.Manifest.Backup.Name, err)
	}
	if bytes.Contains(out.Data, []byte(`\u003c`)) {
		t.Error("the file escapes markup characters; it should hold the text as it is")
	}
	if strings.ContainsAny(out.FileName, `"'<>& /\`) || !strings.HasSuffix(out.FileName, ".json") {
		t.Errorf("file name: %q", out.FileName)
	}
}

func TestExportRefusals(t *testing.T) {
	cases := []struct {
		name  string
		world []string
		id    int64
		want  string
		kind  error
	}{
		{"no backup number in the save", nil, 99, "There is no base backup 99", apierr.ErrNotFound},
		{"no actors linked", []string{`insert into base_backups(id,player_id,base_backup_name,last_edited_by_player_id) values (2,1,'Empty',1);`}, 2, "no actors linked", apierr.ErrBadArgument},
		{"an actor that is not in the save", []string{`insert into base_backup_linked_actors(id,actor_id) values (1,9999);`}, 1, "names 1 actors that are not in the save", apierr.ErrBadArgument},
		{"a Solido in a storage", []string{`insert into items(id,inventory_id,stack_size,position_index,template_id,stats) values (9100,7001,1,5,'BuildingBlueprint_CopyDevice','{}');`}, 1, "Solido", apierr.ErrBadArgument},
		{"an inventory of an item", []string{`insert into inventories(id,actor_id,item_id,inventory_type) values (7100,701,9001,3);`}, 1, "belongs to an item", apierr.ErrBadArgument},
		{"a number that is not finite", []string{`update building_instances set health=9e999 where building_id=602 and instance_id=1;`}, 1, "health holds a number that is not finite", apierr.ErrBadArgument},
		{"JSON that is not valid", []string{`update fgl_entities set components=x'0102ff' where entity_id=5764607523034234881;`}, 1, "components holds data that is not valid JSON", apierr.ErrBadArgument},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := exportOps(t, c.world...)
			out, err := o.ExportBaseBackup(c.id, exportAt, "x")
			if err == nil {
				t.Fatalf("exported: %d bytes", len(out.Data))
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not say %q", err, c.want)
			}
			if !errors.Is(err, c.kind) {
				t.Fatalf("kind of %v", err)
			}
			if o.S.Dirty() || len(o.S.Ops()) != 0 {
				t.Fatal("a refused export changed the save")
			}
		})
	}
}

func TestExportOfASaveWithNoBackupSaysSoAndListsNothing(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	r, err := o.BaseBackups()
	if err != nil || len(r.([]save.Row)) != 0 {
		t.Fatalf("list: %v %v", r, err)
	}
	_, err = o.ExportBaseBackup(1, exportAt, "x")
	if err == nil || !strings.Contains(err.Error(), "This save has no base backup") || !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("error: %v", err)
	}
}

func TestExportSaysWhenABagInAStorageIsNotCarried(t *testing.T) {
	o := exportOps(t, `insert into inventories(id,item_id,inventory_type,max_item_count) values (7200,9002,3,4);`, `insert into items(id,inventory_id,stack_size,position_index,template_id,stats) values (9300,7200,1,0,'CopperOre','{}');`)
	f, err := basefile.Check(mustExport(t, o, 1).Data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Manifest.NotCarried != (basefile.NotCarried{InventoriesOfStoredItems: 1, ItemsInThem: 1}) {
		t.Fatalf("not carried: %+v", f.Manifest.NotCarried)
	}
	found := false
	for _, w := range f.Manifest.Warnings {
		found = found || strings.Contains(w, "belong to stored items")
	}
	if !found {
		t.Fatalf("warnings: %v", f.Manifest.Warnings)
	}
}

func TestExportSaysWhenActorsAreNotInThePickedUpState(t *testing.T) {
	o := exportOps(t, `update actors set state=0 where id=702;`)
	f, err := basefile.Check(mustExport(t, o, 1).Data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Manifest.Source.ActorStates["5"] != 3 || f.Manifest.Source.ActorStates["0"] != 1 || len(f.Manifest.Warnings) == 0 || !strings.Contains(f.Manifest.Warnings[0], "1 of 4 actors are not in the picked-up state") {
		t.Fatalf("source %+v warnings %v", f.Manifest.Source, f.Manifest.Warnings)
	}
	r, _ := o.BaseBackups()
	if r.([]save.Row)[0]["picked_up"].(int64) != 3 {
		t.Fatalf("the list counts the picked-up actors: %v", r)
	}
}

func TestExportFileNameKeepsOnlySafeCharacters(t *testing.T) {
	for in, want := range map[string]string{
		"##Totem_Small_Placeable": "tabr-tau-base-backup-3-Totem_Small_Placeable.json",
		`..\..\evil/name`:         "tabr-tau-base-backup-3-evil_name.json",
		"":                        "tabr-tau-base-backup-3-base.json",
		"   ":                     "tabr-tau-base-backup-3-base.json",
		"Haus <b>x</b>":           "tabr-tau-base-backup-3-Haus_b_x_b.json",
	} {
		if got := exportFileName(3, in); got != want {
			t.Errorf("%q: %s, want %s", in, got, want)
		}
	}
	if long := exportFileName(3, strings.Repeat("a", 300)); len(long) > 90 {
		t.Errorf("long name: %d", len(long))
	}
}

// A file that was changed, cut off or made by hand is refused whole by the check that an importer will use.
func TestCheckRefusesAFileThatIsNotWhatTheExportMakes(t *testing.T) {
	o := exportOps(t)
	good := mustExport(t, o, 1).Data
	if _, err := basefile.Check(good); err != nil {
		t.Fatal(err)
	}
	s := string(good)
	rep := func(old, new string) []byte {
		if !strings.Contains(s, old) {
			t.Fatalf("test file does not contain %q", old)
		}
		return []byte(strings.Replace(s, old, new, 1))
	}
	bad := map[string][]byte{
		"another format":           rep(`"format": "tabr-tau-base"`, `"format": "tabr-tau-kits"`),
		"another version":          rep(`"version": 1,`, `"version": 2,`),
		"another kind":             rep(`"kind": "base-backup"`, `"kind": "solido"`),
		"an unknown key":           rep(`"kind": "base-backup",`, `"kind": "base-backup", "path": "C:\\x",`),
		"an unknown table":         rep(`"totems": [`, `"sql": [], "totems": [`),
		"an extra column":          rep(`"slot_name":"Actor"}`, `"slot_name":"Actor","extra":1}`),
		"a missing column":         rep(`,"slot_name":"Actor"}`, `}`),
		"a changed value":          rep(`"template_id":"CopperOre"`, `"template_id":"SolarisCoin"`),
		"a cell of the wrong kind": rep(`"stack_size":3`, `"stack_size":"3"`),
		"a cut off file":           good[:len(good)/2],
		"text after the object":    append(append([]byte{}, good...), []byte(`{}`)...),
		"not JSON":                 []byte("hello"),
		"empty":                    nil,
		"a wrong checksum":         rep(`"value": "`, `"value": "0`),
		"over the size limit":      append(append([]byte{}, good[:len(good)-2]...), bytes.Repeat([]byte(" "), basefile.MaxFileBytes)...),
	}
	for name, data := range bad {
		if _, err := basefile.Check(data); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Against a real base backup. TAU_REAL_BASE_SAVE_SQLITE (or TAU_REAL_SAVE_SQLITE) names a COPY of a save the game wrote that holds a picked-up base, already decoded to a plain SQLite file.
// Without it, or when the copy has no base backup, the test is skipped, so the suite does not depend on any save being present. The copy is never written to: the export reads it.
func TestExportOfARealPickedUpBase(t *testing.T) {
	src := os.Getenv("TAU_REAL_BASE_SAVE_SQLITE")
	if src == "" {
		src = os.Getenv("TAU_REAL_SAVE_SQLITE")
	}
	if src == "" {
		t.Skip("TAU_REAL_BASE_SAVE_SQLITE is not set: no real save copy with a picked-up base to export")
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
	r, err := o.BaseBackups()
	if err != nil {
		t.Fatal(err)
	}
	list := r.([]save.Row)
	if len(list) == 0 {
		t.Skipf("%s holds no base backup: nothing to export", filepath.Base(src))
	}
	id := list[0]["backup_id"].(int64)
	out := mustExport(t, o, id)
	f, err := basefile.Check(out.Data)
	if err != nil {
		t.Fatalf("the export of the real base does not pass its own check: %v", err)
	}
	t.Logf("real backup %d (%v): %+v; %d bytes; references %+v; warnings %v", id, list[0]["name"], f.Manifest.Summary, len(out.Data), f.Manifest.References, f.Manifest.Warnings)
	// the list and the file agree
	row := list[0]
	if int64(f.Manifest.Summary.Actors) != row["actors"] || int64(f.Manifest.Summary.Pieces) != row["pieces"] || int64(f.Manifest.Summary.Placeables) != row["placeables"] || int64(f.Manifest.Summary.StoredItems) != row["stored_items"] {
		t.Errorf("list %v against file %+v", row, f.Manifest.Summary)
	}
	// every row of the file is a row of the save: the picked-up actors and nothing else
	st, _ := o.S.One(`select count(*) n from actors where state=5`)
	if int64(f.Manifest.Counts["actors"]) > st["n"].(int64) {
		t.Errorf("the file holds %d actors, the save has %d in state 5", f.Manifest.Counts["actors"], st["n"])
	}
	// the operator's save from 2026-10-27 held one backup with 44 actors; when this copy is that one the counts are known
	if f.Manifest.Counts["base_backup_linked_actors"] == 44 && len(list) == 1 {
		want := basefile.Summary{Actors: 44, Pieces: 174, Placeables: 43, StoredItems: 53, Entities: 68, Rows: 589}
		if f.Manifest.Summary != want {
			t.Errorf("the known pick-up has %+v, the export %+v", want, f.Manifest.Summary)
		}
	}
	// nothing private reaches the file: the character's name and the account's ids
	if p, err := o.player(); err == nil && p.Name != "" && strings.Contains(string(out.Data), p.Name) {
		t.Errorf("the file holds the character's name")
	}
	acc, _ := o.S.Query(`select platform_id, funcom_id, "user" u from accounts`)
	for _, a := range acc {
		for k, v := range a {
			if sv, ok := v.(string); ok && len(sv) > 3 && strings.Contains(string(out.Data), sv) {
				t.Errorf("the file holds the account's %s", k)
			}
		}
	}
	// deterministic, and the save was not touched
	again, err := o.ExportBaseBackup(id, exportAt, "x")
	first, _ := o.ExportBaseBackup(id, exportAt, "x")
	if err != nil || !bytes.Equal(again.Data, first.Data) {
		t.Error("the export of the real base is not deterministic")
	}
	if s.Dirty() || len(s.Ops()) != 0 {
		t.Error("the export changed the save")
	}
}

// A tool's empty slots (the real pick-up has two) are counted but are not worth a warning: nothing is lost.
func TestExportDoesNotWarnAboutAnEmptyInventoryOfAStoredItem(t *testing.T) {
	o := exportOps(t, `insert into inventories(id,item_id,inventory_type,max_item_count) values (7200,9002,3,4);`)
	f, err := basefile.Check(mustExport(t, o, 1).Data)
	if err != nil {
		t.Fatal(err)
	}
	if f.Manifest.NotCarried != (basefile.NotCarried{InventoriesOfStoredItems: 1}) || strings.Contains(strings.Join(f.Manifest.Warnings, "|"), "belong to stored items") {
		t.Fatalf("%+v %v", f.Manifest.NotCarried, f.Manifest.Warnings)
	}
}

// The limits of a file are checked with the caps lowered, so that the refusals are reached in milliseconds and no query comes near its time limit; the real values are asserted once.
func TestExportLimitsAreTheDocumentedOnes(t *testing.T) {
	if exportRowCap != 250000 || basefile.MaxRows != 250000 || exportByteCap != 16<<20 || basefile.MaxFileBytes != 16<<20 {
		t.Fatalf("limits: %d rows, %d bytes", exportRowCap, exportByteCap)
	}
}

func TestExportRefusesMoreRowsThanAFileMayHold(t *testing.T) {
	old := exportRowCap
	exportRowCap = 50
	t.Cleanup(func() { exportRowCap = old })
	o := exportOps(t, `with recursive c(n) as (select 1 union all select n+1 from c where n<60) insert into building_instances(building_id,instance_id,building_type) select 602, n+10, 'F' from c;`)
	_, err := o.ExportBaseBackup(1, exportAt, "x")
	if err == nil || !strings.Contains(err.Error(), "more than 50 rows") || !errors.Is(err, apierr.ErrBadArgument) {
		t.Fatalf("error: %v", err)
	}
	if o.S.Dirty() || len(o.S.Ops()) != 0 {
		t.Fatal("a refused export changed the save")
	}
	exportRowCap = 5000 // a few rows under the cap are fine
	if _, err := o.ExportBaseBackup(1, exportAt, "x"); err != nil {
		t.Fatalf("under the cap: %v", err)
	}
}

func TestExportRefusesAFileOverTheSizeLimit(t *testing.T) {
	old := exportByteCap
	exportByteCap = 3000
	t.Cleanup(func() { exportByteCap = old })
	o := exportOps(t)
	_, err := o.ExportBaseBackup(1, exportAt, "x")
	if err == nil || !strings.Contains(err.Error(), "the most a base file holds is 3000 bytes") || !errors.Is(err, apierr.ErrBadArgument) {
		t.Fatalf("error: %v", err)
	}
	exportByteCap = 16 << 20
	if _, err := o.ExportBaseBackup(1, exportAt, "x"); err != nil {
		t.Fatalf("under the cap: %v", err)
	}
}

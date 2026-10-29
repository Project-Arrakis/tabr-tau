package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	cf "github.com/Project-Arrakis/tabr-tau/internal/basefile/consolefixture"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Import of the Dune console's own base backup file (#493): converted by internal/basefile to the tables of a base file, then the same preview, checks and import.

func TestPreviewOfAConsoleFileSaysItIsFromTheConsole(t *testing.T) {
	o := target(t)
	before := dumpSave(t, o.S)
	r, err := o.PreviewBaseImport(cf.File(nil))
	if err != nil {
		t.Fatal(err)
	}
	pv := r.(map[string]any)
	if pv["dryRun"] != true || pv["from"] == nil || !strings.Contains(pv["from"].(string), `base backup "Test Reserve", map HaggaBasin, 3 pieces, 3 placeables, 3 stored items`) || !strings.Contains(pv["from"].(string), "Dune console (v1.4.52)") {
		t.Fatalf("from: %v", pv["from"])
	}
	if c, ok := pv["console"].(*basefile.ConsoleInfo); !ok || c.Name != "Test Reserve" || c.Kinds["BuildingInstance"] != 3 {
		t.Fatalf("console: %v", pv["console"])
	}
	if f := pv["file"].(map[string]any); f["origin"] != "console" || f["format"] != "dune-base-backup" || f["editor"] != "Dune console v1.4.52" {
		t.Fatalf("file: %v", f)
	}
	if s := pv["summary"].(basefile.Summary); s.Actors != 4 || s.Pieces != 3 || s.Placeables != 3 || s.StoredItems != 3 || s.Entities != 3 {
		t.Fatalf("summary: %+v", s)
	}
	warns := strings.Join(pv["warnings"].([]string), "\n")
	for _, w := range []string{"game build 2162777", "set to none", "4 of 4 actors in the file were in dimension 2", "all are written in dimension 0"} {
		if !strings.Contains(warns, w) {
			t.Errorf("warnings %q do not say %q", warns, w)
		}
	}
	if pv["patchesDiffer"] != false {
		t.Error("a console file has no patch set of the editor's kind to compare")
	}
	if pv["fileHash"] != basefile.Hash(cf.File(nil)) {
		t.Error("hash")
	}
	if dumpSave(t, o.S) != before || o.S.Dirty() || len(o.S.Ops()) != 0 {
		t.Fatal("the preview changed the save")
	}
	// a file of the editor says it is the editor's
	pv2, err := exportOps(t).PreviewBaseImport(exportedFile(t))
	if err == nil && pv2.(map[string]any)["from"] != nil {
		t.Error("a file of the editor says it is from the console")
	}
}

func TestImportOfAConsoleFileAddsANewBackupWithNewIDs(t *testing.T) {
	o := target(t, exportWorld)
	backups0, actors0 := cnt(t, o, `select count(*) from base_backups`), cnt(t, o, `select count(*) from actors`)
	im := mustImport(t, o, cf.File(nil))
	if n := cnt(t, o, `select count(*) from base_backups`); n != backups0+1 {
		t.Fatalf("backups %d -> %d", backups0, n)
	}
	if n := cnt(t, o, `select count(*) from actors`); n != actors0+4 {
		t.Fatalf("actors %d -> %d", actors0, n)
	}
	id := cnt(t, o, `select max(id) from base_backups`)
	if id == cf.Backup {
		t.Error("the backup kept the console's id")
	}
	const mine = `(select actor_id from base_backup_linked_actors where id=?)`
	for table, want := range map[string]int64{"actors": 4, "buildings": 1, "building_instances": 3, "placeables": 3, "totems": 1, "landclaim_segments": 1, "permission_actor": 1, "inventories": 2, "items": 3} {
		col := map[string]string{"building_instances": "building_id", "landclaim_segments": "totem_id", "permission_actor": "actor_id", "inventories": "actor_id"}[table]
		if table == "items" {
			if n := cnt(t, o, `select count(*) from items where inventory_id in (select id from inventories where actor_id in `+mine+`)`, id); n != want {
				t.Errorf("items: %d", n)
			}
			continue
		}
		if col == "" {
			col = "id"
		}
		if n := cnt(t, o, `select count(*) from `+table+` where `+col+` in `+mine, id); n != want {
			t.Errorf("%s: %d, want %d", table, n, want)
		}
	}
	// the owner is the save's character; every actor is picked up; the transform array became plain columns
	if r := one(t, o, `select player_id p, last_edited_by_player_id e, base_backup_name n from base_backups where id=?`, id); r["p"].(int64) != 1 || r["e"].(int64) != 0 || r["n"] != "Test Reserve" {
		t.Errorf("backup: %v", r)
	}
	if n := cnt(t, o, `select count(*) from actors where id in `+mine+` and state=5 and owner_account_id is null`, id); n != 4 {
		t.Errorf("picked-up actors: %d", n)
	}
	w := one(t, o, `select location_x x, location_y y, location_z z, rotation_x qx, rotation_y qy, rotation_z qz, rotation_w qw, health h, shelter s, sand_buildup b from building_instances where building_id in `+mine+` and building_type='Wall'`, id)
	if w["x"] != 5010.0 || w["y"] != 6000.5 || w["z"] != 7.0 || w["qx"] != 0.0 || w["qy"] != 0.0 || w["qz"] != 0.7071 || w["qw"] != 0.7071 || w["h"] != 100.5 || w["s"].(int64) != 1 || w["b"].(int64) != -1 {
		t.Errorf("the wall: %v", w)
	}
	a := one(t, o, `select location_x x, rotation_z qz, rotation_w qw, dimension_index d, serial s, typeof(properties) tp from actors where id in `+mine+` and class like '%Chest%'`, id)
	if a["x"] != 5010.0 || a["qz"] != 0.7071 || a["qw"] != 0.7071 || a["d"].(int64) != 0 || a["s"].(int64) != 26 || a["tp"] != "blob" {
		t.Errorf("the chest: %v", a)
	}
	// the references inside JSON name the new rows
	totem := one(t, o, `select a.id, json(a.properties) p from actors a where a.id in `+mine+` and a.class like '%Totem%'`, id)
	if want := fmt.Sprintf(`{"BP_Totem_C":{"m_Other":"!!act#0","m_SelfUniqueID":"!!act#%d"}}`, totem["id"].(int64)); totem["p"] != want {
		t.Errorf("the totem: %v, want %s", totem["p"], want)
	}
	scrap := one(t, o, `select i.id from items i where i.template_id='ScrapMetal' and i.inventory_id in (select id from inventories where actor_id in `+mine+`)`, id)["id"].(int64)
	st := one(t, o, `select json(e.components) c from fgl_entities e join actor_fgl_entities ae on ae.entity_id=e.entity_id where ae.slot_name='ItemCraftingComponent' and ae.actor_id in `+mine, id)
	if want := fmt.Sprintf(`{"ItemCraftingComponent":[[],{"Queue":[{"InstigatorActorId":"!!act#0","ItemUniqueId":"!!itm#%d","Spare":"!!act#0"}]}]}`, scrap); st["c"] != want {
		t.Errorf("the station: %v\nwant %s", st["c"], want)
	}
	// the entities have the shape of the save's: the prefix of its latest session, a counter above everything it holds. The console's own numbers are 6, 7 and 8.
	top := uint64(eOther) & 0xFFFFFFFF
	ents := mustQuery(t, o, `select ae.entity_id e from actor_fgl_entities ae where ae.actor_id in `+mine+` order by ae.entity_id`, id)
	if len(ents) != 3 {
		t.Fatalf("entities: %v", ents)
	}
	for _, e := range ents {
		u := uint64(e["e"].(int64))
		if u>>32 != uint64(eOther)>>32 || u&0xFFFFFFFF <= top {
			t.Errorf("entity %d: prefix %x counter %d (the save's highest was %d)", e["e"], u>>32, u&0xFFFFFFFF, top)
		}
	}
	if n := cnt(t, o, `select count(*) from placeables where id in `+mine+` and owner_entity_id in (select entity_id from actor_fgl_entities where actor_id in `+mine+`)`, id, id); n != 3 {
		t.Errorf("placeables with an owner entity of the new base: %d", n)
	}
	if n := cnt(t, o, `select count(*) from building_instances where building_id in `+mine+` and owner_entity_id in (select entity_id from actor_fgl_entities where actor_id in `+mine+`)`, id, id); n != 3 {
		t.Errorf("pieces with an owner entity of the new base: %d", n)
	}
	// nothing is left naming a row of the console's server or file, and the save is in order
	if r, err := o.S.Query(`pragma foreign_key_check`); err != nil || len(r) != 0 {
		t.Errorf("foreign key check: %v %v", r, err)
	}
	if r := one(t, o, `pragma integrity_check`); r["integrity_check"] != "ok" {
		t.Errorf("integrity: %v", r)
	}
	if im["summary"].(basefile.Summary).Rows != 32 {
		t.Errorf("rows: %+v", im["summary"])
	}
	// the Bases tab lists it, and the file of the editor made from it passes the editor's own check and imports again
	list, err := o.BaseBackups()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, r := range list.([]save.Row) {
		if r["backup_id"].(int64) == id {
			found = r["name"] == "Test Reserve" && r["actors"].(int64) == 4 && r["pieces"].(int64) == 3 && r["placeables"].(int64) == 3 && r["stored_items"].(int64) == 3
		}
	}
	if !found {
		t.Errorf("the Bases tab does not list the backup: %v", list)
	}
	exp, err := o.ExportBaseBackup(id, exportAt, "x")
	if err != nil {
		t.Fatalf("export of the import: %v", err)
	}
	if _, err := basefile.Check(exp.Data); err != nil {
		t.Fatal(err)
	}
	mustImport(t, o, exp.Data)
	if n := cnt(t, o, `select count(*) from base_backups`); n != backups0+2 {
		t.Errorf("backups after the second import: %d", n)
	}
}

func TestConsoleFileIntoASaveWithNoEntitiesOrBase(t *testing.T) {
	o := target(t)
	mustImport(t, o, cf.File(nil))
	list, err := o.BaseBackups()
	if err != nil {
		t.Fatal(err)
	}
	rows := list.([]save.Row)
	if len(rows) != 1 || rows[0]["name"] != "Test Reserve" || rows[0]["pieces"].(int64) != 3 {
		t.Fatalf("the Bases tab lists: %v", rows)
	}
	// the same file twice is two backups
	mustImport(t, o, cf.File(nil))
	if n := cnt(t, o, `select count(*) from base_backups`); n != 2 {
		t.Fatalf("backups: %d", n)
	}
	if n := cnt(t, o, `select count(distinct entity_id) from actor_fgl_entities where actor_id in (select actor_id from base_backup_linked_actors)`); n != 6 {
		t.Fatalf("entities of two imports: %d", n)
	}
}

func TestConsoleFileRefusalsLeaveTheSaveAsItWas(t *testing.T) {
	o := target(t, exportWorld)
	for name, c := range map[string]struct {
		edit func(h cf.Data, e *[]cf.Entry)
		want string
	}{
		"a stored Solido": {func(h cf.Data, e *[]cf.Entry) {
			*e = append(*e, cf.Entry{"id": 99, "kind": "bbp", "data": cf.Data{"item_id": 12}})
		}, "Solido"},
		"a reference to another server's item": {func(h cf.Data, e *[]cf.Entry) {
			cf.DataOf(*e, cf.EntChest)["components"] = cf.Data{"A": "!!itm#9"}
		}, "server the file came from"},
		"a reference to an item that is not in the file": {func(h cf.Data, e *[]cf.Entry) {
			cf.DataOf(*e, cf.EntChest)["components"] = cf.Data{"A": "!!itm@999"}
		}, "not in the file"},
		"an item in no inventory": {func(h cf.Data, e *[]cf.Entry) { cf.DataOf(*e, 12)["inventory_id"] = 4000 }, "not in the file"},
		"a standing actor":        {func(h cf.Data, e *[]cf.Entry) { cf.DataOf(*e, cf.Chest)["state"] = "Normal" }, "BaseBackup"},
		"an unknown field":        {func(h cf.Data, e *[]cf.Entry) { cf.DataOf(*e, cf.Chest)["account"] = 5 }, "does not know"},
		"a player's actor": {func(h cf.Data, e *[]cf.Entry) {
			cf.DataOf(*e, cf.Chest)["class"] = "/Game/BP_PlayerCharacter.BP_PlayerCharacter_C"
		}, "player"},
	} {
		t.Run(name, func(t *testing.T) {
			d := cf.File(c.edit)
			refuses(t, o, d, c.want)
		})
	}
}

func TestConsoleFileLimitsAreTheImportersLimits(t *testing.T) {
	o := target(t)
	data := cf.File(nil)
	was := importByteCap
	importByteCap = len(data) - 1
	t.Cleanup(func() { importByteCap = was })
	refuses(t, o, data, "larger than")
	importByteCap = was
	wasRows := importRowCap
	importRowCap = 10
	t.Cleanup(func() { importRowCap = wasRows })
	refuses(t, o, data, "more than 10")
}

// The real file of the console, on a COPY of a real save. Skipped unless both are named.
func TestImportOfARealConsoleFileIntoACopyOfARealSave(t *testing.T) {
	file, src := os.Getenv("TAU_REAL_CONSOLE_FILE"), os.Getenv("TAU_REAL_BASE_SAVE_SQLITE")
	if file == "" || src == "" {
		t.Skip("TAU_REAL_CONSOLE_FILE and TAU_REAL_BASE_SAVE_SQLITE are not both set: no real console file and save copy to import")
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
	before := dumpSave(t, s)
	pv, err := o.PreviewBaseImport(data)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if dumpSave(t, s) != before {
		t.Fatal("the preview changed the save")
	}
	pm := pv.(map[string]any)
	j, _ := json.Marshal(map[string]any{"from": pm["from"], "summary": pm["summary"], "counts": pm["counts"], "ids": pm["ids"], "owner": pm["owner"], "references": pm["references"], "warnings": pm["warnings"]})
	t.Logf("preview: %s", j)
	fkBefore, _ := s.Query(`pragma foreign_key_check`)
	backups0 := cnt(t, o, `select count(*) from base_backups`)
	im := mustImport(t, o, data)
	sum := im["summary"].(basefile.Summary)
	if sum.Pieces != 811 || sum.Placeables != 204 || sum.StoredItems != 21 {
		t.Errorf("imported %+v", sum)
	}
	if n := cnt(t, o, `select count(*) from base_backups`); n != backups0+1 {
		t.Fatalf("backups %d -> %d", backups0, n)
	}
	id := cnt(t, o, `select max(id) from base_backups`)
	const mine = `(select actor_id from base_backup_linked_actors where id=?)`
	if n := cnt(t, o, `select count(*) from building_instances where building_id in `+mine, id); n != 811 {
		t.Errorf("pieces in the save: %d", n)
	}
	if n := cnt(t, o, `select count(*) from placeables where id in `+mine, id); n != 204 {
		t.Errorf("placeables in the save: %d", n)
	}
	if n := cnt(t, o, `select count(*) from items where inventory_id in (select id from inventories where actor_id in `+mine+`)`, id); n != 21 {
		t.Errorf("items in the save: %d", n)
	}
	if fk, _ := s.Query(`pragma foreign_key_check`); len(fk) != len(fkBefore) {
		t.Errorf("foreign key check rows: %d before, %d after", len(fkBefore), len(fk))
	}
	if r := one(t, o, `pragma integrity_check`); r["integrity_check"] != "ok" {
		t.Fatalf("integrity: %v", r)
	}
	// the export of what was imported is a good file of the editor, and imports again
	second, err := o.ExportBaseBackup(id, exportAt, "x")
	if err != nil {
		t.Fatalf("export of the import: %v", err)
	}
	f, err := basefile.Check(second.Data)
	if err != nil || f.Manifest.Summary.Pieces != 811 {
		t.Fatalf("check of the export: %v", err)
	}
	mustImport(t, o, second.Data)
	if fl := s.Flush(); !fl.Saved || fl.Warning != "" || fl.Stuck {
		t.Fatalf("writing the imported copy: %+v", fl)
	}
}

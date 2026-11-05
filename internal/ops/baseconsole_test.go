package ops

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Export of a base as the Dune console's own file (#534). The source is the made-up world of baseexport_test.go (exportWorld): there the first base was picked up; here it is put back
// as a base that is still standing (state 0, its totem's permission rows and rank back, no backup record), which is the other thing a game can have.

const standingBase = `
update actors set state=0 where id in (601,602,701,702);
delete from base_backup_linked_actors; delete from base_backups;
update actors set serial=0 where id=602;
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (601,'Ore Base',1,3,0,1);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1),(701,5,3),(601,5,2);`

// The same base after the game's pick-up, to compare the two files: the backup record is back and the totem's permission rows are gone. The building actor is the fresh copy.
const pickUpOfBase = `update actors set serial=0 where id=602; update base_backups set base_backup_name='Ore Base', last_edited_by_player_id=0;`

func standingOps(t *testing.T, extra ...string) *Ops {
	t.Helper()
	return exportOps(t, append([]string{standingBase}, extra...)...)
}

func mustStanding(t *testing.T, o *Ops, totem int64) *BaseExport {
	t.Helper()
	out, err := o.ExportStandingBaseConsole(totem, exportAt, "test-1.2.3")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return out
}

// entriesOf reads a console file with the numbers kept as text.
func entriesOf(t *testing.T, data []byte) (map[string]any, []map[string]any) {
	t.Helper()
	var f map[string]json.RawMessage
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	var head map[string]any
	if err := json.Unmarshal(data, &head); err != nil {
		t.Fatal(err)
	}
	var es []map[string]any
	if err := json.Unmarshal(f["entries"], &es); err != nil {
		t.Fatal(err)
	}
	return head, es
}

func ofKind(es []map[string]any, kind string) []map[string]any {
	var out []map[string]any
	for _, e := range es {
		if e["kind"] == kind {
			out = append(out, e)
		}
	}
	return out
}

func TestStandingBaseIsWrittenAsTheConsolesFileAndReadBackByTheEditorsReader(t *testing.T) {
	o := standingOps(t)
	out := mustStanding(t, o, 601)
	b, err := basefile.Load(out.Data, basefile.MaxFileBytes, basefile.MaxRows)
	if err != nil {
		t.Fatalf("the editor's reader refuses the file: %v", err)
	}
	if err := b.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if b.Origin != basefile.OriginConsole || b.File.Format != "dune-base-backup" || b.File.Version != 1 || b.Console == nil || b.Console.SourceKind != "live-base" {
		t.Fatalf("origin: %+v %+v", b.File, b.Console)
	}
	want := map[string]int{"base_backups": 1, "base_backup_linked_actors": 4, "actors": 4, "fgl_entities": 3, "actor_fgl_entities": 3, "permission_actor": 1, "permission_actor_rank": 1,
		"inventories": 2, "items": 3, "actor_inventories": 2, "buildings": 1, "building_instances": 3, "placeables": 3, "totems": 1, "landclaim_segments": 2}
	for tb, n := range want {
		if got := len(b.Of(tb).R); got != n {
			t.Errorf("%s: %d rows, want %d", tb, got, n)
		}
	}
	// the numbers the message repeats are the numbers of the file
	if s := out.Summary; s.Actors != 4 || s.Pieces != 3 || s.Placeables != 3 || s.StoredItems != 3 || s.Entities != 3 {
		t.Errorf("summary: %+v", s)
	}
	if c := b.Console; c.Name != "Ore Base" || c.Map != "HaggaBasin" || c.Declared["pieces"] != 3 || c.Declared["placeables"] != 3 || c.Declared["items"] != 3 {
		t.Errorf("console info: %+v", c)
	}
	// every actor is in the backed-up state
	for i := range b.Of("actors").R {
		if b.Of("actors").R[i][b.Of("actors").Col("state")] != int64(basefile.StateBackup) {
			t.Errorf("actor %d is not in the backed-up state", i)
		}
	}
	// the backup record is built from the totem: the permission actor's name, the owner as the placeholder
	bb := b.Of("base_backups")
	if bb.R[0][bb.Col("base_backup_name")] != "Ore Base" || bb.R[0][bb.Col("last_edited_by_player_id")] != int64(0) {
		t.Errorf("backup record: %v", bb.R[0])
	}
	// the file holds the base's own pieces and nothing of the other base
	for _, c := range []string{"5764607523034235001"} {
		if strings.Contains(string(out.Data), c) {
			t.Errorf("the file names an id of the other base: %s", c)
		}
	}
	if strings.Contains(string(out.Data), "Other base") || strings.Contains(string(out.Data), `"Totem_Placeable"`) {
		t.Error("the other base is in the file")
	}
	if out.FileName != "tabr-tau-console-base-601-Ore_Base.json" {
		t.Errorf("file name %q", out.FileName)
	}
}

func TestStandingBaseGivesWhatAPickUpFollowedByAnExportGives(t *testing.T) {
	live := mustStanding(t, standingOps(t), 601)
	picked, err := exportOps(t, pickUpOfBase).ExportBaseBackupConsole(1, exportAt, "test-1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	// the two files hold the same rows once every id is renumbered: each is imported into an empty save and exported again in the editor's own format
	var canons []string
	for _, f := range [][]byte{live.Data, picked.Data} {
		o := target(t)
		mustImport(t, o, f)
		second, err := o.ExportBaseBackup(cnt(t, o, `select max(id) from base_backups`), exportAt, "test-1.2.3")
		if err != nil {
			t.Fatal(err)
		}
		canons = append(canons, canon(t, second.Data))
	}
	if canons[0] != canons[1] {
		t.Fatalf("a standing base and its pick-up differ:\n--- standing\n%s\n--- picked up\n%s", canons[0], canons[1])
	}
	if live.Summary != picked.Summary {
		t.Errorf("counts: %+v against %+v", live.Summary, picked.Summary)
	}
}

func TestStandingBaseExportShapesTheEntriesLikeAPickUp(t *testing.T) {
	out := mustStanding(t, standingOps(t), 601)
	head, es := entriesOf(t, out.Data)
	if head["format"] != "dune-base-backup" || head["version"] != float64(1) || head["exportedAt"] != "2026-10-28T12:00:00.000Z" || head["ownerPlaceholderTransferId"] != float64(1) {
		t.Fatalf("header: %v", head)
	}
	src := head["source"].(map[string]any)
	if src["kind"] != "live-base" || src["baseId"] != float64(601) || src["backupId"] != nil || src["ownerName"] != "" {
		t.Errorf("source: %v", src)
	}
	if c := src["counts"].(map[string]any); c["pieces"] != float64(3) || c["placeables"] != float64(3) || c["items"] != float64(3) {
		t.Errorf("counts: %v", c)
	}
	// the owner is a placeholder with no data; transfer ids are small and unique
	if es[0]["id"] != float64(1) || es[0]["kind"] != "act" || len(es[0]["data"].(map[string]any)) != 0 {
		t.Errorf("placeholder: %v", es[0])
	}
	seen := map[float64]bool{}
	for _, e := range es {
		id := e["id"].(float64)
		if seen[id] || id > 1000 {
			t.Errorf("entry id %v", id)
		}
		seen[id] = true
	}
	// the building actor is a fresh copy: no entity, no properties, serial 0, and class, map, transform kept
	var building map[string]any
	for _, e := range ofKind(es, "act") {
		if d := e["data"].(map[string]any); d["class"] == "/Game/BP_Building.BP_Building_C" {
			building = d
		}
	}
	if building == nil || building["serial"] != float64(0) || len(building["properties"].(map[string]any)) != 0 || building["state"] != "BaseBackup" || building["owner_account_id"] != nil {
		t.Errorf("building actor: %v", building)
	}
	for _, e := range ofKind(es, "fgl") {
		if e["data"].(map[string]any)["slot_name"] == "" {
			t.Error("an entity with no slot")
		}
	}
	if n := len(ofKind(es, "fgl")); n != 3 {
		t.Errorf("entities: %d", n)
	}
	// no permission row of the totem, no rank of another player, no invoice
	perms := ofKind(es, "PermissionActor")
	if len(perms) != 1 || perms[0]["data"].(map[string]any)["actor_name"] != "Ore Out" {
		t.Errorf("permission rows: %v", perms)
	}
	ranks := ofKind(es, "PermissionActorRank")
	if len(ranks) != 0 {
		// the chest's rank row (701, player 1, rank 2) belongs to the owner and is kept; the other player's is not
		for _, r := range ranks {
			if r["data"].(map[string]any)["player_id"] != float64(1) {
				t.Errorf("rank: %v", r)
			}
		}
	}
	if len(ranks) != 1 {
		t.Errorf("ranks: %d", len(ranks))
	}
	for _, e := range es {
		switch e["kind"] {
		case "TaxInvoice", "Sinkchart", "bbp":
			t.Errorf("an entry of a kind a save has no table for: %v", e["kind"])
		}
	}
	// the references inside JSON name the file's own ids; the id of an actor that is not in the file stays as the game's own export leaves it
	for _, e := range ofKind(es, "act") {
		d := e["data"].(map[string]any)
		if d["class"] == "/Game/BP_Totem.BP_Totem_C" {
			want := "!!act@" + strconv.FormatInt(int64(e["id"].(float64)), 10)
			if got := d["properties"].(map[string]any)["m_SelfUniqueID"]; got != want {
				t.Errorf("the totem's own id: %v, want %s", got, want)
			}
		}
	}
	text := string(out.Data)
	if !strings.Contains(text, `"InstigatorActorId":"!!act#2"`) || !strings.Contains(text, `"Spare":"!!act#0"`) || strings.Contains(text, `!!itm#9003`) || !strings.Contains(text, `"ItemUniqueId":"!!itm@`) {
		t.Errorf("references: %s", text)
	}
	found := false
	for _, n := range out.Notes {
		found = found || strings.Contains(n, "1 references name an actor that is not in the base")
	}
	if !found {
		t.Errorf("notes %q", out.Notes)
	}
	// a 64-bit entity id is never written: the file has its own small ids
	if strings.Contains(text, "5764607523034234881") {
		t.Error("an entity id of the save is in the file")
	}
}

func TestStandingBaseExportTakesOnlyThePiecesTheTotemOwns(t *testing.T) {
	// the building actor 602 also holds a piece that the other base's totem owns; and the other base owns the piece in its own building
	o := standingOps(t, `insert into building_instances(building_id,instance_id,building_type,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,owner_entity_id,building_flags,health,shelter,sand_buildup,last_placed_by_player_id)
		values (602,9,'Foundation',90020,90000,0,0,0,0,1,5764607523034235001,0,100.0,0,0,1);`)
	a := mustStanding(t, o, 601)
	if a.Summary.Pieces != 3 || a.Summary.Actors != 4 {
		t.Errorf("base A: %+v", a.Summary)
	}
	if strings.Contains(string(a.Data), "90020") {
		t.Error("the piece of the other base is in the file")
	}
	b := mustStanding(t, o, 801)
	// base B: its totem, its building 802, and 602 for the one piece it holds there
	if b.Summary.Pieces != 3 || b.Summary.Actors != 3 || b.Summary.Placeables != 1 {
		t.Errorf("base B: %+v", b.Summary)
	}
	// and the row the Bases tab shows for it says the same
	rows, err := o.baseListRows("Owner")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		want := map[int64]basefile.Summary{601: a.Summary, 801: b.Summary}[r["base_id"].(int64)]
		if r["pieces"] != int64(want.Pieces) || r["placeables"] != int64(want.Placeables) {
			t.Errorf("row %v against file %+v", r, want)
		}
	}
}

func TestStandingBaseExportChangesNothingAndIsRepeatable(t *testing.T) {
	o := standingOps(t)
	before := dumpSave(t, o.S)
	a, b := mustStanding(t, o, 601), mustStanding(t, o, 601)
	if string(a.Data) != string(b.Data) {
		t.Error("two exports of the same save differ")
	}
	if dumpSave(t, o.S) != before || o.S.Dirty() || len(o.S.Ops()) != 0 {
		t.Fatal("the export changed the save")
	}
	// there is no backup in this save, whatever the totem says
	if _, err := o.ExportBaseBackupConsole(1, exportAt, "test-1.2.3"); err == nil {
		t.Fatal("a save with no backup exported one")
	}
}

func TestStandingBaseExportRefusesWhatIsNotAStandingBaseOfAnOwner(t *testing.T) {
	for name, c := range map[string]struct {
		world []string
		totem int64
		want  string
	}{
		"no such base":                 {nil, 12345, "There is no base with the totem 12345"},
		"an actor that is not a totem": {nil, 701, "There is no base with the totem 701"},
		"a picked-up base":             {[]string{`update actors set state=5 where id in (601,602,701,702);`}, 601, "picked up with the game's base tool"},
		"no owner":                     {[]string{`delete from permission_actor_rank where permission_actor_id=601;`}, 601, "no owner"},
		"only a co-owner":              {[]string{`update permission_actor_rank set rank=2 where permission_actor_id=601 and player_id=1;`}, 601, "no owner"},
		"no entity":                    {[]string{`delete from actor_fgl_entities where actor_id=601;`}, 601, "has no entity"},
		"a Solido in a chest":          {[]string{`update items set template_id='BuildingBlueprint_CopyDevice' where id=9001;`}, 601, "Solido"},
		"a bag in a chest":             {[]string{`insert into inventories(id,actor_id,item_id,inventory_type,max_item_count,max_item_volume) values (7900,701,9001,1,5,10.0);`}, 601, "belongs to an item"},
	} {
		t.Run(name, func(t *testing.T) {
			o := standingOps(t, c.world...)
			before := dumpSave(t, o.S)
			out, err := o.ExportStandingBaseConsole(c.totem, exportAt, "test-1.2.3")
			if err == nil {
				t.Fatalf("exported: %+v", out.Summary)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not say %q", err, c.want)
			}
			if dumpSave(t, o.S) != before {
				t.Error("a refused export changed the save")
			}
		})
	}
}

func TestStandingBaseExportRefusesWhatTheFileCannotHold(t *testing.T) {
	defer func(r, b int) { exportRowCap, exportByteCap = r, b }(exportRowCap, exportByteCap)
	for name, c := range map[string]struct {
		world []string
		rows  int
		bytes int
		want  string
	}{
		"too many rows":                {nil, 10, basefile.MaxFileBytes, "more than 10 rows"},
		"too big a file":               {nil, basefile.MaxRows, 1000, "the most a base file holds"},
		"a stat that is not an object": {[]string{`update items set stats='[1,2]' where id=9001;`}, basefile.MaxRows, basefile.MaxFileBytes, "stats"},
		"text that is not JSON":        {[]string{`update fgl_entities set components=jsonb('{"a":1}') where entity_id=5764607523034234882; update items set stats='{broken' where id=9001;`}, basefile.MaxRows, basefile.MaxFileBytes, "not valid JSON"},
		"a reference inside a text":    {[]string{`update fgl_entities set components=jsonb('{"a":"see !!act#601"}') where entity_id=5764607523034234882;`}, basefile.MaxRows, basefile.MaxFileBytes, "inside other text"},
	} {
		t.Run(name, func(t *testing.T) {
			o := standingOps(t, c.world...)
			exportRowCap, exportByteCap = c.rows, c.bytes
			_, err := o.ExportStandingBaseConsole(601, exportAt, "test-1.2.3")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want %q", err, c.want)
			}
		})
	}
}

func TestStandingBaseExportNotesAHologramAndAnUnownedPlaceable(t *testing.T) {
	o := standingOps(t, `update placeables set is_hologram=1 where id=702; update placeables set owner_entity_id=5764607523034235001 where id=701; update placeables set owner_entity_id=5764607523034234881 where id=701;`)
	out := mustStanding(t, o, 601)
	if !strings.Contains(strings.Join(out.Notes, "\n"), "1 of the 3 placeables are unfinished placements") {
		t.Errorf("notes %q", out.Notes)
	}
}

func TestPickedUpBaseIsWrittenAsTheConsolesFile(t *testing.T) {
	o := exportOps(t, players77)
	out, err := o.ExportBaseBackupConsole(1, exportAt, "test-1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	b, err := basefile.Load(out.Data, basefile.MaxFileBytes, basefile.MaxRows)
	if err == nil {
		err = b.Verify()
	}
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"base_backups": 1, "base_backup_linked_actors": 4, "actors": 4, "fgl_entities": 3, "permission_actor": 1, "permission_actor_rank": 1, "inventories": 2, "items": 3,
		"actor_inventories": 2, "buildings": 1, "building_instances": 3, "placeables": 3, "totems": 1, "landclaim_segments": 2}
	for tb, n := range want {
		if got := len(b.Of(tb).R); got != n {
			t.Errorf("%s: %d rows, want %d", tb, got, n)
		}
	}
	// the counts of the row of the Base backups card
	rows, _ := o.BaseBackups()
	if r := rows.([]save.Row)[0]; r["pieces"] != int64(out.Summary.Pieces) || r["placeables"] != int64(out.Summary.Placeables) || r["stored_items"] != int64(out.Summary.StoredItems) || r["actors"] != int64(out.Summary.Actors) {
		t.Errorf("the row %v and the file %+v differ", r, out.Summary)
	}
	if out.Summary.Pieces != 3 || out.Summary.Placeables != 3 || out.Summary.StoredItems != 3 || out.Summary.Actors != 4 {
		t.Errorf("summary %+v", out.Summary)
	}
	head, es := entriesOf(t, out.Data)
	if src := head["source"].(map[string]any); src["kind"] != nil || src["backupId"] != float64(1) {
		t.Errorf("source %v", src)
	}
	// a rank of another player than the owner is not carried; every id of a player is the placeholder's or zero
	for _, e := range ofKind(es, "PermissionActorRank") {
		if e["data"].(map[string]any)["player_id"] != float64(1) {
			t.Errorf("rank %v", e)
		}
	}
	if out.FileName != "tabr-tau-console-backup-1-Sub-Fief.json" {
		t.Errorf("file name %q", out.FileName)
	}
	// the other file of the same backup holds the same rows: the editor's own export and this one agree on the counts
	own := mustExport(t, o, 1).Summary
	if own.Actors != out.Summary.Actors || own.Pieces != out.Summary.Pieces || own.Placeables != out.Summary.Placeables || own.StoredItems != out.Summary.StoredItems || own.Entities != out.Summary.Entities {
		t.Errorf("own %+v, console %+v", own, out.Summary)
	}
	if _, err := o.ExportBaseBackupConsole(99, exportAt, "x"); err == nil || !strings.Contains(err.Error(), "no base backup 99") {
		t.Errorf("unknown backup: %v", err)
	}
	if _, err := target(t).ExportBaseBackupConsole(1, exportAt, "x"); err == nil || !strings.Contains(err.Error(), "no base backup") {
		t.Errorf("empty save: %v", err)
	}
}

func TestPickedUpBaseConsoleExportRefusesALoneSolido(t *testing.T) {
	o := exportOps(t, `update items set template_id='BuildingBlueprint_CopyDevice' where id=9001;`)
	if _, err := o.ExportBaseBackupConsole(1, exportAt, "x"); err == nil || !strings.Contains(err.Error(), "Solido") {
		t.Errorf("err %v", err)
	}
}

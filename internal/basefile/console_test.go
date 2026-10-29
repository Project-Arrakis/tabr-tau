package basefile

import (
	"strings"
	"testing"

	cf "github.com/Project-Arrakis/tabr-tau/internal/basefile/consolefixture"
)

// The console's own file (#493): converted to the tables of a base file, strictly.

func loadFixture(t *testing.T, edit func(h cf.Data, e *[]cf.Entry)) *Base {
	t.Helper()
	b, err := Load(cf.File(edit), MaxFileBytes, MaxRows)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := b.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
	return b
}

func refusedConsoleFile(t *testing.T, edit func(h cf.Data, e *[]cf.Entry), want string) {
	t.Helper()
	b, err := Load(cf.File(edit), MaxFileBytes, MaxRows)
	if err == nil {
		err = b.Verify()
	}
	if err == nil {
		t.Fatalf("accepted; want an error with %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not say %q", err, want)
	}
}

func cell(t *testing.T, b *Base, table string, row int, col string) any {
	t.Helper()
	rs := b.Of(table)
	if row >= len(rs.R) {
		t.Fatalf("%s has %d rows", table, len(rs.R))
	}
	return rs.R[row][rs.Col(col)]
}

func TestConsoleFileBecomesTheTablesOfABaseFile(t *testing.T) {
	b := loadFixture(t, nil)
	if b.Origin != OriginConsole || b.Console == nil || b.File.Format != ConsoleFormat || b.File.Kind != KindBackup {
		t.Fatalf("origin %q console %v file %+v", b.Origin, b.Console, b.File)
	}
	want := map[string]int{"base_backups": 1, "base_backup_linked_actors": 4, "actors": 4, "fgl_entities": 3, "actor_fgl_entities": 3, "permission_actor": 1, "permission_actor_rank": 0,
		"inventories": 2, "items": 3, "actor_inventories": 2, "buildings": 1, "building_instances": 3, "placeables": 3, "totems": 1, "landclaim_segments": 1}
	for name, n := range want {
		if got := len(b.Of(name).R); got != n {
			t.Errorf("%s: %d rows, want %d", name, got, n)
		}
	}
	// the placeholder is not an actor; every actor is picked up (state 5) and has no account
	for i := range b.Of("actors").R {
		if cell(t, b, "actors", i, "state") != int64(StateBackup) {
			t.Errorf("actor %d state %v", i, cell(t, b, "actors", i, "state"))
		}
		if cell(t, b, "actors", i, "id") == int64(cf.Placeholder) {
			t.Error("the placeholder became an actor")
		}
	}
	// an actor's transform: a location and the four numbers of the rotation, into plain columns
	if cell(t, b, "actors", 2, "id") != int64(cf.Chest) || cell(t, b, "actors", 2, "location_x") != 5010.0 || cell(t, b, "actors", 2, "rotation_z") != 0.7071 || cell(t, b, "actors", 2, "rotation_w") != 0.7071 {
		t.Errorf("the chest: %v", b.Of("actors").R[2])
	}
	// a piece's transform is one list [x, y, z, qx, qy, qz, qw]
	bi := b.Of("building_instances")
	if r := bi.R[1]; r[bi.Col("location_x")] != 5010.0 || r[bi.Col("location_y")] != 6000.5 || r[bi.Col("rotation_z")] != 0.7071 || r[bi.Col("rotation_w")] != 0.7071 || r[bi.Col("rotation_x")] != 0.0 {
		t.Errorf("piece 2: %v", r)
	}
	// flags are 1 and 0, a number the file calls an integer for a REAL column is a number, null stays null
	pl := b.Of("placeables")
	if pl.R[0][pl.Col("has_buildable_support")] != int64(1) || pl.R[0][pl.Col("is_hologram")] != int64(0) || pl.R[0][pl.Col("health")] != 500.0 {
		t.Errorf("placeable: %v", pl.R[0])
	}
	it := b.Of("items")
	if it.R[0][it.Col("is_new")] != int64(1) || it.R[0][it.Col("volume_override")] != nil || it.R[1][it.Col("volume_override")] != 0.25 {
		t.Errorf("items: %v %v", it.R[0], it.R[1])
	}
	tt := b.Of("totems")
	if tt.R[0][tt.Col("landclaim_original_global_location_y")] != 6000.0 || tt.R[0][tt.Col("landclaim_original_global_yaw_rotation")] != 125.0 || tt.R[0][tt.Col("last_backup_timestamp")] != int64(6636633) {
		t.Errorf("totem: %v", tt.R[0])
	}
	// an entity: its transfer id is its id, the link names its actor, the components are JSON
	if cell(t, b, "fgl_entities", 0, "entity_id") != int64(cf.EntTotem) || cell(t, b, "actor_fgl_entities", 2, "actor_id") != int64(cf.Refinery) || cell(t, b, "actor_fgl_entities", 2, "slot_name") != "ItemCraftingComponent" {
		t.Error("entities")
	}
	if cell(t, b, "base_backups", 0, "player_id") != int64(cf.Placeholder) || cell(t, b, "base_backups", 0, "id") != int64(cf.Backup) || cell(t, b, "base_backups", 0, "base_backup_name") != "Test Reserve" {
		t.Error("backup")
	}
	if cell(t, b, "landclaim_segments", 0, "grid_location_y") != int64(-1) {
		t.Error("segment")
	}
}

func TestConsoleFileSaysWhereItCameFrom(t *testing.T) {
	b := loadFixture(t, nil)
	c := b.Console
	if c.Name != "Test Reserve" || c.Map != "HaggaBasin" || c.ConsoleVersion != "v1.4.52" || c.GameBuild != "2162777" || c.BackupID != 2 || c.Entries != len(cf.Entries()) {
		t.Errorf("%+v", c)
	}
	if c.Kinds["BuildingInstance"] != 3 || c.Kinds["Placeable"] != 3 || c.Kinds["itm"] != 3 {
		t.Errorf("kinds %v", c.Kinds)
	}
	d := c.Describe()
	for _, w := range []string{"Dune console", "v1.4.52", `"Test Reserve"`, "HaggaBasin", "3 pieces", "3 placeables", "3 stored items"} {
		if !strings.Contains(d, w) {
			t.Errorf("%q does not say %q", d, w)
		}
	}
	if b.File.Editor != "Dune console v1.4.52" || b.File.Manifest.Backup.ID != 2 || b.File.Manifest.Source.Patches.Checksum != "" {
		t.Errorf("file %+v", b.File)
	}
	// the build is said, because a save cannot tell whether it matches; nothing else is noted for a good file
	if len(b.Notes) != 2 || !strings.Contains(b.Notes[0], "game build 2162777") || !strings.Contains(b.Notes[1], "set to none") {
		t.Errorf("notes %q", b.Notes)
	}
}

func TestConsoleFileReferencesAreInTheFormTheImporterReads(t *testing.T) {
	b := loadFixture(t, nil)
	// "!!act@2" is the transfer id of the totem, now "!!act#2", which the importer maps to the totem's new id; the null reference stays
	if got := string(cell(t, b, "actors", 0, "properties").(JSONText)); got != `{"BP_Totem_C":{"m_Other":"!!act#0","m_SelfUniqueID":"!!act#2"}}` {
		t.Errorf("totem properties %s", got)
	}
	// "!!act#2" in the file is an id of the server it came from: it named the character's body there, nothing here, so it is none
	got := string(cell(t, b, "fgl_entities", 2, "components").(JSONText))
	if !strings.Contains(got, `"ItemUniqueId":"!!itm#14"`) || !strings.Contains(got, `"InstigatorActorId":"!!act#0"`) || !strings.Contains(got, `"Spare":"!!act#0"`) {
		t.Errorf("station components %s", got)
	}
	found := false
	for _, n := range b.Notes {
		found = found || strings.Contains(n, "1 references") && strings.Contains(n, "set to none")
	}
	if !found {
		t.Errorf("notes %q do not count the far reference", b.Notes)
	}
}

func TestConsoleFileIsRefusedWhole(t *testing.T) {
	setData := func(id int, k string, v any) func(h cf.Data, e *[]cf.Entry) {
		return func(h cf.Data, e *[]cf.Entry) { cf.DataOf(*e, id)[k] = v }
	}
	drop := func(id int) func(h cf.Data, e *[]cf.Entry) {
		return func(h cf.Data, e *[]cf.Entry) {
			var out []cf.Entry
			for _, x := range *e {
				if x["id"] != id {
					out = append(out, x)
				}
			}
			*e = out
		}
	}
	add := func(x cf.Entry) func(h cf.Data, e *[]cf.Entry) {
		return func(h cf.Data, e *[]cf.Entry) { *e = append(*e, x) }
	}
	for name, c := range map[string]struct {
		edit func(h cf.Data, e *[]cf.Entry)
		want string
	}{
		"another version":                      {func(h cf.Data, e *[]cf.Entry) { h["version"] = 2 }, "version 2"},
		"an unknown top key":                   {func(h cf.Data, e *[]cf.Entry) { h["path"] = "C:/x" }, "unknown field"},
		"no time":                              {func(h cf.Data, e *[]cf.Entry) { h["exportedAt"] = "yesterday" }, "exportedAt"},
		"no placeholder":                       {func(h cf.Data, e *[]cf.Entry) { h["ownerPlaceholderTransferId"] = 0 }, "owner placeholder"},
		"placeholder elsewhere":                {func(h cf.Data, e *[]cf.Entry) { h["ownerPlaceholderTransferId"] = cf.Backup }, "not an actor entry"},
		"placeholder with data":                {setData(cf.Placeholder, "class", "/Game/X.X_C"), "carries data"},
		"no entries":                           {func(h cf.Data, e *[]cf.Entry) { *e = nil }, "no entries"},
		"a repeated id":                        {func(h cf.Data, e *[]cf.Entry) { (*e)[3]["id"] = cf.Totem }, "used twice"},
		"a zero id":                            {func(h cf.Data, e *[]cf.Entry) { (*e)[3]["id"] = 0 }, "no valid id"},
		"an unknown kind":                      {add(cf.Entry{"id": 99, "kind": "Spaceship", "data": cf.Data{}}), "does not read"},
		"tax invoices":                         {add(cf.Entry{"id": 99, "kind": "TaxInvoice", "data": cf.Data{"totem_id": cf.Totem}}), "TaxInvoice"},
		"a stored Solido":                      {add(cf.Entry{"id": 99, "kind": "bbp", "data": cf.Data{"item_id": 12}}), "Solido"},
		"sinkcharts":                           {add(cf.Entry{"id": 99, "kind": "Sinkchart", "data": cf.Data{"item_id": 12}}), "sinkcharts"},
		"an entry with no data":                {func(h cf.Data, e *[]cf.Entry) { cf.Find(*e, 17)["data"] = nil }, "has no data"},
		"an unknown field":                     {setData(cf.Chest, "extra", 1), "field (extra)"},
		"a missing field":                      {func(h cf.Data, e *[]cf.Entry) { delete(cf.DataOf(*e, cf.Chest), "serial") }, "has no serial"},
		"a standing actor":                     {setData(cf.Chest, "state", "Normal"), `"Normal"`},
		"a text for a number":                  {setData(cf.Chest, "serial", "7"), "not a whole number"},
		"a float for an int":                   {setData(cf.Chest, "serial", 7.5), "not a whole number"},
		"a bad transform":                      {setData(cf.Chest, "transform", cf.Data{"location": cf.Data{"x": 1}, "rotation": cf.Data{}}), "transform"},
		"a short piece":                        {setData(18, "transform", []float64{1, 2, 3}), "list of 7 numbers"},
		"a flag that is a number":              {setData(21, "is_hologram", 1), "not true or false"},
		"an inventory of a module":             {setData(10, "vehicle_module_id", 5), "vehicle module"},
		"an inventory of an item":              {setData(10, "item_id", 5), "belongs to an item"},
		"a name that is not the game's":        {setData(18, "building_type", "Wall; drop table"), "not one of the game's names"},
		"a reference outside":                  {setData(10, "actor_id", 4000), "not in the file"},
		"a far reference that is not an actor": {func(h cf.Data, e *[]cf.Entry) { cf.DataOf(*e, cf.EntChest)["components"] = cf.Data{"A": "!!itm#77"} }, "server the file came from"},
		"a reference in a text":                {func(h cf.Data, e *[]cf.Entry) { cf.DataOf(*e, cf.EntChest)["components"] = cf.Data{"A": "see !!act@2"} }, "inside other text"},
		"a backup of a player":                 {setData(cf.Backup, "player_id", cf.Chest), "owner placeholder"},
		"a link to another backup":             {setData(27, "id", 99), "not to the backup record"},
		"an actor not linked":                  {drop(27), "not linked"},
		"two backups":                          {add(cf.Entry{"id": 99, "kind": "BaseBackup", "data": cf.Data{"player_id": 1, "base_backup_name": "x", "last_edited_by_player_id": 0}}), "exactly one backup"},
		"an actor with no data":                {func(h cf.Data, e *[]cf.Entry) { cf.Find(*e, cf.Chest)["data"] = cf.Data{} }, "has no data"},
		"a Solido item in a chest":             {setData(12, "template_id", "BuildingBlueprint_CopyDevice"), "Solido"},
	} {
		t.Run(name, func(t *testing.T) { refusedConsoleFile(t, c.edit, c.want) })
	}
}

func TestConsoleFileLimitsAndShape(t *testing.T) {
	data := cf.File(nil)
	if _, err := Load(data, len(data)-1, MaxRows); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("size cap: %v", err)
	}
	if _, err := Load(data, MaxFileBytes, 10); err == nil || !strings.Contains(err.Error(), "more than 10") {
		t.Errorf("row cap: %v", err)
	}
	if _, err := Load(append(append([]byte{}, data...), []byte(` {"a":1}`)...), MaxFileBytes, MaxRows); err == nil || !strings.Contains(err.Error(), "more after") {
		t.Errorf("trailing: %v", err)
	}
	if _, err := Load([]byte(strings.Replace(string(data), `"entries"`, `"entries": 5, "entries"`, 1)), MaxFileBytes, MaxRows); err == nil {
		t.Error("an entries that is not a list was accepted")
	}
	// the file's own counts are only said, not trusted
	b := loadFixture(t, func(h cf.Data, e *[]cf.Entry) {
		h["source"].(cf.Data)["counts"] = cf.Data{"items": 3, "pieces": 800, "placeables": 3}
	})
	found := false
	for _, n := range b.Notes {
		found = found || strings.Contains(n, "says it holds 800 building pieces; it holds 3")
	}
	if !found {
		t.Errorf("notes %q", b.Notes)
	}
	// a file of the editor is still the editor's
	if _, err := Load([]byte(`{"format":"something-else"}`), MaxFileBytes, MaxRows); err == nil || !strings.Contains(err.Error(), "not a base file") {
		t.Errorf("another format: %v", err)
	}
}

func TestConsoleFileDropsWhatASaveHasNoColumnFor(t *testing.T) {
	b := loadFixture(t, func(h cf.Data, e *[]cf.Entry) {
		d := cf.DataOf(*e, cf.Chest)
		d["owner_account_id"] = 123456789 // an account of the server
		d["partition_id"] = 4
		cf.DataOf(*e, 10)["exchange_id"] = nil
		cf.DataOf(*e, cf.Backup)["id"] = cf.Backup
		cf.DataOf(*e, 18)["__lb"] = []int{0}
	})
	for _, c := range b.Of("actors").Table.Cols {
		if c.Name == "owner_account_id" || c.Name == "partition_id" {
			t.Errorf("column %s is in the table", c.Name)
		}
	}
	// ranks are mapped: the player is the placeholder or an actor of the file, the importer gives them to the character
	b = loadFixture(t, func(h cf.Data, e *[]cf.Entry) {
		*e = append(*e, cf.Entry{"id": 99, "kind": "PermissionActorRank", "data": cf.Data{"permission_actor_id": cf.Chest, "player_id": cf.Placeholder, "rank": 2}})
	})
	if cell(t, b, "permission_actor_rank", 0, "rank") != int64(2) || cell(t, b, "permission_actor_rank", 0, "player_id") != int64(cf.Placeholder) {
		t.Errorf("rank %v", b.Of("permission_actor_rank").R)
	}
}

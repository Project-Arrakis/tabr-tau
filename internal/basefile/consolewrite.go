package basefile

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"
)

// Writing the self-hosted Dune console's own base backup file (#534): format "dune-base-backup", version 1, the inverse of console.go. The rows of a base, as Tables lists them,
// become entries[] of {id, kind, data}: every id is a transfer id (small, from 1, handed out here), a column that names another row holds the transfer id of that row, a string
// inside a JSON column that names an id is written "!!act@12" (the null reference "!!act@0"; no "#" form is ever left), and the owner is a placeholder entry that carries no data. The shape of each kind is the one the reader in console.go
// takes (which is the shape of a real export of console v1.4.52); a file this writes is read back by it, and the caller is expected to do that before it hands the file out.
//
// Nothing here knows a save: the caller reads the rows and decides which belong to the file (a base that is still standing needs different rows from one that was picked up).

// ConsoleHeader is what the caller says about the base and the save, for the header of the file. None of it is private: no player name, no account, no path.
type ConsoleHeader struct {
	At        time.Time
	Live      bool   // the base was still standing when it was exported (source.kind "live-base"); false for a base that was picked up
	BaseID    int64  // live: the totem's id in the save
	BackupID  int64  // picked up: the backup's id in the save
	Name      string // what the base is called (shown in the console's list)
	RawName   string // the name the save holds
	Map       string
	TotemType string
	Patches   Patches // the save's applied patches; the checksum is left out on purpose (the game's own checksum cannot be made from a save)
	Editor    string  // the version of this program, for the file's own record of who wrote it
}

// ConsoleResult is a finished file and what it holds, counted from the entries written.
type ConsoleResult struct {
	Data       []byte
	Actors     int // actors, the owner placeholder not counted
	Pieces     int
	Placeables int
	Items      int
	Entities   int
	Entries    int // all entries, the placeholder counted
	FarActors  int // references "!!act#N" to an actor that is not in the file (a queued job's instigator); written as the null reference "!!act@0"
	NulledRefs int // references of another kind to a row that is not in the file; written as the null reference too
}

// WriteConsole turns the rows into the console's file. tabs holds, for each table of Tables, the rows with the cells in the order of Table.Cols (int64, float64, string, nil; a JSON
// cell is its text). base_backups holds exactly one row and base_backup_linked_actors one per actor. A rank row's player is written as the owner placeholder whatever it is, so the
// caller keeps only the owner's rows. It refuses a file over maxBytes or maxEntries, a number that is not finite, text that is not UTF-8 and JSON that is not valid.
func WriteConsole(h ConsoleHeader, tabs map[string][][]any, maxBytes, maxEntries int) (*ConsoleResult, error) {
	w := &cwriter{tabs: map[string]*ctab{}, ids: map[string]map[int64]int64{"act": {}, "fgl": {}, "inv": {}, "itm": {}}}
	for _, t := range Tables {
		ct := &ctab{t: t, rows: tabs[t.Name], col: map[string]int{}}
		for i, c := range t.Cols {
			ct.col[c.Name] = i
		}
		w.tabs[t.Name] = ct
	}
	if n := len(w.tabs["base_backups"].rows); n != 1 {
		return nil, fmt.Errorf("a base file holds one base backup; this one holds %d", n)
	}
	// Transfer ids, in the order the entries are written. The placeholder is 1.
	w.next = 2
	w.order = []struct {
		kind, table string
	}{{"act", "actors"}, {"fgl", "fgl_entities"}, {"PermissionActor", "permission_actor"}, {"PermissionActorRank", "permission_actor_rank"}, {"inv", "inventories"}, {"itm", "items"},
		{"ActorInventory", "actor_inventories"}, {"Building", "buildings"}, {"BuildingInstance", "building_instances"}, {"Placeable", "placeables"}, {"Totem", "totems"},
		{"BaseBackup", "base_backups"}, {"BaseBackupLinkedActor", "base_backup_linked_actors"}, {"LandclaimSegment", "landclaim_segments"}}
	total := 1
	for _, o := range w.order {
		total += len(w.tabs[o.table].rows)
	}
	if total > maxEntries {
		return nil, fmt.Errorf("the base would make %d entries; the most a file holds is %d", total, maxEntries)
	}
	w.tid = map[string][]int64{}
	keyed := map[string]string{"actors": "id", "fgl_entities": "entity_id", "inventories": "id", "items": "id"}
	for _, o := range w.order {
		ct := w.tabs[o.table]
		ids := make([]int64, len(ct.rows))
		for i, r := range ct.rows {
			ids[i] = w.next
			w.next++
			if k, ok := keyed[o.table]; ok {
				n, isInt := r[ct.col[k]].(int64)
				if !isInt {
					return nil, fmt.Errorf("%s: a row has no %s", o.table, k)
				}
				m := w.ids[o.kind]
				if _, dup := m[n]; dup {
					return nil, fmt.Errorf("%s: the id %d is used by two rows", o.table, n)
				}
				m[n] = ids[i]
			}
		}
		w.tid[o.table] = ids
	}

	var entries bytes.Buffer
	n := 0
	emit := func(id int64, kind string, data []byte) {
		if n > 0 {
			entries.WriteString(",\n")
		}
		n++
		entries.WriteString(`    {"id":` + strconv.FormatInt(id, 10) + `,"kind":` + strconv.Quote(kind) + `,"data":`)
		entries.Write(data)
		entries.WriteString("}")
	}
	emit(1, "act", []byte("{}"))
	for _, o := range w.order {
		ct := w.tabs[o.table]
		for i, r := range ct.rows {
			data, err := w.entry(o.kind, ct, r)
			if err != nil {
				return nil, fmt.Errorf("%s, row %d: %w", o.table, i+1, err)
			}
			emit(w.tid[o.table][i], o.kind, data)
		}
	}
	res := &ConsoleResult{Actors: len(w.tabs["actors"].rows), Pieces: len(w.tabs["building_instances"].rows), Placeables: len(w.tabs["placeables"].rows),
		Items: len(w.tabs["items"].rows), Entities: len(w.tabs["fgl_entities"].rows), Entries: n, FarActors: w.far, NulledRefs: w.nulled}

	head, err := w.header(h, res)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.Write(head)
	out.WriteString("  \"entries\": [\n")
	out.Write(entries.Bytes())
	out.WriteString("\n  ]\n}\n")
	if out.Len() > maxBytes {
		return nil, fmt.Errorf("the file would be %d bytes; the most a base file holds is %s", out.Len(), sizeWords(maxBytes))
	}
	res.Data = out.Bytes()
	return res, nil
}

type ctab struct {
	t    Table
	rows [][]any
	col  map[string]int
}

type cwriter struct {
	tabs  map[string]*ctab
	order []struct{ kind, table string }
	next  int64
	tid   map[string][]int64         // table -> the transfer id of each row, in row order
	ids   map[string]map[int64]int64 // reference kind -> id in the save -> transfer id

	links       map[int64][]any // entity id in the save -> its row of actor_fgl_entities
	far, nulled int
}

// get is the cell of a row, nil for NULL.
func (c *ctab) get(r []any, name string) any {
	i, ok := c.col[name]
	if !ok {
		panic("basefile: no column " + name + " in " + c.t.Name)
	}
	return r[i]
}

// cell is the JSON text of a column's value, the same writing as the editor's own file.
func (c *ctab) cell(r []any, name string) ([]byte, error) {
	return c.t.Cols[c.col[name]].Value(r[c.col[name]])
}

// flag writes a 0/1 column as true or false.
func (c *ctab) flag(r []any, name string) ([]byte, error) {
	switch v := c.get(r, name).(type) {
	case nil:
		return []byte("null"), nil
	case int64:
		if v == 0 {
			return []byte("false"), nil
		}
		if v == 1 {
			return []byte("true"), nil
		}
	}
	return nil, fmt.Errorf("%s holds a value that is not 0 or 1", name)
}

type jobj struct {
	b   bytes.Buffer
	n   int
	err error
}

func newObj() *jobj {
	o := &jobj{}
	o.b.WriteByte('{')
	return o
}

func (o *jobj) put(key string, v []byte, err error) {
	if o.err != nil {
		return
	}
	if err != nil {
		o.err = err
		return
	}
	if o.n > 0 {
		o.b.WriteByte(',')
	}
	o.n++
	o.b.WriteString(strconv.Quote(key))
	o.b.WriteByte(':')
	o.b.Write(v)
}

func (o *jobj) done() ([]byte, error) {
	if o.err != nil {
		return nil, o.err
	}
	o.b.WriteByte('}')
	return o.b.Bytes(), nil
}

func itoaBytes(n int64) []byte { return strconv.AppendInt(nil, n, 10) }

// refs writes a JSON column's text with every reference "!!act#12" in the one form the console takes, a transfer id after an @ (#538): a reference that names a row of the file is
// "!!act@T"; the null reference "!!act#0" is "!!act@0", the way the console's own export writes a missing reference; and a reference to a row that is not in the file (a crafting
// queue that names the character who queued it, an actor of the single-player save that is no part of the base) is also "!!act@0", no reference. It is not the owner placeholder: the
// console's import turns the placeholder into the importing player's controller, while a queue names the character's pawn, so the placeholder would name the wrong actor; the null
// reference is what the console's own export writes for a reference it may leave outside the base. No "!!kind#N" is ever left in the text; the strict check (CheckConsoleStrict)
// makes sure of it before a file is handed out.
func (w *cwriter) refs(text string) (string, error) {
	return RewriteRefs(text, func(r Ref) (string, error) {
		s := "!!" + r.Kind
		switch {
		case r.ID < 0:
			return "", fmt.Errorf("a reference %s#%d is not an id", s, r.ID)
		case r.ID == 0:
			return s + "@0", nil
		}
		if m := w.ids[r.Kind]; m != nil {
			if t, ok := m[r.ID]; ok {
				return s + "@" + strconv.FormatInt(t, 10), nil
			}
		}
		if r.Kind == "act" {
			w.far++
		} else {
			w.nulled++
		}
		return s + "@0", nil
	})
}

// jsonCell is a JSON column with its references rewritten, as an object (the reader takes nothing else).
func (w *cwriter) jsonCell(c *ctab, r []any, name string) ([]byte, error) {
	v, err := c.cell(r, name)
	if err != nil {
		return nil, err
	}
	if len(v) == 0 || v[0] != '{' {
		return nil, fmt.Errorf("%s is not a JSON object", name)
	}
	out, err := w.refs(string(v))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return []byte(out), nil
}

// target is the transfer id of the row of kind whose id in the save is the value of the column.
func (w *cwriter) target(kind string, c *ctab, r []any, name string, nullOK bool) ([]byte, error) {
	v := c.get(r, name)
	if v == nil && nullOK {
		return []byte("null"), nil
	}
	n, ok := v.(int64)
	if !ok {
		return nil, fmt.Errorf("%s holds no id", name)
	}
	t, ok := w.ids[kind][n]
	if !ok {
		return nil, fmt.Errorf("%s names the %s %d, which is not in the file", name, kind, n)
	}
	return itoaBytes(t), nil
}

func group(names []string, vals [][]byte) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Quote(n) + ":")
		b.Write(vals[i])
	}
	b.WriteByte('}')
	return b.Bytes()
}

func list(vals [][]byte) []byte {
	return append(append([]byte{'['}, bytes.Join(vals, []byte(","))...), ']')
}

// entry is the data of one entry.
func (w *cwriter) entry(kind string, c *ctab, r []any) ([]byte, error) {
	o := newObj()
	switch kind {
	case "act":
		cl, e1 := c.cell(r, "class")
		mp, e2 := c.cell(r, "map")
		o.put("class", cl, e1)
		o.put("map", mp, e2)
		o.put("state", []byte(`"BaseBackup"`), nil)
		v, err := c.cell(r, "serial")
		o.put("serial", v, err)
		pr, err := w.jsonCell(c, r, "properties")
		o.put("properties", pr, err)
		gas, err := w.jsonCell(c, r, "gas_attributes")
		o.put("gas_attributes", gas, err)
		v, err = c.cell(r, "dimension_index")
		o.put("dimension_index", v, err)
		o.put("owner_account_id", []byte("null"), nil)
		var loc, rot [][]byte
		for _, k := range []string{"location_x", "location_y", "location_z"} {
			v, err := c.cell(r, k)
			if err != nil {
				o.put("transform", nil, err)
			}
			loc = append(loc, v)
		}
		for _, k := range []string{"rotation_x", "rotation_y", "rotation_z", "rotation_w"} {
			v, err := c.cell(r, k)
			if err != nil {
				o.put("transform", nil, err)
			}
			rot = append(rot, v)
		}
		o.put("transform", group([]string{"location", "rotation"}, [][]byte{group([]string{"x", "y", "z"}, loc), group([]string{"x", "y", "z", "w"}, rot)}), nil)
	case "fgl":
		a, err := w.target("act", w.tabs["actor_fgl_entities"], w.linkOf(c, r), "actor_id", false)
		o.put("actor_id", a, err)
		s, err := w.tabs["actor_fgl_entities"].cell(w.linkOf(c, r), "slot_name")
		o.put("slot_name", s, err)
		j, err := w.jsonCell(c, r, "components")
		o.put("components", j, err)
	case "PermissionActor":
		a, err := w.target("act", c, r, "actor_id", false)
		o.put("actor_id", a, err)
		for _, k := range []string{"actor_name", "actor_type", "access_level"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
		f, err := c.flag(r, "is_child")
		o.put("is_child", f, err)
		v, err := c.cell(r, "edited_by_player_id")
		o.put("edited_by_player_id", v, err)
	case "PermissionActorRank":
		a, err := w.target("act", c, r, "permission_actor_id", false)
		o.put("permission_actor_id", a, err)
		o.put("player_id", []byte("1"), nil)
		v, err := c.cell(r, "rank")
		o.put("rank", v, err)
	case "inv":
		a, err := w.target("act", c, r, "actor_id", false)
		o.put("actor_id", a, err)
		for _, k := range []string{"inventory_type", "max_item_count", "max_item_volume"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
		o.put("vehicle_module_id", []byte("null"), nil)
	case "itm":
		inv, err := w.target("inv", c, r, "inventory_id", false)
		o.put("inventory_id", inv, err)
		for _, k := range []string{"stack_size", "position_index", "template_id"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
		f, err := c.flag(r, "is_new")
		o.put("is_new", f, err)
		for _, k := range []string{"acquisition_time"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
		st, err := w.jsonCell(c, r, "stats")
		o.put("stats", st, err)
		for _, k := range []string{"quality_level", "volume_override"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
	case "ActorInventory":
		inv, err := w.target("inv", c, r, "inventory_id", false)
		o.put("inventory_id", inv, err)
		v, err := c.cell(r, "component_name_hash")
		o.put("component_name_hash", v, err)
	case "Building":
		a, err := w.target("act", c, r, "id", false)
		o.put("id", a, err)
		o.put("owner_id", []byte("null"), nil)
	case "BuildingInstance":
		b, err := w.target("act", c, r, "building_id", false)
		o.put("building_id", b, err)
		for _, k := range []string{"instance_id", "building_type"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
		e, err := w.target("fgl", c, r, "owner_entity_id", true)
		o.put("owner_entity_id", e, err)
		for _, k := range []string{"building_flags", "health", "shelter", "sand_buildup", "last_placed_by_player_id"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
		var t [][]byte
		for _, k := range []string{"location_x", "location_y", "location_z", "rotation_x", "rotation_y", "rotation_z", "rotation_w"} {
			v, err := c.cell(r, k)
			if err != nil {
				o.put("transform", nil, err)
			}
			t = append(t, v)
		}
		o.put("transform", list(t), nil)
		// the game writes its arrays 0-based and the console records that per row; without the note the importing server stores 1-based arrays (the game reads some of them by index)
		o.put("__lb", []byte(`{"transform":0}`), nil)
	case "Placeable":
		a, err := w.target("act", c, r, "id", false)
		o.put("id", a, err)
		e, err := w.target("fgl", c, r, "owner_entity_id", true)
		o.put("owner_entity_id", e, err)
		for _, k := range []string{"health", "building_type"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
		for _, k := range []string{"has_hit_ground", "has_buildable_support", "is_hologram"} {
			f, err := c.flag(r, k)
			o.put(k, f, err)
		}
		v, err := c.cell(r, "last_placed_by_player_id")
		o.put("last_placed_by_player_id", v, err)
	case "Totem":
		a, err := w.target("act", c, r, "id", false)
		o.put("id", a, err)
		for _, k := range []string{"landclaim_vertical_level", "last_backup_timestamp", "landclaim_original_global_yaw_rotation"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
		var loc [][]byte
		null := 0
		for _, k := range []string{"landclaim_original_global_location_x", "landclaim_original_global_location_y", "landclaim_original_global_location_z"} {
			v, err := c.cell(r, k)
			if err != nil {
				o.put("landclaim_original_global_location", nil, err)
			}
			if string(v) == "null" {
				null++
			}
			loc = append(loc, v)
		}
		switch null {
		case 3:
			o.put("landclaim_original_global_location", []byte("null"), nil)
		case 0:
			o.put("landclaim_original_global_location", list(loc), nil)
			o.put("__lb", []byte(`{"landclaim_original_global_location":0}`), nil)
		default:
			o.put("landclaim_original_global_location", nil, errors.New("the totem's original location is partly empty"))
		}
	case "BaseBackup":
		o.put("player_id", []byte("1"), nil)
		nm, err := c.cell(r, "base_backup_name")
		o.put("base_backup_name", nm, err)
		v, err := c.cell(r, "last_edited_by_player_id")
		o.put("last_edited_by_player_id", v, err)
	case "BaseBackupLinkedActor":
		o.put("id", itoaBytes(w.tid["base_backups"][0]), nil)
		a, err := w.target("act", c, r, "actor_id", false)
		o.put("actor_id", a, err)
	case "LandclaimSegment":
		a, err := w.target("act", c, r, "totem_id", false)
		o.put("totem_id", a, err)
		for _, k := range []string{"grid_location_x", "grid_location_y"} {
			v, err := c.cell(r, k)
			o.put(k, v, err)
		}
	}
	return o.done()
}

// linkOf is the row of actor_fgl_entities that links the entity of an fgl_entities row to its actor.
func (w *cwriter) linkOf(c *ctab, r []any) []any {
	l := w.tabs["actor_fgl_entities"]
	e, _ := c.get(r, "entity_id").(int64)
	if w.links == nil {
		w.links = map[int64][]any{}
		for _, lr := range l.rows {
			if x, ok := l.get(lr, "entity_id").(int64); ok {
				w.links[x] = lr
			}
		}
	}
	if lr, ok := w.links[e]; ok {
		return lr
	}
	// no link: a row of nothing, so that the caller's lookups say what is missing
	return make([]any, len(l.t.Cols))
}

func (w *cwriter) header(h ConsoleHeader, res *ConsoleResult) ([]byte, error) {
	str := func(s string) []byte {
		b, err := jsonString(s, "header")
		if err != nil {
			return []byte(`""`)
		}
		return b
	}
	for _, s := range []string{h.Name, h.RawName, h.Map, h.TotemType, h.Editor} {
		if !utf8.ValidString(s) {
			return nil, errors.New("a name in the header is not valid UTF-8")
		}
	}
	src := newObj()
	if h.Live {
		src.put("kind", str("live-base"), nil)
		src.put("baseId", itoaBytes(h.BaseID), nil)
		src.put("backupId", []byte("null"), nil)
	} else {
		src.put("backupId", itoaBytes(h.BackupID), nil)
	}
	src.put("name", str(h.Name), nil)
	src.put("rawName", str(h.RawName), nil)
	src.put("map", str(h.Map), nil)
	src.put("totemType", str(h.TotemType), nil)
	src.put("ownerName", []byte(`""`), nil)
	src.put("counts", group([]string{"pieces", "placeables", "items"}, [][]byte{itoaBytes(int64(res.Pieces)), itoaBytes(int64(res.Placeables)), itoaBytes(int64(res.Items))}), nil)
	sb, err := src.done()
	if err != nil {
		return nil, err
	}
	latest := make([][]byte, 0, len(h.Patches.Latest))
	for _, p := range h.Patches.Latest {
		latest = append(latest, str(p))
	}
	game := group([]string{"build", "steamBuildId", "patchesChecksum", "latestPatches", "appliedPatchesCount"},
		[][]byte{[]byte(`""`), []byte("null"), []byte(`""`), list(latest), itoaBytes(int64(h.Patches.Count))})
	console := group([]string{"version", "buildId", "writtenBy"}, [][]byte{[]byte(`""`), []byte(`""`), str("tabr-tau " + h.Editor)})
	var b bytes.Buffer
	b.WriteString("{\n  \"format\": " + strconv.Quote(ConsoleFormat) + ",\n  \"version\": " + strconv.Itoa(ConsoleVersion) + ",\n")
	b.WriteString("  \"exportedAt\": " + strconv.Quote(h.At.UTC().Format("2006-01-02T15:04:05.000Z")) + ",\n")
	b.WriteString("  \"source\": " + string(sb) + ",\n  \"game\": " + string(game) + ",\n  \"console\": " + string(console) + ",\n")
	b.WriteString("  \"ownerPlaceholderTransferId\": 1,\n")
	return b.Bytes(), nil
}

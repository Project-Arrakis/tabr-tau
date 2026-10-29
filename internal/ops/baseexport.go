package ops

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
)

// Export of a base backup as a file (#475, phase 1). A base backup is what the game's own tool leaves when a base is picked up: a row in base_backups, a row in
// base_backup_linked_actors for each actor of the base, and the actors themselves in state 5 with everything that hangs from them (pieces, placeables, entities, storage). The
// buildings stay where they were in the tables; only the claim goes. This reads the save and writes nothing, in the save or anywhere else: the caller gets the file's bytes.

const (
	pickedUpState         = 5 // actors.state of an actor of a base the game's tool picked up
	solidoTemplate        = "buildingblueprint_copydevice"
	noBackupMessage       = "This save has no base backup. Pick a base up with the game's base tool, let the game save, and load the file again."
	changedWhileExporting = "The save changed while the file was being made, so nothing was exported. Try again."
)

// BaseBackups lists the save's base backups with the numbers the export file will hold: the same counts, from the same rows.
func (o *Ops) BaseBackups() (any, error) {
	a := `(select actor_id from base_backup_linked_actors where id=b.id)`
	v := `(select id from inventories where actor_id in ` + a + `)`
	rows, err := o.S.Query(`select b.id backup_id, coalesce(b.base_backup_name,'') backup_name,
		(select count(*) from base_backup_linked_actors l where l.id=b.id) actors,
		(select count(*) from base_backup_linked_actors l join actors x on x.id=l.actor_id where l.id=b.id and x.state=` + strconv.Itoa(pickedUpState) + `) picked_up,
		(select count(*) from building_instances where building_id in ` + a + `) pieces,
		(select count(*) from placeables where id in ` + a + `) placeables,
		(select count(*) from items where inventory_id in ` + v + `) stored_items,
		coalesce((select tp.building_type from totems t join placeables tp on tp.id=t.id where t.id in ` + a + ` order by t.id limit 1),'') totem_type,
		coalesce((select x.map from actors x where x.id in ` + a + ` order by x.id limit 1),'') map
		from base_backups b order by b.id`)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		tt, _ := r["totem_type"].(string)
		name, _ := r["backup_name"].(string)
		if name == "" || name == "None" || strings.HasPrefix(name, "##") {
			name = totemTypes[strings.ToLower(tt)]
			if name == "" {
				name = "Base backup"
			}
		}
		r["name"] = name
		delete(r, "totem_type")
	}
	return rows, nil
}

// BaseExport is a finished file and what to call it.
type BaseExport struct {
	Data     []byte
	FileName string
	Summary  basefile.Summary
}

var fileNameBad = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// exportFileName is the name offered for the file: fixed words, the backup's number and its name with everything but letters, digits, dot, dash and underscore taken out.
func exportFileName(id int64, name string) string {
	n := strings.Trim(fileNameBad.ReplaceAllString(name, "_"), "._-")
	if r := []rune(n); len(r) > 40 {
		n = string(r[:40])
	}
	if n == "" {
		n = "base"
	}
	return "tabr-tau-base-backup-" + strconv.FormatInt(id, 10) + "-" + n + ".json"
}

// ExportBaseBackup makes the base file of the backup with this id. It reads the working copy of the save (the rows as the editor shows them) and changes nothing. at is the
// time written into the file and editor the version of this program; with the same save, the same at and the same editor the bytes are the same.
//
// It refuses, with the reason and without a partial file: no such backup, a backup with no actors or with an actor that is not in the save, a base whose storage holds a Solido
// (its blueprint rows are not carried yet), an inventory that belongs to an item or a vehicle module, a number that is not finite, JSON that is not valid, more rows than a file
// may hold, a file over the size limit, and a save that was edited while the file was being made.
func (o *Ops) ExportBaseBackup(id int64, at time.Time, editor string) (*BaseExport, error) {
	opsAtStart := len(o.S.Ops())
	head, err := o.S.One(`select id, coalesce(base_backup_name,'') name from base_backups where id=?`, id)
	if err != nil {
		return nil, err
	}
	if head == nil {
		if n, _ := o.S.One(`select count(*) n from base_backups`); n != nil && n["n"] == int64(0) {
			return nil, apierr.New(apierr.ErrNotFound, noBackupMessage)
		}
		return nil, apierr.Newf(apierr.ErrNotFound, "There is no base backup %d in this save.", id)
	}
	name, _ := head["name"].(string)
	bid := strconv.FormatInt(id, 10)
	linked := `(select actor_id from base_backup_linked_actors where id=` + bid + `)`
	if err := o.refuseBaseBackup(bid, linked); err != nil {
		return nil, err
	}

	got := make([]exportedTable, len(basefile.Tables))
	total := 0
	for i, t := range basefile.Tables {
		// One more row than a file may hold, so that a base over the limit is seen without reading all of it.
		_, rows, err := o.S.Table(`select ` + t.SelectList() + ` from ` + t.Name + ` where ` + t.ScopeSQL(id) + ` order by ` + t.Order + ` limit ` + strconv.Itoa(exportRowCap+1))
		if err != nil {
			return nil, err
		}
		if total += len(rows); total > exportRowCap {
			return nil, apierr.Newf(apierr.ErrBadArgument, "This base backup holds more than %d rows, more than a base file may hold. Nothing was exported.", exportRowCap)
		}
		got[i] = exportedTable{t, rows}
	}

	// The file, table by table, one row to a line.
	var tables bytes.Buffer
	tables.WriteString("{\n")
	for i, c := range got {
		tables.WriteString("    " + strconv.Quote(c.t.Name) + ": [")
		for j, r := range c.rows {
			line, err := c.t.Row(r)
			if err != nil {
				return nil, apierr.Newf(apierr.ErrBadArgument, "The base backup cannot be exported: %v. Nothing was exported.", err)
			}
			if j > 0 {
				tables.WriteString(",")
			}
			tables.WriteString("\n      ")
			tables.Write(line)
		}
		if len(c.rows) > 0 {
			tables.WriteString("\n    ")
		}
		tables.WriteString("]")
		if i < len(got)-1 {
			tables.WriteString(",")
		}
		tables.WriteString("\n")
	}
	tables.WriteString("  }")

	m, err := o.baseManifest(id, name, got)
	if err != nil {
		return nil, err
	}
	data, err := basefile.Assemble(basefile.KindBackup, at, editor, m, tables.Bytes(), exportByteCap)
	if err != nil {
		return nil, apierr.Newf(apierr.ErrBadArgument, "The base backup cannot be exported: %v. Nothing was exported.", err)
	}
	if len(o.S.Ops()) != opsAtStart {
		return nil, apierr.New(apierr.ErrConflict, changedWhileExporting)
	}
	return &BaseExport{Data: data, FileName: exportFileName(id, name), Summary: m.Summary}, nil
}

// refuseBaseBackup is the checks that need the save and not the rows: the backup must have actors, all of them in the save, and the base must hold nothing the file cannot carry.
func (o *Ops) refuseBaseBackup(bid, linked string) error {
	r, err := o.S.One(`select (select count(*) from base_backup_linked_actors where id=` + bid + `) linked,
		(select count(*) from base_backup_linked_actors l left join actors a on a.id=l.actor_id where l.id=` + bid + ` and a.id is null) missing,
		(select count(*) from items where lower(template_id)='` + solidoTemplate + `' and inventory_id in (select id from inventories where actor_id in ` + linked + `)) solidos,
		(select count(*) from inventories where actor_id in ` + linked + ` and (item_id is not null or vehicle_module_id is not null)) odd`)
	if err != nil {
		return err
	}
	n := func(k string) int64 { v, _ := toInt(r[k]); return v }
	switch {
	case n("linked") == 0:
		return apierr.New(apierr.ErrBadArgument, "This base backup has no actors linked to it, so there is nothing to export.")
	case n("missing") > 0:
		return apierr.Newf(apierr.ErrBadArgument, "This base backup names %d actors that are not in the save, so it is damaged and was not exported.", n("missing"))
	case n("solidos") > 0:
		return apierr.New(apierr.ErrBadArgument, "A storage of this base holds a Solido blueprint, and the blueprint's rows are not carried in a base file yet, so the base was not exported.")
	case n("odd") > 0:
		return apierr.New(apierr.ErrBadArgument, "An inventory of this base belongs to an item or a vehicle, which a base file does not carry, so the base was not exported.")
	}
	return nil
}

// refPattern is a reference the game writes inside JSON: "!!act#12" (an actor), "!!itm#7" (an item), and so on. #0 is the null reference.
var refPattern = regexp.MustCompile(`^!!([a-z]{3})#(-?[0-9]+)$`)

// scanRefs calls fn for each string value in a JSON text that is a reference. It reads numbers as text so that no id is rounded.
func scanRefs(text string, fn func(kind string, id int64)) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) != nil {
		return
	}
	var walk func(x any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			if m := refPattern.FindStringSubmatch(t); m != nil {
				if n, err := strconv.ParseInt(m[2], 10, 64); err == nil {
					fn(m[1], n)
				}
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(v)
}

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// exportedTable is a table of the file as read from the save: the rows in the order of the table's columns.
type exportedTable struct {
	t    basefile.Table
	rows [][]any
}

func (e exportedTable) col(name string) int {
	for i, c := range e.t.Cols {
		if c.Name == name {
			return i
		}
	}
	panic("basefile: no column " + name + " in " + e.t.Name)
}

// int64s are the values of one integer column, skipping NULL.
func (e exportedTable) int64s(name string) []int64 {
	i := e.col(name)
	var out []int64
	for _, r := range e.rows {
		if n, ok := r[i].(int64); ok {
			out = append(out, n)
		}
	}
	return out
}

func idSet(ids []int64) map[int64]bool {
	m := make(map[int64]bool, len(ids))
	for _, n := range ids {
		m[n] = true
	}
	return m
}

// baseManifest works out the manifest from the rows that are going into the file, so it can only say what the file holds. The tables are in basefile.Tables order. The checksum is
// filled in when the file is assembled.
func (o *Ops) baseManifest(id int64, name string, tabs []exportedTable) (basefile.Manifest, error) {
	by := map[string]exportedTable{}
	m := basefile.Manifest{Backup: basefile.BackupInfo{ID: id, Name: name}, Counts: map[string]int{}, Warnings: []string{}, References: []basefile.Reference{}}
	for _, t := range tabs {
		by[t.t.Name] = t
		m.Counts[t.t.Name] = len(t.rows)
		m.Summary.Rows += len(t.rows)
	}
	m.Summary.Actors, m.Summary.Pieces, m.Summary.Placeables = len(by["actors"].rows), len(by["building_instances"].rows), len(by["placeables"].rows)
	m.Summary.StoredItems, m.Summary.Entities = len(by["items"].rows), len(by["fgl_entities"].rows)

	// What is known of the save the file came from, none of it private.
	actors := by["actors"]
	maps, mi, si := map[string]bool{}, actors.col("map"), actors.col("state")
	m.Source.ActorStates = map[string]int{}
	notPicked := 0
	for _, r := range actors.rows {
		if s, ok := r[mi].(string); ok && s != "" {
			maps[s] = true
		}
		st, _ := r[si].(int64)
		m.Source.ActorStates[strconv.FormatInt(st, 10)]++
		if st != pickedUpState {
			notPicked++
		}
	}
	m.Source.Maps = make([]string, 0, len(maps))
	for s := range maps {
		m.Source.Maps = append(m.Source.Maps, s)
	}
	sort.Strings(m.Source.Maps)
	patches, err := o.S.Query(`select name from applied_patches order by date, name`)
	if err != nil {
		return m, err
	}
	names := make([]string, 0, len(patches))
	for _, p := range patches {
		if s, ok := p["name"].(string); ok {
			names = append(names, s)
		}
	}
	m.Source.Patches = basefile.Patches{Count: len(names), Latest: append([]string{}, names[max(0, len(names)-5):]...)}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	m.Source.Patches.Checksum = sha256hex([]byte(strings.Join(sorted, "\n")))

	// References: the strings the game writes inside JSON ("!!act#12") and the owner entity of the pieces and placeables, counted as inside the file, outside it, or null.
	inside := map[string]map[int64]bool{
		"act": idSet(actors.int64s("id")), "itm": idSet(by["items"].int64s("id")), "inv": idSet(by["inventories"].int64s("id")), "fgl": idSet(by["fgl_entities"].int64s("entity_id")),
	}
	refs := map[string]*basefile.Reference{}
	count := func(kind string, n int64, in bool) {
		r := refs[kind]
		if r == nil {
			r = &basefile.Reference{Kind: kind}
			refs[kind] = r
		}
		switch {
		case n == 0:
			r.Null++
		case in:
			r.Inside++
		default:
			r.Outside++
		}
	}
	for _, t := range tabs {
		for ci, c := range t.t.Cols {
			if c.Kind != basefile.JSON {
				continue
			}
			for _, r := range t.rows {
				if s, ok := r[ci].(string); ok {
					scanRefs(s, func(kind string, n int64) { count(kind, n, inside[kind][n]) })
				}
			}
		}
	}
	for _, tn := range []string{"building_instances", "placeables"} {
		t := by[tn]
		ci := t.col("owner_entity_id")
		for _, r := range t.rows {
			n, ok := r[ci].(int64)
			if !ok {
				n = 0
			}
			count("ownerEntity", n, inside["fgl"][n])
		}
	}
	kinds := make([]string, 0, len(refs))
	for k := range refs {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		m.References = append(m.References, *refs[k])
	}

	// What the save holds for these items that the file does not carry: an inventory that belongs to a stored item (a bag in a chest).
	stored := `(select id from items where inventory_id in (select id from inventories where actor_id in ` +
		`(select actor_id from base_backup_linked_actors where id=` + strconv.FormatInt(id, 10) + `)))`
	nc, err := o.S.One(`select (select count(*) from inventories where item_id in ` + stored + `) invs,
		(select count(*) from items where inventory_id in (select id from inventories where item_id in ` + stored + `)) held`)
	if err != nil {
		return m, err
	}
	invs, _ := toInt(nc["invs"])
	held, _ := toInt(nc["held"])
	m.NotCarried = basefile.NotCarried{InventoriesOfStoredItems: int(invs), ItemsInThem: int(held)}

	if notPicked > 0 {
		m.Warnings = append(m.Warnings, fmt.Sprintf("%d of %d actors are not in the picked-up state (5): the base may have been put down again after it was backed up.", notPicked, len(actors.rows)))
	}
	for _, k := range kinds {
		if r := refs[k]; r.Outside > 0 {
			what := map[string]string{"act": "an actor", "itm": "an item", "inv": "an inventory", "fgl": "an entity", "bbp": "a blueprint", "ownerEntity": "an owner entity"}[k]
			if what == "" {
				what = "something ('" + k + "')"
			}
			m.Warnings = append(m.Warnings, fmt.Sprintf("%d references name %s that is not in the file.", r.Outside, what))
		}
	}
	if held > 0 {
		m.Warnings = append(m.Warnings, fmt.Sprintf("%d items are in inventories that belong to stored items (a bag, say); those inventories and the items in them are not in the file.", held))
	}
	return m, nil
}

// The limits of a file. They are variables only so that a test can lower them and reach the refusal in milliseconds, instead of building a base of 250,000 rows or 16 MiB; nothing
// else changes them.
var (
	exportRowCap  = basefile.MaxRows
	exportByteCap = basefile.MaxFileBytes
)

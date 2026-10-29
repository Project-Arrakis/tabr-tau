package ops

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Import of a base file as a base backup (#475, phase 2). The file (internal/basefile) is the rows of a base that was taken out of a save. Importing adds them to this save as a NEW
// base backup (state 5, the game's picked-up state) that the game's Base Reconstruction Tool can list. The pieces are not put in the world: that is what the game's tool does with a
// backup. Nothing already in the save is changed or removed, and nothing is replaced, so the edit needs no question; importing the same file twice adds two backups.
//
// The rules, from docs/research/2026-10-28-base-export-import-from-console.md:
//   - an id from the file is never used as an id in the save. Every actor, inventory, item, entity and the backup get a new one (the allocators the editor already has), and every
//     column that names one is written with the new one;
//   - the ids written as strings inside JSON ("!!act#12", "!!itm#7") are rewritten the same way. An actor reference to something that is not in the file (a queued recipe names the
//     character who started it) becomes the null reference; any other reference that is not in the file refuses the file. After the rewrite the JSON is read back and must hold
//     exactly the references that were written;
//   - every player id (the owner of the backup, the rank rows, who placed a piece, who edited a permission record) becomes the save's own character. The file's account column is
//     not in the file, and the actors get none;
//   - every actor is written in state 5; the buildings' owner is left empty, as the game leaves it when it picks a base up;
//   - one edit, in one transaction. Whatever is wrong refuses the whole file and the save is as it was; a preview is the same code and the edit is rolled back.

// The limits of a file. They are variables only so that a test can lower them and reach the refusal in milliseconds, instead of building a file of 250,000 rows or 16 MiB; nothing
// else changes them.
var (
	importRowCap  = basefile.MaxRows
	importByteCap = basefile.MaxFileBytes
)

// errBaseImportDry ends a preview's edit so that it is rolled back.
var errBaseImportDry = errors.New("base import dry run")

// maxNewID keeps the ids this import gives well inside the numbers a web page reads exactly (2^53).
const maxNewID = int64(1) << 53

// PreviewBaseImport works out what importing the file would do, by doing it in the one transaction an import is and rolling it back. It writes nothing, queues nothing and leaves the
// save as it was. The answer says what would be added (rows per table, the ranges of the new ids), whose the base would become, what was rewritten, and warnings.
func (o *Ops) PreviewBaseImport(data []byte) (any, error) { return o.importBase(data, true) }

// ImportBaseBackup adds the base of the file to the save as a new base backup. It is one edit (written after a backup, or queued while a game process exists, as every edit is by the
// web layer's post) and it refuses the whole file, with the reason and nothing written, in the cases Load, Verify and the checks below name.
func (o *Ops) ImportBaseBackup(data []byte) (any, error) { return o.importBase(data, false) }

func refusedImport(err error) error {
	return apierr.Newf(apierr.ErrBadArgument, "This file cannot be imported: %v. Nothing was written.", err)
}

func (o *Ops) importBase(data []byte, dry bool) (any, error) {
	b, err := basefile.Load(data, importByteCap, importRowCap)
	if err != nil {
		return nil, refusedImport(err)
	}
	if err := b.Verify(); err != nil {
		return nil, refusedImport(err)
	}
	p, err := o.player()
	if err != nil || p.Controller <= 0 {
		return nil, apierr.New(apierr.ErrNotFound, "This save has no character to own the base, so nothing was imported.")
	}
	var res map[string]any
	_, err = o.S.Mutate("Import base backup", func(m *save.Mut) error {
		r, desc, err := importBaseTx(m, b, p, dry)
		if err != nil {
			return err
		}
		res = r
		m.Desc = desc
		if dry {
			return errBaseImportDry
		}
		return nil
	})
	if dry && errors.Is(err, errBaseImportDry) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ids are the file's ids of one kind in ascending order.
func sortedIDs(rs *basefile.Rows, col string) []int64 {
	ci := rs.Col(col)
	out := make([]int64, 0, len(rs.R))
	for _, r := range rs.R {
		out = append(out, r[ci].(int64))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// renumber gives each id a place in a run that starts at first.
func renumber(ids []int64, first int64) map[int64]int64 {
	m := make(map[int64]int64, len(ids))
	for i, id := range ids {
		m[id] = first + int64(i)
	}
	return m
}

// reserveItemIDs gives n item ids from the game's own counter and moves the counter past them, as the give does: above the largest item and above both counters.
func reserveItemIDs(m *save.Mut, n int) (int64, error) {
	r, err := m.One(`select coalesce(max(id),0) m, coalesce((select seq from sqlite_sequence where name='items'),0) s from items`)
	if err != nil || r == nil {
		return 0, errors.New("could not read the item id counter")
	}
	mx, ok1 := toInt(r["m"])
	sq, ok2 := toInt(r["s"])
	if !ok1 || !ok2 {
		return 0, errors.New("could not read the item id counter")
	}
	next := max(mx, sq) + 1
	seq, err := m.Query(`select next_id from items_id_sequencer`)
	if err != nil {
		return 0, err
	}
	if len(seq) > 0 {
		// a counter that holds no number (a save built by hand) is rebuilt from the highest id, and written back as a number below
		if v, isNum := seq[0]["next_id"].(int64); isNum && v > next {
			next = v
		}
		if n > 0 {
			if _, err := m.Exec(`update items_id_sequencer set next_id=?`, next+int64(n)); err != nil {
				return 0, err
			}
		}
	} else if n > 0 {
		if _, err := m.Exec(`insert into items_id_sequencer(next_id) values(?)`, next+int64(n)); err != nil {
			return 0, err
		}
	}
	return next, nil
}

func idRange(first int64, n int) map[string]any {
	if n == 0 {
		return map[string]any{"count": 0}
	}
	return map[string]any{"first": first, "last": first + int64(n) - 1, "count": n}
}

// jsonCell remembers a JSON cell that was rewritten, so it can be read back from the save after it is written.
type jsonCell struct {
	table, expr, where string
	key                int64
	want               []string
}

func importBaseTx(m *save.Mut, b *basefile.Base, p player, dry bool) (map[string]any, string, error) {
	ctl := p.Controller
	if r, err := m.One(`select id from actors where id=?`, ctl); err != nil {
		return nil, "", err
	} else if r == nil {
		return nil, "", apierr.New(apierr.ErrNotFound, "The character of this save is not among its actors, so the base has no owner to be given to. Nothing was imported.")
	}
	T := b.Rows
	actorsT, entsT, invsT, itemsT := T["actors"], T["fgl_entities"], T["inventories"], T["items"]

	// ---- new ids: none of the file's ids is used in the save
	oldActors := sortedIDs(actorsT, "id")
	actorFirst, err := nextID(m, "actors")
	if err != nil {
		return nil, "", err
	}
	invFirst, err := nextID(m, "inventories")
	if err != nil {
		return nil, "", err
	}
	backupFirst, err := nextID(m, "base_backups")
	if err != nil {
		return nil, "", err
	}
	itemFirst, err := reserveItemIDs(m, len(itemsT.R))
	if err != nil {
		return nil, "", err
	}
	for _, c := range []struct {
		name  string
		first int64
		n     int
	}{{"actors", actorFirst, len(actorsT.R)}, {"inventories", invFirst, len(invsT.R)}, {"items", itemFirst, len(itemsT.R)}, {"base backups", backupFirst, 1}} {
		if c.first+int64(c.n) >= maxNewID {
			return nil, "", apierr.Newf(apierr.ErrBadArgument, "This save has no room left for new %s ids, so nothing was imported.", c.name)
		}
	}
	actor := renumber(oldActors, actorFirst)
	inv := renumber(sortedIDs(invsT, "id"), invFirst)
	item := renumber(sortedIDs(itemsT, "id"), itemFirst)
	taken := map[int64]bool{}
	all, err := m.Query(`select entity_id e from fgl_entities`)
	if err != nil {
		return nil, "", err
	}
	for _, r := range all {
		if e, ok := toInt(r["e"]); ok {
			taken[e] = true
		}
	}
	oldEnts := sortedIDs(entsT, "entity_id")
	newEnts, err := newEntityIDs(oldEnts, taken)
	if err != nil {
		return nil, "", apierr.Newf(apierr.ErrBadArgument, "The entity ids of the base cannot be moved into this save: %v. Nothing was imported.", err)
	}
	ent := make(map[int64]int64, len(oldEnts))
	for i, id := range oldEnts {
		ent[id] = newEnts[i]
	}

	// ---- the ids inside JSON
	var warns []string
	refKinds := map[string]int{}
	toNone, nullStayed := 0, 0
	var cells []jsonCell
	valid := map[string]map[int64]bool{"act": {}, "itm": {}, "inv": {}, "fgl": {}}
	for _, n := range actor {
		valid["act"][n] = true
	}
	for _, n := range item {
		valid["itm"][n] = true
	}
	for _, n := range inv {
		valid["inv"][n] = true
	}
	for _, n := range ent {
		valid["fgl"][n] = true
	}
	mapRef := map[string]map[int64]int64{"act": actor, "itm": item, "inv": inv, "fgl": ent}
	rewrite := func(j basefile.JSONText, where string) (basefile.JSONText, []string, error) {
		var expect []string
		out, err := basefile.RewriteRefs(string(j), func(r basefile.Ref) (string, error) {
			s := "!!" + r.Kind + "#"
			switch {
			case r.ID == 0:
				nullStayed++
				s += "0"
			case r.ID < 0:
				return "", fmt.Errorf("%s holds the reference %s%d, which is not an id", where, s, r.ID)
			case mapRef[r.Kind] == nil:
				return "", fmt.Errorf("%s holds a reference of a kind (%s) that this editor does not move", where, r.Kind)
			default:
				if n, ok := mapRef[r.Kind][r.ID]; ok {
					refKinds[r.Kind]++
					s += strconv.FormatInt(n, 10)
				} else if r.Kind == "act" {
					toNone++ // the character in a crafting queue, say: not in the file, and not this save's actor
					s += "0"
				} else {
					return "", fmt.Errorf("%s names %s%d, which is not in the file", where, s, r.ID)
				}
			}
			expect = append(expect, s)
			return s, nil
		})
		if err != nil {
			return "", nil, refusedImport(err)
		}
		// the rewritten text must hold exactly the references that were written, and each is the null reference or a new id of its kind
		found, err := basefile.FindRefs(out)
		if err != nil || strings.Join(found, "\n") != strings.Join(expect, "\n") {
			return "", nil, refusedImport(fmt.Errorf("%s still holds an id that was not rewritten", where))
		}
		for _, f := range found {
			kind, num, _ := strings.Cut(strings.TrimPrefix(f, "!!"), "#")
			n, _ := strconv.ParseInt(num, 10, 64)
			if n != 0 && !valid[kind][n] {
				return "", nil, refusedImport(fmt.Errorf("%s holds the id %s that is not one of the new ids", where, f))
			}
		}
		return basefile.JSONText(out), expect, nil
	}
	// jsonOf rewrites the JSON cell at index ci of a row. A cell with no reference is left as it is.
	jsonOf := func(r []any, ci int, table, expr, where string, key int64, label string) error {
		j, ok := r[ci].(basefile.JSONText)
		if !ok {
			return nil
		}
		out, expect, err := rewrite(j, label)
		if err != nil {
			return err
		}
		r[ci] = out
		if len(expect) > 0 {
			cells = append(cells, jsonCell{table, expr, where, key, expect})
		}
		return nil
	}
	cp := func(r []any) []any { return append([]any(nil), r...) }
	lookup := func(mp map[int64]int64, id int64, what string) (int64, error) {
		n, ok := mp[id]
		if !ok {
			return 0, fmt.Errorf("%s %d is not in the file", what, id) // Verify has seen to it; this is the importer not trusting that
		}
		return n, nil
	}

	// ---- the rows, with new ids, the save's own character and state 5
	out := map[string][][]any{}
	backup := backupFirst
	owners := 0
	notBackedUp := 0
	{
		rs := T["base_backups"]
		for _, r := range rs.R {
			n := cp(r)
			n[rs.Col("id")] = backup
			n[rs.Col("player_id")] = ctl
			if v, _ := r[rs.Col("last_edited_by_player_id")].(int64); v != 0 {
				n[rs.Col("last_edited_by_player_id")] = ctl
			}
			out["base_backups"] = append(out["base_backups"], n)
		}
	}
	{
		rs := T["base_backup_linked_actors"]
		for _, r := range rs.R {
			n := cp(r)
			n[rs.Col("id")] = backup
			if n[rs.Col("actor_id")], err = lookup(actor, r[rs.Col("actor_id")].(int64), "actor"); err != nil {
				return nil, "", err
			}
			out["base_backup_linked_actors"] = append(out["base_backup_linked_actors"], n)
		}
	}
	{
		idc, st, gas, props := actorsT.Col("id"), actorsT.Col("state"), actorsT.Col("gas_attributes"), actorsT.Col("properties")
		for _, r := range actorsT.R {
			old := r[idc].(int64)
			n := cp(r)
			n[idc] = actor[old]
			if r[st].(int64) != basefile.StateBackup {
				notBackedUp++
			}
			n[st] = int64(basefile.StateBackup)
			for _, c := range []int{gas, props} {
				if err := jsonOf(n, c, "actors", "json("+actorsT.Table.Cols[c].Name+")", "id=?", actor[old], fmt.Sprintf("actor %d, %s", old, actorsT.Table.Cols[c].Name)); err != nil {
					return nil, "", err
				}
			}
			out["actors"] = append(out["actors"], n)
		}
	}
	{
		ec, cc := entsT.Col("entity_id"), entsT.Col("components")
		for _, r := range entsT.R {
			old := r[ec].(int64)
			n := cp(r)
			n[ec] = ent[old]
			if err := jsonOf(n, cc, "fgl_entities", "json(components)", "entity_id=?", ent[old], fmt.Sprintf("entity %d, components", old)); err != nil {
				return nil, "", err
			}
			out["fgl_entities"] = append(out["fgl_entities"], n)
		}
	}
	{
		rs := T["actor_fgl_entities"]
		for _, r := range rs.R {
			n := cp(r)
			if n[0], err = lookup(actor, r[rs.Col("actor_id")].(int64), "actor"); err != nil {
				return nil, "", err
			}
			if n[1], err = lookup(ent, r[rs.Col("entity_id")].(int64), "entity"); err != nil {
				return nil, "", err
			}
			out["actor_fgl_entities"] = append(out["actor_fgl_entities"], n)
		}
	}
	{
		rs := T["permission_actor"]
		ac, ed := rs.Col("actor_id"), rs.Col("edited_by_player_id")
		for _, r := range rs.R {
			n := cp(r)
			if n[ac], err = lookup(actor, r[ac].(int64), "actor"); err != nil {
				return nil, "", err
			}
			if v, ok := r[ed].(int64); ok && v != 0 {
				n[ed] = ctl
				owners++
			}
			out["permission_actor"] = append(out["permission_actor"], n)
		}
	}
	collapsed := 0
	{
		// A rank row says who holds a rank over a permission record. Every player of the file becomes the one character of this save, so rows of one record that differ only in the
		// player become one row, with the lowest rank number any of them had.
		rs := T["permission_actor_rank"]
		pa, rk := rs.Col("permission_actor_id"), rs.Col("rank")
		best := map[int64]int64{}
		for _, r := range rs.R {
			a, err := lookup(actor, r[pa].(int64), "actor")
			if err != nil {
				return nil, "", err
			}
			v := r[rk].(int64)
			if cur, ok := best[a]; !ok {
				best[a] = v
			} else {
				collapsed++
				best[a] = min(cur, v)
			}
		}
		keys := make([]int64, 0, len(best))
		for a := range best {
			keys = append(keys, a)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		for _, a := range keys {
			out["permission_actor_rank"] = append(out["permission_actor_rank"], []any{a, ctl, best[a]})
			owners++
		}
		if collapsed > 0 {
			warns = append(warns, fmt.Sprintf("%d rank rows named other players than the character of this save; they were merged into one row each, with the lowest rank number.", collapsed))
		}
	}
	{
		ic, ac := invsT.Col("id"), invsT.Col("actor_id")
		for _, r := range invsT.R {
			n := cp(r)
			n[ic] = inv[r[ic].(int64)]
			if n[ac], err = lookup(actor, r[ac].(int64), "actor"); err != nil {
				return nil, "", err
			}
			out["inventories"] = append(out["inventories"], n)
		}
	}
	unknown := map[string]bool{}
	{
		ic, vc, st := itemsT.Col("id"), itemsT.Col("inventory_id"), itemsT.Col("stats")
		tc := itemsT.Col("template_id")
		for _, r := range itemsT.R {
			old := r[ic].(int64)
			n := cp(r)
			n[ic] = item[old]
			if n[vc], err = lookup(inv, r[vc].(int64), "inventory"); err != nil {
				return nil, "", err
			}
			if err := jsonOf(n, st, "items", "stats", "id=?", item[old], fmt.Sprintf("item %d, stats", old)); err != nil {
				return nil, "", err
			}
			if _, ok := catalog.Name(r[tc].(string)); !ok {
				unknown[r[tc].(string)] = true
			}
			out["items"] = append(out["items"], n)
		}
	}
	{
		rs := T["actor_inventories"]
		for _, r := range rs.R {
			n := cp(r)
			if n[0], err = lookup(inv, r[0].(int64), "inventory"); err != nil {
				return nil, "", err
			}
			out["actor_inventories"] = append(out["actor_inventories"], n)
		}
	}
	{
		rs := T["buildings"]
		for _, r := range rs.R {
			n := cp(r)
			if n[0], err = lookup(actor, r[0].(int64), "actor"); err != nil {
				return nil, "", err
			}
			n[rs.Col("owner_id")] = nil // the game leaves it empty when it picks a base up
			out["buildings"] = append(out["buildings"], n)
		}
	}
	entOf := func(r []any, ci int) (any, error) {
		v, ok := r[ci].(int64)
		if !ok || v == 0 {
			return nil, nil
		}
		return lookup(ent, v, "entity")
	}
	{
		rs := T["building_instances"]
		bc, oc, lp := rs.Col("building_id"), rs.Col("owner_entity_id"), rs.Col("last_placed_by_player_id")
		for _, r := range rs.R {
			n := cp(r)
			if n[bc], err = lookup(actor, r[bc].(int64), "building"); err != nil {
				return nil, "", err
			}
			if n[oc], err = entOf(r, oc); err != nil {
				return nil, "", err
			}
			if v, _ := r[lp].(int64); v != 0 {
				n[lp] = ctl
				owners++
			}
			out["building_instances"] = append(out["building_instances"], n)
		}
	}
	{
		rs := T["placeables"]
		ic, oc, lp := rs.Col("id"), rs.Col("owner_entity_id"), rs.Col("last_placed_by_player_id")
		for _, r := range rs.R {
			n := cp(r)
			if n[ic], err = lookup(actor, r[ic].(int64), "actor"); err != nil {
				return nil, "", err
			}
			if n[oc], err = entOf(r, oc); err != nil {
				return nil, "", err
			}
			if v, _ := r[lp].(int64); v != 0 {
				n[lp] = ctl
				owners++
			}
			out["placeables"] = append(out["placeables"], n)
		}
	}
	for _, name := range []string{"totems", "landclaim_segments"} {
		rs := T[name]
		for _, r := range rs.R {
			n := cp(r)
			if n[0], err = lookup(actor, r[0].(int64), "actor"); err != nil {
				return nil, "", err
			}
			out[name] = append(out[name], n)
		}
	}

	// ---- write, in the order of the file's tables
	counts := map[string]int{}
	total := 0
	for _, t := range basefile.Tables {
		sql := t.InsertSQL()
		for _, r := range out[t.Name] {
			args := make([]any, len(r))
			for i, v := range r {
				args[i] = basefile.Arg(v)
			}
			if _, err := m.Exec(sql, args...); err != nil {
				return nil, "", fmt.Errorf("could not add a row to %s: %w", t.Name, err)
			}
		}
		counts[t.Name] = len(out[t.Name])
		total += len(out[t.Name])
	}
	// ---- read the rewritten JSON back from the save: it holds exactly the references that were written
	for _, c := range cells {
		r, err := m.One(`select `+c.expr+` v from `+c.table+` where `+c.where, c.key)
		if err != nil {
			return nil, "", err
		}
		text, _ := r["v"].(string)
		found, ferr := basefile.FindRefs(text)
		if r == nil || ferr != nil || strings.Join(found, "\n") != strings.Join(c.want, "\n") {
			return nil, "", fmt.Errorf("the references written to %s %d could not be read back as written, so nothing was imported", c.table, c.key)
		}
	}

	// ---- what to say about it
	sum := basefile.Summary{Actors: len(actorsT.R), Pieces: len(T["building_instances"].R), Placeables: len(T["placeables"].R), StoredItems: len(itemsT.R), Entities: len(entsT.R), Rows: total}
	totemType := ""
	{
		tot := map[int64]bool{}
		for _, r := range T["totems"].R {
			tot[r[0].(int64)] = true
		}
		pl := T["placeables"]
		for _, r := range pl.R {
			if id := r[pl.Col("id")].(int64); tot[id] {
				totemType, _ = r[pl.Col("building_type")].(string)
				break
			}
		}
		if len(tot) == 0 {
			warns = append(warns, "The file has no totem, so the backup carries no land claim.")
		}
	}
	name := ""
	if r := T["base_backups"].R; len(r) > 0 {
		name, _ = r[0][T["base_backups"].Col("base_backup_name")].(string)
	}
	display := backupDisplayName(name, totemType)

	if notBackedUp > 0 {
		warns = append(warns, fmt.Sprintf("%d of %d actors in the file were not in the picked-up state (they were standing); all are written as picked up (state 5).", notBackedUp, len(actorsT.R)))
	}
	if toNone > 0 {
		warns = append(warns, fmt.Sprintf("%d references to an actor that is not in the file (a crafting queue naming the character who started it, for example) were set to none.", toNone))
	}
	if len(unknown) > 0 {
		names := make([]string, 0, len(unknown))
		for k := range unknown {
			names = append(names, k)
		}
		sort.Strings(names)
		more := ""
		if len(names) > 5 {
			names, more = names[:5], fmt.Sprintf(" and %d more", len(names)-5)
		}
		warns = append(warns, fmt.Sprintf("%d kinds of stored item are not in the editor's item catalog (%s%s). They are imported as they are.", len(unknown), strings.Join(names, ", "), more))
	}
	if maps := fileMaps(actorsT); len(maps) > 0 {
		for _, mp := range maps {
			if mp != "HaggaBasin" {
				warns = append(warns, fmt.Sprintf("The base is on the map %s. The game redeploys a base only on the map it was picked up on, and a Deep Desert layout can differ between saves; the editor has not been able to check this in the game.", mp))
				break
			}
		}
	}
	patches, err := m.Query(`select name from applied_patches`)
	if err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(patches))
	for _, r := range patches {
		if s, ok := r["name"].(string); ok {
			names = append(names, s)
		}
	}
	if src := b.File.Manifest.Source.Patches; src.Checksum != "" && src.Checksum != patchesChecksum(names) {
		warns = append(warns, fmt.Sprintf("The save the file came from had %d game patches applied; this save has %d, and they are not the same set. The game may have changed how it stores a base; the editor cannot say whether the game loads it.", src.Count, len(names)))
	}
	if n := noRoleActors(T); n > 0 {
		warns = append(warns, fmt.Sprintf("%d actors in the file are neither a building nor a placeable. They are imported as they are.", n))
	}
	for i := range warns {
		warns[i] = strings.TrimSpace(warns[i])
	}

	res := map[string]any{
		"ok": true, "dryRun": dry, "changed": true, "name": display, "state": basefile.StateBackup,
		"summary": sum, "counts": counts,
		"ids": map[string]any{
			"backup": idRange(backupFirst, 1), "actors": idRange(actorFirst, len(actorsT.R)), "inventories": idRange(invFirst, len(invsT.R)), "items": idRange(itemFirst, len(itemsT.R)),
			"entities": entityRange(newEnts),
		},
		"owner":      map[string]any{"controller": ctl, "character": p.Name, "rows": owners},
		"references": map[string]any{"rewritten": refKinds, "toNone": toNone, "nullKept": nullStayed},
		"warnings":   warns,
		"fileHash":   b.Hash,
		"file": map[string]any{"format": b.File.Format, "version": b.File.Version, "kind": b.File.Kind, "exportedAt": b.File.ExportedAt, "editor": b.File.Editor,
			"backupId": strconv.FormatInt(b.File.Manifest.Backup.ID, 10)},
	}
	desc := fmt.Sprintf("Import base backup \"%s\": %d pieces, %d placeables, %d stored items", importLabel(display), sum.Pieces, sum.Placeables, sum.StoredItems)
	return res, desc, nil
}

// entityRange says the lowest and highest new entity id as text: an entity id is a 64-bit number, more than a web page holds without rounding.
func entityRange(ids []int64) map[string]any {
	if len(ids) == 0 {
		return map[string]any{"count": 0}
	}
	lo, hi := ids[0], ids[0]
	for _, id := range ids {
		lo, hi = min(lo, id), max(hi, id)
	}
	return map[string]any{"first": strconv.FormatInt(lo, 10), "last": strconv.FormatInt(hi, 10), "count": len(ids)}
}

func fileMaps(actors *basefile.Rows) []string {
	mc := actors.Col("map")
	seen := map[string]bool{}
	for _, r := range actors.R {
		if s, ok := r[mc].(string); ok {
			seen[s] = true
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// noRoleActors counts the actors of the file that are in neither buildings nor placeables.
func noRoleActors(T map[string]*basefile.Rows) int {
	role := map[int64]bool{}
	for _, name := range []string{"buildings", "placeables"} {
		for _, r := range T[name].R {
			role[r[0].(int64)] = true
		}
	}
	n := 0
	for _, a := range sortedIDs(T["actors"], "id") {
		if !role[a] {
			n++
		}
	}
	return n
}

// importLabel is a name for the list of edits: the name from the file with anything but plain characters taken out, and short.
func importLabel(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && r != 0x7f && r != '"' {
			b.WriteRune(r)
		}
	}
	out := []rune(b.String())
	if len(out) > 60 {
		out = out[:60]
	}
	return string(out)
}

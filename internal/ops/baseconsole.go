package ops

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
)

// Export of a base as the Dune console's own base backup file (#534): format "dune-base-backup", version 1, which the console's base backup import reads. Two sources:
//
//   - a base that is still standing (a totem that is placed), written as the file a pick-up followed by the console's export would give: every actor in the backed-up state,
//     the building actors as the fresh copies a pick-up makes, only the pieces the totem owns, none of the totem's own permission rows, and a backup record built from the totem;
//   - a base the game's tool already picked up, written from its backup record as it is.
//
// Both only read the save and write nothing, in the save or anywhere else. The file is made by basefile.WriteConsole and then read back by the editor's own reader for console
// files (basefile.Load and Verify), and the counts must agree, before it is handed out.

const (
	consolePickedUp = "This base has been picked up with the game's base tool. Export it from Base backups instead."
	noOwnerMessage  = "This base has no owner (nobody holds rank 1 over it), so it cannot be saved as a base backup."
)

// consoleFileName is the name offered for a console file: fixed words, what it is of, its number and the base's name with everything but letters, digits, dot, dash and underscore
// taken out.
func consoleFileName(what string, id int64, name string) string {
	n := exportFileName(id, name)
	// exportFileName gives "tabr-tau-base-backup-<id>-<name>.json"; keep its cleaning of the name and change the words around it
	prefix := "tabr-tau-base-backup-" + strconv.FormatInt(id, 10) + "-"
	return "tabr-tau-console-" + what + "-" + strconv.FormatInt(id, 10) + "-" + n[len(prefix):]
}

// ExportStandingBaseConsole writes the base with this totem as a console file. It reads the working copy of the save and changes nothing. With the same save, the same at and the
// same editor the bytes are the same.
//
// It refuses, with the reason and without a partial file: no such base, a base the game's tool already picked up, a base with no owner, a base whose totem has no entity (the
// pieces cannot be told from another base's), a base that holds a Solido or an inventory that belongs to an item, rows the file cannot hold (a number that is not finite, JSON that
// is not an object), a base over the limits of a file, a file the editor's own reader would not take back, and a save that was edited while the file was being made.
func (o *Ops) ExportStandingBaseConsole(totem int64, at time.Time, editor string) (*BaseExport, error) {
	opsAtStart := len(o.S.Ops())
	head, err := o.S.One(`select a.state state, coalesce(a.map,'') map, coalesce(pa.actor_name,'') name, coalesce(tp.building_type,'') totem_type,
		(select entity_id from actor_fgl_entities where actor_id=t.id and slot_name='Actor' order by entity_id limit 1) ent,
		(select player_id from permission_actor_rank where permission_actor_id=t.id and rank=1 order by player_id limit 1) owner,
		exists (select 1 from base_backup_linked_actors where actor_id=t.id) linked,
		exists (select 1 from permission_actor where actor_id=t.id) has_permission
		from totems t join actors a on a.id=t.id left join permission_actor pa on pa.actor_id=t.id left join placeables tp on tp.id=t.id where t.id=?`, totem)
	if err != nil {
		return nil, err
	}
	if head == nil {
		return nil, apierr.Newf(apierr.ErrNotFound, "There is no base with the totem %d in this save.", totem)
	}
	state, _ := toInt(head["state"])
	linked, _ := toInt(head["linked"])
	hasPerm, _ := toInt(head["has_permission"])
	if state == pickedUpState || (linked == 1 && hasPerm == 0) {
		return nil, apierr.New(apierr.ErrConflict, consolePickedUp)
	}
	owner, ok := toInt(head["owner"])
	if !ok {
		return nil, apierr.New(apierr.ErrConflict, noOwnerMessage)
	}
	ent, ok := toInt(head["ent"])
	if !ok {
		return nil, apierr.New(apierr.ErrBadArgument, "The totem of this base has no entity, so its pieces cannot be told from those of another base. Nothing was exported.")
	}
	rawName, _ := head["name"].(string)
	totemType, _ := head["totem_type"].(string)
	mapName, _ := head["map"].(string)

	// The actors of the base, as the game's pick-up takes them: the totem, the placeables it owns that stand on the base, and the building actors that hold its pieces.
	tid, eid := strconv.FormatInt(totem, 10), strconv.FormatInt(ent, 10)
	set := `select ` + tid + ` union select id from placeables where owner_entity_id=` + eid + ` and has_buildable_support=1 union select building_id from building_instances where owner_entity_id=` + eid
	ids, err := o.S.Query(set)
	if err != nil {
		return nil, err
	}
	if err := o.refuseBaseContents("(" + set + ")"); err != nil {
		return nil, err
	}

	extra := map[string]string{
		"building_instances":    ` and owner_entity_id=` + eid, // a pick-up moves exactly the pieces the totem owns
		"permission_actor":      ` and actor_id<>` + tid,       // the pick-up destroys the totem's own permission rows
		"permission_actor_rank": ` and permission_actor_id<>` + tid + ` and player_id=` + strconv.FormatInt(owner, 10),
		"actor_fgl_entities":    ` and actor_id not in (select id from buildings)`, // building actors are written as fresh copies, with no entity
	}
	got, err := o.readConsoleTables(func(t basefile.Table) (string, bool) {
		switch t.Name {
		case "base_backups", "base_backup_linked_actors":
			return "", false
		}
		return t.ScopeSQLFor(set) + extra[t.Name], true
	})
	if err != nil {
		return nil, err
	}
	rows := map[string][][]any{}
	for _, c := range got {
		rows[c.t.Name] = c.rows
	}
	if len(rows["actors"]) != len(ids) {
		return nil, apierr.Newf(apierr.ErrBadArgument, "This base names %d actors that are not in the save, so it is damaged and was not exported.", len(ids)-len(rows["actors"]))
	}
	var notes []string
	actors := got[tableIndex("actors")]
	building := idSet(exportedTable{t: basefile.Tables[tableIndex("buildings")], rows: rows["buildings"]}.int64s("id"))
	si, ci, gi, pi, ii := actors.col("state"), actors.col("serial"), actors.col("gas_attributes"), actors.col("properties"), actors.col("id")
	other := 0
	for _, r := range rows["actors"] {
		if r[si] != int64(0) {
			other++
		}
		r[si] = int64(pickedUpState)
		if id, _ := r[ii].(int64); building[id] {
			// a fresh copy: class, map, transform and dimension only
			r[ci], r[gi], r[pi] = int64(0), "{}", "{}"
		}
	}
	if other > 0 {
		notes = append(notes, fmt.Sprintf("%d of %d actors were not in the normal state; they are written in the backed-up state like the rest.", other, len(rows["actors"])))
	}
	// building actors have no entity in the file; the entities of the others are the ones the links keep
	keep := idSet(exportedTable{t: basefile.Tables[tableIndex("actor_fgl_entities")], rows: rows["actor_fgl_entities"]}.int64s("entity_id"))
	var ents [][]any
	for _, r := range rows["fgl_entities"] {
		if e, _ := r[0].(int64); keep[e] {
			ents = append(ents, r)
		}
	}
	rows["fgl_entities"] = ents
	// a placeable that is owned by an entity that is not in the file has no owner there
	holo := 0
	pl := got[tableIndex("placeables")]
	oc, hc := pl.col("owner_entity_id"), pl.col("is_hologram")
	lost := 0
	for _, r := range rows["placeables"] {
		if e, ok := r[oc].(int64); ok && !keep[e] {
			r[oc] = nil
			lost++
		}
		if h, _ := r[hc].(int64); h == 1 {
			holo++
		}
	}
	if lost > 0 {
		notes = append(notes, fmt.Sprintf("%d placeables are owned by an entity that is not part of the base; they are written with no owner.", lost))
	}
	if holo > 0 {
		notes = append(notes, fmt.Sprintf("%d of the %d placeables are unfinished placements (holograms), which the Bases list does not count; the file keeps them, as the console's own export does.", holo, len(rows["placeables"])))
	}

	// The backup record is built from the totem: its permission actor's name, the owner as the placeholder, last edited by nobody. The links are the actors, by id.
	name := rawName
	if name == "" {
		name = "##" + totemType
	}
	rows["base_backups"] = [][]any{{totem, owner, name, int64(0)}}
	linkRows := make([][]any, 0, len(ids))
	var actorIDs []int64
	for _, r := range rows["actors"] {
		actorIDs = append(actorIDs, r[ii].(int64))
	}
	for _, a := range actorIDs {
		linkRows = append(linkRows, []any{totem, a})
	}
	rows["base_backup_linked_actors"] = linkRows

	nc, err := o.consoleNotCarried("(" + set + ")")
	if err != nil {
		return nil, err
	}
	notes = append(notes, nc...)
	return o.finishConsoleFile(opsAtStart, rows, basefile.ConsoleHeader{At: at, Live: true, BaseID: totem, Name: backupDisplayName(rawName, totemType), RawName: name, Map: mapName, TotemType: totemType, Editor: editor},
		consoleFileName("base", totem, backupDisplayName(rawName, totemType)), notes)
}

// ExportBaseBackupConsole writes a base backup (a base the game's tool picked up) as a console file. The rows are the backup's, as the game left them; the owner is the
// placeholder, and every actor is written in the backed-up state.
func (o *Ops) ExportBaseBackupConsole(id int64, at time.Time, editor string) (*BaseExport, error) {
	opsAtStart := len(o.S.Ops())
	head, err := o.S.One(`select id, coalesce(base_backup_name,'') name, player_id owner from base_backups where id=?`, id)
	if err != nil {
		return nil, err
	}
	if head == nil {
		if n, _ := o.S.One(`select count(*) n from base_backups`); n != nil && n["n"] == int64(0) {
			return nil, apierr.New(apierr.ErrNotFound, noBackupMessage)
		}
		return nil, apierr.Newf(apierr.ErrNotFound, "There is no base backup %d in this save.", id)
	}
	rawName, _ := head["name"].(string)
	owner, _ := toInt(head["owner"])
	bid := strconv.FormatInt(id, 10)
	linked := `(select actor_id from base_backup_linked_actors where id=` + bid + `)`
	if err := o.refuseBaseBackup(bid, linked); err != nil {
		return nil, err
	}
	extra := map[string]string{"permission_actor_rank": ` and player_id=` + strconv.FormatInt(owner, 10)}
	got, err := o.readConsoleTables(func(t basefile.Table) (string, bool) { return t.ScopeSQL(id) + extra[t.Name], true })
	if err != nil {
		return nil, err
	}
	rows := map[string][][]any{}
	for _, c := range got {
		rows[c.t.Name] = c.rows
	}
	var notes []string
	actors := got[tableIndex("actors")]
	si := actors.col("state")
	notPicked := 0
	for _, r := range rows["actors"] {
		if r[si] != int64(pickedUpState) {
			notPicked++
		}
		r[si] = int64(pickedUpState)
	}
	if notPicked > 0 {
		notes = append(notes, fmt.Sprintf("%d of %d actors are not in the picked-up state (5): the base may have been put down again after it was backed up. They are written in the backed-up state like the rest.", notPicked, len(rows["actors"])))
	}
	// entities the links do not keep (a link to an actor of another base) cannot be written
	keep := idSet(exportedTable{t: basefile.Tables[tableIndex("actor_fgl_entities")], rows: rows["actor_fgl_entities"]}.int64s("entity_id"))
	lost := 0
	known := idSet(exportedTable{t: basefile.Tables[tableIndex("fgl_entities")], rows: rows["fgl_entities"]}.int64s("entity_id"))
	for _, tn := range []string{"placeables", "building_instances"} {
		t := got[tableIndex(tn)]
		c := t.col("owner_entity_id")
		for _, r := range rows[tn] {
			if e, ok := r[c].(int64); ok && !(keep[e] && known[e]) {
				r[c] = nil
				lost++
			}
		}
	}
	if lost > 0 {
		notes = append(notes, fmt.Sprintf("%d pieces and placeables are owned by an entity that is not part of the base; they are written with no owner.", lost))
	}
	nc, err := o.consoleNotCarried(linked)
	if err != nil {
		return nil, err
	}
	notes = append(notes, nc...)
	tt, _ := o.S.One(`select coalesce((select tp.building_type from totems t join placeables tp on tp.id=t.id where t.id in ` + linked + ` order by t.id limit 1),'') totem_type,
		coalesce((select x.map from actors x where x.id in ` + linked + ` order by x.id limit 1),'') map`)
	totemType, _ := tt["totem_type"].(string)
	mapName, _ := tt["map"].(string)
	display := backupDisplayName(rawName, totemType)
	return o.finishConsoleFile(opsAtStart, rows, basefile.ConsoleHeader{At: at, BackupID: id, Name: display, RawName: rawName, Map: mapName, TotemType: totemType, Editor: editor},
		consoleFileName("backup", id, display), notes)
}

func tableIndex(name string) int {
	for i, t := range basefile.Tables {
		if t.Name == name {
			return i
		}
	}
	panic("basefile: no table " + name)
}

// readConsoleTables reads the rows of every table of the file whose scope says so, in Tables order (a table with no scope gets no rows here: the caller builds them).
func (o *Ops) readConsoleTables(scope func(t basefile.Table) (string, bool)) ([]exportedTable, error) {
	got := make([]exportedTable, len(basefile.Tables))
	total := 0
	for i, t := range basefile.Tables {
		got[i] = exportedTable{t: t}
		where, ok := scope(t)
		if !ok {
			continue
		}
		// One more row than a file may hold, so that a base over the limit is seen without reading all of it.
		_, rows, err := o.S.Table(`select ` + t.SelectList() + ` from ` + t.Name + ` where ` + where + ` order by ` + t.Order + ` limit ` + strconv.Itoa(exportRowCap+1))
		if err != nil {
			return nil, err
		}
		if total += len(rows); total > exportRowCap {
			return nil, apierr.Newf(apierr.ErrBadArgument, "This base holds more than %d rows, more than a base file may hold. Nothing was exported.", exportRowCap)
		}
		got[i].rows = rows
	}
	return got, nil
}

// refuseBaseContents is the checks of a base's contents, for a set of actors given as a select: a Solido in a storage (its rows are not carried) and an inventory that belongs to an
// item or a vehicle.
func (o *Ops) refuseBaseContents(actors string) error {
	r, err := o.S.One(`select (select count(*) from items where lower(template_id)='` + solidoTemplate + `' and inventory_id in (select id from inventories where actor_id in ` + actors + `)) solidos,
		(select count(*) from inventories where actor_id in ` + actors + ` and (item_id is not null or vehicle_module_id is not null)) odd`)
	if err != nil {
		return err
	}
	n := func(k string) int64 { v, _ := toInt(r[k]); return v }
	switch {
	case n("solidos") > 0:
		return apierr.New(apierr.ErrBadArgument, "A storage of this base holds a Solido blueprint, and the blueprint's rows are not carried in a base file yet, so the base was not exported.")
	case n("odd") > 0:
		return apierr.New(apierr.ErrBadArgument, "An inventory of this base belongs to an item or a vehicle, which a base file does not carry, so the base was not exported.")
	}
	return nil
}

// consoleNotCarried says what the save holds for the base that the file does not: the inventories that belong to a stored item (a bag in a chest) and the items in them.
func (o *Ops) consoleNotCarried(actors string) ([]string, error) {
	stored := `(select id from items where inventory_id in (select id from inventories where actor_id in ` + actors + `))`
	nc, err := o.S.One(`select (select count(*) from items where inventory_id in (select id from inventories where item_id in ` + stored + `)) held`)
	if err != nil {
		return nil, err
	}
	if held, _ := toInt(nc["held"]); held > 0 {
		return []string{fmt.Sprintf("%d items are in inventories that belong to stored items (a bag, say); those inventories and the items in them are not in the file.", held)}, nil
	}
	return nil, nil
}

// finishConsoleFile writes the file from the rows, reads it back with the editor's own reader and checks that it holds what was written.
func (o *Ops) finishConsoleFile(opsAtStart int, rows map[string][][]any, h basefile.ConsoleHeader, fileName string, notes []string) (*BaseExport, error) {
	patches, err := o.S.Query(`select name from applied_patches order by date, name`)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(patches))
	for _, p := range patches {
		if s, ok := p["name"].(string); ok {
			names = append(names, s)
		}
	}
	h.Patches = basefile.Patches{Count: len(names), Latest: append([]string{}, names[max(0, len(names)-5):]...)}
	res, err := basefile.WriteConsole(h, rows, exportByteCap, exportRowCap)
	if err != nil {
		return nil, apierr.Newf(apierr.ErrBadArgument, "The base cannot be exported as a console file: %v. Nothing was exported.", err)
	}
	// What the editor reads back must be what was written. A file its own reader refuses is not handed out.
	b, err := basefile.Load(res.Data, exportByteCap, exportRowCap)
	if err == nil {
		err = b.Verify()
	}
	if err != nil {
		return nil, apierr.Newf(apierr.ErrBadArgument, "The base cannot be exported as a console file: the editor could not read the file it made (%v). Nothing was exported.", err)
	}
	s := basefile.Summary{Actors: len(b.Of("actors").R), Pieces: len(b.Of("building_instances").R), Placeables: len(b.Of("placeables").R), StoredItems: len(b.Of("items").R),
		Entities: len(b.Of("fgl_entities").R), Rows: res.Entries}
	if s.Actors != res.Actors || s.Pieces != res.Pieces || s.Placeables != res.Placeables || s.StoredItems != res.Items || s.Entities != res.Entities {
		return nil, apierr.Newf(apierr.ErrBadArgument, "The base cannot be exported as a console file: the file read back holds %d pieces, %d placeables and %d stored items, not the %d, %d and %d that were written. Nothing was exported.",
			s.Pieces, s.Placeables, s.StoredItems, res.Pieces, res.Placeables, res.Items)
	}
	if res.FarActors > 0 {
		notes = append(notes, fmt.Sprintf("%d references name an actor that is not in the base (a crafting queue that names a character, say); they are written as the save holds them.", res.FarActors))
	}
	if res.NulledRefs > 0 {
		notes = append(notes, fmt.Sprintf("%d references name an item, inventory or entity that is not in the base; they are written as no reference.", res.NulledRefs))
	}
	if len(o.S.Ops()) != opsAtStart {
		return nil, apierr.New(apierr.ErrConflict, changedWhileExporting)
	}
	return &BaseExport{Data: res.Data, FileName: fileName, Summary: s, Notes: notes}, nil
}

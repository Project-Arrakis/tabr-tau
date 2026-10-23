// Package ops implements the player, base, vehicle, vendor and Landsraad
// operations. It is modelled on console/api/src/duneDb.js from
// dune-awakening-selfhost-docker, translated from Postgres stored procedures to
// plain SQL against the single-player SQLite save.
package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/augments"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// SQLite JSONB encodings used by journey state columns.
var (
	jsonTrue     = []byte{0x01} // true
	jsonEmptyObj = []byte{0x0c} // {}
)

const solariTemplate = "SolarisCoin"

var (
	templateRe = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,128}$`)
	nodeRe     = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,200}$`)
	identRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Ops bundles operations over one save.
type Ops struct{ S *save.Save }

// Args is a decoded JSON request body.
type Args map[string]any

func (a Args) Str(k string) string {
	switch v := a[k].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// Float reads a finite number. A request can spell NaN or Inf as text ("NaN", "Inf", "1e999"), which strconv accepts, and a
// non-finite number stored in a REAL column breaks every view that reads it back (SEC-2 of the 2026-10-15 audit).
func (a Args) Float(k string) (float64, error) {
	var f float64
	switch v := a[k].(type) {
	case float64:
		f = v
	case string:
		var err error
		if f, err = strconv.ParseFloat(strings.TrimSpace(v), 64); err != nil {
			return 0, apierr.Newf(apierr.BadArgument, "%s must be a number", k)
		}
	default:
		return 0, apierr.Newf(apierr.BadArgument, "%s must be a number", k)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, apierr.Newf(apierr.BadArgument, "%s must be an ordinary number, not NaN or infinity", k)
	}
	return f, nil
}

// maxExactInt is the largest integer a JSON number carries exactly (2^53). A larger id would address a different row than
// the one typed, and a conversion of a huge float to int64 does not fail, it wraps (SEC-3).
const maxExactInt = 1 << 53

// Int reads a whole number that is exactly representable. A fraction, or a magnitude above 2^53, is refused instead of
// being truncated or wrapped to some other value.
func (a Args) Int(k string) (int64, error) {
	f, err := a.Float(k)
	if err != nil {
		return 0, err
	}
	if f != math.Trunc(f) {
		return 0, apierr.Newf(apierr.BadArgument, "%s must be a whole number", k)
	}
	if f < -maxExactInt || f > maxExactInt {
		return 0, apierr.Newf(apierr.BadArgument, "%s is too large", k)
	}
	return int64(f), nil
}

func (a Args) IntRange(k string, lo, hi int64) (int64, error) {
	n, err := a.Int(k)
	if err != nil {
		return 0, err
	}
	if n < lo || n > hi {
		return 0, apierr.Newf(apierr.BadArgument, "%s out of range (%d..%d)", k, lo, hi)
	}
	return n, nil
}

func (a Args) Bool(k string, def bool) bool {
	if b, ok := a[k].(bool); ok {
		return b
	}
	return def
}

type player struct {
	Controller, Pawn, Account int64
	Name                      string
}

// player resolves the single character in this save.
func (o *Ops) player() (player, error) {
	r, err := o.S.One(`select ps.id controller, ps.account_id account, ps.character_name name,
		coalesce((select a.id from actors a where a.owner_account_id=ps.account_id and a.class like '%PlayerCharacter%' limit 1),0) pawn
		from player_state ps order by ps.id limit 1`)
	if err != nil {
		return player{}, err
	}
	if r == nil {
		return player{}, errors.New("no player found in this save")
	}
	return player{Controller: r["controller"].(int64), Account: r["account"].(int64), Pawn: r["pawn"].(int64), Name: fmt.Sprint(r["name"])}, nil
}

// ---------------------------------------------------------------- overview

// State is the small, cheap status the UI polls: no table scans, only what the header and the save pane need.
func (o *Ops) State() (any, error) {
	g := save.GameStateFor(o.S.Path, false)
	return map[string]any{
		"path": o.S.Path, "dirty": o.S.Dirty(), "pending": o.S.Pending(),
		"gameRunning": len(g.Processes) > 0, "gameProcesses": g.Processes, "gameCheckError": g.CheckError,
		"saveBlocked": g.Blocked, "blockReason": g.Reason, "gameMode": g.Mode,
		"readOnly": o.S.WriteBlocked(), "changedOnDisk": o.S.ChangedOnDisk(), "locationUnconfirmed": g.NeedsConfirm,
		"restored": o.S.Restored(), "journalOff": o.S.JournalOff(),
		"lastWrite": o.S.LastFlush(), "held": o.S.Held(),
	}, nil
}

func (o *Ops) Overview() (any, error) {
	tabs, err := o.S.Query(`select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name`)
	if err != nil {
		return nil, err
	}
	counts := map[string]any{}
	for _, t := range tabs {
		n := t["name"].(string)
		if !identRe.MatchString(n) { // a save-controlled name is never spliced into SQL text
			continue
		}
		r, _ := o.S.One(`select count(*) c from "` + n + `"`)
		counts[n] = r["c"]
	}
	p, _ := o.player()
	g := save.GameStateFor(o.S.Path, false)
	return map[string]any{
		"path": o.S.Path, "dirty": o.S.Dirty(), "pending": o.S.Pending(), "tables": counts,
		"player": p.Name, "gameRunning": len(g.Processes) > 0, "gameProcesses": g.Processes, "saveBlocked": g.Blocked, "blockReason": g.Reason,
		"gameMode": g.Mode, "readOnly": o.S.WriteBlocked(),
	}, nil
}

// ---------------------------------------------------------------- player

func (o *Ops) Player() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	out := map[string]any{"controllerId": p.Controller, "pawnId": p.Pawn, "accountId": p.Account, "name": p.Name}
	out["account"], _ = o.S.One(`select funcom_id, platform_name, platform_id from accounts where id=?`, p.Account)
	out["state"], _ = o.S.One(`select online_status, life_state, last_login_time, character_state from player_state where id=?`, p.Controller)
	out["actor"], _ = o.S.One(`select map, location_x x, location_y y, location_z z, dimension_index from actors where id=?`, p.Pawn)
	out["respawns"], _ = o.S.Query(`select "group", locator_name, locator_actor_id, map, last_used_timestamp from player_respawn_locations where character_id=?`, p.Controller)
	out["solari"] = o.solari(p)
	journeyTotals, _ := o.S.One(`select count(*) total, sum(complete_condition_state=?) done from journey_story_node where character_id=?`, jsonTrue, p.Controller)
	out["journey"] = journeyTotals
	return out, nil
}

func (o *Ops) solari(p player) int64 {
	r, _ := o.S.One(`select coalesce(sum(i.stack_size),0) n from items i join inventories v on v.id=i.inventory_id
		where v.actor_id=? and i.template_id=?`, p.Pawn, solariTemplate)
	if r == nil {
		return 0
	}
	n, _ := r["n"].(int64)
	return n
}

func ok() map[string]any { return map[string]any{"ok": true} }

// run is a one-statement edit through the single write path. It returns the rows the statement affected.
func (o *Ops) run(desc, q string, args ...any) (int64, error) {
	var n int64
	_, err := o.S.Mutate(desc, func(m *save.Mut) error {
		res, err := m.Exec(q, args...)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n, err
}

// ---------------------------------------------------------------- inventory

// The player's carried inventories the console shows, and its names for them (console playerAdminUtils.ts): the backpack, the worn
// character gear, the loadout (weapons and tools) and the unique-gear schematics. The character's other inventories (emotes,
// contract items, machine slots, ...) are not part of this view; the Database tab shows them.
var invTypeNames = map[int64]string{0: "Backpack", 1: "Character", 15: "Loadout", 30: "Unique schematics"}

// augmentsOf lists the augments applied to an item (its FAugmentedItemStats), each with the quality it was applied at.
func augmentsOf(stats string) []map[string]any {
	var m map[string][]json.RawMessage
	if json.Unmarshal([]byte(stats), &m) != nil {
		return nil
	}
	pair := m["FAugmentedItemStats"]
	if len(pair) < 2 {
		return nil
	}
	var d struct {
		Applied []struct {
			Name string `json:"Name"`
		} `json:"AppliedAugments"`
		Qualities []float64 `json:"AppliedAugmentQualities"`
	}
	if json.Unmarshal(pair[1], &d) != nil {
		return nil
	}
	var out []map[string]any
	for i, a := range d.Applied {
		e := map[string]any{"name": a.Name}
		if i < len(d.Qualities) {
			e["quality"] = d.Qualities[i]
		}
		out = append(out, e)
	}
	return out
}

func durability(stats string) (cur, max any) {
	var m map[string][]json.RawMessage
	if json.Unmarshal([]byte(stats), &m) != nil {
		return nil, nil
	}
	pair := m["FItemStackAndDurabilityStats"]
	if len(pair) < 2 {
		return nil, nil
	}
	var d map[string]any
	if json.Unmarshal(pair[1], &d) != nil {
		return nil, nil
	}
	return d["CurrentDurability"], d["MaxDurability"]
}

func (o *Ops) Inventory() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	items, err := o.S.Query(`select i.id, i.template_id, i.stack_size, i.quality_level, i.position_index, i.inventory_id,
		v.inventory_type, i.stats from items i join inventories v on v.id=i.inventory_id
		where v.actor_id=? and v.inventory_type in (0, 1, 15, 30) order by v.inventory_type, i.position_index`, p.Pawn)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		t, _ := it["inventory_type"].(int64)
		n, found := invTypeNames[t]
		if !found {
			n = fmt.Sprintf("Type %d", t)
		}
		it["inventory_name"] = n
		it["durability"], it["max_durability"] = durability(fmt.Sprint(it["stats"]))
		it["augments"] = augmentsOf(fmt.Sprint(it["stats"]))
		it["aug_limit"] = fitOf(fmt.Sprint(it["template_id"])).Limit
		delete(it, "stats")
	}
	invs, _ := o.S.Query(`select id, inventory_type, max_item_count, max_item_volume from inventories where actor_id=? and inventory_type in (0, 1, 15, 30) order by id`, p.Pawn)
	tmpl, _ := o.S.Query(`select distinct template_id from items order by 1`)
	names := []any{}
	for _, t := range tmpl {
		names = append(names, t["template_id"])
	}
	return map[string]any{"items": items, "inventories": invs, "templates": names}, nil
}

// giveInTx inserts an item into inventoryID, using the next free slot and the
// game's own id sequencer.
// maxAugmentGrade is the best grade of an augment item.
const maxAugmentGrade = 5

// grantGrade is the grade an item is written at. An augment item with no grade chosen gets the best one (5, with every stat roll at its
// best), so a plain Give of one is the strongest version; a grade that was chosen is kept.
func grantGrade(template string, q int64) int64 {
	if q < 1 && augments.IsStandalone(template) {
		return maxAugmentGrade
	}
	return q
}

// giveStacksInTx gives qty of an item the way the game holds it: no slot holds more than the item's stack size (the game's own item tables, see catalog.MaxStack). Partial
// stacks of the same item and grade that are already in the inventory are filled first, then new stacks are added, each in its own slot. The caller's transaction makes it
// all or nothing. An item the catalog does not know has no stack size to honor and goes in as one row, as it always did. It returns the ids of the rows it added (none when
// everything fitted on stacks that were there).
func giveStacksInTx(m *save.Mut, inventoryID int64, template string, qty, quality int64) ([]int64, error) {
	max, known := catalog.MaxStack(template)
	if !known {
		id, err := giveInTx(m, inventoryID, template, qty, quality)
		return []int64{id}, err
	}
	left := qty
	if max > 1 {
		rows, err := m.Query(`select id, stack_size from items where inventory_id=? and template_id=? and quality_level=? and stack_size<? order by position_index, id`, inventoryID, template, quality, max)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			have, _ := r["stack_size"].(int64)
			add := min(max-have, left)
			if add <= 0 {
				continue
			}
			if _, err := m.Exec(`update items set stack_size=? where id=?`, have+add, r["id"]); err != nil {
				return nil, err
			}
			left -= add
		}
	}
	var ids []int64
	for left > 0 {
		n := min(left, max)
		id, err := giveInTx(m, inventoryID, template, n, quality)
		if err != nil {
			if len(ids) > 0 && err.Error() == "inventory is full" {
				return nil, fmt.Errorf("not enough room: %s needs %d stacks of up to %s and the inventory has no more free slots", itemLabel(template), (qty+max-1)/max, commas(max))
			}
			return nil, err
		}
		ids = append(ids, id)
		left -= n
	}
	return ids, nil
}

func giveInTx(m *save.Mut, inventoryID int64, template string, qty, quality int64) (int64, error) {
	rows, err := m.Query(`select id, coalesce(max_item_count,0) cap from inventories where id=?`, inventoryID)
	if err != nil || len(rows) == 0 {
		return 0, apierr.New(apierr.NotFound, "inventory not found")
	}
	capacity := rows[0]["cap"].(int64)
	used := map[int64]bool{}
	pos, _ := m.Query(`select position_index p from items where inventory_id=?`, inventoryID)
	for _, r := range pos {
		if p, ok := r["p"].(int64); ok {
			used[p] = true
		}
	}
	if capacity > 0 && int64(len(used)) >= capacity {
		return 0, errors.New("inventory is full")
	}
	var slot int64
	for used[slot] {
		slot++
	}
	quality = grantGrade(template, quality)
	stats := `{"FCustomizationStats":[[],{}],"FItemStackAndDurabilityStats":[[],{}]}`
	if k, _ := m.Query(`select stats from items where template_id=? and stats is not null order by id limit 1`, template); len(k) > 0 {
		stats = fmt.Sprint(k[0]["stats"])
		if plain, err := augments.Stats(stats, nil); err == nil { // the copied item's augments are its own, not the new item's
			stats = plain
		}
	}
	if augments.IsStandalone(template) { // perfect stat rolls, not the ones of the item copied above
		var err error
		if stats, err = augments.StandaloneStats(stats, template); err != nil {
			return 0, err
		}
	}
	var next, maxID int64
	seq, err := m.Query(`select next_id from items_id_sequencer`)
	if err != nil {
		return 0, err
	}
	mx, err := m.Query(`select coalesce(max(id),0) m from items`)
	if err != nil || len(mx) == 0 {
		return 0, errors.New("could not read the item id counter")
	}
	maxID = mx[0]["m"].(int64)
	next = maxID + 1
	if len(seq) > 0 {
		if n := seq[0]["next_id"].(int64); n > next {
			next = n
		}
		if _, err := m.Exec(`update items_id_sequencer set next_id=?`, next+1); err != nil {
			return 0, err
		}
	} else if _, err := m.Exec(`insert into items_id_sequencer(next_id) values(?)`, next+1); err != nil {
		return 0, err
	}
	_, err = m.Exec(`insert into items(id, inventory_id, stack_size, position_index, template_id, is_new, acquisition_time, stats, quality_level)
		values(?,?,?,?,?,1,strftime('%s','now'),?,?)`, next, inventoryID, qty, slot, template, stats, quality)
	return next, err
}

// knownTemplate says whether an item template id is one the editor may give: it is in the item catalog or already appears on an item
// in this save (ids the catalog lacks, such as new game items). Anything else is a typo or a name that is not an id, and an unknown
// template written into a save is something the game may refuse to load (UX-02 of the 2026-10-15 audit: typing "Spice" gave an item
// no game has).
func (o *Ops) knownTemplate(id string) bool {
	if _, ok := catalog.Name(id); ok {
		return true
	}
	r, err := o.S.One(`select 1 x from items where template_id=? limit 1`, id)
	return err == nil && r != nil
}

func (o *Ops) requireKnownTemplate(id string) error {
	if !templateRe.MatchString(id) {
		return apierr.Newf(apierr.BadArgument, "invalid template id %q: pick an item from the list", id)
	}
	if !o.knownTemplate(id) {
		return apierr.Newf(apierr.BadArgument, "%q is not an item the editor knows (not in the item catalog and not in this save): pick an item from the list", id)
	}
	return nil
}

func (o *Ops) giveArgs(a Args) (inv int64, tmpl string, qty, q int64, err error) {
	tmpl = a.Str("template_id")
	if err = o.requireKnownTemplate(tmpl); err != nil {
		return 0, "", 0, 0, err
	}
	if qty, err = a.IntRange("quantity", 1, 1_000_000); err != nil {
		return
	}
	q = 0
	if _, has := a["quality"]; has && a.Str("quality") != "" {
		if q, err = a.IntRange("quality", 0, 5); err != nil {
			return
		}
	}
	return a.mustInt("inventory_id"), tmpl, qty, q, nil
}

func (a Args) mustInt(k string) int64 { n, _ := a.Int(k); return n }

// GiveItem adds an item to the player's backpack (or the given inventory).
func (o *Ops) GiveItem(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	inv, tmpl, qty, q, err := o.giveArgs(a)
	if err != nil {
		return nil, err
	}
	if inv == 0 {
		r, _ := o.S.One(`select id from inventories where actor_id=? and inventory_type=0 order by id limit 1`, p.Pawn)
		if r == nil {
			return nil, apierr.New(apierr.NotFound, "player backpack not found")
		}
		inv = r["id"].(int64)
	} else if r, _ := o.S.One(`select 1 x from inventories where id=? and actor_id=?`, inv, p.Pawn); r == nil {
		return nil, apierr.New(apierr.BadArgument, "that inventory does not belong to the player")
	}
	var ids []int64
	if _, err := o.S.Mutate(fmt.Sprintf("Give %s x %s (grade %d)", commas(qty), itemLabel(tmpl), grantGrade(tmpl, q)), func(m *save.Mut) (e error) { ids, e = giveStacksInTx(m, inv, tmpl, qty, q); return }); err != nil {
		return nil, err
	}
	return giveResult(ids), nil
}

// maxGiveBatch caps one Give Items queue; the console's queue is a handful of lines, and a typo cannot flood the backpack.
const maxGiveBatch = 50

// GiveItems gives a queue of items to the player's backpack in one edit, all or nothing: args "items" is a list of
// {template_id, quantity, quality}. One review entry and one undo cover the whole queue.
func (o *Ops) GiveItems(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	list, _ := a["items"].([]any)
	if len(list) == 0 {
		return nil, apierr.New(apierr.BadArgument, "the queue is empty")
	}
	if len(list) > maxGiveBatch {
		return nil, apierr.Newf(apierr.BadArgument, "at most %d items at a time", maxGiveBatch)
	}
	type line struct {
		tmpl   string
		qty, q int64
		aug    []string
		grade  int
	}
	lines := make([]line, 0, len(list))
	for i, e := range list {
		m, ok := e.(map[string]any)
		if !ok {
			return nil, apierr.Newf(apierr.BadArgument, "item %d is not an object", i+1)
		}
		_, tmpl, qty, q, err := o.giveArgs(Args(m))
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i+1, err)
		}
		ids, err := augmentIDs(Args(m))
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i+1, err)
		}
		grade, err := augmentGrade(Args(m))
		if err != nil {
			return nil, fmt.Errorf("item %d: %w", i+1, err)
		}
		if err := checkAugments(tmpl, ids); err != nil {
			return nil, fmt.Errorf("item %d: %w", i+1, err)
		}
		lines = append(lines, line{tmpl, qty, q, ids, grade})
	}
	r, _ := o.S.One(`select id from inventories where actor_id=? and inventory_type=0 order by id limit 1`, p.Pawn)
	if r == nil {
		return nil, apierr.New(apierr.NotFound, "player backpack not found")
	}
	inv := r["id"].(int64)
	ids := make([]int64, 0, len(lines))
	if _, err := o.S.Mutate(fmt.Sprintf("give %d queued items", len(lines)), func(m *save.Mut) error {
		for i, l := range lines {
			rows, e := giveStacksInTx(m, inv, l.tmpl, l.qty, l.q)
			if e != nil {
				return fmt.Errorf("item %d (%s): %w", i+1, l.tmpl, e)
			}
			for _, id := range rows {
				if len(l.aug) > 0 {
					if _, e := setAugmentsTx(m, id, l.aug, l.grade); e != nil {
						return fmt.Errorf("item %d (%s): %w", i+1, l.tmpl, e)
					}
				}
				ids = append(ids, id)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "itemIds": ids, "count": len(ids)}, nil
}

// giveResult is what a Give answers: the first row added (0 when everything went onto stacks that were there) and how many stacks were added.
func giveResult(ids []int64) map[string]any {
	var first int64
	if len(ids) > 0 {
		first = ids[0]
	}
	return map[string]any{"ok": true, "itemId": first, "stacks": len(ids)}
}

// GiveToInventory adds an item to any storage inventory (bases tab).
func (o *Ops) GiveToInventory(a Args) (any, error) {
	inv, tmpl, qty, q, err := o.giveArgs(a)
	if err != nil {
		return nil, err
	}
	if inv == 0 {
		return nil, apierr.New(apierr.BadArgument, "inventory_id is required")
	}
	if err := o.requireBaseContainer(inv); err != nil {
		return nil, err
	}
	var ids []int64
	if _, err := o.S.Mutate(fmt.Sprintf("Give %s x %s to container %d", commas(qty), itemLabel(tmpl), inv), func(m *save.Mut) (e error) { ids, e = giveStacksInTx(m, inv, tmpl, qty, q); return }); err != nil {
		return nil, err
	}
	return giveResult(ids), nil
}

func (o *Ops) SetItem(a Args) (any, error) {
	id, err := a.Int("id")
	if err != nil {
		return nil, err
	}
	type edit struct {
		col string
		val int64
	}
	var edits []edit
	var stackEdit int64
	var wantStack bool
	if _, has := a["stack_size"]; has {
		n, err := a.IntRange("stack_size", 1, 10_000_000)
		if err != nil {
			return nil, err
		}
		stackEdit, wantStack = n, true
		edits = append(edits, edit{"stack_size", n})
	}
	if _, has := a["quality"]; has {
		n, err := a.IntRange("quality", 0, 5)
		if err != nil {
			return nil, err
		}
		edits = append(edits, edit{"quality_level", n})
	}
	if len(edits) == 0 {
		return ok(), nil
	}
	if err := o.requireEditableItem(id); err != nil {
		return nil, err
	}
	_, err = o.S.Mutate(fmt.Sprintf("edit item %d", id), func(m *save.Mut) error {
		old, err := m.One(`select template_id, stack_size, quality_level from items where id=?`, id)
		if err != nil {
			return err
		}
		if old == nil {
			return apierr.New(apierr.NotFound, "item not found")
		}
		if max, known := catalog.MaxStack(fmt.Sprint(old["template_id"])); wantStack && known && stackEdit > max { // a slot holds no more than the item's stack size
			return fmt.Errorf("%s stacks to %s: a stack of %s is not possible (use Give to add stacks)", itemLabel(fmt.Sprint(old["template_id"])), commas(max), commas(stackEdit))
		}
		desc := fmt.Sprintf("%s (item %d):", itemLabel(fmt.Sprint(old["template_id"])), id)
		for i, e := range edits {
			res, err := m.Exec(`update items set `+e.col+`=? where id=?`, e.val, id)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return apierr.New(apierr.NotFound, "item not found")
			}
			was, _ := old[e.col].(int64)
			what := "stack"
			if e.col == "quality_level" {
				what = "grade"
			}
			sep := " "
			if i > 0 {
				sep = ", "
			}
			desc += fmt.Sprintf("%s%s %s", sep, what, change(was, e.val))
		}
		m.Desc = desc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ok(), nil
}

func (o *Ops) DeleteItem(a Args) (any, error) {
	id, err := a.Int("id")
	if err != nil {
		return nil, err
	}
	if err := o.requireEditableItem(id); err != nil {
		return nil, err
	}
	// Cascading delete: an item that owns an inventory takes that inventory and everything in it with it, as the
	// game's schema declares (ON DELETE CASCADE), instead of leaving them dangling.
	_, err = o.S.MutateCascade(fmt.Sprintf("delete item %d", id), func(m *save.Mut) error {
		if old, _ := m.One(`select template_id, stack_size from items where id=?`, id); old != nil {
			n, _ := old["stack_size"].(int64)
			m.Desc = fmt.Sprintf("Delete %s x %s (item %d)", commas(n), itemLabel(fmt.Sprint(old["template_id"])), id)
		}
		res, err := m.Exec(`delete from items where id=?`, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apierr.New(apierr.NotFound, "item not found")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ok(), nil
}

// gearDurability is the part of an item's FItemStackAndDurabilityStats the repair decides on. Absent or non-numeric values stay at their zero
// value with their has flag false.
type gearDurability struct {
	cur, max, decayed          float64
	hasCur, hasMax, hasDecayed bool
}

// readGearDurability reads the durability record out of an item's stats text. ok is false when the item has no such record or no
// CurrentDurability (nothing to repair). Nothing decoded here is ever written back: the repair patches the stored text in place.
func readGearDurability(stats string) (d gearDurability, ok bool) {
	var doc map[string][]json.RawMessage
	if json.Unmarshal([]byte(stats), &doc) != nil {
		return d, false
	}
	pair := doc["FItemStackAndDurabilityStats"]
	if len(pair) < 2 {
		return d, false
	}
	var rec map[string]json.RawMessage
	if json.Unmarshal(pair[1], &rec) != nil {
		return d, false
	}
	num := func(key string) (float64, bool) {
		var f float64
		raw, has := rec[key]
		if !has || json.Unmarshal(raw, &f) != nil {
			return 0, false
		}
		return f, true
	}
	d.cur, d.hasCur = num("CurrentDurability")
	d.max, d.hasMax = num("MaxDurability")
	d.decayed, d.hasDecayed = num("DecayedMaxDurability")
	return d, d.hasCur
}

// RepairGear restores durability on equipped gear, armor, weapons and backpack.
// RepairGear raises the durability of everything the character wears (inventory type 1) and carries as a loadout (type 15) to
// its maximum. The backpack and the other inventories are left alone.
//
// The items are read and repaired in one transaction (audit L2 G09), and only the durability keys that change are written, each with
// json_set on the item's own text (audit L2 G10): every other part of the stats document, its key order, 64-bit integers and floats such
// as 100.0 keep exactly the text the game wrote.
func (o *Ops) RepairGear() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	scanned, repaired := 0, 0
	const rec = `$.FItemStackAndDurabilityStats[1].`
	_, err = o.S.Mutate("repair gear", func(m *save.Mut) error {
		scanned, repaired = 0, 0
		rows, err := m.Query(`select i.id, i.stats from items i join inventories v on v.id=i.inventory_id
			where v.actor_id=? and v.inventory_type in (1,15)`, p.Pawn)
		if err != nil {
			return err
		}
		scanned = len(rows)
		for _, r := range rows {
			d, ok := readGearDurability(fmt.Sprint(r["stats"]))
			if !ok {
				continue
			}
			// as good as new: the item's full durability is its MaxDurability when it records one, else 100, the scale of worn gear.
			// A recorded DecayedMaxDurability (what repairs at a workbench wear down) goes back to it too. A repair only raises an item;
			// it never lowers one that is already above.
			target := 100.0
			if d.hasMax && d.max > 0 {
				target = d.max
			}
			wornMax := d.hasDecayed && d.decayed > 0 && d.decayed < target // a decayed maximum of 0 means none was recorded
			if d.cur >= target && !wornMax {
				continue
			}
			var q string
			var args []any
			switch {
			case d.cur < target && wornMax:
				q, args = `update items set stats=json_set(stats,'`+rec+`CurrentDurability',?,'`+rec+`DecayedMaxDurability',?) where id=?`, []any{target, target, r["id"]}
			case d.cur < target:
				q, args = `update items set stats=json_set(stats,'`+rec+`CurrentDurability',?) where id=?`, []any{target, r["id"]}
			default:
				q, args = `update items set stats=json_set(stats,'`+rec+`DecayedMaxDurability',?) where id=?`, []any{target, r["id"]}
			}
			if _, err := m.Exec(q, args...); err != nil {
				return err
			}
			repaired++
		}
		m.Desc = fmt.Sprintf("repair gear: %d items", repaired)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "scanned": scanned, "repaired": repaired}, nil
}

// removeSolariInTx takes want Solari out of the player's backpack stacks, from the last slot back, deleting a stack it empties. It refuses (and changes nothing) when the
// stacks hold less than want, so a removal never reports more than it took.
func removeSolariInTx(m *save.Mut, pawn, want int64) error {
	rows, err := m.Query(`select i.id, i.stack_size from items i join inventories v on v.id=i.inventory_id
		where v.actor_id=? and v.inventory_type=0 and i.template_id=? order by i.position_index desc, i.id desc`, pawn, solariTemplate)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errors.New("player has no Solari")
	}
	var have int64
	for _, r := range rows {
		n, _ := r["stack_size"].(int64)
		have += n
	}
	if have < want {
		return fmt.Errorf("not enough Solari: the backpack holds %s and %s were asked to be removed", commas(have), commas(want))
	}
	left := want
	for _, r := range rows {
		if left == 0 {
			break
		}
		n, _ := r["stack_size"].(int64)
		take := min(n, left)
		if take == n {
			_, err = m.Exec(`delete from items where id=?`, r["id"])
		} else {
			_, err = m.Exec(`update items set stack_size=? where id=?`, n-take, r["id"])
		}
		if err != nil {
			return err
		}
		left -= take
	}
	return nil
}

// AddSolari adjusts the player's Solari (an item stack in this game version).
func (o *Ops) AddSolari(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	delta, err := a.IntRange("amount", -1_000_000_000, 1_000_000_000)
	if err != nil || delta == 0 {
		return nil, apierr.New(apierr.BadArgument, "amount must be a non-zero integer")
	}
	_, err = o.S.Mutate(fmt.Sprintf("solari %+d", delta), func(m *save.Mut) error {
		// Solari are item stacks, and a slot holds no more than the item tables give (50,000): adding fills the stacks that are there and makes new ones, removing takes
		// from the stacks from the last slot back, so nothing is removed from one stack that it does not hold.
		if delta < 0 {
			return removeSolariInTx(m, p.Pawn, -delta)
		}
		rows, err := m.Query(`select i.inventory_id from items i join inventories v on v.id=i.inventory_id
			where v.actor_id=? and v.inventory_type=0 and i.template_id=? order by i.id limit 1`, p.Pawn, solariTemplate)
		if err != nil {
			return err
		}
		var inv int64
		if len(rows) > 0 {
			inv, _ = rows[0]["inventory_id"].(int64)
		} else {
			bp, err := m.Query(`select id from inventories where actor_id=? and inventory_type=0 order by id limit 1`, p.Pawn)
			if err != nil {
				return err
			}
			if len(bp) == 0 {
				return apierr.New(apierr.NotFound, "backpack not found")
			}
			inv, _ = bp[0]["id"].(int64)
		}
		_, err = giveStacksInTx(m, inv, solariTemplate, delta, 0)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "solari": o.solari(p)}, nil
}

// ---------------------------------------------------------------- progression

func (o *Ops) Factions() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	rows, err := o.S.Query(`select f.id faction_id, f.name, coalesce(r.reputation_amount,0) reputation
		from factions f left join player_faction_reputation r on r.faction_id=f.id and r.actor_id=? order by f.id`, p.Controller)
	return rows, err
}

func (o *Ops) SetReputation(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	f, err := a.IntRange("faction_id", 1, 32767)
	if err != nil {
		return nil, err
	}
	v, err := a.IntRange("amount", 0, 12474)
	if err != nil {
		return nil, err
	}
	if _, err := o.S.Mutate(fmt.Sprintf("faction %d reputation = %d", f, v), func(m *save.Mut) error {
		name := fmt.Sprintf("faction %d", f)
		if r, _ := m.One(`select name from factions where id=?`, f); r != nil {
			name = fmt.Sprint(r["name"])
		}
		var was int64
		if r, _ := m.One(`select reputation_amount from player_faction_reputation where actor_id=? and faction_id=?`, p.Controller, f); r != nil {
			was, _ = r["reputation_amount"].(int64)
		}
		if _, err := m.Exec(`insert into player_faction_reputation(actor_id, faction_id, reputation_amount) values(?,?,?)
			on conflict(actor_id, faction_id) do update set reputation_amount=excluded.reputation_amount`, p.Controller, f, v); err != nil {
			return err
		}
		m.Desc = fmt.Sprintf("Reputation with %s: %s", name, change(was, v))
		return nil
	}); err != nil {
		return nil, err
	}
	return ok(), nil
}

func (o *Ops) SetSpec(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	t, e1 := a.IntRange("track_type", 0, 255)
	xp, e2 := a.IntRange("xp", 0, maxSpecXP) // a track never holds more than the XP of its last level (DBA-7)
	lvl, e3 := a.Float("level")
	if err := errors.Join(e1, e2, e3); err != nil {
		return nil, err
	}
	if lvl < 0 || lvl > 100 {
		return nil, apierr.New(apierr.BadArgument, "level out of range (0..100)")
	}
	if _, err := o.run(fmt.Sprintf("specialization %d: xp=%d level=%g", t, xp, lvl), `insert into specialization_tracks(player_id, track_type, xp_amount, level) values(?,?,?,?)
		on conflict(player_id, track_type) do update set xp_amount=excluded.xp_amount, level=excluded.level`, p.Controller, t, xp, lvl); err != nil {
		return nil, err
	}
	return ok(), nil
}

func (o *Ops) Journey(q string) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	return o.S.Query(`select story_node_id id, has_pending_reward pending, complete_condition_state=? complete
		from journey_story_node where character_id=? and story_node_id like ? order by story_node_id`, jsonTrue, p.Controller, "%"+q+"%")
}

// JourneySet completes or resets a story node and every child ("node.*").
func (o *Ops) JourneySet(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	id := a.Str("node_id")
	if !nodeRe.MatchString(id) {
		return nil, apierr.New(apierr.BadArgument, "invalid node id")
	}
	complete := a.Bool("complete", true)
	state := jsonEmptyObj
	if complete {
		state = jsonTrue
	}
	var n int64
	_, err = o.S.Mutate("", func(m *save.Mut) error {
		// The children are the rows whose id starts with "<id>.". A plain prefix compare is used rather than LIKE, whose "_" matches any
		// character (node ids hold underscores) and which ignores case.
		res, err := m.Exec(`update journey_story_node set complete_condition_state=?, reveal_condition_state=?
			where character_id=? and (story_node_id=? or substr(story_node_id, 1, length(?) + 1) = ? || '.')`, state, jsonTrue, p.Controller, id, id, id)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		if complete {
			// The node's own row too, when only children have rows: completing a node must complete the node itself.
			own, err := m.One(`select count(*) c from journey_story_node where character_id=? and story_node_id=?`, p.Controller, id)
			if err != nil {
				return err
			}
			if c, _ := toInt(own["c"]); c == 0 {
				if _, err := m.Exec(`insert into journey_story_node(character_id, story_node_id, has_pending_reward, complete_condition_state,
					reveal_condition_state, fail_condition_state, metadata_state, reset_group) values(?,?,0,?,?,?,?,0)`,
					p.Controller, id, jsonTrue, jsonTrue, jsonEmptyObj, jsonEmptyObj); err != nil {
					return err
				}
				n++
			}
		}
		m.Desc = fmt.Sprintf("journey %s %s (%d rows)", id, map[bool]string{true: "completed", false: "reset"}[complete], n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "rows": n}, nil
}

func (o *Ops) Tutorials() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	return o.S.Query(`select t.id, t.name, tp.tutorial_state state from tutorials t
		left join tutorial_per_player tp on tp.tutorial_id=t.id and tp.player_id=? order by t.id`, p.Controller)
}

func (o *Ops) TutorialSet(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	id, err := a.IntRange("id", 1, 32767)
	if err != nil {
		return nil, err
	}
	desc := fmt.Sprintf("tutorial %d updated", id)
	if a.Bool("complete", true) {
		_, err = o.run(desc, `insert into tutorial_per_player(player_id, tutorial_id, tutorial_state) values(?,?,2)
			on conflict(player_id, tutorial_id) do update set tutorial_state=2`, p.Controller, id)
	} else {
		_, err = o.run(desc, `delete from tutorial_per_player where player_id=? and tutorial_id=?`, p.Controller, id)
	}
	if err != nil {
		return nil, err
	}
	return ok(), nil
}

func (o *Ops) Tags() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	return o.S.Query(`select tag from player_tags where character_id=? order by tag`, p.Controller)
}

func (o *Ops) TagSet(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	tag := a.Str("tag")
	if !nodeRe.MatchString(tag) {
		return nil, apierr.New(apierr.BadArgument, "invalid tag")
	}
	desc := fmt.Sprintf("tag %s updated", tag)
	if a.Bool("add", true) {
		_, err = o.run(desc, `insert or ignore into player_tags(character_id, tag) values(?,?)`, p.Controller, tag)
	} else {
		_, err = o.run(desc, `delete from player_tags where character_id=? and tag=?`, p.Controller, tag)
	}
	if err != nil {
		return nil, err
	}
	return ok(), nil
}

func (o *Ops) Recipes() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	sets, _ := o.S.Query(`select learned_building_set name from building_progression_learned_building_sets where character_id=? order by 1`, p.Controller)
	pieces, _ := o.S.Query(`select new_buildable_piece name from building_progression_new_buildable_pieces where character_id=? order by 1`, p.Controller)
	return map[string]any{"learnedSets": sets, "newPieces": pieces}, nil
}

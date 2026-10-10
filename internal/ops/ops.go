// Package ops implements the player, base, vehicle, vendor and Landsraad
// operations. It is modelled on console/api/src/duneDb.js from
// dune-awakening-selfhost-docker, translated from Postgres stored procedures to
// plain SQL against the single-player SQLite save.
package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/Project-Arrakis/tabr-tau/internal/augments"
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

func (a Args) Float(k string) (float64, error) {
	switch v := a[k].(type) {
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	}
	return 0, fmt.Errorf("%s must be a number", k)
}

func (a Args) Int(k string) (int64, error) {
	f, err := a.Float(k)
	if err != nil {
		return 0, err
	}
	return int64(f), nil
}

func (a Args) IntRange(k string, lo, hi int64) (int64, error) {
	n, err := a.Int(k)
	if err != nil {
		return 0, err
	}
	if n < lo || n > hi {
		return 0, fmt.Errorf("%s out of range (%d..%d)", k, lo, hi)
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
		"readOnly": o.S.WriteBlocked(),
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
func giveInTx(m *save.Mut, inventoryID int64, template string, qty, quality int64) (int64, error) {
	rows, err := m.Query(`select id, coalesce(max_item_count,0) cap from inventories where id=?`, inventoryID)
	if err != nil || len(rows) == 0 {
		return 0, errors.New("inventory not found")
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
	stats := `{"FCustomizationStats":[[],{}],"FItemStackAndDurabilityStats":[[],{}]}`
	if k, _ := m.Query(`select stats from items where template_id=? and stats is not null limit 1`, template); len(k) > 0 {
		stats = fmt.Sprint(k[0]["stats"])
		if plain, err := augments.Stats(stats, nil); err == nil { // the copied item's augments are its own, not the new item's
			stats = plain
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

func (o *Ops) giveArgs(a Args) (inv int64, tmpl string, qty, q int64, err error) {
	tmpl = a.Str("template_id")
	if !templateRe.MatchString(tmpl) {
		return 0, "", 0, 0, errors.New("invalid template id")
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
			return nil, errors.New("player backpack not found")
		}
		inv = r["id"].(int64)
	} else if r, _ := o.S.One(`select 1 x from inventories where id=? and actor_id=?`, inv, p.Pawn); r == nil {
		return nil, errors.New("that inventory does not belong to the player")
	}
	var id int64
	if _, err := o.S.Mutate(fmt.Sprintf("give %dx %s (grade %d)", qty, tmpl, q), func(m *save.Mut) (e error) { id, e = giveInTx(m, inv, tmpl, qty, q); return }); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "itemId": id}, nil
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
		return nil, errors.New("the queue is empty")
	}
	if len(list) > maxGiveBatch {
		return nil, fmt.Errorf("at most %d items at a time", maxGiveBatch)
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
			return nil, fmt.Errorf("item %d is not an object", i+1)
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
		return nil, errors.New("player backpack not found")
	}
	inv := r["id"].(int64)
	ids := make([]int64, 0, len(lines))
	if _, err := o.S.Mutate(fmt.Sprintf("give %d queued items", len(lines)), func(m *save.Mut) error {
		for i, l := range lines {
			id, e := giveInTx(m, inv, l.tmpl, l.qty, l.q)
			if e != nil {
				return fmt.Errorf("item %d (%s): %w", i+1, l.tmpl, e)
			}
			if len(l.aug) > 0 {
				if _, e := setAugmentsTx(m, id, l.aug, l.grade); e != nil {
					return fmt.Errorf("item %d (%s): %w", i+1, l.tmpl, e)
				}
			}
			ids = append(ids, id)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "itemIds": ids, "count": len(ids)}, nil
}

// GiveToInventory adds an item to any storage inventory (bases tab).
func (o *Ops) GiveToInventory(a Args) (any, error) {
	inv, tmpl, qty, q, err := o.giveArgs(a)
	if err != nil {
		return nil, err
	}
	if inv == 0 {
		return nil, errors.New("inventory_id is required")
	}
	var id int64
	if _, err := o.S.Mutate(fmt.Sprintf("give %dx %s to inventory %d", qty, tmpl, inv), func(m *save.Mut) (e error) { id, e = giveInTx(m, inv, tmpl, qty, q); return }); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "itemId": id}, nil
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
	if _, has := a["stack_size"]; has {
		n, err := a.IntRange("stack_size", 1, 10_000_000)
		if err != nil {
			return nil, err
		}
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
	_, err = o.S.Mutate(fmt.Sprintf("edit item %d", id), func(m *save.Mut) error {
		for _, e := range edits {
			res, err := m.Exec(`update items set `+e.col+`=? where id=?`, e.val, id)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return errors.New("item not found")
			}
		}
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
	// Cascading delete: an item that owns an inventory takes that inventory and everything in it with it, as the
	// game's schema declares (ON DELETE CASCADE), instead of leaving them dangling.
	_, err = o.S.MutateCascade(fmt.Sprintf("delete item %d", id), func(m *save.Mut) error {
		res, err := m.Exec(`delete from items where id=?`, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errors.New("item not found")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ok(), nil
}

// RepairGear restores durability on equipped gear, armor, weapons and backpack.
// RepairGear raises the durability of everything the character wears (inventory type 1) and carries as a loadout (type 15) to
// its maximum. The backpack and the other inventories are left alone.
func (o *Ops) RepairGear() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	rows, err := o.S.Query(`select i.id, i.stats from items i join inventories v on v.id=i.inventory_id
		where v.actor_id=? and v.inventory_type in (1,15)`, p.Pawn)
	if err != nil {
		return nil, err
	}
	repaired := 0
	_, err = o.S.Mutate("repair gear", func(m *save.Mut) error {
		for _, r := range rows {
			var st map[string][]any
			if json.Unmarshal([]byte(fmt.Sprint(r["stats"])), &st) != nil {
				continue
			}
			pair := st["FItemStackAndDurabilityStats"]
			if len(pair) < 2 {
				continue
			}
			d, isMap := pair[1].(map[string]any)
			if !isMap {
				continue
			}
			if _, has := d["CurrentDurability"]; !has {
				continue
			}
			target := 100.0
			if mx, ok := d["MaxDurability"].(float64); ok && mx > 0 {
				target = mx
			}
			d["CurrentDurability"] = target
			b, _ := json.Marshal(st)
			if _, err := m.Exec(`update items set stats=? where id=?`, string(b), r["id"]); err != nil {
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
	return map[string]any{"ok": true, "scanned": len(rows), "repaired": repaired}, nil
}

// AddSolari adjusts the player's Solari (an item stack in this game version).
func (o *Ops) AddSolari(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	delta, err := a.IntRange("amount", -1_000_000_000, 1_000_000_000)
	if err != nil || delta == 0 {
		return nil, errors.New("amount must be a non-zero integer")
	}
	_, err = o.S.Mutate(fmt.Sprintf("solari %+d", delta), func(m *save.Mut) error {
		rows, _ := m.Query(`select i.id, i.stack_size from items i join inventories v on v.id=i.inventory_id
			where v.actor_id=? and v.inventory_type=0 and i.template_id=? order by i.id limit 1`, p.Pawn, solariTemplate)
		if len(rows) > 0 {
			n := rows[0]["stack_size"].(int64) + delta
			if n <= 0 {
				_, err := m.Exec(`delete from items where id=?`, rows[0]["id"])
				return err
			}
			_, err := m.Exec(`update items set stack_size=? where id=?`, n, rows[0]["id"])
			return err
		}
		if delta < 0 {
			return errors.New("player has no Solari")
		}
		inv, _ := m.Query(`select id from inventories where actor_id=? and inventory_type=0 order by id limit 1`, p.Pawn)
		if len(inv) == 0 {
			return errors.New("backpack not found")
		}
		_, err := giveInTx(m, inv[0]["id"].(int64), solariTemplate, delta, 0)
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
	if _, err := o.run(fmt.Sprintf("faction %d reputation = %d", f, v), `insert into player_faction_reputation(actor_id, faction_id, reputation_amount) values(?,?,?)
		on conflict(actor_id, faction_id) do update set reputation_amount=excluded.reputation_amount`, p.Controller, f, v); err != nil {
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
	xp, e2 := a.IntRange("xp", 0, 1_000_000_000)
	lvl, e3 := a.Float("level")
	if err := errors.Join(e1, e2, e3); err != nil {
		return nil, err
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
		return nil, errors.New("invalid node id")
	}
	complete := a.Bool("complete", true)
	state := jsonEmptyObj
	if complete {
		state = jsonTrue
	}
	var n int64
	_, err = o.S.Mutate("", func(m *save.Mut) error {
		res, err := m.Exec(`update journey_story_node set complete_condition_state=?, reveal_condition_state=?
			where character_id=? and (story_node_id=? or story_node_id like ? || '.%')`, state, jsonTrue, p.Controller, id, id)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		if n == 0 && complete {
			if _, err := m.Exec(`insert into journey_story_node(character_id, story_node_id, has_pending_reward, complete_condition_state,
				reveal_condition_state, fail_condition_state, metadata_state, reset_group) values(?,?,0,?,?,?,?,0)`,
				p.Controller, id, jsonTrue, jsonTrue, jsonEmptyObj, jsonEmptyObj); err != nil {
				return err
			}
			n = 1
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
		return nil, errors.New("invalid tag")
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

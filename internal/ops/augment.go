package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/augments"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// fitOf is what an item takes in augments, from its template id and the catalog's name and category.
func fitOf(template string) augments.Fit {
	name, _ := catalog.Name(template)
	return augments.For(template, name, catalog.Category(template))
}

func checkAugments(template string, ids []string) error {
	name, _ := catalog.Name(template)
	return augments.Check(template, name, catalog.Category(template), ids)
}

// augmentIDs reads the "augments" argument: a list of augment ids, empty ones dropped.
func augmentIDs(a Args) ([]string, error) {
	raw, has := a["augments"]
	if !has || raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, errors.New("augments must be a list")
	}
	var ids []string
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			return nil, errors.New("augments must be a list of augment ids")
		}
		if s = strings.TrimSpace(s); s != "" {
			ids = append(ids, s)
		}
	}
	return ids, nil
}

func augmentGrade(a Args) (int, error) {
	if _, has := a["grade"]; !has {
		return 1, nil
	}
	g, err := a.IntRange("grade", 1, 5)
	return int(g), err
}

// slotKeystonePattern is the name of the specialization keystones that open the augment slots for an item kind: the save's
// own map names them (Crafting_CraftingKeystone_ArmoirAugmentSlots10, ...MeleeWeaponAugmentSlots3, ...RangedWeaponAugmentSlots1).
func slotKeystonePatterns(template string) []string {
	f := fitOf(template)
	switch f.Kind {
	case augments.KindClothing:
		return []string{"Crafting_CraftingKeystone_ArmoirAugmentSlots%"}
	case augments.KindWeapon:
		name, _ := catalog.Name(template)
		tags := strings.Join(augments.Tags(template, name), " ")
		melee, ranged := strings.Contains(tags, "MeleeWeapons"), strings.Contains(tags, "RangedWeapons")
		switch {
		case melee && !ranged:
			return []string{"Crafting_CraftingKeystone_MeleeWeaponAugmentSlots%"}
		case ranged && !melee:
			return []string{"Crafting_CraftingKeystone_RangedWeaponAugmentSlots%"}
		}
		return []string{"Crafting_CraftingKeystone_MeleeWeaponAugmentSlots%", "Crafting_CraftingKeystone_RangedWeaponAugmentSlots%"}
	}
	return nil
}

// unlockSlots buys the augment-slot keystones for the kind of item, as the console does so the slots show in game. It only
// adds missing rows and returns how many.
func unlockSlots(m *save.Mut, controller int64, template string) (int64, error) {
	var total int64
	for _, pat := range slotKeystonePatterns(template) {
		res, err := m.Exec(`insert or ignore into purchased_specialization_keystones(player_id, keystone_id)
			select ?, id from specialization_keystones_map where name like ?`, controller, pat)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

// setAugmentsTx writes exactly the given augments onto an item (the ones already on it keep their stored grade and rolls; new
// ones get the grade with perfect rolls). It returns the item's template id.
func setAugmentsTx(m *save.Mut, itemID int64, ids []string, grade int) (string, error) {
	rows, err := m.Query(`select template_id, stats from items where id=?`, itemID)
	if err != nil || len(rows) == 0 {
		return "", errors.New("item not found")
	}
	tmpl := fmt.Sprint(rows[0]["template_id"])
	if err := checkAugments(tmpl, ids); err != nil {
		return "", err
	}
	stats := ""
	if s, ok := rows[0]["stats"].(string); ok {
		stats = s
	}
	have := map[string]augments.Applied{}
	for _, ap := range augments.ReadApplied(stats) {
		have[ap.ID] = ap
	}
	next := make([]augments.Applied, 0, len(ids))
	for _, id := range ids {
		if ap, ok := have[id]; ok {
			next = append(next, ap)
		} else {
			next = append(next, augments.Perfect(id, grade))
		}
	}
	out, err := augments.Stats(stats, next)
	if err != nil {
		return "", err
	}
	if _, err := m.Exec(`update items set stats=? where id=?`, out, itemID); err != nil {
		return "", err
	}
	return tmpl, nil
}

func (o *Ops) ownedItem(itemID int64) (template, stats string, err error) {
	p, err := o.player()
	if err != nil {
		return "", "", err
	}
	r, _ := o.S.One(`select i.template_id, i.stats from items i join inventories v on v.id=i.inventory_id where i.id=? and v.actor_id=?`, itemID, p.Pawn)
	if r == nil {
		return "", "", errors.New("that item is not in the player's inventory")
	}
	stats, _ = r["stats"].(string)
	return fmt.Sprint(r["template_id"]), stats, nil
}

// AugmentOptions says what the item takes and what it has: its augment limit, the augments that fit with their best-grade
// effects, and the ones applied now. args: item_id, or template_id to ask about an item that is not in the save yet.
func (o *Ops) AugmentOptions(a Args) (any, error) {
	var tmpl, stats string
	if _, has := a["item_id"]; has {
		id, err := a.Int("item_id")
		if err != nil {
			return nil, err
		}
		if tmpl, stats, err = o.ownedItem(id); err != nil {
			return nil, err
		}
	} else {
		tmpl = a.Str("template_id")
		if !templateRe.MatchString(tmpl) {
			return nil, errors.New("invalid template id")
		}
	}
	f := fitOf(tmpl)
	applied := []map[string]any{}
	for _, ap := range augments.ReadApplied(stats) {
		applied = append(applied, map[string]any{"id": ap.ID, "name": augments.Name(ap.ID), "quality": string(ap.Quality)})
	}
	return map[string]any{"template_id": tmpl, "kind": f.Kind, "limit": f.Limit, "options": f.Options, "applied": applied}, nil
}

// AugmentItem sets the augments on an item in the player's inventory: args item_id, augments (the full list wanted, empty to
// remove them all), grade 1 to 5 for the ones being added, and unlock_slots to also buy the augment-slot keystones.
func (o *Ops) AugmentItem(a Args) (any, error) {
	id, err := a.Int("item_id")
	if err != nil {
		return nil, err
	}
	ids, err := augmentIDs(a)
	if err != nil {
		return nil, err
	}
	grade, err := augmentGrade(a)
	if err != nil {
		return nil, err
	}
	tmpl, _, err := o.ownedItem(id)
	if err != nil {
		return nil, err
	}
	if err := checkAugments(tmpl, ids); err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	unlock, _ := a["unlock_slots"].(bool)
	var unlocked int64
	label := fmt.Sprintf("set %d augment(s) on item %d (%s)", len(ids), id, tmpl)
	if _, err := o.S.Mutate(label, func(m *save.Mut) error {
		if _, err := setAugmentsTx(m, id, ids, grade); err != nil {
			return err
		}
		if unlock && len(ids) > 0 {
			var e error
			unlocked, e = unlockSlots(m, p.Controller, tmpl)
			return e
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "augments": len(ids), "slotsUnlocked": unlocked}, nil
}

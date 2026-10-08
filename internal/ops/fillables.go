package ops

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// waterCapacity is how much water each water container holds when full. The unit matches the save's
// FFillableItemStats.CurrentAmount (a half-full Literjon Mk6 holds 11000 of 20000).
//
// Source: read in game by the operator and cross-checked against the item pages of dune.gaming.tools
// (stat "fillableCapacity"), 2026-10-08. The numbers are game facts; no site content is copied. The site's
// terms require permission for bulk scraping, so this table is grown one verified item at a time.
// A container with a fill record that is not listed here is skipped and reported, never guessed. An empty
// container that is not listed has no record to recognise it by, so it is left alone without a report.
var waterCapacity = map[string]float64{
	"Literjon":                1000,
	"Literjon_T6":             20000,
	"Decajon":                 10000,
	"HighCapacityLiterjon":    1500,
	"HighCapacityLiterjon_02": 1750,
	"HighCapacityLiterjon_03": 2000,
	"HighCapacityLiterjon_04": 2250,
	"HighCapacityLiterjon_05": 2500,
	"HighCapacityLiterjon_06": 3000,
}

// RefillContainers fills every water container the player carries to its capacity. A container holding
// something other than water (a blood sack) is left alone. The fill level lives in the item's own stats as
// {"FFillableItemStats":[[],{"CurrentAmount":N,"FillableType":"Water"}]}; an empty container has no such key.
func (o *Ops) RefillContainers() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	rows, err := o.S.Query(`select i.id, i.template_id,
		cast(json_extract(i.stats,'$.FFillableItemStats[1].CurrentAmount') as real) amount,
		json_extract(i.stats,'$.FFillableItemStats[1].FillableType') kind,
		json_extract(i.stats,'$.FFillableItemStats') has
		from items i join inventories v on v.id=i.inventory_id
		where v.actor_id=? and json_valid(i.stats) order by i.id`, p.Pawn)
	if err != nil {
		return nil, err
	}
	type target struct {
		id  any
		cap float64
	}
	var todo []target
	full := 0
	unknown := map[string]int{}
	for _, r := range rows {
		tpl := fmt.Sprint(r["template_id"])
		capacity, known := waterCapacity[tpl]
		if !known {
			// A record without a type or amount is an empty container; blood sacks (issue #60) are not water.
			if r["has"] != nil && !strings.HasPrefix(tpl, "Bloodsack") { // a container we cannot size
				if r["kind"] == nil || r["kind"] == "Water" {
					unknown[tpl]++
				}
			}
			continue
		}
		if r["kind"] != nil && r["kind"] != "Water" {
			continue
		}
		if cur, ok := r["amount"].(float64); ok && cur >= capacity {
			full++
			continue
		}
		todo = append(todo, target{r["id"], capacity})
	}
	_, err = o.S.Mutate("refill containers", func(m *save.Mut) error {
		for _, t := range todo {
			// json_set keeps every other key and writes the number the way the game does (20000.0).
			if _, err := m.Exec(`update items set stats=case
				when json_extract(stats,'$.FFillableItemStats') is null
				then json_set(stats,'$.FFillableItemStats',json_array(json_array(),json_object('CurrentAmount',?,'FillableType','Water')))
				else json_set(stats,'$.FFillableItemStats[1].CurrentAmount',?,'$.FFillableItemStats[1].FillableType','Water') end
				where id=?`, t.cap, t.cap, t.id); err != nil {
				return err
			}
		}
		m.Desc = fmt.Sprintf("refill containers: %d filled", len(todo))
		return nil
	})
	if err != nil {
		return nil, err
	}
	skipped := make([]string, 0, len(unknown))
	for k, n := range unknown {
		skipped = append(skipped, fmt.Sprintf("%s x%d", k, n))
	}
	sort.Strings(skipped)
	return map[string]any{"ok": true, "filled": len(todo), "alreadyFull": full, "skippedUnknown": skipped}, nil
}

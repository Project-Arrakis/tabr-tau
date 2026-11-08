package ops

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/fillables"
	"github.com/Project-Arrakis/tabr-tau/kit/schema"
)

// The capacities of the fillable items are the game's own table of them (DT_ItemTableFillables, the row named by the item's template, field MaxAmount), kept in the kit module as
// kit/gamedata/fillables (a snapshot of the table, read from the installed game's content files; issues #472, #482 and #551; docs/design/2026-10-28-fillable-capacities.md
// and docs/design/2026-10-28-blood-sack-capacity.md have the whole table). The editor sorts the rows into three maps by the rule it has always followed, and tests that pin
// every number a second time (fillable_capacity_test.go, bloodsack_capacity_test.go) keep a change of the table from going unnoticed. The unit matches the save's
// FFillableItemStats.CurrentAmount (a half-full Literjon Mk6 holds 11000 of 20000).
//
// A container with a fill record that is not in the maps is skipped and reported, never guessed. An empty container that is not listed has no record to recognise it by, so it
// is left alone without a report.
var (
	// waterCapacity is how much water each water container holds when full: the rows whose liquid is Water and that are containers (the Literjons of every grade, the Decajon and
	// the Dewpack). The first nine were first read in game; the table gives the same numbers.
	waterCapacity = map[string]float64{}

	// stillsuitCapacity is the water store of the worn stillsuits and their kin: the rows whose liquid is Water and that are not containers (issue #482). The suit is not a
	// container the person fills: the game puts the water it recovers into it and changes the amount all the time, and the save keeps no liquid type for it. So the editor shows
	// the amount against this capacity but never writes it (Fill water containers and Fill all liquids leave a suit alone, without reporting it as unknown). 500 each, 1500 for
	// the high capacity suit.
	stillsuitCapacity = map[string]float64{}

	// bloodCapacity is how much blood each blood sack holds when full, in the unit of the save's FFillableItemStats.CurrentAmount: the rows whose liquid is Blood. Bloodsack_01
	// and Bloodsack_Unique_Durable_06 were first entered from the largest amount seen in a save (2026-10-23, issue #60); the table gives the same numbers. A blood sack template
	// that is not listed is left alone and reported, because its capacity is unknown.
	bloodCapacity = map[string]float64{}
)

// notFillableByThisRule are the table rows that belong to none of the three maps: WaterPack_Consumable is drunk, it is not a container and not a worn suit, and it has never been
// in the maps (a fill of it would be a fill of something the person is about to use up).
var notFillableByThisRule = map[string]bool{"WaterPack_Consumable": true}

func init() {
	for _, r := range fillables.Table() {
		switch {
		case notFillableByThisRule[r.Name]:
		case r.Restriction == fillables.Blood && r.Container:
			bloodCapacity[r.Name] = float64(r.MaxAmount)
		case r.Restriction == fillables.Water && r.Container:
			waterCapacity[r.Name] = float64(r.MaxAmount)
		case r.Restriction == fillables.Water:
			stillsuitCapacity[r.Name] = float64(r.MaxAmount)
		}
	}
}

// waterQuery lists the items of the character's own inventories (backpack, worn gear, loadout) that have stats, with what each holds.
const waterQuery = `select i.id, i.template_id,
		cast(json_extract(i.stats,'` + schema.FillableCurrentAmount + `') as real) amount,
		json_extract(i.stats,'` + schema.FillableLiquid + `') kind,
		json_extract(i.stats,'` + schema.FillableKey + `') has
		from items i join inventories v on v.id=i.inventory_id
		where v.actor_id=? and json_valid(i.stats) order by i.id`

type waterTarget struct {
	id  any
	cap float64
}

// waterState is what a look at the character's water containers finds.
type waterState struct {
	todo    []waterTarget  // containers of a known kind that are not full
	full    int            // containers at or above capacity
	unknown map[string]int // containers with a fill record that are not in waterCapacity: left alone
	suits   int            // worn stillsuits (stillsuitCapacity): left alone, the game fills them
}

// lookAtWater sorts the character's containers into what to fill and what to leave alone. query is the edit's own query when it runs inside an edit.
func lookAtWater(query func(string, ...any) ([]save.Row, error), pawn int64) (waterState, error) {
	st := waterState{unknown: map[string]int{}}
	rows, err := query(waterQuery, pawn)
	if err != nil {
		return st, err
	}
	for _, r := range rows {
		tpl := fmt.Sprint(r["template_id"])
		capacity, known := waterCapacity[tpl]
		if !known {
			if _, suit := stillsuitCapacity[tpl]; suit { // the game fills a suit itself: not ours to fill, and not a container of unknown size
				st.suits++
				continue
			}
			// A record without a type or amount is an empty container; blood sacks (issue #60) are not water.
			if r["has"] != nil && !strings.HasPrefix(tpl, "Bloodsack") { // a container we cannot size
				if r["kind"] == nil || r["kind"] == "Water" {
					st.unknown[tpl]++
				}
			}
			continue
		}
		if r["kind"] != nil && r["kind"] != "Water" {
			continue
		}
		if cur, ok := r["amount"].(float64); ok && cur >= capacity {
			st.full++
			continue
		}
		st.todo = append(st.todo, waterTarget{r["id"], capacity})
	}
	return st, nil
}

// refillContainersTx fills the containers a look finds to their capacity, inside the edit m. The Refill Literjons button and Fill all liquids both end here.
func refillContainersTx(m *save.Mut, pawn int64) (waterState, error) {
	st, err := lookAtWater(m.Query, pawn)
	if err != nil {
		return st, err
	}
	for _, t := range st.todo {
		// json_set keeps every other key and writes the number the way the game does (20000.0).
		if _, err := m.Exec(`update items set stats=case
				when json_extract(stats,'`+schema.FillableKey+`') is null
				then json_set(stats,'`+schema.FillableKey+`',json_array(json_array(),json_object('CurrentAmount',?,'FillableType','Water')))
				else json_set(stats,'`+schema.FillableCurrentAmount+`',?,'`+schema.FillableLiquid+`','Water') end
				where id=?`, t.cap, t.cap, t.id); err != nil {
			return st, err
		}
	}
	return st, nil
}

func (st waterState) answer() map[string]any {
	skipped := make([]string, 0, len(st.unknown))
	for k, n := range st.unknown {
		skipped = append(skipped, fmt.Sprintf("%s x%d", k, n))
	}
	sort.Strings(skipped)
	return map[string]any{"ok": true, "filled": len(st.todo), "alreadyFull": st.full, "skippedUnknown": skipped, "stillsuits": st.suits}
}

// RefillContainers fills every water container the player carries to its capacity. A container holding
// something other than water (a blood sack) is left alone. The fill level lives in the item's own stats as
// {"FFillableItemStats":[[],{"CurrentAmount":N,"FillableType":"Water"}]}; an empty container has no such key.
func (o *Ops) RefillContainers() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var st waterState
	// The containers are looked at inside the edit, through the edit's own query: what the edit read is then kept with it, and an edit put back onto a newer file is not applied
	// to a list of containers made from the older one.
	_, err = o.S.Mutate("refill containers", func(m *save.Mut) error {
		var rerr error
		if st, rerr = refillContainersTx(m, p.Pawn); rerr != nil {
			return rerr
		}
		m.Desc = fmt.Sprintf("refill containers: %d filled", len(st.todo))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return st.answer(), nil
}

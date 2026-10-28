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
// Source: the game's item data, the table of fillable items (DT_ItemTableFillables, the row named by the item's template, field MaxAmount, with
// FillableTypeRestriction Water and bIsContainer true), read 2026-10-28 (issue #482; docs/design/2026-10-28-fillable-capacities.md has the whole table).
// The first nine entries were first read in game by the operator on 2026-10-08; the table gives the same numbers. A container with a fill record
// that is not listed here is skipped and reported, never guessed. An empty container that is not listed has no record to recognise it by, so it is left
// alone without a report.
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
	// the other Literjon grades and the Dewpack (issue #482)
	"Literjon_03": 1100,
	"Literjon_04": 1200,
	"Literjon_05": 1300,
	"Literjon_06": 1400,
	"Literjon_07": 1500,
	"Literjon_08": 1600,
	"Literjon_09": 1700,
	"Dewpack":     250,
}

// stillsuitCapacity is the water store of the worn stillsuits and their kin: the rows of the game's table of fillable items whose FillableTypeRestriction
// is Water and whose bIsContainer is false (issue #482). The suit is not a container the person fills: the game puts the water it recovers into it and
// changes the amount all the time, and the save keeps no liquid type for it. So the editor shows the amount against this capacity but never writes it
// (Fill water containers and Fill all liquids leave a suit alone, without reporting it as unknown). 500 each, 1500 for the high capacity suit.
var stillsuitCapacity = map[string]float64{
	"AdvancedStillsuit":                    500,
	"Combat_Nati_FremenExile04_Top":        500,
	"SimpleStillsuit":                      500,
	"Stillsuit_Choam_01_Top":               500,
	"Stillsuit_Choam_02_Top":               500,
	"Stillsuit_Choam_04_Top":               500,
	"Stillsuit_Choam_05_Top":               500,
	"Stillsuit_Choam_06_Top":               500,
	"Stillsuit_Choam_Unique_Dashed02_Top":  500,
	"Stillsuit_Choam_Unique_Dashed03_Top":  500,
	"Stillsuit_Choam_Unique_Dashed04_Top":  500,
	"Stillsuit_Choam_Unique_Dashed05_Top":  500,
	"Stillsuit_Choam_Unique_Dashed06_Top":  500,
	"Stillsuit_Nati_05_Body":               500,
	"Stillsuit_Nati_06_Body":               500,
	"Stillsuit_Nati_07_Body":               500,
	"Stillsuit_Nati_08_Body":               500,
	"Stillsuit_Nati_Arrakeen05_Body":       500,
	"Stillsuit_Neut_Leaking01_Top":         500,
	"Stillsuit_Neut_Patchy02_Top":          500,
	"Stillsuit_Unique_Armored_01_Top":      500,
	"Stillsuit_Unique_Armored_02_Top":      500,
	"Stillsuit_Unique_Armored_03_Top":      500,
	"Stillsuit_Unique_Armored_04_Top":      500,
	"Stillsuit_Unique_Armored_05_Top":      500,
	"Stillsuit_Unique_Armored_06_Top":      500,
	"Stillsuit_Unique_Efficient_04_Top":    500,
	"Stillsuit_Unique_Efficient_05_Top":    500,
	"Stillsuit_Unique_Efficient_06_Top":    500,
	"Stillsuit_Unique_HighCapacity_06_Top": 1500,
	"Stillsuit_Unique_ThermalSuit_06_Top":  500,
}

// bloodCapacity is how much blood each blood sack holds when full, in the unit of the save's FFillableItemStats.CurrentAmount.
//
// Source: the game's item data, the table of fillable items (DT_ItemTableFillables, the row named by the item's template, field MaxAmount),
// read 2026-10-28 (issue #472; docs/design/2026-10-28-blood-sack-capacity.md has the table and how it was checked against the wiki and saved
// values). Bloodsack_01 and Bloodsack_Unique_Durable_06 were first entered from the largest amount seen in a save (2026-10-23, issue #60); the table
// gives the same numbers. A blood sack template that is not listed here is left alone and reported, because its capacity is unknown.
var bloodCapacity = map[string]float64{
	"Bloodsack_01":                3000,
	"Bloodsack_02":                9000,
	"Bloodsack_03":                15000,
	"Bloodsack_T6":                24000,
	"Bloodsack_Unique_Durable_02": 13000,
	"Bloodsack_Unique_Durable_03": 16000,
	"Bloodsack_Unique_Durable_04": 19000,
	"Bloodsack_Unique_Durable_05": 22000,
	"Bloodsack_Unique_Durable_06": 30000,
}

// waterQuery lists the items of the character's own inventories (backpack, worn gear, loadout) that have stats, with what each holds.
const waterQuery = `select i.id, i.template_id,
		cast(json_extract(i.stats,'$.FFillableItemStats[1].CurrentAmount') as real) amount,
		json_extract(i.stats,'$.FFillableItemStats[1].FillableType') kind,
		json_extract(i.stats,'$.FFillableItemStats') has
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
				when json_extract(stats,'$.FFillableItemStats') is null
				then json_set(stats,'$.FFillableItemStats',json_array(json_array(),json_object('CurrentAmount',?,'FillableType','Water')))
				else json_set(stats,'$.FFillableItemStats[1].CurrentAmount',?,'$.FFillableItemStats[1].FillableType','Water') end
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

package ops

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// waterDeviceCapacity is how much water each base device stores when full (FWaterStorageComponent.m_WaterStored),
// keyed by the lower-case building_type. Same values as the Dune Docker console's refill table
// (dune-awakening-selfhost-docker, console/api/src/duneDb.js WATER_TYPES, MIT, RedBlink).
// A building with a water component that is not listed is skipped and reported.
var waterDeviceCapacity = map[string]int64{
	"watercistern_placeable":                 5000,
	"mediumwatercistern_placeable":           25000,
	"largewatercistern_placeable":            100000,
	"windtrap_placeable":                     500,
	"largewindtrap_placeable":                500,
	"bloodwaterextractor_placeable":          1000,
	"bloodwaterextractionadvanced_placeable": 1000,
}

// RefillBaseWater fills every built Water Cistern, Windtrap, Large Windtrap and Blood Purifier in the save to its capacity.
// Single-player has one world and no per-base ownership worth splitting on, so it works on the whole save.
func (o *Ops) RefillBaseWater() (any, error) {
	rows, err := o.S.Query(`select p.id, lower(p.building_type) kind, fe.entity_id entity,
		cast(json_extract(fe.components,'$.FWaterStorageComponent[1].m_WaterStored') as integer) stored
		from placeables p
		join actor_fgl_entities afe on afe.actor_id=p.id
		join fgl_entities fe on fe.entity_id=afe.entity_id
		where p.is_hologram=0 and json_valid(fe.components,8) and json_type(fe.components,'$.FWaterStorageComponent') is not null order by p.id`)
	if err != nil {
		return nil, err
	}
	type target struct {
		entity any
		cap    int64
		name   string
	}
	var todo []target
	full := 0
	unknown := map[string]int{}
	for _, r := range rows {
		kind := fmt.Sprint(r["kind"])
		capacity, known := waterDeviceCapacity[kind]
		if !known {
			unknown[kind]++
			continue
		}
		if cur, ok := r["stored"].(int64); ok && cur >= capacity {
			full++
			continue
		}
		todo = append(todo, target{r["entity"], capacity, waterName(kind)})
	}
	if len(todo) > 0 { // nothing to fill must not leave an empty pending edit behind
		counts := map[string]int{}
		for _, t := range todo {
			counts[t.name]++
		}
		_, err = o.S.Mutate("refill base water", func(m *save.Mut) error {
			for _, t := range todo {
				if _, err := m.Exec(`update fgl_entities set components=jsonb_set(components,'$.FWaterStorageComponent[1].m_WaterStored',?) where entity_id=?`, t.cap, t.entity); err != nil {
					return err
				}
			}
			m.Desc = "refill base water: " + countSummary(counts)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	names := map[string]int{}
	for _, t := range todo {
		names[t.name]++
	}
	return map[string]any{"ok": true, "filled": len(todo), "alreadyFull": full, "skippedUnknown": sortedCounts(unknown), "summary": countSummary(names)}, nil
}

// fuelSpec is what a power device burns and how full it can be, taken from the Dune Docker console (console/api/src/duneDb.js
// GENERATOR_TYPES and refillBaseGenerators, MIT, RedBlink): topUp are the fuels a refill writes (the first stocked one, else def),
// count are the fuels that count toward the total (a windtrap's four filter tiers all take up its volume), stack is the most the game
// keeps in one row, maxStacks the most rows, and total what the device is filled to. A fuel or spice generator is one stack of 499;
// an omnidirectional wind turbine takes Low-grade lubricant and a directional one Industrial-grade, five stacks of 100 filled to 499;
// a windtrap takes one stack of 5 filters. Unlike the console, which keeps whichever filter tier a windtrap holds, a windtrap here is
// always filled with the best filter for its kind (upgrade): the Standard Filter in a Windtrap, the Advanced Particulate Filter in a
// Large Windtrap, and a lower-grade filter already inside is changed to it.
type fuelSpec struct {
	topUp, count []string
	def          string
	stack        int64
	maxStacks    int
	total        int64
	upgrade      bool
}

var windtrapFilters = []string{"WindTrapFilter1", "WindTrapFilter2", "WindTrapFilter3", "WindTrapFilter4"}

var generatorFuel = map[string]fuelSpec{
	"generator_placeable":                  {[]string{"Oil"}, []string{"Oil"}, "Oil", 499, 1, 499, false},
	"spicegenerator_placeable":             {[]string{"SpicedFuelCell"}, []string{"SpicedFuelCell"}, "SpicedFuelCell", 499, 1, 499, false},
	"windturbineomnidirectional_placeable": {[]string{"WindTurbineLubricant1"}, []string{"WindTurbineLubricant1"}, "WindTurbineLubricant1", 100, 5, 499, false},
	"windturbinedirectional_placeable":     {[]string{"WindTurbineLubricant2"}, []string{"WindTurbineLubricant2"}, "WindTurbineLubricant2", 100, 5, 499, false},
	"windtrap_placeable":                   {[]string{"WindTrapFilter1", "WindTrapFilter2"}, windtrapFilters, "WindTrapFilter2", 5, 1, 5, true},
	"largewindtrap_placeable":              {[]string{"WindTrapFilter3", "WindTrapFilter4"}, windtrapFilters, "WindTrapFilter4", 5, 1, 5, true},
}

// RefillGenerators tops every built power device (generators, wind turbines, windtraps; placeholders that are not built yet are left
// alone) up to full, the way the console's refill does: partial stacks are filled before new rows are added, so a device never ends
// up with more rows than the game would have made itself. Nothing is removed.
func (o *Ops) RefillGenerators() (any, error) {
	rows, err := o.S.Query(`select p.id actor, lower(p.building_type) kind,
		(select v.id from inventories v where v.actor_id=p.id order by v.id limit 1) inv
		from placeables p where p.is_hologram=0 order by p.id`)
	if err != nil {
		return nil, err
	}
	type stackRow struct{ id, size int64 }
	type job struct {
		actor, inv int64
		name       string
		fuel       string
		spec       fuelSpec
		have       []stackRow // stacks of the chosen fuel, in slot order
		held       int64      // everything that counts toward the total
		retemplate int64      // a lower-grade filter row to change to the best one (0: none)
	}
	var jobs []job
	full := 0
	for _, r := range rows {
		spec, known := generatorFuel[fmt.Sprint(r["kind"])]
		if !known || r["inv"] == nil {
			continue
		}
		actor, _ := toInt(r["actor"])
		inv, _ := toInt(r["inv"])
		stocked, err := o.S.Query(`select id, stack_size, template_id from items where inventory_id=? order by position_index, id`, inv)
		if err != nil {
			return nil, err
		}
		fuel := spec.def
		if !spec.upgrade { // the first stocked fuel the device accepts is the one that is topped up
			for _, s := range stocked {
				if t := fmt.Sprint(s["template_id"]); containsFold(spec.topUp, t) {
					fuel = t
					break
				}
			}
		}
		j := job{actor: actor, inv: inv, name: powerName(fmt.Sprint(r["kind"])), fuel: fuel, spec: spec}
		var lower []stackRow
		for _, s := range stocked {
			t := fmt.Sprint(s["template_id"])
			id, _ := toInt(s["id"])
			sz, _ := toInt(s["stack_size"])
			switch {
			case strings.EqualFold(t, fuel):
				j.have = append(j.have, stackRow{id, sz})
				j.held += sz
			case spec.upgrade && containsFold(spec.topUp, t): // a lower grade of the same kind of filter
				lower = append(lower, stackRow{id, sz})
			case containsFold(spec.count, t): // takes up room but is never topped up
				j.held += sz
			}
		}
		if len(j.have) == 0 && len(lower) > 0 { // change the lower-grade filter to the best one instead of adding a second row
			j.retemplate = lower[0].id
			j.have = []stackRow{lower[0]}
			j.held += lower[0].size
		}
		if j.held >= spec.total && j.retemplate == 0 {
			full++
			continue
		}
		jobs = append(jobs, j)
	}
	filled := 0
	var skipped []string
	byName := map[string]int{}
	if len(jobs) > 0 { // nothing to fill must not leave an empty pending edit behind
		_, err = o.S.Mutate("refill generators", func(m *save.Mut) error {
			for _, j := range jobs {
				if j.retemplate != 0 {
					if _, err := m.Exec(`update items set template_id=? where id=?`, j.fuel, j.retemplate); err != nil {
						return err
					}
				}
				need := j.spec.total - j.held
				start := need
				for _, s := range j.have { // top up what is there
					if need <= 0 {
						break
					}
					add := min(need, max(j.spec.stack-s.size, 0))
					if add == 0 {
						continue
					}
					if _, err := m.Exec(`update items set stack_size=? where id=?`, s.size+add, s.id); err != nil {
						return err
					}
					need -= add
				}
				rowsNow := len(j.have)
				for need > 0 && rowsNow < j.spec.maxStacks { // then add stacks
					qty := min(need, j.spec.stack)
					if _, err := giveInTx(m, j.inv, j.fuel, qty, 0); err != nil {
						if err.Error() == "inventory is full" { // other items fill its slots: leave the rest, keep going
							skipped = append(skipped, fmt.Sprintf("%s #%v (no free slot)", j.name, j.actor))
							break
						}
						return fmt.Errorf("%s #%v: %w", j.name, j.actor, err)
					}
					need -= qty
					rowsNow++
				}
				if need < start || j.retemplate != 0 { // one that got nothing (no free slot) is not counted
					filled++
					byName[j.name]++
				}
			}
			m.Desc = "refill power: " + countSummary(byName)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"ok": true, "filled": filled, "alreadyFull": full, "skipped": skipped, "summary": countSummary(byName)}, nil
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// RepairVehicles restores every vehicle module to the in-game repair level: CurrentDurability is raised to the
// module's DecayedMaxDurability (what a repair at a workbench does). Wear that lowered the decayed maximum itself
// is not undone, because the original maximum of each module template is not stored in the save; modules with no
// DecayedMaxDurability are counted and left alone rather than guessed.
func (o *Ops) RepairVehicles() (any, error) { return o.RepairVehiclesBelow(100) }

// RepairVehiclesBelow raises the modules of the player's own vehicles whose durability is below pct percent of their current
// maximum (1 to 100, the console's "Repair Below") up to that maximum. Modules with no recorded maximum are left alone.
func (o *Ops) RepairVehiclesBelow(pct int64) (any, error) { return o.RepairVehiclesWhere(pct, 0) }

// RepairVehiclesWhere is RepairVehiclesBelow limited to one vehicle (vehicleID, which must be one of the player's own); 0 means all
// of them.
func (o *Ops) RepairVehiclesWhere(pct, vehicleID int64) (any, error) {
	if pct < 1 || pct > 100 {
		return nil, errors.New("the repair threshold must be between 1 and 100 percent")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	if vehicleID != 0 {
		if own, _ := o.S.One(`select 1 x from vehicles where id=? and id in (`+ownedVehicleSQL+`)`, vehicleID, p.Controller); own == nil {
			return nil, errors.New("that vehicle is not one of yours")
		}
	}
	rows, err := o.S.Query(`select count(*) total,
		coalesce(sum(case when json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') is not null
			and coalesce(json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability'),0) < json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') * ?/100.0 then 1 else 0 end),0) repairable,
		coalesce(sum(case when json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') is null then 1 else 0 end),0) nomax
		from vehicle_modules where vehicle_id in (`+ownedVehicleSQL+`) and json_valid(stats,8) and json_type(stats,'$.FVehicleModuleDurabilityStats') is not null and (? = 0 or vehicle_id = ?)`, pct, p.Controller, vehicleID, vehicleID)
	if err != nil {
		return nil, err
	}
	var n int64
	_, err = o.S.Mutate("repair vehicles", func(m *save.Mut) error {
		res, err := m.Exec(`update vehicle_modules set stats=jsonb_set(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability',
				json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability'))
			where vehicle_id in (`+ownedVehicleSQL+`) and json_valid(stats,8) and json_type(stats,'$.FVehicleModuleDurabilityStats') is not null
			and json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') is not null
			and coalesce(json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability'),0) < json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') * ?/100.0 and (? = 0 or vehicle_id = ?)`, p.Controller, pct, vehicleID, vehicleID)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		m.Desc = fmt.Sprintf("repair vehicles below %d%%: %d modules", pct, n)
		if vehicleID != 0 {
			m.Desc = fmt.Sprintf("repair vehicle %d below %d%%: %d modules", vehicleID, pct, n)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "modules": rows[0]["total"], "repaired": n, "withoutKnownMax": rows[0]["nomax"]}, nil
}

func sortedCounts(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k, n := range m {
		out = append(out, fmt.Sprintf("%s x%d", strings.TrimSuffix(k, "_placeable"), n))
	}
	sort.Strings(out)
	return out
}

// plural is the plural of an in-game building name ("Windtrap" -> "Windtraps").
func plural(name string) string { return name + "s" }

// countSummary lists what a refill changed by in-game name, most numerous first ("2 Medium Water Cisterns, 1 Windtrap").
func countSummary(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	parts := make([]string, 0, len(names))
	for _, n := range names {
		if counts[n] == 1 {
			parts = append(parts, "1 "+n)
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", counts[n], plural(n)))
		}
	}
	return strings.Join(parts, ", ")
}

// powerName is the in-game name of a power building by its building_type (lower case).
func powerName(kind string) string {
	for _, k := range powerKinds {
		if k.Kind == kind {
			return k.Name
		}
	}
	return kind
}

// waterName is the in-game name of a water building by its building_type (lower case).
func waterName(kind string) string {
	if n, ok := waterNames[kind]; ok {
		return n
	}
	return kind
}

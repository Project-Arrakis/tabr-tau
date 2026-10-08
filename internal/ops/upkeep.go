package ops

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// waterDeviceCapacity is how much water each base device stores when full (FWaterStorageComponent.m_WaterStored),
// keyed by the lower-case building_type. Same values as the Dune Docker console's refill table
// (dune-awakening-selfhost-docker, console/api/src/duneDb.js WATER_TYPES, MIT, RedBlink).
// A device with a water component that is not listed is skipped and reported.
var waterDeviceCapacity = map[string]int64{
	"watercistern_placeable":                 5000,
	"mediumwatercistern_placeable":           25000,
	"largewatercistern_placeable":            100000,
	"windtrap_placeable":                     500,
	"largewindtrap_placeable":                500,
	"bloodwaterextractor_placeable":          1000,
	"bloodwaterextractionadvanced_placeable": 1000,
}

// RefillBaseWater fills every water device in the save (cisterns, windtraps, blood purifiers) to its capacity.
// Single-player has one world and no per-base ownership worth splitting on, so it works on the whole save.
func (o *Ops) RefillBaseWater() (any, error) {
	rows, err := o.S.Query(`select p.id, lower(p.building_type) kind, fe.entity_id entity,
		cast(json_extract(fe.components,'$.FWaterStorageComponent[1].m_WaterStored') as integer) stored
		from placeables p
		join actor_fgl_entities afe on afe.actor_id=p.id
		join fgl_entities fe on fe.entity_id=afe.entity_id
		where json_valid(fe.components,8) and json_type(fe.components,'$.FWaterStorageComponent') is not null order by p.id`)
	if err != nil {
		return nil, err
	}
	type target struct {
		entity any
		cap    int64
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
		todo = append(todo, target{r["entity"], capacity})
	}
	if len(todo) > 0 { // nothing to fill must not leave an empty pending edit behind
		_, err = o.S.Mutate("refill base water", func(m *save.Mut) error {
			for _, t := range todo {
				if _, err := m.Exec(`update fgl_entities set components=jsonb_set(components,'$.FWaterStorageComponent[1].m_WaterStored',?) where entity_id=?`, t.cap, t.entity); err != nil {
					return err
				}
			}
			m.Desc = fmt.Sprintf("refill base water: %d devices", len(todo))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"ok": true, "filled": len(todo), "alreadyFull": full, "skippedUnknown": sortedCounts(unknown)}, nil
}

// generatorFuel is the fuel a generator burns and how much a full tank is (one stack; the stack limit is from
// the game's item data and 500 x 0.2 volume fills the generator's 100 volume exactly).
// Wind turbines take lubricant in up to five stacks and are not handled until verified in game.
var generatorFuel = map[string]struct {
	template string
	full     int64
}{
	"generator_placeable":      {"Oil", 500},
	"spicegenerator_placeable": {"SpicedFuelCell", 500},
}

// RefillGenerators tops the fuel of every fuel and spice generator up to a full stack, adding the stack when the
// generator is empty.
func (o *Ops) RefillGenerators() (any, error) {
	rows, err := o.S.Query(`select p.id actor, lower(p.building_type) kind, v.id inv
		from placeables p join inventories v on v.actor_id=p.id and v.inventory_type=3 order by p.id`)
	if err != nil {
		return nil, err
	}
	type job struct {
		actor, inv any
		fuel       string
		full       int64
		stack      any // existing stack to top up; nil when the generator has none
	}
	var jobs []job
	full := 0
	for _, r := range rows {
		spec, known := generatorFuel[fmt.Sprint(r["kind"])]
		if !known {
			continue
		}
		cur, err := o.S.Query(`select id, stack_size from items where inventory_id=? and template_id=? order by stack_size desc limit 1`, r["inv"], spec.template)
		if err != nil {
			return nil, err
		}
		switch {
		case len(cur) == 0:
			jobs = append(jobs, job{r["actor"], r["inv"], spec.template, spec.full, nil})
		case cur[0]["stack_size"].(int64) >= spec.full:
			full++
		default:
			jobs = append(jobs, job{r["actor"], r["inv"], spec.template, spec.full, cur[0]["id"]})
		}
	}
	filled := 0
	var skipped []string
	if len(jobs) > 0 { // nothing to fill must not leave an empty pending edit behind
		_, err = o.S.Mutate("refill generators", func(m *save.Mut) error {
			for _, j := range jobs {
				if j.stack != nil {
					if _, err := m.Exec(`update items set stack_size=? where id=?`, j.full, j.stack); err != nil {
						return err
					}
					filled++
					continue
				}
				if _, err := giveInTx(m, j.inv.(int64), j.fuel, j.full, 0); err != nil {
					if err.Error() == "inventory is full" { // other items fill its slots: leave this one, keep going
						skipped = append(skipped, fmt.Sprintf("generator %v (no free slot)", j.actor))
						continue
					}
					return fmt.Errorf("generator %v: %w", j.actor, err)
				}
				filled++
			}
			m.Desc = fmt.Sprintf("refill generators: %d filled", filled)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return map[string]any{"ok": true, "filled": filled, "alreadyFull": full, "skipped": skipped}, nil
}

// RepairVehicles restores every vehicle module to the in-game repair level: CurrentDurability is raised to the
// module's DecayedMaxDurability (what a repair at a workbench does). Wear that lowered the decayed maximum itself
// is not undone, because the original maximum of each module template is not stored in the save; modules with no
// DecayedMaxDurability are counted and left alone rather than guessed.
func (o *Ops) RepairVehicles() (any, error) {
	rows, err := o.S.Query(`select count(*) total,
		coalesce(sum(case when json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') is not null
			and coalesce(json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability'),0) < json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') then 1 else 0 end),0) repairable,
		coalesce(sum(case when json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') is null then 1 else 0 end),0) nomax
		from vehicle_modules where json_valid(stats,8) and json_type(stats,'$.FVehicleModuleDurabilityStats') is not null`)
	if err != nil {
		return nil, err
	}
	var n int64
	_, err = o.S.Mutate("repair vehicles", func(m *save.Mut) error {
		res, err := m.Exec(`update vehicle_modules set stats=jsonb_set(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability',
				json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability'))
			where json_valid(stats,8) and json_type(stats,'$.FVehicleModuleDurabilityStats') is not null
			and json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability') is not null
			and coalesce(json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability'),0) < json_extract(stats,'$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability')`)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		m.Desc = fmt.Sprintf("repair vehicles: %d modules", n)
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

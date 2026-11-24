// Command buildcostsgen makes internal/catalog/buildcosts.json (#1176) from dumps of the game's own building data tables. The file says what one building piece and one placeable
// costs to place: for every type, the item template ids and the whole numbers of them. Recycle base reads it to give back what a base cost. It reads only JSON files, never the
// game: producing the dumps takes a reader of the game's content files, which this repository does not contain (see internal/catalog/PROVENANCE.md, "buildcosts.json").
//
// Inputs: -dumps names a folder that holds one JSON file per table, named as the table is (DT_BuildingData_Sardaukar.json and so on). A file is one object, row name to row; a
// map property of a row is a list of {"key": ..., "value": ...} pairs, in the table's own order. The tables read:
//
//	DT_BuildingData_<Set>              one row per building piece type (the row name is building_instances.building_type of a save): m_BuildableGroupType, m_BuildableTier
//	DT_PlaceableData_<Kind>            one row per placeable type (the row name is placeables.building_type): the same two fields. DT_PlaceableData_Vehicle is not read:
//	                                   vehicles are not part of a base's recycle
//	DT_BuildableGroupData_Building     one row per group of pieces: m_BuildableTierData[<tier>].m_PlacementCosts, item id to amount
//	DT_BuildableGroupData_Placeable    the same for placeables
//	DT_BuildableMapRegion              one row per map: m_DefaultRegionData.m_DefaultModifiers.m_PlacementCostMultiplier (0.5 for the Deep Desert: a piece costs half there;
//	                                   -1 means the game's default, which is the full cost)
//
// The rule (repeatable, and the same on every run): cost(type) = group table[row.m_BuildableGroupType].tiers[row.m_BuildableTier].m_PlacementCosts. A type whose row names no
// group or tier, whose group or tier is not in the group table, or whose cost list is empty has no entry in the file (Recycle base refuses a base that holds such a type and
// names it; nothing is ever given a cost of nothing). A type that two tables list with different costs is left out as well and printed. An amount that is not a whole number
// above 0 is an error. The file holds the game's identifiers and whole numbers only: no names, text, images or code.
//
// Usage: go run ./tools/buildcostsgen -dumps <folder> -build 26592269 -read 2026-11-23 -out internal/catalog/buildcosts.json
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// File is internal/catalog/buildcosts.json.
type File struct {
	Build string `json:"build"` // the game build the tables were read from
	Read  string `json:"read"`  // the date they were read
	// PlacementCostPercent is, for the maps that have their own, the share of the cost a piece or placeable costs there, in percent (the Deep Desert: 50).
	PlacementCostPercent map[string]int64 `json:"placementCostPercent"`
	// Pieces and Placeables are type to item template id to amount, for one unit.
	Pieces     map[string]map[string]int64 `json:"pieces"`
	Placeables map[string]map[string]int64 `json:"placeables"`
}

// Report is what a run left out, for the person who runs it.
type Report struct {
	NoCost    []string // types with a row and no cost
	Conflicts []string // types two tables list with different costs
}

type pair struct {
	Key   json.RawMessage `json:"key"`
	Value json.RawMessage `json:"value"`
}

type typeRow struct {
	Group json.RawMessage `json:"m_BuildableGroupType"`
	Tier  json.RawMessage `json:"m_BuildableTier"`
}

type groupRow struct {
	Tiers json.RawMessage `json:"m_BuildableTierData"`
}

func str(raw json.RawMessage) string {
	var s string
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

func readTable(path string, into any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, into); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

// groupCosts reads a group table into group -> tier -> item -> amount.
func groupCosts(path string) (map[string]map[string]map[string]int64, error) {
	var rows map[string]groupRow
	if err := readTable(path, &rows); err != nil {
		return nil, err
	}
	out := map[string]map[string]map[string]int64{}
	for g, r := range rows {
		var tiers []pair
		if len(r.Tiers) == 0 || json.Unmarshal(r.Tiers, &tiers) != nil {
			continue // a group with no readable tiers gives no cost: its types are left out and reported
		}
		out[g] = map[string]map[string]int64{}
		for _, t := range tiers {
			var v struct {
				Costs []pair `json:"m_PlacementCosts"`
			}
			tier := str(t.Key)
			if tier == "" || json.Unmarshal(t.Value, &v) != nil {
				continue
			}
			cost := map[string]int64{}
			for _, c := range v.Costs {
				item := str(c.Key)
				var n float64
				if item == "" || json.Unmarshal(c.Value, &n) != nil {
					return nil, fmt.Errorf("%s: group %s tier %s has a cost entry that is not an item id and a number", filepath.Base(path), g, tier)
				}
				if n <= 0 || n != math.Trunc(n) || n > 1e9 {
					return nil, fmt.Errorf("%s: group %s tier %s costs %v of %s, which is not a whole number above 0", filepath.Base(path), g, tier, n, item)
				}
				cost[item] += int64(n)
			}
			out[g][tier] = cost
		}
	}
	return out, nil
}

func sameCost(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// typeCosts reads every type table that matches the pattern and looks each row up in the group costs.
func typeCosts(dir, pattern string, skip func(name string) bool, groups map[string]map[string]map[string]int64, rep *Report) (map[string]map[string]int64, error) {
	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	out := map[string]map[string]int64{}
	bad := map[string]bool{}
	none := map[string]bool{}
	n := 0
	for _, f := range files {
		if skip != nil && skip(filepath.Base(f)) {
			continue
		}
		n++
		var rows map[string]typeRow
		if err := readTable(f, &rows); err != nil {
			return nil, err
		}
		names := make([]string, 0, len(rows))
		for k := range rows {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, name := range names {
			r := rows[name]
			typ := name
			if i := strings.Index(typ, "#dup"); i >= 0 { // a row name a table lists twice: the dump marks the second one
				typ = typ[:i]
			}
			cost := groups[str(r.Group)][str(r.Tier)]
			if len(cost) == 0 {
				none[typ] = true
				continue
			}
			if old, has := out[typ]; has && !sameCost(old, cost) {
				bad[typ] = true
				continue
			}
			out[typ] = cost
		}
	}
	if n == 0 {
		return nil, fmt.Errorf("no table matches %s in %s", pattern, dir)
	}
	for typ := range bad {
		delete(out, typ)
		rep.Conflicts = append(rep.Conflicts, typ)
	}
	for typ := range none {
		if _, has := out[typ]; has { // one table gives the type a cost and another none: that is a disagreement too
			delete(out, typ)
			rep.Conflicts = append(rep.Conflicts, typ)
		} else if !bad[typ] {
			rep.NoCost = append(rep.NoCost, typ)
		}
	}
	return out, nil
}

// Generate reads the dumps in dir and makes the file.
func Generate(dir, build, read string) (*File, *Report, error) {
	if build == "" || read == "" {
		return nil, nil, errors.New("the game build and the date the tables were read are needed")
	}
	rep := &Report{}
	bg, err := groupCosts(filepath.Join(dir, "DT_BuildableGroupData_Building.json"))
	if err != nil {
		return nil, nil, err
	}
	pg, err := groupCosts(filepath.Join(dir, "DT_BuildableGroupData_Placeable.json"))
	if err != nil {
		return nil, nil, err
	}
	f := &File{Build: build, Read: read, PlacementCostPercent: map[string]int64{}}
	if f.Pieces, err = typeCosts(dir, "DT_BuildingData_*.json", nil, bg, rep); err != nil {
		return nil, nil, err
	}
	notVehicles := func(name string) bool { return name == "DT_PlaceableData_Vehicle.json" }
	if f.Placeables, err = typeCosts(dir, "DT_PlaceableData_*.json", notVehicles, pg, rep); err != nil {
		return nil, nil, err
	}
	var regions map[string]struct {
		Map    string `json:"m_Map"`
		Region struct {
			Mod struct {
				Mult float64 `json:"m_PlacementCostMultiplier"`
			} `json:"m_DefaultModifiers"`
		} `json:"m_DefaultRegionData"`
	}
	if err := readTable(filepath.Join(dir, "DT_BuildableMapRegion.json"), &regions); err != nil {
		return nil, nil, err
	}
	for _, r := range regions {
		if r.Map == "" || r.Region.Mod.Mult <= 0 { // -1: the game's default, the full cost
			continue
		}
		pct := r.Region.Mod.Mult * 100
		if pct != math.Trunc(pct) {
			return nil, nil, fmt.Errorf("the map %s has a placement cost multiplier of %v, which is not a whole percentage", r.Map, r.Region.Mod.Mult)
		}
		f.PlacementCostPercent[r.Map] = int64(pct)
	}
	sort.Strings(rep.NoCost)
	sort.Strings(rep.Conflicts)
	return f, rep, nil
}

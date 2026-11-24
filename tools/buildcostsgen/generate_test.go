package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type obj = map[string]any

func kv(k string, v any) obj { return obj{"key": k, "value": v} }

func tier(name string, costs ...obj) obj {
	return kv(name, obj{"m_PlacementCosts": costs, "m_MaxHealth": 100.0})
}

func write(t *testing.T, dir, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// dumps writes a small set of tables in the shape the dumps have. extra replaces or adds files.
func dumps(t *testing.T, extra map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]any{
		"DT_BuildableGroupData_Building.json": obj{
			"Wall":       obj{"m_BuildableTierData": []obj{tier("Tier1", kv("Stone", 10)), tier("Tier2", kv("Plastone", 12))}},
			"Foundation": obj{"m_BuildableTierData": []obj{tier("Tier1", kv("Stone", 15))}},
			"Free":       obj{"m_BuildableTierData": []obj{tier("Tier1")}},
		},
		"DT_BuildableGroupData_Placeable.json": obj{
			"Totem":   obj{"m_BuildableTierData": []obj{tier("Tier1", kv("ScrapMetal", 40))}},
			"Cistern": obj{"m_BuildableTierData": []obj{tier("Tier2", kv("DuraluminumRod", 115), kv("Silicone", 120))}},
		},
		"DT_BuildingData_SetA.json": obj{
			"A_Wall":       obj{"m_BuildableGroupType": "Wall", "m_BuildableTier": "Tier1"},
			"A_Wall_Hard":  obj{"m_BuildableGroupType": "Wall", "m_BuildableTier": "Tier2"},
			"A_Foundation": obj{"m_BuildableGroupType": "Foundation", "m_BuildableTier": "Tier1"},
			"A_NoTier":     obj{"m_BuildableGroupType": "Wall", "m_BuildableTier": "Tier9"},
			"A_NoGroup":    obj{"m_BuildableGroupType": "Nowhere", "m_BuildableTier": "Tier1"},
			"A_Free":       obj{"m_BuildableGroupType": "Free", "m_BuildableTier": "Tier1"},
			"Twice":        obj{"m_BuildableGroupType": "Wall", "m_BuildableTier": "Tier1"},
			"Same":         obj{"m_BuildableGroupType": "Wall", "m_BuildableTier": "Tier1"},
		},
		"DT_BuildingData_SetB.json": obj{
			"Twice": obj{"m_BuildableGroupType": "Wall", "m_BuildableTier": "Tier2"},
			"Same":  obj{"m_BuildableGroupType": "Wall", "m_BuildableTier": "Tier1"},
		},
		"DT_PlaceableData_Functional.json": obj{
			"Totem_Placeable":   obj{"m_BuildableGroupType": "Totem", "m_BuildableTier": "Tier1"},
			"Cistern_Placeable": obj{"m_BuildableGroupType": "Cistern", "m_BuildableTier": "Tier2"},
			"Ladder_Placeable":  obj{"m_BuildableGroupType": "Totem", "m_BuildableTier": "Tier0"},
		},
		"DT_PlaceableData_Vehicle.json": obj{
			"Buggy_Placeable": obj{"m_BuildableGroupType": "Totem", "m_BuildableTier": "Tier1"},
		},
		"DT_BuildableMapRegion.json": obj{
			"NewRow":   obj{"m_Map": "HaggaBasin", "m_DefaultRegionData": obj{"m_DefaultModifiers": obj{"m_RefundPercentage": -1.0, "m_PlacementCostMultiplier": -1.0}}},
			"NewRow_0": obj{"m_Map": "DeepDesert", "m_DefaultRegionData": obj{"m_DefaultModifiers": obj{"m_RefundPercentage": -1.0, "m_PlacementCostMultiplier": 0.5}}},
		},
	}
	for name, v := range extra {
		files[name] = v
	}
	for name, v := range files {
		write(t, dir, name, v)
	}
	return dir
}

func TestGenerateLooksEveryTypeUpInItsGroupAndTier(t *testing.T) {
	f, rep, err := Generate(dumps(t, nil), "123", "2026-11-23")
	if err != nil {
		t.Fatal(err)
	}
	if f.Build != "123" || f.Read != "2026-11-23" {
		t.Fatalf("build and date: %+v", f)
	}
	if got, want := fmt.Sprint(f.Pieces), "map[A_Foundation:map[Stone:15] A_Wall:map[Stone:10] A_Wall_Hard:map[Plastone:12] Same:map[Stone:10]]"; got != want {
		t.Fatalf("pieces: %s, want %s", got, want)
	}
	// vehicles are not read; a placeable is looked up in the placeable groups
	if got, want := fmt.Sprint(f.Placeables), "map[Cistern_Placeable:map[DuraluminumRod:115 Silicone:120] Totem_Placeable:map[ScrapMetal:40]]"; got != want {
		t.Fatalf("placeables: %s, want %s", got, want)
	}
	// only a map with its own multiplier is listed, as a whole percentage
	if fmt.Sprint(f.PlacementCostPercent) != "map[DeepDesert:50]" {
		t.Fatalf("cost percent: %v", f.PlacementCostPercent)
	}
	// a type with no cost is left out and reported, never given a cost of nothing; so is a type two tables disagree on
	if got := strings.Join(rep.NoCost, ","); got != "A_Free,A_NoGroup,A_NoTier,Ladder_Placeable" {
		t.Fatalf("no cost: %s", got)
	}
	if got := strings.Join(rep.Conflicts, ","); got != "Twice" {
		t.Fatalf("conflicts: %s", got)
	}
}

func TestGenerateRefusesAmountsThatAreNotWholeNumbersAboveZero(t *testing.T) {
	for _, bad := range []any{0, -3, 2.5, "ten"} {
		dir := dumps(t, map[string]any{"DT_BuildableGroupData_Building.json": obj{"Wall": obj{"m_BuildableTierData": []obj{tier("Tier1", kv("Stone", bad))}}}})
		if _, _, err := Generate(dir, "1", "2026-11-23"); err == nil {
			t.Errorf("an amount of %v was accepted", bad)
		}
	}
	if _, _, err := Generate(dumps(t, nil), "", "2026-11-23"); err == nil {
		t.Error("a file with no game build was made")
	}
	if _, _, err := Generate(t.TempDir(), "1", "2026-11-23"); err == nil {
		t.Error("an empty folder gave a file")
	}
	dir := dumps(t, map[string]any{"DT_BuildableMapRegion.json": obj{"R": obj{"m_Map": "Odd", "m_DefaultRegionData": obj{"m_DefaultModifiers": obj{"m_PlacementCostMultiplier": 0.333}}}}})
	if _, _, err := Generate(dir, "1", "2026-11-23"); err == nil {
		t.Error("a multiplier that is not a whole percentage was accepted")
	}
}

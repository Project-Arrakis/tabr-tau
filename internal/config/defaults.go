package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Defaults of the settings in ServerCustomSettings.ini (issue #348).
//
// Source: the default value, the allowed range and the allowed words of each key are copied from the Dune Docker console's settings tables
// (https://github.com/Red-Blink/dune-awakening-selfhost-docker, MIT, copyright RedBlink; the notice is in THIRD-PARTY-NOTICES.md):
// SERVER_CUSTOM_FIELDS, SERVER_CUSTOM_NUMERIC_BOUNDS and SERVER_CUSTOM_ENUM_VALUES in runtime/scripts/usersettings.py, whose defaults agree with the second list in
// console/api/src/services/publicDirectory.js for all 45 keys (checked 2026-10-24). The console says its constraints are the ones Funcom publishes with the settings class
// UserServerCustomSettings, the class this file is the single-player copy of. That the single-player game uses the same defaults is the one thing not checked
// against a running single-player game: the page says where the numbers come from.
//
// A key that is not in this table has no default the editor can vouch for (DifficultyLevel, ColdBuildupRate, anything added by hand): it is never reset, and the page
// says "default unknown" for it. The table is never extended by guessing.

// Kind says how a value is compared with its default.
type Kind string

const (
	KindFloat Kind = "number" // 1, 1.0 and 1.000000 are the same value
	KindInt   Kind = "whole"
	KindBool  Kind = "switch" // True and true are the same value
	KindEnum  Kind = "word"   // one of Options, compared without regard to case
)

// Default is the verified default of one key.
type Default struct {
	Key      string
	Value    string // as the game writes it: "1.000000", "True", "Limited"
	Kind     Kind
	Min, Max *float64 // the range the console documents; nil where there is no bound
	Options  []string
}

func f64(v float64) *float64 { return &v }

// DefaultSection is the one section of ServerCustomSettings.ini the table covers.
const DefaultSection = "/Script/DuneSandbox.UserServerCustomSettings"

// customSettingsDefaults are the 45 keys of the section whose default is known, in the console's order.
var customSettingsDefaults = []Default{
	{Key: "PVPMode", Value: "Limited", Kind: KindEnum, Options: []string{"NoPVP", "Limited", "FullPVP"}},
	{Key: "GatheringAmount", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "CraftingCost", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "WaterExtractionRate", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "CraftingTimeMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(5.0)},
	{Key: "BuildingCostMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "ResourceRespawnSpeed", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "LootRespawnSpeed", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "FuelBurnTimeMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "InventoryVolumeMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "PlayerDamageToPlayer", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "PlayerDamageToNPC", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "PlayerDamageToVehicle", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "PlayerStaminaDrain", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "IntelPointsGainMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "NPCHealth", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "NPCDamageToPlayer", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "NPCDamageToNPC", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "NPCRespawnMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "PVPDamageStructures", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "GlobalXpMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "CombatXp", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "GatheringXp", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "MissionXp", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "ItemDurabilityDrainMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "bEnableItemMaxDurabilityLoss", Value: "True", Kind: KindBool},
	{Key: "PlayerShieldDamageAbsorptionMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "NPCShieldDamageAbsorptionMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: f64(10.0)},
	{Key: "HeatBuildupRate", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "ThirstMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "DropEquipmentOnDeath", Value: "Default", Kind: KindEnum, Options: []string{"All", "Backpack", "Default", "None"}},
	{Key: "bAllowDynamicBuildingDamage", Value: "True", Kind: KindBool},
	{Key: "bAllowSandstorms", Value: "True", Kind: KindBool},
	{Key: "bAllowSandworms", Value: "True", Kind: KindBool},
	{Key: "SandwormConsequences", Value: "All", Kind: KindEnum, Options: []string{"All", "Backpack", "Default", "None"}},
	{Key: "PlayerDeathLootRule", Value: "DependsOnSecurityZone", Kind: KindEnum, Options: []string{"DependsOnSecurityZone", "NeverAllowOtherPlayers", "AlwaysAllowOtherPlayers"}},
	{Key: "bIsBuildingRestrictionsEnabled", Value: "True", Kind: KindBool},
	{Key: "FiefdomLimit", Value: "3", Kind: KindInt, Min: f64(0), Max: f64(10)},
	{Key: "BuildingPieceLimitMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.1), Max: nil},
	{Key: "bBuildingInfiniteStability", Value: "False", Kind: KindBool},
	{Key: "BaseBackupToolTimeRestriction", Value: "16.000000", Kind: KindFloat, Min: f64(0.2), Max: nil},
	{Key: "LandsraadContributionMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "LandsraadSpecializationXpMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "LandsraadFactionStandingMultiplier", Value: "1.000000", Kind: KindFloat, Min: f64(0.0), Max: f64(10.0)},
	{Key: "bLandsraadDisableDecreeRerollLimit", Value: "False", Kind: KindBool},
}

var defaultByKey = func() map[string]Default {
	m := make(map[string]Default, len(customSettingsDefaults))
	for _, d := range customSettingsDefaults {
		m[d.Key] = d
	}
	return m
}()

// DefaultFor returns the verified default of key in the named file's settings section.
func DefaultFor(file, section, key string) (Default, bool) {
	if file != "ServerCustomSettings.ini" || section != DefaultSection {
		return Default{}, false
	}
	d, ok := defaultByKey[key]
	return d, ok
}

// Equals reports whether value is the default, by the kind of the setting: numbers by value, switches and words without regard to case.
func (d Default) Equals(value string) bool {
	v := strings.TrimSpace(value)
	switch d.Kind {
	case KindFloat, KindInt:
		a, err1 := strconv.ParseFloat(v, 64)
		b, err2 := strconv.ParseFloat(d.Value, 64)
		return err1 == nil && err2 == nil && a == b
	default:
		return strings.EqualFold(v, d.Value)
	}
}

// DefaultInfo is what the page shows for one entry of a file that has defaults.
type DefaultInfo struct {
	Line      int      `json:"line"`
	Key       string   `json:"key"`
	Known     bool     `json:"known"` // a verified default exists for the key
	Default   string   `json:"default,omitempty"`
	AtDefault bool     `json:"atDefault"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Options   []string `json:"options,omitempty"`
}

// ResetCounts says what a reset of a whole file changes and leaves alone.
type ResetCounts struct {
	Change     int `json:"change"`     // known keys that are not at their default
	AtDefault  int `json:"atDefault"`  // known keys that already are
	NoDefault  int `json:"noDefault"`  // keys of the section with no verified default (hand-added, or not in the table)
	OtherParts int `json:"otherParts"` // entries in other sections of the file
}

// hasDefaults says whether the file has a table of defaults.
func hasDefaults(name string) bool { return name == "ServerCustomSettings.ini" }

// defaultsOf describes every entry of the parsed file and counts what a reset of all would do.
func defaultsOf(name string, entries []Entry) ([]DefaultInfo, ResetCounts) {
	infos := []DefaultInfo{}
	var c ResetCounts
	for _, e := range entries {
		d, ok := DefaultFor(name, e.Section, e.Key)
		info := DefaultInfo{Line: e.Line, Key: e.Key, Known: ok}
		switch {
		case ok:
			info.Default, info.Min, info.Max, info.Options = d.Value, d.Min, d.Max, d.Options
			info.AtDefault = d.Equals(e.Value)
			if info.AtDefault {
				c.AtDefault++
			} else {
				c.Change++
			}
		case e.Section == DefaultSection:
			c.NoDefault++
		default:
			c.OtherParts++
		}
		infos = append(infos, info)
	}
	return infos, c
}

// ResetResult is the answer of a reset.
type ResetResult struct {
	Changed int `json:"changed"`
	ResetCounts
}

// Reset puts settings back to the verified default: the one at line (checked against key, as Set does) when line >= 0, else every known setting of the file that is not at its
// default. expect (the number the page showed) must equal the number that would change, or nothing is written ("file changed; reload"). It is one edit: one backup, one write.
// A setting already at its default is not rewritten, and when nothing changes the file is not written at all. Unknown keys, other sections, comments, the order and the line
// endings are left as they were; the default is written as the game writes it ("1.000000"), not by removing the key, because the game's own file lists every key.
func (d Dir) Reset(name string, line int, key string, expect int) (ResetResult, error) {
	editMu.Lock()
	defer editMu.Unlock()
	var res ResetResult
	if !hasDefaults(name) {
		return res, errors.New("this file has no table of defaults")
	}
	dc, err := d.load(name)
	if err != nil {
		return res, err
	}
	ents := dc.entries()
	_, res.ResetCounts = defaultsOf(name, ents)
	var todo []Entry
	if line >= 0 {
		var found *Entry
		for i := range ents {
			if ents[i].Line == line {
				found = &ents[i]
			}
		}
		if found == nil || found.Key != key {
			return res, errors.New("file changed; reload")
		}
		def, ok := DefaultFor(name, found.Section, found.Key)
		if !ok {
			return res, fmt.Errorf("default unknown for %s: nothing is reset", found.Key)
		}
		if !def.Equals(found.Value) {
			todo = append(todo, *found)
		}
	} else {
		if expect != res.Change {
			return res, errors.New("file changed; reload")
		}
		for _, e := range ents {
			if def, ok := DefaultFor(name, e.Section, e.Key); ok && !def.Equals(e.Value) {
				todo = append(todo, e)
			}
		}
	}
	if len(todo) == 0 {
		return res, nil
	}
	for _, e := range todo {
		def, _ := DefaultFor(name, e.Section, e.Key)
		dc.lines[e.Line] = e.Key + "=" + def.Value
	}
	if err := d.write(name, dc.String()); err != nil {
		return res, err
	}
	res.Changed = len(todo)
	return res, nil
}

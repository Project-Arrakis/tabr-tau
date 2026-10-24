package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sec = "[/Script/DuneSandbox.UserServerCustomSettings]"

// a file with values at, above and below their defaults, a key with no verified default (DifficultyLevel, ColdBuildupRate), a key added by hand, a comment and another section
const resetFile = sec + "\r\n" +
	"DifficultyLevel=Custom\r\n" + // no default known
	"PVPMode=FullPVP\r\n" + // word, not the default (Limited)
	"GatheringAmount=4.000000\r\n" + // above 1
	"CraftingCost=0.500000\r\n" + // below 1
	"ResourceRespawnSpeed=1.0\r\n" + // the default written another way
	"LootRespawnSpeed=1.000000\r\n" + // the default
	"; kept comment\r\n" +
	"bAllowSandstorms=false\r\n" + // switch, not the default (True)
	"bAllowSandworms=TRUE\r\n" + // the default in another case
	"ColdBuildupRate=1.000000\r\n" + // no default known
	"MyOwnKey=7\r\n" + // added by hand
	"FiefdomLimit=6\r\n" + // whole number
	"\r\n[Other]\r\nGatheringAmount=9\r\n" // same key name in another section: not touched

const custom = "ServerCustomSettings.ini"

func TestTheTableMatchesTheConsoleKeysAndValues(t *testing.T) {
	if len(customSettingsDefaults) != 45 || len(defaultByKey) != 45 {
		t.Fatalf("the table has %d entries (%d distinct keys), want the console's 45", len(customSettingsDefaults), len(defaultByKey))
	}
	for _, d := range customSettingsDefaults {
		switch d.Kind {
		case KindFloat:
			if !strings.HasSuffix(d.Value, "000000") {
				t.Errorf("%s: %q is not written the way the game writes a float", d.Key, d.Value)
			}
		case KindEnum:
			ok := false
			for _, o := range d.Options {
				ok = ok || o == d.Value
			}
			if !ok {
				t.Errorf("%s: default %q is not one of its words %v", d.Key, d.Value, d.Options)
			}
		case KindBool:
			if d.Value != "True" && d.Value != "False" {
				t.Errorf("%s: %q", d.Key, d.Value)
			}
		}
		if d.Min != nil && d.Max != nil && (*d.Min > *d.Max) {
			t.Errorf("%s: range reversed", d.Key)
		}
	}
	// a sample checked by eye against both console tables
	for key, want := range map[string]string{"PVPMode": "Limited", "FiefdomLimit": "3", "BaseBackupToolTimeRestriction": "16.000000", "bIsBuildingRestrictionsEnabled": "True", "bBuildingInfiniteStability": "False", "PlayerDeathLootRule": "DependsOnSecurityZone"} {
		if got := defaultByKey[key].Value; got != want {
			t.Errorf("%s default %q, want %q", key, got, want)
		}
	}
	for _, unknown := range []string{"DifficultyLevel", "ColdBuildupRate"} {
		if _, ok := defaultByKey[unknown]; ok {
			t.Errorf("%s has no verified default and must not be in the table", unknown)
		}
	}
}

func TestReadListsDefaultsAndWhatAResetWouldDo(t *testing.T) {
	d := setup(t, map[string]string{custom: resetFile, "Game.ini": resetFile})
	r, err := d.Read(custom)
	if err != nil {
		t.Fatal(err)
	}
	counts := r["resetCounts"].(ResetCounts)
	// not at default: PVPMode, GatheringAmount, CraftingCost, bAllowSandstorms, FiefdomLimit = 5; at default: ResourceRespawnSpeed, LootRespawnSpeed, bAllowSandworms = 3
	if counts.Change != 5 || counts.AtDefault != 3 || counts.NoDefault != 3 || counts.OtherParts != 1 {
		t.Fatalf("counts: %+v", counts)
	}
	for _, i := range r["defaults"].([]DefaultInfo) {
		switch i.Key {
		case "DifficultyLevel", "ColdBuildupRate", "MyOwnKey":
			if i.Known || i.Default != "" {
				t.Errorf("%s must be 'default unknown': %+v", i.Key, i)
			}
		case "ResourceRespawnSpeed", "bAllowSandworms", "LootRespawnSpeed":
			if !i.Known || !i.AtDefault {
				t.Errorf("%s is at its default: %+v", i.Key, i)
			}
		case "PVPMode":
			if i.Known && (i.AtDefault || i.Default != "Limited" || len(i.Options) != 3) {
				t.Errorf("PVPMode: %+v", i)
			}
		case "GatheringAmount":
			if i.Line == 3 && (i.AtDefault || i.Default != "1.000000" || i.Min == nil || *i.Min != 0.1 || *i.Max != 10) {
				t.Errorf("GatheringAmount: %+v", i)
			}
		}
	}
	// a file without a table says nothing about defaults
	g, _ := d.Read("Game.ini")
	if _, has := g["defaults"]; has {
		t.Error("Game.ini has no defaults")
	}
}

func TestResetOneChangesOnlyThatLine(t *testing.T) {
	d := setup(t, map[string]string{custom: resetFile})
	res, err := d.Reset(custom, 3, "GatheringAmount", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed != 1 {
		t.Fatalf("changed %d", res.Changed)
	}
	want := strings.Replace(resetFile, "GatheringAmount=4.000000", "GatheringAmount=1.000000", 1)
	if got := read(t, d, custom); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	if n := len(d.Backups(custom)); n != 1 {
		t.Fatalf("backups: %d, want 1", n)
	}
}

func TestResetOneRefusesStaleUnknownAndDoesNotWriteWhenAlreadyDefault(t *testing.T) {
	d := setup(t, map[string]string{custom: resetFile})
	if _, err := d.Reset(custom, 3, "CraftingCost", 0); err == nil || !strings.Contains(err.Error(), "reload") {
		t.Errorf("wrong key for the line: %v", err)
	}
	if _, err := d.Reset(custom, 99, "CraftingCost", 0); err == nil {
		t.Error("line past the end")
	}
	if _, err := d.Reset(custom, 1, "DifficultyLevel", 0); err == nil || !strings.Contains(err.Error(), "default unknown") {
		t.Errorf("a key with no verified default: %v", err)
	}
	if _, err := d.Reset(custom, 11, "MyOwnKey", 0); err == nil || !strings.Contains(err.Error(), "default unknown") {
		t.Errorf("a key added by hand: %v", err)
	}
	// the same key name in another section has no default there
	if _, err := d.Reset(custom, 15, "GatheringAmount", 0); err == nil {
		t.Error("GatheringAmount in [Other] is not a game setting")
	}
	res, err := d.Reset(custom, 6, "LootRespawnSpeed", 0)
	if err != nil || res.Changed != 0 {
		t.Fatalf("already at default: %+v %v", res, err)
	}
	if got := read(t, d, custom); got != resetFile {
		t.Error("nothing should have been written")
	}
	if len(d.Backups(custom)) != 0 {
		t.Error("no write, no backup")
	}
}

func TestResetAllChangesOnlyKnownKeysAndKeepsTheRest(t *testing.T) {
	d := setup(t, map[string]string{custom: resetFile})
	res, err := d.Reset(custom, -1, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed != 5 || res.AtDefault != 3 || res.NoDefault != 3 || res.OtherParts != 1 {
		t.Fatalf("result: %+v", res)
	}
	want := sec + "\r\n" +
		"DifficultyLevel=Custom\r\n" +
		"PVPMode=Limited\r\n" +
		"GatheringAmount=1.000000\r\n" +
		"CraftingCost=1.000000\r\n" +
		"ResourceRespawnSpeed=1.0\r\n" + // already the default: not rewritten
		"LootRespawnSpeed=1.000000\r\n" +
		"; kept comment\r\n" +
		"bAllowSandstorms=True\r\n" +
		"bAllowSandworms=TRUE\r\n" +
		"ColdBuildupRate=1.000000\r\n" +
		"MyOwnKey=7\r\n" +
		"FiefdomLimit=3\r\n" +
		"\r\n[Other]\r\nGatheringAmount=9\r\n"
	if got := read(t, d, custom); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	if n := len(d.Backups(custom)); n != 1 {
		t.Fatalf("one edit, one backup; got %d", n)
	}
	// the backup holds the file as it was
	bk := d.Backups(custom)[0]["name"].(string)
	b, _ := os.ReadFile(filepath.Join(d.backupDir(), bk))
	if string(b) != resetFile {
		t.Error("backup is not the old file")
	}
	// a second reset has nothing to do and writes nothing
	res, err = d.Reset(custom, -1, "", 0)
	if err != nil || res.Changed != 0 {
		t.Fatalf("second reset: %+v %v", res, err)
	}
	if n := len(d.Backups(custom)); n != 1 {
		t.Errorf("no-op wrote a backup (%d)", n)
	}
}

func TestResetAllRefusesAStaleCountAndAFileWithoutDefaults(t *testing.T) {
	d := setup(t, map[string]string{custom: resetFile, "Game.ini": resetFile})
	if _, err := d.Reset(custom, -1, "", 4); err == nil || !strings.Contains(err.Error(), "reload") {
		t.Errorf("the page counted 4 but 5 would change: %v", err)
	}
	if got := read(t, d, custom); got != resetFile {
		t.Error("a refused reset must write nothing")
	}
	if _, err := d.Reset("Game.ini", -1, "", 5); err == nil {
		t.Error("Game.ini has no table")
	}
}

func TestResetKeepsLFFilesAndTheByteOrderMark(t *testing.T) {
	text := bomStr + sec + "\nCraftingCost=2.000000\n"
	d := setup(t, map[string]string{custom: text})
	if _, err := d.Reset(custom, -1, "", 1); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, d, custom), bomStr+sec+"\nCraftingCost=1.000000\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEqualsByKind(t *testing.T) {
	f := defaultByKey["GatheringAmount"]
	for v, want := range map[string]bool{"1": true, "1.0": true, "1.000000": true, " 1.000000 ": true, "1.5": false, "": false, "abc": false} {
		if f.Equals(v) != want {
			t.Errorf("number %q: want %v", v, want)
		}
	}
	b := defaultByKey["bAllowSandstorms"]
	if !b.Equals("true") || !b.Equals("True") || b.Equals("False") || b.Equals("1") {
		t.Error("switch comparison")
	}
	e := defaultByKey["PVPMode"]
	if !e.Equals("limited") || e.Equals("FullPVP") {
		t.Error("word comparison")
	}
}

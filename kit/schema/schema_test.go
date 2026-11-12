package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// ddlHash is the SHA-256 of DDL() with line endings as LF. It changes when the schema file is refreshed from a newer game build, on
// purpose, in the same pull request as the table count below and the CHANGELOG entry.
const ddlHash = "e43bc6d49cd666c816c4c2d38845f54958a4812ffaf88ac6abf38978b71ee18b"

// ddlMapNamesHash is the same for DDLMapNames(), the layout the game wrote after its update of early November 2026.
const ddlMapNamesHash = "4668ba0f0e16bbd1b2f429b90b023216865ea42741efe33f4f817e92aeeb030e"

var (
	createTable = regexp.MustCompile(`(?m)^CREATE TABLE (\w+) ?\(\n((?:.*\n)*?)\s*\)`)
	columnLine  = regexp.MustCompile(`(?m)^\s*"(\w+)"\s`)
	oneLine     = regexp.MustCompile(`(?m)^CREATE TABLE (\w+)\((\w+) `)
)

// tables parses the DDL into table name -> column names. It reads the shape the game's DDL has: each column on its own line, quoted,
// before the constraints. The one table written on a single line (applied_patches) is listed with its first column only.
func tables(t *testing.T) map[string][]string { return tablesOf(t, DDL()) }

func tablesOf(t *testing.T, ddl string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, m := range oneLine.FindAllStringSubmatch(ddl, -1) {
		out[m[1]] = []string{m[2]}
	}
	for _, m := range createTable.FindAllStringSubmatch(ddl, -1) {
		var cols []string
		for _, c := range columnLine.FindAllStringSubmatch(m[2], -1) {
			cols = append(cols, c[1])
		}
		if len(cols) == 0 {
			t.Fatalf("table %s: no column found", m[1])
		}
		out[m[1]] = cols
	}
	return out
}

func TestTheDDLIsTheAuditedSchema(t *testing.T) {
	d := DDL()
	sum := sha256.Sum256([]byte(strings.ReplaceAll(d, "\r\n", "\n")))
	if got := hex.EncodeToString(sum[:]); got != ddlHash {
		t.Errorf("schema.sql hash %s, want %s (a refreshed schema changes the hash, the table count and the CHANGELOG together)", got, ddlHash)
	}
	if n := strings.Count(d, "CREATE TABLE "); n != 94 {
		t.Errorf("%d CREATE TABLE statements, want 94", n)
	}
	if n := len(tables(t)); n != 94 {
		t.Errorf("parsed %d tables, want 94", n)
	}
	if n := strings.Count(d, "CREATE TRIGGER "); n != 1 {
		t.Errorf("%d triggers, want the game's one", n)
	}
}

func TestTheMapNamesDDLIsTheAuditedSchema(t *testing.T) {
	d := DDLMapNames()
	sum := sha256.Sum256([]byte(strings.ReplaceAll(d, "\r\n", "\n")))
	if got := hex.EncodeToString(sum[:]); got != ddlMapNamesHash {
		t.Errorf("schema_map_names.sql hash %s, want %s (a refreshed schema changes the hash, the table count and the CHANGELOG together)", got, ddlMapNamesHash)
	}
	if n := strings.Count(d, "CREATE TABLE "); n != 97 {
		t.Errorf("%d CREATE TABLE statements, want 97", n)
	}
	if n := len(tablesOf(t, d)); n != 97 {
		t.Errorf("parsed %d tables, want 97", n)
	}
	if n := strings.Count(d, "CREATE TRIGGER "); n != 1 {
		t.Errorf("%d triggers, want the game's one", n)
	}
	if strings.Contains(d, "CREATE TABLE sqlite_") {
		t.Error("an sqlite_ internal table is in the schema: the engine makes those itself")
	}
}

// The module holds the schema only: no statement that puts a row into a table.
func TestTheDDLHoldsNoRows(t *testing.T) {
	for name, d := range map[string]string{"schema.sql": DDL(), "schema_map_names.sql": DDLMapNames()} {
		for _, bad := range []string{"INSERT ", "insert ", "REPLACE INTO", "VALUES"} {
			if strings.Contains(d, bad) {
				t.Errorf("%s contains %q: the module carries the schema only, never a row", name, bad)
			}
		}
	}
}

// The two layouts differ in exactly what the CHANGELOG says: three new tables, and markers and player_markers with map_name_id in place of
// map_name. Every other table is defined the same way, so a program that reads a table other than those two needs no second code path.
func TestTheTwoLayoutsDifferOnlyInTheMapNames(t *testing.T) {
	oldT, newT := tables(t), tablesOf(t, DDLMapNames())
	for name, cols := range oldT {
		got, ok := newT[name]
		if !ok {
			t.Errorf("table %s is in the first layout and not in the second", name)
			continue
		}
		switch name {
		case "markers", "player_markers":
			want := strings.Join(cols, ",")
			want = strings.Replace(want, "map_name", "map_name_id", 1)
			if strings.Join(got, ",") != want {
				t.Errorf("%s: columns %v in the second layout, want %s", name, got, want)
			}
		default:
			if strings.Join(got, ",") != strings.Join(cols, ",") {
				t.Errorf("%s: columns %v in the second layout, %v in the first", name, got, cols)
			}
		}
	}
	for _, added := range []string{"map_names", "landsraad_one_time_house_rewards", "landsraad_player_one_time_house_rewards"} {
		if _, ok := newT[added]; !ok {
			t.Errorf("table %s is not in the second layout", added)
		}
		if _, ok := oldT[added]; ok {
			t.Errorf("table %s is in the first layout", added)
		}
	}
	if len(newT) != len(oldT)+3 {
		t.Errorf("%d tables against %d: want three more", len(newT), len(oldT))
	}
}

// Every name in the package is a table or a column of the DDL.
func TestEveryNameIsInTheSchema(t *testing.T) {
	have := tables(t)
	for _, v := range []any{Actors, ActorFglEntities, BackupVehicles, FglEntities, Inventories, Items, PermissionActor, PermissionActorRank,
		PlayerState, RecoveredVehicles, VehicleModules, Vehicles} {
		rv := reflect.ValueOf(v)
		table := rv.FieldByName("Table").String()
		cols, ok := have[table]
		if !ok {
			t.Errorf("%s: no such table in the schema", table)
			continue
		}
		for i := 0; i < rv.NumField(); i++ {
			name, col := rv.Type().Field(i).Name, rv.Field(i).String()
			if name == "Table" {
				continue
			}
			found := false
			for _, c := range cols {
				found = found || c == col
			}
			if !found {
				t.Errorf("%s.%s = %q: not a column of %s (columns: %v)", table, name, col, table, cols)
			}
		}
	}
}

// MapNames is a table and its columns of the second layout, and it is not in the first.
func TestMapNamesIsInTheSecondLayoutOnly(t *testing.T) {
	cols, ok := tablesOf(t, DDLMapNames())[MapNames.Table]
	if !ok {
		t.Fatalf("%s: no such table in DDLMapNames", MapNames.Table)
	}
	if strings.Join(cols, ",") != MapNames.MapNameID+","+MapNames.MapName {
		t.Errorf("%s columns %v", MapNames.Table, cols)
	}
	if _, ok := tables(t)[MapNames.Table]; ok {
		t.Errorf("%s is in DDL: the first layout has no such table", MapNames.Table)
	}
}

func TestTheJSONPathsAreTheOnesTheGameWrites(t *testing.T) {
	for got, want := range map[string]string{
		VehicleModuleDurabilityKey:        "$.FVehicleModuleDurabilityStats",
		VehicleModuleDurability:           "$.FVehicleModuleDurabilityStats[1].",
		VehicleModuleCurrentDurability:    "$.FVehicleModuleDurabilityStats[1].CurrentDurability",
		VehicleModuleDecayedMaxDurability: "$.FVehicleModuleDurabilityStats[1].DecayedMaxDurability",
		ItemDurabilityKey:                 "$.FItemStackAndDurabilityStats",
		ItemDurability:                    "$.FItemStackAndDurabilityStats[1].",
		ItemCurrentDurability:             "$.FItemStackAndDurabilityStats[1].CurrentDurability",
		ItemDecayedMaxDurability:          "$.FItemStackAndDurabilityStats[1].DecayedMaxDurability",
		FillableKey:                       "$.FFillableItemStats",
		Fillable:                          "$.FFillableItemStats[1].",
		FillableCurrentAmount:             "$.FFillableItemStats[1].CurrentAmount",
		FillableLiquid:                    "$.FFillableItemStats[1].FillableType",
	} {
		if got != want {
			t.Errorf("path %q, want %q", got, want)
		}
	}
}

func TestTheStoredStatesAreTwoAndFour(t *testing.T) {
	if StateVehicleBackup != 2 || StateVehicleRecovery != 4 {
		t.Errorf("states %d and %d, want 2 (backup) and 4 (recovery)", StateVehicleBackup, StateVehicleRecovery)
	}
	if StoredStates != "2, 4" || GearInventoryTypes != "0,1,15" {
		t.Errorf("lists %q and %q", StoredStates, GearInventoryTypes)
	}
}

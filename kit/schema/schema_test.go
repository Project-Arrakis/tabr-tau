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

var (
	createTable = regexp.MustCompile(`(?m)^CREATE TABLE (\w+) ?\(\n((?:.*\n)*?)\s*\)`)
	columnLine  = regexp.MustCompile(`(?m)^\s*"(\w+)"\s`)
	oneLine     = regexp.MustCompile(`(?m)^CREATE TABLE (\w+)\((\w+) `)
)

// tables parses the DDL into table name -> column names. It reads the shape the game's DDL has: each column on its own line, quoted,
// before the constraints. The one table written on a single line (applied_patches) is listed with its first column only.
func tables(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, m := range oneLine.FindAllStringSubmatch(DDL(), -1) {
		out[m[1]] = []string{m[2]}
	}
	for _, m := range createTable.FindAllStringSubmatch(DDL(), -1) {
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

// The module holds the schema only: no statement that puts a row into a table.
func TestTheDDLHoldsNoRows(t *testing.T) {
	for _, bad := range []string{"INSERT ", "insert ", "REPLACE INTO", "VALUES"} {
		if strings.Contains(DDL(), bad) {
			t.Errorf("schema.sql contains %q: the module carries the schema only, never a row", bad)
		}
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

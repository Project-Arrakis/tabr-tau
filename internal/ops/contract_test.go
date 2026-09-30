package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

func oneLine(q string) string { return strings.Join(strings.Fields(q), " ") }

// Every read path is run against a populated save built from the REAL game schema. Any SQL error (unknown table or
// column, wrong type) is a divergence between this code and the game's database, even when the code swallows it.
// This is the contract test the QA audit asked for (F-02): the old hand-written fixtures could never fail this way.
func TestReadPathsMatchRealSchema(t *testing.T) {
	s := testsave.Filled(t)
	var errs []string
	save.SetSQLErrorHook(func(q string, err error) { errs = append(errs, err.Error()+"  <-  "+oneLine(q)) })
	t.Cleanup(func() { save.SetSQLErrorHook(nil) })

	o := &Ops{S: s}
	reads := map[string]func() (any, error){
		"Overview": o.Overview, "Player": o.Player, "Inventory": o.Inventory, "Factions": o.Factions, "Specs": o.Specs,
		"Tutorials": o.Tutorials, "Tags": o.Tags, "Recipes": o.Recipes, "Bases": o.Bases, "Storage": o.Storage,
		"Vehicles": o.Vehicles, "Exchange": o.Exchange, "Landsraad": o.Landsraad, "Tables": o.Tables,
		"Journey":      func() (any, error) { return o.Journey("") },
		"StorageItems": func() (any, error) { return o.StorageItems(1) },
		"TableRows":    func() (any, error) { return o.TableRows("items", "", 10, 0) },
	}
	for name, f := range reads {
		if _, err := f(); err != nil {
			t.Errorf("%s returned an error: %v", name, err)
		}
	}
	for _, e := range errs {
		t.Errorf("SQL error against the real schema: %s", e)
	}
}

// The contract test above is only meaningful if the hook really sees errors that the code swallows.
func TestSQLErrorHookSeesFailures(t *testing.T) {
	s := testsave.Empty(t)
	var seen []string
	save.SetSQLErrorHook(func(q string, err error) { seen = append(seen, err.Error()) })
	t.Cleanup(func() { save.SetSQLErrorHook(nil) })

	s.Query(`select no_such_column from items`)          // swallowed by the caller on purpose
	s.One(`select 1 from no_such_table`)                 // One -> Query -> Table
	s.Exec(`update items set no_such_column = 1`)        // Exec
	s.ExecScript(`insert into no_such_table values (1)`) // ExecScript
	if len(seen) != 4 {
		t.Fatalf("hook saw %d errors, want 4: %v", len(seen), seen)
	}
	for _, e := range seen {
		if !strings.Contains(e, "no such") && !strings.Contains(e, "no column") {
			t.Errorf("unexpected error text: %s", e)
		}
	}
}

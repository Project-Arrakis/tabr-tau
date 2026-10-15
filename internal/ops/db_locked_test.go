package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Issue #71: the Tables view must not let account identity, primary keys or links be edited by hand.

func TestUpdateRowRefusesAccountsAndIdentityColumns(t *testing.T) {
	o := newPlayerOps(t)
	// The value must be one the column accepts (the schema is STRICT, so "x" in an INTEGER column fails on its own): the lock is
	// what has to refuse the edit, not the column type.
	for _, c := range []struct {
		table, col string
		val        any
	}{
		{"accounts", "user", "x"}, {"accounts", "funcom_id", "x"}, {"accounts", "platform_id", "x"}, {"accounts", "platform_name", "x"},
		{"items_id_sequencer", "next_id", float64(7)}, // the whole table is locked
	} {
		_, err := o.UpdateRow(Args{"table": c.table, "column": c.col, "rowid": float64(1), "value": c.val})
		if err == nil || !strings.Contains(err.Error(), "cannot be edited here") {
			t.Errorf("%s.%s must be refused by the lock, got %v", c.table, c.col, err)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused edit must leave no pending change")
	}
	if n := mustOne(t, o, `select next_id n from items_id_sequencer`)["n"].(int64); n != 500 {
		t.Fatalf("the item id counter changed to %d", n)
	}
	// the value is acceptable to the column: with no lock the same edit goes through
	if _, err := o.S.Mutate("probe", func(m *save.Mut) error {
		_, err := m.Exec(`update items_id_sequencer set next_id=7`)
		return err
	}); err != nil {
		t.Fatalf("the test value must be valid for the column: %v", err)
	}
}

func TestUpdateRowRefusesPrimaryKeysAndLinkColumnsButNotPlainOnes(t *testing.T) {
	o := newPlayerOps(t)
	// items.id is the primary key and items.inventory_id links to inventories
	for _, col := range []string{"id", "inventory_id"} {
		_, err := o.UpdateRow(Args{"table": "items", "column": col, "rowid": float64(1), "value": float64(1)})
		if err == nil || !strings.Contains(err.Error(), "cannot be edited here") {
			t.Errorf("items.%s must be refused, got %v", col, err)
		}
	}
	row, err := o.S.One(`select rowid r from items limit 1`)
	if err != nil || row == nil {
		t.Fatalf("fixture has no item: %v", err)
	}
	rid, _ := toInt(row["r"])
	if _, err := o.UpdateRow(Args{"table": "items", "column": "template_id", "rowid": float64(rid), "value": "Spice"}); err != nil {
		t.Errorf("a plain column stays editable: %v", err)
	}
}

func TestTableRowsReportsWhyColumnsAreLocked(t *testing.T) {
	o := newPlayerOps(t)
	res, err := o.TableRows("items", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	locked := res.(map[string]any)["locked"].(map[string]string)
	if locked["id"] != "primary key" || locked["inventory_id"] != "link to another table" {
		t.Fatalf("locked columns of items: %v", locked)
	}
	if _, plain := locked["template_id"]; plain {
		t.Fatal("template_id is a plain column")
	}
	acc, err := o.TableRows("accounts", "", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if why := acc.(map[string]any)["tableLocked"].(string); why == "" {
		t.Fatal("accounts must be reported as a locked table")
	}
}

func TestSQLWriteRefusesTheAccountsTable(t *testing.T) {
	o := newPlayerOps(t)
	for _, q := range []string{`update accounts set user='x'`, `delete from accounts`, `update items set template_id='x' where id in (select id from accounts)`} {
		if _, err := o.ExecSQL(q); err == nil {
			t.Errorf("%q must be refused", q)
		}
	}
}

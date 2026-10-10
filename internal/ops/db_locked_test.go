package ops

import (
	"strings"
	"testing"
)

// Issue #71: the Tables view must not let account identity, primary keys or links be edited by hand.

func TestUpdateRowRefusesAccountsAndIdentityColumns(t *testing.T) {
	o := newPlayerOps(t)
	for _, c := range []struct{ table, col string }{
		{"accounts", "user"}, {"accounts", "funcom_id"}, {"accounts", "platform_id"}, {"accounts", "platform_name"},
		{"items_id_sequencer", "next_id"}, // the whole table is locked
	} {
		if _, err := o.UpdateRow(Args{"table": c.table, "column": c.col, "rowid": float64(1), "value": "x"}); err == nil {
			t.Errorf("%s.%s must not be editable here", c.table, c.col)
		}
	}
	if o.S.Dirty() {
		t.Fatal("a refused edit must leave no pending change")
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

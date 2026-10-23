package ops

import (
	"strings"
	"testing"
)

// F-18 of the 2026-09-30 audit: the SQL console and the tables view's cell edit reach a JSON column through the same single write path, so a statement that leaves a stored
// document unusable is refused whole and the cell keeps what it held.

func jsonStatsOf(t *testing.T, o *Ops, id int) string {
	t.Helper()
	return mustOne(t, o, `select stats s from items where id=?`, id)["s"].(string)
}

func TestTheSQLConsoleRefusesAnUpdateThatLeavesBadOrHugeJSON(t *testing.T) {
	o := newPlayerOps(t)
	before := jsonStatsOf(t, o, 10)
	for name, q := range map[string]string{
		"not JSON":                           `update items set stats='{"broken' where id=10`,
		"over 1 MiB":                         `update items set stats=json_set(stats,'$.pad',(with recursive r(s,n) as (select 'aaaaaaaaaaaaaaaa',1 union all select s||s,n+1 from r where n<17) select s from r order by n desc limit 1)) where id=10`,
		"unreadable bytes in a jsonb column": `update actors set properties=x'00010203' where id=(select min(id) from actors)`,
	} {
		_, err := o.ExecSQL(q)
		if err == nil || !strings.Contains(err.Error(), "nothing was changed") {
			t.Errorf("%s: want a refusal that says nothing was changed, got %v", name, err)
		}
	}
	if jsonStatsOf(t, o, 10) != before || o.S.Dirty() {
		t.Fatalf("a refused statement changed something (dirty=%v)", o.S.Dirty())
	}
	if _, err := o.ExecSQL(`update items set stats=json_set(stats,'$.tabrTest',1) where id=10`); err != nil {
		t.Fatalf("an ordinary json_set goes through: %v", err)
	}
	if !strings.Contains(jsonStatsOf(t, o, 10), `"tabrTest":1`) {
		t.Fatal("the ordinary edit stands")
	}
}

func TestTheTablesViewCellEditRefusesTextThatIsNotJSONInAJSONColumn(t *testing.T) {
	o := newPlayerOps(t)
	row := mustOne(t, o, `select rowid r, stats s from items where id=10`)
	rid := float64(row["r"].(int64))
	_, err := o.UpdateRow(Args{"table": "items", "column": "stats", "rowid": rid, "value": "this is not json", "expected": row["s"].(string)})
	if err == nil || !strings.Contains(err.Error(), "items.stats #10: not valid JSON") {
		t.Fatalf("a cell edit that leaves bad JSON is refused and names the cell: %v", err)
	}
	if jsonStatsOf(t, o, 10) != row["s"].(string) || o.S.Dirty() {
		t.Fatal("the cell kept what it held")
	}
	if _, err := o.UpdateRow(Args{"table": "items", "column": "stats", "rowid": rid, "value": `{"edited":true}`, "expected": row["s"].(string)}); err != nil {
		t.Fatalf("valid JSON is accepted: %v", err)
	}
	if jsonStatsOf(t, o, 10) != `{"edited":true}` {
		t.Fatalf("the edit stands: %s", jsonStatsOf(t, o, 10))
	}
}

package ops

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// quoteIdent quotes a SQL identifier, doubling embedded quotes, so a column name taken from a save can never
// change the shape of a statement.
func quoteIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func (o *Ops) tableInfo(name string) ([]string, []string, error) {
	if !identRe.MatchString(name) {
		return nil, nil, errors.New("invalid table name")
	}
	rows, err := o.S.Query(`select name, type from pragma_table_info(?)`, name)
	if err != nil || len(rows) == 0 {
		return nil, nil, errors.New("no such table")
	}
	var cols, types []string
	for _, r := range rows {
		cols = append(cols, r["name"].(string))
		types = append(types, strings.ToUpper(fmt.Sprint(r["type"])))
	}
	return cols, types, nil
}

// lockedTables cannot be edited from the Tables view or by a SQL write, whatever the column: they identify the account or
// belong to the game's own bookkeeping.
var lockedTables = map[string]string{
	"accounts":           "account identity: changing it can stop the game recognising the character",
	"applied_patches":    "the game's version history",
	"items_id_sequencer": "the game's item id counter",
	"sqlite_sequence":    "SQLite's own counters",
}

// identityColumns name the account in any table.
var identityColumns = map[string]bool{"account_id": true, "owner_account_id": true, "platform_id": true, "funcom_id": true, "platform_name": true, "user": true}

// lockedColumns says why each column of a table cannot be edited by hand: it is part of the primary key, it links to another
// table, or it identifies the account. Editing those through the review pane would still be possible, and could break the
// links the game follows. The first return value is the reason for the whole table when it is locked.
func (o *Ops) lockedColumns(table string, cols []string) (string, map[string]string) {
	lt := strings.ToLower(table)
	if why, ok := lockedTables[lt]; ok {
		return why, nil
	}
	if strings.HasPrefix(lt, "sqlite_") {
		return "SQLite's own table", nil
	}
	locked := map[string]string{}
	if pk, err := o.S.Query(`select name from pragma_table_info(?) where pk > 0`, table); err == nil {
		for _, r := range pk {
			locked[fmt.Sprint(r["name"])] = "primary key"
		}
	}
	if fk, err := o.S.Query(`select "from" f from pragma_foreign_key_list(?)`, table); err == nil {
		for _, r := range fk {
			if _, dup := locked[fmt.Sprint(r["f"])]; !dup {
				locked[fmt.Sprint(r["f"])] = "link to another table"
			}
		}
	}
	for _, c := range cols {
		if identityColumns[strings.ToLower(c)] {
			locked[c] = "identifies the account"
		}
	}
	return "", locked
}

func (o *Ops) Tables() (any, error) {
	tabs, err := o.S.Query(`select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name`)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, t := range tabs {
		n := t["name"].(string)
		if !identRe.MatchString(n) { // a save-controlled name is never spliced into SQL text
			continue
		}
		cols, types, _ := o.tableInfo(n)
		c, _ := o.S.One(`select count(*) c from "` + n + `"`)
		out = append(out, map[string]any{"name": n, "rows": c["c"], "columns": cols, "types": types})
	}
	return out, nil
}

func (o *Ops) TableRows(name, q string, limit, offset int) (any, error) {
	cols, _, err := o.tableInfo(name)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 5000 {
		limit = 200
	}
	where, args := "", []any{}
	if q != "" {
		var parts []string
		for _, c := range cols {
			parts = append(parts, `cast(`+quoteIdent(c)+` as text) like ?`)
			args = append(args, "%"+q+"%")
		}
		where = " where " + strings.Join(parts, " or ")
	}
	tot, _ := o.S.One(`select count(*) c from "`+name+`"`+where, args...)
	c, rows, err := o.S.Table(`select rowid _rowid_, * from "`+name+`"`+where+` limit ? offset ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, err
	}
	whole, locked := o.lockedColumns(name, cols)
	return map[string]any{"columns": c, "rows": rows, "total": tot["c"], "tableLocked": whole, "locked": locked}, nil
}

func (o *Ops) UpdateRow(a Args) (any, error) {
	table, col := a.Str("table"), a.Str("column")
	cols, types, err := o.tableInfo(table)
	if err != nil {
		return nil, err
	}
	whole, locked := o.lockedColumns(table, cols)
	if whole != "" {
		return nil, fmt.Errorf("%s cannot be edited here: %s", table, whole)
	}
	if why, bad := locked[col]; bad {
		return nil, fmt.Errorf("%s.%s cannot be edited here: %s", table, col, why)
	}
	rowid, err := a.Int("rowid")
	if err != nil {
		return nil, err
	}
	for i, c := range cols {
		if c == col {
			if types[i] == "BLOB" {
				return nil, errors.New("BLOB columns cannot be edited here")
			}
			n, err := o.run(fmt.Sprintf("update %s[%d].%s", table, rowid, col), `update `+quoteIdent(table)+` set `+quoteIdent(col)+`=? where rowid=?`, a["value"], rowid)
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return nil, errors.New("no such row")
			}
			return ok(), nil
		}
	}
	return nil, errors.New("no such column")
}

// SQL runs a read-only query.
func (o *Ops) SQL(q string) (any, error) {
	if err := vetRead(q); err != nil {
		return nil, err
	}
	cols, rows, err := o.S.ReadOnlyTable(q, 1000)
	if err != nil {
		return nil, err
	}
	return map[string]any{"columns": cols, "rows": rows}, nil
}

// ExecSQL runs write statements against the working copy. Changes stay pending
// until the save is committed (which backs up the original first).
func (o *Ops) ExecSQL(q string) (any, error) {
	if err := vetWrite(q); err != nil {
		return nil, err
	}
	short := strings.Join(strings.Fields(q), " ")
	if len(short) > 100 {
		short = short[:100] + "…"
	}
	n, err := o.S.Mutate("sql: "+short, func(m *save.Mut) error { _, err := m.Exec(q); return err })
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "changes": n}, nil
}

// Export renders a table as CSV or JSON.
func (o *Ops) Export(name, format string) ([]byte, string, error) {
	if _, _, err := o.tableInfo(name); err != nil {
		return nil, "", err
	}
	cols, rows, err := o.S.Table(`select * from "` + name + `"`)
	if err != nil {
		return nil, "", err
	}
	if format == "json" {
		out := []map[string]any{}
		for _, r := range rows {
			m := map[string]any{}
			for i, c := range cols {
				m[c] = r[i]
			}
			out = append(out, m)
		}
		b, _ := json.MarshalIndent(out, "", " ")
		return b, "application/json", nil
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write(cols)
	for _, r := range rows {
		rec := make([]string, len(r))
		for i, v := range r {
			if v != nil {
				rec[i] = csvCell(v)
			}
		}
		w.Write(rec)
	}
	w.Flush()
	return buf.Bytes(), "text/csv", nil
}

// csvCell renders one cell. Text that a spreadsheet would run as a formula (leading = + - @ tab or CR) gets a
// leading apostrophe; numbers are not text and stay as they are.
func csvCell(v any) string {
	str, ok := v.(string)
	if ok && str != "" && strings.ContainsRune("=+-@\t\r", rune(str[0])) {
		return "'" + str
	}
	return fmt.Sprint(v)
}

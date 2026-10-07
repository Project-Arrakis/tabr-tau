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
			parts = append(parts, `cast("`+c+`" as text) like ?`)
			args = append(args, "%"+q+"%")
		}
		where = " where " + strings.Join(parts, " or ")
	}
	tot, _ := o.S.One(`select count(*) c from "`+name+`"`+where, args...)
	c, rows, err := o.S.Table(`select rowid _rowid_, * from "`+name+`"`+where+` limit ? offset ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, err
	}
	return map[string]any{"columns": c, "rows": rows, "total": tot["c"]}, nil
}

func (o *Ops) UpdateRow(a Args) (any, error) {
	table, col := a.Str("table"), a.Str("column")
	cols, types, err := o.tableInfo(table)
	if err != nil {
		return nil, err
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
			n, err := o.run(fmt.Sprintf("update %s[%d].%s", table, rowid, col), `update "`+table+`" set "`+col+`"=? where rowid=?`, a["value"], rowid)
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return nil, errors.New("no such row, or the value is unchanged")
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

package ops

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// quoteIdent quotes a SQL identifier, doubling embedded quotes, so a column name taken from a save can never
// change the shape of a statement.
func quoteIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func (o *Ops) tableInfo(name string) ([]string, []string, error) {
	if !identRe.MatchString(name) {
		return nil, nil, apierr.New(apierr.ErrBadArgument, "invalid table name")
	}
	rows, err := o.S.Query(`select name, type from pragma_table_info(?)`, name)
	if errors.Is(err, save.ErrStopped) { // a read that ran out of time is not a missing table
		return nil, nil, err
	}
	if err != nil || len(rows) == 0 {
		return nil, nil, apierr.New(apierr.ErrNotFound, "no such table")
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
// table, or it identifies the account. Editing those by hand would still be possible, and could break the
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
		cols, types, terr := o.tableInfo(n)
		if errors.Is(terr, save.ErrStopped) {
			return nil, terr
		}
		// Each count has the read deadline, so a table that cannot be counted in time (a hostile file) ends the list with an error instead of holding the save's lock.
		c, cerr := o.S.One(`select count(*) c from "` + n + `"`)
		if errors.Is(cerr, save.ErrStopped) {
			return nil, cerr
		}
		out = append(out, map[string]any{"name": n, "rows": c["c"], "columns": cols, "types": types})
	}
	return out, nil
}

// rowKey says how the rows of a table are addressed from the Tables view. Most tables have a rowid. The game's 61
// WITHOUT ROWID tables have none (selecting it fails with "no such column: rowid"), so their rows are addressed by
// the values of their primary-key columns, carried as a JSON array of strings (a BLOB key as hex).
type rowKey struct {
	withoutRowid bool
	pk           []string // primary-key columns in key order (WITHOUT ROWID tables only)
	blob         []bool   // pk[i] is declared BLOB
}

// keyText is the SQL that renders one key column as the text the Tables view carries and compares.
func keyText(col string, blob bool, prefix string) string {
	if blob {
		return `hex(` + prefix + quoteIdent(col) + `)`
	}
	return `cast(` + prefix + quoteIdent(col) + ` as text)`
}

func (o *Ops) rowKeyOf(table string, cols, types []string) (rowKey, error) {
	r, err := o.S.One(`select wr from pragma_table_list where schema='main' and name=?`, table)
	if err != nil {
		return rowKey{}, err
	}
	if r == nil || fmt.Sprint(r["wr"]) != "1" {
		return rowKey{}, nil
	}
	pk, err := o.S.Query(`select name from pragma_table_info(?) where pk>0 order by pk`, table)
	if err != nil || len(pk) == 0 {
		return rowKey{}, errors.New("the table has no primary key to address its rows by")
	}
	k := rowKey{withoutRowid: true}
	for _, p := range pk {
		n := fmt.Sprint(p["name"])
		k.pk = append(k.pk, n)
		isBlob := false
		for i, c := range cols {
			if c == n {
				isBlob = types[i] == "BLOB"
			}
		}
		k.blob = append(k.blob, isBlob)
	}
	return k, nil
}

// keyColumn is the select-list entry that addresses each row, and its name in the result (the first column).
func (k rowKey) keyColumn() string {
	if !k.withoutRowid {
		return `rowid _rowid_`
	}
	parts := make([]string, len(k.pk))
	for i, c := range k.pk {
		parts[i] = keyText(c, k.blob[i], "")
	}
	return `json_array(` + strings.Join(parts, ",") + `) _key_`
}

func (o *Ops) TableRows(name, q string, limit, offset int) (any, error) {
	cols, types, err := o.tableInfo(name)
	if err != nil {
		return nil, err
	}
	key, err := o.rowKeyOf(name, cols, types)
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
	c, rows, err := o.S.Table(`select `+key.keyColumn()+`, * from "`+name+`"`+where+` limit ? offset ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, err
	}
	whole, locked := o.lockedColumns(name, cols)
	return map[string]any{"columns": c, "rows": rows, "total": tot["c"], "tableLocked": whole, "locked": locked}, nil
}

// expectedCond is the extra condition of a cell edit that says what the cell held when the table was shown: the update only goes through if it still holds it. The page sends the
// text it showed. It is compared as the column compares text (a number column reads "5" as 5, so 5.0 matches), and nothing shown stands for an empty cell, which is NULL or empty text. A
// whole number beyond 2^53 is shown rounded by the page, so it is compared as the rounded number. ok is false when the request carries no expected value (older callers).
func expectedCond(a Args, col string) (cond string, args []any, ok bool, err error) {
	v, has := a["expected"]
	if !has {
		return "", nil, false, nil
	}
	c := quoteIdent(col)
	switch x := v.(type) {
	case nil:
		return ` and (` + c + ` is null or ` + c + `='')`, nil, true, nil
	case string:
		if x == "" {
			return ` and (` + c + ` is null or ` + c + `='')`, nil, true, nil
		}
		if n, perr := strconv.ParseInt(x, 10, 64); (perr == nil && (n >= maxExactInt || n <= -maxExactInt)) || (perr != nil && errors.Is(perr, strconv.ErrRange)) {
			return ` and cast(` + c + ` as real)=cast(? as real)`, []any{x}, true, nil
		}
		return ` and ` + c + ` is ?`, []any{x}, true, nil
	case float64:
		return ` and ` + c + ` is ?`, []any{x}, true, nil
	}
	return "", nil, false, apierr.New(apierr.ErrBadArgument, "expected must be the text the cell showed")
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
	rk, err := o.rowKeyOf(table, cols, types)
	if err != nil {
		return nil, err
	}
	// where says which row: by rowid, or by the primary-key values of a WITHOUT ROWID table
	where, whereArgs, label := "rowid=?", []any(nil), ""
	if rk.withoutRowid {
		var vals []string
		if s, isStr := a["key"].(string); isStr {
			if err := json.Unmarshal([]byte(s), &vals); err != nil {
				return nil, apierr.New(apierr.ErrBadArgument, "key must be a JSON array of the primary-key values")
			}
		}
		if len(vals) != len(rk.pk) {
			return nil, fmt.Errorf("%s is addressed by %d primary-key value(s): key is required", table, len(rk.pk))
		}
		conds := make([]string, len(rk.pk))
		for i, c := range rk.pk {
			// compared as the column's own type (the bound text takes the column's affinity), not as text, so a REAL or large integer key
			// matches exactly and the primary-key index is used; only a BLOB is compared through its hex
			if rk.blob[i] {
				conds[i] = keyText(c, true, "") + "=?"
			} else {
				conds[i] = quoteIdent(c) + "=?"
			}
			whereArgs = append(whereArgs, vals[i])
		}
		where, label = strings.Join(conds, " and "), "["+strings.Join(vals, ",")+"]"
	} else {
		rowid, err := a.Int("rowid")
		if err != nil {
			return nil, err
		}
		whereArgs, label = []any{rowid}, fmt.Sprintf("[%d]", rowid)
	}
	for i, c := range cols {
		if c == col {
			if types[i] == "BLOB" {
				return nil, apierr.New(apierr.ErrBadArgument, "BLOB columns cannot be edited here")
			}
			cond, condArgs, hasExpected, err := expectedCond(a, col)
			if err != nil {
				return nil, err
			}
			var n int64
			_, err = o.S.Mutate(fmt.Sprintf("update %s%s.%s", table, label, col), func(m *save.Mut) error {
				args := append(append([]any{a["value"]}, whereArgs...), condArgs...)
				res, err := m.Exec(`update `+quoteIdent(table)+` set `+quoteIdent(col)+`=? where (`+where+`)`+cond, args...)
				if err != nil {
					return err
				}
				n, _ = res.RowsAffected()
				if n > 1 { // a key must name one row; refuse and roll back rather than change several
					return fmt.Errorf("the key matched %d rows; nothing was changed", n)
				}
				if n == 0 && hasExpected { // the row is there but its cell is not what the table showed: the table is out of date (the game saved, or the queue was put back onto a newer file)
					if r, qerr := m.One(`select 1 x from `+quoteIdent(table)+` where `+where, whereArgs...); qerr == nil && r != nil {
						return apierr.New(apierr.ErrConflict, "this cell has changed since the table was shown, so nothing was changed: reload the table and try again")
					}
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return nil, apierr.New(apierr.ErrNotFound, "no such row")
			}
			return ok(), nil
		}
	}
	return nil, apierr.New(apierr.ErrNotFound, "no such column")
}

// sqlRowLimit is how many rows a read-only query returns.
const sqlRowLimit = 1000

// SQL runs a read-only query.
func (o *Ops) SQL(q string) (any, error) {
	if err := vetRead(q); err != nil {
		return nil, err
	}
	cols, rows, err := o.S.ReadOnlyTable(q, sqlRowLimit)
	if err != nil {
		return nil, err
	}
	// a result of exactly the limit may have been cut: say so rather than let it look complete
	return map[string]any{"columns": cols, "rows": rows, "truncated": len(rows) >= sqlRowLimit}, nil
}

// SQLWriteTimeout bounds one statement of the write console.
var SQLWriteTimeout = 15 * time.Second

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
	n, err := o.S.Mutate("sql: "+short, func(m *save.Mut) error {
		// The save is locked while this runs, so a statement that never ends (a recursive query with no stop, say) is interrupted after SQLWriteTimeout, like a read is after save.ReadTimeout.
		ctx, cancel := context.WithTimeout(context.Background(), SQLWriteTimeout)
		defer cancel()
		_, err := m.ExecContext(ctx, q)
		if err != nil && ctx.Err() != nil {
			return fmt.Errorf("the statement was stopped after %s: nothing was changed", SQLWriteTimeout)
		}
		return err
	})
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
	head := make([]string, len(cols)) // a column name comes from the save too: a name that starts with = is a formula to a spreadsheet
	for i, c := range cols {
		head[i] = csvCell(c)
	}
	w.Write(head)
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

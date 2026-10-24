package save

import (
	"database/sql"
	"fmt"
	"strings"
)

// MaxJSONCell is the largest stored JSON document an edit may leave in one cell, in bytes (characters for a text column). The game's own documents are a few kilobytes; a
// cell far past that is a statement gone wrong (a value repeated, a path that wrote a copy of the whole document into itself), and a save that holds one is slow to read and
// to write.
const MaxJSONCell = 1 << 20

// MaxJSONDepth is the deepest nesting of arrays and objects an edit may leave in one stored document. The game's documents nest a handful of levels; a document far deeper
// is a statement gone wrong, and every reader of the document pays for it.
const MaxJSONDepth = 32

// jsonColumn is a column that holds a JSON document, and how: blob columns hold binary JSON (JSONB), text columns hold JSON text. key is the column that names a row.
type jsonColumn struct {
	table, col, key string
	blob            bool
}

// jsonColumns are the columns the editor edits as JSON documents. Another column is added here when an edit starts to write it.
var jsonColumns = []jsonColumn{
	{"actors", "properties", "id", true},
	{"actors", "gas_attributes", "id", true},
	{"fgl_entities", "components", "entity_id", true},
	{"items", "stats", "id", false},
}

// jsonWatch notes which cells an edit wrote, so that only those are checked afterwards (a scan of every document of the save would read the whole save again after each
// edit). It is made of temporary triggers on the edit's own transaction: an insert, or an update that names the column in its SET list, writes one line to a temporary
// table. Nothing of it outlives the edit (it is dropped inside the transaction, so a rolled back edit leaves nothing either). A statement the editor was given by a person goes
// through the same triggers, so the SQL console and the Database tab are covered without a case of their own.
type jsonWatch struct {
	tx      *sql.Tx
	columns []jsonColumn // the covered columns that exist in this save
}

// startJSONWatch installs the triggers. Columns that the save does not have (a toy schema) are left out.
func startJSONWatch(tx *sql.Tx) (*jsonWatch, error) {
	w := &jsonWatch{tx: tx}
	for _, c := range jsonColumns {
		var n int
		if err := tx.QueryRow(`select count(*) from pragma_table_info(?) where name in (?, ?)`, c.table, c.col, c.key).Scan(&n); err != nil {
			return nil, err
		}
		if n == 2 {
			w.columns = append(w.columns, c)
		}
	}
	if len(w.columns) == 0 {
		return w, nil
	}
	if _, err := tx.Exec(`create temp table if not exists tabr_json_edits(tbl text, col text, k integer)`); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`delete from temp.tabr_json_edits`); err != nil {
		return nil, err
	}
	for _, c := range w.columns {
		note := fmt.Sprintf(`insert into temp.tabr_json_edits values (%s, %s, new.%s);`, qLit(c.table), qLit(c.col), qIdent(c.key))
		for _, t := range []struct{ name, event string }{{"upd", "update of " + qIdent(c.col)}, {"ins", "insert"}} {
			trg := qIdent(fmt.Sprintf("tabr_json_%s_%s_%s", t.name, c.table, c.col))
			if _, err := tx.Exec(`create temp trigger ` + trg + ` after ` + t.event + ` on main.` + qIdent(c.table) + ` begin ` + note + ` end`); err != nil {
				return nil, err
			}
		}
	}
	return w, nil
}

func qLit(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// logged is how many notes the triggers have written so far.
func (w *jsonWatch) logged() (n int64, err error) {
	if len(w.columns) == 0 {
		return 0, nil
	}
	err = w.tx.QueryRow(`select count(*) from temp.tabr_json_edits`).Scan(&n)
	return n, err
}

// finish checks the cells that were written, removes the triggers and the table, and returns how many lines the triggers wrote (each one is a row change in
// total_changes() that the edit did not make). A bad cell comes back as an error that names it and says nothing was changed.
func (w *jsonWatch) finish() (logged int64, err error) {
	if len(w.columns) == 0 {
		return 0, nil
	}
	if logged, err = w.logged(); err != nil {
		return 0, err
	}
	var problems []string
	if logged > 0 {
		if problems, err = w.check(); err != nil {
			return logged, err
		}
	}
	for _, c := range w.columns {
		for _, n := range []string{"upd", "ins"} {
			if _, err := w.tx.Exec(`drop trigger temp.` + qIdent(fmt.Sprintf("tabr_json_%s_%s_%s", n, c.table, c.col))); err != nil {
				return logged, err
			}
		}
	}
	if _, err := w.tx.Exec(`drop table temp.tabr_json_edits`); err != nil {
		return logged, err
	}
	if len(problems) > 0 {
		more := ""
		if len(problems) > 5 {
			more = fmt.Sprintf(" and %d more", len(problems)-5)
			problems = problems[:5]
		}
		return logged, fmt.Errorf("this edit would leave stored JSON the game may not be able to read (%s%s); nothing was changed", strings.Join(problems, "; "), more)
	}
	return logged, nil
}

// reasons are the words for the reason codes the check query returns.
var jsonReasons = map[string]string{
	"type":      "stored as text, not as binary JSON",
	"text":      "stored as text, not as binary JSON",
	"blob":      "stored as binary JSON, not as text",
	"invalid":   "not valid JSON",
	"oversize":  "larger than 1 MiB",
	"deep":      "nested more than 32 levels deep",
	"roundtrip": "does not read back as the same document",
}

// check judges every written cell: it has the type of its column, it is valid JSON of that kind, it is not over the cap, and reading it out as text and storing that text again
// gives the same document.
func (w *jsonWatch) check() ([]string, error) {
	var out []string
	for _, c := range w.columns {
		want, valid, textOf := "text", "json_valid(t."+qIdent(c.col)+")", "t."+qIdent(c.col)
		if c.blob {
			want, valid, textOf = "blob", "json_valid(t."+qIdent(c.col)+", 8)", "json(t."+qIdent(c.col)+")"
		}
		v := "t." + qIdent(c.col)
		q := `select k, r from (select l.k k, case
			when ` + v + ` is null then null
			when typeof(` + v + `) <> '` + want + `' then 'type'
			when not ` + valid + ` then (case when ` + isTextJSON(c) + ` then 'text' else 'invalid' end)
			when length(` + v + `) > ` + fmt.Sprint(MaxJSONCell) + ` then 'oversize'
			when json(jsonb(` + textOf + `)) is not json(` + textOf + `) then 'roundtrip'
			end r
			from (select distinct k from temp.tabr_json_edits where tbl=? and col=?) l join main.` + qIdent(c.table) + ` t on t.` + qIdent(c.key) + `=l.k)
			where r is not null order by k limit 20`
		rows, err := w.tx.Query(q, c.table, c.col)
		if err != nil {
			return nil, err
		}
		bad := map[int64]bool{}
		for rows.Next() {
			var k int64
			var r string
			if err := rows.Scan(&k, &r); err != nil {
				rows.Close()
				return nil, err
			}
			bad[k] = true
			out = append(out, fmt.Sprintf("%s.%s #%d: %s", c.table, c.col, k, jsonReasons[r]))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
		deep, err := w.tooDeep(c, textOf, bad)
		if err != nil {
			return nil, err
		}
		for _, k := range deep {
			out = append(out, fmt.Sprintf("%s.%s #%d: %s", c.table, c.col, k, jsonReasons["deep"]))
		}
	}
	return out, nil
}

// tooDeep returns (at most 20 of) the written cells, other than the ones already judged bad, whose document nests deeper than MaxJSONDepth. The cells are valid JSON by now,
// so counting the brackets that are outside strings is exact.
func (w *jsonWatch) tooDeep(c jsonColumn, textOf string, skip map[int64]bool) ([]int64, error) {
	rows, err := w.tx.Query(`select l.k, `+textOf+` from (select distinct k from temp.tabr_json_edits where tbl=? and col=?) l join main.`+qIdent(c.table)+` t on t.`+qIdent(c.key)+`=l.k where t.`+qIdent(c.col)+` is not null and length(t.`+qIdent(c.col)+`) > ? order by l.k`, c.table, c.col, MaxJSONDepth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() && len(out) < 20 {
		var k int64
		var doc string
		if err := rows.Scan(&k, &doc); err != nil {
			return nil, err
		}
		if !skip[k] && jsonDepth(doc) > MaxJSONDepth {
			out = append(out, k)
		}
	}
	return out, rows.Err()
}

// jsonDepth is the deepest nesting of [ and { in a valid JSON text.
func jsonDepth(doc string) int {
	depth, max := 0, 0
	inStr := false
	for i := 0; i < len(doc); i++ {
		switch ch := doc[i]; {
		case inStr:
			if ch == '\\' {
				i++
			} else if ch == '"' {
				inStr = false
			}
		case ch == '"':
			inStr = true
		case ch == '[' || ch == '{':
			if depth++; depth > max {
				max = depth
			}
		case ch == ']' || ch == '}':
			depth--
		}
	}
	return max
}

// isTextJSON is true for a blob column whose bytes are JSON text: the document was stored without being turned into binary JSON.
func isTextJSON(c jsonColumn) string {
	if !c.blob {
		return "0"
	}
	return "json_valid(cast(t." + qIdent(c.col) + " as text))"
}

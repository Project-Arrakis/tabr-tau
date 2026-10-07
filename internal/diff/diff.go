// Package diff compares two SQLite databases (a pristine save and an edited one) at row and JSON-path level.
//
// It is a leaf package: it knows nothing about saves, files or the web UI, only about two read-only *sql.DB handles.
// It is the Go port of tools/snapdiff.py and is what the review-before-save screen, the CLI and the post-write
// verification use. Rows are matched and compared on RAW values; identifiers (platform and Funcom ids, account and
// character names, 15+ digit numbers) are masked only in the output, by default, because a diff is exactly the kind of
// text that gets pasted into a bug report. Masking never hides a difference: an edited identifier is still listed,
// as "<redacted>" -> "<redacted>".
package diff

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// NoiseColumns change without any player action (measured from the noise-control live runs).
var NoiseColumns = map[string]bool{
	"actors.serial": true, "farm_variables.universe_time_timestamp": true, "farm_variables.universe_lastactive_timestamp": true,
	"farm_variables.down_time_accumulation": true, "player_state.last_avatar_activity": true, "player_state.last_login_time": true,
}

// redactColumns hold personal identifiers; their values are always shown as <redacted>. A column is also treated
// as personal when its name matches redactName. TestEveryColumnIsClassified keeps this list honest against the real
// schema: a new column that looks personal must be classified deliberately.
var redactColumns = map[string]bool{
	"platform_id": true, "funcom_id": true, "platform_name": true, "user": true, "character_name": true,
	"guild_identifier": true,
}
var redactName = regexp.MustCompile(`(?i)(^|_)(identifier|steam\w*|email|funcom\w*|platform\w*)$`)
var longNum = regexp.MustCompile(`\d{15,}`)

// IsPersonalColumn reports whether values of a column are masked in redacted output.
func IsPersonalColumn(col string) bool { return personalColumn(col) }

func personalColumn(col string) bool { return redactColumns[col] || redactName.MatchString(col) }

// Options controls a comparison. The zero value compares everything, redacted, with a 1e-6 float tolerance.
type Options struct {
	Tables        []string        // only these tables (empty: all)
	IgnoreTables  []string        // skip these tables
	IgnoreColumns map[string]bool // "column" or "table.column"
	Noise         bool            // also ignore NoiseColumns
	FloatEps      float64         // relative tolerance for floats (default 1e-6)
	MaxRows       int             // rows listed per table and kind (default 40); counts are always complete
	NoRedact      bool            // show identifiers (private local analysis only)
}

// Change is one difference inside a modified row: "+" added, "-" removed, "~" changed.
type Change struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// RowChange is a modified row and what changed in it.
type RowChange struct {
	Key     []any    `json:"key"`
	Changes []Change `json:"changes"`
}

// Row is an added or removed row.
type Row struct {
	Key  []any          `json:"key"`
	Data map[string]any `json:"data"`
}

// TableDiff is the difference for one table. Added/Removed/Modified hold at most MaxRows entries each;
// NumAdded/NumRemoved/NumModified are the true totals.
type TableDiff struct {
	Name        string      `json:"name"`
	Key         []string    `json:"key"`
	NumAdded    int         `json:"numAdded"`
	NumRemoved  int         `json:"numRemoved"`
	NumModified int         `json:"numModified"`
	Added       []Row       `json:"added,omitempty"`
	Removed     []Row       `json:"removed,omitempty"`
	Modified    []RowChange `json:"modified,omitempty"`
}

// Result is a whole comparison.
type Result struct {
	Tables     []TableDiff `json:"tables"`
	OnlyBefore []string    `json:"onlyBefore,omitempty"`
	OnlyAfter  []string    `json:"onlyAfter,omitempty"`
	Redacted   bool        `json:"redacted"`
}

// Empty reports whether nothing differs.
func (r *Result) Empty() bool {
	return len(r.Tables) == 0 && len(r.OnlyBefore) == 0 && len(r.OnlyAfter) == 0
}

type querier interface {
	Query(q string, args ...any) (*sql.Rows, error)
}

// Compare diffs before against after. Both handles are only read.
func Compare(before, after *sql.DB, opt Options) (*Result, error) {
	if opt.FloatEps == 0 {
		opt.FloatEps = 1e-6
	}
	if opt.MaxRows <= 0 {
		opt.MaxRows = 40
	}
	if opt.MaxRows > 1000 {
		opt.MaxRows = 1000
	}
	ta, err := tableNames(before)
	if err != nil {
		return nil, err
	}
	tb, err := tableNames(after)
	if err != nil {
		return nil, err
	}
	res := &Result{Redacted: !opt.NoRedact}
	inA, inB := toSet(ta), toSet(tb)
	for _, t := range ta {
		if !inB[t] {
			res.OnlyBefore = append(res.OnlyBefore, t)
		}
	}
	for _, t := range tb {
		if !inA[t] {
			res.OnlyAfter = append(res.OnlyAfter, t)
		}
	}
	only, skip := toSet(opt.Tables), toSet(opt.IgnoreTables)
	ignore := map[string]bool{}
	for k := range opt.IgnoreColumns {
		ignore[k] = true
	}
	if opt.Noise {
		for k := range NoiseColumns {
			ignore[k] = true
		}
	}
	for _, t := range ta {
		if !inB[t] || (len(only) > 0 && !only[t]) || skip[t] {
			continue
		}
		ra, pk, err := readRows(before, t)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t, err)
		}
		rb, _, err := readRows(after, t)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t, err)
		}
		td := TableDiff{Name: t, Key: pk}
		drop := func(d map[string]any) map[string]any {
			out := make(map[string]any, len(d))
			for k, v := range d {
				if ignore[k] || ignore[t+"."+k] {
					continue
				}
				out[k] = v
			}
			return out
		}
		keysA, keysB := sortedKeys(ra), sortedKeys(rb)
		for _, k := range keysB {
			if _, ok := ra[k]; !ok {
				td.NumAdded++
				if len(td.Added) < opt.MaxRows {
					td.Added = append(td.Added, present(rb[k].key, rb[k].data, !opt.NoRedact))
				}
			}
		}
		for _, k := range keysA {
			x := ra[k]
			y, ok := rb[k]
			if !ok {
				td.NumRemoved++
				if len(td.Removed) < opt.MaxRows {
					td.Removed = append(td.Removed, present(x.key, x.data, !opt.NoRedact))
				}
				continue
			}
			changes := jdiff(drop(x.data), drop(y.data), "", opt.FloatEps)
			if len(changes) == 0 {
				continue
			}
			td.NumModified++
			if len(td.Modified) < opt.MaxRows {
				td.Modified = append(td.Modified, presentChange(x.key, changes, !opt.NoRedact))
			}
		}
		if td.NumAdded+td.NumRemoved+td.NumModified > 0 {
			res.Tables = append(res.Tables, td)
		}
	}
	return res, nil
}

type keyed struct {
	key  []any
	data map[string]any
}

func toSet(l []string) map[string]bool {
	m := make(map[string]bool, len(l))
	for _, s := range l {
		m[s] = true
	}
	return m
}

func sortedKeys(m map[string]keyed) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// qi quotes an identifier, doubling embedded quotes, so a table or column name from a save can never change the
// shape of a statement.
func qi(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func tableNames(db querier) ([]string, error) {
	rs, err := db.Query(`select name from sqlite_master where type='table' and name not like 'sqlite\_%' escape '\' order by name`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rs.Err()
}

// blobKinds classifies every BLOB-holding column in one scan of the table: "jsonb" when all its blobs are valid
// JSONB, else "opaque".
func blobKinds(db querier, t string, cols []string) (map[string]string, error) {
	out := map[string]string{}
	if len(cols) == 0 {
		return out, nil
	}
	exprs := make([]string, 0, 2*len(cols))
	for _, c := range cols {
		exprs = append(exprs, fmt.Sprintf(`coalesce(sum(typeof(%[1]s)='blob'),0)`, qi(c)),
			fmt.Sprintf(`coalesce(sum(typeof(%[1]s)='blob' and json_valid(%[1]s,8)),0)`, qi(c)))
	}
	rs, err := db.Query(`select ` + strings.Join(exprs, ", ") + ` from ` + qi(t)) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli (identifiers cannot be bound parameters; every name goes through qi(), which doubles quotes)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	vals := make([]int64, len(exprs))
	ptrs := make([]any, len(exprs))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
	}
	for i, c := range cols {
		n, ok := vals[2*i], vals[2*i+1]
		if n > 0 {
			if n == ok {
				out[c] = "jsonb"
			} else {
				out[c] = "opaque"
			}
		}
	}
	return out, rs.Err()
}

// redactValue masks identifiers in a value read from column col: whole personal columns, any 15+ digit number
// (as text, integer or float) and long-digit text inside nested JSON.
func redactValue(col string, v any) any {
	switch x := v.(type) {
	case string:
		if personalColumn(col) && x != "" {
			return "<redacted>"
		}
		return longNum.ReplaceAllString(x, "<id>")
	case int64:
		if personalColumn(col) || x >= 1e14 || x <= -1e14 {
			return "<redacted>"
		}
	case float64:
		if personalColumn(col) || math.Abs(x) >= 1e14 {
			return "<redacted>"
		}
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[longNum.ReplaceAllString(k, "<id>")] = redactValue("", e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = redactValue("", e)
		}
		return out
	}
	return v
}

// RedactText masks long digit runs in free text (for example an edit description that quotes SQL).
func RedactText(s string) string { return longNum.ReplaceAllString(s, "<id>") }

// present builds an output row; with redaction on it masks the data and the key (comparison already happened on
// raw values, so masking can never hide a difference).
func present(key []any, data map[string]any, redact bool) Row {
	if !redact {
		return Row{Key: key, Data: data}
	}
	k := make([]any, len(key))
	for i, v := range key {
		k[i] = redactValue("", v)
	}
	d := make(map[string]any, len(data))
	for c, v := range data {
		d[c] = redactValue(c, v)
	}
	return Row{Key: k, Data: d}
}

// presentChange masks the key, paths and values of a modified row. A change whose masked before and after look
// identical (an identifier was edited) is still listed, so the review shows that the field changed.
func presentChange(key []any, changes []Change, redact bool) RowChange {
	if !redact {
		return RowChange{Key: key, Changes: changes}
	}
	k := make([]any, len(key))
	for i, v := range key {
		k[i] = redactValue("", v)
	}
	out := make([]Change, len(changes))
	for i, c := range changes {
		col := c.Path
		if cut := strings.IndexAny(col, ".["); cut >= 0 {
			col = col[:cut]
		}
		out[i] = Change{Kind: c.Kind, Path: longNum.ReplaceAllString(c.Path, "<id>"), Before: redactValue(col, c.Before), After: redactValue(col, c.After)}
	}
	return RowChange{Key: k, Changes: out}
}

func readRows(db querier, t string) (map[string]keyed, []string, error) {
	rs, err := db.Query(fmt.Sprintf(`select name, pk from pragma_table_info(%s) order by cid`, "'"+strings.ReplaceAll(t, "'", "''")+"'")) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli (identifiers cannot be bound parameters; every name goes through qi(), which doubles quotes)
	if err != nil {
		return nil, nil, err
	}
	type col struct {
		name string
		pk   int
	}
	var cols []col
	for rs.Next() {
		var c col
		if err := rs.Scan(&c.name, &c.pk); err != nil {
			rs.Close()
			return nil, nil, err
		}
		cols = append(cols, c)
	}
	rs.Close()
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.name
	}
	var pkCols []string
	byPK := append([]col(nil), cols...)
	sort.SliceStable(byPK, func(i, j int) bool { return byPK[i].pk < byPK[j].pk })
	for _, c := range byPK {
		if c.pk > 0 {
			pkCols = append(pkCols, c.name)
		}
	}
	kinds, err := blobKinds(db, t, names)
	if err != nil {
		return nil, nil, err
	}
	sel := make([]string, len(names))
	for i, n := range names {
		switch kinds[n] {
		case "jsonb":
			sel[i] = "json(" + qi(n) + ")"
		case "opaque":
			sel[i] = "hex(" + qi(n) + ")"
		default:
			sel[i] = qi(n)
		}
	}
	rows, err := db.Query(`select ` + strings.Join(sel, ", ") + ` from ` + qi(t)) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query, go.lang.security.audit.sqli.gosql-sqli.gosql-sqli (identifiers cannot be bound parameters; every name goes through qi(), which doubles quotes)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := map[string]keyed{}
	multiset := map[string][]map[string]any{}
	for rows.Next() {
		vals := make([]any, len(names))
		ptrs := make([]any, len(names))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		d := make(map[string]any, len(names))
		for i, n := range names {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			if kinds[n] == "jsonb" {
				if s, ok := v.(string); ok {
					if j, err := decodeJSON(s); err != nil {
						v = "<malformed-jsonb>"
					} else {
						v = j
					}
				}
			}
			d[n] = v
		}
		if len(pkCols) > 0 {
			key := make([]any, len(pkCols))
			var ks []string
			for i, k := range pkCols {
				key[i] = d[k]
				ks = append(ks, fmt.Sprintf("%T:%v", d[k], d[k])) // typed, so 1 and "1" and NULL and "<nil>" differ
			}
			out[strings.Join(ks, "\x1f")] = keyed{key: key, data: d}
		} else { // no primary key: identity is the full row content, as a multiset
			b, _ := json.Marshal(d) // map keys are sorted by encoding/json
			h := sha256.Sum256(b)
			hs := hex.EncodeToString(h[:6])
			multiset[hs] = append(multiset[hs], d)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(pkCols) == 0 {
		for h, lst := range multiset {
			for i, d := range lst {
				out[fmt.Sprintf("%s\x1f%d", h, i)] = keyed{key: []any{h, i}, data: d}
			}
		}
		pkCols = []string{"<row-hash>"}
	}
	return out, pkCols, rows.Err()
}

// decodeJSON parses JSON keeping integers as int64 (so counters and timestamps compare exactly) and only true
// fractions as float64 (which compare with a tolerance). Integers too large for int64 stay as text.
func decodeJSON(s string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var j any
	if err := dec.Decode(&j); err != nil {
		return nil, err
	}
	return normalizeNumbers(j), nil
}

func normalizeNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil && strings.ContainsAny(x.String(), ".eE") {
			return f
		}
		return x.String()
	case map[string]any:
		for k, e := range x {
			x[k] = normalizeNumbers(e)
		}
	case []any:
		for i, e := range x {
			x[i] = normalizeNumbers(e)
		}
	}
	return v
}

func short(v any) any {
	s, ok := v.(string)
	if !ok {
		b, err := json.Marshal(v)
		if err != nil {
			return v
		}
		if len(b) <= 120 {
			return v
		}
		s = string(b)
	}
	if len(s) > 120 {
		return s[:117] + "..."
	}
	return s
}

// jdiff lists the differences between two JSON-like values. Floats compare with a relative tolerance.
func jdiff(a, b any, path string, eps float64) []Change {
	var out []Change
	switch x := a.(type) {
	case map[string]any:
		if y, ok := b.(map[string]any); ok {
			keys := map[string]bool{}
			for k := range x {
				keys[k] = true
			}
			for k := range y {
				keys[k] = true
			}
			sorted := make([]string, 0, len(keys))
			for k := range keys {
				sorted = append(sorted, k)
			}
			sort.Strings(sorted)
			for _, k := range sorted {
				p := k
				if path != "" {
					p = path + "." + k
				}
				xv, inX := x[k]
				yv, inY := y[k]
				switch {
				case !inX:
					out = append(out, Change{"+", p, nil, short(yv)})
				case !inY:
					out = append(out, Change{"-", p, short(xv), nil})
				default:
					out = append(out, jdiff(xv, yv, p, eps)...)
				}
			}
			return out
		}
	case []any:
		if y, ok := b.([]any); ok {
			if len(x) != len(y) {
				return []Change{{"~", fmt.Sprintf("%s[len %d->%d]", path, len(x), len(y)), short(x), short(y)}}
			}
			for i := range x {
				out = append(out, jdiff(x[i], y[i], fmt.Sprintf("%s[%d]", path, i), eps)...)
			}
			return out
		}
	case float64:
		if y, ok := b.(float64); ok {
			if math.Abs(x-y) > eps*math.Max(1, math.Max(math.Abs(x), math.Abs(y))) {
				return []Change{{"~", path, x, y}}
			}
			return nil
		}
	}
	if !equalValues(a, b) {
		return []Change{{"~", path, short(a), short(b)}}
	}
	return nil
}

// equalValues compares scalars, treating int64 and float64 with the same value as equal.
func equalValues(a, b any) bool {
	if ia, ok := a.(int64); ok {
		if ib, ok := b.(int64); ok {
			return ia == ib // exact: integers beyond float64's precision must not compare equal through a float
		}
	}
	if fa, ok := toFloat(a); ok {
		if fb, ok := toFloat(b); ok {
			return fa == fb
		}
	}
	return fmt.Sprintf("%T:%v", a, a) == fmt.Sprintf("%T:%v", b, b)
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

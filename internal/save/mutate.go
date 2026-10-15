package save

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// fkCounts runs PRAGMA foreign_key_check and returns the dangling references, keyed per child row
// ("child table #rowid -> parent table (fk id)"). Rows of WITHOUT ROWID tables report no rowid, so they are keyed by
// their primary-key values instead ("child table [pk] -> parent table (fk id)", see withoutRowidKeys); only if those
// cannot be worked out do they fall back to one "child table -> parent table (fk id)" group, which cannot tell an
// edit that repairs one orphan and creates another from no change. It works whatever the connection's foreign_keys
// setting is.
func fkCounts(q interface {
	Query(q string, args ...any) (*sql.Rows, error)
}) (map[string]int, error) {
	rs, err := q.Query(`pragma foreign_key_check`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[string]int{}
	type group struct {
		table, parent string
		fkid          int
	}
	groups := map[group]int{} // WITHOUT ROWID violations, counted per foreign key
	for rs.Next() {
		var table, parent string
		var rowid sql.NullInt64
		var fkid int
		if err := rs.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return nil, err
		}
		if rowid.Valid {
			out[fmt.Sprintf("%s #%d -> %s (fk %d)", table, rowid.Int64, parent, fkid)]++
		} else {
			groups[group{table, parent, fkid}]++
		}
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	rs.Close() // the single connection must be free before the keys are looked up
	for g, n := range groups {
		keys := withoutRowidKeys(q, g.table, g.parent, g.fkid)
		if len(keys) != n { // could not name every row: count the group
			out[fmt.Sprintf("%s -> %s (fk %d)", g.table, g.parent, g.fkid)] += n
			continue
		}
		for _, k := range keys {
			out[fmt.Sprintf("%s [%s] -> %s (fk %d)", g.table, k, g.parent, g.fkid)]++
		}
	}
	return out, nil
}

// maxKeyedOrphans bounds the per-row keys kept for one foreign key of a WITHOUT ROWID table; a save with more
// existing orphans than this is counted as one group.
const maxKeyedOrphans = 20000

func qIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

// withoutRowidKeys names the rows of a WITHOUT ROWID table that break one foreign key, by the JSON array of their
// primary-key values, by looking for children that have no matching parent. It returns nil whenever it cannot, so the
// caller counts the group instead. Child columns that are NULL never break a key (SQLite's default MATCH SIMPLE).
func withoutRowidKeys(q interface {
	Query(q string, args ...any) (*sql.Rows, error)
}, table, parent string, fkid int) []string {
	cols := func(query string, args ...any) []string {
		rs, err := q.Query(query, args...)
		if err != nil {
			return nil
		}
		defer rs.Close()
		var out []string
		for rs.Next() {
			var c sql.NullString
			if rs.Scan(&c) != nil {
				return nil
			}
			out = append(out, c.String)
		}
		return out
	}
	from := cols(`select "from" from pragma_foreign_key_list(?) where id=? order by seq`, table, fkid)
	to := cols(`select coalesce("to",'') from pragma_foreign_key_list(?) where id=? order by seq`, table, fkid)
	pk := cols(`select name from pragma_table_info(?) where pk>0 order by pk`, table)
	if len(from) == 0 || len(from) != len(to) || len(pk) == 0 {
		return nil
	}
	if to[0] == "" { // the parent's primary key
		to = cols(`select name from pragma_table_info(?) where pk>0 order by pk`, parent)
		if len(to) != len(from) {
			return nil
		}
	}
	var present, match, key []string
	for i := range from {
		present = append(present, "c."+qIdent(from[i])+" is not null")
		match = append(match, "p."+qIdent(to[i])+" = c."+qIdent(from[i]))
	}
	for _, c := range pk {
		key = append(key, "cast(c."+qIdent(c)+" as text)")
	}
	rs, err := q.Query(`select json_array(` + strings.Join(key, ",") + `) from ` + qIdent(table) + ` c where ` +
		strings.Join(present, " and ") + ` and not exists (select 1 from ` + qIdent(parent) + ` p where ` + strings.Join(match, " and ") + `)`)
	if err != nil {
		return nil
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var k string
		if rs.Scan(&k) != nil || len(out) >= maxKeyedOrphans {
			return nil
		}
		out = append(out, k)
	}
	if rs.Err() != nil {
		return nil
	}
	return out
}

// worse lists the dangling references that exist after an edit but not before it. Per-row keys mean an edit that
// repairs one orphan and creates a different one is still refused.
func worse(before, after map[string]int) []string {
	var bad []string
	for k, n := range after {
		if n > before[k] {
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	return bad
}

// Mut is the handle a mutation receives. It is valid only inside the function passed to Mutate and is the only way
// to change the working copy: every edit is one transaction, so a failed step rolls the whole edit back.
type Mut struct {
	tx *sql.Tx
	// Desc is the human-readable description recorded for the edit when it changed something. Mutate sets it from
	// its desc argument; the function may replace it once it knows the outcome (for example a row count).
	Desc string
}

// Exec runs one statement (or, with no arguments, a script of several) inside the edit's transaction.
func (m *Mut) Exec(q string, args ...any) (sql.Result, error) {
	res, err := m.tx.Exec(q, args...)
	noteSQLError(q, err)
	return res, err
}

// Query returns rows as maps, inside the edit's transaction.
func (m *Mut) Query(q string, args ...any) ([]Row, error) { return TxQuery(m.tx, q, args...) }

// One returns the first row, or nil.
func (m *Mut) One(q string, args ...any) (Row, error) {
	rows, err := m.Query(q, args...)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// Mutate is the single write path. It runs fn in one transaction on the working copy: any error rolls everything
// back and nothing is recorded; on success the save becomes dirty if rows changed and the edit's description is
// recorded in the op list (Ops; Pending is derived from it). It returns the number of rows the transaction changed (SQLite total_changes, so rows
// changed by triggers count too). A save that is read-only (unknown database objects) refuses the edit.
func (s *Save) Mutate(desc string, fn func(m *Mut) error) (int64, error) {
	return s.mutate(desc, false, fn)
}

// MutateCascade is Mutate with SQLite's foreign-key enforcement switched on for this one edit, so a delete cascades
// (or sets NULL) exactly as the game's schema declares. Use it for deletes of rows other rows hang from, such as an
// item that owns an inventory. The connection normally runs with enforcement off (the game's own setting is not yet
// confirmed, audit F-08 / T4), so this is opt-in per edit.
func (s *Save) MutateCascade(desc string, fn func(m *Mut) error) (int64, error) {
	return s.mutate(desc, true, fn)
}

func (s *Save) mutate(desc string, cascade bool, fn func(m *Mut) error) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.blocked != "" {
		return 0, errReadOnly(s.blocked)
	}
	if fn == nil {
		return 0, errors.New("no edit given")
	}
	if cascade {
		// The pragma cannot change inside a transaction. The pool has one connection, so this applies to the
		// transaction below, and is switched back off however the edit ends.
		if _, err := s.db.Exec("PRAGMA foreign_keys=ON"); err != nil {
			return 0, err
		}
		defer func() {
			// If enforcement cannot be switched back off, later edits would run with it on; refuse them instead.
			if _, err := s.db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
				s.blocked = "could not restore the connection's foreign-key setting: " + err.Error() + "; restart the editor"
			}
		}()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	// A panic inside fn must not leave the transaction open: the pool has one connection, so an open transaction
	// would block every later call until the editor is restarted.
	finished := false
	defer func() {
		if !finished {
			tx.Rollback()
		}
	}()
	var before, after int64
	if err := tx.QueryRow("select total_changes()").Scan(&before); err != nil {
		tx.Rollback()
		return 0, err
	}
	m := &Mut{tx: tx, Desc: desc}
	if err := fn(m); err != nil {
		tx.Rollback()
		return 0, err
	}
	if err := tx.QueryRow("select total_changes()").Scan(&after); err != nil {
		tx.Rollback()
		return 0, err
	}
	// Gate: an edit may not leave more dangling references than the save had before it (audit F-08). Existing
	// violations in the file are tolerated (the game wrote them); only new ones refuse the edit.
	// A scan is O(save size), so it is skipped when the edit changed no rows (nothing can have been broken).
	fk := s.fkBase
	if after > before {
		var err error
		if fk, err = fkCounts(tx); err != nil {
			return 0, err
		}
		if bad := worse(s.fkBase, fk); len(bad) > 0 {
			more := ""
			if len(bad) > 5 {
				more = fmt.Sprintf(" and %d more", len(bad)-5)
				bad = bad[:5]
			}
			return 0, fmt.Errorf("this edit would leave dangling references (%s%s); nothing was changed", strings.Join(bad, "; "), more)
		}
	}
	finished = true
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	s.fkBase = fk
	n := after - before
	if n > 0 {
		s.dirty = true
		if m.Desc != "" {
			s.ops = append(s.ops, Op{Desc: m.Desc, Rows: n, At: time.Now()})
		}
	}
	return n, nil
}

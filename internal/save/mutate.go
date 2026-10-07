package save

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// fkCounts runs PRAGMA foreign_key_check and returns the number of dangling references per
// "child table -> parent table (fk id)". It works whatever the connection's foreign_keys setting is.
func fkCounts(q interface {
	Query(q string, args ...any) (*sql.Rows, error)
}) (map[string]int, error) {
	rs, err := q.Query(`pragma foreign_key_check`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[string]int{}
	for rs.Next() {
		var table, parent string
		var rowid sql.NullInt64
		var fkid int
		if err := rs.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return nil, err
		}
		out[fmt.Sprintf("%s -> %s (fk %d)", table, parent, fkid)]++
	}
	return out, rs.Err()
}

// worse lists the dangling-reference groups that grew between before and after.
func worse(before, after map[string]int) []string {
	var bad []string
	for k, n := range after {
		if n > before[k] {
			bad = append(bad, fmt.Sprintf("%d new in %s", n-before[k], k))
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
// added to the pending list. It returns the number of rows the transaction changed (SQLite total_changes, so rows
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
	if cascade {
		// The pragma cannot change inside a transaction. The pool has one connection, so this applies to the
		// transaction below, and is switched back off however the edit ends.
		if _, err := s.db.Exec("PRAGMA foreign_keys=ON"); err != nil {
			return 0, err
		}
		defer s.db.Exec("PRAGMA foreign_keys=OFF")
	}
	if fn == nil {
		return 0, errors.New("no edit given")
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
	fk, err := fkCounts(tx)
	if err != nil {
		return 0, err
	}
	if bad := worse(s.fkBase, fk); len(bad) > 0 {
		return 0, fmt.Errorf("this edit would leave dangling references (%s); nothing was changed", strings.Join(bad, "; "))
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
			s.pending = append(s.pending, m.Desc)
		}
	}
	return n, nil
}

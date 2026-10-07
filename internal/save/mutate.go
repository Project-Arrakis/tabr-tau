package save

import (
	"database/sql"
	"errors"
)

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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.blocked != "" {
		return 0, errReadOnly(s.blocked)
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
	finished = true
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	n := after - before
	if n > 0 {
		s.dirty = true
		if m.Desc != "" {
			s.pending = append(s.pending, m.Desc)
		}
	}
	return n, nil
}

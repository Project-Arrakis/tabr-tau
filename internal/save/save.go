package save

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const GameProcess = "DuneSandbox-Win64-Shipping.exe"

// Row is one result row keyed by column name.
type Row = map[string]any

// Save is an editable copy of a save file. Nothing touches the real file until
// Commit is called.
type Save struct {
	Path string

	mu       sync.Mutex
	work     string // decoded working copy on disk
	db       *sql.DB
	diskHash [32]byte
	dirty    bool
	pending  []string
}

// sqlErrorHook, when set, is called for every statement that fails. It exists so tests can notice SQL errors that
// the code under test swallows (for example a query naming a column the real game schema does not have).
// Not safe for concurrent use: set it once at the start of a test that does not run in parallel.
var sqlErrorHook func(q string, err error)

// SetSQLErrorHook installs (or, with nil, removes) the test hook described on sqlErrorHook.
func SetSQLErrorHook(f func(q string, err error)) { sqlErrorHook = f }

func noteSQLError(q string, err error) {
	if err != nil && sqlErrorHook != nil {
		sqlErrorHook(q, err)
	}
}

// Open decodes path (a save file, or the folder containing game.db).
func Open(path string) (*Save, error) {
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		path = filepath.Join(path, "game.db")
	}
	s := &Save{Path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Save) load() error {
	blob, err := os.ReadFile(s.Path)
	if err != nil {
		return err
	}
	raw, err := Decode(blob)
	if err != nil {
		return err
	}
	if s.db != nil {
		s.db.Close()
		os.Remove(s.work)
	}
	f, err := os.CreateTemp("", "tabr-tau-*.sqlite")
	if err != nil {
		return err
	}
	s.work = f.Name()
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	f.Close()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(s.work)+"?_pragma=journal_mode(delete)&_pragma=foreign_keys(0)")
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1) // one connection keeps PRAGMAs and read-only mode consistent
	db.SetConnMaxLifetime(0)
	s.db = db
	s.diskHash = sha256.Sum256(blob)
	s.dirty = false
	s.pending = nil
	return nil
}

// Close releases the working copy.
func (s *Save) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		s.db.Close()
		os.Remove(s.work)
	}
}

func (s *Save) Dirty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty
}

func (s *Save) Pending() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.pending...)
}

// Log records a human-readable description of an edit.
func (s *Save) Log(format string, a ...any) {
	s.mu.Lock()
	s.pending = append(s.pending, fmt.Sprintf(format, a...))
	s.mu.Unlock()
}

// Exec runs one write statement against the working copy.
func (s *Save) Exec(q string, args ...any) (sql.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(q, args...)
	noteSQLError(q, err)
	if err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			s.dirty = true
		}
	}
	return res, err
}

// Tx runs fn in a transaction; any error rolls it back. Reads inside fn must
// use the tx (the pool holds a single connection).
func (s *Save) Tx(fn func(tx *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.dirty = true
	return nil
}

// ExecScript runs one or more write statements atomically. Nothing is kept if
// any statement fails. It returns the number of rows changed.
func (s *Save) ExecScript(script string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	var before, after int64
	tx.QueryRow("select total_changes()").Scan(&before)
	if _, err := tx.Exec(script); err != nil {
		noteSQLError(script, err)
		tx.Rollback()
		return 0, err
	}
	tx.QueryRow("select total_changes()").Scan(&after)
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if after > before {
		s.dirty = true
	}
	return after - before, nil
}

// Query returns rows as maps. BLOBs become "<blob NB> hex" strings.
func (s *Save) Query(q string, args ...any) ([]Row, error) {
	cols, rows, err := s.Table(q, args...)
	if err != nil {
		return nil, err
	}
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		m := Row{}
		for i, c := range cols {
			m[c] = r[i]
		}
		out = append(out, m)
	}
	return out, nil
}

// One returns the first row, or nil.
func (s *Save) One(q string, args ...any) (Row, error) {
	rows, err := s.Query(q, args...)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// Table returns ordered columns and rows.
func (s *Save) Table(q string, args ...any) ([]string, [][]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cols, rows, err := queryTable(s.db, q, args...)
	noteSQLError(q, err)
	return cols, rows, err
}

type querier interface {
	Query(q string, args ...any) (*sql.Rows, error)
}

func queryTable(db querier, q string, args ...any) ([]string, [][]any, error) {
	rs, err := db.Query(q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	cols, _ := rs.Columns()
	out := [][]any{}
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				n := len(b)
				if n > 24 {
					b = b[:24]
				}
				vals[i] = fmt.Sprintf("<blob %dB> %x", n, b)
			}
		}
		out = append(out, vals)
	}
	return cols, out, rs.Err()
}

// TxQuery is Query inside a transaction.
func TxQuery(tx *sql.Tx, q string, args ...any) ([]Row, error) {
	cols, rows, err := queryTable(tx, q, args...)
	noteSQLError(q, err)
	if err != nil {
		return nil, err
	}
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		m := Row{}
		for i, c := range cols {
			m[c] = r[i]
		}
		out = append(out, m)
	}
	return out, nil
}

// ReadOnlyTable runs an arbitrary query with PRAGMA query_only enabled.
func (s *Save) ReadOnlyTable(q string, limit int) ([]string, [][]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec("PRAGMA query_only=ON")
	defer s.db.Exec("PRAGMA query_only=OFF")
	cols, rows, err := queryTable(s.db, q)
	if err == nil && len(rows) > limit {
		rows = rows[:limit]
	}
	return cols, rows, err
}

// Discard throws away unsaved edits.
func (s *Save) Discard() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// GameRunning reports whether the Dune client process is alive (Windows only).
func GameRunning() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+GameProcess, "/NH").Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), strings.ToLower(GameProcess))
}

func (s *Save) backupDir() string { return filepath.Join(filepath.Dir(s.Path), "tabr-tau-backups") }

func (s *Save) backup() (string, error) {
	if err := os.MkdirAll(s.backupDir(), 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(s.Path)
	base := strings.TrimSuffix(filepath.Base(s.Path), ext)
	dest := filepath.Join(s.backupDir(), base+"-"+time.Now().Format("20060102-150405")+ext)
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return "", err
	}
	return dest, os.WriteFile(dest, b, 0o644)
}

// Commit backs up the original file, then atomically writes the edited save.
func (s *Save) Commit(force bool) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return map[string]any{"saved": false, "reason": "no changes"}, nil
	}
	if !force && GameRunning() {
		return nil, fmt.Errorf("%s is running; close the game first", GameProcess)
	}
	cur, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	if !force && sha256.Sum256(cur) != s.diskHash {
		return nil, errors.New("the save changed on disk since it was loaded (game autosave?); discard and redo the edits")
	}
	var chk string
	if err := s.db.QueryRow("PRAGMA integrity_check").Scan(&chk); err != nil || chk != "ok" {
		return nil, fmt.Errorf("integrity check failed: %v %s", err, chk)
	}
	raw, err := os.ReadFile(s.work)
	if err != nil {
		return nil, err
	}
	blob, err := Encode(raw)
	if err != nil {
		return nil, err
	}
	if back, err := Decode(blob); err != nil || !bytes.Equal(back, raw) {
		return nil, errors.New("round-trip verification failed; nothing written")
	}
	bak, err := s.backup()
	if err != nil {
		return nil, err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return nil, err
	}
	ops := s.pending
	if err := s.load(); err != nil {
		return nil, err
	}
	return map[string]any{"saved": true, "backup": bak, "ops": ops}, nil
}

// Backups lists backups, newest first.
func (s *Save) Backups() []Row {
	ents, _ := os.ReadDir(s.backupDir())
	sort.Slice(ents, func(i, j int) bool { return ents[i].Name() > ents[j].Name() })
	out := []Row{}
	for _, e := range ents {
		if info, err := e.Info(); err == nil {
			out = append(out, Row{"name": e.Name(), "size": info.Size(), "modified": info.ModTime().Format(time.RFC3339)})
		}
	}
	return out
}

// Restore replaces the save with a backup (the current file is backed up first).
func (s *Save) Restore(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if GameRunning() {
		return fmt.Errorf("%s is running; close the game first", GameProcess)
	}
	b, err := os.ReadFile(filepath.Join(s.backupDir(), filepath.Base(name)))
	if err != nil {
		return err
	}
	if _, err := Decode(b); err != nil {
		return err
	}
	if _, err := s.backup(); err != nil {
		return err
	}
	if err := os.WriteFile(s.Path, b, 0o644); err != nil {
		return err
	}
	return s.load()
}

// Discover finds game.db under the default Dune client storage location.
func Discover() []string {
	base := filepath.Join(os.Getenv("LOCALAPPDATA"), "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage")
	m, _ := filepath.Glob(filepath.Join(base, "*", "*", "game.db"))
	return m
}

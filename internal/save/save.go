package save

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Row is one result row keyed by column name.
type Row = map[string]any

// Save is an editable copy of a save file. Nothing touches the real file until
// Commit is called.
type Save struct {
	Path string

	mu       sync.Mutex
	work     string // decoded working copy on disk
	db       *sql.DB
	rdb      *sql.DB        // separate read-only connection for the SQL console
	blocked  string         // why writes are refused (unknown database objects), or ""
	fkBase   map[string]int // dangling references present before the current edit (see Mutate)
	diskHash [32]byte
	dirty    bool
	ops      []Op    // structured record of the edits applied since load
	orig     string  // pristine decoded copy of the file as loaded; never written
	odb      *sql.DB // read-only handle on orig
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

// Op is one recorded edit: what it was, how many rows it changed, and when.
type Op struct {
	Desc string    `json:"desc"`
	Rows int64     `json:"rows"`
	At   time.Time `json:"at"`
}

// KnownSchema is the DDL of the real single-player game database (tables, indexes and the game's one trigger).
//
//go:embed schema.sql
var KnownSchema string

// WriteBlocked reports why this save is read-only, or "" when edits are allowed. A save carrying triggers, views
// or virtual tables the editor does not know could run hidden logic whenever an ordinary edit touches a table, and
// the editor's diff would never show it (audit SEC-4).
func (s *Save) WriteBlocked() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blocked
}

func errReadOnly(reason string) error {
	return fmt.Errorf("this save is read-only: %s", reason)
}

// codeObjects returns the database objects that can carry logic, keyed by "type name", with their exact SQL.
// Triggers and views are always included; a virtual table is recognised by rootpage 0 (not by its SQL text, which
// comments can disguise). Only SQLite's own internal tables (sqlite_sequence, sqlite_stat1, ...) are skipped, and
// only when they are tables: a trigger or view named sqlite_* is still reported.
func codeObjects(q interface {
	Query(q string, args ...any) (*sql.Rows, error)
}) (map[string]string, error) {
	rs, err := q.Query(`select type, name, coalesce(sql,''), rootpage from sqlite_master`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[string]string{}
	for rs.Next() {
		var typ, name, def string
		var root sql.NullInt64
		if err := rs.Scan(&typ, &name, &def, &root); err != nil {
			return nil, err
		}
		switch {
		case typ == "trigger" || typ == "view":
		case typ == "table" && root.Valid && root.Int64 == 0:
			if strings.HasPrefix(strings.ToLower(name), "sqlite_") {
				continue
			}
		default:
			continue
		}
		out[typ+" "+name] = def
	}
	return out, rs.Err()
}

var knownCode = sync.OnceValues(func() (map[string]string, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(KnownSchema); err != nil {
		return nil, err
	}
	return codeObjects(db)
})

// fingerprint compares the code-carrying objects of db with the real game schema.
func fingerprint(db *sql.DB) string {
	want, err := knownCode()
	if err != nil {
		return "the built-in game schema could not be loaded: " + err.Error()
	}
	got, err := codeObjects(db)
	if err != nil {
		return "the save's schema could not be read: " + err.Error()
	}
	var bad []string
	for k, v := range got {
		if w, ok := want[k]; !ok {
			bad = append(bad, "unknown "+k)
		} else if w != v {
			bad = append(bad, "modified "+k)
		}
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			bad = append(bad, "missing "+k)
		}
	}
	if len(bad) == 0 {
		return ""
	}
	sort.Strings(bad)
	return strings.Join(bad, "; ")
}

// ReadTimeout bounds every read-only console query.
var ReadTimeout = 5 * time.Second

// Open decodes path (a save file, or the folder containing game.db).
func Open(path string) (*Save, error) {
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		path = filepath.Join(path, "game.db")
	}
	s := &Save{Path: path}
	cleanStaleTemps(path)
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// openCopy writes raw to a new private temp file and opens it with the given DSN options. On any error nothing is
// left behind.
func openCopy(raw []byte, pattern, opts string) (path string, db *sql.DB, err error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	path = f.Name()
	if _, err := f.Write(raw); err != nil {
		f.Close()
		os.Remove(path)
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", nil, err
	}
	db, err = sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?"+opts)
	if err != nil {
		os.Remove(path)
		return "", nil, err
	}
	db.SetMaxOpenConns(1) // one connection keeps PRAGMAs and read-only mode consistent
	db.SetConnMaxLifetime(0)
	return path, db, nil
}

// load (re)reads the file on disk into a fresh working copy, read-only handle and pristine baseline. The new copies
// are built first and swapped in only when all of them succeeded, so a failure leaves the previous state intact and
// leaks no temp files.
func (s *Save) load() error {
	blob, err := os.ReadFile(s.Path)
	if err != nil {
		return err
	}
	raw, err := Decode(blob)
	if err != nil {
		return err
	}
	work, db, err := openCopy(raw, "tabr-tau-*.sqlite", "_pragma=journal_mode(delete)&_pragma=foreign_keys(0)")
	if err != nil {
		return err
	}
	rdb, err := sql.Open("sqlite", "file:"+filepath.ToSlash(work)+"?mode=ro&_pragma=busy_timeout(2000)&_pragma=query_only(1)")
	if err != nil {
		db.Close()
		os.Remove(work)
		return err
	}
	rdb.SetMaxOpenConns(1)
	rdb.SetConnMaxLifetime(0)
	// The pristine baseline: an immutable copy of what was loaded, kept so the editor can show exactly what changed
	// (review before save) and prove afterwards that only the intended changes were written.
	orig, odb, err := openCopy(raw, "tabr-tau-orig-*.sqlite", "mode=ro&_pragma=query_only(1)")
	if err != nil {
		rdb.Close()
		db.Close()
		os.Remove(work)
		return err
	}
	blocked := fingerprint(db)
	base, fkErr := fkCounts(db)
	if fkErr != nil && blocked == "" { // never hide a more specific reason (unknown database objects)
		blocked = "the save's foreign-key state could not be read: " + fkErr.Error()
	}
	// swap
	if s.rdb != nil {
		s.rdb.Close()
	}
	if s.db != nil {
		s.db.Close()
		os.Remove(s.work)
	}
	if s.odb != nil {
		s.odb.Close()
	}
	if s.orig != "" {
		os.Remove(s.orig)
	}
	s.work, s.db, s.rdb = work, db, rdb
	s.orig, s.odb = orig, odb
	s.blocked, s.fkBase = blocked, base
	s.diskHash = sha256.Sum256(blob)
	s.dirty = false
	s.ops = nil
	return nil
}

// Close releases the working copy.
func (s *Save) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rdb != nil {
		s.rdb.Close()
	}
	if s.odb != nil {
		s.odb.Close()
	}
	if s.orig != "" {
		os.Remove(s.orig)
	}
	if s.db != nil {
		s.db.Close()
		os.Remove(s.work)
	}
}

// Ops returns the structured record of edits applied since the save was loaded (or last saved).
func (s *Save) Ops() []Op {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Op{}, s.ops...)
}

// ErrReviewChanged is returned when the pending changes are not the ones that were reviewed.
var ErrReviewChanged = errors.New("the pending changes are different from the ones you reviewed; review them again")

// reviewTokenLocked names the exact state a person reviews: the file this session loaded, the recorded edits and the
// content of the working copy (so an edit with no description, or two edits with the same description, still differ).
func (s *Save) reviewTokenLocked() (string, error) {
	raw, err := os.ReadFile(s.work)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(s.diskHash[:])
	fmt.Fprintf(h, "%d\n", len(s.ops))
	for _, o := range s.ops {
		fmt.Fprintf(h, "%d:%s\n", len(o.Desc), o.Desc)
	}
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

// ReviewToken returns the token for the current state.
func (s *Save) ReviewToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reviewTokenLocked()
}

// WithBaseline calls fn with read-only handles on the pristine copy of the save as loaded and on the working copy
// as edited so far. fn runs with the save locked, so it must not call back into the Save.
func (s *Save) WithBaseline(fn func(orig, current *sql.DB) error) error {
	return s.WithBaselineState(func(orig, current *sql.DB, _ bool, _ []Op) error { return fn(orig, current) })
}

// WithBaselineState is WithBaseline that also hands fn the dirty flag and the recorded ops as they are at the same
// instant as the databases, so a review is internally consistent.
func (s *Save) WithBaselineState(fn func(orig, current *sql.DB, dirty bool, ops []Op) error) error {
	return s.WithReview(func(orig, current *sql.DB, dirty bool, ops []Op, _ string) error {
		return fn(orig, current, dirty, ops)
	})
}

// WithReview is WithBaselineState that also hands fn the review token of that same instant. Like the others, fn runs
// with the save locked and must not call back into the Save.
func (s *Save) WithReview(fn func(orig, current *sql.DB, dirty bool, ops []Op, token string) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.odb == nil || s.rdb == nil {
		return errors.New("no save is loaded")
	}
	tok, err := s.reviewTokenLocked()
	if err != nil {
		return err
	}
	return fn(s.odb, s.rdb, s.dirty, append([]Op{}, s.ops...), tok)
}

func (s *Save) Dirty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty
}

func (s *Save) Pending() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.ops))
	for i, o := range s.ops {
		out[i] = o.Desc
	}
	return out
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
	// Reads use the read-only connection, so a statement that writes cannot slip in through Query/Table/One and
	// bypass Mutate (it fails with "attempt to write a readonly database").
	cols, rows, err := queryTable(s.rdb, q, args...)
	noteSQLError(q, err)
	return cols, rows, err
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	Query(q string, args ...any) (*sql.Rows, error)
}

func queryTable(db querier, q string, args ...any) ([]string, [][]any, error) {
	return queryTableCtx(context.Background(), db, 0, q, args...)
}

// queryTableCtx is queryTable with a context and a row cap (limit <= 0 means no cap). Reaching the cap stops
// reading and returns the rows so far without an error.
func queryTableCtx(ctx context.Context, db interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}, limit int, q string, args ...any) ([]string, [][]any, error) {
	rs, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	cols, _ := rs.Columns()
	out := [][]any{}
	for rs.Next() {
		if limit > 0 && len(out) >= limit {
			break
		}
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

// ReadOnlyTable runs an arbitrary query on a dedicated read-only connection (opened with mode=ro and
// query_only), so even a statement the caller failed to vet cannot write. It sees every committed edit of the
// working copy. At most limit rows are returned and the query is interrupted after ReadTimeout.
func (s *Save) ReadOnlyTable(q string, limit int) ([]string, [][]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), ReadTimeout)
	defer cancel()
	cols, rows, err := queryTableCtx(ctx, s.rdb, limit, q)
	if err != nil && ctx.Err() != nil {
		err = fmt.Errorf("query stopped after %s: %w", ReadTimeout, err)
	}
	return cols, rows, err
}

// Discard throws away unsaved edits.
func (s *Save) Discard() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Save) backupDir() string { return filepath.Join(filepath.Dir(s.Path), "tabr-tau-backups") }

func hashOf(b []byte) [32]byte { return sha256.Sum256(b) }

// backup copies the current file into the backup folder under a unique name, flushes it, and reads it back to
// prove the copy is identical. A backup that cannot be verified is an error, so no edit proceeds on a bad one.
func (s *Save) backup() (string, error) {
	if err := os.MkdirAll(s.backupDir(), 0o700); err != nil {
		return "", err
	}
	ext := filepath.Ext(s.Path)
	base := strings.TrimSuffix(filepath.Base(s.Path), ext)
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(s.backupDir(), fmt.Sprintf("%s-%s-%d%s", base, time.Now().Format("20060102-150405"), time.Now().UnixNano()%1_000_000_000, ext))
	if err := writeDurable(dest, b, 0o600); err != nil {
		return "", err
	}
	if back, err := os.ReadFile(dest); err != nil || !bytes.Equal(back, b) {
		os.Remove(dest)
		return "", errors.New("backup verification failed; nothing was changed")
	}
	return dest, nil
}

// Commit backs up the original file, then atomically writes the edited save. force skips only the check that the
// game is running; it never skips the changed-on-disk check. Commit does not require a review: the web route uses
// CommitReviewed, so a save from the UI is always one the person was shown.
func (s *Save) Commit(force bool) (map[string]any, error) { return s.commit(force, "", false) }

// CommitReviewed is Commit for a save the person reviewed (force has the same meaning as for Commit): token must be the one ReviewToken/WithReview returned for
// exactly the state being written. The comparison happens under the same lock as the write, so an edit that lands
// after the review cannot be saved unseen.
func (s *Save) CommitReviewed(token string, force bool) (map[string]any, error) {
	return s.commit(force, token, true)
}

func (s *Save) commit(force bool, token string, needToken bool) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return map[string]any{"saved": false, "reason": "no changes"}, nil
	}
	if needToken {
		if token == "" {
			return nil, errors.New("review the changes before saving")
		}
		want, err := s.reviewTokenLocked()
		if err != nil {
			return nil, err
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(want)) != 1 {
			return nil, ErrReviewChanged
		}
	}
	if s.blocked != "" {
		return nil, errReadOnly(s.blocked)
	}
	if err := s.requirePlainFile(); err != nil {
		return nil, err
	}
	if blocked, why := SaveBlocked(s.Path); !force && blocked {
		return nil, errors.New(why)
	}
	cur, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	if hashOf(cur) != s.diskHash {
		return nil, ErrChangedOnDisk
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
	if err := s.installBlob(blob); err != nil {
		return nil, err
	}
	ops := make([]string, len(s.ops))
	for i, o := range s.ops {
		ops[i] = o.Desc
	}
	// The file is written. From here a problem is a warning, not a failed save.
	s.diskHash = hashOf(blob)
	s.dirty = false
	s.ops = nil
	res := map[string]any{"saved": true, "backup": bak, "ops": ops}
	if err := reloadAfterCommit(s); err != nil {
		// The baseline and recorded state no longer match the file just written; refuse further edits rather than
		// let a stale baseline mislead the review.
		s.blocked = "the save was written but reloading it failed (" + err.Error() + "); restart the editor"
		res["warning"] = "saved, but reloading the file failed: " + err.Error() + " (further edits are disabled; restart the editor)"
	}
	return res, nil
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

// Restore replaces the save with a backup through the same durable path as Commit (and, because a restore reloads the file, only when no edits are pending): the backup must decode, pass
// an integrity check and carry no applied patch the current save lacks (it would come from a newer game); the
// current file is backed up first and is never truncated in place.
func (s *Save) Restore(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == "" || name != filepath.Base(name) || name == "." || name == ".." {
		return errors.New("invalid backup name")
	}
	if blocked, why := SaveBlocked(s.Path); blocked {
		return errors.New(why)
	}
	if s.dirty { // restoring reloads the file and would silently drop the pending edits
		return errors.New("you have unsaved edits; save or discard them first (restoring a backup would lose them)")
	}
	if err := s.requirePlainFile(); err != nil {
		return err
	}
	bp := filepath.Join(s.backupDir(), name)
	if st, err := os.Lstat(bp); err != nil || !st.Mode().IsRegular() {
		return errors.New("no such backup")
	}
	b, err := os.ReadFile(bp)
	if err != nil {
		return err
	}
	raw, err := Decode(b)
	if err != nil {
		return err
	}
	want, err := inspect(raw)
	if err != nil {
		return fmt.Errorf("backup is not a sound save: %w", err)
	}
	cur, err := os.ReadFile(s.Path)
	if err != nil {
		return err
	}
	if hashOf(cur) != s.diskHash {
		return ErrChangedOnDisk
	}
	curRaw, err := Decode(cur)
	if err != nil {
		return err
	}
	have, err := inspect(curRaw)
	if err != nil {
		return err
	}
	for p := range want {
		if !have[p] {
			return fmt.Errorf("backup has applied patch %q that the current save lacks (it is from a newer game version)", p)
		}
	}
	if _, err := s.backup(); err != nil {
		return err
	}
	if err := s.installBlob(b); err != nil {
		return err
	}
	s.diskHash = hashOf(b)
	if err := reloadAfterCommit(s); err != nil {
		// The file on disk is the restored backup but the working copy is stale: refuse further edits so the
		// stale copy can never be saved over the restore.
		s.blocked = "the backup was restored but reloading it failed (" + err.Error() + "); restart the editor"
		return errors.New(s.blocked)
	}
	return nil
}

// Discover finds game.db under the default Dune client storage location.
func Discover() []string {
	base := filepath.Join(os.Getenv("LOCALAPPDATA"), "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage")
	m, _ := filepath.Glob(filepath.Join(base, "*", "*", "game.db"))
	return m
}

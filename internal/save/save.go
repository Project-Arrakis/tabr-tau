package save

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
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
	diskSize int64 // size and modification time of the file when diskHash was taken: a cheap first test for "changed on disk"
	diskMod  time.Time

	// the size and modification time of a file already found to differ from the one loaded: the status poll asks again every few seconds, and reading and hashing a large
	// save each time made the answer slow, which is when a refresh decided from it could be out of date
	changedSize int64
	changedMod  time.Time

	dirty bool
	ops   []Op // structured record of the edits applied since load

	// the journal of pending edits (journal.go): where it is kept, what has been kept, and what became of the edits found in it at start
	journalPath string
	journalOff  string // why it was turned off for this session
	recs        []opRec
	recStmts    int
	replaying   bool
	restored    *Restored

	// the automatic write of pending edits (flush.go): one attempt at a time, and what became of the last
	flushMu   sync.Mutex
	lastFlush *Flush
	// the last write the editor made (an automatic one): the hash of the file it wrote, and the backup it was made after. A following write may reuse that backup only while
	// the file is still the one that write left, so a file the game saved since is never overwritten without a backup of its own.
	lastWrote   [32]byte
	lastBackup  string
	lastBackupT time.Time
	// the bytes of that backup when it was taken (it is reused only while they still are), and the first automatic backup of this session: the state before the editor's first
	// write, which the pruning never removes
	lastBackupHash [32]byte
	firstBackup    string
	// writing is the hash of the file being written, kept in the journal while it is written: a restart that finds the file to be that one knows the edits are already in it
	writing string
	// held is the queue the editor could not put back onto the game's newer file (an edit conflicted with what the game changed): none of it was written, and nothing new is
	// accepted until it is discarded. The queue stays in the journal.
	held *Held
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
	return apierr.Newf(apierr.ReadOnly, "this save is read-only: %s", reason)
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
		// Line endings are not logic: a real game save stores LF, the embedded schema file may be CRLF on a Windows checkout.
		out[typ+" "+name] = strings.ReplaceAll(def, "\r\n", "\n")
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

// Open decodes path (a save file, or the folder containing game.db). It removes nothing next to the save: the temp files a crashed run left are removed by LockSave, once this
// editor holds the lock, so a second editor started on the same save cannot delete the temp file of the first while it is writing.
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

// load (re)reads the file on disk into a fresh working copy and read-only handle. The new copies are built first and
// swapped in only when all of them succeeded, so a failure leaves the previous state intact and leaks no temp files.
func (s *Save) load() error {
	blob, err := readCapped(s.Path)
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
	s.work, s.db, s.rdb = work, db, rdb
	s.blocked, s.fkBase = blocked, base
	s.noteDisk(blob)
	s.dirty = false
	s.ops = nil
	s.recs, s.recStmts = nil, 0 // the journal file is left to whoever called: Open restores from it, Discard drops it
	return nil
}

// noteDisk records the file as it now is (its content hash, size and modification time) as the baseline the editor works from.
func (s *Save) noteDisk(blob []byte) {
	s.diskHash = hashOf(blob)
	s.diskSize, s.diskMod = 0, time.Time{}
	s.changedSize, s.changedMod = 0, time.Time{}
	if st, err := os.Stat(s.Path); err == nil {
		s.diskSize, s.diskMod = st.Size(), st.ModTime()
	}
}

// ChangedOnDisk says whether the file has been rewritten since this editor loaded or last wrote it. The game saves about every five minutes
// while a single-player session runs and again when it ends, and a save onto a file that changed is refused (ErrChangedOnDisk), so the
// editor says so as soon as it happens, not when Save is pressed. It compares size and modification time first and reads the file only
// when they differ, which keeps the status poll cheap.
func (s *Save) ChangedOnDisk() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := os.Stat(s.Path)
	if err != nil {
		return true
	}
	if st.Size() == s.diskSize && st.ModTime().Equal(s.diskMod) {
		return false
	}
	if st.Size() == s.changedSize && st.ModTime().Equal(s.changedMod) {
		return true // this very file was already read and found different
	}
	b, err := readCapped(s.Path)
	if err != nil {
		return true
	}
	if hashOf(b) == s.diskHash { // touched, not changed
		s.diskSize, s.diskMod = st.Size(), st.ModTime()
		return false
	}
	s.changedSize, s.changedMod = st.Size(), st.ModTime()
	return true
}

// Close releases the working copy.
func (s *Save) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rdb != nil {
		s.rdb.Close()
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

// WithBaseline calls fn with read-only handles on the pristine copy of the save as loaded and on the working copy as edited so far, so a test (or a diagnostic) can show exactly
// what an edit changed. The pristine copy is made from the file on disk when this is called (the editor does not keep one), so it is only available while the file is still
// the one that was loaded or last written; otherwise it returns an error. fn runs with the save locked, so it must not call back into the Save.
func (s *Save) WithBaseline(fn func(orig, current *sql.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rdb == nil {
		return errors.New("no save is loaded")
	}
	blob, err := readCapped(s.Path)
	if err != nil {
		return err
	}
	if hashOf(blob) != s.diskHash {
		return errors.New("the file changed on disk since it was loaded, so the state as loaded is no longer available")
	}
	raw, err := Decode(blob)
	if err != nil {
		return err
	}
	orig, odb, err := openCopy(raw, "tabr-tau-orig-*.sqlite", "mode=ro&_pragma=query_only(1)")
	if err != nil {
		return err
	}
	defer func() {
		odb.Close()
		os.Remove(orig)
	}()
	return fn(odb, s.rdb)
}

func (s *Save) Dirty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty || s.held != nil
}

func (s *Save) Pending() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held != nil {
		out := make([]string, len(s.held.ops))
		for i, o := range s.held.ops {
			out[i] = o.Desc
		}
		return out
	}
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
			switch x := v.(type) {
			case []byte:
				n := len(x)
				if n > 24 {
					x = x[:24]
				}
				vals[i] = fmt.Sprintf("<blob %dB> %x", n, x)
			case float64:
				if math.IsNaN(x) || math.IsInf(x, 0) { // JSON cannot carry these; one such cell must not blank a whole view (SEC-1)
					vals[i] = nil
				}
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

// Discard throws away unsaved edits. It waits for an automatic write that is running (flushMu), so the two never interleave. Throwing a queue away also ends the notice of
// what became of it; with nothing queued (the automatic refresh reading a newer file) the notice of edits dropped earlier stays until it is dismissed.
func (s *Save) Discard() error {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	queued := s.dirty || s.held != nil
	if err := s.load(); err != nil {
		return err
	}
	s.dropJournal()
	if queued {
		s.restored = nil
	}
	return nil
}

func (s *Save) backupDir() string { return filepath.Join(filepath.Dir(s.Path), "tabr-tau-backups") }

func hashOf(b []byte) [32]byte { return sha256.Sum256(b) }

// backup copies the current file into the backup folder under a unique name, flushes it, and reads it back to
// prove the copy is identical. A backup that cannot be verified is an error, so no edit proceeds on a bad one.
func (s *Save) backup() (string, error) {
	p, _, err := s.backupNamed("")
	return p, err
}

// backupNamed is backup with a tag in the file's name ("-auto" for the ones an automatic write takes: only those are ever removed by the pruning). It also returns the hash of
// the bytes it copied.
func (s *Save) backupNamed(tag string) (string, [32]byte, error) {
	if err := os.MkdirAll(s.backupDir(), 0o700); err != nil {
		return "", [32]byte{}, err
	}
	ext := filepath.Ext(s.Path)
	base := strings.TrimSuffix(filepath.Base(s.Path), ext)
	b, err := readCapped(s.Path)
	if err != nil {
		return "", [32]byte{}, err
	}
	now := time.Now()
	dest := filepath.Join(s.backupDir(), fmt.Sprintf("%s-%s-%d%s%s", base, now.Format("20060102-150405"), now.UnixNano()%1_000_000_000, tag, ext))
	if err := writeDurable(dest, b, 0o600); err != nil {
		return "", [32]byte{}, err
	}
	if back, err := readCapped(dest); err != nil || !bytes.Equal(back, b) {
		os.Remove(dest)
		return "", [32]byte{}, errors.New("backup verification failed; nothing was changed")
	}
	return dest, hashOf(b), nil
}

// Commit backs up the original file, then atomically writes the edited save, after the same checks as the automatic write (Flush): the game must not be running and the file
// must be the one that was loaded. Nothing skips them. The editor itself writes through Flush; this is the form that always takes a backup of its own.
func (s *Save) Commit() (map[string]any, error) { return s.commit(false, false) }

// commit is Commit; quick is the automatic write after each edit (Flush): it takes a backup only when the newest one is not from the last couple of minutes (a burst of
// edits is one restore point, taken before the first of them), and keeps the newest backups only.
func (s *Save) commit(force, quick bool) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return map[string]any{"saved": false, "reason": "no changes"}, nil
	}
	if s.blocked != "" {
		return nil, errReadOnly(s.blocked)
	}
	if err := s.requirePlainFile(); err != nil {
		return nil, err
	}
	if blocked, why := SaveBlocked(s.Path); !force && blocked {
		return nil, apierr.New(apierr.GameBlocked, why)
	}
	cur, err := readCapped(s.Path)
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
	var bak string
	if quick {
		bak, err = s.backupIfStale()
	} else {
		var sum [32]byte
		if bak, sum, err = s.backupNamed(""); err == nil && sum != s.diskHash {
			return nil, ErrChangedOnDisk // the game saved after the check above: the copy is not of the file the edits were made on
		}
	}
	if err != nil {
		return nil, err
	}
	s.noteWriting(blob)
	if err := s.installBlob(blob); err != nil {
		s.writing = ""
		s.writeJournal()
		return nil, err
	}
	if quick {
		s.lastWrote, s.lastBackup = hashOf(blob), bak
	}
	ops := make([]string, len(s.ops))
	for i, o := range s.ops {
		ops[i] = o.Desc
	}
	// The file is written. From here a problem is a warning, not a failed save.
	s.noteDisk(blob)
	s.dirty = false
	s.ops = nil
	s.dropJournal()
	if r := s.restored; r != nil { // the edits put back are written; the notice of the ones that were not stays until the person has seen it
		if len(r.Conflicts) == 0 {
			s.restored = nil
		} else {
			kept := *r
			kept.Written = true
			s.restored = &kept
		}
	}
	res := map[string]any{"saved": true, "backup": bak, "ops": ops}
	if err := reloadAfterCommit(s); err != nil {
		// The working copy and the recorded state no longer match the file just written; refuse further edits rather than
		// build them on a copy that is out of date.
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
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == "" || name != filepath.Base(name) || name == "." || name == ".." {
		return errors.New("invalid backup name")
	}
	if blocked, why := SaveBlocked(s.Path); blocked {
		return apierr.New(apierr.GameBlocked, why)
	}
	if s.dirty || s.held != nil { // restoring reloads the file and would silently drop the pending edits
		return apierr.New(apierr.Queued, "changes are still queued; wait until they are written, or discard them, first (restoring a backup would lose them)")
	}
	if err := s.requirePlainFile(); err != nil {
		return err
	}
	bp := filepath.Join(s.backupDir(), name)
	if st, err := os.Lstat(bp); err != nil || !st.Mode().IsRegular() {
		return errors.New("no such backup")
	}
	b, err := readCapped(bp)
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
	cur, err := readCapped(s.Path)
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
	s.noteDisk(b)
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

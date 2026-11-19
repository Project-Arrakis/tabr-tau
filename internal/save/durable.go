package save

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/kit/durable"
	"github.com/Project-Arrakis/tabr-tau/kit/savefile"
)

// Hooks so tests can inject faults (a locked file, a failing reload). Production code never changes them.
var (
	fsRename          = os.Rename
	renameBackoff     = 150 * time.Millisecond
	reloadAfterCommit = func(s *Save) error { return s.load() }
	syncFile          = func(f *os.File) error { return f.Sync() }
)

const renameAttempts = 6

// durableOpts is how the editor replaces files: six tries of the rename with a growing pause, and the hooks above so that tests can inject faults. The steps themselves are
// kit/durable's.
func durableOpts() durable.Options {
	return durable.Options{
		Attempts: renameAttempts,
		Pause:    renameBackoff,
		Growing:  true,
		Hooks:    durable.Hooks{Rename: fsRename, SyncFile: syncFile},
	}
}

// writeDurable writes data to a new file (exclusive create, so an existing file is never overwritten), flushes it
// to stable storage and closes it.
func writeDurable(path string, data []byte, mode os.FileMode) error {
	return durable.WriteNew(path, data, mode, durableOpts())
}

// renameRetry retries with a growing pause: on Windows an antivirus scanner, OneDrive or the game itself can hold
// the destination for a moment. want is the hash of the file the caller loaded and checked just before the first try (installBlob: s.diskHash): the file is read and compared again before
// every retry (#794), because a retry comes after a pause, up to about 2.25 s in all, and the game can save in that time. A file that changed, or that cannot be read to be compared, ends
// the tries with nothing renamed (the caller removes the temp file). The window that is left is the time between the last compare and the rename call.
func renameRetry(from, to string, want [32]byte) error {
	var refused error
	o := durableOpts()
	o.BeforeRetry = func(int) error {
		cur, err := savefile.ReadCapped(to)
		switch {
		case err != nil:
			refused = apierr.Wrap(apierr.ErrWriteFailed, "the save could not be read again to check that it is unchanged ("+shortErr(err)+"), so nothing was replaced; the edits stay queued", err)
		case hashOf(cur) != want:
			refused = ErrChangedOnDisk
		default:
			return nil
		}
		return refused
	}
	err := durable.Rename(from, to, o)
	if refused != nil && err == refused {
		return refused
	}
	if err != nil {
		return apierr.Wrap(apierr.ErrWriteFailed, fmt.Sprintf("could not replace %s (is another program holding it?): %v", filepath.Base(to), err), err)
	}
	return nil
}

// tmpPath is a unique sibling name; the ".tabr-*.tmp" pattern is what cleanStaleTemps removes.
func (s *Save) tmpPath() string {
	return fmt.Sprintf("%s.tabr-%d.tmp", s.Path, time.Now().UnixNano())
}

// cleanStaleTemps removes temp files a crashed earlier run left next to the save.
func cleanStaleTemps(path string) {
	m, _ := filepath.Glob(path + ".tabr-*.tmp")
	for _, f := range m {
		os.Remove(f)
	}
}

// requirePlainFile refuses a save path that is a symlink or not a regular file, so a link can never redirect a
// write to another location.
func (s *Save) requirePlainFile() error {
	st, err := os.Lstat(s.Path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file (symlinks are refused)", filepath.Base(s.Path))
	}
	return nil
}

// RequirePlainFolder refuses a backups folder that is a link (a symbolic link, or a junction on Windows), as requirePlainFile does for the save: a link would send the backups,
// and the removal of the old ones, to a folder somewhere else. A folder that does not exist yet is fine (it is created as a plain folder); one that is a file is not.
func RequirePlainFolder(dir string) error {
	st, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || !st.IsDir() {
		return fmt.Errorf("%s is not a plain folder (links are refused): the backups would not stay next to the save", filepath.Base(dir))
	}
	return nil
}

// installBlob atomically replaces the save file with blob: temp file, fsync, re-check that the file on disk is
// still the one this session loaded, rename with retry, then read back and compare.
func (s *Save) installBlob(blob []byte) error {
	tmp := s.tmpPath()
	if err := writeDurable(tmp, blob, 0o600); err != nil {
		return apierr.Wrap(apierr.ErrWriteFailed, "could not write the new file next to the save ("+shortErr(err)+"); nothing was changed", err)
	}
	// The game-running check of the caller was made well before this point: for the automatic write, in Flush before the hash and the encode, and again after the encode and before
	// the backup (commit); for the other write, at the top of commit. The backup and the temp file came after it, and on a large save that is seconds, in which a single-player session can start and the game rewrite the file). It is made again right before the rename, which is the step that cannot be undone
	// by the game's own write: a session that began in between leaves the edits queued instead. It comes before the look at the file on disk, not after, because the check
	// itself takes time in which the game can save: what the file is must be read last.
	if blocked, why := s.gate(); blocked {
		os.Remove(tmp)
		return apierr.New(apierr.ErrGameBlocked, why)
	}
	cur, err := savefile.ReadCapped(s.Path)
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if hashOf(cur) != s.diskHash {
		os.Remove(tmp)
		return ErrChangedOnDisk
	}
	if err := renameRetry(tmp, s.Path, s.diskHash); err != nil {
		os.Remove(tmp)
		return err
	}
	// The file's own time is the gate's second signal (#612): the editor says which file it installed, right after the rename, so its time is not taken for the game's. A write
	// by the game after this changes size or time and the record stops matching; a write that is not confirmed below is forgotten.
	noteOwnWrite(s.Path)
	back, err := readBackRetry(s.Path)
	if err != nil {
		forgetOwnWrite(s.Path)
		// The rename went through: the file on disk is, as far as anyone can tell, the edited save. Saying the edits "stay queued" would have them written a second time onto it.
		return writtenUnverified(err)
	}
	if bytes.Equal(back, blob) {
		return nil
	}
	forgetOwnWrite(s.Path)
	// The file is not what was written. The usual reason is that the game saved right after the rename, so the file now on disk may be the game's newest and must not be
	// replaced by the copy taken before the edits: look at what is there now and only say what is true. Nothing is written from here.
	now, rerr := readBackRetry(s.Path)
	switch {
	case rerr != nil:
		return apierr.Wrap(apierr.ErrWriteFailed, "the written save could not be read back to check it ("+shortErr(rerr)+"); the file was left as it is, and the copy taken before the write is in the backups folder", rerr)
	case bytes.Equal(now, blob): // a read that failed to see it once
		return nil
	case hashOf(now) == hashOf(cur):
		return apierr.New(apierr.ErrWriteFailed, "the replacement did not take effect: the file on disk is still the one that was loaded, and nothing was changed; the edits stay queued")
	}
	return apierr.New(apierr.ErrWriteFailed, "the file on disk is neither the edited save nor the one that was loaded (the game may have saved right after the write); it was left as it is, nothing was put back over it, and the copy taken before the write is in the backups folder")
}

// readBackTries and readBackBackoff are how long the file just written is tried for before it counts as unreadable: a scanner or an indexer opens a fresh file exclusively for a
// moment (5 tries with pauses of 0.2, 0.4, 0.6 and 0.8 s, about 2 s). A test sets the pause to 0.
var (
	readBackTries   = 5
	readBackBackoff = 200 * time.Millisecond
)

// readBackRetry reads the file, again after a growing pause while it cannot be read.
func readBackRetry(path string) ([]byte, error) {
	var b []byte
	var err error
	for i := 0; i < readBackTries; i++ {
		if i > 0 {
			time.Sleep(readBackBackoff * time.Duration(i))
		}
		if b, err = savefile.ReadCapped(path); err == nil {
			return b, nil
		}
	}
	return nil, err
}

// errWrittenUnverified is the kind of the error of a save that was written (the rename went through) and could not be read back to check it. The edits are very probably in the
// file; the next attempt looks again, and does not write them a second time.
var errWrittenUnverified = errors.New("written, not verified")

type unverifiedError struct{ *apierr.E }

func (e unverifiedError) Is(target error) bool {
	return target == errWrittenUnverified || e.E.Is(target)
}

func writtenUnverified(cause error) error {
	return unverifiedError{&apierr.E{
		Msg:   "the save was written, but it could not be verified (the file could not be read back: " + shortErr(cause) + "); it is looked at again on the next attempt, and the edits are not written a second time. The copy taken before the write is in the backups folder",
		Kind:  apierr.ErrWriteFailed,
		Cause: cause,
	}}
}

// ErrChangedOnDisk is returned when the file on disk is no longer the one this session loaded.
var ErrChangedOnDisk = errors.New("the save changed on disk since it was loaded (game autosave?); discard and redo the edits")

// openInspect opens a decoded copy of a save (the current file, or a backup chosen for Restore) read-only with the schema not trusted, like every connection that opens a save
// (a trigger or a view in a file from outside must not call functions with side effects). One connection, so the pragmas hold.
func openInspect(name string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(name)+"?mode=ro&_pragma=trusted_schema(0)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// inspect checks a decoded save is a sound SQLite database and returns its applied_patches names.
func inspect(raw []byte) (map[string]bool, error) {
	f, err := tempCopy("tabr-tau-inspect-*.sqlite")
	if err != nil {
		return nil, err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()
	db, err := openInspect(name)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	// The file may have come from outside (a backup chosen for Restore): a database built to make the check run for ever must not hold the editor, so both steps have a deadline.
	ctx, cancel := context.WithTimeout(context.Background(), inspectTimeout)
	defer cancel()
	var chk string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&chk); err != nil || chk != "ok" {
		if err != nil && ctx.Err() != nil {
			return nil, stopped(ctx, inspectTimeout, err)
		}
		return nil, fmt.Errorf("integrity check failed: %v %s", err, chk)
	}
	out := map[string]bool{}
	rs, err := db.QueryContext(ctx, `select name from applied_patches`)
	if err != nil {
		if ctx.Err() != nil {
			return nil, stopped(ctx, inspectTimeout, err)
		}
		return out, nil // an older save without the table has no patches
	}
	defer rs.Close()
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, stopped(ctx, inspectTimeout, rs.Err())
}

// AutoPassDesc is the name of the edit the automatic pass makes (the description of its entry in the queue). The gate reads it: a queue made only of such edits is held to the
// rule of an unattended write (#612).
const AutoPassDesc = "automatic pass"

// PassByHandDesc is the name of the edit of a pass a person asked for. It does not begin with AutoPassDesc: a person is looking, so it has the gate of an edit made by hand.
const PassByHandDesc = "run by hand: automatic pass"

// onlyPassEditsLocked says whether every queued edit (the held queue, when one is held) is the automatic pass's. The caller holds s.mu. An empty queue is not.
func (s *Save) onlyPassEditsLocked() bool {
	var descs []string
	if s.held != nil {
		for _, o := range s.held.ops {
			descs = append(descs, o.Desc)
		}
	} else {
		for _, o := range s.ops {
			descs = append(descs, o.Desc)
		}
	}
	if len(descs) == 0 {
		return false
	}
	for _, d := range descs {
		if !strings.HasPrefix(d, AutoPassDesc) {
			return false
		}
	}
	return true
}

// gate is the one question every route to the save file asks right before it writes: may the game's file be replaced now? It is GameStateFor for an edit made by hand, and
// UnattendedStateFor (which also waits for every game process) when the queue holds nothing but the automatic pass's own edit: nobody watches that write, wherever it is made
// from (the pass itself, the next tick of the autosave, a queue restored from the journal after a restart) (#612, #499). The caller holds s.mu (installBlob and commit do); Flush,
// which does not, takes the answer of onlyPassEditsLocked under the lock and calls gateFor. A mistaken name only gets the stricter rule, the safe direction; a hand edit queued
// with the pass's edit makes the queue a hand queue, because a person is then at the page.
func (s *Save) gate() (bool, string) { return s.gateFor(s.onlyPassEditsLocked()) }

// GateState is the answer the page shows (the status it polls, the overview, Check again): the rule the queue will actually be written under. A queue made only of the automatic
// pass's edit waits for every game process (UnattendedStateFor); any other queue, or none, has the rule of an edit made by hand (GameStateFor). Asked this way the page and the
// write agree: with the game open at its menu a pass-only queue is shown as blocked, with the reason the write will give, instead of "edits are written at once" (#819).
// fresh bypasses the process cache, as in GameStateFor.
func (s *Save) GateState(fresh bool) GameState {
	s.mu.Lock()
	passOnly := s.onlyPassEditsLocked()
	s.mu.Unlock()
	if passOnly {
		return UnattendedStateFor(s.Path, fresh)
	}
	return GameStateFor(s.Path, fresh)
}

func (s *Save) gateFor(passOnly bool) (bool, string) {
	g := GameStateFor(s.Path, true)
	if passOnly {
		g = UnattendedStateFor(s.Path, true)
	}
	return g.Blocked, g.Reason
}

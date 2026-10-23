package save

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// Hooks so tests can inject faults (a locked file, a failing reload). Production code never changes them.
var (
	fsRename          = os.Rename
	renameBackoff     = 150 * time.Millisecond
	reloadAfterCommit = func(s *Save) error { return s.load() }
	syncFile          = func(f *os.File) error { return f.Sync() }
)

const renameAttempts = 6

// writeDurable writes data to a new file (exclusive create, so an existing file is never overwritten), flushes it
// to stable storage and closes it.
func writeDurable(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := syncFile(f); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

// renameRetry retries with a growing pause: on Windows an antivirus scanner, OneDrive or the game itself can hold
// the destination for a moment.
func renameRetry(from, to string) error {
	var err error
	for i := 1; i <= renameAttempts; i++ {
		if err = fsRename(from, to); err == nil {
			return nil
		}
		time.Sleep(renameBackoff * time.Duration(i))
	}
	return apierr.Wrap(apierr.ErrWriteFailed, fmt.Sprintf("could not replace %s (is another program holding it?): %v", filepath.Base(to), err), err)
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

// installBlob atomically replaces the save file with blob: temp file, fsync, re-check that the file on disk is
// still the one this session loaded, rename with retry, then read back and compare.
func (s *Save) installBlob(blob []byte) error {
	tmp := s.tmpPath()
	if err := writeDurable(tmp, blob, 0o600); err != nil {
		return apierr.Wrap(apierr.ErrWriteFailed, "could not write the new file next to the save ("+shortErr(err)+"); nothing was changed", err)
	}
	// The game-running check of the caller was made well before this point (the encode, the round trip, the backup and the temp file came after it, and on a large save that
	// is seconds, in which a single-player session can start and the game rewrite the file). It is made again right before the rename, which is the step that cannot be undone
	// by the game's own write: a session that began in between leaves the edits queued instead. It comes before the look at the file on disk, not after, because the check
	// itself takes time in which the game can save: what the file is must be read last.
	if blocked, why := SaveBlocked(s.Path); blocked {
		os.Remove(tmp)
		return apierr.New(apierr.ErrGameBlocked, why)
	}
	cur, err := readCapped(s.Path)
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if hashOf(cur) != s.diskHash {
		os.Remove(tmp)
		return ErrChangedOnDisk
	}
	if err := renameRetry(tmp, s.Path); err != nil {
		os.Remove(tmp)
		return err
	}
	back, err := readBackRetry(s.Path)
	if err != nil {
		// The rename went through: the file on disk is, as far as anyone can tell, the edited save. Saying the edits "stay queued" would have them written a second time onto it.
		return writtenUnverified(err)
	}
	if bytes.Equal(back, blob) {
		return nil
	}
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
		if b, err = readCapped(path); err == nil {
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
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(name)+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var chk string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&chk); err != nil || chk != "ok" {
		return nil, fmt.Errorf("integrity check failed: %v %s", err, chk)
	}
	out := map[string]bool{}
	rs, err := db.Query(`select name from applied_patches`)
	if err != nil {
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
	return out, rs.Err()
}

package save

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Hooks so tests can inject faults (a locked file, a failing reload). Production code never changes them.
var (
	fsRename          = os.Rename
	renameBackoff     = 150 * time.Millisecond
	reloadAfterCommit = func(s *Save) error { return s.load() }
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
	if err := f.Sync(); err != nil {
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
	return fmt.Errorf("could not replace %s (is another program holding it?): %w", filepath.Base(to), err)
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
		return fmt.Errorf("%s is not a regular file (symlinks are refused)", s.Path)
	}
	return nil
}

// installBlob atomically replaces the save file with blob: temp file, fsync, re-check that the file on disk is
// still the one this session loaded, rename with retry, then read back and compare.
func (s *Save) installBlob(blob []byte) error {
	tmp := s.tmpPath()
	if err := writeDurable(tmp, blob, 0o600); err != nil {
		return err
	}
	cur, err := os.ReadFile(s.Path)
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
	back, err := os.ReadFile(s.Path)
	if err != nil || !bytes.Equal(back, blob) {
		// The new file is not what we wrote: put the previous content back the same way and say so.
		rb := s.tmpPath()
		if werr := writeDurable(rb, cur, 0o600); werr == nil {
			if rerr := renameRetry(rb, s.Path); rerr != nil {
				os.Remove(rb)
			}
		}
		return errors.New("the written save did not read back identically; the previous file was restored")
	}
	return nil
}

// ErrChangedOnDisk is returned when the file on disk is no longer the one this session loaded.
var ErrChangedOnDisk = errors.New("the save changed on disk since it was loaded (game autosave?); discard and redo the edits")

// inspect checks a decoded save is a sound SQLite database and returns its applied_patches names.
func inspect(raw []byte) (map[string]bool, error) {
	f, err := os.CreateTemp("", "tabr-tau-inspect-*.sqlite")
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

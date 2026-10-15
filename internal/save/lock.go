package save

import (
	"errors"
	"path/filepath"
)

// ErrSaveInUse means another tabr-tau already holds the editing lock of this save.
var ErrSaveInUse = errors.New("another tabr-tau already has this save open: close that one first (look for its window or terminal). Two editors on one save would overwrite each other's work")

// LockSave takes the editing lock of the save at path, so a second editor cannot open the same file (NET-1 of the 2026-10-15 audit: a second
// instance used to start on another port and work on the same save). The lock is held by the operating system on a small file next to the
// save, so it is released when the program ends, however it ends; a leftover file from a crash locks nothing. release gives it up.
//
// Only a lock that is really held by another editor is an error (ErrSaveInUse). If the lock file cannot be made at all (a read-only folder, say)
// the editor still starts: it could not write its backups there either, and the changed-on-disk check still guards every save.
func LockSave(path string) (release func(), err error) {
	return lockFile(filepath.Join(filepath.Dir(path), filepath.Base(path)+".tabr-tau.lock"))
}

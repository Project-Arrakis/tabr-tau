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
// Once the lock is held, the temp files that a crashed run left next to the save are removed. Only a lock that is really held by another editor is an error (ErrSaveInUse). If the lock file cannot be made at all (a read-only folder, say)
// the editor still starts: it could not write its backups there either, and the changed-on-disk check still guards every save. It then holds no lock, so it removes nothing: the
// sweeps above are for the editor that holds the lock, because another editor's write or session may be in flight.
func LockSave(path string) (func(), error) {
	release, _, err := lockAndSweep(path)
	return release, err
}

// ErrNoLock means the lock file could not be made, so no lock is held. LockSaveStrict and LockPlain return it; LockSave never does.
var ErrNoLock = errors.New("the lock file (the file's name plus .tabr-tau.lock) could not be made")

// LockSaveStrict is LockSave for a run that must not go on without the lock (layout-roll): where LockSave goes on holding no lock, this refuses with ErrNoLock.
func LockSaveStrict(path string) (func(), error) {
	release, held, err := lockAndSweep(path)
	if err == nil && !held {
		return nil, ErrNoLock
	}
	return release, err
}

// LockPlain is the same lock for a small file that is no save (the log of the layout roll): it sweeps nothing, and refuses with ErrNoLock as LockSaveStrict does.
func LockPlain(path string) (func(), error) {
	release, held, err := lockOnly(path)
	if err == nil && !held {
		return nil, ErrNoLock
	}
	return release, err
}

// lockOnly takes the lock on the small file next to path. held false with no error means the lock file could not be made.
func lockOnly(path string) (func(), bool, error) {
	return lockFile(filepath.Join(filepath.Dir(path), filepath.Base(path)+".tabr-tau.lock"))
}

// lockAndSweep takes the lock and, only when it is held, sweeps.
func lockAndSweep(path string) (func(), bool, error) {
	release, held, err := lockOnly(path)
	if err == nil && held {
		cleanStaleTemps(path) // only the editor that holds the lock may remove a temp file next to the save: another editor's write may be in flight
		SweepWorkCopies()     // and the decoded copies of sessions that have ended (a crash or a kill leaves them), found by their process id
	}
	return release, held, err
}

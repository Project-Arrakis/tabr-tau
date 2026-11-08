// Package durable replaces a file so that a crash or a power cut leaves the old file or the new one, never a part of either.
//
// The steps are the ones a careful program takes by hand: the data goes to a new file in the same folder, is flushed to stable storage
// (fsync) and closed; the new file is then renamed over the target, with a few tries because on Windows a virus scanner, an indexer or
// another program can hold the target for a moment; and the folder is flushed afterwards where the system allows it, so that the rename
// itself survives a power cut.
//
// WriteFile does all of it. WriteNew and Rename are its two halves, for a caller that has to check something between the flush and the
// rename (is the file still the one that was read? is the game running?). The package never reads a file back and never keeps a backup:
// what to compare and what to keep are the caller's decisions.
//
// Every step that touches the disk can be replaced through Options.Hooks, so that a test can make the rename fail, the flush fail, or
// the process "die" right before the rename. Production code leaves the hooks nil.
package durable

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// DefaultAttempts and DefaultPause are the values of Defaults: six tries of the rename with pauses of 150, 300, 450 ... milliseconds,
// about three seconds in all.
const (
	DefaultAttempts = 6
	DefaultPause    = 150 * time.Millisecond
)

// Hooks replace single steps. A nil field means the real step. They exist for tests (of this package and of its callers); a program
// that does not inject faults never sets them.
type Hooks struct {
	// Rename replaces os.Rename.
	Rename func(from, to string) error
	// SyncFile replaces (*os.File).Sync, the flush of the new file to stable storage.
	SyncFile func(f *os.File) error
	// SyncDir replaces SyncDir, the flush of the folder after the rename.
	SyncDir func(dir string) error
	// BeforeRename is called by WriteFile after the new file is flushed and closed and before the first try of the rename. A test makes
	// it panic to stand for a crash at that moment: the old file is whole and the new one is left beside it.
	BeforeRename func(dst string)
}

// Options says how a write is done. The zero value is the plainest write: one try of the rename, no pause, no folder flush.
// Defaults returns the values most callers want.
type Options struct {
	// Attempts is how many times the rename is tried. A value below 1 means 1.
	Attempts int
	// Pause is the wait after a failed try of the rename. The wait follows every failed try, the last one too.
	Pause time.Duration
	// Growing makes the wait after the nth failed try n times Pause (150, 300, 450 ms ...) instead of Pause every time.
	Growing bool
	// Retry, when not nil, is asked about the error of a failed rename: false returns the error at once, without waiting for another
	// try. A nil Retry tries again after every error.
	Retry func(err error) bool
	// SyncDir makes WriteFile flush the folder after the rename. It does nothing on Windows (see SyncDir). A folder that cannot be
	// flushed is not a failed write: the file has been replaced by then.
	SyncDir bool
	// TempPattern is the pattern of the new file's name, as for os.CreateTemp (the last "*" becomes a random number). The default is
	// the target's name, a dot, "*" and ".tmp". A caller that removes the leftovers of a crash chooses a pattern it can recognise.
	TempPattern string
	// Hooks replaces single steps in tests.
	Hooks Hooks
}

// Defaults returns the options most callers want: DefaultAttempts tries of the rename with a growing pause of DefaultPause, and a
// flush of the folder.
func Defaults() Options {
	return Options{Attempts: DefaultAttempts, Pause: DefaultPause, Growing: true, SyncDir: true}
}

func (o Options) syncFile(f *os.File) error {
	if o.Hooks.SyncFile != nil {
		return o.Hooks.SyncFile(f)
	}
	return f.Sync()
}

func (o Options) rename(from, to string) error {
	if o.Hooks.Rename != nil {
		return o.Hooks.Rename(from, to)
	}
	return os.Rename(from, to)
}

func (o Options) syncDir(dir string) error {
	if o.Hooks.SyncDir != nil {
		return o.Hooks.SyncDir(dir)
	}
	return SyncDir(dir)
}

// WriteFile puts data in path. The data goes to a new file next to path (created with a name of its own, so that neither a leftover of
// a crash nor another program's file is ever written over), is flushed and closed, gets the mode perm, and is renamed over path; then
// the folder is flushed if o.SyncDir says so. If anything fails before the rename succeeds, the new file is removed and path is
// untouched. The error of a rename that never succeeded is the last error of the system. The folder of path must exist.
func WriteFile(path string, data []byte, perm os.FileMode, o Options) error {
	pattern := o.TempPattern
	if pattern == "" {
		pattern = filepath.Base(path) + ".*.tmp"
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := writeFlushClose(f, data, o); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		os.Remove(tmp)
		return err
	}
	if o.Hooks.BeforeRename != nil {
		o.Hooks.BeforeRename(path)
	}
	if err := Rename(tmp, path, o); err != nil {
		os.Remove(tmp)
		return err
	}
	if o.SyncDir {
		_ = o.syncDir(dir) // best effort: the rename is done, a folder that will not flush does not undo it
	}
	return nil
}

// WriteNew writes data to a new file at path with the mode perm and flushes it to stable storage. The file is created exclusively: if
// path exists the write fails (errors.Is(err, os.ErrExist)) and the existing file is not touched. If the write, the flush or the close
// fails, the new file is removed. WriteNew does not rename and does not flush the folder.
func WriteNew(path string, data []byte, perm os.FileMode, o Options) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if err := writeFlushClose(f, data, o); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

// writeFlushClose writes, flushes and closes f. On an error f is closed and the caller removes the file.
func writeFlushClose(f *os.File, data []byte, o Options) error {
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := o.syncFile(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Rename renames from over to, trying o.Attempts times with the pause of o. It returns nil at the first success and otherwise the error
// of the last try, as the system gave it (a *os.LinkError for the real rename). It does not remove from: a caller that gives up
// removes it.
func Rename(from, to string, o Options) error {
	attempts := o.Attempts
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for i := 1; i <= attempts; i++ {
		if err = o.rename(from, to); err == nil {
			return nil
		}
		if o.Retry != nil && !o.Retry(err) {
			return err
		}
		pause := o.Pause
		if o.Growing {
			pause *= time.Duration(i)
		}
		time.Sleep(pause)
	}
	return err
}

// SyncDir flushes the folder entry of a rename, so that the rename survives a power cut. It does so where the system can: on Linux and
// macOS. On Windows it does nothing and returns nil: Go cannot open a folder for a flush there, and NTFS journals the rename itself.
// Elsewhere a failure to open or flush the folder is returned, and WriteFile ignores it.
func SyncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	serr := d.Sync()
	if cerr := d.Close(); serr == nil {
		serr = cerr
	}
	if serr != nil {
		return fmt.Errorf("flushing the folder %s: %w", filepath.Base(dir), serr)
	}
	return nil
}

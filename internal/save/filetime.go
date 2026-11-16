package save

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// The save file's own modification time is the second signal of the live-save gate (#612, D2-1): the log's text can be reworded by a game update, the time a file was last
// written cannot. The game rewrites the file while a world is loaded, so a file written moments ago was probably written by it. The editor's own writes also move the time, so
// the editor remembers what it wrote (size and time, taken right after its rename) and a file that still is that one is not counted as the game's.

// fileStamp is the size and the modification time of a file.
type fileStamp struct {
	size int64
	mod  time.Time
}

// ownWrites is the stamp of the file the editor installed last, by the save's path. A later write by anything else changes the stamp, so the entry stops matching.
var ownWrites = struct {
	sync.Mutex
	m map[string]fileStamp
}{m: map[string]fileStamp{}}

func ownKey(path string) string { return filepath.Clean(path) }

// noteOwnWrite records the file at path as the editor's own, as it is right now. Called right after the rename that installs the editor's version. A file that cannot be
// looked at is not recorded, which only means its time counts.
func noteOwnWrite(path string) {
	st, err := os.Stat(path)
	if err != nil {
		forgetOwnWrite(path)
		return
	}
	ownWrites.Lock()
	ownWrites.m[ownKey(path)] = fileStamp{st.Size(), st.ModTime()}
	ownWrites.Unlock()
}

// forgetOwnWrite drops the record: the file was not the one the editor wrote (the game wrote after the rename), or the write could not be checked.
func forgetOwnWrite(path string) {
	ownWrites.Lock()
	delete(ownWrites.m, ownKey(path))
	ownWrites.Unlock()
}

// sidecarSuffixes are files the game's database engine may keep next to the save and write instead of the save itself (a write-ahead log, a rollback journal): the newest time of
// the file and of these is the time of the game's last write. Only ones that exist count.
var sidecarSuffixes = []string{"-wal", "-journal"}

// fileTimeFor says when the save was last written by anything but the editor: the newest modification time of the file and its sidecars, and own = true when the file is the
// one the editor itself installed (and no sidecar is newer). An error (the file cannot be looked at) is returned and the caller treats it as unknown. A variable so tests can
// stand in for the machine.
var fileTimeFor = statFileTime

// statFileTime is fileTimeFor as shipped.
func statFileTime(savePath string) (mod time.Time, own bool, err error) {
	st, err := os.Stat(savePath)
	if err != nil {
		return time.Time{}, false, err
	}
	mod = st.ModTime()
	ownWrites.Lock()
	rec, ok := ownWrites.m[ownKey(savePath)]
	ownWrites.Unlock()
	own = ok && rec.size == st.Size() && rec.mod.Equal(st.ModTime())
	for _, suf := range sidecarSuffixes {
		if side, serr := os.Stat(savePath + suf); serr == nil && side.ModTime().After(mod) {
			mod, own = side.ModTime(), false // a sidecar newer than the file is not the editor's
		}
	}
	return mod, own, nil
}

// The quiet of the save file must be observed, not only computed from the clock (#612): after a sleep or a hibernation, or when the clock is set forward, the file's time is far
// in the past and looks like a long quiet although the game may have been writing until the moment the computer slept. So the editor remembers, per save, when it last
// evaluated the gate with a game process present and what the file looked like, and holds for observeQuietFor from the first evaluation, from the first one after a gap of more
// than observeGap (a clock that went backward is a gap), and from a change of the file's time that is not the editor's own write.
const (
	observeGap = 30 * time.Second
)

// observeQuietFor is how long the editor must have watched the file, evaluating the gate at least every observeGap and seeing the file unchanged, before a game process's quiet
// is believed: one menuQuietAfterWrite. A variable so the tests of other rules (which call the gate once) can switch it off; the tests of this one use the real value.
var observeQuietFor = menuQuietAfterWrite

type observation struct {
	lastAt time.Time // the last evaluation with a game process present
	from   time.Time // the start of the unbroken watch
	mod    time.Time // the file's time at the last evaluation
}

var observed = struct {
	sync.Mutex
	m map[string]observation
}{m: map[string]observation{}}

// observeFile records one evaluation of the gate with a game process present and returns when the unbroken watch of the file began. A gap since the last evaluation, a first
// evaluation, a file whose time cannot be read, or a time that changed other than by the editor's own write start the watch again at now.
func observeFile(savePath string, now, mod time.Time, own bool, terr error) time.Time {
	observed.Lock()
	defer observed.Unlock()
	k := ownKey(savePath)
	o, ok := observed.m[k]
	switch {
	case !ok, now.Before(o.lastAt), now.Sub(o.lastAt) > observeGap, terr != nil:
		o = observation{from: now}
	case !o.mod.Equal(mod) && !own:
		o.from = now
	}
	o.lastAt = now
	if terr == nil {
		o.mod = mod
	}
	observed.m[k] = o
	return o.from
}

// forgetObserved drops the watch: no game process is present, so the next evaluation with one starts a new watch.
func forgetObserved(savePath string) {
	observed.Lock()
	delete(observed.m, ownKey(savePath))
	observed.Unlock()
}

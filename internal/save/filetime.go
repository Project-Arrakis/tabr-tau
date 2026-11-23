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
// than observeGap (a clock that went backward by a second or more is a gap; a reading less than a second early counts as the same moment), and from a change of the file's time that is not the editor's own write.
const (
	observeGap = 30 * time.Second
	// clockSkew is how far before the last evaluation a clock reading may be and still be the same moment: two threads read the clock, then take the lock in the other order.
	clockSkew = time.Second
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
	// Two evaluations made at nearly the same moment (the page's status poll and the autosave's tick) read the clock before they take this lock, so the later reading can get the
	// lock first and the earlier one arrive after it. That is not a clock that went backward: a reading less than clockSkew before the last one counts as the last one (#779).
	if ok && now.Before(o.lastAt) && o.lastAt.Sub(now) < clockSkew {
		now = o.lastAt
	}
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

// observeEvery is how often KeepObserving looks at the save by itself while a game process exists: a third of observeGap, so that a tick that comes late still leaves the watch unbroken.
// A variable so the tests can run it faster than the clock they stand in for.
var observeEvery = observeGap / 3

// KeepObserving is one evaluation of the gate, made by the editor itself, so that the watch of the save file (observeFile) does not depend on a page being open and visible (#1139). The
// status poll is the one thing that evaluates the gate when no edit is waiting, and the page stops polling while its window is hidden or minimised; a Tau window behind the game for more
// than observeGap broke the watch, and the first poll after the person came back started a new observeQuietFor wait however old the file was. The editor's autosave loop calls this every
// few seconds; it looks at most once every observeEvery.
//
// It changes no rule and writes nothing. It is the observation the gate already makes (a stat of the file's time and the list of game processes, the same answers the status poll uses,
// the process list from its short cache), made at the moment it is, and the answer is not used: whether a write may go ahead is decided by the gates that run before the backup and before
// the rename, which are asked as before and read the process list afresh. What it records is true: the file's time as it was at that moment. A gap longer than observeGap (a sleep, a hibernation,
// a clock stepped forward or back, a loop that did not run) still starts the watch again, a change of the file's time that is not the editor's own write still starts it again, a
// session the log shows as active still blocks, and with no game process the watch is dropped, as ever. It takes none of the save's locks, so it cannot hold up a write, and a write
// cannot hold it up.
func (s *Save) KeepObserving() {
	now := nowFunc()
	s.obsMu.Lock()
	due := s.obsAt.IsZero() || now.Sub(s.obsAt) >= observeEvery || now.Before(s.obsAt)
	if due {
		s.obsAt = now
	}
	s.obsMu.Unlock()
	if !due || !IsGamesOwnSave(s.Path) {
		return
	}
	processState(s.Path, false)
}

// watchEvery is how often keepWatching evaluates the gate while a write is being prepared: a third of observeGap, so that one late tick still leaves the watch unbroken. A variable
// so the tests can run it faster than the clock they stand in for.
var watchEvery = observeGap / 3

// watchTicks gives keepWatching the ticks that tell it to evaluate the gate, and the function that stops them. As shipped it is a time.Ticker of the given period; a variable so a
// test can send the ticks itself, one at a time, and know that an evaluation has finished before it moves its clock (#875).
var watchTicks = func(every time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(every)
	return t.C, t.Stop
}

// keepWatching evaluates the gate every watchEvery until the returned function is called, which stops it and waits for it to finish. A write holds the save's lock across the hash
// of the file, the integrity check, the encode and its round trip, the backup and the temp file, and on a large save any one of these can by itself take longer than observeGap;
// a gate asked once after them would be the first evaluation after a gap, start the watch again and refuse every time, so the edits would never be written while a game process
// exists (#821). The answers are not used: each evaluation is only the observation that keeps the watch unbroken, and the gates that decide (before the backup, before the
// rename) are asked as before. passOnly is the rule the queue is written under, taken by the caller under the save's lock.
func (s *Save) keepWatching(passOnly bool) (stop func()) {
	quit, done := make(chan struct{}), make(chan struct{})
	every := watchEvery
	go func() {
		defer close(done)
		ticks, stopTicks := watchTicks(every)
		defer stopTicks()
		for {
			select {
			case <-quit:
				return
			case <-ticks:
				s.gateFor(passOnly)
			}
		}
	}()
	return func() {
		close(quit)
		<-done
	}
}

// prepared is called by a write once the hash of the file, the integrity check and the encode with its round trip are done. Nothing in it as shipped; a variable so a test can make
// those steps take as long as they would on a large save.
var prepared = func() {}

package save

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// quickBackupEvery is how recent the backup of the last automatic write must be for the next to go without a new one: a burst of edits is one restore point, taken before the
// first (and only while the file is still the one that write left: a file the game saved since gets a backup of its own, and the backup is reused only while its bytes are
// still the ones recorded when it was taken). keepBackups is how many of the newest backups that automatic writes take the folder keeps; besides those it always keeps the
// oldest one, the first one this session took (the state before the editor's first write) and the one just taken. The others (every one an earlier version or a person made,
// and any file the editor did not name itself) are never counted and never removed.
const (
	quickBackupEvery = 2 * time.Minute
	keepBackups      = 60
	autoTag          = "-auto"
)

// Flush is what became of the pending edits at the last attempt to write them: written, or waiting (Queued of them) for the reason in Why.
type Flush struct {
	At      time.Time `json:"at"`
	Saved   bool      `json:"saved"`             // the edits were written to the game's file
	Queued  int       `json:"queued"`            // how many edits are still waiting
	Why     string    `json:"why,omitempty"`     // why they are waiting
	Backup  string    `json:"backup,omitempty"`  // the backup the write was made after
	Ops     []string  `json:"ops,omitempty"`     // what was written
	Warning string    `json:"warning,omitempty"` // written, but the file could not be read back in (the editor needs a restart)
	Stuck   bool      `json:"stuck,omitempty"`   // the edits cannot be written and will not be: they were not kept, the game saved, so they can only be discarded and made again
	// Restored says that the edits were put back onto a newer file first (the game saved since they were made), and which could not be
	Restored *Restored `json:"restored,omitempty"`
}

// LastFlush is the result of the last attempt that had edits to write, or nil when there has been none.
func (s *Save) LastFlush() *Flush {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastFlush == nil {
		return nil
	}
	f := *s.lastFlush
	return &f
}

func (s *Save) setLastFlush(f Flush) Flush {
	s.mu.Lock()
	s.lastFlush = &f
	s.mu.Unlock()
	return f
}

// Flush writes the pending edits to the game's file when the file can be written: the save is not read-only, no single-player session is using it, and, when the game saved
// since the edits were made, they are put back onto the newer file first. Otherwise they stay pending (they are kept across restarts, see the journal) and the next call
// tries again. It is what the editor calls after every edit and every few seconds while edits are waiting, so there is no step where a person has to save. A call with
// nothing pending changes nothing and returns the zero Flush.
func (s *Save) Flush() Flush {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	held := s.held
	dirty, n, blocked := s.dirty, len(s.ops), s.blocked
	if held != nil {
		dirty, n = true, len(held.ops)
	}
	s.mu.Unlock()
	if !dirty {
		return Flush{}
	}
	f := Flush{At: time.Now(), Queued: n}
	if blocked != "" {
		f.Why = errReadOnly(blocked).Error()
		return s.setLastFlush(f)
	}
	if g := GameStateFor(s.Path, true); g.Blocked {
		f.Why = g.Reason
		return s.setLastFlush(f)
	}
	if held != nil && !s.ChangedOnDisk() { // nothing new from the game: trying again gives the same answer
		f.Why = heldWhy(held)
		return s.setLastFlush(f)
	}
	if held != nil || s.ChangedOnDisk() { // the game saved since the edits were made: all of them go back onto its file, or none are written
		r, err := s.rebaseAtomic()
		if err != nil {
			f.Why = err.Error()
			f.Stuck = errors.Is(err, ErrNotKept)
			return s.setLastFlush(f)
		}
		f.Restored = r
		if h := s.Held(); h != nil {
			f.Why = heldWhy(h)
			return s.setLastFlush(f)
		}
		s.mu.Lock()
		dirty, f.Queued = s.dirty, len(s.ops)
		s.mu.Unlock()
		if !dirty {
			return s.setLastFlush(f)
		}
	}
	res, err := s.commit(true)
	if err != nil {
		f.Why = err.Error()
		if errors.Is(err, ErrChangedOnDisk) {
			f.Why = "the game saved while the edits were being written; they are put back onto its file next"
		}
		return s.setLastFlush(f)
	}
	f.Saved, _ = res["saved"].(bool)
	f.Queued = 0
	f.Backup, _ = res["backup"].(string)
	f.Ops, _ = res["ops"].([]string)
	f.Warning, _ = res["warning"].(string)
	if f.Saved {
		s.appendWriteLog(f) // the write has happened; recording it cannot undo it, and a failure is reported through WriteLogProblem
	}
	return s.setLastFlush(f)
}

// heldWhy is the sentence for a queue that was held.
func heldWhy(h *Held) string {
	why := "the game saved changes that conflict with the queued ones, so none of them was written"
	if len(h.Conflicts) > 0 {
		why += " (" + h.Conflicts[0].Desc + ": " + h.Conflicts[0].Why + ")"
	}
	return why
}

// backupIfStale makes a backup unless the last automatic write was made after one from the last quickBackupEvery and the file is still the one it wrote, in which case that
// backup holds the state before the burst and is returned again (after its bytes have been read and found to be the ones recorded when it was taken). A new backup must be of
// the file the edits were made on: when the game saved between the check and the copy, nothing is written (ErrChangedOnDisk) and the copy stays as a backup of the game's file.
func (s *Save) backupIfStale() (string, error) {
	if s.canReuseBackup() {
		return s.lastBackup, nil
	}
	b, sum, err := s.backupNamed(autoTag)
	if err != nil {
		return "", err
	}
	if sum != s.diskHash {
		return "", ErrChangedOnDisk
	}
	s.lastBackupT, s.lastBackupHash = time.Now(), sum
	if s.firstBackup == "" {
		s.firstBackup = b
	}
	s.pruneBackups(b)
	return b, nil
}

// canReuseBackup says whether the backup of the last automatic write still stands for this one: it is recent, the file is still the one that write left, and the backup is
// on disk with the bytes it was taken with.
func (s *Save) canReuseBackup() bool {
	if s.lastBackup == "" || s.diskHash != s.lastWrote || time.Since(s.lastBackupT) >= quickBackupEvery {
		return false
	}
	b, err := readCapped(s.lastBackup)
	return err == nil && hashOf(b) == s.lastBackupHash
}

// Backup names are <name>-<yyyymmdd-hhmmss>Z-<nanoseconds, 9 digits>[-auto]<ext>: the time is UTC (the Z), so a change of the clock's zone or a daylight-saving change cannot make a
// newer backup look older, and the nanoseconds are padded so the names sort as the times do. Names made by earlier versions have the local time, no Z and unpadded nanoseconds; they
// are still understood (the local time is read in the zone the computer is in now).
var backupStamp = regexp.MustCompile(`^(\d{8}-\d{6})(Z?)-(\d{1,9})$`)

// newBackupName is the name of a backup taken now. The time in it is later than that of every backup already in the folder, whatever the clock says: a clock that was set back
// would otherwise give a new backup an old time, and the pruning would take it for one of the oldest.
func (s *Save) newBackupName(base, tag, ext string) string {
	now := time.Now().UTC()
	ents, _ := os.ReadDir(s.backupDir())
	for _, e := range ents {
		if at, ok := s.backupInstant(e.Name()); ok && at >= now.UnixNano() {
			now = time.Unix(0, at).UTC().Add(time.Millisecond)
		}
	}
	return fmt.Sprintf("%s-%sZ-%09d%s%s", base, now.Format("20060102-150405"), now.Nanosecond(), tag, ext)
}

// backupInstant is the time in the name of a backup of this save (its name, then the stamp, then the optional -auto and the extension), in nanoseconds since 1970.
func (s *Save) backupInstant(name string) (int64, bool) {
	at, _, ok := s.parseBackupName(name)
	return at, ok
}

func (s *Save) parseBackupName(name string) (at int64, auto bool, ok bool) {
	ext := filepath.Ext(s.Path)
	pre := strings.TrimSuffix(filepath.Base(s.Path), ext) + "-"
	if !strings.HasPrefix(name, pre) || !strings.HasSuffix(name, ext) || len(name) <= len(pre)+len(ext) {
		return 0, false, false
	}
	mid := name[len(pre) : len(name)-len(ext)]
	if strings.HasSuffix(mid, autoTag) {
		mid, auto = strings.TrimSuffix(mid, autoTag), true
	}
	m := backupStamp.FindStringSubmatch(mid)
	if m == nil {
		return 0, false, false
	}
	n, err := strconv.ParseInt(m[3], 10, 64)
	if err != nil {
		return 0, false, false
	}
	loc := time.Local
	if m[2] == "Z" {
		loc = time.UTC
	}
	t, err := time.ParseInLocation("20060102-150405", m[1], loc)
	if err != nil {
		return 0, false, false
	}
	return t.UnixNano() + n, auto, true
}

// pruneBackups removes the oldest of the editor's automatic backups beyond keepBackups. Ordered by the time in their names, read as an instant (not the file times, which anything can set, and not the text of the name, whose local time can run backwards), the
// ones it never removes are the oldest, the first of this session and justTaken, and nothing that is not an automatic backup of this save is looked at.
func (s *Save) pruneBackups(justTaken string) {
	ents, _ := os.ReadDir(s.backupDir())
	type bk struct {
		path string
		at   int64
	}
	var all []bk
	for _, e := range ents {
		if at, auto, ok := s.parseBackupName(e.Name()); ok && auto && e.Type().IsRegular() {
			all = append(all, bk{filepath.Join(s.backupDir(), e.Name()), at})
		}
	}
	if len(all) <= keepBackups {
		return
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at > all[j].at })
	keep := map[string]bool{justTaken: true, s.firstBackup: true, all[len(all)-1].path: true}
	for _, b := range all[keepBackups:] {
		if !keep[b.path] {
			os.Remove(b.path)
		}
	}
}

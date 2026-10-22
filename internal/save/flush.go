package save

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// quickBackupEvery is how recent the newest backup must be for an automatic write to go without a new one: a burst of edits is one restore point, taken before the first.
// keepBackups is how many the backup folder keeps after an automatic write.
const (
	quickBackupEvery = 2 * time.Minute
	keepBackups      = 60
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
	dirty, n, blocked := s.dirty, len(s.ops), s.blocked
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
	if s.ChangedOnDisk() { // the game saved since the edits were made: they go back onto its file
		r, err := s.Rebase()
		if err != nil {
			f.Why = err.Error()
			return s.setLastFlush(f)
		}
		f.Restored = r
		s.mu.Lock()
		dirty, f.Queued = s.dirty, len(s.ops)
		s.mu.Unlock()
		if !dirty { // none of them could be put back
			return s.setLastFlush(f)
		}
	}
	res, err := s.commit(false, true)
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
	return s.setLastFlush(f)
}

// backupIfStale makes a backup unless the newest one is from the last quickBackupEvery, in which case it returns that one. It keeps the newest keepBackups.
func (s *Save) backupIfStale() (string, error) {
	newest, mod := s.newestBackup()
	if newest != "" && time.Since(mod) < quickBackupEvery {
		return newest, nil
	}
	b, err := s.backup()
	if err == nil {
		s.pruneBackups()
	}
	return b, err
}

func (s *Save) newestBackup() (string, time.Time) {
	var name string
	var mod time.Time
	ents, _ := os.ReadDir(s.backupDir())
	for _, e := range ents {
		if info, err := e.Info(); err == nil && info.Mode().IsRegular() && info.ModTime().After(mod) {
			name, mod = filepath.Join(s.backupDir(), e.Name()), info.ModTime()
		}
	}
	return name, mod
}

// pruneBackups removes the oldest backups beyond keepBackups (the editor's own folder next to the save, nothing else).
func (s *Save) pruneBackups() {
	ents, _ := os.ReadDir(s.backupDir())
	type bk struct {
		path string
		mod  time.Time
	}
	var all []bk
	for _, e := range ents {
		if info, err := e.Info(); err == nil && info.Mode().IsRegular() {
			all = append(all, bk{filepath.Join(s.backupDir(), e.Name()), info.ModTime()})
		}
	}
	if len(all) <= keepBackups {
		return
	}
	sort.Slice(all, func(i, j int) bool { return all[i].mod.After(all[j].mod) })
	for _, b := range all[keepBackups:] {
		os.Remove(b.path)
	}
}

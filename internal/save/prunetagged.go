package save

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The pruning of the backups a person (or the weekly layout roll) asked for (#1227). The automatic ones have their own pruning (pruneBackups); the "-manual" and "-layoutroll"
// ones were never removed, so every weekly roll added a whole save copy for good. After one of them has been taken and verified, the older ones of the same kind are removed,
// under rules that only ever remove less:
//   - the newest keepTaggedBackups of the kind are kept, and the one just taken is always among them;
//   - nothing younger than keepTaggedAge is removed, by the time in its name and by its file time (both must be older);
//   - only a regular file in the backup folder whose whole name is this save's backup name for that one kind is looked at (the name is matched from the first character to the
//     last, so a name that merely contains it, or has another tag as well, is not ours); the folder must be a plain folder, and a link is never followed or removed;
//   - the save, the automatic backups, the game's own files and everything else in the folder are never looked at;
//   - a file that cannot be removed is logged and left, and the backup or the edit that called this is not failed by it.
const (
	keepTaggedBackups = 8
	keepTaggedAge     = 7 * 24 * time.Hour
)

// removeBackupFile is os.Remove; a variable so that a test can make one removal fail.
var removeBackupFile = os.Remove

// taggedBackupName is the exact name of a backup of this save of one kind: <name>-<stamp>-<nanoseconds><tag><ext>, the stamp as backupStamp reads it (the Z is optional because
// earlier versions wrote none).
func (s *Save) taggedBackupName(tag string) *regexp.Regexp {
	ext := filepath.Ext(s.Path)
	base := strings.TrimSuffix(filepath.Base(s.Path), ext)
	return regexp.MustCompile(`^` + regexp.QuoteMeta(base) + `-\d{8}-\d{6}Z?-\d{1,9}` + regexp.QuoteMeta(tag) + regexp.QuoteMeta(ext) + `$`)
}

// pruneTagged removes the old backups of one kind (manualTag or checkpointTag) beyond the newest keepTaggedBackups, by the rules above. justTaken is the path of the backup that
// was just written and is never removed. It returns nothing: whatever goes wrong is logged and leaves the files as they are.
func (s *Save) pruneTagged(tag, justTaken string) {
	if tag != manualTag && tag != checkpointTag {
		return // only these two kinds are ours to prune here; the automatic ones have pruneBackups
	}
	dir := s.backupDir()
	if RequirePlainFolder(dir) != nil { // never remove files through a link
		return
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("backups: could not list the backup folder to prune old %s backups (nothing removed): %s", strings.TrimPrefix(tag, "-"), shortErr(err))
		return
	}
	re := s.taggedBackupName(tag)
	type bk struct {
		path string
		name string
		at   int64
		mod  time.Time
	}
	var all []bk
	for _, e := range ents {
		if !e.Type().IsRegular() || !re.MatchString(e.Name()) {
			continue
		}
		at, _, ok := s.parseBackupName(e.Name())
		if !ok {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue // cannot be looked at: kept
		}
		all = append(all, bk{filepath.Join(dir, e.Name()), e.Name(), at, info.ModTime()})
	}
	if len(all) <= keepTaggedBackups {
		return
	}
	sort.Slice(all, func(i, j int) bool { return all[i].at > all[j].at })
	now := time.Now()
	for i, b := range all {
		if i < keepTaggedBackups || i == 0 || b.path == justTaken {
			continue // the newest ones, and always the one just taken
		}
		if now.Sub(time.Unix(0, b.at)) < keepTaggedAge || now.Sub(b.mod) < keepTaggedAge {
			continue // younger than the age limit by its name or by its file time
		}
		if err := removeBackupFile(b.path); err != nil {
			log.Printf("backups: could not remove the old backup %s (kept): %s", b.name, shortErr(err))
			continue
		}
		log.Printf("backups: removed the old backup %s (only the newest %d %s backups are kept)", b.name, keepTaggedBackups, strings.TrimPrefix(tag, "-"))
	}
}

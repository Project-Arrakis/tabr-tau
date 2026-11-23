package save

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/kit/savefile"
)

// ManualBackup is what BackupNow took: the name in the backup folder, the size and the SHA-256 (in hex) of the database file as it was copied, and the time it was taken.
type ManualBackup struct {
	Name   string    `json:"name"`
	Path   string    `json:"path"`
	Size   int64     `json:"size"`
	SHA256 string    `json:"sha256"`
	At     time.Time `json:"at"`
}

// afterManualCopy is called by BackupNow once the copy is written and verified against the bytes that were read, before the file is read again to see that it did not change.
// Nothing in it as shipped; a variable so a test can change the file at that point.
var afterManualCopy = func() {}

// BackupNow takes a backup of the save as the file is now, on the person's word (Database > Save backups > Back up now; the weekly run of #1167 can call it as its checkpoint step).
// It is the backup every write takes (backupNamed: the same folder, the same plain-folder check, the same durable write and read-back, the same name with the time in it), tagged
// "-manual" so the list can say it was taken by hand. It reads the save file and writes nothing to it: no gate, no queue, no working copy is touched, so it works while the game
// is running (it copies what the game last wrote). The copy is verified by its size and its SHA-256 against the bytes that were read, and the save file is read once more after the
// copy: if the game wrote to it in between, the copy may be of neither state, so it is removed and nothing is kept. The pruning never counts or removes a manual backup.
func (s *Save) BackupNow() (ManualBackup, error) {
	s.mu.Lock() // an edit being written (commit holds s.mu from the check to the reload) is not copied half way
	defer s.mu.Unlock()
	if err := s.requirePlainFile(); err != nil {
		return ManualBackup{}, err
	}
	dest, sum, err := s.backupNamed(manualTag)
	if err != nil {
		return ManualBackup{}, err
	}
	fail := func(msg string) (ManualBackup, error) {
		os.Remove(dest)
		return ManualBackup{}, apierr.New(apierr.ErrWriteFailed, msg)
	}
	back, err := savefile.ReadCapped(dest)
	if err != nil || hashOf(back) != sum {
		return fail("the backup could not be verified (its size or checksum is not the file's); nothing was kept")
	}
	afterManualCopy()
	now, err := savefile.ReadCapped(s.Path)
	if err != nil || hashOf(now) != sum || len(now) != len(back) {
		return fail("the save file changed while it was being copied (the game wrote to it); nothing was kept, try again")
	}
	return ManualBackup{Name: filepath.Base(dest), Path: dest, Size: int64(len(back)), SHA256: hex.EncodeToString(sum[:]), At: time.Now()}, nil
}

// isManualBackup says whether a name in the backup folder is one BackupNow made: a backup of this save with a time in its name and the manual tag before the extension.
func (s *Save) isManualBackup(name string) bool {
	if _, _, ok := s.parseBackupName(name); !ok {
		return false
	}
	ext := filepath.Ext(s.Path)
	return strings.HasSuffix(strings.TrimSuffix(name, ext), manualTag)
}

package save

import (
	"fmt"
	"path/filepath"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// freeBytes says how many bytes a program running as this person can still write in dir's volume, and whether that could be found out. It is a variable so a test can say the
// disk is full. Windows and Linux know it (space_windows.go, space_linux.go); elsewhere it cannot be told and the write is simply tried.
var freeBytes = freeBytesOf

// spaceMargin is kept free on top of what a write needs: the temp file and the backup are not the only things the disk is asked for while the game runs.
const spaceMargin = 16 << 20

func megabytes(n int64) string { return fmt.Sprintf("%.1f MB", float64(n)/(1<<20)) }

// requireFreeSpace refuses a write that the disk cannot hold, before anything is written: a backup of the file on disk and the new file are both written next to the save, so
// need is the size of the two. A disk that fills up in between still fails the backup or the temp file before the save is replaced, and changes nothing (SECURITY.md); this
// is the earlier and clearer message for the usual case. When the free space cannot be found out the write is tried.
func (s *Save) requireFreeSpace(need int64) error {
	free, ok := freeBytes(filepath.Dir(s.Path))
	if !ok || free >= uint64(need+spaceMargin) {
		return nil
	}
	return apierr.New(apierr.ErrWriteFailed, fmt.Sprintf("there is not enough free space on the drive that holds the save: the write needs about %s (a backup of the save and the new file) and %s are free. Nothing was changed; free some space and the edit is written.", megabytes(need+spaceMargin), megabytes(int64(free))))
}

package save

import (
	"encoding/hex"
	"os"
	"time"
)

// An automatic write replaces the file and then reads it back to compare. When the file cannot be read back (another program holds it for longer than the retries last) the write is
// unverified: the file is, as far as anyone can tell, the edited save, but nothing has proved it. The hash of what was written stays in s.writing, and the next attempt (Flush,
// Rebase) looks at the file first: if it is that file the edits are written and are not put in a second time.

// unverifiedWhy is the sentence for a write that could not be checked yet.
const unverifiedWhy = "the save was written, but it could not be verified yet (the file could not be read back); it is looked at again on the next attempt, and the edits are not written a second time"

// settleWrittenLocked looks at the file when a write is waiting to be verified (s.writing is set). If the file is the one that was written, the edits that write held are done with:
// the baseline moves to it, they leave the queue, the write log records the write, and what was queued after it stays queued. It returns what was written (nil when there was no such
// write, or the file could not be read, or it is not the file that was written; s.writing says which of the last two). The caller holds s.mu.
func (s *Save) settleWrittenLocked() (wrote []string, backup string, settled bool) {
	if s.writing == "" {
		return nil, "", false
	}
	st, statErr := os.Stat(s.Path) // taken before the read, as load does: a rewrite in between is then seen as a change
	b, err := readCapped(s.Path)
	if err != nil {
		return nil, "", false // still not readable: the write stays unverified
	}
	h := hashOf(b)
	if hex.EncodeToString(h[:]) != s.writing {
		// Not the file that was written (the replacement did not take effect, or the game saved since): the marker says nothing about this file any more, and the edits go through the
		// usual checks.
		s.writing, s.writingOps, s.writingRecs, s.writingBackup = "", 0, 0, ""
		s.writeJournal()
		return nil, "", false
	}
	n := s.writingOps
	if n > len(s.ops) {
		n = len(s.ops)
	}
	for _, o := range s.ops[:n] {
		wrote = append(wrote, o.Desc)
	}
	backup = s.writingBackup
	var extraOps []Op
	var extraRecs []opRec
	extraOps = append(extraOps, s.ops[n:]...)
	if len(s.recs) == len(s.ops) { // the journal kept every edit: the ones after the write are the tail
		extraRecs = append(extraRecs, s.recs[n:]...)
	}
	s.diskHash = h
	s.diskSize, s.diskMod = 0, time.Time{}
	s.changedSize, s.changedMod = 0, time.Time{}
	if statErr == nil {
		s.diskSize, s.diskMod = st.Size(), st.ModTime()
	}
	s.ops, s.recs, s.recStmts = extraOps, extraRecs, 0
	for _, r := range extraRecs {
		s.recStmts += len(r.Stmts)
	}
	s.dirty = len(extraOps) > 0
	s.writing, s.writingOps, s.writingRecs, s.writingBackup = "", 0, 0, ""
	if s.dirty {
		s.writeJournal()
	} else {
		s.dropJournal()
	}
	if backup != "" {
		s.lastWrote, s.lastBackup = h, backup
	}
	if r := s.restored; r != nil { // the edits put back are written; the notice of the ones that were not stays until the person has seen it
		if len(r.Conflicts) == 0 {
			s.restored = nil
		} else {
			kept := *r
			kept.Written = true
			s.restored = &kept
		}
	}
	s.appendWriteLogLocked(Flush{At: time.Now(), Saved: true, Ops: wrote, Backup: backup})
	return wrote, backup, true
}

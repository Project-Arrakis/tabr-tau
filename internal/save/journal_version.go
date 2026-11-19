package save

import (
	"encoding/hex"
	"strconv"
	"strings"
)

// otherVersionJournal settles a journal that another version of the editor kept (RestoreJournal, the caller holds s.mu). An older journal whose write already landed (its Writing is the
// file on disk now, and it holds no edit made after that write) has nothing left to put back: it is removed without a notice, so the person is not told that changes "could not be put
// back" that are in the file. Any other journal is set aside and reported, as before; the reason lists what it held, as the reason of an incomplete journal does, so the person knows
// what to make again. A journal of a newer version is never judged here: its edits are not known to this build.
func (s *Save) otherVersionJournal(j journalFile) *Restored {
	if j.Version < journalVersion && j.Writing != "" && j.Writing == hex.EncodeToString(s.diskHash[:]) && len(j.Ops) > 0 && (j.WritingOps == 0 || j.WritingOps >= len(j.Ops)) {
		s.dropJournal()
		return nil
	}
	why := "it was kept by a different version of the editor, so its changes were not put back"
	if held := descsOf(j.Ops); held != "" {
		why += "; it held: " + held + ". Make them again"
	}
	return s.skipJournal(why)
}

// descsOf lists the descriptions of queued edits for a notice: the first five, then how many more.
func descsOf(ops []opRec) string {
	var d []string
	for i, o := range ops {
		if i == 5 {
			d = append(d, "and "+strconv.Itoa(len(ops)-i)+" more")
			break
		}
		d = append(d, o.Desc)
	}
	return strings.Join(d, "; ")
}

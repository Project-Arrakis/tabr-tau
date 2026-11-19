package save

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A journal of another version whose write already landed (#778 L4): the version check used to come first, so an old journal whose edits are in the file was reported as "could not be put back".
// The journal is of version 1 or 3 here; ops carry a statement that would change the row if it were replayed, so a replay shows.
func oldJournal(t *testing.T, p, dir string, version int, writing string, writingOps int) string {
	t.Helper()
	jp := filepath.Join(dir, "pending.json")
	ops := `[{"desc":"landsraad Kenola: your contribution 1300 -> 6000","rows":1,"stmts":[{"q":"update jt set name='from the old build' where a=1"}]},{"desc":"landsraad Vernius: reopened","rows":1,"stmts":[{"q":"update jt set name='from the old build' where a=2"}]}]`
	body := fmt.Sprintf(`{"version":%d,"path":"%s","baseHash":"","ops":%s,"writing":"%s","writingOps":%d}`, version, strings.ReplaceAll(p, `\`, `\\`), ops, writing, writingOps)
	if err := os.WriteFile(jp, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return jp
}

func TestAnOldJournalWhoseWriteAlreadyLandedIsDroppedWithoutANotice(t *testing.T) {
	p := makeSave(t, journalDDL)
	probe := openJournaled(t, p, t.TempDir())
	onDisk := hex.EncodeToString(probe.diskHash[:])
	before, _ := probe.One(`select name from jt where a=1`)
	dir := t.TempDir()
	jp := oldJournal(t, p, dir, 1, onDisk, 0)
	s := openJournaled(t, p, dir)
	r, err := s.RestoreJournal()
	if err != nil || r != nil {
		t.Fatalf("a journal whose edits are in the file is no notice: %+v %v", r, err)
	}
	if s.Restored() != nil || s.Dirty() {
		t.Fatalf("notice=%v dirty=%v", s.Restored() != nil, s.Dirty())
	}
	if _, err := os.Stat(jp); err == nil {
		t.Fatal("the journal of a write that landed is removed")
	}
	if m, _ := filepath.Glob(jp + ".skipped*"); len(m) != 0 {
		t.Fatalf("nothing is set aside: %v", m)
	}
	if after, _ := s.One(`select name from jt where a=1`); after["name"] != before["name"] {
		t.Fatalf("the old journal was replayed: %v", after)
	}
}

func TestAnOldJournalIsSetAsideWhenItsWriteDidNotLandOrEditsFollowedIt(t *testing.T) {
	p := makeSave(t, journalDDL)
	probe := openJournaled(t, p, t.TempDir())
	onDisk := hex.EncodeToString(probe.diskHash[:])
	for name, c := range map[string]struct {
		version    int
		writing    string
		writingOps int
	}{
		"another file was written":               {1, strings.Repeat("0", 64), 0},
		"no write recorded":                      {1, "", 0},
		"an edit was made after that write":      {1, onDisk, 1},
		"a journal of a newer build is not ours": {3, onDisk, 0},
	} {
		dir := t.TempDir()
		jp := oldJournal(t, p, dir, c.version, c.writing, c.writingOps)
		s := openJournaled(t, p, dir)
		r, err := s.RestoreJournal()
		if err != nil || r == nil || len(r.Conflicts) != 1 || !strings.Contains(r.Conflicts[0].Why, "different version of the editor") {
			t.Fatalf("%s: the journal must be set aside with the notice: %+v %v", name, r, err)
		}
		// the reason lists what the journal held, so the person knows what to make again
		for _, w := range []string{"landsraad Kenola: your contribution 1300 -> 6000", "landsraad Vernius: reopened", "Make them again"} {
			if !strings.Contains(r.Conflicts[0].Why, w) {
				t.Fatalf("%s: the reason does not say %q: %s", name, w, r.Conflicts[0].Why)
			}
		}
		if m, _ := filepath.Glob(jp + ".skipped*"); len(m) == 0 {
			t.Fatalf("%s: the journal is kept aside, not deleted", name)
		}
	}
}

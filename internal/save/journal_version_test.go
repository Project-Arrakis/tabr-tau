package save

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A journal of another version whose write already landed (#778 L4): the version check used to come first, so an old journal whose edits are in the file was reported as "could not be put back".
// The journal is of an older or a newer version here; ops carry a statement that would change the row if it were replayed, so a replay shows.
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
		"a journal of a newer build is not ours": {journalVersion + 1, onDisk, 0},
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

// A journal whose WritingOps is every op it holds: all of them were in that write, so it is dropped like one with no count.
func TestAnOldJournalWhoseWriteHeldEveryOpIsDropped(t *testing.T) {
	p := makeSave(t, journalDDL)
	probe := openJournaled(t, p, t.TempDir())
	onDisk := hex.EncodeToString(probe.diskHash[:])
	dir := t.TempDir()
	jp := oldJournal(t, p, dir, 1, onDisk, 2) // two ops, both written
	s := openJournaled(t, p, dir)
	if r, err := s.RestoreJournal(); err != nil || r != nil {
		t.Fatalf("no notice: %+v %v", r, err)
	}
	if _, err := os.Stat(jp); err == nil {
		t.Fatal("the journal is removed")
	}
}

// A journal of another save is never judged by its hash: it is set aside whatever file is on disk.
func TestAnOldJournalOfAnotherSaveIsSetAsideEvenWhenItsHashMatches(t *testing.T) {
	p := makeSave(t, journalDDL)
	probe := openJournaled(t, p, t.TempDir())
	onDisk := hex.EncodeToString(probe.diskHash[:])
	dir := t.TempDir()
	jp := oldJournal(t, filepath.Join(dir, "another-save", "game.db"), dir, 1, onDisk, 0)
	s := openJournaled(t, p, dir)
	r, err := s.RestoreJournal()
	if err != nil || r == nil || len(r.Conflicts) != 1 {
		t.Fatalf("set aside with the notice: %+v %v", r, err)
	}
	if m, _ := filepath.Glob(jp + ".skipped*"); len(m) == 0 {
		t.Fatal("kept aside, not deleted")
	}
}

// When the journal of a landed write cannot be removed (the file is in use, Windows), the mark names the file the edits were written as, so the next start drops it without a notice instead of
// reporting changes that are in the file.
func TestAnOldJournalThatCannotBeRemovedIsDroppedQuietlyAtTheNextStart(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("a file in use cannot be removed on Windows only")
	}
	p := makeSave(t, journalDDL)
	probe := openJournaled(t, p, t.TempDir())
	onDisk := hex.EncodeToString(probe.diskHash[:])
	dir := t.TempDir()
	jp := oldJournal(t, p, dir, 1, onDisk, 0)
	held, err := os.Open(jp) // an open handle without delete sharing: os.Remove fails
	if err != nil {
		t.Fatal(err)
	}
	s := openJournaled(t, p, dir)
	r, err := s.RestoreJournal()
	if err != nil || r != nil {
		t.Fatalf("no notice for a write that landed: %+v %v", r, err)
	}
	if _, err := os.Stat(s.markPath()); err != nil {
		held.Close()
		t.Fatalf("the failed removal is marked: %v", err)
	}
	held.Close()
	s2 := openJournaled(t, p, dir)
	if r, err := s2.RestoreJournal(); err != nil || r != nil {
		t.Fatalf("the next start drops it quietly: %+v %v", r, err)
	}
	if _, err := os.Stat(jp); err == nil {
		t.Fatal("the journal is gone")
	}
	if _, err := os.Stat(s2.markPath()); err == nil {
		t.Fatal("and so is the mark")
	}
}

// A journal of version 2, kept by an earlier build, may hold a queued Recycle base that never read the Reserve (#1208), so putting it back would write it into a Reserve that holds
// stacks. It is set aside with the notice and never replayed, for this very save and with edits that would apply.
func TestAJournalOfVersion2IsSetAsideNotReplayed(t *testing.T) {
	p := makeSave(t, journalDDL)
	before, _ := openJournaled(t, p, t.TempDir()).One(`select name from jt where a=1`)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	old := `{"version":2,"path":"` + strings.ReplaceAll(p, `\`, `\\`) + `","baseHash":"","ops":[{"desc":"Recycle base Home","rows":1,"stmts":[{"q":"update jt set name='from the old build' where a=1"}]}]}`
	if err := os.WriteFile(jp, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s := openJournaled(t, p, dir)
	r, err := s.RestoreJournal()
	if err != nil || r == nil || r.Applied != 0 || len(r.Conflicts) != 1 || !strings.Contains(r.Conflicts[0].Why, "different version of the editor") || !strings.Contains(r.Conflicts[0].Why, "Recycle base Home") {
		t.Fatalf("a version 2 journal must be set aside with the notice: %+v %v", r, err)
	}
	if s.Dirty() || len(s.Pending()) != 0 {
		t.Fatalf("dirty=%v pending=%v", s.Dirty(), s.Pending())
	}
	if after, _ := s.One(`select name from jt where a=1`); after["name"] != before["name"] {
		t.Fatalf("the old journal was replayed: %v", after)
	}
	if m, _ := filepath.Glob(jp + ".skipped*"); len(m) == 0 {
		t.Fatal("the old journal is kept aside, not deleted")
	}
}

// A version 2 journal can hold notes (the Landsraad board's record of what it paid). Edits of it that are in the file hand their notes over, whether the journal is then dropped, set
// aside because edits followed the write, or found marked incomplete. The notes of edits that are not in the file are never handed over.
func TestTheNotesOfAVersion2JournalOfAWrittenQueueAreHandedOver(t *testing.T) {
	p := makeSave(t, journalDDL)
	probe := openJournaled(t, p, t.TempDir())
	onDisk := hex.EncodeToString(probe.diskHash[:])
	edit := func(v int) string {
		return fmt.Sprintf(`{"desc":"edit %d","rows":1,"stmts":[{"q":"update jt set name='from the old build' where a=1"}],"notes":[{"kind":"x","data":{"v":%d}}]}`, v, v)
	}
	for name, c := range map[string]struct {
		writing    string
		writingOps int
		marked     bool
		want       []int
		aside      bool
	}{
		"every edit written":            {onDisk, 0, false, []int{1, 2}, false},
		"an edit made after that write": {onDisk, 1, false, []int{1}, true},
		"another file was written":      {strings.Repeat("0", 64), 0, false, nil, true},
		"marked incomplete, written":    {onDisk, 0, true, []int{1, 2}, false},
	} {
		dir := t.TempDir()
		jp := filepath.Join(dir, "pending.json")
		body := fmt.Sprintf(`{"version":2,"path":"%s","baseHash":"","ops":[%s,%s],"writing":"%s","writingOps":%d}`, strings.ReplaceAll(p, `\`, `\\`), edit(1), edit(2), c.writing, c.writingOps)
		if err := os.WriteFile(jp, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if c.marked {
			if err := os.WriteFile(jp+".incomplete", []byte(`{"version":2,"reason":"the kept file could not be removed","writing":"`+c.writing+`"}`), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		s := openJournaled(t, p, dir)
		var k sunk
		s.SetNoteSink(k.sink)
		r, err := s.RestoreJournal()
		if err != nil || (r != nil) != c.aside || s.Dirty() {
			t.Fatalf("%s: %+v %v dirty=%v", name, r, err, s.Dirty())
		}
		if got := values(t, k.got); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Fatalf("%s: notes %v, want %v", name, got, c.want)
		}
		for _, n := range k.got {
			if !n.Again {
				t.Fatalf("%s: a journal that outlived the write may have delivered before: its notes are marked as a repeat", name)
			}
		}
		if c.aside { // the person is told to make again only what is not in the file
			why := r.Conflicts[0].Why
			if !strings.Contains(why, "edit 2") || strings.Contains(why, "edit 1") != (len(c.want) == 0) {
				t.Fatalf("%s: the reason: %s", name, why)
			}
		}
		if m, _ := filepath.Glob(jp + ".skipped*"); (len(m) != 0) != c.aside {
			t.Fatalf("%s: set aside: %v", name, m)
		}
		if _, err := os.Stat(jp); err == nil {
			t.Fatalf("%s: the journal is still in place", name)
		}
	}
}

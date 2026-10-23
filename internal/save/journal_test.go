package save

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const journalDDL = `create table jt(a integer primary key, name text, f real, b blob, n integer);
insert into jt values (1,'one',1.5,x'0102',10),(2,'two',2.5,x'0304',20);`

// open starts an editor session on the save at p with the journal in dir/pending.json.
func openJournaled(t *testing.T, p, dir string) *Save {
	t.Helper()
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.EnableJournal(filepath.Join(dir, "pending.json"))
	return s
}

func rowName(t *testing.T, s *Save, a int) string {
	t.Helper()
	r, err := s.One(`select name from jt where a=?`, a)
	if err != nil {
		t.Fatal(err)
	}
	if r == nil {
		return "<none>"
	}
	return r["name"].(string)
}

func TestPendingEditsComeBackAfterTheEditorIsClosedAndOpenedAgain(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	if _, err := s1.Mutate("rename one", func(m *Mut) error {
		_, err := m.Exec(`update jt set name=?, f=?, b=?, n=? where a=?`, "uno", 2.25, []byte{9, 8, 7}, nil, 1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s1.Mutate("add three", func(m *Mut) error {
		_, err := m.Exec(`insert into jt values (?,?,?,?,?)`, 3, "three", 3.5, []byte{5}, int64(30))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json")); err != nil {
		t.Fatalf("the journal should be on disk after an edit: %v", err)
	}
	s1.Close()

	s2 := openJournaled(t, p, dir)
	if s2.Dirty() {
		t.Fatal("a fresh open has nothing pending until the journal is restored")
	}
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 2 || len(r.Conflicts) != 0 || r.Rebased {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if got := s2.Pending(); len(got) != 2 || got[0] != "rename one" || got[1] != "add three" {
		t.Fatalf("pending after restore: %v", got)
	}
	row, _ := s2.One(`select name, f, hex(b) hb, n from jt where a=1`)
	if row["name"] != "uno" || row["f"] != 2.25 || row["hb"] != "090807" || row["n"] != nil {
		t.Fatalf("values were not kept exactly (a float, a blob and a null among them): %v", row)
	}
	if rowName(t, s2, 3) != "three" {
		t.Fatal("the inserted row should be back")
	}
	if s2.Restored() == nil || s2.Restored().Applied != 2 {
		t.Fatal("the notice about what was put back should be kept until dismissed")
	}
	s2.DismissRestored()
	if s2.Restored() != nil {
		t.Fatal("dismissed")
	}
	// the restored edits are journaled again, so a second restart still has them
	s2.Close()
	s3 := openJournaled(t, p, dir)
	if r, _ := s3.RestoreJournal(); r == nil || r.Applied != 2 {
		t.Fatalf("a second restart: %+v", r)
	}
}

// The game saves while the editor is closed: the edits are put back onto the newer file, rows the game changed elsewhere stay as the game wrote them.
func TestPendingEditsAreRebasedOntoAFileTheGameSavedInTheMeantime(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	if _, err := s1.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	s1.Close()
	newer := makeSave(t, journalDDL+`insert into jt values (3,'game row',0,x'00',0); update jt set n=99 where a=2;`)
	b, _ := os.ReadFile(newer)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 1 || !r.Rebased {
		t.Fatalf("rebase: %+v %v", r, err)
	}
	if rowName(t, s2, 1) != "uno" || rowName(t, s2, 3) != "game row" {
		t.Fatal("the edit and the game's own change should both be there")
	}
	if row, _ := s2.One(`select n from jt where a=2`); row["n"] != int64(99) {
		t.Fatalf("the game's change to another row is kept: %v", row)
	}
}

// An edit that no longer fits (the row it changed is gone, or it now changes a different number of rows) is reported, not forced.
func TestAnEditThatNoLongerAppliesIsReportedNotForced(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	for _, d := range []struct{ desc, q string }{
		{"rename one", `update jt set name='uno' where a=1`},
		{"rename two", `update jt set name='dos' where a=2`},
	} {
		if _, err := s1.Mutate(d.desc, func(m *Mut) error { _, err := m.Exec(d.q); return err }); err != nil {
			t.Fatal(err)
		}
	}
	s1.Close()
	newer := makeSave(t, journalDDL+`delete from jt where a=1;`) // the game deleted row 1
	b, _ := os.ReadFile(newer)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 1 || r.Conflicts[0].Desc != "rename one" {
		t.Fatalf("restore: %+v %v", r, err)
	}
	if !strings.Contains(r.Conflicts[0].Why, "0 rows") {
		t.Fatalf("the reason should say what differed: %q", r.Conflicts[0].Why)
	}
	if got := s2.Pending(); len(got) != 1 || got[0] != "rename two" {
		t.Fatalf("only the edit that applied is pending: %v", got)
	}
	if rowName(t, s2, 1) != "<none>" {
		t.Fatal("the deleted row must stay deleted")
	}
}

func TestDiscardAndSaveRemoveTheJournal(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	s := openJournaled(t, p, dir)
	edit := func() {
		t.Helper()
		if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
			t.Fatal(err)
		}
	}
	edit()
	if _, err := os.Stat(jp); err != nil {
		t.Fatal("journal expected")
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jp); !os.IsNotExist(err) {
		t.Fatal("Discard drops the journal: the person chose to throw the edits away")
	}
	edit()
	defer NoGameRunningForTests()()
	if _, err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jp); !os.IsNotExist(err) {
		t.Fatal("a saved edit is no longer pending: the journal goes")
	}
}

func TestReloadNowKeepsTheEditsWhereTheyStillApply(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	newer := makeSave(t, journalDDL+`insert into jt values (3,'game row',0,x'00',0);`)
	b, _ := os.ReadFile(newer)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if !s.ChangedOnDisk() {
		t.Fatal("the game's save should be noticed")
	}
	r, err := s.Rebase()
	if err != nil || r == nil || r.Applied != 1 || !r.Rebased {
		t.Fatalf("rebase: %+v %v", r, err)
	}
	if s.ChangedOnDisk() {
		t.Fatal("after the reload the editor works from the new file")
	}
	if rowName(t, s, 1) != "uno" || rowName(t, s, 3) != "game row" || len(s.Pending()) != 1 {
		t.Fatalf("the edit is on the new file: %q %q %v", rowName(t, s, 1), rowName(t, s, 3), s.Pending())
	}
	if _, err := s.Commit(); err == nil {
		// the game-running guard may refuse here; what matters is that the file check does not (it would be ErrChangedOnDisk)
		t.Log("committed")
	} else if err == ErrChangedOnDisk {
		t.Fatal("after a rebase the save onto the new file is not a stale one")
	}
}

// Only simple statements are kept; an edit that ran anything else turns the journal off for the session rather than keep part of the edits.
func TestAnEditTheJournalCannotKeepTurnsItOffInsteadOfKeepingPartOfTheEdits(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	s := openJournaled(t, p, dir)
	if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jp); err != nil {
		t.Fatal("journal expected")
	}
	if _, err := s.Mutate("a script", func(m *Mut) error {
		_, err := m.Exec(`update jt set name='x' where a=1; update jt set name='y' where a=2`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if s.JournalOff() == "" {
		t.Fatal("a script is not kept")
	}
	if _, err := os.Stat(jp); !os.IsNotExist(err) {
		t.Fatal("a journal with only some of the edits would restore a state the person never had: it is removed")
	}
	if _, err := s.Mutate("rename three", func(m *Mut) error { _, err := m.Exec(`update jt set name='z' where a=2`); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jp); !os.IsNotExist(err) {
		t.Fatal("and later edits do not bring it back")
	}
	if len(s.Pending()) != 3 {
		t.Fatalf("the edits themselves are unaffected: %v", s.Pending())
	}
}

// Turning the journal off for one edit must not outlive it: once the edits are saved or discarded, the next edit is kept again, and Reload now works.
func TestTheJournalComesBackOnceTheEditThatTurnedItOffIsGone(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	s := openJournaled(t, p, dir)
	if _, err := s.Mutate("a script", func(m *Mut) error {
		_, err := m.Exec(`update jt set name='x' where a=1; update jt set name='y' where a=2`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if s.JournalOff() == "" {
		t.Fatal("a script turns the journal off")
	}
	if _, err := s.Rebase(); err == nil {
		t.Fatal("with unkept edits pending, Reload now cannot put them back and must say so")
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if s.JournalOff() != "" {
		t.Fatal("nothing is pending after Discard: the journal is on again")
	}
	if _, err := s.Rebase(); err != nil {
		t.Fatalf("a clean editor reloads: %v", err)
	}
	if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jp); err != nil {
		t.Fatal("the next edit is journaled again")
	}
}

func TestAJournalOfAnotherSaveOrTamperedWithIsNotUsed(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	for name, content := range map[string]string{
		"another save": `{"version":1,"path":"C:\\other\\game.db","baseHash":"","ops":[{"desc":"x","rows":1,"stmts":[{"q":"update jt set name='bad' where a=1"}]}]}`,
		"not json":     `{oops`,
		"new version":  `{"version":9,"path":"` + strings.ReplaceAll(p, `\`, `\\`) + `","ops":[{"desc":"x","rows":1,"stmts":[{"q":"update jt set name='bad' where a=1"}]}]}`,
	} {
		if err := os.WriteFile(jp, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		s := openJournaled(t, p, dir)
		if r, err := s.RestoreJournal(); r != nil || err != nil || s.Dirty() {
			t.Fatalf("%s: %+v %v dirty=%v", name, r, err, s.Dirty())
		}
		s.Close()
	}
	// a statement that is not an INSERT, UPDATE, DELETE or REPLACE in a journal of this save is refused when it is replayed
	tamper := `{"version":1,"path":"` + strings.ReplaceAll(p, `\`, `\\`) + `","baseHash":"","ops":[{"desc":"drop","rows":0,"stmts":[{"q":"drop table jt"}]}]}`
	if err := os.WriteFile(jp, []byte(tamper), 0o600); err != nil {
		t.Fatal(err)
	}
	s := openJournaled(t, p, dir)
	r, err := s.RestoreJournal()
	if err != nil || r == nil || r.Applied != 0 || len(r.Conflicts) != 1 {
		t.Fatalf("a DDL statement in a journal must not run: %+v %v", r, err)
	}
	if row, _ := s.One(`select count(*) c from jt`); row["c"] != int64(2) {
		t.Fatal("the table must still be there")
	}
}

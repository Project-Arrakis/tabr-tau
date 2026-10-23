package save

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DBA-10 of the 2026-10-22 audit: what the journal keeps of an edit must be what the edit ran, down to the type of every value, and a journal that cannot be written must say so.

// restartAndRestore closes the session, opens the save again and puts the journal back, as a restart of the editor does.
func restartAndRestore(t *testing.T, p, dir string, s *Save) *Save {
	t.Helper()
	s.Close()
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || len(r.Conflicts) != 0 {
		t.Fatalf("restore: %+v %v", r, err)
	}
	return s2
}

func TestANilBlobStaysNullAndAnEmptyOneStaysABlob(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	if _, err := s.Mutate("null and empty", func(m *Mut) error {
		if _, err := m.Exec(`update jt set b=? where a=1`, []byte(nil)); err != nil {
			return err
		}
		_, err := m.Exec(`update jt set b=? where a=2`, []byte{})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Query(`select a, typeof(b) t from jt order by a`)
	if before[0]["t"] != "null" || before[1]["t"] != "blob" {
		t.Fatalf("what the edit made: %v", before)
	}
	s2 := restartAndRestore(t, p, dir, s)
	after, _ := s2.Query(`select a, typeof(b) t from jt order by a`)
	if after[0]["t"] != "null" || after[1]["t"] != "blob" {
		t.Fatalf("put back, a nil blob is NULL and an empty blob is a blob: %v", after)
	}
}

func TestTextThatIsNotValidUTF8IsPutBackByteForByte(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	bad := "a\xffb\xc3"
	if _, err := s.Mutate("odd text", func(m *Mut) error { _, err := m.Exec(`update jt set name=? where a=1`, bad); return err }); err != nil {
		t.Fatal(err)
	}
	s2 := restartAndRestore(t, p, dir, s)
	r, _ := s2.One(`select hex(name) h, typeof(name) t from jt where a=1`)
	if r["h"] != "61FF62C3" || r["t"] != "text" {
		t.Fatalf("the bytes are the ones the edit ran, not U+FFFD: %v", r)
	}
}

func TestAFloat32IsPutBackWithTheValueThatWasStored(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	if _, err := s.Mutate("single precision", func(m *Mut) error { _, err := m.Exec(`update jt set f=? where a=1`, float32(0.1)); return err }); err != nil {
		t.Fatal(err)
	}
	stored, _ := s.One(`select f from jt where a=1`)
	s2 := restartAndRestore(t, p, dir, s)
	again, _ := s2.One(`select f from jt where a=1`)
	if stored["f"] != again["f"] || again["f"] != float64(float32(0.1)) {
		t.Fatalf("stored %v, put back %v: the value must not be widened or rounded differently", stored["f"], again["f"])
	}
}

func TestASemicolonInsideAQuotedValueDoesNotStopTheJournal(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s := openJournaled(t, p, dir)
	for i, q := range []string{
		`update jt set name='a;b' where a=1`,
		`update jt set name="x;y" where a=999 or a=2`, // "x;y" is read by SQLite as a name; the statement still runs
		`update jt set n=7 where a=1; -- a comment; with one`,
	} {
		if _, err := s.Mutate("edit "+string(rune('a'+i)), func(m *Mut) error { _, err := m.Exec(q); return err }); err != nil {
			t.Fatalf("edit %d: %v", i, err)
		}
	}
	if why := s.JournalOff(); why != "" {
		t.Fatalf("the journal is on while the statements are single ones: %q", why)
	}
	if _, err := os.Stat(filepath.Join(dir, "pending.json")); err != nil {
		t.Fatal(err)
	}
	s2 := restartAndRestore(t, p, dir, s)
	if rowName(t, s2, 1) != "a;b" {
		t.Fatalf("the value with a semicolon came back: %q", rowName(t, s2, 1))
	}
}

func TestOneStatementKnowsWhereAStatementEnds(t *testing.T) {
	for q, want := range map[string]bool{
		`update t set a=1`:                          true,
		`update t set a=1;`:                         true,
		"update t set a=1 ;\n\t ":                   true,
		`update t set a='x;y'`:                      true,
		`update t set a='it''s;'`:                   true,
		`update t set "a;b"=1`:                      true,
		"update t set `a;b`=1":                      true,
		`update t set [a;b]=1`:                      true,
		"update t set a=1 -- ; not a second\n":      true,
		`update t set a=1 /* ; */`:                  true,
		`update t set a=1; -- done`:                 true,
		`update t set a=1;;`:                        true,
		`update t set a=1; update t set b=2`:        false,
		`update t set a=1; update t set b=2;`:       false,
		`update t set a='x'; delete from t`:         false,
		`update t set a='unterminated`:              false,
		`update t set a=1; /* c */ delete from t`:   false,
		`update t set a=1 /* ; */ ; delete from t`:  false,
		`update t set a='x;y'; update t set b='z;'`: false,
	} {
		if got := oneStatement(q); got != want {
			t.Errorf("oneStatement(%q) = %v, want %v", q, got, want)
		}
	}
	if journalable(`update t set a='x;y'`) != true || journalable(`update t set a=1; update t set b=2`) != false {
		t.Error("journalable follows oneStatement")
	}
	if !readable(`select * from t where a='x;y'`) || readable(`select 1; select 2`) {
		t.Error("readable follows oneStatement")
	}
}

func TestAJournalThatCannotBeWrittenIsTurnedOffAndSaysWhy(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-folder")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	s.EnableJournal(filepath.Join(blocker, "pending.json")) // its folder cannot be made: a file is in the way
	if _, err := s.Mutate("edit", func(m *Mut) error { _, err := m.Exec(`update jt set name='x' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	if why := s.JournalOff(); !strings.Contains(why, "could not be written") {
		t.Fatalf("a failed journal write is not swallowed: %q", why)
	}
	if !s.Dirty() || len(s.Pending()) != 1 {
		t.Fatal("the edit itself is still made and pending")
	}
	if strings.Contains(s.JournalOff(), dir) {
		t.Fatalf("the reason names no folder: %q", s.JournalOff())
	}
}

func TestTheJournalIsFlushedToDiskBeforeItReplacesTheOldOne(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	var synced []string
	old := syncFile
	t.Cleanup(func() { syncFile = old })
	syncFile = func(f *os.File) error {
		_, err := os.Stat(jp)
		synced = append(synced, f.Name()+" final-exists="+map[bool]string{true: "no", false: "yes"}[os.IsNotExist(err)])
		return old(f)
	}
	s := openJournaled(t, p, dir)
	if _, err := s.Mutate("edit", func(m *Mut) error { _, err := m.Exec(`update jt set name='x' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	if len(synced) != 1 || !strings.HasSuffix(synced[0], "pending.json.tmp final-exists=no") {
		t.Fatalf("the temp file is flushed before the rename: %v", synced)
	}
	// a flush that fails leaves no half journal and turns the journal off with the reason
	syncFile = func(*os.File) error { return errors.New("disk full") }
	if _, err := s.Mutate("edit 2", func(m *Mut) error { _, err := m.Exec(`update jt set name='y' where a=2`); return err }); err != nil {
		t.Fatal(err)
	}
	if why := s.JournalOff(); !strings.Contains(why, "disk full") {
		t.Fatalf("the reason is the error: %q", why)
	}
	// no half journal and no temp file; the journal that was kept before stays (it holds the first edit), because deleting it would lose that edit as well
	if _, err := os.Stat(jp + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("the temp file must not be left behind: %v", err)
	}
	if kept, err := os.ReadFile(jp); err != nil || !strings.Contains(string(kept), `"edit"`) || strings.Contains(string(kept), `"edit 2"`) {
		t.Fatalf("the journal that was kept stays as it was: %v %s", err, kept)
	}
}

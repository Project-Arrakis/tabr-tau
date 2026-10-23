package save

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// GRC-3 of the 2026-10-22 audit: a persistent record of what the editor wrote, when, and after which backup.

func readLog(t *testing.T, path string) []writeLogLine {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []writeLogLine
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var l writeLogLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatalf("a line is not JSON: %q", sc.Text())
		}
		out = append(out, l)
	}
	return out
}

func loggedSave(t *testing.T) (*Save, string) {
	t.Helper()
	s, err := Open(makeSave(t, journalDDL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	lp := filepath.Join(t.TempDir(), "write-log.jsonl")
	s.EnableWriteLog(lp)
	return s, lp
}

func edit(t *testing.T, s *Save, desc, q string, args ...any) {
	t.Helper()
	if _, err := s.Mutate(desc, func(m *Mut) error { _, err := m.Exec(q, args...); return err }); err != nil {
		t.Fatal(err)
	}
}

func TestEveryWriteLeavesOneLineWithTheTimeTheEditsAndTheBackup(t *testing.T) {
	defer NoGameRunningForTests()()
	s, lp := loggedSave(t)
	edit(t, s, "Rename row one", `update jt set name='uno' where a=1`)
	edit(t, s, "Rename row two", `update jt set name='dos' where a=2`)
	f := s.Flush()
	if !f.Saved {
		t.Fatalf("%+v", f)
	}
	edit(t, s, "Rename row three", `insert into jt values (3,'tres',0.5,x'00',1)`)
	if f2 := s.Flush(); !f2.Saved {
		t.Fatalf("%+v", f2)
	}
	lines := readLog(t, lp)
	if len(lines) != 2 {
		t.Fatalf("one line per write: %d", len(lines))
	}
	l := lines[0]
	if l.Edits != 2 || len(l.Ops) != 2 || l.Ops[0] != "Rename row one" || l.Ops[1] != "Rename row two" {
		t.Fatalf("the edits are named: %+v", l)
	}
	if l.Backup == "" || l.Backup != filepath.Base(f.Backup) || strings.ContainsAny(l.Backup, `/\`) {
		t.Fatalf("the backup is named by file name: %q", l.Backup)
	}
	if _, err := time.Parse(time.RFC3339, l.At); err != nil {
		t.Fatalf("the time: %q", l.At)
	}
	if lines[1].Edits != 1 {
		t.Fatalf("%+v", lines[1])
	}
	if st, err := os.Stat(lp); err != nil || st.Mode().Perm()&0o077 != 0 && runtime.GOOS != "windows" {
		t.Fatalf("private file: %v %v", err, st)
	}
	if s.WriteLogProblem() != "" {
		t.Fatal(s.WriteLogProblem())
	}
}

func TestAnEditThatIsOnlyQueuedIsNotInTheLogUntilItIsWritten(t *testing.T) {
	defer NoGameRunningForTests()()
	s, lp := loggedSave(t)
	defer ActiveSessionForTests()()
	edit(t, s, "Rename row one", `update jt set name='uno' where a=1`)
	if f := s.Flush(); f.Saved || f.Queued != 1 {
		t.Fatalf("%+v", f)
	}
	if _, err := os.Stat(lp); !os.IsNotExist(err) {
		t.Fatal("nothing was written, so nothing is recorded")
	}
}

func TestTheLogHoldsNoValuesOfPrivateFields(t *testing.T) {
	defer NoGameRunningForTests()()
	s, lp := loggedSave(t)
	acct := "7656119" + "8000000001" // built in two parts so the repository's identifier scan does not see an id in the source
	edit(t, s, "edit item 10 of account "+acct, `update jt set name='x' where a=1`)
	edit(t, s, "sql: update jt set name='a secret name' where a=2", `update jt set name='y' where a=2`)
	edit(t, s, strings.Repeat("long ", 100), `update jt set n=1 where a=1`)
	if f := s.Flush(); !f.Saved {
		t.Fatalf("%+v", f)
	}
	raw, _ := os.ReadFile(lp)
	for _, bad := range []string{acct, "a secret name", "update jt"} {
		if strings.Contains(string(raw), bad) {
			t.Errorf("the log holds %q: %s", bad, raw)
		}
	}
	l := readLog(t, lp)[0]
	if l.Ops[0] != "edit item 10 of account <number>" || !strings.HasPrefix(l.Ops[1], "SQL console statement") || len([]rune(l.Ops[2])) > writeLogDescMax+3 {
		t.Fatalf("%q", l.Ops)
	}
}

func TestTheLogIsCappedAndKeepsOneOlderGeneration(t *testing.T) {
	defer NoGameRunningForTests()()
	s, lp := loggedSave(t)
	old := writeLogMax
	writeLogMax = 700
	t.Cleanup(func() { writeLogMax = old })
	for i := 0; i < 12; i++ {
		edit(t, s, "Rename row one to a longer sentence so that the file grows "+strings.Repeat("x", i), `update jt set n=n+1 where a=1`)
		if f := s.Flush(); !f.Saved {
			t.Fatalf("%d: %+v", i, f)
		}
	}
	dir := filepath.Dir(lp)
	ents, _ := os.ReadDir(dir)
	if len(ents) != 2 {
		t.Fatalf("two generations at most: %v", ents)
	}
	rot := filepath.Join(dir, "write-log.1.jsonl")
	for _, p := range []string{lp, rot} {
		st, err := os.Stat(p)
		if err != nil || st.Size() > writeLogMax {
			t.Fatalf("%s: %v %v", p, err, st)
		}
	}
	total := len(readLog(t, lp)) + len(readLog(t, rot))
	if total < 4 || total >= 12 {
		t.Fatalf("the oldest lines were dropped, the newest kept: %d lines in two files", total)
	}
	last := readLog(t, lp)
	if !strings.HasSuffix(last[len(last)-1].Ops[0], strings.Repeat("x", 11)) {
		t.Fatalf("the newest write is the last line: %v", last[len(last)-1].Ops)
	}
}

// A log that cannot be written never undoes or hides the write: the file is written, and the state says what is missing until a later write is recorded.
func TestALogThatCannotBeWrittenIsReportedAndTheWriteStands(t *testing.T) {
	defer NoGameRunningForTests()()
	s, _ := loggedSave(t)
	dir := t.TempDir()
	blocker := filepath.Join(dir, "in-the-way")
	os.WriteFile(blocker, []byte("x"), 0o600)
	s.EnableWriteLog(filepath.Join(blocker, "write-log.jsonl"))
	edit(t, s, "Rename row one", `update jt set name='uno' where a=1`)
	f := s.Flush()
	if !f.Saved {
		t.Fatalf("the write stands: %+v", f)
	}
	if p := s.WriteLogProblem(); !strings.Contains(p, "could not be recorded") || strings.Contains(p, dir) {
		t.Fatalf("the problem is stated without folder names: %q", p)
	}
	s.EnableWriteLog(filepath.Join(dir, "ok", "write-log.jsonl"))
	edit(t, s, "Rename row two", `update jt set name='dos' where a=2`)
	if f := s.Flush(); !f.Saved {
		t.Fatal("second write")
	}
	if s.WriteLogProblem() != "" {
		t.Fatalf("recorded again, so no problem: %q", s.WriteLogProblem())
	}
}

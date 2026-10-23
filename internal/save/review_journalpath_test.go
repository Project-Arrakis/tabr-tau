package save

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A journal kept under the same file in another spelling of the path (case, on Windows) is the same save's: it is put back, not ignored.
func TestAJournalWhosePathDiffersOnlyByCaseIsPutBackOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("paths are case-sensitive here")
	}
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	if _, err := s1.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	s1.Close()
	jp := filepath.Join(dir, "pending.json")
	b, _ := os.ReadFile(jp)
	var jf journalFile
	if err := json.Unmarshal(b, &jf); err != nil {
		t.Fatal(err)
	}
	jf.Path = strings.ToUpper(jf.Path)
	if jf.Path == p {
		t.Fatal("the test did not change the path")
	}
	swapped, _ := json.Marshal(jf)
	if err := os.WriteFile(jp, []byte(swapped), 0o600); err != nil {
		t.Fatal(err)
	}
	s2 := openJournaled(t, p, dir)
	if r, err := s2.RestoreJournal(); err != nil || r == nil || r.Applied != 1 {
		t.Fatalf("the same file under another case: %+v %v", r, err)
	}
	if rowName(t, s2, 1) != "uno" {
		t.Fatal("the edit is back")
	}
}

// A journal that cannot be used is reported, and the next edit does not write over it.
func TestAnUnusableJournalIsReportedAndNotOverwritten(t *testing.T) {
	p := makeSave(t, journalDDL)
	dir := t.TempDir()
	jp := filepath.Join(dir, "pending.json")
	if err := os.WriteFile(jp, []byte(`{"version":7,"ops":[{"desc":"x"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := openJournaled(t, p, dir)
	r, err := s.RestoreJournal()
	if err != nil || r == nil || len(r.Conflicts) != 1 || !strings.Contains(r.Conflicts[0].Why, "different version") {
		t.Fatalf("reported with the reason: %+v %v", r, err)
	}
	if _, err := s.Mutate("rename one", func(m *Mut) error { _, err := m.Exec(`update jt set name='uno' where a=1`); return err }); err != nil {
		t.Fatal(err)
	}
	kept, err := os.ReadFile(jp + ".skipped")
	if err != nil || !strings.Contains(string(kept), `"version":7`) {
		t.Fatalf("the unusable journal is still there: %v %s", err, kept)
	}
}

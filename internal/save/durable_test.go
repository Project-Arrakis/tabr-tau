package save

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Durable Commit / Restore (audit F-07): unique verified backups, fsync, rename with retry, no truncate-in-place,
// the on-disk hash check is never skipped, stale temp files are cleaned, symlinks are refused.

func openToy(t *testing.T) (*Save, string) {
	t.Helper()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, p
}

func dirNames(t *testing.T, dir string) []string {
	ents, _ := os.ReadDir(dir)
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func TestBackupNamesAreUniqueWithinOneSecond(t *testing.T) {
	s, _ := openToy(t)
	var baks []string
	for i := 0; i < 3; i++ {
		if _, err := s.ExecScript(`insert into t values (2)`); err != nil {
			t.Fatal(err)
		}
		r, err := s.Commit(false)
		if err != nil {
			t.Fatal(err)
		}
		baks = append(baks, r["backup"].(string))
	}
	seen := map[string]bool{}
	for _, b := range baks {
		if seen[b] {
			t.Fatalf("backup name reused: %s", b)
		}
		seen[b] = true
	}
	if n := len(s.Backups()); n != 3 {
		t.Fatalf("want 3 distinct backups, have %d", n)
	}
}

func TestForceNeverSkipsTheOnDiskChangeCheck(t *testing.T) {
	s, p := openToy(t)
	s.ExecScript(`insert into t values (2)`)
	other := makeSave(t, `create table t(a integer); insert into t values (9);`)
	b, _ := os.ReadFile(other)
	os.WriteFile(p, b, 0o600)
	if _, err := s.Commit(true); err == nil {
		t.Fatal("force must not bypass the changed-on-disk check")
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, b) {
		t.Fatal("a refused commit must leave the file alone")
	}
}

func TestRenameRetriesThenSucceeds(t *testing.T) {
	s, p := openToy(t)
	calls := 0
	old := fsRename
	fsRename = func(a, b string) error {
		calls++
		if calls < 3 {
			return errors.New("sharing violation")
		}
		return os.Rename(a, b)
	}
	oldWait := renameBackoff
	renameBackoff = 0
	t.Cleanup(func() { fsRename = old; renameBackoff = oldWait })
	s.ExecScript(`insert into t values (2)`)
	r, err := s.Commit(false)
	if err != nil || r["saved"] != true {
		t.Fatalf("commit should survive transient rename failures: %v %v", r, err)
	}
	if calls != 3 {
		t.Fatalf("rename calls = %d", calls)
	}
	s2, _ := Open(p)
	defer s2.Close()
	if r, _ := s2.One(`select count(*) c from t`); r["c"].(int64) != 2 {
		t.Fatalf("saved file wrong: %v", r)
	}
}

func TestRenameFailureLeavesOriginalAndCleansTemp(t *testing.T) {
	s, p := openToy(t)
	orig, _ := os.ReadFile(p)
	old := fsRename
	fsRename = func(a, b string) error { return errors.New("locked") }
	oldWait := renameBackoff
	renameBackoff = 0
	t.Cleanup(func() { fsRename = old; renameBackoff = oldWait })
	s.ExecScript(`insert into t values (2)`)
	if _, err := s.Commit(false); err == nil {
		t.Fatal("expected failure")
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, orig) {
		t.Fatal("original must be untouched")
	}
	for _, n := range dirNames(t, filepath.Dir(p)) {
		if strings.HasSuffix(n, ".tmp") {
			t.Fatalf("temp file left behind: %s", n)
		}
	}
	if !s.Dirty() {
		t.Fatal("the user's edits must still be pending after a failed commit")
	}
}

func TestStaleTempFilesAreCleanedAtOpen(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	stale := p + ".tabr-123.tmp"
	os.WriteFile(stale, []byte("junk"), 0o600)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("stale temp file should be removed at open")
	}
}

func TestBackupIsVerifiedAndPrivate(t *testing.T) {
	s, p := openToy(t)
	orig, _ := os.ReadFile(p)
	s.ExecScript(`insert into t values (2)`)
	r, err := s.Commit(false)
	if err != nil {
		t.Fatal(err)
	}
	bak := r["backup"].(string)
	if b, _ := os.ReadFile(bak); !bytes.Equal(b, orig) {
		t.Fatal("backup differs from original")
	}
	if runtime.GOOS != "windows" {
		for _, f := range []string{bak, p} {
			if st, _ := os.Stat(f); st.Mode().Perm()&0o077 != 0 {
				t.Fatalf("%s is group/world accessible: %v", f, st.Mode())
			}
		}
	}
}

func TestSymlinkedSaveIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	_, p := openToy(t)
	link := filepath.Join(t.TempDir(), "game.db")
	if err := os.Symlink(p, link); err != nil {
		t.Skip(err)
	}
	s, err := Open(link)
	if err != nil {
		return // refusing at open is fine
	}
	defer s.Close()
	s.ExecScript(`insert into t values (2)`)
	if _, err := s.Commit(false); err == nil {
		t.Fatal("commit through a symlink must be refused")
	}
}

func TestRestoreNeverTruncatesTheLiveFile(t *testing.T) {
	s, p := openToy(t)
	s.ExecScript(`insert into t values (2)`)
	r, err := s.Commit(false)
	if err != nil {
		t.Fatal(err)
	}
	bakName := filepath.Base(r["backup"].(string))
	cur, _ := os.ReadFile(p)
	old := fsRename
	fsRename = func(a, b string) error { return errors.New("locked") }
	oldWait := renameBackoff
	renameBackoff = 0
	t.Cleanup(func() { fsRename = old; renameBackoff = oldWait })
	if err := s.Restore(bakName); err == nil {
		t.Fatal("expected failure")
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, cur) {
		t.Fatal("a failed restore must leave the live file byte-identical (no in-place truncate)")
	}
}

func TestRestoreRefusesCorruptBackupAndPathTricks(t *testing.T) {
	s, p := openToy(t)
	s.ExecScript(`insert into t values (2)`)
	if _, err := s.Commit(false); err != nil {
		t.Fatal(err)
	}
	bdir := filepath.Join(filepath.Dir(p), "tabr-tau-backups")
	os.WriteFile(filepath.Join(bdir, "bad.db"), []byte("not a save"), 0o600)
	if err := s.Restore("bad.db"); err == nil {
		t.Fatal("corrupt backup must be refused")
	}
	if err := s.Restore("../game.db"); err == nil {
		t.Fatal("path components in a backup name must be refused")
	}
}

func TestRestoreRefusesBackupWithUnknownPatches(t *testing.T) {
	s, p := openToy(t)
	// current save has one patch; a backup carrying a patch the current save lacks came from a newer game
	s.ExecScript(`insert into applied_patches(name,date) values ('p1',1)`)
	if _, err := s.Commit(false); err != nil {
		t.Fatal(err)
	}
	s.ExecScript(`insert into applied_patches(name,date) values ('p2',2)`)
	r, err := s.Commit(false)
	if err != nil {
		t.Fatal(err)
	}
	// r's backup holds {p1}; make the live save older ({}), then restoring the {p1} backup must be refused
	s.ExecScript(`delete from applied_patches`)
	if _, err := s.Commit(false); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(filepath.Base(r["backup"].(string))); err == nil {
		t.Fatal("a backup with patches the current save does not have must be refused")
	}
	_ = p
}

func TestReloadFailureAfterRenameIsAWarningNotAnError(t *testing.T) {
	s, p := openToy(t)
	s.ExecScript(`insert into t values (2)`)
	old := reloadAfterCommit
	reloadAfterCommit = func(*Save) error { return errors.New("boom") }
	t.Cleanup(func() { reloadAfterCommit = old })
	r, err := s.Commit(false)
	if err != nil {
		t.Fatalf("the file was written; a reload problem must not be reported as a failed save: %v", err)
	}
	if r["saved"] != true || r["warning"] == nil {
		t.Fatalf("want saved with a warning, got %v", r)
	}
	s2, _ := Open(p)
	defer s2.Close()
	if r, _ := s2.One(`select count(*) c from t`); r["c"].(int64) != 2 {
		t.Fatal("file not written")
	}
}

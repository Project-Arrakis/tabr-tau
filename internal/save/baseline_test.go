package save

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The editor keeps one working copy of the save on disk, not two: the state as loaded is rebuilt from the file when a test or a diagnostic asks for it (WithBaseline).

// ownTempDir points the temporary folder at one of the test's own, so a copy made by something else running on the machine is not counted.
func ownTempDir(t *testing.T) {
	t.Helper()
	d := t.TempDir()
	t.Setenv("TMP", d)
	t.Setenv("TEMP", d)
	t.Setenv("TMPDIR", d)
}

func origCopies(t *testing.T) int {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(os.TempDir(), "tabr-tau-orig-*.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	return len(m)
}

func TestLoadingASaveMakesNoPristineCopy(t *testing.T) {
	defer NoGameRunningForTests()()
	ownTempDir(t)
	before := origCopies(t)
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	setA(t, s, 2)
	if err := s.Discard(); err != nil { // Discard reads the file again
		t.Fatal(err)
	}
	if n := origCopies(t); n != before {
		t.Fatalf("loading and reloading left %d pristine copies behind", n-before)
	}
}

func TestWithBaselineBuildsTheStateAsLoadedAndRemovesItAgain(t *testing.T) {
	defer NoGameRunningForTests()()
	ownTempDir(t)
	before := origCopies(t)
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	setA(t, s, 2)
	var was, now int64
	if err := s.WithBaseline(func(orig, cur *sql.DB) error {
		if err := orig.QueryRow(`select a from t`).Scan(&was); err != nil {
			return err
		}
		return cur.QueryRow(`select a from t`).Scan(&now)
	}); err != nil {
		t.Fatal(err)
	}
	if was != 1 || now != 2 {
		t.Fatalf("baseline %d, current %d; want 1 and 2", was, now)
	}
	if n := origCopies(t); n != before {
		t.Fatalf("the pristine copy was left behind (%d)", n-before)
	}
}

// Once the game has saved, the file on disk is not the state that was loaded any more, and the baseline says so rather than showing the game's file as the editor's.
func TestWithBaselineRefusesAFileThatChangedOnDisk(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	setA(t, s, 2)
	theGameSaves(t, p, `create table t(a integer); insert into t values (7);`)
	called := false
	err := s.WithBaseline(func(orig, cur *sql.DB) error { called = true; return nil })
	if err == nil || !strings.Contains(err.Error(), "changed on disk") || called {
		t.Fatalf("called=%v err=%v", called, err)
	}
}

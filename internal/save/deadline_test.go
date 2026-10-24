package save

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// S7c of #40: a read of the working copy (Table, Query, One), the soundness check of a decoded save (inspect) and the list of tables each have a deadline. They hold the save's lock,
// so a statement or a file built to run for ever would otherwise freeze every other request.

const neverEnds = `with recursive c(x) as (select 1 union all select x+1 from c) select count(*) from c`

// within fails the test, instead of letting it hang, when f has not returned after a generous limit. The limit is only a net under a regression: nothing is measured against it.
func within(t *testing.T, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); f() }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the call did not return: it has no deadline")
	}
}

func TestAReadThatNeverEndsIsStoppedAndTheSaveStaysUsable(t *testing.T) {
	s, _ := openToy(t)
	defer DeadlinesForTests(100 * time.Millisecond)()
	var err error
	within(t, func() { _, err = s.Query(neverEnds) })
	if !errors.Is(err, ErrStopped) || !strings.Contains(err.Error(), "stopped after") {
		t.Fatalf("Query must be stopped and say so: %v", err)
	}
	within(t, func() { _, _, err = s.Table(neverEnds) })
	if !errors.Is(err, ErrStopped) {
		t.Fatalf("Table must be stopped: %v", err)
	}
	restore := DeadlinesForTests(5 * time.Second)
	defer restore()
	rows, err := s.Query(`select a from t`)
	if err != nil || len(rows) != 1 {
		t.Fatalf("the save keeps working after a stopped read: %v %v", rows, err)
	}
}

func TestAReadThatIsPastItsDeadlineFailsWithoutRunning(t *testing.T) {
	s, _ := openToy(t)
	defer DeadlinesForTests(-time.Second)()
	if _, err := s.One(`select 1 x`); !errors.Is(err, ErrStopped) {
		t.Fatalf("One: %v", err)
	}
}

func TestInspectOfADecodedSaveHasADeadline(t *testing.T) {
	_, p := openToy(t)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inspect(raw); err != nil {
		t.Fatalf("a sound save passes: %v", err)
	}
	defer DeadlinesForTests(-time.Second)()
	if _, err := inspect(raw); !errors.Is(err, ErrStopped) {
		t.Fatalf("inspect must be stopped by its deadline, not report an unsound save: %v", err)
	}
}

// S8c of #40: a backups folder that is a link would send the backups (and the removal of the old ones) somewhere else.
func TestALinkedBackupsFolderIsRefused(t *testing.T) {
	s, p := openToy(t)
	elsewhere := t.TempDir()
	link := filepath.Join(filepath.Dir(p), "tabr-tau-backups")
	makeFolderLink(t, link, elsewhere)
	if _, err := s.ExecScript(`insert into t values (2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitForTests(); err == nil || !strings.Contains(err.Error(), "links are refused") {
		t.Fatalf("a backup into a linked folder must be refused: %v", err)
	}
	if got := dirNames(t, elsewhere); len(got) != 0 {
		t.Fatalf("nothing may be written through the link: %v", got)
	}
	if err := s.Restore("x"); err == nil {
		t.Fatal("restore from a linked folder must be refused")
	}
}

func TestAMissingBackupsFolderIsFineAndAPlainOneToo(t *testing.T) {
	d := t.TempDir()
	if err := RequirePlainFolder(filepath.Join(d, "none")); err != nil {
		t.Fatal(err)
	}
	if err := RequirePlainFolder(d); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(d, "afile")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RequirePlainFolder(f); err == nil {
		t.Fatal("a file where the folder should be is refused")
	}
}

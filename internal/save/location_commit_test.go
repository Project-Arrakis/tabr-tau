package save

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Commit and Restore refuse a save in a place the guard cannot vouch for, change nothing, and go ahead once the person has confirmed.
func TestCommitAndRestoreRefuseAnUntrustedLocationUntilConfirmed(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.ExecScript(`insert into t values (1)`)
	before, _ := os.ReadFile(p)

	oldOS, oldLoc := goos, locationFor
	goos, locationFor = "windows", defaultLocationFor
	t.Cleanup(func() { goos, locationFor = oldOS, oldLoc })
	fakeGame(t, nil, "", spStatus{}, nil)

	if _, err := s.CommitForTests(); err == nil || !strings.Contains(err.Error(), "cannot tell whether the game is using it") {
		t.Fatalf("Commit must refuse and say why: %v", err)
	}
	if now, _ := os.ReadFile(p); !bytes.Equal(before, now) || len(s.Backups()) != 0 || !s.Dirty() {
		t.Fatal("a refused Commit must change nothing and keep the edit")
	}
	if err := s.Restore("whatever"); err == nil || !strings.Contains(err.Error(), "cannot tell whether the game is using it") {
		t.Fatalf("Restore must refuse too: %v", err)
	}
	ConfirmLocation(p)
	if res, err := s.CommitForTests(); err != nil || res["saved"] != true {
		t.Fatalf("after the confirmation the same edit saves: %v %v", res, err)
	}
}

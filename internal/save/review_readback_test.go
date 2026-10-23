package save

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// The game saved right after the rename: the file is its newest. The read-back finds it is not the edited save, and must not put the copy taken before the edits over it.
func TestAReadBackMismatchNeverOverwritesTheGamesNewerFile(t *testing.T) {
	defer NoGameRunningForTests()()
	s, p := openToy(t)
	s.ExecScript(`insert into t values (2)`)
	newer := []byte("the game saved something else right after the rename")
	old := fsRename
	calls := 0
	fsRename = func(a, b string) error {
		if err := os.Rename(a, b); err != nil {
			return err
		}
		if calls++; calls == 1 {
			return os.WriteFile(b, newer, 0o600) // the game's own write, once
		}
		return nil
	}
	t.Cleanup(func() { fsRename = old })
	_, err := s.CommitForTests()
	if err == nil {
		t.Fatal("a file that is not the edited save is not a success")
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, newer) {
		t.Fatalf("the game's newer file must be left alone, got %d bytes", len(got))
	}
	if strings.Contains(err.Error(), "was restored") {
		t.Fatalf("nothing was restored, and the message must not say so: %v", err)
	}
	if !s.Dirty() {
		t.Fatal("the edits stay queued")
	}
}

// When the replacement did not take effect at all (the file is still the loaded one), the message says that, and nothing is rewritten.
func TestAReadBackThatFindsThePreviousFileSaysSo(t *testing.T) {
	defer NoGameRunningForTests()()
	s, p := openToy(t)
	orig, _ := os.ReadFile(p)
	s.ExecScript(`insert into t values (2)`)
	old := fsRename
	calls := 0
	fsRename = func(a, b string) error { // the first one reports success and replaces nothing
		if calls++; calls == 1 {
			os.Remove(a)
			return nil
		}
		return os.Rename(a, b)
	}
	t.Cleanup(func() { fsRename = old })
	_, err := s.CommitForTests()
	if err == nil || !strings.Contains(err.Error(), "did not take effect") {
		t.Fatalf("the message names what happened: %v", err)
	}
	if got, _ := os.ReadFile(p); !bytes.Equal(got, orig) {
		t.Fatal("the file is the loaded one, untouched")
	}
}

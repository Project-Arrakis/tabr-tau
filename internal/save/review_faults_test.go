package save

import (
	"errors"
	"os"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// A write that fails on the disk side is a fault, not a refusal of the request: the error says so (the web layer answers 500 with the code write_failed).
func TestAWriteThatCannotReplaceTheFileIsAWriteFault(t *testing.T) {
	defer NoGameRunningForTests()()
	s, _ := openToy(t)
	s.ExecScript(`insert into t values (2)`)
	old, oldWait := fsRename, renameBackoff
	fsRename = func(a, b string) error { return errors.New("locked") }
	renameBackoff = 0
	t.Cleanup(func() { fsRename, renameBackoff = old, oldWait })
	_, err := s.CommitForTests()
	if !errors.Is(err, apierr.ErrWriteFailed) {
		t.Fatalf("a rename that never works is a write fault: %v", err)
	}
}

func TestAReadBackMismatchIsAWriteFault(t *testing.T) {
	defer NoGameRunningForTests()()
	s, _ := openToy(t)
	s.ExecScript(`insert into t values (2)`)
	old := fsRename
	calls := 0
	fsRename = func(a, b string) error {
		if err := os.Rename(a, b); err != nil {
			return err
		}
		if calls++; calls == 1 {
			return os.WriteFile(b, []byte("something else"), 0o600)
		}
		return nil
	}
	t.Cleanup(func() { fsRename = old })
	if _, err := s.CommitForTests(); !errors.Is(err, apierr.ErrWriteFailed) {
		t.Fatalf("a file that does not read back is a write fault: %v", err)
	}
}

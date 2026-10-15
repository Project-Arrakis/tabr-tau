package save

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A second editor on the same save is refused while the first runs (NET-1 of the 2026-10-15 audit), and the lock is gone when the first
// lets go.
func TestOnlyOneEditorCanHoldASave(t *testing.T) {
	p := filepath.Join(t.TempDir(), "game.db")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	release, err := LockSave(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LockSave(p); !errors.Is(err, ErrSaveInUse) {
		t.Fatalf("a second editor on the same save must be refused: %v", err)
	}
	other := filepath.Join(filepath.Dir(p), "other.db")
	releaseOther, err := LockSave(other)
	if err != nil {
		t.Fatalf("another save is free: %v", err)
	}
	releaseOther()
	release()
	again, err := LockSave(p)
	if err != nil {
		t.Fatalf("after release the save is free again: %v", err)
	}
	again()
	if m, _ := filepath.Glob(p + ".tabr-tau.lock"); len(m) != 0 {
		t.Fatalf("the lock file is cleaned up: %v", m)
	}
}

// If the lock file cannot be made at all, the editor still starts (it could not write its backups there either).
func TestAFolderThatCannotHoldALockFileDoesNotStopTheEditor(t *testing.T) {
	release, err := LockSave(filepath.Join(t.TempDir(), "no-such-folder", "game.db"))
	if err != nil {
		t.Fatalf("%v", err)
	}
	release()
}

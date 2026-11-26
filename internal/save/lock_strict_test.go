package save

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// #1209: where LockSave goes on holding no lock, LockSaveStrict refuses; held and free are the same as for LockSave.
func TestLockSaveStrictRefusesWhenTheLockFileCannotBeMade(t *testing.T) {
	p := filepath.Join(t.TempDir(), "game.db")
	release, err := LockSaveStrict(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LockSaveStrict(p); !errors.Is(err, ErrSaveInUse) {
		t.Fatalf("held: %v", err)
	}
	release()
	if err := os.Mkdir(p+".tabr-tau.lock", 0o700); err != nil { // a folder where the lock file should be
		t.Fatal(err)
	}
	for _, path := range []string{p, filepath.Join(t.TempDir(), "no-such-folder", "game.db")} {
		if release, err := LockSaveStrict(path); !errors.Is(err, ErrNoLock) || release != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}

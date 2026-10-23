package save

import (
	"os"
	"path/filepath"
	"testing"
)

// The editor goes on when the lock file cannot be made for a reason other than another editor holding it, but it then holds no lock and must not sweep: the temp file next to
// the save may be another editor's write in flight.
func TestAnEditorThatHoldsNoLockSweepsNothing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "game.db")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p+".tabr-tau.lock", 0o700); err != nil { // a folder where the lock file should be: it cannot be created
		t.Fatal(err)
	}
	stale := p + ".tabr-123.tmp"
	if err := os.WriteFile(stale, []byte("another editor's write in flight"), 0o600); err != nil {
		t.Fatal(err)
	}
	release, err := LockSave(p)
	if err != nil {
		t.Fatalf("the editor still starts: %v", err)
	}
	release()
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("no lock held, nothing swept: %v", err)
	}
}

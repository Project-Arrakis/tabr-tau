package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A config file is replaced the way the save and the settings are (kit/durable): the new text is flushed to the disk before it replaces the file, the rename is tried again
// while another program holds the file, and a failure leaves the old file whole with no temp file beside it.

func hooksBack(t *testing.T) {
	t.Helper()
	old := configHooks
	t.Cleanup(func() { configHooks = old })
}

func TestAConfigFileIsFlushedBeforeTheRenameAndTheFolderAfterIt(t *testing.T) {
	hooksBack(t)
	d := setup(t, map[string]string{"Game.ini": settings})
	var steps []string
	configHooks.SyncFile = func(f *os.File) error { steps = append(steps, "sync"); return f.Sync() }
	configHooks.Rename = func(a, b string) error {
		steps = append(steps, "rename:"+filepath.Base(b))
		if !strings.HasPrefix(filepath.Base(a), "Game.ini.tabr-") || !strings.HasSuffix(a, ".tmp") {
			t.Errorf("the temp file is named %s", filepath.Base(a))
		}
		return os.Rename(a, b)
	}
	configHooks.SyncDir = func(string) error { steps = append(steps, "dir"); return nil }
	if err := d.Set("Game.ini", "Other", "A", "2", 5); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(steps, ","); got != "sync,rename:Game.ini,dir" {
		t.Fatalf("want flush, rename, folder flush; got %s", got)
	}
}

func TestAConfigRenameThatFailsForAMomentIsTriedAgain(t *testing.T) {
	hooksBack(t)
	d := setup(t, map[string]string{"Game.ini": settings})
	calls := 0
	configHooks.Rename = func(a, b string) error {
		if calls++; calls < 3 {
			return errors.New("sharing violation")
		}
		return os.Rename(a, b)
	}
	// the pause is the package's own (a fraction of a second here); the point is that the write goes through
	if err := d.Set("Game.ini", "Other", "A", "2", 5); err != nil {
		t.Fatalf("a rename that works on the third try is a success: %v", err)
	}
	if calls != 3 {
		t.Fatalf("rename calls = %d", calls)
	}
	if !strings.Contains(read(t, d, "Game.ini"), "A=2") {
		t.Fatal("the edit is not in the file")
	}
}

func TestAConfigFileThatCannotBeFlushedIsNotReplacedAndLeavesNoTempFile(t *testing.T) {
	hooksBack(t)
	d := setup(t, map[string]string{"Game.ini": settings})
	before := read(t, d, "Game.ini")
	configHooks.SyncFile = func(*os.File) error { return errors.New("disk full") }
	if err := d.Set("Game.ini", "Other", "A", "2", 5); err == nil {
		t.Fatal("a file that could not be flushed must not be reported as written")
	}
	if got := read(t, d, "Game.ini"); got != before {
		t.Fatal("the config file changed")
	}
	if m, _ := filepath.Glob(filepath.Join(d.Path, "*.tabr-*.tmp")); len(m) != 0 {
		t.Fatalf("temp files left behind: %v", m)
	}
}

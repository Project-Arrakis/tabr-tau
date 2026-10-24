package save

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// D8c of #39: a write the disk cannot hold is refused before anything is written, and the message says why.

func diskWith(t *testing.T, free uint64) {
	t.Helper()
	old := freeBytes
	freeBytes = func(string) (uint64, bool) { return free, true }
	t.Cleanup(func() { freeBytes = old })
}

func TestAWriteTheDiskCannotHoldIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	s, p := openToy(t)
	before, _ := os.ReadFile(p)
	if _, err := s.ExecScript(`insert into t values (2)`); err != nil {
		t.Fatal(err)
	}
	diskWith(t, 1<<10)
	_, err := s.CommitForTests()
	if err == nil || !strings.Contains(err.Error(), "not enough free space") || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("a full disk is said in words: %v", err)
	}
	if _, serr := os.Stat(filepath.Join(filepath.Dir(p), "tabr-tau-backups")); serr == nil {
		t.Fatal("no backup folder is made for a write that is refused")
	}
	if got, _ := os.ReadFile(p); string(got) != string(before) {
		t.Fatal("the save must be as it was")
	}
	if !s.Dirty() {
		t.Fatal("the edit stays queued")
	}
	diskWith(t, 1<<40)
	if _, err := s.CommitForTests(); err != nil {
		t.Fatalf("with room the same edit is written: %v", err)
	}
}

func TestAFullDiskAlsoRefusesARestore(t *testing.T) {
	s, _ := openToy(t)
	s.ExecScript(`insert into t values (2)`)
	r, err := s.CommitForTests()
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(r["backup"].(string))
	diskWith(t, 1<<10)
	if err := s.Restore(name); err == nil || !strings.Contains(err.Error(), "not enough free space") {
		t.Fatalf("restore: %v", err)
	}
}

func TestTheFreeSpaceOfARealFolderIsKnownOnWindowsAndLinux(t *testing.T) {
	free, ok := freeBytesOf(t.TempDir())
	if !ok {
		t.Skip("free space cannot be told on this system")
	}
	if free == 0 {
		t.Fatal("a folder with files in it reports no free space")
	}
}

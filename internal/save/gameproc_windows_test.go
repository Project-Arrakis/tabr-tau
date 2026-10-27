package save

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The real process snapshot (no program is started): it lists this test's own process, and matchGameImages applies the same prefix rule to what it returns.
func TestTheProcessSnapshotListsRunningProcesses(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	names, err := processImages()
	if err != nil {
		t.Fatalf("the process snapshot failed: %v", err)
	}
	base := filepath.Base(exe)
	found := false
	for _, n := range names {
		if strings.EqualFold(n, base) {
			found = true
		}
	}
	if !found {
		t.Fatalf("this test process %q is not in the snapshot of %d processes", base, len(names))
	}
	if got := matchGameImages(names); len(got) > 0 {
		t.Logf("a Dune process is running on this machine: %v", got)
	}
}

// The real file check (#449): a handle opened the way Go and most programs open files (read and write shared, delete not) is seen, a closed one is not, a missing file is an
// error (the caller then queues), and the editor's own open Save does not hold the save file. A handle that shares delete is NOT seen: the known limit of the check.
func TestSaveFileHeldSeesAnOpenHandle(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	if held, err := saveFileHeld(p); err != nil || held {
		t.Fatalf("nothing has the file open: held=%v err=%v", held, err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	held, err := saveFileHeld(p)
	f.Close()
	if err != nil || !held {
		t.Fatalf("an open handle must be seen: held=%v err=%v", held, err)
	}
	if held, err := saveFileHeld(p); err != nil || held {
		t.Fatalf("closed again: held=%v err=%v", held, err)
	}
	if _, err := saveFileHeld(p + ".missing"); err == nil {
		t.Fatal("a missing file is a check that cannot be made, not 'not held'")
	}
	// a handle that shares delete (limit of the check, written down so a change shows)
	pp, _ := syscall.UTF16PtrFromString(p)
	h, err := syscall.CreateFile(pp, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	held, err = saveFileHeld(p)
	syscall.CloseHandle(h)
	if err != nil || held {
		t.Fatalf("known limit: a handle that shares delete is not seen: held=%v err=%v", held, err)
	}
	// the editor itself does not keep its save file open
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if held, err := saveFileHeld(p); err != nil || held {
		t.Fatalf("an open Save must not hold the file: held=%v err=%v", held, err)
	}
}

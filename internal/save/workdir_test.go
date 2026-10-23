package save

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// SEC-3 of the 2026-10-22 audit: the decoded copies of a save hold its account and platform ids. They live in a folder of their own, are named by the process that made them,
// are removed when the session ends, and the ones a dead process left are swept at the next start.

func listWork(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(os.TempDir(), workDirName))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func TestADecodedCopyLivesInTheEditorsWorkFolderNamedByItsProcessAndGoesWithTheSession(t *testing.T) {
	p := makeSave(t, journalDDL)
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "p" + strconv.Itoa(os.Getpid()) + "-tabr-tau-"
	found := 0
	for _, n := range listWork(t) {
		if strings.HasPrefix(n, prefix) {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("one copy named by this process in the work folder: %v", listWork(t))
	}
	loose, _ := filepath.Glob(filepath.Join(os.TempDir(), "tabr-tau-*.sqlite"))
	if len(loose) != 0 {
		t.Fatalf("no copy loose in the temp folder: %v", loose)
	}
	if st, err := os.Stat(filepath.Join(os.TempDir(), workDirName)); err != nil || st.Mode().Perm()&0o077 != 0 && !isWindows() {
		t.Fatalf("the folder is the owner's alone: %v %v", err, st)
	}
	s.Close()
	for _, n := range listWork(t) {
		if strings.HasPrefix(n, prefix) {
			t.Fatalf("closing the session removes its copy: %v", listWork(t))
		}
	}
}

func isWindows() bool { return runtime.GOOS == "windows" }

func TestTheSweepRemovesTheCopiesOfProcessesThatAreGoneAndNoOthers(t *testing.T) {
	// a process that has ended: run the test binary itself, doing nothing
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	dead := cmd.ProcessState.Pid()
	if processAlive(dead) {
		t.Skip("the process id was reused at once")
	}
	if !processAlive(os.Getpid()) {
		t.Fatal("this process is alive")
	}
	dir := workDir()
	mk := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	gone := mk("p" + strconv.Itoa(dead) + "-tabr-tau-1.sqlite")
	mine := mk("p" + strconv.Itoa(os.Getpid()) + "-tabr-tau-2.sqlite")
	odd := mk("something-else.sqlite") // not named by a process: not ours to remove
	oldLoose := filepath.Join(os.TempDir(), "tabr-tau-123.sqlite")
	newLoose := filepath.Join(os.TempDir(), "tabr-tau-456.sqlite")
	otherFile := filepath.Join(os.TempDir(), "unrelated-old-file.sqlite")
	for _, p := range []string{oldLoose, newLoose, otherFile} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldLoose, old, old)
	os.Chtimes(otherFile, old, old)

	if n := SweepWorkCopies(); n != 2 {
		t.Fatalf("removed %d, want the dead process's copy and the old loose one", n)
	}
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
	if exists(gone) || exists(oldLoose) {
		t.Fatal("the copy of a dead process and a day-old loose copy are removed")
	}
	if !exists(mine) || !exists(odd) || !exists(newLoose) || !exists(otherFile) {
		t.Fatal("this process's copy, a file that is not a copy, a recent loose copy and an unrelated file stay")
	}
}

func TestLockingASaveSweepsTheCopiesOfDeadProcesses(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(workDir(), "p"+strconv.Itoa(cmd.ProcessState.Pid())+"-tabr-tau-9.sqlite")
	os.WriteFile(gone, []byte("x"), 0o600)
	release, err := LockSave(makeSave(t, journalDDL))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := os.Stat(gone); err == nil {
		t.Fatal("holding the lock sweeps the stale copy")
	}
}

package durable

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// All files here are made by the tests in a temporary folder.

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

// quick is Defaults without the waiting.
func quick() Options {
	o := Defaults()
	o.Pause = 0
	return o
}

func TestWriteFileCreatesAndReplaces(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.json")
	if err := WriteFile(p, []byte("one"), 0o600, quick()); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "one" {
		t.Fatalf("created file holds %q", got)
	}
	if err := WriteFile(p, []byte("two, longer"), 0o600, quick()); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "two, longer" {
		t.Fatalf("replaced file holds %q", got)
	}
	if err := WriteFile(p, nil, 0o600, quick()); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "" {
		t.Fatalf("an empty write empties the file, got %q", got)
	}
	if n := names(t, filepath.Dir(p)); len(n) != 1 {
		t.Fatalf("only the target stays in the folder: %v", n)
	}
}

func TestWriteFileKeepsTheModeItWasGiven(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no permission bits beyond read-only")
	}
	p := filepath.Join(t.TempDir(), "a")
	if err := WriteFile(p, []byte("x"), 0o640, quick()); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v", st.Mode().Perm())
	}
}

func TestTheStepsRunInOrderFlushRenameFolder(t *testing.T) {
	var steps []string
	o := quick()
	o.Hooks = Hooks{
		SyncFile:     func(f *os.File) error { steps = append(steps, "sync"); return f.Sync() },
		BeforeRename: func(dst string) { steps = append(steps, "before:"+filepath.Base(dst)) },
		Rename:       func(a, b string) error { steps = append(steps, "rename"); return os.Rename(a, b) },
		SyncDir:      func(string) error { steps = append(steps, "dir"); return nil },
	}
	if err := WriteFile(filepath.Join(t.TempDir(), "x.json"), []byte("d"), 0o600, o); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(steps, ","); got != "sync,before:x.json,rename,dir" {
		t.Fatalf("steps = %s", got)
	}
}

func TestTheFolderIsNotFlushedUnlessAsked(t *testing.T) {
	o := quick()
	o.SyncDir = false
	o.Hooks.SyncDir = func(string) error { t.Error("the folder was flushed"); return nil }
	if err := WriteFile(filepath.Join(t.TempDir(), "x"), []byte("d"), 0o600, o); err != nil {
		t.Fatal(err)
	}
}

func TestAFolderThatCannotBeFlushedIsNotAFailedWrite(t *testing.T) {
	o := quick()
	o.Hooks.SyncDir = func(string) error { return errors.New("not supported") }
	p := filepath.Join(t.TempDir(), "x")
	if err := WriteFile(p, []byte("d"), 0o600, o); err != nil {
		t.Fatalf("the file has been replaced by then: %v", err)
	}
	if read(t, p) != "d" {
		t.Fatal("the data is missing")
	}
}

func TestACrashBetweenTheTempFileAndTheRenameLeavesTheOldFileWhole(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.json")
	if err := WriteFile(p, []byte("old"), 0o600, quick()); err != nil {
		t.Fatal(err)
	}
	crash := errors.New("power cut")
	o := quick()
	o.Hooks.BeforeRename = func(dst string) { panic(crash) }
	func() {
		defer func() {
			if r := recover(); r != crash {
				t.Fatalf("want the simulated crash, got %v", r)
			}
		}()
		WriteFile(p, []byte("new"), 0o600, o)
	}()
	if got := read(t, p); got != "old" {
		t.Fatalf("the target changed although the rename never happened: %q", got)
	}
	var tmps []string
	for _, n := range names(t, filepath.Dir(p)) {
		if strings.HasSuffix(n, ".tmp") {
			tmps = append(tmps, n)
		}
	}
	if len(tmps) != 1 || read(t, filepath.Join(filepath.Dir(p), tmps[0])) != "new" {
		t.Fatalf("a crash leaves the whole new file beside the old one: %v", tmps)
	}
}

func TestAFailedFlushWritesNothingAndLeavesNoTempFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.json")
	if err := WriteFile(p, []byte("old"), 0o600, quick()); err != nil {
		t.Fatal(err)
	}
	disk := errors.New("disk full")
	o := quick()
	o.Hooks.SyncFile = func(*os.File) error { return disk }
	o.Hooks.Rename = func(a, b string) error { t.Error("renamed a file that was not flushed"); return nil }
	if err := WriteFile(p, []byte("new"), 0o600, o); !errors.Is(err, disk) {
		t.Fatalf("a file that could not be flushed must not be reported as written: %v", err)
	}
	if read(t, p) != "old" {
		t.Fatal("the target changed")
	}
	if n := names(t, filepath.Dir(p)); len(n) != 1 {
		t.Fatalf("temp file left behind: %v", n)
	}
}

func TestRenameIsTriedAgainAndThenSucceeds(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.json")
	calls := 0
	o := quick()
	o.Hooks.Rename = func(a, b string) error {
		calls++
		if calls < 3 {
			return errors.New("sharing violation")
		}
		return os.Rename(a, b)
	}
	if err := WriteFile(p, []byte("new"), 0o600, o); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || read(t, p) != "new" {
		t.Fatalf("calls = %d, file = %q", calls, read(t, p))
	}
}

func TestARenameThatNeverSucceedsReturnsTheLastErrorAndLeavesTheOldFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.json")
	if err := WriteFile(p, []byte("old"), 0o600, quick()); err != nil {
		t.Fatal(err)
	}
	calls := 0
	o := quick()
	o.Hooks.Rename = func(a, b string) error { calls++; return fmt.Errorf("locked %d", calls) }
	err := WriteFile(p, []byte("new"), 0o600, o)
	if err == nil || err.Error() != "locked 6" || calls != DefaultAttempts {
		t.Fatalf("want the last of %d errors, got %v after %d calls", DefaultAttempts, err, calls)
	}
	if read(t, p) != "old" {
		t.Fatal("the target changed")
	}
	if n := names(t, filepath.Dir(p)); len(n) != 1 {
		t.Fatalf("temp file left behind: %v", n)
	}
}

func TestTheZeroOptionsTryTheRenameOnce(t *testing.T) {
	calls := 0
	o := Options{Hooks: Hooks{Rename: func(a, b string) error { calls++; return errors.New("no") }}}
	if err := Rename("a", "b", o); err == nil || calls != 1 {
		t.Fatalf("want one failed try, got %v after %d", err, calls)
	}
}

func TestOnlyErrorsThatRetryAcceptsAreTriedAgain(t *testing.T) {
	busy := errors.New("busy")
	calls := 0
	o := quick()
	o.Attempts = 10
	o.Retry = func(err error) bool { return errors.Is(err, busy) }
	o.Hooks.Rename = func(a, b string) error {
		calls++
		if calls < 3 {
			return busy
		}
		return os.ErrPermission
	}
	err := Rename("a", "b", o)
	if !errors.Is(err, os.ErrPermission) || calls != 3 {
		t.Fatalf("want the permanent error at once on the third call, got %v after %d", err, calls)
	}
}

func TestThePauseGrowsOrStaysFixed(t *testing.T) {
	fail := func(string, string) error { return errors.New("no") }
	// Sleeping is never shorter than asked, so the elapsed time has a safe lower bound.
	grow := Options{Attempts: 3, Pause: 20 * time.Millisecond, Growing: true, Hooks: Hooks{Rename: fail}}
	start := time.Now()
	Rename("a", "b", grow)
	if d := time.Since(start); d < 120*time.Millisecond { // 20 + 40 + 60
		t.Fatalf("growing pause waited only %v", d)
	}
	fixed := Options{Attempts: 3, Pause: 20 * time.Millisecond, Hooks: Hooks{Rename: fail}}
	start = time.Now()
	Rename("a", "b", fixed)
	if d := time.Since(start); d < 60*time.Millisecond { // 20 + 20 + 20
		t.Fatalf("fixed pause waited only %v", d)
	}
}

func TestTheTempFileNameFollowsThePattern(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Game.ini")
	var seen string
	o := quick()
	o.TempPattern = "Game.ini.tabr-*.tmp"
	o.Hooks.Rename = func(a, b string) error { seen = filepath.Base(a); return os.Rename(a, b) }
	if err := WriteFile(p, []byte("x"), 0o600, o); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(seen, "Game.ini.tabr-") || !strings.HasSuffix(seen, ".tmp") {
		t.Fatalf("temp file = %q", seen)
	}
	o.TempPattern = ""
	if err := WriteFile(p, []byte("x"), 0o600, o); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(seen, "Game.ini.") || !strings.HasSuffix(seen, ".tmp") {
		t.Fatalf("default temp file = %q", seen)
	}
}

func TestAMissingFolderIsAnErrorAndNothingIsCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nope")
	if err := WriteFile(filepath.Join(dir, "a"), []byte("x"), 0o600, quick()); err == nil {
		t.Fatal("want an error")
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("the folder was created")
	}
}

func TestWriteNewRefusesAnExistingFileAndLeavesItAlone(t *testing.T) {
	p := filepath.Join(t.TempDir(), "backup.db")
	if err := os.WriteFile(p, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := WriteNew(p, []byte("other"), 0o600, quick())
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("want ErrExist, got %v", err)
	}
	if read(t, p) != "keep" {
		t.Fatal("an existing file was overwritten or removed")
	}
}

func TestWriteNewWritesAndFlushes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "b")
	synced := 0
	o := quick()
	o.Hooks.SyncFile = func(f *os.File) error { synced++; return f.Sync() }
	data := bytes.Repeat([]byte("0123456789"), 100000)
	if err := WriteNew(p, data, 0o600, o); err != nil {
		t.Fatal(err)
	}
	if synced != 1 || read(t, p) != string(data) {
		t.Fatalf("synced %d times, content ok = %v", synced, read(t, p) == string(data))
	}
}

func TestWriteNewRemovesWhatItCreatedWhenTheFlushFails(t *testing.T) {
	p := filepath.Join(t.TempDir(), "b")
	o := quick()
	o.Hooks.SyncFile = func(*os.File) error { return errors.New("disk full") }
	if err := WriteNew(p, []byte("x"), 0o600, o); err == nil {
		t.Fatal("a file that could not be flushed must not be reported as written")
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("the unflushed file was left")
	}
}

func TestRenameDoesNotRemoveTheSource(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "from"), filepath.Join(dir, "to")
	os.WriteFile(from, []byte("x"), 0o600)
	o := quick()
	o.Hooks.Rename = func(a, b string) error { return errors.New("locked") }
	if err := Rename(from, to, o); err == nil {
		t.Fatal("want an error")
	}
	if _, err := os.Stat(from); err != nil {
		t.Fatal("the caller decides whether the source goes")
	}
}

func TestSyncDir(t *testing.T) {
	if err := SyncDir(t.TempDir()); err != nil {
		t.Fatalf("an existing folder: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "nope")
	err := SyncDir(missing)
	if runtime.GOOS == "windows" {
		if err != nil {
			t.Fatalf("on Windows it is a no-op, got %v", err)
		}
	} else if err == nil {
		t.Fatal("a missing folder cannot be flushed")
	}
}

// Writers that replace one file at once leave one of the complete contents, never a mix, and no temp file.
func TestConcurrentWritersLeaveOneWholeFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a")
	o := Options{Attempts: 100, Pause: 5 * time.Millisecond}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := WriteFile(p, bytes.Repeat([]byte{byte('a' + i)}, 50000), 0o600, o); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got := read(t, p)
	if len(got) != 50000 || strings.Trim(got, got[:1]) != "" {
		t.Fatalf("the file is a mix: %d bytes", len(got))
	}
	if n := names(t, filepath.Dir(p)); len(n) != 1 {
		t.Fatalf("temp files left: %v", n)
	}
}

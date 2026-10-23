package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Review 2026-10-23, group C: config writes take a lock, use a temp file of their own, and prune their backups.

// Many edits at once (each reads the file, changes a line, writes it back) must all be in the file afterwards.
func TestConcurrentConfigEditsAreNotLost(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[S]\r\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&sb, "K%d=0\r\n", i)
	}
	d := setup(t, map[string]string{"Game.ini": sb.String()})
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- d.Set("Game.ini", "S", fmt.Sprintf("K%d", i), "1", i+1)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := read(t, d, "Game.ini")
	for i := 0; i < 20; i++ {
		if !strings.Contains(got, fmt.Sprintf("K%d=1\r\n", i)) {
			t.Fatalf("the edit of K%d was lost:\n%s", i, got)
		}
	}
}

// A file that happens to be called like the old fixed temp name is not written over, and no temp file is left behind.
func TestAConfigWriteUsesATempFileOfItsOwn(t *testing.T) {
	d := setup(t, map[string]string{"Game.ini": settings, "Game.ini.tmp": "somebody else's file"})
	if err := d.Set("Game.ini", "Other", "A", "2", 5); err != nil {
		t.Fatal(err)
	}
	if got := read(t, d, "Game.ini.tmp"); got != "somebody else's file" {
		t.Fatalf("a file with the old temp name was written over: %q", got)
	}
	m, _ := filepath.Glob(filepath.Join(d.Path, "*.tabr-*.tmp"))
	if len(m) != 0 {
		t.Fatalf("temp files left behind: %v", m)
	}
}

// The backups of a file are kept to the newest sixty and the oldest; another file whose name starts the same keeps its own.
func TestConfigBackupsArePruned(t *testing.T) {
	const wantKept = 60
	d := setup(t, map[string]string{"Game.ini": settings, "Game-extra.ini": settings})
	if err := d.Set("Game-extra.ini", "Other", "A", "9", 5); err != nil { // one backup of the other file
		t.Fatal(err)
	}
	for i := 0; i < wantKept+15; i++ {
		if err := d.Set("Game.ini", "Other", "A", fmt.Sprint(i+2), 5); err != nil {
			t.Fatal(err)
		}
	}
	own, other := 0, 0
	ents, _ := os.ReadDir(d.backupDir())
	for _, e := range ents {
		switch {
		case strings.HasPrefix(e.Name(), "Game-extra-"):
			other++
		case strings.HasPrefix(e.Name(), "Game-"):
			own++
		}
	}
	if own != wantKept+1 { // the newest sixty and the oldest
		t.Fatalf("%d backups of Game.ini kept, want %d", own, wantKept+1)
	}
	if other != 1 {
		t.Fatalf("the other file's backup must be left alone: %d", other)
	}
}

// ---- Review R6 (L5): config edits are read back, and the temp files of a crash are swept at start

// The file is read back after the rename: text that is not what was written, or a file that cannot be read, is reported (it used to be silent).
func TestAConfigWriteIsReadBackAndAMismatchIsReported(t *testing.T) {
	d := setup(t, map[string]string{"Game.ini": settings})
	oldRead, oldTries, oldWait := readConfigFile, readBackTries, readBackBackoff
	t.Cleanup(func() { readConfigFile, readBackTries, readBackBackoff = oldRead, oldTries, oldWait })
	readBackBackoff = 0

	readConfigFile = func(string) ([]byte, error) { return []byte("another program wrote this"), nil }
	err := d.Set("Game.ini", "Other", "A", "2", 5)
	if err == nil || !strings.Contains(err.Error(), "other text than was written") || !strings.Contains(err.Error(), "tabr-tau-backups") {
		t.Fatalf("a different text is reported with the place of the backup: %v", err)
	}

	d = setup(t, map[string]string{"Game.ini": settings}) // the file was written with the text above: a fresh one for each case
	calls := 0
	readConfigFile = func(string) ([]byte, error) { calls++; return nil, fmt.Errorf("sharing violation") }
	err = d.Set("Game.ini", "Other", "A", "3", 5)
	if err == nil || !strings.Contains(err.Error(), "could not be read back") || calls != readBackTries {
		t.Fatalf("a file that cannot be read back is reported after %d tries: %v (%d)", readBackTries, err, calls)
	}

	d = setup(t, map[string]string{"Game.ini": settings})
	calls = 0
	readConfigFile = func(p string) ([]byte, error) {
		if calls++; calls < 3 {
			return nil, fmt.Errorf("sharing violation")
		}
		return os.ReadFile(p)
	}
	if err := d.Set("Game.ini", "Other", "A", "4", 5); err != nil {
		t.Fatalf("a read that works on a later try is a success: %v", err)
	}
}

// Temp files that a crash left are removed at start when they are older than the start; a young one (another editor writing now), a file that is not one of ours and a folder are kept.
func TestStaleConfigTempFilesAreSweptAtStartAndOnlyThose(t *testing.T) {
	d := setup(t, map[string]string{
		"Game.ini":                  settings,
		"Game.ini.tabr-123.tmp":     "left by a crash",
		"Engine.ini.tabr-456.tmp":   "left by a crash too",
		"Game.ini.tabr-789.tmp":     "another editor is writing this now",
		"Game.ini.tmp":              "somebody else's file",
		"Notes.tabr-111.tmp":        "not a temp file of a config write",
		"Game.ini.tabr-abc.tmp":     "not digits",
		"Game.ini.tabr-222.tmp.bak": "a backup of someone",
	})
	start := time.Now()
	old := start.Add(-2 * time.Hour)
	for _, n := range []string{"Game.ini.tabr-123.tmp", "Engine.ini.tabr-456.tmp", "Game.ini.tmp", "Notes.tabr-111.tmp", "Game.ini.tabr-abc.tmp", "Game.ini.tabr-222.tmp.bak"} {
		if err := os.Chtimes(filepath.Join(d.Path, n), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(d.Path, "Dir.ini.tabr-333.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(filepath.Join(d.Path, "Dir.ini.tabr-333.tmp"), old, old)
	if n := d.SweepStaleTemps(start); n != 2 {
		t.Fatalf("removed %d, want the 2 old temp files", n)
	}
	for n, want := range map[string]bool{
		"Game.ini.tabr-123.tmp": false, "Engine.ini.tabr-456.tmp": false,
		"Game.ini.tabr-789.tmp": true, "Game.ini": true, "Game.ini.tmp": true, "Notes.tabr-111.tmp": true,
		"Game.ini.tabr-abc.tmp": true, "Game.ini.tabr-222.tmp.bak": true, "Dir.ini.tabr-333.tmp": true,
	} {
		if _, err := os.Stat(filepath.Join(d.Path, n)); (err == nil) != want {
			t.Errorf("%s: exists=%v, want %v", n, err == nil, want)
		}
	}
	if got := read(t, d, "Game.ini"); got != settings {
		t.Fatal("the config file itself is untouched")
	}
	if n := (Dir{Path: filepath.Join(d.Path, "missing")}).SweepStaleTemps(start); n != 0 {
		t.Fatalf("a folder that is not there: %d", n)
	}
}

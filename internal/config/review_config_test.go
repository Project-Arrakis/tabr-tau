package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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

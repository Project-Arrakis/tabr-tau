package gamefolder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPaths(t *testing.T) {
	local := filepath.Join("C:", "Users", "x", "AppData", "Local")
	saved := SavedDir(local)
	for got, want := range map[string]string{
		saved:                      filepath.Join(local, "DuneSandbox", "Saved"),
		ClientStorageDir(saved):    filepath.Join(local, "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage"),
		ConfigDir(saved):           filepath.Join(local, "DuneSandbox", "Saved", "Config", "Windows"),
		LogsDir(saved):             filepath.Join(local, "DuneSandbox", "Saved", "Logs"),
		LogPath(saved):             filepath.Join(local, "DuneSandbox", "Saved", "Logs", "DuneSandbox.log"),
		SavedDir(""):               filepath.Join("DuneSandbox", "Saved"),
		ClientStorageDir("custom"): filepath.Join("custom", "Cloud", "PlayerClientStorage"),
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestLogPathFor(t *testing.T) {
	p := filepath.Join("C:", "Users", "x", "AppData", "Local", "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage", "FLS_retail", "123", "game.db")
	got, err := LogPathFor(p)
	want := filepath.Join("C:", "Users", "x", "AppData", "Local", "DuneSandbox", "Saved", "Logs", "DuneSandbox.log")
	if err != nil || got != want {
		t.Fatalf("%q %v, want %q", got, err, want)
	}
	// the name of the Saved folder is compared without regard to case, and any service folder will do
	lower := filepath.Join("d:", "x", "saved", "Cloud", "PlayerClientStorage", "Other", "9", "game.db")
	if got, err := LogPathFor(lower); err != nil || got != filepath.Join("d:", "x", "saved", "Logs", "DuneSandbox.log") {
		t.Fatalf("%q %v", got, err)
	}
	for _, bad := range []string{
		filepath.Join("tmp", "somewhere", "game.db"),
		filepath.Join("C:", "Saved", "Cloud", "game.db"), // too shallow: Saved is three folders up
		filepath.Join("C:", "Saved", "Cloud", "PlayerClientStorage", "FLS_retail", "1", "deeper", "game.db"),
		"game.db",
		"",
	} {
		if _, err := LogPathFor(bad); err == nil || !strings.Contains(err.Error(), "standard folder") {
			t.Errorf("%q: the log of a save outside the standard tree must be an error, got %v", bad, err)
		}
	}
}

func put(t *testing.T, saved, service, id, file string, mod time.Time) string {
	t.Helper()
	dir := filepath.Join(saved, "Cloud", "PlayerClientStorage", service, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, file)
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSaves(t *testing.T) {
	base := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	saved := t.TempDir()
	if got := Saves(saved, ""); len(got) != 0 {
		t.Fatalf("an empty tree: %v", got)
	}
	a := put(t, saved, "FLS_retail", "2", "game.db", base)
	b := put(t, saved, "FLS_retail", "1", "game.db", base)
	c := put(t, saved, "Other", "1", "game.db", base)
	put(t, saved, "FLS_retail", "1", "game-1.db", base)
	put(t, saved, "FLS_retail", "1", "game_prepatch.db", base)
	if err := os.MkdirAll(filepath.Join(saved, "Cloud", "PlayerClientStorage", "FLS_retail", "1", "sub", "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	put(t, saved, "FLS_retail", "1/sub", "game.db", base) // one folder too deep
	if got, want := strings.Join(Saves(saved, "FLS_retail"), "|"), strings.Join([]string{b, a}, "|"); got != want {
		t.Errorf("the retail service in name order: %s, want %s", got, want)
	}
	if got, want := strings.Join(Saves(saved, ""), "|"), strings.Join([]string{b, a, c}, "|"); got != want {
		t.Errorf("any service: %s, want %s", got, want)
	}
	if got := Saves(saved, "Missing"); len(got) != 0 {
		t.Errorf("a service with no saves: %v", got)
	}
}

func TestNewestSave(t *testing.T) {
	base := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	saved := t.TempDir()
	if _, err := NewestSave(saved); err == nil || err.Error() != "no game.db found under "+saved {
		t.Fatalf("no save: %v", err)
	}
	old := put(t, saved, "FLS_retail", "1001", "game.db", base)
	if got, err := NewestSave(saved); err != nil || got != old {
		t.Fatalf("one save: %q %v", got, err)
	}
	// other files of the tree, another service and another tree are not candidates
	put(t, saved, "FLS_retail", "1001", "game_prepatch.db", base.Add(time.Hour))
	put(t, saved, "OtherService", "1001", "game.db", base.Add(2*time.Hour))
	put(t, filepath.Join(saved, "elsewhere"), "FLS_retail", "1001", "game.db", base.Add(3*time.Hour))
	if got, err := NewestSave(saved); err != nil || got != old {
		t.Fatalf("only FLS_retail/<id>/game.db counts: %q %v", got, err)
	}
	newer := put(t, saved, "FLS_retail", "1002", "game.db", base.Add(time.Minute))
	if got, err := NewestSave(saved); err != nil || got != newer {
		t.Fatalf("the newer of two: %q %v", got, err)
	}
	// a tie goes to the first in name order
	put(t, saved, "FLS_retail", "1003", "game.db", base.Add(time.Minute))
	if got, err := NewestSave(saved); err != nil || got != newer {
		t.Fatalf("a tie: %q %v, want %q", got, err, newer)
	}
	first := put(t, saved, "FLS_retail", "0999", "game.db", base.Add(time.Minute))
	if got, err := NewestSave(saved); err != nil || got != first {
		t.Fatalf("a tie goes to the first name: %q %v, want %q", got, err, first)
	}
	last := put(t, saved, "FLS_retail", "1000", "game.db", base.Add(time.Hour))
	if got, err := NewestSave(saved); err != nil || got != last {
		t.Fatalf("by time, whatever the name: %q %v", got, err)
	}
}

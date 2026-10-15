package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ARCH-10: two routes changed the file by reading it, changing one field and writing it back, so two requests at once lost one
// of the changes; and a file that could not be read was treated as empty, so the next save wiped the other field.

func TestUpdatesFromManyRequestsAllLand(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	const n = 60
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := Update(p, func(s *Settings) error {
				if i%2 == 0 {
					s.AutoRefillOnOpen = true
					return nil
				}
				if s.TrackNumbers == nil {
					s.TrackNumbers = map[string]int64{}
				}
				s.TrackNumbers[fmt.Sprintf("track%d", i)] = int64(i)
				return nil
			})
			if err != nil {
				t.Errorf("update %d: %v", i, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	got, err := LoadStrict(p)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AutoRefillOnOpen || len(got.TrackNumbers) != n/2 {
		t.Fatalf("a change was lost: autoRefill=%v, %d of %d track numbers kept", got.AutoRefillOnOpen, len(got.TrackNumbers), n/2)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(p), "*.tmp")); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestUpdateRefusesADamagedFileAndLeavesItAlone(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"junk.json":  "not json at all",
		"array.json": `[true]`,
		"cut.json":   `{"autoRefillOnOpen": true, "trackNumbers": {"a": 1`,
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		called := false
		_, err := Update(p, func(s *Settings) error { called = true; s.AutoRefillOnOpen = true; return nil })
		if err == nil || !strings.Contains(err.Error(), "damaged") || !strings.Contains(err.Error(), p) {
			t.Errorf("%s: want an error naming the file as damaged, got %v", name, err)
		}
		if called {
			t.Errorf("%s: the change must not even be attempted on a file that could not be read", name)
		}
		if b, _ := os.ReadFile(p); string(b) != content {
			t.Errorf("%s: the damaged file was changed: %q", name, b)
		}
		if _, err := LoadStrict(p); err == nil {
			t.Errorf("%s: LoadStrict must report the damage", name)
		}
	}
}

func TestLoadStrictTellsAbsentFromDamaged(t *testing.T) {
	dir := t.TempDir()
	if s, err := LoadStrict(filepath.Join(dir, "none.json")); err != nil || s.AutoRefillOnOpen {
		t.Fatalf("a missing file is the defaults, not an error: %+v %v", s, err)
	}
	if s, err := LoadStrict(""); err != nil || s.AutoRefillOnOpen {
		t.Fatalf("no path is the defaults: %+v %v", s, err)
	}
	empty := filepath.Join(dir, "empty.json")
	os.WriteFile(empty, []byte(" \n"), 0o600)
	if _, err := LoadStrict(empty); err != nil {
		t.Fatalf("an empty file holds nothing to lose: %v", err)
	}
	big := filepath.Join(dir, "big.json")
	os.WriteFile(big, append([]byte(`{"autoRefillOnOpen": true}`), make([]byte, 70<<10)...), 0o600)
	if _, err := LoadStrict(big); err == nil {
		t.Fatal("an oversized file is reported")
	}
	if _, err := LoadStrict(dir); err == nil { // a folder where the file should be
		t.Fatal("an unreadable path is reported")
	}
	good := filepath.Join(dir, "good.json")
	os.WriteFile(good, []byte(`{"autoRefillOnOpen": true, "trackNumbers": {"a": 3}}`), 0o600)
	if s, err := LoadStrict(good); err != nil || !s.AutoRefillOnOpen || s.TrackNumbers["a"] != 3 {
		t.Fatalf("a good file reads back whole: %+v %v", s, err)
	}
}

func TestUpdateThatFailsWritesNothing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if _, err := Update(p, func(s *Settings) error { s.AutoRefillOnOpen = true; return nil }); err != nil {
		t.Fatal(err)
	}
	want := errors.New("refused")
	if _, err := Update(p, func(s *Settings) error { s.AutoRefillOnOpen = false; return want }); !errors.Is(err, want) {
		t.Fatalf("want the callback's error, got %v", err)
	}
	if got, _ := LoadStrict(p); !got.AutoRefillOnOpen {
		t.Fatal("a refused change must not be written")
	}
	if _, err := Update("", func(s *Settings) error { return nil }); err == nil {
		t.Fatal("no settings path must fail loudly")
	}
}

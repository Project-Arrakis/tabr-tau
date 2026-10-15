package save

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A file over the cap is refused from its size, before it is read: a multi-gigabyte file used to be read in full and only then
// rejected by Decode, and a 256 MB save existed as several copies (SEC-6 of the 2026-10-15 audit).
func TestAnOversizedFileIsRefusedBeforeItIsRead(t *testing.T) {
	withLimit(t, 64<<20)
	p := filepath.Join(t.TempDir(), "game.db")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(96 << 20); err != nil { // 96 MB of zeros, never read
		t.Fatal(err)
	}
	f.Close()

	check := func(name string, call func() error) {
		t.Helper()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		err := call()
		runtime.ReadMemStats(&after)
		if err == nil || !strings.Contains(err.Error(), "over the") {
			t.Fatalf("%s must refuse an oversized file: %v", name, err)
		}
		if got := after.TotalAlloc - before.TotalAlloc; got > 8<<20 {
			t.Fatalf("%s read the file before refusing it: %d bytes allocated", name, got)
		}
	}
	check("readCapped", func() error { _, err := readCapped(p); return err })
	check("Open", func() error { _, err := Open(p); return err })
	check("OpenReadOnly", func() error { _, _, err := OpenReadOnly(p); return err })
}

func TestAFileWithinTheCapIsReadWhole(t *testing.T) {
	withLimit(t, 1<<20)
	p := filepath.Join(t.TempDir(), "x")
	want := []byte(strings.Repeat("ab", 1000))
	if err := os.WriteFile(p, want, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readCapped(p)
	if err != nil || string(got) != string(want) {
		t.Fatalf("%v %d", err, len(got))
	}
	if _, err := readCapped(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("a missing file is an error")
	}
}

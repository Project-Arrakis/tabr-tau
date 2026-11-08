package savefile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A file over the cap is refused from its size, before it is read: a multi-gigabyte file used to be read in full and only then
// rejected by Decode, and a 256 MB save existed as several copies (finding SEC-6 of the editor's 2026-10-15 audit).
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

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = ReadCapped(p)
	runtime.ReadMemStats(&after)
	if err == nil || !strings.Contains(err.Error(), "over the") {
		t.Fatalf("ReadCapped must refuse an oversized file: %v", err)
	}
	if got := after.TotalAlloc - before.TotalAlloc; got > 8<<20 {
		t.Fatalf("ReadCapped read the file before refusing it: %d bytes allocated", got)
	}
}

func TestAFileWithinTheCapIsReadWhole(t *testing.T) {
	withLimit(t, 1<<20)
	p := filepath.Join(t.TempDir(), "x")
	want := []byte(strings.Repeat("ab", 1000))
	if err := os.WriteFile(p, want, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadCapped(p)
	if err != nil || string(got) != string(want) {
		t.Fatalf("%v %d", err, len(got))
	}
	if _, err := ReadCapped(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("a missing file is an error")
	}
}

// A file exactly at the cap is read; one byte more is refused. Without the size check (a file whose size is not reported), the read
// itself stops at the cap plus one byte.
func TestTheCapIsInclusive(t *testing.T) {
	withLimit(t, 1000)
	dir := t.TempDir()
	at, over := filepath.Join(dir, "at"), filepath.Join(dir, "over")
	if err := os.WriteFile(at, make([]byte, 1000), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(over, make([]byte, 1001), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadCapped(at); err != nil || len(b) != 1000 {
		t.Fatalf("a file at the cap must be read: %v %d", err, len(b))
	}
	if _, err := ReadCapped(over); err == nil || !strings.Contains(err.Error(), "over the") {
		t.Fatalf("a file one byte over the cap must be refused: %v", err)
	}
}

// The exported constant and the working variable agree, so a caller that reports Limit reports what the functions enforce.
func TestLimitIs256MiB(t *testing.T) {
	if Limit != 256<<20 || limit != Limit {
		t.Fatalf("Limit = %d, limit = %d", int64(Limit), limit)
	}
}

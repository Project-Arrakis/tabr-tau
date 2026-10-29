package save

import (
	"os"
	"testing"
)

// realSyncFile is the flush to stable storage as shipped (captured before TestMain swaps it). The tests wait for the disk only where
// waiting is what they are about (see realFlush); every other test would pay for a flush it does not look at, and on a hosted Windows
// runner that is slow enough to run the job out of time (#437).
var realSyncFile = syncFile

// realFlush puts the real flush to disk back for the rest of the test. Use it in a test whose subject is that data reaches the disk
// (a durable write, a backup, an atomic replace); a test that only needs a file to exist does not.
func realFlush(t testing.TB) {
	t.Helper()
	old := syncFile
	syncFile = realSyncFile
	t.Cleanup(func() { syncFile = old })
}

// defaultLocationFor is the location check as shipped (captured before TestMain swaps it), so a test can show it is what runs.
var defaultLocationFor = locationFor

// Test saves live in temp folders, which the shipped location check does not trust (it asks for a confirmation); the tests of the
// guard's other rules should not need one. Tests of the location rule put the real check back.
func TestMain(m *testing.M) {
	locationFor = func(string) string { return "" }
	ownSaveFor = func(string) bool { return true } // a test save is the game's own unless the test puts the real rule back (#498)
	syncFile = func(*os.File) error { return nil } // see realSyncFile
	// The tests make decoded copies and sweep the temp folder for stale ones: they do it in a folder of their own, never in the developer's.
	tmp, err := os.MkdirTemp("", "tabr-tau-save-tests-")
	if err == nil {
		for _, k := range []string{"TMP", "TEMP", "TMPDIR"} {
			os.Setenv(k, tmp)
		}
	}
	code := m.Run()
	if err == nil {
		os.RemoveAll(tmp)
	}
	os.Exit(code)
}

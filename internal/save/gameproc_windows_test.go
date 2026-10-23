package save

import (
	"os"
	"path/filepath"
	"strings"
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

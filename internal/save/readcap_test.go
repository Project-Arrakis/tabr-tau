package save

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/kit/savefile"
)

// A file over the cap is refused from its size, before it is read, by the ways the editor opens a save. The cap itself and the read are
// tested in the kit module (kit/savefile); this test is that Open and OpenReadOnly use it: a multi-gigabyte file used to be read in full
// and only then rejected by Decode, and a 256 MB save existed as several copies (SEC-6 of the 2026-10-15 audit).
func TestAnOversizedFileIsRefusedBeforeItIsRead(t *testing.T) {
	p := filepath.Join(t.TempDir(), "game.db")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	// One byte over the module's limit, never read. The file is sparse where the file system allows it.
	if err := f.Truncate(savefile.Limit + 1); err != nil {
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
	check("Open", func() error { _, err := Open(p); return err })
	check("OpenReadOnly", func() error { _, _, err := OpenReadOnly(p); return err })
}

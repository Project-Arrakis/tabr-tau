package ops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// A paid-ground file the next write cannot use is not destroyed (#811): a file of a newer version is left as it is and the write is refused, and any other unusable file is moved aside
// before the new one is written.

func keptNote(house string, paid float64) save.Note {
	return paidNoteOf(7, house, paid)
}

// asideFiles lists the files next to path that hold what was moved aside.
func asideFiles(t *testing.T, path string) []string {
	t.Helper()
	m, err := filepath.Glob(path + ".damaged-*")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPaidAFileOfANewerVersionIsNeverReplacedByARecord(t *testing.T) {
	entry := `{"player":1,"term":7,"house":"Old","faction":2,"paid":6000}`
	for name, content := range map[string]string{
		"same shape":      `{"version":2,"save":"c:\\x\\game.db","entries":[` + entry + `]}`,
		"changed shape":   `{"version":3,"save":"c:\\x\\game.db","entries":{"Old":{"paid":"6000"}}}`,
		"another save":    `{"version":2,"save":"c:\\other\\game.db","entries":[]}`,
		"version as text": `{"version":2,"save":7,"entries":"x"}`,
	} {
		path := filepath.Join(t.TempDir(), "p.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		l := NewPaidLedger(path, `C:\X\game.db`)
		if err := l.Record([]save.Note{keptNote("New", 1200)}); err == nil {
			t.Errorf("%s: a record of a newer version was overwritten without an error", name)
		}
		if after, _ := os.ReadFile(path); string(after) != content {
			t.Errorf("%s: the newer record was changed: %s", name, after)
		}
		if a := asideFiles(t, path); len(a) != 0 {
			t.Errorf("%s: a newer record is left in place, not moved: %v", name, a)
		}
		if _, err := l.snapshot(); !errors.Is(err, errPaidNewerRefused) {
			t.Errorf("%s: a read for a raise says why it is refused: %v", name, err)
		}
	}
}

func TestPaidAnUnusableFileIsMovedAsideBeforeTheNewOneIsWritten(t *testing.T) {
	entry := `"entries":[{"player":1,"term":7,"house":"Old","faction":2,"paid":6000}]`
	mine := `c:\\x\\game.db`
	for name, content := range map[string]string{
		"garbage":         "{not json",
		"truncated":       `{"version":1,"save":"` + mine + `","entries":[{"player":1,"te`,
		"empty":           "",
		"another save":    `{"version":1,"save":"c:\\other\\game.db",` + entry + `}`,
		"an older format": `{"version":0,"save":"` + mine + `",` + entry + `}`,
		"entries not set": `{"version":1,"save":"` + mine + `","entries":7}`,
		"too large":       `{"version":1,"save":"` + mine + `",` + entry + `,"pad":"` + strings.Repeat("x", paidMaxBytes) + `"}`,
	} {
		path := filepath.Join(t.TempDir(), "p.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		l := NewPaidLedger(path, `C:\X\game.db`)
		if err := l.Record([]save.Note{keptNote("New", 1200)}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		a := asideFiles(t, path)
		if len(a) != 1 {
			t.Fatalf("%s: the old file was not kept: %v", name, a)
		}
		if got, _ := os.ReadFile(a[0]); string(got) != content {
			t.Errorf("%s: the copy kept aside is not the old file", name)
		}
		if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].House != "New" || f.Entries[0].Paid != 1200 {
			t.Errorf("%s: the new file does not hold the new note: %+v", name, f)
		}
	}
}

// Two files moved aside in the same instant both stay.
func TestPaidTwoFilesMovedAsideAtTheSameTimeBothStay(t *testing.T) {
	old := paidClock
	paidClock = func() time.Time { return time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC) }
	defer func() { paidClock = old }()
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, `C:\X\game.db`)
	for _, c := range []string{"{first", "{second"} {
		if err := os.WriteFile(path, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := l.Record([]save.Note{keptNote("New", 1200)}); err != nil {
			t.Fatal(err)
		}
	}
	a := asideFiles(t, path)
	if len(a) != 2 {
		t.Fatalf("both files are kept: %v", a)
	}
	var seen []string
	for _, f := range a {
		b, _ := os.ReadFile(f)
		seen = append(seen, string(b))
	}
	if !(strings.Contains(strings.Join(seen, "|"), "{first") && strings.Contains(strings.Join(seen, "|"), "{second")) {
		t.Fatalf("what was kept: %v", seen)
	}
}

// A raise with a record of a newer version is refused with nothing written, and the record is left alone; a tier removal needs no record and still goes through.
func TestPaidARaiseIsRefusedWhileTheRecordIsOfANewerVersion(t *testing.T) {
	o, path := paidOps(t, "")
	content := `{"version":2,"save":"x","entries":[{"player":1,"term":7,"house":"Other","faction":2,"paid":6000}]}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	solari := wallet(t, o, 0)
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	if !errors.Is(err, apierr.ErrWriteFailed) || !strings.Contains(err.Error(), "newer version") || !strings.Contains(err.Error(), "nothing was written") {
		t.Fatalf("the raise is refused with the reason: %v", err)
	}
	if strings.Contains(err.Error(), os.TempDir()) {
		t.Fatalf("the refusal names a folder: %v", err)
	}
	if got := wallet(t, o, 0); got != solari {
		t.Fatalf("a refused raise paid: %d", got)
	}
	if o.S.Flush().Queued != 0 {
		t.Fatal("a refused raise left an edit queued")
	}
	if after, _ := os.ReadFile(path); string(after) != content {
		t.Fatalf("the newer record was changed: %s", after)
	}
	if a := asideFiles(t, path); len(a) != 0 {
		t.Fatalf("a newer record was moved: %v", a)
	}
}

// A damaged record does not stop a raise (it reads as nothing paid, as before), but the write that follows keeps the damaged file aside instead of destroying it.
func TestPaidARaiseOverADamagedRecordKeepsTheDamagedFile(t *testing.T) {
	o, path := paidOps(t, "")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	a := asideFiles(t, path)
	if len(a) != 1 {
		t.Fatalf("the damaged file was not kept: %v", a)
	}
	if got, _ := os.ReadFile(a[0]); string(got) != "{not json" {
		t.Fatalf("what was kept: %q", got)
	}
	if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].Paid != 6000 {
		t.Fatalf("the new record: %+v", f)
	}
}

package ops

import (
	"errors"
	"fmt"
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
		"same shape":    `{"version":2,"save":"c:\\x\\game.db","entries":[` + entry + `]}`,
		"changed shape": `{"version":3,"save":"c:\\x\\game.db","entries":{"Old":{"paid":"6000"}}}`,
		"another save":  `{"version":2,"save":"c:\\other\\game.db","entries":[]}`,
		"changed types": `{"version":2,"save":7,"entries":"x"}`,
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

// The version is read from the one field and read strictly: a version that is there and is not a whole number at or below the one this build writes is a newer record, whatever its
// form, and a file that is larger than the cap is probed first. Each of these is left in place byte for byte, a record is refused with an error, a read for a raise says why, and nothing is
// moved aside (#811).
func TestPaidAVersionThatIsNotAWholeNumberAtOrBelowOneIsNeverReplaced(t *testing.T) {
	mine := `c:\\x\\game.db`
	body := `"save":"` + mine + `","entries":[{"player":1,"term":7,"house":"Old","faction":2,"paid":6000}]`
	pad := strings.Repeat("x", paidMaxBytes)
	for name, content := range map[string]string{
		"text":                    `{"version":"2",` + body + `}`,
		"text of the current":     `{"version":"1",` + body + `}`,
		"2.0":                     `{"version":2.0,` + body + `}`,
		"2e0":                     `{"version":2e0,` + body + `}`,
		"1.5":                     `{"version":1.5,` + body + `}`,
		"1.0":                     `{"version":1.0,` + body + `}`,
		"too large for a number":  `{"version":99999999999999999999,` + body + `}`,
		"max int64":               `{"version":9223372036854775807,` + body + `}`,
		"null":                    `{"version":null,` + body + `}`,
		"a list":                  `{"version":[2],` + body + `}`,
		"upper case name":         `{"VERSION":2,` + body + `}`,
		"mixed case name, text":   `{"Version":"2",` + body + `}`,
		"duplicate ending in 2":   `{"version":1,"version":2,` + body + `}`,
		"trailing newline":        `{"version":2,` + body + "}\n",
		"over the cap, 2":         `{"version":2,` + body + `,"pad":"` + pad + `"}`,
		"over the cap, text":      `{"version":"2",` + body + `,"pad":"` + pad + `"}`,
		"over the cap, 99999...9": `{"version":99999999999999999999,"pad":"` + pad + `"}`,
		"over the cap, 2.0":       `{"version":2.0,"pad":"` + pad + `"}`,
	} {
		path := filepath.Join(t.TempDir(), "p.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		l := NewPaidLedger(path, `C:\X\game.db`)
		err := l.Record([]save.Note{keptNote("New", 1200)})
		if err == nil || !errors.Is(err, errPaidNewer) {
			t.Errorf("%s: a record over a newer file is refused with the reason: %v", name, err)
		}
		if after, _ := os.ReadFile(path); string(after) != content {
			t.Errorf("%s: the newer record was changed", name)
		}
		if a := asideFiles(t, path); len(a) != 0 {
			t.Errorf("%s: a newer record is left in place, not moved: %v", name, a)
		}
		if _, err := l.snapshot(); !errors.Is(err, errPaidNewerRefused) {
			t.Errorf("%s: a read for a raise says why it is refused: %v", name, err)
		}
	}
}

// A duplicate version key reads as the last one, as the decoder does: one ending in 1 is this build's own format.
func TestPaidADuplicateVersionEndingInOneReadsAsTheCurrentFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	content := `{"version":2,"version":1,"save":"c:\\x\\game.db","entries":[{"player":1,"term":7,"house":"Old","faction":2,"paid":6000}]}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	l := NewPaidLedger(path, `C:\X\game.db`)
	if es, err := l.snapshot(); err != nil || len(es) != 1 {
		t.Fatalf("the file reads as version 1: %v %v", es, err)
	}
	if err := l.Record([]save.Note{keptNote("New", 1200)}); err != nil {
		t.Fatal(err)
	}
	if f := readPaid(t, path); len(f.Entries) != 2 {
		t.Fatalf("the old entry and the new one: %+v", f)
	}
	if a := asideFiles(t, path); len(a) != 0 {
		t.Fatalf("a usable file was moved: %v", a)
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
		// no version that can be read: absent, negative, past the cap, or a file that is not a whole object
		"no version":                  `{"save":"` + mine + `",` + entry + `}`,
		"negative version":            `{"version":-1,"save":"` + mine + `",` + entry + `}`,
		"too large, version past cap": `{"pad":"` + strings.Repeat("x", paidMaxBytes) + `","version":2,"save":"` + mine + `",` + entry + `}`,
		"byte order mark":             "\ufeff" + `{"version":1,"save":"` + mine + `",` + entry + `}`,
		"trailing garbage":            `{"version":1,"save":"` + mine + `",` + entry + `}x`,
		"newer with trailing garbage": `{"version":2,"save":"` + mine + `",` + entry + `}x`,
		"array wrapper":               `[{"version":1,"save":"` + mine + `",` + entry + `}]`,
		"null":                        `null`,
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
	// the board is still read
	if taskByID(t, o, 175) == nil {
		t.Fatal("the board read does not show the task")
	}
	// a tier removal needs no record: it goes through, flushes, and the newer record is still byte for byte as it was
	if m := applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 0.0, "lower": true})); m["contribution_lowered"].(int64) != 1 {
		t.Fatalf("the removal did not lower the contribution: %v", m)
	}
	flush(t, o)
	if after, _ := os.ReadFile(path); string(after) != content {
		t.Fatalf("the newer record was changed by a removal: %s", after)
	}
	if a := asideFiles(t, path); len(a) != 0 || o.S.NotesProblem() != "" {
		t.Fatalf("after the removal: moved %v, problem %q", a, o.S.NotesProblem())
	}
	// and a raise is still refused
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 0.0, "mine_to": 6000.0}))); !errors.Is(err, apierr.ErrWriteFailed) {
		t.Fatalf("a raise after the removal is refused: %v", err)
	}
}

// A damaged record that cannot be moved aside is not replaced: the write is an error, the file stays as it is, and the save keeps the notes for the next write (#811). Here every name the
// file could be kept under is taken.
func TestPaidAWriteThatCannotMoveTheDamagedFileAsideIsAnErrorAndTheSaveKeepsTheNotes(t *testing.T) {
	old := paidClock
	at := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	paidClock = func() time.Time { return at }
	defer func() { paidClock = old }()
	o, path := paidOps(t, "")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := at.Format("20060102T150405.000000")
	taken := []string{path + ".damaged-" + stamp}
	for i := 1; i <= 99; i++ {
		taken = append(taken, fmt.Sprintf("%s.damaged-%s-%d", path, stamp, i))
	}
	for _, f := range taken {
		if err := os.WriteFile(f, []byte("taken"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := NewPaidLedger(path, o.S.Path).Record([]save.Note{keptNote("New", 1200)}); err == nil || !strings.Contains(err.Error(), "could not be kept aside") {
		t.Fatalf("a file that cannot be moved aside is an error: %v", err)
	}
	fillKenolaDoubled(t, o, 1300, 6000)
	flushWithWarning(t, o) // the game's file is written, the record is not
	if b, _ := os.ReadFile(path); string(b) != "{not json" {
		t.Fatalf("the damaged file was replaced: %q", b)
	}
	// a free name again: the next write hands over the kept note, keeps the damaged file aside and writes the record
	for _, f := range taken {
		if err := os.Remove(f); err != nil {
			t.Fatal(err)
		}
	}
	removeTo(t, o, 6000, 0)
	flush(t, o)
	if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].Paid != 6000 || o.S.NotesProblem() != "" {
		t.Fatalf("the kept note was not recorded: %+v, %q", f, o.S.NotesProblem())
	}
	if a := asideFiles(t, path); len(a) != 1 {
		t.Fatalf("the damaged file was not kept: %v", a)
	} else if b, _ := os.ReadFile(a[0]); string(b) != "{not json" {
		t.Fatalf("what was kept: %q", b)
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

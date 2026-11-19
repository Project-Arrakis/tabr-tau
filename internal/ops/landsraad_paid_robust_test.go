package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The record file's own limits and the way it is kept: the size cap derived from the entry bound (#838, #840), a version cut by the read cap (#840), a version that costs nothing to judge
// (#840), the migration that must come before a version bump (#838), and the file moved aside by the durable rename, kept in a bounded number and told to the person (#837, #838).

// The cap is what the largest legal file comes to: 4,096 entries whose house names JSON escapes to 6 bytes a byte, with numbers of the longest form, and the longest save path.
func TestPaidTheSizeCapHoldsTheLargestFileTheEditorCanWrite(t *testing.T) {
	floor := 0.000012345678901234567
	worst := paidNote{Player: 9223372036854775807, Term: 9223372036854775807, House: strings.Repeat("<", paidMaxHouseLen), Faction: -9223372036854775808,
		Paid: 0.000012345678901234567, Back: 0.000012345678901234567, Floor: &floor}
	if !worst.valid() {
		t.Fatal("the worst entry is not a valid one, so it says nothing")
	}
	one, err := json.Marshal(worst)
	if err != nil {
		t.Fatal(err)
	}
	if len(one) > paidMaxEntryBytes {
		t.Fatalf("an entry of the worst case is %d bytes, above the %d the cap counts on: %s", len(one), paidMaxEntryBytes, one)
	}
	if !strings.Contains(string(one), `\u003c`) {
		t.Fatalf("the worst case has to be one that JSON escapes: %s", one)
	}
	es := make([]paidNote, paidMaxEntries)
	for i := range es {
		es[i] = worst
		es[i].Player = int64(9223372036854775807 - i) // distinct, all of the longest form
	}
	savePath := strings.Repeat("<", 32767) // a save path of the most characters Windows allows, each escaped to six bytes
	b, err := json.Marshal(paidFile{Version: paidFileVersion, Save: savePath, Entries: es})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > paidMaxBytes {
		t.Fatalf("a file of the most entries of the worst case is %d bytes, above the cap of %d: a file the editor wrote could read as too large", len(b), paidMaxBytes)
	}
	// and it is read back whole, not as too large
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, "x")
	l.save = savePath
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := l.snapshot()
	if err != nil || len(got) != paidMaxEntries {
		t.Fatalf("a file at the largest size the editor writes is read: %d entries, %v", len(got), err)
	}
}

// A file the editor itself is about to write above the cap is an error, never written: Record would otherwise make a file that the next read calls too large.
func TestPaidAWriteAboveTheCapIsAnErrorAndWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, strings.Repeat("a", paidMaxBytes)) // a save path as long as the cap: only here is the file larger than the cap
	err := l.Record([]save.Note{paidNoteOf(7, "A", 1200)})
	if err == nil || !strings.Contains(err.Error(), "more than the") {
		t.Fatalf("a file above the cap is not written: %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("a file above the cap was written")
	}
}

// The version key straddles the byte after the cap: every cut of "version":10 from inside the key to the end of the value is a newer record, and none that ends before the key or after the
// value is (#840).
func TestPaidAVersionThatTheReadCapCutsIsNewerAtEveryByte(t *testing.T) {
	for _, tail := range []string{`10`, `1`, `1.5`, `"2"`, `null`, `[2]`, `99999999999999999999`, `2e0`, `{"a":1}`, `true`} {
		key := `"version":`
		text := `{"pad":"x",` + key + tail
		start := strings.Index(text, key)
		for n := 3; n <= len(text); n++ { // (a window of `{"` alone may be the start of any key)
			_, _, cut := paidVersionOf([]byte(text[:n]), true)
			inside := n > start && n <= len(text) // from one byte into the key to the end of the value
			if cut != inside {
				t.Errorf("%s: cut after %d bytes (%q): cut=%v, want %v", tail, n, text[max(0, n-14):n], cut, inside)
			}
		}
		// the value is whole and the next byte is in: not cut. A file that is not capped never reads as cut, whatever it ends with
		_, ok, cut := paidVersionOf([]byte(text+`,"z":1}`), true)
		if cut || !ok {
			t.Errorf("%s: a version followed by more is not cut: ok=%v cut=%v", tail, ok, cut)
		}
		if _, _, cut := paidVersionOf([]byte(text), false); cut {
			t.Errorf("%s: a file that is not capped has no cut", tail)
		}
	}
	// a key cut in two that may be "version" (any letter case, an escape, or a byte that is not ASCII), and one that cannot be
	for part, want := range map[string]bool{`"`: true, `"v`: true, `"VERS`: true, `"version`: true, `"ver\`: true, `"ver\u00`: true, "\"v\u00e9": true, `"versions`: false, `"ve1`: false, `"pad`: false, ``: false} {
		if _, _, cut := paidVersionOf([]byte(`{"a":1,`+part), true); cut != want {
			t.Errorf("a cut inside the key %q: cut=%v, want %v", part, cut, want)
		}
	}
	// a key that is not "version" with its value cut is no version
	if _, ok, cut := paidVersionOf([]byte(`{"entries":[1,2`), true); ok || cut {
		t.Errorf("a cut inside another key's value: ok=%v cut=%v", ok, cut)
	}
}

// End to end through the file: a record larger than the cap whose version is cut by it is never replaced and never moved aside, wherever in the key or the value the cap falls; one whose
// version lies wholly before the cap, or wholly after it, is read as it was before.
func TestPaidAnOverCapFileWithACutVersionIsKeptAsItIs(t *testing.T) {
	t.Parallel() // #767: state-free and slow, so it runs beside the others (its save, its record file and its ops are its own)
	fill := func(n int) string { return strings.Repeat("x", n) }
	build := func(tail string, into int) string { // the cap falls `into` bytes into `"version":<tail>`
		head, key := `{"pad":"`, `"version":`
		pad := paidMaxBytes + 1 - into - len(head) - len(`",`)
		return head + fill(pad) + `",` + key + tail + `,"save":"c:\\x\\game.db","entries":[]}`
	}
	for _, tc := range []struct {
		tail  string
		into  int
		newer bool
	}{
		{"10", 1, true}, {"10", 5, true}, {"10", 9, true}, {"10", 10, true}, {"10", 11, true}, {"10", 12, true}, // inside the key; the colon; before the value; after 1 (it may go on); whole
		{"1", 10, true}, {"1", 11, true}, // the value is 1, and ends where the bytes end: it may be 10
		{"1.5", 11, true}, {`"2"`, 11, true}, {"null", 12, true}, {"[2]", 12, true},
		{"1", 12, false}, // the comma after 1 is read: version 1, and the file is only too large
		{"10", 13, true}, // the comma after 10 is read: a version of the next format that is whole
		{"1", 0, false},  // the cap falls before the key: no version is seen
	} {
		content := build(tc.tail, tc.into)
		path := filepath.Join(t.TempDir(), "p.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		l := NewPaidLedger(path, `C:\X\game.db`)
		err := l.Record([]save.Note{keptNote("New", 1200)})
		name := fmt.Sprintf("version %s, the cap %d bytes in", tc.tail, tc.into)
		wantNewer := tc.newer
		if wantNewer != errors.Is(err, errPaidNewer) {
			t.Errorf("%s: newer=%v, want %v (%v)", name, errors.Is(err, errPaidNewer), wantNewer, err)
		}
		after, _ := os.ReadFile(path)
		if wantNewer {
			if string(after) != content || len(asideFiles(t, path)) != 0 {
				t.Errorf("%s: a record of a version that is not known was changed or moved", name)
			}
			if _, err := l.snapshot(); !errors.Is(err, errPaidNewerRefused) {
				t.Errorf("%s: a raise is not refused: %v", name, err)
			}
		} else if err != nil || len(asideFiles(t, path)) != 1 {
			t.Errorf("%s: a file that is only too large is moved aside and a new one written: %v %v", name, err, asideFiles(t, path))
		}
	}
}

// A version of a million digits costs one pass and no number: the old judgement built a big number (about a second and gigabytes of allocation per read).
func TestPaidAHugeVersionCostsNothingToJudge(t *testing.T) {
	huge := json.RawMessage(strings.Repeat("9", 1_000_000))
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	start := time.Now()
	if !paidVersionIsNewer(huge) {
		t.Fatal("a huge version is newer")
	}
	runtime.ReadMemStats(&m1)
	if alloc := m1.TotalAlloc - m0.TotalAlloc; alloc > 16<<20 {
		t.Fatalf("judging a version of a million digits allocated %d bytes", alloc)
	}
	if d := time.Since(start); d > 700*time.Millisecond {
		t.Fatalf("judging a version of a million digits took %v", d)
	}
}

func TestPaidAVersionIsJudgedByItsDigits(t *testing.T) {
	for v, want := range map[string]bool{
		"0": false, "1": false, "-0": false, "-1": false, "-99999999999999999999": false, "2": true, "10": true, "99999999999999999999": true,
		"1.0": true, "1.5": true, "2e0": true, "1e0": true, `"1"`: true, `null`: true, `[1]`: true, `true`: true, "": true, "-": true, " 1 ": false, "00": false, "01": false, "02": true,
		"000000000000000000000000000000000000000000000001": false, "0000000000000000000000000000000000000000000000002": true,
	} {
		if got := paidVersionIsNewer(json.RawMessage(v)); got != want {
			t.Errorf("version %q: newer=%v, want %v", v, got, want)
		}
	}
}

// The migration steps (#838): the day paidFileVersion becomes 2, a record of version 1 is carried over, not moved aside as damaged and paid again.
func TestPaidEveryOlderVersionHasAStep(t *testing.T) {
	for v := 1; v < paidFileVersion; v++ {
		if paidMigrations[v] == nil {
			t.Errorf("paidFileVersion is %d and there is no step from version %d: a record of that version would be refused", paidFileVersion, v)
		}
	}
}

func TestPaidMigrationRunsOneVersionAtATimeAndNeverFallsBackToDamaged(t *testing.T) {
	up := func(f func(paidFile) paidFile) func(paidFile) (paidFile, error) {
		return func(pf paidFile) (paidFile, error) { return f(pf), nil }
	}
	steps := map[int]func(paidFile) (paidFile, error){
		1: up(func(pf paidFile) paidFile {
			pf.Version = 2
			for i := range pf.Entries {
				pf.Entries[i].Paid *= 2
			}
			return pf
		}),
		2: up(func(pf paidFile) paidFile { pf.Version = 3; return pf }),
	}
	in := paidFile{Version: 1, Save: "x", Entries: []paidNote{{Player: 1, Term: 7, House: "A", Faction: 2, Paid: 600}}}
	out, err := migratePaid(in, 3, steps)
	if err != nil || out.Version != 3 || out.Entries[0].Paid != 1200 {
		t.Fatalf("1 to 3 through both steps: %+v, %v", out, err)
	}
	if out, err := migratePaid(paidFile{Version: 3}, 3, steps); err != nil || out.Version != 3 {
		t.Fatalf("a record of the current version is not touched: %+v %v", out, err)
	}
	// no step: not read, not replaced (it wraps errPaidNewer, so the read, the raise and the write all leave the file as it is)
	if _, err := migratePaid(in, 3, map[int]func(paidFile) (paidFile, error){1: steps[1]}); !errors.Is(err, errPaidNewer) {
		t.Fatalf("a version with no step is refused as a newer one: %v", err)
	}
	if _, err := migratePaid(in, 2, nil); !errors.Is(err, errPaidNewer) {
		t.Fatalf("no steps at all: %v", err)
	}
	// a step that fails, or that does not move the version by one
	if _, err := migratePaid(in, 2, map[int]func(paidFile) (paidFile, error){1: func(paidFile) (paidFile, error) { return paidFile{}, errors.New("bad entry") }}); !errors.Is(err, errPaidNewer) {
		t.Fatalf("a step that fails: %v", err)
	}
	for _, to := range []int{1, 3, 0} {
		if _, err := migratePaid(in, 2, map[int]func(paidFile) (paidFile, error){1: up(func(pf paidFile) paidFile { pf.Version = to; return pf })}); !errors.Is(err, errPaidNewer) {
			t.Fatalf("a step to version %d: %v", to, err)
		}
	}
}

// What read does with the versions around the current one: version 0 and a negative one are damaged (moved aside), the current one is read, and a version above it is refused.
func TestPaidTheVersionsAroundTheCurrentOne(t *testing.T) {
	mine := `c:\\x\\game.db`
	entry := `"entries":[{"player":1,"term":7,"house":"Old","faction":2,"paid":6000}]`
	for v, want := range map[string]string{"0": "damaged", "-1": "damaged", "1": "read", "2": "newer", "99": "newer"} {
		path := filepath.Join(t.TempDir(), "p.json")
		if err := os.WriteFile(path, []byte(`{"version":`+v+`,"save":"`+mine+`",`+entry+`}`), 0o600); err != nil {
			t.Fatal(err)
		}
		l := NewPaidLedger(path, `C:\X\game.db`)
		es, err := l.snapshot()
		switch want {
		case "read":
			if err != nil || len(es) != 1 {
				t.Errorf("version %s: %v %v", v, es, err)
			}
		case "damaged":
			if err != nil || len(es) != 0 {
				t.Errorf("version %s reads as nothing paid: %v %v", v, es, err)
			}
			if err := l.Record([]save.Note{keptNote("New", 1200)}); err != nil || len(asideFiles(t, path)) != 1 {
				t.Errorf("version %s is moved aside: %v %v", v, err, asideFiles(t, path))
			}
		case "newer":
			if !errors.Is(err, errPaidNewerRefused) {
				t.Errorf("version %s is refused: %v", v, err)
			}
		}
	}
}

// The move aside waits for a program that holds the file for a moment, as the write of the record does (#837): a rename that fails twice and then goes through still keeps the file.
func TestPaidMovingTheDamagedFileAsideWaitsForAProgramThatHoldsIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := NewPaidLedger(path, `C:\X\game.db`)
	l.opts.Pause = 0
	tries := 0
	l.opts.Hooks.Rename = func(from, to string) error {
		if from == path {
			if tries++; tries <= 2 {
				return errors.New("the file is used by another process")
			}
		}
		return os.Rename(from, to)
	}
	if err := l.Record([]save.Note{keptNote("New", 1200)}); err != nil {
		t.Fatalf("a rename that goes through at the third try is not an error: %v", err)
	}
	a := asideFiles(t, path)
	if tries != 3 || len(a) != 1 {
		t.Fatalf("tries %d, kept %v", tries, a)
	}
	if b, _ := os.ReadFile(a[0]); string(b) != "{not json" {
		t.Fatalf("what was kept: %q", b)
	}
}

// A file that is held for good is not replaced: the write is an error, the file is as it was, and the note stays for the next write.
func TestPaidAFileHeldForGoodIsNotReplaced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := NewPaidLedger(path, `C:\X\game.db`)
	l.opts.Pause = 0
	l.opts.Hooks.Rename = func(from, to string) error { return errors.New("the file is used by another process") }
	err := l.Record([]save.Note{keptNote("New", 1200)})
	if err == nil || !strings.Contains(err.Error(), "could not be kept aside") {
		t.Fatalf("a file that cannot be moved is an error: %v", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "{not json" || len(asideFiles(t, path)) != 0 {
		t.Fatalf("the file was changed: %q %v", b, asideFiles(t, path))
	}
	if l.Notice() != "" {
		t.Fatalf("nothing was moved, so there is nothing to tell: %q", l.Notice())
	}
}

// A file that is gone by the time it is to be moved is nothing to keep: the write goes on and says nothing about a file.
func TestPaidAFileThatIsGoneIsNothingToMoveAside(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, `C:\X\game.db`)
	kept, err := l.moveAside()
	if err != nil || kept != "" || l.Notice() != "" {
		t.Fatalf("a missing file: %q %v %q", kept, err, l.Notice())
	}
}

// Only the newest paidKeepAside copies stay; the files next to them that are not copies are left alone; the copy just made is always there (#838).
func TestPaidTheOldestCopiesMovedAsideAreRemoved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.json")
	other := []string{filepath.Join(dir, "p.json.old"), filepath.Join(dir, "q.json.damaged-20200101T000000.000000"), filepath.Join(dir, "p.json.damaged")}
	for _, f := range other {
		if err := os.WriteFile(f, []byte("mine"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := paidClock
	defer func() { paidClock = old }()
	n := 0
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	paidClock = func() time.Time { n++; return at.Add(time.Duration(n) * time.Second) }
	l := NewPaidLedger(path, `C:\X\game.db`)
	for i := 0; i < paidKeepAside+5; i++ {
		if err := os.WriteFile(path, []byte(fmt.Sprintf("{broken %d", i)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := l.Record([]save.Note{keptNote("New", 1200)}); err != nil {
			t.Fatal(err)
		}
	}
	got := asideFiles(t, path)
	if len(got) != paidKeepAside {
		t.Fatalf("%d copies kept, want %d: %v", len(got), paidKeepAside, got)
	}
	for i := 5; i < paidKeepAside+5; i++ { // the copies of the last paidKeepAside files
		found := false
		for _, g := range got {
			if b, _ := os.ReadFile(g); string(b) == fmt.Sprintf("{broken %d", i) {
				found = true
			}
		}
		if !found {
			t.Errorf("the copy of file %d (one of the newest) was removed", i)
		}
	}
	for _, f := range other {
		if b, err := os.ReadFile(f); err != nil || string(b) != "mine" {
			t.Errorf("%s: a file that is not a copy was touched: %q %v", filepath.Base(f), b, err)
		}
	}
}

// The person is told which file holds the old record, by name and not by folder, and only when one was moved (#838).
func TestPaidAFileMovedAsideIsNamedToThePerson(t *testing.T) {
	o, path := paidOps(t, "")
	if o.Paid.Notice() != "" {
		t.Fatalf("a notice before anything happened: %q", o.Paid.Notice())
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	a := asideFiles(t, path)
	if len(a) != 1 {
		t.Fatalf("the damaged file was not kept: %v", a)
	}
	notice := o.Paid.Notice()
	if !strings.Contains(notice, filepath.Base(a[0])) || strings.Contains(notice, filepath.Dir(path)) || !strings.Contains(notice, "kept as") {
		t.Fatalf("the notice names the copy by its name only: %q", notice)
	}
	st, err := o.State()
	if err != nil {
		t.Fatal(err)
	}
	if got := st.(map[string]any)["recordNotice"]; got != notice {
		t.Fatalf("the state of the page carries the notice: %v", got)
	}
	// a usable record moves nothing and says nothing
	o2, _ := paidOps(t, "")
	fillKenolaDoubled(t, o2, 1300, 6000)
	flush(t, o2)
	if st2, _ := o2.State(); st2.(map[string]any)["recordNotice"] != "" {
		t.Fatalf("a usable record has a notice: %v", st2.(map[string]any)["recordNotice"])
	}
	var nilLedger *PaidLedger
	if nilLedger.Notice() != "" {
		t.Fatal("a ledger that is not there has no notice")
	}
}

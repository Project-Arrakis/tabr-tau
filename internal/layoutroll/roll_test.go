package layoutroll

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Every test works in a temp folder; none reads or writes a real Game.ini or a real settings folder.

type env struct {
	t       *testing.T
	dir     string
	state   string
	now     time.Time
	running bool
	checks  int
	pick    int // the number the fake random source returns
}

func newEnv(t *testing.T, ini string) *env {
	t.Helper()
	d := t.TempDir()
	e := &env{t: t, dir: filepath.Join(d, "Windows"), state: filepath.Join(d, "cfg", "layout-roll.json"),
		now: time.Date(2026, 11, 24, 5, 0, 30, 0, time.Local)} // a Tuesday, just after 05:00
	if err := os.Mkdir(e.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if ini != "" {
		e.write(ini)
	}
	return e
}

func (e *env) write(s string) {
	e.t.Helper()
	if err := os.WriteFile(filepath.Join(e.dir, FileName), []byte(s), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) ini() string {
	e.t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dir, FileName))
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b)
}

func (e *env) opts() Options {
	return Options{
		ConfigDir: e.dir, StatePath: e.state,
		Now:  func() time.Time { return e.now },
		Intn: func(n int) int { return e.pick % n },
		GameRunning: func() (bool, string) {
			e.checks++
			if e.running {
				return true, "Dune.exe is running"
			}
			return false, ""
		},
	}
}

func (e *env) backups() []string {
	ents, _ := os.ReadDir(e.dir)
	var out []string
	for _, x := range ents {
		if strings.HasPrefix(x.Name(), backupPrefix) {
			out = append(out, x.Name())
		}
	}
	return out
}

func (e *env) files() map[string]string {
	m := map[string]string{}
	ents, _ := os.ReadDir(e.dir)
	for _, x := range ents {
		b, _ := os.ReadFile(filepath.Join(e.dir, x.Name()))
		m[x.Name()] = string(b)
	}
	return m
}

func ptr(i int) *int { return &i }

const baseIni = "[Settings.Audio]\r\nVolume=5\r\n\r\n" + sec + "\r\n"

func iniWith(n string) string { return baseIni + lineWith(n) + "\r\n[Tail]\r\nB=3\r\n" }

func TestARollChangesOnlyTheNumberAfterABackupAndRecordsIt(t *testing.T) {
	old := iniWith("11")
	e := newEnv(t, old)
	e.pick = 0 // the first candidate that is not 11 is 0
	r, err := Roll(e.opts())
	if err != nil {
		t.Fatal(err)
	}
	if !r.Wrote || r.Old != 11 || r.New != 0 {
		t.Fatalf("%+v", r)
	}
	if got, want := e.ini(), strings.Replace(old, "=11)))", "=0)))", 1); got != want {
		t.Fatalf("Game.ini is not the old text with only the number changed:\n%q", got)
	}
	bs := e.backups()
	if len(bs) != 1 {
		t.Fatalf("backups: %v", bs)
	}
	if b, _ := os.ReadFile(filepath.Join(e.dir, bs[0])); string(b) != old {
		t.Fatal("the backup is not the old file, byte for byte")
	}
	if ls := StateLinesForTest(t, e.state); !strings.Contains(ls, `"from": 11`) || !strings.Contains(ls, `"to": 0`) {
		t.Fatalf("state: %s", ls)
	}
	if e.checks < 2 {
		t.Fatal("the process list was not asked again right before the write")
	}
	// no temp files are left behind
	for n := range e.files() {
		if strings.HasSuffix(n, ".tmp") {
			t.Fatalf("leftover %s", n)
		}
	}
}

func StateLinesForTest(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestARollNeverPicksTheCurrentLayout(t *testing.T) {
	for cur := 0; cur <= MaxIndex; cur++ {
		for pick := 0; pick < 12; pick++ {
			e := newEnv(t, iniWith(strconv.Itoa(cur)))
			e.pick = pick
			r, err := Roll(e.opts())
			if err != nil || r.New == cur || r.New < MinIndex || r.New > MaxIndex {
				t.Fatalf("current %d pick %d: %+v %v", cur, pick, r, err)
			}
		}
	}
}

func TestNoLineMeansTheGameDefaultIsCurrentAndTheLineIsAdded(t *testing.T) {
	old := "[Settings.Audio]\r\nVolume=5\r\n"
	e := newEnv(t, old)
	e.pick = 10 // candidates 0..10, so 10
	r, err := Roll(e.opts())
	if err != nil || r.Old != DefaultIndex || r.New != 10 {
		t.Fatalf("%+v %v", r, err)
	}
	if got := e.ini(); !strings.HasPrefix(got, old+"\r\n"+sec+"\r\n"+Key+"=") {
		t.Fatalf("the old text must stay first, the section added after it: %q", got[:80])
	}
	if len(e.backups()) != 1 {
		t.Fatal("no backup")
	}
}

func TestAMissingGameIniIsCreatedWithoutABackup(t *testing.T) {
	e := newEnv(t, "")
	if _, err := Roll(e.opts()); err != nil {
		t.Fatal(err)
	}
	if got := e.ini(); !strings.HasPrefix(got, sec+"\r\n"+Key+"=") {
		t.Fatalf("%q", got[:60])
	}
	if len(e.backups()) != 0 {
		t.Fatal("a backup of nothing was made")
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	before := e.files()
	o := e.opts()
	o.DryRun = true
	r, err := Roll(o)
	if err != nil || r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	if !strings.Contains(strings.Join(r.Lines, "\n"), "Dry run: would change Game.ini from layout 3 to layout") {
		t.Fatalf("%v", r.Lines)
	}
	after := e.files()
	if len(after) != len(before) || after[FileName] != old {
		t.Fatalf("a dry run changed the folder: %v", after)
	}
	if _, err := os.Stat(e.state); err == nil {
		t.Fatal("a dry run wrote the state file")
	}
	if _, err := os.Stat(filepath.Dir(e.state)); err == nil {
		t.Fatal("a dry run made the state folder")
	}
	// a dry run while the game runs writes nothing either, and says so
	e.running = true
	r, err = Roll(o)
	if err != nil || !strings.Contains(strings.Join(r.Lines, "\n"), "it would refuse") {
		t.Fatalf("%v %v", r.Lines, err)
	}
	if _, err := os.Stat(e.state); err == nil {
		t.Fatal("a dry run wrote the state file")
	}
}

func TestItRefusesWhileTheGameRuns(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	e.running = true
	r, err := Roll(e.opts())
	if err == nil || r.Wrote || !strings.Contains(err.Error(), "Dune.exe is running") {
		t.Fatalf("%+v %v", r, err)
	}
	if e.ini() != old || len(e.backups()) != 0 {
		t.Fatal("something was written while the game ran")
	}
	// the refusal is noted as the last run, but it is not a roll: the same week may still roll once the game is closed
	if s := StateLinesForTest(t, e.state); !strings.Contains(s, "the game is running") || strings.Contains(s, "lastRoll") {
		t.Fatalf("state: %s", s)
	}
	e.running = false
	if r, err := Roll(e.opts()); err != nil || !r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestAGameThatStartsDuringTheRunStopsTheWrite(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	o := e.opts()
	calls := 0
	o.GameRunning = func() (bool, string) { calls++; return calls >= 2, "Dune.exe is running" }
	if _, err := Roll(o); err == nil {
		t.Fatal("no refusal")
	}
	if e.ini() != old {
		t.Fatal("Game.ini was written")
	}
}

func TestOnlyOneRollPerTuesdayFiveWeek(t *testing.T) {
	e := newEnv(t, iniWith("3"))
	if r, err := Roll(e.opts()); err != nil || !r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	after := e.ini()
	// the same week, later: skipped, nothing written, no error
	e.now = e.now.Add(3 * 24 * time.Hour) // Friday
	r, err := Roll(e.opts())
	if err != nil || r.Wrote || !strings.Contains(strings.Join(r.Lines, "\n"), "Skipped: a roll already happened") {
		t.Fatalf("%+v %v", r, err)
	}
	if e.ini() != after || len(e.backups()) != 1 {
		t.Fatal("a skipped run wrote")
	}
	// Tuesday 04:59 the next week still belongs to the old week
	e.now = time.Date(2026, 12, 1, 4, 59, 0, 0, time.Local)
	if r, _ := Roll(e.opts()); r.Wrote {
		t.Fatal("rolled before the new week began")
	}
	// --force rolls again
	o := e.opts()
	o.Force = true
	if r, err := Roll(o); err != nil || !r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	// at 05:00 the new week begins
	e.now = time.Date(2026, 12, 8, 5, 0, 0, 0, time.Local)
	if r, err := Roll(e.opts()); err != nil || !r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestAChosenLayoutIsNotHeldBackByTheWeekButIsRecorded(t *testing.T) {
	e := newEnv(t, iniWith("3"))
	if _, err := Roll(e.opts()); err != nil {
		t.Fatal(err)
	}
	o := e.opts()
	o.Set = ptr(7)
	r, err := Roll(o)
	if err != nil || !r.Wrote || r.New != 7 {
		t.Fatalf("%+v %v", r, err)
	}
	if !strings.Contains(e.ini(), "Singleplayer=7)") {
		t.Fatal("not set")
	}
	// the same layout again: nothing to do, no new backup
	n := len(e.backups())
	r, err = Roll(o)
	if err != nil || r.Wrote || len(e.backups()) != n {
		t.Fatalf("%+v %v", r, err)
	}
	for _, bad := range []int{-1, 12, 15, 16, 17} {
		o.Set = ptr(bad)
		if _, err := Roll(o); err == nil {
			t.Errorf("--set %d was accepted", bad)
		}
	}
	if !strings.Contains(e.ini(), "Singleplayer=7)") {
		t.Fatal("a refused --set changed the file")
	}
}

func TestBackupsAreKeptToTheNewestTen(t *testing.T) {
	e := newEnv(t, iniWith("3"))
	// twelve old backups, one other file that looks alike and one that must stay
	for i := 1; i <= 12; i++ {
		n := backupPrefix + time.Date(2026, 1, i, 5, 0, 0, 0, time.Local).Format("20060102-150405.000") + backupSuffix
		if err := os.WriteFile(filepath.Join(e.dir, n), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	keep := []string{"Game.ini.layout-roll-notes.bak", "Game.ini.tabr-backup-20261107-1230", "Engine.ini"}
	for _, k := range keep {
		os.WriteFile(filepath.Join(e.dir, k), []byte("mine"), 0o644)
	}
	o := e.opts()
	o.Force = true
	if _, err := Roll(o); err != nil {
		t.Fatal(err)
	}
	bs := e.backups()
	var mine []string
	for _, b := range bs {
		if backupName.MatchString(b) {
			mine = append(mine, b)
		}
	}
	if len(mine) != KeepBackups {
		t.Fatalf("%d kept: %v", len(mine), mine)
	}
	// the newest is the one just made; the oldest three (Jan 1 to 3) are gone
	if _, err := os.Stat(filepath.Join(e.dir, backupPrefix+"20260101-050000.000"+backupSuffix)); err == nil {
		t.Fatal("the oldest backup was kept")
	}
	if _, err := os.Stat(filepath.Join(e.dir, backupPrefix+"20260112-050000.000"+backupSuffix)); err != nil {
		t.Fatal("a recent backup was removed")
	}
	for _, k := range keep {
		if _, err := os.Stat(filepath.Join(e.dir, k)); err != nil {
			t.Fatalf("%s was removed", k)
		}
	}
}

func TestTwoBackupsInTheSameMillisecondDoNotShareAName(t *testing.T) {
	e := newEnv(t, iniWith("3"))
	a, err := writeBackup(e.dir, "one", e.now, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := writeBackup(e.dir, "two", e.now, 0)
	if err != nil || a == b {
		t.Fatalf("%s %s %v", a, b, err)
	}
	if x, _ := os.ReadFile(a); string(x) != "one" {
		t.Fatal("the first backup was overwritten")
	}
}

func TestFilesItDoesNotKnowHowToEditAreLeftAlone(t *testing.T) {
	cases := map[string]string{
		"the key twice": sec + "\n" + lineWith("1") + "\n" + lineWith("2") + "\n",
		"utf-16":        "\xff\xfe[\x00A\x00]\x00",
		"a NUL":         "[A]\nx=\x001\n",
	}
	for name, in := range cases {
		e := newEnv(t, in)
		if _, err := Roll(e.opts()); err == nil {
			t.Errorf("%s: no refusal", name)
		}
		if e.ini() != in || len(e.backups()) != 0 {
			t.Errorf("%s: the folder was changed", name)
		}
	}
	// a Game.ini that is a folder, a missing config folder
	e := newEnv(t, "")
	if err := os.Mkdir(filepath.Join(e.dir, FileName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Roll(e.opts()); err == nil {
		t.Error("a folder named Game.ini was accepted")
	}
	o := e.opts()
	o.ConfigDir = filepath.Join(e.dir, "nope")
	if _, err := Roll(o); err == nil {
		t.Error("a missing config folder was accepted")
	}
	o.ConfigDir = ""
	if _, err := Roll(o); err == nil {
		t.Error("an empty config folder was accepted")
	}
}

func TestADamagedStateFileStopsTheRollUnlessForced(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	if err := os.MkdirAll(filepath.Dir(e.state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.state, []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Roll(e.opts()); err == nil || e.ini() != old {
		t.Fatalf("a damaged state file did not stop the roll: %v", err)
	}
	o := e.opts()
	o.Force = true
	if r, err := Roll(o); err != nil || !r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	if s := StateLinesForTest(t, e.state); !strings.Contains(s, "lastRoll") {
		t.Fatalf("the state was not rewritten: %s", s)
	}
}

func TestAFileThatChangesBetweenTheReadAndTheWriteIsNotWritten(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	o := e.opts()
	n := 0
	o.GameRunning = func() (bool, string) {
		n++
		if n == 2 { // after the backup, before the write
			e.write(old + "extra=1\r\n")
		}
		return false, ""
	}
	if _, err := Roll(o); err == nil || !strings.Contains(err.Error(), "changed while this ran") {
		t.Fatalf("%v", err)
	}
	if e.ini() != old+"extra=1\r\n" {
		t.Fatal("the other program's change was overwritten")
	}
}

func TestWeekStart(t *testing.T) {
	loc := time.Local
	at := func(y int, m time.Month, d, h, mi int) time.Time { return time.Date(y, m, d, h, mi, 0, 0, loc) }
	for _, c := range []struct{ now, want time.Time }{
		{at(2026, 11, 24, 5, 0), at(2026, 11, 24, 5, 0)},  // Tuesday 05:00 exactly
		{at(2026, 11, 24, 4, 59), at(2026, 11, 17, 5, 0)}, // a minute before
		{at(2026, 11, 24, 23, 0), at(2026, 11, 24, 5, 0)},
		{at(2026, 11, 25, 0, 0), at(2026, 11, 24, 5, 0)},  // Wednesday
		{at(2026, 11, 30, 12, 0), at(2026, 11, 24, 5, 0)}, // Monday
		{at(2026, 12, 1, 4, 59), at(2026, 11, 24, 5, 0)},
		{at(2027, 1, 5, 5, 0), at(2027, 1, 5, 5, 0)},
		{at(2027, 1, 2, 9, 0), at(2026, 12, 29, 5, 0)}, // across the year
	} {
		if got := WeekStart(c.now); !got.Equal(c.want) {
			t.Errorf("%v: got %v want %v", c.now, got, c.want)
		}
	}
}

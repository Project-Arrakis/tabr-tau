package layoutroll

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// #1209: a run that writes Game.ini never goes on without its locks.

// blockLock puts a folder where the lock file of path should be, so that it cannot be made (nobody holds it).
func blockLock(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path+".tabr-tau.lock", 0o700); err != nil {
		t.Fatal(err)
	}
}

// free fails the test when the lock of path is still held.
func free(t *testing.T, path string) {
	t.Helper()
	release, err := save.LockSaveStrict(path)
	if err != nil {
		t.Fatalf("the lock of %s was not given back: %v", filepath.Base(path), err)
	}
	release()
}

func TestARealRunThatCannotMakeALockIsRefusedAndChangesNothing(t *testing.T) {
	cases := []struct {
		name  string
		block func(e *env) string // the file whose lock file cannot be made
		want  string              // said in the error and in the log
	}{
		{"the lock of the save", func(e *env) string { return e.save }, "no lock could be taken on the save"},
		{"the lock of the record", func(e *env) string { return e.state }, "no lock could be taken on the record layout-roll.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
			putState(t, e, staleEntry(t, e, 1010)) // a note a run without the lock must not settle
			state, err := os.ReadFile(e.state)
			if err != nil {
				t.Fatal(err)
			}
			before := sha(t, e.save)
			blocked := c.block(e)
			blockLock(t, blocked)
			for _, force := range []bool{false, true} {
				o.Force = force
				r, err := Roll(o)
				if err == nil || r.Wrote || len(r.Lines) != 0 {
					t.Fatalf("force %v: %v\n%s", force, err, lines(r))
				}
				mustContain(t, err.Error(), "not changed", c.want, "Nothing was changed")
				if strings.Contains(err.Error(), "another layout roll is running") {
					t.Fatalf("nobody holds the lock: %v", err)
				}
			}
			after, _ := os.ReadFile(e.state)
			if string(after) != string(state) || sha(t, e.save) != before || e.ini() != iniWith("11") || len(checkpoints(t, e.save)) != 0 || len(e.backups()) != 0 {
				t.Fatal("a run without its lock changed something")
			}
			mustContain(t, logOf(t, e), "layout roll refused: "+c.want, "nothing changed")
			if blocked != e.save {
				free(t, e.save) // the refused run gave back the lock of the save it had taken
			}
			// a dry run takes no lock, so it still works
			o.DryRun = true
			if _, err := Roll(o); err != nil {
				t.Fatalf("a dry run: %v", err)
			}
		})
	}
}

// The case of the issue: --save names a place that does not exist, so no lock file can be made there. It used to count as locked, and the refusal wrote layout-roll.json.
func TestASavePathInAFolderThatIsNotThereTakesNoLockAndWritesNoRecord(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	o := e.opts()
	o.SavePath = filepath.Join(t.TempDir(), "mistyped", "game.db")
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "no lock could be taken on the save", "--save")
	if _, serr := os.Stat(e.state); serr == nil {
		t.Fatal("the record was written by a run that holds no lock")
	}
	mustContain(t, logOf(t, e), "no lock could be taken on the save")
}

func TestARunRefusedAtTheRecordsLockGivesBackTheLockOfItsSave(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	if err := os.MkdirAll(filepath.Dir(e.state), 0o700); err != nil {
		t.Fatal(err)
	}
	hold, err := save.LockSave(e.state) // a running roll on another save
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	if _, err := Roll(e.opts()); err == nil || !strings.Contains(err.Error(), "another layout roll is running") {
		t.Fatalf("%v", err)
	}
	free(t, e.save)
}

func TestARealRunWithNoPlaceForTheRecordIsRefusedEvenWithForce(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	o := e.opts()
	o.StatePath, o.Force = "", true
	before := sha(t, e.save)
	r, err := Roll(o)
	if err == nil || r.Wrote || e.ini() != iniWith("11") || sha(t, e.save) != before || len(checkpoints(t, e.save)) != 0 || len(e.backups()) != 0 {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	mustContain(t, err.Error(), "not changed", "cannot be locked")
	free(t, e.save)
}

// layout-roll vet N writes Game.ini too: it takes the lock of the record, so a vet on another save (or on none) cannot change Game.ini under a running roll.
func TestVetSetIsRefusedWhileARollIsRunningOnAnotherSave(t *testing.T) {
	for _, noSave := range []bool{false, true} {
		e := newEnv(t, iniWith("11"))
		other := newEnv(t, iniWith("11"))
		v := e.opts() // the same Game.ini and record, another save
		v.SavePath = other.save
		if noSave {
			v.SavePath = ""
		}
		o := e.opts()
		var verr error
		var vr Result
		var during string
		started := false
		inner := o.GameRunning
		o.GameRunning = func() (bool, string) {
			if !started {
				started = true
				vr, verr = VetSet(v, 8)
				during = e.ini()
				if !noSave {
					free(t, other.save) // the refused vet gave back the lock of its save
				}
			}
			return inner()
		}
		r, err := Roll(o)
		if err != nil || !r.Wrote {
			t.Fatalf("the roll was harmed: %v\n%s", err, lines(r))
		}
		if verr == nil || vr.Wrote || during != iniWith("11") || len(e.backups()) != 1 {
			t.Fatalf("no save %v: the vet was not refused: %v\n%s", noSave, verr, lines(vr))
		}
		mustContain(t, verr.Error(), "not changed", "another layout roll is running", "layout-roll.json")
		if got, err := VetSet(v, 8); err != nil || !got.Wrote { // and goes ahead once the roll has ended
			t.Fatalf("%v", err)
		}
		free(t, e.state)
	}
}

func TestVetSetThatCannotLockIsRefused(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *env, o *Options)
		want  string
	}{
		{"the lock of the save cannot be made", func(e *env, o *Options) { blockLock(e.t, e.save) }, "no lock could be taken on the save"},
		{"the lock of the record cannot be made", func(e *env, o *Options) { blockLock(e.t, e.state) }, "no lock could be taken on the record"},
		{"no place for the record", func(e *env, o *Options) { o.StatePath = "" }, "cannot be locked"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			o := e.opts()
			c.setup(e, &o)
			r, err := VetSet(o, 8)
			if err == nil || r.Wrote || e.ini() != iniWith("11") || len(e.backups()) != 0 {
				t.Fatalf("%v\n%s", err, lines(r))
			}
			mustContain(t, err.Error(), "not changed", c.want)
		})
	}
}

// A record that cannot be read refuses the run without --force; the record is left as it is and the log says why. A dry run writes nothing.
func TestARefusalForARecordThatCannotBeReadIsInTheLog(t *testing.T) {
	for _, withSave := range []bool{true, false} {
		e := newEnv(t, iniWith("11"))
		if err := os.MkdirAll(filepath.Dir(e.state), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(e.state, []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		o := e.opts()
		if !withSave {
			o.SavePath = ""
		}
		o.DryRun = true
		if _, err := Roll(o); err == nil || logOf(t, e) != "" {
			t.Fatalf("dry run: %v, log %q", err, logOf(t, e))
		}
		o.DryRun = false
		if _, err := Roll(o); err == nil {
			t.Fatal("not refused")
		}
		mustContain(t, logOf(t, e), "layout roll refused: the state file layout-roll.json is not valid: nothing changed")
		if b, _ := os.ReadFile(e.state); string(b) != "{not json" {
			t.Fatal("the record was written")
		}
	}
}

func TestTheLogIsRenamedPast64KBOnlyUnderItsLock(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	o := e.opts()
	p := filepath.Join(filepath.Dir(e.state), LogName)
	old := filepath.Join(filepath.Dir(e.state), "layout-roll.1.log")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("x", maxLog+1)
	for path, text := range map[string]string{p: big, old: "older\n"} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	hold, err := save.LockSave(p) // another refused run is renaming it
	if err != nil {
		t.Fatal(err)
	}
	noteRefusal(o, e.now, "one")
	if read(old) != "older\n" || !strings.HasPrefix(read(p), big) || !strings.HasSuffix(read(p), " one\n") {
		t.Fatal("the log was renamed without its lock, or the line was lost")
	}
	hold()
	full := read(p)
	noteRefusal(o, e.now, "two")
	if read(old) != full || read(p) != e.now.Format("2006-01-02 15:04:05")+" two\n" {
		t.Fatalf("not renamed: new log %q", read(p))
	}
	noteRefusal(o, e.now, "three") // a small log is left alone, and so is the older copy
	if read(old) != full || strings.Count(read(p), "\n") != 2 {
		t.Fatal("a small log was renamed")
	}
	free(t, p)
}

// The other branch of the check of #1205 after the first base was asked about: a save that cannot be read again counts as changed.
func TestASaveThatCannotBeReadAgainAfterTheFirstQuestionRefusesBeforeAnyWrite(t *testing.T) {
	e, o := mvEnv(t, rcWorld(ddBases(2)...)+rcBoxes(10, 10)+rcReserveItem+"update inventories set max_item_count=3 where id=12;")
	before := sha(t, e.save)
	asked := false
	real := emptyReserveInto
	swapVar(t, &emptyReserveInto, func(s *save.Save, exclude []int64, previewOnly bool) (movedReserve, error) {
		asked = asked || !previewOnly
		return real(s, exclude, previewOnly)
	})
	realSHA := fileSHA
	swapVar(t, &fileSHA, func(path string) (string, error) {
		if asked {
			return "", errors.New("the file is in use")
		}
		return realSHA(path)
	})
	_, err := Roll(o)
	if err == nil || e.ini() != iniWith("11") || sha(t, e.save) != before {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "the save file is not the one the checkpoint", "the file is in use", "0 of 2 base(s) can be recycled now", "nothing was recycled or exported")
	noExportFolder(t, e)
}

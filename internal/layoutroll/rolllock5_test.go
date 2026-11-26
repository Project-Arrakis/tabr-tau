package layoutroll

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/durable"
)

// The review of #1254: Game.ini is read after the locks, the locks are held until the write, and a vet takes them in the roll's order.

// whileWaiting makes the lock behind the variable run change before it is taken: what a vet or a roll that was just ending does while this run waits for it.
func whileWaiting(t *testing.T, lock *func(string) (func(), error), change func()) {
	t.Helper()
	real := *lock
	swapVar(t, lock, func(p string) (func(), error) {
		change()
		return real(p)
	})
}

func TestAGameIniChangedWhileARollWaitsForItsLocksIsTheOneItWorksFrom(t *testing.T) {
	for name, lock := range map[string]*func(string) (func(), error){"waiting for the save": &lockSave, "waiting for the record": &lockState} {
		t.Run(name, func(t *testing.T) {
			// the case of the review: bases are recycled and the save is written, and the run then stopped because Game.ini was not the text it had read
			e, o := mvEnv(t, rcWorld(ddBases(1)...)+rcBoxes(10, 10))
			whileWaiting(t, lock, func() { e.write(iniWith("4")) })
			r, err := Roll(o)
			if err != nil || !r.Wrote || r.Old != 4 {
				t.Fatalf("%v\n%s", err, lines(r))
			}
			bs := e.backups()
			if len(bs) != 1 {
				t.Fatalf("backups: %v", bs)
			}
			if b, _ := os.ReadFile(filepath.Join(e.dir, bs[0])); string(b) != iniWith("4") {
				t.Fatalf("the backup is not the file as it was after the wait: %q", b)
			}
		})
		t.Run(name+", --set to the layout just set", func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			o := e.opts()
			o.Set = ptr(7)
			whileWaiting(t, lock, func() { e.write(iniWith("7")) })
			r, err := Roll(o)
			if err != nil || r.Wrote || e.ini() != iniWith("7") || len(e.backups()) != 0 {
				t.Fatalf("%v\n%s", err, lines(r))
			}
			mustContain(t, lines(r), "Nothing to do: Game.ini already sets layout 7")
		})
	}
	for name, lock := range map[string]*func(string) (func(), error){"a vet waiting for the save": &lockSave, "a vet waiting for the record": &lockState} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			whileWaiting(t, lock, func() { e.write(iniWith("4")) })
			r, err := VetSet(e.opts(), 8)
			if err != nil || !r.Wrote || r.Old != 4 || e.ini() != iniWith("8") {
				t.Fatalf("%v\n%s", err, lines(r))
			}
		})
	}
}

// lockedNow says whether the lock of path is held.
func lockedNow(t *testing.T, path string) bool {
	t.Helper()
	release, err := save.LockPlain(path)
	if errors.Is(err, save.ErrSaveInUse) {
		return true
	}
	if err != nil {
		t.Fatal(err)
	}
	release()
	return false
}

func TestARollHoldsBothLocksUntilGameIniIsWritten(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	var saveHeld, recordHeld bool
	writes := 0
	o := e.opts()
	inner := o.GameRunning
	o.GameRunning = func() (bool, string) {
		if !lockedNow(t, e.save) || !lockedNow(t, e.state) {
			t.Error("a lock is not held at a check of the running programs")
		}
		return inner()
	}
	real := writeIniFile
	swapVar(t, &writeIniFile, func(path string, data []byte, mode fs.FileMode, opts durable.Options) error {
		writes++
		saveHeld, recordHeld = lockedNow(t, e.save), lockedNow(t, e.state)
		return real(path, data, mode, opts)
	})
	if r, err := Roll(o); err != nil || !r.Wrote {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if writes != 1 || !saveHeld || !recordHeld {
		t.Fatalf("at the write of Game.ini (%d): save locked %v, record locked %v", writes, saveHeld, recordHeld)
	}
	free(t, e.save)
	free(t, e.state)
}

// A roll started while a vet is between its last check and its write is refused, on the vet's save and on another one, and changes nothing.
func TestARollStartedDuringAVetIsRefused(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	other := newEnv(t, iniWith("11"))
	o := e.opts()
	calls := 0
	o.GameRunning = func() (bool, string) {
		calls++
		if calls != 2 { // the second check is the one right before the write
			return false, ""
		}
		for _, c := range []struct{ savePath, want string }{
			{e.save, "the save is in use"},
			{other.save, "another layout roll is running (it holds the record layout-roll.json)"},
		} {
			roll := e.opts()
			roll.SavePath = c.savePath
			r, err := Roll(roll)
			if err == nil || r.Wrote || !strings.Contains(err.Error(), c.want) {
				t.Errorf("a roll during the vet: %v\n%s", err, lines(r))
			}
		}
		if e.ini() != iniWith("11") {
			t.Error("Game.ini was changed during the vet")
		}
		return false, ""
	}
	if r, err := VetSet(o, 8); err != nil || !r.Wrote || e.ini() != iniWith("8") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if calls != 2 {
		t.Fatalf("the running programs were checked %d times", calls)
	}
	free(t, e.save)
	free(t, e.state)
}

func TestARollAndAVetTakeTheSaveThenTheRecord(t *testing.T) {
	for name, run := range map[string]func(o Options) (Result, error){"a roll": Roll, "a vet": func(o Options) (Result, error) { return VetSet(o, 8) }} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			var order []string
			for what, lock := range map[string]*func(string) (func(), error){"save": &lockSave, "record": &lockState} {
				real := *lock
				swapVar(t, lock, func(p string) (func(), error) {
					order = append(order, what)
					return real(p)
				})
			}
			if r, err := run(e.opts()); err != nil || !r.Wrote {
				t.Fatalf("%v\n%s", err, lines(r))
			}
			if strings.Join(order, ",") != "save,record" {
				t.Fatalf("%v", order)
			}
		})
	}
}

// The lock of the log is a plain one: writing a line sweeps nothing.
func TestALogLineSweepsNoFile(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	temp := filepath.Join(filepath.Dir(e.state), LogName+".tabr-123.tmp")
	if err := os.MkdirAll(filepath.Dir(temp), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(temp, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	noteRefusal(e.opts(), e.now, "a line")
	if _, err := os.Stat(temp); err != nil {
		t.Fatalf("the lock of the log swept a file: %v", err)
	}
}

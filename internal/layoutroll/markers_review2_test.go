package layoutroll

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
	"github.com/Project-Arrakis/tabr-tau/kit/durable"
)

// The second review of the Deep Desert markers step (#1216): every stop says its text once, the note kept by the state file is said loudly, the check before the step, and the stops
// at Game.ini after the markers were written.

// The stop text of a markers stop is said once in what the command prints (the lines, then the error), on every path: the plain "not changed" stops, the write that was not saved (file
// unchanged, or changed), the written file that does not read back, and the write that reported a warning. No step before the markers wrote anything here.
func TestTheMarkersStopTextIsSaidOnce(t *testing.T) {
	realFlush := func(s *save.Save) save.Flush { return s.Flush() }
	notSaved := func(s *save.Save) save.Flush {
		f := realFlush(s)
		f.Saved, f.Why = false, "the write was not confirmed in time"
		return f
	}
	leftOver := func(string, ops.MarkerSurvey) error { return errors.New("read back: 2 marker row(s) are left") }
	cases := []struct {
		name  string
		setup func(t *testing.T)
		said  string // a phrase of the stop's own text
	}{
		{"cannot be counted", func(t *testing.T) {
			swapVar(t, &surveyMarkers, func(*save.Save) (ops.MarkerSurvey, error) { return ops.MarkerSurvey{}, errors.New("no such table") })
		}, "no such table"},
		{"the edit fails", func(t *testing.T) {
			swapVar(t, &clearMarkers, func(*save.Save) (ops.MarkerSurvey, error) {
				return ops.MarkerSurvey{}, errors.New("a row could not be removed")
			})
		}, "a row could not be removed"},
		{"not saved, file unchanged", func(t *testing.T) {
			swapVar(t, &flushSave, func(*save.Save) save.Flush { return save.Flush{Why: "the disk is full"} })
		}, "the disk is full"},
		{"not saved, file changed", func(t *testing.T) { swapVar(t, &flushSave, notSaved) }, "the write was not reported as saved"},
		{"not saved, file changed, does not read back", func(t *testing.T) {
			swapVar(t, &flushSave, notSaved)
			swapVar(t, &verifyMarkers, leftOver)
		}, "2 marker row(s) are left"},
		{"written, does not read back", func(t *testing.T) { swapVar(t, &verifyMarkers, leftOver) }, "2 marker row(s) are left"},
		{"written with a warning", func(t *testing.T) {
			swapVar(t, &flushSave, func(s *save.Save) save.Flush {
				f := realFlush(s)
				f.Warning = "the file could not be read back in time"
				return f
			})
		}, "the file could not be read back in time"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			e.save = testsave.PlayerWithSQL(t, mkWorld).Path
			c.setup(t)
			r, err := Roll(e.opts())
			if err == nil || r.Wrote {
				t.Fatalf("%v", err)
			}
			all := lines(r) + "\n" + err.Error()
			if n := strings.Count(all, c.said); n != 1 {
				t.Fatalf("%q said %d times:\n%s", c.said, n, all)
			}
		})
	}
}

// The intent note is written, the write is not saved and the file is left alone, and then the state file cannot be written: the note is still the last word in layout-roll.json, so the
// run says so loudly (once).
func TestAStateFileThatKeepsTheMarkersNoteIsSaidLoudly(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, mkWorld).Path
	failing := false
	swapVar(t, &writeStateFile, func(path string, s state) error {
		if failing {
			return errors.New("the disk is full")
		}
		err := writeState(path, s)
		if s.LastRun != nil && strings.Contains(s.LastRun.Outcome, "about to write the save with the Deep Desert markers removed") {
			failing = true
		}
		return err
	})
	swapVar(t, &flushSave, func(*save.Save) save.Flush { return save.Flush{Why: "the disk is full"} })
	before := sha(t, e.save)
	r, err := Roll(e.opts())
	if err == nil || r.Wrote || sha(t, e.save) != before || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	mustContain(t, err.Error(), "WARNING: the state file "+filepath.Base(e.state)+" could not be updated", "still holds the note that the save was about to be written")
	if n := strings.Count(lines(r)+"\n"+err.Error(), "WARNING: the state file"); n != 1 {
		t.Fatalf("the warning is said %d times", n)
	}
	if stt := readState(t, e.state); stt.LastRun == nil || !strings.Contains(stt.LastRun.Outcome, "about to write the save with the Deep Desert markers removed") {
		t.Fatalf("%+v", stt.LastRun)
	}
}

// The game starts, or Game.ini is changed by another program, between the guard and the markers step: the FIRST of the two checks stops the run before the save is touched.
func TestGameIniChecksBeforeTheMarkersStepStopTheRunBeforeTheSaveIsTouched(t *testing.T) {
	for _, c := range []struct{ name, said string }{{"the game starts", "it started while this ran"}, {"Game.ini changes", "Game.ini changed while this ran"}} {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			e.save = testsave.PlayerWithSQL(t, mkWorld).Path
			o := e.opts()
			calls := 0
			o.GameRunning = func() (bool, string) { // the first call is the guard of the run, the second the check right before the markers step
				calls++
				if calls == 2 {
					if c.name == "the game starts" {
						return true, "Dune.exe is running"
					}
					e.write(iniWith("4"))
				}
				return false, ""
			}
			before := sha(t, e.save)
			r, err := Roll(o)
			if err == nil || r.Wrote {
				t.Fatalf("%v", err)
			}
			mustContain(t, err.Error(), c.said)
			if sha(t, e.save) != before || survey(t, e.save).DeepDesert.Total() != 8 {
				t.Fatal("the save was written")
			}
			if len(e.backups()) != 0 || calls != 2 {
				t.Fatalf("backups %v, checks %d", e.backups(), calls)
			}
			if strings.Contains(err.Error(), "WAS changed") {
				t.Fatalf("it says the save was changed: %v", err)
			}
		})
	}
}

// A backup of Game.ini that cannot be made, or a Game.ini that cannot be written, after the markers were written: the state file and the log say that Game.ini was not changed and the
// save WAS, with the checkpoint, instead of keeping "Game.ini is next".
func TestAGameIniStopAfterTheMarkersIsRecorded(t *testing.T) {
	for _, c := range []struct{ name, say string }{{"the backup", "the backup could not be made"}, {"the write", "the file could not be written"}} {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			e.save = testsave.PlayerWithSQL(t, mkWorld).Path
			if c.name == "the backup" {
				swapVar(t, &writeBackupFile, func(string, string, time.Time, os.FileMode) (string, error) {
					return "", errors.New("the disk is full")
				})
			} else {
				swapVar(t, &writeIniFile, func(string, []byte, os.FileMode, durable.Options) error { return errors.New("the disk is full") })
			}
			r, err := Roll(e.opts())
			if err == nil || r.Wrote || e.ini() != iniWith("11") {
				t.Fatalf("%v", err)
			}
			stt := readState(t, e.state)
			cp := stt.LastCheck.Checkpoint
			if cp == "" || stt.LastRoll != nil {
				t.Fatalf("%+v", stt)
			}
			want := "stopped: Game.ini was not changed (" + c.say + "), the save WAS changed earlier in this run (checkpoint " + cp + ")"
			if stt.LastRun.Outcome != want {
				t.Fatalf("outcome %q, want %q", stt.LastRun.Outcome, want)
			}
			mustContain(t, logText(t, e), "layout roll: "+want)
			mustContain(t, err.Error(), "the disk is full", "NOTE: the Deep Desert markers", cp)
			if survey(t, e.save).DeepDesert.Total() != 0 {
				t.Fatal("the markers were not removed")
			}
		})
	}
}

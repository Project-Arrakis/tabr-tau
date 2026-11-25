package layoutroll

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The Deep Desert markers step (#1216). A single-player Coriolis never wipes the markers, so after a roll the game's own map would still show places of the earlier layouts. A real run
// therefore removes, after the bases and the vehicles and before Game.ini is changed, the Deep Desert's rows of markers, player_markers and resourcefield_state (and nothing else: never
// another map, never an actor, base, vehicle or inventory); the game adds markers again as the character discovers places. The step is one verified edit of its own (ops/ddmarkers.go):
// the rows are counted before and after inside the edit, the save is written through the editor's own write path, and the written file is opened afresh and counted once more. The run's
// own checkpoint, taken before anything else, is the way back. --keep-markers (Options.KeepMarkers) leaves them as they are. A dry run only counts.

// The parts of the step that touch the save. They are variables only so a test can make one fail; nothing else changes them.
var (
	surveyMarkers = func(s *save.Save) (ops.MarkerSurvey, error) { return (&ops.Ops{S: s}).SurveyDeepDesertMarkers() }
	clearMarkers  = func(s *save.Save) (ops.MarkerSurvey, error) { return (&ops.Ops{S: s}).ClearDeepDesertMarkers() }
	verifyMarkers = func(path string, before ops.MarkerSurvey) error {
		fresh, err := openSave(path)
		if err != nil {
			return err
		}
		defer fresh.Close()
		return (&ops.Ops{S: fresh}).VerifyDeepDesertMarkersCleared(before)
	}
)

// markerWords is the counts as one plain phrase.
func markerWords(c ops.MarkerCounts) string {
	return fmt.Sprintf("%d marker(s), %d discovered row(s) and %d resource-field row(s)", c.Markers, c.Discovered, c.ResourceFields)
}

// markersNote is said with every stop after the markers were written to the save by this run.
func (c *checkRecord) markersNote() string {
	return fmt.Sprintf("NOTE: the Deep Desert markers (%s) WERE removed and written to the save earlier in this run. %s", c.Markers, c.wayBack())
}

// noteLog appends one dated line to layout-roll.log; like the refusals it never fails the run.
func noteLog(o Options, now time.Time, line string) { noteRefusal(o, now, line) }

// clearDeepDesertMarkers is the step. It returns a non-nil error when the run must stop before Game.ini is changed (the lines say why); a run that removes nothing, or only counts, returns
// nil. It is called after every other guard of the run and right before Game.ini is written.
func clearDeepDesertMarkers(o Options, r *Result, st *state, now time.Time) error {
	if o.KeepMarkers {
		r.say("Deep Desert markers: left as they are (--keep-markers).")
		return nil
	}
	if o.SavePath == "" {
		return nil // the guard refused such a run before this point
	}
	s, err := openSave(o.SavePath)
	if err != nil {
		return stopMarkers(o, r, st, now, "not changed: the Deep Desert markers could not be counted", fmt.Errorf("not changed: the save could not be read to count the Deep Desert markers (%v)", err), nil)
	}
	defer s.Close()
	sv, err := surveyMarkers(s)
	if err != nil {
		return stopMarkers(o, r, st, now, "not changed: the Deep Desert markers could not be counted", fmt.Errorf("not changed: the Deep Desert markers could not be counted (%v)", err), nil)
	}
	d := sv.DeepDesert
	if d.Total() == 0 {
		r.say("Deep Desert markers: none to remove.")
		return nil
	}
	if o.DryRun {
		r.say("Dry run: would remove the Deep Desert markers from the save, after the vehicles and before Game.ini: %s. Other maps are not touched, and the game adds markers again as you discover places (--keep-markers leaves them).", markerWords(d))
		return nil
	}
	rec := st.LastCheck
	fail := func(outcome, format string, a ...any) error {
		return stopMarkers(o, r, st, now, outcome, errors.New("not changed: "+fmt.Sprintf(format, a...)), rec)
	}
	// the file as it is now must be known: without that a write could not be told from no write afterwards
	before, berr := fileSHA(s.Path)
	if berr != nil || before == "" {
		s.Discard()
		return fail("not changed: the save file could not be read before the write", "the save file cannot be read before the markers are removed (%v), so a write could not be told from no write afterwards; the save was not written", berr)
	}
	// one edit; it counts the rows before and after inside itself and fails as a whole (changing nothing) unless the Deep Desert has none left and every other map has as many as before
	done, err := clearMarkers(s)
	if err != nil {
		s.Discard()
		return fail("not changed: the Deep Desert markers could not be removed", "the Deep Desert markers could not be removed (%v); the save was not changed by this step", err)
	}
	words := markerWords(done.DeepDesert)
	f := flushSave(s)
	changed := func(outcome, why string) error {
		msg := fmt.Sprintf("the save WAS changed (the Deep Desert markers were removed: %s) but %s", words, why)
		if rec != nil {
			msg += ". The checkpoint " + rec.Checkpoint + " holds the save as it was before; Database > Save backups restores it. Look at it before the game starts"
		}
		r.say("Stopped: %s.", msg)
		return stopMarkers(o, r, st, now, outcome, errors.New("stopped: "+msg), rec)
	}
	mark := func() {
		if rec != nil {
			rec.markersWritten, rec.Markers = true, words
			if rec.SaveFile == "" {
				rec.SaveFile = s.Path
			}
		}
	}
	if !f.Saved {
		same, why := fileLeftAlone(s.Path, before)
		if same {
			s.Discard() // the edit stays out of the file; throw it away so nothing is left queued
			return fail("not changed: the Deep Desert markers could not be written", "the Deep Desert markers were counted but the save could not be written (%s); the save was not changed by this step", f.Why)
		}
		// the file is different, but what made it different is not known until it is read: it is opened afresh and counted
		if verr := verifyMarkers(s.Path, done); verr != nil {
			return changed("stopped: the save file changed and does not read back as planned", fmt.Sprintf("the write was not reported as saved (%s) and %s; counted afresh, it does not read back as cleared (%v). It may have been changed in part, or by another program", f.Why, why, verr))
		}
		mark()
		return changed("stopped: the write of the markers was not confirmed and the save file changed", "the write was not reported as saved ("+f.Why+") and "+why)
	}
	// from here the save file HAS been changed
	mark()
	if f.Warning != "" {
		return changed("stopped: the written save needs a look", "the write reported: "+f.Warning)
	}
	if err := verifyMarkers(s.Path, done); err != nil {
		return changed("stopped: the written save does not read back", "the file does not read back as cleared ("+err.Error()+")")
	}
	r.say("Removed the Deep Desert markers from the save in one edit, read back inside the edit and again from the written file: %s. Rows of other maps were counted before and after and are unchanged (%s).", words, markerWords(done.Others))
	// recorded at once (the state file and the log), so the run's own record says what was removed even if a later step stops
	if rec != nil && o.lockHeld {
		warnStateAfterWrite(o, r, rec, saveRunErr(o.StatePath, *st, now, "in progress: the Deep Desert markers were removed from the save ("+words+"); Game.ini is next"))
	}
	noteLog(o, now, "layout roll: removed the Deep Desert markers from the save ("+words+"); other maps unchanged; checkpoint "+checkpointName(rec))
	return nil
}

func checkpointName(rec *checkRecord) string {
	if rec == nil || rec.Checkpoint == "" {
		return "none"
	}
	return rec.Checkpoint
}

// stopMarkers ends the run at this step: the outcome goes in the state file (and the log, when the run holds no lock), and the error says what an earlier step of this run already wrote.
func stopMarkers(o Options, r *Result, st *state, now time.Time, outcome string, e error, rec *checkRecord) error {
	if o.DryRun {
		r.say("Dry run: it would stop here: %v.", e)
		return nil
	}
	if rec != nil {
		if rec.markersWritten {
			outcome += " (the Deep Desert markers were removed and written earlier in this run)"
		}
		if rec.vehiclesWritten {
			outcome += " (vehicles were taken out and written earlier in this run)"
		}
		if rec.written {
			outcome += " (bases were recycled and written earlier in this run)"
		}
	}
	if !o.lockHeld {
		noteRefusal(o, now, "layout roll stopped: "+e.Error())
		if rec != nil && rec.savedChanged() {
			return fmt.Errorf("%v. %s", e, rec.writtenNote())
		}
		return e
	}
	if rec != nil {
		rec.Blockers = []string{strings.TrimPrefix(strings.TrimPrefix(e.Error(), "not changed: "), "stopped: ")}
	}
	if werr := saveRunErr(o.StatePath, *st, now, outcome); werr != nil && rec != nil && rec.savedChanged() {
		rec.stateWarn = fmt.Sprintf("WARNING: the state file %s could not be updated (%v). Keep this output: it names the way back", "layout-roll.json", werr)
		r.say("%s", rec.stateWarn)
	}
	if rec == nil || !rec.savedChanged() {
		r.say("%s", e.Error())
		return e
	}
	n := rec.writtenNote()
	r.say("%s", n)
	return fmt.Errorf("%v. %s", e, n)
}

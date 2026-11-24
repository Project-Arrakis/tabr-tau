package layoutroll

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/durable"
)

// The Deep Desert guard (#1167). A layout change puts new terrain under the Deep Desert, so a base, the character or a vehicle left there is left inside it. Before Game.ini is
// touched the run (with the game closed): takes a verified checkpoint of the whole save, reads the save and refuses the roll when a base of the player or the character is in the Deep
// Desert, and exports each such base to a console base backup file first. The vehicles of the player that stand in the Deep Desert are taken out by the run itself (#1174): stored in
// the game's own vehicle backup (a Scout, a Sandbike) or moved to the character's map in front of the character (every other type), in one edit of the save that is read back, written
// through the editor's own write path (the game must be closed) and read back again from the file; then the guard looks once more and must find none. A run that is going to refuse for
// another reason (a base, the character) changes nothing, vehicles included. No base is moved, deleted or picked up by this run unless it was started with --recycle-bases, which recycles the player's bases first (recycle.go).

// exportFolderPrefix names the dated folder (beside the state file) that holds the base files of one refused run.
const exportFolderPrefix = "layout-roll-bases-"

// The steps that touch the save. They are variables only so a test can make one fail (a checkpoint that cannot be taken, a file that cannot be written); nothing else changes them.
var (
	openSave       = save.Open
	takeCheckpoint = func(s *save.Save) (save.ManualBackup, error) { return s.BackupCheckpoint() }
	inspectSave    = func(s *save.Save) (ops.DeepDesertHolds, error) { return (&ops.Ops{S: s}).DeepDesertHolds() }
	// the vehicle step (#1174): the plan alone (a dry run), the edit, the write to the file, the lock that keeps a second editor off the save, and the check of the written file
	planVehicles  = func(s *save.Save) ([]ops.VehicleMove, error) { return (&ops.Ops{S: s}).PlanDeepDesertVehicles() }
	moveVehicles  = func(s *save.Save) ([]ops.VehicleMove, error) { return (&ops.Ops{S: s}).MoveDeepDesertVehicles() }
	flushSave     = func(s *save.Save) save.Flush { return s.Flush() }
	lockSave      = save.LockSave
	verifyWritten = func(path string, plan []ops.VehicleMove) error {
		fresh, err := openSave(path)
		if err != nil {
			return err
		}
		defer fresh.Close()
		return (&ops.Ops{S: fresh}).VerifyVehicleMoves(plan)
	}
	exportBaseFile = func(s *save.Save, id int64, at time.Time, editor string) (*ops.BaseExport, error) {
		return (&ops.Ops{S: s}).ExportStandingBaseConsole(id, at, editor)
	}
)

// checkRecord is what the last run that looked at the save found, kept in the state file so the person can read it later.
type checkRecord struct {
	At               time.Time `json:"at"`
	Checkpoint       string    `json:"checkpoint,omitempty"`       // the name of the checkpoint in the save's backup folder
	CheckpointSHA256 string    `json:"checkpointSha256,omitempty"` // its SHA-256, as verified when it was taken
	Blockers         []string  `json:"blockers,omitempty"`         // why the roll was refused, one line each
	ExportFolder     string    `json:"exportFolder,omitempty"`     // the dated folder the base files were written to
	ExportFiles      []string  `json:"exportFiles,omitempty"`      // their names
	Vehicles         []string  `json:"vehicles,omitempty"`         // what was done with each vehicle of the player in the Deep Desert, one line each (#1174)

	// --recycle-bases (#1167): the checkpoint's full path and the save file (for the way back), the bases recycled and written in this run (per base: the Solido device and blueprint,
	// what moved to the Reserve), and the room table of the plan
	CheckpointPath string           `json:"checkpointPath,omitempty"`
	SaveFile       string           `json:"saveFile,omitempty"`
	Recycled       []recycledRecord `json:"recycled,omitempty"`
	RecycleSpace   []string         `json:"recycleSpace,omitempty"`
	written        bool             // the save file was written by the recycle step of this run
	history        []string         // the checkpoints of the EARLIER runs that recycled bases (oldest first, never this run's own), for the way back; not part of this record (the state file keeps them)
	stateWarn      string           // set when the state file could not be written after the save was written: said with every later stop

	// the Reserve moved into the boxes between the bases (#1191), and the note written before the save is written: the bases about to be recycled and the file as it was. The intent is
	// removed once the write is confirmed, or taken back when the save was not written; one that is still there means the run ended without recording how the write went.
	ReserveMoves []reserveMoveRecord `json:"reserveMoves,omitempty"`
	Intent       *writeIntent        `json:"writeIntent,omitempty"`
}

// writeIntent is the note the run leaves in the state file before it writes the save (#1167).
type writeIntent struct {
	Bases        []int64 `json:"bases"`
	SaveFile     string  `json:"saveFile"`
	BeforeSHA256 string  `json:"saveSha256Before"` // the save file as it was before the write
}

// wayBack says where the save from before this run is and how to put it back.
func (c *checkRecord) wayBack() string {
	return fmt.Sprintf("The checkpoint %s holds the save as it was before this run; to go back, close the game and restore it in Database > Save backups, or in PowerShell: Copy-Item -Force \"%s\" \"%s\"", c.Checkpoint, c.CheckpointPath, c.SaveFile)
}

// recycledNote is said with every stop after the bases were written: what was done and the way back.
func (c *checkRecord) recycledNote() string {
	n := fmt.Sprintf("NOTE: %d base(s) WERE recycled and written to the save earlier in this run. %s", len(c.Recycled), c.wayBack())
	if len(c.history) > 0 {
		n += fmt.Sprintf(". Checkpoints of the earlier runs that recycled bases, oldest first (the oldest is the save from before the first recycle): %s", strings.Join(c.history, "; "))
	}
	if c.stateWarn != "" {
		n += ". " + c.stateWarn
	}
	return n
}

// fileSHA is the SHA-256 of the file as it is now (a variable only so that a test can make the read fail).
var fileSHA = func(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// fileLeftAlone says whether the file is still exactly what it was (before is its SHA-256 from before the write), and when it is not, why in words. A file that cannot be read now, or
// whose state before was not known, counts as changed: the write is not reported as saved, but the file may have been replaced (the editor could not read it back in time).
func fileLeftAlone(path, before string) (bool, string) {
	if before == "" {
		return false, "the save file could not be compared with how it was before the write, so it may have been changed"
	}
	now, err := fileSHA(path)
	if err != nil {
		return false, fmt.Sprintf("the save file cannot be read now (%v), so it may have been changed", err)
	}
	if now != before {
		return false, "the save file is different from before the write"
	}
	return true, ""
}

// exportFolderOf is the dated folder, beside the state file, for the base files of this run.
func exportFolderOf(o Options, rec *checkRecord) string {
	return filepath.Join(filepath.Dir(o.StatePath), exportFolderPrefix+rec.At.Format("20060102-150405.000"))
}

// guardDeepDesert is the whole guard. done is true when the run must end now: with err when it refused or stopped (nothing was changed), and with err nil in a dry run that shows it
// would refuse. The checks of a dry run are the same reads; it takes no checkpoint and writes nothing.
func guardDeepDesert(o Options, r *Result, st *state, now time.Time) (done bool, err error) {
	var rec checkRecord
	rec.At = now
	stop := func(outcome string, e error) (bool, error) {
		if o.DryRun {
			r.say("Dry run: it would stop here: %v.", e)
			return true, nil
		}
		if rec.written {
			outcome += " (bases were recycled and written earlier in this run)"
		}
		st.LastCheck = &rec
		if werr := saveRunErr(o.StatePath, *st, now, outcome); werr != nil && (rec.written || rec.Intent != nil) && rec.stateWarn == "" {
			// after the save was written (or while a note of the write is still in the state file) a state file that cannot be written is said loudly, never dropped
			rec.stateWarn = fmt.Sprintf("WARNING: the state file %s could not be updated (%v). Keep this output: it names the way back", filepath.Base(o.StatePath), werr)
		}
		if rec.written {
			n := rec.recycledNote()
			r.say("%s", n)
			e = fmt.Errorf("%v. %s", e, n)
		} else if rec.stateWarn != "" {
			r.say("%s", rec.stateWarn)
			e = fmt.Errorf("%v. %s", e, rec.stateWarn)
		}
		return true, e
	}
	if o.SavePath == "" {
		why := o.SaveProblem
		if why == "" {
			why = "no save is known to check"
		}
		rec.Blockers = []string{why}
		r.say("Not changed: %s. The roll looks at the save first (a base, the character or a vehicle in the Deep Desert would be left inside new terrain), so it needs the save; pass --save with the save file or its folder.", why)
		return stop("not changed: the save to check is not known", fmt.Errorf("not changed: %s", why))
	}
	s, err := openSave(o.SavePath)
	if err != nil {
		rec.Blockers = []string{"the save could not be read: " + err.Error()}
		r.say("Not changed: the save could not be read (%v), so the Deep Desert cannot be checked.", err)
		return stop("not changed: the save could not be read", fmt.Errorf("not changed: the save could not be read (%v)", err))
	}
	defer s.Close()

	// 2. the checkpoint, before anything else is done with the save
	if o.DryRun {
		r.say("Dry run: would first take a checkpoint of the whole save (a verified copy in the save's backup folder, tagged -layoutroll, never removed by the automatic pruning).")
	} else {
		cp, err := takeCheckpoint(s)
		if err != nil {
			rec.Blockers = []string{"the checkpoint of the save could not be taken: " + err.Error()}
			r.say("Not changed: the checkpoint of the save could not be taken (%v).", err)
			return stop("not changed: the checkpoint of the save could not be taken", fmt.Errorf("not changed: the checkpoint of the save could not be taken (%v)", err))
		}
		rec.Checkpoint, rec.CheckpointSHA256, rec.CheckpointPath = cp.Name, cp.SHA256, cp.Path
		r.say("Checkpoint of the save: %s (%d bytes, SHA-256 %s), verified. It stays in the save's backup folder.", cp.Name, cp.Size, cp.SHA256)
	}

	// 3. the read
	h, err := inspectSave(s)
	if err != nil {
		rec.Blockers = []string{"the save could not be checked: " + err.Error()}
		r.say("Not changed: the save could not be checked (%v).", err)
		return stop("not changed: the save could not be checked", fmt.Errorf("not changed: the save could not be checked (%v)", err))
	}
	r.say("Deep Desert check: %d base(s) and %d vehicle(s) of yours looked at, and the character.", h.Checked.Bases, h.Checked.Vehicles)

	// 3a (#1167, only with --recycle-bases). The bases of the player in the Deep Desert are recycled, one at a time, when nothing else blocks the roll (recycle.go). A run that stops
	// there ends the guard; one that recycled every base goes on, and the save is looked at again: no base may be left.
	if o.RecycleBases {
		// the state file is written at once after the save has been written (the run could die after that): the history of the earlier runs stays, this run's entry is added
		rec.history = st.historyLinesExcept(rec.At)
		write := func(outcome string) error {
			if o.DryRun {
				return nil
			}
			st.recordRecycleRun(&rec)
			rec.history = st.historyLinesExcept(rec.At)
			st.LastCheck = &rec
			return saveRunErr(o.StatePath, *st, now, outcome)
		}
		sw := stateWriter{
			intent:   write, // the history entry then holds the checkpoint and the planned bases, marked as not confirmed
			persist:  write,
			withdraw: func() { st.dropRecycleRun(rec.At) },
		}
		out, err := recycleStep(o, r, &rec, s, h, stop, sw, rec.history)
		if out.done {
			return true, err
		}
		if rec.written {
			// recycled and read back; the way back is listed again with the result, whatever comes next
			r.say("Checkpoints of the runs that recycled bases, oldest first (any of them restores the save to how it was before that run): %s.", strings.Join(st.historyLines(), "; "))
		}
		switch {
		case out.planned:
			h.Bases = nil // the plan above is what a real run does; nothing was written, so there is nothing to look at again
		case len(rec.Recycled) > 0:
			if h, err = inspectSave(s); err != nil {
				rec.Blockers = append(rec.Blockers, "the save could not be checked again: "+err.Error())
				r.say("Stopped: the save could not be checked again after the bases were recycled (%v).", err)
				_, e := stop("stopped: the save could not be checked again", fmt.Errorf("stopped: the save could not be checked again (%v)", err))
				return true, e
			}
			r.say("Deep Desert check again: %d base(s) of yours still stand in the Deep Desert.", len(h.Bases))
			if len(h.Bases) > 0 {
				msg := fmt.Sprintf("the check after the recycle still finds %d base(s) of yours in the Deep Desert (%s)", len(h.Bases), strings.Join(blockerLines(h), "; "))
				rec.Blockers = append(rec.Blockers, msg)
				r.say("Stopped: %s.", msg)
				_, e := stop("stopped: a base is still in the Deep Desert after the recycle", errors.New("stopped: "+msg))
				return true, e
			}
		}
	}

	// 3b (#1174). The vehicles of the player in the Deep Desert are stored or moved out, but only when nothing else blocks the roll: a run that will refuse changes nothing.
	if len(h.Vehicles) > 0 && !h.BlockedBesidesVehicles() {
		if done, err := clearVehicles(o, r, &rec, s, stop); done {
			return true, err
		}
		if o.DryRun {
			h.Vehicles = nil // the plan above is what a real run does; nothing was written, so there is nothing to look at again
		} else {
			// the check again, on the save as written: the vehicles must now be out, and the guard must find nothing of them
			if h, err = inspectSave(s); err != nil {
				rec.Blockers = []string{"the save could not be checked again: " + err.Error()}
				r.say("Stopped: the save could not be checked again after the vehicles were taken out (%v). The checkpoint %s holds the save as it was before.", err, rec.Checkpoint)
				return stop("stopped: the save could not be checked again", fmt.Errorf("stopped: the save could not be checked again (%v)", err))
			}
			r.say("Deep Desert check again: %d vehicle(s) of yours still stand in the Deep Desert.", len(h.Vehicles))
			if len(h.Vehicles) > 0 {
				// the edit was written and read back, yet the guard still finds a vehicle: not expected; the layout is not rolled, and the save has been changed, so say so
				msg := fmt.Sprintf("the save WAS changed, but the check after it still finds %d vehicle(s) of yours in the Deep Desert (%s). The checkpoint %s holds the save as it was before; Database > Save backups restores it", len(h.Vehicles), strings.Join(blockerLines(h), "; "), rec.Checkpoint)
				rec.Blockers = []string{msg}
				r.say("Stopped: %s.", msg)
				return stop("stopped: a vehicle is still in the Deep Desert after the edit", errors.New("stopped: "+msg))
			}
		}
	}
	if !h.Blocked() {
		r.say("Nothing of yours stands in the Deep Desert: no base, not the character, no vehicle.")
		if !o.DryRun {
			st.LastCheck = &rec
		}
		return false, nil
	}
	rec.Blockers = blockerLines(h)
	for _, b := range rec.Blockers {
		r.say("  - %s", b)
	}

	// 4. the bases are exported before the refusal, so a copy of each exists
	if len(h.Bases) > 0 {
		folder := exportFolderOf(o, &rec)
		if o.DryRun {
			r.say("Dry run: would write a console base backup file of each of the %d base(s) to %s and read each back.", len(h.Bases), folder)
		} else {
			rec.ExportFolder = folder
			files, err := exportBases(s, h.Bases, folder, now, o.Editor)
			rec.ExportFiles = files
			for _, f := range files {
				r.say("Exported and read back: %s", filepath.Join(folder, f))
			}
			if err != nil {
				rec.Blockers = append(rec.Blockers, "a base could not be exported: "+err.Error())
				r.say("Stopped: %v. Nothing else was changed.", err)
				return stop("stopped: a base could not be exported", fmt.Errorf("stopped: %v", err))
			}
		}
	}
	r.say("Not changed: %s", refusalSummary(h))
	r.say("What to do: take the character out of the Deep Desert; pick up each base with the game's base tool, which also keeps it as a base backup (the files above are extra copies). Your vehicles there are stored or moved by this run itself once nothing else blocks it. Then run this again. Nothing was moved or deleted by this run.")
	if o.DryRun {
		r.say("Dry run: it would refuse for the reasons above.")
		return true, nil
	}
	st.LastCheck = &rec
	saveRun(o.StatePath, *st, now, "not changed: "+refusalSummary(h))
	return true, fmt.Errorf("not changed: %s", refusalSummary(h))
}

// clearVehicles is the vehicle step (#1174): it takes the player's vehicles out of the Deep Desert, or, in a dry run, says what it would do. done is true when the run must end now.
// A real run: the save lock (a second editor would write over it), ONE edit that stores or moves every vehicle and reads the rows back inside itself, the write of the save through the
// editor's own write path (which refuses while the game runs and takes its own backup), and a check of the file that was written, opened afresh. A failure before the write leaves the
// save as it was; a failure after it says the save WAS changed and names the checkpoint that restores it.
func clearVehicles(o Options, r *Result, rec *checkRecord, s *save.Save, stop func(string, error) (bool, error)) (done bool, err error) {
	if o.DryRun {
		plan, err := planVehicles(s)
		if err != nil {
			rec.Blockers = []string{"the vehicles cannot be taken out of the Deep Desert: " + err.Error()}
			r.say("Not changed: the vehicles of yours in the Deep Desert cannot be taken out (%v).", err)
			return stop("not changed: the vehicles could not be placed", fmt.Errorf("not changed: the vehicles cannot be taken out (%v)", err))
		}
		r.say("Dry run: would take %d vehicle(s) of yours out of the Deep Desert, in one edit of the save that is read back (after the checkpoint):", len(plan))
		for _, l := range ops.VehicleMoveLines(plan) {
			r.say("  - %s", l)
		}
		return false, nil
	}
	fail := func(outcome string, format string, a ...any) (bool, error) {
		msg := fmt.Sprintf(format, a...)
		rec.Blockers = []string{msg}
		r.say("Not changed: %s", msg)
		return stop(outcome, errors.New("not changed: "+msg))
	}
	release, err := lockSave(s.Path)
	if err != nil {
		return fail("not changed: the save is in use", "the save is open in another TABR TAU window or program, so it is not written (%v). Close that and run this again", err)
	}
	defer release()
	plan, err := moveVehicles(s)
	if err != nil {
		return fail("not changed: the vehicles could not be taken out of the Deep Desert", "the vehicles of yours in the Deep Desert could not be taken out (%v); the save was not changed", err)
	}
	if len(plan) == 0 {
		return fail("not changed: the vehicles could not be taken out of the Deep Desert", "the check found vehicles of yours in the Deep Desert but the plan for them is empty (a bug); the save was not changed")
	}
	// a write that is not reported as saved may still have replaced the file (it could not be read back in time): the file is compared with how it was before the write
	// the file as it is now must be known: without that a write could not be told from no write afterwards, so a save whose file cannot be read is not written
	before, berr := fileSHA(s.Path)
	if berr != nil || before == "" {
		s.Discard()
		return fail("not changed: the save file could not be read before the write", "the save file cannot be read before the write (%v), so a write could not be told from no write afterwards; the save was not written", berr)
	}
	f := flushSave(s)
	changed := func(outcome, why string) (bool, error) {
		msg := fmt.Sprintf("the save WAS changed (%d vehicle(s) taken out of the Deep Desert) but %s. The checkpoint %s holds the save as it was before; Database > Save backups restores it. Look at it before the game starts", len(plan), why, rec.Checkpoint)
		rec.Blockers = []string{msg}
		r.say("Stopped: %s.", msg)
		return stop(outcome, errors.New("stopped: "+msg))
	}
	if !f.Saved {
		same, why := fileLeftAlone(s.Path, before)
		if !same {
			// the file is different, but what made it different is not known until it is read: it is opened afresh and checked against the plan before anything is said about vehicles
			if verr := verifyWritten(s.Path, plan); verr != nil {
				msg := fmt.Sprintf("the save file is different from before the write, and it does not read back as the vehicles taken out (%v). The write was not reported as saved (%s) and %s, so it is NOT known what changed the file (the vehicles in part, or another program). The checkpoint %s holds the save as it was before; Database > Save backups restores it. Look at it before the game starts", verr, f.Why, why, rec.Checkpoint)
				rec.Blockers = []string{msg}
				r.say("Stopped: %s.", msg)
				return stop("stopped: the save file changed and does not read back as planned", errors.New("stopped: "+msg))
			}
			return changed("stopped: the write was not confirmed and the save file changed", "the write was not reported as saved ("+f.Why+") and "+why)
		}
		s.Discard() // the edit stays out of the file; throw it away so nothing is left queued
		return fail("not changed: the vehicles could not be written", "the vehicles were planned but the save could not be written (%s); the save was not changed", f.Why)
	}
	// from here the save file HAS been changed
	if f.Warning != "" {
		return changed("stopped: the written save needs a look", "the write reported: "+f.Warning)
	}
	if err := verifyWritten(s.Path, plan); err != nil {
		return changed("stopped: the written save does not read back", "the file does not read back as written ("+err.Error()+")")
	}
	rec.Vehicles = ops.VehicleMoveLines(plan)
	r.say("Took %d vehicle(s) of yours out of the Deep Desert in one edit of the save, read back inside the edit and again from the written file:", len(plan))
	for _, l := range rec.Vehicles {
		r.say("  - %s", l)
	}
	return false, nil
}

// blockerLines is one plain line per thing that blocks the roll.
func blockerLines(h ops.DeepDesertHolds) []string {
	var out []string
	for _, b := range h.Bases {
		out = append(out, fmt.Sprintf("base %d %q stands %s (x %.0f, y %.0f)", b.ID, b.Name, placeName(b.Map), b.X, b.Y))
	}
	if c := h.Character; c != nil {
		out = append(out, fmt.Sprintf("the character is %s (its %s; x %.0f, y %.0f)", placeName(c.Map), c.Which, c.X, c.Y))
	}
	for _, v := range h.Vehicles {
		out = append(out, fmt.Sprintf("vehicle %d (%s) is %s", v.ID, v.Name, placeName(v.Map)))
	}
	out = append(out, h.Problems...)
	return out
}

func placeName(m string) string {
	if m == "" {
		return "on a map the save does not name (counted as the Deep Desert)"
	}
	return "in the Deep Desert (" + m + ")"
}

// refusalSummary is the short reason, for the screen and for "lastRun" in the state file.
func refusalSummary(h ops.DeepDesertHolds) string {
	var parts []string
	if n := len(h.Bases); n > 0 {
		parts = append(parts, fmt.Sprintf("%d base(s)", n))
	}
	if h.Character != nil {
		parts = append(parts, "the character")
	}
	if n := len(h.Vehicles); n > 0 {
		parts = append(parts, fmt.Sprintf("%d vehicle(s)", n))
	}
	if n := len(h.Problems); n > 0 {
		parts = append(parts, fmt.Sprintf("%d thing(s) that could not be read", n))
	}
	s := ""
	for i, p := range parts {
		switch {
		case i == 0:
			s = p
		case i == len(parts)-1:
			s += " and " + p
		default:
			s += ", " + p
		}
	}
	return s + " of yours in the Deep Desert; the layout was not rolled"
}

// exportBases writes the console base backup file of each base into folder (made new, never an existing one) and reads each back: the bytes must be the ones written and the editor's
// own reader must take the file. It returns the names written so far, and the first failure.
func exportBases(s *save.Save, bases []ops.HoldBase, folder string, at time.Time, editor string) ([]string, error) {
	if err := os.MkdirAll(filepath.Dir(folder), 0o700); err != nil {
		return nil, fmt.Errorf("the folder for the base files could not be made (%v)", err)
	}
	if err := os.Mkdir(folder, 0o700); err != nil {
		return nil, fmt.Errorf("the folder for the base files could not be made (%v)", err)
	}
	var written []string
	for _, b := range bases {
		out, err := exportBaseFile(s, b.ID, at, editor)
		if err != nil {
			return written, fmt.Errorf("base %d %q could not be exported (%v)", b.ID, b.Name, err)
		}
		if out == nil || len(out.Data) == 0 || filepath.Base(out.FileName) != out.FileName || out.FileName == "" {
			return written, fmt.Errorf("base %d %q could not be exported (no usable file was made)", b.ID, b.Name)
		}
		dest := filepath.Join(folder, out.FileName)
		if err := durable.WriteNew(dest, out.Data, 0o600, durable.Options{}); err != nil {
			return written, fmt.Errorf("the file of base %d %q could not be written (%v)", b.ID, b.Name, err)
		}
		written = append(written, out.FileName)
		back, err := os.ReadFile(dest)
		if err != nil || !bytes.Equal(back, out.Data) {
			return written, fmt.Errorf("the file of base %d %q does not read back as written", b.ID, b.Name)
		}
		f, err := basefile.Load(back, basefile.MaxFileBytes, basefile.MaxRows)
		if err == nil {
			err = f.Verify()
		}
		if err != nil {
			return written, fmt.Errorf("the file of base %d %q is not taken back by the editor's own reader (%v)", b.ID, b.Name, err)
		}
	}
	if len(written) != len(bases) {
		return written, errors.New("not every base was exported (a bug)")
	}
	return written, nil
}

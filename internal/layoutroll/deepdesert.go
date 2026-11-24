package layoutroll

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/durable"
)

// The Deep Desert guard (#1167). A layout change puts new terrain under the Deep Desert, so a base, the character or a vehicle left there is left inside it. Before Game.ini is
// touched the run (with the game closed): takes a verified checkpoint of the whole save, reads the save and refuses the roll when a base of the player, the character or a vehicle
// of the player is in the Deep Desert, and exports each such base to a console base backup file first. The save is only ever read: the checkpoint copies it into the backup folder,
// nothing is written to it. Nothing is moved, deleted or picked up by this run.

// exportFolderPrefix names the dated folder (beside the state file) that holds the base files of one refused run.
const exportFolderPrefix = "layout-roll-bases-"

// The steps that touch the save. They are variables only so a test can make one fail (a checkpoint that cannot be taken, a file that cannot be written); nothing else changes them.
var (
	openSave       = save.Open
	takeCheckpoint = func(s *save.Save) (save.ManualBackup, error) { return s.BackupCheckpoint() }
	inspectSave    = func(s *save.Save) (ops.DeepDesertHolds, error) { return (&ops.Ops{S: s}).DeepDesertHolds() }
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
		st.LastCheck = &rec
		saveRun(o.StatePath, *st, now, outcome)
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
		rec.Checkpoint, rec.CheckpointSHA256 = cp.Name, cp.SHA256
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
		folder := filepath.Join(filepath.Dir(o.StatePath), exportFolderPrefix+now.Format("20060102-150405.000"))
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
	r.say("What to do: take the character out of the Deep Desert; move or store each vehicle; pick up each base with the game's base tool, which also keeps it as a base backup (the files above are extra copies). Then run this again. Nothing was moved or deleted by this run.")
	if o.DryRun {
		r.say("Dry run: it would refuse for the reasons above.")
		return true, nil
	}
	st.LastCheck = &rec
	saveRun(o.StatePath, *st, now, "not changed: "+refusalSummary(h))
	return true, fmt.Errorf("not changed: %s", refusalSummary(h))
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
		kind := "placed"
		if v.Stored {
			kind = "stored"
		}
		out = append(out, fmt.Sprintf("vehicle %d (%s, %s) is %s", v.ID, v.Name, kind, placeName(v.Map)))
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

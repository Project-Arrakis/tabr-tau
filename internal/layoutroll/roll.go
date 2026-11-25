package layoutroll

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/kit/durable"
)

const (
	// FileName is the game's own config file that holds the setting. It is the only existing file this package changes; it also writes its backups, its state file, the base files of a refused run (deepdesert.go) and, through the save package, one checkpoint copy of the save in the save's backup folder. The save itself is written only by the vehicle step, the removal of the Deep Desert markers (markers.go) and, with RecycleBases, the recycle of the Deep Desert bases (deepdesert.go, recycle.go).
	FileName = "Game.ini"
	// KeepBackups is how many timestamped copies of Game.ini are kept beside it.
	KeepBackups = 10
	// maxIni is the largest Game.ini that is read; the game's is a few kilobytes.
	maxIni = 1 << 20
	// backupPrefix and backupSuffix name a backup: Game.ini.layout-roll-20261124-050001.123.bak
	backupPrefix = FileName + ".layout-roll-"
	backupSuffix = ".bak"
)

var backupName = regexp.MustCompile(`^` + regexp.QuoteMeta(backupPrefix) + `\d{8}-\d{6}\.\d{3}` + regexp.QuoteMeta(backupSuffix) + `$`)

// Options is one run of the roll. Every outside thing it touches can be replaced, so a test never goes near a real Game.ini, the process list or the clock.
type Options struct {
	ConfigDir string // the game's Saved\Config\Windows folder
	StatePath string // the small file that records the last roll (beside the editor's settings)
	DryRun    bool   // say what would be done; write nothing, not even the state file
	Force     bool   // roll although one already happened in the current week
	Set       *int   // a chosen layout (0 to 11) instead of a random one; an explicit choice is not held back by the weekly rule
	// SavePath is the save to check (game.db or its folder) before Game.ini is touched (#1167); SaveProblem says why it is empty when the caller could not tell which save it is.
	// The run writes it only when it recycles bases (RecycleBases), moves vehicles out of the Deep Desert or clears the Deep Desert markers (KeepMarkers off); it also copies the verified checkpoint into the save's backup folder. Editor is the version written into the base files that are exported.
	SavePath    string
	SaveProblem string
	Editor      string
	// RecycleBases switches on the step that recycles the player's Deep Desert bases before the roll (#1167, recycle.go). Off, a standing base refuses the roll, as it always did.
	RecycleBases bool
	// KeepMarkers leaves the Deep Desert's markers in the save (#1216). Off, a real run removes them (markers.go) after the bases and vehicles and before Game.ini is changed.
	KeepMarkers bool
	// lockHeld is set by Roll on its own copy of the options once it holds the save's lock (#1201). Nothing reads or writes layout-roll.json's notes without it in a real run.
	lockHeld bool
	Now      func() time.Time
	Intn     func(n int) int
	// VettedFlag is the list given with --vetted (#1219): when set (even empty, which is refused) it is used instead of layout-roll-vetted.txt for this run. A random roll picks only from the vetted list.
	VettedFlag *string
	// GameRunning says whether the game may be running and, if so, why (the process found, or that the check could not be made: a check that cannot run counts as running).
	GameRunning func() (bool, string)
}

// Result is what a run did, in lines the command prints.
type Result struct {
	Lines []string
	Wrote bool // Game.ini was changed
	Old   int
	New   int
}

func (r *Result) say(format string, a ...any) { r.Lines = append(r.Lines, fmt.Sprintf(format, a...)) }

// Roll is one run: the guards in order, then the backup, the write and the read-back. It returns the lines to print. A non-nil error means nothing was changed, or (when the text says
// so) that the write needs a look; the lines say what was done up to that point.
func Roll(o Options) (Result, error) {
	var r Result
	if o.Now == nil || o.Intn == nil || o.GameRunning == nil {
		return r, errors.New("layout-roll: not set up (a bug)")
	}
	now := o.Now()
	if o.Set != nil && (*o.Set < MinIndex || *o.Set > MaxIndex) {
		return r, fmt.Errorf("--set takes a layout from %d to %d, not %d", MinIndex, MaxIndex, *o.Set)
	}
	if err := plainFolder(o.ConfigDir); err != nil {
		return r, err
	}
	path := filepath.Join(o.ConfigDir, FileName)
	old, existed, mode, err := readIni(path)
	if err != nil {
		return r, err
	}
	cur, present, err := CurrentIndex(old)
	if err != nil {
		return r, err
	}
	// The roll lock (#1201): a real run holds the save's own lock (the one the editor and every other write to the save take; no second lock exists) from BEFORE the state file is read until
	// the run has ended, so two runs never interleave their notes and their writes of layout-roll.json: a second run is refused here, at once, and changes nothing. A dry run takes no lock:
	// it writes nothing (no save, no state file, no Game.ini), so it cannot spoil another run, and it must keep working while the editor has the save open; it says so.
	if !o.DryRun && o.SavePath != "" {
		release, err := lockSave(lockedSavePath(o.SavePath))
		if err != nil {
			// The state file is NOT touched (that is the point of the lock), so the refusal is made visible in an append-only log beside it (layout-roll.log): a scheduled run has no
			// window and no console, and the task shows only its exit code, which is non-zero (the error below).
			noteRefusal(o, now, "layout roll refused: the save is open in another TABR TAU window (or another layout roll is running): nothing changed ("+err.Error()+")")
			return r, fmt.Errorf("not changed: %s", lockRefusal(err))
		}
		defer release()
		o.lockHeld = true
	}
	st, err := loadState(o.StatePath)
	if err != nil {
		if !o.Force {
			return r, fmt.Errorf("%v; fix or delete that file, or run with --force", err)
		}
		r.say("note: %v (ignored because of --force)", err)
		st = state{}
	}
	if present {
		r.say("Game.ini now sets layout %d.", cur)
	} else {
		r.say("Game.ini has no layout line, so the game uses its own default, layout %d.", cur)
	}
	if st.LastRun != nil {
		r.say("Last run: %s, %s.", st.LastRun.At.Local().Format("2006-01-02 15:04"), st.LastRun.Outcome)
	}
	if st.LastRoll != nil {
		r.say("Last roll: %s, layout %d -> %d.", st.LastRoll.At.Local().Format("2006-01-02 15:04"), st.LastRoll.From, st.LastRoll.To)
	} else {
		r.say("No roll is recorded yet.")
	}
	if o.DryRun {
		r.say("Dry run: the lock on the save, which a real run holds from before it reads this record until it has ended (a second layout roll is refused while it is held), is not taken: a dry run writes nothing.")
	}

	// the vetted list (#1219): a random roll picks only from it. A list that cannot be used refuses the run here and writes nothing but the note of the refusal. --set is the person's own choice.
	var vet vettedChoice
	if o.Set == nil {
		var verr error
		if vet, verr = chooseVetted(o, old, cur); verr != nil {
			if !o.DryRun {
				if o.lockHeld {
					saveRun(o.StatePath, st, now, "not changed: "+verr.Error())
				} else {
					noteRefusal(o, now, "layout roll refused: "+verr.Error())
				}
			}
			return r, fmt.Errorf("not changed: %v", verr)
		}
		for _, l := range vet.lines(cur) {
			r.say("%s", l)
		}
	}

	// guard 1: once per Tuesday-05:00 week (not for a chosen layout, and not with --force)
	ws := WeekStart(now)
	if o.Set == nil && !o.Force && st.LastRoll != nil && !st.LastRoll.At.Before(ws) {
		r.say("Skipped: a roll already happened in the week that began %s. Use --force to roll again.", ws.Format("Mon 2006-01-02 15:04"))
		return r, nil
	}

	// choose
	var next int
	if o.Set != nil {
		next = *o.Set
		if next == cur && present {
			r.say("Nothing to do: Game.ini already sets layout %d.", cur)
			return r, nil
		}
	} else {
		if next, err = Pick(cur, vet.list, o.Intn); err != nil {
			return r, fmt.Errorf("not changed: %v", err)
		}
	}
	text, err := SetIndex(old, next)
	if err != nil {
		return r, err
	}
	// the edit is checked in memory before anything is written: it must read back as the layout asked for, and nothing but the number (or the added line) may differ
	if got, _, err := CurrentIndex(text); err != nil || got != next {
		return r, fmt.Errorf("the edited text does not read back as layout %d (a bug); nothing is changed", next)
	}
	r.Old, r.New = cur, next

	// guard 2: the game must not be running. Checked now and again right before the write.
	if running, why := o.GameRunning(); running {
		r.say("Not changed: %s. Game.ini is rewritten by the game when it exits, so it is left alone while the game runs. Close the game and run this again (or wait for the next scheduled run).", why)
		if o.DryRun {
			r.say("Dry run: it would refuse now for that reason.")
			return r, nil
		}
		if o.lockHeld {
			saveRun(o.StatePath, st, now, "not changed: the game is running")
		} else {
			noteRefusal(o, now, "layout roll refused: "+why+": nothing changed")
		}
		return r, fmt.Errorf("not changed: %s", why)
	}

	// guard 3 (#1167): the Deep Desert. A checkpoint of the save, then a read of it; the roll is refused while a base, the character or a vehicle of the player is in the Deep Desert,
	// and each such base is exported first. --set and --force do not skip it.
	if done, err := guardDeepDesert(o, &r, &st, now); done {
		return r, err
	}
	// a failure after this point may come after bases were recycled and written (--recycle-bases): the message then says so and names the way back
	late := func(err error) error {
		if c := st.LastCheck; c != nil && c.savedChanged() {
			return fmt.Errorf("%v. %s", err, c.writtenNote())
		}
		return err
	}

	// the Deep Desert markers (#1216): counted in a dry run, removed in a real one right before Game.ini is written (below), so a run that stops earlier has not touched them
	if o.DryRun {
		if err := clearDeepDesertMarkers(o, &r, &st, now); err != nil {
			if errors.Is(err, errWouldStop) {
				r.say("Nothing was written.")
				return r, nil // it said it would stop here: nothing more is said about Game.ini
			}
			return r, err
		}
	}

	if o.DryRun {
		pick := ""
		if o.Set == nil {
			pick = " (a random pick; a real run picks again)"
		}
		r.say("Dry run: would change Game.ini from layout %d to layout %d%s (%s).", cur, next, pick, describe(present, existed))
		r.say("Dry run: would first copy Game.ini to %s<time>%s beside it (the newest %d copies are kept).", backupPrefix, backupSuffix, KeepBackups)
		r.say("Nothing was written. The game must be restarted for a new layout to apply.")
		return r, nil
	}

	// the game must still not be running and Game.ini must still be what was read: checked before the save is written and again right before Game.ini is (the markers step takes time)
	stillSafe := func() error {
		if running, why := o.GameRunning(); running {
			outcome := "not changed: the game is running"
			if c := st.LastCheck; c != nil && c.savedChanged() {
				outcome = "stopped: Game.ini was not changed because the game started during the run, but the save WAS changed earlier in this run (checkpoint " + checkpointName(c) + ")"
			}
			saveRun(o.StatePath, st, now, outcome)
			return late(fmt.Errorf("not changed: %s (it started while this ran)", why))
		}
		again, _, _, err := readIni(path)
		if err != nil || again != old {
			if c := st.LastCheck; c != nil && c.savedChanged() && o.lockHeld {
				saveRun(o.StatePath, st, now, "stopped: Game.ini was not changed because it changed while the run was going, but the save WAS changed earlier in this run (checkpoint "+checkpointName(c)+")")
			}
			return late(errors.New("not changed: Game.ini changed while this ran; run it again"))
		}
		return nil
	}
	if err := stillSafe(); err != nil {
		return r, err
	}
	// every write to the save is done before Game.ini is changed: the Deep Desert markers go now, in an edit of their own that is read back
	if err := clearDeepDesertMarkers(o, &r, &st, now); err != nil {
		return r, err
	}
	// a stop after the save was written by this run leaves the last word in the state file and the log: without it layout-roll.json would keep "in progress ... Game.ini is next"
	iniNotChanged := func(why string) {
		c := st.LastCheck
		if c == nil || !c.savedChanged() {
			return
		}
		outcome := "stopped: Game.ini was not changed (" + why + "), the save WAS changed earlier in this run (checkpoint " + checkpointName(c) + ")"
		if o.lockHeld {
			saveRun(o.StatePath, st, now, outcome)
		}
		noteLog(o, now, "layout roll: "+outcome)
	}
	// the same two checks again, right before Game.ini is written; the backup is made only after them, so a run that stopped earlier has used no slot of the newest 10 on a copy that no change followed
	if err := stillSafe(); err != nil {
		return r, err
	}
	// backup, written exclusively and compared with what was read
	backup := ""
	if existed {
		if backup, err = writeBackupFile(o.ConfigDir, old, now, mode); err != nil {
			iniNotChanged("the backup could not be made")
			return r, late(fmt.Errorf("not changed: the backup could not be made (%v)", err))
		}
		r.say("Backup: %s", filepath.Base(backup))
		if n := pruneBackups(o.ConfigDir); n > 0 {
			r.say("Removed %d older backup(s); the newest %d are kept.", n, KeepBackups)
		}
	}
	if mode == 0 {
		mode = 0o644
	}
	if err := writeIniFile(path, []byte(text), mode, writeOptions()); err != nil {
		iniNotChanged("the file could not be written")
		return r, late(fmt.Errorf("the file Game.ini could not be written (%v); it was left as it was%s", err, backupNote(backup)))
	}
	back, _, _, err := readIni(path)
	if err != nil {
		return r, fmt.Errorf("the file Game.ini was written but could not be read back to check it (%v); look at it before the game starts%s", err, backupNote(backup))
	}
	if back != text {
		return r, fmt.Errorf("the file Game.ini was written but now holds other text than was written (another program changed it?); look at it before the game starts%s", backupNote(backup))
	}
	if got, _, err := CurrentIndex(back); err != nil || got != next {
		return r, fmt.Errorf("the file Game.ini was written but does not read back as layout %d; look at it before the game starts%s", next, backupNote(backup))
	}
	r.Wrote = true
	r.say("Changed Game.ini from layout %d to layout %d (%s) and read it back to check it.", cur, next, describe(present, existed))
	r.say("Restart the game for it to apply. A base stands on different terrain on each layout.")
	pickNote := ""
	if o.Set == nil {
		pickNote = vet.note()
	}
	if err := saveRoll(o.StatePath, st, now, cur, next, pickNote); err != nil {
		if c := st.LastCheck; c != nil && c.savedChanged() {
			// bases were recycled or vehicles taken out and written in this run: the state file is where the way back is kept, so a failure to write it is said loudly
			what := "the recycled bases"
			if c.vehiclesWritten {
				what = "the vehicles taken out of the Deep Desert"
				if c.written {
					what = "the recycled bases and the vehicles taken out of the Deep Desert"
				}
			}
			if c.markersWritten {
				switch {
				case c.written || c.vehiclesWritten:
					what += " and the removed Deep Desert markers"
				default:
					what = "the removed Deep Desert markers"
				}
			}
			r.say("WARNING: the record of this roll and of %s could not be saved (%v); the roll itself is done. %s", what, err, c.writtenNote())
		} else {
			r.say("Note: the record of this roll could not be saved (%v); the roll itself is done.", err)
		}
	}
	return r, nil
}

func describe(present, existed bool) string {
	switch {
	case present:
		return "only the number in the existing line changes"
	case existed:
		return "the layout line is added; the rest of the file stays as it is"
	}
	return "Game.ini does not exist and is created with only that line"
}

func backupNote(backup string) string {
	if backup == "" {
		return ""
	}
	return "; the copy taken before the edit is " + filepath.Base(backup)
}

// plainFolder refuses a config folder that is missing, is not a folder, or is a link (a link would send the write and the backups somewhere else).
func plainFolder(dir string) error {
	if dir == "" {
		return errors.New("no game config folder is known (LOCALAPPDATA is not set); pass --config")
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("the game config folder cannot be read (%v); pass --config if it is elsewhere", err)
	}
	if st.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || !st.IsDir() {
		return errors.New("the game config folder is not a plain folder (links are refused)")
	}
	return nil
}

// readIni reads Game.ini. A missing file is fine (existed false, empty text). A link, a folder, a file over the limit, or text that is not plain 8-bit text is refused.
func readIni(path string) (text string, existed bool, mode fs.FileMode, err error) {
	st, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, 0, nil
	}
	if err != nil {
		return "", false, 0, err
	}
	if !st.Mode().IsRegular() {
		return "", true, 0, fmt.Errorf("%s is not a plain file (links are refused)", FileName)
	}
	if st.Size() > maxIni {
		return "", true, 0, fmt.Errorf("%s is larger than %d bytes; it is not changed", FileName, maxIni)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", true, 0, err
	}
	if bytes.HasPrefix(b, []byte{0xFF, 0xFE}) || bytes.HasPrefix(b, []byte{0xFE, 0xFF}) || bytes.IndexByte(b, 0) >= 0 {
		return "", true, 0, fmt.Errorf("%s is not 8-bit text (UTF-16?); it is not changed", FileName)
	}
	return string(b), true, st.Mode().Perm(), nil
}

// writeOptions is how Game.ini is replaced: a temp file next to it with the name the editor's config sweep knows (Game.ini.tabr-N.tmp), flushed before the rename.
func writeOptions() durable.Options {
	o := durable.Defaults()
	o.TempPattern = FileName + ".tabr-*.tmp"
	return o
}

// The two writes next to Game.ini, as variables only so that a test can make one fail; nothing else changes them.
var (
	writeBackupFile = writeBackup
	writeIniFile    = durable.WriteFile
)

// writeBackup puts the exact bytes of Game.ini in a new file beside it, never over an existing one, and reads it back.
func writeBackup(dir, content string, now time.Time, mode fs.FileMode) (string, error) {
	if mode == 0 {
		mode = 0o644
	}
	at := now
	for try := 0; try < 1000; try++ {
		dest := filepath.Join(dir, backupPrefix+at.Format("20060102-150405.000")+backupSuffix)
		if _, err := os.Lstat(dest); err == nil {
			at = at.Add(time.Millisecond)
			continue
		}
		if err := durable.WriteNew(dest, []byte(content), mode, durable.Options{}); err != nil {
			return "", err
		}
		b, err := os.ReadFile(dest)
		if err != nil || string(b) != content {
			return "", errors.New("the copy does not match the file")
		}
		return dest, nil
	}
	return "", errors.New("no free backup name")
}

// pruneBackups removes all but the newest KeepBackups backups that carry this package's exact backup name (plain files only; nothing else in the folder is touched). It returns how many it removed.
func pruneBackups(dir string) int {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var names []string
	for _, e := range ents {
		if backupName.MatchString(e.Name()) && e.Type().IsRegular() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // the name holds the time, so the oldest sort first
	removed := 0
	for len(names)-removed > KeepBackups {
		if os.Remove(filepath.Join(dir, names[removed])) == nil {
			removed++
		} else {
			break
		}
	}
	return removed
}

// WeekStart is the most recent Tuesday 05:00 (local time of now's zone) at or before now: the start of the week a roll belongs to.
func WeekStart(now time.Time) time.Time {
	d := (int(now.Weekday()) - int(time.Tuesday) + 7) % 7
	ws := time.Date(now.Year(), now.Month(), now.Day()-d, 5, 0, 0, 0, now.Location())
	if ws.After(now) {
		ws = time.Date(ws.Year(), ws.Month(), ws.Day()-7, 5, 0, 0, 0, now.Location())
	}
	return ws
}

// ---- the state file: the last roll and the last run, beside the editor's settings ----

type rollRecord struct {
	At   time.Time `json:"at"`
	From int       `json:"from"`
	To   int       `json:"to"`
}

type runRecord struct {
	At      time.Time `json:"at"`
	Outcome string    `json:"outcome"`
}

type state struct {
	LastRoll  *rollRecord  `json:"lastRoll,omitempty"`
	LastRun   *runRecord   `json:"lastRun,omitempty"`
	LastCheck *checkRecord `json:"lastCheck,omitempty"` // what the last run that looked at the save found: the checkpoint, the reasons for a refusal, the base files exported (#1167)

	// RecycleHistory is one entry per run that wrote recycled bases to the save, oldest first: LastCheck is replaced by the next run, so without this the checkpoint of an earlier run
	// (the way back to before the first recycle) would be lost from the state file when a later run recycles the next base (#1167).
	RecycleHistory []recycleHistoryEntry `json:"recycleHistory,omitempty"`

	// VehicleNote is the note a run leaves BEFORE it writes the save to take the vehicles out of the Deep Desert (#1201): the checkpoint, the plan and how the save file looked. It is removed
	// when the write is confirmed (or taken back when the save was not written); one that is still there means the run ended without recording how the write went, and the next run
	// settles it first (reconcile.go). There is at most one: runs are serialized by the save's lock, and a run never goes on past a note it could not settle.
	VehicleNote *vehicleNote `json:"vehicleNote,omitempty"`

	// VehicleHistory keeps what a settled vehicle note leaves when it is recorded as written by a LATER run: the earlier run's checkpoint (its way back) and the vehicles, because the
	// record of that run (lastCheck) was replaced and would otherwise be lost. Oldest first, capped at maxRecycleHistory.
	VehicleHistory []vehicleHistoryEntry `json:"vehicleHistory,omitempty"`
}

// vehicleHistoryEntry is one vehicle step whose write was found, by a later run, to have taken effect without having been recorded.
type vehicleHistoryEntry struct {
	At             time.Time `json:"at"` // the run that wrote the save
	Checkpoint     string    `json:"checkpoint"`
	CheckpointPath string    `json:"checkpointPath,omitempty"`
	SaveFile       string    `json:"saveFile,omitempty"`
	Vehicles       []int64   `json:"vehicles"` // their actor numbers
	SettledAt      time.Time `json:"settledAt"`
}

// recordSettledVehicleNote moves a vehicle note that was found to have taken effect into the vehicle history.
func (s *state) recordSettledVehicleNote(n vehicleNote, at time.Time) {
	e := vehicleHistoryEntry{At: n.At, Checkpoint: n.Checkpoint, CheckpointPath: n.CheckpointPath, SaveFile: n.SaveFile, SettledAt: at}
	for _, v := range n.Vehicles {
		e.Vehicles = append(e.Vehicles, v.ID)
	}
	s.VehicleHistory = append(s.VehicleHistory, e)
	if len(s.VehicleHistory) > maxRecycleHistory {
		s.VehicleHistory = s.VehicleHistory[len(s.VehicleHistory)-maxRecycleHistory:]
	}
}

// plannedVehicle is what a note keeps of one vehicle of the plan: enough to tell, from the save file alone, whether the vehicle is still where it was or has been taken out.
type plannedVehicle struct {
	ID      int64  `json:"id"`
	Action  string `json:"action"`          // "store" or "move"
	State   int64  `json:"state"`           // the actor's state before the edit (a stored vehicle is at state 2 afterwards)
	FromMap string `json:"fromMap"`         // the map the vehicle stood on (the Deep Desert)
	ToMap   string `json:"toMap,omitempty"` // move: the map it was put on
}

// vehicleNote is the unconfirmed note of the vehicle step (#1201), the counterpart of an unconfirmed recycleHistoryEntry.
type vehicleNote struct {
	At             time.Time        `json:"at"`
	Checkpoint     string           `json:"checkpoint"`
	CheckpointPath string           `json:"checkpointPath,omitempty"`
	SaveFile       string           `json:"saveFile,omitempty"`
	BeforeSHA256   string           `json:"saveSha256Before,omitempty"`
	Vehicles       []plannedVehicle `json:"vehicles"`
	// ReadBackFailed: the run that wrote the note opened the file it had written and it did not read back as planned; the next run stops on the note and never accepts the file
	ReadBackFailed bool `json:"readBackFailed,omitempty"`
}

// recordVehicleNote puts the note of the run that began at rec.At into the state while rec holds an intent for vehicles, and removes it when it does not (the write was confirmed, or the
// save was not written).
func (s *state) recordVehicleNote(rec *checkRecord) {
	if rec.Intent == nil || len(rec.Intent.Vehicles) == 0 {
		s.dropVehicleNote(rec.At)
		return
	}
	s.VehicleNote = &vehicleNote{At: rec.At, Checkpoint: rec.Checkpoint, CheckpointPath: rec.CheckpointPath, SaveFile: rec.Intent.SaveFile, BeforeSHA256: rec.Intent.BeforeSHA256,
		Vehicles: append([]plannedVehicle(nil), rec.Intent.Vehicles...), ReadBackFailed: rec.Intent.ReadBackFailed}
}

// dropVehicleNote removes the note of the run that began at the given time.
func (s *state) dropVehicleNote(at time.Time) {
	if s.VehicleNote != nil && s.VehicleNote.At.Equal(at) {
		s.VehicleNote = nil
	}
}

// maxRecycleHistory keeps the state file small; a weekly run adds at most one entry.
const maxRecycleHistory = 20

type recycleHistoryEntry struct {
	At             time.Time `json:"at"`
	Checkpoint     string    `json:"checkpoint"`
	CheckpointPath string    `json:"checkpointPath,omitempty"`
	SaveFile       string    `json:"saveFile,omitempty"`
	Bases          []int64   `json:"bases"`
	SolidoItems    []int64   `json:"solidoItems,omitempty"` // the ids of the Solido devices that hold the new Solidos

	// Unconfirmed is set on the entry written BEFORE the save is written (the bases are the ones about to be recycled): the entry is replaced when the write is confirmed and removed when
	// the save was not written, so one that stays says the run ended without recording how the write went.
	Unconfirmed bool `json:"unconfirmed,omitempty"`
	// BeforeSHA256 is the SHA-256 of the save file as it was before the write, kept on an unconfirmed entry: the next run compares it with the file as it is then (reconcile.go).
	BeforeSHA256     string  `json:"saveSha256Before,omitempty"`
	SolidoBlueprints []int64 `json:"solidoBlueprints,omitempty"` // as the Solido items, on an unconfirmed entry: what the next run checks the file for
	// ReadBackFailed: the run that wrote the note opened the file it had written and it did not read back as planned; the next run stops on the entry and never accepts the file
	ReadBackFailed bool `json:"readBackFailed,omitempty"`
}

// line is the entry as one plain sentence.
func (e recycleHistoryEntry) line() string {
	ids := func(l []int64) string {
		var p []string
		for _, v := range l {
			p = append(p, fmt.Sprintf("%d", v))
		}
		return strings.Join(p, ", ")
	}
	if e.Unconfirmed {
		return fmt.Sprintf("%s (run of %s; base(s) %s were about to be recycled; the run did not record whether the save was written: look at the save)", e.Checkpoint, e.At.Format("2006-01-02 15:04"), ids(e.Bases))
	}
	return fmt.Sprintf("%s (run of %s; base(s) %s; Solido device id(s) %s)", e.Checkpoint, e.At.Format("2006-01-02 15:04"), ids(e.Bases), ids(e.SolidoItems))
}

// recordRecycleRun adds or replaces (same run time) the history entry of the run that recycled the bases of rec, or, while the save has not been written yet, the entry for the bases
// the run is about to recycle (its Intent).
func (s *state) recordRecycleRun(rec *checkRecord) {
	e := recycleHistoryEntry{At: rec.At, Checkpoint: rec.Checkpoint, CheckpointPath: rec.CheckpointPath, SaveFile: rec.SaveFile}
	for _, b := range rec.Recycled {
		e.Bases = append(e.Bases, b.ID)
		e.SolidoItems = append(e.SolidoItems, b.SolidoItem)
	}
	if rec.Intent != nil && len(rec.Recycled) == 0 {
		e.Bases, e.Unconfirmed, e.BeforeSHA256 = rec.Intent.Bases, true, rec.Intent.BeforeSHA256
		e.SolidoItems, e.SolidoBlueprints = rec.Intent.SolidoItems, rec.Intent.SolidoBlueprints
		e.ReadBackFailed = rec.Intent.ReadBackFailed
	}
	replaced := false
	for i := range s.RecycleHistory {
		if s.RecycleHistory[i].At.Equal(rec.At) {
			s.RecycleHistory[i] = e
			replaced = true
			break
		}
	}
	if !replaced {
		s.RecycleHistory = append(s.RecycleHistory, e)
	}
	s.trimHistory()
}

// trimHistory keeps the newest maxRecycleHistory CONFIRMED entries. An unconfirmed entry (written before the save is written) does not count against the cap and is never removed
// by it: noting an intent must not push the oldest checkpoint out before anything has been written.
func (s *state) trimHistory() {
	confirmed := 0
	for _, e := range s.RecycleHistory {
		if !e.Unconfirmed {
			confirmed++
		}
	}
	var keep []recycleHistoryEntry
	for _, e := range s.RecycleHistory {
		if !e.Unconfirmed && confirmed > maxRecycleHistory {
			confirmed--
			continue
		}
		keep = append(keep, e)
	}
	s.RecycleHistory = keep
}

// historyLines is the history, oldest first, one sentence each.
func (s *state) historyLines() []string { return s.historyLinesExcept(time.Time{}) }

// historyLinesExcept is the history without the entry of the run that began at the given time (a run's own checkpoint is not one of the EARLIER runs'). The zero time leaves nothing out.
func (s *state) historyLinesExcept(at time.Time) []string {
	var l []string
	for _, e := range s.RecycleHistory {
		if !at.IsZero() && e.At.Equal(at) {
			continue
		}
		l = append(l, e.line())
	}
	return l
}

// dropRecycleRun removes the entry of the run that began at the given time.
func (s *state) dropRecycleRun(at time.Time) {
	var keep []recycleHistoryEntry
	for _, e := range s.RecycleHistory {
		if !e.At.Equal(at) {
			keep = append(keep, e)
		}
	}
	s.RecycleHistory = keep
}

const maxState = 64 << 10

// loadState reads the state file. A missing file is an empty state; one that cannot be read or is not the expected JSON is an error (the weekly rule cannot be applied on a guess).
func loadState(path string) (state, error) {
	var s state
	if path == "" {
		return s, errors.New("no place for the state file is known")
	}
	st, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("the state file cannot be read: %v", err)
	}
	if st.Size() > maxState {
		return s, errors.New("the state file is too large")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return s, fmt.Errorf("the state file cannot be read: %v", err)
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return state{}, fmt.Errorf("the state file %s is not valid", filepath.Base(path))
	}
	return s, nil
}

func writeState(path string, s state) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	o := durable.Defaults()
	o.TempPattern = filepath.Base(path) + ".*.tmp"
	return durable.WriteFile(path, append(b, '\n'), 0o600, o)
}

func saveRoll(path string, s state, now time.Time, from, to int, pickNote string) error {
	s.LastRoll = &rollRecord{At: now, From: from, To: to}
	outcome := fmt.Sprintf("rolled layout %d -> %d", from, to)
	if pickNote != "" {
		outcome += " (" + pickNote + ")"
	}
	// a warning of the recycle step that the person must still see the next day (the Reserve that could not be emptied after the last base) is part of the outcome
	if c := s.LastCheck; c != nil && c.ReserveNote != "" {
		outcome += "; WARNING: " + c.ReserveNote
	}
	if c := s.LastCheck; c != nil && c.Markers != "" {
		outcome += "; removed the Deep Desert markers: " + c.Markers
	}
	s.LastRun = &runRecord{At: now, Outcome: outcome}
	return writeStateFile(path, s)
}

// saveRun records a run that changed nothing, so the person can see on the next look why. A failure to write it is ignored: it is a note, not a guard.
func saveRun(path string, s state, now time.Time, outcome string) {
	_ = saveRunErr(path, s, now, outcome)
}

// LogName is the append-only log beside the state file. A refusal that cannot be recorded in the state file (the run holds no lock, so it must not read-modify-write it) is written here, as
// one dated line (#1201); the person finds it when a scheduled run, which has no window, did nothing.
const LogName = "layout-roll.log"

// maxLog keeps the log small: past it the log is renamed to layout-roll.1.log (one older copy is kept).
const maxLog = 64 << 10

// noteRefusal appends one dated line to the log beside the state file. A failure to write it is ignored: it is a note, never a reason to fail or change the run.
func noteRefusal(o Options, now time.Time, line string) {
	if o.StatePath == "" {
		return
	}
	p := filepath.Join(filepath.Dir(o.StatePath), LogName)
	if os.MkdirAll(filepath.Dir(p), 0o700) != nil {
		return
	}
	if st, err := os.Stat(p); err == nil && st.Size() > maxLog {
		old := strings.TrimSuffix(p, ".log") + ".1.log"
		os.Remove(old)
		_ = os.Rename(p, old)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	_, _ = f.WriteString(now.Format("2006-01-02 15:04:05") + " " + line + "\n")
	_ = f.Close()
}

// writeStateFile is writeState as a variable only so that a test can make the write of the state file fail; nothing else changes it.
var writeStateFile = writeState

// saveRunErr is saveRun that says whether the state file was written. It is used where the state file matters (after a save was written, or before one is).
func saveRunErr(path string, s state, now time.Time, outcome string) error {
	s.LastRun = &runRecord{At: now, Outcome: outcome}
	return writeStateFile(path, s)
}

// Rand is the random source of a real roll.
func Rand() func(int) int { return randomIntn }

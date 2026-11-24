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
	"time"

	"github.com/Project-Arrakis/tabr-tau/kit/durable"
)

const (
	// FileName is the game's own config file that holds the setting. It is the only existing file this package changes; it also writes its backups, its state file, the base files of a refused run (deepdesert.go) and, through the save package, one checkpoint copy of the save in the save's backup folder. The save itself is only read.
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
	// The run only ever reads it, apart from the verified checkpoint copied into the save's backup folder. Editor is the version written into the base files that are exported.
	SavePath    string
	SaveProblem string
	Editor      string
	Now         func() time.Time
	Intn        func(n int) int
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
		next = Pick(cur, o.Intn)
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
		saveRun(o.StatePath, st, now, "not changed: the game is running")
		return r, fmt.Errorf("not changed: %s", why)
	}

	// guard 3 (#1167): the Deep Desert. A checkpoint of the save, then a read of it; the roll is refused while a base, the character or a vehicle of the player is in the Deep Desert,
	// and each such base is exported first. --set and --force do not skip it.
	if done, err := guardDeepDesert(o, &r, &st, now); done {
		return r, err
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

	// backup first, written exclusively and compared with what was read
	backup := ""
	if existed {
		if backup, err = writeBackup(o.ConfigDir, old, now, mode); err != nil {
			return r, fmt.Errorf("not changed: the backup could not be made (%v)", err)
		}
		r.say("Backup: %s", filepath.Base(backup))
		if n := pruneBackups(o.ConfigDir); n > 0 {
			r.say("Removed %d older backup(s); the newest %d are kept.", n, KeepBackups)
		}
	}
	// the file must still be what was read, and the game still not running: otherwise another program changed it in between and nothing is written
	if running, why := o.GameRunning(); running {
		saveRun(o.StatePath, st, now, "not changed: the game is running")
		return r, fmt.Errorf("not changed: %s (it started while this ran)", why)
	}
	again, _, _, err := readIni(path)
	if err != nil || again != old {
		return r, errors.New("not changed: Game.ini changed while this ran; run it again")
	}
	if mode == 0 {
		mode = 0o644
	}
	if err := durable.WriteFile(path, []byte(text), mode, writeOptions()); err != nil {
		return r, fmt.Errorf("the file Game.ini could not be written (%v); it was left as it was%s", err, backupNote(backup))
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
	if err := saveRoll(o.StatePath, st, now, cur, next); err != nil {
		r.say("Note: the record of this roll could not be saved (%v); the roll itself is done.", err)
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

func saveRoll(path string, s state, now time.Time, from, to int) error {
	s.LastRoll = &rollRecord{At: now, From: from, To: to}
	s.LastRun = &runRecord{At: now, Outcome: fmt.Sprintf("rolled layout %d -> %d", from, to)}
	return writeState(path, s)
}

// saveRun records a run that changed nothing, so the person can see on the next look why. A failure to write it is ignored: it is a note, not a guard.
func saveRun(path string, s state, now time.Time, outcome string) {
	s.LastRun = &runRecord{At: now, Outcome: outcome}
	_ = writeState(path, s)
}

// Rand is the random source of a real roll.
func Rand() func(int) int { return randomIntn }

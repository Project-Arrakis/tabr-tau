package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

const moveReserveUsage = `usage: move-reserve [--boxes "DD-1,DD-2,DD-3"] [--dry-run]`

// moveReserveEnv is what a run asks of the machine. A test replaces it; a real run uses realMoveReserveEnv.
type moveReserveEnv struct {
	GameRunning func() (bool, string) // asked right now, never from a cache
	Discover    func() []string
	Flush       func(*save.Save) save.Flush // writes the edit; a test makes it fail
}

func realMoveReserveEnv() moveReserveEnv {
	return moveReserveEnv{GameRunning: gameRunningNow, Discover: save.Discover, Flush: func(s *save.Save) save.Flush { return s.Flush() }}
}

// runMoveReserve is `tabr-tau move-reserve [--boxes "Name A,Name B"] [--dry-run]` (#1191): every stack of the character's Reserve goes into the Developer storage boxes, whole and
// unchanged, in one all-or-nothing edit. Without --boxes the boxes are DD-1, DD-2 ... DD-6 as far as they run in a row from DD-1 (#1214); --boxes names up to six boxes of its own. There is no window. The preview (the same edit, rolled back) is always made and printed first; a dry run stops there and writes nothing. A real
// run refuses while a game process exists (there is no editor here to hold a queued edit), takes a verified backup copy of the save, writes through the editor's own write path (which takes
// its own backup too), and then opens the written file afresh to see that the Reserve is empty.
func runMoveReserve(args []string, savePath string, out io.Writer, env moveReserveEnv) error {
	fs := flag.NewFlagSet("move-reserve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	boxes := fs.String("boxes", "", "the names of the storage boxes, separated by a comma (at most 6; matched exactly, capitals included). Without it the boxes DD-1, DD-2 ... DD-6 found in a row from DD-1 are used")
	dry := fs.Bool("dry-run", false, "say what would be done and write nothing")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%s: %v", moveReserveUsage, err)
	}
	if fs.NArg() > 0 {
		return errors.New(moveReserveUsage)
	}
	var names []string
	for _, n := range strings.Split(*boxes, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	given := false
	fs.Visit(func(f *flag.Flag) { given = given || f.Name == "boxes" })
	if len(names) > ops.MaxReserveBoxes || (len(names) == 0 && given) {
		return fmt.Errorf("%s (one to %d names, or none to use DD-1, DD-2 ... as far as they run in a row; a name cannot contain a comma)", moveReserveUsage, ops.MaxReserveBoxes)
	}
	path, problem := resolveLayoutSave(savePath, env.Discover)
	if problem != "" {
		return fmt.Errorf("not changed: %s (name the save with --save before the command)", problem)
	}
	say := func(format string, a ...any) { fmt.Fprintf(out, format+"\n", a...) }
	lines := func(v any) (nothing bool) {
		m, _ := v.(map[string]any)
		for _, l := range m["summary"].([]string) {
			say("  %s", l)
		}
		for _, w := range m["warnings"].([]string) {
			say("  Note: %s", w)
		}
		n, _ := m["nothingToDo"].(bool)
		return n
	}

	if !*dry {
		if running, why := env.GameRunning(); running {
			return fmt.Errorf("not changed: %s. Close the game and run this again", why)
		}
	}
	s, err := save.Open(path)
	if err != nil {
		return fmt.Errorf("not changed: the save could not be opened: %v", err)
	}
	defer s.Close()
	if !*dry {
		release, err := save.LockSave(s.Path)
		if err != nil {
			return fmt.Errorf("not changed: the save is open in another TABR TAU window or program (%v). Close that and run this again", err)
		}
		defer release()
	}
	o := &ops.Ops{S: s}
	opt := ops.MoveReserveOptions{}

	pv, err := o.PreviewMoveReserve(names, opt)
	if err != nil {
		return fmt.Errorf("not changed: %v", err)
	}
	if *dry {
		say("Dry run: nothing is written. A real run would do this:")
		lines(pv)
		return nil
	}
	say("The question (the edit, undone again):")
	if lines(pv) {
		return nil // the Reserve is empty: nothing to write
	}
	cp, err := s.BackupNow()
	if err != nil {
		return fmt.Errorf("not changed: the backup copy of the save could not be taken: %v", err)
	}
	say("Backup copy of the save: %s (%d bytes, SHA-256 %s), verified. It is in the save's backup folder (the newest 8 manual backups are kept, older ones over 7 days old are removed).", cp.Name, cp.Size, cp.SHA256)
	if len(cp.Pruned) > 0 {
		say("Removed %d older manual backup(s) beyond the newest 8: %s", len(cp.Pruned), strings.Join(cp.Pruned, ", "))
	}
	r, err := o.MoveReserve(names, pv.(map[string]any)["fingerprint"].(string), opt)
	if err != nil {
		return fmt.Errorf("not changed: %v", err)
	}
	flush := env.Flush
	if flush == nil {
		flush = realMoveReserveEnv().Flush
	}
	f := flush(s)
	if !f.Saved {
		s.Discard() // the edit stays out of the file: throw it away so nothing is left queued
		return fmt.Errorf("not changed: the edit was worked out but could not be written (%s)", f.Why)
	}
	// from here the save file HAS been changed
	changed := func(why string) error {
		return fmt.Errorf("the save WAS changed but %s. The backup copy %s holds the save as it was before; Database > Save backups restores it. Look at it before the game starts", why, cp.Name)
	}
	if f.Warning != "" {
		return changed("the write reported: " + f.Warning)
	}
	fresh, err := save.Open(s.Path)
	if err != nil {
		return changed("the written file could not be opened again (" + err.Error() + ")")
	}
	defer fresh.Close()
	again, err := (&ops.Ops{S: fresh}).PreviewMoveReserve(names, opt)
	if m, _ := again.(map[string]any); err != nil || m["nothingToDo"] != true {
		return changed("the Reserve of the written file is not empty")
	}
	say("Written, and read back from the written file: the Reserve is empty.")
	lines(r)
	return nil
}

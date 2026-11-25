package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/buildinfo"
	"github.com/Project-Arrakis/tabr-tau/internal/layoutroll"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// layoutStatePath is the file that records the last roll: beside the editor's settings. Empty when the system has no user config folder.
func layoutStatePath() string {
	p := settings.Path()
	if p == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(p), "layout-roll.json")
}

// gameRunningNow asks the process list right now (never the cache). A check that cannot be made counts as the game running.
func gameRunningNow() (bool, string) {
	if save.GameRunningNow() {
		return true, save.RunningMessage()
	}
	return false, ""
}

// runLayoutRoll is `tabr-tau layout-roll [--dry-run] [--set N] [--vetted 9,11] [--force] [--recycle-bases]` (#1141): one roll of the single-player Deep Desert layout, printed in plain lines. There is no window.
func runLayoutRoll(args []string, cfgDir string, out io.Writer, base layoutroll.Options) error {
	fs := flag.NewFlagSet("layout-roll", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dry := fs.Bool("dry-run", false, "say what would be done and write nothing")
	force := fs.Bool("force", false, "roll although one already happened in the current Tuesday 05:00 week")
	set := fs.Int("set", -1, "use this layout (0 to 11) instead of a random one")
	vetted := fs.String("vetted", "", "the layouts a random roll may pick from, for this run only (for example 9,11); without it the file "+layoutroll.VettedFileName+" is used, and without that file the built-in list")
	recycle := fs.Bool("recycle-bases", false, "recycle your Deep Desert bases (one at a time, each with a Solido) before the roll, instead of refusing while one stands")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("usage: layout-roll [--dry-run] [--set N] [--vetted 9,11] [--force] [--recycle-bases]: %v", err)
	}
	if fs.NArg() > 0 {
		return errors.New("usage: layout-roll [--dry-run] [--set N] [--vetted 9,11] [--force] [--recycle-bases]")
	}
	o := base
	o.ConfigDir, o.DryRun, o.Force, o.RecycleBases = cfgDir, *dry, *force, *recycle
	setGiven := false
	vettedGiven := false
	fs.Visit(func(f *flag.Flag) {
		setGiven = setGiven || f.Name == "set"
		vettedGiven = vettedGiven || f.Name == "vetted"
	})
	if setGiven {
		o.Set = set
	}
	if vettedGiven {
		o.VettedFlag = vetted
	}
	r, err := layoutroll.Roll(o)
	for _, l := range r.Lines {
		fmt.Fprintln(out, l)
	}
	return err
}

// realLayoutRollOptions is what a real run uses: the clock, the random source, the state file, the process list and the save to check. The save is the one named with --save, else the
// only one found on this machine; with none or several found the run has no save to check and refuses (it changes nothing), saying why.
func realLayoutRollOptions(savePath string) layoutroll.Options {
	path, problem := resolveLayoutSave(savePath, save.Discover)
	return layoutroll.Options{StatePath: layoutStatePath(), Now: time.Now, Intn: layoutroll.Rand(), GameRunning: gameRunningNow, SavePath: path, SaveProblem: problem, Editor: buildinfo.Get().Version}
}

// resolveLayoutSave names the save the roll checks: the one given, else the only one found. The second answer is the reason when there is none.
func resolveLayoutSave(explicit string, discover func() []string) (path, problem string) {
	if explicit != "" {
		return explicit, ""
	}
	switch found := discover(); len(found) {
	case 1:
		return found[0], ""
	case 0:
		return "", "no save (game.db) was found on this machine"
	default:
		return "", fmt.Sprintf("%d saves were found, so it is not clear which one to check", len(found))
	}
}

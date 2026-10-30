package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #508: the automatic refill and repair pass does not run on a save that is not the game's own (a copy). The tests put the real "is this the game's own save" rule back; the
// save of autoServer lives in a temp folder, so it is a copy, and ownAutoServer puts the same data where the game keeps its saves.

const copyNote = "Automatic refill not run: this is a copy, not the game's save."
const copyWhy = "This is a copy, not the game's save: the automatic pass only runs on the game's own save."

// copyWorld puts the real own-save rule back and points the game's folder at an empty one, so the temp save of a test is a copy.
func copyWorld(t *testing.T) string {
	t.Helper()
	t.Cleanup(save.RealOwnSaveCheckForTests())
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	return local
}

// ownAutoServer is autoServer with the file moved into the game's folder layout: the game's own save.
func ownAutoServer(t *testing.T, local string, st settings.Settings) (*Server, string) {
	t.Helper()
	tmp := testsave.PlayerWithSQL(t, autoPassSQL)
	b, err := os.ReadFile(tmp.Path)
	if err != nil {
		t.Fatal(err)
	}
	own := filepath.Join(local, "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage", "FLS_retail", "1", "game.db")
	if err := os.MkdirAll(filepath.Dir(own), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(own, b, 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := save.Open(own)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	s := New(g, config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(s.SettingsPath, st); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func TestLaunchPassDoesNotRunOnACopyAndSaysSo(t *testing.T) {
	cases := map[string]func() func(){
		"no game":      func() func() { return func() {} },
		"game running": save.ActiveSessionForTests,
		"game at menu": save.GameAtMenuForTests,
	}
	for name, game := range cases {
		t.Run(name, func(t *testing.T) {
			copyWorld(t)
			defer game()()
			s, dir := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
			before := fileBytes(t, s)
			s.ScheduleLaunchPass(shortDelay, false)
			if st := launchState(t, s); st["pending"] != false || st["running"] != false {
				t.Fatalf("no countdown on a copy: %v", st)
			}
			if note, warn := s.takeStartup(); note != copyNote || warn {
				t.Fatalf("the banner says why: %q warn=%v", note, warn)
			}
			s.ScheduleLaunchPass(0, false) // the pass at once (no countdown) is held back the same way
			s.RunStartupTasks()            // and so is a pass started by anything else
			nothingWritten(t, s, dir, before)
			if n, _ := s.takeStartup(); !strings.Contains(n, "not run: this is a copy") {
				t.Fatalf("note %q", n)
			}
			w := postEditGet(t, s, "/api/settings")
			if !strings.Contains(w, `"copy":true`) || strings.Contains(w, `"waiting":true`) {
				t.Fatalf("the card says it is a copy and does not wait: %s", w)
			}
		})
	}
}

func TestLaunchPassStillRunsOnTheGamesOwnSave(t *testing.T) {
	local := copyWorld(t)
	s, _ := ownAutoServer(t, local, settings.Settings{AutoRefillOnOpen: true})
	s.ScheduleLaunchPass(0, false)
	note, warn := s.takeStartup()
	if warn || !strings.Contains(note, "Automatic refill saved ") || !strings.Contains(note, " to the game (the previous file is backed up as ") {
		t.Fatalf("the game's own save gets the pass and the words 'to the game': %q", note)
	}
	if w := postEditGet(t, s, "/api/settings"); !strings.Contains(w, `"copy":false`) {
		t.Fatalf("not a copy: %s", w)
	}
}

// With a session running the game's own save still waits for the game; only a copy is never handled.
func TestOwnSaveWaitsForTheGameAndACopyIsNotQueued(t *testing.T) {
	local := copyWorld(t)
	defer save.ActiveSessionForTests()()
	s, _ := ownAutoServer(t, local, settings.Settings{AutoRefillOnOpen: true})
	s.ScheduleLaunchPass(0, false)
	if note, _ := s.takeStartup(); !strings.HasPrefix(note, "Automatic refill queued: ") {
		t.Fatalf("the game's own save queues the pass: %q", note)
	}
	c, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	c.ScheduleLaunchPass(0, false)
	if note, _ := c.takeStartup(); note != copyNote {
		t.Fatalf("a copy neither runs nor queues: %q", note)
	}
}

func TestRunPassNowOnACopyExplainsWhy(t *testing.T) {
	for name, game := range map[string]func() func(){"no game": func() func() { return func() {} }, "game running": save.ActiveSessionForTests} {
		t.Run(name, func(t *testing.T) {
			copyWorld(t)
			defer game()()
			// the master switch is off: "run now" runs whatever the switch says on the game's own save, and on a copy it says why not
			s, dir := autoServer(t, autoPassSQL, settings.Settings{})
			before := fileBytes(t, s)
			w := postEdit(t, s, "/api/autopass/run", `{}`)
			if w.Code != 200 || !strings.Contains(w.Body.String(), copyWhy) || !strings.Contains(w.Body.String(), `"warn":true`) {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			nothingWritten(t, s, dir, before)
		})
	}
}

func TestRunPassNowStillRunsOnTheGamesOwnSave(t *testing.T) {
	local := copyWorld(t)
	s, _ := ownAutoServer(t, local, settings.Settings{})
	w := postEdit(t, s, "/api/autopass/run", `{}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Automatic refill saved ") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

// The manual Fill and Refill buttons are edits by hand: they still work on a copy, and what they answer does not say "the game".
func TestManualFillStillWorksOnACopy(t *testing.T) {
	copyWorld(t)
	s := liquidsServer(t)
	s.KeepPendingEdits(t.TempDir())
	w := postEdit(t, s, "/api/player/fill-all-liquids", `{"vehicles":true}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != "written" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got := fuelsInFile(t, s.Save.Path); got[601] != 100 {
		t.Fatalf("the vehicle is full: %v", got)
	}
	if strings.Contains(w.Body.String(), "to the game") {
		t.Fatalf("a copy is not the game: %s", w.Body.String())
	}
}

// The Restore list marks the backups of a copy as made from a copy; the game's own save has no such mark.
func TestBackupsOfACopyAreMarkedAsFromACopy(t *testing.T) {
	local := copyWorld(t)
	c, _ := autoServer(t, autoPassSQL, settings.Settings{})
	postEdit(t, c, "/api/player/fill-all-liquids", `{"vehicles":true}`)
	if len(c.Save.Backups()) != 1 {
		t.Fatalf("a backup was taken: %v", c.Save.Backups())
	}
	if w := postEditGet(t, c, "/api/save/backups"); !strings.Contains(w, `"copy":true`) {
		t.Fatalf("made from a copy: %s", w)
	}
	own, _ := ownAutoServer(t, local, settings.Settings{})
	postEdit(t, own, "/api/player/fill-all-liquids", `{"vehicles":true}`)
	if w := postEditGet(t, own, "/api/save/backups"); !strings.Contains(w, `"name"`) || strings.Contains(w, `"copy":true`) {
		t.Fatalf("the game's own backups are not marked: %s", w)
	}
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// #1201: a run that meets the open editor (which holds the save's lock) returns an error, so the task's exit code is not 0, and leaves one dated line in layout-roll.log beside the
// state file; the state file is not written and Game.ini is not changed.
func TestLayoutRollRefusedByTheEditorsLockIsAnErrorAndLeavesALogLine(t *testing.T) {
	o, dir := layoutTestOptions(t, false)
	release, err := save.LockSave(o.SavePath) // what the editor does for as long as its window is open
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var out bytes.Buffer
	err = runLayoutRoll(nil, dir, &out, o)
	if err == nil || !strings.Contains(err.Error(), "another layout roll is running") {
		t.Fatalf("%v", err)
	}
	b, rerr := os.ReadFile(filepath.Join(filepath.Dir(o.StatePath), "layout-roll.log"))
	if rerr != nil || !strings.HasPrefix(string(b), "2026-11-24 05:01:00 layout roll refused: the save is open in another TABR TAU window") || !strings.Contains(string(b), "nothing changed") {
		t.Fatalf("%v %q", rerr, b)
	}
	if _, serr := os.Stat(o.StatePath); serr == nil {
		t.Fatal("the state file was written by a refused run")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Game.ini")); string(b) != "[A]\r\nx=1\r\n" {
		t.Fatal("Game.ini was written")
	}
}

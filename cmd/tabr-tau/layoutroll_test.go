package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/layoutroll"
)

// The layout-roll verb works in temp folders only: no real Game.ini, state file or process list is touched.
func layoutTestOptions(t *testing.T, running bool) (layoutroll.Options, string) {
	t.Helper()
	d := t.TempDir()
	ini := filepath.Join(d, "Game.ini")
	if err := os.WriteFile(ini, []byte("[A]\r\nx=1\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return layoutroll.Options{
		StatePath:   filepath.Join(d, "state", "layout-roll.json"),
		Now:         func() time.Time { return time.Date(2026, 11, 24, 5, 1, 0, 0, time.Local) },
		Intn:        func(n int) int { return 0 },
		GameRunning: func() (bool, string) { return running, "Dune.exe is running" },
	}, d
}

func TestLayoutRollVerbDryRunSetAndRefusals(t *testing.T) {
	o, dir := layoutTestOptions(t, false)
	var out bytes.Buffer
	if err := runLayoutRoll([]string{"--dry-run", "--set", "4"}, dir, &out, o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "would change Game.ini from layout 11 to layout 4") || strings.Contains(out.String(), "random pick") {
		t.Fatalf("%s", out.String())
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Game.ini")); string(b) != "[A]\r\nx=1\r\n" {
		t.Fatal("a dry run wrote Game.ini")
	}
	// a real chosen roll
	out.Reset()
	if err := runLayoutRoll([]string{"--set", "4"}, dir, &out, o); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Game.ini")); !strings.Contains(string(b), "Singleplayer=4)") {
		t.Fatal("not set")
	}
	// the arguments are checked
	for _, args := range [][]string{{"--set", "12"}, {"--set", "x"}, {"--bogus"}, {"extra"}} {
		if err := runLayoutRoll(args, dir, &out, o); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
	// the game running: an error, so a scheduled run shows a failure
	o2, dir2 := layoutTestOptions(t, true)
	if err := runLayoutRoll(nil, dir2, &out, o2); err == nil || !strings.Contains(err.Error(), "Dune.exe is running") {
		t.Fatalf("%v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir2, "Game.ini")); string(b) != "[A]\r\nx=1\r\n" {
		t.Fatal("Game.ini written while the game ran")
	}
}

func TestLayoutRollStateSitsBesideTheSettings(t *testing.T) {
	p := layoutStatePath()
	if p == "" || filepath.Base(p) != "layout-roll.json" || filepath.Base(filepath.Dir(p)) != "tabr-tau" {
		t.Fatalf("%q", p)
	}
	if o := realLayoutRollOptions(); o.GameRunning == nil || o.Now == nil || o.Intn == nil || o.StatePath != p {
		t.Fatal("the real options are not complete")
	}
}

// the verb is wired into the command list: a bad argument reaches its own usage text.
func TestLayoutRollIsInTheCommandList(t *testing.T) {
	if err := runCommand([]string{"layout-roll", "extra"}, t.TempDir()); err == nil || !strings.Contains(err.Error(), "usage: layout-roll") {
		t.Fatalf("%v", err)
	}
}

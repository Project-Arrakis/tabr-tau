package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/layoutroll"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
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
		SavePath:    testsave.PlayerWithSQL(t, "").Path, // a made-up save with nothing in the Deep Desert
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
	if o := realLayoutRollOptions("some-game.db"); o.GameRunning == nil || o.Now == nil || o.Intn == nil || o.StatePath != p || o.SavePath != "some-game.db" || o.Editor == "" {
		t.Fatal("the real options are not complete")
	}
}

// the verb is wired into the command list: a bad argument reaches its own usage text.
func TestLayoutRollIsInTheCommandList(t *testing.T) {
	if err := runCommand([]string{"layout-roll", "extra"}, t.TempDir(), ""); err == nil || !strings.Contains(err.Error(), "usage: layout-roll") {
		t.Fatalf("%v", err)
	}
}

// The save the roll checks is the one named, else the only one found; with none or several it is not guessed, and the run refuses with the reason (see the layoutroll tests).
func TestLayoutRollNamesTheSaveToCheck(t *testing.T) {
	never := func() []string { t.Fatal("a named save is not searched for"); return nil }
	if p, why := resolveLayoutSave("a-game.db", never); p != "a-game.db" || why != "" {
		t.Fatalf("%q %q", p, why)
	}
	for _, c := range []struct {
		found []string
		path  string
		why   string
	}{
		{[]string{"a-game.db"}, "a-game.db", ""},
		{nil, "", "no save"},
		{[]string{"a-game.db", "a-game.db"}, "", "2 saves were found"},
	} {
		p, why := resolveLayoutSave("", func() []string { return c.found })
		if p != c.path || !strings.Contains(why, c.why) || (c.why != "" && why == "") {
			t.Errorf("%v: %q %q", c.found, p, why)
		}
	}
}

// A run with no save to check changes nothing and says why.
func TestLayoutRollWithNoSaveRefuses(t *testing.T) {
	o, dir := layoutTestOptions(t, false)
	o.SavePath, o.SaveProblem = "", "no save (game.db) was found on this machine"
	var out bytes.Buffer
	if err := runLayoutRoll(nil, dir, &out, o); err == nil || !strings.Contains(err.Error(), "no save") {
		t.Fatalf("%v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Game.ini")); string(b) != "[A]\r\nx=1\r\n" {
		t.Fatal("Game.ini written without a save to check")
	}
}

// --recycle-bases is off unless given, and given it reaches the roll: a dry run with it says that no base stands, and one without it does not mention the step.
func TestLayoutRollRecycleBasesFlagIsOptIn(t *testing.T) {
	o, dir := layoutTestOptions(t, false)
	var out bytes.Buffer
	if err := runLayoutRoll([]string{"--dry-run"}, dir, &out, o); err != nil || strings.Contains(out.String(), "--recycle-bases") {
		t.Fatalf("%v\n%s", err, out.String())
	}
	out.Reset()
	if err := runLayoutRoll([]string{"--dry-run", "--recycle-bases"}, dir, &out, o); err != nil || !strings.Contains(out.String(), "--recycle-bases: no base stands in the Deep Desert") {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Game.ini")); string(b) != "[A]\r\nx=1\r\n" {
		t.Fatal("a dry run wrote Game.ini")
	}
}

// --keep-markers (#1216) reaches the roll: with Deep Desert markers in the save a dry run says it would remove them, and with the flag it says they are left.
func TestLayoutRollKeepMarkersFlag(t *testing.T) {
	o, dir := layoutTestOptions(t, false)
	o.SavePath = testsave.PlayerWithSQL(t, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values (1,0,'DeepDesert','Cave',1,1,0);`).Path
	var out bytes.Buffer
	if err := runLayoutRoll([]string{"--dry-run"}, dir, &out, o); err != nil || !strings.Contains(out.String(), "would remove the Deep Desert markers from the save") || !strings.Contains(out.String(), "1 marker(s)") {
		t.Fatalf("%v\n%s", err, out.String())
	}
	out.Reset()
	if err := runLayoutRoll([]string{"--dry-run", "--keep-markers"}, dir, &out, o); err != nil || !strings.Contains(out.String(), "left as they are (--keep-markers)") {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

func TestLayoutRollVettedFlag(t *testing.T) {
	o, dir := layoutTestOptions(t, false)
	var out bytes.Buffer
	// the built-in list (9 and 11) is used without the flag; the current layout is 11 (the game's default), so a random pick can only be 9
	if err := runLayoutRoll([]string{"--dry-run"}, dir, &out, o); err != nil {
		t.Fatal(err)
	}
	if s := out.String(); !strings.Contains(s, "Vetted layouts: 9, 11.") || !strings.Contains(s, "may choose from: 9 ") || !strings.Contains(s, "to layout 9 (a random pick") {
		t.Fatalf("%s", out.String())
	}
	// --vetted overrides it for the run
	out.Reset()
	if err := runLayoutRoll([]string{"--dry-run", "--vetted", "5,6"}, dir, &out, o); err != nil {
		t.Fatal(err)
	}
	if s := out.String(); !strings.Contains(s, "Vetted layouts: 5, 6. Source: the --vetted flag.") || !strings.Contains(s, "may choose from: 5, 6 ") {
		t.Fatalf("%s", out.String())
	}
	// an empty or bad list is refused, also when a scheduled task would pass it
	for _, v := range []string{"", "x", "9,99"} {
		out.Reset()
		if err := runLayoutRoll([]string{"--vetted", v}, dir, &out, o); err == nil || !strings.Contains(err.Error(), "--vetted") {
			t.Errorf("--vetted %q: %v", v, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Game.ini")); string(b) != "[A]\r\nx=1\r\n" {
		t.Fatal("a refused or dry run wrote Game.ini")
	}
}

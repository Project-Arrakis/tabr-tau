package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// layout-roll vet (#1222) works in temp folders only: the options come from layoutTestOptions, so no real Game.ini, settings folder or process list is touched.

func TestLayoutRollVetSetsThenRecordsThenLists(t *testing.T) {
	o, dir := layoutTestOptions(t, false)
	settings := filepath.Dir(o.StatePath)
	var out bytes.Buffer
	if err := runLayoutRoll([]string{"vet", "8"}, dir, &out, o); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Game.ini")); !strings.Contains(string(b), "Singleplayer=8)") {
		t.Fatal("layout 8 was not set")
	}
	if !strings.Contains(out.String(), "Sandworm Threat") || !strings.Contains(out.String(), "vet 8 fail") {
		t.Fatalf("%s", out.String())
	}
	out.Reset()
	if err := runLayoutRoll([]string{"vet", "8", "fail", "worm", "threat", "100%", "on", "rock"}, dir, &out, o); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(settings, "layout-roll-vetting.txt")); !strings.HasSuffix(string(b), " layout 8 fail worm threat 100% on rock\r\n") {
		t.Fatalf("%q", b)
	}
	out.Reset()
	if err := runLayoutRoll([]string{"vet", "10", "pass"}, dir, &out, o); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(settings, "layout-roll-vetted.txt")); !strings.Contains(string(b), "9, 10, 11") {
		t.Fatalf("%q", b)
	}
	out.Reset()
	if err := runLayoutRoll([]string{"vet", "list"}, dir, &out, o); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Vetted list in use: 9, 10, 11", "Game.ini sets layout 8 now", "fail ", "note: worm threat 100% on rock", "pass "} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out.String())
		}
	}
}

func TestLayoutRollVetArgumentsAreChecked(t *testing.T) {
	o, dir := layoutTestOptions(t, false)
	var out bytes.Buffer
	for _, args := range [][]string{
		{"vet"}, {"vet", "x"}, {"vet", "-1"}, {"vet", "+3"}, {"vet", "list", "x"}, {"vet", "3", "maybe"}, {"vet", "3", "maybe", "pass"}, {"vet", "12"}, {"vet", "--set", "3"},
	} {
		if err := runLayoutRoll(args, dir, &out, o); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Game.ini")); string(b) != "[A]\r\nx=1\r\n" {
		t.Fatal("a refused argument wrote Game.ini")
	}
	if _, err := os.Stat(filepath.Dir(o.StatePath)); err == nil {
		t.Fatal("a refused argument made the settings folder")
	}
}

func TestLayoutRollVetRefusesWhileTheGameRunsAndPrintsTheReason(t *testing.T) {
	o, dir := layoutTestOptions(t, true)
	var out bytes.Buffer
	if err := runLayoutRoll([]string{"vet", "8"}, dir, &out, o); err == nil || !strings.Contains(err.Error(), "Dune.exe is running") {
		t.Fatalf("%v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Game.ini")); string(b) != "[A]\r\nx=1\r\n" {
		t.Fatal("Game.ini written while the game ran")
	}
}

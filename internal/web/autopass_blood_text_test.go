package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// #1240: the result of a pass that filled blood sacks says the fill is not checked in the game, whether it was saved at once or kept on the
// card; a pass that filled no sack does not.
func TestAutoPassResultSaysTheBloodFillIsUnchecked(t *testing.T) {
	const said = "The fill of blood sacks has not been checked in the game yet."
	s, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true})
	s.RunStartupTasks()
	note, _ := s.takeStartup()
	if !strings.Contains(note, "Automatic refill saved ") || !strings.Contains(note, said) {
		t.Fatalf("note %q", note)
	}
	if w := postEditGet(t, s, "/api/settings"); !strings.Contains(w, strings.TrimSuffix(said, ".")) { // the card line drops the last full stop
		t.Fatalf("the card keeps it: %s", w)
	}
	s.RunStartupTasks() // nothing left to fill
	if note, _ = s.takeStartup(); strings.Contains(note, said) {
		t.Fatalf("nothing was filled, note %q", note)
	}

	off := false
	s2, _ := autoServer(t, autoPassSQL, settings.Settings{AutoRefillOnOpen: true, AutoFillBloodSacks: &off})
	s2.RunStartupTasks()
	if note, _ = s2.takeStartup(); !strings.Contains(note, "Automatic refill saved ") || strings.Contains(note, said) {
		t.Fatalf("blood job off, note %q", note)
	}
}

// The same sentence when the pass made its edit but the file could not be written yet.
func TestAutoPassNotYetSavedResultSaysTheBloodFillIsUnchecked(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, autoPassSQL), config.Dir{Path: t.TempDir()}) // no kept pending edits: a changed file is not read again
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(s.SettingsPath, settings.Settings{AutoRefillOnOpen: true}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(s.Save.Path, os.O_APPEND|os.O_WRONLY, 0) // the file changes on disk after it was read, so the write is refused
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte{0})
	f.Close()
	s.RunStartupTasks()
	note, _ := s.takeStartup()
	if !strings.Contains(note, "could not save it yet") || !strings.Contains(note, "The fill of blood sacks has not been checked in the game yet.") {
		t.Fatalf("note %q", note)
	}
}

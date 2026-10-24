package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// S8c of #40: a backups folder that is a link would send the copy of a config file, and the removal of old copies, to a folder somewhere else; the edit is refused and the file stays.
func TestALinkedBackupsFolderRefusesTheEdit(t *testing.T) {
	d := setup(t, map[string]string{"Game.ini": settings})
	elsewhere := t.TempDir()
	link := filepath.Join(d.Path, "tabr-tau-backups")
	if err := os.Symlink(elsewhere, link); err != nil {
		if runtime.GOOS != "windows" {
			t.Skip(err)
		}
		if out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, elsewhere).CombinedOutput(); jerr != nil {
			t.Skipf("no link could be made: %v %s", jerr, out)
		}
	}
	if err := d.Set("Game.ini", "", "GatheringAmount", "5.0", 1); err == nil {
		t.Fatal("an edit whose backup would go through a link must be refused")
	}
	if got := read(t, d, "Game.ini"); got != settings {
		t.Fatalf("the file must stay as it was: %q", got)
	}
	if ents, _ := os.ReadDir(elsewhere); len(ents) != 0 {
		t.Fatalf("nothing may be written through the link: %v", ents)
	}
}

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T, files map[string]string) Dir {
	t.Helper()
	d := t.TempDir()
	for n, c := range files {
		os.WriteFile(filepath.Join(d, n), []byte(c), 0o644)
	}
	return Dir{Path: d}
}

func read(t *testing.T, d Dir, n string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(d.Path, n))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const settings = "[/Script/DuneSandbox.UserServerCustomSettings]\r\nGatheringAmount=2.000000\r\nCraftingCost=0.500000\r\n\r\n[Other]\r\nA=1\r\n"

func TestSetPreservesEverythingElse(t *testing.T) {
	d := setup(t, map[string]string{"ServerCustomSettings.ini": settings})
	if err := d.Set("ServerCustomSettings.ini", "", "GatheringAmount", "5.0", 1); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(settings, "GatheringAmount=2.000000", "GatheringAmount=5.0", 1)
	if got := read(t, d, "ServerCustomSettings.ini"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if len(d.Backups("ServerCustomSettings.ini")) != 1 {
		t.Fatal("expected a backup")
	}
}

func TestStaleEditRefused(t *testing.T) {
	d := setup(t, map[string]string{"Game.ini": settings})
	if err := d.Set("Game.ini", "", "CraftingCost", "1", 1); err == nil {
		t.Fatal("line 1 is GatheringAmount; edit must be refused")
	}
	if err := d.Delete("Game.ini", "Nope", 1); err == nil {
		t.Fatal("delete with wrong key must be refused")
	}
}

func TestAddKeyToSectionAndNewSection(t *testing.T) {
	d := setup(t, map[string]string{"Game.ini": settings})
	d.Set("Game.ini", "/Script/DuneSandbox.UserServerCustomSettings", "NewKey", "7", -1)
	d.Set("Game.ini", "Brand.New", "K", "V", -1)
	got := read(t, d, "Game.ini")
	if !strings.Contains(got, "CraftingCost=0.500000\r\nNewKey=7\r\n\r\n[Other]") {
		t.Fatalf("key not appended at end of its section:\n%q", got)
	}
	if !strings.HasSuffix(got, "[Brand.New]\r\nK=V\r\n") {
		t.Fatalf("bad new section / line endings:\n%q", got)
	}
}

func TestDeleteAndRepeatedKeys(t *testing.T) {
	d := setup(t, map[string]string{"Engine.ini": "[Core.System]\nPaths=a\nPaths=b\nPaths=c\n"})
	if err := d.Delete("Engine.ini", "Paths", 2); err != nil {
		t.Fatal(err)
	}
	if got := read(t, d, "Engine.ini"); got != "[Core.System]\nPaths=a\nPaths=c\n" {
		t.Fatalf("got %q", got)
	}
}

func TestInputValidation(t *testing.T) {
	d := setup(t, map[string]string{"Game.ini": settings})
	for _, name := range []string{"../x.ini", "a/b.ini", "Game.txt", ""} {
		if _, err := d.Read(name); err == nil {
			t.Fatalf("%q must be rejected", name)
		}
	}
	if err := d.Set("Game.ini", "S", "k", "a\nb", -1); err == nil {
		t.Fatal("newline in value must be rejected")
	}
	if err := d.Set("Game.ini", "S", "k=x", "v", -1); err == nil {
		t.Fatal("= in key must be rejected")
	}
}

func TestValidateAndCreate(t *testing.T) {
	d := setup(t, map[string]string{"Game.ini": "[Settings.Audio]\nMasterVolume=1\n", "Input.ini": "[Settings.Input]\n"})
	v := d.Validate()
	if v["ok"] != false || v["dirExists"] != true {
		t.Fatalf("expected problems: %v", v)
	}
	missing := map[string]bool{}
	for _, f := range v["files"].([]map[string]any) {
		if f["exists"] != true {
			missing[f["name"].(string)] = true
		}
	}
	if !missing["ServerCustomSettings.ini"] || missing["Game.ini"] {
		t.Fatalf("wrong missing set %v", missing)
	}
	for _, n := range []string{"ServerCustomSettings.ini", "Game.ini", "GameUserSettings.ini", "Engine.ini"} {
		if err := d.Create(n); err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
	}
	if v := d.Validate(); v["ok"] != true {
		t.Fatalf("still invalid after create: %v", v)
	}
	if got := read(t, d, "Game.ini"); !strings.Contains(got, "MasterVolume=1") || !strings.Contains(got, "[Settings.Video]") {
		t.Fatalf("existing content lost or section missing: %q", got)
	}
	if d.Create("Random.ini") == nil {
		t.Fatal("only expected files may be created")
	}
}

func TestMissingDir(t *testing.T) {
	v := Dir{Path: filepath.Join(t.TempDir(), "nope")}.Validate()
	if v["ok"] != false || v["dirExists"] != false {
		t.Fatalf("%v", v)
	}
}

func TestBOMPreserved(t *testing.T) {
	d := setup(t, map[string]string{"Game.ini": bomStr + "[S]\nA=1\n"})
	if err := d.Set("Game.ini", "", "A", "2", 1); err != nil {
		t.Fatal(err)
	}
	if got := read(t, d, "Game.ini"); got != bomStr+"[S]\nA=2\n" {
		t.Fatalf("got %q", got)
	}
}

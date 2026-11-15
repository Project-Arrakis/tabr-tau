package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNumberReadsTheLastLineOfTheSectionAndChangesNothing(t *testing.T) {
	dir := t.TempDir()
	text := "[A]\nK=9\n[" + DefaultSection + "]\nK=1.5\nK = 2.500000 \n[B]\nK=7\n"
	p := filepath.Join(dir, "ServerCustomSettings.ini")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	d := Dir{Path: dir}
	if v, found, err := d.Number("ServerCustomSettings.ini", DefaultSection, "K", 0, 10); err != nil || !found || v != 2.5 {
		t.Fatalf("%v %v %v", v, found, err)
	}
	if _, found, err := d.Number("ServerCustomSettings.ini", DefaultSection, "Missing", 0, 10); err != nil || found {
		t.Fatalf("a key that is not there: %v %v", found, err)
	}
	if _, found, err := d.Number("Absent.ini", DefaultSection, "K", 0, 10); err != nil || found {
		t.Fatalf("a file that is not there: %v %v", found, err)
	}
	if _, _, err := d.Number("../x.ini", DefaultSection, "K", 0, 10); err == nil {
		t.Fatal("a name that is not a config file was accepted")
	}
	if b, _ := os.ReadFile(p); string(b) != text {
		t.Fatal("the file changed")
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Fatal("a backup or another file was written")
	}
}

func TestNumberRefusesAValueOutsideTheRange(t *testing.T) {
	for _, v := range []string{"abc", "-0.5", "10.5", "NaN", "+Inf", "1e999", ""} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "ServerCustomSettings.ini"), []byte("["+DefaultSection+"]\nK="+v+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, found, err := Dir{Path: dir}.Number("ServerCustomSettings.ini", DefaultSection, "K", 0, 10)
		if err == nil || !found || !strings.Contains(err.Error(), "not a number from 0 to 10") {
			t.Errorf("%q: found %v, err %v", v, found, err)
		}
	}
}

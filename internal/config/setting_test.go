package config

import (
	"errors"
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

// The game reads its settings files without regard to case, so a file edited by hand with another spelling of the section or the key is read, not taken for "key absent" (#753).
func TestNumberMatchesTheSectionAndTheKeyWithoutRegardToCase(t *testing.T) {
	for name, text := range map[string]string{
		"the key in lower case":     "[" + DefaultSection + "]\nlandsraadcontributionmultiplier=2.5\n",
		"the key in upper case":     "[" + DefaultSection + "]\nLANDSRAADCONTRIBUTIONMULTIPLIER=2.5\n",
		"the section in lower case": "[" + strings.ToLower(DefaultSection) + "]\nLandsraadContributionMultiplier=2.5\n",
		"both, and another key too": "[" + strings.ToUpper(DefaultSection) + "]\nOther=1\nlandsraadContributionMULTIPLIER=2.5\n",
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "ServerCustomSettings.ini"), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if v, found, err := (Dir{Path: dir}).Number("ServerCustomSettings.ini", DefaultSection, "LandsraadContributionMultiplier", 0, 10); err != nil || !found || v != 2.5 {
			t.Errorf("%s: %v %v %v", name, v, found, err)
		}
	}
	// the last line still wins, whatever its spelling
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ServerCustomSettings.ini"), []byte("["+DefaultSection+"]\nK=1\nk=3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, _, err := (Dir{Path: dir}).Number("ServerCustomSettings.ini", DefaultSection, "K", 0, 10); err != nil || v != 3 {
		t.Errorf("the last line: %v %v", v, err)
	}
}

// A value to correct and a file that cannot be read are told apart by the type of the error (#753).
func TestNumberTellsABadValueFromAFileThatCannotBeRead(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ServerCustomSettings.ini"), []byte("["+DefaultSection+"]\nK=abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, found, err := Dir{Path: dir}.Number("ServerCustomSettings.ini", DefaultSection, "K", 0, 10)
	var bad *BadValueError
	if !errors.As(err, &bad) || !found || bad.Key != "K" || bad.Raw != "abc" {
		t.Fatalf("a bad value: %v %v", found, err)
	}
	// a folder where the file should be cannot be read, and that is not a bad value
	dir = t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ServerCustomSettings.ini"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, _, err = Dir{Path: dir}.Number("ServerCustomSettings.ini", DefaultSection, "K", 0, 10)
	if err == nil || errors.As(err, &bad) {
		t.Fatalf("an unreadable file: %v", err)
	}
}

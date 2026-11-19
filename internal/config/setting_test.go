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

// Numbers answers several keys of a section from one read of the file, each as Number answers it (#827): a bad value is that key's own error, a missing key is not found, and a file
// that is not there answers every key as not found; a file that cannot be read is the error of the call.
func TestNumbersAnswersEachKeyFromOneRead(t *testing.T) {
	dir := t.TempDir()
	text := "[" + DefaultSection + "]\nA=2.5\nB=abc\nA=3\n[Other]\nC=9\n"
	if err := os.WriteFile(filepath.Join(dir, "ServerCustomSettings.ini"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	d := Dir{Path: dir}
	keys := []NumberKey{{"A", 0, 10}, {"B", 0, 10}, {"C", 0, 10}, {"a", 0, 2}}
	vs, err := d.Numbers("ServerCustomSettings.ini", DefaultSection, keys)
	if err != nil || len(vs) != len(keys) {
		t.Fatalf("%v %v", vs, err)
	}
	if !vs[0].Found || vs[0].Value != 3 || vs[0].Err != nil {
		t.Errorf("A: %+v", vs[0])
	}
	var bad *BadValueError
	if !vs[1].Found || !errors.As(vs[1].Err, &bad) || bad.Key != "B" {
		t.Errorf("B: %+v", vs[1])
	}
	if vs[2].Found || vs[2].Err != nil {
		t.Errorf("C is in another section: %+v", vs[2])
	}
	if !vs[3].Found || !errors.As(vs[3].Err, &bad) || bad.Max != 2 {
		t.Errorf("a key is matched without regard to case and checked against its own range: %+v", vs[3])
	}
	for i, k := range keys {
		v, found, err := d.Number("ServerCustomSettings.ini", DefaultSection, k.Key, k.Min, k.Max)
		if v != vs[i].Value || found != vs[i].Found || (err == nil) != (vs[i].Err == nil) {
			t.Errorf("%s: Number says %v %v %v, Numbers %+v", k.Key, v, found, err, vs[i])
		}
	}
	if vs, err := d.Numbers("Absent.ini", DefaultSection, keys); err != nil || vs[0].Found || len(vs) != len(keys) {
		t.Fatalf("a file that is not there: %v %v", vs, err)
	}
	if _, err := d.Numbers("../x.ini", DefaultSection, keys); err == nil {
		t.Fatal("a name that is not a config file was accepted")
	}
	if err := os.Mkdir(filepath.Join(dir, "Unreadable.ini"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Numbers("Unreadable.ini", DefaultSection, keys); err == nil {
		t.Fatal("a file that cannot be read is an error")
	}
}

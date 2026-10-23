package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The switches of the automatic jobs (#352): absent is the job's default, not off.

func TestAFileWithoutTheJobKeysGivesEachJobItsDefault(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(p, []byte(`{"autoRefillOnOpen": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Load(p)
	if !s.AutoRefillOnOpen {
		t.Fatal("the master switch is read as before")
	}
	for _, k := range JobKeys {
		if !s.JobOn(k, true) || s.JobOn(k, false) {
			t.Errorf("%s: a key the file does not have must read as the default the caller gives", k)
		}
	}
	if (Settings{}).JobOn("autoSomethingElse", true) {
		t.Error("an unknown key is off")
	}
	if (&Settings{}).SetJob("autoSomethingElse", true) {
		t.Error("an unknown key cannot be set")
	}
}

func TestEachJobSwitchIsKeptWithItsOwnJSONName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	for _, k := range JobKeys {
		for _, v := range []bool{false, true} {
			var s Settings
			if !s.SetJob(k, v) {
				t.Fatalf("%s is not a job key", k)
			}
			if err := Save(p, s); err != nil {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(p)
			if want := `"` + k + `": ` + map[bool]string{true: "true", false: "false"}[v]; !strings.Contains(string(b), want) {
				t.Fatalf("%s: the file %s lacks %s", k, b, want)
			}
			got := Load(p)
			if got.JobOn(k, !v) != v {
				t.Fatalf("%s=%v did not come back (default %v): %s", k, v, !v, b)
			}
			for _, other := range JobKeys {
				if other != k && !got.JobOn(other, true) {
					t.Fatalf("setting %s moved %s", k, other)
				}
			}
		}
	}
}

// A file that never had a switch set is not given keys the person did not choose.
func TestSavingWithoutJobSwitchesWritesNoJobKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(p, Settings{AutoRefillOnOpen: true}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	for _, k := range JobKeys {
		if strings.Contains(string(b), k) {
			t.Fatalf("%s written without being set: %s", k, b)
		}
	}
}

// A switch that is not true or false makes the file damaged (defaults, nothing turned on), like any other bad value.
func TestAJobSwitchThatIsNotABooleanIsADamagedFile(t *testing.T) {
	for _, k := range JobKeys {
		p := filepath.Join(t.TempDir(), "s.json")
		os.WriteFile(p, []byte(`{"autoRefillOnOpen": true, "`+k+`": "no"}`), 0o600)
		if _, err := LoadStrict(p); err == nil {
			t.Errorf("%s=\"no\" must be reported", k)
		}
		if Load(p).AutoRefillOnOpen {
			t.Errorf("%s: a damaged file must not turn the refill on", k)
		}
	}
}

func TestANullJobSwitchIsTheDefault(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	os.WriteFile(p, []byte(`{"autoRepairGear": null}`), 0o600)
	if !Load(p).JobOn("autoRepairGear", true) {
		t.Fatal("null is not off")
	}
}

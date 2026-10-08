package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "settings.json")
	if got := Load(p); got.AutoRefillOnOpen {
		t.Fatal("a missing file means the defaults, everything off")
	}
	if err := Save(p, Settings{AutoRefillOnOpen: true}); err != nil {
		t.Fatal(err)
	}
	if !Load(p).AutoRefillOnOpen {
		t.Fatal("saved value not read back")
	}
	if err := Save(p, Settings{}); err != nil || Load(p).AutoRefillOnOpen {
		t.Fatalf("turning it off must stick: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(p), "*.tmp")); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestDamagedOrHostileFilesGiveDefaults(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"junk.json":    "not json at all",
		"wrong.json":   `{"autoRefillOnOpen": "yes"}`,
		"array.json":   `[true]`,
		"unknown.json": `{"somethingNew": 1}`,
		"empty.json":   ``,
	} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(content), 0o600)
		if Load(p).AutoRefillOnOpen {
			t.Errorf("%s must not turn the feature on", name)
		}
	}
	big := filepath.Join(dir, "big.json")
	os.WriteFile(big, append([]byte(`{"autoRefillOnOpen": true}`), make([]byte, 70<<10)...), 0o600)
	if Load(big).AutoRefillOnOpen {
		t.Error("an oversized file must be ignored")
	}
	if Load("").AutoRefillOnOpen {
		t.Error("no path means defaults")
	}
	if err := Save("", Settings{}); err == nil {
		t.Error("saving without a path must fail loudly")
	}
}

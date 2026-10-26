package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/kits"
)

// The kits live in the settings file (#417), so a new save does not lose them, and every other setting keeps its place beside them.

func aKit(name string) kits.Kit {
	return kits.Kit{Name: name, Items: []kits.Line{{Item: "SpicedFuelCell", Quantity: 100}}, XP: 500}
}

func TestKitsAreKeptBesideTheOtherSettings(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if _, err := Update(p, func(s *Settings) error {
		s.AutoRefillOnOpen = true
		s.TrackNumbers = map[string]int64{"Combat": 3}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(p, func(s *Settings) error {
		var err error
		s.Kits, err = kits.Put(s.Kits, "", aKit("Starter"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// another change does not drop the kits, and the kits do not drop the other settings
	if _, err := Update(p, func(s *Settings) error { s.AutoRefillOnOpen = false; return nil }); err != nil {
		t.Fatal(err)
	}
	got, err := LoadStrict(p)
	if err != nil || got.AutoRefillOnOpen || got.TrackNumbers["Combat"] != 3 || len(got.Kits) != 1 || got.Kits[0].Name != "Starter" || got.Kits[0].Items[0].Quantity != 100 || got.Kits[0].XP != 500 {
		t.Fatalf("%v %+v", err, got)
	}
	// a file from before kits existed has none, and loads
	old := filepath.Join(t.TempDir(), "old.json")
	os.WriteFile(old, []byte(`{"autoRefillOnOpen": true}`), 0o600)
	if s, err := LoadStrict(old); err != nil || !s.AutoRefillOnOpen || len(s.Kits) != 0 {
		t.Fatalf("%v %+v", err, s)
	}
	// with no kits the file has no kits key
	empty := filepath.Join(t.TempDir(), "e.json")
	Save(empty, Settings{AutoRefillOnOpen: true})
	if b, _ := os.ReadFile(empty); strings.Contains(string(b), "kits") {
		t.Fatalf("%s", b)
	}
}

func TestAFileTheEditorWouldRefuseToReadIsNeverWritten(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	Save(p, Settings{AutoRefillOnOpen: true})
	s := Settings{AutoRefillOnOpen: true}
	for i := 0; i < 600; i++ {
		s.Kits = append(s.Kits, kits.Kit{Name: strings.Repeat("N", 59) + "x", Items: []kits.Line{{Item: strings.Repeat("A", 1000), Quantity: 1}}})
	}
	if err := Save(p, s); err == nil || !strings.Contains(err.Error(), "KiB") {
		t.Fatalf("%v", err)
	}
	if !Load(p).AutoRefillOnOpen {
		t.Fatal("the file that was there is still there and still reads")
	}
	if _, err := Update(p, func(c *Settings) error { c.Kits = s.Kits; return nil }); err == nil {
		t.Fatal("Update must refuse too")
	}
}

func TestTwentyFullKitsStillLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	var list []kits.Kit
	line := kits.Line{Item: "Combat_Heavy_Unique_Reinforced_Helmet_06", Quantity: 1000000, Grade: 5, Augments: []string{"T6_Augment_Armor10"}, AugmentGrade: 5}
	for i := 0; i < kits.MaxKits; i++ {
		k := kits.Kit{Name: "Full kit " + string(rune('A'+i)), XP: kits.MaxXP, SkillPoints: kits.MaxSkillPoints}
		for j := 0; j < kits.MaxLines; j++ {
			k.Items = append(k.Items, line)
		}
		var err error
		if list, err = kits.Put(list, "", k); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Update(p, func(s *Settings) error { s.Kits = list; s.AutoRefillOnOpen = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadStrict(p); err != nil || len(got.Kits) != kits.MaxKits || !got.AutoRefillOnOpen {
		t.Fatalf("%v", err)
	}
}

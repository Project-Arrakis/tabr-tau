package catalog

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/kit/gamedata/vehicles"
)

// The vehicle tables are in kit/gamedata/vehicles; what needs the item names is tested here: the in-game name of a module follows its grade (Mk6 is the _6 part).
func TestModuleNameOfAMkSixPartEndsInMkSix(t *testing.T) {
	if n, _ := Name("BuggyChassis_6"); n != "Buggy Chassis Mk6" {
		t.Fatalf("the item catalog names BuggyChassis_6 %q", n)
	}
	b, _ := vehicles.ClassFor("Buggy")
	tpl, ok := b.Template("T6_Combat")
	if !ok {
		t.Fatal("no T6_Combat template")
	}
	for _, slot := range []string{"Chassis", "Generator", "Engine", "CockpitHull", "BodyHull", "Locomotion", "Locomotion2", "Locomotion3", "Locomotion4", "UtilitySlot1", "UtilitySlot2"} {
		id := tpl.Modules[slot]
		if n := ModuleName(id); !strings.HasSuffix(n, "Mk6") {
			t.Errorf("%s is %q, not a Mk6 part", id, n)
		}
	}
}

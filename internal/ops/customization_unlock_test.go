package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The character actor (2) as a real save has it: a customization library with one unlocked cosmetic, next to other state that must not change.
const libraryProps = `{"Other":{"keep":[1,2,3]},"CustomizationLibraryActorComponent":{"m_UnlockedCustomizationSerializableList":{"m_UnlockedCustomizationIds":[` +
	`{"m_CustomizationId":"MTX_Frameblade_Knife"}]}}}`

func libraryOps(t *testing.T) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, `update actors set properties=jsonb('`+libraryProps+`') where id=2;`)}
}

func libraryIDs(t *testing.T, o *Ops) map[string]bool {
	t.Helper()
	p, err := o.player()
	if err != nil {
		t.Fatal(err)
	}
	lib, ok := o.customizationLibrary(p.Pawn)
	if !ok {
		t.Fatal("the library list is gone")
	}
	return lib
}

func TestTheCustomizationLibraryIsReadAndMarksWhatIsUnlocked(t *testing.T) {
	o := libraryOps(t)
	r, err := o.Customizations()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["libraryAvailable"] != true || m["libraryCount"] != 1 {
		t.Fatalf("%v %v", m["libraryAvailable"], m["libraryCount"])
	}
	by := map[string]CatalogRow{}
	for _, row := range m["rows"].([]CatalogRow) {
		by[row.ID] = row
	}
	if k := by["MTX_Frameblade_Knife_Variant"]; !k.Unlockable || !k.Unlocked {
		t.Fatalf("the token whose id is in the library: %+v", k)
	}
	if s := by["B1C3_Atre_Sword"]; !s.Unlockable || s.Unlocked {
		t.Fatalf("a cosmetic that is not unlocked yet: %+v", s)
	}
}

func TestUnlockingASetAddsItsIdsWithoutTouchingTheBackpackOrTheRestOfTheActor(t *testing.T) {
	o := libraryOps(t)
	items := func() int64 { return mustOne(t, o, `select count(*) n from items`)["n"].(int64) }
	before := items()
	want := map[string]bool{}
	for _, e := range catalog.All() {
		if e.Category == "customizations" && e.Source == "Customizations" && strings.HasPrefix(e.ID, "B1C3_Atre") {
			for _, id := range catalog.Unlocks(e.ID) {
				want[id] = true
			}
		}
	}
	if len(want) < 10 {
		t.Fatalf("the Atreides set should unlock many ids, got %d", len(want))
	}
	r, err := o.UnlockCustomizations(Args{"group": "atreides"})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.(map[string]any)["added"]; got != len(want) {
		t.Fatalf("added %v, want %d", got, len(want))
	}
	lib := libraryIDs(t, o)
	for id := range want {
		if !lib[id] {
			t.Fatalf("%s was not added", id)
		}
	}
	if !lib["MTX_Frameblade_Knife"] || len(lib) != len(want)+1 {
		t.Fatalf("the old entry stays and nothing else is added: %d", len(lib))
	}
	if items() != before {
		t.Fatal("nothing goes into a backpack")
	}
	if k := mustOne(t, o, `select json_extract(properties,'$.Other.keep[2]') n from actors where id=2`)["n"]; k != int64(3) {
		t.Fatalf("the rest of the actor's properties: %v", k)
	}
	// a second time adds nothing, and says so
	r2, err := o.UnlockCustomizations(Args{"group": "atreides"})
	if err != nil || r2.(map[string]any)["added"] != 0 {
		t.Fatalf("%v %v", r2, err)
	}
	if len(libraryIDs(t, o)) != len(lib) {
		t.Fatal("the same ids were added twice")
	}
}

func TestUnlockingOneCosmeticByItsItemId(t *testing.T) {
	o := libraryOps(t)
	r, err := o.UnlockCustomizations(Args{"id": "B1C3_Atre_Sword"})
	if err != nil || r.(map[string]any)["added"] != 1 {
		t.Fatalf("%v %v", r, err)
	}
	if !libraryIDs(t, o)["Atre_Sword"] {
		t.Fatal("Atre_Sword is the id the game's table gives for that item")
	}
}

func TestUnlockingRefusesWhatItCannotDo(t *testing.T) {
	o := libraryOps(t)
	for name, a := range map[string]Args{"nothing chosen": {}, "an unknown set": {"group": "nope"}, "an unknown cosmetic": {"id": "NotAThing"}} {
		if _, err := o.UnlockCustomizations(a); err == nil {
			t.Errorf("%s: should be refused", name)
		}
	}
	// a character with no library list: refused, nothing written
	bare := &Ops{S: testsave.Player(t)}
	if _, err := bare.UnlockCustomizations(Args{"group": "atreides"}); err == nil || !strings.Contains(err.Error(), "no customization library") {
		t.Fatalf("%v", err)
	}
	if bare.S.Dirty() {
		t.Fatal("nothing should be pending")
	}
}

func TestUnlockingIsOneEditInTheReviewAndCanBeDiscarded(t *testing.T) {
	o := libraryOps(t)
	if _, err := o.UnlockCustomizations(Args{"group": "smuggler"}); err != nil {
		t.Fatal(err)
	}
	ops := o.S.Ops()
	if len(ops) != 1 || !strings.HasPrefix(ops[0].Desc, "unlock ") || !strings.Contains(ops[0].Desc, "(smuggler)") {
		t.Fatalf("%+v", ops)
	}
	if err := o.S.Discard(); err != nil {
		t.Fatal(err)
	}
	if len(libraryIDs(t, o)) != 1 {
		t.Fatal("discard puts the library back")
	}
}

package augments

import (
	"strings"
	"testing"
)

// An augmented item as a real save stores it (a Bulwark Helmet with two grade-5 augments).
const realStats = `{"FAugmentedItemStats":[[],{"AppliedAugmentQualities":[5,5],"AppliedAugmentRollData":[{"AppliedEffectIndices":[],"StatRolls":[1,1,1,1]},{"AppliedEffectIndices":[],"StatRolls":[1,1]}],"AppliedAugments":[{"Name":"T6_Augment_Armor16"},{"Name":"T6_Augment_Armor10"}]}],"FClothingHydrationStats":[[],{}],"FCustomizationStats":[[],{"VariantId":"Fremen_FedaykinArmor_Helmet"}],"FDeteriorationStats":[[],{}],"FItemStackAndDurabilityStats":[[],{"CurrentDurability":99.732658}]}`

func TestStatsReproducesARealItemExactly(t *testing.T) {
	plain, err := Stats(realStats, nil)
	if err != nil || strings.Contains(plain, "FAugmentedItemStats") {
		t.Fatalf("clearing: %v %s", err, plain)
	}
	got, err := Stats(plain, []Applied{Perfect("T6_Augment_Armor16", 5), Perfect("T6_Augment_Armor10", 5)})
	if err != nil {
		t.Fatal(err)
	}
	if got != realStats {
		t.Fatalf("not the game's own shape:\n got %s\nwant %s", got, realStats)
	}
}

func TestRollCountsMatchRealAugments(t *testing.T) {
	if RollCount("T6_Augment_Armor16") != 4 || RollCount("T6_Augment_Armor10") != 2 {
		t.Fatalf("rolls: %d %d", RollCount("T6_Augment_Armor16"), RollCount("T6_Augment_Armor10"))
	}
}

func TestReadAppliedKeepsStoredQualityAndRolls(t *testing.T) {
	a := ReadApplied(realStats)
	if len(a) != 2 || a[0].ID != "T6_Augment_Armor16" || string(a[0].Quality) != "5" || string(a[1].RollData) != `{"AppliedEffectIndices":[],"StatRolls":[1,1]}` {
		t.Fatalf("%+v", a)
	}
	again, err := Stats(realStats, a)
	if err != nil || again != realStats {
		t.Fatalf("round trip changed the stats: %v\n%s", err, again)
	}
}

func TestFitFollowsTheConsoleRules(t *testing.T) {
	h := For("Combat_Heavy_Unique_Reinforced_Helmet_06", "Bulwark Helmet", "clothing")
	if h.Kind != KindClothing || h.Limit != 2 || len(h.Options) == 0 {
		t.Fatalf("helmet: kind=%s limit=%d options=%d", h.Kind, h.Limit, len(h.Options))
	}
	found := false
	for _, o := range h.Options {
		if o.ID == "T6_Augment_Armor16" {
			found = true
		}
		if strings.Contains(o.ID, "Acuracy") {
			t.Fatalf("a ranged-weapon augment must not fit a helmet: %s", o.ID)
		}
	}
	if !found {
		t.Fatal("the augments on the real helmet must be offered for it")
	}
	if err := Check("Combat_Heavy_Unique_Reinforced_Helmet_06", "Bulwark Helmet", "clothing", []string{"T6_Augment_Armor16", "T6_Augment_Armor10"}); err != nil {
		t.Fatalf("the real helmet's own augments must be valid: %v", err)
	}
	if Check("Combat_Heavy_Unique_Reinforced_Helmet_06", "Bulwark Helmet", "clothing", []string{"T6_Augment_Armor16", "T6_Augment_Armor10", "T6_Augment_Armor11"}) == nil {
		t.Fatal("clothing holds two")
	}
	if Check("Combat_Heavy_Unique_Reinforced_Helmet_06", "Bulwark Helmet", "clothing", []string{"T6_Augment_Acuracy1"}) == nil {
		t.Fatal("an augment for another kind of item must be refused")
	}
	if Check("Combat_Heavy_Unique_Reinforced_Helmet_06", "Bulwark Helmet", "clothing", []string{"T6_Augment_Armor16", "T6_Augment_Armor16"}) == nil {
		t.Fatal("duplicates must be refused")
	}
}

func TestItemsThatTakeNoAugments(t *testing.T) {
	for _, c := range [][3]string{{"CopperOre", "Copper Ore", "resources"}, {"Crysknife_Schematic", "Crysknife Schematic", "schematics"}, {"Unknown_Gun_XYZ", "Unknown", "weapons"}} {
		f := For(c[0], c[1], c[2])
		if f.Limit != 0 || len(f.Options) != 0 {
			t.Errorf("%v must offer nothing, got %+v", c, f)
		}
	}
	if Check("CopperOre", "Copper Ore", "resources", []string{"T6_Augment_Armor16"}) == nil {
		t.Fatal("ore takes no augments")
	}
}

func TestStatsKeepsOtherPartsAndRejectsGarbage(t *testing.T) {
	out, err := Stats(`{"FItemStackAndDurabilityStats":[[],{"CurrentDurability":1.5}]}`, []Applied{Perfect("T6_Augment_Armor10", 3)})
	if err != nil || !strings.Contains(out, `"CurrentDurability":1.5`) || !strings.Contains(out, `"AppliedAugmentQualities":[3]`) {
		t.Fatalf("%v %s", err, out)
	}
	if _, err := Stats("not json", nil); err == nil {
		t.Fatal("unreadable stats must be refused, not overwritten")
	}
}

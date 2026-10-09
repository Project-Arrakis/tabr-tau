package journey

import "testing"

func TestDisplayNames(t *testing.T) {
	if got := DisplayName("DA_MQ_ANewBeginning"); got != "A New Beginning" {
		t.Errorf("a catalog alias is used: %q", got)
	}
	if got := DisplayName("DA_LDR_Crafting_ReadingTheMail_01.Place Sensors.1"); got != "Place Sensors 1" {
		t.Errorf("a numbered step carries its parent's name: %q", got)
	}
	if got := DisplayName("DA_Dunipedia_KnownUniverse"); got != "Dunipedia Known Universe" {
		t.Errorf("one prefix is stripped and camel case tidied, as in the console: %q", got)
	}
	if got := DisplayName("DA_X"); got == "" {
		t.Error("never empty")
	}
}

func TestTreeAndOrder(t *testing.T) {
	ids := map[string]bool{"A": true, "A.B": true, "A.B.C": true, "A.D": true}
	if Parent("A.B.C", ids) != "A.B" || Parent("A", ids) != "" || Depth("A.B.C", ids) != 2 {
		t.Fatal("parent and depth")
	}
	// a missing middle level is skipped
	if Parent("A.X.Y", ids) != "A" {
		t.Fatal("nearest existing ancestor")
	}
	if !Less("A", "A.B") || Less("A.B", "A") {
		t.Fatal("a parent comes before its children")
	}
	if Less("A", "A") {
		t.Fatal("irreflexive")
	}
}

func TestGroupsAndContracts(t *testing.T) {
	if Group("DA_CT_X") != "contract" || Group("DA_LDR_X") != "contract" || Group("DA_MQ_X") != "story" {
		t.Fatal("groups")
	}
	cs := Contracts()
	if len(cs) < 100 || len(ContractTags(cs[0])) == 0 || ContractName(cs[0]) == "" {
		t.Fatalf("contracts: %d", len(cs))
	}
}

package ops

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

func TestCommas(t *testing.T) {
	for in, want := range map[int64]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 1234567: "1,234,567", -1234: "-1,234", -999: "-999", 1000000000000: "1,000,000,000,000"} {
		if got := commas(in); got != want {
			t.Errorf("commas(%d) = %q, want %q", in, got, want)
		}
	}
}

// A waiting edit is listed in words: the item by its in-game name with its id, and a value as before to after (UX-04 of the 2026-10-15
// audit: "edit item 4521" and "currency 0 +100" said neither what nor how much).
func TestReviewDescriptionsNameTheItemAndShowBeforeAndAfter(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	r, err := o.GiveItem(Args{"template_id": "MelangeSpice", "quantity": 5.0, "quality": 2.0})
	if err != nil {
		t.Fatal(err)
	}
	id := r.(map[string]any)["itemId"].(int64)
	if _, err := o.SetItem(Args{"id": float64(id), "stack_size": 25.0, "quality": 3.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.SetItem(Args{"id": float64(id), "stack_size": 500.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.DeleteItem(Args{"id": float64(id)}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.AddCurrency(Args{"currency": 0.0, "amount": 1500.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.AddCurrency(Args{"currency": 1.0, "amount": 100.0}); err != nil {
		t.Fatal(err)
	}
	got := o.S.Pending()
	want := []string{
		"Give 5 x Spice Melange (grade 2)",
		"Spice Melange (item " + itoa(id) + "): stack 5 → 25, grade 2 → 3",
		"Spice Melange (item " + itoa(id) + "): stack 25 → 500",
		"Delete 500 x Spice Melange (item " + itoa(id) + ")",
		"Solari in wallet +1,500 (",
		"House credit in wallet +100 (",
	}
	if len(got) != len(want) {
		t.Fatalf("pending = %q", got)
	}
	for i, w := range want {
		if !strings.HasPrefix(got[i], w) {
			t.Errorf("edit %d: got %q, want it to start with %q", i+1, got[i], w)
		}
	}
	if !strings.Contains(got[4], "→") || !strings.Contains(got[5], "→") {
		t.Errorf("a currency edit shows the balance before and after: %q %q", got[4], got[5])
	}
}

func TestReputationDescriptionNamesTheFaction(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into factions(id,name) values (9,'House Testing');`)}
	if _, err := o.SetReputation(Args{"faction_id": 9.0, "amount": 500.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.SetReputation(Args{"faction_id": 9.0, "amount": 1500.0}); err != nil {
		t.Fatal(err)
	}
	got := o.S.Pending()
	if len(got) != 2 || got[0] != "Reputation with House Testing: 0 → 500" || got[1] != "Reputation with House Testing: 500 → 1,500" {
		t.Fatalf("pending = %q", got)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

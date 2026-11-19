package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The wallet write is one helper: the one the Add currency route uses is the one a Landsraad raise uses (#827). It answers the balance it found, 0 for a wallet with no row, and
// creates the row.
func TestAddWalletReturnsTheBalanceBeforeAndCreatesAMissingWallet(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `insert into player_virtual_currency_balances(player_controller_id,currency_id,balance) values (1,0,250000);`)}
	steps := []struct {
		currency, amount, before, after int64
	}{{0, 1000, 250000, 251000}, {1, 50, 0, 50}, {1, 25, 50, 75}}
	for _, s := range steps {
		var before int64
		if _, err := o.S.Mutate("test wallet", func(m *save.Mut) error {
			var err error
			before, err = addWallet(m, 1, s.currency, s.amount)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if before != s.before || balance(t, o, int(s.currency)).(int64) != s.after {
			t.Fatalf("currency %d +%d: before %d (want %d), after %v (want %d)", s.currency, s.amount, before, s.before, balance(t, o, int(s.currency)), s.after)
		}
	}
}

// One cut of the house prefix, for a message and for the line table alike (#827).
func TestHouseShortCutsThePrefixInOnePlace(t *testing.T) {
	for in, want := range map[string]string{
		"DA_HouseKenola": "Kenola", "Kenola": "Kenola", "DA_House": "DA_House", "DA_Hous": "DA_Hous", "": "", "DA_HouseDA_HouseX": "DA_HouseX",
	} {
		if got := houseShort(in); got != want {
			t.Errorf("houseShort(%q) = %q, want %q", in, got, want)
		}
	}
	if lineOfHouse("DA_HouseKenola") != "Gathering" || lineOfHouse("Kenola") != "Gathering" || lineOfHouse("DA_House") != "" || lineOfHouse("DA_HouseNobody") != "" {
		t.Fatal("the line of a house follows the one cut")
	}
	long := "DA_HouseKenola"
	if houseOf(save.Row{"house_name": long}) != "Kenola" {
		t.Fatal("a message names the house through the same cut")
	}
}

// The tier rule: rows in, tier out. The largest threshold whose item stands, from whichever rows are given (tiers and swatch), and 0 when none stands (#827).
func TestStandingTierIsTheHighestThresholdWhoseItemStands(t *testing.T) {
	rows := []save.Row{
		{"threshold": int64(1200), "template_id": "a"}, {"threshold": 2400.0, "template_id": "b"}, {"threshold": int64(6000), "template_id": "swatch"},
		{"threshold": nil, "template_id": "c"},
	}
	stands := func(want ...string) func(any) bool {
		return func(tmpl any) bool {
			for _, w := range want {
				if tmpl == w {
					return true
				}
			}
			return false
		}
	}
	for name, c := range map[string]struct {
		stands func(any) bool
		want   float64
	}{
		"all stand": {stands("a", "b", "swatch", "c"), 6000}, "the swatch alone": {stands("swatch"), 6000}, "the lower tiers": {stands("a", "b"), 2400},
		"an item that is not a row": {stands("z"), 0}, "none": {stands(), 0}, "a tier with no number": {stands("c"), 0},
	} {
		if got := standingTier(rows, c.stands); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
	if standingTier(nil, stands("a")) != 0 {
		t.Fatal("no rows, no tier")
	}
}

// A delivered row with no time, or one before the term, does not stand (the same for the board and for the write).
func TestStandsInNeedsADeliveredRowOfThisTerm(t *testing.T) {
	delivered := map[string]save.Row{
		deliveredKey("H", "a"): {"last_updated": int64(150)}, deliveredKey("H", "b"): {"last_updated": int64(50)}, deliveredKey("H", "c"): {"last_updated": nil},
		deliveredKey("G", "a"): {"last_updated": int64(150)},
	}
	stands := standsIn(delivered, "H", 100)
	if !stands("a") || stands("b") || stands("c") || stands("d") {
		t.Fatalf("a %v b %v c %v d %v", stands("a"), stands("b"), stands("c"), stands("d"))
	}
	if standsIn(delivered, "G", 100)("b") || !standsIn(delivered, "G", 100)("a") {
		t.Fatal("another house's rows are its own")
	}
}

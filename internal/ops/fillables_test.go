package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

func addItem(t *testing.T, o *Ops, id, inv int, tpl, stats string) {
	t.Helper()
	_, err := o.S.Mutate("fixture", func(m *save.Mut) error {
		_, err := m.Exec(`insert into items values (?, ?, 1, ?, ?, 0, 0, ?, 0)`, id, inv, 20+id, tpl, stats)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRefillContainers(t *testing.T) {
	o := newOps(t)
	_, err := o.S.Mutate("fixture", func(m *save.Mut) error {
		_, err := m.Exec(`insert into actors values (99, 'Other', 'HaggaBasin', 0, 0, 0, 2); insert into inventories values (9, 99, null, 4, 3, 100)`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	dur := `"FItemStackAndDurabilityStats":[[],{}]`
	addItem(t, o, 201, 1, "Literjon_T6", `{"FFillableItemStats":[[],{"CurrentAmount":11000.0,"FillableType":"Water"}],"FItemStackAndDurabilityStats":[[],{"CurrentDurability":99.5}]}`)
	addItem(t, o, 202, 1, "Literjon", `{`+dur+`}`) // empty: no fill record at all
	addItem(t, o, 203, 1, "Decajon", `{`+dur+`,"FFillableItemStats":[[],{"CurrentAmount":10000.0,"FillableType":"Water"}]}`)
	addItem(t, o, 204, 1, "Bloodsack_02", `{"FFillableItemStats":[[],{}],`+dur+`}`)
	addItem(t, o, 205, 1, "Mystery_Flask", `{"FFillableItemStats":[[],{"CurrentAmount":5.0,"FillableType":"Water"}],`+dur+`}`)
	addItem(t, o, 206, 9, "Literjon", `{`+dur+`}`)                                                                                        // in someone else's storage: not the player's
	addItem(t, o, 207, 1, "Junk", `{bad json`)                                                                                            // damaged stats must not stop the refill
	addItem(t, o, 208, 1, "HighCapacityLiterjon_06", `{`+dur+`,"FFillableItemStats":[[],{"CurrentAmount":3000,"FillableType":"Water"}]}`) // integer amount, already full
	addItem(t, o, 209, 1, "Literjon", `{`+dur+`,"FFillableItemStats":[[],{"CurrentAmount":40.0,"FillableType":"Blood"}]}`)                // not water

	before, _ := o.S.One(`select stats from items where id=203`)
	r, err := o.RefillContainers()
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["filled"].(int) != 2 || res["alreadyFull"].(int) != 2 {
		t.Fatalf("filled/full wrong: %v", res)
	}
	if sk := res["skippedUnknown"].([]string); len(sk) != 1 || sk[0] != "Mystery_Flask x1" {
		t.Fatalf("unknown container not reported: %v", sk)
	}
	stats := func(id int) string {
		row, _ := o.S.One(`select stats from items where id=?`, id)
		return row["stats"].(string)
	}
	if got := stats(201); !strings.Contains(got, `"CurrentAmount":20000.0`) || !strings.Contains(got, `"CurrentDurability":99.5`) {
		t.Fatalf("Mk6 not filled to 20000 with durability kept: %s", got)
	}
	if got, want := stats(202), `{`+dur+`,"FFillableItemStats":[[],{"CurrentAmount":1000.0,"FillableType":"Water"}]}`; got != want {
		t.Fatalf("empty Literjon\n got %s\nwant %s", got, want)
	}
	if stats(203) != before["stats"] {
		t.Fatal("a full container must not be rewritten")
	}
	if !strings.Contains(stats(204), `"FFillableItemStats":[[],{}]`) {
		t.Fatalf("blood sack touched: %s", stats(204))
	}
	if !strings.Contains(stats(205), `"CurrentAmount":5.0`) {
		t.Fatalf("unknown container touched: %s", stats(205))
	}
	if stats(206) != `{`+dur+`}` {
		t.Fatalf("another actor's container touched: %s", stats(206))
	}
	if !strings.Contains(stats(209), `"FillableType":"Blood"`) || !strings.Contains(stats(209), `"CurrentAmount":40.0`) {
		t.Fatalf("a container holding something else was touched: %s", stats(209))
	}
	if !strings.Contains(stats(208), `"CurrentAmount":3000,`) {
		t.Fatalf("a full container must not be rewritten: %s", stats(208))
	}
	again, _ := o.RefillContainers()
	if again.(map[string]any)["filled"].(int) != 0 {
		t.Fatalf("second run must find nothing to fill: %v", again)
	}
}

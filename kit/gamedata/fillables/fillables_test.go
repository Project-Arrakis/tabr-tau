package fillables

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// rowsHash is the SHA-256 of Marshal("", Table()): the rows alone, in the canonical text. It changes when a row is added, removed or changed, and not when the source line is
// reworded. A refreshed snapshot changes it on purpose, in the same pull request as the tests below that pin the counts.
const rowsHash = "71c5b015695c5eb5d892f6ecaee844b2cfd8a811ee3dda287b7e3637ce6d258d"

func TestTheSnapshotHasTheRowsOfTheGamesTable(t *testing.T) {
	tab := Table()
	if len(tab) != 58 {
		t.Fatalf("%d rows, want 58", len(tab))
	}
	var water, blood, waterContainers, waterNotContainers, bloodContainers int
	for _, r := range tab {
		switch r.Restriction {
		case Water:
			water++
			if r.Container {
				waterContainers++
			} else {
				waterNotContainers++
			}
		case Blood:
			blood++
			if r.Container {
				bloodContainers++
			}
		default:
			t.Errorf("%s takes %q, a liquid this package does not name", r.Name, r.Restriction)
		}
	}
	// 49 rows take Water (17 containers; 32 that are not: the worn stillsuits and the consumable WaterPack) and 9 take Blood (all containers).
	if water != 49 || blood != 9 || waterContainers != 17 || waterNotContainers != 32 || bloodContainers != 9 {
		t.Errorf("water %d (containers %d, others %d), blood %d (containers %d)", water, waterContainers, waterNotContainers, blood, bloodContainers)
	}
}

func TestKnownRows(t *testing.T) {
	for name, want := range map[string]Row{
		"Literjon":                             {Name: "Literjon", MaxAmount: 1000, Restriction: Water, Container: true},
		"Literjon_T6":                          {Name: "Literjon_T6", MaxAmount: 20000, Restriction: Water, Container: true},
		"Decajon":                              {Name: "Decajon", MaxAmount: 10000, Restriction: Water, Container: true},
		"Dewpack":                              {Name: "Dewpack", MaxAmount: 250, Restriction: Water, Container: true},
		"HighCapacityLiterjon_06":              {Name: "HighCapacityLiterjon_06", MaxAmount: 3000, Restriction: Water, Container: true},
		"Bloodsack_01":                         {Name: "Bloodsack_01", MaxAmount: 3000, Restriction: Blood, Container: true},
		"Bloodsack_Unique_Durable_06":          {Name: "Bloodsack_Unique_Durable_06", MaxAmount: 30000, Restriction: Blood, Container: true},
		"AdvancedStillsuit":                    {Name: "AdvancedStillsuit", MaxAmount: 500, Restriction: Water},
		"Stillsuit_Unique_HighCapacity_06_Top": {Name: "Stillsuit_Unique_HighCapacity_06_Top", MaxAmount: 1500, Restriction: Water},
		"WaterPack_Consumable":                 {Name: "WaterPack_Consumable", MaxAmount: 100, CurrentAmount: 100, Restriction: Water},
	} {
		got, ok := Lookup(name)
		if !ok || got != want {
			t.Errorf("%s: %+v (found %v), want %+v", name, got, ok, want)
		}
	}
	if _, ok := Lookup("NoSuchItem"); ok {
		t.Error("an unknown template was found")
	}
	if _, ok := Lookup(""); ok {
		t.Error("the empty template was found")
	}
}

func TestOnlyTheConsumableStartsWithSomethingInIt(t *testing.T) {
	for _, r := range Table() {
		if (r.CurrentAmount != 0) != (r.Name == "WaterPack_Consumable") {
			t.Errorf("%s starts with %v", r.Name, r.CurrentAmount)
		}
		if r.CurrentAmount > float64(r.MaxAmount) {
			t.Errorf("%s starts above its capacity", r.Name)
		}
	}
}

func TestRowsAreSortedUniqueAndHaveACapacity(t *testing.T) {
	tab := Table()
	if !sort.SliceIsSorted(tab, func(a, b int) bool { return tab[a].Name < tab[b].Name }) {
		t.Error("rows are not sorted by name")
	}
	seen := map[string]bool{}
	for _, r := range tab {
		if r.Name == "" || seen[r.Name] || r.MaxAmount <= 0 {
			t.Errorf("row %+v", r)
		}
		seen[r.Name] = true
	}
}

func TestTheSourceNamesTheGamesTable(t *testing.T) {
	if !strings.Contains(Source(), "DT_ItemTableFillables") || !strings.Contains(Source(), "read-only") {
		t.Errorf("source %q", Source())
	}
}

// The committed file is the canonical text of its own rows, so a hand edit that breaks the order, the indent or a number's spelling is seen.
func TestTheSnapshotIsInCanonicalForm(t *testing.T) {
	out, err := Marshal(Source(), Table())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, snapshot) {
		t.Error("item_table_fillables.json is not what Marshal makes of its own rows (a hand edit, or a CRLF checkout)")
	}
}

func TestTheRowsHaveNotChangedUnnoticed(t *testing.T) {
	out, err := Marshal("", Table())
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(out)
	if got := hex.EncodeToString(sum[:]); got != rowsHash {
		t.Errorf("the rows hash to %s, expected %s: a row of the snapshot was added, removed or changed; update this hash with the CHANGELOG entry and the tests that pin the counts", got, rowsHash)
	}
}

func TestTableReturnsACopy(t *testing.T) {
	a := Table()
	a[0].MaxAmount = -1
	a[1] = Row{}
	if b := Table(); b[0].MaxAmount <= 0 || b[1].Name == "" {
		t.Errorf("a change to the returned slice reached the table: %+v %+v", b[0], b[1])
	}
	if r, _ := Lookup(a[0].Name); r.MaxAmount <= 0 {
		t.Error("a change to the returned slice reached Lookup")
	}
}

func TestMarshalSortsAndParseReadsItBack(t *testing.T) {
	in := []Row{
		{Name: "Zeta", MaxAmount: 250, Restriction: Water, Container: true},
		{Name: "Alpha_Stillsuit", MaxAmount: 500, Restriction: Water},
		{Name: "WaterPack", MaxAmount: 100, CurrentAmount: 100, Restriction: Water},
	}
	a, err := Marshal("test source", in)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Marshal("test source", []Row{in[1], in[2], in[0]})
	if !bytes.Equal(a, b) {
		t.Error("the same rows in another order gave other bytes")
	}
	if in[0].Name != "Zeta" {
		t.Error("Marshal sorted the caller's slice")
	}
	if !bytes.HasSuffix(a, []byte("}\n")) {
		t.Error("no final newline")
	}
	src, rows, err := Parse(a)
	if err != nil || src != "test source" || len(rows) != 3 {
		t.Fatalf("%q %v %v", src, rows, err)
	}
	if rows[0].Name != "Alpha_Stillsuit" || rows[1].Name != "WaterPack" || rows[2].Name != "Zeta" || rows[2].MaxAmount != 250 || !rows[2].Container || rows[1].CurrentAmount != 100 {
		t.Errorf("rows %+v", rows)
	}
}

func TestParseRefusesWhatIsNotAGoodTable(t *testing.T) {
	good := Row{Name: "A", MaxAmount: 1, Restriction: Water}
	for name, rows := range map[string][]Row{
		"duplicate":       {good, good},
		"unsorted":        {{Name: "B", MaxAmount: 1, Restriction: Water}, good},
		"empty":           nil,
		"no name":         {{MaxAmount: 1, Restriction: Water}},
		"negative amount": {{Name: "A", MaxAmount: -1, Restriction: Water}},
	} {
		b, err := json.Marshal(file{Source: "s", Rows: rows})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := Parse(b); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	for name, in := range map[string]string{
		"not json":      `nope`,
		"unknown field": `{"source":"s","rows":[{"name":"A","maxAmount":1,"restriction":"Water","container":true,"extra":1}]}`,
		"unknown top":   `{"source":"s","rows":[{"name":"A","maxAmount":1}],"extra":1}`,
		"fraction":      `{"source":"s","rows":[{"name":"A","maxAmount":1.5}]}`,
		"too large":     `{"source":"s","rows":[{"name":"A","maxAmount":99999999999}]}`,
	} {
		if _, _, err := Parse([]byte(in)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

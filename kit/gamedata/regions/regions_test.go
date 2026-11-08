package regions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// haggaBasin is the table as the game's twelve areas of the Hagga Basin are known, written a second time so that a changed row cannot pass unnoticed.
var haggaBasin = map[int64]string{
	2: "Western Vermillius Gap", 3: "Hagga Rift", 4: "Mysa Tarill", 5: "Eastern Shield Wall", 6: "The O'odham", 7: "Jabal Eifrit Al-janub",
	8: "Western Shield Wall", 11: "Sheol", 12: "Hagga Basin South", 13: "Jabal Eifrit Al-sharq", 14: "Eastern Vermillius Gap", 16: "Jabal Eifrit Al-gharb",
}

// tableHash is the SHA-256 of the lines "id=name" of the Hagga Basin in id order. It changes when an area is added, removed or renamed; a refreshed table changes it on purpose,
// in the same pull request as haggaBasin above and the CHANGELOG entry.
const tableHash = "97873a5c57d22fe0349878546468a630a497bf4c7ad1d53f7566d02110d627b8"

func TestTheTableHasTheTwelveAreasOfTheHaggaBasin(t *testing.T) {
	if got := Maps(); len(got) != 1 || got[0] != "HaggaBasin" {
		t.Fatalf("maps: %v", got)
	}
	names := Names("HaggaBasin")
	if len(names) != 12 {
		t.Fatalf("%d names, want 12: %v", len(names), names)
	}
	for id, want := range haggaBasin {
		if got := Name("HaggaBasin", id); got != want {
			t.Errorf("area %d: %q, want %q", id, got, want)
		}
		if names[id] != want {
			t.Errorf("Names: area %d: %q, want %q", id, names[id], want)
		}
	}
	ids := make([]int64, 0, len(names))
	for id := range names {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	var sb strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&sb, "%d=%s\n", id, names[id])
	}
	sum := sha256.Sum256([]byte(sb.String()))
	if got := hex.EncodeToString(sum[:]); got != tableHash {
		t.Errorf("the table hashes to %s, expected %s: an area was added, removed or renamed; update the hash with the CHANGELOG entry", got, tableHash)
	}
}

// No name is made up: area 0 (no area), an id that has no name, a negative id, another map (whose ids mean other places) and an empty map name give nothing.
func TestUnknownAreasAndOtherMapsHaveNoName(t *testing.T) {
	for _, c := range []struct {
		m  string
		id int64
	}{{"HaggaBasin", 0}, {"HaggaBasin", 1}, {"HaggaBasin", 9}, {"HaggaBasin", 99}, {"HaggaBasin", -3}, {"DeepDesert", 3}, {"", 3}, {"Arrakeen", 2}, {"_comment", 3}} {
		if got := Name(c.m, c.id); got != "" {
			t.Errorf("%s %d: %q", c.m, c.id, got)
		}
	}
	if Names("DeepDesert") != nil || Names("") != nil || Names("_comment") != nil {
		t.Error("a map with no names has no table")
	}
}

// The table handed out is a copy: a caller cannot change the names.
func TestNamesReturnsACopy(t *testing.T) {
	m := Names("HaggaBasin")
	m[3] = "changed"
	delete(m, 4)
	if Name("HaggaBasin", 3) != "Hagga Rift" || Names("HaggaBasin")[3] != "Hagga Rift" || Name("HaggaBasin", 4) != "Mysa Tarill" {
		t.Fatal("the table was changed through the copy")
	}
}

func TestParse(t *testing.T) {
	good := `{"_comment": "note", "A": {"1": "One", "-2": "Minus two"}, "B": {"10": "Ten"}}`
	got, err := Parse([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["A"][1] != "One" || got["A"][-2] != "Minus two" || got["B"][10] != "Ten" {
		t.Errorf("parsed %v", got)
	}
	for name, text := range map[string]string{
		"not json":           `nope`,
		"no map":             `{"_comment": "only"}`,
		"empty object":       `{}`,
		"empty map":          `{"A": {}}`,
		"a map that is text": `{"A": "x"}`,
		"comment not text":   `{"_comment": 3, "A": {"1": "One"}}`,
		"id not a number":    `{"A": {"x": "One"}}`,
		"id with a sign":     `{"A": {"+1": "One"}}`,
		"id leading zero":    `{"A": {"01": "One"}}`,
		"id fraction":        `{"A": {"1.5": "One"}}`,
		"empty name":         `{"A": {"1": ""}}`,
		"name not text":      `{"A": {"1": 5}}`,
	} {
		if _, err := Parse([]byte(text)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// The data comes from a MIT-licensed project, so the module's NOTICE has that project's copyright and licence and names the file.
func TestTheNoticeCarriesTheLicenceOfTheTable(t *testing.T) {
	b, err := os.ReadFile("../../NOTICE")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"hagga-regions.json", "Copyright (c) 2026 RedBlink", "Permission is hereby granted", "82548d72"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("NOTICE does not say %q", want)
		}
	}
}

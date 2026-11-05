package basefile

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// Writing the console's file (#534): the inverse of console.go. The golden test reads the made-up console file of consolefixture with the reader, writes it again, reads that again,
// and the two must hold the same rows once every id is renumbered.

func rowsOf(b *Base) map[string][][]any {
	out := map[string][][]any{}
	for _, t := range Tables {
		rs := b.Of(t.Name)
		for _, r := range rs.R {
			row := make([]any, len(r))
			for i, v := range r {
				row[i] = Arg(v)
			}
			out[t.Name] = append(out[t.Name], row)
		}
	}
	return out
}

var (
	idColumn = map[string]bool{"id": true, "actor_id": true, "entity_id": true, "inventory_id": true, "building_id": true, "owner_entity_id": true, "totem_id": true,
		"permission_actor_id": true, "player_id": true}
	farRef = regexp.MustCompile(`(!![a-z]{3})#[1-9][0-9]*`)
)

// shape is every table of a base with the ids left out and the references inside JSON made alike, the rows sorted: two bases with the same shape differ only in their ids.
func shape(b *Base) string {
	var out strings.Builder
	for _, t := range Tables {
		out.WriteString("== " + t.Name + "\n")
		var lines []string
		for _, r := range b.Of(t.Name).R {
			var parts []string
			for i, c := range t.Cols {
				if c.Kind == Int && idColumn[c.Name] {
					continue
				}
				s := fmt.Sprint(Arg(r[i]))
				s = farRef.ReplaceAllString(s, "$1#N")
				parts = append(parts, c.Name+"="+s)
			}
			lines = append(lines, strings.Join(parts, " "))
		}
		sort.Strings(lines)
		out.WriteString(strings.Join(lines, "\n") + "\n")
	}
	return out.String()
}

func headerOf() ConsoleHeader {
	return ConsoleHeader{At: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC), Name: "Test Reserve", RawName: "Test Reserve", Map: "HaggaBasin", TotemType: "Totem_Placeable",
		Patches: Patches{Count: 2, Latest: []string{"A", "B"}}, Editor: "9.9.9"}
}

func TestConsoleFileWrittenFromTheFixtureReadsBackAsTheSameRows(t *testing.T) {
	first := loadFixture(t, nil)
	res, err := WriteConsole(headerOf(), rowsOf(first), MaxFileBytes, MaxRows)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load(res.Data, MaxFileBytes, MaxRows)
	if err != nil {
		t.Fatalf("the reader refuses what the writer made: %v", err)
	}
	if err := second.Verify(); err != nil {
		t.Fatal(err)
	}
	if a, b := shape(first), shape(second); a != b {
		t.Fatalf("the rows differ after writing and reading again:\n--- fixture\n%s\n--- written\n%s", a, b)
	}
	// the counts the writer reports are the counts the reader finds
	k := second.Console.Kinds
	if res.Pieces != k["BuildingInstance"] || res.Placeables != k["Placeable"] || res.Items != k["itm"] || res.Entities != k["fgl"] || res.Actors != k["act"]-1 || res.Entries != second.Console.Entries {
		t.Errorf("counts %+v against kinds %v", res, k)
	}
	if c := second.Console; c.Name != "Test Reserve" || c.Map != "HaggaBasin" || c.Declared["pieces"] != 3 || c.Declared["placeables"] != 3 || c.Declared["items"] != 3 || c.GamePatches != 2 {
		t.Errorf("info %+v", c)
	}
	// the ids inside JSON name the file's own entries ("!!act@N" in the file): the totem's own id and the crafting queue's item
	got := second.Of("actors").R[0][second.Of("actors").Col("properties")]
	if !strings.Contains(string(got.(JSONText)), `"m_SelfUniqueID":"!!act#`) {
		t.Errorf("properties %v", got)
	}
	// writing is deterministic
	again, _ := WriteConsole(headerOf(), rowsOf(first), MaxFileBytes, MaxRows)
	if string(again.Data) != string(res.Data) {
		t.Error("two writes of the same rows differ")
	}
	// and a file that is read, written and read again, written again, is the file
	third, _ := Load(again.Data, MaxFileBytes, MaxRows)
	res3, err := WriteConsole(headerOf(), rowsOf(third), MaxFileBytes, MaxRows)
	if err != nil || string(res3.Data) != string(res.Data) {
		t.Errorf("a second round trip changes the file: %v", err)
	}
}

func TestConsoleWriterRefusesRowsItCannotWriteAsTheFile(t *testing.T) {
	base := func() map[string][][]any { return rowsOf(loadFixture(t, nil)) }
	col := func(table, name string) int {
		tb, _ := Find(table)
		for i, c := range tb.Cols {
			if c.Name == name {
				return i
			}
		}
		t.Fatal(name)
		return -1
	}
	for name, c := range map[string]struct {
		edit func(m map[string][][]any)
		want string
	}{
		"two backup records":          {func(m map[string][][]any) { m["base_backups"] = append(m["base_backups"], m["base_backups"][0]) }, "holds 2"},
		"a number that is not finite": {func(m map[string][][]any) { m["actors"][0][col("actors", "location_x")] = math.Inf(1) }, "not finite"},
		"a number marked non-finite":  {func(m map[string][][]any) { m["actors"][0][col("actors", "location_x")] = markNonFinite }, "not finite"},
		"text that is not UTF-8":      {func(m map[string][][]any) { m["placeables"][0][col("placeables", "building_type")] = "\xff\xfe" }, "UTF-8"},
		"JSON that is not valid":      {func(m map[string][][]any) { m["items"][0][col("items", "stats")] = "{broken" }, "valid JSON"},
		"JSON that is not an object":  {func(m map[string][][]any) { m["items"][0][col("items", "stats")] = "[1]" }, "not a JSON object"},
		"a reference inside a text":   {func(m map[string][][]any) { m["actors"][0][col("actors", "properties")] = `{"a":"see !!act#5"}` }, "inside other text"},
		"a flag that is not a flag":   {func(m map[string][][]any) { m["placeables"][0][col("placeables", "is_hologram")] = int64(7) }, "not 0 or 1"},
		"a row that names nothing":    {func(m map[string][][]any) { m["inventories"][0][col("inventories", "actor_id")] = int64(424242) }, "not in the file"},
		"an entity with no actor":     {func(m map[string][][]any) { m["actor_fgl_entities"] = m["actor_fgl_entities"][1:] }, "holds no id"},
		"a repeated actor":            {func(m map[string][][]any) { m["actors"] = append(m["actors"], m["actors"][0]) }, "two rows"},
	} {
		t.Run(name, func(t *testing.T) {
			m := base()
			c.edit(m)
			_, err := WriteConsole(headerOf(), m, MaxFileBytes, MaxRows)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want %q", err, c.want)
			}
		})
	}
	// limits
	if _, err := WriteConsole(headerOf(), base(), 100, MaxRows); err == nil || !strings.Contains(err.Error(), "the most a base file holds") {
		t.Errorf("size: %v", err)
	}
	if _, err := WriteConsole(headerOf(), base(), MaxFileBytes, 5); err == nil || !strings.Contains(err.Error(), "most a file holds") {
		t.Errorf("entries: %v", err)
	}
}

func TestConsoleWriterTreatsReferencesLikeTheGamesOwnExport(t *testing.T) {
	m := rowsOf(loadFixture(t, nil))
	tb, _ := Find("fgl_entities")
	var cc int
	for i, c := range tb.Cols {
		if c.Name == "components" {
			cc = i
		}
	}
	// a reference to anything outside the file, and the null reference, are the console's null reference "@0"; no "#" form is left (#538)
	m["fgl_entities"][0][cc] = `{"A":"!!act#9999","B":"!!itm#8888","C":"!!inv#7777","D":"!!act#0","E":"!!itm#0"}`
	res, err := WriteConsole(headerOf(), m, MaxFileBytes, MaxRows)
	if err != nil {
		t.Fatal(err)
	}
	s := string(res.Data)
	for _, w := range []string{`"A":"!!act@0"`, `"B":"!!itm@0"`, `"C":"!!inv@0"`, `"D":"!!act@0"`, `"E":"!!itm@0"`} {
		if !strings.Contains(s, w) {
			t.Errorf("file does not say %s", w)
		}
	}
	if res.FarActors != 1 || res.NulledRefs != 2 {
		t.Errorf("counts: far %d, nulled %d", res.FarActors, res.NulledRefs)
	}
	m["fgl_entities"][0][cc] = `{"A":"!!act#-4"}`
	if _, err := WriteConsole(headerOf(), m, MaxFileBytes, MaxRows); err == nil || !strings.Contains(err.Error(), "not an id") {
		t.Errorf("negative id: %v", err)
	}
}

func TestConsoleFileHeaderHasWhatTheConsoleChecksAndNothingPrivate(t *testing.T) {
	res, err := WriteConsole(headerOf(), rowsOf(loadFixture(t, nil)), MaxFileBytes, MaxRows)
	if err != nil {
		t.Fatal(err)
	}
	s := string(res.Data)
	for _, w := range []string{`"format": "dune-base-backup"`, `"version": 1`, `"exportedAt": "2026-11-01T10:00:00.000Z"`, `"ownerPlaceholderTransferId": 1`, `"appliedPatchesCount":2`,
		`"patchesChecksum":""`, `"ownerName":""`, `"writtenBy":"tabr-tau 9.9.9"`} {
		if !strings.Contains(s, w) {
			t.Errorf("the file does not say %s", w)
		}
	}
	// the owner and the account of the actors are not in it
	if strings.Contains(s, "owner_account_id\":1") || strings.Contains(s, "partition_id") {
		t.Error("a column that must not travel is in the file")
	}
}

// The console's own file records, per row, that the game's arrays are 0-based ("__lb"), on the pieces' transform and the totem's original location; the importing server restores
// the bounds from it, and the game reads some of those arrays by index (#538, compared with a file the console made).
func TestConsoleFileNotesTheZeroBasedArraysAsTheConsoleDoes(t *testing.T) {
	res, err := WriteConsole(headerOf(), rowsOf(loadFixture(t, nil)), MaxFileBytes, MaxRows)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Entries []struct {
			Kind string         `json:"kind"`
			Data map[string]any `json:"data"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(res.Data, &f); err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, e := range f.Entries {
		lb, has := e.Data["__lb"]
		switch e.Kind {
		case "BuildingInstance":
			seen[e.Kind]++
			if m, _ := lb.(map[string]any); !has || len(m) != 1 || m["transform"] != float64(0) {
				t.Errorf("a piece: __lb %v", lb)
			}
		case "Totem":
			seen[e.Kind]++
			if m, _ := lb.(map[string]any); !has || len(m) != 1 || m["landclaim_original_global_location"] != float64(0) {
				t.Errorf("the totem: __lb %v", lb)
			}
		default:
			if has {
				t.Errorf("%s has __lb %v", e.Kind, lb)
			}
		}
	}
	if seen["BuildingInstance"] != res.Pieces || seen["Totem"] != 1 {
		t.Errorf("seen %v", seen)
	}
}

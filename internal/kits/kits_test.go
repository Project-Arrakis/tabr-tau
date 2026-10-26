package kits

import (
	"encoding/json"
	"strings"
	"testing"
)

// Items the tests use: a helmet with two augment slots (the augment data lists T6_Augment_Armor10 and T6_Augment_Armor16 for it) and a stackable resource.
const (
	helmet = "Combat_Heavy_Unique_Reinforced_Helmet_06"
	fuel   = "SpicedFuelCell"
)

func starter() Kit {
	return Kit{Name: "Starter", Items: []Line{{Item: fuel, Quantity: 100}, {Item: helmet, Quantity: 1, Grade: 3, Augments: []string{"T6_Augment_Armor10"}, AugmentGrade: 4}}, XP: 5000, SkillPoints: 3}
}

func mustFail(t *testing.T, err error, words ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a refusal mentioning %v", words)
	}
	for _, w := range words {
		if !strings.Contains(err.Error(), w) {
			t.Fatalf("the refusal %q does not mention %q", err, w)
		}
	}
}

func TestAGoodKitIsValidAndNormalizeTidiesIt(t *testing.T) {
	k := starter()
	if err := k.Validate(); err != nil {
		t.Fatal(err)
	}
	k = Kit{Name: "  Tools ", Items: []Line{{Item: " " + fuel + " ", Quantity: 1, Augments: []string{" ", ""}}}}
	k.Normalize()
	if k.Name != "Tools" || k.Items[0].Item != fuel || k.Items[0].Augments != nil {
		t.Fatalf("%+v", k)
	}
	if err := k.Validate(); err != nil {
		t.Fatal(err)
	}
	xp := Kit{Name: "Levels", XP: 1000}
	xp.Normalize()
	if err := xp.Validate(); err != nil || xp.Items == nil {
		t.Fatalf("a kit of XP alone is fine and has an empty list, not null: %v %+v", err, xp)
	}
}

func TestValidateRefusesWhatCannotBeGiven(t *testing.T) {
	cases := []struct {
		name  string
		mod   func(*Kit)
		words []string
	}{
		{"unknown item", func(k *Kit) { k.Items[0].Item = "NoSuchThing" }, []string{"item 1", "not an item in the editor's catalog"}},
		{"item typed as a name", func(k *Kit) { k.Items[0].Item = "Spice" }, []string{"not an item"}},
		{"empty item", func(k *Kit) { k.Items[0].Item = "" }, []string{"not an item"}},
		{"zero quantity", func(k *Kit) { k.Items[0].Quantity = 0 }, []string{"quantity", "1 to 1000000"}},
		{"negative quantity", func(k *Kit) { k.Items[0].Quantity = -5 }, []string{"quantity"}},
		{"huge quantity", func(k *Kit) { k.Items[0].Quantity = 1_000_001 }, []string{"quantity"}},
		{"grade 6", func(k *Kit) { k.Items[1].Grade = 6 }, []string{"grade", "0 to 5"}},
		{"negative grade", func(k *Kit) { k.Items[1].Grade = -1 }, []string{"grade"}},
		{"augment grade 6", func(k *Kit) { k.Items[1].AugmentGrade = 6 }, []string{"augment grade"}},
		{"augment grade without augments", func(k *Kit) { k.Items[0].AugmentGrade = 2 }, []string{"no augments"}},
		{"augments on an item that takes none", func(k *Kit) { k.Items[0].Augments = []string{"T6_Augment_Armor10"} }, []string{"does not take augments"}},
		{"an augment that does not fit", func(k *Kit) { k.Items[1].Augments = []string{"NoSuchAugment"} }, []string{"does not fit"}},
		{"the same augment twice", func(k *Kit) { k.Items[1].Augments = []string{"T6_Augment_Armor10", "T6_Augment_Armor10"} }, []string{"twice"}},
		{"negative XP", func(k *Kit) { k.XP = -1 }, []string{"XP"}},
		{"too much XP", func(k *Kit) { k.XP = MaxXP + 1 }, []string{"XP"}},
		{"too many skill points", func(k *Kit) { k.SkillPoints = MaxSkillPoints + 1 }, []string{"skill points"}},
		{"nothing in it", func(k *Kit) { *k = Kit{Name: "Empty"} }, []string{"at least one"}},
		{"too many items", func(k *Kit) {
			k.Items = nil
			for i := 0; i <= MaxLines; i++ {
				k.Items = append(k.Items, Line{Item: fuel, Quantity: 1})
			}
		}, []string{"at most 50"}},
	}
	for _, c := range cases {
		k := starter()
		c.mod(&k)
		mustFail(t, k.Validate(), c.words...)
		_ = c.name
	}
}

func TestNamesCarryNoPathMarkupOrControlCharacter(t *testing.T) {
	for _, n := range []string{"", " lead", "trail ", "a/b", `a\b`, "..\\x", "C:", "<b>x</b>", "a\x00b", "a\nb", "a\tb", "x=1", "a;b", `"q"`, strings.Repeat("x", MaxNameRunes+1), "bad\xffutf8"} {
		if CheckName(n) == nil {
			t.Errorf("%q must be refused", n)
		}
	}
	for _, n := range []string{"Starter", "Armor set (Mk 2)", "Ärmel & Hut #1", "new-game_kit v1.2, ok!", "O'Neil", strings.Repeat("é", MaxNameRunes)} {
		if err := CheckName(n); err != nil {
			t.Errorf("%q: %v", n, err)
		}
	}
}

func TestPutCreatesReplacesRenamesAndRefusesCollisions(t *testing.T) {
	var list []Kit
	list, err := Put(list, "", starter())
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", err, list)
	}
	tools := Kit{Name: "Tools", Items: []Line{{Item: fuel, Quantity: 1}}}
	if list, err = Put(list, "", tools); err != nil || len(list) != 2 {
		t.Fatal(err)
	}
	_, err = Put(list, "", Kit{Name: "starter", XP: 1})
	mustFail(t, err, "exists already")
	_, err = Put(list, "Tools", Kit{Name: "STARTER", XP: 1})
	mustFail(t, err, "exists already")
	_, err = Put(list, "Missing", tools)
	mustFail(t, err, "no kit called")
	// replacing keeps the place; the case of the name may change
	k2 := tools
	k2.Name = "TOOLS"
	k2.XP = 7
	out, err := Put(list, "Tools", k2)
	if err != nil || len(out) != 2 || out[1].Name != "TOOLS" || out[1].XP != 7 {
		t.Fatalf("%v %+v", err, out)
	}
	if list[1].Name != "Tools" || list[1].XP != 0 {
		t.Fatal("the list that was given changed")
	}
	// a rename
	k2.Name = "Hand tools"
	if out, err = Put(out, "TOOLS", k2); err != nil || Find(out, "hand TOOLS") != 1 || Find(out, "Tools") != -1 {
		t.Fatalf("%v %+v", err, out)
	}
	// a bad kit is refused and changes nothing
	_, err = Put(out, "", Kit{Name: "Bad", Items: []Line{{Item: "Nope", Quantity: 1}}})
	mustFail(t, err, "catalog")
}

func TestAtMostTwentyKitsAndTheirSizeIsBounded(t *testing.T) {
	var list []Kit
	var err error
	for i := 0; i < MaxKits; i++ {
		if list, err = Put(list, "", Kit{Name: "Kit " + string(rune('A'+i)), XP: 1}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = Put(list, "", Kit{Name: "One more", XP: 1})
	mustFail(t, err, "at most 20 kits")
	// 20 kits of 50 lines, each line a graded item with one augment, are the most the list is meant to hold, and they fit in the settings file with room to spare
	line := Line{Item: helmet, Quantity: 1000000, Grade: 5, Augments: []string{"T6_Augment_Armor10"}, AugmentGrade: 5}
	var many []Kit
	for i := 0; i < MaxKits; i++ {
		k := Kit{Name: "Full kit " + string(rune('A'+i)), XP: MaxXP, SkillPoints: MaxSkillPoints}
		for j := 0; j < MaxLines; j++ {
			k.Items = append(k.Items, line)
		}
		many = append(many, k)
	}
	b, _ := json.MarshalIndent(many, "  ", "  ")
	t.Logf("20 full kits are %d bytes of the %d allowed", len(b), MaxStoredBytes)
	if err := CheckStored(many); err != nil {
		t.Fatalf("20 full kits must fit: %v", err)
	}
	// lines that are legal one by one but too much together are refused with the size in the words
	over := make([]Kit, 0, MaxKits)
	for _, k := range many {
		k.Items = append([]Line(nil), k.Items...)
		for j := range k.Items {
			k.Items[j].Item = strings.Repeat("A", 400)
		}
		over = append(over, k)
	}
	mustFail(t, CheckStored(over), "KiB")
}

func TestDeleteAndDuplicate(t *testing.T) {
	list, _ := Put(nil, "", starter())
	list, _ = Put(list, "", Kit{Name: "Tools", Items: []Line{{Item: fuel, Quantity: 2}}})
	out, err := Delete(list, "tools")
	if err != nil || len(out) != 1 || out[0].Name != "Starter" || len(list) != 2 {
		t.Fatalf("%v %v", err, out)
	}
	_, err = Delete(out, "Tools")
	mustFail(t, err, "no kit called")

	dup, cp, err := Duplicate(list, "Starter")
	if err != nil || len(dup) != 3 || cp.Name != "Starter (copy)" || dup[2].Name != cp.Name {
		t.Fatalf("%v %+v", err, dup)
	}
	cp.Items[1].Augments[0] = "changed"
	dup[2].Items[1].Augments[0] = "changed too"
	if list[0].Items[1].Augments[0] != "T6_Augment_Armor10" {
		t.Fatal("the copy shares its lines with the original")
	}
	dup, _ = Put(dup, "", Kit{Name: "x", XP: 1}) // keeps the list valid
	again, cp2, err := Duplicate(dup, "Starter")
	if err != nil || cp2.Name != "Starter (copy 2)" || len(again) != 5 {
		t.Fatalf("%v %v", err, cp2.Name)
	}
	long := Kit{Name: strings.Repeat("L", MaxNameRunes), XP: 1}
	ll, _ := Put(nil, "", long)
	_, cl, err := Duplicate(ll, long.Name)
	if err != nil || CheckName(cl.Name) != nil || !strings.HasSuffix(cl.Name, " (copy)") {
		t.Fatalf("a copy of a name at the limit is still a valid name: %v %q", err, cl.Name)
	}
	_, _, err = Duplicate(list, "Missing")
	mustFail(t, err, "no kit called")
}

func TestExportThenParseGivesTheSameKits(t *testing.T) {
	list, _ := Put(nil, "", starter())
	list, _ = Put(list, "", Kit{Name: "Levels", XP: 1000})
	b, err := Export(list, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"format": "tabr-tau-kits"`) || !strings.Contains(string(b), `"version": 1`) {
		t.Fatalf("%s", b)
	}
	got, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(list)
	c, _ := json.Marshal(got)
	if string(a) != string(c) {
		t.Fatalf("\n%s\n%s", a, c)
	}
	one, err := Export(list, []string{"levels"})
	if err != nil {
		t.Fatal(err)
	}
	if k, err := Parse(one); err != nil || len(k) != 1 || k[0].Name != "Levels" {
		t.Fatalf("%v %v", err, k)
	}
	_, err = Export(list, []string{"Nope"})
	mustFail(t, err, "no kit called")
	_, err = Export(nil, nil)
	mustFail(t, err, "no kit to export")
}

func TestParseRefusesAFileThatIsNotAGoodKitFile(t *testing.T) {
	good := `{"format":"tabr-tau-kits","version":1,"kits":[{"name":"A","items":[{"item":"SpicedFuelCell","quantity":5}]}]}`
	if _, err := Parse([]byte(good)); err != nil {
		t.Fatal(err)
	}
	line := func(s string) string {
		return `{"format":"tabr-tau-kits","version":1,"kits":[{"name":"A","items":[` + s + `]}]}`
	}
	cases := []struct {
		name, text string
		words      []string
	}{
		{"empty", ``, []string{"not a kit file"}},
		{"not JSON", `hello`, []string{"not a kit file"}},
		{"an array", `[]`, []string{"not a kit file"}},
		{"trailing text", good + ` {}`, []string{"more after"}},
		{"two objects", good + good, []string{"more after"}},
		{"another format", `{"format":"something","version":1,"kits":[]}`, []string{"format"}},
		{"no format", `{"kits":[]}`, []string{"format"}},
		{"another version", `{"format":"tabr-tau-kits","version":2,"kits":[{"name":"A","xp":1}]}`, []string{"version 2"}},
		{"no kits", `{"format":"tabr-tau-kits","version":1,"kits":[]}`, []string{"no kits"}},
		{"null kits", `{"format":"tabr-tau-kits","version":1,"kits":null}`, []string{"no kits"}},
		{"unknown top-level key", `{"format":"tabr-tau-kits","version":1,"kits":[],"path":"C:\\x"}`, []string{"unknown field"}},
		{"a path in a kit", `{"format":"tabr-tau-kits","version":1,"kits":[{"name":"A","xp":1,"path":"C:\\Windows"}]}`, []string{"unknown field"}},
		{"a command in a line", line(`{"item":"SpicedFuelCell","quantity":1,"run":"calc.exe"}`), []string{"unknown field"}},
		{"unknown item", line(`{"item":"NoSuchThing","quantity":1}`), []string{"catalog"}},
		{"an item that is a path", line(`{"item":"..\\..\\x","quantity":1}`), []string{"catalog"}},
		{"quantity 0", line(`{"item":"SpicedFuelCell","quantity":0}`), []string{"quantity"}},
		{"quantity fraction", line(`{"item":"SpicedFuelCell","quantity":1.5}`), []string{"wrong kind"}},
		{"quantity text", line(`{"item":"SpicedFuelCell","quantity":"5"}`), []string{"wrong kind"}},
		{"quantity huge", line(`{"item":"SpicedFuelCell","quantity":99999999999999999999}`), []string{"not a kit file"}},
		{"item a number", line(`{"item":5,"quantity":1}`), []string{"wrong kind"}},
		{"bad name", `{"format":"tabr-tau-kits","version":1,"kits":[{"name":"../x","xp":1}]}`, []string{"name"}},
		{"two kits of one name", `{"format":"tabr-tau-kits","version":1,"kits":[{"name":"A","xp":1},{"name":"a","xp":2}]}`, []string{"two kits are named"}},
		{"21 kits", `{"format":"tabr-tau-kits","version":1,"kits":[` + strings.Repeat(`{"name":"A","xp":1},`, 20) + `{"name":"A","xp":1}]}`, []string{"21 kits"}},
	}
	for _, c := range cases {
		_, err := Parse([]byte(c.text))
		if err == nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		for _, w := range c.words {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: %q does not mention %q", c.name, err, w)
			}
		}
	}
	// one bad kit refuses the whole file, and the message names which
	_, err := Parse([]byte(`{"format":"tabr-tau-kits","version":1,"kits":[{"name":"A","xp":1},{"name":"B","items":[{"item":"Nope","quantity":1}]}]}`))
	mustFail(t, err, "kit 2 (B)", "item 1")
}

func TestParseRefusesAnOversizedFileBeforeReadingIt(t *testing.T) {
	pad := strings.Repeat(" ", MaxFileBytes)
	_, err := Parse([]byte(`{"format":"tabr-tau-kits","version":1,"kits":[{"name":"A","xp":1}]}` + pad))
	mustFail(t, err, "larger than 64 KiB")
	_, err = Parse([]byte(`{"format":"tabr-tau-kits","version":1,"kits":[{"name":"` + strings.Repeat("a", MaxFileBytes) + `","xp":1}]}`))
	mustFail(t, err, "larger than 64 KiB")
}

func TestMergeNeverReplacesAKitAndRenamesACollision(t *testing.T) {
	list, _ := Put(nil, "", starter())
	imp := []Kit{starter(), {Name: "Fresh", XP: 9}}
	out, renamed, err := Merge(list, imp)
	if err != nil || renamed != 1 || len(out) != 3 || out[1].Name != "Starter (imported)" || out[2].Name != "Fresh" {
		t.Fatalf("%v %d %+v", err, renamed, out)
	}
	out, renamed, err = Merge(out, imp)
	if err != nil || renamed != 2 || out[3].Name != "Starter (imported 2)" || out[4].Name != "Fresh (imported)" {
		t.Fatalf("%v %d %+v", err, renamed, out)
	}
	if len(list) != 1 {
		t.Fatal("the list that was given changed")
	}
	var full []Kit
	for i := 0; i < MaxKits; i++ {
		full, _ = Put(full, "", Kit{Name: "K" + string(rune('A'+i)), XP: 1})
	}
	_, _, err = Merge(full, []Kit{{Name: "Extra", XP: 1}})
	mustFail(t, err, "at most 20 kits")
}

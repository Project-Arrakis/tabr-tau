package basefile

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// The console's rule for ids (#538): after the export no "!!kind#N" string is left anywhere, and the strict check refuses a file that has one.

var localForm = regexp.MustCompile(`!![a-z]{3}#`)

// localForms lists every string value (and key) of every entry that holds a local-form id mark, by a walk over the nested JSON, not a search of the text.
func localForms(t *testing.T, data []byte) []string {
	t.Helper()
	var f struct {
		Entries []struct {
			ID   int64 `json:"id"`
			Data any   `json:"data"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	var found []string
	var walk func(v any, where string)
	walk = func(v any, where string) {
		switch x := v.(type) {
		case string:
			if localForm.MatchString(x) {
				found = append(found, where+" = "+x)
			}
		case []any:
			for _, c := range x {
				walk(c, where+".*")
			}
		case map[string]any:
			for k, c := range x {
				if localForm.MatchString(k) {
					found = append(found, where+" key "+k)
				}
				walk(c, where+"."+k)
			}
		}
	}
	for _, e := range f.Entries {
		walk(e.Data, "entry")
	}
	return found
}

// worldWithLocalIds is the fixture's rows with the three kinds of leftover that the console refused (#538): an actor with a weapon component whose favourite weapon is none, a crafting
// queue whose job was queued by a character who is not in the base, and a reference to an item that is in the file.
func worldWithLocalIds(t *testing.T) map[string][][]any {
	t.Helper()
	m := rowsOf(loadFixture(t, nil))
	acts, _ := Find("actors")
	ents, _ := Find("fgl_entities")
	col := func(tb Table, name string) int {
		for i, c := range tb.Cols {
			if c.Name == name {
				return i
			}
		}
		t.Fatal(name)
		return -1
	}
	m["actors"][1][col(acts, "properties")] = `{"WeaponActorComponent":{"m_FavoriteWeaponItemDatabaseId":"!!itm#0","m_Other":["!!itm#0",{"x":"!!act#0"}]}}`
	m["fgl_entities"][0][col(ents, "components")] = `{"FItemCraftingComponent":[[],[{"RequestsQueue":[{"InstigatorActorId":"!!act#424242","Other":"!!act#0"},{"InstigatorActorId":"!!act#2"}]}]]}`
	return m
}

func TestConsoleFileLeavesNoLocalFormIdAnywhere(t *testing.T) {
	m := worldWithLocalIds(t)
	res, err := WriteConsole(headerOf(), m, MaxFileBytes, MaxRows)
	if err != nil {
		t.Fatal(err)
	}
	if left := localForms(t, res.Data); len(left) > 0 {
		t.Fatalf("local-form ids left in the file: %v", left)
	}
	s := string(res.Data)
	for _, w := range []string{`"m_FavoriteWeaponItemDatabaseId":"!!itm@0"`, `"InstigatorActorId":"!!act@0"`} {
		if !strings.Contains(s, w) {
			t.Errorf("the file does not say %s", w)
		}
	}
	// the instigator is no placeholder and no actor of the file: the placeholder would become the importing player's controller, not the character that queued the job
	if strings.Contains(s, `"InstigatorActorId":"!!act@1"`) {
		t.Error("the instigator is the owner placeholder")
	}
	// both the editor's reader and the strict check take the file
	if _, err := Load(res.Data, MaxFileBytes, MaxRows); err != nil {
		t.Fatalf("the reader: %v", err)
	}
	st, err := CheckConsoleStrict(res.Data)
	if err != nil {
		t.Fatalf("the strict check: %v", err)
	}
	if st.Entries != res.Entries || st.NoneRefs < 5 || st.Refs < st.NoneRefs {
		t.Errorf("strict counts %+v for %d entries", st, res.Entries)
	}
	// the fixture without the additions passes too, and so does a written-again file
	plain, err := WriteConsole(headerOf(), rowsOf(loadFixture(t, nil)), MaxFileBytes, MaxRows)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CheckConsoleStrict(plain.Data); err != nil {
		t.Errorf("the fixture file: %v", err)
	}
	if left := localForms(t, plain.Data); len(left) > 0 {
		t.Errorf("local-form ids left in the fixture file: %v", left)
	}
}

func TestStrictCheckRefusesWhatTheConsoleRefuses(t *testing.T) {
	res, err := WriteConsole(headerOf(), worldWithLocalIds(t), MaxFileBytes, MaxRows)
	if err != nil {
		t.Fatal(err)
	}
	good := string(res.Data)
	if !strings.Contains(good, `"m_FavoriteWeaponItemDatabaseId":"!!itm@0"`) || !strings.Contains(good, `"m_SelfUniqueID":"!!act@2"`) {
		t.Fatalf("the file the cases are made from: %s", good)
	}
	for name, c := range map[string]struct{ from, to, want string }{
		"the weapon's none id as a local id":    {`"m_FavoriteWeaponItemDatabaseId":"!!itm@0"`, `"m_FavoriteWeaponItemDatabaseId":"!!itm#0"`, "non-transfer id !!itm#0"},
		"an instigator as a local id":           {`"InstigatorActorId":"!!act@0"`, `"InstigatorActorId":"!!act#424242"`, "non-transfer id !!act#424242"},
		"a transfer id that is not in the file": {`"m_SelfUniqueID":"!!act@2"`, `"m_SelfUniqueID":"!!act@999999"`, "names no act entry"},
		"a transfer id of another kind":         {`"m_SelfUniqueID":"!!act@2"`, `"m_SelfUniqueID":"!!itm@2"`, "names no itm entry"},
		"an id inside other text":               {`"m_SelfUniqueID":"!!act@2"`, `"m_SelfUniqueID":"see !!act@2 here"`, "inside other text"},
		"an id that is not a number":            {`"m_SelfUniqueID":"!!act@2"`, `"m_SelfUniqueID":"!!act@two"`, "non-transfer id"},
		"an id as a key":                        {`"m_SelfUniqueID":"!!act@2"`, `"!!act#5":"x"`, "non-transfer id !!act#5"},
		"a number that does not fit":            {`"m_SelfUniqueID":"!!act@2"`, `"m_SelfUniqueID":"!!act@99999999999999999999"`, "does not fit"},
	} {
		t.Run(name, func(t *testing.T) {
			bad := strings.Replace(good, c.from, c.to, 1)
			if bad == good {
				t.Fatal("the case changes nothing")
			}
			_, err := CheckConsoleStrict([]byte(bad))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want %q", err, c.want)
			}
			if !strings.Contains(err.Error(), "entry ") {
				t.Errorf("the message does not say which entry: %v", err)
			}
		})
	}
	// the null reference and the owner placeholder are accepted
	for _, ok := range []string{`"!!act@0"`, `"!!itm@0"`, `"!!act@1"`} {
		f := strings.Replace(good, `"m_SelfUniqueID":"!!act@2"`, `"m_SelfUniqueID":`+ok, 1)
		if _, err := CheckConsoleStrict([]byte(f)); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	// the owner placeholder only stands for an actor
	if _, err := CheckConsoleStrict([]byte(strings.Replace(good, `"m_SelfUniqueID":"!!act@2"`, `"m_SelfUniqueID":"!!itm@1"`, 1))); err == nil {
		t.Error("the placeholder was taken for an item")
	}
	// a long list of problems is cut
	many := strings.ReplaceAll(strings.ReplaceAll(good, `"!!act@0"`, `"!!act#7"`), `"!!itm@0"`, `"!!itm#7"`)
	if _, err := CheckConsoleStrict([]byte(many)); err == nil || !strings.Contains(err.Error(), "more") {
		t.Errorf("many problems: %v", err)
	}
	// not a file at all
	for _, bad := range []string{``, `[]`, `{"ownerPlaceholderTransferId":1,"entries":[]} 1`, `{"entries":[]}`} {
		if _, err := CheckConsoleStrict([]byte(bad)); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

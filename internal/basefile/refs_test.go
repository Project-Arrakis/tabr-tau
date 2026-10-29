package basefile

import (
	"strings"
	"testing"
)

func TestRewriteRefsChangesOnlyTheReferences(t *testing.T) {
	in := `{"z":1.50,"a":["!!act#12","!!itm#7",{"k":"x\"y","!!inv#3":"!!fgl#9007199254740993"}],"big":18446744073709551615,"n":"!!act#0","s":"!!act#5","t":"plain !text"}`
	got, err := RewriteRefs(in, func(r Ref) (string, error) {
		if r.ID == 0 {
			return "!!" + r.Kind + "#0", nil
		}
		return "!!" + r.Kind + "#1" + strings.Repeat("0", int(r.ID%3)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// key order, the spelling of numbers (1.50, a number above 2^64) and the escaped string are untouched; the escaped reference is a reference too
	want := `{"z":1.50,"a":["!!act#1","!!itm#10",{"k":"x\"y","!!inv#1":"!!fgl#1"}],"big":18446744073709551615,"n":"!!act#0","s":"!!act#100","t":"plain !text"}`
	if got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	found, err := FindRefs(got)
	if err != nil || strings.Join(found, ",") != "!!act#1,!!itm#10,!!inv#1,!!fgl#1,!!act#0,!!act#100" {
		t.Fatalf("found %v %v", found, err)
	}
	if same, _ := RewriteRefs(`{"a":1}`, nil); same != `{"a":1}` {
		t.Fatalf("no reference: %s", same)
	}
}

func TestRewriteRefsRefusesWhatItCannotRewriteSafely(t *testing.T) {
	id := func(r Ref) (string, error) { return "!!act#1", nil }
	for in, want := range map[string]string{
		`{"a":"see !!act#5"}`:                               "inside other text",
		`{"a":"!!act#"}`:                                    "inside other text",
		`{"a":"!!act#99999999999999999999"}`:                "does not fit",
		`{"a":"` + strings.Repeat("[", 5) + ``:              "not closed",
		strings.Repeat("[", 101) + strings.Repeat("]", 101): "nested more than",
	} {
		if _, err := RewriteRefs(in, id); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%.40s: %v, want %q", in, err, want)
		}
	}
	// a plain "!!" without the shape of a reference is only text
	if _, err := RewriteRefs(`{"a":"wow!! really"}`, id); err != nil {
		t.Errorf("plain text refused: %v", err)
	}
}

func TestInsertSQLWritesBlobJSONThroughJsonb(t *testing.T) {
	a, _ := Find("actors")
	if got := a.InsertSQL(); !strings.Contains(got, "jsonb(?),jsonb(?)") || !strings.HasPrefix(got, "insert into actors(id,class,map,") || strings.Count(got, "?") != len(a.Cols) {
		t.Fatalf("%s", got)
	}
	it, _ := Find("items")
	if strings.Contains(it.InsertSQL(), "jsonb") {
		t.Fatalf("items.stats is text: %s", it.InsertSQL())
	}
}

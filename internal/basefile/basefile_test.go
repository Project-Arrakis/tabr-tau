package basefile

import (
	"math"
	"strings"
	"testing"
)

func TestValueWritesEachKindExactlyOrRefuses(t *testing.T) {
	cases := []struct {
		c    Column
		v    any
		want string // "" means refused
	}{
		{Column{"id", Int}, int64(5764607523034234881), "5764607523034234881"}, // above 2^53, exact
		{Column{"id", Int}, int64(-7), "-7"},
		{Column{"id", Int}, nil, "null"},
		{Column{"id", Int}, 1.5, ""},
		{Column{"id", Int}, "7", ""},
		{Column{"x", Real}, 1.5, "1.5"},
		{Column{"x", Real}, float64(3), "3"},
		{Column{"x", Real}, nil, "null"},
		{Column{"x", Real}, math.Inf(1), ""},
		{Column{"x", Real}, math.NaN(), ""},
		{Column{"x", Real}, markNonFinite, ""},
		{Column{"s", Text}, "a<b>&\"c\"\n", `"a<b>&\"c\"\n"`}, // markup is kept as text, not escaped to <
		{Column{"s", Text}, "bad\xffutf8", ""},
		{Column{"s", Text}, int64(1), ""},
		{Column{"j", JSON}, ` { "a" : [ 1, 2 ] } `, `{"a":[1,2]}`},
		{Column{"j", JSON}, markInvalidJSON, ""},
		{Column{"j", JSON}, `{"a":`, ""},
		{Column{"j", JSON}, nil, "null"},
	}
	for _, c := range cases {
		got, err := c.c.Value(c.v)
		if c.want == "" {
			if err == nil {
				t.Errorf("%v %v: accepted as %s", c.c, c.v, got)
			}
			continue
		}
		if err != nil || string(got) != c.want {
			t.Errorf("%v %v: %s %v, want %s", c.c, c.v, got, err, c.want)
		}
	}
}

func TestEveryTableHasItsScopeOrderAndColumns(t *testing.T) {
	seen := map[string]bool{}
	for _, tb := range Tables {
		if seen[tb.Name] {
			t.Errorf("table %s twice", tb.Name)
		}
		seen[tb.Name] = true
		if len(tb.Cols) == 0 || tb.Scope == "" || tb.Order == "" {
			t.Errorf("%s is not complete", tb.Name)
		}
		names := map[string]bool{}
		for _, c := range tb.Cols {
			if names[c.Name] {
				t.Errorf("%s: column %s twice", tb.Name, c.Name)
			}
			names[c.Name] = true
		}
		sql := tb.ScopeSQL(12)
		if strings.Contains(sql, "{") || strings.Contains(sql, "}") || !strings.Contains(sql, "12") {
			t.Errorf("%s scope: %s", tb.Name, sql)
		}
	}
	for _, banned := range []string{"owner_account_id", "item_id", "vehicle_module_id"} {
		for _, tb := range Tables {
			for _, c := range tb.Cols {
				if c.Name == banned && !(tb.Name == "items" && banned == "item_id") {
					t.Errorf("%s.%s must not travel", tb.Name, c.Name)
				}
			}
		}
	}
}

func TestChecksumIgnoresLayoutButNotContent(t *testing.T) {
	a, _ := TablesChecksum([]byte(`{"a": [1, 2], "b": "x y"}`))
	b, _ := TablesChecksum([]byte("{\n \"a\":[1,2],\n\"b\":\"x y\"}"))
	c, _ := TablesChecksum([]byte(`{"a":[1,2],"b":"xy"}`))
	if a != b || a == c || len(a) != 64 {
		t.Fatalf("%s %s %s", a, b, c)
	}
	if _, err := TablesChecksum([]byte(`{"a":`)); err == nil {
		t.Fatal("text that is not JSON has no checksum")
	}
}

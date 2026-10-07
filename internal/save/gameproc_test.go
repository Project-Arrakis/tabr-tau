package save

import (
	"reflect"
	"testing"
)

func TestParseTasklist(t *testing.T) {
	cases := []struct {
		name, out string
		want      []string
	}{
		{"csv cut to 25", `"DuneSandbox-Win64-Shippin","4242","Console","1","1,234,567 K"`, []string{"DuneSandbox-Win64-Shippin"}},
		{"full name", `"DuneSandbox-Win64-Shipping.exe","4242","Console","1","9 K"`, []string{"DuneSandbox-Win64-Shipping.exe"}},
		{"a different dune process", `"DuneAwakening.exe","1","Console","1","9 K"`, []string{"DuneAwakening.exe"}},
		{"mixed case", `"dunesandbox.exe","1"`, []string{"dunesandbox.exe"}},
		{"two processes, one repeated", "\"DuneA.exe\",\"1\"\r\n\"DuneB.exe\",\"2\"\r\n\"DuneA.exe\",\"3\"\r\n", []string{"DuneA.exe", "DuneB.exe"}},
		{"no match notice", "INFO: No tasks are running which match the specified criteria.", nil},
		{"other process", `"notepad.exe","1"`, nil},
		{"our own program is not the game", `"tabr-tau-window.exe","1"`, nil},
		{"empty", "", nil},
	}
	for _, c := range cases {
		if got := parseTasklist(c.out); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

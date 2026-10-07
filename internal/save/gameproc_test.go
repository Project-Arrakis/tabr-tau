package save

import "testing"

func TestListedInTasklist(t *testing.T) {
	cases := []struct {
		name, out string
		want      bool
	}{
		{"csv cut to 25", `"DuneSandbox-Win64-Shippin","4242","Console","1","1,234,567 K"`, true},
		{"table cut to 25", "DuneSandbox-Win64-Shippin     4242 Console   1  1,234,567 K", true},
		{"full name", `"DuneSandbox-Win64-Shipping.exe","4242"`, true},
		{"mixed case", `"dunesandbox-win64-shipping.exe"`, true},
		{"no match notice", "INFO: No tasks are running which match the specified criteria.", false},
		{"other process", `"notepad.exe","1"`, false},
		{"empty", "", false},
	}
	for _, c := range cases {
		if got := listedInTasklist(c.out); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

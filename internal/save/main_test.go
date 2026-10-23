package save

import (
	"os"
	"testing"
)

// defaultLocationFor is the location check as shipped (captured before TestMain swaps it), so a test can show it is what runs.
var defaultLocationFor = locationFor

// Test saves live in temp folders, which the shipped location check does not trust (it asks for a confirmation); the tests of the
// guard's other rules should not need one. Tests of the location rule put the real check back.
func TestMain(m *testing.M) {
	locationFor = func(string) string { return "" }
	// The tests make decoded copies and sweep the temp folder for stale ones: they do it in a folder of their own, never in the developer's.
	tmp, err := os.MkdirTemp("", "tabr-tau-save-tests-")
	if err == nil {
		for _, k := range []string{"TMP", "TEMP", "TMPDIR"} {
			os.Setenv(k, tmp)
		}
	}
	code := m.Run()
	if err == nil {
		os.RemoveAll(tmp)
	}
	os.Exit(code)
}

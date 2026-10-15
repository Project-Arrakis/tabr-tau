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
	os.Exit(m.Run())
}

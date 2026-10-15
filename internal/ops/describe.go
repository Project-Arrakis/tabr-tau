package ops

import (
	"fmt"
	"strconv"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
)

// The review pane lists the edits as the person made them, so a description says what changed in the player's words: an item by its in-game
// name with its id in brackets, and a value as "before -> after". "edit item 4521" and "currency 0 +100" told a reader neither what nor how
// much (UX-04 of the 2026-10-15 audit).

// itemLabel is an item template as the person knows it: the in-game name when the catalog has one, else the template id.
func itemLabel(template string) string {
	if name, ok := catalog.Name(template); ok && name != "" {
		return name
	}
	return template
}

// commas writes n with thousands separators: 1234567 is "1,234,567".
func commas(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := ""
	if n < 0 {
		neg, s = "-", s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return neg + s
}

// change is "before → after".
func change(before, after int64) string {
	return fmt.Sprintf("%s → %s", commas(before), commas(after))
}

// currencyName is the wallet the console and the page call currency 0 or 1.
func currencyName(id int64) string {
	if id == 0 {
		return "Solari Credit"
	}
	return "House Credit"
}

package ops

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/augments"
	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
)

// The banner lists the edits that are waiting to be written, as the person made them, so a description says what changed in the player's words: an item by its in-game
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

// currencyName is the wallet that currency 0 or 1 is, in the one set of money names the page uses: Solari in wallet and House credit in
// wallet are the two virtual currencies, Solari in backpack is the item stack.
func currencyName(id int64) string {
	if id == 0 {
		return "Solari in wallet"
	}
	return "House credit in wallet"
}

// giveLineWords is one line of a Give queue in words: "3 x Ambition (grade 5, augments: Plate, Weave at grade 3)". The item is named, then its quantity, its grade (the one the item is
// written at, as a single Give says it) and, when it carries augments, their names and the one grade they are added at (#1149). given is how many of the quantity went in: fewer than the
// quantity reads "340 of 500 x ...", as a single Give does (giveWords).
func giveLineWords(l giveLine, given int64) string {
	detail := fmt.Sprintf("grade %d", grantGrade(l.tmpl, l.q))
	if len(l.aug) > 0 {
		names := make([]string, len(l.aug))
		for i, id := range l.aug {
			if names[i] = augments.Name(id); names[i] == "" {
				names[i] = id
			}
		}
		detail += fmt.Sprintf(", augments: %s at grade %d", strings.Join(names, ", "), l.grade)
	}
	count := commas(l.qty)
	if given < l.qty {
		count = commas(given) + " of " + count
	}
	return fmt.Sprintf("%s x %s (%s)", count, itemLabel(l.tmpl), detail)
}

// giveQueueWords is how a Give queue is listed as a waiting edit (#1149): one item reads as a single Give does ("Give 1 x Ambition (grade 5, augments: ...)"), several as "Give 2 items: "
// and the lines, separated by "; ". given is the units that went in for each line; nil means every line went in whole.
func giveQueueWords(lines []giveLine, given []int64) string {
	parts := make([]string, len(lines))
	for i, l := range lines {
		g := l.qty
		if i < len(given) {
			g = given[i]
		}
		parts[i] = giveLineWords(l, g)
	}
	if len(lines) == 1 {
		return "Give " + parts[0]
	}
	return fmt.Sprintf("Give %d items: %s", len(lines), strings.Join(parts, "; "))
}

package ops

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/policy"
)

// The health the repair functions may copy onto a piece (#613, audit G-009). The repair target of a type is the best health seen on a STANDING piece of the player's own bases (see
// bestHealthOf in world.go), and never above the cap of the type. The caps, how they were measured and the tables are in internal/policy (health.go), which the file check of an import
// shares: a file is refused for a health above the same cap. The caps only limit what a repair writes; no piece is ever lowered.

// capValues is the text of a VALUES list of (type, cap) pairs for the types of a table, to be joined to the best health of each type. The type names are the program's own constants;
// a quote in one is doubled all the same. The list is sorted so that the statement text does not change from run to run.
func capValues(seen map[string]float64) string {
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString("('" + strings.ReplaceAll(n, "'", "''") + "'," + strconv.FormatFloat(policy.HealthCapOf(seen[n]), 'f', -1, 64) + ")")
	}
	return b.String()
}

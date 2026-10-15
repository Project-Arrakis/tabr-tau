package ops

import (
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// inventoryTemplates is every item template in the character's own inventories.
func (o *Ops) inventoryTemplates(pawn int64) (map[string]bool, error) {
	rows, err := o.S.Query(inventoryTemplatesSQL, pawn)
	if err != nil {
		return nil, err
	}
	return templateSet(rows), nil
}

const inventoryTemplatesSQL = `select distinct i.template_id t from inventories v join items i on i.inventory_id=v.id where v.actor_id=? and i.template_id is not null`

func templateSet(rows []save.Row) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		if t, ok := r["t"].(string); ok {
			out[t] = true
		}
	}
	return out
}

func sortRows(rows []CatalogRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := strings.ToLower(rows[i].Name), strings.ToLower(rows[j].Name)
		if a != b {
			return a < b
		}
		return rows[i].ID < rows[j].ID
	})
}

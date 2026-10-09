package ops

import (
	"sort"
	"strings"
)

// inventoryTemplates is every item template in the character's own inventories.
func (o *Ops) inventoryTemplates(pawn int64) (map[string]bool, error) {
	rows, err := o.S.Query(`select distinct i.template_id t from inventories v join items i on i.inventory_id=v.id where v.actor_id=? and i.template_id is not null`, pawn)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, r := range rows {
		if t, ok := r["t"].(string); ok {
			out[t] = true
		}
	}
	return out, nil
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

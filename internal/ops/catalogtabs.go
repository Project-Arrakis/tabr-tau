package ops

import (
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
)

// CatalogRow is one entry of the Building Sets or Customizations tab: an item the character can be given, with what the save says
// about whether they already have it.
type CatalogRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Learned     bool   `json:"learned"`     // a building set the character has learned
	InInventory bool   `json:"inInventory"` // the item is in one of the character's inventories
	InCatalog   bool   `json:"inCatalog"`   // the item catalog knows it (the give action needs that)
}

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

// BuildingSets lists the building items of the catalog (patents and sets) with whether each is a learned set or already in the
// inventory, as the console's Building Sets tab does, plus the learned sets the catalog does not list (store packs), and the
// character's new buildable pieces.
func (o *Ops) BuildingSets() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	learnedRows, _ := o.S.Query(`select learned_building_set s from building_progression_learned_building_sets where character_id=?`, p.Controller)
	learned := map[string]bool{}
	for _, r := range learnedRows {
		learned[r["s"].(string)] = true
	}
	inv, err := o.inventoryTemplates(p.Pawn)
	if err != nil {
		return nil, err
	}
	out := []CatalogRow{}
	seen := map[string]bool{}
	for _, e := range catalog.All() {
		if e.Category != "buildings" || seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		out = append(out, CatalogRow{ID: e.ID, Name: e.Name, Learned: learned[e.ID], InInventory: inv[e.ID], InCatalog: true})
	}
	for id := range learned {
		if !seen[id] {
			out = append(out, CatalogRow{ID: id, Name: tidyWords(strings.TrimSuffix(id, "_Patent")), Learned: true, InInventory: inv[id]})
		}
	}
	sortRows(out)
	pieces, _ := o.S.Query(`select new_buildable_piece name from building_progression_new_buildable_pieces where character_id=? order by 1`, p.Controller)
	return map[string]any{"rows": out, "newPieces": pieces}, nil
}

// Customizations lists the customization items of the catalog with whether each is already in the inventory. The save keeps no list
// of owned customizations that tabr-tau can read, so (as in the console) only the inventory is checked.
func (o *Ops) Customizations() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	inv, err := o.inventoryTemplates(p.Pawn)
	if err != nil {
		return nil, err
	}
	out := []CatalogRow{}
	seen := map[string]bool{}
	for _, e := range catalog.All() {
		if e.Category != "customizations" || seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		out = append(out, CatalogRow{ID: e.ID, Name: e.Name, InInventory: inv[e.ID], InCatalog: true})
	}
	sortRows(out)
	return map[string]any{"rows": out}, nil
}

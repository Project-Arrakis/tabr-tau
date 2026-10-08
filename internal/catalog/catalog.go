// Package catalog maps game template ids to the names the game shows ("Hajra Literjon Mk6" for HighCapacityLiterjon_06).
// It is data only: it never imports the save, so it can be used by any layer.
//
// The names come from the item catalog of the Dune Awakening Docker console (runtime/data/admin-items.json, MIT
// licence, copyright RedBlink; imported at commit db5d7f40). Only the id, the name and the category are kept, not the
// stack sizes or volumes. The licence text is reproduced in internal/notices/THIRD-PARTY-NOTICES.md. Seven ids appear
// twice in the source (the same item listed under two categories); the first entry wins.
package catalog

import (
	_ "embed"
	"encoding/json"
	"sort"
)

//go:embed items.json
var raw []byte

// Entry is one item: its template id, in-game name and category.
type Entry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

var (
	entries []Entry
	byID    map[string]Entry
)

func init() {
	if err := json.Unmarshal(raw, &entries); err != nil {
		panic("catalog: items.json is not valid: " + err.Error())
	}
	byID = make(map[string]Entry, len(entries))
	for _, e := range entries {
		if _, dup := byID[e.ID]; !dup { // the first entry for an id wins
			byID[e.ID] = e
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
}

// All returns every known item, sorted by name. The slice must not be changed.
func All() []Entry { return entries }

// Name returns the in-game name of a template id, and whether the id is known.
func Name(id string) (string, bool) {
	e, ok := byID[id]
	return e.Name, ok
}

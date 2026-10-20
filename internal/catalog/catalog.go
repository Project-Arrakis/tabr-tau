// Package catalog maps game template ids to the names the game shows ("Hajra Literjon Mk6" for HighCapacityLiterjon_06).
// It is data only: it never imports the save, so it can be used by any layer.
//
// The names come from the item catalog of the Dune Awakening Docker console (runtime/data/admin-items.json, MIT
// licence, copyright RedBlink; synced 2026-10-09 with blob 1bde840c). Only the id, the name, the category, the source and the required DLC are kept, not the
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

// customization_unlocks.json maps each cosmetic item (a token the character uses to unlock cosmetics) to the customization ids it unlocks, as the game's own
// item table lists them (DT_ItemTableCustomizations, CustomizationItemStats.UnlockableCustomizations). It holds ids only. A character's unlocked ids are
// kept in its CustomizationLibraryActorComponent, which is how a cosmetic can be unlocked without putting its item in an inventory.
//
//go:embed customization_unlocks.json
var rawUnlocks []byte

// Entry is one item: its template id, in-game name and category, where the console files it (Source, for example "BuildingSets" or
// "Customizations") and the DLC it needs, if any.
type Entry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Source      string `json:"source,omitempty"`
	RequiredDLC string `json:"requiredDlc,omitempty"`
}

var (
	entries []Entry
	byID    map[string]Entry
	unlocks map[string][]string
)

func init() {
	if err := json.Unmarshal(raw, &entries); err != nil {
		panic("catalog: items.json is not valid: " + err.Error())
	}
	if err := json.Unmarshal(rawUnlocks, &unlocks); err != nil {
		panic("catalog: customization_unlocks.json is not valid: " + err.Error())
	}
	byID = make(map[string]Entry, len(entries))
	for _, e := range entries {
		if _, dup := byID[e.ID]; !dup { // the first entry for an id wins
			byID[e.ID] = e
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
}

// Unlocks returns the customization ids a cosmetic item unlocks when it is used, or nil when the game's table has no entry for it. The slice must not be changed.
func Unlocks(id string) []string { return unlocks[id] }

// All returns every known item, sorted by name. The slice must not be changed.
func All() []Entry { return entries }

// Name returns the in-game name of a template id, and whether the id is known.
func Name(id string) (string, bool) {
	e, ok := byID[id]
	return e.Name, ok
}

// Category returns the category of a template id ("clothing", "weapons", ...), or "" when the id is not in the catalog.
func Category(id string) string { return byID[id].Category }

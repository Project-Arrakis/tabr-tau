// Package catalog maps game template ids to the names the game shows ("Hajra Literjon Mk6" for HighCapacityLiterjon_06).
// It is data only: it never imports the save, so it can be used by any layer.
//
// The names come from the item catalog of the Dune Awakening Docker console (runtime/data/admin-items.json, MIT
// licence, copyright RedBlink; synced 2026-10-09 with blob 1bde840c). Only the id, the name, the category, the source and the required DLC are kept, not the
// stack sizes or volumes (those come from the game's own tables: stacks.json and volumes.json). The licence text is reproduced in internal/notices/THIRD-PARTY-NOTICES.md. Seven ids appear
// twice in the source (the same item listed under two categories); the first entry wins.
//
// stacks.json holds the most of an item one inventory slot can hold, as the game's own item tables list it (the base item rows, StackAndDurability.MaxStackSize). Only
// the items that stack are listed; every other item the catalog knows holds one.
//
// volumes.json holds the volume of one unit of an item, as the game's own item tables list it (the base item rows, StaticData.ItemSize; no row has an
// ItemSizeOverride). 0 means the game lists the item with no volume. An item that is not listed is unknown, not zero: the 17 ids that two tables list with different
// sizes are left out rather than guessed.
//
// Where each file in this folder comes from, and what is not recorded about it, is in PROVENANCE.md (the notice is in internal/notices).
package catalog

import (
	_ "embed"
	"encoding/json"
	"sort"
)

//go:embed items.json
var raw []byte

//go:embed stacks.json
var rawStacks []byte

//go:embed volumes.json
var rawVolumes []byte

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
	stacks  map[string]int64
	volumes map[string]float64
)

func init() {
	if err := json.Unmarshal(raw, &entries); err != nil {
		panic("catalog: items.json is not valid: " + err.Error())
	}
	if err := json.Unmarshal(rawUnlocks, &unlocks); err != nil {
		panic("catalog: customization_unlocks.json is not valid: " + err.Error())
	}
	var st struct {
		Max map[string]int64 `json:"max"`
	}
	if err := json.Unmarshal(rawStacks, &st); err != nil {
		panic("catalog: stacks.json is not valid: " + err.Error())
	}
	stacks = st.Max
	var vo struct {
		Size map[string]float64 `json:"size"`
	}
	if err := json.Unmarshal(rawVolumes, &vo); err != nil {
		panic("catalog: volumes.json is not valid: " + err.Error())
	}
	volumes = vo.Size
	byID = make(map[string]Entry, len(entries))
	for _, e := range entries {
		if _, dup := byID[e.ID]; !dup { // the first entry for an id wins
			byID[e.ID] = e
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
}

// MaxStack returns the most of an item one slot holds, and whether it is known: an item the game's tables list with a stack size has it, any other item in the catalog
// holds one (gear, schematics, vehicles), and an id the catalog does not know has no answer.
func MaxStack(id string) (int64, bool) {
	if n, ok := stacks[id]; ok {
		return n, true
	}
	if _, ok := byID[id]; ok {
		return 1, true
	}
	return 0, false
}

// Volume returns the volume of one unit of an item, and whether it is known: only an item the game's tables list has an answer (0 is an answer: the item takes no volume).
func Volume(id string) (float64, bool) {
	v, ok := volumes[id]
	return v, ok
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

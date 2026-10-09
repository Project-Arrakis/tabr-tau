package ops

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/skills"
)

// Building sets and customizations as the Dune Docker console's tabs have them (console/api/src/adminCatalog.js, MIT, RedBlink): the
// building sets are the catalog's items filed under "BuildingSets", sorted into groups with an Experimental one; the customizations
// are cosmetic sets (Atreides, Harkonnen, Smuggler, Dune Man, Filmic Archive), some of which need a DLC or an account entitlement.

// CatalogRow is one entry of the Building Sets or Customizations tab.
type CatalogRow struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Group        string `json:"group"`
	GroupID      string `json:"groupId,omitempty"`
	Experimental bool   `json:"experimental,omitempty"`
	RequiredDLC  string `json:"requiredDlc,omitempty"`
	Entitlement  bool   `json:"entitlement,omitempty"` // controlled by an account entitlement, which this editor cannot grant
	Learned      bool   `json:"learned"`               // a building set the character has learned
	InInventory  bool   `json:"inInventory"`           // the item is in one of the character's inventories
	InCatalog    bool   `json:"inCatalog"`             // the item catalog knows it (the give action needs that)
}

var (
	experimentalRe     = regexp.MustCompile(`(?i)(?:^|[_\s])(Developer|Polar|Test|Placeholder|Debug)(?:$|[_\s])`)
	experimentalIDRe   = regexp.MustCompile(`(?i)^(?:IceRefinery|WaterTower)_Patent$`)
	experimentalNameRe = regexp.MustCompile(`(?i)^(?:PH_|XX|BUILDING_SET_)`)
	factionSetRe       = regexp.MustCompile(`(?i)(?:Atreides|Harkonnen|Choam|Smug|Fremen|House|Faction).*Set`)
	craftingSetRe      = regexp.MustCompile(`(?i)(?:Fabricator|Refinery|Station|Container|Extraction|Cistern|Windtrap|Windturbine|Generator|Recycler|Pentashield|Silo|Deathstill|Compactor|Workbench|Printer)`)
	furnitureSetRe     = regexp.MustCompile(`(?i)(?:Furniture|Bedroom|Dining|Office|Lighting|Table|Chair|Statue|Decor|Mural|Banner|Carpet|Glowglobe|Trophy)`)
	factionPrefixRe    = regexp.MustCompile(`(?i)^(?:Atre_|Hark_|Choam_)`)
	mtxRe              = regexp.MustCompile(`(?i)^MTX_`)
)

// buildingGroup files a building-set item into one of the console's groups. An item with developer-only or incomplete metadata is
// Experimental.
func buildingGroup(id, name string) (group string, experimental bool) {
	experimental = experimentalRe.MatchString(id+" "+name) || experimentalIDRe.MatchString(id) || experimentalNameRe.MatchString(name) || id == name
	switch {
	case experimental:
		return "Experimental", true
	case mtxRe.MatchString(id):
		return "Special & Promotional", false
	case factionPrefixRe.MatchString(id) || factionSetRe.MatchString(id):
		return "Faction & House Sets", false
	case craftingSetRe.MatchString(id):
		return "Crafting & Utilities", false
	case furnitureSetRe.MatchString(id):
		return "Furniture & Decorations", false
	}
	return "Structures & Building Sets", false
}

// learnedAlias says whether a set id counts as learned: the table may hold the id with or without its "_Patent" ending.
func learnedAlias(learned map[string]bool, id string) bool {
	if learned[id] {
		return true
	}
	if strings.HasSuffix(id, "_Patent") {
		return learned[strings.TrimSuffix(id, "_Patent")]
	}
	return learned[id+"_Patent"]
}

func (o *Ops) learnedSets(controller int64) map[string]bool {
	rows, _ := o.S.Query(`select learned_building_set s from building_progression_learned_building_sets where character_id=?`, controller)
	out := map[string]bool{}
	for _, r := range rows {
		out[fmt.Sprint(r["s"])] = true
	}
	return out
}

// BuildingSets lists the catalog's building sets in the console's groups (Experimental among them), with whether each is learned or
// already in the inventory, plus the learned sets the catalog does not list (store packs), and the character's new buildable pieces.
func (o *Ops) BuildingSets() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	learned := o.learnedSets(p.Controller)
	inv, err := o.inventoryTemplates(p.Pawn)
	if err != nil {
		return nil, err
	}
	out := []CatalogRow{}
	seen := map[string]bool{}
	for _, e := range catalog.All() {
		if e.Category != "buildings" || e.Source != "BuildingSets" || seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		g, exp := buildingGroup(e.ID, e.Name)
		out = append(out, CatalogRow{ID: e.ID, Name: e.Name, Group: g, Experimental: exp, RequiredDLC: e.RequiredDLC, Learned: learnedAlias(learned, e.ID), InInventory: inv[e.ID], InCatalog: true})
	}
	for id := range learned {
		if !seen[id] && !seen[id+"_Patent"] && !seen[strings.TrimSuffix(id, "_Patent")] {
			out = append(out, CatalogRow{ID: id, Name: tidyWords(strings.TrimSuffix(id, "_Patent")), Group: "Learned, not in the catalog", Learned: true, InInventory: inv[id]})
		}
	}
	sortRows(out)
	pieces, _ := o.S.Query(`select new_buildable_piece name from building_progression_new_buildable_pieces where character_id=? order by 1`, p.Controller)
	return map[string]any{"rows": out, "newPieces": pieces}, nil
}

var customizationGroups = []struct {
	ID, Name string
	re       *regexp.Regexp
}{
	{"atreides", "Atreides", regexp.MustCompile(`(?i)^B1C3_Atre`)},
	{"harkonnen", "Harkonnen", regexp.MustCompile(`(?i)^B1C3_Hark`)},
	{"smuggler", "Smuggler", regexp.MustCompile(`(?i)^(?:MTX_)?B1C3_Smug`)},
	{"dune-man", "Dune Man", regexp.MustCompile(`(?i)^MTX_B1C2_DuneMan`)},
	{"filmic-archive", "Filmic Archive", regexp.MustCompile(`(?i)^(?:MTX_Fremen_FedaykinArmor_SetVariant|MTX_Atre_CaladanTrenchcoat_SetVariant|MTX_Sard_Scout_SetVariant)$`)},
}

var lostHarvestRe = regexp.MustCompile(`(?i)^(?:MTX_B1C2_DuneMan|MTX_Neut_DesertMechanic)`)

const otherCustomizations = "other"

// customizationRows is the catalog's cosmetic items in the console's sets, then the rest under "Other" (the console lists only the sets;
// the others stay available here).
func (o *Ops) customizationRows(inv map[string]bool) []CatalogRow {
	out := []CatalogRow{}
	seen := map[string]bool{}
	for _, e := range catalog.All() {
		if e.Category != "customizations" || e.Source != "Customizations" || seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		row := CatalogRow{ID: e.ID, Name: e.Name, GroupID: otherCustomizations, Group: "Other (not in the console's sets)", InInventory: inv[e.ID], InCatalog: true}
		for _, g := range customizationGroups {
			if g.re.MatchString(e.ID) {
				row.GroupID, row.Group = g.ID, g.Name
				break
			}
		}
		row.RequiredDLC = e.RequiredDLC
		if row.RequiredDLC == "" && lostHarvestRe.MatchString(e.ID) {
			row.RequiredDLC = "Lost Harvest"
		}
		row.Entitlement = row.RequiredDLC != "" || mtxRe.MatchString(e.ID)
		out = append(out, row)
	}
	sortRows(out)
	return out
}

// CustomizationSet is one cosmetic set: how many cosmetics it has and what it requires.
type CustomizationSet struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Count       int    `json:"count"`
	Requirement string `json:"requirement"`
}

// Customizations lists the cosmetic sets (with their counts and what they require) and every cosmetic, with whether it is already in an
// inventory. The save keeps no list of owned customizations that tabr-tau can read, so (as in the console) only the inventory is checked.
func (o *Ops) Customizations() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	inv, err := o.inventoryTemplates(p.Pawn)
	if err != nil {
		return nil, err
	}
	rows := o.customizationRows(inv)
	var groups []CustomizationSet
	for _, g := range append(customizationGroupList(), struct{ ID, Name string }{otherCustomizations, "Other (not in the console's sets)"}) {
		gr := CustomizationSet{ID: g.ID, Name: g.Name}
		dlc := map[string]bool{}
		ent := false
		for _, r := range rows {
			if r.GroupID == g.ID {
				gr.Count++
				if r.RequiredDLC != "" {
					dlc[r.RequiredDLC] = true
				}
				ent = ent || r.Entitlement
			}
		}
		if len(dlc) > 0 {
			var names []string
			for n := range dlc {
				names = append(names, n)
			}
			gr.Requirement = "Requires " + strings.Join(sortedStrings(names), ", ")
		} else if ent {
			gr.Requirement = "Entitlement controlled"
		}
		if gr.Count > 0 {
			groups = append(groups, gr)
		}
	}
	return map[string]any{"rows": rows, "groups": groups}, nil
}

func customizationGroupList() []struct{ ID, Name string } {
	out := make([]struct{ ID, Name string }, 0, len(customizationGroups))
	for _, g := range customizationGroups {
		out = append(out, struct{ ID, Name string }{g.ID, g.Name})
	}
	return out
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// GrantCustomizations puts the cosmetic items of one set (args group: a set id, or "all" for the console's five sets) into the backpack,
// one of each, skipping what is already in an inventory. It is all or nothing: if the backpack cannot hold them all, nothing is added and
// the message says how many fit.
func (o *Ops) GrantCustomizations(a Args) (any, error) {
	want := a.Str("group")
	if want == "" {
		return nil, errors.New("choose a set")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	inv, err := o.inventoryTemplates(p.Pawn)
	if err != nil {
		return nil, err
	}
	var items []CatalogRow
	valid := want == "all"
	for _, r := range o.customizationRows(inv) {
		if r.GroupID == want {
			valid = true
		}
		if (want == "all" && r.GroupID != otherCustomizations) || r.GroupID == want {
			if !r.InInventory {
				items = append(items, r)
			}
		}
	}
	if !valid {
		return nil, errors.New("unknown set")
	}
	if len(items) == 0 {
		return map[string]any{"ok": true, "granted": 0, "skipped": "all of this set is already in your inventory"}, nil
	}
	bp, _ := o.S.One(`select id from inventories where actor_id=? and inventory_type=0 order by id limit 1`, p.Pawn)
	if bp == nil {
		return nil, errors.New("player backpack not found")
	}
	var ids []int64
	if _, err := o.S.Mutate(fmt.Sprintf("grant %d customizations (%s)", len(items), want), func(m *save.Mut) error {
		for i, it := range items {
			id, e := giveInTx(m, bp["id"].(int64), it.ID, 1, 0)
			if e != nil {
				return fmt.Errorf("the backpack has room for %d of the %d items (%s: %v); make room and try again", i, len(items), it.ID, e)
			}
			ids = append(ids, id)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "granted": len(ids)}, nil
}

// UnlockAllBuildingSets teaches the character every building set of the catalog that is not Experimental and not yet learned, by adding
// it to the learned sets. It adds only learned-set rows (the pieces' "new" markers are left as they are).
func (o *Ops) UnlockAllBuildingSets() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var added int
	if _, err := o.S.Mutate("unlock all building sets", func(m *save.Mut) error {
		learned := map[string]bool{}
		rows, err := m.Query(`select learned_building_set s from building_progression_learned_building_sets where character_id=?`, p.Controller)
		if err != nil {
			return err
		}
		for _, r := range rows {
			learned[fmt.Sprint(r["s"])] = true
		}
		for _, e := range catalog.All() {
			if e.Category != "buildings" || e.Source != "BuildingSets" {
				continue
			}
			if _, exp := buildingGroup(e.ID, e.Name); exp || learnedAlias(learned, e.ID) {
				continue
			}
			if _, err := m.Exec(`insert or ignore into building_progression_learned_building_sets(character_id, learned_building_set) values (?,?)`, p.Controller, e.ID); err != nil {
				return err
			}
			learned[e.ID] = true
			added++
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "added": added}, nil
}

// UnlockAllResearch buys every research entry that unlocks something (recipes and buildings; group markers have nothing of their own),
// adding the recipe or building set each one unlocks, and repairs purchased entries whose recipe or set is missing.
func (o *Ops) UnlockAllResearch() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var bought, repaired, added int
	if _, err := o.S.Mutate("unlock all research", func(m *save.Mut) error {
		rows, err := m.Query(`select i.key idx, json_extract(i.value,'$.ItemKey') k, json_extract(i.value,'$.UnlockedState') st from actors a, json_each(a.properties,'`+researchArr+`') i where a.id=? order by i.key`, p.Pawn)
		if err != nil {
			return err
		}
		for _, r := range rows {
			key, _ := r["k"].(string)
			kind, id, piece := unlockOf(key)
			if key == "" || id == "" {
				continue
			}
			did := false
			switch kind {
			case "recipe":
				a, err := addRecipeTx(m, p.Pawn, id)
				if err != nil {
					return err
				}
				did = a
			case "building":
				r1, err := m.Exec(`insert or ignore into building_progression_learned_building_sets(character_id, learned_building_set) values (?,?)`, p.Controller, id)
				if err != nil {
					return err
				}
				r2, err := m.Exec(`insert or ignore into building_progression_new_buildable_pieces(character_id, new_buildable_piece) values (?,?)`, p.Controller, piece)
				if err != nil {
					return err
				}
				n1, _ := r1.RowsAffected()
				n2, _ := r2.RowsAffected()
				did = n1+n2 > 0
			}
			if did {
				added++
			}
			if r["st"] == "Purchased" {
				if did {
					repaired++
				}
				continue
			}
			idx, _ := toInt(r["idx"])
			base := fmt.Sprintf("%s[%d]", researchArr, idx)
			if _, err := m.Exec(`update actors set properties=jsonb_set(jsonb_set(properties, ?, 'Purchased'), ?, jsonb('false')) where id=?`, base+".UnlockedState", base+".bIsNewEntry", p.Pawn); err != nil {
				return err
			}
			bought++
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "bought": bought, "repaired": repaired, "unlocksAdded": added}, nil
}

// UnlockAllRecipes adds every crafting recipe the character's research tree offers and the character does not know yet.
func (o *Ops) UnlockAllRecipes() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var added int
	if _, err := o.S.Mutate("unlock all crafting recipes", func(m *save.Mut) error {
		rows, err := m.Query(`select json_extract(i.value,'$.ItemKey') k from actors a, json_each(a.properties,'`+researchArr+`') i
			where a.id=? and json_extract(i.value,'$.ItemKey') like 'RCP\_%' escape '\'`, p.Pawn)
		if err != nil {
			return err
		}
		for _, r := range rows {
			key, _ := r["k"].(string)
			id := strings.TrimPrefix(key, "RCP_")
			if id == "" {
				continue
			}
			a, err := addRecipeTx(m, p.Pawn, id)
			if err != nil {
				return err
			}
			if a {
				added++
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "added": added}, nil
}

// MaxSkills sets every skill module of a school (args school: its key, or "all" for the five schools) to its highest rank. It does not
// touch the unspent points.
func (o *Ops) MaxSkills(a Args) (any, error) {
	school := a.Str("school")
	valid := school == "all"
	for _, s := range skills.Schools {
		valid = valid || s.Key == school
	}
	if !valid {
		return nil, errors.New("unknown school")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	changed := 0
	if _, err := o.S.Mutate("max skills: "+school, func(m *save.Mut) error {
		for _, mod := range skills.All() {
			in := false
			for _, s := range skills.Schools {
				if mod.Category == s.Key && (school == "all" || school == s.Key) {
					in = true
				}
			}
			if !in {
				continue
			}
			ent, _, err := characterLevel(m, p.Pawn)
			if err != nil {
				return err
			}
			cur, err := m.Query(`select json_extract(components,'`+modulePath(mod.ID)+`.SkillPointsSpent') s from fgl_entities where entity_id=?`, ent)
			if err != nil {
				return err
			}
			var spent int64
			if len(cur) > 0 {
				spent, _ = toInt(cur[0]["s"])
			}
			if skills.Rank(int(spent), mod, true) >= mod.MaxLevel {
				continue
			}
			if _, _, err := setModuleTx(m, p.Pawn, mod.ID, mod.MaxLevel, false); err != nil {
				return err
			}
			changed++
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "changed": changed}, nil
}

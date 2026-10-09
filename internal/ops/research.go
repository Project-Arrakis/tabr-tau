package ops

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/catalog"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Research and crafting recipes, as the console's Research and Crafting tabs have them (internal/ops/research.go follows
// console/api/src/duneDb.js and console/api/src/duneDb/presentation.js, MIT, RedBlink).
//
// In a save the research tree is the character actor's TechKnowledgePlayerComponent.m_TechKnowledge.m_TechKnowledgeData, a list of
// {ItemKey, UnlockedState (Purchased / NotPurchased), bIsNewEntry}. A "RCP_x" key unlocks the crafting recipe "x", which lives in
// CraftingRecipesLibraryActorComponent.m_KnownItemRecipes; a "BLD_x" key unlocks the building set "x" (or "x_Patent"), which lives in
// the building_progression_* tables. A "DA_GRP_" key is a group marker with nothing of its own to unlock.
const (
	researchArr = "$.TechKnowledgePlayerComponent.m_TechKnowledge.m_TechKnowledgeData"
	recipesArr  = "$.CraftingRecipesLibraryActorComponent.m_KnownItemRecipes"
)

var (
	researchKeyRe = regexp.MustCompile(`^[A-Za-z0-9_().+\-]{1,200}$`)
	recipeIDRe    = regexp.MustCompile(`^[A-Za-z0-9_().\-]{1,200}$`)
	camelA        = regexp.MustCompile(`([a-z])([A-Z0-9])`)
	camelB        = regexp.MustCompile(`([0-9])([A-Z])`)
	wsRe          = regexp.MustCompile(`\s+`)
	rcpPrefix     = regexp.MustCompile(`^(RCP_|DA_GRP_|BLD_)`)
	patentSuffix  = regexp.MustCompile(`(?i)_?Patent$`)
	recipeSuffix  = regexp.MustCompile(`(?i)_?Recipe$`)
)

var researchAliases = map[string]string{"RCP_AssaultRifleRecipe": "Karpov 38"}
var recipeAliases = map[string]string{"AssaultRifleRecipe": "Karpov 38"}

func tidyWords(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	s = camelA.ReplaceAllString(s, "$1 $2")
	s = camelB.ReplaceAllString(s, "$1 $2")
	return strings.TrimSpace(wsRe.ReplaceAllString(s, " "))
}

func researchName(key string) string {
	if a, ok := researchAliases[key]; ok {
		return a
	}
	s := rcpPrefix.ReplaceAllString(key, "")
	s = patentSuffix.ReplaceAllString(s, "")
	s = recipeSuffix.ReplaceAllString(s, "")
	if t := tidyWords(s); t != "" {
		return t
	}
	return key
}

func recipeName(id string) string {
	if a, ok := recipeAliases[id]; ok {
		return a
	}
	if t := tidyWords(recipeSuffix.ReplaceAllString(id, "")); t != "" {
		return t
	}
	return id
}

func researchType(key string) string {
	switch {
	case strings.HasPrefix(key, "RCP_"):
		return "Recipe"
	case strings.HasPrefix(key, "BLD_"):
		return "Building"
	case strings.HasPrefix(key, "DA_GRP_"):
		return "Group"
	}
	return "Research"
}

func anyOf(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func researchCategory(key string) string {
	v := strings.ToLower(key)
	switch {
	case anyOf(v, "unique", "recyclerdummy"):
		return "Uniques"
	case anyOf(v, "vehicle", "sandbike", "buggy", "orni", "ornithopter", "thopter", "repairtool", "welding", "fuel"):
		return "Vehicles"
	case anyOf(v, "stillsuit", "literjon", "blood", "dew", "water", "windtrap", "cistern", "exsanguination", "stilltent"):
		return "Water Discipline"
	case anyOf(v, "armor", "ammo", "rifle", "pistol", "shotgun", "smg", "lmg", "weapon", "lasgun", "compactor", "kindjal", "crysknife", "knife", "sword", "shield", "napalm", "dirk", "rapier", "rocket"):
		return "Combat"
	case anyOf(v, "bld_", "building", "shelter", "totem", "generator", "lighting", "silo", "fabricator", "refinery", "container", "staking", "pentashield", "turbine", "spice"):
		return "Construction"
	case anyOf(v, "scanner", "binocular", "powerpack", "radiation", "cutteray", "mining", "thumper", "suspensor", "probe", "spice", "stabilization"):
		return "Exploration"
	case anyOf(v, "augment"):
		return "Augmentations"
	}
	return "Essentials"
}

func researchProductGroup(key, category string) string {
	v := strings.ToLower(key)
	switch {
	case anyOf(v, "t6", "plastanium", "regis"):
		return "Plastanium Products"
	case anyOf(v, "t5", "duraluminum", "duraluminium"):
		return "Duraluminum Products"
	case anyOf(v, "t4", "aluminum", "aluminium"):
		return "Aluminum Products"
	case anyOf(v, "t3", "steel"):
		return "Steel Products"
	case anyOf(v, "t2", "iron"):
		return "Iron Products"
	case anyOf(v, "copper"):
		return "Copper Products"
	case anyOf(v, "augment"):
		return "Generic Augmentations"
	case category == "Uniques", category == "Vehicles":
		return "Copper Products"
	}
	return "Salvage Products"
}

func recipeCategory(id string) string {
	v := strings.ToLower(id)
	switch {
	case anyOf(v, "buggy", "sandbike", "vehicle", "treadwheel", "ornithopter", "sandcrawler"):
		return "Vehicles"
	case anyOf(v, "stillsuit", "literjon", "bloodsack", "blood_sack", "bodyfluid", "dew", "water", "stilltent"):
		return "Water Discipline"
	case anyOf(v, "ammo", "rifle", "pistol", "shotgun", "smg", "weapon", "lasgun", "flamethrower", "staticcompactor", "kindjal", "crysknife", "knife", "sword", "shield", "napalm", "disruptor"):
		return "Combat"
	case anyOf(v, "building", "basebackup", "portablelight", "decajon", "totem", "refinery", "container", "fabricator", "placeable", "structure", "generator", "turbine", "pentashield", "silo", "lighting"):
		return "Construction"
	case anyOf(v, "scanner", "powerpack", "radiation", "cutteray", "miningtool", "mining_tool", "thumper", "suspensor", "fuel", "harvester"):
		return "Exploration"
	}
	return "Essentials"
}

// unlockOf says what a research key unlocks: a crafting recipe, a building set (and its placeable), or nothing (a group marker).
func unlockOf(key string) (kind, id, piece string) {
	switch {
	case strings.HasPrefix(key, "BLD_"):
		b := strings.TrimPrefix(key, "BLD_")
		id = b
		if !strings.HasSuffix(key, "_Patent") && catalog.Category(b) != "buildings" {
			id = b + "_Patent"
		}
		return "building", id, strings.TrimSuffix(id, "_Patent") + "_Placeable"
	case strings.HasPrefix(key, "RCP_"):
		return "recipe", strings.TrimPrefix(key, "RCP_"), ""
	}
	return "group", "", ""
}

// ResearchRow is one research entry of the character.
type ResearchRow struct {
	ItemKey      string `json:"itemKey"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	ProductGroup string `json:"productGroup"`
	Type         string `json:"type"`
	State        string `json:"state"`
	IsNew        bool   `json:"isNew"`
	UnlockKind   string `json:"unlockKind"`
	UnlockID     string `json:"unlockId"`
	Purchased    bool   `json:"purchased"`
	Materialized bool   `json:"materialized"` // the recipe or building set the entry unlocks is really there
	Unlocked     bool   `json:"unlocked"`     // purchased and materialized
	NeedsRepair  bool   `json:"needsRepair"`  // purchased but its recipe / building set is missing
	Actionable   bool   `json:"actionable"`
}

func (o *Ops) knownRecipes(pawn int64) (map[string]bool, error) {
	rows, err := o.S.Query(`select json_extract(i.value,'$.BaseRecipeId.Name') n from actors a, json_each(a.properties,'`+recipesArr+`') i where a.id=?`, pawn)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, r := range rows {
		if n, ok := r["n"].(string); ok && n != "" {
			out[n] = true
		}
	}
	return out, nil
}

// Research is the character's research tree as the console's Research tab lists it.
func (o *Ops) Research() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	rows, err := o.S.Query(`select json_extract(i.value,'$.ItemKey') k, json_extract(i.value,'$.UnlockedState') st, json_extract(i.value,'$.bIsNewEntry') nw
		from actors a, json_each(a.properties,'`+researchArr+`') i where a.id=?`, p.Pawn)
	if err != nil {
		return nil, err
	}
	known, err := o.knownRecipes(p.Pawn)
	if err != nil {
		return nil, err
	}
	sets := map[string]bool{}
	lr, _ := o.S.Query(`select learned_building_set s from building_progression_learned_building_sets where character_id=?`, p.Controller)
	for _, r := range lr {
		sets[fmt.Sprint(r["s"])] = true
	}
	out := make([]ResearchRow, 0, len(rows))
	for _, r := range rows {
		key, _ := r["k"].(string)
		if key == "" {
			continue
		}
		st, _ := r["st"].(string)
		if st == "" {
			st = "Unknown"
		}
		kind, id, _ := unlockOf(key)
		mat := true
		switch kind {
		case "recipe":
			mat = known[id]
		case "building":
			mat = sets[id]
		}
		cat := researchCategory(key)
		purchased := st == "Purchased"
		out = append(out, ResearchRow{ItemKey: key, Name: researchName(key), Category: cat, ProductGroup: researchProductGroup(key, cat), Type: researchType(key),
			State: st, IsNew: truthy(r["nw"]), UnlockKind: kind, UnlockID: id, Purchased: purchased, Materialized: mat, Unlocked: purchased && mat,
			NeedsRepair: id != "" && purchased && !mat, Actionable: id != ""})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ItemKey < out[j].ItemKey })
	return map[string]any{"rows": out}, nil
}

// newRecipeJSON is a known-recipe entry in the order and shape the game writes them.
func newRecipeJSON(id string) string {
	return fmt.Sprintf(`{"BaseRecipeId":{"Name":%q},"m_QualityLevel":0,"m_bIsNew":true,"m_NumberOfRecipeUses":0,"m_bIsLimitedUseRecipe":false,"m_Source":"SchematicPickup"}`, id)
}

// addRecipeTx adds a recipe to the character's known recipes when it is not there. It reports whether it added one.
func addRecipeTx(m *save.Mut, pawn int64, id string) (bool, error) {
	rows, err := m.Query(`select json_type(properties,'`+recipesArr+`') t,
			(select count(*) from json_each(properties,'`+recipesArr+`') i where json_extract(i.value,'$.BaseRecipeId.Name')=?) c
		from actors where id=?`, id, pawn)
	if err != nil {
		return false, err
	}
	if len(rows) == 0 || rows[0]["t"] != "array" {
		return false, errors.New("this save has no known-recipes list for the character")
	}
	if c, _ := toInt(rows[0]["c"]); c > 0 {
		return false, nil
	}
	_, err = m.Exec(`update actors set properties=jsonb_insert(properties, ?, jsonb(?)) where id=?`, recipesArr+"[#]", newRecipeJSON(id), pawn)
	return err == nil, err
}

// UnlockResearch marks a research entry purchased and adds what it unlocks (its recipe or building set), as the console does.
// Group markers are refused (they unlock nothing of their own), and so are keys the character's research list does not have.
func (o *Ops) UnlockResearch(a Args) (any, error) {
	key := a.Str("item_key")
	if !researchKeyRe.MatchString(key) {
		return nil, errors.New("invalid research key")
	}
	kind, id, piece := unlockOf(key)
	if id == "" {
		return nil, fmt.Errorf("%s is a group marker; unlock its individual Recipe or Building entries instead", key)
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var already, added bool
	_, err = o.S.Mutate("unlock research "+key, func(m *save.Mut) error {
		rows, err := m.Query(`select i.key idx, json_extract(i.value,'$.UnlockedState') st from actors a, json_each(a.properties,'`+researchArr+`') i
			where a.id=? and json_extract(i.value,'$.ItemKey')=?`, p.Pawn, key)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return fmt.Errorf("research key %s is not in this character's research list", key)
		}
		idx, _ := toInt(rows[0]["idx"])
		already = rows[0]["st"] == "Purchased"
		switch kind {
		case "recipe":
			added, err = addRecipeTx(m, p.Pawn, id)
			if err != nil {
				return err
			}
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
			added = n1+n2 > 0
		}
		if already {
			return nil
		}
		base := fmt.Sprintf("%s[%d]", researchArr, idx)
		_, err = m.Exec(`update actors set properties=jsonb_set(jsonb_set(properties, ?, 'Purchased'), ?, jsonb('false')) where id=?`, base+".UnlockedState", base+".bIsNewEntry", p.Pawn)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "alreadyPurchased": already, "unlockKind": kind, "unlockId": id, "unlockAdded": added, "repaired": already && added}, nil
}

// CraftingRow is one crafting recipe, known or available.
type CraftingRow struct {
	RecipeID   string `json:"recipeId"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Source     string `json:"source"` // how the character has it ("Known", "Schematic", ...) or "Research" when it can still be unlocked
	Known      bool   `json:"known"`
	Limited    bool   `json:"limited"` // a limited-use recipe
	Uses       int64  `json:"uses"`
	ResearchOf string `json:"researchKey,omitempty"`
}

// Crafting lists the recipes the character knows and the ones its research tree can still give. The recipe universe is the research
// tree's RCP_ keys plus what the character already knows: in a real save every known recipe is one of those, which the console's
// guess from the item catalog's schematic names is not (it found 15 of 180).
func (o *Ops) Crafting() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	rows, err := o.S.Query(`select json_extract(i.value,'$.BaseRecipeId.Name') n, json_extract(i.value,'$.m_Source') src,
			json_extract(i.value,'$.m_bIsLimitedUseRecipe') lim, json_extract(i.value,'$.m_NumberOfRecipeUses') uses
		from actors a, json_each(a.properties,'`+recipesArr+`') i where a.id=?`, p.Pawn)
	if err != nil {
		return nil, err
	}
	out := []CraftingRow{}
	seen := map[string]bool{}
	for _, r := range rows {
		id, _ := r["n"].(string)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		src, _ := r["src"].(string)
		if src == "" {
			src = "Known"
		}
		uses, _ := toInt(r["uses"])
		out = append(out, CraftingRow{RecipeID: id, Name: recipeName(id), Category: recipeCategory(id), Source: src, Known: true, Limited: truthy(r["lim"]), Uses: uses})
	}
	rs, _ := o.S.Query(`select json_extract(i.value,'$.ItemKey') k from actors a, json_each(a.properties,'`+researchArr+`') i
		where a.id=? and json_extract(i.value,'$.ItemKey') like 'RCP\_%' escape '\'`, p.Pawn)
	for _, r := range rs {
		key, _ := r["k"].(string)
		id := strings.TrimPrefix(key, "RCP_")
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, CraftingRow{RecipeID: id, Name: recipeName(id), Category: recipeCategory(id), Source: "Research", ResearchOf: key})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].RecipeID < out[j].RecipeID
	})
	return map[string]any{"rows": out}, nil
}

// UnlockRecipe adds a crafting recipe to the character's known recipes (args recipe_id). The id must be one the character already
// knows or one its research tree offers (RCP_<id>).
func (o *Ops) UnlockRecipe(a Args) (any, error) {
	id := a.Str("recipe_id")
	if !recipeIDRe.MatchString(id) {
		return nil, errors.New("invalid recipe id")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var added bool
	if _, err := o.S.Mutate("unlock recipe "+id, func(m *save.Mut) error {
		rows, err := m.Query(`select count(*) c from actors a, json_each(a.properties,'`+researchArr+`') i
			where a.id=? and json_extract(i.value,'$.ItemKey')=?`, p.Pawn, "RCP_"+id)
		if err != nil {
			return err
		}
		if c, _ := toInt(rows[0]["c"]); c == 0 {
			k, err := m.Query(`select count(*) c from actors a, json_each(a.properties,'`+recipesArr+`') i where a.id=? and json_extract(i.value,'$.BaseRecipeId.Name')=?`, p.Pawn, id)
			if err != nil {
				return err
			}
			if c, _ := toInt(k[0]["c"]); c == 0 {
				return fmt.Errorf("recipe %s is not in this character's research or known recipes", id)
			}
		}
		added, err = addRecipeTx(m, p.Pawn, id)
		return err
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "alreadyKnown": !added}, nil
}

package ops

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// starterResearch is what a brand-new character already has: the four research entries that are bought in a save made right after the
// character was created (read from such a save; it also holds exactly the four recipes these entries give and no other recipe). A new
// character's list has 50 entries, those four bought and the rest not bought and marked new.
var starterResearch = map[string]bool{
	"RCP_MiningTool_1h_StandardRecipe":      true,
	"RCP_T2_MiscEquipment_PowerPack_Recipe": true,
	"RCP_HealthPackRecipe":                  true,
	"RCP_ScrapMetalKnifeRecipe":             true,
}

var errNoResearchList = errors.New("this save has no research list for the character")

// researchResetSet is what a reset of the research would change, for the page and for the edit itself.
type researchResetSet struct {
	entries []int64  // positions in the research list of the bought entries a new character does not have
	recipes []int64  // positions in the known-recipes list of the recipes those entries give
	sets    []string // building sets those entries give that the character has learned
	pieces  []string // the "new piece" rows the unlock of such a set adds
}

// researchReset reads the entries a reset would take back. A bought entry that is not one of the four a new character starts with goes back to
// not bought; the recipe it gives (RCP_x gives x) leaves the known recipes and the building set it gives (BLD_x gives x or x_Patent) leaves the
// learned sets, with the piece the unlock added. A recipe the character knows from a schematic whose research entry is not bought, a recipe the
// research tree does not list, and a set the tree does not list (a store pack, say) are not touched.
func researchReset(q rowQuerier, pawn, controller int64) (*researchResetSet, error) {
	t, err := q.Query(`select json_type(properties,'`+researchArr+`') r, json_type(properties,'`+recipesArr+`') c from actors where id=?`, pawn)
	if err != nil {
		return nil, err
	}
	if len(t) == 0 || t[0]["r"] != "array" {
		return nil, errNoResearchList
	}
	rows, err := q.Query(`select i.key idx, json_extract(i.value,'$.ItemKey') k from actors a, json_each(a.properties,'`+researchArr+`') i
		where a.id=? and json_extract(i.value,'$.UnlockedState')='Purchased' order by i.key`, pawn)
	if err != nil {
		return nil, err
	}
	out := &researchResetSet{}
	recipeIDs := map[string]bool{}
	pieceOf := map[string]string{}
	for _, r := range rows {
		key, _ := r["k"].(string)
		idx, ok := toInt(r["idx"])
		if key == "" || !ok || starterResearch[key] {
			continue
		}
		out.entries = append(out.entries, idx)
		switch kind, id, piece := unlockOf(key); kind {
		case "recipe":
			recipeIDs[id] = true
		case "building":
			pieceOf[id] = piece
		}
	}
	if t[0]["c"] == "array" && len(recipeIDs) > 0 {
		rs, err := q.Query(`select i.key idx, json_extract(i.value,'$.BaseRecipeId.Name') n from actors a, json_each(a.properties,'`+recipesArr+`') i
			where a.id=? order by i.key`, pawn)
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			if n, _ := r["n"].(string); recipeIDs[n] {
				if idx, ok := toInt(r["idx"]); ok {
					out.recipes = append(out.recipes, idx)
				}
			}
		}
	}
	if len(pieceOf) > 0 {
		ls, err := q.Query(`select learned_building_set s from building_progression_learned_building_sets where character_id=?`, controller)
		if err != nil {
			return nil, err
		}
		for _, r := range ls {
			if s, _ := r["s"].(string); pieceOf[s] != "" {
				out.sets = append(out.sets, s)
			}
		}
		ps, err := q.Query(`select new_buildable_piece p from building_progression_new_buildable_pieces where character_id=?`, controller)
		if err != nil {
			return nil, err
		}
		wanted := map[string]bool{}
		for _, s := range out.sets {
			wanted[pieceOf[s]] = true
		}
		for _, r := range ps {
			if p, _ := r["p"].(string); wanted[p] {
				out.pieces = append(out.pieces, p)
			}
		}
	}
	return out, nil
}

// ResetResearch gives the character's research back to what a new character has (args expected: the number of bought entries the page showed and
// the person confirmed). Every bought entry except the four a new character starts with goes back to not bought and new; the recipes and the
// building sets those entries gave are taken out again (the sets with their "new piece" rows). Intel, the items, the placed buildings, the skills and
// everything else are not touched, and no Intel is given back: the save does not say what an entry cost. It is read inside the edit; a count that
// is not the confirmed one is refused with a conflict and nothing is written.
func (o *Ops) ResetResearch(a Args) (any, error) {
	expected, err := expectedCount(a)
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var set *researchResetSet
	if _, err := o.S.Mutate("reset research", func(m *save.Mut) error {
		s, err := researchReset(m, p.Pawn, p.Controller)
		if err != nil {
			return err
		}
		if int64(len(s.entries)) != expected {
			return resetMismatch("the number of bought research entries", int64(len(s.entries)), expected)
		}
		set = s
		for _, idx := range s.entries {
			base := researchArr + "[" + strconv.FormatInt(idx, 10) + "]"
			if _, err := m.Exec(`update actors set properties=jsonb_set(jsonb_set(properties, ?, 'NotPurchased'), ?, jsonb('true')) where id=?`, base+".UnlockedState", base+".bIsNewEntry", p.Pawn); err != nil {
				return err
			}
		}
		for i := len(s.recipes) - 1; i >= 0; i-- { // from the end, so the positions still to remove do not move
			if _, err := m.Exec(`update actors set properties=jsonb_remove(properties, ?) where id=?`, recipesArr+"["+strconv.FormatInt(s.recipes[i], 10)+"]", p.Pawn); err != nil {
				return err
			}
		}
		for _, set := range s.sets {
			if _, err := m.Exec(`delete from building_progression_learned_building_sets where character_id=? and learned_building_set=?`, p.Controller, set); err != nil {
				return err
			}
		}
		for _, piece := range s.pieces {
			if _, err := m.Exec(`delete from building_progression_new_buildable_pieces where character_id=? and new_buildable_piece=?`, p.Controller, piece); err != nil {
				return err
			}
		}
		m.Desc = fmt.Sprintf("reset research (%d entries, %d recipes and %d building sets taken out)", len(s.entries), len(s.recipes), len(s.sets))
		return nil
	}); err != nil {
		return nil, err
	}
	return resetAnswer(int64(len(set.entries)), map[string]any{"recipes": int64(len(set.recipes)), "sets": int64(len(set.sets))}), nil
}

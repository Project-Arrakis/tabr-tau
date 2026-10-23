package ops

import (
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/skills"
)

// playableSchool says whether a catalog category is one of the five schools the Skills tab lists with ranks. The Hidden group (skills the
// game grants by itself) and modules the catalog omits are not part of a reset, as they are not part of "Max all skills".
func playableSchool(category string) bool {
	for _, s := range skills.Schools {
		if s.Key == category {
			return true
		}
	}
	return false
}

// resettableSkills are the catalog modules of the five schools that hold points, in the catalog's order. A reset puts exactly these at rank 0;
// the Skills tab shows their number, and the reset counts them again inside its own edit.
func resettableSkills(spentBy map[string]int64) []skills.Module {
	var out []skills.Module
	for _, m := range skills.All() {
		if playableSchool(m.Category) && spentBy[m.ID] > 0 {
			out = append(out, m)
		}
	}
	return out
}

// ResetSkills puts every skill module of the five schools at rank 0 (args expected: the number of modules the page showed and the person
// confirmed; charge: also give the points back to the unspent points, the same box as the per-skill rank control). It is the per-skill edit
// (setModuleTx) applied to each module that holds points, in one transaction, so the points and the unspent count stay consistent in exactly
// the way Set rank keeps them. Hidden modules, modules the catalog does not know, the level, the XP and the total of skill points are not
// touched. The modules are read inside the edit; a count that is not the confirmed one is refused with a conflict and nothing is written.
func (o *Ops) ResetSkills(a Args) (any, error) {
	expected, err := expectedCount(a)
	if err != nil {
		return nil, err
	}
	charge := a.Bool("charge", false)
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var changed int64
	if _, err := o.S.Mutate("reset skills", func(m *save.Mut) error {
		if _, _, err := characterLevel(m, p.Pawn); err != nil {
			return err
		}
		spentBy, err := savedModules(m, p.Pawn)
		if err != nil {
			return err
		}
		set := resettableSkills(spentBy)
		if int64(len(set)) != expected {
			return resetMismatch("the number of skills", int64(len(set)), expected)
		}
		for _, mod := range set {
			if _, _, err := setModuleTx(m, p.Pawn, mod.ID, 0, charge); err != nil {
				return err
			}
		}
		changed = int64(len(set))
		if charge {
			m.Desc = fmt.Sprintf("reset skills (%d changed, points given back)", changed)
		} else {
			m.Desc = fmt.Sprintf("reset skills (%d changed)", changed)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return resetAnswer(changed, nil), nil
}

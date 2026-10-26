package ops

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/kits"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Apply kit (#417): a care package (internal/kits) goes to the character in ONE edit. The items go to the backpack by the Give Items path (giveLinesInTx: the slots, the stack sizes,
// the volume, the bound, the augments), then the XP by Give XP's rule (addXPTx) and the skill points (addSkillPointsTx). Like a give, a kit puts in as much as fits and says what did
// not and why; a fault (a save with no level record while the kit has XP, say) refuses the whole kit and nothing is kept. The kit remembers nothing about the save, so applying it twice
// gives it twice. The edit is the ordinary one: it is written after a backup, or queued while a game process exists (the web layer's post does that; nothing here writes a file).

// errKitDryRun ends a dry run's edit so that it is rolled back: the numbers are the real ones, worked out by the same code on the same rows, and nothing is kept.
var errKitDryRun = errors.New("dry run")

func unitsWord(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// ApplyKit applies k to the character, or with dry only works out what it would do and keeps nothing.
// The answer: kit, dryRun, lines (one per item: template_id, name, requested, given, reason, limit), requested and given (units), notGiven (the lines that got less than asked: name,
// requested, short, reason), xp and skillPoints (when the kit has them: requested, applied, capped; xp also has what Give XP answers), complete (everything asked for was given and
// no cap cut the XP or the skill points), changed (the save would change or changed) and volumeNote (the first caveat of the volume check).
func (o *Ops) ApplyKit(k kits.Kit, dry bool) (any, error) {
	k.Normalize()
	if err := k.Validate(); err != nil {
		return nil, apierr.Newf(apierr.ErrBadArgument, "the kit cannot be used: %v", err)
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	lines := make([]giveLine, 0, len(k.Items))
	for _, l := range k.Items {
		grade := int(l.AugmentGrade)
		if grade < 1 {
			grade = 1
		}
		lines = append(lines, giveLine{l.Item, l.Quantity, l.Grade, l.Augments, grade})
	}
	var inv int64
	if len(lines) > 0 {
		r, _ := o.S.One(`select id from inventories where actor_id=? and inventory_type=0 order by id limit 1`, p.Pawn)
		var ok bool
		if r != nil {
			inv, ok = r["id"].(int64)
		}
		if !ok {
			return nil, apierr.New(apierr.ErrNotFound, "player backpack not found")
		}
	}
	var res map[string]any
	_, err = o.S.Mutate(fmt.Sprintf("Apply kit \"%s\"", k.Name), func(m *save.Mut) error {
		res = map[string]any{"ok": true, "kit": k.Name, "dryRun": dry, "lines": []map[string]any{}, "requested": int64(0), "given": int64(0), "notGiven": []map[string]any{}, "volumeNote": ""}
		var parts []string
		complete, changed := true, false
		if len(lines) > 0 {
			_, req, got, results, e := giveLinesInTx(m, inv, lines)
			if e != nil {
				return e
			}
			notGiven := []map[string]any{}
			for i, r := range results {
				r["name"] = itemLabel(lines[i].tmpl)
				if short := lines[i].qty - r["given"].(int64); short > 0 {
					notGiven = append(notGiven, map[string]any{"name": r["name"], "requested": r["requested"], "short": short, "reason": r["reason"], "limit": r["limit"]})
				}
				if n, _ := r["volumeNote"].(string); n != "" && res["volumeNote"] == "" {
					res["volumeNote"] = n
				}
			}
			res["lines"], res["requested"], res["given"], res["notGiven"] = results, req, got, notGiven
			complete = got == req
			changed = got > 0
			if got == 0 && k.XP == 0 && k.SkillPoints == 0 && !dry {
				return apierr.New(apierr.ErrNoRoom, "Nothing was given: "+firstReason(results)+".")
			}
			if got < req {
				parts = append(parts, fmt.Sprintf("%s of %s units fitted", commas(got), commas(req)))
			} else {
				parts = append(parts, fmt.Sprintf("%s, %s units", unitsWord(len(lines), "item", "items"), commas(got)))
			}
		}
		if k.XP > 0 {
			x, e := addXPTx(m, p.Pawn, k.XP)
			if e != nil {
				return e
			}
			x["requested"] = k.XP
			res["xp"] = x
			complete = complete && x["capped"] == false
			applied, _ := x["applied"].(int64)
			changed = changed || applied > 0
			parts = append(parts, fmt.Sprintf("+%s XP", commas(applied)))
		}
		if k.SkillPoints > 0 {
			n, e := addSkillPointsTx(m, p.Pawn, k.SkillPoints)
			if e != nil {
				return e
			}
			res["skillPoints"] = map[string]any{"requested": k.SkillPoints, "applied": n, "capped": n < k.SkillPoints}
			complete = complete && n == k.SkillPoints
			changed = changed || n > 0
			parts = append(parts, fmt.Sprintf("+%s skill points", commas(n)))
		}
		res["complete"], res["changed"] = complete, changed
		m.Desc = fmt.Sprintf("Apply kit \"%s\": %s", k.Name, strings.Join(parts, ", "))
		if dry {
			return errKitDryRun
		}
		return nil
	})
	if dry && errors.Is(err, errKitDryRun) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

// firstReason says in words why the first line that got less than asked got less ("the backpack is full").
func firstReason(results []map[string]any) string {
	for _, r := range results {
		if s, _ := r["reason"].(string); s != "" {
			return s
		}
	}
	return "the backpack is full"
}

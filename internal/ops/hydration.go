package ops

import (
	"errors"
	"strconv"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// maxHydration is the most water the character holds (the summary shows it as the bar's end).
const maxHydration = 100

const hydrationBase = "$.DuneHydrationAttributeSet.CurrentHydration.BaseValue"
const hydrationCurrent = "$.DuneHydrationAttributeSet.CurrentHydration.CurrentValue"

// FillHydration fills the character's water to the maximum. Hydration is kept in the character actor's gas_attributes, as a base and
// a current value (the game stores both as text such as "70.114852"); both are set, as text, so the record keeps its shape, and
// nothing else in it is touched. A save with no hydration record is refused rather than given a made-up one. The result says what it
// was before and what it is now.
func (o *Ops) FillHydration() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var before float64
	_, err = o.S.Mutate("fill hydration", func(m *save.Mut) error {
		var err error
		before, err = fillHydrationTx(m, p.Pawn, false)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "before": before, "after": float64(maxHydration)}, nil
}

// errNoHydration is the answer of fillHydrationTx for a save that holds no hydration record for the character.
var errNoHydration = errors.New("this save has no hydration record for the character")

// fillHydrationTx is FillHydration inside the edit m, for the character whose pawn actor is pawn; it returns the water the character had. With
// onlyBelowMax a character that is already full is not written (the button writes the same value again, harmlessly; the automatic pass must
// not leave an edit behind). It sets no description.
func fillHydrationTx(m *save.Mut, pawn int64, onlyBelowMax bool) (float64, error) {
	full := strconv.FormatFloat(maxHydration, 'f', 6, 64)
	rows, err := m.Query(`select json_extract(gas_attributes, ?) cur from actors where id=? and json_valid(gas_attributes, 8)`, hydrationCurrent, pawn)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 || rows[0]["cur"] == nil {
		return 0, errNoHydration
	}
	var before float64
	switch v := rows[0]["cur"].(type) {
	case float64:
		before = v
	case int64:
		before = float64(v)
	case string:
		f, perr := strconv.ParseFloat(v, 64)
		if perr != nil {
			return 0, errors.New("the character's hydration is not a number")
		}
		before = f
	default:
		return 0, errors.New("the character's hydration is not a number")
	}
	if onlyBelowMax && before >= maxHydration {
		return before, nil
	}
	_, err = m.Exec(`update actors set gas_attributes=jsonb_set(jsonb_set(gas_attributes, ?, ?), ?, ?) where id=?`, hydrationBase, full, hydrationCurrent, full, pawn)
	return before, err
}

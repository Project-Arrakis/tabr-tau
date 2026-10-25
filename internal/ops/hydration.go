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
// a current value. The game's own saves hold both as JSON numbers (59.289321); an older save, or one written by an older editor, holds
// them as text ("70.114852"). Each value is written back as the type it has (a number as a number, text as text), so the record keeps its
// shape and the game reads it the way it reads its own; nothing else in the record is touched. A save with no hydration record is refused
// rather than given a made-up one. The result says what it was before, what it is now and what type the values had.
func (o *Ops) FillHydration() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var w hydrationWrite
	_, err = o.S.Mutate("fill hydration", func(m *save.Mut) error {
		var err error
		w, err = fillHydrationTx(m, p.Pawn, false)
		if err == nil {
			m.Desc = "fill hydration (" + w.describe() + ")"
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "before": w.Before, "after": float64(maxHydration), "type": w.Type, "baseType": w.BaseType}, nil
}

// errNoHydration is the answer of fillHydrationTx for a save that holds no hydration record for the character.
var errNoHydration = errors.New("this save has no hydration record for the character")

// The two types a hydration value is kept as.
const (
	hydrationNumber = "number" // a JSON number: what the game writes in current saves
	hydrationText   = "text"   // a JSON string such as "70.114852": what older saves hold
)

// hydrationWrite is what fillHydrationTx found and did: the water the character had, and the type the current value and the base value
// were kept as (both are written back in that type).
type hydrationWrite struct {
	Before   float64
	Type     string // of CurrentValue
	BaseType string // of BaseValue
}

// describe says the type in words for the write log and the answer: "numbers", "text", or both when the two differ.
func (w hydrationWrite) describe() string {
	if w.Type == w.BaseType {
		return "kept as " + w.Type
	}
	return "current kept as " + w.Type + ", base as " + w.BaseType
}

// hydrationKind names the type of a stored JSON value, "" for anything that is neither a number nor text.
func hydrationKind(jsonType any) string {
	switch jsonType {
	case "integer", "real":
		return hydrationNumber
	case "text":
		return hydrationText
	}
	return ""
}

// fillHydrationTx is FillHydration inside the edit m, for the character whose pawn actor is pawn; it returns the water the character had and
// the type it was kept as. With onlyBelowMax a character that is already full is not written (the button writes the same value again,
// harmlessly; the automatic pass must not leave an edit behind). It sets no description.
func fillHydrationTx(m *save.Mut, pawn int64, onlyBelowMax bool) (hydrationWrite, error) {
	rows, err := m.Query(`select json_extract(gas_attributes, ?) cur, json_type(gas_attributes, ?) curtype, json_type(gas_attributes, ?) basetype
		from actors where id=? and json_valid(gas_attributes, 8)`, hydrationCurrent, hydrationCurrent, hydrationBase, pawn)
	if err != nil {
		return hydrationWrite{}, err
	}
	if len(rows) == 0 || rows[0]["cur"] == nil {
		return hydrationWrite{}, errNoHydration
	}
	notNumber := errors.New("the character's hydration is not a number")
	w := hydrationWrite{Type: hydrationKind(rows[0]["curtype"])}
	if w.Type == "" {
		return hydrationWrite{}, notNumber // a boolean, an object or an array
	}
	switch v := rows[0]["cur"].(type) {
	case float64:
		w.Before = v
	case int64:
		w.Before = float64(v)
	case string:
		f, perr := strconv.ParseFloat(v, 64)
		if perr != nil {
			return hydrationWrite{}, notNumber
		}
		w.Before = f
	default:
		return hydrationWrite{}, notNumber
	}
	// The base value keeps its own type; a record that has none (or one of another type) is given the current value's.
	w.BaseType = hydrationKind(rows[0]["basetype"])
	if w.BaseType == "" {
		w.BaseType = w.Type
	}
	if onlyBelowMax && w.Before >= maxHydration {
		return w, nil
	}
	value := func(kind string) any {
		if kind == hydrationText {
			return strconv.FormatFloat(maxHydration, 'f', 6, 64)
		}
		return float64(maxHydration) // a real number (100.0), as the game writes them
	}
	_, err = m.Exec(`update actors set gas_attributes=jsonb_set(jsonb_set(gas_attributes, ?, ?), ?, ?) where id=?`,
		hydrationBase, value(w.BaseType), hydrationCurrent, value(w.Type), pawn)
	return w, err
}

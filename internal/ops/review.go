package ops

import (
	"database/sql"

	"github.com/Project-Arrakis/tabr-tau/internal/diff"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Review reports what has been changed since the save was loaded: the recorded edits and a row- and JSON-path-level
// diff of the pristine baseline against the working copy. Identifiers are redacted unless unredacted is set.
func (o *Ops) Review(unredacted bool, maxRows int) (any, error) {
	var res *diff.Result
	var dirty bool
	var ops []save.Op
	err := o.S.WithBaselineState(func(orig, cur *sql.DB, d bool, recorded []save.Op) error {
		var err error
		dirty, ops = d, recorded
		res, err = diff.Compare(orig, cur, diff.Options{MaxRows: maxRows, NoRedact: unredacted})
		return err
	})
	if err != nil {
		return nil, err
	}
	if !unredacted { // edit descriptions can quote SQL or ids
		for i := range ops {
			ops[i].Desc = diff.RedactText(ops[i].Desc)
		}
	}
	return map[string]any{"dirty": dirty, "ops": ops, "diff": res}, nil
}

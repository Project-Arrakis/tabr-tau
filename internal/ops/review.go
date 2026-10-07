package ops

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

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
	token := reviewToken(ops) // from the unredacted text, before display redaction
	if !unredacted {          // edit descriptions can quote SQL or ids
		for i := range ops {
			ops[i].Desc = diff.RedactText(ops[i].Desc)
		}
	}
	return map[string]any{"dirty": dirty, "ops": ops, "diff": res, "token": token}, nil
}

// reviewToken names exactly the recorded edits a person was shown. Saving must quote it, so an edit made after the
// review (or a save attempted without one) is refused rather than written unseen.
func reviewToken(ops []save.Op) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\n", len(ops))
	for _, o := range ops {
		fmt.Fprintf(h, "%d:%s\n", len(o.Desc), o.Desc)
	}
	return hex.EncodeToString(h.Sum(nil))[:24]
}

// CheckReviewed refuses a save unless token matches the edits currently pending.
func (o *Ops) CheckReviewed(token string) error {
	if token == "" {
		return errors.New("review the changes before saving")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(reviewToken(o.S.Ops()))) != 1 {
		return errors.New("the pending changes are different from the ones you reviewed; review them again")
	}
	return nil
}

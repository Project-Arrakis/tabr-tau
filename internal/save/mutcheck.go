package save

import (
	"fmt"
	"strings"
)

// CheckReferences runs, inside the edit, the check Mutate runs when the edit's function has returned: an edit may not leave more dangling references than the save had before
// it. A preview that does the whole edit and then rolls it back by returning an error never reaches that check, so it calls this first; a base, say, that the edit could not be
// written for is then refused when the question is asked and not after it was answered. It reads the whole save (as the check at the end does) and changes nothing.
func (m *Mut) CheckReferences() error {
	if m.s == nil {
		return nil
	}
	fk, err := fkCounts(m.tx)
	if err != nil {
		return err
	}
	return danglingError(m.s.fkBase, fk)
}

// danglingError is the refusal of an edit that leaves more dangling references than there were, or nil.
func danglingError(base, now map[string]int) error {
	bad := worse(base, now)
	if len(bad) == 0 {
		return nil
	}
	more := ""
	if len(bad) > 5 {
		more = fmt.Sprintf(" and %d more", len(bad)-5)
		bad = bad[:5]
	}
	return fmt.Errorf("this edit would leave dangling references (%s%s); nothing was changed", strings.Join(bad, "; "), more)
}

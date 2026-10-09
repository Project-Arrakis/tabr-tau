package ops

import (
	"errors"
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

const intelPath = "$.TechKnowledgePlayerComponent.m_TechKnowledgePoints"

// AddIntel adds Intel (the points spent in the research tree) to the character, never past the cap the console uses (2,779). The
// value is a plain number in the character actor's properties, beside the rest of its tech-knowledge record, which is left as it
// is. A save with no such record is refused rather than given a made-up one. The result says what was applied, because the cap
// can make that less than asked.
func (o *Ops) AddIntel(a Args) (any, error) {
	amount, err := a.IntRange("amount", 1, 1_000_000_000)
	if err != nil {
		return nil, errors.New("amount must be a whole number from 1 to 1,000,000,000")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var before, after int64
	_, err = o.S.Mutate(fmt.Sprintf("intel +%d", amount), func(m *save.Mut) error {
		rows, err := m.Query(`select json_extract(properties, ?) n from actors where id=? and json_valid(properties, 8)`, intelPath, p.Pawn)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return errors.New("this save has no Intel record for the character")
		}
		cur, ok := rows[0]["n"].(int64)
		if !ok {
			return errors.New("this save has no Intel record for the character")
		}
		before = cur
		after = min(cur+amount, maxIntel)
		if after == before {
			return nil // already at the cap: nothing to write, and no empty edit left pending
		}
		_, err = m.Exec(`update actors set properties=jsonb_set(properties, ?, ?) where id=?`, intelPath, after, p.Pawn)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "before": before, "after": after, "applied": after - before, "capped": before+amount > maxIntel}, nil
}

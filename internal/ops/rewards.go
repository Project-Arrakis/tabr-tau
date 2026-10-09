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

// AddXP adds experience to the character and keeps the level data consistent the way the game does: the level follows the XP, and
// a character of level n has n-1 skill points in total, so each level gained adds one skill point to both the total and the
// unspent count (checked against a real save at three different levels). Nothing else in the record is touched. XP stops at the
// last level (200). A save without the level record is refused.
func (o *Ops) AddXP(a Args) (any, error) {
	amount, err := a.IntRange("amount", 1, 100_000_000)
	if err != nil {
		return nil, errors.New("amount must be a whole number from 1 to 100,000,000")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	maxXP := xpByLevel[len(xpByLevel)-1]
	var res map[string]any
	_, err = o.S.Mutate(fmt.Sprintf("xp +%d", amount), func(m *save.Mut) error {
		rows, err := m.Query(`select f.entity_id id,
				json_extract(f.components,'$.FLevelComponent[1].TotalXPEarned') xp,
				json_extract(f.components,'$.FLevelComponent[1].TotalSkillPoints') total,
				json_extract(f.components,'$.FLevelComponent[1].UnspentSkillPoints') unspent
			from actor_fgl_entities a join fgl_entities f on f.entity_id=a.entity_id
			where a.actor_id=? and a.slot_name='DuneCharacter' and json_valid(f.components, 8)`, p.Pawn)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return errors.New("this save has no level record for the character")
		}
		xp, ok1 := toInt(rows[0]["xp"])
		total, ok2 := toInt(rows[0]["total"])
		unspent, ok3 := toInt(rows[0]["unspent"])
		if !ok1 || !ok2 || !ok3 {
			return errors.New("this save has no level record for the character")
		}
		after := min(xp+amount, maxXP)
		lb, la := xpToLevel(xp), xpToLevel(after)
		gained := la - lb
		res = map[string]any{"ok": true, "before": xp, "after": after, "applied": after - xp, "levelBefore": lb, "levelAfter": la,
			"skillPointsGained": gained, "capped": xp+amount > maxXP}
		if after == xp {
			return nil // already at the last level: nothing to write
		}
		_, err = m.Exec(`update fgl_entities set components=jsonb_set(jsonb_set(jsonb_set(components,
				'$.FLevelComponent[1].TotalXPEarned', ?), '$.FLevelComponent[1].TotalSkillPoints', ?), '$.FLevelComponent[1].UnspentSkillPoints', ?)
			where entity_id=?`, after, total+int64(gained), unspent+int64(gained), rows[0]["id"])
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// AddCurrency adds to one of the character's virtual currency balances: 0 is Solari Credit, 1 the secondary wallet (House Credit),
// as in the console. This is not the Solari Coin item stack in the inventory (see AddSolari). A wallet with no row yet is created.
func (o *Ops) AddCurrency(a Args) (any, error) {
	id, err := a.IntRange("currency", 0, 1)
	if err != nil {
		return nil, errors.New("currency must be 0 (Solari Credit) or 1 (House Credit)")
	}
	amount, err := a.IntRange("amount", 1, 1_000_000_000_000)
	if err != nil {
		return nil, errors.New("amount must be a whole number from 1 to 1,000,000,000,000")
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var before int64
	_, err = o.S.Mutate(fmt.Sprintf("currency %d +%d", id, amount), func(m *save.Mut) error {
		rows, err := m.Query(`select balance from player_virtual_currency_balances where player_controller_id=? and currency_id=?`, p.Controller, id)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			before, _ = toInt(rows[0]["balance"])
		}
		_, err = m.Exec(`insert into player_virtual_currency_balances(player_controller_id, currency_id, balance) values (?,?,?)
			on conflict(player_controller_id, currency_id) do update set balance=excluded.balance`, p.Controller, id, before+amount)
		return err
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "currency": id, "before": before, "after": before + amount}, nil
}

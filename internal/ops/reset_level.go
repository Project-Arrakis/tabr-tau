package ops

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// levelRec is the character's level record as the XP and skill-point edits see it.
type levelRec struct {
	entity             any // the fgl_entities row that holds FLevelComponent
	xp, total, unspent int64
}

// readLevelTx reads the level record inside an edit. A save without it (or with it unreadable) is refused.
func readLevelTx(m *save.Mut, pawn int64) (levelRec, error) {
	rows, err := m.Query(`select f.entity_id id,
			json_extract(f.components,'$.FLevelComponent[1].TotalXPEarned') xp,
			json_extract(f.components,'$.FLevelComponent[1].TotalSkillPoints') total,
			json_extract(f.components,'$.FLevelComponent[1].UnspentSkillPoints') unspent
		from actor_fgl_entities a join fgl_entities f on f.entity_id=a.entity_id
		where a.actor_id=? and a.slot_name='DuneCharacter' and json_valid(f.components, 8)`, pawn)
	if err != nil {
		return levelRec{}, err
	}
	if len(rows) == 0 {
		return levelRec{}, errors.New("this save has no level record for the character")
	}
	xp, ok1 := toInt(rows[0]["xp"])
	total, ok2 := toInt(rows[0]["total"])
	unspent, ok3 := toInt(rows[0]["unspent"])
	if !ok1 || !ok2 || !ok3 {
		return levelRec{}, errors.New("this save has no level record for the character")
	}
	return levelRec{entity: rows[0]["id"], xp: xp, total: total, unspent: unspent}, nil
}

// writeLevelTx writes the three numbers the level follows and nothing else of the record. Give XP and the level reset both end here.
func writeLevelTx(m *save.Mut, entity any, xp, total, unspent int64) error {
	_, err := m.Exec(`update fgl_entities set components=jsonb_set(jsonb_set(jsonb_set(components,
			'$.FLevelComponent[1].TotalXPEarned', ?), '$.FLevelComponent[1].TotalSkillPoints', ?), '$.FLevelComponent[1].UnspentSkillPoints', ?)
		where entity_id=?`, xp, total, unspent, entity)
	return err
}

// ResetLevel puts the character's XP back to 0 (args expected: the XP the page showed and the person confirmed). The level follows the XP, and Give XP
// adds one skill point to both the total and the unspent count for each level gained, so the reset takes those points back the same way: as many
// as the levels the XP stood for, from the total and from the unspent points, never below 0. Skills already bought, points that did not come from
// levels and everything else in the record stay; reset the skills first (with the points given back) to be left with none. The XP is read inside the
// edit and a different number than the confirmed one is refused with a conflict.
func (o *Ops) ResetLevel(a Args) (any, error) {
	expected, err := expectedCount(a)
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var changed int64
	var levelBefore int
	if _, err := o.S.Mutate("reset level and xp", func(m *save.Mut) error {
		rec, err := readLevelTx(m, p.Pawn)
		if err != nil {
			return err
		}
		if rec.xp != expected {
			return resetMismatch("the XP", rec.xp, expected)
		}
		changed = rec.xp
		levelBefore = xpToLevel(rec.xp)
		if rec.xp == 0 {
			return nil
		}
		lost := int64(levelBefore)
		m.Desc = fmt.Sprintf("reset level and xp (%d XP, level %d to 0)", rec.xp, levelBefore)
		return writeLevelTx(m, rec.entity, 0, max(rec.total-lost, 0), max(rec.unspent-lost, 0))
	}); err != nil {
		return nil, err
	}
	return resetAnswer(changed, map[string]any{"levelBefore": levelBefore}), nil
}

// knownTrackNumbers are the numbers of the tracks the editor has learned, each once, in order.
func knownTrackNumbers(nums map[string]int64) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for _, t := range SpecTracks {
		if n, ok := nums[t]; ok && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// deleteSpecTx removes one track's row (XP and level back to nothing). The single-track Reset and the reset of all tracks both end here.
func deleteSpecTx(m *save.Mut, controller, number int64) (int64, error) {
	res, err := m.Exec(`delete from specialization_tracks where player_id=? and track_type=?`, controller, number)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ResetSpecTracks removes the XP rows of every specialization track whose number the editor knows (args expected: how many such rows the page
// showed). A row whose track number is not known is left alone, since removing it could clear another track's XP; keystones are not touched either,
// as with the per-track Reset. The rows are counted inside the edit and a different number than the confirmed one is refused with a conflict.
func (o *Ops) ResetSpecTracks(nums map[string]int64, a Args) (any, error) {
	expected, err := expectedCount(a)
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var changed int64
	if _, err := o.S.Mutate("reset specialization XP", func(m *save.Mut) error {
		var have int64
		for _, n := range knownTrackNumbers(nums) {
			r, err := m.One(`select count(*) c from specialization_tracks where player_id=? and track_type=?`, p.Controller, n)
			if err != nil {
				return err
			}
			c, _ := toInt(r["c"])
			have += c
		}
		if have != expected {
			return resetMismatch("the number of specialization tracks with XP", have, expected)
		}
		for _, n := range knownTrackNumbers(nums) {
			d, err := deleteSpecTx(m, p.Controller, n)
			if err != nil {
				return err
			}
			changed += d
		}
		m.Desc = fmt.Sprintf("reset specialization XP (%d changed)", changed)
		return nil
	}); err != nil {
		return nil, err
	}
	return resetAnswer(changed, nil), nil
}

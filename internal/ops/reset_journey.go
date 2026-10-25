package ops

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Project-Arrakis/tabr-tau/internal/journey"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// journeyResetSet is what a reset of the journey would change: the Story and Codex nodes whose complete state is true (a Contract is derived from
// tags and read-only on the page, so it is not one of them) and the tutorials that have a state (started or complete). The Journey page shows
// their number, and the reset reads them again inside its own edit. The steps and tutorials a brand-new character already has finished (starterJourneyNodes,
// starterTutorials) are not among them.
func journeyResetSet(q rowQuerier, controller int64) (nodes []string, tutorials []int64, err error) {
	rows, err := q.Query(`select story_node_id id from journey_story_node where character_id=? and complete_condition_state=? order by story_node_id`, controller, jsonTrue)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range rows {
		if id, ok := r["id"].(string); ok && journey.Group(id) != "contract" && !starterJourneyNodes[id] {
			nodes = append(nodes, id)
		}
	}
	trows, err := q.Query(`select tp.tutorial_id id, t.name name, tp.tutorial_state state from tutorial_per_player tp join tutorials t on t.id=tp.tutorial_id
		where tp.player_id=? and tp.tutorial_state<>0 order by tp.tutorial_id`, controller)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range trows {
		if name, _ := r["name"].(string); starterTutorials[name] {
			if st, _ := toInt(r["state"]); st == 2 {
				continue // complete in a new character, and complete here
			}
		}
		if id, ok := toInt(r["id"]); ok {
			tutorials = append(tutorials, id)
		}
	}
	sort.Slice(tutorials, func(i, j int) bool { return tutorials[i] < tutorials[j] })
	return nodes, tutorials, nil
}

// ResetJourney takes the character's journey back (args expected: the number of finished Story and Codex nodes plus started or finished tutorials the
// page showed and the person confirmed). It is the Reset button of each row applied to each of them, in one transaction: a finished node (and its
// children) is no longer complete and is marked revealed, a tutorial is back to not started. Reward-pending flags, tags, contracts and everything
// else are not touched, so the reset neither grants nor takes away a reward. The rows are read inside the edit and a different number than the
// confirmed one is refused with a conflict.
func (o *Ops) ResetJourney(a Args) (any, error) {
	expected, err := expectedCount(a)
	if err != nil {
		return nil, err
	}
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var nn, nt int64
	if _, err := o.S.Mutate("reset journey", func(m *save.Mut) error {
		nodes, tuts, err := journeyResetSet(m, p.Controller)
		if err != nil {
			return err
		}
		if int64(len(nodes)+len(tuts)) != expected {
			return resetMismatch("the number of finished journey steps and tutorials", int64(len(nodes)+len(tuts)), expected)
		}
		// A step resets its children too, and a step a new character has finished can be a child of one that is reset (the Dunipedia fragments are):
		// the statement leaves the rows of those steps out.
		for _, id := range nodes {
			if _, err := setNodeStateKeepingTx(m, p.Controller, id, jsonEmptyObj, starterJSON()); err != nil {
				return err
			}
		}
		for _, id := range tuts {
			if err := clearTutorialTx(m, p.Controller, id); err != nil {
				return err
			}
		}
		nn, nt = int64(len(nodes)), int64(len(tuts))
		m.Desc = fmt.Sprintf("reset journey (%d changed: %d nodes, %d tutorials)", nn+nt, nn, nt)
		return nil
	}); err != nil {
		return nil, err
	}
	return resetAnswer(nn+nt, map[string]any{"nodes": nn, "tutorials": nt}), nil
}

// starterJSON is the steps a new character has finished as a JSON array, for a statement to leave out.
func starterJSON() string {
	ids := make([]string, 0, len(starterJourneyNodes))
	for id := range starterJourneyNodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	b, _ := json.Marshal(ids)
	return string(b)
}

// setNodeStateKeepingTx is setNodeStateTx for the reset of the whole journey: the rows named in keep (a JSON array of step ids) are left as they are.
func setNodeStateKeepingTx(m *save.Mut, controller int64, id string, state []byte, keep string) (int64, error) {
	res, err := m.Exec(`update journey_story_node set complete_condition_state=?, reveal_condition_state=?
		where character_id=? and (story_node_id=? or substr(story_node_id, 1, length(?) + 1) = ? || '.') and story_node_id not in (select value from json_each(?))`,
		state, jsonTrue, controller, id, id, id, keep)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

package ops

import (
	"fmt"
	"sort"

	"github.com/Project-Arrakis/tabr-tau/internal/journey"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// journeyResetSet is what a reset of the journey would change: the Story and Codex nodes whose complete state is true (a Contract is derived from
// tags and read-only on the page, so it is not one of them) and the tutorials that have a state (started or complete). The Journey page shows
// their number, and the reset reads them again inside its own edit.
func journeyResetSet(q rowQuerier, controller int64) (nodes []string, tutorials []int64, err error) {
	rows, err := q.Query(`select story_node_id id from journey_story_node where character_id=? and complete_condition_state=? order by story_node_id`, controller, jsonTrue)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range rows {
		if id, ok := r["id"].(string); ok && journey.Group(id) != "contract" {
			nodes = append(nodes, id)
		}
	}
	trows, err := q.Query(`select tp.tutorial_id id from tutorial_per_player tp join tutorials t on t.id=tp.tutorial_id
		where tp.player_id=? and tp.tutorial_state<>0 order by tp.tutorial_id`, controller)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range trows {
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
		for _, id := range nodes {
			if _, err := setNodeStateTx(m, p.Controller, id, jsonEmptyObj); err != nil {
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

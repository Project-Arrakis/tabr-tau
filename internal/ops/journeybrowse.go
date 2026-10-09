package ops

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/journey"
)

// JourneyRow is one line of the Journey browser, as in the console's: the node, its readable name, where it sits in the tree
// and where the character stands.
type JourneyRow struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Depth         int    `json:"depth"`
	Parent        string `json:"parent"`
	Status        string `json:"status"`
	Complete      bool   `json:"complete"`
	Revealed      bool   `json:"revealed"`
	PendingReward bool   `json:"pendingReward"`
	Tags          int    `json:"tags"`
	Actionable    bool   `json:"actionable"` // Complete / Reset change this row's state in the save
	TutorialState *int64 `json:"state,omitempty"`
}

// JourneyBrowse is the character's Journey as the console's Journey Browser shows it: Story nodes, Contracts, Codex entries and
// Tutorials, each with a readable name, its depth under the parent node, and Complete / Revealed / Incomplete status.
//
// Story and Codex rows come from the save's journey_story_node table (completed means the node's complete condition is true).
// Catalog nodes the character has no row for yet are listed as Incomplete. A Contract row is complete when every tag the catalog
// says the contract sets is among the character's player_tags. Contract rows are read-only here.
func (o *Ops) JourneyBrowse() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	type st struct{ complete, revealed, pending bool }
	state := map[string]st{}
	rows, err := o.S.Query(`select story_node_id id, complete_condition_state=? c, reveal_condition_state=? r, has_pending_reward p
		from journey_story_node where character_id=?`, jsonTrue, jsonTrue, p.Controller)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		state[r["id"].(string)] = st{truthy(r["c"]), truthy(r["r"]), truthy(r["p"])}
	}
	tagRows, _ := o.S.Query(`select tag from player_tags where character_id=?`, p.Controller)
	tags := map[string]bool{}
	for _, r := range tagRows {
		tags[r["tag"].(string)] = true
	}
	// every node id seen in any row (the save has the player's only), plus the catalog's
	all, _ := o.S.Query(`select distinct story_node_id id from journey_story_node`)
	known := map[string]bool{}
	for _, id := range journey.KnownNodes() {
		known[id] = true
	}
	for _, r := range all {
		known[r["id"].(string)] = true
	}
	status := func(s st) string {
		switch {
		case s.complete:
			return "Complete"
		case s.revealed:
			return "Revealed"
		}
		return "Incomplete"
	}
	var storyIDs, contractIDs, codexIDs []string
	for id := range known {
		switch {
		case strings.HasPrefix(id, "DA_Dunipedia_"):
			codexIDs = append(codexIDs, id)
		case journey.Group(id) == "contract":
			contractIDs = append(contractIDs, id)
		default:
			storyIDs = append(storyIDs, id)
		}
	}
	build := func(ids []string, category string, actionable bool) []JourneyRow {
		sort.Slice(ids, func(i, j int) bool { return journey.Less(ids[i], ids[j]) })
		set := make(map[string]bool, len(ids))
		for _, id := range ids {
			set[id] = true
		}
		out := make([]JourneyRow, 0, len(ids))
		for _, id := range ids {
			s := state[id]
			out = append(out, JourneyRow{ID: id, Name: journey.DisplayName(id), Category: category, Depth: journey.Depth(id, set), Parent: journey.Parent(id, set),
				Status: status(s), Complete: s.complete, Revealed: s.revealed, PendingReward: s.pending, Tags: journey.NodeTagCount(id), Actionable: actionable})
		}
		return out
	}
	story := build(storyIDs, "Story", true)
	contract := build(contractIDs, "Contract", false)
	codex := build(codexIDs, "Codex", true)
	// the catalog's own contracts (done when all their tags are on the character)
	for _, full := range journey.Contracts() {
		t := journey.ContractTags(full)
		done := len(t) > 0
		for _, tag := range t {
			done = done && tags[tag]
		}
		s := "Incomplete"
		if done {
			s = "Complete"
		}
		contract = append(contract, JourneyRow{ID: full, Name: journey.ContractName(full), Category: "Contract", Status: s, Complete: done, Tags: len(t)})
	}
	sort.SliceStable(contract, func(i, j int) bool { return contract[i].Name < contract[j].Name })
	tut := []JourneyRow{}
	trows, _ := o.S.Query(`select t.id, t.name, tp.tutorial_state state from tutorials t
		left join tutorial_per_player tp on tp.tutorial_id=t.id and tp.player_id=? order by t.name`, p.Controller)
	for _, r := range trows {
		var ts int64
		if v, ok := r["state"].(int64); ok {
			ts = v
		}
		s := "Not Started"
		switch ts {
		case 2:
			s = "Complete"
		case 1:
			s = "Started"
		}
		id := ts
		tut = append(tut, JourneyRow{ID: fmt.Sprint(r["id"]), Name: journey.DisplayName(r["name"].(string)), Category: "Tutorial", Status: s, Complete: ts == 2, Actionable: true, TutorialState: &id})
	}
	return map[string]any{"story": story, "contract": contract, "codex": codex, "tutorial": tut}, nil
}

func truthy(v any) bool {
	switch n := v.(type) {
	case int64:
		return n != 0
	case bool:
		return n
	}
	return false
}

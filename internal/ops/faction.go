package ops

import (
	"errors"
	"fmt"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Faction assignment as in the console's Admin tab: Neutral, Atreides or Harkonnen. The save's factions table also has Smuggler,
// which the console does not offer, so neither does this.
var assignableFactions = []int64{3, 1, 2} // None (shown as Neutral), Atreides, Harkonnen

func factionLabel(name string) string {
	if name == "None" {
		return "Neutral"
	}
	return name
}

// Faction is the character's current faction and the ones that can be assigned.
func (o *Ops) Faction() (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	var cur any
	if r, _ := o.S.One(`select pf.faction_id id, f.name from player_faction pf join factions f on f.id=pf.faction_id where pf.actor_id=?`, p.Controller); len(r) > 0 {
		cur = map[string]any{"id": r["id"], "name": factionLabel(fmt.Sprint(r["name"]))}
	}
	opts := []map[string]any{}
	for _, id := range assignableFactions {
		if r, _ := o.S.One(`select name from factions where id=?`, id); len(r) > 0 {
			opts = append(opts, map[string]any{"id": id, "name": factionLabel(fmt.Sprint(r["name"]))})
		}
	}
	return map[string]any{"current": cur, "options": opts}, nil
}

// SetFaction assigns the character to a faction (args faction_id: 1 Atreides, 2 Harkonnen, 3 Neutral). It writes the one
// player_faction row of the character, stamping the change time the way the game does for the player (microseconds); reputation is
// not touched (it is under Character > Reputation).
func (o *Ops) SetFaction(a Args) (any, error) {
	p, err := o.player()
	if err != nil {
		return nil, err
	}
	id, err := a.Int("faction_id")
	if err != nil {
		return nil, err
	}
	ok := false
	for _, f := range assignableFactions {
		ok = ok || f == id
	}
	if !ok {
		return nil, errors.New("faction must be Neutral, Atreides or Harkonnen")
	}
	r, _ := o.S.One(`select name from factions where id=?`, id)
	if len(r) == 0 {
		return nil, errors.New("this save has no such faction")
	}
	now := time.Now().UnixMicro()
	if _, err := o.S.Mutate(fmt.Sprintf("assign faction %s", factionLabel(fmt.Sprint(r["name"]))), func(m *save.Mut) error {
		_, e := m.Exec(`insert into player_faction(actor_id, faction_id, utc_time_faction_change) values(?,?,?)
			on conflict(actor_id) do update set faction_id=excluded.faction_id, utc_time_faction_change=excluded.utc_time_faction_change`, p.Controller, id, now)
		return e
	}); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "faction": factionLabel(fmt.Sprint(r["name"]))}, nil
}

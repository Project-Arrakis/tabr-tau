package ops

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// ---------------------------------------------------------------- the factions the Landsraad has (#657)
//
// The game's Landsraad is contested by two factions, Atreides (1) and Harkonnen (2). The save's factions table also holds None (the faction of a character with none) and Smuggler
// (4), which have no Landsraad rows in any save read: a house completed for either is a state the game never writes. "A faction the Landsraad has" is decided here and nowhere
// else: the list the page offers, the faction of the character, and every write that takes a faction id use this one rule.

// landsraadFactionIDs are the ids of the factions the Landsraad has.
var landsraadFactionIDs = []int64{1, 2}

// landsraadFactionCond is the test for a row of the factions table, aliased f, that is a faction the Landsraad has. It is built from landsraadFactionIDs, so the two cannot differ.
var landsraadFactionCond = func() string {
	ids := make([]string, len(landsraadFactionIDs))
	for i, id := range landsraadFactionIDs {
		ids[i] = strconv.FormatInt(id, 10)
	}
	return "f.id in (" + strings.Join(ids, ", ") + ")"
}()

// isLandsraadFaction says whether id is the id of a faction the Landsraad has.
func isLandsraadFaction(id int64) bool {
	for _, f := range landsraadFactionIDs {
		if f == id {
			return true
		}
	}
	return false
}

// errNotLandsraadFaction is the refusal of a faction the Landsraad does not have.
func errNotLandsraadFaction() error {
	return apierr.New(apierr.ErrBadArgument, "that faction is not one the Landsraad has")
}

// landsraadFactionName reads the name of a faction the Landsraad has, with the query of an edit (m.One) or of the save (S.One). A faction it does not have (None, Smuggler, an
// id the save has no row for) is refused as a bad argument, so every write that takes a faction id answers the same way.
func landsraadFactionName(one func(string, ...any) (save.Row, error), id int64) (string, error) {
	if !isLandsraadFaction(id) {
		return "", errNotLandsraadFaction()
	}
	f, err := one(`select f.name from factions f where f.id=? and `+landsraadFactionCond, id)
	if err != nil {
		return "", err
	}
	if f == nil {
		return "", errNotLandsraadFaction()
	}
	return fmt.Sprint(f["name"]), nil
}

// landsraadFactions lists the factions the Landsraad has that the save holds, for the page to offer.
func (o *Ops) landsraadFactions() ([]save.Row, error) {
	return o.S.Query(`select f.id, f.name from factions f where ` + landsraadFactionCond + ` order by f.id`)
}

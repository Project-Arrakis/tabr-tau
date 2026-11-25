package ops

import (
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Clearing the Deep Desert markers for a layout roll (#1216). A single-player Coriolis never wipes the markers table (a server's storm does), so after the layout is rolled the game's
// own map still shows the places of the earlier layouts (a ghost lab was seen on 2026-11-24). The roll therefore removes, for the Deep Desert ONLY:
//
//   - the rows of markers whose map is the Deep Desert (the places the game has put on the map),
//   - the rows of player_markers whose map is the Deep Desert (which of those places the character has discovered), and
//   - the rows of resourcefield_state whose map is the Deep Desert (the spice and resource fields the game keeps a state for).
//
// The game adds markers again as the character discovers places; that was checked in the game on 2026-11-24, on a save whose Deep Desert markers had been removed the same way (none
// left, then 43 appeared with the lab discovered). Nothing else is touched: not the markers of another map, not another map's resource fields, no actor, base, vehicle or inventory.
//
// The save names the map of a marker in one of two ways (save/markers.go): as text in map_name (until the game's update of early November 2026) or as map_name_id, a number that
// refers to map_names. The statements below use whichever the save has. They name the real tables; MarkersTable and PlayerMarkersTable are joined sub-selects for reading and cannot
// be deleted from.

// MarkerCounts is how many rows of the three tables there are for one set of maps.
type MarkerCounts struct {
	Markers        int64 `json:"markers"`        // rows of markers
	Discovered     int64 `json:"discovered"`     // rows of player_markers (what the character has discovered)
	ResourceFields int64 `json:"resourceFields"` // rows of resourcefield_state
}

// Total is every row counted.
func (c MarkerCounts) Total() int64 { return c.Markers + c.Discovered + c.ResourceFields }

// MarkerSurvey is the Deep Desert's rows and every other map's rows, counted in the same read.
type MarkerSurvey struct {
	DeepDesert MarkerCounts `json:"deepDesert"`
	Others     MarkerCounts `json:"others"`
}

// The statements are written out whole, one set per layout, so that none is put together from pieces. map_names is asked by name, so a save whose map_names has no Deep Desert row
// selects nothing for the Deep Desert (and every row for the others).
const (
	ddSub = `(select map_name_id from map_names where map_name='DeepDesert')`

	countDDNewer = `select (select count(*) from markers where map_name_id in ` + ddSub + `) m, (select count(*) from player_markers where map_name_id in ` + ddSub + `) p,
		(select count(*) from resourcefield_state where map='DeepDesert') r`
	countOtherNewer = `select (select count(*) from markers where map_name_id not in ` + ddSub + `) m, (select count(*) from player_markers where map_name_id not in ` + ddSub + `) p,
		(select count(*) from resourcefield_state where map is not 'DeepDesert') r`
	countDDOlder = `select (select count(*) from markers where map_name='DeepDesert') m, (select count(*) from player_markers where map_name='DeepDesert') p,
		(select count(*) from resourcefield_state where map='DeepDesert') r`
	countOtherOlder = `select (select count(*) from markers where map_name is not 'DeepDesert') m, (select count(*) from player_markers where map_name is not 'DeepDesert') p,
		(select count(*) from resourcefield_state where map is not 'DeepDesert') r`

	deletePlayerNewer  = `delete from player_markers where map_name_id in ` + ddSub
	deleteMarkersNewer = `delete from markers where map_name_id in ` + ddSub
	deletePlayerOlder  = `delete from player_markers where map_name='DeepDesert'`
	deleteMarkersOlder = `delete from markers where map_name='DeepDesert'`
	deleteFields       = `delete from resourcefield_state where map='DeepDesert'`
)

// surveyWith counts the rows with the given reader (the save for a read, the edit for the read inside it).
func surveyWith(one func(string, ...any) (save.Row, error), newer bool) (MarkerSurvey, error) {
	var out MarkerSurvey
	dd, other := countDDOlder, countOtherOlder
	if newer {
		dd, other = countDDNewer, countOtherNewer
	}
	for _, c := range []struct {
		into *MarkerCounts
		q    string
	}{{&out.DeepDesert, dd}, {&out.Others, other}} {
		r, err := one(c.q)
		if err != nil {
			return out, err
		}
		if r == nil {
			return out, fmt.Errorf("the marker tables could not be counted")
		}
		m, ok1 := toInt(r["m"])
		p, ok2 := toInt(r["p"])
		f, ok3 := toInt(r["r"])
		if !ok1 || !ok2 || !ok3 {
			return out, fmt.Errorf("the marker tables could not be counted (unexpected values)")
		}
		*c.into = MarkerCounts{Markers: m, Discovered: p, ResourceFields: f}
	}
	return out, nil
}

// SurveyDeepDesertMarkers counts what a roll would remove (the Deep Desert's rows) and what it would leave (every other map's), changing nothing.
func (o *Ops) SurveyDeepDesertMarkers() (MarkerSurvey, error) {
	return surveyWith(o.S.One, o.S.MarkersMapNameID())
}

// ClearDeepDesertMarkers removes the Deep Desert's rows of the three tables in one edit of the save (one transaction), reads the counts back inside the edit, and fails as a whole,
// changing nothing, unless the Deep Desert has no row left and every other map's count is what it was. It returns what the save held before.
func (o *Ops) ClearDeepDesertMarkers() (MarkerSurvey, error) {
	newer := o.S.MarkersMapNameID() // worked out before the edit starts: the save's own lock is held during an edit
	var before MarkerSurvey
	_, err := o.S.Mutate("", func(m *save.Mut) error {
		var err error
		if before, err = surveyWith(m.One, newer); err != nil {
			return err
		}
		if before.DeepDesert.Total() == 0 {
			return nil
		}
		// the discovered rows first: they refer to the markers
		stmts := []string{deletePlayerOlder, deleteMarkersOlder, deleteFields}
		if newer {
			stmts = []string{deletePlayerNewer, deleteMarkersNewer, deleteFields}
		}
		for _, q := range stmts {
			if _, err := m.Exec(q); err != nil {
				return err
			}
		}
		after, err := surveyWith(m.One, newer)
		if err != nil {
			return err
		}
		if err := checkCleared(before, after); err != nil {
			return err
		}
		m.Desc = fmt.Sprintf("removed the Deep Desert markers (%d markers, %d discovered, %d resource fields)", before.DeepDesert.Markers, before.DeepDesert.Discovered, before.DeepDesert.ResourceFields)
		return nil
	})
	if err != nil {
		return MarkerSurvey{}, err
	}
	return before, nil
}

// VerifyDeepDesertMarkersCleared reads the save as it is now and says whether it is as ClearDeepDesertMarkers meant it: the Deep Desert has no row of the three tables, and every other
// map has as many rows as before. The roll makes it on the file it has written, opened afresh.
func (o *Ops) VerifyDeepDesertMarkersCleared(before MarkerSurvey) error {
	after, err := surveyWith(o.S.One, o.S.MarkersMapNameID())
	if err != nil {
		return err
	}
	return checkCleared(before, after)
}

// checkCleared is the read-back rule.
func checkCleared(before, after MarkerSurvey) error {
	if after.DeepDesert.Total() != 0 {
		return fmt.Errorf("read back: the Deep Desert still has %d marker row(s), %d discovered row(s) and %d resource-field row(s)", after.DeepDesert.Markers, after.DeepDesert.Discovered, after.DeepDesert.ResourceFields)
	}
	if after.Others != before.Others {
		return fmt.Errorf("read back: the rows of the other maps changed (markers %d -> %d, discovered %d -> %d, resource fields %d -> %d)",
			before.Others.Markers, after.Others.Markers, before.Others.Discovered, after.Others.Discovered, before.Others.ResourceFields, after.Others.ResourceFields)
	}
	return nil
}

package ops

import (
	"fmt"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// Where the character is (#654). A save keeps the character in three actors: the controller, the body and the player state. The controller and the player state say where the
// player is; a save the game wrote can hold the body (and a vehicle it sits in) on another map, for instance the travel map, with a state that is not 0. Reading "the character's
// place" from the body alone then puts a vehicle where the player cannot walk to it. Every action that works from the character's place reads it here, inside its edit, and
// refuses "next to the character" when the three do not agree.

// characterPlace is what the save says about where the character is.
type characterPlace struct {
	Map  string   // the map the player is on: the controller's
	Dim  int64    // the dimension the player is in: the controller's, else the body's
	Body save.Row // the body's row (map, x, y, z, rx, ry, rz, rw, d, state); nil when it was not found
	Ctl  save.Row // the controller's row, same columns; nil when it was not found
	// Parked is nil when the controller, the body and the player state are on one map and the body's state is 0, so the body's position is the player's.
	// Otherwise it is the plain sentence to refuse "next to the character" with (a kind of ErrBadArgument): the body is somewhere the player is not.
	Parked error
	// ParkedMap is true when Parked is only about the three actors being on different maps (the body's state is 0); a move that writes all three actors to a map the person
	// named puts them on one map, so it may go on. A body whose state is not 0 is not that case.
	ParkedMap bool
}

// pos gives the player's position: the body's when the three agree (the editor's same-map move writes the body only, so the body is the newer of the two), the controller's
// when they do not.
func (c characterPlace) pos() (x, y float64) {
	r := c.Body
	if c.Parked != nil && c.Ctl != nil {
		r = c.Ctl
	}
	return num(r["x"]), num(r["y"])
}

const characterActorCols = `map, location_x x, location_y y, location_z z, rotation_x rx, rotation_y ry, rotation_z rz, rotation_w rw, dimension_index d, state st`

// characterMapCols is the same without the position, for a move that does not depend on where the character stands: an edit put back onto a newer file is then held only
// when the character's map or state changed, not each time it moved.
const characterMapCols = `map, dimension_index d, state st`

// readCharacterPlace reads the controller, the body and the player state through one (the edit's reader, m.One, inside an edit, so the reads are kept with it). withPose false
// leaves the position and the facing out of the reads. The answer's error is a failed read; a character that cannot be placed is in Parked.
func readCharacterPlace(one func(string, ...any) (save.Row, error), p player, withPose bool) (characterPlace, error) {
	cols := characterMapCols
	if withPose {
		cols = characterActorCols
	}
	var c characterPlace
	var err error
	if c.Ctl, err = one(`select `+cols+` from actors where id=?`, p.Controller); err != nil {
		return c, err
	}
	if p.Pawn != 0 {
		if c.Body, err = one(`select `+cols+` from actors where id=?`, p.Pawn); err != nil {
			return c, err
		}
	}
	st, err := one(`select a.map map from player_state ps join actors a on a.id=ps.player_state_id where ps.id=?`, p.Controller)
	if err != nil {
		return c, err
	}
	refuse := func(format string, a ...any) { c.Parked = apierr.Newf(apierr.ErrBadArgument, format, a...) }
	mapOf := func(r save.Row) string {
		if r == nil || r["map"] == nil {
			return ""
		}
		return fmt.Sprint(r["map"])
	}
	c.Map = mapOf(c.Ctl)
	if n, ok := toInt(c.Ctl["d"]); ok && n >= 0 {
		c.Dim = n
	} else if n, ok := toInt(c.Body["d"]); ok && n >= 0 {
		c.Dim = n
	}
	switch {
	case c.Ctl == nil:
		refuse("the character's controller was not found in this save, so the editor cannot tell where the character is. Nothing was changed.")
	case c.Body == nil:
		refuse("the character's body was not found in this save. Nothing was changed.")
	case c.Map == "":
		refuse("the character's map is not recorded in this save. Nothing was changed.")
	case mapOf(c.Body) != c.Map || (st != nil && mapOf(st) != c.Map):
		c.ParkedMap = true
		refuse("this save holds the character's body on another map than the one the player is on (the body is on %s, the player on %s), so the editor does not put anything next to the character: it would end up where you cannot reach it. Nothing was changed.", orNone(mapOf(c.Body)), c.Map)
	default:
		if n, _ := toInt(c.Body["st"]); n != 0 {
			refuse("the character's body is not standing in the world in this save (its state is %d), so the editor does not put anything next to the character. Nothing was changed.", n)
		}
	}
	return c, nil
}

func orNone(s string) string {
	if s == "" {
		return "no map"
	}
	return s
}

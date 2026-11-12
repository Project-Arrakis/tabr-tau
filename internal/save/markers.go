package save

import "database/sql"

// The game changed how its save names the map of a marker (#588). Until its update of early November 2026, markers and player_markers carried the map as text, in the column
// map_name. Since then they carry map_name_id, an integer that refers to the new table map_names (map_name_id, map_name). The editor reads saves of both, and decides from the
// columns of markers when the save is loaded, never from a game version: a save is of the newer layout when markers has map_name_id and no map_name.
//
// A query that wants the map's text takes the table from MarkersTable or PlayerMarkersTable instead of writing "markers" or "player_markers". Both give every column of the
// table under its own name, and map_name besides in the newer layout, so the rest of the query reads the same for either layout.

// markersHaveMapNameID reports whether db has the newer layout of markers. A database with neither column (or without a markers table) is not of the newer layout; a query on it
// fails on its own and says what is missing.
func markersHaveMapNameID(db interface {
	Query(q string, args ...any) (*sql.Rows, error)
}) bool {
	rs, err := db.Query(`select name from pragma_table_info('markers')`)
	if err != nil {
		return false
	}
	defer rs.Close()
	var hasID, hasName bool
	for rs.Next() {
		var n string
		if rs.Scan(&n) != nil {
			return false
		}
		hasID = hasID || n == "map_name_id"
		hasName = hasName || n == "map_name"
	}
	return hasID && !hasName && rs.Err() == nil
}

// MarkersMapNameID reports whether the save is of the game's newer layout (markers.map_name_id and the map_names table).
func (s *Save) MarkersMapNameID() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mapNameIDs
}

// MarkersTable is the SQL table expression to use for the game's map markers in a query: "markers" itself in the older layout, and in the newer one the same rows with the
// column map_name added from map_names. Put an alias after it: "from " + s.MarkersTable() + " m".
func (s *Save) MarkersTable() string {
	if s.MarkersMapNameID() {
		return `(select m0.*, mn.map_name from markers m0 join map_names mn on mn.map_name_id=m0.map_name_id)`
	}
	return `markers`
}

// PlayerMarkersTable is MarkersTable for player_markers: which markers a character has discovered, with map_name in either layout.
func (s *Save) PlayerMarkersTable() string {
	if s.MarkersMapNameID() {
		return `(select p0.*, mn.map_name from player_markers p0 join map_names mn on mn.map_name_id=p0.map_name_id)`
	}
	return `player_markers`
}

package basefile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Reading a base file in order to put it into a save (#475, phase 2). Check (basefile.go) has already said the file is shaped like a base file. Load goes on to turn every cell into
// a typed value and Verify to check what only the whole file can tell: that every id a row names is the id of a row of the file, that no id is used twice, that the text is plain.
// Nothing here knows a save. Nothing from the file is ever used as an id in a save: the importer (internal/ops) gives every actor, inventory, item and entity a new one.

// JSONText is a JSON cell, compacted. A Text cell is a plain string and an Int cell an int64, so the kind of a cell can be told from its Go type.
type JSONText string

// Rows is the rows of one table with the cells in the order of Table.Cols: int64, float64, string, JSONText, or nil for NULL.
type Rows struct {
	Table Table
	R     [][]any
}

// Col is the index of a column of the table. A name that is not a column is a mistake in the program, not in a file.
func (r *Rows) Col(name string) int {
	for i, c := range r.Table.Cols {
		if c.Name == name {
			return i
		}
	}
	panic("basefile: no column " + name + " in " + r.Table.Name)
}

// Base is a file that passed Check and Load: its header, and every table's rows, typed.
type Base struct {
	File *File
	Rows map[string]*Rows
	Hash string // SHA-256 of the file's bytes, as text
}

// Of is the rows of a table.
func (b *Base) Of(name string) *Rows { return b.Rows[name] }

// Hash is the fingerprint of a file's text, which a preview gives and an import must be given back, so that the file imported is the file that was previewed.
func Hash(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

// Load checks a file (Check, with these limits) and reads its rows into typed values. It refuses text that is not valid UTF-8, a text cell with a control character in it, a JSON cell
// that holds a NUL and a number that does not fit its column. It does not look at what the rows mean; Verify does.
func Load(data []byte, maxBytes, maxRows int) (*Base, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("this is not a base file: it is not valid UTF-8 text")
	}
	f, tabs, err := check(data, maxBytes, maxRows)
	if err != nil {
		return nil, err
	}
	b := &Base{File: f, Rows: map[string]*Rows{}, Hash: Hash(data)}
	for _, t := range Tables {
		rs := &Rows{Table: t, R: make([][]any, len(tabs[t.Name]))}
		for i, row := range tabs[t.Name] {
			vals := make([]any, len(t.Cols))
			for j, c := range t.Cols {
				v, err := c.typed(row[c.Name])
				if err != nil {
					return nil, fmt.Errorf("row %d of %s: %w", i+1, t.Name, err)
				}
				vals[j] = v
			}
			rs.R[i] = vals
		}
		b.Rows[t.Name] = rs
	}
	return b, nil
}

// typed is the value of a cell Check has already passed.
func (c Column) typed(cell json.RawMessage) (any, error) {
	s := strings.TrimSpace(string(cell))
	if s == "null" {
		return nil, nil
	}
	switch c.Kind {
	case Int:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s is not a whole number", c.Name)
		}
		return n, nil
	case Real:
		x, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("%s is not a finite number", c.Name)
		}
		return x, nil
	case Text:
		var v string
		if err := json.Unmarshal(cell, &v); err != nil {
			return nil, fmt.Errorf("%s is not text", c.Name)
		}
		for _, r := range v {
			if r < 0x20 || r == 0x7f {
				return nil, fmt.Errorf("%s holds a control character", c.Name)
			}
		}
		return v, nil
	case JSON:
		if bytes.Contains(cell, []byte(`\u0000`)) {
			return nil, fmt.Errorf("%s holds a NUL character", c.Name)
		}
		var b bytes.Buffer
		if err := json.Compact(&b, cell); err != nil {
			return nil, fmt.Errorf("%s is not valid JSON", c.Name)
		}
		return JSONText(b.String()), nil
	}
	return nil, fmt.Errorf("%s has no kind", c.Name)
}

// Names a file may use for classes, maps and templates: letters, digits and the few marks the game's own names use. A name outside this is not one of the game's.
var (
	classPattern    = regexp.MustCompile(`^[A-Za-z0-9_/.:\-]{1,300}$`)
	mapPattern      = regexp.MustCompile(`^[A-Za-z0-9_.\-]{1,80}$`)
	namePattern     = regexp.MustCompile(`^[A-Za-z0-9_]{1,120}$`)
	playerClassPart = regexp.MustCompile(`(?i)player(controller|character|state|pawn)`)
)

// MaxText is the most characters one text cell may hold (the names a player types are far shorter).
const MaxText = 1000

// The actor states a base file may hold: 5 (picked up, what the game's tool leaves) and 0 (an actor that was still standing when it was exported).
const (
	StateStanding = 0
	StateBackup   = 5
)

// required lists the columns of each table that the game's schema declares NOT NULL (or that a base cannot do without). A row with a NULL there is refused.
var required = map[string][]string{
	"base_backups":              {"id"},
	"base_backup_linked_actors": {"id", "actor_id"},
	"actors":                    {"id", "class", "map", "location_x", "location_y", "location_z", "rotation_x", "rotation_y", "rotation_z", "rotation_w", "dimension_index", "gas_attributes", "properties", "serial", "state"},
	"fgl_entities":              {"entity_id"},
	"actor_fgl_entities":        {"actor_id", "entity_id", "slot_name"},
	"permission_actor":          {"actor_id", "actor_type", "access_level", "is_child"},
	"permission_actor_rank":     {"permission_actor_id", "player_id", "rank"},
	"inventories":               {"id", "actor_id"},
	"items":                     {"id", "inventory_id", "stack_size", "position_index", "template_id", "acquisition_time", "stats", "quality_level"},
	"actor_inventories":         {"inventory_id", "component_name_hash"},
	"buildings":                 {"id"},
	"building_instances": {"building_id", "instance_id", "building_type", "location_x", "location_y", "location_z", "rotation_x", "rotation_y", "rotation_z", "rotation_w",
		"health", "shelter", "sand_buildup", "last_placed_by_player_id"},
	"placeables":         {"id", "building_type", "has_hit_ground", "has_buildable_support", "is_hologram", "last_placed_by_player_id"},
	"totems":             {"id"},
	"landclaim_segments": {"totem_id", "grid_location_x", "grid_location_y"},
}

// Verify checks what only the whole file can tell. It refuses, with the row and the reason: a NULL where the game's schema has NOT NULL; an id that is not positive where the game
// wants one; an id used by two rows; a row that names an id that is not a row of the file (every reference of a base points inside the base; the references inside JSON are the
// importer's, see RewriteRefs); an actor of another state than 0 or 5; an actor class that is not a name of the game's or is a player's; text that is not plain; a Solido blueprint
// in a storage (its rows are not carried); and a file that is not one base backup with all its actors.
func (b *Base) Verify() error {
	for _, t := range Tables {
		rs := b.Rows[t.Name]
		for _, name := range required[t.Name] {
			ci := rs.Col(name)
			for i, r := range rs.R {
				if r[ci] == nil {
					return fmt.Errorf("row %d of %s has no %s", i+1, t.Name, name)
				}
			}
		}
		for ci, c := range t.Cols {
			if c.Kind != Text {
				continue
			}
			for i, r := range rs.R {
				if s, ok := r[ci].(string); ok && utf8.RuneCountInString(s) > MaxText {
					return fmt.Errorf("row %d of %s: %s is longer than %d characters", i+1, t.Name, c.Name, MaxText)
				}
			}
		}
	}
	// Ids: each table's own key is unique, and positive where the game's schema says so.
	set := func(table, col string, positive bool) (map[int64]bool, error) {
		rs := b.Rows[table]
		ci := rs.Col(col)
		m := make(map[int64]bool, len(rs.R))
		for i, r := range rs.R {
			n, ok := r[ci].(int64)
			if !ok {
				continue // a NULL; the caller decides if that is allowed
			}
			if positive && n <= 0 {
				return nil, fmt.Errorf("row %d of %s: %s must be a positive number", i+1, table, col)
			}
			if m[n] {
				return nil, fmt.Errorf("row %d of %s: %s %d is used by two rows", i+1, table, col, n)
			}
			m[n] = true
		}
		return m, nil
	}
	var err error
	var actors, entities, invs, perm, buildings, totems map[int64]bool
	if _, err = set("base_backups", "id", true); err != nil {
		return err
	}
	if n := len(b.Rows["base_backups"].R); n != 1 {
		return fmt.Errorf("a base file holds one base backup; this one holds %d", n)
	}
	if actors, err = set("actors", "id", true); err != nil {
		return err
	}
	if len(actors) == 0 {
		return fmt.Errorf("the file has no actors, so there is no base in it")
	}
	if entities, err = set("fgl_entities", "entity_id", false); err != nil {
		return err
	}
	for _, e := range b.Rows["fgl_entities"].R {
		if e[0].(int64) == 0 {
			return fmt.Errorf("fgl_entities: entity_id must not be 0")
		}
	}
	if invs, err = set("inventories", "id", true); err != nil {
		return err
	}
	if _, err = set("items", "id", true); err != nil {
		return err
	}
	if perm, err = set("permission_actor", "actor_id", false); err != nil {
		return err
	}
	if buildings, err = set("buildings", "id", false); err != nil {
		return err
	}
	if _, err = set("placeables", "id", false); err != nil {
		return err
	}
	if totems, err = set("totems", "id", false); err != nil {
		return err
	}
	if _, err = set("actor_inventories", "inventory_id", false); err != nil {
		return err
	}

	// A reference column: every value is NULL (when allowed) or the id of a row of the file.
	ref := func(table, col string, in map[int64]bool, target string, nullOK bool) error {
		rs := b.Rows[table]
		ci := rs.Col(col)
		for i, r := range rs.R {
			n, ok := r[ci].(int64)
			if !ok {
				if nullOK {
					continue
				}
				return fmt.Errorf("row %d of %s has no %s", i+1, table, col)
			}
			if !in[n] {
				return fmt.Errorf("row %d of %s names %s %d that is not in the file", i+1, table, target, n)
			}
		}
		return nil
	}
	backup := b.Rows["base_backups"].R[0][b.Rows["base_backups"].Col("id")].(int64)
	linked := map[int64]bool{}
	{
		rs := b.Rows["base_backup_linked_actors"]
		ic, ac := rs.Col("id"), rs.Col("actor_id")
		for i, r := range rs.R {
			if r[ic].(int64) != backup {
				return fmt.Errorf("row %d of base_backup_linked_actors belongs to backup %d, not to the backup of the file", i+1, r[ic].(int64))
			}
			a := r[ac].(int64)
			if !actors[a] {
				return fmt.Errorf("row %d of base_backup_linked_actors names actor %d that is not in the file", i+1, a)
			}
			if linked[a] {
				return fmt.Errorf("row %d of base_backup_linked_actors: actor %d is linked twice", i+1, a)
			}
			linked[a] = true
		}
	}
	for a := range actors {
		if !linked[a] {
			return fmt.Errorf("actor %d is in the file but not linked to its base backup", a)
		}
	}
	// the entities: each is linked to exactly one actor of the file (an entity with no actor is what the game's own clean-up deletes)
	{
		rs := b.Rows["actor_fgl_entities"]
		if err := ref("actor_fgl_entities", "actor_id", actors, "actor", false); err != nil {
			return err
		}
		if err := ref("actor_fgl_entities", "entity_id", entities, "entity", false); err != nil {
			return err
		}
		ec := rs.Col("entity_id")
		seen := map[int64]bool{}
		for i, r := range rs.R {
			e := r[ec].(int64)
			if seen[e] {
				return fmt.Errorf("row %d of actor_fgl_entities: entity %d is linked to two actors", i+1, e)
			}
			seen[e] = true
			if s := r[rs.Col("slot_name")].(string); !namePattern.MatchString(s) {
				return fmt.Errorf("row %d of actor_fgl_entities: the slot name is not one of the game's names", i+1)
			}
		}
		for e := range entities {
			if !seen[e] {
				return fmt.Errorf("entity %d is in the file but linked to no actor", e)
			}
		}
	}
	for _, c := range []struct {
		table, col string
		in         map[int64]bool
		target     string
		nullOK     bool
	}{
		{"permission_actor", "actor_id", actors, "actor", false},
		{"permission_actor_rank", "permission_actor_id", perm, "permission record", false},
		{"inventories", "actor_id", actors, "actor", false},
		{"items", "inventory_id", invs, "inventory", false},
		{"actor_inventories", "inventory_id", invs, "inventory", false},
		{"buildings", "id", actors, "actor", false},
		{"building_instances", "building_id", buildings, "building", false},
		{"building_instances", "owner_entity_id", entities, "entity", true},
		{"placeables", "id", actors, "actor", false},
		{"placeables", "owner_entity_id", entities, "entity", true},
		{"totems", "id", actors, "actor", false},
		{"landclaim_segments", "totem_id", totems, "totem", false},
	} {
		if err := ref(c.table, c.col, c.in, c.target, c.nullOK); err != nil {
			return err
		}
	}
	// pairs that are keys in the game's schema
	if err := uniquePairs(b, "building_instances", "building_id", "instance_id"); err != nil {
		return err
	}
	if err := uniquePairs(b, "permission_actor_rank", "permission_actor_id", "player_id"); err != nil {
		return err
	}
	{
		rs := b.Rows["landclaim_segments"]
		a, c1, c2 := rs.Col("totem_id"), rs.Col("grid_location_x"), rs.Col("grid_location_y")
		seen := map[[3]int64]bool{}
		for i, r := range rs.R {
			k := [3]int64{r[a].(int64), r[c1].(int64), r[c2].(int64)}
			if seen[k] {
				return fmt.Errorf("row %d of landclaim_segments repeats a segment of its totem", i+1)
			}
			seen[k] = true
		}
	}

	// The actors: a class of the game's names, a map, a state a base can be in.
	{
		rs := b.Rows["actors"]
		cc, mc, sc := rs.Col("class"), rs.Col("map"), rs.Col("state")
		for i, r := range rs.R {
			cl := r[cc].(string)
			if !classPattern.MatchString(cl) {
				return fmt.Errorf("row %d of actors: the class is not one of the game's names", i+1)
			}
			if playerClassPart.MatchString(cl) {
				return fmt.Errorf("row %d of actors: this is a player's actor (%s), not part of a base", i+1, clip(cl))
			}
			if !mapPattern.MatchString(r[mc].(string)) {
				return fmt.Errorf("row %d of actors: the map is not one of the game's names", i+1)
			}
			if st := r[sc].(int64); st != StateBackup && st != StateStanding {
				return fmt.Errorf("row %d of actors: state %d is not the state of a base actor (picked up is %d)", i+1, st, StateBackup)
			}
			if r[rs.Col("dimension_index")].(int64) < 0 {
				return fmt.Errorf("row %d of actors: the dimension is negative", i+1)
			}
		}
	}
	// the names of building types and item templates
	for _, c := range []struct{ table, col string }{{"building_instances", "building_type"}, {"placeables", "building_type"}, {"items", "template_id"}} {
		rs := b.Rows[c.table]
		ci := rs.Col(c.col)
		for i, r := range rs.R {
			if s, ok := r[ci].(string); ok && !namePattern.MatchString(s) {
				return fmt.Errorf("row %d of %s: %s is not one of the game's names", i+1, c.table, c.col)
			}
		}
	}
	{
		rs := b.Rows["items"]
		tc, sc, pc := rs.Col("template_id"), rs.Col("stack_size"), rs.Col("position_index")
		for i, r := range rs.R {
			if strings.EqualFold(r[tc].(string), "buildingblueprint_copydevice") {
				return fmt.Errorf("row %d of items is a Solido blueprint: its rows are not carried in a base file yet", i+1)
			}
			if r[sc].(int64) <= 0 || r[pc].(int64) < 0 {
				return fmt.Errorf("row %d of items has a stack size or a slot the game does not allow", i+1)
			}
		}
	}
	return nil
}

func uniquePairs(b *Base, table, c1, c2 string) error {
	rs := b.Rows[table]
	a, c := rs.Col(c1), rs.Col(c2)
	seen := map[[2]int64]bool{}
	for i, r := range rs.R {
		k := [2]int64{r[a].(int64), r[c].(int64)}
		if seen[k] {
			return fmt.Errorf("row %d of %s repeats a row: %s and %s are the key", i+1, table, c1, c2)
		}
		seen[k] = true
	}
	return nil
}

// blobJSON are the JSON columns the game stores as jsonb (a BLOB); the others hold JSON as text. A value is written to the first through jsonb(?).
var blobJSON = map[string]bool{"actors.gas_attributes": true, "actors.properties": true, "fgl_entities.components": true}

// InsertSQL is the statement that adds one row of the table, with the columns of Cols in order. The statement is built from the list of tables and columns in this package, never
// from anything in a file or a request.
func (t Table) InsertSQL() string {
	names := make([]string, len(t.Cols))
	marks := make([]string, len(t.Cols))
	for i, c := range t.Cols {
		names[i] = c.Name
		marks[i] = "?"
		if c.Kind == JSON && blobJSON[t.Name+"."+c.Name] {
			marks[i] = "jsonb(?)"
		}
	}
	return "insert into " + t.Name + "(" + strings.Join(names, ",") + ") values (" + strings.Join(marks, ",") + ")"
}

// Arg is a cell as a statement argument: JSON text goes in as a string.
func Arg(v any) any {
	if j, ok := v.(JSONText); ok {
		return string(j)
	}
	return v
}

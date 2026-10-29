// Package basefile is the file the editor writes for a base taken out of a save: format "tabr-tau-base", version 1, built like the care package kit file (internal/kits): one
// JSON object that names its format and version, carries no path, no program and no setting of the editor, and is read strictly. A file is the rows of the tables the game keeps a
// base in, with their original ids, and a manifest of what is in it: the counts per table, facts about the save it came from that are not private, and a checksum.
//
// Every table and column that can be in the file is listed in Tables. No table or column name ever comes from a file or a request: a query is built from this list only.
package basefile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// Format and Version name the file. A file of another format or another version is refused.
	Format  = "tabr-tau-base"
	Version = 1

	// KindBackup is a base that the game's own tool picked up (a row in base_backups and its linked actors, state 5).
	KindBackup = "base-backup"

	// MaxFileBytes is the largest file the editor writes or reads. A base of 1,400 pieces is well under 2 MiB; the Dune Docker console caps an upload at 32 MiB.
	MaxFileBytes = 16 << 20
	// MaxRows is the most rows a file may hold, over all tables (the console uses the same number of entries).
	MaxRows = 250000
)

// ChecksumOf names what the checksum covers, so a reader does not have to guess.
const (
	ChecksumAlgorithm = "sha256"
	ChecksumOf        = "tables, with the white space between JSON tokens removed"
)

// Kind is how a column is written in the file.
type Kind int

const (
	Int  Kind = iota // a whole number, any 64-bit value; the file holds it as an integer literal, so an id above 2^53 is exact
	Real             // a finite number
	Text             // a string
	JSON             // a JSON value, written as JSON (a column the game stores as jsonb, or as text that holds JSON)
)

// Column is one column of a table in the file.
type Column struct {
	Name string
	Kind Kind
}

// Table is one table of the file: its columns in file order, the rows that belong to a base backup (Scope, in the tokens below) and the order the rows are written in.
type Table struct {
	Name  string
	Cols  []Column
	Scope string
	Order string
}

// The tokens of a scope. {B} is the backup's id; {A} the actors linked to the backup; {E} the entities of those actors; {V} the inventories of those actors.
const (
	scopeA = `(select actor_id from base_backup_linked_actors where id={B})`
	scopeE = `(select entity_id from actor_fgl_entities where actor_id in ` + scopeA + `)`
	scopeV = `(select id from inventories where actor_id in ` + scopeA + `)`
)

func ints(names ...string) []Column {
	out := make([]Column, len(names))
	for i, n := range names {
		out[i] = Column{n, Int}
	}
	return out
}

func cols(parts ...[]Column) []Column {
	var out []Column
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func one(name string, k Kind) []Column { return []Column{{name, k}} }

// Tables is every table a base file holds, in the order they are written. The columns left out are the ones that must not travel: actors.owner_account_id (an account of the save),
// and inventories.item_id and vehicle_module_id (an inventory of a base belongs to an actor; a base with an inventory that belongs to an item is not exported, see ops).
var Tables = []Table{
	{"base_backups", cols(ints("id", "player_id"), one("base_backup_name", Text), ints("last_edited_by_player_id")), `id={B}`, `id`},
	{"base_backup_linked_actors", ints("id", "actor_id"), `id={B}`, `actor_id`},
	{"actors", cols(ints("id"), one("class", Text), one("map", Text), []Column{{"location_x", Real}, {"location_y", Real}, {"location_z", Real},
		{"rotation_x", Real}, {"rotation_y", Real}, {"rotation_z", Real}, {"rotation_w", Real}}, ints("dimension_index"),
		one("gas_attributes", JSON), one("properties", JSON), ints("serial", "state")), `id in ` + scopeA, `id`},
	{"fgl_entities", cols(ints("entity_id"), one("components", JSON)), `entity_id in ` + scopeE, `entity_id`},
	{"actor_fgl_entities", cols(ints("actor_id", "entity_id"), one("slot_name", Text)), `actor_id in ` + scopeA, `actor_id, entity_id`},
	{"permission_actor", cols(ints("actor_id"), one("actor_name", Text), ints("actor_type", "access_level", "is_child", "edited_by_player_id")), `actor_id in ` + scopeA, `actor_id`},
	{"permission_actor_rank", ints("permission_actor_id", "player_id", "rank"), `permission_actor_id in ` + scopeA, `permission_actor_id, player_id`},
	{"inventories", cols(ints("id", "actor_id", "inventory_type", "max_item_count"), one("max_item_volume", Real)), `actor_id in ` + scopeA, `id`},
	{"items", cols(ints("id", "inventory_id", "stack_size", "position_index"), one("template_id", Text), ints("is_new", "acquisition_time"), one("stats", JSON),
		ints("quality_level"), one("volume_override", Real)), `inventory_id in ` + scopeV, `id`},
	{"actor_inventories", ints("inventory_id", "component_name_hash"), `inventory_id in ` + scopeV, `inventory_id`},
	{"buildings", ints("id", "owner_id"), `id in ` + scopeA, `id`},
	{"building_instances", cols(ints("building_id", "instance_id"), one("building_type", Text), []Column{{"location_x", Real}, {"location_y", Real}, {"location_z", Real},
		{"rotation_x", Real}, {"rotation_y", Real}, {"rotation_z", Real}, {"rotation_w", Real}}, ints("owner_entity_id", "building_flags"), one("health", Real),
		ints("shelter", "sand_buildup", "last_placed_by_player_id")), `building_id in ` + scopeA, `building_id, instance_id`},
	{"placeables", cols(ints("id", "owner_entity_id"), one("health", Real), one("building_type", Text), ints("has_hit_ground", "has_buildable_support", "is_hologram", "last_placed_by_player_id")),
		`id in ` + scopeA, `id`},
	{"totems", cols(ints("id", "landclaim_vertical_level", "last_backup_timestamp"), []Column{{"landclaim_original_global_location_x", Real}, {"landclaim_original_global_location_y", Real},
		{"landclaim_original_global_location_z", Real}, {"landclaim_original_global_yaw_rotation", Real}}), `id in ` + scopeA, `id`},
	{"landclaim_segments", ints("totem_id", "grid_location_x", "grid_location_y"), `totem_id in ` + scopeA, `totem_id, grid_location_x, grid_location_y`},
}

// Find is the table called name, or false.
func Find(name string) (Table, bool) {
	for _, t := range Tables {
		if t.Name == name {
			return t, true
		}
	}
	return Table{}, false
}

// ScopeSQL is the where-clause of a table for one backup. The id is a number made here, never text from a request.
func (t Table) ScopeSQL(backup int64) string {
	return strings.ReplaceAll(t.Scope, "{B}", strconv.FormatInt(backup, 10))
}

// The marks a select puts in place of a value that cannot be written. They are not valid JSON, so they cannot be mistaken for a value.
const (
	markNonFinite   = "__nonfinite__"
	markInvalidJSON = "__invalid_json__"
)

// Select is the SQL expression that reads the column for the file, named as the column. A number that is not finite and JSON that is not valid come back as a mark that Value refuses.
func (c Column) Select() string {
	n := c.Name
	switch c.Kind {
	case Real:
		return `case when abs(` + n + `) > 1.7976931348623157e308 then '` + markNonFinite + `' else ` + n + ` end as ` + n
	case JSON:
		return `case when ` + n + ` is null then null when json_valid(` + n + `, 8) then json(` + n + `) when typeof(` + n + `) = 'text' and json_valid(` + n + `) then json(` + n + `) else '` + markInvalidJSON + `' end as ` + n
	}
	return n
}

// SelectList is the columns of the table as the select writes them.
func (t Table) SelectList() string {
	parts := make([]string, len(t.Cols))
	for i, c := range t.Cols {
		parts[i] = c.Select()
	}
	return strings.Join(parts, ", ")
}

// Value is the text of one cell of a row as the file holds it. v is what the query returned for the column. A JSON cell is compacted so that the same data always gives the same
// bytes; text must be valid UTF-8 (an invalid sequence would be changed on its way into JSON), a number must be finite.
func (c Column) Value(v any) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	switch c.Kind {
	case Int:
		n, ok := v.(int64)
		if !ok {
			return nil, fmt.Errorf("%s holds a value that is not a whole number", c.Name)
		}
		return strconv.AppendInt(nil, n, 10), nil
	case Real:
		switch x := v.(type) {
		case float64:
			if math.IsNaN(x) || math.IsInf(x, 0) {
				return nil, fmt.Errorf("%s holds a number that is not finite", c.Name)
			}
			return json.Marshal(x)
		case int64:
			return json.Marshal(float64(x))
		case string:
			if x == markNonFinite {
				return nil, fmt.Errorf("%s holds a number that is not finite", c.Name)
			}
		}
		return nil, fmt.Errorf("%s holds a value that is not a number", c.Name)
	case Text:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s holds a value that is not text", c.Name)
		}
		return jsonString(s, c.Name)
	case JSON:
		s, ok := v.(string)
		if !ok || s == markInvalidJSON {
			return nil, fmt.Errorf("%s holds data that is not valid JSON", c.Name)
		}
		var b bytes.Buffer
		if !utf8.ValidString(s) || json.Compact(&b, []byte(s)) != nil {
			return nil, fmt.Errorf("%s holds data that is not valid JSON", c.Name)
		}
		return b.Bytes(), nil
	}
	return nil, fmt.Errorf("%s has no kind", c.Name)
}

func jsonString(s, col string) ([]byte, error) {
	if !utf8.ValidString(s) {
		return nil, fmt.Errorf("%s holds text that is not valid UTF-8", col)
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimRight(b.Bytes(), "\n"), nil
}

// Row is one row of a table as a JSON object with the columns in file order. vals is in the order of t.Cols.
func (t Table) Row(vals []any) ([]byte, error) {
	if len(vals) != len(t.Cols) {
		return nil, errors.New("a row does not have the columns of its table")
	}
	var b bytes.Buffer
	b.WriteByte('{')
	for i, c := range t.Cols {
		cell, err := c.Value(vals[i])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Name, err)
		}
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Quote(c.Name))
		b.WriteByte(':')
		b.Write(cell)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// Manifest says what is in the file, so that a person (and a reader of the file) can check it without reading the rows.
type Manifest struct {
	Backup     BackupInfo     `json:"backup"`
	Summary    Summary        `json:"summary"`
	Counts     map[string]int `json:"counts"`
	Source     Source         `json:"source"`
	References []Reference    `json:"references"`
	NotCarried NotCarried     `json:"notCarried"`
	Warnings   []string       `json:"warnings"`
	Checksum   Checksum       `json:"checksum"`
}

// BackupInfo is the backup as the save names it. The id is the one in the save it came from.
type BackupInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Summary is the numbers the Bases tab lists, counted from the rows of the file.
type Summary struct {
	Actors      int `json:"actors"`
	Pieces      int `json:"pieces"`
	Placeables  int `json:"placeables"`
	StoredItems int `json:"storedItems"`
	Entities    int `json:"entities"`
	Rows        int `json:"rows"`
}

// Source is what is known about the save the file came from, and none of it is private: the maps of the actors, the states they are in (5 is picked up), and the game's applied
// patches (how many, a checksum of their names, and the latest few), which tell whether two saves are of the same game version.
type Source struct {
	Maps        []string       `json:"maps"`
	ActorStates map[string]int `json:"actorStates"`
	Patches     Patches        `json:"patches"`
}

// Patches is a summary of the save's applied_patches table.
type Patches struct {
	Count    int      `json:"count"`
	Checksum string   `json:"checksum"`
	Latest   []string `json:"latest"`
}

// Reference counts the places where a row or a JSON value of the base names an id: inside the base (the id is in the file), outside (it is not) or null (the null reference, 0).
// Kind is "act", "itm", "inv", "fgl" or "bbp" for the strings "!!act#12" that the game writes inside JSON, and "ownerEntity" for the owner_entity_id column of the pieces and placeables.
type Reference struct {
	Kind    string `json:"kind"`
	Inside  int    `json:"inside"`
	Outside int    `json:"outside"`
	Null    int    `json:"null"`
}

// NotCarried counts what the save holds for the base that the file does not.
type NotCarried struct {
	InventoriesOfStoredItems int `json:"inventoriesOfStoredItems"` // inventories that belong to an item stored in the base (a tool's slots, a bag)
	ItemsInThem              int `json:"itemsInThem"`              // and the items those inventories hold
}

// Checksum covers the tables of the file.
type Checksum struct {
	Algorithm string `json:"algorithm"`
	Of        string `json:"of"`
	Value     string `json:"value"`
}

// File is the whole file, as a reader sees it.
type File struct {
	Format     string          `json:"format"`
	Version    int             `json:"version"`
	Kind       string          `json:"kind"`
	ExportedAt string          `json:"exportedAt"`
	Editor     string          `json:"editor"`
	Manifest   Manifest        `json:"manifest"`
	Tables     json.RawMessage `json:"tables"`
}

// TablesChecksum is the checksum of the tables part of a file: SHA-256 of its JSON with the white space between tokens removed, so that the layout of the file (the writer puts one
// row on a line) is not part of what is checked.
func TablesChecksum(tables []byte) (string, error) {
	var b bytes.Buffer
	if err := json.Compact(&b, tables); err != nil {
		return "", err
	}
	sum := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

// Assemble is the file: the header fields, the manifest (with the checksum of tables filled in) and the tables, laid out for a person to read. tables is the JSON object of the
// tables, already built.
func Assemble(kind string, at time.Time, editor string, m Manifest, tables []byte) ([]byte, error) {
	sum, err := TablesChecksum(tables)
	if err != nil {
		return nil, err
	}
	m.Checksum = Checksum{Algorithm: ChecksumAlgorithm, Of: ChecksumOf, Value: sum}
	var mb bytes.Buffer
	enc := json.NewEncoder(&mb)
	enc.SetEscapeHTML(false)
	enc.SetIndent("  ", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	head := func(k string, v any) ([]byte, error) {
		var b bytes.Buffer
		e := json.NewEncoder(&b)
		e.SetEscapeHTML(false)
		if err := e.Encode(v); err != nil {
			return nil, err
		}
		return []byte("  " + strconv.Quote(k) + ": " + strings.TrimRight(b.String(), "\n") + ",\n"), nil
	}
	var out bytes.Buffer
	out.WriteString("{\n")
	for _, kv := range []struct {
		k string
		v any
	}{{"format", Format}, {"version", Version}, {"kind", kind}, {"exportedAt", at.UTC().Format(time.RFC3339)}, {"editor", editor}} {
		line, err := head(kv.k, kv.v)
		if err != nil {
			return nil, err
		}
		out.Write(line)
	}
	out.WriteString("  \"manifest\": ")
	out.Write(bytes.TrimRight(mb.Bytes(), "\n"))
	out.WriteString(",\n  \"tables\": ")
	out.Write(tables)
	out.WriteString("\n}\n")
	if out.Len() > MaxFileBytes {
		return nil, fmt.Errorf("the file would be %.1f MiB; the most a base file holds is %d MiB", float64(out.Len())/(1<<20), MaxFileBytes>>20)
	}
	return out.Bytes(), nil
}

// Check reads a base file the way an importer must: whole and strictly. It refuses a file over MaxFileBytes, text that is not one JSON object, a format, version or kind that is not
// this one, a key that is not part of the file, a table or a column that is not in Tables (or one that is missing), a value of the wrong kind, more than MaxRows rows, counts that
// differ from the manifest, and a checksum that does not match the tables. It returns the file with nothing yet applied to any save. (Checking that every id a row names is in the file
// belongs to the importer, which is a later step.)
func Check(data []byte) (*File, error) {
	if len(data) > MaxFileBytes {
		return nil, fmt.Errorf("the file is larger than %d MiB: a base file is smaller", MaxFileBytes>>20)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f File
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("this is not a base file: %w", friendly(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("this is not a base file: there is more after the first JSON object")
	}
	if f.Format != Format {
		return nil, fmt.Errorf("this is not a base file (its format is %q, not %q)", clip(f.Format), Format)
	}
	if f.Version != Version {
		return nil, fmt.Errorf("this base file is version %d; this editor reads version %d", f.Version, Version)
	}
	if f.Kind != KindBackup {
		return nil, fmt.Errorf("this base file is of kind %q; this editor reads %q", clip(f.Kind), KindBackup)
	}
	if _, err := time.Parse(time.RFC3339, f.ExportedAt); err != nil {
		return nil, errors.New("the file has no valid exportedAt time")
	}
	var tables map[string]json.RawMessage
	if err := json.Unmarshal(f.Tables, &tables); err != nil {
		return nil, errors.New("the file has no tables object")
	}
	for name := range tables {
		if _, ok := Find(name); !ok {
			return nil, fmt.Errorf("the file has a table (%s) that a base file does not hold", clip(name))
		}
	}
	total := 0
	for _, t := range Tables {
		raw, ok := tables[t.Name]
		if !ok {
			return nil, fmt.Errorf("the file has no %s table", t.Name)
		}
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, fmt.Errorf("the %s table is not a list of rows", t.Name)
		}
		if total += len(rows); total > MaxRows {
			return nil, fmt.Errorf("the file holds more than %d rows", MaxRows)
		}
		if n := f.Manifest.Counts[t.Name]; n != len(rows) {
			return nil, fmt.Errorf("the manifest says %d rows in %s; the file has %d", n, t.Name, len(rows))
		}
		for i, r := range rows {
			if len(r) != len(t.Cols) {
				return nil, fmt.Errorf("row %d of %s does not have the columns of the table", i+1, t.Name)
			}
			for _, c := range t.Cols {
				cell, ok := r[c.Name]
				if !ok {
					return nil, fmt.Errorf("row %d of %s has no %s", i+1, t.Name, c.Name)
				}
				if err := c.check(cell); err != nil {
					return nil, fmt.Errorf("row %d of %s: %w", i+1, t.Name, err)
				}
			}
		}
	}
	if len(f.Manifest.Counts) != len(Tables) {
		return nil, errors.New("the manifest counts a table that a base file does not hold")
	}
	if total != f.Manifest.Summary.Rows {
		return nil, fmt.Errorf("the manifest says %d rows in all; the file has %d", f.Manifest.Summary.Rows, total)
	}
	sum, err := TablesChecksum(f.Tables)
	if err != nil {
		return nil, errors.New("the tables of the file are not valid JSON")
	}
	if f.Manifest.Checksum.Algorithm != ChecksumAlgorithm || f.Manifest.Checksum.Value != sum {
		return nil, errors.New("the checksum of the file does not match its tables: the file was changed or cut off")
	}
	return &f, nil
}

// check refuses a cell that is not of the kind of its column.
func (c Column) check(cell json.RawMessage) error {
	s := strings.TrimSpace(string(cell))
	if s == "null" {
		return nil
	}
	switch c.Kind {
	case Int:
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			return fmt.Errorf("%s is not a whole number", c.Name)
		}
	case Real:
		if f, err := strconv.ParseFloat(s, 64); err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("%s is not a finite number", c.Name)
		}
	case Text:
		var v string
		if err := json.Unmarshal(cell, &v); err != nil {
			return fmt.Errorf("%s is not text", c.Name)
		}
	case JSON:
		if !json.Valid(cell) {
			return fmt.Errorf("%s is not valid JSON", c.Name)
		}
	}
	return nil
}

func friendly(err error) error {
	var te *json.UnmarshalTypeError
	var se *json.SyntaxError
	switch {
	case errors.As(err, &te):
		return fmt.Errorf("the value of %q is of the wrong kind", te.Field)
	case errors.As(err, &se):
		return fmt.Errorf("it is not valid JSON (at byte %d)", se.Offset)
	}
	return err
}

func clip(s string) string {
	if r := []rune(s); len(r) > 40 {
		return string(r[:40]) + "..."
	}
	return s
}

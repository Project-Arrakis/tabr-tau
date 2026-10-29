package basefile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Reading the self-hosted Dune console's own base backup file (#493): format "dune-base-backup", version 1. The console writes the rows of a base that the game's tool picked up,
// from its Postgres database, as one JSON object: the header (format, version, exportedAt, the source it describes, the game build and the console that wrote it), the id of an
// owner placeholder, and entries[] of {id, kind, data}. The id of an entry is a transfer id; a column that names another row holds the transfer id of that row; a string inside a
// JSON column that names an id is written "!!act@12", with the transfer id after the @.
//
// This file turns such a file into the tables of a base file (Tables), with the transfer ids as the ids of the rows, and the result goes the same way as a file the editor wrote:
// Verify, then the import, which gives every row a new id of the save. Nothing here knows a save. The conversion is strict: a key that is not on the lists below, a kind of entry that
// is not mapped, a value of the wrong kind, an entry kind with no table in a single-player save, or a number or id that is out of place refuses the whole file, and says which.
//
// The shape of each kind is the console's (console/api/src/baseBackups.js of https://github.com/Red-Blink/dune-awakening-selfhost-docker, MIT; see the third-party notices),
// checked against a real export of version 1 (console v1.4.52): the code is written anew.

const (
	// ConsoleFormat and ConsoleVersion name the console's file.
	ConsoleFormat  = "dune-base-backup"
	ConsoleVersion = 1
)

// ConsoleInfo is what a console file says about itself, for the preview. Every text is plain (no control characters) and short; none of it is used as data.
type ConsoleInfo struct {
	Name           string         `json:"name"`
	Map            string         `json:"map"`
	TotemType      string         `json:"totemType"`
	OwnerName      string         `json:"ownerName"`
	SourceKind     string         `json:"sourceKind"` // "live-base" when the console wrote it from a base that was still standing, else empty
	BackupID       int64          `json:"backupId"`
	ConsoleVersion string         `json:"consoleVersion"`
	GameBuild      string         `json:"gameBuild"`
	SteamBuildID   string         `json:"steamBuildId"`
	GamePatches    int            `json:"gamePatches"`
	Declared       map[string]int `json:"declared"` // the counts the file says it holds: pieces, placeables, items
	Entries        int            `json:"entries"`
	Kinds          map[string]int `json:"kinds"` // entries per kind
}

// Describe is one line that says where the file came from and what it holds, as counted from the entries.
func (c *ConsoleInfo) Describe() string {
	var b strings.Builder
	b.WriteString("the Dune console")
	if c.ConsoleVersion != "" {
		b.WriteString(" (" + c.ConsoleVersion + ")")
	}
	if c.Name != "" {
		b.WriteString(", base backup \"" + c.Name + "\"")
	}
	if c.Map != "" {
		b.WriteString(", map " + c.Map)
	}
	fmt.Fprintf(&b, ", %d pieces, %d placeables, %d stored items", c.Kinds["BuildingInstance"], c.Kinds["Placeable"], c.Kinds["itm"])
	return b.String()
}

// consoleKinds are the entry kinds of the file that have a table in a single-player save, and consoleUnsupported the ones that have none: the market-era tables and the Solido
// blueprint tables, which a save does not have. A file that holds an entry of the second list is refused, because its rows cannot be put anywhere.
var (
	consoleKinds = map[string]bool{"act": true, "fgl": true, "PermissionActor": true, "PermissionActorRank": true, "inv": true, "itm": true, "ActorInventory": true,
		"Building": true, "BuildingInstance": true, "Placeable": true, "Totem": true, "BaseBackup": true, "BaseBackupLinkedActor": true, "LandclaimSegment": true}
	consoleUnsupported = map[string]string{
		"TaxInvoice":                   "tax invoices (a save has no such table)",
		"Sinkchart":                    "sinkcharts (a save has no such table)",
		"bbp":                          "Solido blueprints stored in the base (a save of this editor carries none yet)",
		"BuildingBlueprintInstance":    "Solido blueprint pieces (a save of this editor carries none yet)",
		"BuildingBlueprintPlaceable":   "Solido blueprint placeables (a save of this editor carries none yet)",
		"BuildingBlueprintPentashield": "Solido blueprint pentashields (a save of this editor carries none yet)",
	}
)

type consoleEntry struct {
	ID   int64                      `json:"id"`
	Kind string                     `json:"kind"`
	Data map[string]json.RawMessage `json:"data"`
}

type consoleEnvelope struct {
	Format                     string          `json:"format"`
	Version                    int             `json:"version"`
	ExportedAt                 string          `json:"exportedAt"`
	Source                     json.RawMessage `json:"source"`
	Game                       json.RawMessage `json:"game"`
	Console                    json.RawMessage `json:"console"`
	OwnerPlaceholderTransferID int64           `json:"ownerPlaceholderTransferId"`
	Entries                    []consoleEntry  `json:"entries"`
}

// isConsoleFile says whether the text names itself as the console's file, so that Load reads it the console's way (and says what is wrong with it in the console's terms).
func isConsoleFile(data []byte) bool {
	var h struct {
		Format string `json:"format"`
	}
	// the first value only: a file with something after it is still the console's, and is refused as such
	return json.NewDecoder(bytes.NewReader(data)).Decode(&h) == nil && h.Format == ConsoleFormat
}

func refusedConsole(format string, a ...any) error {
	return fmt.Errorf("this is the Dune console's base backup file, and it cannot be read: "+format, a...)
}

// loadConsole reads the console's file whole and strictly and returns it as a Base, with the same limits as the editor's own file.
func loadConsole(data []byte, maxBytes, maxRows int) (*Base, error) {
	if len(data) > maxBytes {
		return nil, fmt.Errorf("the file is larger than %s: a base file is smaller", sizeWords(maxBytes))
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var env consoleEnvelope
	if err := dec.Decode(&env); err != nil {
		return nil, refusedConsole("%w", friendly(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, refusedConsole("there is more after the first JSON object")
	}
	if env.Version != ConsoleVersion {
		return nil, fmt.Errorf("this is the Dune console's base backup file in version %d; this editor reads version %d", env.Version, ConsoleVersion)
	}
	if _, err := time.Parse(time.RFC3339, env.ExportedAt); err != nil {
		return nil, refusedConsole("it has no valid exportedAt time")
	}
	if env.OwnerPlaceholderTransferID <= 0 {
		return nil, refusedConsole("it has no owner placeholder")
	}
	if len(env.Entries) == 0 {
		return nil, refusedConsole("it has no entries")
	}
	if len(env.Entries) > maxRows {
		return nil, refusedConsole("it holds more than %d entries", maxRows)
	}

	cv := &converter{placeholder: env.OwnerPlaceholderTransferID, tabs: map[string]Rowset{}, ids: map[int64]string{}}
	info := cv.describe(&env)
	// The entries: unique positive ids, kinds that a save has a table for, data that is an object.
	kinds := map[string]int{}
	for i, e := range env.Entries {
		if e.ID <= 0 {
			return nil, refusedConsole("entry %d has no valid id", i+1)
		}
		if _, dup := cv.ids[e.ID]; dup {
			return nil, refusedConsole("the entry id %d is used twice", e.ID)
		}
		cv.ids[e.ID] = e.Kind
		if why, no := consoleUnsupported[e.Kind]; no {
			return nil, refusedConsole("it holds %s: %s. Nothing can be imported from this file", clip(e.Kind), why)
		}
		if !consoleKinds[e.Kind] {
			return nil, refusedConsole("it holds an entry of a kind (%s) that this editor does not read", clip(e.Kind))
		}
		if e.Data == nil {
			return nil, refusedConsole("the %s entry %d has no data", e.Kind, e.ID)
		}
		kinds[e.Kind]++
	}
	info.Entries, info.Kinds = len(env.Entries), kinds
	if cv.ids[env.OwnerPlaceholderTransferID] != "act" {
		return nil, refusedConsole("its owner placeholder (entry %d) is not an actor entry", env.OwnerPlaceholderTransferID)
	}
	if kinds["BaseBackup"] != 1 {
		return nil, refusedConsole("it must hold exactly one backup record; this one holds %d", kinds["BaseBackup"])
	}
	for _, e := range env.Entries {
		if e.Kind == "BaseBackup" {
			cv.backup = e.ID
		}
	}
	for _, e := range env.Entries {
		if err := cv.entry(e); err != nil {
			return nil, err
		}
	}
	total := 0
	for _, t := range Tables {
		total += len(cv.tabs[t.Name])
	}
	if total > maxRows {
		return nil, refusedConsole("it makes more than %d rows", maxRows)
	}
	// every cell goes through the checks of a cell of the editor's own file
	for _, t := range Tables {
		for i, r := range cv.tabs[t.Name] {
			for _, c := range t.Cols {
				cell, ok := r[c.Name]
				if !ok {
					return nil, refusedConsole("row %d of %s has no %s", i+1, t.Name, c.Name)
				}
				if err := c.check(cell); err != nil {
					return nil, refusedConsole("row %d of %s: %w", i+1, t.Name, err)
				}
			}
		}
	}
	b, err := typeTables(cv.tabs)
	if err != nil {
		return nil, refusedConsole("%w", err)
	}
	b.Hash, b.Origin, b.Console = Hash(data), OriginConsole, info
	b.File = &File{Format: ConsoleFormat, Version: ConsoleVersion, Kind: KindBackup, ExportedAt: env.ExportedAt, Editor: strings.TrimSpace("Dune console " + info.ConsoleVersion),
		Manifest: Manifest{Backup: BackupInfo{ID: info.BackupID, Name: info.Name}}}
	b.Notes = cv.notes(info)
	return b, nil
}

// converter builds the tables of a base file from the entries of a console file.
type converter struct {
	placeholder int64
	backup      int64
	ids         map[int64]string // every entry id and its kind
	tabs        map[string]Rowset

	farActors int // references "!!act#N" that name an actor of the server the file came from
}

func (cv *converter) add(table string, r map[string]json.RawMessage) {
	cv.tabs[table] = append(cv.tabs[table], r)
}

// describe reads the informational parts of the header. None of it is trusted or used as data; a part that is not what it should be is left out.
func (cv *converter) describe(env *consoleEnvelope) *ConsoleInfo {
	info := &ConsoleInfo{Declared: map[string]int{}}
	obj := func(raw json.RawMessage) map[string]any {
		var m map[string]any
		if len(raw) > 0 && json.Unmarshal(raw, &m) == nil {
			return m
		}
		return nil
	}
	str := func(v any) string {
		switch x := v.(type) {
		case string:
			return plain(x, 80)
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		}
		return ""
	}
	if s := obj(env.Source); s != nil {
		info.Name, info.Map, info.TotemType, info.OwnerName = str(s["name"]), str(s["map"]), str(s["totemType"]), str(s["ownerName"])
		if k, ok := s["kind"].(string); ok {
			info.SourceKind = plain(k, 40)
		}
		if n, ok := s["backupId"].(float64); ok && n > 0 && n < 1<<53 && n == math.Trunc(n) {
			info.BackupID = int64(n)
		}
		if c, ok := s["counts"].(map[string]any); ok {
			for _, k := range []string{"pieces", "placeables", "items"} {
				if n, ok := c[k].(float64); ok && n >= 0 && n < 1<<31 && n == math.Trunc(n) {
					info.Declared[k] = int(n)
				}
			}
		}
	}
	if g := obj(env.Game); g != nil {
		info.GameBuild, info.SteamBuildID = str(g["build"]), str(g["steamBuildId"])
		if n, ok := g["appliedPatchesCount"].(float64); ok && n >= 0 && n < 1<<31 {
			info.GamePatches = int(n)
		}
	}
	if c := obj(env.Console); c != nil {
		info.ConsoleVersion = str(c["version"])
	}
	return info
}

// notes are the things the preview should say about a console file, besides the ones the importer finds: what the header declared against what the entries are, where the file came
// from, and the references that meant nothing here.
func (cv *converter) notes(info *ConsoleInfo) []string {
	var n []string
	for _, c := range []struct{ key, kind, word string }{{"pieces", "BuildingInstance", "building pieces"}, {"placeables", "Placeable", "placeables"}, {"items", "itm", "stored items"}} {
		if d, ok := info.Declared[c.key]; ok && d != info.Kinds[c.kind] {
			n = append(n, fmt.Sprintf("The file says it holds %d %s; it holds %d.", d, c.word, info.Kinds[c.kind]))
		}
	}
	if info.GameBuild != "" {
		n = append(n, fmt.Sprintf("The file was written by a server on game build %s. A save does not say which build it was made with, so the editor cannot tell whether this save's game stores the components the same way.", info.GameBuild))
	}
	if cv.farActors > 0 {
		n = append(n, fmt.Sprintf("%d references inside the file name an actor by the id it has on the server the file came from, which means nothing here; they were set to none.", cv.farActors))
	}
	return n
}

// plain is a short text without control characters.
func plain(s string, max int) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 && r != 0x7f && r != utf8.RuneError {
			b.WriteRune(r)
		}
	}
	out := []rune(strings.TrimSpace(b.String()))
	if len(out) > max {
		out = out[:max]
	}
	return string(out)
}

// ---- one entry

// cd reads the data of one entry. The first mistake is kept and every later call is a no-op, so a builder reads its columns straight down and asks once at the end.
type cd struct {
	cv   *converter
	kind string
	id   int64
	m    map[string]json.RawMessage
	used map[string]bool
	err  error
}

func (cv *converter) data(e consoleEntry) *cd {
	return &cd{cv: cv, kind: e.Kind, id: e.ID, m: e.Data, used: map[string]bool{}}
}

func (d *cd) fail(format string, a ...any) {
	if d.err == nil {
		d.err = refusedConsole("the %s entry %d "+format, append([]any{d.kind, d.id}, a...)...)
	}
}

// raw is the value of a key, or false after a mistake (a missing key). A key is marked as read.
func (d *cd) raw(key string) (json.RawMessage, bool) {
	if d.err != nil {
		return nil, false
	}
	d.used[key] = true
	v, ok := d.m[key]
	if !ok {
		d.fail("has no %s", key)
		return nil, false
	}
	return bytes.TrimSpace(v), true
}

func isNull(v json.RawMessage) bool { return string(v) == "null" }

var (
	intLiteral = regexp.MustCompile(`^-?[0-9]+$`)
	numLiteral = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
)

func (d *cd) integer(key string, nullOK bool) json.RawMessage {
	v, ok := d.raw(key)
	if !ok {
		return nil
	}
	if isNull(v) && nullOK {
		return v
	}
	if _, err := strconv.ParseInt(string(v), 10, 64); err != nil || !intLiteral.Match(v) {
		d.fail("holds %s that is not a whole number", key)
		return nil
	}
	return v
}

func (d *cd) number(key string, nullOK bool) json.RawMessage {
	v, ok := d.raw(key)
	if !ok {
		return nil
	}
	if isNull(v) && nullOK {
		return v
	}
	return d.finite(key, v)
}

func (d *cd) finite(key string, v json.RawMessage) json.RawMessage {
	f, err := strconv.ParseFloat(string(v), 64)
	if err != nil || !numLiteral.Match(v) || math.IsNaN(f) || math.IsInf(f, 0) {
		d.fail("holds %s that is not a finite number", key)
		return nil
	}
	return v
}

// flag reads a true or false and gives the 1 or 0 a save keeps.
func (d *cd) flag(key string, nullOK bool) json.RawMessage {
	v, ok := d.raw(key)
	if !ok {
		return nil
	}
	switch string(v) {
	case "true":
		return json.RawMessage("1")
	case "false":
		return json.RawMessage("0")
	case "null":
		if nullOK {
			return v
		}
	}
	d.fail("holds %s that is not true or false", key)
	return nil
}

func (d *cd) text(key string, nullOK bool) json.RawMessage {
	v, ok := d.raw(key)
	if !ok {
		return nil
	}
	if isNull(v) && nullOK {
		return v
	}
	var s string
	if len(v) == 0 || v[0] != '"' || json.Unmarshal(v, &s) != nil {
		d.fail("holds %s that is not text", key)
		return nil
	}
	return v
}

// jsonValue reads a JSON column. The references inside it are put in the form the editor's own file uses (see refs).
func (d *cd) jsonValue(key string) json.RawMessage {
	v, ok := d.raw(key)
	if !ok {
		return nil
	}
	if len(v) == 0 || v[0] != '{' || !json.Valid(v) {
		d.fail("holds %s that is not a JSON object", key)
		return nil
	}
	var c bytes.Buffer
	if json.Compact(&c, v) != nil {
		d.fail("holds %s that is not valid JSON", key)
		return nil
	}
	out, err := d.cv.refs(c.String(), fmt.Sprintf("%s of the %s entry %d", key, d.kind, d.id))
	if err != nil {
		if d.err == nil {
			d.err = err
		}
		return nil
	}
	return json.RawMessage(out)
}

// numbers reads a key that holds a list of exactly n numbers, or null when nullOK (then nil, nil).
func (d *cd) numbers(key string, n int, nullOK bool) []json.RawMessage {
	v, ok := d.raw(key)
	if !ok {
		return nil
	}
	if isNull(v) && nullOK {
		return nil
	}
	var l []json.RawMessage
	if json.Unmarshal(v, &l) != nil || len(l) != n {
		d.fail("holds %s that is not a list of %d numbers", key, n)
		return nil
	}
	for i := range l {
		l[i] = bytes.TrimSpace(l[i])
		if d.finite(key, l[i]) == nil {
			return nil
		}
	}
	return l
}

// nestedGroup reads the member sub of an object (an actor's transform is a location and a rotation, each a group of numbers).
func (d *cd) nestedGroup(key, sub string, names ...string) []json.RawMessage {
	v, ok := d.raw(key)
	if !ok {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(v, &m) != nil || len(m) != 2 {
		d.fail("holds %s that is not a location and a rotation", key)
		return nil
	}
	inner, ok := m[sub]
	if !ok {
		d.fail("holds %s without %s", key, sub)
		return nil
	}
	return d.groupOf(key+"."+sub, bytes.TrimSpace(inner), names)
}

func (d *cd) groupOf(label string, v json.RawMessage, names []string) []json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(v, &m) != nil || len(m) != len(names) {
		d.fail("holds %s that is not a group of %s", label, strings.Join(names, ", "))
		return nil
	}
	out := make([]json.RawMessage, len(names))
	for i, n := range names {
		c, ok := m[n]
		if !ok {
			d.fail("holds %s without %s", label, n)
			return nil
		}
		if out[i] = d.finite(label+"."+n, bytes.TrimSpace(c)); out[i] == nil {
			return nil
		}
	}
	return out
}

// dropNull accepts a key that a save has no column for, only when it holds nothing (or is not there): a value in it is something this editor cannot carry.
func (d *cd) dropNull(key, why string) {
	if d.err != nil {
		return
	}
	d.used[key] = true
	if v, ok := d.m[key]; ok && !isNull(bytes.TrimSpace(v)) {
		d.fail("holds a value in %s, %s", key, why)
	}
}

// drop accepts keys that a save has no column for and that mean nothing in it: the account of the server that owned an actor, the partition it was in. They are left out.
func (d *cd) drop(keys ...string) {
	for _, k := range keys {
		d.used[k] = true
	}
}

// done asks for the mistake, if any, and refuses a key that nothing read. "__lb" is the console's note of the lower bounds of an array, which a save has none of.
func (d *cd) done() error {
	if d.err != nil {
		return d.err
	}
	var extra []string
	for k := range d.m {
		if !d.used[k] && k != "__lb" {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		d.fail("has a field (%s) that this editor does not know", clip(extra[0]))
	}
	return d.err
}

// refs puts the references of a JSON text in the form the importer reads. "!!act@12" (a transfer id of the file) becomes "!!act#12", which the importer maps to the new id like any other
// id of a file. "!!act#12" is an id of the server the file came from, which names nothing in the file: an actor becomes the null reference, any other kind refuses the file.
func (cv *converter) refs(text, where string) (string, error) {
	out, err := RewriteMarked(text, func(r Ref) (string, error) {
		s := "!!" + r.Kind + "#"
		switch {
		case r.ID < 0:
			return "", refusedConsole("%s holds the reference %s%d, which is not an id", where, s, r.ID)
		case r.ID == 0:
			return s + "0", nil
		case r.Sep == '@':
			return s + strconv.FormatInt(r.ID, 10), nil
		case r.Kind == "act":
			cv.farActors++
			return s + "0", nil
		}
		return "", refusedConsole("%s holds %s%d, an id of the server the file came from that is not in the file", where, s, r.ID)
	})
	if err != nil {
		return "", refusedConsole("%s: %w", where, err)
	}
	return out, nil
}

func itoa(n int64) json.RawMessage { return json.RawMessage(strconv.FormatInt(n, 10)) }

// row is a row of a table: the cells by column name. A nil cell is a mistake already kept by the reader.
type row = map[string]json.RawMessage

func (d *cd) cells(r row) (row, error) {
	if err := d.done(); err != nil {
		return nil, err
	}
	return r, nil
}

// entry converts one entry to its rows.
func (cv *converter) entry(e consoleEntry) error {
	d := cv.data(e)
	switch e.Kind {
	case "act":
		if e.ID == cv.placeholder {
			if len(e.Data) != 0 {
				return refusedConsole("its owner placeholder carries data")
			}
			return nil
		}
		if len(e.Data) == 0 {
			return refusedConsole("the actor entry %d has no data", e.ID)
		}
		d.drop("id", "owner_account_id", "partition_id")
		cls, mp := d.text("class", false), d.text("map", false)
		loc, rot := d.nestedGroup("transform", "location", "x", "y", "z"), d.nestedGroup("transform", "rotation", "x", "y", "z", "w")
		state := d.text("state", false)
		serial, dim := d.integer("serial", false), d.integer("dimension_index", false)
		gas, props := d.jsonValue("gas_attributes"), d.jsonValue("properties")
		if err := d.done(); err != nil {
			return err
		}
		if s := string(state); s != `"BaseBackup"` {
			return refusedConsole("the actor entry %d is in the state %s; a base backup holds actors in the state BaseBackup", e.ID, clip(s))
		}
		cv.add("actors", row{"id": itoa(e.ID), "class": cls, "map": mp, "location_x": loc[0], "location_y": loc[1], "location_z": loc[2],
			"rotation_x": rot[0], "rotation_y": rot[1], "rotation_z": rot[2], "rotation_w": rot[3], "dimension_index": dim,
			"gas_attributes": gas, "properties": props, "serial": serial, "state": itoa(StateBackup)})
	case "fgl":
		d.drop("entity_id")
		act, slot, comp := d.integer("actor_id", false), d.text("slot_name", false), d.jsonValue("components")
		r1, err := d.cells(row{"entity_id": itoa(e.ID), "components": comp})
		if err != nil {
			return err
		}
		cv.add("fgl_entities", r1)
		cv.add("actor_fgl_entities", row{"actor_id": act, "entity_id": itoa(e.ID), "slot_name": slot})
	case "PermissionActor":
		r := row{"actor_id": d.integer("actor_id", false), "actor_name": d.text("actor_name", true), "actor_type": d.integer("actor_type", false),
			"access_level": d.integer("access_level", false), "is_child": d.flag("is_child", false), "edited_by_player_id": d.integer("edited_by_player_id", true)}
		r, err := d.cells(r)
		if err != nil {
			return err
		}
		cv.add("permission_actor", r)
	case "PermissionActorRank":
		r, err := d.cells(row{"permission_actor_id": d.integer("permission_actor_id", false), "player_id": d.integer("player_id", false), "rank": d.integer("rank", false)})
		if err != nil {
			return err
		}
		cv.add("permission_actor_rank", r)
	case "inv":
		d.drop("id")
		d.dropNull("vehicle_module_id", "an inventory that belongs to a vehicle module, which a base file does not carry")
		d.dropNull("item_id", "an inventory that belongs to an item, which a base file does not carry")
		d.dropNull("exchange_id", "an inventory of the market, which a save has none of")
		r, err := d.cells(row{"id": itoa(e.ID), "actor_id": d.integer("actor_id", false), "inventory_type": d.integer("inventory_type", true),
			"max_item_count": d.integer("max_item_count", true), "max_item_volume": d.number("max_item_volume", true)})
		if err != nil {
			return err
		}
		cv.add("inventories", r)
	case "itm":
		d.drop("id")
		r, err := d.cells(row{"id": itoa(e.ID), "inventory_id": d.integer("inventory_id", false), "stack_size": d.integer("stack_size", false),
			"position_index": d.integer("position_index", false), "template_id": d.text("template_id", false), "is_new": d.flag("is_new", true),
			"acquisition_time": d.integer("acquisition_time", false), "stats": d.jsonValue("stats"), "quality_level": d.integer("quality_level", false),
			"volume_override": d.number("volume_override", true)})
		if err != nil {
			return err
		}
		cv.add("items", r)
	case "ActorInventory":
		r, err := d.cells(row{"inventory_id": d.integer("inventory_id", false), "component_name_hash": d.integer("component_name_hash", false)})
		if err != nil {
			return err
		}
		cv.add("actor_inventories", r)
	case "Building":
		r, err := d.cells(row{"id": d.integer("id", false), "owner_id": d.integer("owner_id", true)})
		if err != nil {
			return err
		}
		cv.add("buildings", r)
	case "BuildingInstance":
		t := d.numbers("transform", 7, false)
		r := row{"building_id": d.integer("building_id", false), "instance_id": d.integer("instance_id", false), "building_type": d.text("building_type", false),
			"owner_entity_id": d.integer("owner_entity_id", true), "building_flags": d.integer("building_flags", true), "health": d.number("health", false),
			"shelter": d.integer("shelter", false), "sand_buildup": d.integer("sand_buildup", false), "last_placed_by_player_id": d.integer("last_placed_by_player_id", false)}
		if d.err == nil {
			// [x, y, z, qx, qy, qz, qw]: the position, then the four numbers of the rotation, which a save keeps in plain columns
			for i, c := range []string{"location_x", "location_y", "location_z", "rotation_x", "rotation_y", "rotation_z", "rotation_w"} {
				r[c] = t[i]
			}
		}
		r, err := d.cells(r)
		if err != nil {
			return err
		}
		cv.add("building_instances", r)
	case "Placeable":
		r, err := d.cells(row{"id": d.integer("id", false), "owner_entity_id": d.integer("owner_entity_id", true), "health": d.number("health", true),
			"building_type": d.text("building_type", false), "has_hit_ground": d.flag("has_hit_ground", false), "has_buildable_support": d.flag("has_buildable_support", false),
			"is_hologram": d.flag("is_hologram", false), "last_placed_by_player_id": d.integer("last_placed_by_player_id", false)})
		if err != nil {
			return err
		}
		cv.add("placeables", r)
	case "Totem":
		loc := d.numbers("landclaim_original_global_location", 3, true)
		r := row{"id": d.integer("id", false), "landclaim_vertical_level": d.integer("landclaim_vertical_level", true), "last_backup_timestamp": d.integer("last_backup_timestamp", true),
			"landclaim_original_global_yaw_rotation": d.number("landclaim_original_global_yaw_rotation", true)}
		for i, c := range []string{"landclaim_original_global_location_x", "landclaim_original_global_location_y", "landclaim_original_global_location_z"} {
			if loc != nil {
				r[c] = loc[i]
			} else {
				r[c] = json.RawMessage("null")
			}
		}
		r, err := d.cells(r)
		if err != nil {
			return err
		}
		cv.add("totems", r)
	case "BaseBackup":
		d.drop("id")
		player := d.integer("player_id", false)
		if d.err == nil && string(player) != strconv.FormatInt(cv.placeholder, 10) {
			d.fail("names a player (%s) that is not the owner placeholder of the file", player)
		}
		r, err := d.cells(row{"id": itoa(e.ID), "player_id": player, "base_backup_name": d.text("base_backup_name", false),
			"last_edited_by_player_id": d.integer("last_edited_by_player_id", true)})
		if err != nil {
			return err
		}
		cv.add("base_backups", r)
	case "BaseBackupLinkedActor":
		id := d.integer("id", false)
		if d.err == nil && string(id) != strconv.FormatInt(cv.backup, 10) {
			d.fail("belongs to the backup %s, not to the backup record of the file", id)
		}
		r, err := d.cells(row{"id": id, "actor_id": d.integer("actor_id", false)})
		if err != nil {
			return err
		}
		cv.add("base_backup_linked_actors", r)
	case "LandclaimSegment":
		r, err := d.cells(row{"totem_id": d.integer("totem_id", false), "grid_location_x": d.integer("grid_location_x", false), "grid_location_y": d.integer("grid_location_y", false)})
		if err != nil {
			return err
		}
		cv.add("landclaim_segments", r)
	}
	return nil
}

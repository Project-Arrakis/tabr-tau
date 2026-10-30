// Command placenamesgen makes internal/ops/place-names.json (#500) from dumps of two of the game's own data tables. The file names the trading posts that the saved
// map markers (markers.marker_type "TradingPost") stand for, so that Admin > Movement > Go to a place can say "Pinnacle Station Tradepost" and not "TradingPost". It reads
// only JSON files, never the game: producing the dumps takes a reader of the game's content files, which this repository does not contain (see
// internal/catalog/PROVENANCE.md, "place-names.json").
//
// Inputs (JSON, the shape a dump of the table has: a one-element array):
//
//	-locations  DT_StaticLocationData: one row per static location, keyed by the LocationKey that a marker's payload starts with; a row has m_PackageName (the map object the
//	            location belongs to) and m_StaticLocationTags
//	-dialogue   ST_Localization_Dialogue: the game's dialogue text, keys to text (StringTable.KeysToEntries). The ornithopter pilot's destination choices in it are the
//	            names a player sees for the trading posts ("Pinnacle Station Tradepost.")
//
// The rule (repeatable, and the same on every run): every row of -locations that carries the tag Exploration.POI.TradePost and has a LocationKey of 32 hexadecimal digits is a
// trading-post key. The map object of its package name decides which place it is (the table below, one line per object). A place whose name is in the game's text takes that
// text, without the full stop the dialogue line ends with; a place whose name is not in the game's text takes the name written in the table below and is marked "derived" in
// the file. A trading-post row whose object is not in the table is an error (a new game build added a place: add a line and read the text for it), so a name is never guessed
// silently.
//
// Usage: go run ./tools/placenamesgen -locations ... -dialogue ... -build ... -read 2026-10-30 -out internal/ops/place-names.json
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// tradePostTag is the tag the game gives a trading post in DT_StaticLocationData.
const tradePostTag = "Exploration.POI.TradePost"

// place is one line of the rule: the map object a trading-post row belongs to and the name it is given.
type place struct {
	Object   string // the map object's name: the part of the row's package name after the last "/" and before the first "."
	Dialogue string // the key of the pilot's destination choice in the dialogue table; empty for a place whose name is not in the game's text
	Derived  string // the name used when the game's text does not have one
	Alias    string // another name the place is known by, matched by the filter and not shown
}

// places is the table. Pinnacle Station, Crossroads, Griffin's Reach and The Anvil are the four choices the ornithopter pilot offers (Hagga Basin); Mirzabah's Shadow is the
// name of the map the post is in, and the game calls the post The Anvil (the post's own marker text is "The Anvil", and its map object is the level of Mirzabah's Shadow). The Deep Desert
// posts (three in the table, in the level DeepDesert_1_Travel) have no name in the game's text that was found, so they are named by the map they are on.
var places = []place{
	{Object: "CB_Arrakis_Generic_M_05_PinnacleStation_MarkerName", Dialogue: "DIALOGUE/DIALOGUE_OPTION_D6TNQ"},
	{Object: "CB_Arrakis_Generic_S_02_TP_Crossroads_MarkerName", Dialogue: "DIALOGUE/DIALOGUE_OPTION_LFRWB"},
	{Object: "CB_Arrakis_Generic_S_14_TP_GriffinsReach_Gameplay", Dialogue: "DIALOGUE/DIALOGUE_OPTION_X4D2Y"},
	{Object: "CB_Trading_Post_MirzabahsShadow_Gameplay", Dialogue: "DIALOGUE/DIALOGUE_OPTION_WI65H", Alias: "Mirzabah's Shadow"},
	{Object: "DeepDesert_1_Travel", Derived: "Deep Desert Tradepost"},
}

// Place is one entry of the file.
type Place struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Alias   string `json:"alias,omitempty"`
	Derived bool   `json:"derived,omitempty"`
}

// File is the content of place-names.json.
type File struct {
	Build  string           `json:"build"`  // the game build the tables were read from
	Read   string           `json:"read"`   // the date they were read
	Tables []string         `json:"tables"` // the game tables the file was made from
	Places map[string]Place `json:"places"` // by LocationKey (32 upper-case hexadecimal digits)
}

// Inputs are the paths of the dumps and what to record in the file.
type Inputs struct {
	Locations, Dialogue, Build, Read string
}

var hexKey = regexp.MustCompile(`^[0-9A-Fa-f]{32}$`)

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

type locationRow struct {
	Package string   `json:"m_PackageName"`
	Tags    []string `json:"m_StaticLocationTags"`
}

// objectOf is the map object a package name belongs to: "/Script/X.Y /Game/Maps/CB_A.CB_A:PersistentLevel.Z_0" is "CB_A".
func objectOf(pkg string) string {
	if i := strings.LastIndex(pkg, "/"); i >= 0 {
		pkg = pkg[i+1:]
	}
	if i := strings.Index(pkg, "."); i >= 0 {
		pkg = pkg[:i]
	}
	return pkg
}

// Generate applies the rule above.
func Generate(in Inputs) (*File, error) {
	var loc []struct {
		Rows map[string]locationRow `json:"Rows"`
	}
	if err := readJSON(in.Locations, &loc); err != nil {
		return nil, err
	}
	if len(loc) != 1 || len(loc[0].Rows) == 0 {
		return nil, errors.New("the locations dump is not a one-element array with Rows")
	}
	var dlg []struct {
		StringTable struct {
			Entries map[string]string `json:"KeysToEntries"`
		} `json:"StringTable"`
	}
	if err := readJSON(in.Dialogue, &dlg); err != nil {
		return nil, err
	}
	if len(dlg) != 1 || len(dlg[0].StringTable.Entries) == 0 {
		return nil, errors.New("the dialogue dump is not a one-element array with StringTable.KeysToEntries")
	}
	text := dlg[0].StringTable.Entries

	byObject := map[string]place{}
	for _, p := range places {
		byObject[p.Object] = p
	}
	out := &File{Build: in.Build, Read: in.Read, Tables: []string{"DT_StaticLocationData", "ST_Localization_Dialogue"}, Places: map[string]Place{}}
	names := make([]string, 0, len(loc[0].Rows))
	for k := range loc[0].Rows {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, key := range names {
		row := loc[0].Rows[key]
		isPost := false
		for _, t := range row.Tags {
			if t == tradePostTag {
				isPost = true
			}
		}
		if !isPost || !hexKey.MatchString(key) { // the one row named "TradingPost" is the generic tag row, not a place
			continue
		}
		p, ok := byObject[objectOf(row.Package)]
		if !ok {
			return nil, fmt.Errorf("trading-post row %s belongs to the object %q, which the rule has no line for: add one (and read the game's text for the name)", key, objectOf(row.Package))
		}
		e := Place{Kind: "tradepost", Alias: p.Alias}
		if p.Dialogue != "" {
			t, ok := text[p.Dialogue]
			if !ok {
				return nil, fmt.Errorf("the dialogue table has no %s (the name of %s)", p.Dialogue, p.Object)
			}
			e.Name = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(t), "."))
		} else {
			e.Name, e.Derived = p.Derived, true
		}
		if e.Name == "" {
			return nil, fmt.Errorf("no name for %s", p.Object)
		}
		out.Places[strings.ToUpper(key)] = e
	}
	if len(out.Places) == 0 {
		return nil, errors.New("no trading-post row found: wrong dump?")
	}
	return out, nil
}

// Package augments knows which augments fit which items, and how an applied augment is stored on an item.
//
// The compatibility data (augment-compatibility.json) is the file of the same name in runtime/data of the Dune Awakening
// Docker console (https://github.com/Red-Blink/dune-awakening-selfhost-docker, MIT licence, copyright RedBlink; git blob
// 0c2bd7cf). Its own header says it was compiled from the augmentation database at method.gg; see
// internal/notices/THIRD-PARTY-NOTICES.md. The matching rules here follow the console's (console/web/src/lib/augmentEligibility.ts):
// an item's tags come from its in-game name (methodItems) or its template id (itemAliases), and an augment fits when one of
// its tags equals an item tag or is a prefix of it. Like the console, this only offers augments for clothing and weapons.
//
// It is data and rules only: it never imports the save.
package augments

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed augment-compatibility.json
var raw []byte

type augment struct {
	Name         string              `json:"name"`
	Tags         []string            `json:"tags"`
	GradeEffects map[string][]string `json:"gradeEffects"`
	Summary      string              `json:"effectSummary"`
}

var (
	byID        map[string]augment
	itemsByName map[string][]string // normalised in-game name -> item tags
	itemsByID   map[string][]string // normalised template id -> item tags
)

func init() {
	var d struct {
		Augments    map[string]augment  `json:"augments"`
		MethodItems map[string][]string `json:"methodItems"`
		ItemAliases map[string][]string `json:"itemAliases"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		panic("augments: augment-compatibility.json is not valid: " + err.Error())
	}
	byID = d.Augments
	itemsByName = make(map[string][]string, len(d.MethodItems))
	for k, v := range d.MethodItems {
		itemsByName[norm(k)] = v
	}
	itemsByID = make(map[string][]string, len(d.ItemAliases))
	for k, v := range d.ItemAliases {
		itemsByID[norm(k)] = v
	}
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func norm(s string) string { return nonAlnum.ReplaceAllString(strings.ToLower(s), "") }

// Kinds of item. Only clothing and weapons take augments.
const (
	KindClothing   = "clothing"
	KindWeapon     = "weapon"
	KindSchematic  = "schematic"
	KindOther      = "other"
	maxClothing    = 2
	maxWeapon      = 3
	maxAppliedSane = 20
)

var (
	clothingRe = regexp.MustCompile(`(?i)social|castoffs|garment|helmet|boots|gloves|stillsuit|still_suit|suit|top|bottom|shirt|pants|robe|cloak|hood|wearable|clothing|armor|chest|guard`)
	weaponRe   = regexp.MustCompile(`(?i)weapon|lasgun|lg\b|choamlg|spitdart|jabal|dmr|rifle|longrifle|logrifle|karpov|battle.?rifle|hark.?ar|unique.?ar|\bar\d*|br\d*|disruptor|smg|lmg|vulcan|atre.?lmg|drillshot|shotgun|scattergun|grda|pyrocket|fireball|flamethrower|rocket|missile|pistol|snubnose|rafiq|maula|sda|choamsda|uniquesda|melee|sword|blade|knife|dirk|rapier|kindjal|minotaur|dualblades|crysknife|dewreaper|ghola|hook`)
	schemaRe   = regexp.MustCompile(`(?i)schematic$`)
)

// KindOf classifies an item from its template id, in-game name and catalog category, as the console does (catalog category
// first, then the words in the id and name).
func KindOf(templateID, name, category string) string {
	text := strings.ToLower(templateID + " " + name)
	cat := strings.ToLower(category)
	if cat == "schematics" || schemaRe.MatchString(text) {
		return KindSchematic
	}
	if cat == "clothing" || clothingRe.MatchString(text) {
		return KindClothing
	}
	if cat == "weapons" || weaponRe.MatchString(text) {
		return KindWeapon
	}
	return KindOther
}

// Tags is the item's augment tags: by in-game name first, then by template id. Empty when the item is not known to take augments.
func Tags(templateID, name string) []string {
	if t := itemsByName[norm(name)]; len(t) > 0 {
		return t
	}
	return itemsByID[norm(templateID)]
}

// Limit is how many augments an item of this kind holds: 2 on clothing, 3 on a weapon, 0 otherwise.
func Limit(kind string) int {
	switch kind {
	case KindClothing:
		return maxClothing
	case KindWeapon:
		return maxWeapon
	}
	return 0
}

func tagsMatch(item, aug []string) bool {
	for _, a := range aug {
		for _, i := range item {
			if i == a || strings.HasPrefix(i, a+".") {
				return true
			}
		}
	}
	return false
}

// Option is one augment that fits an item.
type Option struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Effects []string `json:"effects"` // at the best grade (5), as the console shows them
}

// Fit says what an item takes: its kind, how many augments it holds, and the augments that fit, sorted by name. An item that
// takes none has Limit 0 and no options.
type Fit struct {
	Kind    string   `json:"kind"`
	Limit   int      `json:"limit"`
	Options []Option `json:"options"`
}

// For returns what the item takes. Items whose tags are unknown get no options, never a guess.
func For(templateID, name, category string) Fit {
	kind := KindOf(templateID, name, category)
	f := Fit{Kind: kind, Options: []Option{}}
	if Limit(kind) == 0 {
		return f
	}
	tags := Tags(templateID, name)
	if len(tags) == 0 {
		return f
	}
	f.Limit = Limit(kind)
	for id, a := range byID {
		if len(a.Tags) > 0 && tagsMatch(tags, a.Tags) {
			f.Options = append(f.Options, Option{ID: id, Name: a.Name, Effects: effects(a, 5)})
		}
	}
	sort.Slice(f.Options, func(i, j int) bool {
		if f.Options[i].Name != f.Options[j].Name {
			return f.Options[i].Name < f.Options[j].Name
		}
		return f.Options[i].ID < f.Options[j].ID
	})
	return f
}

func effects(a augment, grade int) []string {
	if e := a.GradeEffects[fmt.Sprint(grade)]; len(e) > 0 {
		return e
	}
	if e := a.GradeEffects["5"]; len(e) > 0 {
		return e
	}
	if a.Summary != "" {
		return strings.Split(a.Summary, "; ")
	}
	return []string{}
}

// Check rejects a list of augments that does not fit the item: more than it holds, duplicates, or ones that do not fit.
func Check(templateID, name, category string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	f := For(templateID, name, category)
	if f.Limit == 0 {
		return fmt.Errorf("%s does not take augments (only clothing and weapons known to the augment data do)", templateID)
	}
	if len(ids) > f.Limit {
		return fmt.Errorf("%s holds up to %d augments", templateID, f.Limit)
	}
	ok := make(map[string]bool, len(f.Options))
	for _, o := range f.Options {
		ok[o.ID] = true
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return fmt.Errorf("%s is listed twice", id)
		}
		seen[id] = true
		if !ok[id] {
			return fmt.Errorf("%s does not fit %s", id, templateID)
		}
	}
	return nil
}

// RollCount is how many stat rolls an augment has: the most effects it lists at any grade (the console's rule; checked
// against augments applied in a real save).
func RollCount(id string) int {
	a, ok := byID[id]
	if !ok {
		return 1
	}
	n := 0
	for _, e := range a.GradeEffects {
		if len(e) > n {
			n = len(e)
		}
	}
	if n == 0 && a.Summary != "" {
		n = len(strings.Split(a.Summary, "; "))
	}
	if n == 0 {
		n = 1
	}
	return n
}

// Known reports whether the augment id is in the data.
func Known(id string) bool { _, ok := byID[id]; return ok }

// Name is the in-game name of an augment id.
func Name(id string) string { return byID[id].Name }

// Applied is one augment on an item as stored: its id, quality (the grade, 1 to 5) and roll data.
type Applied struct {
	ID       string
	Quality  json.RawMessage
	RollData json.RawMessage
}

// Perfect is a new augment at a grade with every stat roll at its best, as the console writes it.
func Perfect(id string, grade int) Applied {
	rolls := make([]int, RollCount(id))
	for i := range rolls {
		rolls[i] = 1
	}
	rd, _ := json.Marshal(map[string]any{"AppliedEffectIndices": []int{}, "StatRolls": rolls})
	return Applied{ID: id, Quality: json.RawMessage(fmt.Sprint(grade)), RollData: rd}
}

// Stats returns the item stats JSON with its FAugmentedItemStats set to exactly the given augments (removed when none), every
// other part of the stats left as it was. The keys come out in the game's own (alphabetical) order.
func Stats(stats string, applied []Applied) (string, error) {
	if len(applied) > maxAppliedSane {
		return "", fmt.Errorf("at most %d augments", maxAppliedSane)
	}
	doc := map[string]json.RawMessage{}
	if strings.TrimSpace(stats) != "" {
		if err := json.Unmarshal([]byte(stats), &doc); err != nil {
			return "", fmt.Errorf("the item's stats are not readable: %w", err)
		}
	}
	if len(applied) == 0 {
		delete(doc, "FAugmentedItemStats")
	} else {
		names := make([]map[string]string, len(applied))
		quals := make([]json.RawMessage, len(applied))
		rolls := make([]json.RawMessage, len(applied))
		for i, a := range applied {
			names[i] = map[string]string{"Name": a.ID}
			quals[i] = a.Quality
			rolls[i] = a.RollData
		}
		inner, err := json.Marshal(map[string]any{"AppliedAugments": names, "AppliedAugmentQualities": quals, "AppliedAugmentRollData": rolls})
		if err != nil {
			return "", err
		}
		doc["FAugmentedItemStats"] = json.RawMessage("[[]," + string(inner) + "]")
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ReadApplied reads the augments already on an item.
func ReadApplied(stats string) []Applied {
	var doc map[string][]json.RawMessage
	if json.Unmarshal([]byte(stats), &doc) != nil {
		return nil
	}
	pair := doc["FAugmentedItemStats"]
	if len(pair) < 2 {
		return nil
	}
	var d struct {
		Names []struct {
			Name string `json:"Name"`
		} `json:"AppliedAugments"`
		Quals []json.RawMessage `json:"AppliedAugmentQualities"`
		Rolls []json.RawMessage `json:"AppliedAugmentRollData"`
	}
	if json.Unmarshal(pair[1], &d) != nil {
		return nil
	}
	out := make([]Applied, 0, len(d.Names))
	for i, n := range d.Names {
		a := Applied{ID: n.Name, Quality: json.RawMessage("1"), RollData: json.RawMessage(`{"AppliedEffectIndices":[],"StatRolls":[1]}`)}
		if i < len(d.Quals) {
			a.Quality = d.Quals[i]
		}
		if i < len(d.Rolls) {
			a.RollData = d.Rolls[i]
		}
		out = append(out, a)
	}
	return out
}
